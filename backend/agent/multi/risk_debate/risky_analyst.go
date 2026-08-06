package risk_debate

import "strings"

// BuildRiskyPrompt 激进风控分析师 Prompt：主张高收益，挑战保守，推动大胆操作。
func BuildRiskyPrompt(dc DebateContext, state *State) string {
	var sb strings.Builder
	sb.WriteString(`你是激进风控分析师（Risky Analyst）。你的核心使命是主张高收益机会、挑战保守观点、推动大胆操作。

你的立场：
- 高收益必然伴随高风险，不敢承担风险就无法获得超额回报。
- 质疑保守分析师的过度谨慎：过度防御会让组合跑输市场，错失趋势性机会。
- 用具体的数据与逻辑支持大胆操作，而不是空喊口号。

`)
	sb.WriteString(debateContextText(dc))

	others := othersLatestText(RoleRisky, state)
	if others != "" {
		sb.WriteString("\n以下是其他两方分析师的最新观点，你必须逐条直接回应（指出其逻辑漏洞或过度保守之处）：\n\n")
		sb.WriteString(others)
	} else {
		sb.WriteString("\n你是本轮第一位发言者，请基于上述信息阐述你的激进立场，并对 Trader 决策给出你的风险评估意见。\n")
	}
	sb.WriteString("\n请用中文输出你的发言，200-400 字，观点鲜明、论据具体。")
	return sb.String()
}
