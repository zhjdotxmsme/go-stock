# 大宗商品数据修复设计（v2）

> 修复 go-stock 中大宗商品数据源问题，替换为网络上可用数据源，并保留国际参考。
> 日期：2026-07-02

---

## 1. 概述

### 1.1 目标

- 修复 `commodity.vue` 页面中现货黄金/白银/原油、国内商品期货、商品 ETF 的行情与 K 线数据不可靠问题。
- 关键修复：**国内期货 AU/AG/SC 目前返回的是 COMEX 国际价格（如 GC=F），需要修正为上期所/能源交易中心的人民币价格**。
- 为每类资产提供主备 fallback，单点失败可切换。
- 保留"国际参考"切换，允许用户对比 COMEX 国际价格。
- 前端明确展示数据状态、错误原因与数据来源。

### 1.2 核心设计原则

- **数据正确优先**：错误数据比无数据更危险。
- **主备冗余**：每个资产类型至少 2 个独立数据源。
- **错误可见**：前端展示具体错误与当前使用的数据源。
- **最小依赖**：不引入外部语言依赖，优先复用已有 Go 代码。
- **可测试**：重写错误测试，新增关键解析单元测试。

---

## 2. 当前状态与根因

### 2.1 当前数据链路

```
GetQuote(code)
  ├── AssetSpot    → WallStreetCN API → Yahoo Finance fallback
  ├── AssetFutures → Sina hq.sinajs.cn → Yahoo Finance fallback
  └── AssetETF     → StockDataApi(EastMoney/Sina) → 无 fallback

GetKLine(code, period, count)
  ├── AssetSpot    → WallStreetCN K-line → Yahoo Finance fallback
  ├── AssetFutures → Yahoo Finance ONLY (Sina K-line 代码已成死代码)
  └── AssetETF     → EastMoney push2his → 无 fallback
```

### 2.2 关键问题

| 问题 | 影响 | 严重程度 |
|---|---|---|
| 期货 K 线只走 Yahoo，返回 COMEX 国际价 | AU 显示 GC=F 美元价，不是国内沪金价 | **严重** |
| 期货行情走 Sina `hq.sinajs.cn` | 格式已变化过（NF_ 前缀失效），无国内备用源 | 高 |
| 现货主源 WallStreetCN 不稳定 | 经常无数据 | 中 |
| ETF 行情/K 线无 fallback | 单点故障 | 中 |
| 前端吞掉错误/不显示数据源 | 用户看到 `--` 无法判断 | 中 |

---

## 3. 数据层设计

### 3.1 数据源选择

#### 现货（XAUUSD / XAGUSD / USCL）

| 数据源 | URL/说明 | 优先级 | 备注 |
|---|---|---|---|
| Yahoo Finance v8 | `query1.finance.yahoo.com/v8/finance/chart/{SYM}` | 1 | GC=F/SI=F/CL=F，延迟约 15 分钟 |
| AURUM Rates | `aurumrates.com/api/v1/spot` | 2 | 覆盖黄金/白银/WTI，50 req/天 |
| WallStreetCN | `api-ddc-wscn.awtmt.com` | 3 | 原有主源，降为 fallback |

#### 国内期货（AU / AG / SC）

| 数据源 | URL/说明 | 优先级 | 备注 |
|---|---|---|---|
| EastMoney push2 | `push2.eastmoney.com/api/qt/stock/get?secid={SECID}` | 1 | secid: 113.AU0 / 113.AG0 / 114.SC0 |
| EastMoney push2his | `push2his.eastmoney.com/api/qt/stock/kline/get?secid={SECID}` | 1 | K 线主源 |
| Sina Futures | `hq.sinajs.cn/list={CODE}0` / `stock.finance.sina.com.cn/futures/api/jsonp.php/...` | 2 | 行情+K-line fallback |
| Yahoo Finance | `query1.finance.yahoo.com/v8/finance/chart/GC=F` | 3 | **国际参考** |

#### 商品 ETF（518880 / 159930 / 159981）

| 数据源 | URL/说明 | 优先级 | 备注 |
|---|---|---|---|
| EastMoney push2/push2his | 现有接口 | 1 | ETF 行情+K 线 |
| Tencent Finance | `qt.gtimg.cn/q={CODE}` | 2 | 行情 fallback |
| Sina Stock | `hq.sinajs.cn/list={CODE}` | 3 | 行情 fallback |

### 3.2 国际参考切换

在 `CommodityRegistry` 中为期货品种增加国际参考符号映射：

```go
// CommodityAsset 增加 InternationalRef 字段
{
    Code: "AU",
    Name: "沪金",
    InternationalRef: "GC=F", // COMEX 黄金
}
```

前端 `CommodityAnalysis.vue` / `CommodityOverview.vue` 增加"国际参考"开关：
- 关闭：显示国内 SHFE/INE 价格
- 打开：调用 Yahoo Finance 获取 COMEX 国际价格

### 3.3 API 接口改动

在 `backend/data/commodity_api.go` 中：

1. `getSpotQuote`：Yahoo Finance → AURUM Rates → WallStreetCN。
2. `getSpotKLine`：Yahoo Finance → AURUM Rates（如支持）→ WallStreetCN。
3. `getFuturesQuote`：EastMoney push2 → Sina → Yahoo（国际参考）。
4. `getFuturesKLine`：**启用 `getFuturesKLineFromSina`** → EastMoney push2his → Yahoo（国际参考）。
5. `getETFQuote`：StockDataApi/EastMoney → Tencent Finance → Sina。
6. `getETFKLine`：EastMoney push2his → Sina stock K-line fallback。

### 3.4 新增数据源模块

1. **新增/复用 `YahooFinanceApi`**：已存在于 `backend/data/yahoo_finance_api.go`，调整映射逻辑支持国际参考。
2. **新增 `AurumRatesApi`**：轻量级 HTTP 封装，解析 `aurumrates.com/api/v1/spot`。
3. **新增/复用 EastMoney push2**：封装期货行情接口，复用 `push2.eastmoney.com`。

---

## 4. 前端改动

### 4.1 国际参考开关

修改 `CommodityAnalysis.vue`：
- 在品种选择器旁增加 `<n-switch v-model:value="showInternationalRef" />`
- 切换时重新加载行情和 K 线

### 4.2 数据状态展示

修改 `CommodityOverview.vue` 和 `CommodityAnalysis.vue`：
- 每个报价卡片显示当前数据源（如"东方财富"、"Yahoo"、"Sina"）
- 失败时显示具体错误信息，不再只显示 `--`
- 显示数据时间戳，便于判断 stale 数据

### 4.3 加载与错误状态

修改 `CommodityPriceChart.vue`：
- 增加 `<n-spin>` 加载态
- 空数据/错误时显示 `<n-empty>` + 重试按钮
- 支持深色主题 prop

---

## 5. 测试策略

### 5.1 重写错误测试

`commodity_api_test.go` 中 `TestCommodityApi_GetKLine_Futures_Unavailable` 当前断言 futures K-line 应该失败。修后应期望成功，改为：

```go
func TestCommodityApi_GetKLine_Futures_Domestic(t *testing.T)
```

验证 AU K-line 返回价格约为 570 CNY（不是 3200 USD）。

### 5.2 新增解析测试

- `TestEastMoneyFuturesQuote_Parse`：验证 push2 字段 f43/f44/f45/f46/f60/f169/f170 解析
- `TestAurumRates_Parse`：验证 AURUM JSON 解析
- `TestYahooFinance_InternationalRef`：验证 GC=F 映射

### 5.3 集成测试

- `TestCommodityApi_GetQuote_Futures_Domestic`：验证 AU 返回 "沪金" 名称
- `TestCommodityApi_GetKLine_Spot`：验证 XAUUSD 返回有效 K 线
- `TestCommodityApi_GetQuote_ETF`：验证 518880 返回有效行情

---

## 6. 实施顺序

1. 实现 EastMoney push2 期货行情封装。
2. 启用 Sina futures K-line 并增加 EastMoney push2his fallback。
3. 调整现货链路：Yahoo 为主，AURUM 和 WallStreetCN 为 fallback。
4. 增加 ETF fallback（Tencent/Sina）。
5. 前端：国际参考开关 + 数据源/错误展示。
6. 重写测试并运行验证。
7. 手动测试 commodity.vue 4 个 tab。

---

## 7. 文件清单

| 文件 | 改动 |
|---|---|
| `backend/data/commodity_api.go` | 修改 fallback 链路，启用 Sina K-line |
| `backend/data/commodity_registry.go` | 增加 `InternationalRef` 字段 |
| `backend/data/eastmoney_futures_api.go` | 新增东方财富期货行情封装 |
| `backend/data/aurum_rates_api.go` | 新增 AURUM Rates 数据源 |
| `backend/data/yahoo_finance_api.go` | 调整国际参考映射 |
| `backend/data/commodity_api_test.go` | 重写错误测试，新增集成测试 |
| `frontend/src/components/CommodityAnalysis.vue` | 增加国际参考开关 |
| `frontend/src/components/CommodityOverview.vue` | 显示数据源与错误 |
| `frontend/src/components/CommodityPriceChart.vue` | 加载/错误态 + 深色主题 |

---

## 8. 验收标准

- [ ] `GetCommodityQuote("AU")` 返回沪金国内价格（人民币，非 COMEX 美元）。
- [ ] `GetCommodityKLine("AU", "day", 60)` 返回国内 K 线数据。
- [ ] `GetCommodityQuote("XAUUSD")` 通过 Yahoo Finance 返回有效数据。
- [ ] `GetCommodityQuote("518880")` 至少一个数据源返回有效数据。
- [ ] 国际参考开关开启后，AU 显示 COMEX 国际价格。
- [ ] 前端失败时显示中文错误原因与当前数据源。
- [ ] 新增/重写单元测试全部通过。
- [ ] `go build ./...` 无错误，`vite build` 无错误。
