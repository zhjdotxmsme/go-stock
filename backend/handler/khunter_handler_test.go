package handler

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Task 11：研究/观察/完整回测 API 接线测试。
// 研究运行绝不写生产表；完整回测推送 Task 10 全量报告；观察列表按状态过滤。

func setupKhunterTestDB(t *testing.T) {
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
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}, &models.KLineBar{}, &models.KLineSyncLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := khunter.EnsureMigrate(); err != nil {
		t.Fatalf("khunter migrate: %v", err)
	}
}

// eventRecorder 收集异步事件并按事件名等待。
type eventRecorder struct {
	mu     sync.Mutex
	events map[string][]map[string]any
}

func newEventRecorder() *eventRecorder {
	return &eventRecorder{events: map[string][]map[string]any{}}
}

func (r *eventRecorder) emit(event string, payload ...any) {
	m := map[string]any{}
	if len(payload) > 0 {
		if v, ok := payload[0].(map[string]any); ok {
			m = v
		} else {
			m["value"] = payload[0]
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[event] = append(r.events[event], m)
}

func (r *eventRecorder) waitFor(t *testing.T, event string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		if list := r.events[event]; len(list) > 0 {
			out := list[len(list)-1]
			r.mu.Unlock()
			return out
		}
		r.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Fatalf("等待事件 %s 超时，已收到: %v", event, r.events)
	return nil
}

// seedResearchUniverse 一只股票 + 截至 tradeDate 的 30 根日K（研究扫描输入）。
func seedResearchUniverse(t *testing.T, code, secucode, name, tradeDate string) {
	t.Helper()
	if err := db.Dao.Create(&models.AllStockInfo{
		SECUCODE: secucode, SECURITYCODE: code, SECURITYNAMEABBR: name,
	}).Error; err != nil {
		t.Fatal(err)
	}
	end, err := time.ParseInLocation("2006-01-02", tradeDate, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		date := end.AddDate(0, 0, i-29).Format("2006-01-02")
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh" + code, Period: "day", TradeDate: date, Adjusted: true,
			Open: 10, High: 10, Low: 10, Close: 10, PrevClose: 10, Volume: 1000,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestKhunterHandlerRunResearchPipelineAsync(t *testing.T) {
	setupKhunterTestDB(t)
	seedResearchUniverse(t, "600001", "600001.SH", "研究股", "2026-11-06")

	rec := newEventRecorder()
	h := NewKhunterHandler(rec.emit)
	resp := h.RunResearchPipelineAsync("2026-11-06")
	if resp["started"] != true {
		t.Fatalf("应返回 started: %+v", resp)
	}
	ev := rec.waitFor(t, "khunter:research_progress", 20*time.Second)
	if ev["error"] != nil && ev["error"] != "" {
		t.Fatalf("研究运行失败: %v", ev["error"])
	}
	raw, _ := json.Marshal(ev["result"])
	var result khunter.PipelineResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("结果不可解析: %v", err)
	}
	if result.TradeDate != "2026-11-06" {
		t.Fatalf("交易日不符: %+v", result)
	}
	// 研究模式绝不写生产表：只应留下研究记录
	var signals, scores, summaries, hunting int64
	db.Dao.Model(&models.KhunterSignal{}).Count(&signals)
	db.Dao.Model(&models.KhunterScore{}).Count(&scores)
	db.Dao.Model(&models.KhunterRunSummary{}).Count(&summaries)
	db.Dao.Model(&models.KhunterHunting{}).Count(&hunting)
	if signals != 0 || scores != 0 || summaries != 0 || hunting != 0 {
		t.Fatalf("研究运行写入了生产表: signals=%d scores=%d summaries=%d hunting=%d",
			signals, scores, summaries, hunting)
	}
	record, err := khunter.NewRepo().GetResearchRecord("2026-11-06")
	if err != nil || record == nil {
		t.Fatalf("研究记录缺失: %v", err)
	}
}

func TestKhunterHandlerGetObservations(t *testing.T) {
	setupKhunterTestDB(t)
	repo := khunter.NewRepo()
	rows := []models.KhunterObservation{
		{Code: "600001", Name: "观察股", SignalDate: "2026-11-06", Status: khunter.ObsStatusPending,
			DataStatus: "intraday", TriggerPrice: 12, SignalLow: 11, ATR14: 0.5, Score: 78},
		{Code: "600002", Name: "确认股", SignalDate: "2026-11-05", Status: khunter.ObsStatusConfirmed,
			DataStatus: "valid", TriggerPrice: 20, SignalLow: 18, ATR14: 1, Score: 85},
	}
	for i := range rows {
		if err := repo.UpsertObservation(&rows[i]); err != nil {
			t.Fatal(err)
		}
	}
	h := NewKhunterHandler(nil)
	all, err := h.GetObservations("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("全部观察应有 2 条: %d", len(all))
	}
	pending, err := h.GetObservations(khunter.ObsStatusPending)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Code != "600001" || pending[0].DataStatus != "intraday" {
		t.Fatalf("按状态过滤不符: %+v", pending)
	}
}

// bizDays 生成从 start 起 n 个工作日（跳过周末）。
func bizDays(start string, n int) []string {
	out := make([]string, 0, n)
	d, err := time.ParseInLocation("2006-01-02", start, time.Local)
	if err != nil {
		panic(err)
	}
	for len(out) < n {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			out = append(out, d.Format("2006-01-02"))
		}
		d = d.AddDate(0, 0, 1)
	}
	return out
}

func TestKhunterHandlerRunFullBacktestAsync(t *testing.T) {
	setupKhunterTestDB(t)
	days := bizDays("2026-01-05", 12)
	// 基准（交易日序列）+ 每日研究记录（空入选 → 四组对照只验接线与输入一致性）
	for i, d := range days {
		c := float64(4000 + i)
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: khunter.PortfolioBenchmarkCode, Period: "day", TradeDate: d, Adjusted: false,
			Open: c, High: c, Low: c, Close: c, PrevClose: c - 1,
			Volume: 1000000, VolumeUnit: "share", Amount: c * 1000000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
		payload := khunter.ResearchRecordResult{
			Result: &khunter.PipelineResult{TradeDate: d},
		}
		blob, _ := json.Marshal(payload)
		if err := db.Dao.Create(&models.KhunterResearchRecord{
			TradeDate: d, Status: "success", ResultJSON: string(blob),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	rec := newEventRecorder()
	h := NewKhunterHandler(rec.emit)
	resp := h.RunFullBacktestAsync(nil, days[0], days[len(days)-1])
	if resp["started"] != true {
		t.Fatalf("应返回 started: %+v", resp)
	}
	ev := rec.waitFor(t, "khunter:full_backtest_done", 30*time.Second)
	if ev["error"] != nil && ev["error"] != "" {
		t.Fatalf("完整回测失败: %v", ev["error"])
	}
	raw, _ := json.Marshal(ev["result"])
	var result khunter.FullBacktestResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("完整报告不可解析: %v", err)
	}
	if len(result.Variants) != 4 {
		t.Fatalf("应推送四组对照完整报告: %+v", result.Variants)
	}
	costHash := result.Variants[0].Manifest.CostHash
	for _, v := range result.Variants {
		if v.Manifest.CostHash != costHash || v.Report == nil {
			t.Fatalf("四组共享输入凭证/报告缺失: %+v", v)
		}
	}
	// 回测绝不写生产狩猎池
	var hunting int64
	db.Dao.Model(&models.KhunterHunting{}).Count(&hunting)
	if hunting != 0 {
		t.Fatalf("回测不得写生产狩猎池: %d", hunting)
	}
	if fmt.Sprint(result.StartDate) != days[0] {
		t.Fatalf("开始日期不符: %s", result.StartDate)
	}
}
