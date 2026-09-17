package data

import (
	"path/filepath"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// TestSaveHoldingsDailySummary_UpsertByDate 同一天重新生成覆盖原记录，不同日期新增。
func TestSaveHoldingsDailySummary_UpsertByDate(t *testing.T) {
	db.Init(filepath.Join(t.TempDir(), "test.db"))
	t.Cleanup(func() {
		if sqlDB, err := db.Dao.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	db.AutoMigrate()

	first := &models.HoldingsDailySummary{
		SummaryDate: "2026-09-17",
		Content:     "第一版总结",
		ModelName:   "test-model",
		StockCount:  2,
		TotalProfit: 100,
		ProfitRate:  1.5,
	}
	if err := SaveHoldingsDailySummary(first); err != nil {
		t.Fatalf("first save error = %v", err)
	}
	if first.ID == 0 {
		t.Fatal("first save ID = 0")
	}

	// 同日覆盖：ID 不变，内容更新
	second := &models.HoldingsDailySummary{
		SummaryDate: "2026-09-17",
		Content:     "重新生成的总结",
		ModelName:   "test-model-2",
		StockCount:  3,
		TotalProfit: 200,
		ProfitRate:  2.5,
	}
	if err := SaveHoldingsDailySummary(second); err != nil {
		t.Fatalf("second save error = %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("upsert created new row: id = %d, want %d", second.ID, first.ID)
	}

	got, err := GetHoldingsSummaryByDate("2026-09-17")
	if err != nil {
		t.Fatalf("GetHoldingsSummaryByDate error = %v", err)
	}
	if got == nil || got.Content != "重新生成的总结" || got.StockCount != 3 {
		t.Fatalf("got = %+v", got)
	}

	// 不同日期新增
	third := &models.HoldingsDailySummary{SummaryDate: "2026-09-16", Content: "昨日总结"}
	if err := SaveHoldingsDailySummary(third); err != nil {
		t.Fatalf("third save error = %v", err)
	}
	page, err := GetHoldingsSummaryList(1, 20)
	if err != nil {
		t.Fatalf("GetHoldingsSummaryList error = %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("list total = %d, want 2", page.Total)
	}
	// 按日期倒序：最新的在前，且 Content 被截断为预览
	if len(page.List) != 2 || page.List[0].SummaryDate != "2026-09-17" {
		t.Fatalf("list order = %+v", page.List)
	}
	if page.List[0].Content != "重新生成的总结" {
		t.Fatalf("list content preview = %q", page.List[0].Content)
	}

	detail, err := GetHoldingsSummaryDetail(first.ID)
	if err != nil {
		t.Fatalf("GetHoldingsSummaryDetail error = %v", err)
	}
	if detail.Content != "重新生成的总结" {
		t.Fatalf("detail content = %q", detail.Content)
	}
}

// TestTruncateRunes 中文按字截断并追加省略号。
func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("持仓总结内容", 10); got != "持仓总结内容" {
		t.Fatalf("short text truncated: %q", got)
	}
	got := truncateRunes("这是一段很长的总结内容需要被截断处理", 5)
	if got != "这是一段很…" {
		t.Fatalf("truncate = %q", got)
	}
}
