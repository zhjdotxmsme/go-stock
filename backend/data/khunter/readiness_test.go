package khunter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Task 11 同步完成前置条件：确认只允许在日K同步真正完成后运行。
// pending/running/failed 任务、LastSyncDate 断点与当日 K 线覆盖缺一不可；
// 15:30/17:00 只是 cron 触发时间，从不作为就绪证据。

func TestPipelineRejectsUnfinishedCurrentDaySync(t *testing.T) {
	readiness := DataReadiness{TradeDate: "2026-11-18", Completed: false, Reason: "存在 running 日K同步任务"}
	err := RequireConfirmationReadiness(readiness)
	if !errors.Is(err, ErrDataNotReady) {
		t.Fatalf("同步未完成仍允许确认: %v", err)
	}
}

func TestReadinessFailureHasDiagnosticReason(t *testing.T) {
	readiness := DataReadiness{TradeDate: "2026-11-18", Completed: false, Pending: 2, Running: 1, Reason: "pending=2 running=1"}
	err := RequireConfirmationReadiness(readiness)
	if err == nil || !strings.Contains(err.Error(), "pending=2") {
		t.Fatalf("错误缺少诊断信息: %v", err)
	}
}

func TestRequireConfirmationReadinessAllowsCompleted(t *testing.T) {
	if err := RequireConfirmationReadiness(DataReadiness{TradeDate: "2026-11-18", Completed: true}); err != nil {
		t.Fatalf("同步完成应允许确认: %v", err)
	}
}

func seedSyncLog(t *testing.T, code, status, lastSync string) {
	t.Helper()
	row := models.KLineSyncLog{
		StockCode: code, Period: "day", Adjusted: true,
		StartDate: "2025-01-01", EndDate: "2026-11-18",
		Status: status, LastSyncDate: lastSync,
	}
	if err := db.Dao.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
}

func seedCoverageBar(t *testing.T, code, date string) {
	t.Helper()
	if err := db.Dao.Create(&models.KLineBar{
		StockCode: code, Period: "day", TradeDate: date, Adjusted: false,
		Open: 10, High: 10, Low: 10, Close: 10, PrevClose: 10,
		Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestComputeDataReadinessFromSyncLogs(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Date(2026, 11, 18, 16, 0, 0, 0, time.Local)
	tradeDate := "2026-11-18"

	t.Run("无同步记录不齐", func(t *testing.T) {
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if r.Completed {
			t.Fatalf("无同步记录不得就绪: %+v", r)
		}
		if r.Reason == "" {
			t.Fatal("未就绪必须给出诊断原因")
		}
	})

	t.Run("running 任务阻断", func(t *testing.T) {
		setupTestDB(t)
		if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
			t.Fatal(err)
		}
		seedSyncLog(t, "sh600000", "done", tradeDate)
		seedCoverageBar(t, "sh600000", tradeDate)
		seedSyncLog(t, "sz000001", "running", tradeDate)
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if r.Completed || r.Running != 1 {
			t.Fatalf("running 任务应阻断就绪: %+v", r)
		}
		if err := RequireConfirmationReadiness(r); err == nil || !strings.Contains(err.Error(), "running=1") {
			t.Fatalf("诊断缺少 running 计数: %v", err)
		}
	})

	t.Run("failed 与 pending 任务阻断", func(t *testing.T) {
		setupTestDB(t)
		if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
			t.Fatal(err)
		}
		seedSyncLog(t, "sh600000", "done", tradeDate)
		seedCoverageBar(t, "sh600000", tradeDate)
		seedSyncLog(t, "sz000001", "failed", tradeDate)
		seedSyncLog(t, "sz000002", "pending", tradeDate)
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if r.Completed || r.Failed != 1 || r.Pending != 1 {
			t.Fatalf("failed/pending 任务应阻断就绪: %+v", r)
		}
	})

	t.Run("断点未覆盖目标日", func(t *testing.T) {
		setupTestDB(t)
		if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
			t.Fatal(err)
		}
		seedSyncLog(t, "sh600000", "done", "2026-11-17")
		seedCoverageBar(t, "sh600000", tradeDate)
		seedCoverageBar(t, "sh600000", "2026-11-17")
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if r.Completed {
			t.Fatalf("断点未覆盖不得就绪: %+v", r)
		}
		if !strings.Contains(r.Reason, "2026-11-17") {
			t.Fatalf("诊断应指出断点日期: %s", r.Reason)
		}
	})

	// 少数长期停牌/退市股票的陈旧断点（如 2026-06-24）不应阻断全市场确认：
	// 覆盖率 ≥ readinessMinCoverage 即视为就绪。
	t.Run("个别陈旧断点不阻断全市场", func(t *testing.T) {
		setupTestDB(t)
		if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 200; i++ {
			seedSyncLog(t, fmt.Sprintf("sh%06d", 600000+i), "done", tradeDate)
		}
		seedSyncLog(t, "sz000001", "done", "2026-06-24")
		seedCoverageBar(t, "sh600000", tradeDate)
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if !r.Completed {
			t.Fatalf("覆盖率 %.1f%% 达标应就绪: %+v", 200.0/201*100, r)
		}
	})

	// 覆盖率低于阈值（>1% 任务落后）仍视为未就绪，并给出覆盖率诊断。
	t.Run("覆盖率不足阻断", func(t *testing.T) {
		setupTestDB(t)
		if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			seedSyncLog(t, fmt.Sprintf("sh%06d", 600000+i), "done", tradeDate)
		}
		for i := 0; i < 5; i++ {
			seedSyncLog(t, fmt.Sprintf("sz%06d", 1+i), "done", "2026-11-10")
		}
		seedCoverageBar(t, "sh600000", tradeDate)
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if r.Completed {
			t.Fatalf("覆盖率 95%% 不足不得就绪: %+v", r)
		}
		if !strings.Contains(r.Reason, "覆盖率") || !strings.Contains(r.Reason, "95.2%") {
			t.Fatalf("诊断应包含覆盖率: %s", r.Reason)
		}
	})

	t.Run("同步完成且无未完成任务", func(t *testing.T) {
		setupTestDB(t)
		if err := db.Dao.AutoMigrate(&models.KLineSyncLog{}, &models.KLineBar{}); err != nil {
			t.Fatal(err)
		}
		seedSyncLog(t, "sh600000", "done", tradeDate)
		seedSyncLog(t, "sz000001", "done", tradeDate)
		seedCoverageBar(t, "sh600000", tradeDate)
		r := ComputeDataReadiness(ctx, tradeDate, now)
		if !r.Completed {
			t.Fatalf("同步完成应就绪: %+v", r)
		}
		if err := RequireConfirmationReadiness(r); err != nil {
			t.Fatalf("就绪后不得报错: %v", err)
		}
	})
}

// readinessPipelineSeed 构造可确认场景：平稳股 35 根日K + 一条 T 日待确认观察
// （信号日 2026-11-09，确认日 2026-11-10，触发价会被次日真实K线触及）。
func readinessPipelineSeed(t *testing.T) {
	t.Helper()
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}, &models.KLineBar{}, &models.KLineSyncLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&models.AllStockInfo{
		SECUCODE: "600000.SH", SECURITYCODE: "600000", SECURITYNAMEABBR: "平稳股",
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedBar := func(date string, adjClose, rawClose float64) {
		for _, b := range []models.KLineBar{
			{StockCode: "sh600000", Period: "day", TradeDate: date, Adjusted: true,
				Open: adjClose, High: adjClose + 1, Low: adjClose - 1, Close: adjClose, PrevClose: adjClose - 1,
				Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY"},
			{StockCode: "sh600000", Period: "day", TradeDate: date, Adjusted: false,
				Open: rawClose, High: rawClose + 1, Low: rawClose - 1, Close: rawClose, PrevClose: rawClose - 1,
				Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY"},
		} {
			if err := db.Dao.Create(&b).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	base := time.Date(2026, 10, 10, 0, 0, 0, 0, time.Local)
	for i := 0; i < 35; i++ {
		date := base.AddDate(0, 0, i).Format("2006-01-02")
		seedBar(date, 100, 90)
		// 沪深300交易日序列：流水线确认窗口与风险档都依赖它
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh000300", Period: "day", TradeDate: date, Adjusted: false,
			Open: 4000, High: 4000, Low: 4000, Close: 4000, PrevClose: 3999,
			Volume: 1000000, VolumeUnit: "share", Amount: 100000000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	obs := models.KhunterObservation{
		Code: "600000", Name: "平稳股", SignalDate: "2026-11-09", Status: ObsStatusPending,
		DataStatus: "valid", TriggerPrice: 100, SignalLow: 95, ATR14: 2, Score: 80,
	}
	if err := NewRepo().UpsertObservation(&obs); err != nil {
		t.Fatal(err)
	}
}

func TestLivePipelineConfirmsWhenSyncComplete(t *testing.T) {
	readinessPipelineSeed(t)
	// 同步日志：断点已覆盖确认日，无未完成任务 → 确认放行
	seedSyncLog(t, "sh600000", "done", "2026-11-10")

	res, err := RunPipeline(context.Background(), "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if res.Readiness == nil || !res.Readiness.Completed {
		t.Fatalf("就绪结论应随结果返回: %+v", res.Readiness)
	}
	var huntingCount int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&huntingCount).Error; err != nil {
		t.Fatal(err)
	}
	if huntingCount != 1 || res.ConfirmedCount != 1 || res.Hunted != 1 {
		t.Fatalf("同步完成应确认一笔交易: hunting=%d confirmed=%d hunted=%d", huntingCount, res.ConfirmedCount, res.Hunted)
	}
	summary, err := NewRepo().GetRunSummary("2026-11-10")
	if err != nil || summary == nil {
		t.Fatalf("运行汇总缺失: %v", err)
	}
	if summary.ConfirmedCount != 1 || summary.Status != "success" {
		t.Fatalf("运行汇总确认计数/status 不符: %+v", summary)
	}
}

func TestLivePipelineSkipsConfirmationWhenSyncIncomplete(t *testing.T) {
	readinessPipelineSeed(t)
	// 同步日志：断点覆盖但存在 running 任务 → 观察保留，确认必须跳过并给出诊断
	seedSyncLog(t, "sh600000", "done", "2026-11-10")
	seedSyncLog(t, "sz000001", "running", "2026-11-10")

	res, err := RunPipeline(context.Background(), "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if res.Readiness == nil || res.Readiness.Completed {
		t.Fatalf("未就绪结论应随结果返回: %+v", res.Readiness)
	}
	var huntingCount int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&huntingCount).Error; err != nil {
		t.Fatal(err)
	}
	if huntingCount != 0 || res.ConfirmedCount != 0 {
		t.Fatalf("同步未完成不得确认: hunting=%d confirmed=%d", huntingCount, res.ConfirmedCount)
	}
	summary, err := NewRepo().GetRunSummary("2026-11-10")
	if err != nil || summary == nil {
		t.Fatalf("运行汇总缺失: %v", err)
	}
	if !strings.Contains(summary.Message, ErrDataNotReady.Error()) {
		t.Fatalf("运行汇总应携带诊断错误: %s", summary.Message)
	}
	if summary.Status != "success" {
		t.Fatalf("观察照常保存，运行本身应成功: %s", summary.Status)
	}
	// 待确认观察不得被误迁移（仍观察中，等同步完成后下一交易日再确认）
	pending, err := NewRepo().GetPendingObservations("2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Status != ObsStatusPending {
		t.Fatalf("观察状态被误改: %+v", pending)
	}
}

// 确认引擎默认 readiness（同步日志实现）未就绪时必须返回诊断错误，不得静默跳过。
func TestConfirmEngineReportsDiagnosticWhenSyncIncomplete(t *testing.T) {
	readinessPipelineSeed(t)
	seedSyncLog(t, "sh600000", "done", "2026-11-10")
	seedSyncLog(t, "sz000001", "running", "2026-11-10")

	engine := NewConfirmationEngine(NewRepo())
	cfg := DefaultRiskControlConfig()
	ctx := context.Background()
	tradeDates := []string{"2026-11-09", "2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}
	marketRisk := risk.Level{Name: "正常", PositionLimit: 1, ScoreExtra: 0}
	_, err := engine.ConfirmForDate(ctx, "2026-11-10", tradeDates, marketRisk, cfg, cfg.InitialCash)
	if !errors.Is(err, ErrDataNotReady) {
		t.Fatalf("未就绪应返回 ErrDataNotReady 诊断: %v", err)
	}
	if !strings.Contains(err.Error(), "running=1") {
		t.Fatalf("诊断缺少 running 计数: %v", err)
	}
}
