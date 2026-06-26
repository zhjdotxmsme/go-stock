package datasource

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func setupInMemoryDB(t *testing.T) *gorm.DB {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&models.KLineBar{}))
	return conn
}

func TestQueryUpsertAndMissing(t *testing.T) {
	orig := db.Dao
	db.Dao = setupInMemoryDB(t)
	defer func() { db.Dao = orig }()

	ctx := context.Background()
	store := NewKLineStore()

	bars := []models.KLineBar{
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-02", Adjusted: false, Open: 100, High: 105, Low: 98, Close: 102, Volume: 1000, Amount: 100000, Source: "test"},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Adjusted: false, Open: 102, High: 106, Low: 100, Close: 104, Volume: 1200, Amount: 120000, Source: "test"},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-05", Adjusted: false, Open: 104, High: 108, Low: 102, Close: 106, Volume: 1100, Amount: 110000, Source: "test"},
	}

	err := store.UpsertKLines(ctx, bars)
	require.NoError(t, err)

	queried, err := store.QueryKLines(ctx, "sh600519", "day", "2024-01-02", "2024-01-05", false)
	require.NoError(t, err)
	assert.Len(t, queried, 3)
	assert.Equal(t, "2024-01-02", queried[0].TradeDate)
	assert.Equal(t, "2024-01-03", queried[1].TradeDate)
	assert.Equal(t, "2024-01-05", queried[2].TradeDate)

	upsertDup := []models.KLineBar{
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Adjusted: false, Open: 999, High: 999, Low: 999, Close: 999, Volume: 999, Amount: 999, Source: "test"},
	}
	err = store.UpsertKLines(ctx, upsertDup)
	require.NoError(t, err)

	queried2, err := store.QueryKLines(ctx, "sh600519", "day", "2024-01-02", "2024-01-05", false)
	require.NoError(t, err)
	assert.Len(t, queried2, 3)
	assert.Equal(t, float64(999), queried2[1].Open)
}

func TestFindMissingDateRanges(t *testing.T) {
	orig := db.Dao
	db.Dao = setupInMemoryDB(t)
	defer func() { db.Dao = orig }()

	ctx := context.Background()
	store := NewKLineStore()

	bars := []models.KLineBar{
		{StockCode: "sz000001", Period: "day", TradeDate: "2024-01-02", Adjusted: false, Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 1000, Amount: 10000, Source: "test"},
		{StockCode: "sz000001", Period: "day", TradeDate: "2024-01-03", Adjusted: false, Open: 10.5, High: 11, Low: 10, Close: 10.8, Volume: 1000, Amount: 10000, Source: "test"},
		{StockCode: "sz000001", Period: "day", TradeDate: "2024-01-08", Adjusted: false, Open: 11, High: 12, Low: 10.5, Close: 11.5, Volume: 1000, Amount: 10000, Source: "test"},
	}
	require.NoError(t, store.UpsertKLines(ctx, bars))

	missing, err := store.FindMissingDateRanges(ctx, "sz000001", "day", "2024-01-02", "2024-01-09", false)
	require.NoError(t, err)

	if len(missing) > 0 {
		for _, r := range missing {
			t.Logf("missing range: %s - %s", r.Start, r.End)
		}
	}
}
