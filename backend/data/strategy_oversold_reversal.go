package data

import "math"

// OversoldReversalStrategy scores stocks nearing oversold conditions for reversal.
type OversoldReversalStrategy struct{}

func (s *OversoldReversalStrategy) Name() string        { return "超买超卖逆转" }
func (s *OversoldReversalStrategy) Code() string        { return "oversold_reversal" }
func (s *OversoldReversalStrategy) Description() string { return "识别超卖区域的反转信号" }

func (s *OversoldReversalStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 20 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}

	rsi14 := calcRSI(ctx.CloseP, 14)
	wr14 := calcWR(ctx.HighP, ctx.LowP, ctx.CloseP, 14)
	cci14 := calcCCI(ctx.HighP, ctx.LowP, ctx.CloseP, 14)
	kdj := calcKDJ(ctx.HighP, ctx.LowP, ctx.CloseP, 9, 3)

	// RSI oversold
	rsiScore := 0.0
	switch {
	case rsi14 <= 20:
		rsiScore = 1.0
	case rsi14 <= 30:
		rsiScore = 0.8
	case rsi14 <= 35:
		rsiScore = 0.5
	case rsi14 <= 40:
		rsiScore = 0.3
	}

	// Williams %R oversold
	wrScore := 0.0
	switch {
	case wr14 <= -90:
		wrScore = 1.0
	case wr14 <= -80:
		wrScore = 0.8
	case wr14 <= -70:
		wrScore = 0.5
	case wr14 <= -60:
		wrScore = 0.3
	}

	// CCI oversold
	cciScore := 0.0
	switch {
	case cci14 <= -150:
		cciScore = 1.0
	case cci14 <= -100:
		cciScore = 0.8
	case cci14 <= -70:
		cciScore = 0.5
	case cci14 <= -50:
		cciScore = 0.3
	}

	// KDJ K golden cross at low position
	kVal := kdj["K"]
	dVal := kdj["D"]
	kdjScore := 0.0
	if kVal < 20 && kVal > dVal {
		kdjScore = 1.0 // oversold golden cross
	} else if kVal < 30 && kVal > dVal {
		kdjScore = 0.7
	} else if kVal < 20 {
		kdjScore = 0.4 // oversold but no cross yet
	}

	total := rsiScore*0.25 + wrScore*0.25 + cciScore*0.25 + kdjScore*0.25
	finalScore := math.Round(total * 100)

	return &StrategyResult{
		Score: finalScore,
		Factors: map[string]float64{
			"rsiScore": rsiScore,
			"wrScore":  wrScore,
			"cciScore": cciScore,
			"kdjScore": kdjScore,
			"rsi":      math.Round(rsi14*100) / 100,
			"wr":       wr14,
			"cci":      cci14,
			"kdjK":     kVal,
		},
		Signal: buildOversoldSignal(rsiScore, wrScore, cciScore, kdjScore, rsi14, wr14, cci14, kVal),
	}
}

func buildOversoldSignal(rsiScore, wrScore, cciScore, kdjScore, rsi, wr, cci, k float64) string {
	signalCount := 0
	if rsiScore >= 0.8 {
		signalCount++
	}
	if wrScore >= 0.8 {
		signalCount++
	}
	if cciScore >= 0.8 {
		signalCount++
	}
	if kdjScore >= 0.7 {
		signalCount++
	}
	if signalCount >= 3 {
		return "多指标超卖共振，反转概率较高"
	}
	if signalCount >= 2 {
		return "RSI/WR/CCI等多个指标进入超卖区"
	}
	if rsiScore >= 0.8 {
		return "RSI进入超卖区"
	}
	return "部分指标偏弱"
}
