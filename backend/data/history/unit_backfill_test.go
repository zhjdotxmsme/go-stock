package history

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// seedUnitBar 构造一根指定来源/单位/量额的日 K（单位回填测试用）。
func seedUnitBar(t *testing.T, code, source, volUnit string, volume int64, amount, close float64, tradeDate string) {
	t.Helper()
	bar := models.KLineBar{
		StockCode: code, Period: "day", TradeDate: tradeDate, Adjusted: true,
		Open: close, High: close, Low: close, Close: close, PrevClose: close,
		Volume: volume, VolumeUnit: volUnit, Amount: amount, AmountUnit: "", Source: source,
	}
	if err := db.Dao.Create(&bar).Error; err != nil {
		t.Fatal(err)
	}
}

// 恒等式带内（amount≈volume×close）的空单位行补 share/CNY；
// 带外（前复权因子漂移、量纲不明）保持空单位 fail-closed；
// 已确认来源量>0 额=0 的行只补量单位。
func TestBackfillKLineUnits(t *testing.T) {
	setupSyncTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	seedUnitBar(t, "sh600519", "seed", "", 3000000, 6e9, 1500, "2026-10-09")        // ratio≈1 → share/CNY
	seedUnitBar(t, "sz000001", "seed", "", 5000000, 4e7, 10, "2026-10-09")          // ratio≈0.8 → share/CNY
	seedUnitBar(t, "sh600000", "stockdb", "", 8000000, 6e7, 10, "2026-10-09")       // ratio≈0.75 → share/CNY
	seedUnitBar(t, "sz000002", "seed", "", 7000000, 7e10, 10, "2026-10-09")          // ratio≈100（手量纲误标）→ 不回填
	seedUnitBar(t, "sz000003", "mootdx_kline", "", 6000000, 0, 10, "2026-10-09")    // 额=0 → 只补量单位
	seedUnitBar(t, "sh600004", "tencent_kline", "", 9000000, 9e7, 10, "2026-10-09") // 未确认来源 ratio≈1 → share/CNY

	n, err := BackfillKLineUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("回填行数=%d, want=5（4 行恒等式 + 1 行已确认来源仅量）", n)
	}
	get := func(code string) models.KLineBar {
		t.Helper()
		var b models.KLineBar
		if err := db.Dao.First(&b, "stock_code = ?", code).Error; err != nil {
			t.Fatal(err)
		}
		return b
	}
	if b := get("sh600519"); b.VolumeUnit != "share" || b.AmountUnit != "CNY" {
		t.Fatalf("恒等式行应补 share/CNY: %+v", b)
	}
	if b := get("sz000002"); b.VolumeUnit != "" || b.AmountUnit != "" {
		t.Fatalf("带外行必须保持空单位 fail-closed: %+v", b)
	}
	if b := get("sz000003"); b.VolumeUnit != "share" || b.AmountUnit != "" {
		t.Fatalf("已确认来源额=0 行应只补量单位: %+v", b)
	}
	if b := get("sh600004"); b.VolumeUnit != "share" || b.AmountUnit != "CNY" {
		t.Fatalf("tencent 恒等式行应补 share/CNY: %+v", b)
	}
	// 幂等：重跑 0 行
	if n, err := BackfillKLineUnits(context.Background()); err != nil || n != 0 {
		t.Fatalf("重跑应幂等 0 行: n=%d err=%v", n, err)
	}
}

// 近 30 天存在零量额 sync 行的 done 任务：打回 pending，断点回拨到最早污染日前一天；
// 干净任务不受影响。
func TestRequeuePoisonedSyncTasks(t *testing.T) {
	setupSyncTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}, &models.KLineSyncLog{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	seedUnitBar(t, "sh600519", "sync", "unknown", 0, 0, 1500, "2026-11-10")
	seedUnitBar(t, "sh600519", "sync", "unknown", 0, 0, 1500, "2026-11-11")
	seedUnitBar(t, "sh600519", "mootdx_kline", "share", 3000000, 6e9, 1500, "2026-11-12")
	if err := db.Dao.Create(&models.KLineSyncLog{
		StockCode: "sh600519", Period: "day", Adjusted: true,
		StartDate: "2025-01-01", EndDate: today, LastSyncDate: today,
		Status: SyncStatusDone, UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	// 干净任务：只有健康 mootdx 行
	seedUnitBar(t, "sz000001", "mootdx_kline", "share", 5000000, 4e7, 10, "2026-11-12")
	if err := db.Dao.Create(&models.KLineSyncLog{
		StockCode: "sz000001", Period: "day", Adjusted: true,
		StartDate: "2025-01-01", EndDate: today, LastSyncDate: today,
		Status: SyncStatusDone, UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	n, err := RequeuePoisonedSyncTasks(ctx, "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应重排 1 条污染任务: n=%d", n)
	}
	var poisoned models.KLineSyncLog
	if err := db.Dao.First(&poisoned, "stock_code = ?", "sh600519").Error; err != nil {
		t.Fatal(err)
	}
	if poisoned.Status != SyncStatusPending {
		t.Fatalf("污染任务应转 pending: %+v", poisoned)
	}
	// 最早污染日 2026-11-10 → 断点回拨 2026-11-09
	if poisoned.LastSyncDate != "2026-11-09" {
		t.Fatalf("断点应回拨到污染日前一天: %s", poisoned.LastSyncDate)
	}
	var clean models.KLineSyncLog
	if err := db.Dao.First(&clean, "stock_code = ?", "sz000001").Error; err != nil {
		t.Fatal(err)
	}
	if clean.Status != SyncStatusDone || clean.LastSyncDate != today {
		t.Fatalf("干净任务不得被动: %+v", clean)
	}
}

// 降级块（量或额为 0 的 bar）必须给出拒绝原因；健康块放行。
func TestDegradedChunkReason(t *testing.T) {
	ok := []models.KLineBar{
		{TradeDate: "2026-11-18", Volume: 100, Amount: 1000},
		{TradeDate: "2026-11-19", Volume: 200, Amount: 2000},
	}
	if reason := degradedChunkReason(ok); reason != "" {
		t.Fatalf("健康块不应拒绝: %s", reason)
	}
	bad := []models.KLineBar{
		{TradeDate: "2026-11-18", Volume: 100, Amount: 1000},
		{TradeDate: "2026-11-19", Volume: 0, Amount: 0},
	}
	if reason := degradedChunkReason(bad); !strings.Contains(reason, "2026-11-19") {
		t.Fatalf("降级块应指出问题 bar: %s", reason)
	}
}
