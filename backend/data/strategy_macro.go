package data

import "math"

// MacroEnvironmentStrategy scores based on macro-economic indicators (PMI/CPI/GDP).
// Pre-fetched macro score is passed via StrategyContext.MacroScore.
type MacroEnvironmentStrategy struct{}

func (s *MacroEnvironmentStrategy) Name() string        { return "宏观环境" }
func (s *MacroEnvironmentStrategy) Code() string        { return "macro_environment" }
func (s *MacroEnvironmentStrategy) Description() string { return "基于PMI/CPI/GDP等宏观指标的环境评分策略" }

func (s *MacroEnvironmentStrategy) Score(ctx *StrategyContext) *StrategyResult {
	if ctx.MacroScore <= 0 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "无宏观数据"}
	}

	// MacroScore is 0-1, convert to 0-100
	score := math.Round(ctx.MacroScore * 100)

	var signal string
	switch {
	case ctx.MacroScore >= 0.8:
		signal = "宏观经济环境良好，PMI扩张且通胀温和"
	case ctx.MacroScore >= 0.5:
		signal = "宏观经济环境中性偏暖"
	case ctx.MacroScore >= 0.3:
		signal = "宏观经济环境偏弱"
	default:
		signal = "宏观经济环境承压"
	}

	return &StrategyResult{
		Score: score,
		Factors: map[string]float64{
			"macroScore": ctx.MacroScore,
		},
		Signal: signal,
	}
}
