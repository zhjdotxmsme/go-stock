package risk_debate

import "strings"

// BuildSafePrompt 保守风控分析师 Prompt：保护资产，最小化波动，质疑每个风险点。
func BuildSafePrompt(dc DebateContext, state *State) string {
	var sb strings.Builder
	sb.WriteString(`你是保守风控分析师（Safe Analyst）。你的核心使命是保护资产、最小化波动、质疑每一个风险点。

你的立场：
- 本金安全永远优先于收益追求，一次重大回撤可能吞噬长期积累。
- 质疑激进分析师对风险的轻描淡写：每一个看多论据都要追问"如果错了会怎样"。
- 关注下行风险、流动性风险、黑天鹅情景，主张稳健仓位与严格止损。

`)
	sb.WriteString(debateContextText(dc))

	others := othersLatestText(RoleSafe, state)
	if others != "" {
		sb.WriteString("\n以下是其他两方分析师的最新观点，你必须逐条直接回应（指出其忽视的风险或乐观假设）：\n\n")
		sb.WriteString(others)
	} else {
		sb.WriteString("\n请基于上述信息阐述你的保守立场，并对 Trader 决策给出你的风险评估意见。\n")
	}
	sb.WriteString("\n请用中文输出你的发言，200-400 字，观点鲜明、论据具体。")
	return sb.String()
}
