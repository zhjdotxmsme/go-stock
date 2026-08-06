package scoring

import "math"

// 本文件为因子所需技术指标的纯函数实现。
// 算法移植自 backend/data/tool_indicator.go（calcMACD/calcRSI/calcATR/calcEMA/calcSMA），
// 复制到本包内部是为了避免 scoring 依赖带大量 init() 副作用的 data 包，保持纯函数可测试。

// sma 最近 period 个值的简单平均，不足返回 0。
func sma(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}
	start := len(data) - period
	sum := 0.0
	for i := start; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(period)
}

// ema 指数移动平均（种子为前 period 个值的 SMA）。
func ema(data []float64, period int) float64 {
	n := len(data)
	if n < period {
		return 0
	}
	multiplier := 2.0 / (float64(period) + 1)
	e := sma(data[:period], period)
	for i := period; i < n; i++ {
		e = (data[i]-e)*multiplier + e
	}
	return e
}

// macd 计算 MACD 指标，返回 (DIF, DEA/signal, 柱状图)。算法与 data.calcMACD 一致。
func macd(close []float64, fast, slow, signal int) (dif, dea, histogram float64) {
	n := len(close)
	if n < slow {
		return 0, 0, 0
	}
	dif = ema(close, fast) - ema(close, slow)

	// 构建 DIF 历史用于计算 signal 线
	macdHistory := make([]float64, n)
	emaF, emaS := 0.0, 0.0
	for i := 0; i < n; i++ {
		if i == 0 {
			emaF = close[i]
			emaS = close[i]
		} else {
			emaF = emaF + (2.0/(float64(fast)+1))*(close[i]-emaF)
			emaS = emaS + (2.0/(float64(slow)+1))*(close[i]-emaS)
		}
		macdHistory[i] = emaF - emaS
	}
	dea = ema(macdHistory, signal)
	histogram = dif - dea
	return dif, dea, histogram
}

// rsi 计算 RSI 指标（简单平均法，与 data.calcRSI 一致），数据不足返回 50。
func rsi(data []float64, period int) float64 {
	n := len(data)
	if n < period+1 {
		return 50
	}
	gains, losses := 0.0, 0.0
	for i := n - period; i < n; i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// atr 计算平均真实波幅（与 data.calcATR 一致），数据不足返回 0。
func atr(high, low, close []float64, period int) float64 {
	n := len(close)
	if n < period+1 {
		return 0
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		tr := math.Max(high[i]-low[i], math.Abs(high[i]-close[i-1]))
		tr = math.Max(tr, math.Abs(low[i]-close[i-1]))
		sum += tr
	}
	return sum / float64(period)
}

// volatility 日收益率年化波动率（%，std × sqrt(252) × 100）。
// closes 时间升序，不足 2 个返回 0。
func volatility(closes []float64) float64 {
	n := len(closes)
	if n < 2 {
		return 0
	}
	returns := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		if closes[i-1] <= 0 {
			continue
		}
		returns = append(returns, (closes[i]-closes[i-1])/closes[i-1])
	}
	m := len(returns)
	if m < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(m)
	varSq := 0.0
	for _, r := range returns {
		varSq += (r - mean) * (r - mean)
	}
	std := math.Sqrt(varSq / float64(m-1))
	return std * math.Sqrt(252) * 100
}

// maxDrawdown 最大回撤（%，负数）。closes 时间升序，不足 2 个返回 0。
func maxDrawdown(closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}
	peak := closes[0]
	maxDD := 0.0
	for _, c := range closes {
		if c > peak {
			peak = c
		}
		if peak > 0 {
			dd := (c - peak) / peak * 100
			if dd < maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

// changePctN 最近 n 根 K 线的涨跌幅（%），即 (close[-1]-close[-n])/close[-n]×100。
// n 为间隔数（如 60 日趋势传 60，实际数据不足时用全部历史），数据不足 2 个返回 0。
func changePctN(closes []float64, n int) float64 {
	m := len(closes)
	if m < 2 {
		return 0
	}
	base := m - 1 - n
	if base < 0 {
		base = 0
	}
	if closes[base] <= 0 {
		return 0
	}
	return (closes[m-1] - closes[base]) / closes[base] * 100
}

// closesOf 提取 K 线收盘价序列。
func closesOf(kline []KLineBar) []float64 {
	closes := make([]float64, len(kline))
	for i, b := range kline {
		closes[i] = b.Close
	}
	return closes
}

// highsOf / lowsOf 提取最高/最低价序列。
func highsOf(kline []KLineBar) []float64 {
	highs := make([]float64, len(kline))
	for i, b := range kline {
		highs[i] = b.High
	}
	return highs
}

func lowsOf(kline []KLineBar) []float64 {
	lows := make([]float64, len(kline))
	for i, b := range kline {
		lows[i] = b.Low
	}
	return lows
}
