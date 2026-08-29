<template>
  <n-space vertical>
    <n-page-header>
      <template #title>
        <n-text strong>每日选股</n-text>
      </template>
      <template #extra>
        <n-space>
          <n-date-picker v-model:value="queryDate" type="date" :is-date-disabled="dateDisabled" clearable placeholder="筛选日期" style="width:160px" @update:value="onDateChange" />
          <n-button type="primary" :loading="running" @click="runPick">
            <template #icon><n-icon><DownloadOutline /></n-icon></template>运行选股
          </n-button>
          <n-text v-if="running && progressText" depth="3" style="font-size:12px">{{ progressText }}</n-text>
          <n-button :loading="reviewing" @click="runReview">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>复盘
          </n-button>
        </n-space>
      </template>
    </n-page-header>
    <n-card size="small" title="胜率趋势" v-if="winRateData.length > 0">
      <template #header-extra>
        <n-radio-group v-model:value="trendDays" size="small" @update:value="loadWinRate">
          <n-radio-button :value="7">7天</n-radio-button>
          <n-radio-button :value="30">30天</n-radio-button>
          <n-radio-button :value="90">90天</n-radio-button>
          <n-radio-button :value="0">全部</n-radio-button>
        </n-radio-group>
      </template>
      <TrendLine :data="winRateData" :dark="false" :loading="winRateLoading" height="180" area-style :smooth="true" y-unit="%" />
    </n-card>
    <n-grid :cols="7" :x-gap="12">
      <n-gi><n-card size="small" :bordered="true"><n-statistic label="总推荐" :value="stats.totalPicks" /></n-card></n-gi>
      <n-gi><n-card size="small"><n-statistic label="已复盘" :value="stats.reviewedPicks" /></n-card></n-gi>
      <n-gi><n-card size="small"><n-statistic label="胜利" :value="stats.winCount" /></n-card></n-gi>
      <n-gi><n-card size="small"><n-statistic label="亏损" :value="stats.lossCount" /></n-card></n-gi>
      <n-gi><n-card size="small"><n-statistic label="胜率" :value="stats.winRate + '%'" /></n-card></n-gi>
      <n-gi><n-card size="small"><n-statistic label="平均收益" :value="stats.avgReturn + '%'" /></n-card></n-gi>
      <n-gi><n-card size="small"><n-statistic label="最大收益" :value="stats.maxReturn + '%'" /></n-card></n-gi>
    </n-grid>
    <n-grid :cols="3" :x-gap="12" v-if="picks.length > 0">
      <n-gi>
        <n-card size="small" title="收益分布">
          <DistributionHist :data="returnValues" :height="200" :buckets="10" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small" title="评分 vs 收益">
          <BarCompare :categories="scoreCategories" :series="scoreSeries" :height="200" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small" title="策略表现">
          <BarCompare :categories="strategyCategories" :series="strategySeries" :height="200" />
        </n-card>
      </n-gi>
    </n-grid>
    <n-space justify="space-between" align="center">
      <n-text depth="3" style="font-size:13px">共 {{ pagination.total }} 条记录</n-text>
      <n-space>
        <n-switch v-model:value="showExtraColumns" size="small" />
        <n-text depth="3" style="font-size:12px">显示更多指标</n-text>
      </n-space>
    </n-space>
    <n-alert type="info" :bordered="false" style="margin-bottom: 8px" collapsible>
      <b>功能说明：</b>
      「运行选股」按当日收盘数据跑完整管线（K线初筛 → 研报抓取 → 综合打分），结果按评分排名落库；
      「复盘」用次日行情回填每只入选股的实际开收高低与收益（次日收益 = 次日开盘买入→收盘卖出）；
      「胜率趋势」展示历史复盘的每日胜率走势。
      <b>短期买卖建议</b>规则：以信号日收盘价为参考——参考买入价 = 收盘价；目标一 = +5%、目标二 = +10%；
      止损 = -3%。建议仅为按固定比例生成的参考，请结合大盘与个股基本面自行决策，不构成投资建议。
    </n-alert>
    <n-data-table remote :columns="visibleColumns" :data="picks" :loading="loading" :bordered="true" :single-line="false" :max-height="520" :pagination="tablePagination" striped @update:sorter="onSorterChange" />
    <n-drawer v-model:show="showDetail" placement="bottom" :height="'55vh'">
      <n-drawer-content :title="detailTitle" closable>
        <n-grid :cols="3" :x-gap="16" v-if="selectedPick">
          <n-gi>
            <n-card size="small" title="因子得分">
              <RadarChart :indicators="factorIndicators" :data="factorValues(selectedPick)" :height="260" />
            </n-card>
          </n-gi>
          <n-gi>
            <n-card size="small" title="K线走势">
              <StockLightweightKlineChart style="width:100%" :code="detailStockCode" :chart-height="260" :stock-name="selectedPick.stockName" :dark-theme="false" />
            </n-card>
          </n-gi>
          <n-gi>
            <n-card size="small" title="技术指标">
              <n-descriptions label-placement="left" :column="1">
                <n-descriptions-item label="RSI(14)">{{ selectedPick.rsi14?.toFixed(1) }}</n-descriptions-item>
                <n-descriptions-item label="解读">{{ rsiInterpret(selectedPick.rsi14) }}</n-descriptions-item>
                <n-descriptions-item label="MACD">{{ selectedPick.macd?.toFixed(3) }}</n-descriptions-item>
                <n-descriptions-item label="解读">{{ macdInterpret(selectedPick.macd) }}</n-descriptions-item>
                <n-descriptions-item label="均线形态">{{ maInterpret(selectedPick) }}</n-descriptions-item>
                <n-descriptions-item label="KDJ">{{ selectedPick.kdjK?.toFixed(1) }} / {{ selectedPick.kdjD?.toFixed(1) }} / {{ selectedPick.kdjJ?.toFixed(1) }}</n-descriptions-item>
              </n-descriptions>
            </n-card>
            <n-card size="small" title="选股理由" style="margin-top:8px">
              <n-text>{{ selectedPick.reason || '无' }}</n-text>
            </n-card>
          </n-gi>
        </n-grid>
      </n-drawer-content>
    </n-drawer>
  </n-space>
</template>

<script setup lang="ts">
import { h, onMounted, onUnmounted, ref, reactive, computed } from 'vue'
import { NInput, NTag, NText, useMessage, useDialog } from 'naive-ui'
import { DownloadOutline, RefreshOutline } from '@vicons/ionicons5'
import { format } from 'date-fns'

import TrendLine from './charts/TrendLine.vue'
import DistributionHist from './charts/DistributionHist.vue'
import BarCompare from './charts/BarCompare.vue'
import RadarChart from './charts/RadarChart.vue'
import FactorBar from './charts/FactorBar.vue'

import {
  runDailyPickAsync, getDailyPicks, getDailyPickStats,
  updateDailyPickRemarks, runDailyReview, getReviewTrend,
} from '../api/dailyPick'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'

const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const running = ref(false)
const reviewing = ref(false)
const picks = ref<any[]>([])
const stats = ref({
  totalPicks: 0, reviewedPicks: 0, winCount: 0, lossCount: 0,
  winRate: 0, avgReturn: 0, totalReturn: 0, maxReturn: 0,
  maxDrawdown: 0, avgMaxReturn: 0, avgMaxDrawdown: 0,
})

const queryDate = ref<number | null>(null)
const query = reactive({ page: 1, pageSize: 20, tradeDate: '', reviewed: null })
const pagination = reactive({ page: 1, pageSize: 20, total: 0, pageCount: 0 })
const progressText = ref('')

// Remote pagination: the table renders one server page at a time.
const tablePagination = computed(() => ({
  page: pagination.page,
  pageSize: pagination.pageSize,
  itemCount: pagination.total,
  pageCount: pagination.pageCount,
  onUpdatePage: (p: number) => { pagination.page = p; loadPicks() },
}))

// With remote pagination, column sorters only re-order the current page.
function onSorterChange(sorter: any) {
  if (!sorter || sorter.order === false) { loadPicks(); return }
  if (typeof sorter.sorter === 'function') {
    const dir = sorter.order === 'ascend' ? 1 : -1
    picks.value = [...picks.value].sort((a, b) => sorter.sorter(a, b) * dir)
  }
}

const winRateData = ref<Array<{ date: string; value: number }>>([])
const winRateLoading = ref(false)
const trendDays = ref(30)

const showExtraColumns = ref(false)

const showDetail = ref(false)
const selectedPick = ref<any>(null)
const detailStockCode = computed(() => selectedPick.value?.stockCode || '')
const detailTitle = computed(() => selectedPick.value ? `${selectedPick.value.stockName} (${selectedPick.value.stockCode})` : '')

const factorIndicators = [
  { name: '量比', max: 1 }, { name: '均线', max: 1 }, { name: 'RSI', max: 1 },
  { name: 'MACD', max: 1 }, { name: '价格', max: 1 }, { name: '换手', max: 1 },
]

function factorValues(pick: any): number[] {
  return [
    pick.volumeFactor ?? 0, pick.maFactor ?? 0, pick.rsiFactor ?? 0,
    pick.macdFactor ?? 0, pick.priceFactor ?? 0, pick.turnoverFactor ?? 0,
  ]
}

function scoreTag(score: number) {
  const t = score >= 80 ? 'success' as const : score >= 60 ? 'warning' as const : 'info' as const
  return h(NTag, { type: t, size: 'small', style: { fontWeight: 'bold' } }, { default: () => score.toFixed(1) })
}

function renderScore(row: any) {
  return h('span', { style: { display: 'inline-flex', alignItems: 'center', gap: '8px' } }, [
    scoreTag(row.score),
    h(FactorBar, {
      factors: [
        { name: '量', value: row.volumeFactor ?? 0 },
        { name: '线', value: row.maFactor ?? 0 },
        { name: 'R', value: row.rsiFactor ?? 0 },
        { name: 'M', value: row.macdFactor ?? 0 },
        { name: '价', value: row.priceFactor ?? 0 },
        { name: '换', value: row.turnoverFactor ?? 0 },
      ],
      width: 80, height: 14, showLabel: true, showValue: false,
    }),
  ])
}

function renderReturn(row: any) {
  if (!row.reviewed) return h('span', '-')
  const v = row.nextReturn
  const c = v >= 3 ? '#18a058' : v >= 1 ? '#63e2b7' : v <= -3 ? '#d03050' : v <= -1 ? '#e88080' : '#909399'
  return h(NText, { style: { color: c, fontWeight: 'bold', fontSize: '13px' } }, { default: () => `${v >= 0 ? '+' : ''}${v.toFixed(2)}%` })
}

function renderChange(row: any) {
  if (row.changePercent == null) return h('span', '-')
  const c = row.changePercent > 0 ? '#18a058' : row.changePercent < 0 ? '#d03050' : '#909399'
  return h(NText, { style: { color: c, fontWeight: 'bold' } }, { default: () => `${row.changePercent >= 0 ? '+' : ''}${row.changePercent.toFixed(2)}%` })
}

function renderPnL(row: any) {
  if (!row.reviewed) return h('span', '-')
  return h('span', { style: { fontSize: '12px', whiteSpace: 'nowrap' } }, [
    h('span', { style: { color: (row.nextMaxReturn ?? 0) >= 0 ? '#18a058' : '#909399' } }, `+${(row.nextMaxReturn ?? 0).toFixed(1)}%`),
    h('span', { style: { color: '#909399', margin: '0 2px' } }, ' / '),
    h('span', { style: { color: '#d03050' } }, `${(row.nextMaxDrawdown ?? 0).toFixed(1)}%`),
  ])
}

function renderReason(row: any) {
  if (!row.reason) return h('span', { style: { color: '#909399', fontSize: '12px' } }, '-')
  return h('span', { style: { fontSize: '12px', color: '#606266', cursor: 'pointer', maxWidth: '200px', display: 'inline-block' }, title: row.reason },
    row.reason.length > 30 ? row.reason.slice(0, 30) + '...' : row.reason)
}

function renderActions(row: any) {
  return h('a', { href: '#', style: { color: row.remarks ? '#18a058' : '#909399', fontSize: '12px' },
    onClick: (e: Event) => { e.preventDefault(); editRemarks(row) } }, row.remarks || '添加备注')
}

function renderStockName(row: any) {
  return h('a', { href: '#', style: { color: '#2080f0', fontWeight: 500, cursor: 'pointer' },
    onClick: (e: Event) => { e.preventDefault(); showPickDetail(row) } }, `${row.stockName} (${row.stockCode})`)
}

// 短期买卖建议（仅供参考，非投资建议）：以信号日收盘价为基准，
// 目标一 +5%、目标二 +10%、止损 -3%，可按个人风险偏好调整。
function tradeAdvice(r: any) {
  if (!r?.closePrice || r.closePrice <= 0) return null
  const p = r.closePrice
  const f = (v: number) => v.toFixed(2)
  return {
    buy: f(p),
    target1: f(p * 1.05),
    target2: f(p * 1.1),
    stop: f(p * 0.97),
  }
}

function renderAdvice(r: any) {
  const a = tradeAdvice(r)
  if (!a) return '-'
  return `${a.buy} → 目标 ${a.target1}/${a.target2}，止损 ${a.stop}`
}

const baseColumns: any[] = [
  { title: '排名', key: 'rank', width: 60, align: 'center' },
  { title: '日期', key: 'tradeDate', width: 100 },
  { title: '股票', key: 'stockCode', width: 130, render: renderStockName },
  { title: '综合评分', key: 'score', width: 180, align: 'center', render: renderScore, sorter: (a: any, b: any) => a.score - b.score },
  { title: '涨跌幅', key: 'changePercent', width: 85, align: 'right', render: renderChange },
  { title: '次日收益', key: 'nextReturn', width: 95, align: 'center', render: renderReturn, sorter: (a: any, b: any) => (a.nextReturn || 0) - (b.nextReturn || 0) },
  { title: '潜在盈亏', key: 'nextMaxReturn', width: 110, align: 'center', render: renderPnL },
  { title: '短期买卖建议', key: 'advice', width: 240, render: renderAdvice },
  { title: '选股理由', key: 'reason', ellipsis: { tooltip: true }, render: renderReason },
  { title: '备注', key: 'remarks', width: 80, align: 'center', render: renderActions },
]

const extraColumns: any[] = [
  { title: '收盘价', key: 'closePrice', width: 80, align: 'right', render: (r: any) => r.closePrice ? r.closePrice.toFixed(2) : '-' },
  { title: 'MA5/10/20', key: 'ma5', width: 130, render: (r: any) => r.ma5 ? `${r.ma5.toFixed(2)} / ${r.ma10.toFixed(2)} / ${r.ma20.toFixed(2)}` : '-' },
  { title: 'RSI', key: 'rsi14', width: 60, align: 'center', render: (r: any) => r.rsi14 ? r.rsi14.toFixed(1) : '-' },
  { title: 'MACD', key: 'macd', width: 80, align: 'right', render: (r: any) => r.macd ? r.macd.toFixed(3) : '-' },
  { title: '量比', key: 'volumeFactor', width: 70, align: 'center', render: (r: any) => r.volumeFactor ? r.volumeFactor.toFixed(2) : '-' },
  { title: '均线因子', key: 'maFactor', width: 70, align: 'center', render: (r: any) => r.maFactor ? r.maFactor.toFixed(2) : '-' },
  { title: '最大收益', key: 'nextMaxReturn', width: 80, align: 'right', render: (r: any) => r.reviewed ? ((r.nextMaxReturn ?? 0) >= 0 ? '+' : '') + (r.nextMaxReturn ?? 0).toFixed(2) + '%' : '-' },
  { title: '最大回撤', key: 'nextMaxDrawdown', width: 80, align: 'right', render: (r: any) => r.reviewed ? (r.nextMaxDrawdown ?? 0).toFixed(2) + '%' : '-' },
]

const visibleColumns = computed(() => showExtraColumns.value ? [...baseColumns, ...extraColumns] : baseColumns)

// L1 chart data computeds
const returnValues = computed(() =>
  picks.value.filter((p: any) => p.reviewed && p.nextReturn != null).map((p: any) => p.nextReturn)
)

const scoreCategories = ['0-20', '20-40', '40-60', '60-80', '80-100']
const scoreSeries = computed(() => {
  const buckets: number[][] = [[], [], [], [], []]
  picks.value.filter((p: any) => p.reviewed && p.nextReturn != null).forEach((p: any) => {
    const idx = p.score >= 80 ? 4 : p.score >= 60 ? 3 : p.score >= 40 ? 2 : p.score >= 20 ? 1 : 0
    buckets[idx].push(p.nextReturn)
  })
  return [{
    name: '平均收益',
    data: buckets.map(b => b.length ? b.reduce((s, v) => s + v, 0) / b.length : 0),
  }]
})

const strategyCategories = computed(() => {
  const names = new Set(picks.value.map((p: any) => p.strategyName || '默认'))
  return Array.from(names) as string[]
})
const strategySeries = computed(() => {
  const groups: Record<string, { wins: number; total: number }> = {}
  picks.value.filter((p: any) => p.reviewed).forEach((p: any) => {
    const key = p.strategyName || '默认'
    if (!groups[key]) groups[key] = { wins: 0, total: 0 }
    groups[key].total++
    if ((p.nextReturn ?? 0) > 0) groups[key].wins++
  })
  return [{
    name: '胜率',
    data: Object.values(groups).map(g => g.total ? Number((g.wins / g.total * 100).toFixed(1)) : 0),
  }]
})

async function loadWinRate() {
  const limit = trendDays.value || 0
  winRateLoading.value = true
  try {
    const data = await getReviewTrend(limit)
    winRateData.value = (data || []).map((d: any) => ({
      date: d.date || d.Date,
      value: d.winRate ?? d.WinRate ?? 0,
    }))
  } catch (e) { console.error('loadWinRate failed', e); message.error('加载胜率趋势失败')
  } finally { winRateLoading.value = false }
}

function showPickDetail(row: any) {
  selectedPick.value = row; showDetail.value = true
}

function rsiInterpret(val: number | null | undefined): string {
  if (val == null) return '-'
  if (val >= 70) return '超买'
  if (val >= 50) return '偏强'
  if (val >= 30) return '偏弱'
  return '超卖'
}

function macdInterpret(val: number | null | undefined): string {
  if (val == null) return '-'
  return val > 0 ? '多头' : val < 0 ? '空头' : '中性'
}

function maInterpret(pick: any): string {
  const { closePrice, ma5, ma10, ma20 } = pick
  if (!closePrice || !ma5) return '-'
  if (closePrice >= ma5 && ma5 >= ma10 && ma10 >= ma20) return '多头排列 ↑'
  if (closePrice <= ma5 && ma5 <= ma10 && ma10 <= ma20) return '空头排列 ↓'
  return '均线缠绕'
}

async function loadPicks() {
  loading.value = true
  try {
    const q = { page: pagination.page, pageSize: pagination.pageSize, tradeDate: query.tradeDate, reviewed: query.reviewed }
    const res = await getDailyPicks(q)
    if (res) { picks.value = res.list || []; pagination.total = res.total || 0; pagination.pageCount = res.totalPages || 0 }
  } catch (e) { message.error('加载失败: ' + e)
  } finally { loading.value = false }
}

async function loadStats() {
  try { const s = await getDailyPickStats(); if (s) stats.value = s }
  catch (e) { console.error('loadStats failed', e); message.error('加载统计信息失败') }
}

// Async daily-pick progress events pushed by the backend
// (see DailyPickService.RunDailyPickAsync).
EventsOn('dailyPickProgress', (msg: any) => {
  if (!msg || typeof msg !== 'object') return
  switch (msg.stage) {
    case 'busy':
      message.warning(msg.message || '选股任务正在运行中')
      break
    case 'baseline':
      progressText.value = `K线初筛 ${msg.done}/${msg.total}`
      break
    case 'research':
      progressText.value = `抓取研报 ${msg.done}/${msg.total}`
      break
    case 'final':
      progressText.value = `综合打分 ${msg.done}/${msg.total}`
      break
    case 'done':
      running.value = false
      progressText.value = ''
      message.success(`选股完成，入选 ${msg.count ?? 0} 只`)
      loadPicks(); loadStats(); loadWinRate()
      break
    case 'error':
      running.value = false
      progressText.value = ''
      message.error('选股失败: ' + (msg.message || '未知错误'))
      break
  }
})

onUnmounted(() => {
  EventsOff('dailyPickProgress')
})

async function runPick() {
  if (running.value) return
  running.value = true
  progressText.value = '正在启动选股任务...'
  try {
    const date = queryDate.value ? format(new Date(queryDate.value), 'yyyy-MM-dd') : format(new Date(), 'yyyy-MM-dd')
    // Fire-and-forget: progress and completion arrive via dailyPickProgress events.
    await runDailyPickAsync(date, 5)
  } catch (e) {
    running.value = false
    progressText.value = ''
    message.error('选股启动失败: ' + e)
  }
}

async function runReview() {
  reviewing.value = true
  try {
    await runDailyReview(format(new Date(), 'yyyy-MM-dd'), '')
    message.success('复盘完成')
    await loadPicks(); await loadStats(); await loadWinRate()
  } catch (e) { message.error('复盘失败: ' + e)
  } finally { reviewing.value = false }
}

function onDateChange(ts: number | null) {
  query.tradeDate = ts ? format(new Date(ts), 'yyyy-MM-dd') : ''
  query.page = 1; pagination.page = 1; loadPicks()
}

function editRemarks(row: any) {
  const remarksRef = ref(row.remarks || '')
  const d = dialog.create({
    title: '编辑备注',
    content: () => h(NInput, { value: remarksRef.value, type: 'textarea', placeholder: '输入备注内容...', autosize: { minRows: 3, maxRows: 8 }, onUpdateValue: (val: string) => { remarksRef.value = val } }),
    positiveText: '保存', negativeText: '取消',
    onPositiveHandle: async () => {
      try { await updateDailyPickRemarks(row.id, remarksRef.value); message.success('保存成功'); d.destroy() }
      catch (e) { message.error('保存失败: ' + e) }
    },
  })
}

function dateDisabled() { return false }

onMounted(async () => {
  await loadStats(); await loadPicks(); await loadWinRate()
})
</script>

<style scoped>
.n-card { --n-padding: 12px; }
</style>
