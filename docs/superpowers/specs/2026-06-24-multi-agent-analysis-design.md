# 多智能体个股分析系统设计文档

> 日期：2026-06-24
> 分支：`feat/multi-agent-analysis`
> 参考项目：TradingAgents-CN (https://github.com/hsliuping/TradingAgents-CN) / TauricResearch/TradingAgents

## 1. 概述

将 go-stock 现有的单 Agent（React/PlanExecute）AI 股票分析引擎，升级为基于 Eino Graph 的多智能体协作分析系统，参考 TradingAgents 的多角色分析师架构。

### 目标

- 完全替换现有单 Agent 分析引擎
- 多智能体并行分析，覆盖基本面/技术/情绪/新闻四个维度
- 多空研究员辩论机制
- 各能力等级用户均可使用（不设 VIP 门槛）
- 流式输出增强，展示分析阶段和进度
- 支持为不同分析师角色配置不同 LLM 模型（OpenAI 兼容 / Claude），自定义 Base URL
- 引入 TradingAgents-CN 风格的多数据源分层架构，带降级回退链

## 2. 技术选型

- **框架**：Eino (`github.com/cloudwego/eino`) — 使用其 Graph 编排能力
- **语言**：Go 1.26 — 纯 Go 实现，不依赖 Python
- **通信方式**：Eino Graph 节点间的边传递数据，通过 `runtime.EventsEmit` 流式回传前端

## 3. 包结构

```
backend/agent/
├── agent.go                  # 修改为多 Agent 入口调度
├── agent_api.go              # 修改 API 层
├── tools/                    # 保留，现有工具分组逻辑
└── multi/                    # 🆕 多智能体核心
    ├── engine.go             # Eino Graph 构建和执行引擎
    ├── types.go              # 共享类型定义
    ├── model_config.go       # 🆕 分析师模型配置管理
    ├── fundamental.go        # 基本面分析师节点
    ├── technical.go          # 技术分析师节点
    ├── sentiment.go          # 情绪分析师节点
    ├── news.go               # 新闻分析师节点
    ├── researcher.go         # 多空研究员辩论节点
    ├── synthesis.go          # 总结分析师节点
    └── prompts.go            # 所有系统提示词

backend/data/
├── datasource/                 # 🆕 数据源分层架构
│   ├── router.go              # 数据源路由
│   ├── provider.go            # 数据源接口定义
│   ├── cache.go               # 多级缓存
│   └── fallback/              # 降级链定义
│       ├── quote_chain.go
│       ├── kline_chain.go
│       ├── news_chain.go
│       ├── fundamental_chain.go
│       └── sector_chain.go
└── ...                         # 现有文件不变
```

## 4. Eino Graph 拓扑

```
Orchestrator Node  ← 注入股票上下文、分发任务
  │
  ├── Fundamental Analyst ──┐
  ├── Technical Analyst   ──┤
  ├── Sentiment Analyst   ──┤──→ Researcher Node (辩论) → Synthesis Node → 输出
  └── News Analyst        ──┘
```

- **Orchestrator Node**：入口节点，负责注入股票基础信息（代码、名称、市场）、用户问题、当前时间/行情快照，然后**并行分发**到 4 个分析师节点
- 4 个分析师节点**并行执行**（Eino Graph Parallel 能力），每个节点独立调用工具+LLM
- 研究员节点通过**循环边**实现多轮辩论（可配置 1-3 轮）
- 总结分析师节点接收所有结果，输出最终报告

## 5. 分析师节点职责

### 5.1 基本面分析师

- **职责**：评估公司财务健康度、估值指标、成长性
- **使用的工具**：`tool_stock_info.go`（公司概况）、`tool_financial_report.go`（财报）、`tool_stock_research_report.go`（研报）
- **输出**：`FundamentalReport{ Rating, KeyMetrics[], Strengths[], Risks[], Summary }`

### 5.2 技术分析师

- **职责**：分析 K 线形态、技术指标、趋势判断
- **使用的工具**：`tool_kline.go`、`tool_market_data.go`（通过 TDX/东财数据源）
- **输出**：`TechnicalReport{ Trend, Indicators[], SupportLevel, ResistLevel, Summary }`

### 5.3 情绪分析师

- **职责**：分析市场情绪、舆论倾向
- **使用的工具**：`stock_sentiment_analysis.go`（现有情感分析引擎）、`tool_news.go`
- **输出**：`SentimentReport{ SentimentScore, HotTopics[], PositiveFactors[], NegativeFactors[] }`

### 5.4 新闻分析师

- **职责**：监控宏观新闻、行业动态、公司公告
- **使用的工具**：`tool_news.go`、`tool_company_announcement.go`、`tool_calendar.go`
- **输出**：`NewsReport{ KeyEvents[], MacroImpact, IndustryTrend[], Summary }`

## 6. 研究员辩论机制

辩论轮次机制（N 轮，默认 N=2，可配置 1-3）：
1. **第 1 轮**：Bull 研究员基于 4 份分析师报告提出看多论据 → Bear 研究员提出看空论据
2. **第 2 轮**（默认包含）：双方针对对方论点进行反驳
3. **第 3 轮**（仅 N=3 时）：最终陈述与总结

当 N=1 时：仅第 1 轮（各自陈述），无交互反驳
当 N=2 时：第 1 轮陈述 + 第 2 轮反驳（默认）
当 N=3 时：第 1 轮 + 第 2 轮 + 第 3 轮最终陈述

**输出**：`DebateResult{ BullArguments[], BearArguments[], ConsensusItems[], Disagreements[] }`

## 7. 总结分析师

- **职责**：综合所有分析结果 + 辩论结论，生成最终报告
- **输出**：`FinalReport{ OverallRating, InvestmentThesis, RiskFactors, Catalysts, TimeframeView, Conclusion }`

## 8. 流式输出增强

前端事件类型：

| 事件 | payload | 说明 |
|------|---------|------|
| `agent:phase` | `{ phase, status }` | 各阶段开始/结束 |
| `agent:token` | `{ agent, token }` | 各分析师的 token 流 |
| `agent:debate` | `{ round, side, argument }` | 辩论过程展示 |
| `agent:progress` | `{ pct, label }` | 整体进度条 |
| `agent:final` | `{ report }` | 最终报告 |

## 9. 入口替换

现有：
```
NewChatStream() → AskAiWithTools() → 单 Agent(React/PlanExecute)
```

新链路：
```
NewChatStream() → MultiAgentEngine.Run(stock, question) → Eino Graph 执行 → 流式输出
```

## 10. 错误处理

```
多Agent图执行
  ├── 成功 → 返回最终报告
  ├── 部分失败（1-2个分析师异常）
  │     ├── 有结果的分析师正常参与
  │     ├── 失败节点标记"数据不可用"并跳过
  │     ├── 研究员节点基于已有报告进行辩论（N-1 或 N-2 份报告）
  │     └── 最终报告注明"XX维度分析暂不可用"
  └── 完全失败（图执行异常）
        └── 回退到 AskAiWithTools() 直接回答
```

## 11. 工具复用

多智能体系统直接复用 `backend/data/` 下的现有工具，不重复开发。每个分析师节点通过 Eino 的 ToolNode 机制加载对应的工具集。

## 12. 与现有 Agent 系统的关系

- 完全替换 `agent.go` 中的 `createReactAgent()` 和 `createPlanExecuteAgent()`
- `classifyComplexity()` 函数不再需要，统一走多 Agent 图
- `agent_api.go` 中的 `ChatWithContext()` 入口保留，内部路由到 `MultiAgentEngine.Run()`

## 13. 多模型接入支持

### 13.1 设计目标

每个分析师角色可独立配置 LLM 模型，支持 OpenAI 兼容接口和 Claude 两类接入方式，自定义 Base URL。

### 13.2 模型配置结构

```go
// 每个分析师可独立指定使用的 AI 配置
type AnalystModelConfig struct {
    Role        string  // fundamental / technical / sentiment / news / researcher_bull / researcher_bear / synthesis
    Provider    string  // "openai" (兼容) | "claude"
    BaseUrl     string  // 自定义 API 地址
    ApiKey      string  // API 密钥
    ModelName   string  // 模型名称
    Temperature float64
    MaxTokens   int
}
```

### 13.3 配置层级

```
全局默认 AI 配置 (AIConfig，用户设定)
  └── 各角色覆盖配置 (可选，不覆盖则复用全局配置)
       ├── 基本面分析师 → 可指定独立模型
       ├── 技术分析师   → 可指定独立模型
       ├── 情绪分析师   → 可指定独立模型
       ├── 新闻分析师   → 可指定独立模型
       ├── Bull 研究员  → 可指定独立模型
       ├── Bear 研究员  → 可指定独立模型
       └── 总结分析师   → 可指定独立模型
```

### 13.4 模型路由

复用现成的 `chat_model_factory.go` 实现，其 `detectChatModelProvider()` 已支持：

| Base URL 特征 | 路由到 | 说明 |
|---|---|---|
| 含 `anthropic.com` | Claude Eino 组件 | Anthropic 原生 API |
| 其他任意 URL | OpenAI 兼容 Eino 组件 | 兼容 OpenAI / DeepSeek / 硅基流动 / 阿里云百炼 / 本地模型等 |
| 空/未设置 | 全局默认 AIConfig | 降级到用户设定的默认模型 |

### 13.5 新增包结构

```
backend/agent/multi/
├── model_config.go        # 🆕 分析师模型配置管理
│   ├── AnalystModelConfig 结构体
│   ├── GetModelConfig(role) → 按角色获取模型配置
│   └── CreateChatModel(role) → 创建对应 LLM Client
```

## 14. 数据源架构（TradingAgents-CN 风格）

### 14.1 设计目标

目前 go-stock 的数据源是离散的（东财/通达信/新浪/Tushare 各自独立调用），借鉴 TradingAgents-CN 的多数据源降级回退链思路，构建统一的数据访问层。

### 14.2 数据源分层架构

```
┌─────────────────────────────────────┐
│          数据访问接口层              │
│  DataSourceProvider<T>              │
│  GetKline(), GetQuote(), ...        │
└────────────────┬────────────────────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
┌─────────────────┐ ┌─────────────────┐
│  数据源路由层    │ │  缓存层          │
│  DataSourceRouter│ │  multi_cache.go │
│  按优先级调度    │ │  内存+SQLite     │
└────────┬────────┘ └────────┬────────┘
         │                   │
         ▼                   │
┌────────────────────────────┘
│  数据源实现（带降级链）
│
│  行情数据 (Quote):
│    1️⃣ 通达信 (TDX) ── 主数据源
│    2️⃣ 东方财富        ── 回退1
│    3️⃣ 新浪            ── 回退2
│
│  K线数据 (Kline):
│    1️⃣ 通达信 MAC ── 主数据源（当前已优先）
│    2️⃣ 东方财富    ── 回退1
│
│  新闻数据 (News):
│    1️⃣ 华尔街见闻         ── 主数据源
│    2️⃣ 东方财富           ── 回退1
│    3️⃣ 财联社             ── 回退2
│
│  基本面数据 (Fundamental):
│    1️⃣ Tushare ── 主数据源
│    2️⃣ 东方财富  ── 回退1
│
│  板块/行业数据 (Sector):
│    1️⃣ 通达信板块 ── 主数据源
│    2️⃣ 东方财富    ── 回退1
└─────────────────────────────────────┘
```

### 14.3 数据降级回退链

每种数据类型的采集按优先级排序，当主数据源失败时自动降级：

```
请求 Kline 数据
  ├── TDX 成功 → 返回数据
  ├── TDX 失败 → 降级到 东方财富
  │     ├── 东财成功 → 返回数据
  │     └── 东财失败 → 降级到 新浪
  │           ├── 新浪成功 → 返回数据
  │           └── 全部失败 → 返回错误 + 缓存数据(如有)
  └── 缓存层
        ├── 首次查询：写入 redis / sqlite 缓存
        └── 数据源全部不可用：尝试返回缓存数据(带过期标识)
```

### 14.4 新增包结构

```
backend/data/
├── datasource/                  # 🆕 数据源分层架构
│   ├── router.go                # 数据源路由：按数据类型+优先级调度
│   ├── provider.go              # DataSourceProvider 接口定义
│   ├── cache.go                 # 多级缓存（内存 freecache + SQLite）
│   └── fallback/                # 各数据类型的降级链定义
│       ├── quote_chain.go       # 行情数据降级链 (TDX → 东财 → 新浪)
│       ├── kline_chain.go       # K线数据降级链 (TDX → 东财)
│       ├── news_chain.go        # 新闻数据降级链 (华尔街见闻 → 东财 → 财联社)
│       ├── fundamental_chain.go # 基本面数据降级链 (Tushare → 东财)
│       └── sector_chain.go      # 板块数据降级链 (通达信 → 东财)
│
├── eastmoney_api.go             # 不变，作为数据源实现
├── tdx_kline_api.go             # 不变
├── ...                          # 其他现有数据源不变
```

### 14.5 迁移策略

- 现有数据源文件（`eastmoney_api.go` 等）保持不动，各自实现 `DataSourceProvider` 接口
- 新代码通过 `router.GetData(ctx, QuoteData, "000001.SZ")` 统一调用，不再手动选择数据源
- 前端设置页新增「数据源」Tab，可查看各数据源状态和手动切换优先级
- 分阶段迁移：先在新多智能体分析中使用新数据源层，后续逐步替换旧调用
