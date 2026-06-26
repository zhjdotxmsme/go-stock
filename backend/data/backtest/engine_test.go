package backtest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

var (
	globalMockData sync.Map
	mockOnce       sync.Once
)

type engineMockProvider struct{}

func (engineMockProvider) Name() string                     { return "test-engine" }
func (engineMockProvider) Priority() int                    { return 0 }
func (engineMockProvider) Available(_ context.Context) bool { return true }
func (engineMockProvider) GetKLine(_ context.Context, code string, _ string, _ int) (*datasource.KLineData, error) {
	if d, ok := globalMockData.Load(code); ok {
		return d.(*datasource.KLineData), nil
	}
	return nil, datasource.ErrAllSourcesFailed
}

func registerEngineMock(code string, bars []datasource.KLineBar) {
	mockOnce.Do(func() {
		datasource.GetRouter().RegisterKLineProvider(engineMockProvider{})
	})
	globalMockData.Store(code, &datasource.KLineData{Bars: bars})
}

type price struct {
	date       string
	o, h, l, c float64
}

func makeBars(prices []price) []datasource.KLineBar {
	bars := make([]datasource.KLineBar, len(prices))
	for i, p := range prices {
		t, _ := time.Parse("2006-01-02", p.date)
		bars[i] = datasource.KLineBar{
			Time:   t,
			Open:   p.o,
			High:   p.h,
			Low:    p.l,
			Close:  p.c,
			Volume: 1000,
			Amount: 100000,
		}
	}
	return bars
}

func setupEngineTestDB(t *testing.T) func() {
	orig := db.Dao
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&models.KLineBar{}))
	db.Dao = conn
	return func() {
		time.Sleep(50 * time.Millisecond)
		db.Dao = orig
	}
}

func TestEngineRun_Win(t *testing.T) {
	restoreDB := setupEngineTestDB(t)
	defer restoreDB()

	code := "mock_win"
	registerEngineMock(code, makeBars([]price{
		{"2024-01-02", 100, 102, 99, 101},
		{"2024-01-03", 101, 105, 100, 104},
		{"2024-01-04", 104, 108, 103, 107},
		{"2024-01-05", 107, 110, 106, 109},
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 20,
		StopLoss:    0,
		StopProfit:  0,
	})
	require.NoError(t, err)
	assert.True(t, result.Win)
	assert.Greater(t, result.TotalReturn, 0.0)
	assert.Equal(t, 100.0, result.EntryPrice)
	assert.Equal(t, 109.0, result.ExitPrice)
}

func TestEngineRun_Loss(t *testing.T) {
	restoreDB := setupEngineTestDB(t)
	defer restoreDB()

	code := "mock_loss"
	registerEngineMock(code, makeBars([]price{
		{"2024-01-02", 100, 102, 99, 101},
		{"2024-01-03", 100, 101, 96, 97},
		{"2024-01-04", 97, 98, 94, 95},
		{"2024-01-05", 95, 96, 92, 93},
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 20,
		StopLoss:    0,
		StopProfit:  0,
	})
	require.NoError(t, err)
	assert.False(t, result.Win)
	assert.Less(t, result.TotalReturn, 0.0)
}

func TestEngineRun_StopLoss(t *testing.T) {
	restoreDB := setupEngineTestDB(t)
	defer restoreDB()

	code := "mock_stop"
	registerEngineMock(code, makeBars([]price{
		{"2024-01-02", 100, 102, 99, 101},
		{"2024-01-03", 99, 100, 88, 90},
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 20,
		StopLoss:    0.08,
		StopProfit:  0,
	})
	require.NoError(t, err)
	assert.InDelta(t, -0.08, result.TotalReturn, 0.001)
	assert.LessOrEqual(t, result.ExitPrice, 92.0+1e-6)
}

func TestEngineRun_TakeProfit(t *testing.T) {
	restoreDB := setupEngineTestDB(t)
	defer restoreDB()

	code := "mock_tp"
	registerEngineMock(code, makeBars([]price{
		{"2024-01-02", 100, 102, 99, 101},
		{"2024-01-03", 101, 125, 100, 120},
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 20,
		StopLoss:    0,
		StopProfit:  0.20,
	})
	require.NoError(t, err)
	assert.InDelta(t, 0.20, result.TotalReturn, 0.001)
	assert.GreaterOrEqual(t, result.ExitPrice, 120.0-1e-6)
}
