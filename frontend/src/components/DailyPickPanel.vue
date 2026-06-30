<template>
  <n-space vertical>
    <!-- 页头 -->
    <n-page-header>
      <template #title>
        <n-text strong>每日选股</n-text>
      </template>
      <template #extra>
        <n-space>
          <n-date-picker
            v-model:value="queryDate"
            type="date"
            :is-date-disabled="dateDisabled"
            clearable
            placeholder="筛选日期"
            style="width:160px"
            @update:value="onDateChange"
          />
          <n-button type="primary" :loading="running" @click="runPick">
            <template #icon><n-icon><DownloadOutline /></n-icon></template>
            运行选股
          </n-button>
          <n-button :loading="reviewing" @click="runReview">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>
            复盘
          </n-button>
        </n-space>
      </template>
    </n-page-header>

    <!-- 统计卡片 -->
    <n-grid :cols="7" :x-gap="12">
      <n-gi>
        <n-card size="small" :bordered="true">
          <n-statistic label="总推荐" :value="stats.totalPicks" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="已复盘" :value="stats.reviewedPicks" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="胜利" :value="stats.winCount" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="亏损" :value="stats.lossCount" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="胜率" :value="stats.winRate + '%'" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="平均收益" :value="stats.avgReturn + '%'" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="最大收益" :value="stats.maxReturn + '%'" />
        </n-card>
      </n-gi>
    </n-grid>

    <!-- 选股列表 -->
    <n-data-table
      :columns="columns"
      :data="picks"
      :loading="loading"
      :bordered="true"
      :single-line="false"
      :max-height="580"
      striped
    />
  </n-space>
</template>

<script setup>
import { h, onMounted, ref, reactive } from 'vue'
import { NInput, useMessage, useDialog } from 'naive-ui'
import {
  DownloadOutline,
  RefreshOutline,
  TrendingUpOutline,
  TrendingDownOutline
} from '@vicons/ionicons5'
import { format, parseISO } from 'date-fns'

// ---------- Wails bindings ----------
import {
  RunDailyPick,
  GetDailyPicks,
  GetDailyPickStats,
  DeleteDailyPick,
  UpdateDailyPickRemarks,
  RunDailyReview,
  GetLatestUnreviewedPicks,
  GetReviewTrend,
  GetDateRange
} from '../../wailsjs/go/data/DailyPickService'

const message = useMessage()
const dialog = useDialog()

// ---------- State ----------
const loading = ref(false)
const running = ref(false)
const reviewing = ref(false)
const picks = ref([])
const stats = ref({
  totalPicks: 0,
  reviewedPicks: 0,
  winCount: 0,
  lossCount: 0,
  winRate: 0,
  avgReturn: 0,
  totalReturn: 0,
  maxReturn: 0,
  maxDrawdown: 0,
  avgMaxReturn: 0,
  avgMaxDrawdown: 0,
})

const queryDate = ref(null) // timestamp in ms
const query = reactive({
  page: 1,
  pageSize: 20,
  tradeDate: '',
  reviewed: null,
})
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  pageCount: 0,
})

// ---------- Columns ----------
function renderScore(row) {
  const color = row.score >= 70 ? '#18a058' : row.score >= 50 ? '#d48806' : '#909399'
  return h('span', { style: { color, fontWeight: 'bold' } }, row.score.toFixed(1))
}

function renderReturn(row) {
  if (!row.reviewed) return h('span', '-')
  const val = row.nextReturn
  const color = val > 0 ? '#18a058' : val < 0 ? '#d03050' : '#909399'
  const icon = val >= 0 ? TrendingUpOutline : TrendingDownOutline
  return h('span', { style: { color, fontWeight: 'bold', display: 'flex', alignItems: 'center', gap: '4px' } }, [
    h('span', val >= 0 ? '+' : ''),
    h('span', val.toFixed(2) + '%'),
  ])
}

function renderReason(row) {
  if (!row.reason) return '-'
  return h('span', { style: { fontSize: '12px', color: '#666' } }, row.reason)
}

function renderActions(row) {
  return h('div', { style: { display: 'flex', gap: '8px' } }, [
    h(
      'a',
      {
        href: '#',
        style: { color: row.remarks ? '#18a058' : '#909399', fontSize: '12px' },
        onClick: (e) => {
          e.preventDefault()
          editRemarks(row)
        },
      },
      row.remarks || '添加备注'
    ),
  ])
}

const columns = [
  { title: '排名', key: 'rank', width: 60, align: 'center' },
  { title: '日期', key: 'tradeDate', width: 110 },
  { title: '代码', key: 'stockCode', width: 100 },
  { title: '名称', key: 'stockName', width: 100, ellipsis: { tooltip: true } },
  { title: '综合评分', key: 'score', width: 90, align: 'center', render: renderScore, sorter: (a, b) => a.score - b.score },
  { title: '收盘价', key: 'closePrice', width: 80, align: 'right', render: (r) => r.closePrice ? r.closePrice.toFixed(2) : '-' },
  { title: '涨跌幅', key: 'changePercent', width: 80, align: 'right', render: (r) => r.changePercent ? r.changePercent.toFixed(2) + '%' : '-' },
  { title: 'MA5/10/20', key: 'ma5', width: 130, render: (r) => r.ma5 ? `${r.ma5.toFixed(2)} / ${r.ma10.toFixed(2)} / ${r.ma20.toFixed(2)}` : '-' },
  { title: 'RSI', key: 'rsi14', width: 60, align: 'center', render: (r) => r.rsi14 ? r.rsi14.toFixed(1) : '-' },
  { title: 'MACD', key: 'macd', width: 80, align: 'right', render: (r) => r.macd ? r.macd.toFixed(3) : '-' },
  { title: '量比因子', key: 'volumeFactor', width: 70, align: 'center', render: (r) => r.volumeFactor ? r.volumeFactor.toFixed(2) : '-' },
  { title: '均线因子', key: 'maFactor', width: 70, align: 'center', render: (r) => r.maFactor ? r.maFactor.toFixed(2) : '-' },
  { title: '次日收益', key: 'nextReturn', width: 90, align: 'center', render: renderReturn },
  { title: '最大收益', key: 'nextMaxReturn', width: 80, align: 'right', render: (r) => r.reviewed ? (r.nextMaxReturn >= 0 ? '+' : '') + r.nextMaxReturn.toFixed(2) + '%' : '-' },
  { title: '最大回撤', key: 'nextMaxDrawdown', width: 80, align: 'right', render: (r) => r.reviewed ? r.nextMaxDrawdown.toFixed(2) + '%' : '-' },
  { title: '选股理由', key: 'reason', ellipsis: { tooltip: true }, render: renderReason },
  { title: '备注', key: 'remarks', ellipsis: { tooltip: true }, render: renderActions },
]

// ---------- Methods ----------
async function loadPicks() {
  loading.value = true
  try {
    const q = {
      page: pagination.page,
      pageSize: pagination.pageSize,
      tradeDate: query.tradeDate,
      reviewed: query.reviewed,
    }
    const res = await GetDailyPicks(q)
    if (res) {
      picks.value = res.list || []
      pagination.total = res.total || 0
      pagination.pageCount = res.totalPages || 0
    }
  } catch (e) {
    message.error('加载失败: ' + e)
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const s = await GetDailyPickStats()
    if (s) stats.value = s
  } catch (_) {}
}

async function runPick() {
  running.value = true
  try {
    // 使用日期选择器的日期，未选择时默认今天
    const date = queryDate.value
      ? format(new Date(queryDate.value), 'yyyy-MM-dd')
      : format(new Date(), 'yyyy-MM-dd')
    await RunDailyPick(date, 5)
    message.success('选股完成: ' + date)
    await loadPicks()
    await loadStats()
  } catch (e) {
    message.error('选股失败: ' + e)
  } finally {
    running.value = false
  }
}

async function runReview() {
  reviewing.value = true
  try {
    const date = format(new Date(), 'yyyy-MM-dd')
    await RunDailyReview(date, '')
    message.success('复盘完成')
    await loadPicks()
    await loadStats()
  } catch (e) {
    message.error('复盘失败: ' + e)
  } finally {
    reviewing.value = false
  }
}

function onDateChange(ts) {
  if (ts) {
    query.tradeDate = format(new Date(ts), 'yyyy-MM-dd')
  } else {
    query.tradeDate = ''
  }
  query.page = 1
  loadPicks()
}

function editRemarks(row) {
  const remarksRef = ref(row.remarks || '')
  const d = dialog.create({
    title: '编辑备注',
    content: () => h(NInput, {
      value: remarksRef.value,
      type: 'textarea',
      placeholder: '输入备注内容...',
      autosize: { minRows: 3, maxRows: 8 },
      onUpdateValue: (val) => { remarksRef.value = val },
    }),
    positiveText: '保存',
    negativeText: '取消',
    onPositiveHandle: async () => {
      try {
        await UpdateDailyPickRemarks(row.id, remarksRef.value)
        message.success('保存成功')
        d.destroy()
      } catch (e) {
        message.error('保存失败: ' + e)
      }
    },
  })
}

function dateDisabled(ts) {
  // Allow any date
  return false
}

onMounted(async () => {
  await loadStats()
  await loadPicks()
})
</script>

<style scoped>
.n-card {
  --n-padding: 12px;
}
</style>
