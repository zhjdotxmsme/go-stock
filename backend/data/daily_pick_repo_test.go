package data

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"go-stock/backend/models"
	"go-stock/backend/util/timeutil"
)

// 回归：生产库 daily_picks 由 GORM AutoMigrate 建表，Return3D/Return5D 实际列名为
// return3_d/return5_d；原生 SQL 曾写 return3d/return5d 导致 backfillWindows 每次报
// "no such column: return3d"。本测试用 AutoMigrate 建表后执行真实查询，列名漂移即失败。
func TestRecentPicksMissingWindows_ColumnNames(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&models.DailyPick{}))
	repo := &DailyPickRepository{db: conn}

	today := timeutil.TodayStr()
	picks := []models.DailyPick{
		{StockCode: "sz000651", StockName: "格力电器", TradeDate: today, Reviewed: true, Return3D: 0, Return5D: 0},
		{StockCode: "sh600519", StockName: "贵州茅台", TradeDate: today, Reviewed: true, Return3D: 3.2, Return5D: 5.1},
		{StockCode: "sz000001", StockName: "平安银行", TradeDate: timeutil.DateStr(time.Now().AddDate(0, 0, -30)), Reviewed: true},
	}
	for i := range picks {
		require.NoError(t, conn.Create(&picks[i]).Error)
	}

	got, err := repo.RecentPicksMissingWindows(context.Background(), 15)
	require.NoError(t, err, "列名与 GORM 建表不一致时此处报 no such column")
	require.Len(t, got, 1)
	assert.Equal(t, "sz000651", got[0].StockCode)
}
