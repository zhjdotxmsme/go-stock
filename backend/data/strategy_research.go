package data

import "math"

// ResearchReportStrategy scores based on recent research report coverage.
// Pre-fetched report count is passed via StrategyContext.ResearchReportCount.
type ResearchReportStrategy struct{}

func (s *ResearchReportStrategy) Name() string        { return "研报热度" }
func (s *ResearchReportStrategy) Code() string        { return "research_report" }
func (s *ResearchReportStrategy) Description() string { return "基于近30天机构研报覆盖数量的热度策略" }

func (s *ResearchReportStrategy) Score(ctx *StrategyContext) *StrategyResult {
	cnt := ctx.ResearchReportCount

	var score float64
	var signal string

	switch {
	case cnt >= 10:
		score = 100
		signal = "近30天研报覆盖≥10篇，机构高度关注"
	case cnt >= 5:
		score = 80
		signal = "近30天研报覆盖≥5篇，机构关注度较高"
	case cnt >= 2:
		score = 60
		signal = "近30天有研报覆盖，机构关注度一般"
	case cnt == 1:
		score = 40
		signal = "近30天仅有1篇研报覆盖"
	default:
		score = 10
		signal = "近30天无研报覆盖"
	}

	return &StrategyResult{
		Score: math.Round(score),
		Factors: map[string]float64{
			"researchReportCount": float64(cnt),
		},
		Signal: signal,
	}
}
