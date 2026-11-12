package khunter

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestGetTechProfileUsesTradeDateWindow(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	for i := range 30 {
		price := 10 + float64(i)*0.1
		date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, i)
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh600519", Period: "day", TradeDate: date.Format("2006-01-02"), Adjusted: true,
			Open: price - 0.05, High: price + 0.1, Low: price - 0.1, Close: price, Volume: 1000,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	profile, err := GetTechProfile(context.Background(), "600519.SH", "2026-01-30")
	if err != nil {
		t.Fatal(err)
	}
	if profile == nil || profile["trend"] == "趋势数据不足" {
		t.Fatalf("expected complete profile, got %#v", profile)
	}
	sections, _ := profile["sections"].([]models.TechSection)
	if len(sections) != 6 {
		t.Fatalf("expected six technical sections, got %d", len(sections))
	}

	after, err := GetTechProfile(context.Background(), "600519.SH", "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if after == nil || after["momentum"] != "动能数据不足" {
		t.Fatalf("推荐日之前只用单根K线，动能应样本不足，got %#v", after)
	}
}

func TestBuildTechProfileSections(t *testing.T) {
	bars := make([]models.KLineBar, 80)
	for i := range bars {
		c := 10 + float64(i)*0.08
		bars[i] = models.KLineBar{
			TradeDate: "2024-01-02",
			Open:      c - 0.05,
			High:      c + 0.12,
			Low:       c - 0.10,
			Close:     c,
			Volume:    int64(100000 + i*500),
		}
	}
	profile := BuildTechProfile(bars)
	sections, _ := profile["sections"].([]models.TechSection)
	if len(sections) != 6 {
		t.Fatalf("sections: %#v", profile["sections"])
	}
	var b strings.Builder
	for _, s := range sections {
		b.WriteString(s.Title)
		b.WriteString(s.Text)
	}
	text := b.String()
	for _, want := range []string{"RSI(14)", "MACD", "KDJ", "布林", "ATR", "回撤", "综合"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestMergeTechProfileKeepsStrategies(t *testing.T) {
	bars := make([]models.KLineBar, 40)
	for i := range bars {
		c := 20 + float64(i)*0.02
		bars[i] = models.KLineBar{Open: c, High: c + 0.1, Low: c - 0.1, Close: c, Volume: 1000}
	}
	raw := `{"technical":{"strategies":[{"name":"仙人指路","weight":20}]}}`
	merged := mergeTechProfile(raw, BuildTechProfile(bars))
	if !strings.Contains(merged, "仙人指路") || !strings.Contains(merged, "RSI") {
		t.Fatalf("merge dropped content: %s", merged)
	}
	if !profileHasSections(merged) {
		t.Fatal("merged profile should expose sections")
	}
}
