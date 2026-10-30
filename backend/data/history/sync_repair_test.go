package history

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
)

// 回归：历史假完成修复——syncDateRange 旧缺陷只落尾部 55/1310 根却标 done，
// RepairBrokenSyncTasks 应重置此类任务并把混格式代码收敛为内部格式。
func TestRepairBrokenSyncTasks(t *testing.T) {
	setupSyncTestDB(t)
	ctx := context.Background()

	// 假 done：expected 1310 但只同步 55（旧缺陷实测值），代码为 Tushare 格式
	db.Dao.Create(&models.KLineSyncLog{
		StockCode: "300780.SZ", Period: "day", Adjusted: true,
		StartDate: "2021-10-20", EndDate: "2026-10-29",
		SyncedCount: 55, ExpectedCount: 1310,
		Status: SyncStatusDone, UpdatedAt: time.Now(),
	})
	// 真 done：同步条数达标，不应被动
	db.Dao.Create(&models.KLineSyncLog{
		StockCode: "sz600519", Period: "day", Adjusted: true,
		StartDate: "2021-10-20", EndDate: "2026-10-29",
		SyncedCount: 1300, ExpectedCount: 1310,
		Status: SyncStatusDone, UpdatedAt: time.Now(),
	})

	n, err := RepairBrokenSyncTasks(ctx)
	if err != nil {
		t.Fatalf("RepairBrokenSyncTasks: %v", err)
	}
	if n == 0 {
		t.Fatal("修复数应为正")
	}

	// 假 done 行：代码收敛为内部格式并重置为 pending
	var fixed models.KLineSyncLog
	if err := db.Dao.Where("period = ? AND adjusted = ?", "day", true).
		Where("stock_code IN ?", []string{"sz300780", "300780.SZ"}).
		First(&fixed).Error; err != nil {
		t.Fatalf("修复后的任务不存在: %v", err)
	}
	if fixed.StockCode != "sz300780" {
		t.Fatalf("代码应收敛为内部格式，实际 %q", fixed.StockCode)
	}
	if fixed.Status != SyncStatusPending {
		t.Fatalf("假 done 应重置为 pending，实际 %q", fixed.Status)
	}

	// 真 done 行：保持 done
	var good models.KLineSyncLog
	if err := db.Dao.Where("stock_code = ?", "sz600519").First(&good).Error; err != nil {
		t.Fatalf("真 done 任务丢失: %v", err)
	}
	if good.Status != SyncStatusDone {
		t.Fatalf("达标任务不应被重置，实际 %q", good.Status)
	}
}

// CreateSyncTask 入口应把 Tushare/裸格式统一为内部格式（Rule 3）
func TestCreateSyncTaskNormalizesCode(t *testing.T) {
	setupSyncTestDB(t)
	ctx := context.Background()
	if err := CreateSyncTask(ctx, "300780.SZ", "day", "2024-01-01", "2026-10-29", true); err != nil {
		t.Fatalf("CreateSyncTask: %v", err)
	}
	var log models.KLineSyncLog
	if err := db.Dao.Where("period = ? AND adjusted = ?", "day", true).First(&log).Error; err != nil {
		t.Fatalf("task not found: %v", err)
	}
	if want := stockcode.Normalize("300780.SZ"); log.StockCode != want {
		t.Fatalf("stock_code = %q, want %q", log.StockCode, want)
	}
}
