package data

import (
	"strings"
	"testing"

	"go-stock/backend/models"
)

func TestDailyPickTechBriefCoversIndicators(t *testing.T) {
	p := models.DailyPick{
		ClosePrice: 18, OpenPrice: 17.4, HighPrice: 18.2, LowPrice: 17.3,
		ChangePercent: 1.2, Change60dPct: 12,
		Ma5: 17.8, Ma10: 17.4, Ma20: 17.2, Ma60: 16.5,
		Rsi14: 62, Macd: 0.25, MacdSignal: 0.10,
		KdjK: 65, KdjD: 55, KdjJ: 85,
		BollMid: 17.2, BollUp: 18.6, BollDown: 15.8,
		Breakout20dPct: 1.2, AmplitudePct: 2.5, BodyPct: 0.7,
		TurnoverRate: 4, VolumeRatio20d: 1.6, VolumeFactor: 0.75,
		Atr14: 0.30, Volatility20dPct: 22, MaxDrawdownPct: -8,
	}
	sections := DailyPickTechBrief(p)
	if len(sections) != 6 {
		t.Fatalf("sections: %d", len(sections))
	}
	joined := joinSections(sections)
	for _, want := range []string{"MA5", "MA60", "RSI(14)", "MACD", "KDJ", "布林", "20 日", "ATR", "回撤", "偏多"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
}

func TestDailyPickTechBriefBearish(t *testing.T) {
	p := models.DailyPick{
		ClosePrice: 10, OpenPrice: 10.8, HighPrice: 10.9, LowPrice: 9.8,
		ChangePercent: -3, Change60dPct: -22,
		Ma5: 11, Ma10: 12, Ma20: 13, Ma60: 14,
		Rsi14: 35, Macd: -0.40, MacdSignal: -0.10,
		KdjK: 40, KdjD: 55, KdjJ: 10,
		BollMid: 12, BollUp: 13.5, BollDown: 10.5,
		Breakout20dPct: -12, AmplitudePct: 6, BodyPct: 0.7,
		TurnoverRate: 0.8, VolumeRatio20d: 0.5,
		Atr14: 0.8, Volatility20dPct: 60, MaxDrawdownPct: -32,
	}
	joined := joinSections(DailyPickTechBrief(p))
	if !strings.Contains(joined, "偏空") {
		t.Fatalf("want 偏空, got %s", joined)
	}
	if !strings.Contains(joined, "空头排列") || !strings.Contains(joined, "跌破布林下轨") {
		t.Fatalf("bearish evidence missing: %s", joined)
	}
}

func TestDailyPickTechBriefEmpty(t *testing.T) {
	joined := joinSections(DailyPickTechBrief(models.DailyPick{}))
	if !strings.Contains(joined, "不足") {
		t.Fatalf("empty pick should admit missing data, got %s", joined)
	}
}

func joinSections(sections []models.TechSection) string {
	var b strings.Builder
	for _, s := range sections {
		b.WriteString(s.Title)
		b.WriteString(s.Text)
	}
	return b.String()
}
