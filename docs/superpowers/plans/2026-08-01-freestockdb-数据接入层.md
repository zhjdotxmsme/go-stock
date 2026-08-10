# free-stockdb 本地数据底座接入 · 实施计划（一）：数据接入层

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 free-stockdb 本地行情引擎（HTTP K-V，127.0.0.1:7899）接入 go-stock 数据源统一路由层，作为最高优先级（priority=5）的 K线/报价/板块数据源，并支持配置路径自动拉起进程。

**Architecture:** 新增内聚包 `backend/data/datasource/freestockdb/`（HTTP client → K线语义层[查询/复权/聚合] → BoardIndex → Provider → Manager），Provider 实现现有 `datasource.KLineProvider/QuoteProvider/SectorProvider` 接口注册进 Router；Router 的缓存、SQLite 持久化、TDX→东财降级链路零改动。指标引擎（39 指标 Go 移植）是**独立计划（二）**，不在本计划内。

**Tech Stack:** Go 1.x（stdlib only，新包不引入第三方依赖）、现有 `datasource` Router、GORM Settings 表、Vue3 + naive-ui 设置页。

**Spec:** `.openteams/specs/2026-08-01-freestockdb-底座接入-design.html`

## Global Constraints

- Go module 路径为 `go-stock`；新包只准用标准库 + `go-stock/backend/data/datasource` + `go-stock/backend/logger`。
- free-stockdb 默认地址 `127.0.0.1:7899`；Provider `Priority()` 固定返回 `5`，`Name()` 固定返回 `"freestockdb"`。
- 复权默认 qfq；股票保留 2 位小数，基金（代码 1/5 开头）保留 3 位。
- 语义移植严格对照上游 `pybao/stock_sdk.py` 与 `pybao/zhibiao.py`（见 spec 第 2、5 节）。
- Router、KLineStore、fallback 链、前端 K线图直连路径：**一律不改**。
- 测试一律 stdlib `testing` + `net/http/httptest`，运行命令 `go test ./backend/data/datasource/freestockdb/ -v`。
- 每个 Task 结束独立可测、独立可提交。

---

### Task 1: HTTP K-V Client

**Files:**
- Create: `backend/data/datasource/freestockdb/client.go`
- Test: `backend/data/datasource/freestockdb/client_test.go`

**Interfaces:**
- Consumes: 无（第一个任务）
- Produces: `NewClient(addr string) *Client`；`(*Client).Get(ctx, expr string) (json.RawMessage, error)`；`(*Client).Ping(ctx) bool`；`decodeValues(raw json.RawMessage) ([]json.RawMessage, error)`（Task 2/3/6 复用）

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: FAIL — `package go-stock/backend/data/datasource/freestockdb: no Go files`

- [ ] **Step 3: Write minimal implementation**

```go
// Package freestockdb 实现 free-stockdb 本地行情引擎的接入。
// 引擎是 HTTP K-V 服务：GET http://<addr>/?cmd=get&t=<键表达式>。
package freestockdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client 是 free-stockdb HTTP K-V 服务的客户端。
type Client struct {
	addr string
	hc   *http.Client
}

// NewClient 创建客户端，addr 形如 "127.0.0.1:7899"。
func NewClient(addr string) *Client {
	return &Client{addr: addr, hc: &http.Client{Timeout: 10 * time.Second}}
}

// Get 执行 K-V 查询。expr 例如 "日k:600633:20260620>20260626"。
// 返回原始 JSON：点查为 object，范围/通配为 array（可能是 [key,value] 对）。
func (c *Client) Get(ctx context.Context, expr string) (json.RawMessage, error) {
	u := fmt.Sprintf("http://%s/?cmd=get&t=%s", c.addr, url.QueryEscape(expr))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stockdb: HTTP %d for %q", resp.StatusCode, expr)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

// Ping 探测服务是否可用（2s 超时）。
func (c *Client) Ping(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := c.Get(ctx, "股票代码")
	return err == nil
}

// decodeValues 把 K-V 响应统一拆成值列表：
// 点查 object → [object]；[object,...] → 原样；[[key,value],...] → 取每对的 value。
func decodeValues(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	switch trimmed[0] {
	case '{':
		return []json.RawMessage{json.RawMessage(trimmed)}, nil
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("stockdb: decode array: %w", err)
		}
		out := make([]json.RawMessage, 0, len(items))
		for _, it := range items {
			it = bytes.TrimSpace(it)
			if len(it) == 0 {
				continue
			}
			if it[0] == '[' {
				var pair []json.RawMessage
				if err := json.Unmarshal(it, &pair); err == nil && len(pair) == 2 {
					out = append(out, pair[1])
					continue
				}
			}
			out = append(out, it)
		}
		return out, nil
	}
	return nil, fmt.Errorf("stockdb: unexpected payload: %.64s", trimmed)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS（3 个测试）

- [ ] **Step 5: Commit**

```bash
git add backend/data/datasource/freestockdb/client.go backend/data/datasource/freestockdb/client_test.go
git commit -m "feat(datasource): add free-stockdb HTTP K-V client"
```

---

### Task 2: K线查询层（类型 + 键表达式 + 日K/分钟K 查询）

**Files:**
- Create: `backend/data/datasource/freestockdb/kline.go`
- Test: `backend/data/datasource/freestockdb/kline_test.go`

**Interfaces:**
- Consumes: `(*Client).Get`、`decodeValues`（Task 1）
- Produces: `DayKBar`、`MinuteKBar`、`BuildTimeExpr(start, end string, desc bool) string`、`PadMinuteTime(s string, isEnd bool) string`、`(*Client).QueryDayK(ctx, code, start, end string, desc bool) ([]DayKBar, error)`、`(*Client).QueryMinuteK(...) ([]MinuteKBar, error)`

- [ ] **Step 1: Write the failing test**

```go
package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildTimeExpr(t *testing.T) {
	cases := []struct{ start, end string; desc bool; want string }{
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestBuildTimeExpr|TestPadMinuteTime|TestQueryDayK|TestQueryMinuteK' -v`
Expected: FAIL — undefined: BuildTimeExpr 等

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"context"
	"encoding/json"
)

// DayKBar 对应服务端 "日k" 记录。字段名与上游一致。
type DayKBar struct {
	Date     int64   `json:"date"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	PreClose float64 `json:"pre_close"`
	Volume   float64 `json:"volume"`
	Amount   float64 `json:"amount"`
	PctChg   float64 `json:"pct_chg"`
	Turnover float64 `json:"turnover"`
	VolRatio float64 `json:"vol_ratio"`
	PeTTM    float64 `json:"pe_ttm"`
	Pb       float64 `json:"pb"`
	TotalMv  float64 `json:"total_mv"`
	FloatMv  float64 `json:"float_mv"`
	IsST     bool    `json:"is_st"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
}

// MinuteKBar 对应服务端 "分钟k" 记录（1 分钟粒度，date 为 14 位 YYYYMMDDHHMMSS）。
type MinuteKBar struct {
	Date   int64   `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
	Code   string  `json:"code"`
}

// BuildTimeExpr 构造时间键表达式，对照 stock_sdk._build_time_query。
// "N" 表示最新记录。
func BuildTimeExpr(start, end string, desc bool) string {
	if start == "" && end == "" {
		return "*"
	}
	if start != "" && (end == "" || start == end) {
		return start
	}
	op := ">"
	if desc {
		op = "<"
	}
	if start == "" {
		start = "N"
	}
	if end == "" {
		end = "N"
	}
	return start + op + end
}

// PadMinuteTime 把 8 位日期补全为 14 位分钟K时间戳。
func PadMinuteTime(s string, isEnd bool) string {
	if len(s) != 8 {
		return s
	}
	if isEnd {
		return s + "235959"
	}
	return s + "000000"
}

// QueryDayK 查询日K，按服务端返回顺序（LevelDB 天然按日期升序）。
func (c *Client) QueryDayK(ctx context.Context, code, start, end string, desc bool) ([]DayKBar, error) {
	raw, err := c.Get(ctx, "日k:"+code+":"+BuildTimeExpr(start, end, desc))
	if err != nil {
		return nil, err
	}
	vals, err := decodeValues(raw)
	if err != nil {
		return nil, err
	}
	bars := make([]DayKBar, 0, len(vals))
	for _, v := range vals {
		var b DayKBar
		if err := json.Unmarshal(v, &b); err != nil || b.Date == 0 {
			continue
		}
		bars = append(bars, b)
	}
	return bars, nil
}

// QueryMinuteK 查询分钟K；8 位日期自动补全为当天完整交易时段。
func (c *Client) QueryMinuteK(ctx context.Context, code, start, end string, desc bool) ([]MinuteKBar, error) {
	raw, err := c.Get(ctx, "分钟k:"+code+":"+BuildTimeExpr(PadMinuteTime(start, false), PadMinuteTime(end, true), desc))
	if err != nil {
		return nil, err
	}
	vals, err := decodeValues(raw)
	if err != nil {
		return nil, err
	}
	bars := make([]MinuteKBar, 0, len(vals))
	for _, v := range vals {
		var b MinuteKBar
		if err := json.Unmarshal(v, &b); err != nil || b.Date == 0 {
			continue
		}
		bars = append(bars, b)
	}
	return bars, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS（Task 1 + Task 2 全部）

- [ ] **Step 5: Commit**

```bash
git add backend/data/datasource/freestockdb/kline.go backend/data/datasource/freestockdb/kline_test.go
git commit -m "feat(datasource): free-stockdb day/minute kline query layer"
```

---

### Task 3: 复权因子缓存与折算

**Files:**
- Create: `backend/data/datasource/freestockdb/factor.go`
- Test: `backend/data/datasource/freestockdb/factor_test.go`

**Interfaces:**
- Consumes: `(*Client).Get`、`decodeValues`（Task 1）；`Bar`、`FQ`、`FQQFQ/FQHFQ/FQNone`（本任务定义，Task 4/5 使用）
- Produces: `Bar` 统一K线结构；`NewFactorStore() *FactorStore`；`(*FactorStore).Load(ctx, c *Client) error`；`(*FactorStore).AdjustBars(code string, bars []Bar, fq FQ) []Bar`；`round(v float64, decimals int) float64`

- [ ] **Step 1: Write the failing test**

```go
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
	if err := f.Load(context.Background(), NewClient(srv.URL[len("http://"):]); err != nil {
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestFactor|TestAdjust|TestFundDecimals' -v`
Expected: FAIL — undefined: NewFactorStore 等

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// FQ 复权类型。
type FQ string

const (
	FQNone FQ = ""
	FQQFQ  FQ = "qfq"
	FQHFQ  FQ = "hfq"
)

// Bar 是统一K线结构（日K/分钟K/聚合K线共用）。Date：日K 8 位，分钟K 14 位。
type Bar struct {
	Date     int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
	PreClose float64
	Volume   float64
	Amount   float64
	Turnover float64
	VolRatio float64
	PeTTM    float64
	Pb       float64
	TotalMv  float64
	FloatMv  float64
	IsST     bool
	Code     string
	Name     string
}

// FactorStore 缓存 "复权*" 的 cum 累计因子（对照 stock_sdk 的预加载逻辑）。
type FactorStore struct {
	mu    sync.RWMutex
	dates map[string][]string  // code → 除权日期（升序）
	cums  map[string][]float64 // code → 对应 cum 因子
}

func NewFactorStore() *FactorStore {
	return &FactorStore{dates: map[string][]string{}, cums: map[string][]float64{}}
}

// Load 一次性拉取全部复权因子。服务端返回 [[key, value], ...] 对，
// key = "复权:code:date"，value 为 cum 数值或含 cum 字段的对象。
func (f *FactorStore) Load(ctx context.Context, c *Client) error {
	raw, err := c.Get(ctx, "复权*")
	if err != nil {
		return err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &items); err != nil {
		return fmt.Errorf("stockdb: decode 复权*: %w", err)
	}
	dates := map[string][]string{}
	cums := map[string][]float64{}
	for _, it := range items {
		key, cum, ok := parseFactorPair(it)
		if !ok {
			continue
		}
		parts := strings.Split(key, ":")
		if len(parts) != 3 {
			continue
		}
		code, date := parts[1], parts[2]
		dates[code] = append(dates[code], date)
		cums[code] = append(cums[code], cum)
	}
	f.mu.Lock()
	f.dates, f.cums = dates, cums
	f.mu.Unlock()
	return nil
}

// parseFactorPair 解析一条 [key, value] 对；value 可能是裸数值或 {"cum": x} 对象。
// 若实测服务端格式不同（例如 value 对象内含 key 字段），仅需改本函数。
func parseFactorPair(it json.RawMessage) (key string, cum float64, ok bool) {
	var pair []json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(it), &pair); err != nil || len(pair) != 2 {
		return "", 0, false
	}
	if err := json.Unmarshal(pair[0], &key); err != nil {
		return "", 0, false
	}
	v := bytes.TrimSpace(pair[1])
	if len(v) > 0 && v[0] == '{' {
		var obj map[string]float64
		if err := json.Unmarshal(v, &obj); err != nil {
			return "", 0, false
		}
		cum, ok = obj["cum"]
		return key, cum, ok
	}
	if err := json.Unmarshal(v, &cum); err != nil {
		return "", 0, false
	}
	return key, cum, true
}

// setFactors 测试辅助：直接注入因子。
func (f *FactorStore) setFactors(code string, dates []string, cums []float64) {
	f.mu.Lock()
	f.dates[code], f.cums[code] = dates, cums
	f.mu.Unlock()
}

// factorLE 返回 <= ymd 的最大除权日对应的 cum（二分查找）。
func (f *FactorStore) factorLE(code, ymd string) (float64, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	dates, ok := f.dates[code]
	if !ok {
		return 0, false
	}
	idx := sort.Search(len(dates), func(i int) bool { return dates[i] > ymd }) - 1
	if idx < 0 {
		return 0, false
	}
	return f.cums[code][idx], true
}

// AdjustBars 对 bars（任意顺序，通常升序）做 qfq/hfq 折算。
// qfq: ratio = 最新cum / 当期cum；hfq: ratio = 1 / 当期cum。
// 只折算 OHLC/PreClose，不动 Volume/Amount。对照 stock_sdk._apply_fq_in_memory。
func (f *FactorStore) AdjustBars(code string, bars []Bar, fq FQ) []Bar {
	if fq == FQNone || len(bars) == 0 {
		return bars
	}
	f.mu.RLock()
	dates, ok := f.dates[code]
	cums := f.cums[code]
	f.mu.RUnlock()
	if !ok || len(dates) == 0 {
		return bars
	}
	decimals := 2
	if strings.HasPrefix(code, "1") || strings.HasPrefix(code, "5") {
		decimals = 3
	}
	latest := cums[len(cums)-1]
	out := make([]Bar, len(bars))
	for i, b := range bars {
		ymd := strconv.FormatInt(b.Date, 10)
		if len(ymd) > 8 {
			ymd = ymd[:8]
		}
		idx := sort.Search(len(dates), func(j int) bool { return dates[j] > ymd }) - 1
		fc := 1.0
		if idx >= 0 {
			fc = cums[idx]
		}
		ratio := 1 / fc
		if fq == FQQFQ {
			ratio = latest / fc
		}
		if math.Abs(ratio-1) < 1e-6 {
			out[i] = b
			continue
		}
		b.Open = round(b.Open/ratio, decimals)
		b.High = round(b.High/ratio, decimals)
		b.Low = round(b.Low/ratio, decimals)
		b.Close = round(b.Close/ratio, decimals)
		b.PreClose = round(b.PreClose/ratio, decimals)
		out[i] = b
	}
	return out
}

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
```

> **实现注意**：`parseFactorValue` 假设服务端 `复权*` 返回值对象内含 `key`/`cum` 字段。首次联调真实 stockdb 时若格式不符（例如纯 `[key, cum]` 数组对），改用 `client.Get` 原始返回自行拆对——**只改 `Load`/`parseFactorValue`，其余逻辑不动**。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/data/datasource/freestockdb/factor.go backend/data/datasource/freestockdb/factor_test.go
git commit -m "feat(datasource): free-stockdb adjust-factor store and qfq/hfq conversion"
```

---

### Task 4: 周期聚合（周/月K + 5/15/30/60 分钟）

**Files:**
- Create: `backend/data/datasource/freestockdb/aggregate.go`
- Test: `backend/data/datasource/freestockdb/aggregate_test.go`

**Interfaces:**
- Consumes: `Bar`、`round`（Task 3）
- Produces: `AggregatePeriod(daily []Bar, weekly bool) []Bar`；`AggregateMinutes(mins []Bar, interval int) []Bar`；`parseDate8(v int64) (time.Time, error)`

- [ ] **Step 1: Write the failing test**

```go
package freestockdb

import "testing"

func dayBar(date int64, o, h, l, c, v, a float64) Bar {
	return Bar{Date: date, Open: o, High: h, Low: l, Close: c, Volume: v, Amount: a}
}

func TestAggregateWeekly(t *testing.T) {
	daily := []Bar{
		dayBar(20260622, 10, 11, 9.5, 10.5, 100, 1000), // 周一
		dayBar(20260623, 10.5, 11.2, 10.4, 11.0, 200, 2000),
		dayBar(20260629, 11, 11.5, 10.8, 11.3, 150, 1500), // 下周一
	}
	out := AggregatePeriod(daily, true)
	if len(out) != 2 {
		t.Fatalf("weeks = %d, want 2", len(out))
	}
	w1 := out[0]
	if w1.Open != 10 || w1.High != 11.2 || w1.Low != 9.5 || w1.Close != 11.0 || w1.Volume != 300 {
		t.Errorf("w1 = %+v", w1)
	}
	if w1.Date != 20260623 {
		t.Errorf("week date should be last trading day 20260623, got %d", w1.Date)
	}
	if out[1].PreClose != 11.0 {
		t.Errorf("w2 preClose should be prev week close 11.0, got %v", out[1].PreClose)
	}
}

func TestAggregateMonthly(t *testing.T) {
	daily := []Bar{
		dayBar(20260625, 10, 11, 9.5, 10.5, 100, 1000),
		dayBar(20260701, 10.5, 11.2, 10.4, 11.0, 200, 2000),
	}
	out := AggregatePeriod(daily, false)
	if len(out) != 2 || out[0].Date != 20260625 || out[1].Date != 20260701 {
		t.Fatalf("months = %+v", out)
	}
}

func minBar(hhmmss int64, o, h, l, c, v float64) Bar {
	return Bar{Date: 20260625*1000000 + hhmmss, Open: o, High: h, Low: l, Close: c, Volume: v}
}

func TestAggregateMinutes(t *testing.T) {
	mins := []Bar{
		minBar(93100, 10, 10.1, 9.9, 10.0, 100),  // 09:31
		minBar(93200, 10, 10.2, 9.95, 10.1, 120), // 09:32
		minBar(93300, 10.1, 10.3, 10.0, 10.2, 80),
		minBar(93400, 10.2, 10.4, 10.1, 10.3, 90),
		minBar(93500, 10.3, 10.5, 10.2, 10.4, 110), // 09:35 → 第一个 5m 结束
		minBar(93600, 10.4, 10.6, 10.3, 10.5, 100),
	}
	out := AggregateMinutes(mins, 5)
	if len(out) != 2 {
		t.Fatalf("5m bars = %d, want 2: %+v", len(out), out)
	}
	b1 := out[0]
	if b1.Open != 10 || b1.High != 10.5 || b1.Low != 9.9 || b1.Close != 10.4 || b1.Volume != 500 {
		t.Errorf("b1 = %+v", b1)
	}
	// 对齐结束时刻 09:35 → date = 20260625093500
	if b1.Date != 20260625093500 {
		t.Errorf("b1.Date = %d, want 20260625093500", b1.Date)
	}
}

func TestTradingElapsedBoundary(t *testing.T) {
	if e, ok := tradingElapsed(930); !ok || e != 0 {
		t.Errorf("09:30 → %d,%v", e, ok)
	}
	if e, ok := tradingElapsed(1130); !ok || e != 120 {
		t.Errorf("11:30 → %d,%v", e, ok)
	}
	if e, ok := tradingElapsed(1300); !ok || e != 121 {
		t.Errorf("13:00 → %d,%v", e, ok)
	}
	if _, ok := tradingElapsed(1200); ok {
		t.Error("12:00 should be outside trading hours")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestAggregate|TestTradingElapsed' -v`
Expected: FAIL — undefined: AggregatePeriod 等

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"sort"
	"time"
)

func parseDate8(v int64) (time.Time, error) {
	return time.Parse("20060102", itoa8(v))
}

func itoa8(v int64) string {
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf)
}

// AggregatePeriod 把升序日K聚合为周K（weekly=true，ISO 周）或月K。
// 对照 stock_sdk._merge_to_period。
func AggregatePeriod(daily []Bar, weekly bool) []Bar {
	if len(daily) == 0 {
		return nil
	}
	type gkey struct{ y, n int }
	var order []gkey
	byKey := map[gkey][]Bar{}
	for _, b := range daily {
		t, err := parseDate8(b.Date)
		if err != nil {
			continue
		}
		var k gkey
		if weekly {
			y, w := t.ISOWeek()
			k = gkey{y, w}
		} else {
			k = gkey{t.Year(), int(t.Month())}
		}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], b)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].y != order[j].y {
			return order[i].y < order[j].y
		}
		return order[i].n < order[j].n
	})
	out := make([]Bar, 0, len(order))
	for _, k := range order {
		items := byKey[k]
		m, ok := mergeBars(items)
		if !ok {
			continue
		}
		if len(out) > 0 {
			m.PreClose = out[len(out)-1].Close
		}
		if m.PreClose == 0 {
			m.PreClose = items[0].PreClose
		}
		if m.PreClose == 0 {
			m.PreClose = m.Open
		}
		if m.PreClose != 0 {
			m.Turnover = round(sumField(items, func(b Bar) float64 { return b.Turnover }), 3)
			m.VolRatio = round(avgField(items, func(b Bar) float64 { return b.VolRatio }), 3)
		}
		out = append(out, m)
	}
	return out
}

func mergeBars(items []Bar) (Bar, bool) {
	if len(items) == 0 {
		return Bar{}, false
	}
	m := items[0]
	for _, b := range items[1:] {
		if b.High > m.High {
			m.High = b.High
		}
		if b.Low < m.Low {
			m.Low = b.Low
		}
		m.Volume += b.Volume
		m.Amount += b.Amount
	}
	last := items[len(items)-1]
	m.Date, m.Close = last.Date, last.Close
	m.Code, m.Name = last.Code, last.Name
	m.PeTTM, m.Pb = last.PeTTM, last.Pb
	m.TotalMv, m.FloatMv, m.IsST = last.TotalMv, last.FloatMv, last.IsST
	m.PreClose = 0 // 由调用方填充
	return m, true
}

func sumField(items []Bar, f func(Bar) float64) float64 {
	var s float64
	for _, b := range items {
		s += f(b)
	}
	return s
}

func avgField(items []Bar, f func(Bar) float64) float64 {
	if len(items) == 0 {
		return 0
	}
	return sumField(items, f) / float64(len(items))
}

// tradingElapsed 把 HHMM（如 930）映射为交易时段内分钟序号：
// 9:30-11:30 → 0-120；13:00-15:00 → 121-240。对照 stock_sdk.trading_elapsed。
func tradingElapsed(hhmm int) (int, bool) {
	m := hhmm/100*60 + hhmm%100
	switch {
	case m >= 570 && m <= 690:
		return m - 570, true
	case m >= 780 && m <= 900:
		if m == 780 {
			return 121, true
		}
		return 120 + (m - 780), true
	}
	return 0, false
}

// elapsedToMinuteOfDay 是 tradingElapsed 的逆映射（输出 HHMM）。
func elapsedToMinuteOfDay(e int) int {
	if e > 240 {
		e = 240
	}
	if e <= 120 {
		m := 570 + e
		return m/60*100 + m%60
	}
	m := 780 + (e - 120)
	return m/60*100 + m%60
}

// AggregateMinutes 把升序 1 分钟K聚合为 interval 分钟K（5/15/30/60）。
// 按交易时段对齐，分组键为对齐结束时刻。对照 stock_sdk._merge_minutes_to_period。
func AggregateMinutes(mins []Bar, interval int) []Bar {
	if len(mins) == 0 || interval <= 1 {
		return mins
	}
	type gkey struct {
		ymd int64
		end int
	}
	var order []gkey
	byKey := map[gkey][]Bar{}
	for _, b := range mins {
		ymd := b.Date / 1000000
		hhmm := int(b.Date/10000) % 100 * 100
		hhmm += int(b.Date/100) % 100
		elapsed, ok := tradingElapsed(hhmm)
		if !ok {
			continue
		}
		end := interval
		if elapsed > 0 {
			end = ((elapsed-1)/interval + 1) * interval
		}
		k := gkey{ymd, end}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], b)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].ymd != order[j].ymd {
			return order[i].ymd < order[j].ymd
		}
		return order[i].end < order[j].end
	})
	out := make([]Bar, 0, len(order))
	for _, k := range order {
		m, ok := mergeBars(byKey[k])
		if !ok {
			continue
		}
		hhmm := elapsedToMinuteOfDay(k.end)
		if hhmm/100 >= 24 {
			hhmm = 2359
		}
		m.Date = k.ymd*1000000 + int64(hhmm)*100
		if len(out) > 0 {
			m.PreClose = out[len(out)-1].Close
		} else if m.PreClose == 0 {
			m.PreClose = m.Open
		}
		out = append(out, m)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/data/datasource/freestockdb/aggregate.go backend/data/datasource/freestockdb/aggregate_test.go
git commit -m "feat(datasource): free-stockdb weekly/monthly/minute aggregation"
```

---

### Task 5: KLineService 门面（查询→复权→聚合→截取）

**Files:**
- Create: `backend/data/datasource/freestockdb/service.go`
- Test: `backend/data/datasource/freestockdb/service_test.go`

**Interfaces:**
- Consumes: Task 1-4 全部
- Produces: `Frequency`（`Freq1d/Freq1w/Freq1M/Freq1m/Freq5m/Freq15m/Freq30m/Freq60m`）；`NewKLineService(c *Client, f *FactorStore) *KLineService`；`(*KLineService).GetData(ctx, code, freq, start, end, fq, desc, limit) ([]Bar, error)`；`(*KLineService).LastN(ctx, code string, freq Frequency, count int, fq FQ) ([]Bar, error)`（升序，Task 8 的 Provider 使用）

- [ ] **Step 1: Write the failing test**

```go
package freestockdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestService' -v`
Expected: FAIL — undefined: NewKLineService 等

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"context"
)

// Frequency K线周期。
type Frequency string

const (
	Freq1d  Frequency = "1d"
	Freq1w  Frequency = "1w"
	Freq1M  Frequency = "1M"
	Freq1m  Frequency = "1m"
	Freq5m  Frequency = "5m"
	Freq15m Frequency = "15m"
	Freq30m Frequency = "30m"
	Freq60m Frequency = "60m"
)

// KLineService 组合 查询 → 复权 → 聚合 → 截取，对齐 stock_sdk.get_data。
type KLineService struct {
	c       *Client
	factors *FactorStore
}

func NewKLineService(c *Client, f *FactorStore) *KLineService {
	return &KLineService{c: c, factors: f}
}

func isMinuteFreq(f Frequency) bool {
	switch f {
	case Freq1m, Freq5m, Freq15m, Freq30m, Freq60m:
		return true
	}
	return false
}

func minuteInterval(f Frequency) int {
	switch f {
	case Freq5m:
		return 5
	case Freq15m:
		return 15
	case Freq30m:
		return 30
	case Freq60m:
		return 60
	}
	return 1
}

// rawFreq 返回聚合周期的底层查询周期。
func rawFreq(f Frequency) Frequency {
	switch f {
	case Freq1w, Freq1M:
		return Freq1d
	case Freq5m, Freq15m, Freq30m, Freq60m:
		return Freq1m
	}
	return f
}

// rawMultiplier 返回聚合一根目标K线大致需要的底层K线数。
func rawMultiplier(f Frequency) int {
	switch f {
	case Freq1w:
		return 7
	case Freq1M:
		return 31
	case Freq5m:
		return 5
	case Freq15m:
		return 15
	case Freq30m:
		return 30
	case Freq60m:
		return 60
	}
	return 1
}

func dayToBar(d DayKBar) Bar {
	return Bar{Date: d.Date, Open: d.Open, High: d.High, Low: d.Low, Close: d.Close,
		PreClose: d.PreClose, Volume: d.Volume, Amount: d.Amount, Turnover: d.Turnover,
		VolRatio: d.VolRatio, PeTTM: d.PeTTM, Pb: d.Pb, TotalMv: d.TotalMv, FloatMv: d.FloatMv,
		IsST: d.IsST, Code: d.Code, Name: d.Name}
}

func minuteToBar(m MinuteKBar) Bar {
	return Bar{Date: m.Date, Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
		Volume: m.Volume, Amount: m.Amount, Code: m.Code}
}

// rawBars 查询底层K线并做复权折算（复权先于聚合，对齐 stock_sdk）。
func (s *KLineService) rawBars(ctx context.Context, code string, freq Frequency, start, end string, fq FQ, desc bool) ([]Bar, error) {
	if isMinuteFreq(freq) {
		mins, err := s.c.QueryMinuteK(ctx, code, start, end, desc)
		if err != nil {
			return nil, err
		}
		bars := make([]Bar, 0, len(mins))
		for _, m := range mins {
			bars = append(bars, minuteToBar(m))
		}
		return s.factors.AdjustBars(code, bars, fq), nil
	}
	days, err := s.c.QueryDayK(ctx, code, start, end, desc)
	if err != nil {
		return nil, err
	}
	bars := make([]Bar, 0, len(days))
	for _, d := range days {
		bars = append(bars, dayToBar(d))
	}
	return s.factors.AdjustBars(code, bars, fq), nil
}

// GetData 对齐 stock_sdk.get_data：查询 → 复权 → 聚合 → 排序 → 截取。
func (s *KLineService) GetData(ctx context.Context, code string, freq Frequency, start, end string, fq FQ, desc bool, limit int) ([]Bar, error) {
	requiresAgg := freq == Freq1w || freq == Freq1M || minuteInterval(freq) > 1
	bars, err := s.rawBars(ctx, code, rawFreq(freq), start, end, fq, desc && !requiresAgg)
	if err != nil {
		return nil, err
	}
	switch {
	case freq == Freq1w:
		bars = AggregatePeriod(bars, true)
	case freq == Freq1M:
		bars = AggregatePeriod(bars, false)
	case minuteInterval(freq) > 1:
		bars = AggregateMinutes(bars, minuteInterval(freq))
	}
	if desc && requiresAgg {
		reverseBars(bars)
	}
	if limit > 0 && len(bars) > limit {
		bars = bars[:limit]
	}
	return bars, nil
}

// LastN 返回按时间升序的最后 count 根（Provider 的主要调用方式）。
// 聚合周期先按 count*multiplier 截取底层数据再聚合，避免全量历史聚合。
func (s *KLineService) LastN(ctx context.Context, code string, freq Frequency, count int, fq FQ) ([]Bar, error) {
	if count <= 0 {
		count = 1
	}
	rf, mult := rawFreq(freq), rawMultiplier(freq)
	bars, err := s.rawBars(ctx, code, rf, "", "", fq, false)
	if err != nil {
		return nil, err
	}
	if need := count * mult; len(bars) > need {
		bars = bars[len(bars)-need:]
	}
	switch {
	case freq == Freq1w:
		bars = AggregatePeriod(bars, true)
	case freq == Freq1M:
		bars = AggregatePeriod(bars, false)
	case minuteInterval(freq) > 1:
		bars = AggregateMinutes(bars, minuteInterval(freq))
	}
	if len(bars) > count {
		bars = bars[len(bars)-count:]
	}
	return bars, nil
}

func reverseBars(b []Bar) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS

- [ ] **Step 5: 黄金数据对拍（可选，需本机已运行 stockdb + Python SDK）**

Create `backend/data/datasource/freestockdb/testdata/gen_golden.py`：

```python
# 生成黄金对照数据：需先把 free-stockdb 仓库的 pybao 目录加入 PYTHONPATH，
# 且 stockdb.exe 已启动并有数据。输出 golden_600633.json 到本目录。
import json, os
from stock_sdk import StockDBClient

c = StockDBClient()
out = {}
for fq in ("qfq", "hfq", None):
    out[f"day_{fq}"] = c.get_data("600633", start="20250101", end="20260701", fq=fq)
out["week_qfq"] = c.get_data("600633", start="20250101", end="20260701", frequency="1w", fq="qfq")
out["30m_qfq"] = c.get_data("600633", start="20260625", end="20260626", frequency="30m", fq="qfq")
with open(os.path.join(os.path.dirname(__file__), "golden_600633.json"), "w", encoding="utf-8") as fp:
    json.dump(out, fp, ensure_ascii=False)
```

在 `service_test.go` 追加 env-gated 对拍测试（golden 文件缺失时跳过）：

```go
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
	bars, err := svc.GetData(context.Background(), "600633", Freq1d, "20250101", "20260701", FQQFQ, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	refRows := ref["day_qfq"]
	if len(bars) != len(refRows) {
		t.Fatalf("row count %d != golden %d", len(bars), len(refRows))
	}
	for i, b := range bars {
		rc, _ := refRows[i]["close"].(float64)
		if math.Abs(b.Close-rc) > 1e-3 {
			t.Fatalf("row %d close %v != golden %v", i, b.Close, rc)
		}
	}
}
```

> 需要在 imports 加 `encoding/json`、`math`、`os`。

- [ ] **Step 6: Commit**

```bash
git add backend/data/datasource/freestockdb/service.go backend/data/datasource/freestockdb/service_test.go backend/data/datasource/freestockdb/testdata/gen_golden.py
git commit -m "feat(datasource): free-stockdb KLineService facade with fq+aggregation"
```

---

### Task 6: 板块索引 BoardIndex

**Files:**
- Create: `backend/data/datasource/freestockdb/board.go`
- Test: `backend/data/datasource/freestockdb/board_test.go`

**Interfaces:**
- Consumes: `(*Client).Get`、`decodeValues`（Task 1）
- Produces: `Board`；`CategoryConcept/CategorySW1/CategorySW2/CategorySW3`；`NewBoardIndex() *BoardIndex`；`(*BoardIndex).Load(ctx, c *Client) error`；`(*BoardIndex).OfStock(code string, category int) []*Board`；`(*BoardIndex).SymbolsOfBoard(name string, category int) ([]string, error)`；`(*BoardIndex).SearchName(keyword string, category int) []*Board`（Task 8 SectorProvider 使用）

- [ ] **Step 1: Write the failing test**

```go
package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const boardFixture = `[
	{"code":"300843.TI","name":"5G","source":"ths","type":"concept","group":"概念板块列表","category":"概念","symbols":["000016.SZ","000049.SZ","600633.SH"]},
	{"code":"801760.SL","name":"传媒","source":"sw","type":"sw_1","group":"申万行业指数列表","category":"申万一级","symbols":["600633.SH"]},
	{"code":"888888.TI","name":"5G消息","source":"ths","type":"concept","group":"概念板块列表","category":"概念","symbols":["000016.SZ"]}
]`

func loadFixture(t *testing.T) *BoardIndex {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(boardFixture))
	}))
	defer srv.Close()
	bi := NewBoardIndex()
	if err := bi.Load(context.Background(), NewClient(srv.URL[len("http://"):]); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return bi
}

func TestBoardOfStock(t *testing.T) {
	bi := loadFixture(t)
	got := bi.OfStock("600633", CategoryConcept)
	if len(got) != 1 || got[0].Name != "5G" {
		t.Fatalf("OfStock concept = %+v", got)
	}
	got = bi.OfStock("600633.SH", CategorySW1) // 带后缀代码也应命中
	if len(got) != 1 || got[0].Name != "传媒" {
		t.Fatalf("OfStock sw1 = %+v", got)
	}
}

func TestBoardSymbolsOf(t *testing.T) {
	bi := loadFixture(t)
	syms, err := bi.SymbolsOfBoard("5G", CategoryConcept)
	if err != nil || len(syms) != 3 || syms[0] != "000016" {
		t.Fatalf("symbols = %v err=%v", syms, err)
	}
}

func TestBoardSearchName(t *testing.T) {
	bi := loadFixture(t)
	got := bi.SearchName("5G", CategoryConcept)
	if len(got) != 2 { // "5G" 与 "5G消息"
		t.Fatalf("search = %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestBoard' -v`
Expected: FAIL — undefined: NewBoardIndex 等

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// 板块类别（对齐 zhibiao.py CATEGORY_MAP）。
const (
	CategoryConcept = 0 // 概念
	CategorySW1     = 1 // 申万一级
	CategorySW2     = 2 // 申万二级
	CategorySW3     = 3 // 申万三级
)

var categoryNames = map[int]string{
	CategoryConcept: "概念",
	CategorySW1:     "申万一级",
	CategorySW2:     "申万二级",
	CategorySW3:     "申万三级",
}

// Board 对应服务端 "板块" 记录。
type Board struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Source   string   `json:"source"`
	Type     string   `json:"type"`
	Group    string   `json:"group"`
	Category string   `json:"category"`
	Symbols  []string `json:"symbols"`
}

// BoardIndex 板块四向内存索引（对照 zhibiao.py BoardIndex）。
type BoardIndex struct {
	mu         sync.RWMutex
	byCode     map[string]*Board
	byStock    map[string][]*Board
	byName     map[string][]*Board // key = category + "_" + name
	byCategory map[string][]*Board
}

func NewBoardIndex() *BoardIndex {
	return &BoardIndex{
		byCode:     map[string]*Board{},
		byStock:    map[string][]*Board{},
		byName:     map[string][]*Board{},
		byCategory: map[string][]*Board{},
	}
}

// stockCode 统一为 6 位数字代码（去掉 .SH/.SZ 等后缀）。
func stockCode(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	return s
}

// Load 拉取 "板块*" 全量并重建索引。
func (bi *BoardIndex) Load(ctx context.Context, c *Client) error {
	raw, err := c.Get(ctx, "板块*")
	if err != nil {
		return err
	}
	vals, err := decodeValues(raw)
	if err != nil {
		return err
	}
	nb := NewBoardIndex()
	for _, v := range vals {
		var b Board
		if err := json.Unmarshal(v, &b); err != nil {
			continue
		}
		if b.Code == "" || b.Name == "" || b.Category == "" {
			continue
		}
		syms := make([]string, 0, len(b.Symbols))
		for _, s := range b.Symbols {
			if sc := stockCode(s); sc != "" {
				syms = append(syms, sc)
			}
		}
		b.Symbols = syms
		bb := b
		nb.byCode[b.Code] = &bb
		nb.byName[b.Category+"_"+b.Name] = append(nb.byName[b.Category+"_"+b.Name], &bb)
		nb.byCategory[b.Category] = append(nb.byCategory[b.Category], &bb)
		for _, s := range b.Symbols {
			nb.byStock[s] = append(nb.byStock[s], &bb)
		}
	}
	bi.mu.Lock()
	bi.byCode, bi.byStock, bi.byName, bi.byCategory = nb.byCode, nb.byStock, nb.byName, nb.byCategory
	bi.mu.Unlock()
	return nil
}

// OfStock 查股票所属板块；category 传 -1 表示不限类别。
func (bi *BoardIndex) OfStock(code string, category int) []*Board {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	items := bi.byStock[stockCode(code)]
	if category < 0 {
		return items
	}
	cat := categoryNames[category]
	out := make([]*Board, 0, len(items))
	for _, b := range items {
		if b.Category == cat {
			out = append(out, b)
		}
	}
	return out
}

// SymbolsOfBoard 查板块成分股（6 位代码）。
func (bi *BoardIndex) SymbolsOfBoard(name string, category int) ([]string, error) {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	cat, ok := categoryNames[category]
	if !ok {
		return nil, fmt.Errorf("unknown category %d", category)
	}
	items := bi.byName[cat+"_"+name]
	if len(items) == 0 {
		return nil, fmt.Errorf("board %q not found in %s", name, cat)
	}
	return items[0].Symbols, nil
}

// SearchName 按名称子串模糊匹配；category 传 -1 表示不限类别。
func (bi *BoardIndex) SearchName(keyword string, category int) []*Board {
	bi.mu.RLock()
	defer bi.mu.RUnlock()
	var pool []*Board
	if category < 0 {
		for _, items := range bi.byCategory {
			pool = append(pool, items...)
		}
	} else {
		pool = bi.byCategory[categoryNames[category]]
	}
	out := make([]*Board, 0)
	for _, b := range pool {
		if strings.Contains(b.Name, keyword) {
			out = append(out, b)
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/data/datasource/freestockdb/board.go backend/data/datasource/freestockdb/board_test.go
git commit -m "feat(datasource): free-stockdb board index with bidirectional mapping"
```

---

### Task 7: 进程管理 Manager

**Files:**
- Create: `backend/data/datasource/freestockdb/manager.go`
- Test: `backend/data/datasource/freestockdb/manager_test.go`

**Interfaces:**
- Consumes: `Client`（Task 1）
- Produces: `Config{Enabled, ExePath, Addr, AutoStart}`；`NewManager(cfg Config) *Manager`；`(*Manager).Client() *Client`；`(*Manager).Start(ctx) error`；`(*Manager).Stop()`；`(*Manager).Available(ctx) bool`（Task 8 使用）

- [ ] **Step 1: Write the failing test**

```go
package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManagerAvailableAdoptsRunningInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"0":["000001"]}`))
	}))
	defer srv.Close()
	m := NewManager(Config{Enabled: true, Addr: srv.URL[len("http://"):]})
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m.Available(context.Background()) {
		t.Error("should adopt already-running instance")
	}
	m.Stop() // 不应 panic（非本进程拉起）
}

func TestManagerDisabled(t *testing.T) {
	m := NewManager(Config{Enabled: false})
	if m.Available(context.Background()) {
		t.Error("disabled manager must be unavailable")
	}
}

func TestManagerUnavailableNoAutoStart(t *testing.T) {
	m := NewManager(Config{Enabled: true, Addr: "127.0.0.1:1"})
	if err := m.Start(context.Background()); err == nil {
		t.Error("Start should fail when nothing listening and AutoStart off")
	}
	if m.Available(context.Background()) {
		t.Error("should be unavailable")
	}
}

func TestManagerAvailableCache(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	m := NewManager(Config{Enabled: true, Addr: srv.URL[len("http://"):]})
	m.Available(context.Background())
	m.Available(context.Background()) // 30s 内第二次应走缓存
	if hits != 1 {
		t.Errorf("hits = %d, want 1 (cached)", hits)
	}
	// 测试可注入较短缓存窗口
	m.availableTTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	m.Available(context.Background())
	if hits != 2 {
		t.Errorf("hits = %d, want 2 after ttl", hits)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestManager' -v`
Expected: FAIL — undefined: NewManager 等

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Config free-stockdb 引擎配置（来自 Settings 表）。
type Config struct {
	Enabled   bool   // 总开关
	ExePath   string // stockdb.exe 路径（自动拉起用）
	Addr      string // 默认 127.0.0.1:7899
	AutoStart bool   // 未运行时是否自动拉起
}

// Manager 管理 stockdb 进程生命周期与可用性探测。
type Manager struct {
	cfg    Config
	client *Client

	cmd *exec.Cmd // 仅当由本进程拉起时非空

	mu           sync.Mutex
	checkedAt    time.Time
	ok           bool
	availableTTL time.Duration
}

func NewManager(cfg Config) *Manager {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:7899"
	}
	return &Manager{cfg: cfg, client: NewClient(cfg.Addr), availableTTL: 30 * time.Second}
}

func (m *Manager) Client() *Client { return m.client }

// Start：已在运行则直接采用；否则按配置拉起并做健康检查（5s × 10 次）。
func (m *Manager) Start(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}
	if m.client.Ping(ctx) {
		m.setOK(true)
		return nil
	}
	if !m.cfg.AutoStart || m.cfg.ExePath == "" {
		return fmt.Errorf("freestockdb: %s 未响应且未配置自动拉起", m.cfg.Addr)
	}
	cmd := exec.Command(m.cfg.ExePath)
	cmd.Dir = filepath.Dir(m.cfg.ExePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("freestockdb: 拉起 %s 失败: %w", m.cfg.ExePath, err)
	}
	m.cmd = cmd
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
		if m.client.Ping(ctx) {
			m.setOK(true)
			return nil
		}
	}
	return fmt.Errorf("freestockdb: 健康检查超时（%s）", m.cfg.Addr)
}

// Stop 回收由本进程拉起的 stockdb；用户自行启动的实例不动。
func (m *Manager) Stop() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}
}

// Available 带 30s 缓存的可用性探测（Router 每次调用都会走这里）。
func (m *Manager) Available(ctx context.Context) bool {
	if !m.cfg.Enabled {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.checkedAt) < m.availableTTL {
		return m.ok
	}
	m.ok = m.client.Ping(ctx)
	m.checkedAt = time.Now()
	return m.ok
}

func (m *Manager) setOK(ok bool) {
	m.mu.Lock()
	m.ok, m.checkedAt = ok, time.Now()
	m.mu.Unlock()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/data/datasource/freestockdb/manager.go backend/data/datasource/freestockdb/manager_test.go
git commit -m "feat(datasource): free-stockdb process manager with auto-start"
```

---

### Task 8: Provider 实现 + Router 注册 + Settings 配置 + main.go 接线

**Files:**
- Create: `backend/data/datasource/freestockdb/provider.go`
- Test: `backend/data/datasource/freestockdb/provider_test.go`
- Modify: `backend/data/settings_api.go:55-56`（Settings 结构体尾部加 4 个字段）
- Modify: `main.go:71-84`（initDataSources 内注册）

**Interfaces:**
- Consumes: Task 1-7 全部；`datasource.KLineProvider/QuoteProvider/SectorProvider`（provider.go:31-59）；`datasource.NormalizePeriod`（provider.go:124）；`datasource.GetRouter()`（router.go:33）
- Produces: `NewProvider(m *Manager, svc *KLineService, bi *BoardIndex) *Provider`（实现三个 Provider 接口）；`Setup(router *datasource.Router, cfg Config) *Manager`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/data/datasource/freestockdb/ -run 'TestProvider' -v`
Expected: FAIL — undefined: NewProvider

- [ ] **Step 3: Write minimal implementation**

```go
package freestockdb

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// Provider 同时实现 KLineProvider / QuoteProvider / SectorProvider。
type Provider struct {
	m      *Manager
	svc    *KLineService
	boards *BoardIndex
}

func NewProvider(m *Manager, svc *KLineService, bi *BoardIndex) *Provider {
	return &Provider{m: m, svc: svc, boards: bi}
}

func (p *Provider) Name() string      { return "freestockdb" }
func (p *Provider) Priority() int     { return 5 }
func (p *Provider) Available(ctx context.Context) bool { return p.m.Available(ctx) }

var periodFreq = map[string]Frequency{
	"101": Freq1d, "102": Freq1w, "103": Freq1M,
	"1": Freq1m, "5": Freq5m, "15": Freq15m, "30": Freq30m, "60": Freq60m,
}

// GetKLine period 为 datasource 周期码（"day"/"101"/"5m"/"5" 均可）。
func (p *Provider) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
	freq, ok := periodFreq[datasource.NormalizePeriod(period)]
	if !ok {
		return nil, fmt.Errorf("freestockdb: unsupported period %q", period)
	}
	bars, err := p.svc.LastN(ctx, code, freq, count, FQQFQ)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("freestockdb: empty result for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s from freestockdb (%d bars)", code, len(bars))
	return toKLineData(code, period, bars), nil
}

// GetQuote 用最新一根日K（不复权）实现报价。
func (p *Provider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	bars, err := p.svc.LastN(ctx, code, Freq1d, 1, FQNone)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("freestockdb: no quote for %s", code)
	}
	b := bars[0]
	return &datasource.QuoteData{
		Code:      code,
		Name:      b.Name,
		Price:     b.Close,
		Change:    round(b.Close-b.PreClose, 2),
		ChangePct: round(pctChg(b), 2),
		Volume:    int64(b.Volume),
		Amount:    b.Amount,
		High:      b.High,
		Low:       b.Low,
		Open:      b.Open,
		PrevClose: b.PreClose,
		Time:      barTime(b.Date),
	}, nil
}

func pctChg(b Bar) float64 {
	if b.PreClose == 0 {
		return 0
	}
	return (b.Close - b.PreClose) / b.PreClose * 100
}

// GetSectorData 返回股票所属的第一个概念板块（与现有 sector chain 语义一致）。
func (p *Provider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	items := p.boards.OfStock(code, CategoryConcept)
	if len(items) == 0 {
		return nil, fmt.Errorf("freestockdb: no board for %s", code)
	}
	return &datasource.SectorData{Code: code, Sector: items[0].Name}, nil
}

func barTime(date int64) time.Time {
	if date > 1e12 {
		if t, err := time.Parse("20060102150405", fmt.Sprintf("%d", date)); err == nil {
			return t
		}
	}
	if t, err := time.Parse("20060102", fmt.Sprintf("%d", date)); err == nil {
		return t
	}
	return time.Time{}
}

func toKLineData(code, period string, bars []Bar) *datasource.KLineData {
	dst := &datasource.KLineData{Code: code, Period: period, Bars: make([]datasource.KLineBar, 0, len(bars))}
	for _, b := range bars {
		dst.Bars = append(dst.Bars, datasource.KLineBar{
			Time:      barTime(b.Date),
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			PrevClose: b.PreClose,
			Volume:    int64(b.Volume),
			Amount:    b.Amount,
		})
	}
	return dst
}

// Setup 拉起引擎、预载因子与板块索引，并把 Provider 注册进三条链。
// 引擎不可用时仅记录日志，Router 会自然降级到 TDX → 东财。
func Setup(router *datasource.Router, cfg Config) *Manager {
	m := NewManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		logger.SugaredLogger.Warnf("freestockdb: %v（降级使用远程数据源）", err)
	}
	client := m.Client()
	factors := NewFactorStore()
	bi := NewBoardIndex()
	if m.Available(ctx) {
		if err := factors.Load(ctx, client); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: 复权因子加载失败: %v", err)
		}
		if err := bi.Load(ctx, client); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: 板块索引加载失败: %v", err)
		}
	}
	p := NewProvider(m, NewKLineService(client, factors), bi)
	router.RegisterKLineProvider(p)
	router.RegisterQuoteProvider(p)
	router.RegisterSectorProvider(p)
	return m
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/data/datasource/freestockdb/ -v`
Expected: PASS（全部）

- [ ] **Step 5: Settings 表加配置字段**

Modify `backend/data/settings_api.go` — 在 `Settings` 结构体尾部（`PromptPlazaApiBase` 字段之后）追加：

```go
	// free-stockdb 本地数据引擎
	FreeStockDBEnable    bool   `json:"freeStockDBEnable" gorm:"column:free_stock_db_enable"`
	FreeStockDBPath      string `json:"freeStockDBPath" gorm:"column:free_stock_db_path"`
	FreeStockDBAddr      string `json:"freeStockDBAddr" gorm:"column:free_stock_db_addr"`
	FreeStockDBAutoStart bool   `json:"freeStockDBAutoStart" gorm:"column:free_stock_db_auto_start"`
```

（`main.go:312` 已有 `db.Dao.AutoMigrate(&data.Settings{})`，新列自动迁移，无需额外迁移代码。）

- [ ] **Step 6: main.go 接线**

Modify `main.go` 的 `initDataSources()`（main.go:71-84），在 `fallback.RegisterQuoteChain(router)` 之前插入：

```go
	cfg := data.GetSettingConfig()
	freestockdb.Setup(router, freestockdb.Config{
		Enabled:   cfg.FreeStockDBEnable,
		ExePath:   cfg.FreeStockDBPath,
		Addr:      cfg.FreeStockDBAddr,
		AutoStart: cfg.FreeStockDBAutoStart,
	})
```

并在 import 块加 `"go-stock/backend/data/datasource/freestockdb"`。

> 注意：`initDataSources()` 在 `db.Init("")`（main.go:111）之后调用，Settings 已可读。

- [ ] **Step 7: 编译验证**

Run: `go build ./...`
Expected: 编译通过，无 import cycle

- [ ] **Step 8: Commit**

```bash
git add backend/data/datasource/freestockdb/provider.go backend/data/datasource/freestockdb/provider_test.go backend/data/settings_api.go main.go
git commit -m "feat(datasource): register free-stockdb as top-priority provider chain"
```

---

### Task 9: 前端设置页配置区块

**Files:**
- Modify: `frontend/src/components/settings.vue`（formValue 定义 ~line 27-71、加载 ~line 237-284、保存 ~line 336-344、表单项 ~line 651 附近）

**Interfaces:**
- Consumes: Task 8 的 Settings 字段（`freeStockDBEnable/freeStockDBPath/freeStockDBAddr/freeStockDBAutoStart`，保存/加载走现有 UpdateSettings/GetConfig 通道）
- Produces: 设置页四个表单项

- [ ] **Step 1: formValue 默认值**

在 `formValue` 响应式对象中 `browserPath: '',`（settings.vue:71）之后加：

```js
  freeStockDBEnable: false,
  freeStockDBPath: '',
  freeStockDBAddr: '127.0.0.1:7899',
  freeStockDBAutoStart: false,
```

- [ ] **Step 2: 加载赋值**

在 `formValue.value.browserPath = res.browserPath`（settings.vue:284）之后加：

```js
      formValue.value.freeStockDBEnable = res.freeStockDBEnable
      formValue.value.freeStockDBPath = res.freeStockDBPath
      formValue.value.freeStockDBAddr = res.freeStockDBAddr || '127.0.0.1:7899'
      formValue.value.freeStockDBAutoStart = res.freeStockDBAutoStart
```

（另一处加载点 settings.vue:474 `config.browserPath` 之后同样加一遍，`config.` 前缀。）

- [ ] **Step 3: 保存提交**

在保存对象中 `browserPath: formValue.value.browserPath,`（settings.vue:344）之后加：

```js
    freeStockDBEnable: formValue.value.freeStockDBEnable,
    freeStockDBPath: formValue.value.freeStockDBPath,
    freeStockDBAddr: formValue.value.freeStockDBAddr,
    freeStockDBAutoStart: formValue.value.freeStockDBAutoStart,
```

- [ ] **Step 4: 表单项**

在「浏览器安装路径」表单项（settings.vue:651-653）之后加：

```vue
            <n-form-item-gi :span="3" label="本地数据引擎：" path="freeStockDBEnable">
              <n-switch v-model:value="formValue.freeStockDBEnable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="引擎程序路径：" path="freeStockDBPath">
              <n-input type="text" placeholder="stockdb.exe 完整路径" v-model:value="formValue.freeStockDBPath" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="引擎地址：" path="freeStockDBAddr">
              <n-input type="text" placeholder="127.0.0.1:7899" v-model:value="formValue.freeStockDBAddr" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="自动拉起：" path="freeStockDBAutoStart">
              <n-switch v-model:value="formValue.freeStockDBAutoStart"/>
            </n-form-item-gi>
```

- [ ] **Step 5: 前端构建验证**

Run: `cd frontend && npm run build`
Expected: 构建通过

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/settings.vue
git commit -m "feat(settings): add free-stockdb engine config section"
```

---

### Task 10: tool_indicator.go 切换到 Router 取数

**Files:**
- Modify: `backend/data/tool_indicator.go:62-90`（computeIndicatorsFromKLine 的取数与字段提取部分）

**Interfaces:**
- Consumes: `datasource.GetRouter().GetKLine(ctx, code, period, count)`（router.go:123，注册后自动优先走 free-stockdb）
- Produces: `GetTechnicalIndicators` 行为不变（返回结构、指标值不变），仅取数路径从老直连改为 Router

- [ ] **Step 1: 修改取数逻辑**

`computeIndicatorsFromKLine` 改为接收 ctx 并走 Router（签名同步改，调用点 `GetTechnicalIndicators` 内 `return computeIndicatorsFromKLine(code, period, count)` 改为 `return computeIndicatorsFromKLine(ctx, code, period, count)`）：

```go
// computeIndicatorsFromKLine fetches K-line data via the datasource router and computes indicators locally.
func computeIndicatorsFromKLine(ctx context.Context, code string, period string, count int) (*IndicatorResult, error) {
	klineData, err := datasource.GetRouter().GetKLine(ctx, code, period, count)
	if err != nil || klineData == nil || len(klineData.Bars) == 0 {
		logger.SugaredLogger.Warnf("indicators: no kline data for %s: %v", code, err)
		return &IndicatorResult{}, nil
	}

	bars := klineData.Bars
	n := len(bars)
	if n < 5 {
		return &IndicatorResult{}, nil
	}

	// KLineBar 已是 float64，无需 parseFloat64
	close := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	volume := make([]float64, n)
	for i, k := range bars {
		close[i] = k.Close
		high[i] = k.High
		low[i] = k.Low
		volume[i] = float64(k.Volume)
	}
```

import 块加 `"go-stock/backend/data/datasource"`。函数其余部分（MA/MACD/RSI/KDJ/BOLL 等计算）不动。

- [ ] **Step 2: 编译与测试**

Run: `go build ./... && go test ./backend/data/ -run Indicator -v`
Expected: 编译通过；现有指标测试（若有）通过

- [ ] **Step 3: Commit**

```bash
git add backend/data/tool_indicator.go
git commit -m "refactor(indicator): fetch kline via datasource router"
```

---

## 联调清单（全部 Task 完成后人工验证）

1. 启动 stockdb.exe（已有数据），go-stock 设置页开启「本地数据引擎」→ 重启应用，日志应出现 `datasource: kline ... from freestockdb`。
2. 回测/选股跑一只股票，对比 free-stockdb 与东财 K线数值一致（前复权口径）。
3. 关闭 stockdb.exe → 功能自动降级 TDX/东财，日志出现 `freestockdb(unavailable)`。
4. 配置 stockdb.exe 路径 + 自动拉起 → 冷启动应用，引擎被拉起且数据源生效。
5. （可选）`STOCKDB_ADDR=127.0.0.1:7899 go test ./backend/data/datasource/freestockdb/ -run TestGoldenCompare -v` 黄金对拍通过。
