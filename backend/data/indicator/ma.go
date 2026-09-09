package indicator

// 本文件移植 frontend/src/components/kline/calc.ts 的均线族函数：
//   smaValues (L1)、emaFinite (L15)、weightedMaValues (L84)、
//   hullMaValues (L916)、kamaValues (L259)。
// 逐行忠实移植：相同默认参数、相同算法（含预热期 NaN 约定，对应 JS 的 null）。

import "math"

// isFinite 对应 JS 的 Number.isFinite：非 NaN 且非 ±Inf。
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// nanSlice 返回长度 n 的全 NaN 切片（对应 JS 的 new Array(len).fill(null)）。
func nanSlice(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

// smaValues 对应 JS smaValues（calc.ts L1）。简单移动平均。
// 前 period-1 位为 NaN；包内私有，供 AO 等组合使用（黄金值测试覆盖）。
func smaValues(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	for i := period - 1; i < len(values); i++ {
		var s float64
		for j := 0; j < period; j++ {
			s += values[i-j]
		}
		out[i] = s / float64(period)
	}
	return out
}

// EMA 对应 JS emaFinite（calc.ts L15）：有限值种子版指数移动平均。
// 种子取“下标 >= period-1 且窗口 [i-period+1, i] 全为有限值”的首个窗口均值；
// 种子后遇到非有限输入跳过更新（该位输出 NaN，EMA 状态保持，与 JS 一致）。
func EMA(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	k := 2 / float64(period+1)
	seeded := false
	var ema float64
	for i, v := range values {
		if !isFinite(v) {
			continue // out[i] 保持 NaN
		}
		if !seeded {
			if i < period-1 {
				continue
			}
			var s float64
			ok := true
			for j := i - period + 1; j <= i; j++ {
				if !isFinite(values[j]) {
					ok = false
					break
				}
				s += values[j]
			}
			if !ok {
				continue // 窗口含非有限值：本位 NaN，种子顺延重试
			}
			ema = s / float64(period)
			seeded = true
			out[i] = ema
			continue
		}
		ema = v*k + ema*(1-k)
		out[i] = ema
	}
	return out
}

// weightedMaValues 对应 JS weightedMaValues（calc.ts L84）。线性加权移动平均：
// 窗口内最旧权重 1、最新权重 period，分母 period*(period+1)/2；
// 窗口内任一值非有限则该位 NaN。包内私有，供 HMA 使用（黄金值测试覆盖）。
func weightedMaValues(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	denom := float64(period) * float64(period+1) / 2
	for i := period - 1; i < len(values); i++ {
		var sum float64
		ok := true
		for j := 0; j < period; j++ {
			v := values[i-period+1+j]
			if !isFinite(v) {
				ok = false
				break
			}
			sum += v * float64(j+1)
		}
		if ok {
			out[i] = sum / denom
		}
	}
	return out
}

// HMA 对应 JS hullMaValues（calc.ts L916），默认 period=9。
// HMA = WMA(2*WMA(close, floor(period/2)) - WMA(close, period), floor(sqrt(period)))，
// 中间差序列的空位以 NaN 参与 WMA（窗口含 NaN 则该位 NaN，与 JS null 传播一致）。
func HMA(close []float64, period int) []float64 {
	halfLen := period / 2 // 对正整数即 JS Math.floor(period/2)
	sqrtLen := int(math.Sqrt(float64(period)))
	wmaHalf := weightedMaValues(close, halfLen)
	wmaFull := weightedMaValues(close, period)
	diff := nanSlice(len(close))
	for i := range diff {
		if isFinite(wmaHalf[i]) && isFinite(wmaFull[i]) {
			diff[i] = 2*wmaHalf[i] - wmaFull[i]
		}
	}
	return weightedMaValues(diff, sqrtLen)
}

// KAMA 对应 JS kamaValues（calc.ts L259），默认 period=10, fast=2, slow=30。
// Kaufman 自适应移动平均；种子为 closes[period]（与 JS 一致，非窗口均值）；
// len(close) < period+1 时全 NaN。
func KAMA(close []float64, period, fast, slow int) []float64 {
	n := len(close)
	out := nanSlice(n)
	if n < period+1 {
		return out
	}
	fastSC := 2 / float64(fast+1)
	slowSC := 2 / float64(slow+1)
	kama := close[period]
	out[period] = kama
	for i := period + 1; i < n; i++ {
		direction := math.Abs(close[i] - close[i-period])
		var volatility float64
		for j := 0; j < period; j++ {
			volatility += math.Abs(close[i-j] - close[i-j-1])
		}
		er := 0.0
		if volatility > 0 {
			er = direction / volatility
		}
		sc := math.Pow(er*(fastSC-slowSC)+slowSC, 2)
		kama += sc * (close[i] - kama)
		out[i] = kama
	}
	return out
}
