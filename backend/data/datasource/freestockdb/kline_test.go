package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildTimeExpr(t *testing.T) {
	cases := []struct {
		start, end string
		desc       bool
		want       string
	}{
		{"", "", false, "*"},
		{"20260625", "", false, "20260625"},
		{"20260625", "20260625", true, "20260625"},
		{"20260620", "20260626", false, "20260620>20260626"},
		{"20260620", "20260626", true, "20260620<20260626"},
		{"", "20260626", false, "N>20260626"},
		{"20260620", "", true, "20260620<N"},
	}
	for _, c := range cases {
		if got := BuildTimeExpr(c.start, c.end, c.desc); got != c.want {
			t.Errorf("BuildTimeExpr(%q,%q,%v) = %q, want %q", c.start, c.end, c.desc, got, c.want)
		}
	}
}

func TestPadMinuteTime(t *testing.T) {
	if got := PadMinuteTime("20260625", false); got != "20260625000000" {
		t.Errorf("start pad = %q", got)
	}
	if got := PadMinuteTime("20260625", true); got != "20260625235959" {
		t.Errorf("end pad = %q", got)
	}
	if got := PadMinuteTime("20260625145200", false); got != "20260625145200" {
		t.Errorf("14-digit passthrough = %q", got)
	}
}

func TestQueryDayK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"date":20260624,"open":10.8,"high":11.0,"low":10.7,"close":10.83,"pre_close":10.9,"volume":233130000,"amount":2500000000,"code":"600633","name":"浙数文化"},
			{"date":20260625,"open":10.45,"high":10.62,"low":10.37,"close":10.45,"pre_close":10.83,"volume":18031500,"amount":189010000,"code":"600633","name":"浙数文化"}
		]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL[len("http://"):])
	bars, err := c.QueryDayK(context.Background(), "600633", "20260624", "20260625", false)
	if err != nil {
		t.Fatalf("QueryDayK: %v", err)
	}
	if len(bars) != 2 || bars[1].Close != 10.45 || bars[0].PreClose != 10.9 {
		t.Errorf("bars = %+v", bars)
	}
}

func TestQueryMinuteK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"date":20260625145200,"open":7.95,"high":7.96,"low":7.94,"close":7.95,"volume":53900,"amount":428554,"code":"600422"}]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL[len("http://"):])
	bars, err := c.QueryMinuteK(context.Background(), "600422", "20260625", "20260625", false)
	if err != nil || len(bars) != 1 || bars[0].Date != 20260625145200 {
		t.Fatalf("bars=%+v err=%v", bars, err)
	}
}
