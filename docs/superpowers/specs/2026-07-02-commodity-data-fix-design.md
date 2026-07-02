# 大宗商品数据修复设计

> 修复 go-stock 中黄金、白银、原油数据不可用的问题，为后续多专家 AI 分析框架打好基础。
> 日期：2026-07-02

---

## 1. 概述

### 1.1 目标

- 修复 `commodity.vue` 页面中现货黄金/白银/原油、国内商品期货、商品 ETF 的行情与 K 线数据不可用问题。
- 为每类资产提供至少一个可靠数据源，并增加主备 fallback。
- 前端不再静默显示 `--`，而是明确展示数据状态与错误原因。
- 建立后端单元测试，确保 3 类资产（现货/期货/ETF）的行情与 K 线至少有一个数据源可返回有效数据。

### 1.2 核心设计原则

- **数据优先**：没有可靠数据，后续 AI 分析无意义。
- **最小改动**：优先修复现有链路，不引入外部语言依赖（不新增 Python 依赖）。
- **主备冗余**：每个资产类型至少 2 个独立数据源，单点失败可 fallback。
- **错误可见**：前端展示具体错误，便于用户排查网络/数据源问题。
- **可测试**：新增单元测试覆盖关键数据解析逻辑。

---

## 2. 当前状态与根因分析

### 2.1 现有数据链路

```
GetQuote(code)
  ├── AssetSpot   → WallStreetCN API      → XAUUSD / XAGUSD / USCL
  ├── AssetFutures → Sina Finance API      → AU / AG / SC
  └── AssetETF    → StockDataApi          → 518880 / 159930 / 159981

GetKLine(code, period, count)
  ├── AssetSpot   → WallStreetCN K-line    → XAUUSD / XAGUSD / USCL
  ├── AssetFutures → Sina Futures K-line   → AU / AG / SC
  └── AssetETF    → EastMoney push2his     → 518880 / 159930 / 159981
```

### 2.2 已知问题

| 问题 | 影响 | 当前状态 |
|---|---|---|
| 新浪财经期货接口可能返回空或格式变化 | 国内期货 AU/AG/SC 行情与 K 线不稳定 | GB18030 解码已加，但无 fallback |
| 现货仅依赖 WallStreetCN | XAUUSD/XAGUSD/USCL 单点故障 | 无 fallback |
| ETF 行情依赖 StockDataApi | 可能失败 | 无 fallback |
| ETF K 线依赖 EastMoney | 可能因 BrowserPath 配置问题失败 | 无 fallback |
| 前端吞掉所有错误 | 用户看到 `--`，无法判断是网络还是 bug | 需要增加错误状态 |

### 2.3 失败场景（待验证）

需要运行以下调用确认具体失败点：

1. `GetCommodityQuote("XAUUSD")` — 现货行情
2. `GetCommodityQuote("AU")` — 期货行情
3. `GetCommodityQuote("518880")` — ETF 行情
4. `GetCommodityKLine("XAUUSD", "day", 120)` — 现货 K 线
5. `GetCommodityKLine("AU", "day", 120)` — 期货 K 线
6. `GetCommodityKLine("518880", "day", 120)` — ETF K 线

---

## 3. 数据层设计

### 3.1 数据源选择

#### 现货（XAUUSD / XAGUSD / USCL）

| 数据源 | URL/说明 | 用途 | 优先级 |
|---|---|---|---|
| WallStreetCN | `api-ddc-wscn.awtmt.com` | 主数据源 | 1 |
| Yahoo Finance | `query1.finance.yahoo.com` | fallback | 2 |

Yahoo 商品代码：
- 黄金现货：`GC=F`（COMEX 黄金期货主力，最接近现货）
- 白银现货：`SI=F`（COMEX 白银期货主力）
- 原油现货：`CL=F`（WTI 原油）

#### 国内期货（AU / AG / SC）

| 数据源 | 说明 | 优先级 |
|---|---|---|
| Sina Finance | `hq.sinajs.cn/list=NF_AU0` | 1 |
| EastMoney push2his | 不支持期货 secid，仅作为实验性 fallback | 2 |
| 同花顺 iFinD / 东方财富 futures API | 若 Sina 持续不可用，调研备用 | 3 |

#### 商品 ETF（518880 / 159930 / 159981）

| 数据源 | 说明 | 优先级 |
|---|---|---|
| StockDataApi | 现有股票实时接口 | 1 |
| EastMoney push2his | 现有 K 线接口 | 1 |
| 腾讯财经 | 作为行情 fallback | 2 |

### 3.2 API 接口改动

在 `backend/data/commodity_api.go` 中：

1. `getSpotQuote`：Sina 失败后尝试 Yahoo Finance。
2. `getSpotKLine`：WallStreetCN 失败后尝试 Yahoo Finance 历史数据。
3. `getFuturesQuote`：Sina 失败后增加备用数据源探测逻辑。
4. `getFuturesKLine`：Sina 失败后尝试备用数据源。
5. `getETFQuote`：StockDataApi 失败后尝试腾讯财经。
6. `getETFKLine`：EastMoney 失败后尝试腾讯财经/Sina。

### 3.3 新增数据源模块

新增 `backend/data/yahoo_finance_api.go`：

```go
package data

// YahooFinanceApi 提供 Yahoo Finance 行情与 K 线数据
// 用于商品现货 fallback
type YahooFinanceApi struct{}

// GetQuote 获取实时行情
func (y *YahooFinanceApi) GetQuote(symbol string) (*datasource.QuoteData, error)

// GetKLine 获取历史 K 线
func (y *YahooFinanceApi) GetKLine(symbol, period string, count int) ([]datasource.KLineBar, error)
```

Yahoo Finance 接口说明：
- 行情：`https://query1.finance.yahoo.com/v8/finance/chart/GC=F?interval=1d&range=1d`
- K 线：`https://query1.finance.yahoo.com/v8/finance/chart/GC=F?interval=1d&range=1y`
- 返回 JSON，字段：`chart.result[0].meta`（当前价）和 `chart.result[0].timestamp/indicators.quote`（OHLCV）

### 3.4 错误处理与状态码

为每个失败场景返回明确错误：

```go
var (
    ErrCommodityNotFound      = errors.New("未找到品种")
    ErrSpotDataUnavailable    = errors.New("现货数据源全部不可用，请检查网络")
    ErrFuturesDataUnavailable = errors.New("期货数据源全部不可用，请检查网络")
    ErrETFDataUnavailable     = errors.New("ETF数据源全部不可用，请检查网络")
)
```

---

## 4. 前端改动

### 4.1 错误状态展示

修改 `CommodityOverview.vue`：

- 每个报价卡片增加 `error` 状态，失败时显示具体错误信息。
- K 线图组件 `CommodityKlineChart.vue` 已显示 `errorText`，但需更明确。

### 4.2 数据加载状态

- 增加 `loading` 状态避免重复请求。
- 报价轮询失败后暂停轮询并提示用户。

---

## 5. 测试策略

### 5.1 单元测试

新增 `backend/data/commodity_api_test.go`：

```go
func TestGetCommodityQuote_Spot(t *testing.T)
func TestGetCommodityQuote_Futures(t *testing.T)
func TestGetCommodityQuote_ETF(t *testing.T)
func TestGetCommodityKLine_Spot(t *testing.T)
func TestGetCommodityKLine_Futures(t *testing.T)
func TestGetCommodityKLine_ETF(t *testing.T)
```

测试要求：
- 网络正常时，每个测试至少返回非空数据。
- 失败时返回明确错误，不 panic。

### 5.2 数据源解析测试

新增 `backend/data/yahoo_finance_api_test.go`：

```go
func TestYahooFinanceApi_GetQuote(t *testing.T)
func TestYahooFinanceApi_GetKLine(t *testing.T)
```

---

## 6. 实施顺序

1. 诊断当前 6 个关键调用失败原因。
2. 实现 `YahooFinanceApi` 模块。
3. 修改 `commodity_api.go` 增加 fallback 逻辑。
4. 前端增加错误状态展示。
5. 新增单元测试。
6. 集成验证：手动测试 commodity.vue 4 个 tab。

---

## 7. 文件清单

| 文件 | 改动 |
|---|---|
| `backend/data/yahoo_finance_api.go` | 新增 Yahoo Finance 数据源 |
| `backend/data/yahoo_finance_api_test.go` | 新增测试 |
| `backend/data/commodity_api.go` | 修改：增加 fallback 逻辑 |
| `backend/data/commodity_api_test.go` | 新增集成测试 |
| `frontend/src/components/CommodityOverview.vue` | 修改：错误状态展示 |
| `frontend/src/components/CommodityKlineChart.vue` | 修改：更明确的错误提示 |

---

## 8. 验收标准

- [ ] `GetCommodityQuote("XAUUSD")`、`GetCommodityQuote("AU")`、`GetCommodityQuote("518880")` 至少一个返回有效数据。
- [ ] `GetCommodityKLine(...)` 三类资产均返回非空 K 线。
- [ ] 前端不再只显示 `--`，失败时显示中文错误原因。
- [ ] 新增单元测试全部通过。
- [ ] `go build ./backend/...` 无错误。

---

(End of file)
