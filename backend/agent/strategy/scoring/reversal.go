package scoring

import "math"

// ReversalFactor 反转因子（7 参数）：
//   - 理想当日跌幅 -3%，偏离越远扣分越多（对称扣减）
//   - 崩盘（当日跌幅 <-8%）追加惩罚
//   - RSI 超卖（<30）+10，超买（>70）-14
type ReversalFactor struct {
	IdealDrop        float64 // 理想跌幅 %，默认 -3
	DeviationSlope   float64 // 偏离扣分斜率，默认 8
	CrashThreshold   float64 // 崩盘阈值（当日跌幅 %），默认 -8
	CrashPenalty     float64 // 崩盘追加惩罚分，默认 20
	RSIOversold      float64 // RSI 超卖阈值，默认 30
	RSIOverbought    float64 // RSI 超买阈值，默认 70
	RSIOversoldBonus float64 // 超卖加分，默认 +10
	RSIOverboughtCut float64 // 超买减分，默认 -14
	RSIPeriod        int     // RSI 周期，默认 14
}

func NewReversalFactor() *ReversalFactor {
	return &ReversalFactor{
		IdealDrop:        -3,
		DeviationSlope:   8,
		CrashThreshold:   -8,
		CrashPenalty:     20,
		RSIOversold:      30,
		RSIOverbought:    70,
		RSIOversoldBonus: 10,
		RSIOverboughtCut: -14,
		RSIPeriod:        14,
	}
}

func (f *ReversalFactor) Name() string { return "reversal" }

func (f *ReversalFactor) Score(input *FactorInput) FactorResult {
	// 与理想跌幅的偏离度：越接近 -3% 得分越高
	deviation := math.Abs(input.ChangePercent - f.IdealDrop)
	score := 100 - deviation*f.DeviationSlope

	// 崩盘追加惩罚
	crashPenalty := 0.0
	if input.ChangePercent < f.CrashThreshold {
		crashPenalty = f.CrashPenalty
	}
	score -= crashPenalty

	// RSI 超卖加分 / 超买减分（RSIOverboughtCut 以正数存储扣分幅度）
	rsiVal := rsi(closesOf(input.KLine), f.RSIPeriod)
	rsiAdj := 0.0
	switch {
	case rsiVal < f.RSIOversold:
		rsiAdj = f.RSIOversoldBonus
	case rsiVal > f.RSIOverbought:
		rsiAdj = -math.Abs(f.RSIOverboughtCut)
	}
	score += rsiAdj

	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score),
		Detail: map[string]float64{
			"deviation":     deviation,
			"crash_penalty": crashPenalty,
			"rsi":           rsiVal,
			"rsi_adj":       rsiAdj,
		},
	}
}
