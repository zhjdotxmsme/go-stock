<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { GetCommodityKLine, GetCommodityKLineIntl } from '../../wailsjs/go/main/App'
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

async function loadChart() {
  if (!chartContainerRef.value) return
  loading.value = true
  errorText.value = ''
  try {
    const apiCall = props.internationalRef ? GetCommodityKLineIntl : GetCommodityKLine
    const bars = await apiCall(props.code, props.period, props.count)
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

    const lineData = bars.map((b, i) => ({
      time: (new Date(b.Time)).getTime() / 1000,
      value: b.Close,
    }))

    const volData = bars.map((b, i) => ({
      time: (new Date(b.Time)).getTime() / 1000,
      value: b.Volume,
      color: i > 0 && b.Close >= bars[i - 1].Close
        ? 'rgba(239,83,80,0.5)'
        : 'rgba(38,166,154,0.5)',
    }))

    lineSeries = chart.addSeries(LineSeries, {
      color: props.internationalRef ? '#FF9800' : '#2196F3',
      lineWidth: 2,
      priceFormat: {
        type: 'price',
        precision: 2,
        minMove: 0.01,
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

onMounted(loadChart)

onBeforeUnmount(() => {
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
