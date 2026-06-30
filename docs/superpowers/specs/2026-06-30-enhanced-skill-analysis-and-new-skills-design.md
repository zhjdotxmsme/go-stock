# 增强 Skill 分析与新增 Skill 设计文档

> 日期：2026-06-30  
> 分支：`feat/enhanced-skill-analysis`  
> 关联需求：增强 go-stock 的 skill 分析能力，并新增 yfinance、QUANTAXIS、Karpathy auto-research 三个方向的 skill。

---

## 1. 概述

### 1.1 目标

1. 让现有 Skill 系统真正跑起来：修复死代码，使 Skill 的 systemPrompt / triggerKeywords 能注入 Agent。
2. 补齐 Skill 的“分析”能力：使用统计、效果评分、推荐、缺漏发现。
3. 支持从外部资源（GitHub/Gitee 仓库、技术文章）自动生成 Skill 草稿。
4. 新增 3 个内置 Skill：
   - **全球市场数据（yfinance）**：覆盖美股、港股、ETF、外汇、加密货币。
   - **中国量化（QUANTAXIS）**：覆盖 A 股、期货、回测、账户模拟。
   - **A 股自进化策略（auto-research）**：基于 Karpathy AutoResearch 范式的选股策略自迭代。

### 1.2 非目标

- 不替换现有的多智能体分析引擎（`backend/agent/multi/`）。
- 不强制用户部署 MongoDB/QUANTAXIS；QUANTAXIS Skill 标记为“高级/可选”。
- 不改动 VIP 权限模型（新增功能默认全用户可用）。

### 1.3 关键结论

- 当前 `buildSkillPrompt()` 已存在但未被调用，`GetSkillTools()` 被注释掉，技能管理 UI 被隐藏。
- 当前没有任何 Skill 使用统计、评分、推荐能力。
- 新 Skill 采用 **DB Skill 记录 + Python MCP Server** 双轨：DB 记录负责 prompt 与触发，Python MCP Server 负责真实数据/回测能力。

---

## 2. 现状与缺口

### 2.1 现有 Skill 系统

| 层级 | 文件 | 状态 |
|---|---|---|
| 数据模型 | `backend/models/models.go:1652` | `Skill` 结构完整 |
| CRUD API | `backend/data/skill_api.go` | 完整 |
| 种子数据 | `backend/data/seed_skills.go` | 5 个默认技能 |
| Agent 工具 | `backend/agent/tools/mcp_skill_tools.go:452` | `GetSkillTools()` 已定义但被注释 |
| 提示词注入 | `backend/agent/agent.go:283` | `buildSkillPrompt()` 已定义但无调用 |
| MCP 绑定 | `backend/agent/agent.go:448` | 已支持按 Skill 加载 MCP server |
| 前端 | `frontend/src/components/skill-manager.vue` | 组件完整但入口被隐藏 |

### 2.2 关键缺口

1. **Skill 不生效**：提示词注入未接入系统消息。
2. **无分析能力**：无使用记录、无评分、无推荐、无缺漏发现。
3. **无自动生成**：不能从 URL/文章生成 Skill。
4. **无新数据 Skill**：yfinance / QUANTAXIS / auto-research 未接入。

---

## 3. 设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        前端 (Vue)                            │
│  skill-manager.vue  │  skill-recommend.vue  │  聊天界面       │
└──────────────────────────┬──────────────────────────────────┘
                           │ Wails bindings
┌──────────────────────────▼──────────────────────────────────┐
│                      go-stock (Go)                           │
│  App.go ── SkillApi / SkillUsageApi / SkillAnalysisService   │
│  agent.go ── buildSkillPrompt (接入系统消息)                  │
│  backend/agent/skill_analysis/                               │
│    ├── tracker.go   # 记录 Skill 触发与结果                   │
│    ├── scorer.go    # 计算 effectiveness 分数                 │
│    ├── recommender.go # 匹配/缺漏/推荐                       │
│    └── generator.go # URL → Skill 草稿                       │
│  backend/agent/tools/mcp_skill_tools.go                     │
│    └── 新增 AnalyzeSkill / GetSkillStats / GenerateSkillFromURL│
└──────────────────────────┬──────────────────────────────────┘
                           │ MCP / local process
┌──────────────────────────▼──────────────────────────────────┐
│                   Python MCP Servers                         │
│  yfinance_server.py  │  quantaxis_server.py  │  auto_research_server.py │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 数据模型变更

#### 3.2.1 `Skill` 扩展字段

在 `backend/models/models.go` 的 `Skill` 上新增：

```go
type Skill struct {
    // ... 原字段 ...
    UsageCount int     `json:"usageCount" gorm:"default:0"`        // 使用次数
    AvgScore   float64 `json:"avgScore" gorm:"default:0"`            // 平均效果分
    Source     string  `json:"source" gorm:"default:user"`           // seed / user / generated
    Version    int     `json:"version" gorm:"default:1"`             // 版本号
    Confidence float64 `json:"confidence" gorm:"default:1"`          // 自动生成置信度
}
```

#### 3.2.2 新增 `SkillUsageRecord`

```go
type SkillUsageRecord struct {
    gorm.Model
    SkillID       uint    `json:"skillId" gorm:"index"`
    Query         string  `json:"query" gorm:"type:text"`
    SessionID     string  `json:"sessionId" gorm:"index"`
    Matched       bool    `json:"matched"`          // 是否命中 triggerKeywords
    Triggered     bool    `json:"triggered"`        // 是否实际注入 prompt
    MCPUsed       bool    `json:"mcpUsed"`          // 是否调用了绑定 MCP 工具
    OutputScore   float64 `json:"outputScore"`      // 最终报告评分（如 FinalReport.Score）
    UserRating    int     `json:"userRating"`       // 用户点赞/点踩（1/-1/0）
    TokenCost     int     `json:"tokenCost"`        // 本次注入 token 估算
    ErrorMsg      string  `json:"errorMsg"`         // 错误信息（如有）
}
```

#### 3.2.3 数据库迁移

在 `main.go` 的 `AutoMigrate()` 中追加：

```go
db.Dao.AutoMigrate(&models.SkillUsageRecord{})
```

### 3.3 核心模块

#### 3.3.1 Skill 注入（`agent.go`）

1. 在构造系统消息时调用 `buildSkillPrompt(question)`。
2. 仅注入 `Enable = true` 的 Skill。
3. 命中规则：
   - `TriggerKeywords` 为空 → 始终注入（通用型）。
   - `TriggerKeywords` 非空 → 任一关键词在 query 中子串命中即注入。
4. 同步写入 `SkillUsageRecord.Matched = true`。

#### 3.3.2 使用跟踪（`tracker.go`）

- 在 `NewChatStream` / `MultiAgentEngine.Run` 的入口与出口各加一个 hook：
  - 入口：记录哪些 Skill 被匹配。
  - 出口：记录最终报告 `Score`、MCP 工具调用情况、token 消耗。
- 提供 `TrackSkillUsage(ctx, *SkillUsageRecord)` 方法。

#### 3.3.3 效果评分（`scorer.go`）

评分维度与权重（可配置）：

| 维度 | 权重 | 说明 |
|---|---|---|
| 使用频率 | 0.25 | `usage_count` 标准化 |
| 输出评分 | 0.30 | `FinalReport.Score` 平均值 |
| 用户反馈 | 0.25 | `userRating` 平均 |
| MCP 成功率 | 0.10 | `mcpUsed && error == ""` 占比 |
| 注入 token 效率 | 0.10 | 单位 token 带来的 outputScore |

公式：

```
avg_score = Σ(维度得分 × 权重)
```

归一化与缺数处理：
- 使用频率按所有 Skill 中的最大 `usage_count` 做 min-max 归一化。
- 输出评分先映射到 `[0,1]`（若 `FinalReport.Score` 为 0-100）。
- 用户反馈维度在没有反馈时按中性 0.5 处理，不计入分母。
- MCP 成功率为成功次数 / 总尝试次数。

`SkillApi` 提供 `RecalculateSkillScores()` 定时任务，每天执行一次。

#### 3.3.4 推荐与缺漏发现（`recommender.go`）

- **Skill 匹配推荐**：用户输入后，若存在高分但未启用的 Skill，提示“可启用 XX Skill”。
- **缺漏发现**：若 query 无明显 Skill 命中，且包含股票/量化/回测等关键词，提示“缺少相关 Skill，是否从 yfinance/QUANTAXIS/auto-research 创建？”。
- **相似 Skill 合并提醒**：若两个 Skill 的 triggerKeywords 重叠度 > 80%，提示合并。

#### 3.3.5 URL 自动生成（`generator.go`）

输入：URL（GitHub/Gitee/知乎文章）。
流程：

1. 抓取页面/仓库 README。
2. 使用 LLM 提取：
   - 名称、分类、一句话描述
   - 核心能力列表
   - 建议 triggerKeywords
   - systemPrompt 草稿
   - 示例对话
3. 生成 `Skill` 对象，`Source = generated`，`Confidence = LLM 置信度`，默认 `Enable = false`。
4. 前端展示草稿，用户确认后启用。

### 3.4 Agent 工具扩展

在 `backend/agent/tools/mcp_skill_tools.go` 追加以下工具：

| 工具名 | 功能 |
|---|---|
| `AnalyzeSkillEffectiveness` | 返回指定 skill 的使用次数、平均分、最近使用记录 |
| `GetSkillUsageStats` | 按时间/分类聚合 skill 使用情况 |
| `GenerateSkillFromURL` | 输入 URL，返回生成的 Skill 草稿 |
| `GetSkillRecommendations` | 根据当前 query 推荐 skill |

### 3.5 前端变更

1. **skill-manager.vue**：
   - 新增「分析」Tab：展示 usage_count、avg_score、最近调用记录。
   - 新增「从 URL 生成」按钮。
2. **skill-recommend.vue**（新建）：
   - 聊天界面右侧或设置页展示推荐列表。
   - 一键启用/忽略。
3. **App.vue / researchIndex.vue**：
   - 解除 Skill 管理入口隐藏。

### 3.6 Python MCP Servers

#### 3.6.1 目录与运行方式

```
mcp-servers/
├── yfinance_server.py
├── quantaxis_server.py
└── auto_research_server.py
```

go-stock 通过 `models.MCPServer` 的 `command`/`args`/`env` 字段启动本地 MCP server：

```
command: python
args: ["mcp-servers/yfinance_server.py"]
env: { "PYTHONPATH": "..." }
```

#### 3.6.2 yfinance_server.py 工具清单

| 工具 | 说明 |
|---|---|
| `yf_historical_prices` | 历史 OHLCV |
| `yf_ticker_info` | 公司摘要 |
| `yf_financials` | 财务报表 |
| `yf_option_chain` | 期权链 |
| `yf_news` | 新闻 |
| `yf_screen` | 股票筛选 |

#### 3.6.3 quantaxis_server.py 工具清单

| 工具 | 说明 |
|---|---|
| `qa_stock_daily` | A 股日线 |
| `qa_stock_minute` | 分钟线 |
| `qa_stock_realtime` | 实时行情 |
| `qa_stock_list` | 股票列表 |
| `qa_indicator` | 技术指标 |
| `qa_backtest_run` | 回测 |
| `qa_account_status` | QIFI 账户状态 |

#### 3.6.4 auto_research_server.py 工具清单

| 工具 | 说明 |
|---|---|
| `ar_read_protocol` | 读取 SKILL.md / 策略协议 |
| `ar_edit_strategy` | 修改 workspace/strategy.py |
| `ar_backtest` | 运行回测 |
| `ar_commit` | 保留改进版本 |
| `ar_revert` | 回滚退化版本 |
| `ar_log_result` | 写实验日志 |

### 3.7 新增内置 Skill 种子

在 `seed_skills.go` 追加：

```go
{
    Name:            "全球市场数据",
    Description:     "通过 yfinance 获取美股、港股、ETF、外汇、加密货币的行情与基本面数据",
    Category:        "量化策略",
    SystemPrompt:    `你擅长使用 yfinance 工具。当用户询问海外资产时，优先调用 yfinance_server 的工具...`,
    TriggerKeywords: "美股,港股,yfinance,ETF,期权,财报,info,history",
    Enable:          true,
    SortOrder:       6,
    Source:          "seed",
},
{
    Name:            "中国量化",
    Description:     "通过 QUANTAXIS 获取 A 股、期货数据并运行回测",
    Category:        "量化策略",
    SystemPrompt:    `你擅长使用 QUANTAXIS 工具。当用户询问 A 股量化、回测、期货时...`,
    TriggerKeywords: "量化,回测,A股,期货,QUANTAXIS,QIFI",
    Enable:          true,
    SortOrder:       7,
    Source:          "seed",
},
{
    Name:            "A股自进化策略",
    Description:     "基于 Karpathy AutoResearch 范式，自动迭代选股策略配置",
    Category:        "量化策略",
    SystemPrompt:    `你是一位自进化策略研究员。遵循策略协议，只修改 workspace/strategy.py...`,
    TriggerKeywords: "自进化,auto-research,策略迭代,选股策略,自动迭代",
    Enable:          true,
    SortOrder:       8,
    Source:          "seed",
},
```

---

## 4. 错误处理

- **Skill 注入失败**：降级为无 Skill 运行，记录 `ErrorMsg`。
- **MCP Server 未启动**：DB Skill 的 prompt 仍生效；调用 MCP 工具时返回“服务未启动”并记录失败。
- **自动生成的 Skill 置信度 < 0.6**：前端标红，默认不启用，必须人工确认。
- **QUANTAXIS 依赖缺失**：该 Skill 仍显示，但工具调用返回“请先部署 MongoDB 与 QUANTAXIS”。

---

## 5. 测试计划

| 层级 | 内容 |
|---|---|
| 单元测试 | `buildSkillPrompt` 关键词匹配、`scorer` 分数计算、`generator` URL 摘要解析 |
| 集成测试 | MCP Server 工具调用、Wails CRUD、SkillUsageRecord 写入 |
| 手工测试 | 创建 Skill → 提问 → 检查 usage 表与评分 → 从 URL 生成 Skill |
| 回归测试 | 现有单 Agent / 多 Agent 分析链路不受影响 |

---

## 6. 实施阶段

1. **Phase 1 — 修复 Skill 生效**：接入 `buildSkillPrompt`、放开 UI、启用 `GetSkillTools()`。
2. **Phase 2 — 追踪与评分**：新增 `SkillUsageRecord`、tracker、scorer、前端分析 Tab。
3. **Phase 3 — 推荐与生成**：recommender、generator、新增 Agent 工具。
4. **Phase 4 — 新 Skill**：开发 3 个 Python MCP Server、新增种子 Skill、文档。

---

## 7. 风险与约束

- **Python 运行时**：MCP Server 需要用户本机安装 Python 与依赖，打包方案待后续补充。
- **QUANTAXIS 重量**：MongoDB 依赖可能劝退普通用户，UI 需明确标注“高级功能”。
- **LLM 自动生成成本**：URL 生成 Skill 会消耗额外 token，建议加确认步骤。
- **数据合规**：yfinance 与 QUANTAXIS 均来自第三方，需遵守各数据源使用条款。

---

## 8. 未决问题

1. Python MCP Server 是否使用 Docker 分发？
2. Skill 评分是否需要用户显式反馈（点赞/点踩），还是仅依赖输出评分？
3. auto-research server 是否需要完整的 git 工作目录，还是仅做策略参数调优？
