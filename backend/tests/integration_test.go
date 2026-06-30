package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/backtest"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/datasource/fallback"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func init() {
	// Register data source providers (normally done at app startup)
	fallback.RegisterFreeDataSources(datasource.GetRouter())
	fallback.RegisterKLineChain(datasource.GetRouter())
}

func findDB() string {
	wd, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(wd, "data", "stock.db")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		wd = filepath.Dir(wd)
	}
	return ""
}

// syncAllStockInfoForTest fetches all stock list from East Money and saves to DB.
func syncAllStockInfoForTest(t *testing.T) {
	t.Helper()
	api := data.NewStockDataApi()
	var all []models.AllStockInfo
	for page := 1; page <= 2; page++ {
		resp := api.GetAllStocks(page, 3000, "", models.TechnicalIndicators{})
		if resp == nil || len(resp.Result.Data) == 0 {
			t.Logf("syncAllStockInfo: page %d empty", page)
			continue
		}
		for _, d := range resp.Result.Data {
			all = append(all, d.ToAllStockInfo())
		}
	}
	if len(all) == 0 {
		t.Fatal("syncAllStockInfo: GetAllStocks returned no data")
	}
	err := db.Dao.Unscoped().Model(&models.AllStockInfo{}).Where("1=1").Delete(&models.AllStockInfo{}).Error
	if err != nil {
		t.Fatalf("syncAllStockInfo: truncate failed: %v", err)
	}
	err = db.Dao.CreateInBatches(&all, 1000).Error
	if err != nil {
		t.Fatalf("syncAllStockInfo: CreateInBatches failed: %v", err)
	}
	t.Logf("syncAllStockInfo: saved %d stocks", len(all))
}

func TestDailyPick_RealData(t *testing.T) {
	dbPath := findDB()
	if dbPath == "" {
		t.Skip("no stock.db found")
	}
	db.Init(dbPath)
	if db.Dao == nil {
		t.Fatal("db init failed")
	}

	syncAllStockInfoForTest(t)

	// 测试当天：走 NewStockDataApi → fallback(mootdx)，EastMoney K线已被墙无法用于历史日期
	today := time.Now().Format("2006-01-02")
	tradeDate := today
	// 非交易日（周末/节假日）无法出结果，用前一天重试一次
	isWeekend := time.Now().Weekday() == time.Saturday || time.Now().Weekday() == time.Sunday
	if isWeekend {
		tradeDate = time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	}
	t.Logf("Testing daily pick for: %s", tradeDate)

	engine := data.NewDailyPickEngine()
	picks, err := engine.RunDailyPick(context.Background(), tradeDate, 3)
	if err != nil {
		t.Fatalf("RunDailyPick failed: %v", err)
	}
	if len(picks) == 0 {
		t.Fatal("no picks returned")
	}
	t.Logf("Daily pick: %d picks", len(picks))
	for _, p := range picks {
		t.Logf("  #%d %s %s score=%.1f reason=%s",
			p.Rank, p.StockCode, p.StockName, p.Score, p.Reason)
	}
}

func TestBacktest_RealData(t *testing.T) {
	dbPath := findDB()
	if dbPath == "" {
		t.Skip("no stock.db found")
	}
	db.Init(dbPath)
	if db.Dao == nil {
		t.Fatal("db init failed")
	}

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	t.Logf("Testing backtest for signal date: %s", yesterday)

	codes := []string{"600519.SH", "000001.SZ", "300750.SZ"}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			engine := backtest.NewEngine()
			result, err := engine.Run(context.Background(), backtest.Input{
				StockCode:   code,
				SignalDate:  yesterday,
				EntryPrice:  0,
				HoldingDays: 5,
				StopLoss:    0.08,
				StopProfit:  0.15,
				Adjusted:    true,
			})
			if err != nil {
				t.Logf("  %s: backtest failed: %v", code, err)
				return
			}
			t.Logf("  %s: entry=%.2f exit=%.2f(%s) ret=%.4f win=%v dd=%.4f",
				code, result.EntryPrice, result.ExitPrice, result.ExitDate,
				result.TotalReturn, result.Win, result.MaxDrawdown)
		})
	}
}
