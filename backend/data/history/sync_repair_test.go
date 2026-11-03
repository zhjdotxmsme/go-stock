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
		LastSyncDate: "2026-10-29",
		Status:       SyncStatusDone, UpdatedAt: time.Now(),
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
	if fixed.LastSyncDate != "" {
		t.Fatalf("假 done 的尾部断点必须清掉，实际 %q", fixed.LastSyncDate)
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

func TestRequeueRetryableFailedSkipsConfirmedEmpty(t *testing.T) {
	setupSyncTestDB(t)
	ctx := context.Background()
	db.Dao.Create(&models.KLineSyncLog{
		StockCode: "sz000001", Period: "day", Adjusted: true,
		Status: SyncStatusFailed, ErrorMsg: "区间未取到任何 K 线（源数据暂缺或长期停牌）",
		UpdatedAt: time.Now(),
	})
	db.Dao.Create(&models.KLineSyncLog{
		StockCode: "sz000002", Period: "day", Adjusted: true,
		Status: SyncStatusFailed, ErrorMsg: "dial tcp: i/o timeout",
		UpdatedAt: time.Now(),
	})

	n, err := requeueRetryableFailed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应只重排网络失败，实际 %d", n)
	}
	var kept models.KLineSyncLog
	if err := db.Dao.Where("stock_code = ?", "sz000001").First(&kept).Error; err != nil {
		t.Fatal(err)
	}
	if kept.Status != SyncStatusFailed {
		t.Fatalf("源确认无数据应保持 failed，实际 %s", kept.Status)
	}
	var retried models.KLineSyncLog
	if err := db.Dao.Where("stock_code = ?", "sz000002").First(&retried).Error; err != nil {
		t.Fatal(err)
	}
	if retried.Status != SyncStatusPending {
		t.Fatalf("网络失败应回到 pending，实际 %s", retried.Status)
	}
}
