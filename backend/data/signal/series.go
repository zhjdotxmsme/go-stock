package signal

// series.go 序列级指标计算（MA 及区间统计）。
// MACD/KDJ 不在此实现——统一委托 backend/data/indicator 包（engine.go 中调用），
// 与 tool_indicator.go 的单点计算不同，这里输出完整序列，支持在任意时点判定交叉类信号。

import "math"

// smaSeries 简单移动平均序列；前 period-1 位为 0。
func smaSeries(data []float64, period int) []float64 {
	n := len(data)
	out := make([]float64, n)
	if n < period || period <= 0 {
		return out
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += data[i]
		if i >= period {
			sum -= data[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

// avgVol 区间平均成交量 [start, idx]（自动裁剪）。
func avgVol(vol []float64, idx, window int) float64 {
	start := idx - window + 1
	if start < 0 {
		start = 0
	}
	if idx < 0 || idx >= len(vol) || idx < start {
		return 0
	}
	sum := 0.0
	for i := start; i <= idx; i++ {
		sum += vol[i]
	}
	return sum / float64(idx-start+1)
}

// maxClose / minLow / maxHigh 区间极值 [start, end]。
func maxClose(close []float64, start, end int) float64 {
	v := -math.MaxFloat64
	for i := start; i <= end; i++ {
		if close[i] > v {
			v = close[i]
		}
	}
	return v
}

func maxHigh(high []float64, start, end int) float64 {
	v := -math.MaxFloat64
	for i := start; i <= end; i++ {
		if high[i] > v {
			v = high[i]
		}
	}
	return v
}

func minLow(low []float64, start, end int) float64 {
	v := math.MaxFloat64
	for i := start; i <= end; i++ {
		if low[i] < v {
			v = low[i]
		}
	}
	return v
}
