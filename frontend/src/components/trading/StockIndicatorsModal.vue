<script setup>
import { ref, watch } from 'vue'
import { NDataTable, NModal, NSpin, useMessage } from 'naive-ui'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import * as tradeApi from '../../api/trade'

const props = defineProps({
  show: Boolean,
  code: String,
  name: String,
})
const emit = defineEmits(['update:show'])

const message = useMessage()
const loading = ref(false)
const rows = ref([])
const summaryText = ref('')

const columns = [
  { title: '指标', key: 'name', width: 130 },
  { title: '数值', key: 'value', width: 240 },
  { title: '信号 / 说明', key: 'signal' },
]

function fmt(v) {
  return typeof v === 'number' ? v.toFixed(2) : '-'
}
function get(obj, key) {
  return obj && typeof obj[key] === 'number' ? obj[key] : null
}

function load() {
  if (!props.code) return
  loading.value = true
  rows.value = []
  summaryText.value = ''
  tradeApi.getStockTechnicalIndicators(props.code)
    .then(({ data: res, error }) => {
      if (!res) throw new Error(error?.message || '获取技术指标失败')
      const ind = res.indicators || {}
      const s = res.summary || {}
      const macd = ind.macd || {}
      const kdj = ind.kdj || {}
      const boll = ind.boll || {}
      const ma = ind.ma || {}
      const cci = get(ind, 'cci')
      rows.value = [
        { name: 'MACD(12,26,9)', value: `DIF ${fmt(get(macd, 'MACD'))} / DEA ${fmt(get(macd, 'Signal'))}`, signal: `${s.macdSignal || '-'}，柱 ${fmt(get(macd, 'Histogram'))}` },
        { name: 'RSI14', value: fmt(get(ind.rsi, 'RSI14')), signal: s.rsiStatus || '-' },
        { name: 'KDJ(9,3)', value: `K ${fmt(get(kdj, 'K'))} / D ${fmt(get(kdj, 'D'))} / J ${fmt(get(kdj, 'J'))}`, signal: s.kdjSignal || '-' },
        { name: 'BOLL(20,2)', value: `上 ${fmt(get(boll, 'Up'))} / 中 ${fmt(get(boll, 'Mid'))} / 下 ${fmt(get(boll, 'Down'))}`, signal: s.bollStatus || '-' },
        { name: '均线 MA5/10/20/60', value: `${fmt(get(ma, 'MA5'))} / ${fmt(get(ma, 'MA10'))} / ${fmt(get(ma, 'MA20'))} / ${fmt(get(ma, 'MA60'))}`, signal: `趋势：${s.trend || '-'}` },
        { name: 'ATR(14)', value: fmt(ind.atr), signal: '平均真实波幅' },
        { name: 'CCI(20)', value: fmt(cci), signal: cci == null ? '-' : cci > 100 ? '超买区' : cci < -100 ? '超卖区' : '正常区间' },
        { name: 'WR(14)', value: fmt(ind.wr), signal: '威廉超买超卖' },
        { name: 'BIAS(5)', value: fmt(ind.bias), signal: '5日乖离率' },
        { name: 'OBV', value: ind.obv == null ? '-' : String(ind.obv), signal: '能量潮' },
      ]
      summaryText.value = [
        `**趋势**：${s.trend || '-'} ｜ **MACD**：${s.macdSignal || '-'} ｜ **RSI14**：${typeof s.rsiValue === 'number' ? s.rsiValue.toFixed(1) : '-'}（${s.rsiStatus || '-'}）｜ **KDJ**：${s.kdjSignal || '-'} ｜ **布林**：${s.bollStatus || '-'}`,
        '',
        s.summary ? `**解读**：${s.summary}` : '',
      ].filter(Boolean).join('\n')
    })
    .catch((e) => {
      console.error('获取技术指标失败:', e)
      message.error(e?.message || '获取技术指标失败')
    })
    .finally(() => {
      loading.value = false
    })
}

watch(() => props.show, (v) => {
  if (v) load()
})

function close() {
  emit('update:show', false)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="'技术指标 - ' + (name || code || '')"
    style="width: 820px; max-width: calc(100vw - 32px)"
    @update:show="close"
  >
    <n-spin :show="loading">
      <n-data-table size="small" :columns="columns" :data="rows" :bordered="false" />
      <div v-if="summaryText" style="margin-top: 8px; text-align: left">
        <MdPreview :model-value="summaryText" theme="light" />
      </div>
    </n-spin>
  </n-modal>
</template>
