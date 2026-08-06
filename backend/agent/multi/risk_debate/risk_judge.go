package risk_debate

import (
	"regexp"
	"strings"
)

// BuildJudgePrompt 风控裁判 Prompt：听取三方辩论后裁决 BUY/SELL/HOLD，可否决 Trader 决策。
func BuildJudgePrompt(dc DebateContext, state *State) string {
	var sb strings.Builder
	sb.WriteString(`你是风控裁判（Risk Judge）。激进、保守、中立三位风控分析师已就 Trader 的决策完成辩论，由你做出最终裁决。

你的职责：
- 批判性评估三方论据的有效性，而不是简单调和。
- 你可以否决 Trader 的决策：当风险论据充分时，不要犹豫推翻它。
- 只有在有具体论据强烈支持时才选择持有（HOLD），而不是在所有方面都似乎有效时作为后备选择。
- 裁决必须基于辩论中出现的具体论据，禁止凭空引入辩论之外的信息。

`)
	sb.WriteString(debateContextText(dc))
	sb.WriteString("\n三方风控辩论记录：\n\n")
	sb.WriteString(state.HistoryText())
	sb.WriteString(`
请用中文输出你的裁决理由（200-400 字），然后在最后一行单独输出最终裁决，格式严格为：

最终裁决: BUY
或
最终裁决: SELL
或
最终裁决: HOLD`)
	return sb.String()
}

// decisionLineRe 匹配 "最终裁决: BUY" 等显式裁决行。
var decisionLineRe = regexp.MustCompile(`(?:最终裁决|裁决|decision)\s*[:：]?\s*(BUY|SELL|HOLD|买入|卖出|持有|观望)`)

// decisionWordRe 匹配独立的 BUY/SELL/HOLD 英文单词。
var decisionWordRe = regexp.MustCompile(`\b(BUY|SELL|HOLD)\b`)

// ParseDecision 从裁判输出文本提取 BUY/SELL/HOLD。
// 解析顺序：显式"最终裁决"行 → 最后一个独立英文关键词 → 中文关键词（买入/卖出/持有）。
// 全部失败返回 (HOLD, false)，调用方按 parse_failed 处理。
func ParseDecision(text string) (string, bool) {
	// 1. 显式裁决行（取最后一次出现）
	matches := decisionLineRe.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		if d, ok := NormalizeDecision(matches[len(matches)-1][1]); ok {
			return d, true
		}
	}
	// 2. 独立英文关键词（取最后一次出现，避免被理由正文里的早期提及误导）
	matches = decisionWordRe.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return strings.ToUpper(matches[len(matches)-1][1]), true
	}
	// 3. 中文关键词（取最后一次出现）
	if d, ok := NormalizeDecision(text); ok {
		return d, true
	}
	return DecisionHold, false
}

// NormalizeDecision 将任意文本/词汇归一化为 BUY/SELL/HOLD。
// 参照 D6 disagreement.NormalizeSignal 的思路，按最后一次出现的中文关键词判定。
func NormalizeDecision(s string) (string, bool) {
	trimmed := strings.ToUpper(strings.TrimSpace(s))
	switch trimmed {
	case "BUY", "STRONG_BUY":
		return DecisionBuy, true
	case "SELL", "REDUCE":
		return DecisionSell, true
	case "HOLD", "WATCH":
		return DecisionHold, true
	}
	// 文本中最后一次出现的中文方向词
	lastIdx, decision := -1, ""
	for word, d := range map[string]string{
		"买入": DecisionBuy, "看多": DecisionBuy,
		"卖出": DecisionSell, "减仓": DecisionSell, "看空": DecisionSell,
		"持有": DecisionHold, "观望": DecisionHold,
	} {
		if idx := strings.LastIndex(s, word); idx > lastIdx {
			lastIdx, decision = idx, d
		}
	}
	if lastIdx >= 0 {
		return decision, true
	}
	return "", false
}
