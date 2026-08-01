package freestockdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if p.Name() != "freestockdb" || p.Priority() != 5 {
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
	if kd.Bars[0].Volume != 233130000 {
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
