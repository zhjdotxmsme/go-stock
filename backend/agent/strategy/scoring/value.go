package scoring

// ValueFactor 价值因子：PE(0.35 权重) + PB(0.65 权重) 的百分位排名加权混合。
// 百分位由调用方在候选池内预计算（FactorInput.PEPercentile / PBPercentile，0~1）。
// 估值越低得分越高，故使用 1-百分位。亏损股（PE<=0）PE 分量按最差处理。
type ValueFactor struct {
	PEWeight float64 // PE 权重，默认 0.35
	PBWeight float64 // PB 权重，默认 0.65
}

func NewValueFactor() *ValueFactor {
	return &ValueFactor{PEWeight: 0.35, PBWeight: 0.65}
}

func (f *ValueFactor) Name() string { return "value" }

func (f *ValueFactor) Score(input *FactorInput) FactorResult {
	peScore := 0.0
	if input.PE > 0 {
		peScore = (1 - clamp(input.PEPercentile, 0, 1)) * 100
	}
	pbScore := 0.0
	if input.PB > 0 {
		pbScore = (1 - clamp(input.PBPercentile, 0, 1)) * 100
	}
	total := f.PEWeight + f.PBWeight
	if total <= 0 {
		total = 1
	}
	score := (f.PEWeight*peScore + f.PBWeight*pbScore) / total
	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score),
		Detail: map[string]float64{
			"pe":       input.PE,
			"pb":       input.PB,
			"pe_score": peScore,
			"pb_score": pbScore,
		},
	}
}
