package scoring

import "math"

// ActivityFactor 活跃度因子（8 参数）：
// 量比（理想值 2.0）与换手率（理想值 4.0）均使用"理想值距离"公式：
//
//	子分 = max(0, 100 - |实际值 - 理想值| × 斜率)
//
// 综合分 = 量比子分 × 量比权重 + 换手子分 × 换手权重。
type ActivityFactor struct {
	IdealVolumeRatio  float64 // 理想量比，默认 2.0
	VolumeRatioSlope  float64 // 量比偏离斜率，默认 25（偏离 4 个单位归零）
	IdealTurnover     float64 // 理想换手率 %，默认 4.0
	TurnoverSlope     float64 // 换手率偏离斜率，默认 12.5（偏离 8 个百分点归零）
	VolumeRatioWeight float64 // 量比权重，默认 0.5
	TurnoverWeight    float64 // 换手率权重，默认 0.5
	BaseScore         float64 // 理想值得分，默认 100
	MinScore          float64 // 子分下限，默认 0
}

func NewActivityFactor() *ActivityFactor {
	return &ActivityFactor{
		IdealVolumeRatio:  2.0,
		VolumeRatioSlope:  25,
		IdealTurnover:     4.0,
		TurnoverSlope:     12.5,
		VolumeRatioWeight: 0.5,
		TurnoverWeight:    0.5,
		BaseScore:         100,
		MinScore:          0,
	}
}

func (f *ActivityFactor) Name() string { return "activity" }

// idealDistance "理想值距离"公式：偏离理想值越远得分越低。
func (f *ActivityFactor) idealDistance(value, ideal, slope float64) float64 {
	return math.Max(f.MinScore, f.BaseScore-math.Abs(value-ideal)*slope)
}

func (f *ActivityFactor) Score(input *FactorInput) FactorResult {
	vrScore := f.idealDistance(input.VolumeRatio, f.IdealVolumeRatio, f.VolumeRatioSlope)
	toScore := f.idealDistance(input.TurnoverRate, f.IdealTurnover, f.TurnoverSlope)

	totalWeight := f.VolumeRatioWeight + f.TurnoverWeight
	if totalWeight <= 0 {
		totalWeight = 1
	}
	score := (vrScore*f.VolumeRatioWeight + toScore*f.TurnoverWeight) / totalWeight

	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score),
		Detail: map[string]float64{
			"volume_ratio":   input.VolumeRatio,
			"turnover_rate":  input.TurnoverRate,
			"vr_score":       vrScore,
			"turnover_score": toScore,
		},
	}
}
