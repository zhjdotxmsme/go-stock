package freestockdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"go-stock/backend/data/datasource"
)

func newTestProvider(t *testing.T, dayResp string) *Provider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expr := r.URL.Query().Get("t")
		switch {
		case strings.HasPrefix(expr, "日k"):
			fmt.Fprint(w, dayResp)
		case strings.HasPrefix(expr, "板块"):
			fmt.Fprint(w, boardFixture)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	t.Cleanup(srv.Close)
	addr := srv.URL[len("http://"):]
	m := NewManager(Config{Enabled: true, Addr: addr})
	c := m.Client()
	f := NewFactorStore()
	bi := NewBoardIndex()
	if err := bi.Load(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return NewProvider(m, NewKLineService(c, f), bi)
}

func TestProviderInterface(t *testing.T) {
	p := newTestProvider(t, `[]`)
	if p.Name() != "freestockdb" || p.Priority() != 1 {
		t.Errorf("name=%s priority=%d", p.Name(), p.Priority())
	}
}

func TestProviderGetKLine(t *testing.T) {
	p := newTestProvider(t,
		`[{"date":20260624,"open":10.8,"high":11,"low":10.7,"close":10.83,"volume":233130000,"code":"600633"},
		  {"date":20260625,"open":10.45,"high":10.62,"low":10.37,"close":10.45,"volume":18031500,"code":"600633"}]`)
	kd, err := p.GetKLine(context.Background(), "600633", "day", 2)
	if err != nil {
		t.Fatalf("GetKLine: %v", err)
	}
	if len(kd.Bars) != 2 || kd.Bars[1].Close != 10.45 {
		t.Fatalf("bars = %+v", kd.Bars)
	}
	if kd.Bars[0].Time.Format("2006-01-02") != "2026-06-24" {
		t.Errorf("bar time = %v", kd.Bars[0].Time)
	}
	if kd.Bars[0].Volume != 233130000/100 { // freestockdb 单位为股，链上口径为手
		t.Errorf("volume = %d", kd.Bars[0].Volume)
	}
}

func TestProviderGetKLineEmpty(t *testing.T) {
	p := newTestProvider(t, `[]`)
	if _, err := p.GetKLine(context.Background(), "999999", "day", 10); err == nil {
		t.Error("empty result must return error (router fallback contract)")
	}
}

func TestProviderGetQuote(t *testing.T) {
	p := newTestProvider(t,
		`[{"date":20260625,"open":10.45,"high":10.62,"low":10.37,"close":10.45,"pre_close":10.52,"pct_chg":-0.67,"volume":18031500,"amount":189010000,"code":"600633","name":"浙数文化"}]`)
	q, err := p.GetQuote(context.Background(), "600633")
	if err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	if q.Price != 10.45 || q.PrevClose != 10.52 || q.ChangePct != -0.67 || q.Name != "浙数文化" {
		t.Errorf("quote = %+v", q)
	}
}

func TestProviderGetSectorData(t *testing.T) {
	p := newTestProvider(t, `[]`)
	sd, err := p.GetSectorData(context.Background(), "600633")
	if err != nil {
		t.Fatalf("GetSectorData: %v", err)
	}
	if sd.Sector != "5G" { // boardFixture 中 600633 的第一个概念
		t.Errorf("sector = %q", sd.Sector)
	}
}

// TestSetupRegistersProviders 验证 Setup 同步完成 kline/sector 两条链注册（Enabled=false 时
// 引擎不启动，但 Provider 必须已在链上，Available=false 由 Router 自然跳过）。
// 日内实时报价仍由东财/腾讯链承担，freestockdb 不注册 quote 链（规格 §5.5）。
func TestSetupRegistersProviders(t *testing.T) {
	router := &datasource.Router{}
	m := Setup(router, Config{Enabled: false})
	if m == nil {
		t.Fatal("Setup returned nil Manager")
	}
	rv := reflect.ValueOf(router).Elem()
	for _, field := range []string{"klineProviders", "sectorProviders"} {
		if n := rv.FieldByName(field).Len(); n != 1 {
			t.Errorf("%s len = %d, want 1", field, n)
		}
	}
	if n := rv.FieldByName("quoteProviders").Len(); n != 0 {
		t.Errorf("quoteProviders len = %d, want 0", n)
	}
}
