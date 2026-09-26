<template>
  <n-space vertical>
    <n-page-header>
      <template #title>
        <n-text strong>狩猎场</n-text>
      </template>
      <template #extra>
        <n-space align="center">
          <n-text v-if="lastRunText" depth="3" style="font-size:12px">{{ lastRunText }}</n-text>
          <n-button type="primary" :loading="running" @click="run">
            <template #icon><n-icon><PlayOutline /></n-icon></template>立即运行
          </n-button>
        </n-space>
      </template>
    </n-page-header>

    <n-tabs type="line" animated v-model:value="activeTab">
      <!-- ========== Tab 1: 评分排行 ========== -->
      <n-tab-pane name="scores" tab="评分排行">
        <n-space vertical>
          <n-space align="center">
            <n-date-picker v-model:value="scoreDateTs" type="date" clearable placeholder="评分日期" style="width:160px" @update:value="loadScores" />
            <n-button size="small" :loading="scoresLoading" @click="loadScores">刷新</n-button>
            <n-text depth="3" style="font-size:12px">共 {{ scores.length }} 条</n-text>
          </n-space>
          <n-data-table :columns="scoreColumns" :data="scores" :loading="scoresLoading" :bordered="true"
            :single-line="false" :max-height="560" striped :row-key="(r: any) => r.id" />
        </n-space>
      </n-tab-pane>

      <!-- ========== Tab 2: 策略信号 ========== -->
      <n-tab-pane name="signals" tab="策略信号">
        <n-space vertical>
          <n-space align="center">
            <n-date-picker v-model:value="signalDateTs" type="date" clearable placeholder="信号日期" style="width:160px" @update:value="loadSignals" />
            <n-select v-model:value="strategyFilter" :options="strategyOptions" clearable placeholder="全部策略" style="width:180px" />
            <n-button size="small" :loading="signalsLoading" @click="loadSignals">刷新</n-button>
            <n-text depth="3" style="font-size:12px">共 {{ filteredSignals.length }} 条</n-text>
          </n-space>
          <n-data-table :columns="signalColumns" :data="filteredSignals" :loading="signalsLoading" :bordered="true"
            :single-line="false" :max-height="560" striped :row-key="(r: any) => r.id" />
        </n-space>
      </n-tab-pane>

      <!-- ========== Tab 3: 回测结果 ========== -->
      <n-tab-pane name="backtest" tab="回测结果">
        <n-space vertical>
          <n-space align="center">
            <n-date-picker v-model:value="btForm.startTs" type="date" clearable placeholder="开始日期" style="width:150px" />
            <n-date-picker v-model:value="btForm.endTs" type="date" clearable placeholder="结束日期" style="width:150px" />
            <n-input-number v-model:value="btForm.holdingDays" :min="1" :max="60" placeholder="持有天数" style="width:120px" />
            <n-input v-model:value="btForm.codes" placeholder="股票代码（逗号分隔，留空=狩猎池）" style="width:260px" />
            <n-button type="primary" :loading="btRunning" @click="startBacktest">开始回测</n-button>
          </n-space>
          <n-progress v-if="btRunning || btProgress.total > 0" type="line" :percentage="btPercent"
            :indicator-placement="'inside'" processing />
          <n-data-table :columns="btColumns" :data="btStats" :bordered="true" :single-line="false" striped
            :row-key="(r: any) => r.Strategy || r.strategy" />
        </n-space>
      </n-tab-pane>

      <!-- ========== Tab 4: 风险看板 ========== -->
      <n-tab-pane name="risk" tab="风险看板">
        <n-space vertical>
          <n-button size="small" :loading="riskLoading" @click="loadRisk" style="align-self:flex-start">刷新</n-button>
          <n-alert v-if="risk?.level === '崩溃'" type="error" title="崩溃档预警" :bordered="false">
            当前市场处于崩溃档，仓位上限 {{ pct(risk.positionLimit) }}，请严格控制风险敞口。
          </n-alert>
          <n-alert v-else-if="risk?.level === '危险'" type="warning" title="危险档提示" :bordered="false">
            当前市场处于危险档，仓位上限 {{ pct(risk.positionLimit) }}，注意控制风险。
          </n-alert>
          <n-card v-if="risk" size="small" style="max-width:720px">
            <n-grid :cols="4" :x-gap="12">
              <n-gi>
                <n-statistic label="风险档位">
                  <n-tag :type="riskTagType" size="large" style="font-weight:bold">{{ risk.level || '-' }}</n-tag>
                </n-statistic>
              </n-gi>
              <n-gi><n-statistic label="仓位上限" :value="pct(risk.positionLimit)" /></n-gi>
              <n-gi><n-statistic label="VaR 1日" :value="pct(risk.var1d)" /></n-gi>
              <n-gi><n-statistic label="VaR 5日" :value="pct(risk.var5d)" /></n-gi>
            </n-grid>
            <n-text depth="3" style="font-size:12px">评估日期 {{ risk.date || '-' }}<template v-if="risk.scoreExtra">，评分加成 {{ risk.scoreExtra }}</template></n-text>
          </n-card>
          <n-empty v-else description="暂无风险档位数据，请先运行流水线" />
          <n-card size="small" style="max-width:720px" title="各策略凯利建议仓位">
            <n-data-table v-if="kelly.length" :columns="kellyColumns" :data="kelly" :loading="kellyLoading"
              :bordered="true" :single-line="false" striped size="small" :pagination="false" />
            <n-empty v-else description="未配置凯利参数" />
          </n-card>
        </n-space>
      </n-tab-pane>
    </n-tabs>
  </n-space>
</template>

<script setup lang="ts">
import { h, onMounted, onUnmounted, ref, reactive, computed } from 'vue'
import { NTag, NText, useMessage } from 'naive-ui'
import { PlayOutline } from '@vicons/ionicons5'
import { format } from 'date-fns'

import RadarChart from './charts/RadarChart.vue'
import { runPipeline, getScores, getSignals, getHunting, getRiskLevel, getKellySuggestions, runBacktest } from '../api/khunter'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'

const message = useMessage()
const today = format(new Date(), 'yyyy-MM-dd')
const activeTab = ref('scores')

// ===== 页头：运行流水线 =====
const running = ref(false)
const lastRunText = ref('')

async function run() {
  if (running.value) return
  running.value = true
  lastRunText.value = '流水线运行中...'
  try {
    await runPipeline(today)
  } catch (e) {
    running.value = false
    lastRunText.value = ''
    message.error('启动失败: ' + e)
  }
}

// ===== Tab 1: 评分排行 =====
const scoreDateTs = ref<number | null>(Date.now())
const scores = ref<any[]>([])
const scoresLoading = ref(false)

const scoreIndicators = [
  { name: '技术', max: 100 }, { name: '资金', max: 100 }, { name: '基本面', max: 100 },
  { name: '板块', max: 100 }, { name: '事件', max: 100 },
]

function scoreDateStr(): string {
  return scoreDateTs.value ? format(new Date(scoreDateTs.value), 'yyyy-MM-dd') : today
}

async function loadScores() {
  scoresLoading.value = true
  try {
    scores.value = await getScores(scoreDateStr())
  } catch (e) { message.error('加载评分失败: ' + e)
  } finally { scoresLoading.value = false }
}

function levelTagType(level: string): 'success' | 'info' | 'default' | 'warning' | 'error' {
  switch (level) {
    case '强烈推荐': return 'success'
    case '推荐': return 'info'
    case '谨慎': return 'warning'
    case '回避':
    case '淘汰': return 'error'
    default: return 'default'
  }
}

function renderLevel(row: any) {
  const parts: any[] = [
    h(NTag, { type: levelTagType(row.level), size: 'small', style: { fontWeight: 'bold' } }, { default: () => row.level || '-' }),
  ]
  if (row.degraded) {
    parts.push(h(NTag, { type: 'warning', size: 'tiny', bordered: false, style: { marginLeft: '4px' } }, { default: () => '降级' }))
  }
  return h('span', parts)
}

function renderScoreExpand(row: any) {
  return h('div', { style: { display: 'flex', gap: '24px', alignItems: 'center', padding: '4px 12px' } }, [
    h('div', { style: { width: '320px', flexShrink: 0 } }, [
      h(RadarChart, {
        indicators: scoreIndicators,
        data: [row.technical ?? 0, row.moneyflow ?? 0, row.fundamental ?? 0, row.sector ?? 0, row.event ?? 0],
        height: 220,
      }),
    ]),
    h('div', [
      row.vetoReason
        ? h(NText, { type: 'error' }, { default: () => `否决原因：${row.vetoReason}` })
        : h(NText, { depth: 3 }, { default: () => '无否决项' }),
    ]),
  ])
}

const scoreColumns: any[] = [
  { type: 'expand', renderExpand: renderScoreExpand },
  { title: '代码', key: 'code', width: 100 },
  { title: '名称', key: 'name', width: 110, render: (r: any) => r.name || '-' },
  { title: '总分', key: 'total', width: 90, align: 'center', sorter: (a: any, b: any) => a.total - b.total, defaultSortOrder: 'descend',
    render: (r: any) => h(NText, { strong: true }, { default: () => (r.total ?? 0).toFixed(1) }) },
  { title: '等级', key: 'level', width: 110, align: 'center', render: renderLevel },
  { title: '技术', key: 'technical', width: 70, align: 'center', render: (r: any) => (r.technical ?? 0).toFixed(0) },
  { title: '资金', key: 'moneyflow', width: 70, align: 'center', render: (r: any) => (r.moneyflow ?? 0).toFixed(0) },
  { title: '基本面', key: 'fundamental', width: 70, align: 'center', render: (r: any) => (r.fundamental ?? 0).toFixed(0) },
  { title: '板块', key: 'sector', width: 70, align: 'center', render: (r: any) => (r.sector ?? 0).toFixed(0) },
  { title: '事件', key: 'event', width: 70, align: 'center', render: (r: any) => (r.event ?? 0).toFixed(0) },
  { title: '否决原因', key: 'vetoReason', ellipsis: { tooltip: true },
    render: (r: any) => r.vetoReason ? h(NText, { type: 'error', style: { fontSize: '12px' } }, { default: () => r.vetoReason }) : '-' },
]

// ===== Tab 2: 策略信号 =====
const signalDateTs = ref<number | null>(Date.now())
const signals = ref<any[]>([])
const signalsLoading = ref(false)
const strategyFilter = ref<string | null>(null)

const strategyOptions = computed(() =>
  Array.from(new Set(signals.value.map((s: any) => s.strategy).filter(Boolean)))
    .map(s => ({ label: s, value: s }))
)

const filteredSignals = computed(() =>
  strategyFilter.value ? signals.value.filter((s: any) => s.strategy === strategyFilter.value) : signals.value
)

function signalDateStr(): string {
  return signalDateTs.value ? format(new Date(signalDateTs.value), 'yyyy-MM-dd') : today
}

async function loadSignals() {
  signalsLoading.value = true
  try {
    signals.value = await getSignals(signalDateStr())
  } catch (e) { message.error('加载信号失败: ' + e)
  } finally { signalsLoading.value = false }
}

function parseReasons(raw: string): string {
  if (!raw) return '-'
  try {
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr.join('；') : String(raw)
  } catch { return raw }
}

const signalColumns: any[] = [
  { title: '代码', key: 'code', width: 100 },
  { title: '名称', key: 'name', width: 110 },
  { title: '策略', key: 'strategy', width: 120,
    render: (r: any) => h(NTag, { size: 'small', bordered: false }, { default: () => r.strategy }) },
  { title: '信号日', key: 'signalDate', width: 105 },
  { title: '关键日', key: 'keyDate', width: 150,
    render: (r: any) => r.keyDate ? `${r.keyDate}${r.keyDateType ? ' (' + r.keyDateType + ')' : ''}` : '-' },
  { title: '收盘', key: 'close', width: 80, align: 'right', render: (r: any) => r.close ? r.close.toFixed(2) : '-' },
  { title: '量比', key: 'volumeRatio', width: 70, align: 'center', render: (r: any) => r.volumeRatio ? r.volumeRatio.toFixed(2) : '-' },
  { title: '理由', key: 'reasons', ellipsis: { tooltip: true }, render: (r: any) => parseReasons(r.reasons) },
]

// ===== Tab 3: 回测结果 =====
const btForm = reactive({
  startTs: Date.now() - 180 * 24 * 3600 * 1000,
  endTs: Date.now(),
  holdingDays: 5,
  codes: '',
})
const btRunning = ref(false)
const btProgress = reactive({ done: 0, total: 0 })
const btStats = ref<any[]>([])

const btPercent = computed(() =>
  btProgress.total > 0 ? Math.round((btProgress.done / btProgress.total) * 100) : 0
)

async function startBacktest() {
  if (btRunning.value) return
  let codes = btForm.codes.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean)
  if (codes.length === 0) {
    try {
      const hunting = await getHunting('追踪中')
      codes = hunting.map((it: any) => it.code).filter(Boolean)
    } catch { /* 忽略，走下面的空校验 */ }
  }
  if (codes.length === 0) {
    message.warning('请输入股票代码，或先将股票加入狩猎池')
    return
  }
  btRunning.value = true
  btProgress.done = 0
  btProgress.total = 0
  try {
    await runBacktest(
      codes,
      format(new Date(btForm.startTs), 'yyyy-MM-dd'),
      format(new Date(btForm.endTs), 'yyyy-MM-dd'),
      btForm.holdingDays || 5,
    )
  } catch (e) {
    btRunning.value = false
    message.error('回测启动失败: ' + e)
  }
}

function renderPLRatio(row: any) {
  const ratio = row.ProfitLossRatio ?? row.profitLossRatio ?? 0
  const wins = row.Wins ?? row.wins ?? 0
  if (!ratio) return wins > 0 ? '全胜' : '—'
  return ratio.toFixed(2)
}

const btColumns: any[] = [
  { title: '策略', key: 'Strategy', render: (r: any) => r.Strategy ?? r.strategy },
  { title: '笔数', key: 'Total', width: 80, align: 'center', render: (r: any) => r.Total ?? r.total ?? 0 },
  { title: '胜率', key: 'WinRate', width: 100, align: 'center',
    render: (r: any) => (((r.WinRate ?? r.winRate ?? 0) * 100).toFixed(1) + '%') },
  { title: '盈亏比', key: 'ProfitLossRatio', width: 100, align: 'center', render: renderPLRatio },
]

// ===== Tab 4: 风险看板 =====
const risk = ref<any>(null)
const riskLoading = ref(false)

const riskTagType = computed((): 'success' | 'info' | 'warning' | 'error' => {
  switch (risk.value?.level) {
    case '正常': return 'success'
    case '注意': return 'info'
    case '危险': return 'warning'
    case '崩溃': return 'error'
    default: return 'info'
  }
})

function pct(v: number | null | undefined): string {
  if (v == null) return '-'
  return (v * 100).toFixed(1) + '%'
}

async function loadRisk() {
  riskLoading.value = true
  try {
    risk.value = await getRiskLevel()
  } catch (e) { message.error('加载风险档位失败: ' + e)
  } finally { riskLoading.value = false }
}

// 各策略凯利建议仓位（半凯利，spec D6）
const kelly = ref<any[]>([])
const kellyLoading = ref(false)

const kellyColumns: any[] = [
  { title: '策略', key: 'strategy', render: (r: any) => r.strategy ?? r.Strategy },
  { title: '胜率', key: 'winRate', width: 100, align: 'center',
    render: (r: any) => (((r.winRate ?? 0) * 100).toFixed(1) + '%') },
  { title: '盈亏比', key: 'plRatio', width: 100, align: 'center',
    render: (r: any) => (r.plRatio ?? 0).toFixed(2) },
  { title: '建议仓位', key: 'fraction', width: 120, align: 'center',
    render: (r: any) => {
      const f = r.fraction ?? 0
      return f > 0 ? (f * 100).toFixed(1) + '%' : '不建议开仓'
    } },
]

async function loadKelly() {
  kellyLoading.value = true
  try {
    kelly.value = await getKellySuggestions()
  } catch (e) { message.error('加载凯利建议失败: ' + e)
  } finally { kellyLoading.value = false }
}

// ===== 后端事件 =====
EventsOn('khunter:progress', (msg: any) => {
  if (!msg || typeof msg !== 'object' || !msg.done) return
  running.value = false
  if (msg.error) {
    lastRunText.value = '最近运行：失败'
    message.error('流水线失败: ' + msg.error)
    return
  }
  const r = msg.result || {}
  lastRunText.value = `最近运行：信号 ${r.Signals ?? 0} / 候选 ${r.Candidates ?? 0} / 评分 ${r.Scored ?? 0} / 入池 ${r.Hunted ?? 0} / 移除 ${r.Removed ?? 0}`
  message.success('狩猎场流水线运行完成')
  loadScores(); loadSignals(); loadRisk()
})

EventsOn('khunter:backtest_progress', (msg: any) => {
  if (!msg || typeof msg !== 'object') return
  btProgress.done = msg.done ?? 0
  btProgress.total = msg.total ?? 0
})

EventsOn('khunter:backtest_done', (msg: any) => {
  btRunning.value = false
  if (!msg || typeof msg !== 'object') return
  if (msg.error) {
    message.error('回测失败: ' + msg.error)
    return
  }
  btStats.value = msg.stats || []
  message.success('回测完成')
})

onUnmounted(() => {
  EventsOff('khunter:progress')
  EventsOff('khunter:backtest_progress')
  EventsOff('khunter:backtest_done')
})

onMounted(() => {
  loadScores(); loadSignals(); loadRisk(); loadKelly()
})
</script>

<style scoped>
.n-card { --n-padding: 12px; }
</style>
