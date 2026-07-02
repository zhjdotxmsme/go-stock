<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'
import { useTheme } from '../../hooks/useTheme'

interface DataPoint {
  date: string
  value: number
}

const props = withDefaults(defineProps<{
  data: DataPoint[]
  title?: string
  xKey?: string
  yKey?: string
  dark?: boolean
  loading?: boolean
  empty?: boolean
  height?: number
  smooth?: boolean
  areaStyle?: boolean
  markLine?: { label: string; value: number }
  yUnit?: string
}>(), {
  xKey: 'date',
  yKey: 'value',
  dark: false,
  loading: false,
  empty: false,
  height: 240,
  smooth: true,
  areaStyle: false,
  yUnit: '%',
})

const emit = defineEmits<{
  dataPointClick: [payload: { date: string; value: number }]
}>()

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function buildOption(): echarts.EChartsOption {
  const isDark = props.dark
  const textColor = isDark ? '#a0a0a0' : '#666'
  const axisColor = isDark ? '#333' : '#e0e0e0'

  if (props.empty || !props.data?.length) {
    return {
      title: {
        text: '暂无数据',
        left: 'center',
        top: 'center',
        textStyle: { color: textColor, fontSize: 14, fontWeight: 'normal' },
      },
    }
  }

  const xData = props.data.map(d => d[props.xKey] as string)
  const yData = props.data.map(d => d[props.yKey] as number)

  const series: echarts.SeriesOption = {
    type: 'line',
    data: yData,
    smooth: props.smooth,
    showSymbol: false,
    lineStyle: { width: 2 },
    itemStyle: { color: isDark ? '#5470c6' : '#409eff' },
  }

  if (props.areaStyle) {
    series.areaStyle = {
      color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: isDark ? 'rgba(84,112,198,0.3)' : 'rgba(64,158,255,0.25)' },
        { offset: 1, color: isDark ? 'rgba(84,112,198,0.02)' : 'rgba(64,158,255,0.02)' },
      ]),
    }
  }

  const option: echarts.EChartsOption = {
    title: props.title ? {
      text: props.title,
      textStyle: { fontSize: 13, color: textColor, fontWeight: 500 },
      left: 8,
      top: 4,
    } : undefined,
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? '#1d1d1d' : '#fff',
      borderColor: isDark ? '#333' : '#e0e0e0',
      textStyle: { color: textColor, fontSize: 12 },
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        return `${p.name}<br/>${p.seriesName || '值'}: ${p.value}${props.yUnit}`
      },
    },
    grid: { top: 36, left: 48, right: 16, bottom: 28 },
    xAxis: {
      type: 'category',
      data: xData,
      axisLabel: { color: textColor, fontSize: 10, rotate: xData.length > 14 ? 45 : 0 },
      axisLine: { lineStyle: { color: axisColor } },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: textColor, fontSize: 10, formatter: `{value}${props.yUnit}` },
      axisLine: { show: false },
      splitLine: { lineStyle: { color: axisColor, type: 'dashed' as const } },
    },
    series: [series],
  }

  if (props.markLine) {
    series.markLine = {
      silent: true,
      data: [{ yAxis: props.markLine.value, label: { formatter: props.markLine.label } }],
      lineStyle: { type: 'dashed', color: '#e6a23c' },
    }
  }

  return option
}

function initChart() {
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value, props.dark ? 'dark' : undefined)
  chart.setOption(buildOption())
  chart.on('click', (params: any) => {
    if (params.dataIndex != null) {
      const dp = props.data[params.dataIndex]
      if (dp) emit('dataPointClick', { date: dp.date, value: dp.value })
    }
  })
}

watch(() => [props.data, props.dark, props.loading, props.empty], () => {
  if (chart) {
    if (props.loading) {
      chart.showLoading()
    } else {
      chart.hideLoading()
      chart.setOption(buildOption())
    }
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

onBeforeUnmount(() => {
  resizeObs?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div ref="chartRef" :style="{ height: height + 'px', width: '100%' }" />
</template>
