package data

import "math"

// IndustryStrengthStrategy scores based on the stock's industry money-flow ranking.
// Pre-fetched industry rank data is passed via StrategyContext.IndustryRankScore.
type IndustryStrengthStrategy struct{}

func (s *IndustryStrengthStrategy) Name() string        { return "行业强度" }
func (s *IndustryStrengthStrategy) Code() string        { return "industry_strength" }
func (s *IndustryStrengthStrategy) Description() string { return "基于所属行业资金排名的行业强度策略" }

func (s *IndustryStrengthStrategy) Score(ctx *StrategyContext) *StrategyResult {
	// If no industry rank data available, skip
	if ctx.IndustryRankScore <= 0 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "无行业排名数据"}
	}

	// IndustryRankScore is already 0-1 from pre-fetch mapping
	// Convert to 0-100 scale for strategy competition
	// Weight: industry score contributes up to 15% of the final strategy score
	baseScore := ctx.IndustryRankScore * 100

	// Bonus for stocks in top industries
	var bonus float64
	if ctx.IndustryRankScore >= 0.9 {
		bonus = 10 // top 10% industry → +10 bonus
	} else if ctx.IndustryRankScore >= 0.7 {
		bonus = 5
	}

	finalScore := math.Min(baseScore+bonus, 100)
	finalScore = math.Round(finalScore)

	var signal string
	switch {
	case ctx.IndustryRankScore >= 0.9:
		signal = "所属行业资金排名前10%，行业强势"
	case ctx.IndustryRankScore >= 0.7:
		signal = "所属行业资金排名前30%，行业偏强"
	case ctx.IndustryRankScore >= 0.5:
		signal = "所属行业资金排名中游"
	default:
		signal = "所属行业资金排名偏弱"
	}

	return &StrategyResult{
		Score: finalScore,
		Factors: map[string]float64{
			"industryRankScore": ctx.IndustryRankScore,
		},
		Signal: signal,
	}
}
