package data

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/internal/port/datasource"
)

// 商品期货盘面数据服务（设计规格第 2 节）。
// 每个数据点独立降级：失败 → 记入 Failed 列表，其余数据照常返回。
//
// 已实测验证的数据源（2026-09-07 本机采样）：
//   - 东财 clist 板块 fs=m:113(SHFE) / fs=m:142(INE)：f2 价 / f3 涨跌% / f5 量 / f108 持仓量
//     与 Sina nf_ 报价 parts[13]=持仓、parts[14]=量 两两交叉一致（AU 154549/307857、AG 199514/513715）。
//   - Sina hq.sinajs.cn/list=nf_{CODE}0：parts[13]=持仓量、parts[14]=成交量（回退源）。
//   - 增仓(OIΔ)：公开接口无直接字段，采用"上一交易日持仓快照"差值（本地 data/ 目录持久化）。

// ---------- 输出类型 ----------

// FuturesContractPanel 单个期货品种盘面。
type FuturesContractPanel struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Price        float64  `json:"price"`
	ChangePct    float64  `json:"changePct"`
	Volume       float64  `json:"volume"`
	OpenInterest float64  `json:"openInterest"` // 主力合约持仓量（手）
	OIChange     *float64 `json:"oiChange,omitempty"`
	NearPrice    float64  `json:"nearPrice"`  // 近月（主力）
	FarPrice     float64  `json:"farPrice"`   // 远月（次主力）
	CarryPct     float64  `json:"carryPct"`   // (远月-近月)/近月 * 100
	CarryOK      bool     `json:"carryOk"`
	ATRPct       float64  `json:"atrPct"` // ATR14 / 收盘 * 100
	ATROK        bool     `json:"atrOk"`
	Season5y     float64  `json:"season5y"` // 当前月份近 5 年月均收益 %
	SeasonOK     bool     `json:"seasonOk"`
	SeasonMonth  int      `json:"seasonMonth"`
}

// COTPnl 非商业性（管理基金）净持仓。
type COTPnl struct {
	Product string  `json:"product"` // GC/SI/CL
	Name    string  `json:"name"`
	Net     float64 `json:"net"`     // 净持仓（手）
	Percent float64 `json:"percent"` // 占持仓量 %
	Date    string  `json:"date"`
}

// InventoryItem 交易所库存。
type InventoryItem struct {
	Exchange string  `json:"exchange"`
	Product  string  `json:"product"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Change   *float64 `json:"change,omitempty"` // 与上一期之差
	Date     string  `json:"date"`
}

// CommodityFuturesPanel 期货盘面面板一次调用一屏。
type CommodityFuturesPanel struct {
	Contracts            []FuturesContractPanel `json:"contracts"`
	GoldSilverRatio      *float64               `json:"goldSilverRatio,omitempty"`
	GoldSilverPercentile *float64               `json:"goldSilverPercentile,omitempty"` // 5 年分位 0-100
	Inventories          []InventoryItem        `json:"inventories"`
	COT                  []COTPnl               `json:"cot"`
	Failed               []string               `json:"failed"` // 未获取到的数据点名
	FetchedAt            string                 `json:"fetchedAt"`
}

// ---------- 缓存 ----------

var panelCache = struct {
	sync.Mutex
	v  *CommodityFuturesPanel
	at time.Time
}{}

const panelTTL = 60 * time.Second

// ---------- 东财 clist 板块 ----------

type boardRow struct {
	Code      string
	Name      string
	Price     float64
	ChangePct float64
	Volume    float64
	OpenInt   float64
}

var boardMu sync.Mutex
var boardCache = map[string][]boardRow{} // market#pn -> rows（含 m 主连 / s 次主连 / 各月）
var boardCacheAt = time.Time{}

const boardTTL = 60 * time.Second

// fetchBoardPage 拉取一个板块页。返回是否成功与行数。
func fetchBoardPage(market string, pn int, wantPrefixes []string) ([]boardRow, bool) {
	pageKey := fmt.Sprintf("%s#%d", market, pn)
	boardMu.Lock()
	if time.Since(boardCacheAt) < boardTTL {
		if rows, ok := boardCache[pageKey]; ok {
			boardMu.Unlock()
			return filterBoardRows(rows, wantPrefixes), true
		}
	}
	boardMu.Unlock()

	url := fmt.Sprintf(
		"%%s/api/qt/clist/get?pn=%d&pz=100&po=1&np=1&fltt=2&invt=2&fid=f12&fs=%s&fields=f12,f14,f2,f3,f5,f108",
		pn, market,
	)
	type rawRow struct {
		F2   interface{} `json:"f2"`
		F3   interface{} `json:"f3"`
		F5   interface{} `json:"f5"`
		F108 interface{} `json:"f108"`
		F12  string      `json:"f12"`
		F14  string      `json:"f14"`
	}
	var body []byte
	fetchErr := emFallbackFetch(emQuoteHosts(), &emQuoteHostIdx, 2, func(host string) error {
		u := fmt.Sprintf(url, strings.TrimSuffix(host, "/"))
		r, e := emHTTP11Client(u, 15*time.Second).R().
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
			SetHeader("Referer", "https://quote.eastmoney.com/").
			Get(u)
		if e != nil {
			return e
		}
		if r.StatusCode() != 200 {
			return fmt.Errorf("HTTP %d", r.StatusCode())
		}
		b := r.Body()
		var payload struct {
			RC   int `json:"rc"`
			Data struct {
				Total int    `json:"total"`
				Diff  []rawRow `json:"diff"`
			} `json:"data"`
		}
		if e := json.Unmarshal(b, &payload); e != nil {
			return e
		}
		if payload.RC != 0 || len(payload.Data.Diff) == 0 {
			return fmt.Errorf("rc=%d no rows", payload.RC)
		}
		body = b
		return nil
	})
	if fetchErr != nil {
		return nil, false
	}

	var parsed struct {
		RC   int `json:"rc"`
		Data struct {
			Total int    `json:"total"`
			Diff  []rawRow `json:"diff"`
		} `json:"data"`
	}
	if e := json.Unmarshal(body, &parsed); e != nil {
		return nil, false
	}
	rows := make([]boardRow, 0, len(parsed.Data.Diff))
	for _, rr := range parsed.Data.Diff {
		rows = append(rows, boardRow{
			Code:      rr.F12,
			Name:      rr.F14,
			Price:     boardFloat(rr.F2),
			ChangePct: boardFloat(rr.F3),
			Volume:    boardFloat(rr.F5),
			OpenInt:   boardFloat(rr.F108),
		})
	}

	boardMu.Lock()
	boardCache[pageKey] = rows
	boardCacheAt = time.Now()
	boardMu.Unlock()
	return filterBoardRows(rows, wantPrefixes), true
}

func filterBoardRows(rows []boardRow, prefixes []string) []boardRow {
	out := make([]boardRow, 0, 8)
	for _, r := range rows {
		for _, p := range prefixes {
			if strings.HasPrefix(r.Code, p) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func boardFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

// ---------- 主力/次主力识别 ----------

// mainPair 返回某品种的主力(m)与次主力(s)行。
func mainPair(rows []boardRow, prefix string) (near boardRow, far boardRow, found bool) {
	for _, r := range rows {
		if r.Code == prefix+"m" {
			near = r
		}
		if r.Code == prefix+"s" {
			far = r
		}
	}
	return near, far, near.Price > 0
}

// findBoardContract 在板块行中找品种的主连/次主连，缺失时翻页探测（SHFE 分页上限 100/页）。
func findBoardContract(market string, prefix string, maxPages int) (near, far boardRow, ok bool) {
	want := []string{prefix + "m", prefix + "s", prefix}
	for pn := 1; pn <= maxPages; pn++ {
		rows, okPage := fetchBoardPage(market, pn, want)
		if !okPage {
			continue
		}
		n, f, found := mainPair(rows, prefix)
		if found {
			return n, f, true
		}
		hasPrefix := false
		for _, r := range rows {
			if strings.HasPrefix(r.Code, prefix) {
				hasPrefix = true
				break
			}
		}
		if !hasPrefix && pn < maxPages {
			continue // 该页无此品种前缀，继续下一页
		}
		if !found && hasPrefix {
			return n, f, false
		}
	}
	return boardRow{}, boardRow{}, false
}

// ---------- Sina 持仓回退 ----------

// fetchSinaFuturesOI 从 Sina nf_ 报价取主力持仓量（parts[13]）与成交量（parts[14]）。
func fetchSinaFuturesOI(code string) (oi, volume float64, found bool) {
	sinaSymbol := "nf_" + code + "0"
	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), sinaSymbol)
	resp, err := CreateHTTPClientWithTimeout(10 * time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return 0, 0, false
	}
	body := GB18030ToUTF8(resp.Body())
	startIdx := strings.Index(body, "\"")
	endIdx := strings.LastIndex(body, "\"")
	if startIdx < 0 || endIdx <= startIdx {
		return 0, 0, false
	}
	parts := strings.Split(body[startIdx+1:endIdx], ",")
	if len(parts) < 15 {
		return 0, 0, false
	}
	oi, _ = strconv.ParseFloat(strings.TrimSpace(parts[13]), 64)
	volume, _ = strconv.ParseFloat(strings.TrimSpace(parts[14]), 64)
	return oi, volume, oi > 0
}

// ---------- 增仓快照 ----------

// oiSnapshotFile 持仓快照文件（与 data/stock.db 同级相对路径约定）。可用变量便于单测重定向。
var oiSnapshotFile = "data/commodity_oi_snapshots.json"

type oiSnapshot struct {
	Date string  `json:"d"`
	OI   float64 `json:"o"`
}

var oiSnapMu sync.Mutex

// updateOISnapshot 更新 OI 快照并返回增仓（上一交易日最后快照 → 当前值）。
// 首次观测该品种时无增仓（nil）。
func updateOISnapshot(code, quoteDate string, oi float64) *float64 {
	oiSnapMu.Lock()
	defer oiSnapMu.Unlock()

	file := make(map[string]oiSnapshot)
	if b, err := os.ReadFile(oiSnapshotFile); err == nil {
		_ = json.Unmarshal(b, &file)
	}
	prev, exists := file[code]

	var delta *float64
	if exists && prev.OI > 0 {
		d := oi - prev.OI
		delta = &d
	}

	file[code] = oiSnapshot{Date: quoteDate, OI: oi}

	if err := os.MkdirAll(filepath.Dir(oiSnapshotFile), 0o755); err == nil {
		b, _ := json.MarshalIndent(file, "", "  ")
		_ = os.WriteFile(oiSnapshotFile, b, 0o644)
	}
	return delta
}

// ---------- ATR / 季节性 ----------

// atrPct 计算 ATR(n)/收盘 百分比；bars 须按时间升序、≥ n+1 根。
func atrPct(bars []HighLowClose, n int) (float64, bool) {
	if len(bars) < n+1 {
		return 0, false
	}
	trs := make([]float64, 0, len(bars)-1)
	for i := 1; i < len(bars); i++ {
		h, l, c := bars[i].High, bars[i].Low, bars[i].Close
		pc := bars[i-1].Close
		if h <= 0 || l <= 0 || c <= 0 || pc <= 0 {
			return 0, false
		}
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		trs = append(trs, tr)
	}
	window := trs[len(trs)-n:]
	sum := 0.0
	for _, tr := range window {
		sum += tr
	}
	atr := sum / float64(len(window))
	last := bars[len(bars)-1].Close
	if last <= 0 {
		return 0, false
	}
	return atr / last * 100, true
}

// monthAvgs 近 years 年各日历月的"月内总收益"平均 %（标准季节性：月末收盘/月初收盘-1，
// 按年份样本取均值）。每个日历月需 ≥3 个年度样本，且至少 8 个月有成样才可用。
func monthAvgs(kb []datasource.KLineBar, years int) (map[int]float64, bool) {
	if years <= 0 {
		years = 5
	}
	cutoff := time.Now().AddDate(-years, 0, 0)
	type acc struct {
		sum float64
		n   int
	}
	byMonth := map[int]*acc{}

	type period struct{ first, last float64 }
	finish := func(key int, p period) { // key = year*12 + month(1-12)
		if p.first <= 0 || p.last <= 0 {
			return
		}
		month := (key-1)%12 + 1
		a, ok := byMonth[month]
		if !ok {
			a = &acc{}
			byMonth[month] = a
		}
		a.sum += (p.last / p.first - 1) * 100
		a.n++
	}

	curKey := 0
	cur := period{}
	for _, b := range kb {
		if b.Time.Before(cutoff) || b.Close <= 0 || b.Time.IsZero() {
			continue
		}
		key := b.Time.Year()*12 + int(b.Time.Month())
		if curKey == 0 {
			curKey = key
			cur = period{first: b.Close, last: b.Close}
			continue
		}
		if key != curKey {
			finish(curKey, cur)
			curKey = key
			cur = period{first: b.Close, last: b.Close}
			continue
		}
		cur.last = b.Close
	}
	if curKey != 0 {
		finish(curKey, cur)
	}

	out := make(map[int]float64)
	for m, a := range byMonth {
		if a.n >= 3 {
			out[m] = a.sum / float64(a.n)
		}
	}
	if len(out) < 8 {
		return nil, false
	}
	return out, true
}

// ---------- 金银比 ----------

func goldSilverStats(ka, ks []datasource.KLineBar) (ratio *float64, pctile *float64) {
	if len(ka) < 2 || len(ks) < 2 {
		return nil, nil
	}
	na, nk := len(ka), len(ks)
	n := na
	if nk < n {
		n = nk
	}
	if n < 2 {
		return nil, nil
	}
	g := ka[na-n:]
	s := ks[nk-n:]
	ratios := make([]float64, 0, n)
	for i := range g {
		if g[i].Close > 0 && s[i].Close > 0 {
			ratios = append(ratios, g[i].Close/s[i].Close)
		}
	}
	if len(ratios) < 50 {
		return nil, nil
	}
	cur := ratios[len(ratios)-1]
	r := cur
	// 5 年 ≈ 5*244 交易日的分位
	window := ratios
	if len(window) > 5*244 {
		window = window[len(window)-5*244:]
	}
	sorted := make([]float64, len(window))
	copy(sorted, window)
	sort.Float64s(sorted)
	rank := sort.SearchFloat64s(sorted, cur)
	pct := float64(rank) / float64(len(sorted)) * 100
	pctile = &pct
	return &r, pctile
}

// ---------- CFTC COT ----------
// 数据源：CFTC 官方 Socrata JSON（dataset 6dca-aqww，"Legacy - Futures Only"，含 Noncommercial 口径）。
// 子代理实测验证：GC=088691 / SI=084691 可用；CL 用 090548 尽力而为（限流/缺行时仅缺该项）。
// 字段：report_date_as_yyyy_mm_dd、open_interest_all、noncomm_positions_long_all、
//       noncomm_positions_short_all。周度（周五约 15:30 ET 发布，数据截至周二）。

var cotProductCodes = []struct {
	Code    string
	Name    string
	MktCode string
}{
	{"GC", "黄金", "088691"},
	{"SI", "白银", "084691"},
	{"CL", "WTI原油", "090548"},
}

func fetchCOTOne(mktCode, name string, timeout time.Duration) *COTPnl {
	url := fmt.Sprintf(
		"https://publicreporting.cftc.gov/resource/6dca-aqww.json?cftc_contract_market_code=%s&$order=report_date_as_yyyy_mm%%20DESC&$limit=1",
		mktCode,
	)
	resp, err := CreateHTTPClientWithTimeout(timeout).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Referer", "https://www.cftc.gov/").
		Get(url)
	if err != nil {
		return nil
	}
	body := resp.Body()
	var rows []struct {
		ReportDate string `json:"report_date_as_yyyy_mm_dd"`
		OI         int64  `json:"open_interest_all"`
		NL         int64  `json:"noncomm_positions_long_all"`
		NS         int64  `json:"noncomm_positions_short_all"`
	}
	if e := json.Unmarshal(body, &rows); e != nil {
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	r0 := rows[0]
	if r0.OI <= 0 {
		return nil
	}
	date := r0.ReportDate
	if len(date) >= 10 {
		date = date[:10]
	}
	net := float64(r0.NL - r0.NS)
	return &COTPnl{
		Product: strings.ToUpper(name),
		Name:    name,
		Net:     net,
		Percent: net / float64(r0.OI) * 100,
		Date:    date,
	}
}

// fetchCOT 拉取 GC/SI/CL 非商业净持仓。失败/限流的产品跳过（调用方标记 Failed）。
func fetchCOT(timeout time.Duration) []COTPnl {
	out := make([]COTPnl, 0, len(cotProductCodes))
	for _, p := range cotProductCodes {
		if v := fetchCOTOne(p.MktCode, p.Name, timeout); v != nil {
			v.Product = p.Code
			out = append(out, *v)
		}
	}
	return out
}

var cotMu sync.Mutex
var cotCache = struct {
	v   []COTPnl
	ok  bool
	at  time.Time
	err bool
}{at: time.Time{}}

// getCOTCached 6h TTL，失败 1h 内不重试。
func getCOTCached() []COTPnl {
	cotMu.Lock()
	if (cotCache.ok || cotCache.err) && time.Since(cotCache.at) < 6*time.Hour {
		cotMu.Unlock()
		if cotCache.err {
			return nil
		}
		return cotCache.v
	}
	// 失败后的 1h 重试窗口
	if cotCache.err && time.Since(cotCache.at) < time.Hour {
		cotMu.Unlock()
		return nil
	}
	cotMu.Unlock()

	v := fetchCOT(25 * time.Second)
	cotMu.Lock()
	if v == nil {
		cotCache.v, cotCache.ok, cotCache.err, cotCache.at = nil, false, true, time.Now()
	} else {
		cotCache.v, cotCache.ok, cotCache.err, cotCache.at = v, true, false, time.Now()
	}
	cotMu.Unlock()
	if v == nil {
		return nil
	}
	return v
}

// ---------- 库存 ----------
// 数据源：东财数据中心 RPT_FUTU_STOCKDATA（子代理实测 2026-09-07 验证）：
// 日频期货库存/仓单，AU/AG 齐全。字段 ON_WARRANT_NUM（仓单数量）、ADDCHANGE（日增减）、
// TRADE_DATE、UNIT（千克）。官方校源：SHFE 官网 stockdata/…/shfe/{au,ag}.html（数字三方一致）。

func fetchInventories(timeout time.Duration) []InventoryItem {
	out := make([]InventoryItem, 0, 2)
	names := map[string]string{"AU": "黄金", "AG": "白银"}
	for _, code := range []string{"AU", "AG"} {
		url := fmt.Sprintf(
			"https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_FUTU_STOCKDATA&columns=ALL&filter=(SECURITY_CODE%%3D%%22%s%%22)&pageNumber=1&pageSize=2&sortColumns=TRADE_DATE&sortTypes=-1&source=WEB&client=WEB",
			code,
		)
		resp, err := CreateHTTPClientWithTimeout(timeout).R().
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
			SetHeader("Referer", "https://data.eastmoney.com/").
			Get(url)
		if err != nil {
			continue
		}
		var payload struct {
			Success bool    `json:"success"`
			Data    struct {
				Count  int `json:"count"`
				Result []struct {
					SecurityCode   string  `json:"SECURITY_CODE"`
					TradeDate      string  `json:"TRADE_DATE"`
					OnWarrantNum   float64 `json:"ON_WARRANT_NUM"`
					AddChange      float64 `json:"ADDCHANGE"`
					Unit           string  `json:"UNIT"`
				} `json:"result"`
			} `json:"data"`
		}
		if e := json.Unmarshal(resp.Body(), &payload); e != nil {
			continue
		}
		if !payload.Success || len(payload.Data.Result) == 0 {
			continue
		}
		cur := payload.Data.Result[0]
		if cur.OnWarrantNum <= 0 {
			continue
		}
		item := InventoryItem{
			Exchange: "SHFE",
			Product:  names[code],
			Value:    cur.OnWarrantNum / 1000, // 千克 → 吨
			Unit:     "吨",
			Date:     cur.TradeDate,
		}
		if cur.AddChange != 0 {
			ch := cur.AddChange / 1000
			item.Change = &ch
		} else if len(payload.Data.Result) > 1 && payload.Data.Result[1].OnWarrantNum > 0 {
			ch := cur.OnWarrantNum - payload.Data.Result[1].OnWarrantNum
			ch /= 1000
			item.Change = &ch
		}
		out = append(out, item)
	}
	return out
}

// ---------- 主构建 ----------

var commodityFuturesMeta = []struct {
	Code     string
	Name     string
	Market   string // clist fs 板块
	Prefix   string // 合约前缀
	KLineCode string // ATR/季节性 K线源
}{
	{Code: "AU", Name: "沪金", Market: "m:113", Prefix: "au", KLineCode: "AU"},
	{Code: "AG", Name: "沪银", Market: "m:113", Prefix: "ag", KLineCode: "AG"},
	{Code: "SC", Name: "原油", Market: "m:142", Prefix: "sc", KLineCode: "SC"},
}

func buildFuturesPanel() *CommodityFuturesPanel {
	panel := &CommodityFuturesPanel{
		Contracts:   make([]FuturesContractPanel, 0, 3),
		Inventories: make([]InventoryItem, 0),
		COT:         make([]COTPnl, 0),
		Failed:      make([]string, 0),
		FetchedAt:   time.Now().Format(time.RFC3339),
	}
	failedMu := sync.Mutex{}
	markFailed := func(name string) {
		failedMu.Lock()
		panel.Failed = append(panel.Failed, name)
		failedMu.Unlock()
	}

	api := NewCommodityApi()
	now := time.Now()

	for _, meta := range commodityFuturesMeta {
		// 1) 东财板块：主力/次主力 + 持仓
		near, far, ok := findBoardContract(meta.Market, meta.Prefix, 3)
		c := FuturesContractPanel{
			Code:          meta.Code,
			Name:          meta.Name,
			SeasonMonth:   int(now.Month()),
		}
		if ok {
			c.Price = near.Price
			c.ChangePct = near.ChangePct
			c.Volume = near.Volume
			c.OpenInterest = near.OpenInt
			if far.Price > 0 {
				c.NearPrice = near.Price
				c.FarPrice = far.Price
				c.CarryPct = (far.Price - near.Price) / near.Price * 100
				c.CarryOK = true
			}
		}
		// 2) Sina 回退（持仓量）
		if c.OpenInterest <= 0 {
			oi, vol, found := fetchSinaFuturesOI(meta.Code)
			if found {
				c.OpenInterest = oi
				if c.Volume <= 0 {
					c.Volume = vol
				}
				if c.Price <= 0 {
					q, qerr := api.GetQuote(meta.Code)
					if qerr == nil && q != nil {
						c.Price = q.Price
						c.ChangePct = q.ChangePct
					}
				}
			} else {
				markFailed(meta.Code + ":open_interest")
			}
		}
		// 3) 增仓（本地快照差值）
		if c.OpenInterest > 0 {
			quoteDate := now.Format("2006-01-02")
			delta := updateOISnapshot(meta.Code, quoteDate, c.OpenInterest)
			c.OIChange = delta
		}
		// 4) ATR / 季节性（日 K，本地计算）
		bars, kerr := api.GetKLine(meta.KLineCode, "day", 260)
		if kerr == nil {
			hlc := klineToHLC(bars)
			if atr, aok := atrPct(hlc, 14); aok {
				c.ATRPct = atr
				c.ATROK = true
			}
		}
		bars5y, kerr2 := api.GetKLine(meta.KLineCode, "day", 260*5+20)
		if kerr2 == nil && len(bars5y) >= 200 {
			if avgs, mok := monthAvgs(bars5y, 5); mok {
				if v, has := avgs[int(now.Month())]; has {
					c.Season5y = v
					c.SeasonOK = true
				}
			}
		}
		panel.Contracts = append(panel.Contracts, c)
	}

	// 金银比 + 分位
	ka, errGa := api.GetKLine("XAUUSD", "day", 260*5+20)
	ks, errGs := api.GetKLine("XAGUSD", "day", 260*5+20)
	if errGa == nil && errGs == nil {
		r, p := goldSilverStats(ka, ks)
		panel.GoldSilverRatio = r
		panel.GoldSilverPercentile = p
	} else {
		markFailed("gold_silver_ratio")
	}

	// COT（6h 缓存；拉不到只标记，不重试风暴）
	if v := getCOTCached(); v != nil {
		panel.COT = v
	} else {
		markFailed("cot")
	}

	// 库存（东财 RPT_FUTU_STOCKDATA；拉不到只标记，不重试风暴）
	if inv := fetchInventories(20 * time.Second); len(inv) > 0 {
		panel.Inventories = inv
	} else {
		markFailed("inventories")
	}

	return panel
}

// klineToHLC []KLineBar → []HighLowClose（升序透传）。
func klineToHLC(bars []datasource.KLineBar) []HighLowClose {
	out := make([]HighLowClose, 0, len(bars))
	for _, b := range bars {
		out = append(out, HighLowClose{High: b.High, Low: b.Low, Close: b.Close})
	}
	return out
}

// GetCommodityFuturesPanel 面板入口（60s 进程内缓存）。
func GetCommodityFuturesPanel() (*CommodityFuturesPanel, error) {
	panelCache.Lock()
	if panelCache.v != nil && time.Since(panelCache.at) < panelTTL {
		panelCache.Unlock()
		return panelCache.v, nil
	}
	panelCache.Unlock()

	v := buildFuturesPanel()
	panelCache.Lock()
	panelCache.v = v
	panelCache.at = time.Now()
	panelCache.Unlock()
	return v, nil
}
