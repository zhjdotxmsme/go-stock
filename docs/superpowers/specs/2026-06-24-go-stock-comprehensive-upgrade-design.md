# go-stock 全面升级设计文档（多项目整合）

> 日期：2026-06-24
> 参考项目：TradingAgents-astock, stock-sdk, QuantDinger, daily_stock_analysis
> 基础分支：`feat/multi-agent-analysis`

## 1. 概述

在现有 4 分析师多智能体系统基础上，整合多个开源项目的优秀实践进行全面升级。

### 升级目标

- **分析师扩展**：增加 3 个 A 股特化分析师（政策/游资/解禁），从 4 → 7 人
- **免费数据源**：引入 mootdx + 腾讯财经 + 同花顺 + 百度股市通，零成本直连
- **技术指标**：通过 stock-sdk MCP 接入 14 种技术指标
- **双 LLM 路由**：quick_think（分析师） + deep_think（总结决策）
- **推送**：保持现有钉钉推送不变

## 2. 架构

### 2.1 Graph 拓扑

```
Orchestrator Node
  │  分发 7 分析师并行
  ├── Fundamental Analyst  (基本面)
  ├── Technical Analyst    (技术 → stock-sdk MCP 指标)
  ├── Sentiment Analyst    (情绪)
  ├── News Analyst         (新闻)
  ├── Policy Analyst       (政策)      🆕
  ├── Hot Money Analyst    (游资追踪)   🆕
  └── Lockup Analyst       (解禁监控)   🆕
          │
          ▼
  Researcher Node (Bull vs Bear 辩论, quick_think)
          │
          ▼
  Synthesis Node (deep_think)
          │
          ▼
  最终报告输出
```

### 2.2 双 LLM 路由

| 层级 | 用途 | 模型选型 |
|------|------|---------|
| `quick_think` | 7 分析师 + Bull/Bear 研究员 | DeepSeek-Chat / Qwen-Turbo 等轻量模型 |
| `deep_think` | Synthesis 总结决策 | DeepSeek-Reasoner / Claude 等强模型 |

未配置 deep_think 时自动降级为 quick_think。

## 3. 新增分析师

### 3.1 政策分析师 (Policy Analyst)

- **职责**：分析产业政策、监管政策、窗口指导对个股/行业的影响
- **输入**：股票代码 + 市场
- **数据源**：东财板块新闻、财联社电报、百度股市通概念分类
- **Prompt 核心**：关注近期政策动向对上市公司的影响，A 股是政策市
- **输出**：`PolicyReport{ PolicyEvents[], IndustryImpact, RegulatoryRisk[], Summary }`

### 3.2 游资追踪师 (Hot Money Analyst)

- **职责**：分析龙虎榜、大单流向、主力资金动态
- **输入**：股票代码 + 市场
- **数据源**：东财龙虎榜 API、资金流向数据
- **Prompt 核心**：追踪主力资金动向和短线博弈信号，游资是 A 股短线定价核心力量
- **输出**：`HotMoneyReport{ DragonTiger[], CapitalFlow[], MajorOrders[], Summary }`

### 3.3 解禁监控师 (Lockup Analyst)

- **职责**：监控限售股解禁、大股东减持、股权质押
- **输入**：股票代码 + 市场
- **数据源**：东财解禁数据、公司公告
- **Prompt 核心**：识别供给冲击风险，解禁是 A 股特有重大因素
- **输出**：`LockupReport{ UnlockEvents[], ReductionPlans[], PledgeRatio[], Summary }`

## 4. 免费数据源

新增 `backend/data/datasource/fallback/free_data.go`，注册四个免费直连数据源：

| 数据源 | 协议 | 提供内容 | 优先级 | 限流 |
|--------|------|---------|--------|------|
| **mootdx** | TCP 7709 (通达信) | OHLCV K 线、财务快照 | 5 | 不限 |
| **腾讯财经** | HTTP `qt.gtimg.cn` | PE/PB/市值/换手率实时 | 10 | 不限 |
| **同花顺** | HTTP `basic.10jqka.com.cn` | EPS 一致预期 | 15 | 不限 |
| **百度股市通** | HTTP `finance.pae.baidu.com` | 概念板块分类、资金流向 | 20 | 不限 |

所有数据源实现 `DataSourceProvider` 接口，自动注册到全局 Router。

## 5. 技术指标

通过 stock-sdk MCP 接入，不本地实现：

- **接入方式**：注册 stock-sdk 为 MCP Server（`npx -y stock-sdk mcp`）
- **调用时机**：Technical Analyst 内部，获取 K 线后调用 `get_indicators` 工具
- **覆盖指标**：14 种（MA/MACD/BOLL/SAR/DMI/KDJ/RSI/WR/CCI/BIAS/OBV/VOL/ATR/KC）
- **数据流**：K 线数据 → stock-sdk MCP 指标计算 → 指标结果 + K 线 → LLM 技术分析

## 6. 包结构变更

```
backend/agent/multi/
├── policy.go              🆕 政策分析师
├── hotmoney.go            🆕 游资追踪师
├── lockup.go              🆕 解禁监控师
├── model_config.go        🔄 双 LLM 路由扩展
├── engine.go              🔄 7 分析师并行
└── prompts.go             🔄 追加 3 个提示词

backend/data/datasource/fallback/
├── free_data.go           🆕 免费数据源注册
└── ...                    现有文件不变

backend/data/
├── tool_indicator.go      🆕 stock-sdk MCP 指标工具包装
└── ...                    现有文件不变
```

## 7. 配置变更

设置页新增配置项：

| 配置 | 说明 | 默认值 |
|------|------|--------|
| `QuickThinkModel` | 分析师/研究员用轻量模型 | 当前模型 |
| `DeepThinkModel` | 总结决策用强模型 | 空=降级到 QuickThink |
| `StockSDKEnabled` | 是否启用 stock-sdk MCP 指标 | true |
| `FreeDataSources` | 免费数据源开关列表 | 全部启用 |

## 8. 实施计划

### Phase 1: 免费数据源
1. 实现 mootdx TCP 连接器，对接通达信 7709 端口
2. 实现腾讯财经 HTTP 接口
3. 实现同花顺 HTTP 接口
4. 实现百度股市通 HTTP 接口
5. 注册到 datasource Router

### Phase 2: 新增 3 分析师
1. 政策分析师节点 + prompt
2. 游资追踪师节点 + prompt
3. 解禁监控师节点 + prompt
4. Engine 扩展为 7 路并行
5. 扩展 types.go 新增 Report 类型

### Phase 3: 双 LLM + stock-sdk MCP
1. model_config.go 双 LLM 路由
2. 前端设置页双模型配置
3. stock-sdk MCP Server 注册
4. Technical Analyst 接入指标工具

### Phase 4: 集成测试
1. 7 分析师完整链路测试
2. 双 LLM 降级测试
3. 数据源降级链测试
