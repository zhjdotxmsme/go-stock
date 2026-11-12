<template>
  <n-space vertical>
    <n-page-header>
      <template #title>
        <n-text strong>狩猎场</n-text>
      </template>
      <template #extra>
        <n-space align="center">
          <n-text depth="3" style="font-size:12px">运行日期</n-text>
          <n-date-picker v-model:value="runDateTs" type="date" clearable placeholder="运行日期" style="width:150px" @update:value="loadScores" />
          <n-text v-if="lastRunText" depth="3" style="font-size:12px">{{ lastRunText }}</n-text>
          <n-button type="primary" :loading="running" @click="run">
            <template #icon><n-icon><PlayOutline /></n-icon></template>运行当日策略
          </n-button>
        </n-space>
      </template>
    </n-page-header>

    <n-alert type="info" :bordered="false" collapsible>
      使用顺序：选择交易日 → 运行当日策略 → 查看「候选评分」；符合风险阈值且站上支撑位的股票进入「狩猎池」。
      盘中运行时当日日 K 尚未完整，结果只代表已形成的收盘数据。
    </n-alert>

    <n-tabs type="line" animated v-model:value="activeTab">
      <n-tab-pane name="overview" tab="运行概览">
        <n-space vertical>
          <n-grid :cols="6" :x-gap="12" :y-gap="12">
            <n-gi v-for="m in runSummaryMetrics" :key="m.label">
              <n-statistic :label="m.label" :tabular-nums="true">{{ m.value }}</n-statistic>
            </n-gi>
          </n-grid>
          <n-alert v-if="overviewIssue" :type="overviewIssue.type" :bordered="false" closable>
            {{ overviewIssue.text }}
          </n-alert>
          <n-card size="small" title="运行记录">
            <template #header-extra>
              <n-text depth="3" style="font-size:12px">点击任一行可切换到该交易日</n-text>
            </template>
            <n-data-table
              :columns="runSummaryColumns"
              :data="runSummaries"
              :loading="runSummariesLoading"
              :bordered="true"
              :single-line="false"
              size="small"
              :max-height="380"
              :row-key="(r: any) => r.id"
              :row-props="runSummaryRowProps"
            />
          </n-card>
        </n-space>
      </n-tab-pane>

      <!-- ========== 候选评分 ========== -->
      <n-tab-pane name="scores" tab="候选评分">
        <n-space vertical>
          <n-space align="center">
            <n-text depth="3" style="font-size:12px">日期：{{ runDateStr() }}</n-text>
            <n-button size="small" :loading="scoresLoading" @click="loadScores">刷新</n-button>
            <n-text depth="3" style="font-size:12px">共 {{ scores.length }} 条</n-text>
          </n-space>
          <n-alert v-if="scores.length === 0 && scoreEmptyReason" type="warning" :bordered="false">
            {{ scoreEmptyReason }}
          </n-alert>
          <n-data-table :columns="scoreColumns" :data="scores" :loading="scoresLoading" :bordered="true"
            :single-line="false" :max-height="560" striped :row-key="(r: any) => r.id" />
        </n-space>
      </n-tab-pane>

      <!-- ========== 狩猎池 ========== -->
      <n-tab-pane name="hunting" tab="狩猎池">
        <n-space vertical>
          <n-space align="center">
            <n-select v-model:value="huntingStatus" :options="huntingStatusOptions" style="width:130px" @update:value="loadHunting" />
            <n-button size="small" :loading="huntingLoading" @click="loadHunting">刷新</n-button>
            <n-text depth="3" style="font-size:12px">共 {{ hunting.length }} 条；狩猎池用于跟踪和回测</n-text>
          </n-space>
          <n-data-table :columns="huntingColumns" :data="hunting" :loading="huntingLoading" :bordered="true"
            :single-line="false" :max-height="560" striped :row-key="(r: any) => r.id" />
        </n-space>
      </n-tab-pane>

      <!-- ========== Tab 3: 回测结果 ========== -->
      <n-tab-pane name="backtest" tab="回测验证">
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
      <n-tab-pane name="risk" tab="风险与仓位">
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
import { runPipeline, getKhunterMeta, getScores, getHunting, getRunSummaries, getRiskLevel, getKellySuggestions, runBacktest } from '../api/khunter'
import type { models } from '../../wailsjs/go/models'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { displayStockCode, resolveDisplayName, prepareStockDisplay, fillRowNames } from '../utils/stockDisplay'

const message = useMessage()
const today = format(new Date(), 'yyyy-MM-dd')
const activeTab = ref('overview')

// ===== 页头：运行流水线 =====
const running = ref(false)
const lastRunText = ref('')
const runDateTs = ref<number | null>(Date.now())

function runDateStr(): string {
  return runDateTs.value ? format(new Date(runDateTs.value), 'yyyy-MM-dd') : today
}

function setRunDateFromISO(iso: string) {
  if (!iso || iso.length < 10) return
  runDateTs.value = new Date(iso.slice(0, 10) + 'T12:00:00').getTime()
}

function selectRunDate(iso: string) {
  setRunDateFromISO(iso)
  loadScores()
}

async function run() {
  if (running.value) return
  running.value = true
  lastRunText.value = '流水线运行中...'
  const reqDate = runDateStr()
  try {
    await runPipeline(reqDate)
  } catch (e) {
    running.value = false
    lastRunText.value = ''
    message.error('启动失败: ' + e)
  }
}

// ===== 运行概览 =====
const runSummaries = ref<models.KhunterRunSummary[]>([])
const runSummariesLoading = ref(false)
const latestAvailableDate = ref('')

const currentRunSummary = computed(() =>
  runSummaries.value.find(r => r.tradeDate === runDateStr()) || null
)

const runSummaryMetrics = computed(() => {
  const r = currentRunSummary.value
  return [
    { label: '运行日期', value: r?.tradeDate || runDateStr() },
    { label: '当日K线覆盖', value: r ? `${r.freshStocks}/${r.stockTotal}` : '-' },
    { label: '触发信号', value: r?.signalCount ?? '-' },
    { label: '候选评分', value: r?.scoredCount ?? '-' },
    { label: '进入狩猎池', value: r?.huntedCount ?? '-' },
    { label: '市场风险档', value: r?.riskLevel || '-' },
  ]
})

const overviewIssue = computed(() => {
  const r = currentRunSummary.value
  if (!r) {
    const hint = latestAvailableDate.value && latestAvailableDate.value !== runDateStr()
      ? `库内最近有数据日期为 ${latestAvailableDate.value}。`
      : ''
    return { type: 'info' as const, text: `${runDateStr()} 尚无运行记录；在页头运行当日策略后会保存覆盖与结果汇总。${hint}` }
  }
  if (r.status === 'failed') return { type: 'error' as const, text: `运行失败：${r.message || '未知错误'}` }
  if (r.freshStocks < 100 && r.tradeDate === today) {
    return { type: 'warning' as const, text: `今日日K尚未完整（${r.freshStocks}/${r.stockTotal}）。可等待收盘同步或选择最近完整交易日。` }
  }
  if (r.insufficientHistory > 0 || r.queryFailed > 0) {
    return { type: 'warning' as const, text: `${r.tradeDate} 历史不足 ${r.insufficientHistory} 只，查询失败 ${r.queryFailed} 只；请到「数据管理」检查对应任务。` }
  }
  if (r.signalCount === 0) return { type: 'info' as const, text: `${r.tradeDate} 无策略信号，因此没有候选评分。` }
  return null
})

const runSummaryColumns: any[] = [
  { title: '交易日', key: 'tradeDate', width: 100, fixed: 'left' },
  { title: '覆盖', key: 'freshStocks', width: 95, render: (r: any) => `${r.freshStocks}/${r.stockTotal}` },
  { title: '历史不足', key: 'insufficientHistory', width: 90, align: 'center' },
  { title: '当日缺K', key: 'missingTradeDate', width: 90, align: 'center' },
  { title: '查询失败', key: 'queryFailed', width: 90, align: 'center' },
  { title: '信号', key: 'signalCount', width: 70, align: 'center' },
  { title: '评分', key: 'scoredCount', width: 70, align: 'center' },
  { title: '入池', key: 'huntedCount', width: 70, align: 'center' },
  { title: '风险档', key: 'riskLevel', width: 90, align: 'center', render: (r: any) => r.riskLevel || '-' },
  {
    title: '状态', key: 'status', width: 85, align: 'center',
    render: (r: any) => h(NTag, { type: r.status === 'success' ? 'success' : 'error', bordered: false, size: 'small' }, { default: () => r.status === 'success' ? '成功' : '失败' }),
  },
  { title: '耗时', key: 'durationMs', width: 90, render: (r: any) => `${(r.durationMs / 1000).toFixed(1)}s` },
  { title: '说明', key: 'message', ellipsis: { tooltip: true } },
]

async function loadRunSummaries() {
  runSummariesLoading.value = true
  try {
    runSummaries.value = await getRunSummaries(30)
  } catch (e) {
    message.error('加载运行记录失败: ' + e)
  } finally {
    runSummariesLoading.value = false
  }
}

// 点击运行记录行切换到该交易日，避免用户手动在两个日期选择器间来回切换。
function runSummaryRowProps(row: models.KhunterRunSummary) {
  return {
    style: 'cursor: pointer;',
    onClick: () => selectRunDate(row.tradeDate),
  }
}

// ===== 候选评分 =====
const scores = ref<any[]>([])
const scoresLoading = ref(false)

const scoreIndicators = [
  { name: '技术', max: 100 }, { name: '资金', max: 100 }, { name: '基本面', max: 100 },
  { name: '板块', max: 100 }, { name: '事件', max: 100 },
]

async function loadScores() {
  scoresLoading.value = true
  try {
    await prepareStockDisplay()
    const d = runDateStr()
    scores.value = fillRowNames(await getScores(d), 'code', 'name')
    if (!scores.value.length) await loadRunSummaries()
  } catch (e) { message.error('加载评分失败: ' + e)
  } finally { scoresLoading.value = false }
}

const scoreEmptyReason = computed(() => {
  if (scores.value.length > 0) return ''
  const r = runSummaries.value.find(item => item.tradeDate === runDateStr())
  if (!r) {
    if (latestAvailableDate.value && latestAvailableDate.value !== runDateStr()) {
      return `${runDateStr()} 尚未运行；库内最近有数据日期为 ${latestAvailableDate.value}，可切换日期查看或重新运行。`
    }
    return `${runDateStr()} 尚未运行；可在页头运行当日策略。`
  }
  if (r.status === 'failed') return `${runDateStr()} 运行失败：${r.message || '未知错误'}`
  if (r.signalCount === 0) return `${runDateStr()} 无策略信号，因此没有候选评分。`
  if (r.scoredCount === 0) return `${runDateStr()} 有 ${r.signalCount} 条信号但未生成评分。`
  return `${runDateStr()} 暂无候选评分。`
})

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

function parseScoreDetails(row: any): any {
  if (!row || !row.details || typeof row.details !== 'string') return null
  try {
    return JSON.parse(row.details)
  } catch {
    return null
  }
}

function renderScoreExpand(row: any) {
  const d = parseScoreDetails(row)
  const dimNames: Record<string, string> = { technical: '技术', moneyflow: '资金', fundamental: '基本面', sector: '板块', event: '事件' }

  // 技术面解释：命中策略+理由+技术画像
  const techChildren: any[] = []
  if (d?.technical?.strategies?.length) {
    for (const s of d.technical.strategies) {
      techChildren.push(h('div', { style: { marginBottom: '2px' } }, [
        h(NText, { strong: true, style: { fontSize: '12px' } },
          { default: () => `${s.name}（权重 ${s.weight}）` }),
      ]))
      if (Array.isArray(s.reasons)) {
        for (const r of s.reasons) {
          techChildren.push(h('div', { style: { paddingLeft: '14px', fontSize: '12px' } }, [
            h(NText, { depth: 2 }, { default: () => `· ${r}` }),
          ]))
        }
      }
    }
  } else if (d?.technical) {
    techChildren.push(h(NText, { depth: 3, style: { fontSize: '12px' } }, { default: () => '无策略命中' }))
  }
  if (d?.technical?.profile) {
    const p = d.technical.profile
    const sections = Array.isArray(p.sections) ? p.sections : []
    if (sections.length) {
      for (const s of sections) {
        const title = s.title || s.Title || ''
        const text = s.text || s.Text || ''
        if (!text) continue
        techChildren.push(h('div', { style: { marginTop: '6px' } }, [
          h('div', [h(NText, { strong: true, style: { fontSize: '12px' } }, { default: () => title })]),
          h('div', { style: { fontSize: '12px', lineHeight: '1.55' } }, [h(NText, { depth: 2 }, { default: () => text })]),
        ]))
      }
    } else {
      techChildren.push(h('div', { style: { marginTop: '6px', fontSize: '12px' } }, [
        h(NText, { depth: 3 }, { default: () => `技术画像：${p.trend ?? ''}；${p.volume ?? ''}；${p.position ?? ''}；${p.momentum ?? ''}` }),
      ]))
    }
  }

  // 其他维度：缺数据标注 / 原因
  const dimRows: any[] = []
  for (const key of ['moneyflow', 'fundamental', 'sector', 'event']) {
    const dim = d?.[key]
    if (!dim) continue
    const label = dimNames[key]
    if (dim.missing) {
      dimRows.push(h('div', { key, style: { fontSize: '12px' } }, [
        h(NTag, { type: 'warning', size: 'tiny', bordered: false, style: { marginRight: '4px' } }, { default: () => '缺数据' }),
        h(NText, { depth: 2 }, { default: () => `${label}：${dim.reason ?? '数据缺失，按中性 50 分计'}` }),
      ]))
    } else if (dim.reason) {
      dimRows.push(h('div', { key, style: { fontSize: '12px', lineHeight: '1.55', marginTop: '4px' } }, [
        h(NText, { strong: true }, { default: () => `${label}：` }),
        h(NText, { depth: 2 }, { default: () => dim.reason }),
      ]))
    }
  }

  return h('div', { style: { display: 'flex', gap: '24px', padding: '4px 12px' } }, [
    h('div', { style: { width: '320px', flexShrink: 0 } }, [
      h(RadarChart, {
        indicators: scoreIndicators,
        data: [row.technical ?? 0, row.moneyflow ?? 0, row.fundamental ?? 0, row.sector ?? 0, row.event ?? 0],
        height: 220,
      }),
    ]),
    h('div', { style: { flex: 1, minWidth: 0 } }, [
      h('div', { style: { marginBottom: '6px' } }, [
        row.vetoReason
          ? h(NText, { type: 'error' }, { default: () => `否决原因：${row.vetoReason}` })
          : h(NText, { depth: 3 }, { default: () => '无否决项' }),
      ]),
      h('div', { style: { marginBottom: '4px' } }, [
        h(NText, { strong: true }, { default: () => '技术面命中：' }),
      ]),
      ...techChildren,
      ...(dimRows.length ? [h('div', { style: { marginTop: '8px', display: 'flex', flexDirection: 'column', gap: '2px' } }, dimRows)] : []),
    ]),
  ])
}

const scoreColumns: any[] = [
  { type: 'expand', renderExpand: renderScoreExpand },
  { title: '代码', key: 'code', width: 100, render: (r: any) => displayStockCode(r.code) },
  { title: '名称', key: 'name', width: 110, render: (r: any) => resolveDisplayName(r.code, r.name) || '-' },
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

// ===== 狩猎池 =====
const hunting = ref<any[]>([])
const huntingLoading = ref(false)
const huntingStatus = ref('追踪中')
const huntingStatusOptions = [
  { label: '追踪中', value: '追踪中' },
  { label: '已移除', value: '已移除' },
  { label: '全部', value: '' },
]

async function loadHunting() {
  huntingLoading.value = true
  try {
    await prepareStockDisplay()
    hunting.value = fillRowNames(await getHunting(huntingStatus.value), 'code', 'name')
  } catch (e) { message.error('加载狩猎池失败: ' + e)
  } finally { huntingLoading.value = false }
}

const huntingColumns: any[] = [
  { title: '代码', key: 'code', width: 100, render: (r: any) => displayStockCode(r.code) },
  { title: '名称', key: 'name', width: 110, render: (r: any) => resolveDisplayName(r.code, r.name) || '-' },
  { title: '入选日', key: 'enterDate', width: 105 },
  { title: '入选分', key: 'enterScore', width: 90, align: 'center', render: (r: any) => (r.enterScore ?? 0).toFixed(1) },
  { title: '支撑价', key: 'supportPrice', width: 90, align: 'right', render: (r: any) => (r.supportPrice ?? 0).toFixed(2) },
  { title: '状态', key: 'status', width: 90, align: 'center',
    render: (r: any) => h(NTag, { type: r.status === '追踪中' ? 'success' : 'default', bordered: false, size: 'small' }, { default: () => r.status || '-' }) },
  { title: '跟踪天数', key: 'trackDays', width: 90, align: 'center' },
]

// ===== 回测结果 =====
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
  { title: '最大回撤', key: 'MaxDrawdown', width: 110, align: 'center',
    render: (r: any) => (((r.MaxDrawdown ?? r.maxDrawdown ?? 0) * 100).toFixed(1) + '%') },
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
  const td = r.TradeDate ?? r.tradeDate
  const fresh = r.FreshStocks ?? r.freshStocks ?? 0
  const total = r.StockTotal ?? r.stockTotal ?? 0
  const insufficient = r.InsufficientHistory ?? r.insufficientHistory ?? 0
  const missingDate = r.MissingTradeDate ?? r.missingTradeDate ?? 0
  const queryFailed = r.QueryFailed ?? r.queryFailed ?? 0
  const sigN = r.Signals ?? 0
  lastRunText.value = `最近运行(${td || '-'}): 当日K线 ${fresh}/${total} · 信号 ${sigN} · 入池 ${r.Hunted ?? 0}`
  if (td) setRunDateFromISO(td)
  if (fresh < 100) {
    if (td === today) {
      message.warning(`今日日K尚未完整：当前覆盖 ${fresh}/${total} 只。可等待收盘同步或选择最近完整交易日`)
    } else if (insufficient > 0 || queryFailed > 0) {
      message.warning(`${td} 数据检查：当日K线 ${fresh}/${total}，历史不足 ${insufficient} 只，查询失败 ${queryFailed} 只；请到「数据管理」检查对应任务`)
    } else {
      message.warning(`${td} 当日K线仅覆盖 ${fresh}/${total} 只，${missingDate} 只当日无K线（停牌或尚未同步）；已保留所选日期`)
    }
  }
  if (sigN === 0) {
    message.warning(`${td || '当日'} 策略扫描完成但信号为 0；运行记录会保留本次覆盖和原因`)
  } else {
    message.success('狩猎场流水线运行完成')
  }
  loadRunSummaries(); loadScores(); loadHunting(); loadRisk()
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

onMounted(async () => {
  try {
    const meta = await getKhunterMeta()
    latestAvailableDate.value = meta.latestScoreDate || meta.latestSignalDate || ''
    const preferred = meta.latestScoreDate || meta.latestSignalDate || meta.latestRunDate
    if (preferred) setRunDateFromISO(preferred)
  } catch { /* ignore */ }
  loadRunSummaries(); loadScores(); loadHunting(); loadRisk(); loadKelly()
})
</script>

<style scoped>
.n-card { --n-padding: 12px; }
</style>
