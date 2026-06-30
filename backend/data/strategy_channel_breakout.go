package data

import "math"

// ChannelBreakoutStrategy scores stocks based on Bollinger Band breakout and volatility.
type ChannelBreakoutStrategy struct{}

func (s *ChannelBreakoutStrategy) Name() string        { return "通道突破" }
func (s *ChannelBreakoutStrategy) Code() string        { return "channel_breakout" }
func (s *ChannelBreakoutStrategy) Description() string { return "基于BOLL通道突破和ATR波动率确认" }

func (s *ChannelBreakoutStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 20 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}

	boll := calcBOLL(ctx.CloseP, 20, 2.0)
	bollMid := boll["Mid"]
	bollUp := boll["Up"]
	bollDown := boll["Down"]

	atr14 := calcATR(ctx.HighP, ctx.LowP, ctx.CloseP, 14)
	price := ctx.CloseP[n-1]

	// 1. BOLL position score (0-1)
	bollScore := 0.0
	switch {
	case price >= bollMid && price <= bollUp:
		bollScore = 1.0 // ideal: between mid and upper band
	case price > bollUp && price <= bollUp*1.03:
		bollScore = 0.8 // slight breakout above upper band
	case price < bollMid && price >= bollMid*0.98:
		bollScore = 0.5 // near mid band from below
	case price < bollDown:
		bollScore = 0.1 // below lower band
	case price > bollUp*1.03:
		bollScore = 0.3 // far above upper band, overextended
	}

	// 2. ATR volatility score
	atrRatio := atr14 / math.Max(bollMid, 0.01)
	atrScore := 0.0
	switch {
	case atrRatio >= 0.03 && atrRatio <= 0.06:
		atrScore = 1.0 // healthy volatility
	case atrRatio >= 0.02 && atrRatio < 0.03:
		atrScore = 0.6
	case atrRatio > 0.06 && atrRatio <= 0.10:
		atrScore = 0.5
	case atrRatio >= 0.015 && atrRatio < 0.02:
		atrScore = 0.3
	}

	// 3. Bandwidth contraction/expansion score
	bollWidth := (bollUp - bollDown) / math.Max(bollMid, 0.01)

	// Compute prior band width for comparison (period-1)
	var prevBollWidth float64
	if n >= 21 {
		prevBoll := calcBOLL(ctx.CloseP[:n-1], 20, 2.0)
		prevBollWidth = (prevBoll["Up"] - prevBoll["Down"]) / math.Max(prevBoll["Mid"], 0.01)
	}
	bwScore := 0.0
	widthChange := bollWidth - prevBollWidth
	switch {
	case prevBollWidth > 0 && widthChange > bollWidth*0.05 && bollScore >= 0.5:
		bwScore = 1.0 // contraction then expansion confirmed
	case widthChange > 0:
		bwScore = 0.6
	case widthChange < 0 && bollWidth < 0.15:
		bwScore = 0.3 // contracting, potential breakout soon
	}

	total := bollScore*0.40 + atrScore*0.20 + bwScore*0.40
	finalScore := math.Round(total * 100)

	return &StrategyResult{
		Score: finalScore,
		Factors: map[string]float64{
			"bollScore":   bollScore,
			"atrScore":    atrScore,
			"bwScore":     bwScore,
			"bollWidth%":  math.Round(bollWidth*1000) / 10,
			"atrRatio%":   math.Round(atrRatio*1000) / 10,
		},
		Signal: buildBreakoutSignal(bollScore, bwScore, price, bollMid, bollUp),
	}
}

func buildBreakoutSignal(bollScore, bwScore, price, bollMid, bollUp float64) string {
	if bollScore >= 0.8 && bwScore >= 0.8 {
		return "BOLL通道扩张突破，确认有效"
	}
	if bollScore >= 0.8 {
		return "价格位于BOLL中上轨之间"
	}
	if bwScore >= 0.6 {
		return "BOLL带宽扩张"
	}
	return "通道内整理"
}
