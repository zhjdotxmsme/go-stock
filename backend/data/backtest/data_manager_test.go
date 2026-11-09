package backtest

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func setupDataManagerTestDB(t *testing.T) {
	t.Helper()
	orig := db.Dao
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&models.KLineBar{}, &models.AllStockInfo{}, &models.KLineSyncLog{}))
	db.Dao = conn
	t.Cleanup(func() { db.Dao = orig })
}

func TestGetKLineHistorySearchesNamesAndPagesMixedFormats(t *testing.T) {
	setupDataManagerTestDB(t)
	require.NoError(t, db.Dao.Create(&models.AllStockInfo{
		SECUCODE: "600519.SH", SECURITYCODE: "600519", SECURITYNAMEABBR: "贵州茅台",
	}).Error)
	require.NoError(t, db.Dao.Create(&models.AllStockInfo{
		SECUCODE: "000001.SZ", SECURITYCODE: "000001", SECURITYNAMEABBR: "平安银行",
	}).Error)

	bars := []models.KLineBar{
		{StockCode: "600519", Period: "day", TradeDate: "2024-01-01", Close: 100},
		{StockCode: "600519.SH", Period: "day", TradeDate: "2024-01-02", Close: 101},
		{StockCode: "sh600519", Period: "day", TradeDate: "2024-01-03", Close: 102},
		{StockCode: "sz000001", Period: "day", TradeDate: "2024-01-04", Close: 10},
	}
	for _, bar := range bars {
		require.NoError(t, db.Dao.Create(&bar).Error)
	}

	service := NewService()
	first, err := service.GetKLineHistory("贵州", 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), first.Total)
	assert.Equal(t, 2, first.TotalPages)
	require.Len(t, first.List, 2)
	assert.Equal(t, "2024-01-03", first.List[0].TradeDate)
	assert.Equal(t, "贵州茅台", first.List[0].StockName)
	assert.Equal(t, "sh600519", first.List[0].StockCode)

	last, err := service.GetKLineHistory("贵州", 2, 2)
	require.NoError(t, err)
	require.Len(t, last.List, 1)
	assert.Equal(t, "2024-01-01", last.List[0].TradeDate)
	assert.Equal(t, "贵州茅台", last.List[0].StockName)

	byCode, err := service.GetKLineHistory("600519.SH", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), byCode.Total)

	all, err := service.GetKLineHistory("", 0, 500)
	require.NoError(t, err)
	assert.Equal(t, int64(4), all.Total)
	assert.Equal(t, 1, all.Page)
	assert.Equal(t, 100, all.PageSize)
	assert.Len(t, all.List, 4)
}

func TestGetSyncProgressIncludesDatabaseSummary(t *testing.T) {
	setupDataManagerTestDB(t)
	logs := []models.KLineSyncLog{
		{StockCode: "sh600000", Status: history.SyncStatusPending},
		{StockCode: "sh600001", Status: history.SyncStatusRunning, SyncedCount: 25, ExpectedCount: 100},
		{StockCode: "sh600002", Status: history.SyncStatusDone},
		{StockCode: "sh600003", Status: history.SyncStatusFailed, ErrorMsg: "network unavailable"},
		{StockCode: "sh600004", Status: history.SyncStatusPartial, SyncedCount: 80, ExpectedCount: 100},
	}
	for _, log := range logs {
		require.NoError(t, db.Dao.Create(&log).Error)
	}

	progress, err := NewService().GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, int64(1), progress.Summary.Pending)
	assert.Equal(t, int64(1), progress.Summary.Running)
	assert.Equal(t, int64(1), progress.Summary.Done)
	assert.Equal(t, int64(1), progress.Summary.Failed)
	assert.Equal(t, int64(1), progress.Summary.Partial)
	require.Len(t, progress.Tasks, 5)
	tasks := make(map[string]SyncTaskItem, len(progress.Tasks))
	for _, task := range progress.Tasks {
		tasks[task.StockCode] = task
	}
	assert.Equal(t, 25, tasks["sh600001"].Progress)
	assert.Equal(t, 100, tasks["sh600002"].Progress)
	assert.Equal(t, "network unavailable", tasks["sh600003"].ErrorMsg)
}
