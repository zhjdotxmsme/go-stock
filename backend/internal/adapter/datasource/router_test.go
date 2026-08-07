package datasource

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	portds "go-stock/backend/internal/port/datasource"
)

// mockProvider mock 数据源：记录调用顺序，按预设返回结果/错误。
type mockProvider struct {
	name      string
	priority  int
	available bool
	quote     *portds.QuoteData
	kline     *portds.KLineData
	err       error
	calls     *[]string // 共享调用记录
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Priority() int                       { return m.priority }
func (m *mockProvider) Available(ctx context.Context) bool  { return m.available }
func (m *mockProvider) record() {
	if m.calls != nil {
		*m.calls = append(*m.calls, m.name)
	}
}
func (m *mockProvider) GetQuote(ctx context.Context, code string) (*portds.QuoteData, error) {
	m.record()
	return m.quote, m.err
}
func (m *mockProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	m.record()
	return m.kline, m.err
}

func okKLine() *portds.KLineData {
	return &portds.KLineData{Code: "sh600519", Period: "day",
		Bars: []portds.KLineBar{{Close: 100}}}
}

func TestRouterRegisterSortsByPriority(t *testing.T) {
	r := NewRouter()
	r.Register(
		&mockProvider{name: "c", priority: 30, available: true},
		&mockProvider{name: "a", priority: 10, available: true},
		&mockProvider{name: "b", priority: 20, available: true},
	)
	for i, want := range []string{"a", "b", "c"} {
		if got := r.KLineProviders()[i].Name(); got != want {
			t.Errorf("kline chain[%d]: got %s, want %s", i, got, want)
		}
		if got := r.QuoteProviders()[i].Name(); got != want {
			t.Errorf("quote chain[%d]: got %s, want %s", i, got, want)
		}
	}
}

func TestRouterKLineFallback(t *testing.T) {
	t.Run("首个成功不fallback", func(t *testing.T) {
		var calls []string
		r := NewRouter()
		r.Register(
			&mockProvider{name: "p1", priority: 10, available: true, kline: okKLine(), calls: &calls},
			&mockProvider{name: "p2", priority: 20, available: true, kline: okKLine(), calls: &calls},
		)
		kd, err := r.GetKLine(context.Background(), "600519", "day", 10)
		if err != nil || kd == nil {
			t.Fatalf("err=%v kd=%v", err, kd)
		}
		if len(calls) != 1 || calls[0] != "p1" {
			t.Errorf("calls=%v, want [p1]", calls)
		}
	})

	t.Run("失败后按优先级fallback", func(t *testing.T) {
		var calls []string
		r := NewRouter()
		r.Register(
			&mockProvider{name: "p1", priority: 10, available: true, err: errors.New("boom"), calls: &calls},
			&mockProvider{name: "p2", priority: 20, available: true, kline: &portds.KLineData{}, calls: &calls}, // 空数据
			&mockProvider{name: "p3", priority: 30, available: false, kline: okKLine(), calls: &calls},          // 不可用跳过
			&mockProvider{name: "p4", priority: 40, available: true, kline: okKLine(), calls: &calls},
		)
		kd, err := r.GetKLine(context.Background(), "sh600519", "day", 10)
		if err != nil || kd == nil || len(kd.Bars) != 1 {
			t.Fatalf("err=%v kd=%v", err, kd)
		}
		want := []string{"p1", "p2", "p4"}
		if len(calls) != len(want) {
			t.Fatalf("calls=%v, want %v", calls, want)
		}
		for i := range want {
			if calls[i] != want[i] {
				t.Errorf("calls=%v, want %v", calls, want)
			}
		}
	})

	t.Run("全部失败返回聚合错误", func(t *testing.T) {
		r := NewRouter()
		r.Register(
			&mockProvider{name: "p1", priority: 10, available: true, err: errors.New("boom")},
			&mockProvider{name: "p2", priority: 20, available: true, kline: &portds.KLineData{}},
		)
		_, err := r.GetKLine(context.Background(), "sh600519", "day", 10)
		if err == nil {
			t.Fatal("应返回错误")
		}
		msg := err.Error()
		for _, s := range []string{"p1", "p2", "boom", "无数据"} {
			if !strings.Contains(msg, s) {
				t.Errorf("聚合错误应包含 %q: %s", s, msg)
			}
		}
	})

	t.Run("代码归一化", func(t *testing.T) {
		var gotCode string
		mp := &mockProvider{name: "p1", priority: 10, available: true, kline: okKLine()}
		r := NewRouter()
		r.Register(&codeCapturer{mockProvider: mp, dst: &gotCode})
		if _, err := r.GetKLine(context.Background(), "600519.SH", "day", 10); err != nil {
			t.Fatal(err)
		}
		if gotCode != "sh600519" {
			t.Errorf("code: got %q, want sh600519", gotCode)
		}
	})
}

// codeCapturer 捕获下游收到的归一化代码。
type codeCapturer struct {
	*mockProvider
	dst *string
}

func (c *codeCapturer) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	*c.dst = code
	return c.mockProvider.GetKLine(ctx, code, period, count)
}

func TestRouterQuoteFallback(t *testing.T) {
	var calls []string
	r := NewRouter()
	r.Register(
		&mockProvider{name: "q1", priority: 10, available: true, err: errors.New("timeout"), calls: &calls},
		&mockProvider{name: "q2", priority: 20, available: true,
			quote: &portds.QuoteData{Code: "sh600519", Price: 1688.5, Time: time.Now()}, calls: &calls},
	)
	q, err := r.GetQuote(context.Background(), "600519")
	if err != nil || q == nil || q.Price != 1688.5 {
		t.Fatalf("err=%v q=%v", err, q)
	}
	if len(calls) != 2 || calls[0] != "q1" || calls[1] != "q2" {
		t.Errorf("calls=%v, want [q1 q2]", calls)
	}
}

func TestRouterEmptyChain(t *testing.T) {
	r := NewRouter()
	if _, err := r.GetQuote(context.Background(), "sh600519"); err == nil {
		t.Error("无 provider 时应返回错误")
	}
	if _, err := r.GetKLine(context.Background(), "sh600519", "day", 10); err == nil {
		t.Error("无 provider 时应返回错误")
	}
	if _, err := r.GetQuote(context.Background(), "  "); err == nil {
		t.Error("空代码应返回错误")
	}
}

func TestDefaultRouterChainOrder(t *testing.T) {
	r := NewDefaultRouter()
	var names []string
	for _, p := range r.KLineProviders() {
		names = append(names, p.Name())
	}
	want := []string{"tdx-mac", "eastmoney", "sina", "tencent", "tdx"}
	if len(names) != len(want) {
		t.Fatalf("kline chain=%v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("kline chain=%v, want %v", names, want)
		}
	}
	if len(r.QuoteProviders()) != 1 || r.QuoteProviders()[0].Name() != "tencent" {
		t.Errorf("quote chain 应只有 tencent: %v", r.QuoteProviders())
	}
}
