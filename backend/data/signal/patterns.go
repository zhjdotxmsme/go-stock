package signal

// patterns.go K线蜡烛辅助函数（在判定时点 idx 上取值）。
// 阈值约定见各函数注释；形态定义参考经典K线理论。

import "math"

// body 实体（收盘-开盘，带符号）
func (c *evalCtx) body(i int) float64 { return c.close[i] - c.open[i] }

// bodyPct 实体相对前收盘的百分比
func (c *evalCtx) bodyPct(i int) float64 {
	if i <= 0 || c.close[i-1] <= 0 {
		return 0
	}
	return (c.close[i] - c.open[i]) / c.close[i-1] * 100
}

// changePct 涨跌幅（收盘对前收盘，%）
func (c *evalCtx) changePct(i int) float64 {
	if i <= 0 || c.close[i-1] <= 0 {
		return 0
	}
	return (c.close[i]/c.close[i-1] - 1) * 100
}

func (c *evalCtx) bullish(i int) bool { return c.close[i] > c.open[i] }
func (c *evalCtx) bearish(i int) bool { return c.close[i] < c.open[i] }

// candleRange 振幅（高-低）
func (c *evalCtx) candleRange(i int) float64 { return c.high[i] - c.low[i] }

// upperShadow 上影线长度
func (c *evalCtx) upperShadow(i int) float64 {
	return c.high[i] - math.Max(c.open[i], c.close[i])
}

// lowerShadow 下影线长度
func (c *evalCtx) lowerShadow(i int) float64 {
	return math.Min(c.open[i], c.close[i]) - c.low[i]
}

// absBody 实体绝对长度
func (c *evalCtx) absBody(i int) float64 { return math.Abs(c.body(i)) }

// isBigYang 大阳线：收阳且实体涨幅 ≥ bigBodyPct
func (c *evalCtx) isBigYang(i int) bool { return c.bullish(i) && c.bodyPct(i) >= bigBodyPct }

// isBigYin 大阴线：收阴且实体跌幅 ≤ -bigBodyPct
func (c *evalCtx) isBigYin(i int) bool { return c.bearish(i) && c.bodyPct(i) <= -bigBodyPct }

// isStar 星线：实体不足 candleRange 的 starBodyRatio，且振幅不为 0
func (c *evalCtx) isStar(i int) bool {
	r := c.candleRange(i)
	return r > 0 && c.absBody(i) <= r*starBodyRatio
}

// consecutiveUp / consecutiveDown 以 idx 结尾的连续上涨/下跌天数（收盘对前收）
func (c *evalCtx) consecutiveUp(idx int) int {
	n := 0
	for i := idx; i > 0 && c.close[i] > c.close[i-1]; i-- {
		n++
	}
	return n
}

func (c *evalCtx) consecutiveDown(idx int) int {
	n := 0
	for i := idx; i > 0 && c.close[i] < c.close[i-1]; i-- {
		n++
	}
	return n
}

// pricePosition 当前收盘在近 window 日价格区间中的位置（0=最低，1=最高）
func (c *evalCtx) pricePosition(idx, window int) float64 {
	start := idx - window + 1
	if start < 0 {
		start = 0
	}
	hi := maxHigh(c.high, start, idx)
	lo := minLow(c.low, start, idx)
	if hi <= lo {
		return 0.5
	}
	return (c.close[idx] - lo) / (hi - lo)
}
