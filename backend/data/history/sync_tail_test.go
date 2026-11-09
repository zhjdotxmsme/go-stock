package history

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

type syncTailTestProvider struct {
	mu   sync.RWMutex
	data *datasource.KLineData
	err  error
}

func (p *syncTailTestProvider) Name() string                     { return "sync-tail-test" }
func (p *syncTailTestProvider) Priority() int                    { return 10000 }
func (p *syncTailTestProvider) Available(_ context.Context) bool { return true }
func (p *syncTailTestProvider) GetKLine(_ context.Context, _, _ string, _ int) (*datasource.KLineData, error) {
	return nil, errors.New("tail provider requires an end date")
}
func (p *syncTailTestProvider) GetKLineBefore(_ context.Context, code, period string, _ int, _ string) (*datasource.KLineData, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.data, nil
}

var (
	syncTailProvider     syncTailTestProvider
	syncTailProviderOnce sync.Once
)

func useSyncTailProvider(data *datasource.KLineData, err error) {
	syncTailProviderOnce.Do(func() {
		datasource.GetRouter().RegisterKLineProvider(&syncTailProvider)
	})
	syncTailProvider.mu.Lock()
	syncTailProvider.data = data
	syncTailProvider.err = err
	syncTailProvider.mu.Unlock()
}

func TestSyncKLineForStockAcceptsVerifiedTailEmptySpan(t *testing.T) {
	setupSyncTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := datasource.NewKLineStore()
	var stored []models.KLineBar
	for day := 1; day <= 31; day++ {
		stored = append(stored, models.KLineBar{
			StockCode: "sh600193", Period: "day", TradeDate: time.Date(2024, 1, day, 0, 0, 0, 0, time.Local).Format("2006-01-02"),
			Adjusted: true, Close: 10,
		})
	}
	if err := store.UpsertKLines(ctx, stored); err != nil {
		t.Fatal(err)
	}
	useSyncTailProvider(&datasource.KLineData{Code: "sh600193", Period: "day", Bars: []datasource.KLineBar{
		{Time: time.Date(2024, 1, 31, 0, 0, 0, 0, time.Local), Close: 10},
	}}, nil)

	if err := SyncKLineForStock(ctx, "sh600193", "day", "2024-01-01", "2024-03-31", true); err != nil {
		t.Fatalf("数据源已核实的尾部停牌空窗不应报错: %v", err)
	}
	var log models.KLineSyncLog
	if err := db.Dao.Where("stock_code = ?", "sh600193").First(&log).Error; err == nil && log.EmptySpans != "" {
		t.Fatalf("贴任务结束日的空窗不应持久化: %s", log.EmptySpans)
	}
	gaps := openGaps(ctx, &SyncTask{StockCode: "sh600193", Period: "day", StartDate: "2024-01-01", EndDate: "2024-03-31", Adjusted: true})
	if got := checkpointFromGaps("2024-01-01", "2024-03-31", gaps); got != "2024-01-31" {
		t.Fatalf("尾部空窗仍需保留重扫断点, got %s", got)
	}
}

func TestRunSyncTaskFailureCountsFullTaskWindow(t *testing.T) {
	setupSyncTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := datasource.NewKLineStore()
	var stored []models.KLineBar
	for day := 1; day <= 31; day++ {
		date := time.Date(2024, 1, day, 0, 0, 0, 0, time.Local)
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		stored = append(stored, models.KLineBar{StockCode: "sz000016", Period: "day", TradeDate: date.Format("2006-01-02"), Adjusted: true, Close: 10})
	}
	if err := store.UpsertKLines(ctx, stored); err != nil {
		t.Fatal(err)
	}
	useSyncTailProvider(nil, errors.New("source unavailable"))
	task := &SyncTask{StockCode: "sz000016", Period: "day", StartDate: "2024-01-01", EndDate: "2024-03-31", LastSyncDate: "2024-02-01", Adjusted: true, Status: SyncStatusPending}
	if err := CreateSyncTask(ctx, task.StockCode, task.Period, task.StartDate, task.EndDate, task.Adjusted); err != nil {
		t.Fatal(err)
	}
	if err := RunSyncTask(ctx, task); err == nil {
		t.Fatal("数据源失败应返回错误")
	}
	var log models.KLineSyncLog
	if err := db.Dao.Where("stock_code = ?", task.StockCode).First(&log).Error; err != nil {
		t.Fatal(err)
	}
	if log.SyncedCount != len(stored) {
		t.Fatalf("失败任务应统计全窗口 %d 根，实际 %d", len(stored), log.SyncedCount)
	}
}
