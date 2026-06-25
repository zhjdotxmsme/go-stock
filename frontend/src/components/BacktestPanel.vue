<script setup>
import { computed, ref, reactive } from 'vue'
import { BacktestRecommend, BacktestRecommendBatch } from '../../wailsjs/go/backtest/Service'
import {
  NAlert, NButton, NCard, NDataTable, NDatePicker, NDivider,
  NFlex, NForm, NFormItem, NGi, NGradientText, NGrid,
  NInput, NInputNumber, NNumberAnimation, NSpin, NStatistic,
  NSwitch, NTabPane, NTabs, NTag, NText
} from 'naive-ui'
import { useMessage } from 'naive-ui'

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
  startDate: null,
  endDate: null,
  holdingDays: 5,
  stopLoss: 0.05,
  stopProfit: 0.1,
  adjusted: true,
})

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
    { label: '夏普比率', value: 'N/A', raw: 0 },
  ]
})

const batchMetrics = computed(() => {
  const r = batchResult.value
  if (!r) return []
  return [
    { metric: '总回测数', value: r.Total },
    { metric: '胜率', value: pct(r.WinRate) },
    { metric: '胜场', value: r.WinCount },
    { metric: '平均收益率', value: pct(r.AvgReturn) },
    { metric: '平均最大回撤', value: pct(r.AvgDrawdown) },
    { metric: '平均Alpha', value: pct(r.AvgAlpha) },
    { metric: '夏普比率', value: 'N/A' },
  ]
})

const metricsColumns = [
  { title: '指标', key: 'metric', width: 200 },
  { title: '数值', key: 'value' },
]

const ratingColumns = [
  { title: '信号强度', key: 'rating', width: 120 },
  { title: '数量', key: 'count', width: 80 },
  { title: '胜率', key: 'winRate' },
  { title: '平均收益', key: 'avgReturn' },
  { title: '平均回撤', key: 'avgDrawdown' },
]

const ratingData = computed(() => {
  const r = batchResult.value
  if (!r || !r.ByRating) return []
  return Object.entries(r.ByRating).map(([rating, stats]) => ({
    rating,
    count: stats.Count,
    winRate: stats.Count > 0 ? pct(stats.WinCount / stats.Count) : 'N/A',
    avgReturn: pct(stats.AvgReturn),
    avgDrawdown: pct(stats.AvgDrawdown),
  }))
})

async function runSingle() {
  singleLoading.value = true
  singleError.value = ''
  singleResult.value = null
  try {
    const res = await BacktestRecommend(0, singleForm.holdingDays)
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
    const res = await BacktestRecommendBatch({
      startDate: fmtDate(batchForm.startDate),
      endDate: fmtDate(batchForm.endDate),
      holdingDays: batchForm.holdingDays,
      stopLoss: batchForm.stopLoss,
      stopProfit: batchForm.stopProfit,
      adjusted: batchForm.adjusted,
    })
    batchResult.value = res
    message.success('批量回测完成')
  } catch (e) {
    batchError.value = String(e)
    message.error('批量回测失败: ' + String(e))
  } finally {
    batchLoading.value = false
  }
}
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
                  <n-input v-model:value="singleForm.stockCode" placeholder="如 sh600519" />
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
                <n-form-item label="开始日期">
                  <n-date-picker v-model:value="batchForm.startDate" type="date" style="width: 100%" />
                </n-form-item>
                <n-form-item label="结束日期">
                  <n-date-picker v-model:value="batchForm.endDate" type="date" style="width: 100%" />
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
              <n-button type="primary" @click="runBatch" :loading="batchLoading" style="width: 100%">
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
                  <n-divider v-if="batchResult.ByRating && Object.keys(batchResult.ByRating).length > 0" />
                  <template v-if="batchResult.ByRating && Object.keys(batchResult.ByRating).length > 0">
                    <n-text strong depth="2">按信号强度分组</n-text>
                    <n-data-table
                      :columns="ratingColumns"
                      :data="ratingData"
                      :bordered="true"
                      size="small"
                      style="margin-top: 8px"
                    />
                  </template>
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
    </n-tabs>
  </div>
</template>

<style scoped>
.backtest-panel {
  padding: 8px;
  --wails-draggable: no-drag;
}
</style>
