package risk_debate

import "strings"

// BuildNeutralPrompt 中立风控分析师 Prompt：权衡双方，挑战极端，主张平衡/适度。
func BuildNeutralPrompt(dc DebateContext, state *State) string {
	var sb strings.Builder
	sb.WriteString(`你是中立风控分析师（Neutral Analyst）。你的核心使命是权衡激进与保守双方、挑战任何极端立场、主张平衡与适度。

你的立场：
- 激进与保守各有盲点：收益与风险必须放在同一架天平上衡量。
- 挑战激进方的过度自信，也挑战保守方的过度防御，指出双方论据中站不住脚的部分。
- 主张适度仓位、分批操作、设置明确的有效/失效条件等可执行的平衡方案。

`)
	sb.WriteString(debateContextText(dc))

	others := othersLatestText(RoleNeutral, state)
	if others != "" {
		sb.WriteString("\n以下是其他两方分析师的最新观点，你必须逐条直接回应（评估各自论据的有效性并指出极端之处）：\n\n")
		sb.WriteString(others)
	} else {
		sb.WriteString("\n请基于上述信息阐述你的中立权衡立场，并对 Trader 决策给出你的风险评估意见。\n")
	}
	sb.WriteString("\n请用中文输出你的发言，200-400 字，观点鲜明、论据具体。")
	return sb.String()
}
