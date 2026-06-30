package data

import "math"

// MATrendStrategy scores stocks based on moving average alignment and BIAS.
type MATrendStrategy struct{}

func (s *MATrendStrategy) Name() string        { return "均线趋势" }
func (s *MATrendStrategy) Code() string        { return "ma_trend" }
func (s *MATrendStrategy) Description() string { return "基于均线多头排列的顺势跟踪策略" }

func (s *MATrendStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 20 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	ma5 := calcSMA(ctx.CloseP, 5)
	ma10 := calcSMA(ctx.CloseP, 10)
	ma20 := calcSMA(ctx.CloseP, 20)
	ma60 := calcSMA(ctx.CloseP, 60)
	bias5 := calcBIAS(ctx.CloseP, 5)
	bias10 := calcBIAS(ctx.CloseP, 10)

	// 1. MA alignment score (0-1)
	maScore := 0.0
	if ma5 > ma10 && ma10 > ma20 && ma20 > ma60 {
		maScore = 1.0
	} else if ma5 > ma10 && ma10 > ma20 {
		maScore = 0.8
	} else if ma5 > ma10 {
		maScore = 0.5
	} else if ma5 > ma20 {
		maScore = 0.3
	}

	// 2. MA spread score — wider gap = stronger trend
	spread5_10 := math.Abs(ma5-ma10) / math.Max(ma10, 0.01)
	spread10_20 := math.Abs(ma10-ma20) / math.Max(ma20, 0.01)
	avgSpread := (spread5_10 + spread10_20) / 2
	spreadScore := 0.0
	switch {
	case avgSpread >= 0.03 && avgSpread <= 0.08:
		spreadScore = 1.0
	case avgSpread >= 0.015 && avgSpread < 0.03:
		spreadScore = 0.7
	case avgSpread > 0.08 && avgSpread <= 0.12:
		spreadScore = 0.6
	case avgSpread < 0.015:
		spreadScore = 0.3
	}

	// 3. BIAS score — deviation from MA (healthy 2%-8%)
	biasScore := 0.0
	avgBias := (math.Abs(bias5) + math.Abs(bias10)) / 2
	switch {
	case avgBias >= 2 && avgBias <= 6:
		biasScore = 1.0
	case avgBias >= 1 && avgBias < 2:
		biasScore = 0.7
	case avgBias > 6 && avgBias <= 10:
		biasScore = 0.5
	case avgBias > 10:
		biasScore = 0.2
	default:
		biasScore = 0.1
	}

	// 4. Volume confirmation
	volScore := scoreVolumeFactor(ctx.Volume, ctx.CloseP)

	total := maScore*0.35 + spreadScore*0.25 + biasScore*0.25 + volScore*0.15
	finalScore := math.Round(total * 100)

	return &StrategyResult{
		Score: finalScore,
		Factors: map[string]float64{
			"maScore":      maScore,
			"spreadScore":  spreadScore,
			"biasScore":    biasScore,
			"volScore":     volScore,
		},
		Signal: buildMATrendSignal(maScore, spreadScore, biasScore, ma5, ma10, ma20),
	}
}

func buildMATrendSignal(maScore, spreadScore, biasScore, ma5, ma10, ma20 float64) string {
	if maScore >= 0.8 && spreadScore >= 0.7 && biasScore >= 0.7 {
		return "均线多头排列，价差适中，趋势健康"
	}
	if maScore >= 0.8 {
		return "均线多头排列"
	}
	if maScore >= 0.5 {
		return "MA5>MA10，短多格局"
	}
	return "均线尚未完全多头"
}
