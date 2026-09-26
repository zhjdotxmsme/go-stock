package strategy

import "go-stock/backend/models"

// 涨停回马枪参数（对齐 KHunter 代码实际值）
const (
	lpLimitUpThreshold = 0.095 // 涨停判定涨幅
	lpLimitLookback    = 6     // 近 6 日内有涨停
	lpVolRatioLimitUp  = 2.2   // 涨停日量比（前5日均量）
	lpPullbackMaxDays  = 9
	lpPullbackRangeMax = 0.15 // 回调振幅上限
	lpSupportRatio     = 0.95 // 回调收盘下限（对涨停收盘）
	lpResistanceRatio  = 1.05 // 回调收盘上限
	lpVolShrinkRatio   = 0.5  // 缩量判定
	lpTodayRiseMin     = 0.02 // 今日涨幅
	lpTodayVolRatio    = 1.5  // 今日量 ≥ 昨日×1.5
)

type LimitUpPullback struct{}

func init() { register(LimitUpPullback{}) }

func (s LimitUpPullback) Name() string { return "涨停回马枪" }
func (s LimitUpPullback) Weight() int  { return 50 }
func (s LimitUpPullback) MinBars() int { return 15 }

func (s LimitUpPullback) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	today := n - 1

	// 1. 近 6 日找最近一个涨停日（涨幅≥9.5% 且量比≥2.2）
	lu := -1
	for i := today - 1; i >= 0 && i >= today-lpLimitLookback; i-- {
		if PctChange(bars, i) >= lpLimitUpThreshold {
			if a := AvgVolumeBefore(vols, i, 5); a > 0 && vols[i]/a >= lpVolRatioLimitUp {
				lu = i
				break
			}
		}
	}
	if lu < 0 || today-lu > lpPullbackMaxDays {
		return nil
	}
	luClose, luVol := closes[lu], vols[lu]

	// 2. 回调期（lu+1 ~ today-1）检查
	maxHigh, minLow := 0.0, 0.0
	hasLower, hasShrink := false, false
	for j := lu + 1; j < today; j++ {
		if bars[j].High > maxHigh {
			maxHigh = bars[j].High
		}
		if minLow == 0 || bars[j].Low < minLow {
			minLow = bars[j].Low
		}
		c := closes[j]
		if c < luClose*lpSupportRatio || c > luClose*lpResistanceRatio {
			return nil
		}
		if c < luClose {
			hasLower = true
		}
		if vols[j] <= luVol*lpVolShrinkRatio {
			hasShrink = true
		}
	}
	if today-lu >= 2 {
		if maxHigh > 0 && (maxHigh-minLow)/maxHigh > lpPullbackRangeMax {
			return nil
		}
		if !hasLower || !hasShrink {
			return nil
		}
	}

	// 3. 今日确认：涨幅>2%，量≥昨日×1.5，close>MA5
	if PctChange(bars, today) <= lpTodayRiseMin {
		return nil
	}
	if vols[today] < vols[today-1]*lpTodayVolRatio {
		return nil
	}
	if closes[today] <= SMAAt(closes, today, 5) {
		return nil
	}
	vr := 0.0
	if a := AvgVolumeBefore(vols, today, 5); a > 0 {
		vr = vols[today] / a
	}
	return &Signal{
		StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
		Date: bars[today].TradeDate, KeyDate: bars[lu].TradeDate, KeyDateType: "涨停日",
		Close: closes[today], VolumeRatio: vr,
		Reasons: []string{"涨停后回调企稳", "缩量洗盘", "今日放量启动"},
		Details: map[string]any{"limit_up_close": luClose, "pullback_days": today - lu},
	}
}
