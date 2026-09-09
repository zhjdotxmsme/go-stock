package indicator

// 本文件移植 frontend/src/components/kline/calc.ts 的动量族函数：
//   adxValues (L472)、aroonValues (L601)、cmoValues (L618)、rocValues (L964)、
//   trixValues (L945)、aoValues (L902)、smiValues (L1156)。
// 逐行忠实移植：相同默认参数、相同算法（含预热期 NaN 约定，对应 JS 的 null）。

import "math"

// dxOrZero 对应 JS 的 `dx[i] || 0`（null/NaN 等假值归零）。
func dxOrZero(v float64) float64 {
	if isFinite(v) {
		return v
	}
	return 0
}

// ADX 对应 JS adxValues（calc.ts L472），默认 period=14。
// Wilder 平滑的 ADX / +DI / -DI；len(close) < 2 时三条序列全 NaN。
// dx 汇总处 JS 使用 `dx[i] || 0`，对应 dxOrZero。
func ADX(high, low, close []float64, period int) (adx, plusDI, minusDI []float64) {
	n := len(close)
	adx = nanSlice(n)
	plusDI = nanSlice(n)
	minusDI = nanSlice(n)
	if n < 2 {
		return
	}
	tr := make([]float64, n)
	plusDM := make([]float64, n)
	minusDM := make([]float64, n)
	tr[0] = high[0] - low[0]
	for i := 1; i < n; i++ {
		tr[i] = math.Max(high[i]-low[i],
			math.Max(math.Abs(high[i]-close[i-1]), math.Abs(low[i]-close[i-1])))
		upMove := high[i] - high[i-1]
		downMove := low[i-1] - low[i]
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
	}
	smoothTR := nanSlice(n)
	smoothPDM := nanSlice(n)
	smoothMDM := nanSlice(n)
	var sTR, sPDM, sMDM float64
	for i := 0; i < period && i < n; i++ {
		sTR += tr[i]
		sPDM += plusDM[i]
		sMDM += minusDM[i]
	}
	if n >= period {
		smoothTR[period-1] = sTR
		smoothPDM[period-1] = sPDM
		smoothMDM[period-1] = sMDM
		for i := period; i < n; i++ {
			smoothTR[i] = smoothTR[i-1] - smoothTR[i-1]/float64(period) + tr[i]
			smoothPDM[i] = smoothPDM[i-1] - smoothPDM[i-1]/float64(period) + plusDM[i]
			smoothMDM[i] = smoothMDM[i-1] - smoothMDM[i-1]/float64(period) + minusDM[i]
		}
	}
	dx := nanSlice(n)
	for i := 0; i < n; i++ {
		if isFinite(smoothTR[i]) && smoothTR[i] > 0 {
			plusDI[i] = 100 * smoothPDM[i] / smoothTR[i]
			minusDI[i] = 100 * smoothMDM[i] / smoothTR[i]
			sum := plusDI[i] + minusDI[i]
			if sum > 0 {
				dx[i] = 100 * math.Abs(plusDI[i]-minusDI[i]) / sum
			} else {
				dx[i] = 0
			}
		}
	}
	if n >= period*2-1 {
		var sumDx float64
		for i := period - 1; i < period*2-1 && i < n; i++ {
			sumDx += dxOrZero(dx[i])
		}
		adx[period*2-2] = sumDx / float64(period)
		for i := period*2 - 1; i < n; i++ {
			adx[i] = (adx[i-1]*float64(period-1) + dxOrZero(dx[i])) / float64(period)
		}
	}
	return
}

// Aroon 对应 JS aroonValues（calc.ts L601），默认 period=25。
// up/down 分别衡量窗口内最高/最低价的新近度（0~100）；前 period-1 位为 NaN。
func Aroon(high, low []float64, period int) (up, down []float64) {
	n := len(high)
	up = nanSlice(n)
	down = nanSlice(n)
	for i := period - 1; i < n; i++ {
		highIdx, lowIdx := 0, 0
		for j := 1; j < period; j++ {
			if high[i-j] > high[i-highIdx] {
				highIdx = j
			}
			if low[i-j] < low[i-lowIdx] {
				lowIdx = j
			}
		}
		up[i] = float64(period-1-highIdx) / float64(period-1) * 100
		down[i] = float64(period-1-lowIdx) / float64(period-1) * 100
	}
	return
}

// CMO 对应 JS cmoValues（calc.ts L618），默认 period=14。
// Chande 动量振荡器；前 period 位为 NaN；分母为 0 时按 JS 输出 0。
func CMO(close []float64, period int) []float64 {
	n := len(close)
	out := nanSlice(n)
	for i := period; i < n; i++ {
		var sumUp, sumDown float64
		for j := 0; j < period; j++ {
			diff := close[i-j] - close[i-j-1]
			if diff > 0 {
				sumUp += diff
			} else {
				sumDown -= diff
			}
		}
		if sumUp+sumDown > 0 {
			out[i] = (sumUp - sumDown) / (sumUp + sumDown) * 100
		} else {
			out[i] = 0
		}
	}
	return out
}

// ROC 对应 JS rocValues（calc.ts L964），默认 period=12。
// 变化率（百分比）；前 period 位为 NaN；基准价为 0 时按 JS 保持 NaN。
func ROC(close []float64, period int) []float64 {
	n := len(close)
	out := nanSlice(n)
	for i := period; i < n; i++ {
		if close[i-period] != 0 {
			out[i] = (close[i] - close[i-period]) / close[i-period] * 100
		}
	}
	return out
}

// TRIX 对应 JS trixValues（calc.ts L945），默认 period=15。
// 三重 EMA 的变化率（万分比）。JS 中将上一层结果的 null 映射为 NaN 再喂给
// emaFinite；本包 EMA 本身以 NaN 表示空位，故直接级联即可，语义完全一致。
func TRIX(close []float64, period int) []float64 {
	ema1 := EMA(close, period)
	ema2 := EMA(ema1, period)
	ema3 := EMA(ema2, period)
	out := nanSlice(len(close))
	for i := 1; i < len(close); i++ {
		if isFinite(ema3[i]) && isFinite(ema3[i-1]) && ema3[i-1] != 0 {
			out[i] = (ema3[i] - ema3[i-1]) / ema3[i-1] * 10000
		}
	}
	return out
}

// AO 对应 JS aoValues（calc.ts L902），默认 fast=5, slow=34。
// Awesome Oscillator：中价 (high+low)/2 的快慢简单均线之差。
func AO(high, low []float64, fast, slow int) []float64 {
	n := len(high)
	mid := make([]float64, n)
	for i := 0; i < n; i++ {
		mid[i] = (high[i] + low[i]) / 2
	}
	fastSma := smaValues(mid, fast)
	slowSma := smaValues(mid, slow)
	out := nanSlice(n)
	for i := 0; i < n; i++ {
		if isFinite(fastSma[i]) && isFinite(slowSma[i]) {
			out[i] = fastSma[i] - slowSma[i]
		}
	}
	return out
}

// SMI 对应 JS smiValues（calc.ts L1156），默认 kPeriod=14, dPeriod=3, emaPeriod=3。
// Stochastic Momentum Index 主线。JS 原函数同时返回 { smi, signal } 两条线；
// 按本包 API 契约仅导出 smi 主线，signal 线由 smiBundle 提供（黄金值测试同时覆盖两条线）。
func SMI(high, low, close []float64, kPeriod, dPeriod, emaPeriod int) []float64 {
	smi, _ := smiBundle(high, low, close, kPeriod, dPeriod, emaPeriod)
	return smi
}

// smiBundle 为 JS smiValues 的完整移植，返回 (smi 主线, signal 信号线)。
// JS 中 rawSMI/smiLine 的 null 映射为 NaN 后喂给 emaFinite；本包 EMA 以 NaN
// 表示空位，直接级联即可，语义一致。
func smiBundle(high, low, close []float64, kPeriod, dPeriod, emaPeriod int) (smi, signal []float64) {
	n := len(close)
	highest := nanSlice(n)
	lowest := nanSlice(n)
	for i := kPeriod - 1; i < n; i++ {
		hi := math.Inf(-1)
		lo := math.Inf(1)
		for j := 0; j < kPeriod; j++ {
			if high[i-j] > hi {
				hi = high[i-j]
			}
			if low[i-j] < lo {
				lo = low[i-j]
			}
		}
		highest[i] = hi
		lowest[i] = lo
	}
	raw := nanSlice(n)
	for i := 0; i < n; i++ {
		if isFinite(highest[i]) && isFinite(lowest[i]) {
			r := highest[i] - lowest[i]
			if r != 0 {
				raw[i] = 200 * ((close[i] - (highest[i]+lowest[i])/2) / r)
			}
		}
	}
	smi = EMA(raw, emaPeriod)
	signal = EMA(smi, dPeriod)
	return
}
