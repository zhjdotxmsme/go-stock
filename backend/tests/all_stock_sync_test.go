package tests

import (
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestAllStockInfo_SyncAPI(t *testing.T) {
	dbPath := findDB()
	if dbPath == "" {
		t.Skip("no stock.db found")
	}
	db.Init(dbPath)
	if db.Dao == nil {
		t.Fatal("db init failed")
	}

	api := data.NewStockDataApi()
	if api == nil {
		t.Fatal("NewStockDataApi returned nil")
	}

	// Fetch page 1, small batch to verify connectivity
	resp := api.GetAllStocks(1, 5, "", models.TechnicalIndicators{})
	if resp == nil {
		t.Fatal("GetAllStocks returned nil")
	}

	t.Logf("GetAllStocks: count=%d page=%d", resp.Result.Count, resp.Result.Currentpage)
	if len(resp.Result.Data) == 0 {
		t.Fatal("GetAllStocks returned 0 stocks — EastMoney API may be blocked")
	}

	// Log first few stocks
	for i, s := range resp.Result.Data {
		if i >= 3 {
			break
		}
		t.Logf("  #%d: %s %s %s", i+1, s.SECUCODE, s.SECURITYCODE, s.SECURITYNAMEABBR)
	}
}
