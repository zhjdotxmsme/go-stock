package data

import "math"

// KDJShortStrategy scores stocks based on KDJ short-term trading signals.
type KDJShortStrategy struct{}

func (s *KDJShortStrategy) Name() string        { return "KDJ短线" }
func (s *KDJShortStrategy) Code() string        { return "kdj_short" }
func (s *KDJShortStrategy) Description() string { return "基于KDJ和W%R的超短线交易信号" }

func (s *KDJShortStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 20 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}

	kdj := calcKDJ(ctx.HighP, ctx.LowP, ctx.CloseP, 9, 3)
	wr10 := calcWR(ctx.HighP, ctx.LowP, ctx.CloseP, 10)
	rsi6 := calcRSI(ctx.CloseP, 6)

	kVal := kdj["K"]
	dVal := kdj["D"]
	jVal := kdj["J"]

	// 1. KDJ Golden / Death cross in various zones
	crossScore := 0.0
	goldenCross := kVal > dVal
	prevKDJ := calcKDJ(ctx.HighP[:n-1], ctx.LowP[:n-1], ctx.CloseP[:n-1], 9, 3)
	prevK := prevKDJ["K"]
	prevD := prevKDJ["D"]

	// Detect golden cross (K just crossed above D)
	justGolden := prevK <= prevD && kVal > dVal
	justDead := prevK >= prevD && kVal < dVal

	switch {
	case kVal < 30 && justGolden:
		crossScore = 1.0 // low-position golden cross
	case kVal < 50 && goldenCross:
		crossScore = 0.7
	case kVal < 20 && kVal > dVal:
		crossScore = 0.8 // oversold area with K>D
	case justDead:
		crossScore = 0.1 // death cross, avoid
	}

	// 2. KDJ value zone score
	zoneScore := 0.0
	switch {
	case kVal >= 20 && kVal <= 50:
		zoneScore = 1.0 // sweet spot
	case kVal < 20:
		zoneScore = 0.7 // oversold, potential reversal
	case kVal > 50 && kVal <= 80:
		zoneScore = 0.6 // rising but caution
	case kVal > 80:
		zoneScore = 0.3 // overbought
	}

	// 3. Williams %R short-term confirmation
	wrScore := 0.0
	switch {
	case wr10 >= -20:
		wrScore = 1.0 // strong momentum (in overbought but continuation)
	case wr10 <= -80:
		wrScore = 0.8 // oversold, potential reversal
	case wr10 >= -50 && wr10 < -20:
		wrScore = 0.6 // neutral bullish
	case wr10 < -50:
		wrScore = 0.3 // neutral bearish
	}

	// 4. RSI(6) momentum for short-term strength
	rsiScore := 0.0
	switch {
	case rsi6 >= 50 && rsi6 <= 70:
		rsiScore = 1.0
	case rsi6 < 50 && rsi6 >= 30:
		rsiScore = 0.5
	case rsi6 > 70:
		rsiScore = 0.4
	case rsi6 < 30:
		rsiScore = 0.6
	}

	// 5. J line divergence
	jScore := 0.0
	if jVal < 0 {
		jScore = 0.2 // extreme overshoot, risk
	} else if jVal > 100 {
		jScore = 0.2 // extreme overbought
	} else if kVal > dVal && jVal > 50 {
		jScore = 1.0 // bullish J alignment
	} else {
		jScore = 0.5
	}

	total := crossScore*0.30 + zoneScore*0.20 + wrScore*0.20 + rsiScore*0.15 + jScore*0.15
	finalScore := math.Round(total * 100)

	return &StrategyResult{
		Score: finalScore,
		Factors: map[string]float64{
			"crossScore": crossScore,
			"zoneScore":  zoneScore,
			"wrScore":    wrScore,
			"rsiScore":   rsiScore,
			"jScore":     jScore,
		},
		Signal: buildKDJSignal(kVal, dVal, goldenCross, justGolden),
	}
}

func buildKDJSignal(k, d float64, goldenCross, justGolden bool) string {
	if k < 30 && justGolden {
		return "KDJ低位金叉，短线买入信号"
	}
	if goldenCross && k < 50 {
		return "KDJ金叉区域"
	}
	if k < 20 {
		return "KDJ超卖区"
	}
	if k > 80 {
		return "KDJ超买区，需谨慎"
	}
	return "KDJ中性"
}
