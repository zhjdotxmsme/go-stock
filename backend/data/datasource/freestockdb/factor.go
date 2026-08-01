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
