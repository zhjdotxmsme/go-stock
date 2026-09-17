<script setup>
import { h, ref, watch } from 'vue'
import { NButton, NDataTable, NModal, NSpace, NTag, NText, useMessage } from 'naive-ui'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import * as tradeApi from '../../api/trade'

const props = defineProps({
  show: Boolean,
})
const emit = defineEmits(['update:show'])

const message = useMessage()
const loading = ref(true)
const rows = ref([])
const pagination = ref({ page: 1, pageSize: 20, itemCount: 0, showSizePicker: false })

const showDetail = ref(false)
const detail = ref(null)
const detailLoading = ref(false)
const snapshotRows = ref([])

const snapshotColumns = [
  { title: '代码', key: 'stockCode', width: 110 },
  { title: '名称', key: 'stockName', width: 100 },
  { title: '数量', key: 'volume', width: 70 },
  { title: '成本价', key: 'costPrice', width: 80, render: (row) => fmtNum(row.costPrice, 3) },
  { title: '最新价', key: 'currentPrice', width: 80, render: (row) => (row.currentPrice > 0 ? fmtNum(row.currentPrice, 2) : '-') },
  { title: '市值', key: 'marketValue', width: 90, render: (row) => fmtNum(row.marketValue, 2) },
  { title: '浮动盈亏', key: 'profitAmount', width: 90, render: (row) => fmtNum(row.profitAmount, 2) },
  { title: '盈亏率', key: 'profitPercent', width: 80, render: (row) => (row.currentPrice > 0 ? fmtNum(row.profitPercent, 2) + '%' : '-') },
]

const columns = [
  { title: '日期', key: 'summaryDate', width: 110 },
  { title: '股票数', key: 'stockCount', width: 70 },
  {
    title: '浮动盈亏',
    key: 'totalProfit',
    width: 100,
    render: (row) => h(NText, { type: row.totalProfit > 0 ? 'error' : row.totalProfit < 0 ? 'success' : 'default' }, { default: () => fmtNum(row.totalProfit, 2) }),
  },
  {
    title: '盈亏率',
    key: 'profitRate',
    width: 85,
    render: (row) => h(NText, { type: row.profitRate > 0 ? 'error' : row.profitRate < 0 ? 'success' : 'default' }, { default: () => fmtNum(row.profitRate, 2) + '%' }),
  },
  { title: '模型', key: 'modelName', width: 140, ellipsis: { tooltip: true } },
  { title: '内容摘要', key: 'content', ellipsis: { tooltip: false } },
  {
    title: '操作',
    key: 'actions',
    width: 80,
    render(row) {
      return h(NButton, { size: 'small', type: 'info', tertiary: true, onClick: () => openDetail(row) }, { default: () => '查看' })
    },
  },
]

function fmtNum(v, digits) {
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(digits) : '-'
}

function load(page = 1) {
  loading.value = true
  tradeApi.getHoldingsSummaryList(page, pagination.value.pageSize)
    .then(({ data: res, error }) => {
      if (!res) throw new Error(error?.message || '获取总结历史失败')
      rows.value = res.list || []
      pagination.value.page = res.page || page
      pagination.value.itemCount = res.total || 0
    })
    .catch((e) => message.error(e?.message || '获取总结历史失败'))
    .finally(() => {
      loading.value = false
    })
}

function handlePageChange(page) {
  load(page)
}

function openDetail(row) {
  showDetail.value = true
  detailLoading.value = true
  detail.value = null
  snapshotRows.value = []
  tradeApi.getHoldingsSummaryDetail(row.ID || row.id)
    .then(({ data: res, error }) => {
      if (!res) throw new Error(error?.message || '获取总结详情失败')
      detail.value = res
      try {
        snapshotRows.value = JSON.parse(res.holdingsSnapshot || '[]')
      } catch {
        snapshotRows.value = []
      }
    })
    .catch((e) => message.error(e?.message || '获取总结详情失败'))
    .finally(() => {
      detailLoading.value = false
    })
}

watch(() => props.show, (v) => {
  if (v) load(1)
})

function close() {
  emit('update:show', false)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    title="持仓总结历史"
    style="width: 1000px; max-width: calc(100vw - 32px)"
    @update:show="close"
  >
    <n-data-table
      remote
      size="small"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row) => row.ID"
      @update:page="handlePageChange"
      style="margin-top: 4px"
    />

    <n-modal
      v-model:show="showDetail"
      preset="card"
      :title="'持仓总结详情 - ' + (detail?.summaryDate || '')"
      style="width: 960px; max-width: calc(100vw - 32px)"
    >
      <n-spin :show="detailLoading">
        <n-space vertical>
          <n-space v-if="detail" align="center">
            <n-tag type="info" size="small">模型：{{ detail.modelName || '-' }}</n-tag>
            <n-tag size="small">持仓 {{ detail.stockCount }} 只</n-tag>
            <n-tag :type="detail.totalProfit > 0 ? 'error' : detail.totalProfit < 0 ? 'success' : 'default'" size="small" round>
              浮动盈亏 {{ fmtNum(detail.totalProfit, 2) }}（{{ fmtNum(detail.profitRate, 2) }}%）
            </n-tag>
          </n-space>
          <n-data-table size="small" :columns="snapshotColumns" :data="snapshotRows" :bordered="false" :max-height="180" />
          <div v-if="detail?.content" style="text-align: left; max-height: calc(100vh - 480px); overflow-y: auto; border: 1px solid #e4e9f0; border-radius: 6px; padding: 4px 12px">
            <MdPreview :model-value="detail.content" theme="light" />
          </div>
        </n-space>
      </n-spin>
    </n-modal>
  </n-modal>
</template>
