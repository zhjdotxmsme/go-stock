# 大宗商品分析功能设计

> 为 go-stock 新增黄金、白银、原油及关联商品基金的分析功能
> 日期：2026-06-30

---

## 1. 概述

在现有股票、基金分析能力基础上，扩展支持**国际现货**（黄金/白银/原油）、**国内商品期货**（沪金/沪银/原油）、**商品 ETF/基金** 三大类资产的分析与 AI 智能分析。

### 1.1 目标

- K 线数据格式统一，前端图表组件完全复用
- 数据源优先复用现有基础设施（WallStreetCN、EastMoney push2、基金 API）
- 4 个 AI 分析工具覆盖技术面、基本面、关联分析和报告生成
- 底部导航新增独立入口，内部 tab 切换

### 1.2 核心设计原则

- **统一 K 线模型**：复用 `backend/data/datasource/provider.go` 的 transfer-type `KLineBar`（非 GORM 模型），不加 `AssetType` 字段；商品 K 线不走 DB 持久化，全程实时拉取
- **现有基础设施优先**：WallStreetCN 已有黄金/原油行情，EastMoney push2 复用股票 K 线接口，基金模块复用商品 ETF
- **渐进式**：数据层 → 前端页面 → AI 工具，每层独立可测

---

## 2. 数据层设计

### 2.1 核心类型

```go
// backend/models/commodity.go

type AssetType string

const (
	AssetSpot    AssetType = "spot"    // 国际现货: XAUUSD, XAGUSD, XPTUSD, USCL
	AssetFutures AssetType = "futures" // 国内期货: AU, AG, SC, CU
	AssetETF     AssetType = "etf"     // 商品ETF: 518880, 159930, 159981
)

type CommodityAsset struct {
	Code      string    `json:"code"`      // 统一代码: XAUUSD, AU, 518880
	Name      string    `json:"name"`      // 显示名称: 现货黄金, 沪金, 黄金ETF
	AssetType AssetType `json:"assetType"`
	Exchange  string    `json:"exchange"`  // OTC, SHFE, INE, COMEX
	Symbol    string    `json:"symbol"`    // 数据源原始代码
}
```

### 2.2 品种注册表

```go
// backend/data/commodity_registry.go

var CommodityRegistry = []CommodityAsset{
	// ── 国际现货 (通过 WallStreetCN) ──
	{Code: "XAUUSD", Name: "现货黄金", AssetType: AssetSpot, Exchange: "OTC", Symbol: "XAUUSD.OTC"},
	{Code: "XAGUSD", Name: "现货白银", AssetType: AssetSpot, Exchange: "OTC", Symbol: "XAGUSD.OTC"},
	{Code: "USCL",   Name: "WTI原油",  AssetType: AssetSpot, Exchange: "OTC", Symbol: "USCL.OTC"},

	// ── 国内期货 (通过 EastMoney push2) ──
	{Code: "AU", Name: "沪金", AssetType: AssetFutures, Exchange: "SHFE", Symbol: "113.AU0"},
	{Code: "AG", Name: "沪银", AssetType: AssetFutures, Exchange: "SHFE", Symbol: "113.AG0"},
	{Code: "SC", Name: "原油", AssetType: AssetFutures, Exchange: "INE",  Symbol: "114.SC0"},

	// ── 商品ETF (通过现有基金/股票接口) ──
	{Code: "518880", Name: "华安黄金ETF",   AssetType: AssetETF, Exchange: "SH", Symbol: "518880.SH"},
	{Code: "159930", Name: "能源ETF",       AssetType: AssetETF, Exchange: "SZ", Symbol: "159930.SZ"},
	{Code: "159981", Name: "有色金属ETF",   AssetType: AssetETF, Exchange: "SZ", Symbol: "159981.SZ"},
}
```

### 2.3 统一数据接口

复用现有类型：
- `datasource.QuoteData`（定义在 `backend/data/datasource/provider.go`）
- `datasource.KLineBar`（同上，非 GORM 版）

```go
// backend/data/commodity_api.go

type CommodityApi struct {
	wsClient  *WallstreetcnApi   // 国际现货数据源
	emClient  *EastMoneyKLineApi // 国内期货数据源
}

// GetQuote 获取实时行情（统一入口）
// 返回 datasource.QuoteData:
//   Code, Name, Price, Change, ChangePct, Volume, Amount, High, Low, Open, PrevClose, Time
func (c *CommodityApi) GetQuote(code string) (*datasource.QuoteData, error)

// GetKLine 获取 K 线数据（统一入口）
// period: day/week/month（内部映射到 WallStreetCN timeout 或 EastMoney klt 编码）
// count: 返回 K 线根数
func (c *CommodityApi) GetKLine(code string, period string, count int) ([]datasource.KLineBar, error)
```

### 2.4 数据流路由

```
GetQuote(code)
  → 查 CommodityRegistry 确定 AssetType
  ├── spot     → WallStreetCN.GetMarketReal(symbol)
  ├── futures  → EastMoney push2 /api/qt/stock/get?secid={symbol}
  └── etf      → 现有 QuoteProvider (Tencent/Sina)

GetKLine(code, period, count)
  → 查 CommodityRegistry 确定 AssetType
  ├── spot     → WallStreetCN.GetKline(symbol, periodToSec(period), count)
  ├── futures  → EastMoney push2his /api/qt/stock/kline/get?secid={symbol}&klt={klt}
  └── etf      → 现有 KLineProvider fallback 链
```

### 2.5 DB 迁移策略

**核心决策：商品 K 线不持久化到 `kline_bars` 表。**

原因：
- 商品 K 线仅在页面展示和 AI 分析时临时使用，不需要回测
- 现有 `kline_bars` 表带联合唯一索引 `idx_kline_code_period_date_adj`，加字段需迁移且影响现有数据
- 保持 GORM 模型 `KLineBar` 完全不变，零 DB 迁移

商品 K 线走 `eastmoney_kline_api.go.GetKLineData()` / `wallstreetcn_api.go.GetKline()` → 内存处理 → 前端渲染。回退方案：未来若需持久化（如商品回测），新建独立表 `commodity_kline_bars`，不混用。

### 2.6 现有代码改动

| 文件 | 改动 |
|------|------|
| `wallstreetcn_api.go` | `WSCNProdCodes` 新增 `"XAGUSD.OTC": "现货白银"` |
| `models/models.go` | 新增 `CommodityAsset`、`AssetType` 类型（纯数据型，不带 gorm tag） |
| `backend/data/commodity_registry.go` | **新建**：品种注册表 |
| `backend/data/commodity_api.go` | **新建**：GetQuote / GetKLine 统一入口 |

> `convertStockCode()` 无需改动。期货 secid 格式 `113.AU0` 已包含点号，函数默认走 `return stockCode` 透传。

> `sina_kline_api.go` 不涉及。商品 K 线不走栈式 fallback 链，直接由 `CommodityApi` 路由到对应源。

---

## 3. 前端页面设计

### 3.1 底部导航

在 `App.vue` 的 `menuOptions` 中，基金自选下方新增菜单项：

```
父菜单: 🪙 大宗商品  key=commodity  icon=DiamondOutline
  ├── 行情总览  → EventsEmit("changeCommodityTab", {name: "行情总览"})
  ├── 商品期货  → EventsEmit("changeCommodityTab", {name: "商品期货"})
  ├── 商品基金  → EventsEmit("changeCommodityTab", {name: "商品基金"})
  └── AI分析    → EventsEmit("changeCommodityTab", {name: "AI分析"})
```

### 3.2 路由

```js
// router/router.js
import commodityView from "../components/commodity.vue"
{ path: '/commodity', component: commodityView, name: 'commodity' }
```

### 3.3 Tab 容器（commodity.vue）

复制 `fund.vue` 的结构，4 个 tab：

```vue
<n-tabs type="line" animated v-model:value="nowTab">
  <n-tab-pane name="行情总览"><CommodityOverview /></n-tab-pane>
  <n-tab-pane name="商品期货"><CommodityFutures /></n-tab-pane>
  <n-tab-pane name="商品基金"><CommodityFunds /></n-tab-pane>
  <n-tab-pane name="AI分析"><CommodityAnalysis /></n-tab-pane>
</n-tabs>
```

### 3.4 CommodityOverview.vue — 行情总览

- **报价卡片**：4 个卡片（黄金 XAUUSD、白银 XAGUSD、WTI原油 USCL、沪金 AU）展示最新价、涨跌幅、更新时间
- **K 线图**：点击卡片切换对应品种的轻量 K 线图，复用 `StockLightweightKlineChart.vue`，传入 `assetType` + `code`
- **快讯**：华尔街见闻商品/黄金/原油频道新闻滚动

### 3.5 CommodityFutures.vue — 商品期货

- **期货列表**：表格展示主要品种（AU/AG/SC/CU 等），字段：最新价、涨跌幅、持仓量、成交量
- **交互**：点击品种跳转 K 线分析页面，传入期货代码
- **数据源**：EastMoney push2 `clist` 接口，`fs=m:113,m:114,m:115,m:8`

### 3.6 CommodityFunds.vue — 商品基金

借助 `FundFollow.vue` 的现有搜索 + 列表功能。`FundFollow.vue` 无 `filterType` prop，改用 props 传入 `defaultSearchText="黄金,石油,能源,有色金属"`，组件加载后自动搜索并展示商品类基金。需在 `FundFollow.vue` 中新增 `defaultSearchText` prop（可选，不影响现有基金页面）。

### 3.7 CommodityAnalysis.vue — AI 分析

- **品种选择器**：下拉框选择 CommodityRegistry 中的品种
- **分析周期**：短期 / 中期 / 长期
- **维度勾选**：技术面 / 基本面 / 关联分析
- **AI 分析按钮**：调用后端的 commodity AI Agent 工具
- **结果渲染**：Markdown 展示

### 3.8 新增文件清单

| 文件 | 说明 |
|------|------|
| `frontend/src/components/commodity.vue` | Tab 容器 |
| `frontend/src/components/CommodityOverview.vue` | 行情总览 |
| `frontend/src/components/CommodityFutures.vue` | 商品期货 |
| `frontend/src/components/CommodityFunds.vue` | 商品基金 |
| `frontend/src/components/CommodityAnalysis.vue` | AI 分析 |

---

## 4. AI 分析工具设计

### 4.1 工具清单

注册在 Eino Agent 的 `GroupMarket` 组中，AI 智能体页面和 CommodityAnalysis 页面均可调用。

| 工具函数 | 触发关键词 | 输入 | 输出 |
|---------|-----------|------|------|
| `GetCommodityTechnicals` | 黄金/白银/原油技术面、走势、趋势 | code, period | 趋势方向、支撑压力、MACD/RSI/MA信号、多空判断 |
| `GetCommodityFundamentals` | 基本面、库存、非农、美联储、供需 | code, includeNews | 供需格局、美元关联、宏观事件、CFTC持仓 |
| `GetCorrelationAnalysis` | 金银比、油金比、关联、配置 | primaryCode, secondaryCodes | 相关性热力图、比值走势、配置建议 |
| `GetCommodityReport` | 周报、月报、总结 | codes, reportType | 结构化 Markdown 报告 |

### 4.2 双体系工具注册

代码库有两套独立注册体系，4 个工具需**同时注册到两套体系**。

#### 体系 A：OpenAI 直连模式 — Tool schema

在 `backend/data/tools.go` 的 `appendAgentParityTools()` 函数末尾追加 4 个 `Tool` schema（不在 `Tools()` 中已有工具列表修改，避免合并冲突）：

```go
// backend/data/tools.go - appendAgentParityTools() 末尾
tools = append(tools, Tool{
    Type: "function",
    Function: ToolFunction{
        Name:        "GetCommodityTechnicals",
        Description: "商品技术分析。分析黄金、白银、原油等商品期货的技术面，包括趋势判断、MACD/RSI/布林带指标信号、关键支撑压力位。",
        Parameters: &FunctionParameters{
            Type: "object",
            Properties: map[string]any{
                "code": map[string]any{
                    "type":        "string",
                    "description": "品种代码，如：XAUUSD(黄金)、XAGUSD(白银)、USCL(原油)、AU(沪金)",
                },
                "period": map[string]any{
                    "type":        "string",
                    "description": "分析周期：day（日线）/ week（周线），默认 day",
                },
            },
            Required: []string{"code"},
        },
    },
})
// 同理追加 GetCommodityFundamentals / GetCorrelationAnalysis / GetCommodityReport
// GetCommodityReport 额外参数: codes (string), reportType (string: 周报/月报)
// GetCorrelationAnalysis 额外参数: primaryCode (string), secondaryCodes (string 逗号分隔)
```

对应的 output Go struct 定义：

```go
// backend/data/tool_commodity.go

type CommodityTechnicalOutput struct {
    Trend        string   `json:"trend"`        // 多头/空头/震荡
    SupportPrice float64  `json:"supportPrice"` // 支撑位
    Resistance   float64  `json:"resistance"`   // 压力位
    MacdSignal   string   `json:"macdSignal"`   // MACD 信号
    Rsi          float64  `json:"rsi"`
    RiskLevel    string   `json:"riskLevel"`    // 低/中/高
}

type CommodityFundamentalOutput struct {
    SupplyDemand    string `json:"supplyDemand"`   // 供需格局描述
    DollarIndex     float64 `json:"dollarIndex"`   // 美元指数
    MacroEvents     string `json:"macroEvents"`    // 宏观事件影响
    CftcSentiment   string `json:"cftcSentiment"`  // CFTC 持仓情绪
}

type CorrelationOutput struct {
    PrimaryCode      string             `json:"primaryCode"`
    Pairs            []CorrelationPair  `json:"pairs"`
    RatioCurrent     float64            `json:"ratioCurrent"`
    RatioPercentile  float64            `json:"ratioPercentile"`
}

type CorrelationPair struct {
    Code          string  `json:"code"`
    PearsonR      float64 `json:"pearsonR"`  // 基于对数收益率
    Interpretation string `json:"interpretation"`
}

type CommodityReportOutput struct {
    Summary          string `json:"summary"`
    MarketReview     string `json:"marketReview"`
    TechnicalView    string `json:"technicalView"`
    FundamentalView  string `json:"fundamentalView"`
    CorrelationView  string `json:"correlationView"`
    Outlook          string `json:"outlook"`
    RiskWarning      string `json:"riskWarning"`
}
```

在 `tool_groups.go` 的 `dataToolGroupMap` 中追加映射：

| 工具名 | 所属 Group |
|--------|-----------|
| `GetCommodityTechnicals` | `GroupMarket` |
| `GetCommodityFundamentals` | `GroupMarket` |
| `GetCorrelationAnalysis` | `GroupMarket` |
| `GetCommodityReport` | `GroupMarket` |

#### 体系 B：Eino Agent 模式 — DataToolWrapper

在 `backend/agent/tools/data_tools_wrapper.go` 的 `GetAllDataTools()` 函数末尾追加 4 个 `NewDataToolWrapper`：

```go
tools = append(tools, NewDataToolWrapper(
    "GetCommodityTechnicals",
    "商品技术分析。分析黄金、白银、原油等商品期货的技术面，包括趋势判断、MACD/RSI/布林带指标信号、关键支撑压力位。",
    map[string]*schema.ParameterInfo{
        "code": {
            Type:     "string",
            Desc:     "品种代码，如：XAUUSD(黄金)、XAGUSD(白银)、USCL(原油)、AU(沪金)",
            Required: true,
        },
        "period": {
            Type:     "string",
            Desc:     "分析周期：day（日线）/ week（周线），默认 day",
            Required: false,
        },
    },
    func(args string) (string, error) {
        code := gjson.Get(args, "code").String()
        period := gjson.Get(args, "period").String()
        // 调用 CommodityApi.GetKLine + 技术指标计算
        return result, nil
    },
))
// 同理追加其他 3 个 wrapper
```

#### 关键词扩展

```go
// tool_groups.go - GroupMarket 关键词扩展
"黄金", "白银", "原油", "商品", "期货", "金银比", "美元指数",
"金价", "银价", "油价", "非农", "美联储", "库存", "CFTC",
"沪金", "沪银", "WTI"
```

#### toolRequiredKey 映射

4 个商品工具不依赖外部 API Key，`toolRequiredKey` 无需追加条目（无 key 要求的工具默认保留）。

### 4.3 CommodityTechnicalAnalysis 实现逻辑

```
1. 调用 CommodityApi.GetKLine(code, period, count) 获取 K 线
2. 计算指标：MA5/10/20/60、MACD、RSI(14)、布林带
3. 判断趋势：均线排列 → 多头/空头/震荡
4. 判断超买超卖：RSI < 30 超卖, > 70 超买
5. 标记关键支撑压力位：近期高低点 + 布林带上下轨
6. 输出结构化分析结果
```

### 4.4 CommodityFundamentalAnalysis 实现逻辑

```
1. 调用 WallStreetCN.GetLives(channel) 获取相关新闻
2. 调用 WallStreetCN.GetCalendar() 筛选相关经济事件
3. 调用 CommodityApi.GetQuote(code) 获取当前报价
4. 综合分析新闻情绪 + 事件影响 + 价格位置
5. 输出结构化基本面评估
```

### 4.5 CorrelationAnalysis 实现逻辑

```
1. 拉取 primaryCode + secondaryCodes 的历史 K 线
2. 收盘价转对数收益率: r(t) = ln(close(t) / close(t-1))
3. 计算皮尔逊相关系数矩阵（基于对数收益率，消除价格序列非平稳性带来的伪相关）
4. 计算比值序列（如 XAUUSD/XAGUSD 金银比）
5. 判断比值历史分位
6. 输出相关性分析 + 配置建议
```

### 4.6 CommodityReport 实现逻辑

```
1. 依次调用技术面/基本面/关联分析工具
2. 组装成结构化报告：
   ## 行情回顾
   ## 技术面分析
   ## 基本面分析
   ## 关联分析
   ## 下周/月展望
   ## 风险提示
3. 输出 Markdown 格式报告
```

---

## 5. 前端-后端 Wails 绑定

前端通过 Wails 调用 Go 方法，需在 `App` 结构体上注册公开方法：

### 5.1 CommodityApi 绑定

`CommodityApi` 不直接绑定。改为在 `app.go`（或 `app_common.go`）上注册代理方法：

```go
// app_common.go

func (a *App) GetCommodityKLine(code string, period string, count int) ([]datasource.KLineBar, error) {
    api := data.NewCommodityApi()
    return api.GetKLine(code, period, count)
}

func (a *App) GetCommodityQuote(code string) (*datasource.QuoteData, error) {
    api := data.NewCommodityApi()
    return api.GetQuote(code)
}
```

### 5.2 main.go Bind 数组

`CommodityApi` 无需追加 Bind（通过 App 代理暴露）；`app` 已在 Bind 中，新增的 `GetCommodityKLine` / `GetCommodityQuote` 方法自动对前端可用。

### 5.3 前端 Wails 绑定

```js
// frontend 自动生成，通过 wailsjs/go/main/App 调用
import { GetCommodityKLine, GetCommodityQuote } from '../../wailsjs/go/main/App'
```

### 5.4 K 线图表组件策略

`StockLightweightKlineChart.vue` 当前硬编码调用 `GetStockEastMoneyKLine`。改造方案：

> **不修改** `StockLightweightKlineChart.vue`。新建 `CommodityKlineChart.vue`，内部使用 `GetCommodityKLine` / `GetCommodityQuote`，复用 chart 渲染逻辑（`lightweight-charts`）但不复用指标系统。

理由：
- `StockLightweightKlineChart.vue` 高度耦合股票指标（chip/MACD/RSI 等 1000+ 行）
- 商品 K 线指标计算走 AI 工具，前端不需要本地指标运算
- CommodityKlineChart.vue ≈ 80 行轻量组件，远低于修改风险

```vue
<!-- CommodityKlineChart.vue 核心逻辑 -->
<script setup>
import { GetCommodityKLine, GetCommodityQuote } from '../../wailsjs/go/main/App'
import { createChart, CandlestickSeries, HistogramSeries } from 'lightweight-charts'

async function fetchKLine() {
  const bars = await GetCommodityKLine(props.code, props.period, 200)
  candleSeries.setData(bars.map(b => ({
    time: b.Time,
    open: b.Open, high: b.High, low: b.Low, close: b.Close
  })))
}
</script>
```

---

## 6. 后端文件清单

| 文件 | 说明 |
|------|------|
| `backend/models/commodity.go` | CommodityAsset、AssetType 类型定义 |
| `backend/data/commodity_registry.go` | 品种注册表 + 查询函数 |
| `backend/data/commodity_api.go` | GetQuote / GetKLine 统一入口 |
| `backend/data/tool_commodity.go` | 4 个 AI 工具的 handler + schema |
| 修改 `wallstreetcn_api.go` | 新增 XAGUSD.OTC |
| 修改 `tool_groups.go` | 商品关键词扩展 |
| 修改 `data_tools_wrapper.go` | 注册 4 个新 wrapper |

---

## 7. 测试策略

| 层级 | 测试内容 |
|------|---------|
| 单元测试 | CommodityRegistry 查询、AssetType 路由、secid 转换 |
| API 测试 | 各数据源 GetQuote / GetKLine 响应解析 |
| AI 工具测试 | 每个工具 handler 输入 → 输出格式验证 |
| 前端测试 | 4 个 tab 切换、K 线图传参、AI 分析按钮 |

---

## 8. 实施顺序

1. 数据层：类型定义 → 品种注册表 → 现有 API 扩展 → CommodityApi 统一接口
2. AI 工具：注册 4 个工具 handler → tool_groups 关键词扩展 → wrapper 注册
3. 前端：commodity.vue → CommodityOverview → CommodityFutures → CommodityFunds → CommodityAnalysis
4. 导航+路由：App.vue 菜单项 → router.js 路由
5. 集成测试：端到端验证数据流
