package datasource

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/data"
)

func TestKlineRowsToPort(t *testing.T) {
	t.Run("显式映射与PrevClose补算", func(t *testing.T) {
		rows := &[]data.KLineData{
			{Day: "2024-01-02", Open: "100", High: "105", Low: "99", Close: "103", Volume: "12345", Amount: "1270000.5"},
			{Day: "2024-01-03", Open: "103", High: "108", Low: "102", Close: "107", Volume: "15000.7", Amount: "1600000"},
			{Day: "2024-01-04", Open: "107", High: "109", Low: "101", Close: "bad", Volume: "", Amount: ""},
		}
		kd, err := klineRowsToPort("sh600519", "day", rows)
		if err != nil {
			t.Fatal(err)
		}
		if kd.Code != "sh600519" || kd.Period != "day" || len(kd.Bars) != 3 {
			t.Fatalf("meta: %+v", kd)
		}
		b0 := kd.Bars[0]
		if b0.Open != 100 || b0.High != 105 || b0.Low != 99 || b0.Close != 103 ||
			b0.Volume != 12345 || b0.Amount != 1270000.5 {
			t.Errorf("bar0: %+v", b0)
		}
		if b0.Time.IsZero() || b0.Time.Day() != 2 {
			t.Errorf("bar0 time: %v", b0.Time)
		}
		// 成交量兼容小数截断
		if kd.Bars[1].Volume != 15000 {
			t.Errorf("bar1 volume: %d, want 15000", kd.Bars[1].Volume)
		}
		// PrevClose 由前一根 Close 补算
		if kd.Bars[1].PrevClose != 103 || kd.Bars[2].PrevClose != 107 {
			t.Errorf("PrevClose: %v/%v", kd.Bars[1].PrevClose, kd.Bars[2].PrevClose)
		}
		if kd.Bars[0].PrevClose != 0 {
			t.Errorf("首根 PrevClose 应为 0: %v", kd.Bars[0].PrevClose)
		}
		// 非法数值容错为 0
		if kd.Bars[2].Close != 0 || kd.Bars[2].Volume != 0 {
			t.Errorf("bar2 容错: %+v", kd.Bars[2])
		}
	})

	t.Run("分钟K时间布局", func(t *testing.T) {
		rows := &[]data.KLineData{{Day: "2024-01-02 10:30", Close: "1"}}
		kd, err := klineRowsToPort("sh600519", "5", rows)
		if err != nil {
			t.Fatal(err)
		}
		if kd.Bars[0].Time.Hour() != 10 || kd.Bars[0].Time.Minute() != 30 {
			t.Errorf("分钟K时间: %v", kd.Bars[0].Time)
		}
	})

	t.Run("空输入返回errNoData", func(t *testing.T) {
		if _, err := klineRowsToPort("sh600519", "day", nil); !errors.Is(err, errNoData) {
			t.Errorf("nil: %v", err)
		}
		if _, err := klineRowsToPort("sh600519", "day", &[]data.KLineData{}); !errors.Is(err, errNoData) {
			t.Errorf("empty: %v", err)
		}
	})
}

func TestStockInfoToQuote(t *testing.T) {
	t.Run("字段映射与涨跌补算", func(t *testing.T) {
		q := stockInfoToQuote(&data.StockInfo{
			Code: "600519", Name: "贵州茅台",
			Price: "1688.50", PreClose: "1670.00",
			Open: "1671", High: "1690", Low: "1665",
			Volume: "23456", Amount: "39500000",
			Date: "2024-01-02", Time: "15:00:00",
			Bid: "1688.4", Ask: "1688.6", A1P: "1688.6", B1P: "1688.4", Market: "SH",
		})
		if q == nil {
			t.Fatal("nil quote")
		}
		if q.Code != "sh600519" || q.Name != "贵州茅台" {
			t.Errorf("code/name: %s/%s", q.Code, q.Name)
		}
		if q.Price != 1688.5 || q.PrevClose != 1670 || q.Open != 1671 || q.High != 1690 || q.Low != 1665 {
			t.Errorf("价格字段: %+v", q)
		}
		// 源头未填充二次计算字段时按 Price/PreClose 补算
		if q.Change != 18.5 {
			t.Errorf("Change: %v, want 18.5", q.Change)
		}
		if q.ChangePct < 1.10 || q.ChangePct > 1.11 {
			t.Errorf("ChangePct: %v, want ~1.108", q.ChangePct)
		}
		if q.Volume != 23456 || q.Amount != 39500000 {
			t.Errorf("量额: %v/%v", q.Volume, q.Amount)
		}
		if q.Time.Hour() != 15 {
			t.Errorf("time: %v", q.Time)
		}
		if q.Extra["ask1"] != 1688.6 || q.Extra["market"] != "SH" {
			t.Errorf("Extra: %v", q.Extra)
		}
	})

	t.Run("源头已填充时保留原值", func(t *testing.T) {
		q := stockInfoToQuote(&data.StockInfo{
			Code: "sh600519", Price: "100", PreClose: "90",
			ChangePrice: 9.5, ChangePercent: 10.5,
		})
		if q.Change != 9.5 || q.ChangePct != 10.5 {
			t.Errorf("应保留源头值: %v/%v", q.Change, q.ChangePct)
		}
	})

	t.Run("nil输入", func(t *testing.T) {
		if stockInfoToQuote(nil) != nil {
			t.Error("nil 应返回 nil")
		}
	})
}

func TestNormalizePeriod(t *testing.T) {
	for in, want := range map[string]string{"": "101", "day": "101", "DAILY": "101",
		"week": "102", "w": "102", "month": "103", "M": "103", "quarter": "104", "5": "5"} {
		if got := normalizePeriod(in); got != want {
			t.Errorf("normalizePeriod(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestAdapterGuardsWithoutNetwork(t *testing.T) {
	ctx := context.Background()

	t.Run("A股守卫_港美股提前拒绝", func(t *testing.T) {
		sina := NewSinaKLineProvider()
		if _, err := sina.GetKLine(ctx, "hk00700", "day", 10); !errors.Is(err, errOnlyAShare) {
			t.Errorf("sina hk: %v", err)
		}
		tdx := NewTdxKLineProvider()
		if _, err := tdx.GetKLine(ctx, "usAAPL", "day", 10); !errors.Is(err, errOnlyAShare) {
			t.Errorf("tdx us: %v", err)
		}
		tx := NewTencentKLineProvider()
		if _, err := tx.GetKLine(ctx, "00700.HK", "day", 10); !errors.Is(err, errOnlyAShare) {
			t.Errorf("tencent hk: %v", err)
		}
	})

	t.Run("db未初始化时配置错误而非panic", func(t *testing.T) {
		// 测试环境 db.Dao 为 nil：应返回 errConfigUnavailable 而不是 panic
		if _, err := NewEastMoneyKLineProvider().GetKLine(ctx, "sh600519", "day", 10); !errors.Is(err, errConfigUnavailable) {
			t.Errorf("eastmoney: %v", err)
		}
		if _, err := NewSinaKLineProvider().GetKLine(ctx, "sh600519", "day", 10); !errors.Is(err, errConfigUnavailable) {
			t.Errorf("sina: %v", err)
		}
		if _, err := NewTencentQuoteProvider().GetQuote(ctx, "sh600519"); !errors.Is(err, errConfigUnavailable) {
			t.Errorf("tencent quote: %v", err)
		}
	})

	t.Run("优先级声明", func(t *testing.T) {
		chain := []struct {
			name string
			prio int
		}{
			{NewTdxMACKLineProvider(0).Name(), 10},
			{NewEastMoneyKLineProvider().Name(), 20},
			{NewSinaKLineProvider().Name(), 30},
			{NewTencentKLineProvider().Name(), 40},
			{NewTdxKLineProvider().Name(), 50},
		}
		wantNames := []string{"tdx-mac", "eastmoney", "sina", "tencent", "tdx"}
		wantPrio := []int{10, 20, 30, 40, 50}
		for i, c := range chain {
			if c.name != wantNames[i] || c.prio != wantPrio[i] {
				t.Errorf("[%d] got %s/%d, want %s/%d", i, c.name, c.prio, wantNames[i], wantPrio[i])
			}
		}
	})
}
