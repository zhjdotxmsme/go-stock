# AI 配置选股 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 用户在 SelectStock 页面输入自然语言选股需求，AI 解析为策略配置，DailyPickEngine 按配置执行评分排序

**Architecture:** 新增 `AIConfiguredStockPick` 后端接口，LLM 输出 StrategyConfig JSON，Engine 新增 `RunWithConfig` 方法支持策略选择/参数覆盖/后置过滤。5 个策略修改为从 `ctx.Overrides` 读取参数（含 fallback 到默认值）。

**Tech Stack:** Go, Eino (LLM), Vue/NaiveUI (前端)

---

### Task 1: 定义 StrategyConfig / FilterCondition 结构体

**Files:**
- Modify: `backend/models/daily_pick.go` — 新增 StrategyConfig / FilterCondition 类型

- [ ] **Step 1: 新增 StrategyConfig 和 FilterCondition**

```go
type StrategyConfig struct {
    EnabledStrategies []string            `json:"enabled_strategies"`
    StrategyWeights   map[string]float64  `json:"strategy_weights,omitempty"`
    StrategyParams    map[string]float64  `json:"strategy_params,omitempty"`
    Filters           []FilterCondition   `json:"filters,omitempty"`
    TopN              int                 `json:"top_n"`
}

type FilterCondition struct {
    Field string  `json:"field"`
    Op    string  `json:"op"`
    Value float64 `json:"value"`
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```
git add backend/models/daily_pick.go
git commit -m "feat(daily-pick): add StrategyConfig/FilterCondition types for AI-configured picking"
```

---

### Task 2: StrategyContext 添加 Overrides 字段

**Files:**
- Modify: `backend/data/strategy.go` — StrategyContext 加 Overrides map

- [ ] **Step 1: Overrides 字段**

```go
type StrategyContext struct {
    // ... existing fields ...
    Overrides map[string]float64 `json:"-"` // parameter overrides from AI config
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```
git add backend/data/strategy.go
git commit -m "feat(daily-pick): add Overrides map to StrategyContext for AI param injection"
```

---

### Task 3: 5 个策略修改为从 ctx.Overrides 读取参数

**Files:**
- Modify: `backend/data/strategy_oversold_reversal.go` — rsi_period
- Modify: `backend/data/strategy_ma_trend.go` — ma_fast, ma_slow
- Modify: `backend/data/strategy_channel_breakout.go` — boll_period
- Modify: `backend/data/strategy_kdj_short.go` — kdj_k_period
- Modify: `backend/data/strategy_momentum.go` — volume_min_ratio

**模式**：每个策略的 Score() 方法开头从 `ctx.Overrides` 读取参数，存在则覆盖默认值。

**Step 1: 超买超卖逆转**

```go
func (s *OversoldReversalStrategy) Score(ctx *StrategyContext) *StrategyResult {
    rsiPeriod := 14
    if v, ok := ctx.Overrides["rsi_period"]; ok && v > 0 {
        rsiPeriod = int(v)
    }
    rsi14 := calcRSI(ctx.CloseP, rsiPeriod)
    // ... rest unchanged, using rsiPeriod variable instead of hardcoded 14
}
```

**Step 2: 均线趋势**

```go
func (s *MATrendStrategy) Score(ctx *StrategyContext) *StrategyResult {
    maFast := 5
    maSlow := 20
    if v, ok := ctx.Overrides["ma_fast"]; ok && v > 0 { maFast = int(v) }
    if v, ok := ctx.Overrides["ma_slow"]; ok && v > 0 { maSlow = int(v) }
    ma5 := calcSMA(ctx.CloseP, maFast)
    ma20 := calcSMA(ctx.CloseP, maSlow)
    // ... rest unchanged
}
```

**Step 3: 通道突破**

```go
func (s *ChannelBreakoutStrategy) Score(ctx *StrategyContext) *StrategyResult {
    bollPeriod := 20
    if v, ok := ctx.Overrides["boll_period"]; ok && v > 0 { bollPeriod = int(v) }
    boll := calcBOLL(ctx.CloseP, bollPeriod, 2.0)
    // ... rest unchanged
}
```

**Step 4: KDJ短线**

```go
func (s *KDJShortStrategy) Score(ctx *StrategyContext) *StrategyResult {
    kdjKPeriod := 9
    if v, ok := ctx.Overrides["kdj_k_period"]; ok && v > 0 { kdjKPeriod = int(v) }
    kdj := calcKDJ(ctx.HighP, ctx.LowP, ctx.CloseP, kdjKPeriod, 3)
    // ... rest unchanged
}
```

**Step 5: 动量策略**

```go
func (s *MomentumStrategy) Score(ctx *StrategyContext) *StrategyResult {
    volumeMinRatio := 1.0
    if v, ok := ctx.Overrides["volume_min_ratio"]; ok && v > 0 { volumeMinRatio = v }
    // 在成交量评分处使用 volumeMinRatio 替代硬编码值
}
```

- [ ] **Step 6: 编译验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```
git add backend/data/strategy_oversold_reversal.go backend/data/strategy_ma_trend.go backend/data/strategy_channel_breakout.go backend/data/strategy_kdj_short.go backend/data/strategy_momentum.go
git commit -m "feat(daily-pick): 5 strategies read params from ctx.Overrides with fallback to defaults"
```

---

### Task 4: 实现策略注册表上下文生成 (buildStrategyConfigPrompt) + callLLMForConfig

**Files:**
- Create: `backend/data/daily_pick_config.go` — 新文件

- [ ] **Step 1: buildStrategyConfigPrompt() + callLLMForConfig()**

**LLM 调用链（Momus 指出的精确流程）：**

```go
import (
    "context"
    "encoding/json"
    "go-stock/backend/logger"
    "time"
    
    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/schema"
)

func callLLMForConfig(query string) (*StrategyConfig, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    
    // 1. 获取 AI 配置
    settingConfig := GetSettingConfig()
    if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
        return nil, fmt.Errorf("no AI config available")
    }
    // 取第一个启用的配置（默认配置）
    var aiConfig *AIConfig
    for _, cfg := range settingConfig.AiConfigs {
        if cfg.Enabled {
            aiConfig = cfg
            break
        }
    }
    if aiConfig == nil {
        aiConfig = settingConfig.AiConfigs[0]
    }
    
    // 2. 创建 ChatModel（复用 data 包内已有工厂方法）
    chatModel, err := createChatModel(aiConfig) // 复用内部工厂函数
    if err != nil {
        return nil, fmt.Errorf("create chat model: %w", err)
    }
    
    // 3. 构建消息
    prompt := buildStrategyConfigPrompt(query)
    messages := []*schema.Message{
        {Role: schema.System, Content: "你是一个选股策略配置专家。只输出 JSON，不要多余文字。"},
        {Role: schema.User, Content: prompt},
    }
    
    // 4. 调用 LLM
    result, err := chatModel.Generate(ctx, messages)
    if err != nil {
        return nil, fmt.Errorf("LLM generate: %w", err)
    }
    
    // 5. 解析 JSON
    var config StrategyConfig
    if err := json.Unmarshal([]byte(result.Content), &config); err != nil {
        return nil, fmt.Errorf("parse LLM response: %w", err)
    }
    
    return &config, nil
}

// buildStrategyConfigPrompt 生成包含策略注册表的 prompt
func buildStrategyConfigPrompt(query string) string {
    // 策略描述列表
    strategies := []struct{
        Code string
        Name string
        Desc string
    }{
        {"ma_trend", "均线趋势", "基于MA5/MA10/MA20多头排列的顺势跟踪"},
        {"oversold_reversal", "超买超卖逆转", "识别RSI/WR/CCI超卖区域的反转信号"},
        {"momentum", "动量策略", "基于MACD金叉死叉和OBV能量潮的动量跟踪"},
        {"channel_breakout", "通道突破", "基于BOLL通道突破和ATR波动率确认"},
        {"kdj_short", "KDJ短线", "基于KDJ和W%R的超短线交易信号"},
        {"industry_strength", "行业强度", "基于行业资金流向排名的评分"},
        {"research_report", "研报热度", "基于近期机构研报数量的评分"},
        {"macro_environment", "宏观环境", "基于PMI/CPI/GDP宏观数据的评分"},
    }
    
    // 构建 prompt 内容（硬编码策略列表，避免包循环引用）
    prompt := fmt.Sprintf(`用户选股需求：%s

可用策略及说明：
`, query)
    for _, s := range strategies {
        prompt += fmt.Sprintf("- %s (%s): %s\n", s.Code, s.Name, s.Desc)
    }
    prompt += `
可覆盖参数：
- rsi_period (默认14): RSI计算周期，影响"超买超卖逆转"策略
- ma_fast (默认5): 快线周期，影响"均线趋势"策略
- ma_slow (默认20): 慢线周期，影响"均线趋势"策略
- boll_period (默认20): BOLL周期，影响"通道突破"策略
- kdj_k_period (默认9): KDJ K值周期，影响"KDJ短线"策略

返回严格的 JSON 格式（不要多余文字）：
{"enabled_strategies":["strategy_code1","strategy_code2"],"strategy_weights":{"code":0.6},"strategy_params":{"param":value},"filters":[{"field":"rsi14","op":"<","value":70}],"top_n":10}`
    
    return prompt
}
```

**注意**：`createChatModel` 是 `data` 包内部已存在的函数（在 `openai_api.go` 或 `chat_model_factory.go` 中），直接引用即可。如果不在 `data` 包中，则需要通过 `settingConfig` 和 `AIConfig` 调用 Eino 的原生 ChatModel 创建流程。

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```
git add backend/data/daily_pick_config.go
git commit -m "feat(daily-pick): add buildStrategyConfigPrompt + callLLMForConfig"
```

---

### Task 5: 实现 RunWithConfig — Engine 按配置执行

**Files:**
- Modify: `backend/data/daily_pick_engine.go` — 新增 RunWithConfig 方法
- Create: `backend/data/daily_pick_config_test.go` — 单元测试

- [ ] **Step 1: RunWithConfig()**

```go
func (e *DailyPickEngine) RunWithConfig(ctx context.Context, tradeDate string, config *StrategyConfig) ([]models.DailyPick, error) {
    // 1. nil config → 回退到 RunDailyPick
    if config == nil {
        return e.RunDailyPick(ctx, tradeDate, config.TopN)
    }
    
    // 2. 按 EnabledStrategies 过滤
    if len(config.EnabledStrategies) > 0 {
        enabled := make(map[string]bool)
        for _, code := range config.EnabledStrategies {
            enabled[code] = true
        }
        filtered := make([]ScoringStrategy, 0, len(config.EnabledStrategies))
        for _, s := range e.strategies {
            if enabled[s.Code()] {
                filtered = append(filtered, s)
            }
        }
        e.strategies = filtered
    }
    
    // 3. 执行评分（具体重写 scoreStock 以支持权重覆盖）
    // ... （复用现有逻辑，在 scoreStock 中检查 config.StrategyWeights）
    
    // 4. 后置过滤
    if len(config.Filters) > 0 {
        picks = applyFilters(picks, config.Filters)
    }
    
    // 5. TopN
    if config.TopN > 0 && len(picks) > config.TopN {
        picks = picks[:config.TopN]
    }
    
    return picks, nil
}
```

**scoreStock 改动**：在合成最终分数时，如果 `config.StrategyWeights` 中有对应策略的权重，用权重加权替代默认竞争逻辑。

- [ ] **Step 2: applyFilters + getFilterFieldValue + compareValues**

```go
func applyFilters(picks []models.DailyPick, filters []FilterCondition) []models.DailyPick {
    for _, f := range filters {
        filtered := make([]models.DailyPick, 0, len(picks))
        for _, p := range picks {
            val := getFilterFieldValue(p, f.Field)
            if compareValues(val, f.Op, f.Value) {
                filtered = append(filtered, p)
            }
        }
        picks = filtered
    }
    return picks
}

func getFilterFieldValue(p models.DailyPick, field string) float64 {
    switch field {
    case "score":   return p.Score
    case "price":   return p.ClosePrice
    case "volume":  return p.Volume
    case "turnover": return p.TurnoverFactor
    default:
        // 从 Factors map 读取
        if v, ok := p.Factors[field]; ok { return v }
        return 0
    }
}

func compareValues(val float64, op string, target float64) bool {
    switch op {
    case ">":  return val > target
    case "<":  return val < target
    case ">=": return val >= target
    case "<=": return val <= target
    case "==": return math.Abs(val-target) < 0.001
    default:   return true
    }
}
```

- [ ] **Step 3: 单元测试 applyFilters**

```go
func TestApplyFilters(t *testing.T) {
    picks := []models.DailyPick{
        {Score: 80, ClosePrice: 15.0, Volume: 1000000},
        {Score: 60, ClosePrice: 10.0, Volume: 500000},
        {Score: 40, ClosePrice: 5.0, Volume: 200000},
    }
    
    // Test ">"
    result := applyFilters(picks, []FilterCondition{{Field: "score", Op: ">", Value: 50}})
    assert.Equal(t, 2, len(result))
    
    // Test multiple filters
    result = applyFilters(picks, []FilterCondition{
        {Field: "score", Op: ">", Value: 50},
        {Field: "price", Op: "<", Value: 20},
    })
    assert.Equal(t, 1, len(result))
}
```

- [ ] **Step 4: 运行单元测试**

Run: `go test ./backend/data/ -run TestApplyFilters -v`
Expected: PASS

- [ ] **Step 5: 编译验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```
git add backend/data/daily_pick_engine.go backend/data/daily_pick_config_test.go
git commit -m "feat(daily-pick): add RunWithConfig + applyFilters with tests"
```

---

### Task 6: 实现 AIConfiguredStockPick App 接口

**Files:**
- Modify: `app_common.go` — 新增 AIConfiguredStockPick 方法

- [ ] **Step 1: AIConfiguredStockPick()**

```go
func (a *App) AIConfiguredStockPick(query string, topN int) ([]models.DailyPick, error) {
    if topN <= 0 {
        topN = 10
    }
    
    config, err := callLLMForConfig(query)
    if err != nil {
        logger.SugaredLogger.Warnf("LLM config failed, fallback to default: %v", err)
        engine := NewDailyPickEngine()
        return engine.RunDailyPick(context.Background(), time.Now().Format("2006-01-02"), topN)
    }
    if config.TopN <= 0 {
        config.TopN = topN
    }
    
    engine := NewDailyPickEngine()
    return engine.RunWithConfig(context.Background(), time.Now().Format("2006-01-02"), config)
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```
git add app_common.go
git commit -m "feat(app): add AIConfiguredStockPick Wails binding"
```

---

### Task 7: 前端 SelectStock.vue 添加 AI 配置开关

**Files:**
- Modify: `frontend/src/components/SelectStock.vue` — 添加 AI 切换 + 结果适配

- [ ] **Step 1: 添加 useAIConfig 开关和数据绑定**

```vue
<script setup lang="ts">
// 新增
import {AIConfiguredStockPick} from "../../wailsjs/go/main/App";
const useAIConfig = ref(false)
const aiLoading = ref(false)

// 修改 Search 函数
async function Search() {
  if (!search.value) return
  
  if (useAIConfig.value) {
    aiLoading.value = true
    try {
      const res = await AIConfiguredStockPick(search.value, 10)
      displayAIPickResult(res)
    } catch(e) {
      message.error('AI 配置选股失败: ' + e)
    } finally {
      aiLoading.value = false
    }
    return
  }
  // 原有 SearchStock 逻辑不变...
}

function displayAIPickResult(picks: any[]) {
  if (!picks || picks.length === 0) {
    traceInfo.value = 'AI 配置选股无符合条件的结果'
    columns.value = []
    dataList.value = []
    return
  }
  columns.value = [
    {title: '排名', key: 'rank', width: 60},
    {title: '股票代码', key: 'stockCode', width: 100},
    {title: '股票名称', key: 'stockName', width: 120, ellipsis: {tooltip: true}},
    {title: '评分', key: 'score', width: 80, sorter: (a, b) => a.score - b.score},
    {title: '策略', key: 'strategyName', width: 100},
    {title: '得分原因', key: 'reason', width: 400, ellipsis: {tooltip: true}},
  ]
  dataList.value = picks.map((p, i) => ({
    rank: i + 1,
    stockCode: p.StockCode,
    stockName: p.StockName,
    score: p.Score,
    strategyName: p.StrategyName,
    reason: p.Reason,
    SECURITY_CODE: p.StockCode,
    SECURITY_SHORT_NAME: p.StockName,
  }))
  traceInfo.value = `AI 配置选股结果（共 ${picks.length} 只）`
}
```

```vue
<template>
  <!-- 搜索框旁添加开关 -->
  <n-space align="center" style="margin-bottom: 8px">
    <n-switch v-model:value="useAIConfig" />
    <n-text depth="3">AI配置选股</n-text>
    <n-tag v-if="useAIConfig" type="warning" size="small">消耗token</n-tag>
  </n-space>
  
  <!-- 搜索按钮文字随模式变化 -->
  <n-button type="primary" @click="Search" :loading="aiLoading">
    {{ useAIConfig ? 'AI配置选股' : '搜股' }}
  </n-button>
</template>
```

- [ ] **Step 2: 前端编译验证**

Run: `cd frontend && npx vue-tsc --noEmit` 或 `npm run build`
Expected: PASS

- [ ] **Step 3: Commit**

```
git add frontend/src/components/SelectStock.vue
git commit -m "feat(ui): add AI config toggle to SelectStock page"
```

---

### Task 8: Wails 重新生成绑定

**Files:**
- Auto-generated: `frontend/wailsjs/go/main/App.*`, `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: wails generate module**

Run: `wails generate module`
Expected: 成功生成

- [ ] **Step 2: 验证生成文件包含新接口**

检查 `frontend/wailsjs/go/main/App.d.ts` 和 `App.js` 是否有 `AIConfiguredStockPick`
检查 `frontend/wailsjs/go/models.ts` 是否有 `StrategyConfig` / `FilterCondition`

- [ ] **Step 3: Commit**

```
git add frontend/wailsjs/go/
git commit -m "chore: sync Wails bindings for AIConfiguredStockPick"
```

---

### Task 9: go build 最终验证

- [ ] **Step 1: go build**

Run: `go build ./...`
Expected: exit code 0

- [ ] **Step 2: 运行全部单元测试**

Run: `go test ./backend/data/ -run "TestApplyFilters" -v`
Expected: PASS

---

### Task 10: 端到端手动验证

- [ ] **Step 1: 运行 app，打开 SelectStock 页面**
- [ ] **Step 2: 切换 AI 配置开关，输入"找放量突破且RSI<70的股票"，点击搜索**
- [ ] **Step 3: 验证返回结果表格渲染正确**
- [ ] **Step 4: 切换回普通模式，确认原搜索功能不受影响**
- [ ] **Step 5: 测试空输入/LLM超时等边界情况**
