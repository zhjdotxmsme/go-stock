package data

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

func deepBars(rows [][4]float64) []datasource.KLineBar {
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

func TestComputeKeyPriceLevelsBasic(t *testing.T) {
	// 30 根K线：价格从 9 涨到 11，最后一根收 10.0
	rows := make([][4]float64, 0, 30)
	price := 9.0
	for i := 0; i < 30; i++ {
		price = 9.0 + float64(i)*0.07
		rows = append(rows, [4]float64{price, price + 0.1, price - 0.1, price})
	}
	bars := deepBars(rows)
	ind := &IndicatorResult{
		MA:   map[string]float64{"MA20": 10.5, "MA60": 0},
		BOLL: map[string]float64{"Up": 10.9, "Mid": 10.2, "Down": 9.5},
		ATR:  0.3,
	}
	levels := computeKeyPriceLevels(bars, ind)
	if levels.Current != 10.9 && levels.Current != roundPrice(bars[len(bars)-1].Close) {
		t.Errorf("Current = %v", levels.Current)
	}
	if len(levels.Resistance) == 0 {
		t.Error("expect at least 1 resistance (20日高点 above current)")
	}
	for _, r := range levels.Resistance {
		if r.Price < levels.Current {
			t.Errorf("resistance %v below current %v", r.Price, levels.Current)
		}
	}
	for _, s := range levels.Support {
		if s.Price > levels.Current {
			t.Errorf("support %v above current %v", s.Price, levels.Current)
		}
	}
	if len(levels.SellPoints) == 0 {
		t.Error("expect ATR stop in sell points")
	}
}

func TestComputeKeyPriceLevelsGapSupport(t *testing.T) {
	// 构造向上跳空缺口 [10.0,10.5]，现价 10.8：缺口上沿应为支撑候选
	rows := [][4]float64{
		{9.5, 10.0, 9.4, 9.8},
		{10.6, 11.2, 10.5, 11.0},
		{10.9, 11.1, 10.7, 10.8},
	}
	bars := deepBars(rows)
	ind := &IndicatorResult{
		MA:   map[string]float64{},
		BOLL: map[string]float64{},
		GAP: &GapAnalysis{Recent: []GapInfo{{
			Direction: "up", GapLow: 10.0, GapHigh: 10.5, Filled: false,
		}}},
	}
	levels := computeKeyPriceLevels(bars, ind)
	found := false
	for _, s := range levels.Support {
		if s.Price == 10.5 {
			found = true
		}
	}
	if !found {
		t.Errorf("expect gap-high 10.5 in supports, got %+v", levels.Support)
	}
	if len(levels.BuyPoints) == 0 {
		t.Error("expect gap-retest buy point")
	}
}

func TestDedupeLevelsMerge(t *testing.T) {
	levels := []PriceLevel{
		{Price: 10.0, Label: "20日高点"},
		{Price: 10.05, Label: "布林上轨"},
		{Price: 11.0, Label: "60日高点"},
	}
	merged := dedupeLevels(levels, 9.5, false, 3)
	if len(merged) != 2 {
		t.Fatalf("expect 2 merged levels, got %d", len(merged))
	}
	if merged[0].Price != 10.0 || !containsStr(merged[0].Label, "20日高点") || !containsStr(merged[0].Label, "布林上轨") {
		t.Errorf("merge result wrong: %+v", merged[0])
	}
}

func TestConsecutiveFlowDays(t *testing.T) {
	rows := []models.StockMoneyDataHis{
		{F62: "1000000"}, {F62: "2000000"}, {F62: "-500000"}, {F62: "300000"},
	}
	days, inflow := consecutiveFlowDays(rows)
	if days != 2 || !inflow {
		t.Errorf("expect 2 inflow days, got %d inflow=%v", days, inflow)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || strings.Contains(s, sub))
}
