package strategy

// SMAAt 含 idx 在内的向前 period 根简单均值；不足返回 0
func SMAAt(values []float64, idx, period int) float64 {
	if idx-period+1 < 0 || idx >= len(values) {
		return 0
	}
	sum := 0.0
	for i := idx - period + 1; i <= idx; i++ {
		sum += values[i]
	}
	return sum / float64(period)
}

// LinReg 对 values 做 OLS 线性回归（x=0..n-1），返回斜率和 R²（下限截断 0）
func LinReg(values []float64) (slope, r2 float64) {
	n := float64(len(values))
	if n < 2 {
		return 0, 0
	}
	var sx, sy, sxy, sxx float64
	for i, v := range values {
		x := float64(i)
		sx += x
		sy += v
		sxy += x * v
		sxx += x * x
	}
	denom := n*sxx - sx*sx
	if denom == 0 {
		return 0, 0
	}
	slope = (n*sxy - sx*sy) / denom
	intercept := (sy - slope*sx) / n
	var ssRes, ssTot float64
	mean := sy / n
	for i, v := range values {
		fit := slope*float64(i) + intercept
		ssRes += (v - fit) * (v - fit)
		ssTot += (v - mean) * (v - mean)
	}
	if ssTot == 0 {
		return slope, 0
	}
	r2 = 1 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}
	return slope, r2
}
