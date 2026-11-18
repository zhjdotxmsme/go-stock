package strategy

import (
	"strings"

	"go-stock/backend/data/trading"
	"go-stock/backend/models"
)

// 涨停回马枪参数（对齐 KHunter 代码实际值）
const (
	lpLimitLookback    = 6   // 近 6 日内有涨停
	lpVolRatioLimitUp  = 2.2 // 涨停日量比（前5日均量）
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

// lpEffectivePrevClose 第 i 根的有效前收：优先 PrevClose 字段，旧数据缺失时回退前一根收盘价。
// 两者都无效返回 0，调用方必须跳过涨跌停判定（不得用近似涨幅命名涨停）。
func lpEffectivePrevClose(bars []models.KLineBar, i int) float64 {
	if bars[i].PrevClose > 0 {
		return bars[i].PrevClose
	}
	if i > 0 && bars[i-1].Close > 0 {
		return bars[i-1].Close
	}
	return 0
}

// lpIsSTName 按股票简称判断 ST（与 IsInvalidName 同一口径）。
func lpIsSTName(name string) bool {
	return strings.HasPrefix(name, "ST") || strings.HasPrefix(name, "*ST")
}

func (s LimitUpPullback) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate != selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	today := n - 1

	// 1. 近 6 日找最近一个真实涨停日（收盘=板块真实涨停价 且量比≥2.2）
	//    创业板/科创板 20%、主板 10%、主板 ST 5%、北交所 30%：不再以固定 9.5% 涨幅命名涨停。
	lu := -1
	for i := today - 1; i >= 0 && i >= today-lpLimitLookback; i-- {
		prev := lpEffectivePrevClose(bars, i)
		if prev <= 0 {
			continue // 关键日前收无效：无法判定真实涨停，跳过该日
		}
		limit, ok := trading.DailyPriceLimit(bars[0].StockCode, prev, lpIsSTName(stockName))
		if !ok || !trading.ClosedAtLimitUp(bars[i], limit) {
			continue
		}
		if a := AvgVolumeBefore(vols, i, 5); a > 0 && vols[i]/a >= lpVolRatioLimitUp {
			lu = i
			break
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
