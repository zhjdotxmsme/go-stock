package multi

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

// emitToken sends a streaming token event to the frontend via AgentContext's StreamCh.
func emitToken(ac *AgentContext, agent string, token string) {
	if ac == nil || ac.StreamCh == nil {
		return
	}
	payload := map[string]string{
		"type":  "agent:token",
		"agent": agent,
		"token": token,
	}
	raw, _ := sonic.Marshal(payload)
	ac.StreamCh <- &schema.Message{Role: schema.Assistant, Content: string(raw)}
}

// emitDebate sends a debate-round event to the frontend.
func emitDebate(ac *AgentContext, round int, side string, argument string) {
	if ac == nil || ac.StreamCh == nil {
		return
	}
	payload := map[string]interface{}{
		"type":     "agent:debate",
		"round":    round,
		"side":     side,
		"argument": argument,
	}
	raw, _ := sonic.Marshal(payload)
	ac.StreamCh <- &schema.Message{Role: schema.Assistant, Content: string(raw)}
}

// sanitizeJSON ensures the argument string is valid for JSON embedding by truncating if needed.
func sanitizeJSON(s string) string {
	if len(s) > 5000 {
		return s[:5000] + "...[truncated]"
	}
	return s
}

// truncateSummary returns the first n characters of content as a summary.
func truncateSummary(content string, n int) string {
	if content == "" {
		return "暂无分析结果"
	}
	runes := []rune(content)
	if len(runes) <= n {
		return content
	}
	return string(runes[:n]) + "..."
}

// extractRating attempts to find a bullish/bearish/neutral rating in the LLM response.
func extractRating(content string) string {
	if content == "" {
		return "neutral"
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "强烈看多") || strings.Contains(lower, "strong_buy") || strings.Contains(lower, "强烈推荐") {
		return "strong_buy"
	}
	if strings.Contains(lower, "看多") || strings.Contains(lower, "bullish") || strings.Contains(lower, "推荐买入") {
		return "bullish"
	}
	if strings.Contains(lower, "强烈看空") || strings.Contains(lower, "strong_sell") {
		return "strong_sell"
	}
	if strings.Contains(lower, "看空") || strings.Contains(lower, "bearish") || strings.Contains(lower, "建议卖出") {
		return "bearish"
	}
	return "neutral"
}
