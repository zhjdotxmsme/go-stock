package multi

import "strings"

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
// Uses simple keyword matching — the researcher/synthesis nodes will refine this.
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
