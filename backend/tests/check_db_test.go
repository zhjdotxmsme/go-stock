package tests

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestCheckDB(t *testing.T) {
	dbPath := findDB()
	if dbPath == "" {
		t.Skip("no stock.db found")
	}
	db.Init(dbPath)

	var infoCount int64
	db.Dao.Model(&models.AllStockInfo{}).Count(&infoCount)
	t.Logf("all_stock_info: %d rows", infoCount)

	var pickCount int64
	db.Dao.Model(&models.DailyPick{}).Count(&pickCount)
	t.Logf("daily_picks: %d rows", pickCount)

	var barCount int64
	db.Dao.Model(&models.KLineBar{}).Count(&barCount)
	t.Logf("kline_bars: %d rows", barCount)

	// Show some all_stock_info samples if available
	if infoCount > 0 {
		var samples []models.AllStockInfo
		db.Dao.Select("secucode, securitynameabbr").Limit(5).Find(&samples)
		for _, s := range samples {
			t.Logf("  stock: %s %s", s.SECUCODE, s.SECURITYNAMEABBR)
		}
	}

	// Show daily pick samples
	if pickCount > 0 {
		var latest models.DailyPick
		db.Dao.Order("trade_date DESC").First(&latest)
		t.Logf("  latest daily pick: %s %s score=%.1f", latest.StockCode, latest.StockName, latest.Score)

		// Count by date
		type dateCount struct {
			TradeDate string
			Cnt       int
		}
		var dateStats []dateCount
		db.Dao.Model(&models.DailyPick{}).
			Select("trade_date, COUNT(*) as cnt").
			Group("trade_date").
			Order("trade_date DESC").
			Limit(5).
			Scan(&dateStats)
		for _, ds := range dateStats {
			t.Logf("    %s: %d picks", ds.TradeDate, ds.Cnt)
		}
	}
}
