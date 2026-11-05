package datasource

import (
	"context"
	"errors"
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
	empty      bool
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
	if p.empty {
		return nil, ErrNoBarsBefore
	}
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

// 锚定 provider 失败时，只有最近两周的结束日才退化为尾部窗口拉取。
func TestFetchKLineNoPersistBefore_FallsBackOnAnchorFailure(t *testing.T) {
	setupAnchoredTestDB(t)
	anchored := &fakeAnchoredProvider{fail: true}
	plain := &fakeKLineProvider{close: 100}
	r := &Router{klineProviders: []KLineProvider{plain, anchored}}

	endDate := time.Now().Format("2006-01-02")
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

// 历史结束日锚定失败时不能改拉最新一段。生产上这段会先卡通达信 25 秒，再被判成锚定未被遵守。
func TestFetchKLineNoPersistBefore_HistoricalMissDoesNotFetchTail(t *testing.T) {
	setupAnchoredTestDB(t)
	anchored := &fakeAnchoredProvider{fail: true}
	plain := &fakeKLineProvider{close: 100}
	r := &Router{klineProviders: []KLineProvider{plain, anchored}}

	_, err := r.FetchKLineNoPersistBefore(context.Background(), "sz301248", "day", 130, "2022-04-19")
	if err == nil {
		t.Fatal("历史锚定失败应返回错误")
	}
	if plain.calls != 0 {
		t.Fatalf("历史缺口不应走尾部拉取，实际 %d 次", plain.calls)
	}
}

// 锚定源确认没有 K 线（上市前）时返回 ErrNoBarsBefore，同样不拉最新一段。
func TestFetchKLineNoPersistBefore_ConfirmedEmpty(t *testing.T) {
	setupAnchoredTestDB(t)
	anchored := &fakeAnchoredProvider{empty: true}
	plain := &fakeKLineProvider{close: 100}
	r := &Router{klineProviders: []KLineProvider{plain, anchored}}

	_, err := r.FetchKLineNoPersistBefore(context.Background(), "sz301248", "day", 130, "2022-04-19")
	if !errors.Is(err, ErrNoBarsBefore) {
		t.Fatalf("want ErrNoBarsBefore, got %v", err)
	}
	if plain.calls != 0 {
		t.Fatalf("确认空窗不应走尾部拉取，实际 %d 次", plain.calls)
	}
}

// 一个锚定源确认没有 K 线，另一个只是页面/501。历史结束日应记空窗，不能把传输错误当成整段失败。
func TestFetchKLineNoPersistBefore_EmptyWinsOverHistoricalTransportError(t *testing.T) {
	setupAnchoredTestDB(t)
	bad := &fakeAnchoredProvider{fail: true}
	empty := &fakeAnchoredProvider{empty: true}
	plain := &fakeKLineProvider{close: 100}
	r := &Router{klineProviders: []KLineProvider{bad, empty, plain}}

	_, err := r.FetchKLineNoPersistBefore(context.Background(), "sz001395", "day", 200, "2025-01-26")
	if !errors.Is(err, ErrNoBarsBefore) {
		t.Fatalf("want ErrNoBarsBefore, got %v", err)
	}
	if plain.calls != 0 {
		t.Fatalf("历史空窗不应走尾部拉取，实际 %d 次", plain.calls)
	}
}
