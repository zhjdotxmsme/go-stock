package scoring

// StabilityFactor 稳定性因子（18 参数）：
// 78 分起步逐项扣减：
//   - 年化波动率 > 45%
//   - 最大回撤 < -12%
//   - ATR(14)/价格 > 6%
//   - 低质量日线（近期大阴线 / 长上影）逐根扣分
//
// 数据不足时按中性偏稳处理（返回起步分的一半）。
type StabilityFactor struct {
	BaseScore           float64 // 起步分，默认 78
	VolThreshold        float64 // 波动率阈值（年化 %），默认 45
	VolPenalty          float64 // 波动率超标扣分，默认 15
	DrawdownThreshold   float64 // 最大回撤阈值（%），默认 -12
	DrawdownPenalty     float64 // 回撤超标扣分，默认 15
	ATRThreshold        float64 // ATR/价格阈值（%），默认 6
	ATRPenalty          float64 // ATR 超标扣分，默认 12
	ATRPeriod           int     // ATR 周期，默认 14
	BearBodyThreshold   float64 // 大阴线实体阈值（%），默认 -5
	BearPenalty         float64 // 每根大阴线扣分，默认 4
	UpperShadowThres    float64 // 长上影阈值（%），默认 3
	UpperShadowPenalty  float64 // 每根长上影扣分，默认 3
	LowQualityLookback  int     // 低质量日线回看根数，默认 10
	LowQualityMaxDeduct float64 // 低质量日线扣分上限，默认 20
	MinBars             int     // 最少需要的 K 线根数，默认 20
	InsufficientScore   float64 // 数据不足时的中性分，默认 50
}

func NewStabilityFactor() *StabilityFactor {
	return &StabilityFactor{
		BaseScore:           78,
		VolThreshold:        45,
		VolPenalty:          15,
		DrawdownThreshold:   -12,
		DrawdownPenalty:     15,
		ATRThreshold:        6,
		ATRPenalty:          12,
		ATRPeriod:           14,
		BearBodyThreshold:   -5,
		BearPenalty:         4,
		UpperShadowThres:    3,
		UpperShadowPenalty:  3,
		LowQualityLookback:  10,
		LowQualityMaxDeduct: 20,
		MinBars:             20,
		InsufficientScore:   50,
	}
}

func (f *StabilityFactor) Name() string { return "stability" }

func (f *StabilityFactor) Score(input *FactorInput) FactorResult {
	if len(input.KLine) < f.MinBars {
		return FactorResult{
			Name:   f.Name(),
			Score:  f.InsufficientScore,
			Detail: map[string]float64{"bars": float64(len(input.KLine))},
		}
	}

	closes := closesOf(input.KLine)
	deduct := 0.0

	// 波动率扣减
	vol := volatility(closes)
	volDeduct := 0.0
	if vol > f.VolThreshold {
		volDeduct = f.VolPenalty
	}
	deduct += volDeduct

	// 最大回撤扣减
	mdd := maxDrawdown(closes)
	ddDeduct := 0.0
	if mdd < f.DrawdownThreshold {
		ddDeduct = f.DrawdownPenalty
	}
	deduct += ddDeduct

	// ATR/价格 扣减
	atrVal := atr(highsOf(input.KLine), lowsOf(input.KLine), closes, f.ATRPeriod)
	atrPct := 0.0
	if input.Price > 0 {
		atrPct = atrVal / input.Price * 100
	}
	atrDeduct := 0.0
	if atrPct > f.ATRThreshold {
		atrDeduct = f.ATRPenalty
	}
	deduct += atrDeduct

	// 低质量日线：近期大阴线 / 长上影，逐根扣分并封顶
	lqDeduct := 0.0
	start := len(input.KLine) - f.LowQualityLookback
	if start < 0 {
		start = 0
	}
	for i := start; i < len(input.KLine); i++ {
		bar := input.KLine[i]
		if bar.Open <= 0 {
			continue
		}
		bodyPct := (bar.Close - bar.Open) / bar.Open * 100
		if bodyPct < f.BearBodyThreshold {
			lqDeduct += f.BearPenalty
		}
		if bar.Close > 0 {
			upperShadow := (bar.High - max(bar.Open, bar.Close)) / bar.Close * 100
			if upperShadow > f.UpperShadowThres {
				lqDeduct += f.UpperShadowPenalty
			}
		}
	}
	lqDeduct = clamp(lqDeduct, 0, f.LowQualityMaxDeduct)
	deduct += lqDeduct

	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(f.BaseScore - deduct),
		Detail: map[string]float64{
			"volatility":      vol,
			"max_drawdown":    mdd,
			"atr_pct":         atrPct,
			"vol_deduct":      volDeduct,
			"drawdown_deduct": ddDeduct,
			"atr_deduct":      atrDeduct,
			"lq_deduct":       lqDeduct,
		},
	}
}
