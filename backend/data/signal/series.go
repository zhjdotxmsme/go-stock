package signal

// series.go 序列级指标计算（MA/EMA/MACD/KDJ）。
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

// emaSeries 指数移动平均序列（首项为首个值）。
func emaSeries(data []float64, period int) []float64 {
	n := len(data)
	out := make([]float64, n)
	if n == 0 || period <= 0 {
		return out
	}
	m := 2.0 / (float64(period) + 1)
	out[0] = data[0]
	for i := 1; i < n; i++ {
		out[i] = out[i-1] + m*(data[i]-out[i-1])
	}
	return out
}

// macdSeries 计算 DIF 与 DEA 序列（标准 12/26/9）。
func macdSeries(close []float64) (dif, dea []float64) {
	emaFast := emaSeries(close, 12)
	emaSlow := emaSeries(close, 26)
	n := len(close)
	dif = make([]float64, n)
	for i := 0; i < n; i++ {
		dif[i] = emaFast[i] - emaSlow[i]
	}
	dea = emaSeries(dif, 9)
	return dif, dea
}

// kdjSeries 计算 K/D 序列（9,3,3；初值 50，SMA 递推）。
func kdjSeries(high, low, close []float64, period int) (k, d []float64) {
	n := len(close)
	k = make([]float64, n)
	d = make([]float64, n)
	prevK, prevD := 50.0, 50.0
	for i := 0; i < n; i++ {
		start := i - period + 1
		if start < 0 {
			start = 0
		}
		hh, ll := -math.MaxFloat64, math.MaxFloat64
		for j := start; j <= i; j++ {
			if high[j] > hh {
				hh = high[j]
			}
			if low[j] < ll {
				ll = low[j]
			}
		}
		var rsv float64
		if hh != ll {
			rsv = (close[i] - ll) / (hh - ll) * 100
		} else {
			rsv = 50
		}
		prevK = 2.0/3.0*prevK + 1.0/3.0*rsv
		prevD = 2.0/3.0*prevD + 1.0/3.0*prevK
		k[i], d[i] = prevK, prevD
	}
	return k, d
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
