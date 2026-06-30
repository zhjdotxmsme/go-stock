<script setup>
import {ref, onMounted, onBeforeUnmount, watch} from 'vue'
import {createChart, CandlestickSeries, HistogramSeries} from 'lightweight-charts'
import {GetCommodityKLine} from '../../wailsjs/go/main/App'

const props = defineProps({
  code: {type: String, default: ''},
  name: {type: String, default: ''},
  period: {type: String, default: 'day'},
  height: {type: Number, default: 360},
  darkTheme: {type: Boolean, default: false},
})

const chartContainerRef = ref(null)
const loading = ref(false)
const errorText = ref('')
let chart = null
let candleSeries = null
let volumeSeries = null

const CLR_RISE = '#ef5350'
const CLR_FALL = '#26a69a'

function timeToChart(t) {
  if (!t) return null
  const d = new Date(t)
  if (Number.isNaN(d.getTime())) return null
  return d.toISOString().split('T')[0]
}

async function fetchKLine() {
  if (!props.code || !chart) return
  loading.value = true
  errorText.value = ''
  try {
    const bars = await GetCommodityKLine(props.code, props.period, 200)
    if (!bars || bars.length === 0) {
      errorText.value = '暂无K线数据'
      loading.value = false
      return
    }
    const candles = []
    const volumes = []
    for (const b of bars) {
      const time = timeToChart(b.Time)
      if (!time) continue
      candles.push({time, open: b.Open, high: b.High, low: b.Low, close: b.Close})
      const c = b.Close
      const o = b.Open
      volumes.push({
        time,
        value: Number(b.Volume) || 0,
        color: c >= o ? CLR_RISE + '80' : CLR_FALL + '80',
      })
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
})

onBeforeUnmount(() => {
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
    <div ref="chartContainerRef" :style="{height: height + 'px'}"></div>
  </div>
</template>
