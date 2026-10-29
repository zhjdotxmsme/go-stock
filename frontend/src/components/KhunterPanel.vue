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
import { runPipeline, getKhunterMeta, getScores, getSignals, getHunting, getRiskLevel, getKellySuggestions, runBacktest } from '../api/khunter'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { displayStockCode, resolveDisplayName, prepareStockDisplay, fillRowNames } from '../utils/stockDisplay'

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
  const reqDate = scoreDateStr()
  try {
    await runPipeline(reqDate)
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

function setScoreDateFromISO(iso: string) {
  if (!iso || iso.length < 10) return
  scoreDateTs.value = new Date(iso.slice(0, 10) + 'T12:00:00').getTime()
}

async function loadScores() {
  scoresLoading.value = true
  try {
    await prepareStockDisplay()
    const d = scoreDateStr()
    scores.value = fillRowNames(await getScores(d), 'code', 'name')
    if (!scores.value.length) {
      const meta = await getKhunterMeta().catch(() => ({}))
      const hint = meta.latestScoreDate && meta.latestScoreDate !== d
        ? `；库内最新评分日为 ${meta.latestScoreDate}，可切换日期查看`
        : ''
      message.info(`${d} 暂无评分（当日可能 0 条策略信号）${hint}`)
    }
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
    techChildren.push(h('div', { style: { marginTop: '6px', fontSize: '12px' } }, [
      h(NText, { depth: 3 }, { default: () => `技术画像：${p.trend ?? ''}；${p.volume ?? ''}；${p.position ?? ''}；${p.momentum ?? ''}` }),
    ]))
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
      dimRows.push(h('div', { key, style: { fontSize: '12px' } }, [
        h(NText, { depth: 2 }, { default: () => `${label}：${dim.reason}` }),
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
    await prepareStockDisplay()
    const d = signalDateStr()
    signals.value = fillRowNames(await getSignals(d), 'code', 'name')
    if (!signals.value.length) {
      message.info(`${d} 暂无策略信号，可先「立即运行」或换一个有数据的交易日`)
    }
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
  { title: '代码', key: 'code', width: 100, render: (r: any) => displayStockCode(r.code) },
  { title: '名称', key: 'name', width: 110, render: (r: any) => resolveDisplayName(r.code, r.name) || '-' },
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
  const td = r.TradeDate ?? r.tradeDate
  const fresh = r.FreshStocks ?? r.freshStocks ?? 0
  const total = r.StockTotal ?? r.stockTotal ?? 0
  const sigN = r.Signals ?? 0
  lastRunText.value = `最近运行(${td || '-'}): 新鲜K线 ${fresh}/${total} · 信号 ${sigN} · 入池 ${r.Hunted ?? 0}`
  if (fresh < 100) {
    message.warning('本地日K覆盖不足，请到「数据管理」启动历史同步后再跑狩猎场')
  }
  if (sigN === 0) {
    message.warning(`${td || '当日'} 策略扫描完成但信号为 0，评分表不会生成；可切换到有历史信号的交易日查看`)
  } else {
    message.success('狩猎场流水线运行完成')
  }
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

onMounted(async () => {
  try {
    const meta = await getKhunterMeta()
    if (meta.latestScoreDate) setScoreDateFromISO(meta.latestScoreDate)
    else if (meta.latestSignalDate) setScoreDateFromISO(meta.latestSignalDate)
  } catch { /* ignore */ }
  loadScores(); loadSignals(); loadRisk(); loadKelly()
})
</script>

<style scoped>
.n-card { --n-padding: 12px; }
</style>
