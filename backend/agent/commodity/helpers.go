package commodity

import (
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

func emitToken(cc *CommodityContext, agent string, token string) {
	if cc == nil || cc.StreamCh == nil {
		return
	}
	payload := map[string]string{
		"type":  "agent:token",
		"agent": agent,
		"token": token,
	}
	raw, _ := sonic.Marshal(payload)
	cc.StreamCh <- &schema.Message{Role: schema.Assistant, Content: string(raw)}
}

func emitDebate(cc *CommodityContext, round int, side string, argument string) {
	if cc == nil || cc.StreamCh == nil {
		return
	}
	payload := map[string]interface{}{
		"type":     "agent:debate",
		"round":    round,
		"side":     side,
		"argument": argument,
	}
	raw, _ := sonic.Marshal(payload)
	cc.StreamCh <- &schema.Message{Role: schema.Assistant, Content: string(raw)}
}

func emitPhase(cc *CommodityContext, phase string, status string, label string) {
	if cc == nil || cc.StreamCh == nil {
		return
	}
	payload := map[string]string{
		"type":   "agent:phase",
		"phase":  phase,
		"status": status,
		"label":  label,
	}
	raw, _ := sonic.Marshal(payload)
	cc.StreamCh <- &schema.Message{Role: schema.Assistant, Content: string(raw)}
}

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

func sanitizeJSON(s string) string {
	if len(s) > 5000 {
		return s[:5000] + "...[truncated]"
	}
	return s
}
