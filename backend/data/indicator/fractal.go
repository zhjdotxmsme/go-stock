package indicator

// 本文件移植 frontend/src/components/kline/calc.ts 的分形函数：
//   fractalValues (L975)。
// 逐行忠实移植：相同默认参数、相同算法（含预热期 NaN 约定，对应 JS 的 null）。

// FractalLevels 对应 JS fractalValues（calc.ts L975），默认 period=2。
//
// JS 原函数返回 { fractalHigh, fractalLow } 两条与输入等长的“分形价位序列”：
// 第 i 根构成上分形（摆动高点）时 fractalHigh[i] = highs[i]，否则为 null；
// 下分形对称。因此本函数导出为价位序列（而非 bool 突破序列）：
//
//	upLevels[i]   = 第 i 根为上分形时的高点价，否则 math.NaN()
//	downLevels[i] = 第 i 根为下分形时的低点价，否则 math.NaN()
//
// 判定规则（与 JS 逐字一致，严格不等号）：对 j ∈ [1, period]，当
// high[i] > high[i-j] 且 high[i] > high[i+j] 对所有 j 成立时 i 为上分形；
// low[i] < low[i-j] 且 low[i] < low[i+j] 时为下分形。序列两端各留 period 根
// 观察窗，故前 period 位与后 period 位恒为 NaN。
//
// 与 upBreak/downBreak（突破判断）的关系：本包不额外导出 bool 序列；消费方
// 的向上突破 upBreak[i] 定义为“收盘价上穿最近的上分形价位”，即存在
// j < i 使 upLevels[j] 非 NaN（最近一个这样的价位记 L），且 close[i-1] <= L、
// close[i] > L；向下突破 downBreak 对称（收盘价下穿最近 downLevels）。
// 实际使用时可配合 helpers.go 的 CrossOver/CrossUnder 对“最近分形价位”
// 构造的常量序列判断。
func FractalLevels(high, low []float64, period int) (upLevels, downLevels []float64) {
	n := len(high)
	upLevels = nanSlice(n)
	downLevels = nanSlice(n)
	for i := period; i < n-period; i++ {
		isHigh, isLow := true, true
		for j := 1; j <= period; j++ {
			if high[i] <= high[i-j] || high[i] <= high[i+j] {
				isHigh = false
			}
			if low[i] >= low[i-j] || low[i] >= low[i+j] {
				isLow = false
			}
		}
		if isHigh {
			upLevels[i] = high[i]
		}
		if isLow {
			downLevels[i] = low[i]
		}
	}
	return
}
