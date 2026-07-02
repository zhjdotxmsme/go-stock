<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'

interface SeriesItem {
  name: string
  data: number[]
  color?: string
}

const props = withDefaults(defineProps<{
  categories: string[]
  series: SeriesItem[]
  title?: string
  dark?: boolean
  loading?: boolean
  empty?: boolean
  height?: number
  horizontal?: boolean
}>(), {
  dark: false,
  loading: false,
  empty: false,
  height: 240,
  horizontal: false,
})

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function buildOption(): echarts.EChartsOption {
  const isDark = props.dark
  const textColor = isDark ? '#a0a0a0' : '#666'
  const axisColor = isDark ? '#333' : '#e0e0e0'

  if (props.empty || !props.categories?.length || !props.series?.length) {
    return {
      title: {
        text: '暂无数据', left: 'center', top: 'center',
        textStyle: { color: textColor, fontSize: 14, fontWeight: 'normal' },
      },
    }
  }

  const colors = ['#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399', '#5470c6']
  const seriesList: echarts.SeriesOption[] = props.series.map((s, i) => ({
    name: s.name,
    type: props.horizontal ? 'bar' : 'bar',
    data: s.data,
    itemStyle: { color: s.color || colors[i % colors.length], borderRadius: [2, 2, 0, 0] },
  }))

  return {
    title: props.title ? {
      text: props.title, textStyle: { fontSize: 13, color: textColor, fontWeight: 500 },
      left: 8, top: 4,
    } : undefined,
    tooltip: {
      trigger: 'axis', backgroundColor: isDark ? '#1d1d1d' : '#fff',
      borderColor: isDark ? '#333' : '#e0e0e0', textStyle: { color: textColor, fontSize: 12 },
      axisPointer: { type: 'shadow' },
    },
    legend: props.series.length > 1 ? {
      data: props.series.map(s => s.name),
      textStyle: { color: textColor, fontSize: 11 },
      top: 4, right: 8,
    } : undefined,
    grid: { top: props.series.length > 1 ? 40 : 36, left: props.horizontal ? 72 : 48, right: 16, bottom: 28 },
    xAxis: props.horizontal ? {
      type: 'value',
      axisLabel: { color: textColor, fontSize: 10 },
      splitLine: { lineStyle: { color: axisColor, type: 'dashed' as const } },
      axisLine: { show: false },
    } : {
      type: 'category', data: props.categories,
      axisLabel: { color: textColor, fontSize: 10, rotate: props.categories.length > 8 ? 45 : 0 },
      axisLine: { lineStyle: { color: axisColor } },
    },
    yAxis: props.horizontal ? {
      type: 'category', data: props.categories,
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: axisColor } },
    } : {
      type: 'value',
      axisLabel: { color: textColor, fontSize: 10 },
      splitLine: { lineStyle: { color: axisColor, type: 'dashed' as const } },
      axisLine: { show: false },
    },
    series: seriesList,
  }
}

function initChart() {
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value, props.dark ? 'dark' : undefined)
  chart.setOption(buildOption())
}

watch(() => [props.categories, props.series, props.dark, props.loading, props.empty], () => {
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
