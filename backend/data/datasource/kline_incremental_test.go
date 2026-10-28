package datasource

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/util/timeutil"
)

// fakeKLineProvider 按"最近 count 个工作日"动态生成 bars，记录调用次数。
// close 参数模拟前复权序列：除权后同一股票收盘价整体平移。
type fakeKLineProvider struct {
	close float64
	calls int32
}

func (p *fakeKLineProvider) Name() string                     { return "fake" }
func (p *fakeKLineProvider) Priority() int                    { return 1 }
func (p *fakeKLineProvider) Available(_ context.Context) bool { return true }

func (p *fakeKLineProvider) GetKLine(_ context.Context, code, period string, count int) (*KLineData, error) {
	atomic.AddInt32(&p.calls, 1)
	if count <= 0 {
		count = 10
	}
	// 生成最近 count 个工作日（含今天），与真实源"最近 N 根"语义一致
	dates := recentWeekdays(time.Now(), count)
	bars := make([]KLineBar, 0, count)
	for _, d := range dates {
		t, err := timeutil.ParseDate(d)
		if err != nil {
			continue
		}
		bars = append(bars, KLineBar{Time: t, Open: p.close, High: p.close, Low: p.close, Close: p.close, Volume: 1000})
	}
	return &KLineData{Code: code, Period: period, Bars: bars}, nil
}

func recentWeekdays(now time.Time, n int) []string {
	out := make([]string, 0, n)
	for d := now; len(out) < n; d = d.AddDate(0, 0, -1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		out = append([]string{timeutil.DateStr(d)}, out...)
	}
	return out
}

func seedStoredBars(t *testing.T, store *KLineStore, code string, dates []string, close float64) {
	t.Helper()
	bars := make([]models.KLineBar, 0, len(dates))
	for _, d := range dates {
		bars = append(bars, models.KLineBar{
			StockCode: code, Period: "day", TradeDate: d, Adjusted: true,
			Open: close, High: close, Low: close, Close: close, Volume: 1000, Source: "seed",
		})
	}
	require.NoError(t, store.UpsertKLines(context.Background(), bars))
}

func setupIncrementalTest(t *testing.T, close float64) (*Router, *fakeKLineProvider, *KLineStore) {
	t.Helper()
	orig := db.Dao
	db.Dao = setupInMemoryDB(t)
	origStore := globalKLineStore
	store := NewKLineStore()
	globalKLineStore = store
	t.Cleanup(func() {
		db.Dao = orig
		globalKLineStore = origStore
	})
	p := &fakeKLineProvider{close: close}
	r := &Router{klineProviders: []KLineProvider{p}}
	return r, p, store
}

func TestDetectAdjustedSeriesShift(t *testing.T) {
	stored := []models.KLineBar{
		{TradeDate: "2026-10-20", Close: 100},
		{TradeDate: "2026-10-21", Close: 100},
		{TradeDate: "2026-10-22", Close: 100},
	}
	// 同值 → 无平移
	assert.False(t, DetectAdjustedSeriesShift(stored, []models.KLineBar{{TradeDate: "2026-10-22", Close: 100}}, 8))
	// 除权：重叠日收盘 100 → 94（6% 平移）→ 检出
	assert.True(t, DetectAdjustedSeriesShift(stored, []models.KLineBar{{TradeDate: "2026-10-22", Close: 94}}, 8))
	// 容差内（0.3%）→ 不误报
	assert.False(t, DetectAdjustedSeriesShift(stored, []models.KLineBar{{TradeDate: "2026-10-22", Close: 100.3}}, 8))
	// 无重叠日 → 无法判定，不误报
	assert.False(t, DetectAdjustedSeriesShift(stored, []models.KLineBar{{TradeDate: "2026-10-27", Close: 94}}, 8))
	// 空输入 → false
	assert.False(t, DetectAdjustedSeriesShift(nil, nil, 8))
	// overlapLimit 之外的历史错位不参与判定
	oldShift := []models.KLineBar{
		{TradeDate: "2026-10-20", Close: 50},
		{TradeDate: "2026-10-21", Close: 100},
		{TradeDate: "2026-10-22", Close: 100},
	}
	assert.False(t, DetectAdjustedSeriesShift(oldShift, []models.KLineBar{{TradeDate: "2026-10-21", Close: 100}, {TradeDate: "2026-10-22", Close: 100}}, 2))
}

func TestExpectedLatestTradeDate(t *testing.T) {
	mk := func(s string) time.Time {
		ts, err := timeutil.ParseDateTime(s)
		require.NoError(t, err)
		return ts
	}
	// 周三盘中 → 当天（盘中实时 bar）
	assert.Equal(t, "2026-10-28", expectedLatestTradeDate(mk("2026-10-28 10:00:00")))
	// 周三未开盘 → 前一工作日
	assert.Equal(t, "2026-10-27", expectedLatestTradeDate(mk("2026-10-28 08:00:00")))
	// 周六 → 周五
	assert.Equal(t, "2026-10-30", expectedLatestTradeDate(mk("2026-10-31 12:00:00")))
	// 周日 → 周五
	assert.Equal(t, "2026-10-30", expectedLatestTradeDate(mk("2026-11-01 12:00:00")))
	// 周五盘后 → 周五
	assert.Equal(t, "2026-10-30", expectedLatestTradeDate(mk("2026-10-30 15:30:00")))
}

func TestGetKLineIncremental_StoreHitNoFetch(t *testing.T) {
	r, p, store := setupIncrementalTest(t, 100)
	// 库内已含今天（盘中场景），且数量覆盖 limit → 应零网络请求
	seedStoredBars(t, store, "sh600519", recentWeekdays(time.Now(), 20), 100)

	k, err := r.GetKLineIncremental(context.Background(), "sh600519", "day", 10)
	require.NoError(t, err)
	assert.Equal(t, "store", k.Source)
	assert.Len(t, k.Bars, 10)
	assert.Equal(t, int32(0), atomic.LoadInt32(&p.calls), "本地新鲜数据不应发起网络请求")
}

func TestGetKLineIncremental_TailFetchAndPersist(t *testing.T) {
	r, p, store := setupIncrementalTest(t, 100)
	// 库内只有 30 天前的数据 → 需要尾部补拉
	oldDates := recentWeekdays(time.Now().AddDate(0, 0, -30), 20)
	seedStoredBars(t, store, "sh600519", oldDates, 100)

	k, err := r.GetKLineIncremental(context.Background(), "sh600519", "day", 10)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&p.calls), "尾部缺口应只发起一次小 count 补拉")
	assert.Equal(t, "fake", k.Source)
	assert.Len(t, k.Bars, 10)
	// 最新一根应为今天
	assert.Equal(t, timeutil.TodayStr(), timeutil.DateStr(k.Bars[len(k.Bars)-1].Time))
	// 补拉数据已同步落库
	persisted, err := store.QueryKLines(context.Background(), "sh600519", "day", "1990-01-01", timeutil.DateStr(time.Now().AddDate(0, 0, 1)), true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(persisted), 20, "尾部补拉后库内数据应增长")
	assert.Equal(t, timeutil.TodayStr(), persisted[len(persisted)-1].TradeDate)
}

func TestGetKLineIncremental_ShiftTriggersFullRefresh(t *testing.T) {
	r, p, store := setupIncrementalTest(t, 100)
	dates := recentWeekdays(time.Now().AddDate(0, 0, -5), 20)
	seedStoredBars(t, store, "sh600519", dates, 100)

	// 除权：源端序列整体平移为 94
	p.close = 94

	k, err := r.GetKLineIncremental(context.Background(), "sh600519", "day", 10)
	require.NoError(t, err)
	assert.Contains(t, k.Source, "(refreshed)", "除权应触发全量重拉")
	assert.Equal(t, int32(2), atomic.LoadInt32(&p.calls), "先尾部补拉发现平移，再全量重拉，共 2 次")

	// 库内旧 bar 的 close 应被覆盖为平移后的值（历史不能保持 100）
	persisted, err := store.QueryKLines(context.Background(), "sh600519", "day", dates[0], dates[len(dates)-1], true)
	require.NoError(t, err)
	for _, b := range persisted {
		assert.Equal(t, float64(94), b.Close, "除权后历史 close 必须被全量覆盖: %s", b.TradeDate)
	}
}

func TestGetKLineIncremental_FetchFailureServesStale(t *testing.T) {
	r, p, store := setupIncrementalTest(t, 100)
	// 库内 30 天前数据，但源端故障
	dates := recentWeekdays(time.Now().AddDate(0, 0, -30), 20)
	seedStoredBars(t, store, "sh600519", dates, 100)
	failing := &failingKLineProvider{}
	r.klineProviders = []KLineProvider{failing}

	k, err := r.GetKLineIncremental(context.Background(), "sh600519", "day", 10)
	require.NoError(t, err, "库内够量时源端故障应回退旧数据而不是报错")
	assert.Equal(t, "store(stale)", k.Source)
	assert.Len(t, k.Bars, 10)
	_ = p
}

type failingKLineProvider struct{}

func (f *failingKLineProvider) Name() string                     { return "failing" }
func (f *failingKLineProvider) Priority() int                    { return 1 }
func (f *failingKLineProvider) Available(_ context.Context) bool { return true }
func (f *failingKLineProvider) GetKLine(_ context.Context, code, period string, count int) (*KLineData, error) {
	return nil, fmt.Errorf("simulated upstream failure")
}
