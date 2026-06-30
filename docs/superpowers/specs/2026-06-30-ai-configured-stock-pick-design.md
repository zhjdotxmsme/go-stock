# AI 配置选股 — 设计文档

## 概述

将 SelectStock 页面的自然语言选股能力从东方财富 API 搜索升级为 AI 驱动：用户输入自然语言选股需求，AI 解析为结构化策略配置，由 `DailyPickEngine` 按此配置执行评分排序。

## 架构

```
SelectStock.vue
  ├─ [现有] 搜索框 → SearchStock() → 东方财富 API → 表格
  └─ [新增] "AI配置选股" 开关
                  ↓
          AIConfiguredStockPick(query, topN)
                  ↓
          ┌──────────────────────────────┐
          │  NL→Config Agent             │
          │  输入: 自然语言 + 策略注册表   │
          │  输出: StrategyConfig JSON    │
          └──────────────────────────────┘
                  ↓
          ┌──────────────────────────────┐
          │  DailyPickEngine            │
          │  .RunWithConfig(config)     │
          │  → 选策略 / 调参 / 过滤      │
          └──────────────────────────────┘
                  ↓
          表格展示 (复用现有模板)
```

## StrategyConfig 结构体

```go
type StrategyConfig struct {
    // 策略选择：空=启用全部
    EnabledStrategies []string `json:"enabled_strategies"`

    // 策略权重覆盖：key=策略Code, value=0.0~1.0
    StrategyWeights map[string]float64 `json:"strategy_weights,omitempty"`

    // 参数覆盖
    StrategyParams map[string]float64 `json:"strategy_params,omitempty"`

    // 后置过滤条件
    Filters []FilterCondition `json:"filters,omitempty"`

    // 返回数量
    TopN int `json:"top_n"`
}

type FilterCondition struct {
    Field string  `json:"field"`  // rsi14|volume|price|turnover|score|macd
    Op    string  `json:"op"`     // >|<|>=|<=|==
    Value float64 `json:"value"`
}
```

## 可覆盖参数

| 参数名 | 影响策略 | 默认值 | 说明 |
|--------|---------|--------|------|
| `rsi_period` | 超买超卖逆转 | 14 | RSI 计算周期 |
| `ma_fast` | 均线趋势 | 5 | 快线周期 |
| `ma_slow` | 均线趋势 | 20 | 慢线周期 |
| `boll_period` | 通道突破 | 20 | BOLL 周期 |
| `kdj_k_period` | KDJ短线 | 9 | KDJ K 值周期 |
| `volume_min_ratio` | 动量策略 | 1.0 | 最小量比 |

## AI Agent 上下文

调用 LLM 时注入到 system prompt 的信息：
- 策略注册表：8 个策略的 Name / Code / Description
- 可覆盖参数列表
- 要求输出严格的 StrategyConfig JSON，无多余文字
- 如果用户需求与任何现有策略都不匹配，应当启用全部策略并用 Filters 过滤

**AI 模型来源**：复用当前设置中的默认 AI 配置（`GetSettingConfig().AiConfigs` 中标记为默认的那个），与现有 `StockAiAgent` 使用的配置一致。不新增 AI 配置入口。

**参数注入机制**：`StrategyContext` 新增 `map[string]float64` 字段 `Overrides`，`RunWithConfig` 将 `StrategyParams` 写入。各策略的 `Score()` 方法按需从 `ctx.Overrides` 读取——如果 key 存在则覆盖默认值。策略无需感知是否被覆盖。

## 后端新增接口

```go
// app_common.go 新增
func (a *App) AIConfiguredStockPick(query string, topN int) ([]models.DailyPick, error)
```

内部流程：
1. 构建 system prompt（含策略注册表）
2. 调用 LLM（复用 AgentInstance），解析返回的 JSON
3. 若 JSON 解析失败或格式错误，Fallback 为全策略 + 无过滤
4. 调用 engine.RunWithConfig(config)
5. 返回结果

## Engine 改动：RunWithConfig

在 `DailyPickEngine` 新增方法：

```go
func (e *DailyPickEngine) RunWithConfig(ctx context.Context, tradeDate string, config *StrategyConfig) ([]models.DailyPick, error)
```

与 `RunDailyPick` 的差异：
1. **策略列表**：按 `EnabledStrategies` 过滤注册策略，空=全部
2. **参数注入**：`StrategyParams` 写入 `StrategyContext`，各策略的 `Score()` 方法读取
3. **权重调整**：`scoreStock` 中按 `StrategyWeights` 加权合成总分
4. **后置过滤**：评分完成后按 `Filters` 逐条件过滤
5. **TopN**：按 `config.TopN` 而非固定值

## 前端改动

SelectStock.vue：
- 搜索框旁添加 `useAIConfig` switch 开关
- 关：现有 SearchStock() 行为不变
- 开：调用 `AIConfiguredStockPick(search.value, 10)`
- 结果表格复用现有 `dataList` / `columns` 渲染逻辑
- 表头需要适配 DailyPick 字段（StockCode, StockName, Score, StrategyName 等）
- 加"AI配置选股"需要消耗 token 的提示

## 错误处理

- LLM 调用失败 → 回退到全策略 + 无过滤的默认 Engine 运行
- JSON 解析失败 → log 警告 + 回退默认
- 无符合条件的股票 → 返回空列表 + 提示信息
- 超时（>30s）→ 返回超时错误

## 不涉及改动

- 不修改现有 8 个策略的 Score() 内部逻辑
- 不修改 RunDailyPick 的现有调用方
- 不新增前端页面或组件
- 不修改现有策略注册表中的策略定义

## 实现计划

1. 定义 StrategyConfig / FilterCondition 结构体
2. 编写 buildStrategyConfigPrompt() 构建 AI 上下文
3. 实现 callLLMForConfig() 调 LLM 并解析 JSON
4. 实现 RunWithConfig() 方法
5. 实现 AIConfiguredStockPick() 接口
6. 前端 SelectStock.vue 添加 AI 配置开关和数据适配
7. Wails 重新生成绑定
8. go build 验证编译
9. 端到端测试
