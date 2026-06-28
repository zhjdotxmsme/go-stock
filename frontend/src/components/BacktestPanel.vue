<script setup>
import { computed, ref, reactive, h, onMounted } from 'vue'
import { RunSingleBacktest, RunBatchBacktest, GetBacktestResults } from '../../wailsjs/go/backtest/Service'
import { GetStockList } from '../../wailsjs/go/main/App'
import {
  NAutoComplete, NAlert, NButton, NCard, NDataTable, NDatePicker, NDivider,
  NFlex, NForm, NFormItem, NGi, NGradientText, NGrid,
  NIcon, NInput, NInputNumber, NNumberAnimation, NSpin, NStatistic,
  NSwitch, NTabPane, NTabs, NTag, NText, NPagination
} from 'naive-ui'
import { useMessage } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'

const message = useMessage()
const activeTab = ref('single')

const singleLoading = ref(false)
const batchLoading = ref(false)
const singleResult = ref(null)
const batchResult = ref(null)
const singleError = ref('')
const batchError = ref('')

const singleForm = reactive({
  stockCode: '',
  signalDate: null,
  entryPrice: 0,
  holdingDays: 5,
  stopLoss: 0.05,
  stopProfit: 0.1,
  adjusted: true,
})

const batchForm = reactive({
  stockCode: '',
  startDate: null,
  endDate: null,
  period: 'day',
  entryPrice: 0,
  holdingDays: 5,
  stopLoss: 0.05,
  stopProfit: 0.1,
  adjusted: true,
})

// ── 股票搜索自动补全 ──
const stockList = ref([])
const singleStockOptions = ref([])
const batchStockOptions = ref([])

function findStockList(formOptions, query) {
  if (!query || !query.trim()) {
    formOptions.value = []
    return
  }
  const q = query.trim().toLowerCase()
  formOptions.value = stockList.value
    .filter(item =>
      item.name.toLowerCase().includes(q) ||
      item.ts_code.toLowerCase().includes(q)
    )
    .slice(0, 30)
    .map(item => ({
      label: item.name + ' - ' + item.ts_code,
      value: item.ts_code,
    }))
}

function handleSelectStock(form, val) {
  form.stockCode = val
}

function fmtDate(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function pct(v) {
  if (v == null) return 'N/A'
  return (v * 100).toFixed(2) + '%'
}

const singleMetrics = computed(() => {
  const r = singleResult.value
  if (!r) return []
  return [
    { label: '总收益率', value: pct(r.TotalReturn), raw: r.TotalReturn * 100 },
    { label: '最大回撤', value: pct(r.MaxDrawdown), raw: r.MaxDrawdown * 100 },
    { label: 'Alpha收益', value: pct(r.Alpha), raw: r.Alpha * 100 },
  ]
})

const batchMetrics = computed(() => {
  const r = batchResult.value
  if (!r) return []
  return [
    { metric: '总回测数', value: r.TotalTrades },
    { metric: '胜率', value: pct(r.WinRate) },
    { metric: '胜场', value: r.WinCount },
    { metric: '平均收益率', value: pct(r.AvgReturn) },
    { metric: '总收益率', value: pct(r.TotalReturn) },
    { metric: '平均持仓(天)', value: r.AvgHoldingDays?.toFixed(1) },
    { metric: '最大回撤', value: pct(r.MaxDrawdown) },
    { metric: '夏普比率', value: r.SharpeRatio?.toFixed(2) ?? 'N/A' },
  ]
})

const metricsColumns = [
  { title: '指标', key: 'metric', width: 200 },
  { title: '数值', key: 'value' },
]

async function runSingle() {
  if (!singleForm.stockCode) {
    message.warning('请输入股票代码')
    return
  }
  singleLoading.value = true
  singleError.value = ''
  singleResult.value = null
  try {
    const res = await RunSingleBacktest(
      singleForm.stockCode,
      fmtDate(singleForm.signalDate),
      singleForm.entryPrice,
      singleForm.holdingDays,
      singleForm.stopLoss,
      singleForm.stopProfit,
      singleForm.adjusted,
    )
    singleResult.value = res
    message.success('单次回测完成')
  } catch (e) {
    singleError.value = String(e)
    message.error('回测失败: ' + String(e))
  } finally {
    singleLoading.value = false
  }
}

async function runBatch() {
  batchLoading.value = true
  batchError.value = ''
  batchResult.value = null
  try {
    const res = await RunBatchBacktest(
      batchForm.stockCode,
      fmtDate(batchForm.startDate),
      fmtDate(batchForm.endDate),
      batchForm.period,
      batchForm.adjusted,
      batchForm.entryPrice,
      batchForm.holdingDays,
      batchForm.stopLoss,
      batchForm.stopProfit,
    )
    batchResult.value = res
    message.success('批量回测完成')
  } catch (e) {
    batchError.value = String(e)
    message.error('批量回测失败: ' + String(e))
  } finally {
    batchLoading.value = false
  }
}

// ── 历史记录 ──
const historyLoading = ref(false)
const historySearchCode = ref('')
const historyData = ref(null)
const historyPage = ref(1)
const historyPageSize = ref(15)

const historyColumns = [
  { title: '股票代码', key: 'StockCode', width: 110 },
  { title: '股票名称', key: 'StockName', width: 110 },
  { title: '信号日期', key: 'SignalDate', width: 100 },
  { title: '入场价格', key: 'EntryPrice', width: 95, render: (r) => r.EntryPrice?.toFixed(2) },
  { title: '出场价格', key: 'ExitPrice', width: 95, render: (r) => r.ExitPrice?.toFixed(2) },
  { title: '总收益率', key: 'TotalReturn', width: 105,
    render: (r) => {
      const v = (r.TotalReturn * 100).toFixed(2)
      return h(NText, { type: r.TotalReturn >= 0 ? 'success' : 'error' }, { default: () => `${v}%` })
    }
  },
  { title: '持仓天数', key: 'HoldingDays', width: 80 },
  { title: '最大回撤', key: 'MaxDrawdown', width: 105,
    render: (r) => (r.MaxDrawdown * 100).toFixed(2) + '%'
  },
  { title: 'Alpha', key: 'Alpha', width: 95,
    render: (r) => (r.Alpha * 100).toFixed(2) + '%'
  },
  { title: '结果', key: 'Win', width: 70,
    render: (r) => h(NTag, { type: r.Win ? 'success' : 'error', size: 'small' }, { default: () => r.Win ? '盈利' : '亏损' })
  },
]

async function loadHistory(page = 1) {
  historyLoading.value = true
  historyPage.value = page
  try {
    const res = await GetBacktestResults(historySearchCode.value || '', page, historyPageSize.value)
    historyData.value = res
  } catch (e) {
    message.error('加载回测历史失败: ' + String(e))
    historyData.value = null
  } finally {
    historyLoading.value = false
  }
}

onMounted(() => {
  GetStockList('').then(result => {
    stockList.value = result || []
  }).catch(err => {
    console.error('GetStockList error:', err)
  })
})

const historyPagination = computed(() => {
  const d = historyData.value
  if (!d) return false
  return {
    page: d.page,
    pageSize: d.pageSize,
    itemCount: d.total,
    showSizePicker: true,
    pageSizes: [10, 15, 30, 50],
    onUpdatePage: (p) => loadHistory(p),
    onUpdatePageSize: (s) => { historyPageSize.value = s; loadHistory(1) },
  }
})
</script>

<template>
  <div class="backtest-panel">
    <n-tabs v-model:value="activeTab" type="line" animated>
      <n-tab-pane name="single" tab="单次回测">
        <n-grid :cols="24" :x-gap="16">
          <n-gi :span="10">
            <n-card title="回测参数" size="small">
              <n-form label-placement="left" label-width="100">
                <n-form-item label="股票代码">
                  <n-auto-complete
                    v-model:value="singleForm.stockCode"
                    :options="singleStockOptions"
                    placeholder="搜索股票名称或代码"
                    clearable
                    :on-select="(val) => handleSelectStock(singleForm, val)"
                    @update:value="(val) => findStockList(singleStockOptions, val)"
                  />
                </n-form-item>
                <n-form-item label="信号日期">
                  <n-date-picker v-model:value="singleForm.signalDate" type="date" style="width: 100%" />
                </n-form-item>
                <n-form-item label="入场价格">
                  <n-input-number v-model:value="singleForm.entryPrice" placeholder="0=信号日收盘价" style="width: 100%" :min="0" />
                </n-form-item>
                <n-form-item label="持仓天数">
                  <n-input-number v-model:value="singleForm.holdingDays" :min="1" style="width: 100%" />
                </n-form-item>
                <n-form-item label="止损比例">
                  <n-input-number v-model:value="singleForm.stopLoss" :min="0" :step="0.01" placeholder="0.05 = 5%" style="width: 100%" />
                </n-form-item>
                <n-form-item label="止盈比例">
                  <n-input-number v-model:value="singleForm.stopProfit" :min="0" :step="0.01" placeholder="0.10 = 10%" style="width: 100%" />
                </n-form-item>
                <n-form-item label="前复权">
                  <n-switch v-model:value="singleForm.adjusted" />
                </n-form-item>
              </n-form>
              <n-button type="primary" @click="runSingle" :loading="singleLoading" :disabled="!singleForm.stockCode">
                开始回测
              </n-button>
            </n-card>
          </n-gi>
          <n-gi :span="14">
            <n-card title="回测结果" size="small">
              <n-spin :show="singleLoading">
                <template v-if="singleError">
                  <n-alert type="error" closable @close="singleError = ''">
                    <template #header>
                      回测出错
                    </template>
                    {{ singleError }}
                  </n-alert>
                </template>
                <template v-else-if="singleResult">
                  <n-grid :cols="2" :x-gap="12" :y-gap="12">
                    <n-gi v-for="m in singleMetrics" :key="m.label">
                      <n-statistic :label="m.label" :tabular-nums="true">
                        <n-number-animation
                          v-if="m.raw > 0"
                          :from="0"
                          :to="m.raw"
                          :duration="800"
                        />
                        <template v-else>
                          {{ m.value }}
                        </template>
                        <template #suffix>
                          <n-text v-if="m.raw > 0" depth="3">%</n-text>
                        </template>
                      </n-statistic>
                    </n-gi>
                  </n-grid>
                  <n-divider />
                  <n-grid :cols="3" :x-gap="12" :y-gap="8">
                    <n-gi>
                      <n-text depth="3">入场价格</n-text>
                      <br><n-text>{{ singleResult.EntryPrice?.toFixed(2) }}</n-text>
                    </n-gi>
                    <n-gi>
                      <n-text depth="3">出场价格</n-text>
                      <br><n-text>{{ singleResult.ExitPrice?.toFixed(2) }}</n-text>
                    </n-gi>
                    <n-gi>
                      <n-text depth="3">出场日期</n-text>
                      <br><n-text>{{ singleResult.ExitDate || '-' }}</n-text>
                    </n-gi>
                    <n-gi>
                      <n-text depth="3">实际持仓</n-text>
                      <br><n-text>{{ singleResult.HoldingDays }} 天</n-text>
                    </n-gi>
                    <n-gi>
                      <n-text depth="3">基准收益</n-text>
                      <br><n-text>{{ pct(singleResult.BenchmarkReturn) }}</n-text>
                    </n-gi>
                    <n-gi>
                      <n-tag :type="singleResult.Win ? 'success' : 'error'" :bordered="false">
                        {{ singleResult.Win ? '盈利 ✓' : '亏损 ✗' }}
                      </n-tag>
                    </n-gi>
                  </n-grid>
                  <n-divider v-if="singleResult.SlippageWarning" />
                  <n-tag v-if="singleResult.SlippageWarning" type="warning" :bordered="false">
                    {{ singleResult.SlippageWarning }}
                  </n-tag>
                </template>
                <template v-else>
                  <div style="padding: 40px 0; text-align: center">
                    <n-text depth="3">输入参数后点击「开始回测」</n-text>
                  </div>
                </template>
              </n-spin>
            </n-card>
          </n-gi>
        </n-grid>
      </n-tab-pane>

      <n-tab-pane name="batch" tab="批量回测">
        <n-grid :cols="24" :x-gap="16">
          <n-gi :span="8">
            <n-card title="批量设置" size="small">
              <n-form label-placement="left" label-width="100">
                <n-form-item label="股票代码">
                  <n-auto-complete
                    v-model:value="batchForm.stockCode"
                    :options="batchStockOptions"
                    placeholder="搜索股票名称或代码"
                    clearable
                    :on-select="(val) => handleSelectStock(batchForm, val)"
                    @update:value="(val) => findStockList(batchStockOptions, val)"
                  />
                </n-form-item>
                <n-form-item label="开始日期">
                  <n-date-picker v-model:value="batchForm.startDate" type="date" style="width: 100%" />
                </n-form-item>
                <n-form-item label="结束日期">
                  <n-date-picker v-model:value="batchForm.endDate" type="date" style="width: 100%" />
                </n-form-item>
                <n-form-item label="入场价格">
                  <n-input-number v-model:value="batchForm.entryPrice" placeholder="0=信号日收盘价" style="width: 100%" :min="0" />
                </n-form-item>
                <n-form-item label="持仓天数">
                  <n-input-number v-model:value="batchForm.holdingDays" :min="1" style="width: 100%" />
                </n-form-item>
                <n-form-item label="止损比例">
                  <n-input-number v-model:value="batchForm.stopLoss" :min="0" :step="0.01" placeholder="0.05 = 5%" style="width: 100%" />
                </n-form-item>
                <n-form-item label="止盈比例">
                  <n-input-number v-model:value="batchForm.stopProfit" :min="0" :step="0.01" placeholder="0.10 = 10%" style="width: 100%" />
                </n-form-item>
                <n-form-item label="前复权">
                  <n-switch v-model:value="batchForm.adjusted" />
                </n-form-item>
              </n-form>
              <n-button type="primary" @click="runBatch" :loading="batchLoading" :disabled="!batchForm.stockCode" style="width: 100%">
                批量回测
              </n-button>
            </n-card>
          </n-gi>
          <n-gi :span="16">
            <n-card title="批量结果" size="small">
              <n-spin :show="batchLoading">
                <template v-if="batchError">
                  <n-alert type="error" closable @close="batchError = ''">
                    <template #header>
                      批量回测出错
                    </template>
                    {{ batchError }}
                  </n-alert>
                </template>
                <template v-else-if="batchResult">
                  <n-data-table
                    :columns="metricsColumns"
                    :data="batchMetrics"
                    :bordered="true"
                    :single-line="false"
                    size="small"
                  />
                </template>
                <template v-else>
                  <div style="padding: 40px 0; text-align: center">
                    <n-text depth="3">设置参数后点击「批量回测」</n-text>
                  </div>
                </template>
              </n-spin>
            </n-card>
          </n-gi>
        </n-grid>
      </n-tab-pane>

      <n-tab-pane name="history" tab="历史记录">
        <n-flex vertical :size="12">
          <n-flex align="center" :size="8">
            <n-input
              v-model:value="historySearchCode"
              placeholder="按股票代码筛选（留空=全部）"
              clearable
              style="width: 260px"
              @keyup.enter="loadHistory(1)"
            >
              <template #prefix>
                <n-icon :component="SearchOutline" />
              </template>
            </n-input>
            <n-button type="primary" size="small" @click="loadHistory(1)" :loading="historyLoading">查询</n-button>
          </n-flex>
          <n-data-table
            :columns="historyColumns"
            :data="historyData?.list || []"
            :loading="historyLoading"
            :bordered="true"
            :single-line="false"
            size="small"
            :max-height="560"
            virtual-scroll
          />
          <n-flex v-if="historyData && historyData.total > historyData.pageSize" justify="flex-end">
            <n-pagination
              :page="historyData.page"
              :page-size="historyData.pageSize"
              :item-count="historyData.total"
              :page-slot="7"
              show-size-picker
              :page-sizes="[10, 15, 30, 50]"
              @update:page="loadHistory"
              @update:page-size="(s) => { historyPageSize = s; loadHistory(1) }"
            />
          </n-flex>
          <n-text v-if="!historyLoading && (!historyData || historyData.list.length === 0)" depth="3" style="text-align: center; padding: 40px 0">
            暂无回测历史记录
          </n-text>
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<style scoped>
.backtest-panel {
  padding: 8px;
  --wails-draggable: no-drag;
}
</style>
