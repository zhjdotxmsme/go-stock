package backtest

import (
	"context"
	"math"
	"sort"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestGenerateSignalDates(t *testing.T) {
	orig := db.Dao
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&models.KLineBar{}))
	db.Dao = conn
	defer func() { db.Dao = orig }()

	bars := []models.KLineBar{
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-02", Adjusted: false},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Adjusted: false},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-05", Adjusted: false},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Adjusted: false},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-04", Adjusted: true},
	}
	for _, b := range bars {
		db.Dao.Create(&b)
	}

	dates, err := generateSignalDates(context.Background(), "sh600519", "day", "2024-01-01", "2024-01-10", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"2024-01-02", "2024-01-03", "2024-01-05"}, dates)
	assert.True(t, sort.StringsAreSorted(dates), "dates must be sorted")
}

func TestAggregateResults(t *testing.T) {
	results := []*Result{
		{Win: true, TotalReturn: 0.10, HoldingDays: 5, MaxDrawdown: 0.02},
		{Win: true, TotalReturn: 0.05, HoldingDays: 3, MaxDrawdown: 0.01},
		{Win: false, TotalReturn: -0.03, HoldingDays: 4, MaxDrawdown: 0.05},
		{Win: true, TotalReturn: 0.08, HoldingDays: 6, MaxDrawdown: 0.03},
		{Win: false, TotalReturn: -0.02, HoldingDays: 2, MaxDrawdown: 0.04},
	}

	agg := aggregateResults(results)
	assert.Equal(t, 5, agg.TotalTrades)
	assert.Equal(t, 3, agg.WinCount)
	assert.Equal(t, 2, agg.LossCount)
	assert.InDelta(t, 0.6, agg.WinRate, 0.001)
	assert.InDelta(t, (0.10+0.05-0.03+0.08-0.02)/5.0, agg.AvgReturn, 0.001)

	expectedTotal := (1.10 * 1.05 * 0.97 * 1.08 * 0.98) - 1
	assert.InDelta(t, expectedTotal, agg.TotalReturn, 0.001)

	assert.InDelta(t, 0.03, agg.MaxDrawdown, 0.001)

	assert.Greater(t, agg.SharpeRatio, 0.0)
}

func TestAggregateResults_SingleResult(t *testing.T) {
	results := []*Result{
		{Win: true, TotalReturn: 0.10, HoldingDays: 5, MaxDrawdown: 0.02},
	}
	agg := aggregateResults(results)
	assert.Equal(t, 1, agg.TotalTrades)
	assert.Equal(t, 1, agg.WinCount)
	assert.InDelta(t, 1.0, agg.WinRate, 0.001)
	assert.InDelta(t, 0.10, agg.AvgReturn, 0.001)
	assert.InDelta(t, 0.0, agg.SharpeRatio, 0.001)
}

func TestAggregateResults_SharpeCalculation(t *testing.T) {
	const eps = 1e-9

	r1 := 0.05
	r2 := -0.02
	r3 := 0.03
	mean := (r1 + r2 + r3) / 3.0
	variance := ((r1-mean)*(r1-mean) + (r2-mean)*(r2-mean) + (r3-mean)*(r3-mean)) / 2.0
	std := math.Sqrt(variance)
	avgHolding := (5.0 + 4.0 + 6.0) / 3.0
	expectedSharpe := (mean / std) * math.Sqrt(252.0/avgHolding)

	results := []*Result{
		{Win: r1 > 0, TotalReturn: r1, HoldingDays: 5},
		{Win: r2 > 0, TotalReturn: r2, HoldingDays: 4},
		{Win: r3 > 0, TotalReturn: r3, HoldingDays: 6},
	}
	agg := aggregateResults(results)
	assert.InDelta(t, expectedSharpe, agg.SharpeRatio, 0.001)
}
