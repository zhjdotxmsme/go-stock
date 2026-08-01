package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFactorStoreLoad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// [key, cum] 对；key = 复权:code:date
		w.Write([]byte(`[["复权:600633:20240101",1.0],["复权:600633:20250610",1.2],["复权:000001:20230101",1.0]]`))
	}))
	defer srv.Close()
	f := NewFactorStore()
	// 注：外层括号是本环境 Go 工具链的要求——if 初始化语句中的选择器切片需加括号才能解析。
	if err := f.Load(context.Background(), NewClient((srv.URL[len("http://"):]))); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, ok := f.factorLE("600633", "20260101"); !ok || got != 1.2 {
		t.Errorf("factorLE = %v,%v", got, ok)
	}
	if got, ok := f.factorLE("600633", "20240201"); !ok || got != 1.0 {
		t.Errorf("factorLE mid = %v,%v", got, ok)
	}
	if _, ok := f.factorLE("600633", "20230101"); ok {
		t.Error("before first factor should be !ok")
	}
}

func TestAdjustBarsQFQ(t *testing.T) {
	f := NewFactorStore()
	f.setFactors("600633", []string{"20240101", "20250610"}, []float64{1.0, 1.2})
	bars := []Bar{
		{Date: 20250105, Open: 12, High: 12.6, Low: 11.9, Close: 12.4, PreClose: 12.1, Volume: 100},
		{Date: 20250701, Open: 10, High: 10.5, Low: 9.9, Close: 10.3, PreClose: 10.1, Volume: 200},
	}
	out := f.AdjustBars("600633", bars, FQQFQ)
	// qfq: ratio = latest(1.2)/f_current；20250105 的 f_current=1.0 → 12.4/1.2≈10.33
	if out[0].Close != round(12.4/1.2, 2) {
		t.Errorf("qfq close = %v, want %v", out[0].Close, round(12.4/1.2, 2))
	}
	// 最后一根 ratio=1 不动
	if out[1].Close != 10.3 {
		t.Errorf("latest bar should be unchanged, got %v", out[1].Close)
	}
	if out[0].Volume != 100 {
		t.Error("volume must not be adjusted")
	}
}

func TestAdjustBarsHFQ(t *testing.T) {
	f := NewFactorStore()
	f.setFactors("600633", []string{"20240101", "20250610"}, []float64{1.0, 1.2})
	bars := []Bar{{Date: 20250105, Close: 12.4}}
	out := f.AdjustBars("600633", bars, FQHFQ)
	// hfq: ratio = 1/f_current = 1 → 不变（f_current=1.0）
	if out[0].Close != 12.4 {
		t.Errorf("hfq close = %v", out[0].Close)
	}
}

func TestAdjustBarsNoFactor(t *testing.T) {
	f := NewFactorStore()
	bars := []Bar{{Date: 20250105, Close: 12.4}}
	out := f.AdjustBars("999999", bars, FQQFQ)
	if out[0].Close != 12.4 {
		t.Error("no factor → passthrough")
	}
}

func TestFundDecimals(t *testing.T) {
	f := NewFactorStore()
	f.setFactors("510300", []string{"20240101", "20250610"}, []float64{1.0, 1.1})
	bars := []Bar{{Date: 20250105, Close: 4.4}}
	out := f.AdjustBars("510300", bars, FQQFQ)
	if out[0].Close != round(4.4/1.1, 3) {
		t.Errorf("fund should use 3 decimals: %v", out[0].Close)
	}
}
