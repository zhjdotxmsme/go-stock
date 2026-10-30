package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func setupSyncTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = d
	t.Cleanup(func() {
		if sqlDB, err := d.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db.Dao = old
	})
	_ = db.Dao.AutoMigrate(&models.KLineSyncLog{})
}

func TestCreateSyncTask_SkipsUpToDateDone(t *testing.T) {
	setupSyncTestDB(t)
	ctx := context.Background()
	_ = db.Dao.Create(&models.KLineSyncLog{
		StockCode: "600519.SH", Period: "day", Adjusted: true,
		StartDate: "2024-01-01", EndDate: "2026-10-20", Status: SyncStatusDone,
		UpdatedAt: time.Now(),
	}).Error
	if err := CreateSyncTask(ctx, "600519.SH", "day", "2024-01-01", "2026-10-20", true); err != nil {
		t.Fatal(err)
	}
	var log models.KLineSyncLog
	_ = db.Dao.First(&log, "stock_code = ?", "600519.SH").Error
	if log.Status != SyncStatusDone {
		t.Fatalf("expected still done, got %s", log.Status)
	}
}

func TestCreateSyncTask_ExtendsEndDateWhenDone(t *testing.T) {
	setupSyncTestDB(t)
	ctx := context.Background()
	// 种子用内部格式（CreateSyncTask 现按 Rule 3 入口规范化，Tushare 格式入参
	// 会收敛为 sz000001 后再查找/更新既有任务）
	_ = db.Dao.Create(&models.KLineSyncLog{
		StockCode: "sz000001", Period: "day", Adjusted: true,
		StartDate: "2024-01-01", EndDate: "2026-10-01", Status: SyncStatusDone,
		UpdatedAt: time.Now(),
	}).Error
	if err := CreateSyncTask(ctx, "000001.SZ", "day", "2024-01-01", "2026-10-20", true); err != nil {
		t.Fatal(err)
	}
	var logs []models.KLineSyncLog
	_ = db.Dao.Find(&logs, "stock_code IN ?", []string{"sz000001", "000001.SZ"}).Error
	if len(logs) != 1 {
		t.Fatalf("任务应恰有 1 条（格式收敛），实际 %d 条", len(logs))
	}
	log := logs[0]
	if log.Status != SyncStatusPending || log.EndDate != "2026-10-20" {
		t.Fatalf("want pending end 2026-10-20, got %s %s", log.Status, log.EndDate)
	}
	if log.StockCode != "sz000001" {
		t.Fatalf("code should be normalized, got %s", log.StockCode)
	}
}
