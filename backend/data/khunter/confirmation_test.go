package khunter

import (
	"context"
	"math"
	"testing"
	"time"

	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/data/trading"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestConfirmationPriceAndGap(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	allow := RiskDecision{AllowConfirm: true}
	limit, _ := trading.DailyPriceLimit("600000", 90, false)
	tests := []struct {
		name string
		bar  models.KLineBar
		entry float64
		confirmed, skip bool
	}{
		{"开盘位于H与H+2%之间", models.KLineBar{Open: 101, High: 102, Low: 100, Close: 101}, 101, true, false},
		{"开盘低于H且盘中触及", models.KLineBar{Open: 99, High: 100, Low: 98, Close: 99.5}, 100, true, false},
		{"开盘高于H+2%", models.KLineBar{Open: 102.01, High: 103, Low: 101, Close: 102}, 0, false, true},
		{"未触及H", models.KLineBar{Open: 99, High: 99.9, Low: 98, Close: 99}, 0, false, false},
	}
	for _, tt := range tests {
		d := EvaluateConfirmationDay(100, 95, 2, tt.bar, limit, allow, cfg)
		if d.Confirmed != tt.confirmed || d.SkipDay != tt.skip || math.Abs(d.EntryPrice-tt.entry) > 1e-9 {
			t.Fatalf("%s: %+v", tt.name, d)
		}
	}
}

func TestConfirmationWindowAndRawPrice(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	days := []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-16"}
	if got := ConfirmationWindowState(days[0], days[3], days, cfg); got != "open" {
		t.Fatalf("T+3 应可确认: %s", got)
	}
	if got := ConfirmationWindowState(days[0], days[4], days, cfg); got != "expired" {
		t.Fatalf("T+4 应过期: %s", got)
	}
	raw, err := RawSignalLevel(120, 100, 90)
	if err != nil || raw != 108 {
		t.Fatalf("不复权信号价转换错误: raw=%v err=%v", raw, err)
	}
}

func TestInitialStopAndRiskBudget(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	d := EvaluateConfirmationDay(100, 96, 3,
		models.KLineBar{Open: 99, High: 101, Low: 98, Close: 100},
		trading.PriceLimit{UpPrice: 110, DownPrice: 90}, RiskDecision{AllowConfirm: true}, cfg)
	if !d.Confirmed || d.StopPrice != 96 || d.InitialRisk != 4 {
		t.Fatalf("初始失效价错误: %+v", d)
	}
	if shares := RiskBudgetShares(1e6, 100, 96, 1, 0, cfg); shares != 1200 {
		t.Fatalf("风险预算股数=%d", shares)
	}
}

func TestConfirmWindowStateEdgeCases(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	days := []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-16"}
	if got := ConfirmationWindowState(days[0], days[0], days, cfg); got != "signal_day" {
		t.Fatalf("signal_day expected, got %s", got)
	}
	if got := ConfirmationWindowState(days[0], "2026-11-09", days, cfg); got != "invalid" {
		t.Fatalf("invalid expected, got %s", got)
	}
	if got := ConfirmationWindowState("2026-11-09", days[0], days, cfg); got != "invalid" {
		t.Fatalf("invalid expected when signal not in tradeDates, got %s", got)
	}
}

func TestRawSignalLevelInvalidInputs(t *testing.T) {
	if _, err := RawSignalLevel(0, 100, 90); err == nil {
		t.Fatal("expected error for zero level")
	}
	if _, err := RawSignalLevel(100, 0, 90); err == nil {
		t.Fatal("expected error for zero adjustedClose")
	}
	if _, err := RawSignalLevel(100, 100, 0); err == nil {
		t.Fatal("expected error for zero rawClose")
	}
}

func TestRiskBudgetSharesRespectsPositionLimitAndLotSize(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	// risk budget: 1e6 * 0.005 / 4 = 1250 -> round to 1200
	if got := RiskBudgetShares(1e6, 100, 96, 1, 0, cfg); got != 1200 {
		t.Fatalf("expected 1200, got %d", got)
	}
	// position limit caps exposure: 1e6*0.1 - 0 = 100k -> 1000 shares, risk budget 1250 -> constrained to 1000
	if got := RiskBudgetShares(1e6, 100, 96, 0.1, 0, cfg); got != 1000 {
		t.Fatalf("expected 1000, got %d", got)
	}
	// used exposure eats into position limit
	if got := RiskBudgetShares(1e6, 100, 96, 0.1, 50000, cfg); got != 500 {
		t.Fatalf("expected 500, got %d", got)
	}
	// negative available exposure -> 0
	if got := RiskBudgetShares(1e6, 100, 96, 0.1, 200000, cfg); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestConfirmEngineIdempotent(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}, &models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&models.AllStockInfo{
		SECUCODE: "600000.SH", SECURITYCODE: "600000", SECURITYNAMEABBR: "平稳股",
	}).Error; err != nil {
		t.Fatal(err)
	}

	seedBar := func(date string, adjClose, rawClose float64) {
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: date, Adjusted: true,
			Open: adjClose, High: adjClose + 1, Low: adjClose - 1, Close: adjClose,
			PrevClose: adjClose - 1,
			Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: date, Adjusted: false,
			Open: rawClose, High: rawClose + 1, Low: rawClose - 1, Close: rawClose,
			PrevClose: rawClose - 1,
			Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 10, 10, 0, 0, 0, 0, time.Local)
	for i := 0; i < 35; i++ {
		date := base.AddDate(0, 0, i).Format("2006-01-02")
		seedBar(date, 100, 90)
	}

	r := NewRepo()
	obs := models.KhunterObservation{
		Code: "600000", Name: "平稳股", SignalDate: "2026-11-09", Status: ObsStatusPending,
		DataStatus: "valid", TriggerPrice: 100, SignalLow: 95, ATR14: 2, Score: 80,
	}
	if err := r.UpsertObservation(&obs); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultRiskControlConfig()
	ctx := context.Background()
	tradeDates := []string{"2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}
	engine := NewConfirmationEngine(r)
	engine.Readiness = alwaysReadyChecker{}
	marketRisk := risk.Level{Name: "正常", PositionLimit: 1, ScoreExtra: 0}
	if _, err := engine.ConfirmForDate(ctx, "2026-11-10", tradeDates, marketRisk, cfg, cfg.InitialCash); err != nil {
		t.Fatal(err)
	}

	var firstCount int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&firstCount).Error; err != nil {
		t.Fatal(err)
	}
	if firstCount != 1 {
		t.Fatalf("应确认生成一笔交易，got %d", firstCount)
	}

	// 重复运行同一日不得产生第二笔交易
	if _, err := engine.ConfirmForDate(ctx, "2026-11-10", tradeDates, marketRisk, cfg, cfg.InitialCash); err != nil {
		t.Fatal(err)
	}
	var secondCount int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&secondCount).Error; err != nil {
		t.Fatal(err)
	}
	if secondCount != 1 {
		t.Fatalf("重复确认不得生成第二笔交易，got %d", secondCount)
	}
}

type alwaysReadyChecker struct{}

func (alwaysReadyChecker) Ready(context.Context, string) bool { return true }
