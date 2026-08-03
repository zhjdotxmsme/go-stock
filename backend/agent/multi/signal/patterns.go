package signal

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// PricePattern 价格模式定义
type PricePattern struct {
	Name    string
	Pattern *regexp.Regexp
	Weight  float64
}

var pricePatterns []PricePattern

func init() {
	pricePatterns = []PricePattern{
		{
			Name: "range_dash",
			Pattern: regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[-~～至]\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 1.0,
		},
		{
			Name: "around_price",
			Pattern: regexp.MustCompile(`(?:价格约|约为|大概)\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.9,
		},
		{
			Name: "above_price",
			Pattern: regexp.MustCompile(`(?:高于|超过|突破)\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.8,
		},
		{
			Name: "below_price",
			Pattern: regexp.MustCompile(`(?:低于|跌破|下探)\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.8,
		},
		{
			Name: "support_resistance",
			Pattern: regexp.MustCompile(`(?:支撑|压力|阻力)\s*(?:位|价格)?\s*(?:约)?\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.85,
		},
		{
			Name: "target_price",
			Pattern: regexp.MustCompile(`(?:目标价|目标价格|预期价)\s*(?:约)?\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.95,
		},
		{
			Name: "buy_zone",
			Pattern: regexp.MustCompile(`(?:买入|建仓|入场)\s*(?:区间|区域)?\s*(?:在)?\s*(\d+(?:\.\d+)?)\s*[到-]\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 1.0,
		},
		{
			Name: "sell_zone",
			Pattern: regexp.MustCompile(`(?:卖出|止盈|出场)\s*(?:区间|区域)?\s*(?:在)?\s*(\d+(?:\.\d+)?)\s*[到-]\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 1.0,
		},
		{
			Name: "stop_loss",
			Pattern: regexp.MustCompile(`(?:止损|风控)\s*(?:位|价)?\s*(?:在)?\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.9,
		},
		{
			Name: "take_profit",
			Pattern: regexp.MustCompile(`(?:止盈|目标)\s*(?:位|价)?\s*(?:在)?\s*(\d+(?:\.\d+)?)\s*元`),
			Weight: 0.9,
		},
		{
			Name: "chinese_num_price",
			Pattern: regexp.MustCompile(`(?:价格为|约)\s*([一二三四五六七八九十百千万\d]+)\s*元`),
			Weight: 0.7,
		},
		{
			Name: "percent_change",
			Pattern: regexp.MustCompile(`(?:上涨|下跌|涨幅|跌幅)\s*(\d+(?:\.\d+)?)\s*%`),
			Weight: 0.6,
		},
	}
}

// tryPatternMatch Tier 3: 中文价格正则模式匹配
func tryPatternMatch(text string) (ExtractionResult, bool) {
	result := ExtractionResult{}
	foundAny := false

	lowerText := strings.ToLower(text)

	// 提取买入/入场区间
	if entryLow, entryHigh, ok := extractPriceByPatterns(lowerText, []string{"买入", "建仓", "入场", "支撑", "低吸"}); ok {
		result.EntryLow = entryLow
		result.EntryHigh = entryHigh
		foundAny = true
	}

	// 提取卖出/出场区间
	if exitLow, exitHigh, ok := extractPriceByPatterns(lowerText, []string{"卖出", "止盈", "出场", "压力", "阻力", "高抛"}); ok {
		result.ExitLow = exitLow
		result.ExitHigh = exitHigh
		foundAny = true
	}

	// 提取趋势信号
	if trend := extractTrendByPatterns(lowerText); trend != "" {
		result.Trend = trend
		foundAny = true
	}

	// 提取风险等级
	if risk := extractRiskByPatterns(lowerText); risk != "" {
		result.RiskLevel = risk
		foundAny = true
	}

	// 估算评分 (基于牛熊分析师数量和关键词)
	if score := estimateScoreByPatterns(lowerText); score > 0 {
		result.Score = score
		foundAny = true
	}

	return result, foundAny
}

// extractPriceByPatterns 使用模式提取价格区间
func extractPriceByPatterns(text string, keywords []string) (low, high float64, ok bool) {
	var prices []float64

	for _, keyword := range keywords {
		idx := strings.Index(text, keyword)
		if idx >= 0 {
			start := max(0, idx-100)
			end := min(len(text), idx+150)
			context := text[start:end]

			for _, pattern := range pricePatterns {
				matches := pattern.Pattern.FindStringSubmatch(context)
				if len(matches) > 1 {
					if len(matches) > 2 {
						// 有两个价格捕获组
						price1, _ := strconv.ParseFloat(matches[1], 64)
						price2, _ := strconv.ParseFloat(matches[2], 64)
						if price1 > 0 && price2 > 0 {
							low = math.Min(price1, price2)
							high = math.Max(price1, price2)
							return low, high, true
						}
					} else {
						// 单个价格，加入候选
						price, _ := strconv.ParseFloat(matches[1], 64)
						if price > 0 {
							prices = append(prices, price)
						}
					}
				}
			}
		}
	}

	// 如果找到多个价格，取它们的区间
	if len(prices) >= 2 {
		// 找出最小和最大作为区间
		minPrice := prices[0]
		maxPrice := prices[0]
		for _, p := range prices[1:] {
			if p < minPrice {
				minPrice = p
			}
			if p > maxPrice {
				maxPrice = p
			}
		}
		// 确保不是相同价格
		if maxPrice > minPrice * 1.01 {
			return minPrice, maxPrice, true
		}
	}

	// 只有单个价格，浮动 5%
	if len(prices) == 1 && prices[0] > 0 {
		return prices[0] * 0.95, prices[0] * 1.05, true
	}

	return 0, 0, false
}

// extractTrendByPatterns 模式提取趋势
func extractTrendByPatterns(text string) string {
	bullishPatterns := []string{"上涨", "看涨", "看多", "买入", "做多", "突破", "上行", "反弹", "拉升", "走强"}
	bearishPatterns := []string{"下跌", "看跌", "看空", "卖出", "做空", "跌破", "下行", "回调", "走弱", "承压"}

	bullishCount := 0
	bearishCount := 0

	for _, p := range bullishPatterns {
		bullishCount += strings.Count(text, p)
	}
	for _, p := range bearishPatterns {
		bearishCount += strings.Count(text, p)
	}

	if bullishCount > bearishCount+2 {
		return "up"
	}
	if bearishCount > bullishCount+2 {
		return "down"
	}
	if bullishCount > 0 || bearishCount > 0 {
		return "sideways"
	}
	return ""
}

// extractRiskByPatterns 模式提取风险等级
func extractRiskByPatterns(text string) string {
	highRiskPatterns := []string{"高风险", "风险较高", "波动大", "不确定性", "注意风险", "谨慎", "风险较大"}
	mediumRiskPatterns := []string{"中等风险", "有一定风险", "存在波动"}
	lowRiskPatterns := []string{"低风险", "风险较低", "相对稳健", "安全边际"}

	highCount := 0
	mediumCount := 0
	lowCount := 0

	for _, p := range highRiskPatterns {
		highCount += strings.Count(text, p)
	}
	for _, p := range mediumRiskPatterns {
		mediumCount += strings.Count(text, p)
	}
	for _, p := range lowRiskPatterns {
		lowCount += strings.Count(text, p)
	}

	if highCount > mediumCount && highCount > lowCount {
		return "high"
	}
	if mediumCount > highCount && mediumCount > lowCount {
		return "medium"
	}
	if lowCount > highCount && lowCount > mediumCount {
		return "low"
	}
	return ""
}

// estimateScoreByPatterns 模式估算评分
func estimateScoreByPatterns(text string) float64 {
	// 基于积极和消极关键词数量
	positiveWords := []string{"强烈推荐", "推荐", "看好", "买入", "增持", "乐观", "积极", "超预期", "优秀", "良好"}
	negativeWords := []string{"卖出", "减持", "悲观", "风险", "谨慎", "回避", "担忧", "不及预期", "较差", "下滑"}

	positiveCount := 0
	negativeCount := 0

	for _, w := range positiveWords {
		positiveCount += strings.Count(text, w)
	}
	for _, w := range negativeWords {
		negativeCount += strings.Count(text, w)
	}

	// 归一化到 1-10 分
	score := 5.5 + float64(positiveCount - negativeCount) * 0.5
	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}
	return score
}

// trySmartEstimate Tier 4: 智能估算 (基于分析师评级)
func trySmartEstimate(provider AnalystReportProvider) (ExtractionResult, bool) {
	if provider == nil || provider.GetReportCount() == 0 {
		return ExtractionResult{}, false
	}

	result := ExtractionResult{}

	// 统计牛熊分析师数量
	bullish := 0
	bearish := 0
	neutral := 0
	count := provider.GetReportCount()

	for i := 0; i < count; i++ {
		rating := provider.GetReportRating(i)
		switch rating {
		case "strong_buy", "bullish":
			bullish++
		case "strong_sell", "bearish":
			bearish++
		default:
			neutral++
		}
	}

	total := bullish + bearish + neutral
	if total == 0 {
		return ExtractionResult{}, false
	}

	// 计算综合评分
	score := 5.0
	score += float64(bullish) * 1.0
	score -= float64(bearish) * 1.0
	score += float64(neutral) * 0.3

	if score > 10 {
		score = 10
	}
	if score < 1 {
		score = 1
	}
	result.Score = score

	// 确定趋势
	if float64(bullish)/float64(total) > 0.6 {
		result.Trend = "up"
	} else if float64(bearish)/float64(total) > 0.6 {
		result.Trend = "down"
	} else {
		result.Trend = "sideways"
	}

	// 估算风险等级
	if bearish > bullish {
		result.RiskLevel = "high"
	} else if neutral > bullish+bearish {
		result.RiskLevel = "medium"
	} else {
		result.RiskLevel = "low"
	}

	return result, true
}
