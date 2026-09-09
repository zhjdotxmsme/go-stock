// 通道类指标：Keltner / Donchian / SuperTrend / TTMSqueeze，
// 以及 system.go / volume.go 共用的内部移植工具（atrValues / bollingerBands / smaNullAsZero）。
//
// 移植基准：frontend/src/components/kline/calc.ts（同名函数、相同默认参数、相同运算顺序）。
// 约定：返回序列与输入等长，预热期（JS null）填 math.NaN()；bool 序列无信号期为 false。
// 复用 ma.go 的包内私有工具 isFinite / nanSlice / EMA（语义见其文档）。

package indicator

import "math"

// smaNullAsZero 对应 JS smaValues（calc.ts L1）在“输入含 null”时的真实算术语义：
// JS 里 null + number 中 null 被强转为 +0，即 null 位按 0 计入窗口和再除以 period
// （不是跳过、也不是传播 NaN）。对全有限输入与普通 SMA 完全一致。
// 前 period-1 位为 NaN。供 bollingerBands、SATS 的 atrBase 等使用。
func smaNullAsZero(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	for i := period - 1; i < len(values); i++ {
		var s float64
		for j := 0; j < period; j++ {
			if v := values[i-j]; isFinite(v) {
				s += v
			}
		}
		out[i] = s / float64(period)
	}
	return out
}

// atrValues 对应 JS atrValues（calc.ts L198，默认 period=14）。
// tr[0]=high-low；其后 tr=max(h-l, |h-prevC|, |l-prevC|)；
// 首个值 = 前 period 根 tr 的简单和均值（下标 period-1），之后 Wilder 平滑。
// len<2 或 len<period 时全 NaN。
func atrValues(high, low, close []float64, period int) []float64 {
	n := len(close)
	out := nanSlice(n)
	if n < 2 {
		return out
	}
	tr := make([]float64, n)
	tr[0] = high[0] - low[0]
	for i := 1; i < n; i++ {
		tr[i] = math.Max(high[i]-low[i],
			math.Max(math.Abs(high[i]-close[i-1]), math.Abs(low[i]-close[i-1])))
	}
	var sum float64
	for i := 0; i < period && i < n; i++ {
		sum += tr[i]
	}
	if n >= period {
		out[period-1] = sum / float64(period)
		for i := period; i < n; i++ {
			out[i] = (out[i-1]*float64(period-1) + tr[i]) / float64(period)
		}
	}
	return out
}

// bollingerBands 对应 JS bollingerBands（calc.ts L101，BOLL 旧实现不重复导出，
// 仅供 TTMSqueeze 内联调用逻辑）。mid=smaNullAsZero(close, period)；
// std 为总体标准差（除以 period）；upper=mid+mult*std，lower=mid-mult*std。
func bollingerBands(close []float64, period int, mult float64) (mid, upper, lower []float64) {
	mid = smaNullAsZero(close, period)
	n := len(close)
	upper, lower = nanSlice(n), nanSlice(n)
	for i := period - 1; i < n; i++ {
		m := mid[i]
		var sumSq float64
		for j := 0; j < period; j++ {
			d := close[i-j] - m
			sumSq += d * d
		}
		std := math.Sqrt(sumSq / float64(period))
		upper[i] = m + mult*std
		lower[i] = m - mult*std
	}
	return mid, upper, lower
}

// Keltner 肯特纳通道，移植自 JS keltnerChannelValues（calc.ts L281，
// 默认 emaPeriod=20, atrPeriod=10, mult=1.5）。
// mid=EMA(close, emaPeriod)；upper/lower=mid±mult*ATR(atrPeriod)；
// mid 或 ATR 任一为预热期时该位 upper/lower 为 NaN（mid/ATR 各自保留）。
func Keltner(high, low, close []float64, emaPeriod, atrPeriod int, mult float64) (mid, upper, lower []float64) {
	mid = EMA(close, emaPeriod)
	atr := atrValues(high, low, close, atrPeriod)
	n := len(close)
	upper, lower = nanSlice(n), nanSlice(n)
	for i := 0; i < n; i++ {
		if !math.IsNaN(mid[i]) && !math.IsNaN(atr[i]) {
			upper[i] = mid[i] + mult*atr[i]
			lower[i] = mid[i] - mult*atr[i]
		}
	}
	return mid, upper, lower
}

// Donchian 唐奇安通道，移植自 JS donchianChannelValues（calc.ts L453，默认 period=20）。
// upper/lower 为窗口内最高高/最低低，mid=(upper+lower)/2；前 period-1 位 NaN。
func Donchian(high, low []float64, period int) (upper, mid, lower []float64) {
	n := len(high)
	upper, mid, lower = nanSlice(n), nanSlice(n), nanSlice(n)
	for i := period - 1; i < n; i++ {
		hi, lo := math.Inf(-1), math.Inf(1)
		for j := 0; j < period; j++ {
			if high[i-j] > hi {
				hi = high[i-j]
			}
			if low[i-j] < lo {
				lo = low[i-j]
			}
		}
		upper[i] = hi
		lower[i] = lo
		mid[i] = (hi + lo) / 2
	}
	return upper, mid, lower
}

// SuperTrend 超级趋势，移植自 JS supertrendValues（calc.ts L298，
// 默认 atrPeriod=10, multiplier=3）。
// 返回 line（趋势线：多头取下轨、空头取上轨；ATR 预热期为 NaN）与
// bull（JS direction==1 → true；预热期 direction=0 → false）。
// 轨道锁定（rawUpper/rawLower 被 prevUpper/prevLower 钳制）的条件
// `closes[i-1] <= prevUpper` 等按 JS 原样移植。
func SuperTrend(high, low, close []float64, atrPeriod int, mult float64) (line []float64, bull []bool) {
	n := len(close)
	line = nanSlice(n)
	bull = make([]bool, n)
	atr := atrValues(high, low, close, atrPeriod)
	var prevUpper, prevLower float64
	haveBands := false
	prevDir := 0
	for i := 0; i < n; i++ {
		if math.IsNaN(atr[i]) {
			continue // 对应 JS：supertrend[i]/direction[i] 保持 null/0
		}
		hl2 := (high[i] + low[i]) / 2
		rawUpper := hl2 + mult*atr[i]
		rawLower := hl2 - mult*atr[i]
		if haveBands && rawUpper >= prevUpper && close[i-1] <= prevUpper {
			rawUpper = prevUpper
		}
		if haveBands && rawLower <= prevLower && close[i-1] >= prevLower {
			rawLower = prevLower
		}
		var dir int
		switch {
		case prevDir == 0:
			dir = 1
		case prevDir == 1:
			if close[i] < rawLower {
				dir = -1
			} else {
				dir = 1
			}
		default:
			if close[i] > rawUpper {
				dir = 1
			} else {
				dir = -1
			}
		}
		upperBand, lowerBand := rawUpper, rawLower
		if dir == 1 {
			line[i] = lowerBand
		} else {
			line[i] = upperBand
		}
		bull[i] = dir == 1
		prevUpper, prevLower = upperBand, lowerBand
		prevDir = dir
		haveBands = true
	}
	return line, bull
}

// TTMSqueeze TTM 挤压，移植自 JS ttmSqueezeValues（calc.ts L385，
// 默认 bollPeriod=20, bollMult=2, keltnerPeriod=20, keltnerAtrPeriod=10, keltnerMult=1.5）。
// 注意 Go 签名先收齐 int 参数再收 float 参数，与 JS 参数顺序不同，按名对应：
// bollPeriod↔bollPeriod、keltnerPeriod↔keltnerPeriod、keltnerAtrPeriod↔keltnerAtrPeriod、
// bollMult↔bollMult、keltnerMult↔keltnerMult。
// squeezeOn = BOLL 下轨 ≥ Keltner 下轨 且 BOLL 上轨 ≤ Keltner 上轨（任一为预热期则 false）；
// momentum = tp - EMA(tp, bollPeriod)（tp=(h+l+c)/3；EMA 预热期为 NaN）。
func TTMSqueeze(high, low, close []float64, bollPeriod, keltnerPeriod, keltnerAtrPeriod int, bollMult, keltnerMult float64) (squeezeOn []bool, momentum []float64) {
	n := len(close)
	_, bollUpper, bollLower := bollingerBands(close, bollPeriod, bollMult)
	_, kUpper, kLower := Keltner(high, low, close, keltnerPeriod, keltnerAtrPeriod, keltnerMult)
	squeezeOn = make([]bool, n)
	momentum = nanSlice(n)
	for i := 0; i < n; i++ {
		if !math.IsNaN(bollLower[i]) && !math.IsNaN(kLower[i]) {
			squeezeOn[i] = bollLower[i] >= kLower[i] && bollUpper[i] <= kUpper[i]
		}
	}
	tp := make([]float64, n)
	for i := 0; i < n; i++ {
		tp[i] = (high[i] + low[i] + close[i]) / 3
	}
	emaTp := EMA(tp, bollPeriod)
	for i := 0; i < n; i++ {
		if !math.IsNaN(emaTp[i]) {
			momentum[i] = tp[i] - emaTp[i]
		}
	}
	return squeezeOn, momentum
}
