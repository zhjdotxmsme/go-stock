package khunter

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/util/timeutil"
)

func TestSupportPrice(t *testing.T) {
	bars := make([]models.KLineBar, 30)
	for i := range bars {
		bars[i] = models.KLineBar{Close: 10, Low: 9.5, TradeDate: "2026-09-01"}
	}
	// 全部收盘 10 → 支撑位 10
	if got := SupportPrice(bars); got != 10 {
		t.Fatalf("expect 10, got %v", got)
	}
	// 近 10 根涨到 20+，支撑位仍是窗口内最低收盘 10
	for i := 20; i < 30; i++ {
		bars[i].Close = 20 + float64(i)
		bars[i].Low = bars[i].Close - 0.5
	}
	if got := SupportPrice(bars); got != 10 {
		t.Fatalf("expect 10, got %v", got)
	}
	// 不足 20 根：取全部
	bars2 := make([]models.KLineBar, 5)
	for i := range bars2 {
		bars2[i].Close = 5 + float64(i)
	}
	if got := SupportPrice(bars2); got != 5 {
		t.Fatalf("expect 5, got %v", got)
	}
}

func TestHuntingThreshold(t *testing.T) {
	// 风险"危险"档 scoreExtra=15 → 阈值 60+15=75
	if got := HuntingThreshold(risk.Level{Name: "危险", PositionLimit: 0.4, ScoreExtra: 15}); got != 75 {
		t.Fatalf("expect 75, got %v", got)
	}
	if got := HuntingThreshold(risk.Level{Name: "正常", PositionLimit: 1.0, ScoreExtra: 0}); got != 60 {
		t.Fatalf("expect 60, got %v", got)
	}
}

func TestRunPipelineReportsLocalCoverageBreakdown(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}, &models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	infos := []models.AllStockInfo{
		{SECUCODE: "600001.SH", SECURITYCODE: "600001", SECURITYNAMEABBR: "覆盖正常"},
		{SECUCODE: "600002.SH", SECURITYCODE: "600002", SECURITYNAMEABBR: "历史较短"},
		{SECUCODE: "600003.SH", SECURITYCODE: "600003", SECURITYNAMEABBR: "当日停牌"},
	}
	for _, info := range infos {
		if err := db.Dao.Create(&info).Error; err != nil {
			t.Fatal(err)
		}
	}
	seed := func(code string, count int, end time.Time) {
		for i := range count {
			date := end.AddDate(0, 0, i-count+1)
			if err := db.Dao.Create(&models.KLineBar{
				StockCode: code, Period: "day", TradeDate: date.Format("2006-01-02"), Adjusted: true,
				Open: 10, High: 10, Low: 10, Close: 10, PrevClose: 10, Volume: 1000,
			}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	seed("sh600001", 30, time.Date(2026, 11, 6, 0, 0, 0, 0, time.Local))
	seed("sh600002", 10, time.Date(2026, 11, 6, 0, 0, 0, 0, time.Local))
	seed("sh600003", 30, time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local))

	result, err := RunPipeline(context.Background(), "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if result.StockTotal != 3 || result.FreshStocks != 1 || result.InsufficientHistory != 1 || result.MissingTradeDate != 1 || result.QueryFailed != 0 {
		t.Fatalf("覆盖统计不符: %+v", result)
	}
}

func TestResolveTradeDate(t *testing.T) {
	// 显式传入照用
	if got := resolveTradeDate("2026-09-24"); got != "2026-09-24" {
		t.Fatalf("explicit: %v", got)
	}
	// 空值：取本地日 K 最新交易日
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatalf("migrate kline: %v", err)
	}
	// 多数股票在最近完整交易日，离群一只在更近日期 → 应选覆盖面接近日峰值的交易日
	older := timeutil.DateStr(time.Now().AddDate(0, 0, -3))
	newer := timeutil.DateStr(time.Now().AddDate(0, 0, -1))
	outlier := timeutil.TodayStr()
	for _, code := range []string{"sh600001", "sh600002", "sh600003"} {
		db.Dao.Create(&models.KLineBar{StockCode: code, Period: "day", TradeDate: older, Adjusted: true, Close: 1})
	}
	db.Dao.Create(&models.KLineBar{StockCode: "sh600519", Period: "day", TradeDate: outlier, Adjusted: true, Close: 1})
	db.Dao.Create(&models.KLineBar{StockCode: "sh600519", Period: "week", TradeDate: newer, Adjusted: true, Close: 1})
	store := datasource.NewKLineStore()
	if got := store.LatestMarketTradeDate(context.Background(), 45, 1); got != older {
		t.Fatalf("expect 覆盖最广交易日 %s（非离群 MAX）, got %v", older, got)
	}
}
