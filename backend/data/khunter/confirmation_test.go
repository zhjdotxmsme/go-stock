package khunter

import (
	"context"
	"math"
	"strings"
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
		name            string
		bar             models.KLineBar
		entry           float64
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
			Volume:    1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: date, Adjusted: false,
			Open: rawClose, High: rawClose + 1, Low: rawClose - 1, Close: rawClose,
			PrevClose: rawClose - 1,
			Volume:    1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
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

// seedConfirmScenarioBars 为确认链路测试播种 sh600000 的复权+不复权日K（35 天）。
// 复权口径恒为平价 100；不复权基准 bar 可按日期用 rawOverride 替换
// （如 PrevClose=0、一字涨停日）。
func seedConfirmScenarioBars(t *testing.T, baseRaw models.KLineBar, rawOverride map[string]models.KLineBar) {
	t.Helper()
	baseDate := time.Date(2026, 10, 10, 0, 0, 0, 0, time.Local)
	for i := 0; i < 35; i++ {
		date := baseDate.AddDate(0, 0, i).Format("2006-01-02")
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: date, Adjusted: true,
			Open: 100, High: 101, Low: 99, Close: 100, PrevClose: 99,
			Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
		raw := baseRaw
		raw.StockCode = "sh600000"
		raw.Period = "day"
		raw.TradeDate = date
		if o, ok := rawOverride[date]; ok {
			raw = o
			raw.StockCode = "sh600000"
			raw.Period = "day"
			raw.TradeDate = date
		}
		if err := db.Dao.Create(&raw).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func runConfirmOnDate(t *testing.T, engine *ConfirmationEngine, tradeDate string, tradeDates []string) (ConfirmStats, error) {
	t.Helper()
	return engine.ConfirmForDate(context.Background(), tradeDate, tradeDates,
		risk.Level{Name: "正常", PositionLimit: 1, ScoreExtra: 0},
		DefaultRiskControlConfig(), DefaultRiskControlConfig().InitialCash)
}

// 到期必须持久化终态：status=已过期、confirm_expire 落库；重跑同日不得重复计入
// ExpiredCount，也不得报错（回归：UpsertObservation 不写 status，观察永远待确认）。
func TestConfirmForDatePersistsExpiryOnce(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	obs := models.KhunterObservation{
		Code: "600000", Name: "过期股", SignalDate: "2026-11-09", Status: ObsStatusPending,
		DataStatus: "valid", TriggerPrice: 100, SignalLow: 95, ATR14: 2, Score: 80,
	}
	if err := r.UpsertObservation(&obs); err != nil {
		t.Fatal(err)
	}

	engine := NewConfirmationEngine(r)
	engine.Readiness = alwaysReadyChecker{}
	tradeDates := []string{"2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-20"}
	// 2026-11-20 相对信号日为 T+5，超出 T+1~T+3 确认窗口：应判过期并持久化
	stats, err := runConfirmOnDate(t, engine, "2026-11-20", tradeDates)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Expired != 1 {
		t.Fatalf("首次运行应恰好过期 1 条，got %d", stats.Expired)
	}
	var got models.KhunterObservation
	if err := db.Dao.First(&got, obs.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != ObsStatusExpired {
		t.Fatalf("到期状态必须持久化为已过期，got %s", got.Status)
	}
	if got.ConfirmExpire != "2026-11-20" {
		t.Fatalf("confirm_expire 必须随过期落库，got %q", got.ConfirmExpire)
	}
	if !strings.Contains(got.RejectReason, "确认窗口已过期") {
		t.Fatalf("过期必须留拒绝原因，got %q", got.RejectReason)
	}

	// 重跑同日：观察已退出待确认列表，不得再次计入过期
	stats2, err := runConfirmOnDate(t, engine, "2026-11-20", tradeDates)
	if err != nil {
		t.Fatal(err)
	}
	if stats2.Expired != 0 {
		t.Fatalf("重跑不得重复计数过期，got %d", stats2.Expired)
	}
	var again models.KhunterObservation
	if err := db.Dao.First(&again, obs.ID).Error; err != nil {
		t.Fatal(err)
	}
	if again.Status != ObsStatusExpired || again.ConfirmExpire != "2026-11-20" {
		t.Fatalf("重跑不得改变终态: %+v", again)
	}
}

// 确认日 bar 缺前收（PrevClose=0）必须拒绝确认并留痕，不得用 close×1.1/0.9
// 合成涨跌停带近似（回归：合成带在数学上关闭一字涨停判定，且 10% 对 20%/30% 板错误）。
func TestConfirmForDateRejectsMissingPrevClose(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	flat := models.KLineBar{
		Open: 90, High: 91, Low: 89, Close: 90, PrevClose: 89,
		Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
	}
	seedConfirmScenarioBars(t, flat, map[string]models.KLineBar{
		"2026-11-10": {Open: 90, High: 91, Low: 89, Close: 90, PrevClose: 0,
			Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY"},
	})
	r := NewRepo()
	if err := r.UpsertObservation(&models.KhunterObservation{
		Code: "600000", Name: "平稳股", SignalDate: "2026-11-09", Status: ObsStatusPending,
		DataStatus: "valid", TriggerPrice: 100, SignalLow: 95, ATR14: 2, Score: 80,
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewConfirmationEngine(r)
	engine.Readiness = alwaysReadyChecker{}
	tradeDates := []string{"2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}
	stats, err := runConfirmOnDate(t, engine, "2026-11-10", tradeDates)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Confirmed != 0 || stats.Rejected != 1 {
		t.Fatalf("缺前收必须拒绝确认: %+v", stats)
	}
	var n int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("缺前收不得生成交易，got %d", n)
	}
	var obs models.KhunterObservation
	if err := db.Dao.Where("code=? AND signal_date=?", "600000", "2026-11-09").First(&obs).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(obs.RejectReason, "收盘价") || !strings.Contains(obs.RejectReason, "涨跌停") {
		t.Fatalf("拒绝原因必须说明缺前收/涨跌停: %q", obs.RejectReason)
	}
}

// 入场日一字涨停（真实前收推得涨停价）仍不可确认：OnePriceLimitUp 守卫必须生效。
func TestConfirmForDateOnePriceLimitUpCannotConfirm(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	flat := models.KLineBar{
		Open: 100, High: 101, Low: 99, Close: 100, PrevClose: 99,
		Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
	}
	// 确认日 OHLC 全为 110 = 前收 100 × 1.1（真实涨停价）：全天封死买不进
	seedConfirmScenarioBars(t, flat, map[string]models.KLineBar{
		"2026-11-10": {Open: 110, High: 110, Low: 110, Close: 110, PrevClose: 100,
			Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY"},
	})
	r := NewRepo()
	if err := r.UpsertObservation(&models.KhunterObservation{
		Code: "600000", Name: "平稳股", SignalDate: "2026-11-09", Status: ObsStatusPending,
		DataStatus: "valid", TriggerPrice: 100, SignalLow: 95, ATR14: 2, Score: 80,
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewConfirmationEngine(r)
	engine.Readiness = alwaysReadyChecker{}
	tradeDates := []string{"2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}
	stats, err := runConfirmOnDate(t, engine, "2026-11-10", tradeDates)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Confirmed != 0 {
		t.Fatalf("一字涨停不得确认: %+v", stats)
	}
	var n int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("一字涨停不得生成交易，got %d", n)
	}
	var obs models.KhunterObservation
	if err := db.Dao.Where("code=? AND signal_date=?", "600000", "2026-11-09").First(&obs).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(obs.RejectReason, "一字涨停") {
		t.Fatalf("拒绝原因必须点名一字涨停: %q", obs.RejectReason)
	}
}
