# go-stock 升级路线规划

> 基于 daily_stock_analysis(48k⭐)、TradingAgents-astock(967⭐)、TradingAgents-CN(27k⭐) 三个开源项目的差距分析，制定的逐阶段升级方案。
>
> 核心原则：每阶段交付一组完整可用的功能，互不阻塞，按功能域分阶段迭代。
>
> 本文档仅为路线设计，各阶段的详细实现文档（DD）和实现计划（Plan）将在后续分别产出。

---

## 总体路线

```
Phase 0 ──── Phase 1 ──── Phase 2 ──── Phase 3 ══════> 持续迭代
 回测约束     决策仪表盘     交易策略库     推送+导出+模拟交易
```

| 阶段 | 核心主题 | 依赖关系 |
|------|---------|---------|
| Phase 0 | A 股回测约束增强（T+1/涨跌停/手数/ST） | 独立，无阻塞 |
| Phase 1 | AI 决策仪表盘结构化（FinalReport schema + 前端卡片） | 独立，无阻塞 |
| Phase 2 | 15 种内置交易策略库 | 独立，无阻塞 |
| Phase 3 | 多渠道推送 + 专业导出 + 模拟交易 | 子模块独立 |

---

## Phase 0：A 股回测约束增强

### 目标

当前回测引擎 `engine.go` 没有考虑 A 股特有的交易规则，导致回测结果偏高（可在涨跌停日交易、可 T+0 卖出）。此阶段增加四项约束，让回测真实反映 A 股实际交易环境。

### 改造点

#### 1. T+1 约束

- **现状**：`Engine.Run` 中 `for i := 1; i <= in.HoldingDays; i++` 从 `i=1` 开始检查退出条件，意味着信号日当天即检查退出。
- **改造**：确保持仓天数从 1 开始计算，信号日买入后至少下一个交易日（`i >= 2`，且 bars[i].TradeDate 在 bars[0].TradeDate 之后）才能卖出。若 bars 中无连续交易日序列（如存在周末/节假日 gap），按 K-line bar 索引推进。
- **文件**：`backend/data/backtest/engine.go`

#### 2. 涨跌停约束

- **现状**：即使 bar 的 close 等于 涨停价/跌停价，仍允许买入和卖出。
- **改造**：
  - 买入日检查：若 `bars[0].Close >= bars[0].PrevClose * 1.099`（沪深主板 10%），标记为涨停，禁止买入，回测跳过信号。
  - 卖出日检查：若 `bar.Close <= bar.PrevClose * 0.901`，标记为跌停，禁止卖出，持仓延后。
  - 阈值规则：主板 10%（`1.099/0.901`）、科创板/创业板 20%（代码 300/301/688 开头，`1.199/0.801`）、北交所 30%（代码 8 开头，`1.299/0.701`）、ST 5%（`1.049/0.951`）。
- **依赖前提**：`KLineBar` 需增加 `PrevClose float64` 字段，否则无法计算涨跌停阈值。若当前数据源不提供前一交易日收盘价，KLineStore 需在入库时计算并填充。
- **文件**：`backend/data/backtest/engine.go`、`backend/data/datasource/provider.go`

#### 3. 最小交易单位（手数）

- **现状**：持仓金额通过 `EntryPrice` 传入，未考虑整手约束。
- **改造**：
  - `Input` 结构新增 `Shares int` 字段（默认为 100 的倍数）。
  - 新增 `ValidateOrder(input) error` 检查：`Shares % 100 != 0` 返回错误。
  - 仓位价值 = `Shares * EntryPrice`。
- **文件**：`backend/data/backtest/engine.go` 中 `Input` 结构

#### 4. ST 股票约束

- **现状**：所有股票统一 10% 涨跌幅。
- **改造**：
  - `Input` 新增 `IsST bool` 字段。
  - `IsST == true` 时，涨跌停阈值改为 5%（`1.049` / `0.951`）。
- **文件**：`backend/data/backtest/engine.go`

### 数据结构变更

```go
// Input 新增字段
type Input struct {
    // ... 现有字段保持不变
    Shares      int     // 持仓股数，默认 100
    IsST        bool    // 是否为 ST 股票，默认 false
}
```

### 测试

新增 `backend/data/backtest/engine_a_test.go`：

| 用例 | 验证点 |
|------|--------|
| TestTPlus1Constraint | 信号日买入，同日不可卖出 |
| TestPriceLimitBuy | 涨停日买入被拒绝 |
| TestPriceLimitSell | 跌停日卖出被拒绝 |
| TestLotSizeValidation | 非 100 整数倍报错 |
| TestSTLimit | ST 股 5% 涨跌停约束 |
| TestNormalStockLimit | 普通股 10% 涨跌停约束 |

### 前端变更

**无**。所有约束透明生效，回测结果自动反映 A 股实际交易环境。

---

## Phase 1：AI 决策仪表盘结构化

### 目标

当前 `FinalReport` 的 `Conclusion` 为大段自由文本，前端按 Markdown 渲染。升级为结构化 schema + 卡片式前端组件，让用户一眼获取关键决策信息。

### 新增结构化字段

```go
type FinalReport struct {
    // 现有字段保持不变
    OverallRating      string   // strong_buy / buy / hold / sell / strong_sell
    InvestmentThesis   string
    Strengths          []string
    RiskFactors        []string
    Catalysts          []string
    MultiTimeframeView map[string]string
    Conclusion         string

    // 新增结构化字段
    Score       float64         // 1-10 综合评分
    Trend       string          // up / down / sideways
    EntryZone   *PriceZone      // 买入区间 {Low, High}
    ExitZone    *PriceZone      // 卖出区间 {Low, High}
    RiskLevel   string          // low / medium / high
    Checklist   []ChecklistItem // 操作检查清单
}

type PriceZone struct {
    Low  float64 `json:"low"`
    High float64 `json:"high"`
}

type ChecklistItem struct {
    Action      string `json:"action"`      // 操作描述
    Priority    string `json:"priority"`    // high / medium / low
    IsCompleted bool   `json:"is_completed"` // 默认 false
}
```

### 实现方案

在 `RunSynthesis` 中新增一个**结构化提取步骤**（非替换当前流程）：

```
当前流程：
  Analyst Reports → LLM Synthesis → FinalReport.Conclusion (自由文本)

新增步骤：
  FinalReport.Conclusion → LLM 结构化提取 → FinalReport.Score/Trend/EntryZone/ExitZone/...
```

**原则**：不改变现有合成流程，只在尾部增加一次轻量 LLM 调用（使用 `LLMTierFast`），从已生成的结论中抽取出结构化字段。结构化提取失败时，自由文本结论仍然可用，前端降级显示。

### 前端组件树

```
agent-chat.vue
  └── MultiAgentResult.vue (现有)
        └── DecisionDashboard.vue (新增)
              ├── 评分环 ScoreRing.vue (1-10 数值环)
              ├── 趋势方向 TrendIndicator.vue (up/down/sideways 箭头)
              ├── 买卖区间 PriceZoneCard.vue (Entry/Exit 价格卡片)
              ├── 风险等级 RiskBadge.vue (颜色标签)
              ├── 催化剂列表 CatalystTimeline.vue (时间线)
              └── 操作清单 ActionChecklist.vue (勾选框列表)
```

### 降级策略

- LLM 结构化提取失败 → 仅显示自由文本结论（维持现状），Score/RiskLevel 从已有字段推演。
- 部分字段缺失 → 显示可用字段，隐藏缺失卡片。
- Score 默认值 = `0` 时隐藏评分环。

### 文件变更清单

| 文件 | 操作 |
|------|------|
| `backend/agent/multi/types.go` | FinalReport 增加结构化字段 + PriceZone/ChecklistItem |
| `backend/agent/multi/synthesis.go` | 尾部增加结构化提取步骤 |
| `backend/agent/multi/model_config.go` | 确认 LLMTierFast 可用 |
| `frontend/src/components/DecisionDashboard.vue` | **新建**：仪表盘主组件 |
| `frontend/src/components/MultiAgentResult.vue` | 嵌入 DecisionDashboard |
| `frontend/src/components/agent-chat.vue` | 无改动 |

---

## Phase 2：内置交易策略库

### 目标

从 daily_stock_analysis 中迁移 15 种交易策略，作为可复用的 LLM 分析视角，每种策略本质是一份专业级 prompt + 数据需求声明。

### 架构概览

```
backend/agent/strategy/
├── strategy.go         # Strategy 接口 + 注册表
├── registry.go         // init() 自动注册所有策略
├── registry_test.go    // 注册完整性检查
│
├── moving_average.go   # 均线策略
├── chan_theory.go      # 缠论策略
├── wave_theory.go      # 波浪理论
├── trend.go            # 趋势跟踪
├── hot_topics.go       # 热点题材
├── event_driven.go     # 事件驱动
├── growth.go           # 成长投资
├── value.go            # 价值投资
├── expectation.go      # 预期差
├── momentum.go         # 动量策略
├── mean_reversion.go   # 均值回归
├── sentiment_strat.go  # 情绪驱动
├── sector_rotation.go  # 板块轮动
├── fundamental_strat.go# 基本面选股 (区分于 FundamentalAnalyst)
└── technical_strat.go  # 技术指标选股 (区分于 TechnicalAnalyst)
```

### Strategy 接口

```go
// Strategy 定义一种交易分析策略。
type Strategy struct {
    Name        string   // 均线策略
    Code        string   // moving_average
    Description string   // 基于均线系统判断趋势和买卖点
    Category    string   // technical / fundamental / sentiment / event
    Prompt      string   // LLM 分析 prompt
    DataNeeds   []string // kline / news / fundamental / sentiment
    Enabled     bool     // 默认 true
}
```

### 策略实现模式

每个策略文件 = 一个 `init()` 注册调用：

```go
func init() {
    Register(&Strategy{
        Name:        "缠论策略",
        Code:        "chan_theory",
        Description: "基于缠论笔段划分、中枢识别、背驰判断的买卖点分析",
        Category:    "technical",
        Prompt:      ChanTheoryPrompt,  // 加载 prompt 常量
        DataNeeds:   []string{"kline"},
    })
}
```

### 策略 Prompt 注入机制

策略不替换分析师，而是**作为分析视角注入现有 pipeline**：

```
全分析模式 (strategy=""):
  [4分析师 + 3特化] → 辩论 → 合成(泛金融视角) → 输出

策略模式 (strategy="moving_average"):
  [4分析师 + 3特化] → 辩论 → 合成(均线专家视角) → 输出
                            ↑
                    策略 Prompt 作为 System Message 注入
                    追加: 要求分析师重点从均线角度解读
```

具体实现：
1. `SynthesisPrompt` 尾部追加策略专属 Prompt 后缀。
2. 现有分析师仍并行运行，但 Synthesis 阶段要求从策略视角整合结论。
3. 策略 Prompt 中定义的 `DataNeeds` 控制是否跳过无数据支撑的分析师（例如仅需 kline 的策略可跳过 News/Sentiment Analyst 以节省 token）。

`engine.go` 的 `Run()` 新增 `StrategyCode string` 参数：

- 若 `StrategyCode == ""`：当前全分析师流程（不变）
- 若 `StrategyCode != ""`：
  - 跳过全分析师并行流程
  - 仅运行 `RunStrategy(ctx, ac, strategy)` -> 单路径分析
  - 结果直接进入 Synthesis（跳过 Debate）

```go
// engine.go Run() 流程调整
if ac.StrategyCode != "" {
    // 策略模式：单分析师 + 合成
    report := runStrategy(ctx, ac, strategy)
    ac.Reports = []AgentReport{*report}
    finalReport, _ := RunSynthesis(ctx, ac)
    emitFinalReport(ch, finalReport)
    return
}
// 全分析师模式（不变）
```

### 前端集成

`agent-chat.vue` 增加策略选择器：

```
[分析模式选择]
├── 全维度分析 (默认)     -> Run() with StrategyCode=""
├── 均线策略             -> Run() with StrategyCode="moving_average"
├── 缠论策略             -> Run() with StrategyCode="chan_theory"
├── 波浪理论             -> Run() with StrategyCode="wave_theory"
├── ... (共 15 种)
└── 自定义策略            -> 使用已有 CustomStrategy 框架
```

选择器放在输入框上方或侧边，选中后策略名称作为 `strategy` 参数传入 `Run()`。

### 分批实施策略（建议）

| 批次 | 策略 | 数量 | 特点 |
|------|------|------|------|
| Batch 1 | 均线/趋势跟踪/动量/均值回归 | 4 | 技术面，复用已有 K-line 数据 |
| Batch 2 | 缠论/波浪理论/技术指标选股 | 3 | 技术面，AI 输出质量依赖 Prompt 质量 |
| Batch 3 | 热点题材/板块轮动/事件驱动/情绪驱动 | 4 | 结合新闻和板块数据 |
| Batch 4 | 成长/价值/预期差/基本面选股 | 4 | 需要财务数据支撑 |

### 文件变更清单

| 文件 | 操作 |
|------|------|
| `backend/agent/strategy/strategy.go` | **新建**：接口 + 注册表 |
| `backend/agent/strategy/*.go` (15 文件) | **新建**：每策略一个文件 |
| `backend/agent/multi/engine.go` | Run() 增加 strategy 支持 |
| `frontend/src/components/agent-chat.vue` | 增加策略选择器 |
| `frontend/src/components/strategy-selector.vue` | **新建**（可选拆组件） |

---

## Phase 3：推送 + 导出 + 模拟交易

此阶段包含三个独立子模块，互无依赖，可按任意顺序实施。

### 3a. 多渠道推送

**架构**：泛化现有钉钉通知模式。

```go
// backend/data/notify/notifier.go
type Message struct {
    Title   string
    Content string
    Stock   string // 关联股票代码（可选）
}

type Notifier interface {
    Name() string
    Send(ctx context.Context, msg Message) error
}
```

**渠道实现**：

| 渠道 | 实现文件 | 配置方式 | 复用程度 |
|------|---------|---------|---------|
| 钉钉（已有） | `dingding_api.go` | 现有配置 | 适配 Notifier 接口 |
| 企业微信 | `wxwork.go` | webhook URL | 新增，类似钉钉 |
| 飞书 | `feishu.go` | webhook URL | 新增，类似钉钉 |
| Telegram | `telegram.go` | Bot Token + Chat ID | 新增 |
| 邮件 | `email.go` | SMTP 配置 | 新增 |

**触发时机**：仅在多智能体全分析流程（非 fast path 简单查询）完成后触发推送。推送内容为结构化摘要（评分/趋势/风险等级/操作建议三行摘要）。用户可在配置中选择推送开启/关闭以及目标渠道。不会因推送失败阻塞主分析流程。

### 3b. 专业报告导出

**导出格式**：

| 格式 | 前端方案 | 后端方案（备选） |
|------|---------|----------------|
| Markdown | 直接下载 `.md` 文件 | 已有的 `Content` 字段 |
| Word (.docx) | 前端 `docx` npm 库转换 | Go `unidoc` / `mattn` 模板 |
| PDF | `window.print()` 或 `html2pdf.js` | `chromedp` 渲染打印 |

**优先前端方案**：减少后端依赖，利用 Wails 的 Webview 环境。

**前端触发点**：`agent-chat.vue` 和 `MultiAgentResult.vue` 增加"导出"按钮，弹出 `ExportDialog.vue` 选择格式。

### 3c. 模拟交易系统

轻量级虚拟交易系统，用于验证分析策略的有效性。

**数据模型**：

```go
// backend/models/trade_sim.go
type TradeSimAccount struct {
    ID         uint      `gorm:"primaryKey"`
    Name       string    // 账户名称，如"均线策略测试"
    Balance    float64   // 初始资金
    CreatedAt  time.Time
}

type TradeSimOrder struct {
    ID          uint      `gorm:"primaryKey"`
    AccountID   uint      // 关联账户
    StockCode   string
    StockName   string
    Direction   string    // buy / sell
    Shares      int
    Price       float64
    Amount      float64
    Strategy    string    // 关联策略 code（可选）
    SignalDate  string    // 关联分析信号日期
    Status      string    // open / closed
    ClosedAt    *time.Time
    PnL         *float64  // 平仓盈亏
    CreatedAt   time.Time
}
```

**功能范围**：
- 创建/选择虚拟账户
- 手动建仓（基于分析结果）
- 自动平仓（按持仓规则）
- 查看持仓列表和盈亏
- 关联策略分析记录

**与现有代码的关系**：
- 复用 `data.TradingRecord`（已有）作为交易流水基础
- 新增 `TradeSimAccount` 和 `TradeSimOrder` 模型
- 前端新增 `TradingSimulation.vue` 页面，路由 `/trade-sim`

---

## 阶段独立性说明

阶段的划分确保每阶段可独立交付、独立测试、独立上线：

| 条件 | Phase 0 | Phase 1 | Phase 2 | Phase 3 |
|------|---------|---------|---------|---------|
| 阻塞后续阶段？ | 否 | 否 | 否 | 否 |
| 需要其他阶段先完成？ | 否 | 否 | 否 | 否 |
| 可单独测试？ | 是（单元测试） | 是（前端预览） | 是（功能测试） | 是 |
| 可单独上线？ | 是 | 是 | 是 | 是 |

---

## 技术风险

| 风险 | 阶段 | 影响 | 缓解措施 |
|------|------|------|---------|
| LLM 结构化提取不稳定 | P1 | 仪表盘数据不准 | 降级策略 + Score 字段接受 LLM 输出偏差 |
| 策略 Prompt 质量参差不齐 | P2 | 分析结果不可用 | 分批实施，每批完成后走 review |
| 涨跌停判断逻辑过于严格 | P0 | 有效信号被跳过过多 | 增加可配置的容忍度参数 |
| Webview 导出 PDF 格式乱 | P3b | 导出不可用 | 备选后端方案 + Markdown 保底 |

---

## 版本日志

| 日期 | 变更 |
|------|------|
| 2026-06-26 | 初版创建。基于 three-project 差距分析。Phase 0-3 结构确认。 |
