package khunter

import (
	"context"
	"testing"

	"go-stock/backend/data/khunter/strategy"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestLoadDailyBarsAsOfExcludesFuture(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"2026-11-10", "2026-11-11", "2026-11-12"} {
		bar := models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: d,
			Adjusted: true, Open: 10, High: 11, Low: 9, Close: 10,
			Volume: 10000, Amount: 100000,
		}
		if err := db.Dao.Create(&bar).Error; err != nil {
			t.Fatal(err)
		}
	}
	bars, err := strategy.LoadDailyBarsAsOf(context.Background(), "600000", "2026-11-11", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 || bars[len(bars)-1].TradeDate != "2026-11-11" {
		t.Fatalf("历史加载包含未来数据: %+v", bars)
	}
}

func TestGetMoneyFlowAsOfUsesUpperBound(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	rows := []models.KhunterMoneyFlowDaily{
		{Code: "600000", Date: "2026-11-11", MainNet: 1},
		{Code: "600000", Date: "2026-11-12", MainNet: 999},
	}
	if err := r.UpsertMoneyFlow(rows); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetMoneyFlowAsOf("600000", "2026-11-11", 5)
	if err != nil || len(got) != 1 || got[0].MainNet != 1 {
		t.Fatalf("资金流越过评分日: rows=%+v err=%v", got, err)
	}
}
