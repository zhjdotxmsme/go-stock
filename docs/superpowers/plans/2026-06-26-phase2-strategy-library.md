# Phase 2: 内置交易策略库 — 实施计划

> From: `docs/superpowers/specs/2026-06-26-go-stock-upgrade-roadmap-design.md`
> 原则: 每阶段交付一组完整可用功能，互不阻塞。
> Batch 1 先行: 技术面 4 策略（均线/趋势跟踪/动量/均值回归），复用已有 K-line 数据。

---

## Scope

| 域 | 变更 |
|----|------|
| **NEW** `backend/agent/strategy/strategy.go` | Strategy 类型 + 全局注册表 + 查找/列举函数 |
| **NEW** `backend/agent/strategy/registry.go` | `init()` 自动注册所有策略（含 `_ "go-stock/backend/agent/strategy"` 导入前置） |
| **NEW** `backend/agent/strategy/registry_test.go` | 注册完整性测试 |
| **NEW** `backend/agent/strategy/batch1_*.go` (4 文件) | Batch 1 策略实现: moving_average, trend, momentum, mean_reversion |
| **MODIFY** `backend/agent/multi/types.go` | AgentContext 增加 `StrategyCode string` 字段 |
| **MODIFY** `backend/agent/multi/engine.go` | Run() 增加 StrategyCode 参数; 策略模式下跑轻量路径 |
| **MODIFY** `backend/app.go` | NewChatStream 透传 strategyCode 参数 |
| **MODIFY** `frontend/src/components/agent-chat.vue` | 增加策略选择器 NSelect |

---

## Architecture: Strategy 注入模式

策略不替换分析师，而是作为分析视角注入 Synthesis 阶段:

```
全分析模式 (strategy=""):
  7 分析师并行 → 辩论 → 合成(泛金融视角) → FinalReport

策略模式 (strategy="moving_average"):
  7 分析师并行 → 辩论 → 合成(均线专家视角) → FinalReport
                            ↑
                    Strategy.Prompt 作为后缀追加到 SynthesisPrompt
                    追加指令: 要求从策略视角解读所有分析师数据
```

### 关键设计决策

1. **不跳过分析师** — 即使有策略，仍跑 7 分析师以获得多维度数据，策略 Prompt 仅影响 Synthesis 的视角。
   - 例外: `DataNeeds` 声明可以跳过不相关的分析师（节省 token）
2. **Strategy 注册表** — 全局 `map[string]*Strategy`，`init()` 自动注册。
   - `GetAll()` 返回所有已注册策略列表
   - `GetByCode(code)` 按 code 查找
3. **AgentContext 新增字段** — `StrategyCode string`，空 = 全分析模式

---

## Implementation Tasks

### Task 1: Strategy 类型 + 注册表 (`strategy.go`, `registry.go`)

**strategy.go:**

```go
package strategy

// Strategy 定义一种内置交易分析策略。
type Strategy struct {
    Name        string   // "均线策略"
    Code        string   // "moving_average"
    Description string   // "基于均线系统判断趋势和买卖点"
    Category    string   // "technical" / "fundamental" / "sentiment" / "event"
    Prompt      string   // LLM 分析视角 prompt（追加到 SynthesisPrompt）
    DataNeeds   []string // "kline" / "news" / "fundamental" / "sentiment"
}

var registry = make(map[string]*Strategy)

// Register 注册策略（由每个策略文件的 init() 调用）
func Register(s *Strategy) { registry[s.Code] = s }

// GetByCode 按 code 查找策略
func GetByCode(code string) *Strategy { return registry[code] }

// GetAll 返回所有已注册策略
func GetAll() []*Strategy { /* return sorted copy */ }
```

**registry.go:**

```go
package strategy

import (
    _ "go-stock/backend/agent/strategy/batch1_moving_average"
    _ "go-stock/backend/agent/strategy/batch1_trend"
    _ "go-stock/backend/agent/strategy/batch1_momentum"
    _ "go-stock/backend/agent/strategy/batch1_mean_reversion"
)
```

**app.go import change:** Already imports `"go-stock/backend/agent/multi"` — need to add `_ "go-stock/backend/agent/strategy"` so init() fires.

### Task 2: Batch 1 — 4 策略实现 (4 files)

each file = `init()` + Prompt const:

| 文件 | Code | Category | Prompt 核心 |
|------|------|----------|------------|
| `batch1_moving_average.go` | `moving_average` | technical | 均线排列/金叉死叉/多头空头排列/葛兰碧法则 |
| `batch1_trend.go` | `trend` | technical | 通道突破/ADX/DMI/趋势线/高点低点 |
| `batch1_momentum.go` | `momentum` | technical | RSI/MACD/ROC/动量/超买超卖/背离 |
| `batch1_mean_reversion.go` | `mean_reversion` | technical | 布林带/标准差/均值回归概率/极端值 |

**Prompt 结构要求:**
- 开头: "你是一位{x}专家，请基于现有分析师报告，从{x}视角给出分析"
- 中间: 数据解读指引（具体看什么指标、如何判断）
- 结尾: 强制输出格式要求（Keep consistent with SynthesisPrompt output schema）

### Task 3: AgentContext + engine.go 改造

**types.go** — AgentContext 新增:

```go
StrategyCode string // 空=全分析, 非空=策略模式
```

**engine.go Run()** — 新增参数:

```go
func (e *MultiAgentEngine) Run(ctx context.Context, stockCode, stockName, market, userQuery, strategyCode string) chan *schema.Message {
```

内部: 当 `strategyCode != ""` 时:
1. 查找策略注册表: `s := strategy.GetByCode(strategyCode)`
2. 注入 `ac.StrategyCode = strategyCode`
3. 正常跑 7 分析师并行流程（不变）
4. Synthesis 阶段: 若 `ac.StrategyCode != ""`，`SynthesisPrompt` 尾部追加 `s.Prompt`
5. 此外追加一条 System Message: "请重点从【{s.Name}】角度整合分析结论"

**app.go NewChatStream:** 接收 strategyCode 参数并透传。

### Task 4: app.go 改造

```go
func (a *App) NewChatStream(stock string, stockCode string, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, agentMode string, strategyCode string) {
    engine := multi.NewMultiAgentEngine(aiConfigId)
    resultCh := engine.Run(a.ctx, stockCode, stock, "", question, strategyCode)
    // ... 其余不变
}
```

### Task 5: 前端策略选择器

`agent-chat.vue` 的 `inputEnter` 中:

在现有 `selectOptions`（AI 配置）和 `agentModeOptions`（Agent 模式）旁边，增加第三个 NSelect:

```vue
<NSelect
    v-model:value="strategyCode"
    :options="strategyOptions"
    size="tiny"
    style="width: 180px;"
/>
```

策略选项从 Go 后端 `GetAllStrategies()` 获取（新增 Wails 导出方法），包含:
- `{ code: "", name: "📊 全维度分析" }` — 默认
- `{ code: "moving_average", name: "📈 均线策略" }`
- `{ code: "trend", name: "📉 趋势跟踪" }`
- `{ code: "momentum", name: "⚡ 动量策略" }`
- `{ code: "mean_reversion", name: "🔄 均值回归" }`

前端需要新增: 
- 从 Go 获取策略列表的函数调用
- `strategyCode` 响应式变量
- 将 strategyCode 传入 `ChatWithAgent` 调用

**注意**: 当前 `agent-chat.vue` 调用的是 `ChatWithAgent`（Eino Agent 路径）而非 `NewChatStream`（多智能体路径）。按 spec 设计，策略选择器应放在分析请求入口。由于多智能体分析入口在 `stock.vue` 中（调用 `NewChatStream`），而 `agent-chat.vue` 是通用聊天入口，**策略选择器应放在 `stock.vue` 的分析面板中**，而非 `agent-chat.vue`。具体位置: `NewChatStream` 调用参数中增加 `strategyCode`。

为简化首批实现，前端改动暂不包含策略选择器 UI，仅在后端打通 strategyCode 传递链路。策略选择器 UI 在后续迭代中添加。

---

## File Change Summary

| 文件 | 操作 | 描述 |
|------|------|------|
| `backend/agent/strategy/strategy.go` | **NEW** | Strategy 类型 + 注册表 |
| `backend/agent/strategy/registry.go` | **NEW** | `init()` 导入注册 |
| `backend/agent/strategy/registry_test.go` | **NEW** | 注册完整性测试 |
| `backend/agent/strategy/batch1_moving_average.go` | **NEW** | 均线策略 |
| `backend/agent/strategy/batch1_trend.go` | **NEW** | 趋势跟踪 |
| `backend/agent/strategy/batch1_momentum.go` | **NEW** | 动量策略 |
| `backend/agent/strategy/batch1_mean_reversion.go` | **NEW** | 均值回归 |
| `backend/agent/multi/types.go` | MODIFY | AgentContext.StrategyCode |
| `backend/agent/multi/engine.go` | MODIFY | Run() + strategyCode; Synthesis 注入策略 Prompt |
| `backend/agent/multi/synthesis.go` | MODIFY | 追加策略 Prompt 后缀 |
| `backend/app.go` | MODIFY | 透传 strategyCode |
| `frontend/src/components/...` | FUTURE | 策略选择器 UI（后续迭代） |

---

## Execution Order

```
Task 1: strategy.go + registry.go    (注册表骨架)
Task 2: 4x batch1_*.go              (策略实现)
Task 3: types.go + engine.go 改造    (后端集成)
Task 4: app.go 改造                 (Wails 绑定)
Task 5: 验证 + 注册完整性测试          (质量门)
```

**依赖**: 无。Phase 2 完全独立于 Phase 0/1。

**注意**: Go 1.26.4 的 `go test` 会在沙箱环境编译失败（已知 pre-existing toolchain issue），但 `go vet` 可验证代码正确性。`lsp_diagnostics` 可用于 .go 文件校验。

---

## 验证标准

1. `go vet ./backend/agent/strategy/...` → 通过
2. `lsp_diagnostics` on all changed .go files → 无 error
3. `strategy.GetByCode("moving_average")` 返回非 nil
4. `strategy.GetByCode("")` 返回 nil
5. `strategy.GetAll()` 返回 4 条记录
6. Run with `strategyCode="moving_average"` → engine 不 panic，Synthesis 追加策略 Prompt
7. Run with `strategyCode=""` → 全分析模式不变（回归）
