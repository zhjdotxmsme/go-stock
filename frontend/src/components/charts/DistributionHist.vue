<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'

const props = withDefaults(defineProps<{
  data: number[]
  buckets?: number
  title?: string
  dark?: boolean
  loading?: boolean
  empty?: boolean
  height?: number
  showStats?: boolean
}>(), {
  buckets: 20,
  dark: false,
  loading: false,
  empty: false,
  height: 240,
  showStats: true,
})

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function computeHistogram(values: number[], bucketCount: number) {
  if (!values.length) return { bins: [], mean: 0, median: 0, std: 0 }

  const sorted = [...values].sort((a, b) => a - b)
  const mean = sorted.reduce((s, v) => s + v, 0) / sorted.length
  const median = sorted.length % 2 === 0
    ? (sorted[sorted.length / 2 - 1] + sorted[sorted.length / 2]) / 2
    : sorted[Math.floor(sorted.length / 2)]
  const std = Math.sqrt(sorted.reduce((s, v) => s + (v - mean) ** 2, 0) / sorted.length)

  const min = sorted[0]
  const max = sorted[sorted.length - 1]
  const binWidth = (max - min) / bucketCount || 1

  const bins = Array.from({ length: bucketCount }, (_, i) => ({
    x0: min + i * binWidth,
    x1: min + (i + 1) * binWidth,
    count: 0,
  }))

  for (const v of values) {
    const idx = Math.min(Math.floor((v - min) / binWidth), bucketCount - 1)
    bins[idx].count += 1
  }

  return { bins, mean, median, std }
}

function buildOption() {
  const isDark = props.dark
  const textColor = isDark ? '#a0a0a0' : '#666'
  const axisColor = isDark ? '#333' : '#e0e0e0'

  if (props.empty || !props.data?.length) {
    return {
      title: {
        text: '暂无数据', left: 'center', top: 'center',
        textStyle: { color: textColor, fontSize: 14, fontWeight: 'normal' },
      },
    }
  }

  const { bins, mean, median, std } = computeHistogram(props.data, props.buckets)

  const categories = bins.map(b => `${b.x0.toFixed(1)}-${b.x1.toFixed(1)}`)
  const counts = bins.map(b => b.count)

  const markLines: any[] = []
  if (props.showStats) {
    markLines.push(
      { xAxis: mean, label: { formatter: `均值 ${mean.toFixed(2)}%` }, lineStyle: { color: '#409eff', type: 'dashed' as const } },
      { xAxis: median, label: { formatter: `中位数 ${median.toFixed(2)}%` }, lineStyle: { color: '#67c23a', type: 'dashed' as const } },
    )
  }

  return {
    title: props.title ? {
      text: props.title, textStyle: { fontSize: 13, color: textColor, fontWeight: 500 },
      left: 8, top: 4,
    } : undefined,
    tooltip: {
      trigger: 'axis', backgroundColor: isDark ? '#1d1d1d' : '#fff',
      borderColor: isDark ? '#333' : '#e0e0e0', textStyle: { color: textColor, fontSize: 12 },
    },
    grid: { top: 36, left: 8, right: 16, bottom: 28 },
    xAxis: {
      type: 'category', data: categories,
      axisLabel: { color: textColor, fontSize: 9, rotate: 45, interval: 3 },
      axisLine: { lineStyle: { color: axisColor } },
    },
    yAxis: {
      type: 'value', axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { show: false }, splitLine: { lineStyle: { color: axisColor, type: 'dashed' as const } },
    },
    series: [{
      type: 'bar', data: counts,
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: isDark ? 'rgba(84,112,198,0.6)' : 'rgba(64,158,255,0.5)' },
          { offset: 1, color: isDark ? 'rgba(84,112,198,0.1)' : 'rgba(64,158,255,0.1)' },
        ]),
        borderRadius: [2, 2, 0, 0],
      },
      markLine: { silent: true, data: markLines },
    }],
  }
}

function initChart() {
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value, props.dark ? 'dark' : undefined)
  chart.setOption(buildOption())
}

watch(() => [props.data, props.dark, props.loading, props.empty], () => {
  if (chart) {
    if (props.loading) chart.showLoading()
    else { chart.hideLoading(); chart.setOption(buildOption()) }
  }
}, { deep: true })

let resizeObs: ResizeObserver | null = null

onMounted(() => {
  initChart()
  if (chartRef.value) {
    resizeObs = new ResizeObserver(() => chart?.resize())
    resizeObs.observe(chartRef.value)
  }
})

onBeforeUnmount(() => { resizeObs?.disconnect(); chart?.dispose() })
</script>

<template>
  <div ref="chartRef" :style="{ height: height + 'px', width: '100%' }" />
</template>
