# go-stock × Yahoo Finance 深度集成方案

## 一、现状分析

### 1.1 现有 Yahoo Finance 集成（已存在但能力有限）

go-stock 已有 `backend/data/yahoo_finance_api.go`，但当前仅支持：
- **商品期货**：黄金(GC=F)、白银(SI=F)、原油(CL=F/BZ=F)
- **少量 ETF**：TLT(美债)、TIP(抗通胀债)
- **个别 LOF**：国投白银(161226.SZ)

**不支持**：全球股票、全球指数、外汇、加密货币、基本面数据

### 1.2 Yahoo Finance 实际能力（被严重低估）

Yahoo Finance v8 Chart API 是一个**全球级别的免费数据源**，可覆盖：

| 数据类型 | 覆盖范围 | 频率 | 历史深度 |
|---------|---------|------|---------|
| **全球股票实时行情** | 全球60+交易所 | 15分钟延迟(美股)/实时(部分) | - |
| **全球股票K线** | 全球 | 日/周/月/分钟 | 数十年 |
| **全球指数** | 标普/纳指/道指/恒指/日经/DAX/CAC等 | 日/周/月 | 数十年 |
| **外汇** | 170+货币对 | 日K | 多年 |
| **加密货币** | BTC/ETH/主流币 | 日K | 多年 |
| **商品期货** | 黄金/白银/原油/铜/农产品 | 日K | 多年 |
| **ETF/基金** | 全球ETF | 日K | 多年 |
| **基本面数据** | 财报/PE/PB/ROE/分红 | 季度/年度 | 多年 |

**关键优势**：完全免费、无需API Key、全球覆盖、数据质量高

---

## 二、核心问题：为什么现有集成这么弱？

```go
// 现有代码的限制：resolveSymbol 只认识商品期货
var yahooCommoditySymbols = map[string]string{
    "XAUUSD": "GC=F",
    "XAGUSD": "SI=F",
    // ... 只有 10 个映射
}

func (y *YahooFinanceApi) resolveSymbol(code string) (string, error) {
    if sym, ok := yahooCommoditySymbols[code]; ok {
        return sym, nil
    }
    return "", fmt.Errorf("Yahoo Finance 不支持品种: %s", code)  // ❌ 大量股票被拒之门外
}
```

**解决方案**：重写 `resolveSymbol`，支持 go-stock 所有市场代码到 Yahoo Finance 格式的智能映射。

---

## 三、完整集成方案

### 3.1 第一步：扩展现有 `yahoo_finance_api.go`

**替换 `resolveSymbol` 函数**，支持全球股票代码映射：

```go
// === 新增：Yahoo Finance 全局股票代码映射表 ===

// YahooSymbolResolver 将 go-stock 内部代码转换为 Yahoo Finance 格式
type YahooSymbolResolver struct{}

// go-stock 内部代码格式：
//   A股: "sh600519" / "sz000001" / "bj430047"
//   港股: "hk00700" / "hk09988"
//   美股: "usAAPL" / "usTSLA" / "gb_AAPL" (gb_ 是新浪格式)
//   指数: "us_SPX" / "hk_HSI" / "jp_N225"
//   商品: "XAUUSD" / "USCL"
// Yahoo Finance 代码格式：
//   A股: "600519.SS" / "000001.SZ" / "430047.BJ"
//   港股: "0700.HK" / "9988.HK"
//   美股: "AAPL" / "TSLA" (无需后缀)
//   指数: "^GSPC" / "^HSI" / "^N225"
//   商品: "GC=F" / "CL=F"

func (r *YahooSymbolResolver) Resolve(code string) (string, error) {
    code = strings.ToLower(strings.TrimSpace(code))
    
    // 1. 美股 (usXXX / gb_XXX)
    if strings.HasPrefix(code, "us") || strings.HasPrefix(code, "gb_") {
        symbol := strings.TrimPrefix(strings.TrimPrefix(code, "us"), "gb_")
        symbol = strings.TrimSpace(symbol)
        symbol = strings.ToUpper(symbol)
        // 移除可能的后缀如 .US
        symbol = strings.TrimSuffix(symbol, ".US")
        return symbol, nil
    }
    
    // 2. 港股 (hkXXXX)
    if strings.HasPrefix(code, "hk") {
        symbol := strings.TrimPrefix(code, "hk")
        symbol = strings.TrimSpace(symbol)
        // 港股代码统一为 5 位，不足前面补 0
        symbol = fmt.Sprintf("%05s", symbol)
        return symbol + ".HK", nil
    }
    
    // 3. A股上交所 (shXXXXXX)
    if strings.HasPrefix(code, "sh") {
        symbol := strings.TrimPrefix(code, "sh")
        return strings.ToUpper(symbol) + ".SS", nil  // Yahoo 用 .SS 表示上交所
    }
    
    // 4. A股深交所 (szXXXXXX)
    if strings.HasPrefix(code, "sz") {
        symbol := strings.TrimPrefix(code, "sz")
        return strings.ToUpper(symbol) + ".SZ", nil
    }
    
    // 5. A股北交所 (bjXXXXXX)
    if strings.HasPrefix(code, "bj") {
        symbol := strings.TrimPrefix(code, "bj")
        return strings.ToUpper(symbol) + ".BJ", nil
    }
    
    // 6. 全球指数
    if idxSym, ok := globalIndexMap[code]; ok {
        return idxSym, nil
    }
    
    // 7. 商品期货
    if sym, ok := yahooCommoditySymbols[code]; ok {
        return sym, nil
    }
    
    // 8. 直接传入的 Yahoo 格式（如 "AAPL", "GC=F", "^GSPC"）
    // 如果已经是大写的标准格式，直接返回
    if isValidYahooSymbol(code) {
        return strings.ToUpper(code), nil
    }
    
    return "", fmt.Errorf("无法将代码 %s 映射到 Yahoo Finance 格式", code)
}

// 全球指数映射表
var globalIndexMap = map[string]string{
    // 美洲
    "us_SPX":  "^GSPC",   // 标普500
    "us_NDX":  "^IXIC",   // 纳斯达克
    "us_DJI":  "^DJI",    // 道琼斯
    "us_RUT":  "^RUT",    // 罗素2000
    "us_VIX":  "^VIX",    // 波动率指数
    // 亚洲
    "hk_HSI":  "^HSI",    // 恒生指数
    "hk_HSTECH": "^HSTECH", // 恒生科技
    "jp_N225": "^N225",   // 日经225
    "kr_KOSPI": "^KS11",  // 韩国KOSPI
    "in_SENSEX": "^BSESN", // 印度Sensex
    "sg_STI":  "^STI",    // 新加坡海峡时报
    // 欧洲
    "uk_FTSE": "^FTSE",   // 英国富时100
    "de_DAX":  "^GDAXI",  // 德国DAX
    "fr_CAC":  "^FCHI",   // 法国CAC40
    "eu_STOXX50": "^STOXX50E", // 欧洲斯托克50
    "it_FTSEMIB": "^FTSEMIB.MI", // 意大利富时MIB
    "es_IBEX": "^IBEX",   // 西班牙IBEX
    // 其他
    "au_ASX":  "^AXJO",   // 澳洲ASX200
    "br_Bovespa": "^BVSP", // 巴西Bovespa
    "mx_IPC":  "^MXX",    // 墨西哥IPC
}

// 新增商品映射
var yahooCommoditySymbols = map[string]string{
    // 贵金属
    "XAUUSD": "GC=F",  // COMEX 黄金期货
    "XAGUSD": "SI=F",  // COMEX 白银期货
    "XAU":    "GC=F",
    "XAG":    "SI=F",
    "GC":     "GC=F",
    "SI":     "SI=F",
    // 能源
    "USCL":   "CL=F",  // WTI 原油
    "USCO":   "BZ=F",  // 布伦特原油
    "CL":     "CL=F",
    "BZ":     "BZ=F",
    // 工业金属
    "CU":     "HG=F",  // 铜
    "HG":     "HG=F",
    // 农产品
    "ZC":     "ZC=F",  // 玉米
    "ZW":     "ZW=F",  // 小麦
    "ZS":     "ZS=F",  // 大豆
    // 国内期货映射
    "AU":     "GC=F",
    "AG":     "SI=F",
    "SC":     "CL=F",
    // 基金
    "161226": "161226.SZ",
    // 宏观ETF
    "TLT":    "TLT",
    "TIP":    "TIP",
    "GLD":    "GLD",   // 黄金ETF
    "SLV":    "SLV",   // 白银ETF
    "SPY":    "SPY",   // 标普500ETF
    "QQQ":    "QQQ",   // 纳斯达克100ETF
}

func isValidYahooSymbol(s string) bool {
    // 简单校验：包含字母，且符合 Yahoo 格式
    s = strings.ToUpper(s)
    // 美股直接是字母
    if matched, _ := regexp.MatchString(`^[A-Z]+$`, s); matched {
        return true
    }
    // 带后缀的格式
    if matched, _ := regexp.MatchString(`^[A-Z0-9]+\.[A-Z]+$`, s); matched {
        return true
    }
    // 指数格式
    if strings.HasPrefix(s, "^") {
        return true
    }
    // 期货格式
    if strings.HasSuffix(s, "=F") {
        return true
    }
    return false
}
```

### 3.2 第二步：让 YahooFinanceApi 实现 Provider 接口

**重写 `YahooFinanceApi` 结构体**，实现 `QuoteProvider` 和 `KLineProvider`：

```go
// === 重写后的 YahooFinanceApi ===

// YahooFinanceApi 提供 Yahoo Finance 全球数据，实现 datasource Provider 接口。
type YahooFinanceApi struct {
    resolver *YahooSymbolResolver
}

func NewYahooFinanceApi() *YahooFinanceApi {
    return &YahooFinanceApi{
        resolver: &YahooSymbolResolver{},
    }
}

// --- DataSourceProvider 接口 ---

func (y *YahooFinanceApi) Name() string    { return "yahoo" }
func (y *YahooFinanceApi) Priority() int   { return 25 }  // 优先级：TDX(10) < EastMoney(20) < Yahoo(25) < Sina(30)
func (y *YahooFinanceApi) Available(ctx context.Context) bool {
    return true // Yahoo Finance 始终可用（无需API Key）
}

// --- QuoteProvider 接口 ---

func (y *YahooFinanceApi) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
    symbol, err := y.resolver.Resolve(code)
    if err != nil {
        return nil, err
    }
    
    url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", url2.QueryEscape(symbol))
    body, err := y.yahooFetch(url)
    if err != nil {
        return nil, fmt.Errorf("yahoo quote fetch: %w", err)
    }
    
    chart, err := parseYahooChart(body)
    if err != nil {
        return nil, fmt.Errorf("yahoo quote parse: %w", err)
    }
    
    result := chart.Chart.Result[0]
    meta := result.Meta
    
    // 取最新价格
    price := meta.RegularMarketPrice
    if price == 0 && len(result.Indicators.Quote) > 0 {
        q := result.Indicators.Quote[0]
        for i := len(q.Close) - 1; i >= 0; i-- {
            if q.Close[i] != nil {
                price = *q.Close[i]
                break
            }
        }
    }
    if price == 0 {
        return nil, fmt.Errorf("yahoo quote: no price for %s", symbol)
    }
    
    // 计算涨跌幅
    prevClose := meta.PreviousClose
    if prevClose == 0 && meta.ChartPreviousClose != 0 {
        prevClose = meta.ChartPreviousClose
    }
    change := price - prevClose
    changePct := 0.0
    if prevClose != 0 {
        changePct = change / prevClose * 100
    }
    
    // 取当日最高最低价
    var high, low float64
    if len(result.Indicators.Quote) > 0 {
        q := result.Indicators.Quote[0]
        for i := 0; i < len(q.High); i++ {
            if q.High[i] != nil && *q.High[i] > high {
                high = *q.High[i]
            }
            if q.Low[i] != nil && (low == 0 || *q.Low[i] < low) {
                low = *q.Low[i]
            }
        }
    }
    
    // 取成交量
    var volume int64
    if len(result.Indicators.Quote) > 0 {
        q := result.Indicators.Quote[0]
        for i := 0; i < len(q.Volume); i++ {
            if q.Volume[i] != nil {
                volume += *q.Volume[i]
            }
        }
    }
    
    return &datasource.QuoteData{
        Code:      code,
        Name:      meta.Symbol,
        Price:     price,
        Change:    change,
        ChangePct: changePct,
        Volume:    volume,
        High:      high,
        Low:       low,
        Open:      meta.RegularMarketPrice - change, // 近似
        PrevClose: prevClose,
        Time:      time.Now(),
        Extra: map[string]interface{}{
            "currency": meta.Currency,
            "source":   "yahoo",
        },
    }, nil
}

// --- KLineProvider 接口 ---

func (y *YahooFinanceApi) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
    symbol, err := y.resolver.Resolve(code)
    if err != nil {
        return nil, err
    }
    
    interval := "1d"
    switch period {
    case "week":
        interval = "1wk"
    case "month":
        interval = "1mo"
    }
    
    rangeParam := yahooRangeForCount(count, period)
    url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=%s&range=%s", 
        url2.QueryEscape(symbol), interval, rangeParam)
    
    body, err := y.yahooFetch(url)
    if err != nil {
        return nil, fmt.Errorf("yahoo kline fetch: %w", err)
    }
    
    chart, err := parseYahooChart(body)
    if err != nil {
        return nil, fmt.Errorf("yahoo kline parse: %w", err)
    }
    
    bars, err := yahooBarsFromChart(chart, code, count)
    if err != nil {
        return nil, err
    }
    
    return &datasource.KLineData{
        Code:   code,
        Period: period,
        Bars:   bars,
    }, nil
}
```

### 3.3 第三步：注册到 fallback chain

在 `backend/data/datasource/fallback/quote_chain.go` 中新增 Yahoo Provider：

```go
// YahooQuoteProvider wraps Yahoo Finance as global stock fallback.
type YahooQuoteProvider struct {
    api *data.YahooFinanceApi
}

func NewYahooQuoteProvider() *YahooQuoteProvider {
    return &YahooQuoteProvider{api: data.NewYahooFinanceApi()}
}

func (p *YahooQuoteProvider) Name() string                      { return "yahoo" }
func (p *YahooQuoteProvider) Priority() int                     { return 25 }
func (p *YahooQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *YahooQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
    // Yahoo 对美股/全球股票支持更好，对 A股支持有限
    // 优先用于非 A 股代码
    if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
        return nil, fmt.Errorf("yahoo quote: skip A-share %s", code)
    }
    return p.api.GetQuote(ctx, code)
}

// 修改 RegisterQuoteChain
func RegisterQuoteChain(router *datasource.Router) {
    router.RegisterQuoteProvider(&TDXQuoteProvider{})        // Priority 10
    router.RegisterQuoteProvider(&EastMoneyQuoteProvider{})  // Priority 20
    router.RegisterQuoteProvider(NewYahooQuoteProvider())    // Priority 25 ← 新增
    router.RegisterQuoteProvider(&SinaQuoteProvider{})       // Priority 30
}
```

在 `backend/data/datasource/fallback/kline_chain.go` 中新增：

```go
// YahooKLineProvider wraps Yahoo Finance K-line for global stocks.
type YahooKLineProvider struct {
    api *data.YahooFinanceApi
}

func NewYahooKLineProvider() *YahooKLineProvider {
    return &YahooKLineProvider{api: data.NewYahooFinanceApi()}
}

func (p *YahooKLineProvider) Name() string                      { return "yahoo_kline" }
func (p *YahooKLineProvider) Priority() int                     { return 25 }
func (p *YahooKLineProvider) Available(ctx context.Context) bool { return true }

func (p *YahooKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
    // Yahoo 对美股/港股/全球指数K线支持极好，对A股支持有限
    if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
        return nil, fmt.Errorf("yahoo kline: skip A-share %s", code)
    }
    return p.api.GetKLine(ctx, code, period, count)
}

// 修改 RegisterKLineChain
func RegisterKLineChain(router *datasource.Router) {
    router.RegisterKLineProvider(&TDXKLineProvider{})        // Priority 10
    router.RegisterKLineProvider(&EastMoneyKLineProvider{})  // Priority 20
    router.RegisterKLineProvider(NewYahooKLineProvider())    // Priority 25 ← 新增
}
```

### 3.4 第四步：新增基本面数据支持

Yahoo Finance 还能提供丰富的基本面数据，实现 `FundamentalProvider`：

```go
// backend/data/yahoo_fundamental.go（新增文件）
package data

import (
    "context"
    "encoding/json"
    "fmt"
    "go-stock/backend/data/datasource"
    "strconv"
    "strings"
)

// YahooFundamentalApi 提供 Yahoo Finance 基本面数据
type YahooFundamentalApi struct {
    resolver *YahooSymbolResolver
}

func NewYahooFundamentalApi() *YahooFundamentalApi {
    return &YahooFundamentalApi{resolver: &YahooSymbolResolver{}}
}

func (y *YahooFundamentalApi) Name() string    { return "yahoo_fundamental" }
func (y *YahooFundamentalApi) Priority() int   { return 25 }
func (y *YahooFundamentalApi) Available(ctx context.Context) bool { return true }

func (y *YahooFundamentalApi) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
    symbol, err := y.resolver.Resolve(code)
    if err != nil {
        return nil, err
    }
    
    // Yahoo Finance 基本面数据模块
    // 使用 v10 finance/quoteSummary API
    url := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=summaryDetail,defaultKeyStatistics,financialData",
        url2.QueryEscape(symbol))
    
    body, err := NewYahooFinanceApi().yahooFetch(url)
    if err != nil {
        return nil, fmt.Errorf("yahoo fundamental fetch: %w", err)
    }
    
    var result struct {
        QuoteSummary struct {
            Result []struct {
                SummaryDetail        *yahooSummaryDetail        `json:"summaryDetail"`
                DefaultKeyStatistics *yahooDefaultKeyStatistics `json:"defaultKeyStatistics"`
                FinancialData        *yahooFinancialData        `json:"financialData"`
            } `json:"result"`
            Error *struct {
                Code        string `json:"code"`
                Description string `json:"description"`
            } `json:"error"`
        } `json:"quoteSummary"`
    }
    
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, fmt.Errorf("yahoo fundamental parse: %w", err)
    }
    if result.QuoteSummary.Error != nil {
        return nil, fmt.Errorf("yahoo fundamental api error: %s", result.QuoteSummary.Error.Description)
    }
    if len(result.QuoteSummary.Result) == 0 {
        return nil, fmt.Errorf("yahoo fundamental: no data for %s", code)
    }
    
    r := result.QuoteSummary.Result[0]
    
    fd := &datasource.FundamentalData{}
    
    // 从 summaryDetail 提取 PE/PB
    if r.SummaryDetail != nil {
        fd.PE = parseYahooNumber(r.SummaryDetail.TrailingPE)
        fd.PB = parseYahooNumber(r.SummaryDetail.PriceToBook)
    }
    
    // 从 defaultKeyStatistics 提取 ROE
    if r.DefaultKeyStatistics != nil {
        fd.ROE = parseYahooNumber(r.DefaultKeyStatistics.ReturnOnEquity)
    }
    
    // 从 financialData 提取营收/净利润
    if r.FinancialData != nil {
        fd.Revenue = parseYahooNumber(r.FinancialData.TotalRevenue)
        fd.NetProfit = parseYahooNumber(r.FinancialData.NetIncomeToCommon)
        fd.DebtRatio = parseYahooNumber(r.FinancialData.DebtToEquity)
    }
    
    return fd, nil
}

// --- Yahoo 响应结构体 ---

type yahooNumber struct {
    Raw float64 `json:"raw"`
    Fmt string  `json:"fmt"`
}

type yahooSummaryDetail struct {
    TrailingPE    *yahooNumber `json:"trailingPE"`
    PriceToBook   *yahooNumber `json:"priceToBook"`
    MarketCap     *yahooNumber `json:"marketCap"`
    DividendYield *yahooNumber `json:"dividendYield"`
}

type yahooDefaultKeyStatistics struct {
    ReturnOnEquity      *yahooNumber `json:"returnOnEquity"`
    EnterpriseValue     *yahooNumber `json:"enterpriseValue"`
    FloatShares         *yahooNumber `json:"floatShares"`
    SharesOutstanding   *yahooNumber `json:"sharesOutstanding"`
}

type yahooFinancialData struct {
    TotalRevenue      *yahooNumber `json:"totalRevenue"`
    NetIncomeToCommon *yahooNumber `json:"netIncomeToCommon"`
    DebtToEquity      *yahooNumber `json:"debtToEquity"`
    RevenueGrowth     *yahooNumber `json:"revenueGrowth"`
    EarningsGrowth    *yahooNumber `json:"earningsGrowth"`
    CurrentRatio      *yahooNumber `json:"currentRatio"`
    QuickRatio        *yahooNumber `json:"quickRatio"`
}

func parseYahooNumber(n *yahooNumber) float64 {
    if n == nil {
        return 0
    }
    return n.Raw
}
```

### 3.5 第五步：新增分红历史数据

```go
// backend/data/yahoo_dividend.go（新增文件）
package data

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
)

// YahooDividend 表示一次分红记录
type YahooDividend struct {
    Date   time.Time `json:"date"`
    Amount float64   `json:"amount"`
}

// GetDividendHistory 获取股票分红历史
func (y *YahooFinanceApi) GetDividendHistory(code string, years int) ([]YahooDividend, error) {
    symbol, err := y.resolver.Resolve(code)
    if err != nil {
        return nil, err
    }
    
    rangeParam := fmt.Sprintf("%dy", years)
    url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%s&events=div",
        url2.QueryEscape(symbol), rangeParam)
    
    body, err := y.yahooFetch(url)
    if err != nil {
        return nil, err
    }
    
    var result struct {
        Chart struct {
            Result []struct {
                Events *struct {
                    Dividends map[string]struct {
                        Amount float64 `json:"amount"`
                        Date   int64   `json:"date"`
                    } `json:"dividends"`
                } `json:"events"`
            } `json:"result"`
        } `json:"chart"`
    }
    
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, err
    }
    
    if len(result.Chart.Result) == 0 || result.Chart.Result[0].Events == nil {
        return []YahooDividend{}, nil
    }
    
    var dividends []YahooDividend
    for _, div := range result.Chart.Result[0].Events.Dividends {
        dividends = append(dividends, YahooDividend{
            Date:   time.Unix(div.Date, 0),
            Amount: div.Amount,
        })
    }
    
    // 按日期倒序
    for i, j := 0, len(dividends)-1; i < j; i, j = i+1, j-1 {
        dividends[i], dividends[j] = dividends[j], dividends[i]
    }
    
    return dividends, nil
}
```

---

## 四、AI Agent 工具函数扩展

在 `backend/data/tools*.go` 中注册 Yahoo 数据工具：

```go
// backend/data/tools_yahoo.go（新增文件）
package data

import (
    "context"
    "fmt"
    "go-stock/backend/data/datasource"
    "strings"
)

func init() {
    // 注册到全局工具注册表
    registerToolHandler("YahooGlobalQuote", handleYahooGlobalQuote)
    registerToolHandler("YahooGlobalKLine", handleYahooGlobalKLine)
    registerToolHandler("YahooFundamental", handleYahooFundamental)
    registerToolHandler("YahooDividendHistory", handleYahooDividendHistory)
}

func handleYahooGlobalQuote(o *OpenAi, funcArguments string, ctx *ToolContext) error {
    // 解析参数...
    args := parseToolArgs(funcArguments)
    code := args["code"]
    
    router := datasource.GetRouter()
    quote, err := router.GetQuote(context.Background(), code)
    if err != nil {
        return toolError(ctx, "YahooGlobalQuote", fmt.Sprintf("获取 %s 行情失败: %v", code, err))
    }
    
    content := fmt.Sprintf("[%s] %s: 最新价 %.2f, 涨跌 %.2f (%.2f%%), 成交量 %d",
        quote.Code, quote.Name, quote.Price, quote.Change, quote.ChangePct, quote.Volume)
    
    return toolSuccess(ctx, "YahooGlobalQuote", funcArguments, content)
}

func handleYahooGlobalKLine(o *OpenAi, funcArguments string, ctx *ToolContext) error {
    args := parseToolArgs(funcArguments)
    code := args["code"]
    period := args["period"] // day/week/month
    count := parseInt(args["count"], 30)
    
    router := datasource.GetRouter()
    kline, err := router.GetKLine(context.Background(), code, period, count)
    if err != nil {
        return toolError(ctx, "YahooGlobalKLine", fmt.Sprintf("获取 %s K线失败: %v", code, err))
    }
    
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("## %s %sK线数据 (最近%d条)\n\n", code, period, len(kline.Bars)))
    sb.WriteString("| 日期 | 开盘 | 最高 | 最低 | 收盘 | 成交量 |\n")
    sb.WriteString("|------|------|------|------|------|--------|\n")
    
    for _, bar := range kline.Bars {
        sb.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.2f | %.2f | %d |\n",
            bar.Time.Format("2006-01-02"), bar.Open, bar.High, bar.Low, bar.Close, bar.Volume))
    }
    
    return toolSuccess(ctx, "YahooGlobalKLine", funcArguments, sb.String())
}

func handleYahooFundamental(o *OpenAi, funcArguments string, ctx *ToolContext) error {
    args := parseToolArgs(funcArguments)
    code := args["code"]
    
    api := NewYahooFundamentalApi()
    fd, err := api.GetFundamental(context.Background(), code)
    if err != nil {
        return toolError(ctx, "YahooFundamental", fmt.Sprintf("获取 %s 基本面失败: %v", code, err))
    }
    
    content := fmt.Sprintf("## %s 基本面数据\n\n- PE: %.2f\n- PB: %.2f\n- ROE: %.2f%%\n- 营收: %.0f\n- 净利润: %.0f\n- 负债率: %.2f%%",
        code, fd.PE, fd.PB, fd.ROE*100, fd.Revenue, fd.NetProfit, fd.DebtRatio)
    
    return toolSuccess(ctx, "YahooFundamental", funcArguments, content)
}

func handleYahooDividendHistory(o *OpenAi, funcArguments string, ctx *ToolContext) error {
    args := parseToolArgs(funcArguments)
    code := args["code"]
    years := parseInt(args["years"], 5)
    
    api := NewYahooFinanceApi()
    dividends, err := api.GetDividendHistory(code, years)
    if err != nil {
        return toolError(ctx, "YahooDividendHistory", fmt.Sprintf("获取 %s 分红历史失败: %v", code, err))
    }
    
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("## %s 近%d年分红历史\n\n", code, years))
    sb.WriteString("| 日期 | 分红金额 |\n")
    sb.WriteString("|------|----------|\n")
    
    total := 0.0
    for _, d := range dividends {
        sb.WriteString(fmt.Sprintf("| %s | %.4f |\n", d.Date.Format("2006-01-02"), d.Amount))
        total += d.Amount
    }
    sb.WriteString(fmt.Sprintf("\n**累计分红: %.4f**\n", total))
    
    return toolSuccess(ctx, "YahooDividendHistory", funcArguments, sb.String())
}
```

---

## 五、前端展示增强

### 5.1 新增全球股票搜索支持

```typescript
// frontend/src/api/market.ts（新增）

/**
 * 获取 Yahoo Finance 支持的全球指数列表
 */
export async function getGlobalIndexList(): Promise<any> {
  return callApi(StockHandler.GetGlobalIndexList)
}

/**
 * 获取 Yahoo Finance 全球股票行情
 */
export async function getYahooQuote(stockCode: string): Promise<any> {
  return callApi(StockHandler.GetYahooQuote, stockCode)
}

/**
 * 获取 Yahoo Finance K线（支持全球指数/美股/港股/日股/欧股）
 */
export async function getYahooKLine(stockCode: string, period: string = 'day', count: number = 100): Promise<any> {
  return callApi(StockHandler.GetYahooKLine, stockCode, period, count)
}

/**
 * 获取分红历史
 */
export async function getDividendHistory(stockCode: string, years: number = 5): Promise<any> {
  return callApi(StockHandler.GetDividendHistory, stockCode, years)
}
```

### 5.2 全球指数行情卡片（示例）

```vue
<!-- frontend/src/components/GlobalIndexCard.vue -->
<template>
  <n-card :title="indexName" size="small">
    <div class="index-price">
      <span class="price">{{ quote?.price?.toFixed(2) || '--' }}</span>
      <span :class="['change', quote?.change >= 0 ? 'up' : 'down']">
        {{ quote?.change >= 0 ? '+' : '' }}{{ quote?.change?.toFixed(2) }} 
        ({{ quote?.changePct >= 0 ? '+' : '' }}{{ quote?.changePct?.toFixed(2) }}%)
      </span>
    </div>
    <MiniKLine :data="klineData" />
  </n-card>
</template>

<script setup>
// 支持的全球指数代码：
// us_SPX(标普500), us_NDX(纳斯达克), us_DJI(道琼斯), us_VIX(波动率)
// hk_HSI(恒生), hk_HSTECH(恒生科技), jp_N225(日经225)
// uk_FTSE(富时100), de_DAX(德国DAX), fr_CAC(法国CAC40)
const props = defineProps({ code: String, indexName: String })

onMounted(async () => {
  // 通过 Yahoo Finance 获取全球指数实时行情
  quote.value = await getYahooQuote(props.code)
  klineData.value = await getYahooKLine(props.code, 'day', 60)
})
</script>
```

---

## 六、数据覆盖矩阵（集成后）

| 市场/数据 | 实时行情 | K线 | 基本面 | 分红 | 接入状态 |
|----------|---------|-----|--------|------|---------|
| A股 | ✅ 新浪/腾讯 | ✅ TDX/东财 | ⚠️ 有限 | ❌ | 已有 |
| 港股 | ✅ 腾讯 | ✅ 东财 | ⚠️ 有限 | ❌ | 已有 |
| **美股** | ✅ **Yahoo** | ✅ **Yahoo** | ✅ **Yahoo** | ✅ **Yahoo** | **增强** |
| **日股** | ✅ **Yahoo** | ✅ **Yahoo** | ✅ **Yahoo** | ✅ **Yahoo** | **新增** |
| **欧股** | ✅ **Yahoo** | ✅ **Yahoo** | ✅ **Yahoo** | ✅ **Yahoo** | **新增** |
| **全球指数** | ✅ **Yahoo** | ✅ **Yahoo** | - | - | **新增** |
| **外汇** | ✅ **Yahoo** | ✅ **Yahoo** | - | - | **新增** |
| **加密货币** | ✅ **Yahoo** | ✅ **Yahoo** | - | - | **新增** |
| 商品期货 | ✅ Yahoo | ✅ Yahoo | - | - | 已有 |

---

## 七、实施清单

### 必须修改的文件

1. **`backend/data/yahoo_finance_api.go`**
   - [ ] 新增 `YahooSymbolResolver` 结构体和 `Resolve` 方法
   - [ ] 新增 `globalIndexMap` 和扩展 `yahooCommoditySymbols`
   - [ ] 重写 `YahooFinanceApi` 实现 `QuoteProvider`/`KLineProvider` 接口
   - [ ] 新增 `GetDividendHistory` 方法

2. **`backend/data/yahoo_fundamental.go`**（新建）
   - [ ] 实现 `YahooFundamentalApi` 和 `FundamentalProvider` 接口

3. **`backend/data/datasource/fallback/quote_chain.go`**
   - [ ] 新增 `YahooQuoteProvider` 并注册到 chain

4. **`backend/data/datasource/fallback/kline_chain.go`**
   - [ ] 新增 `YahooKLineProvider` 并注册到 chain

5. **`backend/data/datasource/fallback/fundamental_chain.go`**（新建或修改）
   - [ ] 新增 `YahooFundamentalProvider` 并注册到 chain

6. **`backend/data/tools_yahoo.go`**（新建）
   - [ ] 实现 4 个 AI Agent 工具函数

7. **前端**
   - [ ] 新增全球指数行情组件
   - [ ] 新增分红历史展示页面
   - [ ] 股票搜索支持全球代码

### 无需修改的文件（复用已有架构）

- `backend/data/datasource/router.go` — 已有 fallback 和缓存机制
- `backend/data/datasource/cache.go` — 已有缓存层
- `backend/data/datasource/kline_store.go` — 已有 K线持久化
- `frontend/src/api/client.ts` — 已有 Wails 调用封装

---

## 八、成本与限制

| 项目 | 说明 |
|------|------|
| **费用** | 完全免费，无需 API Key |
| **限流** | 约 2000 次/小时/IP，超出后返回 429 |
| **延迟** | 美股 15 分钟延迟（免费版）；非美股可能实时 |
| **A股覆盖** | Yahoo 对 A股数据不完整，不建议作为主要 A股数据源 |
| **最佳场景** | 美股/港股/全球指数/日股/欧股/外汇/加密货币 |
| **降级策略** | 已有 PowerShell WinHTTP fallback 可绕过 TLS 指纹限流 |

---

## 九、总结

**Yahoo Finance 是 go-stock 全球数据扩展的「最佳免费选择」**：

1. **零成本**：完全免费，无需 API Key，无调用次数焦虑
2. **高覆盖**：美股/港股/日股/欧股/指数/外汇/加密货币全覆盖
3. **质量好**：数据准确，历史悠久，K线可回溯数十年
4. **架构匹配**：go-stock 已有的 Router + Fallback + Cache 架构天然支持
5. **AI 增强**：可为 AI Agent 提供全球跨市场分析能力

**实施路径**：
```
Week 1: 扩展 yahoo_finance_api.go（代码映射 + Provider 接口）
Week 2: 接入 fallback chain（quote + kline）
Week 3: 新增基本面/分红数据 + AI 工具函数
Week 4: 前端全球指数卡片 + 搜索增强
```
