package freestockdb

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeStockDB 按前缀路由返回固定数据，模拟 K-V 服务。
func fakeStockDB(t *testing.T, dayResp, minResp string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expr := r.URL.Query().Get("t")
		switch {
		case strings.HasPrefix(expr, "日k"):
			fmt.Fprint(w, dayResp)
		default:
			fmt.Fprint(w, minResp)
		}
	}))
}

func TestServiceGetDataDailyQFQ(t *testing.T) {
	srv := fakeStockDB(t,
		`[{"date":20250105,"open":12,"high":12.6,"low":11.9,"close":12.4,"pre_close":12.1,"volume":100,"code":"600633"},
		  {"date":20250701,"open":10,"high":10.5,"low":9.9,"close":10.3,"pre_close":10.1,"volume":200,"code":"600633"}]`,
		`[]`)
	defer srv.Close()
	f := NewFactorStore()
	f.setFactors("600633", []string{"20240101", "20250610"}, []float64{1.0, 1.2})
	svc := NewKLineService(NewClient(srv.URL[len("http://"):]), f)

	bars, err := svc.GetData(context.Background(), "600633", Freq1d, "", "", FQQFQ, false, 0)
	if err != nil || len(bars) != 2 {
		t.Fatalf("bars=%d err=%v", len(bars), err)
	}
	if bars[0].Close != round(12.4/1.2, 2) {
		t.Errorf("qfq close = %v", bars[0].Close)
	}
}

func TestServiceLastNWeekly(t *testing.T) {
	srv := fakeStockDB(t,
		`[{"date":20260622,"open":10,"high":11,"low":9.5,"close":10.5,"volume":100,"code":"600633"},
		  {"date":20260623,"open":10.5,"high":11.2,"low":10.4,"close":11.0,"volume":200,"code":"600633"},
		  {"date":20260629,"open":11,"high":11.5,"low":10.8,"close":11.3,"volume":150,"code":"600633"}]`,
		`[]`)
	defer srv.Close()
	svc := NewKLineService(NewClient(srv.URL[len("http://"):]), NewFactorStore())

	bars, err := svc.LastN(context.Background(), "600633", Freq1w, 1, FQNone)
	if err != nil || len(bars) != 1 {
		t.Fatalf("bars=%d err=%v", len(bars), err)
	}
	if bars[0].Close != 11.3 || bars[0].Date != 20260629 {
		t.Errorf("last week = %+v", bars[0])
	}
}

func TestServiceLastNDailyTail(t *testing.T) {
	srv := fakeStockDB(t,
		`[{"date":20260620,"close":1,"code":"600633"},{"date":20260621,"close":2,"code":"600633"},{"date":20260622,"close":3,"code":"600633"}]`,
		`[]`)
	defer srv.Close()
	svc := NewKLineService(NewClient(srv.URL[len("http://"):]), NewFactorStore())
	bars, err := svc.LastN(context.Background(), "600633", Freq1d, 2, FQNone)
	if err != nil || len(bars) != 2 || bars[0].Date != 20260621 || bars[1].Date != 20260622 {
		t.Fatalf("bars=%+v err=%v", bars, err)
	}
}

func TestGoldenCompare(t *testing.T) {
	if os.Getenv("STOCKDB_ADDR") == "" {
		t.Skip("set STOCKDB_ADDR=127.0.0.1:7899 to run golden compare")
	}
	golden, err := os.ReadFile("testdata/golden_600633.json")
	if err != nil {
		t.Skip("golden file missing; run testdata/gen_golden.py first")
	}
	var ref map[string][]map[string]interface{}
	if err := json.Unmarshal(golden, &ref); err != nil {
		t.Fatal(err)
	}
	f := NewFactorStore()
	c := NewClient(os.Getenv("STOCKDB_ADDR"))
	if err := f.Load(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	svc := NewKLineService(c, f)
	compareGolden(t, svc, ref, Freq1d, "day_qfq", "20250101", "20260701")
	compareGolden(t, svc, ref, Freq1w, "week_qfq", "20250101", "20260701")
	compareGolden(t, svc, ref, Freq30m, "30m_qfq", "20260625", "20260626")
}

// compareGolden 对拍一组黄金数据：先比行数，再逐行比 date 与 OHLC（容差 1e-3）。
func compareGolden(t *testing.T, svc *KLineService, ref map[string][]map[string]interface{}, freq Frequency, goldenKey, start, end string) {
	t.Helper()
	bars, err := svc.GetData(context.Background(), "600633", freq, start, end, FQQFQ, false, 0)
	if err != nil {
		t.Fatalf("%s: %v", goldenKey, err)
	}
	refRows := ref[goldenKey]
	if len(bars) != len(refRows) {
		t.Fatalf("%s row count %d != golden %d", goldenKey, len(bars), len(refRows))
	}
	for i, b := range bars {
		if int64(refRows[i]["date"].(float64)) != b.Date {
			t.Fatalf("%s row %d date %v != golden %v", goldenKey, i, b.Date, refRows[i]["date"])
		}
		for field, got := range map[string]float64{"open": b.Open, "high": b.High, "low": b.Low, "close": b.Close} {
			rc, _ := refRows[i][field].(float64)
			if math.Abs(got-rc) > 1e-3 {
				t.Fatalf("%s row %d %s %v != golden %v", goldenKey, i, field, got, rc)
			}
		}
	}
}
