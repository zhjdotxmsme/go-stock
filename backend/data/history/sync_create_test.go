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
	_ = db.Dao.Create(&models.KLineSyncLog{
		StockCode: "000001.SZ", Period: "day", Adjusted: true,
		StartDate: "2024-01-01", EndDate: "2026-10-01", Status: SyncStatusDone,
		UpdatedAt: time.Now(),
	}).Error
	if err := CreateSyncTask(ctx, "000001.SZ", "day", "2024-01-01", "2026-10-20", true); err != nil {
		t.Fatal(err)
	}
	var log models.KLineSyncLog
	_ = db.Dao.First(&log, "stock_code = ?", "000001.SZ").Error
	if log.Status != SyncStatusPending || log.EndDate != "2026-10-20" {
		t.Fatalf("want pending end 2026-10-20, got %s %s", log.Status, log.EndDate)
	}
}
