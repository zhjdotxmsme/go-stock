package datasource

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// fakeAnchoredProvider 实现锚定取数：返回 endDate 之前的 bars，记录入参供断言。
type fakeAnchoredProvider struct {
	endDateArg string
	countArg   int
	calls      int32
	fail       bool
}

func (p *fakeAnchoredProvider) Name() string                     { return "fake-anchored" }
func (p *fakeAnchoredProvider) Priority() int                    { return 1 }
func (p *fakeAnchoredProvider) Available(_ context.Context) bool { return true }
func (p *fakeAnchoredProvider) GetKLine(_ context.Context, code, period string, count int) (*KLineData, error) {
	return nil, context.DeadlineExceeded
}

func (p *fakeAnchoredProvider) GetKLineBefore(_ context.Context, code, period string, count int, endDate string) (*KLineData, error) {
	atomic.AddInt32(&p.calls, 1)
	p.endDateArg = endDate
	p.countArg = count
	if p.fail {
		return nil, context.DeadlineExceeded
	}
	endT, _ := time.Parse("2006-01-02", endDate)
	bars := make([]KLineBar, 0, count)
	for i := range count {
		bars = append(bars, KLineBar{Time: endT.AddDate(0, 0, -(count - i)), Close: 100})
	}
	return &KLineData{Code: code, Period: period, Bars: bars}, nil
}

func setupAnchoredTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.Dao = d
	t.Cleanup(func() {
		if sqlDB, err := d.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db.Dao = old
	})
	_ = db.Dao.AutoMigrate(&models.KLineBar{})
}

// 回归：FetchKLineNoPersistBefore 必须优先走锚定 provider（历史中段缺口回补的根因修复），
// 锚定失败时才退化为尾部窗口拉取。
func TestFetchKLineNoPersistBefore_PrefersAnchored(t *testing.T) {
	setupAnchoredTestDB(t)
	anchored := &fakeAnchoredProvider{}
	plain := &fakeKLineProvider{close: 100}
	r := &Router{klineProviders: []KLineProvider{plain, anchored}}

	endDate := "2025-06-30"
	k, err := r.FetchKLineNoPersistBefore(context.Background(), "sh600519", "day", 10, endDate)
	if err != nil {
		t.Fatalf("FetchKLineNoPersistBefore: %v", err)
	}
	if anchored.calls != 1 {
		t.Fatalf("anchored provider calls = %d, want 1", anchored.calls)
	}
	if anchored.endDateArg != endDate {
		t.Fatalf("anchored endDate arg = %q, want %q", anchored.endDateArg, endDate)
	}
	if plain.calls != 0 {
		t.Fatalf("非锚定 provider 不应被调用，实际 %d 次", plain.calls)
	}
	if len(k.Bars) != 10 {
		t.Fatalf("bars = %d, want 10", len(k.Bars))
	}
}

// 锚定 provider 失败时退化为尾部窗口拉取，count 按缺口到今天的跨度放大。
func TestFetchKLineNoPersistBefore_FallsBackOnAnchorFailure(t *testing.T) {
	setupAnchoredTestDB(t)
	anchored := &fakeAnchoredProvider{fail: true}
	plain := &fakeKLineProvider{close: 100}
	r := &Router{klineProviders: []KLineProvider{plain, anchored}}

	endDate := "2025-06-30"
	k, err := r.FetchKLineNoPersistBefore(context.Background(), "sh600519", "day", 10, endDate)
	if err != nil {
		t.Fatalf("退化路径不应报错: %v", err)
	}
	if anchored.calls != 1 {
		t.Fatalf("anchored calls = %d, want 1", anchored.calls)
	}
	if plain.calls == 0 {
		t.Fatal("锚定失败后应走尾部拉取")
	}
	if len(k.Bars) == 0 {
		t.Fatal("退化路径应返回尾部 bars")
	}
}
