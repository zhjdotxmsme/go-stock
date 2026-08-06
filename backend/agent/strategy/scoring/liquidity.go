package scoring

import "math"

// LiquidityFactor 流动性因子：log10(成交额) 的百分位排名。
// 成交额越大流动性越好得分越高。百分位由调用方预计算（FactorInput.AmountPercentile）。
type LiquidityFactor struct{}

func NewLiquidityFactor() *LiquidityFactor { return &LiquidityFactor{} }

func (f *LiquidityFactor) Name() string { return "liquidity" }

func (f *LiquidityFactor) Score(input *FactorInput) FactorResult {
	logAmount := 0.0
	if input.Amount > 0 {
		logAmount = math.Log10(input.Amount)
	}
	score := clamp(input.AmountPercentile, 0, 1) * 100
	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score),
		Detail: map[string]float64{
			"amount":     input.Amount,
			"log_amount": logAmount,
		},
	}
}
