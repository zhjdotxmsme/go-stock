package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFactorStoreLoad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// [key, cum] 对；key = 复权:code:date。故意乱序返回，验证 Load 的防御性排序。
		w.Write([]byte(`[["复权:600633:20250610",1.2],["复权:600633:20240101",1.0],["复权:000001:20230101",1.0]]`))
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
	if out[0].Close != 10.33 {
		t.Errorf("qfq close = %v, want %v", out[0].Close, 10.33)
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
	bars := []Bar{
		{Date: 20250105, Close: 12.4},
		{Date: 20250701, Close: 12.4},
	}
	out := f.AdjustBars("600633", bars, FQHFQ)
	// hfq: ratio = 1/f_current = 1 → 不变（f_current=1.0）
	if out[0].Close != 12.4 {
		t.Errorf("hfq close = %v", out[0].Close)
	}
	// 20250701 的 fc=1.2 → 12.4*1.2=14.88，覆盖实际折算路径
	if out[1].Close != 14.88 {
		t.Errorf("hfq adjusted close = %v, want 14.88", out[1].Close)
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

func TestAdjustBarsMismatchedArrays(t *testing.T) {
	fs := NewFactorStore()
	fs.setFactors("600000",
		[]string{"20250101", "20250601", "20251201"},
		[]float64{1.5, 1.3}) // 故意短一个
	bars := []Bar{
		{Date: 20251231, Close: 100, PreClose: 99},
	}
	// 不应 panic，且因长度不匹配跳过复权（返回原数据）
	result := fs.AdjustBars("600000", bars, FQQFQ)
	if len(result) != len(bars) {
		t.Fatalf("expected %d bars, got %d", len(bars), len(result))
	}
	if result[0].Close != bars[0].Close {
		t.Errorf("expected unadjusted close %v, got %v", bars[0].Close, result[0].Close)
	}
}

func TestFactorLEMismatchedArrays(t *testing.T) {
	fs := NewFactorStore()
	fs.setFactors("600000",
		[]string{"20250101", "20250601", "20251201"},
		[]float64{1.5, 1.3})
	// 不应 panic，返回 false
	if _, ok := fs.factorLE("600000", "20251201"); ok {
		t.Error("expected ok=false for mismatched arrays")
	}
}
