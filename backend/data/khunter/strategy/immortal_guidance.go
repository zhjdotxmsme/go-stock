package strategy

import "go-stock/backend/models"

// 仙人指路参数（对齐 KHunter 代码实际值）
const (
	igSurgeThreshold   = 0.08 // 冲高幅度
	igUpperShadowRatio = 0.04 // 上影线/high
	igVolumeRatioMin   = 1.5  // 信号日量比（对前5日均量，不含当日）
	igLookbackDays     = 3    // 信号日在 T-1~T-3
	igTrendLookback    = 20
	igTrendR2Min       = 0.5
	igAntiBodyRatio    = 0.50 // 反包目标 = 上影线 50%
)

type ImmortalGuidance struct{}

func init() { register(ImmortalGuidance{}) }

func (s ImmortalGuidance) Name() string { return "仙人指路" }
func (s ImmortalGuidance) Weight() int  { return 70 }
func (s ImmortalGuidance) MinBars() int { return 30 }

func (s ImmortalGuidance) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	// 停牌/退市过滤：最后一根日期早于选股日则出局
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)

	// 趋势过滤：最近 20 日收盘 slope>0 且 R²≥0.5
	slope, r2 := LinReg(closes[n-igTrendLookback:])
	if slope <= 0 || r2 < igTrendR2Min {
		return nil
	}

	today := n - 1
	for off := 1; off <= igLookbackDays; off++ {
		i := today - off
		if i < 21 { // 需前5日均量 + MA20
			continue
		}
		prev := closes[i-1]
		if prev == 0 || (bars[i].High-prev)/prev < igSurgeThreshold {
			continue
		}
		// 上影线（阳线 high-close，阴线 high-open）
		us := bars[i].High - bars[i].Close
		if bars[i].Close < bars[i].Open {
			us = bars[i].High - bars[i].Open
		}
		if bars[i].High == 0 || us/bars[i].High < igUpperShadowRatio {
			continue
		}
		avg5 := AvgVolumeBefore(vols, i, 5)
		if avg5 <= 0 || vols[i]/avg5 < igVolumeRatioMin {
			continue
		}
		ma5, ma10, ma20 := SMAAt(closes, i, 5), SMAAt(closes, i, 10), SMAAt(closes, i, 20)
		if !(ma5 > ma10 && ma10 > ma20 && ma20 > 0) {
			continue
		}
		// 反包目标价 = 上影线 50% 位置
		target := bars[i].Close + us*igAntiBodyRatio
		if bars[i].Close < bars[i].Open {
			target = bars[i].Open + us*igAntiBodyRatio
		}
		// 提前反包排除：信号日之后、今日之前 close 均须 < target
		preBroken := false
		for j := i + 1; j < today; j++ {
			if closes[j] >= target {
				preBroken = true
				break
			}
		}
		if preBroken {
			continue
		}
		// 今日反包确认
		if closes[today] < target || closes[today] < SMAAt(closes, today, 5) {
			continue
		}
		vr := 0.0
		if a := AvgVolumeBefore(vols, today, 5); a > 0 {
			vr = vols[today] / a
		}
		return &Signal{
			StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
			Date: bars[today].TradeDate, KeyDate: bars[i].TradeDate, KeyDateType: "信号日",
			Close: closes[today], VolumeRatio: vr,
			Reasons: []string{"冲高回落长上影", "放量", "均线多头", "今日反包确认"},
			Details: map[string]any{"signal_idx": i, "target": target, "r2": r2},
		}
	}
	return nil
}
