package khunter

import (
	"testing"

	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/models"
)

func TestSupportPrice(t *testing.T) {
	bars := make([]models.KLineBar, 30)
	for i := range bars {
		bars[i] = models.KLineBar{Close: 10, Low: 9.5, TradeDate: "2026-09-01"}
	}
	// 全部收盘 10 → 支撑位 10
	if got := SupportPrice(bars); got != 10 {
		t.Fatalf("expect 10, got %v", got)
	}
	// 近 10 根涨到 20+，支撑位仍是窗口内最低收盘 10
	for i := 20; i < 30; i++ {
		bars[i].Close = 20 + float64(i)
		bars[i].Low = bars[i].Close - 0.5
	}
	if got := SupportPrice(bars); got != 10 {
		t.Fatalf("expect 10, got %v", got)
	}
	// 不足 20 根：取全部
	bars2 := make([]models.KLineBar, 5)
	for i := range bars2 {
		bars2[i].Close = 5 + float64(i)
	}
	if got := SupportPrice(bars2); got != 5 {
		t.Fatalf("expect 5, got %v", got)
	}
}

func TestHuntingThreshold(t *testing.T) {
	// 风险"危险"档 scoreExtra=15 → 阈值 60+15=75
	if got := HuntingThreshold(risk.Level{Name: "危险", PositionLimit: 0.4, ScoreExtra: 15}); got != 75 {
		t.Fatalf("expect 75, got %v", got)
	}
	if got := HuntingThreshold(risk.Level{Name: "正常", PositionLimit: 1.0, ScoreExtra: 0}); got != 60 {
		t.Fatalf("expect 60, got %v", got)
	}
}
