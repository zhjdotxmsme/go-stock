// 量能类指标：OBV / MFI / CMF / ADLine / ForceIndex / ChaikinOsc。
//
// 移植基准：frontend/src/components/kline/calc.ts（同名函数、相同默认参数、相同运算顺序）。
// 约定：返回序列与输入等长，预热期（JS null）填 math.NaN()；
// 例外：OBV 与 ADLine 为累计序列，JS 自首根起即有值，无预热期（全序列有效）。

package indicator

import "math"

// volOr0 对应 JS `vols[i] || 0`：NaN/0 归一为 0，其余原样（JS 中 0、NaN 均为假值）。
func volOr0(v float64) float64 {
	if v == 0 || math.IsNaN(v) {
		return 0
	}
	return v
}

// OBV 能量潮，移植自 JS obvValues（calc.ts L124）。
// 首根 = volume[0]（经 || 0 归一），其后按收盘涨跌累加/累减当根成交量。
// 无预热期：与输入等长且全有效。
func OBV(close, volume []float64) []float64 {
	n := len(close)
	if n == 0 {
		return []float64{}
	}
	out := make([]float64, n)
	obv := volOr0(volume[0])
	out[0] = obv
	for i := 1; i < n; i++ {
		ch := close[i] - close[i-1]
		if ch > 0 {
			obv += volOr0(volume[i])
		} else if ch < 0 {
			obv -= volOr0(volume[i])
		}
		out[i] = obv
	}
	return out
}

// MFI 资金流量指标，移植自 JS mfiValues（calc.ts L240，默认 period=14）。
// tp=(h+l+c)/3，mf=tp*volume；对 i>=period 统计窗口内 tp 升/降的 mf 和，
// negMF==0 时输出 100。前 period 位（及 len<2 时全序列）为 NaN。
func MFI(high, low, close, volume []float64, period int) []float64 {
	n := len(close)
	out := nanSlice(n)
	if n < 2 {
		return out
	}
	tp := make([]float64, n)
	mf := make([]float64, n)
	for i := 0; i < n; i++ {
		tp[i] = (high[i] + low[i] + close[i]) / 3
		mf[i] = tp[i] * volume[i]
	}
	for i := period; i < n; i++ {
		posMF, negMF := 0.0, 0.0
		for j := 0; j < period; j++ {
			idx := i - j
			if tp[idx] > tp[idx-1] {
				posMF += mf[idx]
			} else if tp[idx] < tp[idx-1] {
				negMF += mf[idx]
			}
		}
		if negMF == 0 {
			out[i] = 100
		} else {
			out[i] = 100 - 100/(1+posMF/negMF)
		}
	}
	return out
}

// CMF 资金流量（Chaikin Money Flow），移植自 JS cmfValues（calc.ts L583，默认 period=20）。
// 窗口内 MFV=((c-l)-(h-c))/(h-l)*volume（h==l 时 MFV=0）求和除以成交量之和；
// 量和为 0 时该位 NaN。前 period-1 位 NaN。
func CMF(high, low, close, volume []float64, period int) []float64 {
	n := len(close)
	out := nanSlice(n)
	for i := period - 1; i < n; i++ {
		sumMFV, sumVol := 0.0, 0.0
		for j := 0; j < period; j++ {
			idx := i - j
			rng := high[idx] - low[idx]
			mfv := 0.0
			if rng > 0 {
				mfv = ((close[idx] - low[idx]) - (high[idx] - close[idx])) / rng * volume[idx]
			}
			sumMFV += mfv
			sumVol += volume[idx]
		}
		if sumVol > 0 {
			out[i] = sumMFV / sumVol
		}
	}
	return out
}

// ADLine 累积/派发线，移植自 JS adValues（calc.ts L929）。
// 逐根累计 MFV（乘数 = ((c-l)-(h-c))/(h-l)，h==l 时为 0），JS 自首根起即有值。
// 无预热期：与输入等长且全有效。
func ADLine(high, low, close, volume []float64) []float64 {
	n := len(close)
	if n == 0 {
		return []float64{}
	}
	ad := make([]float64, n)
	for i := 0; i < n; i++ {
		rng := high[i] - low[i]
		mfm := 0.0
		if rng > 0 {
			mfm = ((close[i] - low[i]) - (high[i] - close[i])) / rng
		}
		mfv := mfm * volOr0(volume[i])
		prev := 0.0
		if i > 0 {
			prev = ad[i-1]
		}
		ad[i] = prev + mfv
	}
	return ad
}

// ForceIndex 力量指数，移植自 JS forceIndexValues（calc.ts L634，默认 period=13）。
// raw[0]=0，raw[i]=(close[i]-close[i-1])*volume[i]，再取 EMA（JS emaFinite）。
// len<2 时全 NaN；EMA 种子前为 NaN。
func ForceIndex(close, volume []float64, period int) []float64 {
	n := len(close)
	if n < 2 {
		return nanSlice(n)
	}
	raw := make([]float64, n)
	raw[0] = 0
	for i := 1; i < n; i++ {
		raw[i] = (close[i] - close[i-1]) * volume[i]
	}
	return EMA(raw, period)
}

// ChaikinOsc 蔡金震荡器，移植自 JS chaikinOscValues（calc.ts L1043，
// 默认 fastPeriod=3, slowPeriod=10）。
// ADLine 的快慢 EMA 之差（对 ad 序列做 JS 的 isFinite 过滤后取 emaFinite）；
// 任一 EMA 为预热期时该位 NaN。
func ChaikinOsc(high, low, close, volume []float64, fast, slow int) []float64 {
	n := len(close)
	ad := ADLine(high, low, close, volume)
	adClean := make([]float64, n) // 对应 JS ad.map(v => Number.isFinite(v) ? v : NaN)
	for i, v := range ad {
		if !isFinite(v) {
			adClean[i] = math.NaN()
		} else {
			adClean[i] = v
		}
	}
	fastEma := EMA(adClean, fast)
	slowEma := EMA(adClean, slow)
	out := nanSlice(n)
	for i := 0; i < n; i++ {
		if !math.IsNaN(fastEma[i]) && !math.IsNaN(slowEma[i]) {
			out[i] = fastEma[i] - slowEma[i]
		}
	}
	return out
}
