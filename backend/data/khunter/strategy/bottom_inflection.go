package strategy

import (
	"go-stock/backend/models"
)

// 底部趋势拐点参数（对齐 KHunter 代码实际值）
const (
	biLookbackWindow = 120  // 深跌/最低价窗口
	biAnchorOffMin   = 3    // 锚点在倒数第 3~5 根（跳过最近 2 根）
	biAnchorOffMax   = 5
	biAnchorRise     = 0.08 // 锚点涨幅 > 8%
	biAnchorVolRatio = 2.5  // 锚点量比（对前10日均量）
	biNearLowMax     = 0.15 // 锚点 close 距窗口最低价 ≤ 15%
	biDeclineMin     = 0.45 // 深跌 > 45%（先高后低）
	biDivergenceDays = 20   // MACD 背离观察窗口（锚点之前）
)

type BottomInflection struct{}

func init() { register(BottomInflection{}) }

func (s BottomInflection) Name() string { return "底部趋势拐点" }
func (s BottomInflection) Weight() int  { return 50 }
func (s BottomInflection) MinBars() int { return 120 }

// biEMA 递推式指数移动平均，对齐 pandas ewm(adjust=False)：
// ema[0]=x[0]，ema[i]=alpha*x[i]+(1-alpha)*ema[i-1]，alpha=2/(period+1)。
func biEMA(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n)
	if n == 0 {
		return out
	}
	alpha := 2.0 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < n; i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

// biMACD 递推式 EMA 级联构造 MACD 序列（DIF/DEA/HIST），hist = dif - dea。
func biMACD(close []float64, fast, slow, signal int) (dif, dea, hist []float64) {
	ef := biEMA(close, fast)
	es := biEMA(close, slow)
	n := len(close)
	dif = make([]float64, n)
	for i := 0; i < n; i++ {
		dif[i] = ef[i] - es[i]
	}
	dea = biEMA(dif, signal)
	hist = make([]float64, n)
	for i := 0; i < n; i++ {
		hist[i] = dif[i] - dea[i]
	}
	return dif, dea, hist
}

func (s BottomInflection) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	lows, highs := Lows(bars), Highs(bars)
	today := n - 1

	for off := biAnchorOffMin; off <= biAnchorOffMax; off++ {
		anchor := today - off
		if anchor < biLookbackWindow {
			continue
		}
		// 锚点：涨幅>8% 且量比（前10日）≥2.5
		if PctChange(bars, anchor) <= biAnchorRise {
			continue
		}
		if a := AvgVolumeBefore(vols, anchor, 10); a <= 0 || vols[anchor]/a < biAnchorVolRatio {
			continue
		}
		// 窗口最低价（含锚点当日往前 120 根）
		winStart := anchor - biLookbackWindow + 1
		lowest := lows[winStart]
		for i := winStart; i <= anchor; i++ {
			if lows[i] < lowest {
				lowest = lows[i]
			}
		}
		if lowest <= 0 || (closes[anchor]-lowest)/lowest > biNearLowMax {
			continue
		}
		// 锚点之后所有 close ≥ 锚点开盘价
		broken := false
		for i := anchor + 1; i <= today; i++ {
			if closes[i] < bars[anchor].Open {
				broken = true
				break
			}
		}
		if broken {
			continue
		}
		// 深跌：窗口内先出现最高价、后出现最低价，跌幅 >45%
		hiIdx, hiVal := winStart, highs[winStart]
		for i := winStart; i <= anchor; i++ {
			if highs[i] > hiVal {
				hiVal, hiIdx = highs[i], i
			}
		}
		loVal := hiVal
		found := false
		for i := hiIdx; i <= anchor; i++ {
			if lows[i] < loVal {
				loVal = lows[i]
				found = true
			}
		}
		if !found || hiVal <= 0 || (hiVal-loVal)/hiVal <= biDeclineMin {
			continue
		}
		// MACD 底背离：锚点前 20 日分两半，各取最低价所在日的 low 与 hist
		dEnd := anchor - 1
		dStart := dEnd - biDivergenceDays + 1
		if dStart < 1 {
			continue
		}
		_, _, hist := biMACD(closes[:dEnd+1], 12, 26, 9)
		mid := dStart + biDivergenceDays/2
		lowA, histA := lows[dStart], hist[dStart]
		for i := dStart; i < mid; i++ {
			if lows[i] < lowA {
				lowA, histA = lows[i], hist[i]
			}
		}
		lowB, histB := lows[mid], hist[mid]
		for i := mid; i <= dEnd; i++ {
			if lows[i] < lowB {
				lowB, histB = lows[i], hist[i]
			}
		}
		if !(lowB < lowA && histB > histA) {
			continue
		}
		vr := 0.0
		if a := AvgVolumeBefore(vols, today, 5); a > 0 {
			vr = vols[today] / a
		}
		return &Signal{
			StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
			Date: bars[today].TradeDate, KeyDate: bars[anchor].TradeDate, KeyDateType: "放量长阳日",
			Close: closes[today], VolumeRatio: vr,
			Reasons: []string{"深度下跌后放量长阳", "回调不破起涨点", "MACD底背离"},
			Details: map[string]any{"anchor": bars[anchor].TradeDate, "decline": (hiVal - loVal) / hiVal,
				"low_a": lowA, "low_b": lowB, "hist_a": histA, "hist_b": histB},
		}
	}
	return nil
}
