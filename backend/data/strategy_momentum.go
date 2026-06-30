package data

import "math"

// MomentumStrategy scores stocks based on short-term momentum signals.
type MomentumStrategy struct{}

func (s *MomentumStrategy) Name() string        { return "短线动量" }
func (s *MomentumStrategy) Code() string        { return "momentum" }
func (s *MomentumStrategy) Description() string { return "捕捉短期强劲动量信号" }

func (s *MomentumStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 30 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}

	macd := calcMACD(ctx.CloseP, 12, 26, 9)
	macdLine := macd["MACD"]
	signalLine := macd["Signal"]

	// MACD score
	macdScore := 0.0
	if macdLine > 0 && macdLine > signalLine {
		macdScore = 1.0 // golden cross above zero
	} else if macdLine > 0 {
		macdScore = 0.6 // above zero
	} else if macdLine > signalLine {
		macdScore = 0.4 // golden cross below zero
	}

	// OBV trend score — compare recent 5-day vs 20-day average OBV
	obvScore := 0.0
	if n >= 25 {
		calcOBV(ctx.CloseP, ctx.Volume)
		// Approximate trend: compute OBV over last 10 bars vs 20-10 bars
		half := len(ctx.Volume) / 2
		recentOBV := calcOBV(ctx.CloseP[half:], ctx.Volume[half:])
		earlierOBV := calcOBV(ctx.CloseP[:half], ctx.Volume[:half])
		if recentOBV > earlierOBV {
			obvScore = 1.0
		} else if math.Abs(recentOBV-earlierOBV) < 0.01 {
			obvScore = 0.5
		}
	}

	// Volume ratio
	volRatio := 0.0
	if n >= 21 {
		todayVol := ctx.Volume[n-1]
		var sum float64
		for i := n - 21; i < n-1; i++ {
			sum += ctx.Volume[i]
		}
		avgVol := sum / 20
		if avgVol > 0 {
			volRatio = todayVol / avgVol
		}
	}
	volScore := 0.0
	switch {
	case volRatio >= 2.0:
		volScore = 1.0
	case volRatio >= 1.5:
		volScore = 0.7
	case volRatio >= 1.2:
		volScore = 0.4
	case volRatio >= 1.0:
		volScore = 0.2
	}

	// Change percent momentum
	chg := parseFloat64(ctx.KLines[n-1].ChangePercent)
	chgScore := 0.0
	switch {
	case chg >= 3 && chg <= 8:
		chgScore = 1.0
	case chg >= 1 && chg < 3:
		chgScore = 0.6
	case chg >= 0 && chg < 1:
		chgScore = 0.3
	case chg < 0:
		chgScore = 0.0
	}

	total := macdScore*0.30 + obvScore*0.30 + volScore*0.20 + chgScore*0.20
	finalScore := math.Round(total * 100)

	return &StrategyResult{
		Score: finalScore,
		Factors: map[string]float64{
			"macdScore": macdScore,
			"obvScore":  obvScore,
			"volScore":  volScore,
			"chgScore":  chgScore,
		},
		Signal: buildMomentumSignal(macdScore, obvScore, volScore, chg, macdLine),
	}
}

func buildMomentumSignal(macdScore, obvScore, volScore, chg, macdLine float64) string {
	if macdScore >= 0.8 && obvScore >= 0.8 && volScore >= 0.7 {
		return "MACD零上金叉，OBV量能配合，动能强劲"
	}
	if macdScore >= 0.8 {
		return "MACD零轴上方运行"
	}
	if chg >= 3 {
		return "当日涨幅显著"
	}
	return "动量偏弱"
}
