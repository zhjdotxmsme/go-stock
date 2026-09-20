package data

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
)

func mkBars(rows [][4]float64) []datasource.KLineBar {
	// 每行 [open, high, low, close]，日期从 2026-01-01 起逐日递增
	bars := make([]datasource.KLineBar, 0, len(rows))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for i, r := range rows {
		bars = append(bars, datasource.KLineBar{
			Time: base.AddDate(0, 0, i),
			Open: r[0], High: r[1], Low: r[2], Close: r[3],
		})
	}
	return bars
}

func TestCalcGapAnalysisUpGapUnfilled(t *testing.T) {
	// 第2天向上跳空：前高10.00，当日低10.50 → 缺口 [10.00,10.50]，宽度 5%
	// 之后价格一直在缺口上方运行 → 未回补
	bars := mkBars([][4]float64{
		{9.5, 10.0, 9.4, 9.8},
		{10.6, 11.0, 10.5, 10.9},
		{10.9, 11.2, 10.7, 11.0},
	})
	gap := calcGapAnalysis(bars, 60)
	if len(gap.Recent) != 1 {
		t.Fatalf("expect 1 gap, got %d", len(gap.Recent))
	}
	g := gap.Recent[0]
	if g.Direction != "up" || g.Filled {
		t.Errorf("expect unfilled up gap, got dir=%s filled=%v", g.Direction, g.Filled)
	}
	if g.GapLow != 10.0 || g.GapHigh != 10.5 {
		t.Errorf("gap zone = [%v,%v], want [10,10.5]", g.GapLow, g.GapHigh)
	}
	if gap.UnfilledCount != 1 {
		t.Errorf("UnfilledCount = %d, want 1", gap.UnfilledCount)
	}
}

func TestCalcGapAnalysisDownGapFilled(t *testing.T) {
	// 第2天向下跳空：前低9.00，当日高8.50 → 缺口 [8.50,9.00]
	// 第3天最高 8.90 未触及 9.00；第4天最高 9.10 → 回补
	bars := mkBars([][4]float64{
		{9.5, 10.0, 9.0, 9.2},
		{8.4, 8.5, 8.0, 8.2},
		{8.2, 8.9, 8.1, 8.6},
		{8.7, 9.1, 8.5, 9.0},
	})
	gap := calcGapAnalysis(bars, 60)
	if len(gap.Recent) != 1 {
		t.Fatalf("expect 1 gap, got %d", len(gap.Recent))
	}
	g := gap.Recent[0]
	if g.Direction != "down" || !g.Filled {
		t.Errorf("expect filled down gap, got dir=%s filled=%v", g.Direction, g.Filled)
	}
	if g.GapLow != 8.5 || g.GapHigh != 9.0 {
		t.Errorf("gap zone = [%v,%v], want [8.5,9]", g.GapLow, g.GapHigh)
	}
	if g.FillDate != "2026-01-04" {
		t.Errorf("FillDate = %q, want 2026-01-04", g.FillDate)
	}
	if gap.UnfilledCount != 0 {
		t.Errorf("UnfilledCount = %d, want 0", gap.UnfilledCount)
	}
}

func TestCalcGapAnalysisFiltersTinyGaps(t *testing.T) {
	// 宽度 0.01（约0.1%）< gapMinWidthPct(0.2%)，应被过滤
	bars := mkBars([][4]float64{
		{9.5, 10.0, 9.4, 9.8},
		{10.02, 10.3, 10.01, 10.2},
	})
	gap := calcGapAnalysis(bars, 60)
	if len(gap.Recent) != 0 {
		t.Errorf("expect tiny gap filtered, got %d", len(gap.Recent))
	}
	if gap.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestCalcGapAnalysisPartiallyFilledStaysUnfilled(t *testing.T) {
	// 向上缺口 [10.00,10.50]，后续最低 10.30 进入缺口区间但未触及下沿 10.00 → 仍算未回补
	bars := mkBars([][4]float64{
		{9.5, 10.0, 9.4, 9.8},
		{10.6, 11.0, 10.5, 10.9},
		{10.9, 11.2, 10.3, 11.0},
	})
	gap := calcGapAnalysis(bars, 60)
	if len(gap.Recent) != 1 || gap.Recent[0].Filled {
		t.Errorf("expect partially filled gap still unfilled, got %+v", gap.Recent)
	}
}

func TestGapStatusText(t *testing.T) {
	a := &GapAnalysis{Window: 60, Recent: []GapInfo{
		{Direction: "up", Date: "2026-01-02", GapLow: 10, GapHigh: 10.5, AgeDays: 3, WidthPct: 5},
	}}
	got := gapStatusText(a)
	if got == "" || !strings.Contains(got, "向上跳空") {
		t.Errorf("gapStatusText = %q", got)
	}
}
