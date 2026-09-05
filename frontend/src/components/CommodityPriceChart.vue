<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as commodityApi from '../api/commodity'
import { createChart, LineSeries, HistogramSeries } from 'lightweight-charts'

const props = defineProps({
  code: { type: String, default: 'XAUUSD' },
  period: { type: String, default: 'day' },
  count: { type: Number, default: 120 },
  chartHeight: { type: Number, default: 320 },
  internationalRef: { type: Boolean, default: false },
  darkTheme: { type: Boolean, default: false },
})

const chartContainerRef = ref(null)
const loading = ref(false)
const errorText = ref('')
let chart = null
let lineSeries = null
let volumeSeries = null
let resizeObserver = null

// 自动推断价格精度：价格 < 1 用 4 位小数，< 10 用 3 位，< 100 用 2 位，否则 2 位
function inferPricePrecision(bars) {
  if (!bars || bars.length === 0) return { precision: 2, minMove: 0.01 }
  const firstClose = Number(bars[0]?.close ?? bars[0]?.Close ?? 0)
  if (firstClose < 1) return { precision: 4, minMove: 0.0001 }
  if (firstClose < 10) return { precision: 3, minMove: 0.001 }
  return { precision: 2, minMove: 0.01 }
}

async function loadChart() {
  if (!chartContainerRef.value) return
  loading.value = true
  errorText.value = ''
  try {
    const apiCall = props.internationalRef ? commodityApi.getCommodityKLineIntl : commodityApi.getCommodityKLine
    const bars = (await apiCall(props.code, props.period, props.count)).data
    if (!bars || bars.length === 0) {
      errorText.value = '暂无数据'
      loading.value = false
      return
    }

    if (chart) {
      chart.remove()
      chart = null
      lineSeries = null
      volumeSeries = null
    }

    const bg = props.darkTheme ? '#1e1e1e' : '#ffffff'
    const text = props.darkTheme ? '#ccc' : '#666'
    const grid = props.darkTheme ? '#333' : '#f0f0f0'
    const border = props.darkTheme ? '#444' : '#e0e0e0'

    chart = createChart(chartContainerRef.value, {
      height: props.chartHeight,
      layout: {
        background: { color: bg },
        textColor: text,
      },
      grid: {
        vertLines: { color: grid },
        horzLines: { color: grid },
      },
      rightPriceScale: {
        borderColor: border,
      },
      timeScale: {
        borderColor: border,
        timeVisible: true,
      },
      crosshair: {
        mode: 0,
      },
    })

    // 兼容 Go JSON 小写字段（time, open, close, high, low, volume）
    const getField = (b, key) => {
      if (b[key] !== undefined && b[key] !== null) return b[key]
      const capKey = key.charAt(0).toUpperCase() + key.slice(1)
      return b[capKey]
    }

    const lineData = []
    const volData = []
    for (let i = 0; i < bars.length; i++) {
      const b = bars[i]
      const timeMs = new Date(getField(b, 'time')).getTime()
      if (!Number.isFinite(timeMs)) continue
      const close = Number(getField(b, 'close'))
      if (!Number.isFinite(close)) continue
      const t = Math.floor(timeMs / 1000)
      lineData.push({ time: t, value: close })
      const prevClose = i > 0 ? Number(getField(bars[i - 1], 'close')) : close
      volData.push({
        time: t,
        value: Number(getField(b, 'volume')) || 0,
        color: close >= prevClose
          ? 'rgba(239,83,80,0.5)'
          : 'rgba(38,166,154,0.5)',
      })
    }

    if (lineData.length === 0) {
      errorText.value = '暂无有效K线数据'
      loading.value = false
      return
    }

    const priceFmt = inferPricePrecision(bars)
    lineSeries = chart.addSeries(LineSeries, {
      color: props.internationalRef ? '#FF9800' : '#2196F3',
      lineWidth: 2,
      priceFormat: {
        type: 'price',
        precision: priceFmt.precision,
        minMove: priceFmt.minMove,
      },
    })
    lineSeries.setData(lineData)

    volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: 'volume' },
      priceScaleId: 'volume',
    })
    chart.priceScale('volume').applyOptions({
      scaleMargins: { top: 0.85, bottom: 0 },
    })
    volumeSeries.setData(volData)

    chart.timeScale().fitContent()
  } catch (e) {
    console.error('commodity chart load error:', e)
    errorText.value = (e && (e.message || e.toString())) || '加载失败'
  } finally {
    loading.value = false
  }
}

watch(() => props.code, () => { nextTick(loadChart) })
watch(() => props.period, () => { nextTick(loadChart) })
watch(() => props.internationalRef, () => { nextTick(loadChart) })
watch(() => props.darkTheme, () => { nextTick(loadChart) })

onMounted(() => {
  loadChart()
  // 监听容器大小变化
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
</script>

<template>
  <div style="position: relative; width: 100%;">
    <n-spin :show="loading">
      <div ref="chartContainerRef" style="width: 100%; min-height: 100px;"></div>
    </n-spin>
    <n-empty
      v-if="!loading && errorText"
      :description="errorText"
      style="position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%);"
    >
      <template #extra>
        <n-button size="small" @click="loadChart">重试</n-button>
      </template>
    </n-empty>
  </div>
</template>
