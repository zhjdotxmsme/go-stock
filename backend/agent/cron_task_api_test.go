package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Task 11：cron 层暴露日K同步就绪状态（kline_sync 任务的真实完成信号），
// 供狩猎场确认前置条件与前端诊断使用；15:30 只是触发时间，不作就绪证据。

func setupSyncReadinessTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Dao = d
	t.Cleanup(func() {
		if sqlDB, err := d.DB(); err == nil {
			sqlDB.Close()
		}
		db.Dao = old
	})
	if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func createSyncLog(t *testing.T, code, status, lastSync string) {
	t.Helper()
	row := models.KLineSyncLog{
		StockCode: code, Period: "day", Adjusted: true,
		StartDate: "2025-01-01", EndDate: "2026-11-18",
		Status: status, LastSyncDate: lastSync, UpdatedAt: time.Now(),
	}
	if err := db.Dao.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCronTaskApiDailyKLineSyncReadiness(t *testing.T) {
	setupSyncReadinessTestDB(t)
	ctx := context.Background()
	tradeDate := "2026-11-18"

	// 无同步记录：不得就绪（不能以触发时点代替完成信号）
	r := NewCronTaskApi().GetDailyKLineSyncReadiness(ctx, tradeDate)
	if r.Completed {
		t.Fatalf("无同步记录不得就绪: %+v", r)
	}
	if r.TradeDate != tradeDate {
		t.Fatalf("交易日不符: %+v", r)
	}

	// 同步完成且断点覆盖：就绪
	createSyncLog(t, "sh600000", "done", tradeDate)
	createSyncLog(t, "sz000001", "done", tradeDate)
	if err := db.Dao.Create(&models.KLineBar{
		StockCode: "sh600000", Period: "day", TradeDate: tradeDate, Adjusted: false,
		Open: 10, High: 10, Low: 10, Close: 10, PrevClose: 10,
		Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
	}).Error; err != nil {
		t.Fatal(err)
	}
	r = NewCronTaskApi().GetDailyKLineSyncReadiness(ctx, tradeDate)
	if !r.Completed {
		t.Fatalf("同步完成应就绪: %+v", r)
	}

	// running 任务出现：立即不齐，且诊断带计数
	createSyncLog(t, "sz000002", "running", tradeDate)
	r = NewCronTaskApi().GetDailyKLineSyncReadiness(ctx, tradeDate)
	if r.Completed || r.Running != 1 {
		t.Fatalf("running 任务应阻断就绪: %+v", r)
	}
	if !strings.Contains(r.Reason, "running=1") {
		t.Fatalf("诊断缺少 running 计数: %s", r.Reason)
	}
}

// 任务类型必须注册到分发器：ctx 已取消时应命中 executeKhunterReview 的守卫并返回 ctx.Err()，
// 而不是落入 default 的"未知任务类型"。
func TestExecuteTaskByTypeKhunterReviewDispatch(t *testing.T) {
	a := NewCronTaskApi()
	task := &models.CronTask{Name: "狩猎场复盘回填", TaskType: "khunter_review"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := a.executeTaskByType(ctx, task)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// 新任务类型必须出现在类型清单（前端下拉与任务创建依赖）。
func TestGetTaskTypesIncludesKhunterReview(t *testing.T) {
	for _, tt := range NewCronTaskApi().GetTaskTypes() {
		if tt.A == "khunter_review" {
			return
		}
	}
	t.Fatal("khunter_review missing from GetTaskTypes")
}
