package strategy

import "go-stock/backend/models"

// 主升低吸参数（对齐 KHunter 代码实际值）
const (
	udRecentDays       = 60   // 一拉 swing high 扫描窗口
	udRallyLowLookback = 45   // L 的回望窗口
	udRallyMinPct      = 0.30 // 一拉涨幅下限
	udRallyMaxPct      = 1.00 // 一拉涨幅上限
	udPullbackMinPct   = 0.18 // 二调回撤下限
	udPullbackGapPct   = 0.08 // 回撤 < 一拉涨幅 - 8%
	udSwingWindow      = 2
	udMinBodyPct       = 0.05 // 今日阳线实体 ≥5%
	udBreakoutMaxPct   = 0.02 // 贴紧突破 ≤2%
	udVolumeRatio      = 1.8
)

type UptrendDipBuy struct{}

func init() { register(UptrendDipBuy{}) }

func (s UptrendDipBuy) Name() string { return "主升低吸" }
func (s UptrendDipBuy) Weight() int  { return 50 }
func (s UptrendDipBuy) MinBars() int { return 106 }

func (s UptrendDipBuy) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	highs, lows := Highs(bars), Lows(bars)
	today := n - 1

	// 快速过滤：今日阳线实体 ≥5%
	if bars[today].Open <= 0 ||
		(bars[today].Close-bars[today].Open)/bars[today].Open < udMinBodyPct {
		return nil
	}

	// C1 一拉：最近 60 日窗口内 swing high，按高度从高到低找首个满足涨幅的 H
	winStart := n - udRecentDays
	var candidates []int
	for _, i := range SwingHighs(highs, udSwingWindow) {
		if i >= winStart && i < today {
			candidates = append(candidates, i)
		}
	}
	// 按高度降序
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if highs[candidates[j]] > highs[candidates[i]] {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	for _, iH := range candidates {
		H := highs[iH]
		// L = H 之前 45 日最低价
		lStart := iH - udRallyLowLookback
		if lStart < 0 {
			lStart = 0
		}
		L := lows[lStart]
		for i := lStart; i < iH; i++ {
			if lows[i] < L {
				L = lows[i]
			}
		}
		if L <= 0 {
			continue
		}
		rally := (H - L) / L
		if rally < udRallyMinPct || (udRallyMaxPct > 0 && rally > udRallyMaxPct) {
			continue
		}
		// C2 二调：H 之后最低点 P
		P := lows[iH+1]
		for i := iH + 1; i <= today; i++ {
			if lows[i] < P {
				P = lows[i]
			}
		}
		pullback := (H - P) / H
		if pullback < udPullbackMinPct || pullback >= rally-udPullbackGapPct || P <= L {
			continue
		}
		// C3 R：H 之后、今日之前最后一个 swing high，且 R < H
		R := 0.0
		for _, i := range SwingHighs(highs, udSwingWindow) {
			if i > iH && i < today && highs[i] < H {
				R = highs[i] // SwingHighs 正序返回，取最后一个
			}
		}
		if R <= 0 {
			continue
		}
		// C4 今日贴紧突破
		c := closes[today]
		if c <= R || (c-R)/R > udBreakoutMaxPct {
			continue
		}
		if a := AvgVolumeBefore(vols, today, 5); a <= 0 || vols[today]/a < udVolumeRatio {
			continue
		}
		if c <= SMAAt(closes, today, 5) {
			continue
		}
		vr := vols[today] / AvgVolumeBefore(vols, today, 5)
		return &Signal{
			StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
			Date: bars[today].TradeDate, KeyDate: bars[iH].TradeDate, KeyDateType: "一拉高点",
			Close: c, VolumeRatio: vr,
			Reasons: []string{"一拉二调结构", "贴紧突破R", "放量确认"},
			Details: map[string]any{"H": H, "L": L, "P": P, "R": R, "rally": rally, "pullback": pullback},
		}
	}
	return nil
}
