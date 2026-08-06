package scoring

import "math"

// SizeFactor 市值因子：log10(总市值) 的百分位排名。
// 小市值弹性更大得分更高，故使用 1-百分位。百分位由调用方预计算（FactorInput.CapPercentile）。
type SizeFactor struct{}

func NewSizeFactor() *SizeFactor { return &SizeFactor{} }

func (f *SizeFactor) Name() string { return "size" }

func (f *SizeFactor) Score(input *FactorInput) FactorResult {
	logCap := 0.0
	if input.TotalCap > 0 {
		logCap = math.Log10(input.TotalCap)
	}
	score := (1 - clamp(input.CapPercentile, 0, 1)) * 100
	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score),
		Detail: map[string]float64{
			"total_cap": input.TotalCap,
			"log_cap":   logCap,
		},
	}
}
