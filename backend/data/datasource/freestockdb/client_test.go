package freestockdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetPointQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cmd") != "get" {
			t.Errorf("missing cmd=get, got %q", r.URL.RawQuery)
		}
		if r.URL.Query().Get("t") != "日k:600633:20260625" {
			t.Errorf("unexpected expr %q", r.URL.Query().Get("t"))
		}
		w.Write([]byte(`{"date":20260625,"close":10.62}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL[len("http://"):])
	raw, err := c.Get(context.Background(), "日k:600633:20260625")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if v["close"].(float64) != 10.62 {
		t.Errorf("close = %v, want 10.62", v["close"])
	}
}

func TestClientPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"0":["000001"]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL[len("http://"):])
	if !c.Ping(context.Background()) {
		t.Error("Ping should be true against live server")
	}
	bad := NewClient("127.0.0.1:1")
	if bad.Ping(context.Background()) {
		t.Error("Ping should be false against dead port")
	}
}

func TestDecodeValues(t *testing.T) {
	// 点查 object
	vals, err := decodeValues(json.RawMessage(`{"date":1}`))
	if err != nil || len(vals) != 1 {
		t.Fatalf("object: vals=%d err=%v", len(vals), err)
	}
	// 数组
	vals, err = decodeValues(json.RawMessage(`[{"date":1},{"date":2}]`))
	if err != nil || len(vals) != 2 {
		t.Fatalf("array: vals=%d err=%v", len(vals), err)
	}
	// [key, value] 对
	vals, err = decodeValues(json.RawMessage(`[["日k:600633:1",{"date":1}],["日k:600633:2",{"date":2}]]`))
	if err != nil || len(vals) != 2 {
		t.Fatalf("pairs: vals=%d err=%v", len(vals), err)
	}
	// null / 空
	vals, err = decodeValues(json.RawMessage(`null`))
	if err != nil || len(vals) != 0 {
		t.Fatalf("null: vals=%d err=%v", len(vals), err)
	}
}
