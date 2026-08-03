package signal

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"go-stock/backend/logger"
)

// ExtractionTier 提取层级
type ExtractionTier int

const (
	TierJSONParse     ExtractionTier = iota // Tier 1: 直接 JSON 解析
	TierKeywordExtract                       // Tier 2: 关键词提取
	TierPatternMatch                         // Tier 3: 中文价格正则模式
	TierSmartEstimate                        // Tier 4: 智能估算
)

// ExtractionResult 提取结果
type ExtractionResult struct {
	Tier      ExtractionTier // 使用的提取层级
	Score     float64
	Trend     string
	EntryLow  float64
	EntryHigh float64
	ExitLow   float64
	ExitHigh  float64
	RiskLevel string
}

// ReportWriter 用于写入提取结果的接口 (避免循环依赖)
type ReportWriter interface {
	SetScore(float64)
	SetTrend(string)
	SetEntryZone(low, high float64)
	SetExitZone(low, high float64)
	SetRiskLevel(string)
}

// AnalystReportProvider 提供分析师报告信息 (避免循环依赖)
type AnalystReportProvider interface {
	GetReportCount() int
	GetReportRating(index int) string
}

// ExtractStructured 多层降级提取结构化字段
func ExtractStructured(text string, report ReportWriter, provider AnalystReportProvider) ExtractionResult {
	result := ExtractionResult{}

	// Tier 1: 尝试直接 JSON 解析 (从 LLM 响应)
	if tier1Result, ok := tryJSONParse(text); ok {
		applyExtraction(tier1Result, report)
		result = tier1Result
		result.Tier = TierJSONParse
		logger.SugaredLogger.Debugf("signal extraction: used TierJSONParse")
		return result
	}

	// Tier 2: 关键词提取
	if tier2Result, ok := tryKeywordExtract(text); ok {
		applyExtraction(tier2Result, report)
		result = tier2Result
		result.Tier = TierKeywordExtract
		logger.SugaredLogger.Debugf("signal extraction: used TierKeywordExtract")
		return result
	}

	// Tier 3: 中文价格正则模式匹配
	if tier3Result, ok := tryPatternMatch(text); ok {
		applyExtraction(tier3Result, report)
		result = tier3Result
		result.Tier = TierPatternMatch
		logger.SugaredLogger.Debugf("signal extraction: used TierPatternMatch")
		return result
	}

	// Tier 4: 智能估算 (基于分析师评级)
	if tier4Result, ok := trySmartEstimate(provider); ok {
		applyExtraction(tier4Result, report)
		result = tier4Result
		result.Tier = TierSmartEstimate
		logger.SugaredLogger.Debugf("signal extraction: used TierSmartEstimate")
		return result
	}

	return result
}

// tryJSONParse Tier 1: 尝试直接 JSON 解析
func tryJSONParse(text string) (ExtractionResult, bool) {
	// 提取 JSON 块
	jsonStr := extractJSONBlock(text)
	if jsonStr == "" {
		return ExtractionResult{}, false
	}

	var extracted struct {
		Score     float64  `json:"score"`
		Trend     string   `json:"trend"`
		EntryLow  float64  `json:"entryLow"`
		EntryHigh float64  `json:"entryHigh"`
		ExitLow   float64  `json:"exitLow"`
		ExitHigh  float64  `json:"exitHigh"`
		RiskLevel string   `json:"riskLevel"`
		EntryZone *struct { // 兼容旧格式
			Low  float64 `json:"low"`
			High float64 `json:"high"`
		} `json:"entryZone"`
		ExitZone *struct {
			Low  float64 `json:"low"`
			High float64 `json:"high"`
		} `json:"exitZone"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		return ExtractionResult{}, false
	}

	result := ExtractionResult{
		Score:     extracted.Score,
		Trend:     extracted.Trend,
		RiskLevel: extracted.RiskLevel,
	}

	// 支持两种格式
	if extracted.EntryZone != nil {
		result.EntryLow = extracted.EntryZone.Low
		result.EntryHigh = extracted.EntryZone.High
	} else {
		result.EntryLow = extracted.EntryLow
		result.EntryHigh = extracted.EntryHigh
	}

	if extracted.ExitZone != nil {
		result.ExitLow = extracted.ExitZone.Low
		result.ExitHigh = extracted.ExitZone.High
	} else {
		result.ExitLow = extracted.ExitLow
		result.ExitHigh = extracted.ExitHigh
	}

	return result, true
}

// tryKeywordExtract Tier 2: 关键词提取
func tryKeywordExtract(text string) (ExtractionResult, bool) {
	result := ExtractionResult{}
	foundAny := false

	// 提取评分
	if score := extractScore(text); score > 0 {
		result.Score = score
		foundAny = true
	}

	// 提取趋势
	if trend := extractTrend(text); trend != "" {
		result.Trend = trend
		foundAny = true
	}

	// 提取风险等级
	if risk := extractRiskLevel(text); risk != "" {
		result.RiskLevel = risk
		foundAny = true
	}

	// 提取价格区间
	if entryLow, entryHigh, ok := extractPriceZone(text, "买入", "入场", "建仓"); ok {
		result.EntryLow = entryLow
		result.EntryHigh = entryHigh
		foundAny = true
	}
	if exitLow, exitHigh, ok := extractPriceZone(text, "卖出", "止盈", "出场"); ok {
		result.ExitLow = exitLow
		result.ExitHigh = exitHigh
		foundAny = true
	}

	return result, foundAny
}

// applyExtraction 应用提取结果到报告
func applyExtraction(r ExtractionResult, report ReportWriter) {
	if r.Score >= 1 && r.Score <= 10 {
		report.SetScore(r.Score)
	}
	switch r.Trend {
	case "up", "down", "sideways":
		report.SetTrend(r.Trend)
	}
	if r.EntryLow > 0 && r.EntryHigh > 0 {
		report.SetEntryZone(r.EntryLow, r.EntryHigh)
	}
	if r.ExitLow > 0 && r.ExitHigh > 0 {
		report.SetExitZone(r.ExitLow, r.ExitHigh)
	}
	if r.RiskLevel != "" {
		report.SetRiskLevel(r.RiskLevel)
	}
}

// extractJSONBlock 从文本中提取 JSON 块
func extractJSONBlock(text string) string {
	// 处理 ```json ... ``` 格式
	re := regexp.MustCompile("```json\\s*([\\s\\S]*?)\\s*```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		return matches[1]
	}

	// 处理 ``` ... ``` 格式
	re = regexp.MustCompile("```\\s*([\\s\\S]*?)\\s*```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		return matches[1]
	}

	// 尝试找到完整的 JSON 对象
	re = regexp.MustCompile("\\{[\\s\\S]*\\}")
	if matches := re.FindStringSubmatch(text); len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// extractScore 提取评分
func extractScore(text string) float64 {
	patterns := []string{
		`综合评分[：:]\s*(\d+(?:\.\d+)?)`,
		`评分[：:]\s*(\d+(?:\.\d+)?)`,
		`得分[：:]\s*(\d+(?:\.\d+)?)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			if score, err := strconv.ParseFloat(matches[1], 64); err == nil {
				return score
			}
		}
	}
	return 0
}

// extractTrend 提取趋势
func extractTrend(text string) string {
	if strings.Contains(text, "上行") || strings.Contains(text, "上涨") || strings.Contains(text, "看多") {
		return "up"
	}
	if strings.Contains(text, "下行") || strings.Contains(text, "下跌") || strings.Contains(text, "看空") {
		return "down"
	}
	if strings.Contains(text, "震荡") || strings.Contains(text, "盘整") {
		return "sideways"
	}
	return ""
}

// extractRiskLevel 提取风险等级
func extractRiskLevel(text string) string {
	if strings.Contains(text, "高风险") || strings.Contains(text, "风险高") {
		return "high"
	}
	if strings.Contains(text, "中风险") || strings.Contains(text, "中等风险") {
		return "medium"
	}
	if strings.Contains(text, "低风险") || strings.Contains(text, "风险低") {
		return "low"
	}
	return ""
}

// extractPriceZone 提取价格区间
func extractPriceZone(text string, keywords ...string) (low, high float64, ok bool) {
	lowerText := strings.ToLower(text)

	for _, keyword := range keywords {
		// 查找关键词附近的价格
		idx := strings.Index(lowerText, keyword)
		if idx >= 0 {
			// 在关键词前后 100 字符内查找价格
			start := max(0, idx-50)
			end := min(len(lowerText), idx+100)
			context := lowerText[start:end]

			// 匹配价格范围: "100-120元", "100 ~ 120"
			pricePatterns := []string{
				`(\d+(?:\.\d+)?)\s*[-~～至]\s*(\d+(?:\.\d+)?)`,
				`(\d+(?:\.\d+)?)\s*元\s*到\s*(\d+(?:\.\d+)?)`,
			}

			for _, pattern := range pricePatterns {
				re := regexp.MustCompile(pattern)
				if matches := re.FindStringSubmatch(context); len(matches) > 2 {
					low, _ = strconv.ParseFloat(matches[1], 64)
					high, _ = strconv.ParseFloat(matches[2], 64)
					if low > 0 && high > 0 && low < high {
						return low, high, true
					}
				}
			}

			// 匹配单个价格
			singlePattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*元`)
			if matches := singlePattern.FindStringSubmatch(context); len(matches) > 1 {
				price, _ := strconv.ParseFloat(matches[1], 64)
				if price > 0 {
					// 以该价格为中心上下浮动 5%
					return price * 0.95, price * 1.05, true
				}
			}
		}
	}
	return 0, 0, false
}

// max returns the maximum of a and b
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of a and b
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
