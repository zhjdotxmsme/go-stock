<script setup>
import {ref, computed, onMounted, onBeforeUnmount, watch} from 'vue'
import {createChart, CandlestickSeries, HistogramSeries} from 'lightweight-charts'
import * as commodityApi from '../api/commodity'

// 这些品种没有本币 K 线源（国内期新浪/东财不可用、WSCN 无数据），
// K 线只能走 Yahoo 国际参考合约（USD 计价），因此需要给用户一个明确提示。
const INTL_REF_CODES = ['XAU', 'XAG', 'USCO', 'AU', 'AG', 'SC']

const props = defineProps({
  code: {type: String, default: ''},
  name: {type: String, default: ''},
  period: {type: String, default: 'day'},
  height: {type: Number, default: 360},
  darkTheme: {type: Boolean, default: false},
})

const isIntlRef = computed(() => INTL_REF_CODES.includes((props.code || '').toUpperCase()))

const chartContainerRef = ref(null)
const loading = ref(false)
const errorText = ref('')
let chart = null
let candleSeries = null
let volumeSeries = null
let resizeObserver = null

const CLR_RISE = '#ef5350'
const CLR_FALL = '#26a69a'

function timeToChart(t) {
  if (!t) return null
  const d = new Date(t)
  if (Number.isNaN(d.getTime())) return null
  // lightweight-charts 支持 BusinessDay 格式：{ year, month, day }
  // 使用 UTC 日期避免时区偏移导致日期错位
  return {
    year: d.getUTCFullYear(),
    month: d.getUTCMonth() + 1,
    day: d.getUTCDate(),
  }
}

// Wails/Go 序列化的 K 线键名可能为小写({time,open,...})或驼峰({Time,Open,...})。
// 这里统一按"小写优先、再取首字母大写"读取，兼容两种契约，避免因键名大小写导致整张 K 线空白。
function barField(b, key) {
  if (!b) return undefined
  const lower = b[key]
  if (lower !== undefined && lower !== null) return lower
  return b[key.charAt(0).toUpperCase() + key.slice(1)]
}

async function fetchKLine() {
  if (!props.code || !chart) return
  loading.value = true
  errorText.value = ''
  try {
    const bars = (await commodityApi.getCommodityKLine(props.code, props.period, 200)).data
    if (!bars || bars.length === 0) {
      errorText.value = '暂无K线数据'
      loading.value = false
      return
    }
    const candles = []
    const volumes = []
    for (const b of bars) {
      const open = Number(barField(b, 'open'))
      const close = Number(barField(b, 'close'))
      const high = Number(barField(b, 'high'))
      const low = Number(barField(b, 'low'))
      const time = timeToChart(barField(b, 'time'))
      if (!time || !Number.isFinite(open) || !Number.isFinite(close)) continue
      candles.push({
        time,
        open,
        close,
        high: Number.isFinite(high) ? high : open,
        low: Number.isFinite(low) ? low : open,
      })
      volumes.push({
        time,
        value: Number(barField(b, 'volume')) || 0,
        color: close >= open ? CLR_RISE + '80' : CLR_FALL + '80',
      })
    }
    // 有 bar 但时间戳全非法/键名对不上导致无有效蜡烛 → 干净降级，而不是空白图
    if (candles.length === 0) {
      errorText.value = '暂无K线数据'
      loading.value = false
      return
    }
    if (candleSeries) chart.removeSeries(candleSeries)
    if (volumeSeries) chart.removeSeries(volumeSeries)

    candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: CLR_RISE,
      downColor: CLR_FALL,
      borderUpColor: CLR_RISE,
      borderDownColor: CLR_FALL,
      wickUpColor: CLR_RISE,
      wickDownColor: CLR_FALL,
    })
    candleSeries.setData(candles)

    volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: {type: 'volume'},
      priceScaleId: 'volume',
    })
    volumeSeries.priceScale().applyOptions({scaleMargins: {top: 0.8, bottom: 0}})
    volumeSeries.setData(volumes)

    chart.timeScale().fitContent()
  } catch (e) {
    errorText.value = '加载失败: ' + (e.message || e)
  } finally {
    loading.value = false
  }
}

function createChartInstance() {
  if (!chartContainerRef.value) return
  if (chart) {
    chart.remove()
    chart = null
  }
  const bg = props.darkTheme ? '#1e1e1e' : '#ffffff'
  const txt = props.darkTheme ? '#d1d4dc' : '#333'
  const grid = props.darkTheme ? '#2b2b43' : '#e1e1e1'
  chart = createChart(chartContainerRef.value, {
    width: chartContainerRef.value.clientWidth,
    height: props.height,
    layout: {background: {color: bg}, textColor: txt},
    grid: {vertLines: {color: grid}, horzLines: {color: grid}},
    crosshair: {mode: 0},
    rightPriceScale: {borderColor: grid},
    timeScale: {borderColor: grid},
  })
}

onMounted(() => {
  createChartInstance()
  fetchKLine()
  // 监听容器大小变化，自动调整图表尺寸
  if (window.ResizeObserver && chartContainerRef.value) {
    resizeObserver = new ResizeObserver(() => {
      if (chart) {
        chart.applyOptions({ width: chartContainerRef.value.clientWidth })
      }
    })
    resizeObserver.observe(chartContainerRef.value)
  }
})

onBeforeUnmount(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (chart) {
    chart.remove()
    chart = null
  }
})

watch(() => props.code, () => {
  fetchKLine()
})

watch(() => props.period, () => {
  fetchKLine()
})
</script>

<template>
  <div>
    <div v-if="errorText" class="text-center text-red-500 py-2">{{ errorText }}</div>
    <div v-if="isIntlRef && !errorText" class="intl-ref-hint">
      注：此 K 线来自国际参考合约（USD 计价），非本币主力合约，仅供参考。
    </div>
    <div ref="chartContainerRef" :style="{height: height + 'px'}"></div>
  </div>
</template>

<style scoped>
.intl-ref-hint {
  color: #e0a63b;
  font-size: 12px;
  line-height: 1.5;
  padding: 4px 8px;
  margin-bottom: 6px;
  background: rgba(224, 166, 59, 0.08);
  border-left: 3px solid #e0a63b;
  border-radius: 6px;
}
</style>
