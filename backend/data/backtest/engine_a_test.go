package backtest

import (
	"context"
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

// ----- T+1 约束测试 -----

func TestTPlus1_NoExitOnSignalDay(t *testing.T) {
	// T+1: 买入日仅一根 bar，次日无数据 → 应返回错误（无退出机会）
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_tplus1"
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 102, 99, 101, 99.5}, // 信号日，PrevClose=99.5
	}))

	engine := NewEngine()
	_, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 5,
	})
	assert.ErrorContains(t, err, "no exit found")
}

func TestTPlus1_CanExitOnNextDay(t *testing.T) {
	// T+1: 信号日次日起可卖出
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_tplus1_exit"
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 102, 99, 101, 99.5}, // 信号日
		{"2024-01-03", 101, 105, 100, 104, 101}, // 次日卖出
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 5,
	})
	require.NoError(t, err)
	assert.True(t, result.Win)
	assert.Equal(t, "2024-01-03", result.ExitDate)
}

// ----- 涨跌停约束测试 -----

func TestPriceLimit_BuyOnLimitUp_Rejected(t *testing.T) {
	// 涨停日买入被拒绝
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_limit_up"
	// PrevClose=100, Close=110 = 10% 涨停
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 110, 100, 110, 100}, // 涨停日，PrevClose=100
		{"2024-01-03", 110, 112, 108, 111, 110},
	}))

	engine := NewEngine()
	_, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  110,
		HoldingDays: 5,
	})
	assert.ErrorContains(t, err, "price limit")
}

// ----- 最小手数约束测试 -----

func TestLotSize_InvalidShares_Error(t *testing.T) {
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_lot"
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 102, 99, 101, 99.5},
		{"2024-01-03", 101, 103, 100, 102, 101},
	}))

	engine := NewEngine()
	_, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 3,
		Shares:      150, // 非 100 整数倍
	})
	assert.ErrorContains(t, err, "multiple of 100")
}

func TestLotSize_ValidShares_Passes(t *testing.T) {
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_lot_ok"
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 102, 99, 101, 99.5},
		{"2024-01-03", 101, 104, 100, 103, 101},
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 3,
		Shares:      200,
	})
	require.NoError(t, err)
	assert.True(t, result.Win)
}

// ----- ST 约束测试 -----

func TestST_Limit5Percent(t *testing.T) {
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_st"
	// ST 股：PrevClose=100, Close=105 = 5% 涨停阈值 (100*1.049=104.9)
	// 判断 Close >= 104.9 触发涨停拒绝
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 105, 100, 105, 100}, // ST 涨停
		{"2024-01-03", 105, 106, 104, 105, 105},
	}))

	engine := NewEngine()
	_, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  105,
		HoldingDays: 3,
		IsST:        true,
	})
	assert.ErrorContains(t, err, "price limit")
}

func TestST_NonST_Still10Percent(t *testing.T) {
	restore := setupAEngineTestDB(t)
	defer restore()

	code := "test_normal"
	// 主板非 ST：PrevClose=100, Close=105 (5% 未到 10% 阈值)，可以买入
	registerEngineMock(code, makeABars([]aprice{
		{"2024-01-02", 100, 105, 100, 105, 100},
		{"2024-01-03", 105, 108, 104, 107, 105},
	}))

	engine := NewEngine()
	result, err := engine.Run(context.Background(), Input{
		StockCode:   code,
		SignalDate:  "2024-01-02",
		EntryPrice:  100,
		HoldingDays: 3,
		IsST:        false,
	})
	require.NoError(t, err)
	assert.True(t, result.Win) // 5% 涨幅，非 ST 应正常交易
}

// ----- 测试辅助函数 (与 existing engine_test.go 兼容) -----

type aprice struct {
	date             string
	o, h, l, c, prev float64
}

func makeABars(prices []aprice) []datasource.KLineBar {
	bars := make([]datasource.KLineBar, len(prices))
	for i, p := range prices {
		t, _ := time.Parse("2006-01-02", p.date)
		bars[i] = datasource.KLineBar{
			Time:      t,
			Open:      p.o,
			High:      p.h,
			Low:       p.l,
			Close:     p.c,
			PrevClose: p.prev,
			Volume:    1000,
			Amount:    100000,
		}
	}
	return bars
}

func setupAEngineTestDB(t *testing.T) func() {
	orig := db.Dao
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&models.KLineBar{}))
	db.Dao = conn
	return func() {
		db.Dao = orig
	}
}
