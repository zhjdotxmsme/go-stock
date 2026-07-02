<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, computed } from 'vue'
import * as echarts from 'echarts'

interface MonthData {
  year: number
  month: number
  value: number
}

const props = withDefaults(defineProps<{
  data: MonthData[]
  title?: string
  dark?: boolean
  loading?: boolean
  height?: number
}>(), { dark: false, loading: false, height: 200 })

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

const years = computed(() => [...new Set(props.data.map(d => d.year))].sort())

function buildOption(): echarts.EChartsOption {
  const isDark = props.dark
  const tc = isDark ? '#a0a0a0' : '#666'
  if (!props.data?.length) return { title: { text: '暂无数据', left: 'center', top: 'center', textStyle: { color: tc, fontSize: 14, fontWeight: 'normal' as const } } }

  const maxAbs = Math.max(...props.data.map(d => Math.abs(d.value)), 0.01)
  const cellData = props.data.map(d => [`${d.year}-${String(d.month).padStart(2, '0')}-01`, d.year, d.month, d.value])

  return {
    title: props.title ? { text: props.title, textStyle: { fontSize: 13, color: tc, fontWeight: 500 }, left: 8, top: 4 } : undefined,
    tooltip: {
      backgroundColor: isDark ? '#1d1d1d' : '#fff',
      borderColor: isDark ? '#333' : '#e0e0e0',
      textStyle: { color: tc, fontSize: 12 },
      formatter: (p: any) => `${p.data[0]}: ${p.data[3].toFixed(2)}%`,
    },
    visualMap: {
      min: -maxAbs, max: maxAbs,
      calculable: true, orient: 'vertical', right: 0, top: 36, bottom: 8,
      inRange: { color: isDark ? ['#3b0a0a', '#1d1d1d', '#0a3b0a'] : ['#fde2e2', '#fff', '#e1f3d8'] },
    },
    calendar: [
      { orient: 'horizontal', range: years.value.map(String), left: 8, right: 48, top: 36, cellSize: ['auto', 20], splitLine: { lineStyle: { color: isDark ? '#333' : '#e0e0e0' } }, dayLabel: { show: false }, monthLabel: { color: tc, fontSize: 10 }, yearLabel: { color: tc, fontSize: 10 } },
    ],
    series: [{
      type: 'heatmap', coordinateSystem: 'calendar',
      data: cellData,
      label: { show: true, fontSize: 10, color: tc, formatter: (p: any) => `${p.data[3].toFixed(1)}%` },
    }],
  }
}

function initChart() {
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value, props.dark ? 'dark' : undefined)
  chart.setOption(buildOption())
}

watch(() => [props.data, props.dark, props.loading], () => {
  if (chart) { if (props.loading) chart.showLoading(); else { chart.hideLoading(); chart.setOption(buildOption()) } }
}, { deep: true })

let ro: ResizeObserver | null = null
onMounted(() => { initChart(); if (chartRef.value) { ro = new ResizeObserver(() => chart?.resize()); ro.observe(chartRef.value) } })
onBeforeUnmount(() => { ro?.disconnect(); chart?.dispose() })
</script>

<template>
  <div ref="chartRef" :style="{ height: height + 'px', width: '100%' }" />
</template>
