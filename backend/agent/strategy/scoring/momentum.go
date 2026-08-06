package scoring

// MomentumFactor 动量因子（12 参数）：
//   - 日内动量：60 分起步 + 当日涨跌幅 × 斜率
//   - 60 日趋势：55 分起步 + 60 日涨跌幅 × 斜率
//   - MACD 信号：金叉/零轴上加分，死叉/零轴下减分
//   - 惩罚：追涨（当日涨幅 >5%）、破位（60 日跌幅 <-20%）、过热（60 日涨幅 >45%）
//
// 综合分 = 日内×日内权重 + 趋势×趋势权重 + MACD×MACD权重 - 各项惩罚。
type MomentumFactor struct {
	IntradayBase   float64 // 日内起步分，默认 60
	IntradaySlope  float64 // 日内涨跌幅斜率，默认 4
	TrendBase      float64 // 趋势起步分，默认 55
	TrendSlope     float64 // 60 日涨跌幅斜率，默认 0.8
	TrendDays      int     // 趋势窗口天数，默认 60
	MACDBonus      float64 // MACD 多头信号加分，默认 70（信号分基准）
	MACDPenalty    float64 // MACD 空头信号得分，默认 35
	IntradayWeight float64 // 日内权重，默认 0.4
	TrendWeight    float64 // 趋势权重，默认 0.4
	MACDWeight     float64 // MACD 权重，默认 0.2
	ChaseThreshold float64 // 追涨阈值（当日涨幅 %），默认 5
	ChasePenalty   float64 // 追涨惩罚分，默认 15
	BreakThreshold float64 // 破位阈值（60 日跌幅 %），默认 -20
	BreakPenalty   float64 // 破位惩罚分，默认 25
	HeatThreshold  float64 // 过热阈值（60 日涨幅 %），默认 45
	HeatPenalty    float64 // 过热惩罚分，默认 20
}

func NewMomentumFactor() *MomentumFactor {
	return &MomentumFactor{
		IntradayBase:   60,
		IntradaySlope:  4,
		TrendBase:      55,
		TrendSlope:     0.8,
		TrendDays:      60,
		MACDBonus:      70,
		MACDPenalty:    35,
		IntradayWeight: 0.4,
		TrendWeight:    0.4,
		MACDWeight:     0.2,
		ChaseThreshold: 5,
		ChasePenalty:   15,
		BreakThreshold: -20,
		BreakPenalty:   25,
		HeatThreshold:  45,
		HeatPenalty:    20,
	}
}

func (f *MomentumFactor) Name() string { return "momentum" }

func (f *MomentumFactor) Score(input *FactorInput) FactorResult {
	// 日内动量：60 起步 + 涨跌幅×斜率
	intraday := clamp100(f.IntradayBase + input.ChangePercent*f.IntradaySlope)

	closes := closesOf(input.KLine)
	trendPct := changePctN(closes, f.TrendDays)

	// 60 日趋势：55 起步 + 60 日涨跌幅×斜率
	trend := clamp100(f.TrendBase + trendPct*f.TrendSlope)

	// MACD 信号分：DIF>0 且柱状图>0 为多头，DIF<0 且柱状图<0 为空头，其余中性
	macdScore := 50.0
	if len(closes) >= 26 {
		dif, _, hist := macd(closes, 12, 26, 9)
		switch {
		case dif > 0 && hist > 0:
			macdScore = f.MACDBonus
		case dif < 0 && hist < 0:
			macdScore = f.MACDPenalty
		case hist > 0:
			macdScore = 60
		case hist < 0:
			macdScore = 40
		}
	}

	totalWeight := f.IntradayWeight + f.TrendWeight + f.MACDWeight
	if totalWeight <= 0 {
		totalWeight = 1
	}
	score := (intraday*f.IntradayWeight + trend*f.TrendWeight + macdScore*f.MACDWeight) / totalWeight

	// 惩罚项
	penalty := 0.0
	if input.ChangePercent > f.ChaseThreshold {
		penalty += f.ChasePenalty // 追涨
	}
	if trendPct < f.BreakThreshold {
		penalty += f.BreakPenalty // 破位
	}
	if trendPct > f.HeatThreshold {
		penalty += f.HeatPenalty // 过热
	}

	return FactorResult{
		Name:  f.Name(),
		Score: clamp100(score - penalty),
		Detail: map[string]float64{
			"intraday":   intraday,
			"trend_pct":  trendPct,
			"trend":      trend,
			"macd_score": macdScore,
			"penalty":    penalty,
		},
	}
}
