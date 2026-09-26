package scorer

import (
	"math"

	"go-stock/backend/models"
)

// 资金面子权重（对齐 KHunter 代码实际值 0.55/0.10/0.10/0.25）
const (
	mfWMain  = 0.55
	mfWBig   = 0.10
	mfWNorth = 0.10 // 北向无数据源，固定中性 50 分
	mfWDir   = 0.25

	mfNorthNeutral   = 50.0
	mfVetoMainAmount = -1e8 // 5日主力净额否决阈值（元）
	mfVetoMainRatio  = -5.0 // 大单净流入占比 5 日均值（%）
	mfVetoDistRatio  = 1.0  // 出货信号阈值（%）
)

// ScoreMoneyFlow 资金面评分。flows 正序，至少 5 天，否则 degraded 中性 30。
func ScoreMoneyFlow(flows []models.KhunterMoneyFlowDaily) DimScore {
	if len(flows) < 5 {
		return DimScore{Score: 30, Degraded: true, Reason: "资金流数据不足5天"}
	}
	f := flows[len(flows)-5:]

	// 主力净流入（5日累计）
	mainSum := 0.0
	for _, d := range f {
		mainSum += d.MainNet
	}
	mainScore := 0.0
	switch {
	case mainSum > 1e8:
		mainScore = 100
	case mainSum > 5e7:
		mainScore = 80
	case mainSum > 1e6:
		mainScore = 60
	case mainSum > 0:
		mainScore = 40
	case mainSum == 0:
		mainScore = 30
	default:
		mainScore = -20
	}

	// 大单占比逐日累加
	bigScore := 0.0
	lgRatioSum, smRatioSum := 0.0, 0.0
	for _, d := range f {
		switch {
		case d.LgNetRatio > 5:
			bigScore += 40
		case d.LgNetRatio > 1:
			bigScore += 20
		case d.LgNetRatio > 0:
			bigScore += 10
		case d.LgNetRatio < 0:
			bigScore -= 30
		}
		lgRatioSum += d.LgNetRatio
		smRatioSum += d.SmNetRatio
	}
	lgRatioAvg := lgRatioSum / 5
	smRatioAvg := smRatioSum / 5

	// 方向分（5日累计大单/小单占比）
	dirScore := 0.0
	switch {
	case lgRatioSum > 0 && smRatioSum < 0:
		dirScore = 100
	case lgRatioSum > 0 && smRatioSum >= 0:
		dirScore = 60
	case lgRatioSum < 0 && smRatioSum > 0:
		dirScore = -100
	}

	total := mainScore*mfWMain + bigScore*mfWBig + mfNorthNeutral*mfWNorth + dirScore*mfWDir
	total = math.Max(-100, math.Min(100, total))
	out := DimScore{Score: total, Detail: map[string]any{
		"main_5d": mainSum, "lg_ratio_avg": lgRatioAvg, "sm_ratio_avg": smRatioAvg,
	}}

	// 一票否决 1：主力净额 < -1亿 且大单占比均值 < -5%
	if mainSum < mfVetoMainAmount && lgRatioAvg < mfVetoMainRatio {
		out.Veto, out.Score, out.Reason = true, -100, "主力资金大幅流出"
		return out
	}
	// 一票否决 2：出货信号（大单流出>1% 且小单流入>1%）
	if lgRatioAvg < -mfVetoDistRatio && smRatioAvg > mfVetoDistRatio {
		out.Veto, out.Score, out.Reason = true, -100, "出货信号（大单出小单进）"
	}
	return out
}
