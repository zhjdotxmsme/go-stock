<template>
  <div class="multi-agent-result">
    <!-- Phase Progress -->
    <div class="phase-bar">
      <div
        v-for="phase in phases"
        :key="phase.key"
        class="phase-step"
        :class="{
          active: phase.key === currentPhase,
          done: phase.done,
        }"
      >
        <n-icon size="18">
          <CheckCircle v-if="phase.done" />
          <Spinner v-else-if="phase.key === currentPhase" />
          <Circle v-else />
        </n-icon>
        <span class="phase-label">{{ phase.label }}</span>
      </div>
    </div>

    <!-- Phase status line -->
    <n-text v-if="phaseLabel" depth="3" class="phase-status">{{ phaseLabel }}</n-text>

    <!-- Collapsible Analyst Reports -->
    <div v-if="visibleAnalysts.length > 0" class="analyst-section">
      <h4 class="section-title">📊 各维度分析</h4>
      <n-collapse>
        <n-collapse-item
          v-for="rep in visibleAnalysts"
          :key="rep.role"
          :title="analystTitle(rep.role) + ' — ' + ratingTag(rep.rating)"
          :name="rep.role"
        >
          <MdPreview :modelValue="rep.content || '分析中...'" />
        </n-collapse-item>
      </n-collapse>
    </div>

    <!-- Debate Section -->
    <div v-if="debates.length > 0" class="debate-section">
      <h4 class="section-title">⚖️ 多空辩论</h4>
      <div v-for="d in debates" :key="d.round + d.side" class="debate-item" :class="d.side">
        <n-tag :type="d.side === 'bull' ? 'success' : 'error'" size="small">
          {{ d.side === 'bull' ? '看多方' : '看空方' }} 第{{ d.round }}轮
        </n-tag>
        <div class="debate-arg">{{ truncate(d.argument, 300) }}</div>
      </div>
    </div>

    <!-- Final Report -->
    <div v-if="finalReport" class="final-section">
      <h4 class="section-title">📝 最终报告</h4>
      <div class="final-rating">
        <n-tag :type="overallRatingType" size="medium">
          {{ ratingLabel(finalReport.overallRating) }}
        </n-tag>
      </div>
      <div class="final-detail">
        <MdPreview :modelValue="finalReport.conclusion || '分析完成'" />
      </div>
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
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { CheckCircle, Circle, Spinner } from '@vicons/fa'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'

const props = defineProps({
  state: {
    type: Object,
    default: () => ({
      currentPhase: '',
      phaseLabel: '',
      phases: {},
      reports: {},
      debates: [],
      finalReport: null,
      done: false,
    }),
  },
})

const phases = [
  { key: 'analysts', label: '分析师分析', done: false },
  { key: 'debate', label: '多空辩论', done: false },
  { key: 'synthesis', label: '生成报告', done: false },
]

const currentPhase = computed(() => props.state.currentPhase)
const phaseLabel = computed(() => props.state.phaseLabel)

// Mark phases as done
phases.forEach((p) => {
  if (props.state.phases && props.state.phases[p.key]) {
    p.done = true
  }
})

const visibleAnalysts = computed(() => {
  const keys = ['fundamental', 'technical', 'sentiment', 'news', 'policy', 'hotmoney', 'lockup']
  return keys
    .filter((k) => props.state.reports[k])
    .map((k) => ({
      role: k,
      content: props.state.reports[k],
      rating: props.state.ratings[k] || 'neutral',
    }))
})

const debates = computed(() => props.state.debates || [])
const finalReport = computed(() => props.state.finalReport)

const overallRatingType = computed(() => {
  if (!finalReport.value) return 'default'
  const r = finalReport.value.overallRating
  if (r === 'buy' || r === 'strong_buy') return 'success'
  if (r === 'sell' || r === 'strong_sell') return 'error'
  return 'warning'
})

function analystTitle(role) {
  const map = { fundamental: '基本面', technical: '技术面', sentiment: '情绪面', news: '新闻面',
    policy: '政策面', hotmoney: '资金面', lockup: '解禁面' }
  return map[role] || role
}

function ratingTag(rating) {
  const map = {
    strong_buy: '强烈看多',
    bullish: '看多',
    neutral: '中性',
    bearish: '看空',
    strong_sell: '强烈看空',
  }
  return map[rating] || rating
}

function ratingLabel(rating) {
  const map = {
    buy: '买入',
    strong_buy: '强烈买入',
    hold: '持有',
    sell: '卖出',
    strong_sell: '强烈卖出',
  }
  return map[rating] || rating
}

function truncate(text, max) {
  if (!text || text.length <= max) return text
  return text.slice(0, max) + '...'
}
</script>

<style scoped>
.multi-agent-result {
  text-align: left;
  padding: 8px 0;
}
.phase-bar {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: center;
  margin-bottom: 12px;
  padding: 12px;
  background: var(--n-color);
  border-radius: 8px;
}
.phase-step {
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0.4;
  transition: opacity 0.3s;
}
.phase-step.active {
  opacity: 1;
  color: #18a058;
}
.phase-step.done {
  opacity: 0.7;
  color: #2080f0;
}
.phase-label {
  font-size: 13px;
  white-space: nowrap;
}
.phase-status {
  display: block;
  text-align: center;
  margin-bottom: 12px;
  font-size: 12px;
}
.section-title {
  margin: 12px 0 8px;
  font-size: 15px;
  font-weight: 600;
}
.analyst-section,
.debate-section,
.final-section {
  margin-bottom: 16px;
}
.debate-item {
  margin: 8px 0;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--n-color);
}
.debate-item.bull {
  border-left: 3px solid #18a058;
}
.debate-item.bear {
  border-left: 3px solid #d03050;
}
.debate-arg {
  margin-top: 4px;
  font-size: 13px;
  line-height: 1.5;
}
.final-rating {
  margin-bottom: 8px;
}
.final-detail {
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--n-color);
}
.final-catalysts ul,
.final-risks ul {
  padding-left: 20px;
}
.final-catalysts li,
.final-risks li {
  margin: 4px 0;
  font-size: 13px;
}
</style>
