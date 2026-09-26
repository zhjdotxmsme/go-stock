package strategy

import (
	"math"

	"go-stock/backend/models"
)

// W底参数（对齐 KHunter 代码实际值）
const (
	wbPatternDays    = 40   // 形态扫描窗口
	wbExcludeLatest  = 5    // 排除最新 5 根（留给突破检测）
	wbLowWindow      = 5    // 局部低点窗口
	wbMinGap         = 10   // 两底间隔（严格大于）
	wbBottomDiff     = 0.03 // 两底价差上限
	wbNecklineSpace  = 1.1  // H ≥ L1 × 1.1
	wbBreakoutMargin = 1.01 // 突破确认 close ≥ H × 1.01
	wbVolExpandRatio = 1.2  // 放量确认日量比（代码实际值，非注释的 1.5）
	wbConfirmRise    = 0.08 // 放量确认日涨幅 > 8%
	wbQuickCheckRise = 0.05 // 预检：近5日有涨幅>5%
	wbPriorDropBars  = 30
	wbPriorDropRatio = 1.2  // L1 前 30 日最高价 > L1 × 1.2
	wbSupportRatio   = 0.02 // 突破后 close ≥ H × 0.98
)

type WBottom struct{}

func init() { register(WBottom{}) }

func (s WBottom) Name() string { return "W底" }
func (s WBottom) Weight() int  { return 50 }
func (s WBottom) MinBars() int { return 60 }

func (s WBottom) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
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

	// 1. 预检：近5日存在涨幅>5%
	quick := false
	for i := today - 4; i <= today; i++ {
		if i > 0 && PctChange(bars, i) > wbQuickCheckRise {
			quick = true
			break
		}
	}
	if !quick {
		return nil
	}

	// 2. 放量确认日：近5日内涨幅>8% 且量≥前5日均量×1.2（取最近一个）
	confirm := -1
	for i := today; i >= today-4 && i > 0; i-- {
		if PctChange(bars, i) > wbConfirmRise {
			if a := AvgVolumeBefore(vols, i, 5); a > 0 && vols[i] >= a*wbVolExpandRatio {
				confirm = i
				break
			}
		}
	}
	if confirm < 0 {
		return nil
	}

	// 3. 形态窗口（排除最新5根）找局部低点，间隔不足时保留更低者
	end := today - wbExcludeLatest
	start := end - wbPatternDays
	if start < wbLowWindow {
		start = wbLowWindow
	}
	var lowIdx []int
	for i := start; i <= end; i++ {
		if IsLocalLow(lows, i, wbLowWindow) {
			if m := len(lowIdx); m > 0 && i-lowIdx[m-1] <= wbMinGap {
				// 间隔不足：保留价格更低者
				if lows[i] < lows[lowIdx[m-1]] {
					lowIdx[m-1] = i
				}
				continue
			}
			lowIdx = append(lowIdx, i)
		}
	}
	if len(lowIdx) < 2 {
		return nil
	}
	iL2 := lowIdx[len(lowIdx)-1] // 最新低点
	iL1 := lowIdx[len(lowIdx)-2] // 次新低点
	L1, L2 := lows[iL1], lows[iL2]
	if iL2-iL1 <= wbMinGap || L1 <= 0 {
		return nil
	}
	if math.Abs(L2-L1)/L1 > wbBottomDiff {
		return nil
	}
	// 颈线 H = 两底之间最高价
	H := 0.0
	for i := iL1; i <= iL2; i++ {
		if highs[i] > H {
			H = highs[i]
		}
	}
	if H <= L1 || H <= L2 || H < L1*wbNecklineSpace {
		return nil
	}

	// 4. 颈线突破：确认日 close ≥ H×1.01 且前一日 close < H
	if closes[confirm] < H*wbBreakoutMargin || closes[confirm-1] >= H {
		return nil
	}

	// 5. 趋势：MA10 > MA30（今日）
	if SMAAt(closes, today, 10) <= SMAAt(closes, today, 30) {
		return nil
	}

	// 6. 假W底过滤
	priorStart := iL1 - wbPriorDropBars
	if priorStart < 0 {
		priorStart = 0
	}
	priorHigh := 0.0
	for i := priorStart; i < iL1; i++ {
		if highs[i] > priorHigh {
			priorHigh = highs[i]
		}
	}
	if priorHigh <= L1*wbPriorDropRatio {
		return nil
	}
	for i := confirm; i <= today; i++ {
		if closes[i] < H*(1-wbSupportRatio) {
			return nil
		}
	}

	vr := 0.0
	if a := AvgVolumeBefore(vols, today, 5); a > 0 {
		vr = vols[today] / a
	}
	return &Signal{
		StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
		Date: bars[today].TradeDate, KeyDate: bars[confirm].TradeDate, KeyDateType: "放量确认日",
		Close: closes[today], VolumeRatio: vr,
		Reasons: []string{"双底形态", "颈线突破", "MA10上穿MA30"},
		Details: map[string]any{"L1": L1, "L2": L2, "neckline": H,
			"l1_date": bars[iL1].TradeDate, "l2_date": bars[iL2].TradeDate},
	}
}
