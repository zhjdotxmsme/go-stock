package data

// 信号引擎确定性单测（不联网）。
// 覆盖设计规格第 5 节要求：MA 交叉方向、唐奇安突破边界(±1% 容差)、
// 动量分档边界、缺信号权重归一化(3 种缺失组合)、综合分边界 ±1。

import (
	"math"
	"testing"
)

// genCloses 生成 n 根收盘价：从 start 起每日按 r 递增（r 为小数，如 0.001 = +0.1%/日）。
func genCloses(n int, start, r float64) []float64 {
	out := make([]float64, n)
	v := start
	for i := 0; i < n; i++ {
		out[i] = v
		v *= (1 + r)
	}
	return out
}

// genBars 生成 n 根高低收：收盘从 start 起按 r 递增，high/low 各外扩 0.5%。
func genBars(n int, start, r float64) []HighLowClose {
	closes := genCloses(n, start, r)
	out := make([]HighLowClose, n)
	for i, c := range closes {
		out[i] = HighLowClose{High: c * 1.005, Low: c * 0.995, Close: c}
	}
	return out
}

func TestComputeTrend_Bull(t *testing.T) {
	// 持续上行：MA20>MA60 且收盘>MA20
	level, ok := ComputeTrend(genCloses(80, 100, 0.002))
	if !ok || level != TrendBull {
		t.Fatalf("want bull, got %q ok=%v", level, ok)
	}
}

func TestComputeTrend_Bear(t *testing.T) {
	level, ok := ComputeTrend(genCloses(80, 200, -0.002))
	if !ok || level != TrendBear {
		t.Fatalf("want bear, got %q ok=%v", level, ok)
	}
}

func TestComputeTrend_Transition(t *testing.T) {
	// 前 60 根线性缓涨（i=0..59: 100+0.2i → 末 111.8），后 15 根线性缓跌（每根 -0.3 → 末 107.3）。
	// MA20 = (111.0+111.2+111.4+111.6+111.8 + Σ111.5..107.3)/20 = (557.0+1641.0)/20 = 109.9
	// MA60 = (Σ(103.0..111.8) + 1641.0)/60 = (4833.0+1641.0)/60 = 107.9
	// → MA20 > MA60（多头交叉未翻转）但 last = 107.3 < MA20 = 109.9 → 过渡。
	closes := make([]float64, 75)
	for i := 0; i < 60; i++ {
		closes[i] = 100 + 0.2*float64(i)
	}
	for i := 0; i < 15; i++ {
		closes[60+i] = 111.8 - 0.3*float64(i+1)
	}
	level, ok := ComputeTrend(closes)
	if !ok {
		t.Fatalf("want ok, got false")
	}
	if level != TrendTransition {
		t.Fatalf("want transition, got %q", level)
	}
}

func TestComputeTrend_InsufficientData(t *testing.T) {
	if _, ok := ComputeTrend(genCloses(59, 100, 0.001)); ok {
		t.Fatalf("59 closes must be insufficient (need 60)")
	}
}

func TestComputeMomentum_Bands(t *testing.T) {
	// 强多头：20 日 > +3% 且 60 日 > +6%。0.4%/日 复利 20 日 ≈ +8.4%，60 日 ≈ +27%
	level, ok := ComputeMomentum(genCloses(80, 100, 0.004))
	if !ok || level != MomentumStrongBull {
		t.Fatalf("want strong_bull, got %q", level)
	}

	// 强空头：-0.4%/日 → r20≈-7.7%、r60≈-21.7%，双负且超阈值
	level, ok = ComputeMomentum(genCloses(80, 200, -0.004))
	if !ok || level != MomentumStrongBear {
		t.Fatalf("want strong_bear, got %q", level)
	}

	// 中性：前 60 根从 100 逐日降 0.5（closes[40]=80），末根 85 →
	// r20 = 85/80-1 = +6.25% > 0；r60 = 85/100-1 = -15% < 0 → 符号相反 → 中性
	closes := make([]float64, 61)
	for i := 0; i < 60; i++ {
		closes[i] = 100 - 0.5*float64(i)
	}
	closes[60] = 85
	level, ok = ComputeMomentum(closes)
	if !ok {
		t.Fatalf("want ok")
	}
	if level != MomentumNeutral {
		t.Fatalf("want neutral, got %q (r20>0, r60<0 expected)", level)
	}
}

func TestComputeMomentum_InsufficientData(t *testing.T) {
	if _, ok := ComputeMomentum(genCloses(60, 100, 0.001)); ok {
		t.Fatalf("60 closes must be insufficient (need 61)")
	}
}

// genWideChannelBars 生成 21 根 K 线：前 20 根构成宽通道（high=110, low=90, close=100），
// 最后一根由调用方指定——避免窄通道导致上下 ±1% 容差带重叠、不存在"区间"的问题。
func genWideChannelBars(last HighLowClose) []HighLowClose {
	out := make([]HighLowClose, 21)
	for i := 0; i < 20; i++ {
		out[i] = HighLowClose{High: 110, Low: 90, Close: 100}
	}
	out[20] = last
	return out
}

func TestComputeBreakout_Bands(t *testing.T) {
	// 多头突破边界：20 日通道 high=110。breakout 边界 = 110*0.99 = 108.9（容差内）。
	// 收盘 109 ∈ [108.9, 110] → up
	level, ok := ComputeBreakout(genWideChannelBars(HighLowClose{High: 110.5, Low: 99, Close: 109}))
	if !ok || level != BreakoutUp {
		t.Fatalf("want up at close=109 (just above 108.9 boundary), got %q ok=%v", level, ok)
	}

	// 收盘 108.8 < 108.9 但 > 90*1.01=90.9 → 区间
	level, ok = ComputeBreakout(genWideChannelBars(HighLowClose{High: 110, Low: 90, Close: 108.8}))
	if !ok || level != BreakoutRange {
		t.Fatalf("want range at close=108.8, got %q ok=%v", level, ok)
	}

	// 空头突破边界：通道 low=90，边界 = 90*1.01 = 90.9（容差内）。收盘 90.5 ∈ [90, 90.9] → down
	level, ok = ComputeBreakout(genWideChannelBars(HighLowClose{High: 102, Low: 89.5, Close: 90.5}))
	if !ok || level != BreakoutDown {
		t.Fatalf("want down at close=90.5 (below 90.9 boundary), got %q ok=%v", level, ok)
	}
	// 收盘 91：在 90.9 边界之外（缓冲带）→ 仍是区间
	level, ok = ComputeBreakout(genWideChannelBars(HighLowClose{High: 102, Low: 90.1, Close: 91}))
	if !ok || level != BreakoutRange {
		t.Fatalf("want range at close=91 (outside down tolerance), got %q ok=%v", level, ok)
	}

	// 明确的通道中部 → 区间
	level, ok = ComputeBreakout(genWideChannelBars(HighLowClose{High: 101, Low: 99, Close: 100}))
	if !ok || level != BreakoutRange {
		t.Fatalf("want range mid-channel, got %q ok=%v", level, ok)
	}
}

func TestComputeBreakout_InsufficientData(t *testing.T) {
	if _, ok := ComputeBreakout(genBars(20, 100, 0)); ok {
		t.Fatalf("20 bars must be insufficient (need 21)")
	}
}

func TestComputeCarry_Bands(t *testing.T) {
	level, ok := ComputeCarry(100, 98.9) // -1.1% 贴水
	if !ok || level != CarryBackwardation {
		t.Fatalf("want backwardation, got %q ok=%v", level, ok)
	}
	level, ok = ComputeCarry(100, 101.1) // +1.1% 升水
	if !ok || level != CarryContango {
		t.Fatalf("want contango, got %q ok=%v", level, ok)
	}
	level, ok = ComputeCarry(100, 100.5) // +0.5% 中性带内
	if !ok || level != CarryNeutral {
		t.Fatalf("want neutral, got %q ok=%v", level, ok)
	}
	if _, ok := ComputeCarry(0, 100); ok {
		t.Fatalf("near=0 must be invalid")
	}
}

func TestComputeOIQuadrant_AllFour(t *testing.T) {
	cases := []struct {
		price, oi float64
		want      OIQuadrant
	}{
		{+1.2, +500, OIQLongAttack},
		{+1.2, -500, OIQShortCover},
		{-1.2, +500, OIQShortAttack},
		{-1.2, -500, OIQLongExit},
	}
	for _, c := range cases {
		q, ok := ComputeOIQuadrant(c.price, c.oi)
		if !ok || q != c.want {
			t.Fatalf("price=%v oi=%v: want %q, got %q ok=%v", c.price, c.oi, c.want, q, ok)
		}
	}
	if _, ok := ComputeOIQuadrant(0, 0); ok {
		t.Fatalf("flat price + flat OI must be no-signal")
	}
}

func TestCompositeScore_FullBull(t *testing.T) {
	trend, momentum, breakout := string(TrendBull), string(MomentumStrongBull), string(BreakoutUp)
	carry, oi := string(CarryBackwardation), string(OIQLongAttack)
	score, n, err := CompositeScore(SignalParts{
		Trend: &trend, Momentum: &momentum, Breakout: &breakout, Carry: &carry, OI: &oi,
	})
	if err != nil || n != 5 {
		t.Fatalf("score=%v n=%d err=%v", score, n, err)
	}
	// 0.3*1 + 0.25*1 + 0.2*1 + 0.15*0.5 + 0.1*1 = 0.30+0.25+0.20+0.075+0.10 = 0.925
	if math.Abs(score-0.925) > 1e-9 {
		t.Fatalf("score=%v want 0.925", score)
	}
	if Verdict(score) != "偏多" {
		t.Fatalf("want 偏多")
	}
}

func TestCompositeScore_MissingCombos_Normalization(t *testing.T) {
	// 组合 1：现货标的可能只有 trend+momentum+breakout（缺 carry/oi）。
	// 全 0 信号 → score 0（无论权重如何归一化）。
	trend, momentum, breakout := string(TrendTransition), string(MomentumNeutral), string(BreakoutRange)
	score, n, err := CompositeScore(SignalParts{Trend: &trend, Momentum: &momentum, Breakout: &breakout})
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if score != 0 {
		t.Fatalf("all-neutral → 0, got %v", score)
	}

	// 组合 2：仅单信号 trend=bull → 归一化后 score 必须为 1（边界 +1）。
	trendBull := string(TrendBull)
	score, n, err = CompositeScore(SignalParts{Trend: &trendBull})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if score != 1 {
		t.Fatalf("single bull trend → +1, got %v", score)
	}
	score, _, _ = CompositeScore(SignalParts{Trend: pointer(string(TrendBear))})
	if score != -1 {
		t.Fatalf("single bear trend → -1, got %v", score)
	}

	// 组合 3：缺 carry（现货），其余全空 → 0
	trendT, momentumN, breakoutR := string(TrendTransition), string(MomentumNeutral), string(BreakoutRange)
	score, n, err = CompositeScore(SignalParts{Trend: &trendT, Momentum: &momentumN, Breakout: &breakoutR, Carry: nil, OI: nil})
	if err != nil || n != 3 || score != 0 {
		t.Fatalf("spot-like 3 signals: n=%d score=%v err=%v", n, score, err)
	}
}

func TestCompositeScore_NoSignals(t *testing.T) {
	if _, _, err := CompositeScore(SignalParts{}); err == nil {
		t.Fatalf("must error when no signal present")
	}
}

func pointer(s string) *string { return &s }
