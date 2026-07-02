# Plan B: 大宗商品多专家 AI 分析框架

> 在现有股票多智能体引擎基础上，为商品分析构建独立的多专家框架
> 日期：2026-07-02

---

## 1. 目标

为黄金/白银/原油提供结构化多专家分析，包含宏观、技术面、供需、情绪、跨品种关联五个维度，经多空辩论后综合出投资建议。

### 1.1 与现有系统的关系

| 现有组件 | Plan B 复用方式 |
|---------|----------------|
| `backend/agent/multi/` 股票引擎 | 复用 LLM 分层、Prompt DB 管理、流式输出模式 |
| `backend/data/commodity_api.go` | 直接调用 GetQuote/GetKLine 获取商品数据 |
| `backend/data/tool_commodity.go` | 复用 GetCommodityTechnicals/Correlation 逻辑 |
| `backend/data/wallstreetcn_api.go` | 扩展新增 TIPS/国债收益率 tickers |
| `backend/agent/multi/model_config.go` | 复用 GetChatModelWithTier + LLMTier 枚举 |
| `backend/data/prompt_template_api.go` | 复用 GetRolePrompt + DB 初始化 |

### 1.2 不做的事

- 不重构现有股票引擎（`multi/` 包保持不变）
- 不引入新的外部依赖（纯 Go + 现有 HTTP 客户端）
- 不持久化商品分析结果到独立表（复用 `AIResponseResult`，ModelName 标记为 `commodity-expert`）

---

## 2. 架构设计

### 2.1 包结构

```
backend/agent/commodity/
├── types.go           # CommodityContext, ExpertReport, DebateResult, CommodityReport
├── expert.go          # Expert 接口 + 注册表
├── engine.go          # CommodityEngine 编排管道
├── macro_expert.go    # 宏观：DXY/TIPS/国债收益率/GDP/CPI/PMI
├── technical_expert.go# 技术面：K线/MA/MACD/RSI/布林带
├── supply_expert.go   # 供需：库存/CFTC持仓/产量（数据可用时）
├── sentiment_expert.go# 情绪：新闻流/市场情绪
├── correlation_expert.go # 跨品种：金银比/金油比/DXY相关性
├── researcher.go      # 多空辩论（2轮）
├── synthesis.go       # 综合报告 + 结构化字段提取
└── prompts.go         # 6个专家 + 多空 + 综合 Prompt
```

### 2.2 Expert 接口

与股票引擎的裸函数不同，商品专家使用接口实现可扩展注册：

```go
type Expert interface {
    Role() string
    Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error)
}
```

```go
type CommodityContext struct {
    Code        string   // XAUUSD / XAGUSD / USCL / AU / AG / SC
    Name        string   // 现货黄金 / 沪金 ...
    UserQuery   string
    AIConfigID  int
    Reports     []ExpertReport
    Debate      *DebateResult
    FinalReport *CommodityReport
    StreamCh    chan *schema.Message
}
```

```go
type ExpertReport struct {
    Role    string  // macro / technical / supply / sentiment / correlation
    Content string  // 完整 Markdown 分析
    Summary string  // 一段话摘要
    Rating  string  // bullish / bearish / neutral
    Data    map[string]any // 结构化数据快照（价格/指标值等）
    Error   string
}
```

### 2.3 管道流程

```
用户请求 (code + question)
  │
  ├─ Phase 1: 5 个专家并行执行（LLMTierQuick）
  │   ├─ MacroExpert:       DXY + TIPS + 国债收益率 + GDP/CPI/PMI
  │   ├─ TechnicalExpert:   K线 + MA/MACD/RSI/布林带 + 支撑压力
  │   ├─ SupplyExpert:      新闻中的供需线索 + CFTC（如可用）
  │   ├─ SentimentExpert:   WallStreetCN 新闻流情绪分析
  │   └─ CorrelationExpert: 金银比/金油比/DXY相关性/跨品种联动
  │
  ├─ Phase 2: 多空辩论 2 轮（LLMTierDeep）
  │   ├─ Bull: 供给约束/避险需求/技术突破论点
  │   └─ Bear: 需求疲软/美元走强/技术破位论点
  │
  ├─ Phase 3: 综合报告（LLMTierDeep）
  │   ├─ 投资建议（买入/持有/减持/观望）
  │   ├─ 价格区间（支撑/阻力/目标位）
  │   ├─ 风险等级
  │   └─ 关键催化剂
  │
  ├─ Phase 4: 结构化字段提取（LLMTierQuick）
  │   └─ JSON: score, trend, entryZone, exitZone, riskLevel, checklist
  │
  └─ Phase 5: 持久化 + 流式输出
```

---

## 3. 数据源扩展

### 3.1 新增宏观指标

当前缺失 TIPS 收益率、美债收益率曲线。通过 WallStreetCN 扩展：

| 指标 | WallStreetCN Ticker | 用途 |
|------|---------------------|------|
| 美国10年期TIPS实际收益率 | `US10YTIPS.OTC`（待验证） | 黄金核心负相关指标 |
| 美国10年期国债收益率 | `US10YT.OTC`（待验证） | 机会成本/收益率曲线 |
| 美国2年期国债收益率 | `US02YT.OTC`（待验证） | 短端利率/倒挂信号 |
| 美国30年期国债收益率 | `US30YT.OTC`（待验证） | 长端利率/通胀预期 |
| CRB商品指数 | `CRB.OTC`（待验证） | 商品整体趋势 |

验证方式：实现前先 curl 测试上述 ticker，不可用则降级为"指标不可用"提示，不阻塞分析。

### 3.2 数据获取函数

```go
// backend/data/commodity_api.go 新增

func (c *CommodityApi) GetMacroIndicators() (*MacroSnapshot, error)

type MacroSnapshot struct {
    DXY           float64 `json:"dxy"`
    US10YYield    float64 `json:"us10yYield"`
    US10YTIPS     float64 `json:"us10yTips"`     // 实际收益率
    US02YYield    float64 `json:"us02yYield"`
    US30YYield    float64 `json:"us30yYield"`
    BreakevenInfl float64 `json:"breakevenInfl"` // 10Y名义 - 10Y TIPS
    YieldCurve    string  `json:"yieldCurve"`     // normal/inverted/steep
    Timestamp     time.Time `json:"timestamp"`
}
```

### 3.3 新闻数据

复用 WallStreetCN `GetLives(channel)` 已有接口，新增商品频道映射：

```go
var commodityNewsChannels = map[string]string{
    "gold":       "gold-resource",      // 黄金资源
    "oil":        "energy",              // 能源
    "commodity":  "commodity",           // 大宗商品
}
```

---

## 4. Expert 实现细节

### 4.1 MacroExpert（宏观专家）

数据输入：
- `GetMacroIndicators()` → DXY/TIPS/国债收益率/收益率曲线形态
- `GetEconomicData("CPI")` / `GetEconomicData("PMI")` → 通胀和景气度
- WallStreetCN 财经日历 → 近期 Fed 议息/非农/CPI 发布

分析维度：
- 实际利率趋势（TIPS 方向）→ 黄金核心驱动
- 美元指数方向 → 商品定价货币
- 收益率曲线形态 → 衰退/扩张信号
- 通胀预期（breakeven）→ 抗通胀需求

### 4.2 TechnicalExpert（技术专家）

复用现有 `GetCommodityTechnicals` 核心逻辑：
- 拉取 K 线（120 日）
- 计算 MA5/10/20/60、MACD、RSI(14)、布林带
- 判断趋势方向、支撑压力位
- 输出结构化指标 + 文字解读

### 4.3 SupplyExpert（供需专家）

当前数据限制：
- CFTC 持仓：暂无数据源（stub）
- 库存数据（API/EIA）：暂无数据源
- 产量数据：暂无数据源

降级策略：基于新闻流提取供需线索（LLM 分析新闻中的增产/减产/库存变化关键词），标注"基于新闻推断"。

### 4.4 SentimentExpert（情绪专家）

数据输入：
- WallStreetCN 黄金/原油频道最近 20 条快讯
- 财经日历近期事件

分析维度：
- 新闻情绪倾向（利多/利空/中性）
- 市场关注度热度
- 事件驱动风险（非农/Fed/CPI 发布窗口）

### 4.5 CorrelationExpert（关联专家）

复用现有 `GetCorrelationAnalysis` 核心逻辑：
- 金银比（XAUUSD/XAGUSD）+ 365 日分位
- 金油比（XAUUSD/USCL）
- DXY 与黄金负相关性
- 跨品种联动信号

---

## 5. Prompt 设计

### 5.1 Role Keys

| Role Key | 专家 |
|----------|------|
| `commodity_macro` | 宏观专家 |
| `commodity_technical` | 技术专家 |
| `commodity_supply` | 供需专家 |
| `commodity_sentiment` | 情绪专家 |
| `commodity_correlation` | 关联专家 |
| `commodity_bull` | 多头研究员 |
| `commodity_bear` | 空头研究员 |
| `commodity_synthesis` | 综合报告 |
| `commodity_struct_extract` | 结构化提取 |

### 5.2 初始化

在 `prompt_template_api.go` 的 `InitDefaultMultiAgentPrompts()` 后追加 `InitDefaultCommodityPrompts()`，写入 9 个 Prompt 到 DB（`type = "commodity_agent"`）。

---

## 6. 入口与前端集成

### 6.1 Wails 绑定

```go
// app.go
func (a *App) NewCommodityAnalysisStream(
    code string, name string, question string, aiConfigId int,
) {
    engine := commodity.NewCommodityEngine(aiConfigId)
    resultCh := engine.Run(a.ctx, code, name, question)
    for msg := range resultCh {
        runtime.EventsEmit(a.ctx, "commodityAnalysisStream", msg)
    }
    runtime.EventsEmit(a.ctx, "commodityAnalysisStream", "DONE")
}
```

### 6.2 前端改造

`CommodityAnalysis.vue` 新增"多专家深度分析"按钮，调用 `NewCommodityAnalysisStream`，监听 `commodityAnalysisStream` 事件渲染流式输出。

---

## 7. 实施顺序

1. **数据层扩展**：WallStreetCN 新增 TIPS/国债 ticker 验证 + `GetMacroIndicators()`
2. **框架骨架**：types.go + expert.go（接口+注册表）+ engine.go（管道）
3. **5 个专家实现**：macro → technical → correlation → sentiment → supply
4. **多空辩论**：researcher.go
5. **综合报告**：synthesis.go + 结构化提取
6. **Prompt 初始化**：prompts.go + DB 初始化
7. **入口绑定**：app.go Wails 方法
8. **前端集成**：CommodityAnalysis.vue 多专家模式
9. **测试**：单元测试 + 集成测试

---

## 8. 风险与降级

| 风险 | 影响 | 降级方案 |
|------|------|---------|
| TIPS/国债 ticker 不可用 | 宏观专家数据不完整 | 标注"指标不可用"，不阻塞分析 |
| CFTC/库存数据无源 | 供需专家无法量化 | 基于新闻 LLM 推断，标注来源 |
| Yahoo Finance 403 | 期货数据不可用 | Plan A 已处理，Sina 主源兜底 |
| LLM 调用失败 | 单个专家报告缺失 | 跳过该专家，综合报告标注缺失维度 |
