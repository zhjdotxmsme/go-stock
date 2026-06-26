# Phase 1: AI 决策仪表盘结构化 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将多智能体分析的最终报告从自由文本升级为结构化决策仪表盘（评分/趋势/买卖区间/风险/检查清单），前端卡片化渲染。

**Architecture:** 后端在现有合成流程尾部增加一次轻量 LLM 结构化提取步骤（`LLMTierQuick`），从已生成的 `Conclusion` 文本中抽取出结构化字段存入 `FinalReport`；前端新建 `DecisionDashboard.vue` 嵌入 `MultiAgentResult.vue`，降级策略保证自由文本始终可用。

**Tech Stack:** Go 1.26, Vue 3 + NaiveUI, md-editor-v3

---

## 文件变更总览

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/agent/multi/types.go` | 修改 | 新增 `PriceZone`/`ChecklistItem` 类型 + FinalReport 结构化字段 |
| `backend/agent/multi/synthesis.go` | 修改 | 尾部增加 `extractStructuredFields()` 轻量 LLM 调用 |
| `frontend/src/components/DecisionDashboard.vue` | **新建** | 仪表盘主组件（编排子组件） |
| `frontend/src/components/ScoreRing.vue` | **新建** | 1-10 评分环 |
| `frontend/src/components/TrendIndicator.vue` | **新建** | 趋势方向箭头（up/down/sideways） |
| `frontend/src/components/PriceZoneCard.vue` | **新建** | 买卖区间价格卡片 |
| `frontend/src/components/RiskBadge.vue` | **新建** | 风险等级标签 |
| `frontend/src/components/ActionChecklist.vue` | **新建** | 操作检查清单 |
| `frontend/src/components/CatalystTimeline.vue` | **新建** | 催化剂时间线 |
| `frontend/src/components/MultiAgentResult.vue` | 修改 | 嵌入 DecisionDashboard |

---

### Task 1: 后端新增 PriceZone/ChecklistItem 类型 + FinalReport 结构字段

**Files:**
- Modify: `backend/agent/multi/types.go`

---

- [ ] **Step 1: 新增 PriceZone 和 ChecklistItem 类型**

在 `types.go` 中 `FinalReport` 之前添加：

```go
// PriceZone 表示价格区间
type PriceZone struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// ChecklistItem 表示操作检查项
type ChecklistItem struct {
	Action      string `json:"action"`      // 操作描述
	Priority    string `json:"priority"`    // high / medium / low
	IsCompleted bool   `json:"is_completed"` // 默认 false
}
```

---

- [ ] **Step 2: FinalReport 增加结构化字段**

将 `FinalReport` 修改为：

```go
type FinalReport struct {
	OverallRating      string // strong_buy / buy / hold / sell / strong_sell
	InvestmentThesis   string
	Strengths          []string
	RiskFactors        []string
	Catalysts          []string
	MultiTimeframeView map[string]string
	Conclusion         string

	// 新增结构化字段
	Score       float64              `json:"score"`        // 1-10 综合评分，0=未评估
	Trend       string               `json:"trend"`        // up / down / sideways
	EntryZone   *PriceZone           `json:"entryZone"`    // 买入区间，nil=未提供
	ExitZone    *PriceZone           `json:"exitZone"`     // 卖出区间，nil=未提供
	RiskLevel   string               `json:"riskLevel"`    // low / medium / high
	Checklist   []ChecklistItem      `json:"checklist"`    // 操作检查清单
}
```

---

- [ ] **Step 3: 编译验证**

Run: `cd /mnt/e/open-source/ai/go-stock && GOTOOLCHAIN=go1.26.4 go vet ./backend/agent/multi/...`
Expected: 无错误

---

- [ ] **Step 4: 提交**

```bash
git add backend/agent/multi/types.go
git commit -m "feat(agent): FinalReport 增加结构化字段 (Score/Trend/PriceZone/Risk/Checklist)"
```

---

### Task 2: 后端结构化提取步骤

**Files:**
- Modify: `backend/agent/multi/synthesis.go`

---

- [ ] **Step 1: 添加结构化提取 prompt 常量**

在 `prompts.go` 末尾添加：

```go
// StructExtractPrompt 从已生成的结论中提取结构化字段
// 输入是 synthesis 输出的 Conclusion 文本，输出为结构化 JSON
const StructExtractPrompt = `你是一位结构化数据提取专家。请从以下股票分析结论中提取结构化信息。

请严格按以下 JSON 格式输出，不要包含任何其他文本：

{
  "score": 0.0,
  "trend": "",
  "entryZone": null,
  "exitZone": null,
  "riskLevel": "",
  "checklist": []
}

字段说明：
- score: 1-10 的综合评分，精确到 0.5。从结论中推断，找不到证据时给 5.0。0=无法评估。
- trend: 趋势方向。up=上涨趋势, down=下跌趋势, sideways=横盘震荡。从结论中判断。
- entryZone: 买入价格区间。如果结论提到了买入价位或支撑位，提取为 {"low": 最低价, "high": 最高价}。未提及则为 null。
- exitZone: 卖出价格区间。如果结论提到了目标价或压力位，提取为 {"low": 最低价, "high": 最高价}。未提及则为 null。
- riskLevel: 风险等级。low=低风险, medium=中等风险, high=高风险。从风险因素数量/严重程度判断。
- checklist: 操作检查清单。从结论中提取最多 5 条具体操作项。[{"action": "操作描述", "priority": "high/medium/low", "is_completed": false}]

分析结论：`
```

---

- [ ] **Step 2: 添加结构化提取函数**

在 `synthesis.go` 末尾添加：

```go
// extractStructuredFields 从已生成的 Conclusion 文本中提取结构化字段。
// 使用轻量 LLM 调用，失败不影响主流程（降级使用默认值）。
func extractStructuredFields(ctx context.Context, ac *AgentContext, report *FinalReport) {
	chatModel, err := GetChatModelWithTier(ctx, "struct_extract", LLMTierQuick, ac.AIConfigID)
	if err != nil {
		logger.SugaredLogger.Warnf("struct extract LLM unavailable, skipping: %v", err)
		return
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: StructExtractPrompt},
		{Role: schema.User, Content: report.Conclusion},
	}

	result, err := chatModel.Generate(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Warnf("struct extract LLM error, skipping: %v", err)
		return
	}

	content := result.Content
	if content == "" {
		return
	}

	// Try to extract JSON from the response (the LLM should output pure JSON)
	// Handle the case where the LLM wraps JSON in markdown codeblocks
	jsonStr := content
	if idx := strings.Index(content, "```json\n"); idx >= 0 {
		content = content[idx+8:]
		if end := strings.Index(content, "\n```"); end >= 0 {
			jsonStr = content[:end]
		}
	} else if idx := strings.Index(content, "```"); idx >= 0 {
		content = content[idx+3:]
		if end := strings.Index(content, "```"); end >= 0 {
			jsonStr = content[:end]
		}
	}

	var extracted struct {
		Score     float64          `json:"score"`
		Trend     string           `json:"trend"`
		EntryZone *PriceZone       `json:"entryZone"`
		ExitZone  *PriceZone       `json:"exitZone"`
		RiskLevel string           `json:"riskLevel"`
		Checklist []ChecklistItem  `json:"checklist"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		logger.SugaredLogger.Warnf("struct extract JSON parse error: %v", err)
		return
	}

	// Apply extracted values (validate ranges)
	if extracted.Score >= 1 && extracted.Score <= 10 {
		report.Score = extracted.Score
	}
	switch extracted.Trend {
	case "up", "down", "sideways":
		report.Trend = extracted.Trend
	}
	if extracted.EntryZone != nil && extracted.EntryZone.Low > 0 && extracted.EntryZone.High > 0 {
		report.EntryZone = extracted.EntryZone
	}
	if extracted.ExitZone != nil && extracted.ExitZone.Low > 0 && extracted.ExitZone.High > 0 {
		report.ExitZone = extracted.ExitZone
	}
	switch extracted.RiskLevel {
	case "low", "medium", "high":
		report.RiskLevel = extracted.RiskLevel
	}
	if len(extracted.Checklist) > 0 {
		report.Checklist = extracted.Checklist
	}

	logger.SugaredLogger.Infof("struct extract successful: score=%.1f trend=%s risk=%s items=%d",
		report.Score, report.Trend, report.RiskLevel, len(report.Checklist))
}
```

---

- [ ] **Step 3: 在 RunSynthesis 尾部调用 extractStructuredFields**

在 `RunSynthesis` 函数末尾，`return report, nil` 之前添加：

```go
	// 结构化提取：从结论文本中提取结构化字段（轻量 LLM 调用）
	extractStructuredFields(ctx, ac, report)
```

---

- [ ] **Step 4: 更新 synthesis.go 的 import**

确保 synthesis.go 导入 `"encoding/json"` 和 `"strings"`：

```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/logger"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)
```

---

- [ ] **Step 5: 编译验证**

Run: `cd /mnt/e/open-source/ai/go-stock && GOTOOLCHAIN=go1.26.4 go vet ./backend/agent/multi/...`
Expected: 无错误

---

- [ ] **Step 6: 提交**

```bash
git add backend/agent/multi/synthesis.go backend/agent/multi/prompts.go
git commit -m "feat(agent): 新增结构化提取步骤 — 从结论中提取 Score/Trend/PriceZone/Risk"
```

---

### Task 3: ScoreRing.vue 评分环组件

**Files:**
- Create: `frontend/src/components/ScoreRing.vue`

---

- [ ] **Step 1: 创建 ScoreRing.vue**

```vue
<template>
  <div class="score-ring" :style="{ '--score': score, '--color': ringColor }">
    <svg viewBox="0 0 120 120" class="ring-svg">
      <circle cx="60" cy="60" r="50" fill="none" stroke="#e5e5e5" stroke-width="8" />
      <circle
        cx="60" cy="60" r="50"
        fill="none"
        :stroke="ringColor"
        stroke-width="8"
        stroke-linecap="round"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="dashOffset"
        transform="rotate(-90, 60, 60)"
        class="ring-fill"
      />
    </svg>
    <div class="score-label">
      <span class="score-value">{{ displayScore }}</span>
      <span class="score-unit">/10</span>
    </div>
    <div class="score-text">{{ label }}</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  score: { type: Number, default: 0 },
})

const circumference = 2 * Math.PI * 50

const displayScore = computed(() => {
  if (props.score <= 0) return '--'
  return props.score.toFixed(1)
})

const dashOffset = computed(() => {
  if (props.score <= 0) return circumference
  const pct = Math.min(props.score / 10, 1)
  return circumference * (1 - pct)
})

const ringColor = computed(() => {
  const s = props.score
  if (s <= 0) return '#999'
  if (s >= 8) return '#18a058'   // green - buy
  if (s >= 6) return '#2080f0'   // blue - hold positive
  if (s >= 4) return '#f0a020'   // orange - hold cautious
  return '#d03050'               // red - sell
})

const label = computed(() => {
  const s = props.score
  if (s <= 0) return '未评估'
  if (s >= 8) return '强烈推荐'
  if (s >= 6) return '推荐'
  if (s >= 4) return '谨慎'
  return '规避'
})
</script>

<style scoped>
.score-ring {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  position: relative;
  width: 120px;
}
.ring-svg {
  width: 120px;
  height: 120px;
}
.ring-fill {
  transition: stroke-dashoffset 0.6s ease;
}
.score-label {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -60%);
  display: flex;
  align-items: baseline;
  gap: 2px;
}
.score-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--score-color, #333);
}
.score-unit {
  font-size: 12px;
  color: #999;
}
.score-text {
  margin-top: 4px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color);
}
</style>
```

- [ ] **Step 2: 提交**

```bash
git add frontend/src/components/ScoreRing.vue
git commit -m "feat(ui): ScoreRing 评分环组件 — 1-10 数值环 + 颜色标签"
```

---

### Task 4: TrendIndicator + PriceZoneCard + RiskBadge

**Files:**
- Create: `frontend/src/components/TrendIndicator.vue`
- Create: `frontend/src/components/PriceZoneCard.vue`
- Create: `frontend/src/components/RiskBadge.vue`

---

- [ ] **Step 1: 创建 TrendIndicator.vue**

```vue
<template>
  <div class="trend-indicator" :class="trend">
    <n-icon size="28" :color="iconColor">
      <ArrowUp v-if="trend === 'up'" />
      <ArrowDown v-else-if="trend === 'down'" />
      <Remove v-else />
    </n-icon>
    <span class="trend-label" :style="{ color: iconColor }">{{ label }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ArrowUp, ArrowDown, Remove } from '@vicons/fa'

const props = defineProps({
  trend: { type: String, default: 'sideways' }, // up / down / sideways
})

const iconColor = computed(() => {
  if (props.trend === 'up') return '#18a058'
  if (props.trend === 'down') return '#d03050'
  return '#999'
})

const label = computed(() => {
  if (props.trend === 'up') return '上涨趋势'
  if (props.trend === 'down') return '下跌趋势'
  return '横盘震荡'
})
</script>

<style scoped>
.trend-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  background: var(--n-color);
}
.trend-indicator.up { border-left: 3px solid #18a058; }
.trend-indicator.down { border-left: 3px solid #d03050; }
.trend-indicator.sideways { border-left: 3px solid #999; }
.trend-label {
  font-size: 14px;
  font-weight: 500;
}
</style>
```

---

- [ ] **Step 2: 创建 PriceZoneCard.vue**

```vue
<template>
  <div class="price-zone-card" :class="type">
    <div class="zone-header">
      <n-icon size="16" :color="iconColor">
        <ArrowRight v-if="type === 'entry'" />
        <ArrowLeft v-else />
      </n-icon>
      <span class="zone-title">{{ title }}</span>
    </div>
    <div class="zone-prices">
      <span class="price-low">¥{{ formatPrice(low) }}</span>
      <span class="price-sep">—</span>
      <span class="price-high">¥{{ formatPrice(high) }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ArrowRight, ArrowLeft } from '@vicons/fa'

const props = defineProps({
  type: { type: String, default: 'entry' }, // entry / exit
  low: { type: Number, required: true },
  high: { type: Number, required: true },
})

const title = computed(() => props.type === 'entry' ? '买入区间' : '卖出区间')
const iconColor = computed(() => props.type === 'entry' ? '#18a058' : '#d03050')

function formatPrice(v) {
  if (!v) return '--'
  return v.toFixed(2)
}
</script>

<style scoped>
.price-zone-card {
  display: inline-flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--n-color);
  min-width: 140px;
}
.price-zone-card.entry { border-left: 3px solid #18a058; }
.price-zone-card.exit { border-left: 3px solid #d03050; }
.zone-header {
  display: flex;
  align-items: center;
  gap: 6px;
}
.zone-title {
  font-size: 13px;
  font-weight: 600;
}
.zone-prices {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.price-low { color: #18a058; }
.price-high { color: #d03050; }
.price-sep { color: #999; font-size: 14px; }
</style>
```

---

- [ ] **Step 3: 创建 RiskBadge.vue**

```vue
<template>
  <div class="risk-badge" :class="level">
    <n-icon size="16">
      <Warning v-if="level === 'high'" />
      <Info v-else-if="level === 'medium'" />
      <CheckCircle v-else />
    </n-icon>
    <span class="risk-label">{{ label }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Warning, Info, CheckCircle } from '@vicons/fa'

const props = defineProps({
  level: { type: String, default: 'medium' }, // low / medium / high
})

const label = computed(() => {
  if (props.level === 'high') return '高风险'
  if (props.level === 'medium') return '中等风险'
  return '低风险'
})
</script>

<style scoped>
.risk-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 16px;
  font-size: 13px;
  font-weight: 500;
}
.risk-badge.low { background: #e8f8e8; color: #18a058; }
.risk-badge.medium { background: #fff3e0; color: #f0a020; }
.risk-badge.high { background: #fce8e8; color: #d03050; }
</style>
```

---

- [ ] **Step 4: 提交**

```bash
git add frontend/src/components/TrendIndicator.vue frontend/src/components/PriceZoneCard.vue frontend/src/components/RiskBadge.vue
git commit -m "feat(ui): TrendIndicator + PriceZoneCard + RiskBadge 仪表盘子组件"
```

---

### Task 5: CatalystTimeline + ActionChecklist

**Files:**
- Create: `frontend/src/components/CatalystTimeline.vue`
- Create: `frontend/src/components/ActionChecklist.vue`

---

- [ ] **Step 1: 创建 CatalystTimeline.vue**

```vue
<template>
  <div class="catalyst-timeline" v-if="items.length > 0">
    <h5 class="tl-title"><n-icon size="16" color="#2080f0"><Rocket /></n-icon> 催化剂</h5>
    <n-timeline>
      <n-timeline-item
        v-for="(item, i) in items"
        :key="i"
        :content="item"
        :type="'info'"
        :time="''"
      />
    </n-timeline>
  </div>
</template>

<script setup>
import { Rocket } from '@vicons/fa'

defineProps({
  items: { type: Array, default: () => [] },
})
</script>

<style scoped>
.catalyst-timeline {
  margin: 8px 0;
}
.tl-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
</style>
```

---

- [ ] **Step 2: 创建 ActionChecklist.vue**

```vue
<template>
  <div class="action-checklist" v-if="items.length > 0">
    <h5 class="cl-title"><n-icon size="16" color="#2080f0"><List /></n-icon> 操作清单</h5>
    <n-checkbox-group v-model:value="completedActions">
      <div v-for="(item, i) in items" :key="i" class="checklist-row">
        <n-checkbox :value="i" :disabled="false">
          <span class="check-action" :class="{ done: completedActions.includes(i) }">
            {{ item.action }}
          </span>
        </n-checkbox>
        <n-tag v-if="item.priority === 'high'" size="tiny" type="error">高优</n-tag>
        <n-tag v-else-if="item.priority === 'low'" size="tiny" type="info">低优</n-tag>
      </div>
    </n-checkbox-group>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { List } from '@vicons/fa'

const props = defineProps({
  items: { type: Array, default: () => [] },
})

const completedActions = ref(
  props.items.map((item, i) => item.isCompleted ? i : -1).filter(i => i >= 0)
)
</script>

<style scoped>
.action-checklist {
  margin: 8px 0;
}
.cl-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
.checklist-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
}
.check-action {
  font-size: 13px;
  transition: color 0.2s;
}
.check-action.done {
  text-decoration: line-through;
  color: #999;
}
</style>
```

---

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/CatalystTimeline.vue frontend/src/components/ActionChecklist.vue
git commit -m "feat(ui): CatalystTimeline + ActionChecklist 仪表盘子组件"
```

---

### Task 6: DecisionDashboard.vue 仪表盘主组件

**Files:**
- Create: `frontend/src/components/DecisionDashboard.vue`

---

- [ ] **Step 1: 创建 DecisionDashboard.vue**

```vue
<template>
  <div class="decision-dashboard" v-if="hasStructuredData">
    <h4 class="dashboard-title">📊 决策仪表盘</h4>
    <div class="dashboard-grid">
      <!-- 评分环 -->
      <div class="db-card score-card">
        <ScoreRing :score="report.score" />
      </div>

      <!-- 趋势方向 -->
      <div class="db-card trend-card">
        <TrendIndicator :trend="report.trend" />
      </div>

      <!-- 买卖区间 -->
      <div class="db-card zone-card" v-if="report.entryZone">
        <PriceZoneCard type="entry" :low="report.entryZone.low" :high="report.entryZone.high" />
      </div>
      <div class="db-card zone-card" v-if="report.exitZone">
        <PriceZoneCard type="exit" :low="report.exitZone.low" :high="report.exitZone.high" />
      </div>

      <!-- 风险等级 -->
      <div class="db-card risk-card" v-if="report.riskLevel">
        <RiskBadge :level="report.riskLevel" />
      </div>
    </div>

    <!-- 催化剂时间线 -->
    <CatalystTimeline v-if="report.catalysts && report.catalysts.length > 0" :items="report.catalysts" />

    <!-- 操作清单 -->
    <ActionChecklist v-if="report.checklist && report.checklist.length > 0" :items="report.checklist" />

    <!-- 多时间维度 -->
    <div v-if="hasTimeframeView" class="timeframe-section">
      <h5 class="tf-title"><n-icon size="16" color="#2080f0"><Clock /></n-icon> 多时间维度</h5>
      <div class="tf-grid">
        <div v-for="(view, period) in report.multiTimeframeView" :key="period" class="tf-item">
          <span class="tf-period">{{ period }}</span>
          <span class="tf-view">{{ view }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Clock } from '@vicons/fa'
import ScoreRing from './ScoreRing.vue'
import TrendIndicator from './TrendIndicator.vue'
import PriceZoneCard from './PriceZoneCard.vue'
import RiskBadge from './RiskBadge.vue'
import CatalystTimeline from './CatalystTimeline.vue'
import ActionChecklist from './ActionChecklist.vue'

const props = defineProps({
  report: { type: Object, default: () => ({}) },
})

const hasStructuredData = computed(() => {
  const r = props.report
  return r.score > 0 || r.trend || r.entryZone || r.exitZone || r.riskLevel ||
    (r.checklist && r.checklist.length > 0) ||
    (r.catalysts && r.catalysts.length > 0)
})

const hasTimeframeView = computed(() => {
  const v = props.report.multiTimeframeView
  return v && Object.keys(v).length > 0
})
</script>

<style scoped>
.decision-dashboard {
  margin: 16px 0;
  padding: 16px;
  background: var(--n-color);
  border-radius: 10px;
  border: 1px solid #e5e5e5;
}
.dashboard-title {
  margin: 0 0 12px;
  font-size: 16px;
  font-weight: 600;
}
.dashboard-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: flex-start;
}
.db-card {
  flex: 0 0 auto;
}
.timeframe-section {
  margin-top: 16px;
}
.tf-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
.tf-grid {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.tf-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 14px;
  border-radius: 8px;
  background: #f5f5f5;
  min-width: 100px;
}
.tf-period {
  font-size: 12px;
  color: #999;
  font-weight: 500;
}
.tf-view {
  font-size: 13px;
  font-weight: 600;
}
</style>
```

---

- [ ] **Step 2: 提交**

```bash
git add frontend/src/components/DecisionDashboard.vue
git commit -m "feat(ui): DecisionDashboard 决策仪表盘主组件 — 编排子组件展示结构化决策数据"
```

---

### Task 7: 集成 DecisionDashboard 到 MultiAgentResult

**Files:**
- Modify: `frontend/src/components/MultiAgentResult.vue`

---

- [ ] **Step 1: 在 MultiAgentResult.vue 中导入并嵌入 DecisionDashboard**

在 template 中，Final Report 区域之前或之后嵌入。建议放在最终报告区域末尾，`final-section` 内部：

```vue
    <!-- Final Report -->
    <div v-if="finalReport" class="final-section">
      <!-- 现有内容保持不变... -->

      <!-- Decision Dashboard (structured data) -->
      <DecisionDashboard :report="finalReport" />

      <!-- 现有的催化剂和风险因素可通过仪表盘展示，但保留 Markdown 预览 -->
      <div class="final-detail">
        <MdPreview :modelValue="finalReport.conclusion || '分析完成'" />
      </div>
    </div>
```

具体修改：在 `final-section` div 中，把 `final-detail`（MdPreview）放在决策仪表盘后面。删除原有的 `final-catalysts` 和 `final-risks` div（它们现在由 DecisionDashboard 的 CatalystTimeline 和 RiskBadge 展示）。

修改后的 `final-section` 区域：

```vue
    <!-- Final Report -->
    <div v-if="finalReport" class="final-section">
      <h4 class="section-title">📝 最终报告</h4>
      <div class="final-rating">
        <n-tag :type="overallRatingType" size="medium">
          {{ ratingLabel(finalReport.overallRating) }}
        </n-tag>
      </div>

      <DecisionDashboard :report="finalReport" />

      <div class="final-detail">
        <MdPreview :modelValue="finalReport.conclusion || '分析完成'" />
      </div>
    </div>
```

删除的代码（约 63-74 行）：
```vue
      <div v-if="finalReport.catalysts && finalReport.catalysts.length > 0" class="final-catalysts">
        <h5>📈 催化剂</h5>
        <ul>
          <li v-for="(c, i) in finalReport.catalysts" :key="i">{{ c }}</li>
        </ul>
      </div>
      <div v-if="finalReport.riskFactors && finalReport.riskFactors.length > 0" class="final-risks">
        <h5>⚠️ 风险因素</h5>
        <ul>
          <li v-for="(r, i) in finalReport.riskFactors" :key="i">{{ r }}</li>
        </ul>
      </div>
```

In `<script setup>`, add the import:

```vue
import DecisionDashboard from './DecisionDashboard.vue'
```

---

- [ ] **Step 2: 提交**

```bash
git add frontend/src/components/MultiAgentResult.vue
git commit -m "feat(ui): 集成 DecisionDashboard 到 MultiAgentResult — 结构化决策卡片展示"
```

---

## 自审检查

### Spec 覆盖率

| Spec 要求 | Task |
|-----------|------|
| FinalReport 新增 Score/Trend/EntryZone/ExitZone/RiskLevel/Checklist | Task 1 |
| PriceZone/ChecklistItem 类型 | Task 1 |
| 结构化提取步骤（轻量 LLM） | Task 2 |
| StructExtractPrompt | Task 2 |
| 降级策略（提取失败不阻塞） | Task 2 (Step 2 函数内静默降级) |
| ScoreRing.vue 评分环 | Task 3 |
| TrendIndicator.vue 趋势方向 | Task 4 |
| PriceZoneCard.vue 买卖区间 | Task 4 |
| RiskBadge.vue 风险等级 | Task 4 |
| CatalystTimeline.vue 催化剂时间线 | Task 5 |
| ActionChecklist.vue 操作清单 | Task 5 |
| DecisionDashboard.vue 主组件 | Task 6 |
| MultiAgentResult.vue 嵌入 | Task 7 |

### 降级一致性检查

- LLM 提取函数内所有 error 只 `Warnf` + `return`，不会导致 RunSynthesis 失败
- Score 默认 0 → ScoreRing 显示 "--"(未评估)
- EntryZone/ExitZone nil → 不渲染 PriceZoneCard
- RiskLevel "" → 不渲染 RiskBadge
- Checklist nil/空 → 不渲染 ActionChecklist
- 自由文本 Conclusion 始终展示（MdPreview 保留）

### 类型一致性检查

- `FinalReport.Score float64` 1-10 → ScoreRing 映射正确
- `FinalReport.Trend string` up/down/sideways → TrendIndicator 映射正确
- `FinalReport.EntryZone *PriceZone` → PriceZoneCard type="entry"
- `FinalReport.ExitZone *PriceZone` → PriceZoneCard type="exit"
- `FinalReport.RiskLevel string` low/medium/high → RiskBadge 映射正确
- `FinalReport.Checklist []ChecklistItem` → ActionChecklist 映射正确

### Placeholder 扫描

- [x] 所有步骤包含完整代码（非 "TBD"/"TODO"）
- [x] 所有步骤包含确切文件路径
- [x] 所有步骤包含提交命令
- [x] 无 "implement similar to" 模式
