<script setup>
import {ref, reactive, onMounted, onUnmounted, nextTick, computed} from 'vue'
import {
  GetCommodityRegistry,
  GetCommodityTechnicals,
  GetCommodityFundamentals,
  GetCommodityCorrelation,
  GetCommodityReport,
  NewCommodityAnalysisStream,
} from '../../wailsjs/go/main/App'
import {GetTradableCommodities} from '../../wailsjs/go/main/App'
import {EventsOn, EventsOff} from '../../wailsjs/runtime'
import {MdPreview} from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import CommodityPriceChart from './CommodityPriceChart.vue'

const registry = ref([])
const selectedCode = ref('XAUUSD')
const selectedName = ref('现货黄金')
const selectedModes = ref(['technical'])
const period = ref('day')
const result = ref('')
const loading = ref(false)
const secondaryCodes = ref('XAGUSD,USCL')
const deepLoading = ref(false)
const aiConfigId = ref(0)
const deepQuestion = ref('')
const showInternationalRef = ref(false)
const currentAsset = ref(null)

const modeOptions = [
  {label: '技术面', value: 'technical'},
  {label: '基本面', value: 'fundamental'},
  {label: '关联分析', value: 'correlation'},
]

const periodOptions = [
  {label: '日线', value: 'day'},
  {label: '周线', value: 'week'},
]

// All possible expert role titles (dynamic routing)
const expertTitles = {
  macro: '宏观',
  technical: '技术面',
  sentiment: '情绪',
  // 贵金属专属
  monetary: '货币属性',
  safehaven: '避险情绪',
  // 能源专属
  oil_supply: '供需基本面',
  oil_geo: '地缘政治',
  // 基金专属
  fund_tracking: '基金跟踪',
  // debate/synthesis
  bull: '看多',
  bear: '看空',
  synthesis: '综合',
}

const expertState = reactive({
  active: false,
  currentPhase: '',
  phaseLabel: '',
  phases: {},
  reports: {},
  debates: [],
  finalReport: null,
  done: false,
})

// Dynamic expert order: use the order tokens arrive via SSE
const expertOrder = computed(() => {
  return Object.keys(expertState.reports).filter(key =>
    !['bull', 'bear', 'synthesis'].includes(key)
  )
})

function expertTitle(role) {
  return expertTitles[role] || role
}

const scrollRef = ref(null)

function scrollToBottom() {
  nextTick(() => {
    if (scrollRef.value) {
      scrollRef.value.scrollTop = scrollRef.value.scrollHeight
    }
  })
}

async function loadRegistry() {
  const all = await GetCommodityRegistry()
  registry.value = all
  // 仅展示可交易标的（排除宏观指标）
  const tradable = await GetTradableCommodities()
  currentAsset.value = tradable.find(i => i.code === selectedCode.value) || null
}

function assetOptions() {
  // 仅展示可交易标的
  return registry.value.filter(i => i.isTradable).map(item => ({label: item.name, value: item.code}))
}

function onCodeChange(val) {
  const item = registry.value.find(i => i.code === val)
  selectedName.value = item ? item.name : val
  currentAsset.value = item || null
}

async function runAnalysis() {
  if (selectedModes.value.length === 0) return
  loading.value = true
  result.value = ''
  try {
    const parts = []
    for (const mode of selectedModes.value) {
      if (mode === 'technical') {
        const r = await GetCommodityTechnicals(selectedCode.value, period.value)
        parts.push('## 技术面分析\n' + r)
      } else if (mode === 'fundamental') {
        const r = await GetCommodityFundamentals(selectedCode.value)
        parts.push('## 基本面分析\n' + r)
      } else if (mode === 'correlation') {
        const r = await GetCommodityCorrelation(selectedCode.value, secondaryCodes.value)
        parts.push('## 关联分析\n' + r)
      }
    }
    if (selectedModes.value.length > 1) {
      const report = await GetCommodityReport(selectedCode.value, '周报')
      parts.push('## 综合报告\n' + report)
    }
    result.value = parts.join('\n\n')
  } catch (e) {
    result.value = '分析失败: ' + (e.message || e)
  } finally {
    loading.value = false
  }
}

function resetExpertState() {
  expertState.active = false
  expertState.currentPhase = ''
  expertState.phaseLabel = ''
  expertState.phases = {}
  expertState.reports = {}
  expertState.debates = []
  expertState.finalReport = null
  expertState.done = false
}

function runDeepAnalysis() {
  resetExpertState()
  deepLoading.value = true
  expertState.active = true

  NewCommodityAnalysisStream(
    selectedCode.value,
    selectedName.value,
    deepQuestion.value || `请全面分析${selectedName.value}(${selectedCode.value})的走势和投资机会`,
    aiConfigId.value,
  )
}

EventsOn('commodityAnalysisStream', (msg) => {
  if (msg === 'DONE') {
    deepLoading.value = false
    expertState.done = true
    scrollToBottom()
    return
  }

  try {
    const rawContent = msg.Content || msg.content || ''
    let parsed = null
    if (typeof rawContent === 'string' && rawContent.startsWith('{')) {
      parsed = JSON.parse(rawContent)
    }

    if (parsed && parsed.type === 'agent:phase') {
      expertState.currentPhase = parsed.phase
      expertState.phaseLabel = parsed.label
      if (parsed.status === 'end') {
        expertState.phases[parsed.phase] = true
      }
      scrollToBottom()
      return
    }

    if (parsed && parsed.type === 'agent:token') {
      const agent = parsed.agent
      if (!expertState.reports[agent]) {
        expertState.reports[agent] = ''
      }
      expertState.reports[agent] += parsed.token
      scrollToBottom()
      return
    }

    if (parsed && parsed.type === 'agent:debate') {
      expertState.debates.push({
        round: parsed.round,
        side: parsed.side,
        argument: parsed.argument,
      })
      scrollToBottom()
      return
    }

    if (parsed && parsed.type === 'agent:final') {
      expertState.finalReport = parsed.report
      scrollToBottom()
      return
    }
  } catch (e) {
    // non-structured message, ignore
  }
})

onMounted(() => {
  loadRegistry()
})

onUnmounted(() => {
  EventsOff('commodityAnalysisStream')
})
</script>

<template>
  <n-space vertical>
    <n-space align="center">
      <n-select
        v-model:value="selectedCode"
        :options="assetOptions()"
        style="width: 160px"
        @update:value="onCodeChange"
      />
      <n-select v-model:value="period" :options="periodOptions" style="width: 120px"/>
    </n-space>

    <n-divider style="margin: 4px 0">价格走势</n-divider>

    <n-space align="center" style="margin-bottom: 4px" v-if="currentAsset && currentAsset.internationalRef">
      <n-switch v-model:value="showInternationalRef" size="small" />
      <n-text depth="3" style="font-size: 12px;">国际参考 (COMEX: {{ currentAsset.internationalRef }})</n-text>
      <n-tag v-if="showInternationalRef" type="warning" size="tiny">COMEX 国际价</n-tag>
    </n-space>

    <CommodityPriceChart
      :code="selectedCode"
      :period="period"
      :count="120"
      :chart-height="280"
      :international-ref="showInternationalRef"
    />

    <n-divider style="margin: 4px 0">快速分析</n-divider>

    <n-space align="center">
      <n-checkbox-group v-model:value="selectedModes" :options="modeOptions"/>
      <n-button type="primary" :loading="loading" @click="runAnalysis">AI 分析</n-button>
    </n-space>

    <n-space v-if="selectedModes.includes('correlation')" align="center">
      <n-text depth="3">关联品种:</n-text>
      <n-input v-model:value="secondaryCodes" style="width: 240px"/>
    </n-space>

    <n-spin :show="loading">
      <n-card v-if="result" size="small">
        <MdPreview v-if="result" :modelValue="result" :theme="'light'"/>
        <div v-else class="whitespace-pre-wrap">{{ result }}</div>
      </n-card>
    </n-spin>

    <n-divider style="margin: 4px 0">多专家深度分析</n-divider>

    <n-space vertical>
      <n-space align="center">
        <n-input
          v-model:value="deepQuestion"
          placeholder="输入分析问题（留空则全面分析）"
          style="width: 400px"
          @keyup.enter="runDeepAnalysis"
        />
        <n-input-number v-model:value="aiConfigId" :min="0" placeholder="AI配置ID" style="width: 120px"/>
        <n-button type="warning" :loading="deepLoading" @click="runDeepAnalysis">
          启动多专家分析
        </n-button>
      </n-space>

      <div ref="scrollRef" style="max-height: 70vh; overflow-y: auto; padding-right: 4px">
        <template v-if="expertState.active">
          <n-space vertical size="large">
            <n-alert :type="expertState.done ? 'success' : 'info'" :show-icon="true">
              {{ expertState.done ? '分析完成' : (expertState.phaseLabel || '准备中...') }}
            </n-alert>

            <!-- Dynamic expert cards: rendered in SSE arrival order -->
            <n-card
              v-for="role in expertOrder"
              :key="role"
              size="small"
              :title="`${expertTitle(role)}专家`"
            >
              <MdPreview :modelValue="expertState.reports[role]" :theme="'light'"/>
            </n-card>

            <n-card v-if="expertState.debates.length > 0" size="small" title="多空辩论">
              <n-space vertical>
                <div v-for="(d, i) in expertState.debates" :key="i">
                  <n-tag :type="d.side === 'bull' ? 'success' : 'error'" size="small">
                    {{ d.side === 'bull' ? '看多' : '看空' }} · 第{{ d.round }}轮
                  </n-tag>
                  <div class="debate-text">{{ d.argument }}</div>
                </div>
              </n-space>
            </n-card>

            <n-card v-if="expertState.finalReport" size="small" title="综合报告">
              <n-space vertical>
                <n-space align="center">
                  <n-tag :type="
                    expertState.finalReport.overallRating === 'buy' ? 'success' :
                    expertState.finalReport.overallRating === 'sell' ? 'error' : 'default'
                  ">
                    评级: {{ expertState.finalReport.overallRating || '待定' }}
                  </n-tag>
                  <n-tag v-if="expertState.finalReport.score" type="info">
                    评分: {{ expertState.finalReport.score }}/10
                  </n-tag>
                  <n-tag v-if="expertState.finalReport.trend">
                    趋势: {{ expertState.finalReport.trend }}
                  </n-tag>
                  <n-tag v-if="expertState.finalReport.riskLevel" :type="
                    expertState.finalReport.riskLevel === 'high' ? 'warning' : 'default'
                  ">
                    风险: {{ expertState.finalReport.riskLevel }}
                  </n-tag>
                </n-space>

                <MdPreview
                  v-if="expertState.finalReport.conclusion"
                  :modelValue="expertState.finalReport.conclusion"
                  :theme="'light'"
                />

                <n-space v-if="expertState.finalReport.entryZone" align="center">
                  <n-text>买入区间: {{ expertState.finalReport.entryZone.low }} ~ {{ expertState.finalReport.entryZone.high }}</n-text>
                </n-space>
                <n-space v-if="expertState.finalReport.exitZone" align="center">
                  <n-text>目标区间: {{ expertState.finalReport.exitZone.low }} ~ {{ expertState.finalReport.exitZone.high }}</n-text>
                </n-space>

                <div v-if="expertState.finalReport.catalysts && expertState.finalReport.catalysts.length > 0">
                  <n-text strong>催化剂</n-text>
                  <ul>
                    <li v-for="c in expertState.finalReport.catalysts" :key="c">{{ c }}</li>
                  </ul>
                </div>
                <div v-if="expertState.finalReport.riskFactors && expertState.finalReport.riskFactors.length > 0">
                  <n-text strong>风险因素</n-text>
                  <ul>
                    <li v-for="r in expertState.finalReport.riskFactors" :key="r">{{ r }}</li>
                  </ul>
                </div>
                <div v-if="expertState.finalReport.checklist && expertState.finalReport.checklist.length > 0">
                  <n-text strong>操作清单</n-text>
                  <ul>
                    <li v-for="item in expertState.finalReport.checklist" :key="item.action">
                      <n-tag size="tiny" :type="item.priority === 'high' ? 'warning' : 'default'">{{ item.priority }}</n-tag>
                      {{ item.action }}
                    </li>
                  </ul>
                </div>
              </n-space>
            </n-card>
          </n-space>
        </template>
      </div>
    </n-space>
  </n-space>
</template>

<style scoped>
.whitespace-pre-wrap {
  white-space: pre-wrap;
}
.debate-text {
  margin-top: 4px;
  padding: 8px;
  background: var(--n-color);
  border-radius: 4px;
  white-space: pre-wrap;
}
</style>
