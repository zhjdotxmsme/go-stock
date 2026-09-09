package data

// strategy_combos_test.go：组合策略合成 K 线测试。
//  - 120 根上升趋势数据：多头 combo（classic/trendSwing/sats 等）Score>0，全部无 panic；
//  - 120 根下降趋势数据：离场/观望逻辑正确给 0 或低分，无 panic；
//  - 3 根短数据：全部策略安全返回 0 分；
//  - Overrides 生效：adx_threshold 提高后趋势波段得分下降。

import (
	"math"
	"strconv"
	"testing"
	"time"
)

// buildComboKLines 生成 n 根等差漂移的合成日 K（字段全为 string，日期自 2026-01-01 起）。
func buildComboKLines(n int, startPrice, drift float64) ([]KLineData, []float64, []float64, []float64, []float64) {
	// upBars=n、downStep=0 → 全程上涨段，退化为纯线性序列
	return buildComboWaveKLines(n, startPrice, drift, n, 0, 0)
}

// buildComboWaveKLines 生成「上涨 leg + 回调 leg」锯齿上升序列（需真实摆动点的
// 策略如 SMC 用）。upBars*upStep 为上涨段、downBars*downStep 为回调段，且回调幅度
// 小于上涨幅度保证整体上行；影线用黄金角伪随机独立生成，避免 high 在反转处恒等。
func buildComboWaveKLines(n int, startPrice, upStep float64, upBars int, downStep float64, downBars int) (klines []KLineData, closeP, highP, lowP, volP []float64) {
	klines = make([]KLineData, n)
	closeP = make([]float64, n)
	highP = make([]float64, n)
	lowP = make([]float64, n)
	volP = make([]float64, n)
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cycle := upBars + downBars
	price := startPrice
	for i := 0; i < n; i++ {
		open := price
		if i > 0 {
			if i%cycle < upBars {
				price += upStep
			} else {
				price -= downStep
			}
		}
		close := price
		// 影线独立于 open/close，逐根不同，保证 high/low 严格可比
		wickUp := 0.05 + 0.1*math.Abs(math.Sin(float64(i)*2.399963))
		wickDn := 0.05 + 0.1*math.Abs(math.Sin(float64(i)*1.7+0.8))
		high := math.Max(open, close) + wickUp
		low := math.Min(open, close) - wickDn
		vol := 1000000 + float64(i)*1000
		closeP[i], highP[i], lowP[i], volP[i] = close, high, low, vol
		klines[i] = KLineData{
			Day:           day.AddDate(0, 0, i).Format("2006-01-02"),
			Open:          strconv.FormatFloat(open, 'f', 2, 64),
			Close:         strconv.FormatFloat(close, 'f', 2, 64),
			High:          strconv.FormatFloat(high, 'f', 2, 64),
			Low:           strconv.FormatFloat(low, 'f', 2, 64),
			Volume:        strconv.FormatFloat(vol, 'f', 0, 64),
			Amount:        strconv.FormatFloat(vol*close, 'f', 2, 64),
			ChangePercent: strconv.FormatFloat((close-open)/open*100, 'f', 2, 64),
		}
	}
	return
}

// allComboStrategies 返回 10 个组合策略实例（注册由引擎负责，这里只为测试）。
func allComboStrategies() []ScoringStrategy {
	return []ScoringStrategy{
		&ComboClassicStrategy{},
		&ComboTrendSwingStrategy{},
		&ComboRangeDipStrategy{},
		&ComboVolPriceStrategy{},
		&ComboTTMBreakStrategy{},
		&ComboIchimokuStrategy{},
		&ComboAlligatorStrategy{},
		&ComboElderTripleStrategy{},
		&ComboSATSStrategy{},
		&ComboSMCStrategy{},
	}
}

func comboCodes() []string {
	return []string{
		"combo_classic", "combo_trend_swing", "combo_range_dip", "combo_vol_price",
		"combo_ttm_break", "combo_ichimoku", "combo_alligator", "combo_elder_triple",
		"combo_sats", "combo_smc",
	}
}

func comboCtx(closeP, highP, lowP, volP []float64, klines []KLineData) *StrategyContext {
	return &StrategyContext{
		KLines: klines, CloseP: closeP, HighP: highP, LowP: lowP, Volume: volP,
		StockCode: "000001", StockName: "合成测试",
	}
}

// checkComboResult 通用健全性断言：0<=Score<=100、Factors/Signal 非空。
func checkComboResult(t *testing.T, code string, res *StrategyResult) {
	t.Helper()
	if res == nil {
		t.Fatalf("%s: Score 返回 nil", code)
	}
	if res.Score < 0 || res.Score > 100 || math.IsNaN(res.Score) {
		t.Errorf("%s: Score=%.2f 越界", code, res.Score)
	}
	if res.Factors == nil {
		t.Errorf("%s: Factors 为 nil", code)
	}
	if res.Signal == "" {
		t.Errorf("%s: Signal 为空", code)
	}
}

func TestComboStrategiesUptrend(t *testing.T) {
	klines, closeP, highP, lowP, volP := buildComboKLines(120, 10, 0.05)
	ctx := comboCtx(closeP, highP, lowP, volP, klines)

	scores := make(map[string]float64)
	for i, st := range allComboStrategies() {
		code := comboCodes()[i]
		res := st.Score(ctx)
		checkComboResult(t, code, res)
		scores[code] = res.Score
		t.Logf("%s score=%.0f signal=%s", code, res.Score, res.Signal)
	}

	// 多头组合在稳定上升趋势中必须有得分（核心方向成立 → 至少基础分 40）
	for _, code := range []string{
		"combo_classic", "combo_trend_swing", "combo_vol_price", "combo_ichimoku",
		"combo_alligator", "combo_elder_triple", "combo_sats",
	} {
		if scores[code] <= 0 {
			t.Errorf("%s: 上升趋势中 Score=%.0f，期望 >0", code, scores[code])
		}
	}
}

func TestComboStrategiesDowntrend(t *testing.T) {
	// 锯齿阴跌：6 根×−0.3 下跌 + 3 根×+0.25 反弹（downStep 取负使「下跌段」反弹），
	// 整体下行且最后一段处于下跌 leg，使 MACD 死叉等离场状态明确
	// （纯线性序列下 DIF≈DEA 属浮点噪声，不可用作断言）。
	klines, closeP, highP, lowP, volP := buildComboWaveKLines(120, 20, -0.3, 6, -0.25, 3)
	ctx := comboCtx(closeP, highP, lowP, volP, klines)

	for i, st := range allComboStrategies() {
		code := comboCodes()[i]
		res := st.Score(ctx)
		checkComboResult(t, code, res)
		t.Logf("%s score=%.0f signal=%s", code, res.Score, res.Signal)
		switch code {
		case "combo_classic":
			// 无基础分，仅规则分，应处于低位（spec：离场命中归 0 或低位；
			// 是否触发零轴下死叉取决于最后一根 K 线在回调周期中的位置）
			if res.Score >= 50 {
				t.Errorf("%s: 下降趋势中 Score=%.0f，期望 <50", code, res.Score)
			}
		case "combo_sats", "combo_alligator", "combo_elder_triple", "combo_ichimoku":
			// 持续下跌应触发离场/观望 → 0 分
			if res.Score != 0 {
				t.Errorf("%s: 下降趋势中 Score=%.0f，期望 0（离场）", code, res.Score)
			}
		case "combo_trend_swing":
			// EMA 空头排列无基础分，仅规则分，应处于低位
			if res.Score >= 50 {
				t.Errorf("%s: 下降趋势中 Score=%.0f，期望 <50", code, res.Score)
			}
		}
	}
}

func TestComboStrategiesShortData(t *testing.T) {
	klines, closeP, highP, lowP, volP := buildComboKLines(3, 10, 0.05)
	ctx := comboCtx(closeP, highP, lowP, volP, klines)

	for i, st := range allComboStrategies() {
		code := comboCodes()[i]
		res := st.Score(ctx) // 不得 panic
		checkComboResult(t, code, res)
		if res.Score != 0 {
			t.Errorf("%s: 3根K线 Score=%.0f，期望 0", code, res.Score)
		}
	}
}

func TestComboStrategiesOverrides(t *testing.T) {
	klines, closeP, highP, lowP, volP := buildComboKLines(120, 10, 0.05)
	base := comboCtx(closeP, highP, lowP, volP, klines)
	tight := comboCtx(closeP, highP, lowP, volP, klines)
	// 线性趋势中 ADX 饱和于 100，取 150 作为现实中不可达的阈值
	tight.Overrides = map[string]float64{"adx_threshold": 150}

	var defaultScore, tightScore float64
	for _, st := range allComboStrategies() {
		if st.Code() != "combo_trend_swing" {
			continue
		}
		defaultScore = st.Score(base).Score
		tightScore = st.Score(tight).Score
	}
	if tightScore >= defaultScore {
		t.Errorf("adx_threshold=150 后得分未下降: %.0f -> %.0f", defaultScore, tightScore)
	}
}

// TestComboStrategiesZigzagSMC：SMC 依赖摆动点，纯线性序列不存在摆动点（无事件、
// 得 0 分是正确行为），这里用「上涨+回调」锯齿数据验证 BOS 顺势打分。
// 8 根×+0.3 上涨、6 根×−0.3 回调：回调段 ≥ internalLen(5)+1 保证摆动点可确认。
func TestComboStrategiesZigzagSMC(t *testing.T) {
	klines, closeP, highP, lowP, volP := buildComboWaveKLines(160, 10, 0.3, 8, 0.3, 6)
	ctx := comboCtx(closeP, highP, lowP, volP, klines)

	for _, st := range allComboStrategies() {
		res := st.Score(ctx) // 波浪数据下全部策略不得 panic
		checkComboResult(t, st.Code(), res)
		if st.Code() == "combo_smc" {
			t.Logf("combo_smc (zigzag) score=%.0f signal=%s", res.Score, res.Signal)
			if res.Score <= 0 {
				t.Errorf("combo_smc: 上升波浪数据 Score=%.0f，期望 >0（应存在多头结构事件）", res.Score)
			}
		}
	}
}
