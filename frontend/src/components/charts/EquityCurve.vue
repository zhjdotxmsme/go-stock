<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'

interface CurvePoint {
  date: string
  value: number
}

const props = withDefaults(defineProps<{
  dailyValues: CurvePoint[]
  benchmark?: CurvePoint[]
  drawdown?: boolean
  title?: string
  dark?: boolean
  loading?: boolean
  height?: number
  initialCapital?: number
}>(), {
  drawdown: true,
  dark: false,
  loading: false,
  height: 300,
  initialCapital: 1,
})

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function buildOption(): echarts.EChartsOption {
  const isDark = props.dark
  const textColor = isDark ? '#a0a0a0' : '#666'
  const axisColor = isDark ? '#333' : '#e0e0e0'

  if (!props.dailyValues?.length) {
    return {
      title: {
        text: '暂无数据', left: 'center', top: 'center',
        textStyle: { color: textColor, fontSize: 14, fontWeight: 'normal' },
      },
    }
  }

  const dates = props.dailyValues.map(d => d.date)
  const values = props.dailyValues.map(d => d.value)
  const benchValues = props.benchmark?.map(d => d.value)

  // 计算回撤序列
  const ddValues: (number | undefined)[] = []
  if (props.drawdown) {
    let peak = values[0]
    for (const v of values) {
      if (v > peak) peak = v
      ddValues.push(peak > 0 ? ((v - peak) / peak * 100) : undefined)
    }
  }

  const series: echarts.SeriesOption[] = [
    {
      name: '策略净值',
      type: 'line',
      data: values,
      smooth: true,
      showSymbol: false,
      lineStyle: { width: 2, color: '#409eff' },
      itemStyle: { color: '#409eff' },
      yAxisIndex: 0,
      z: 2,
    },
  ]

  if (benchValues) {
    series.push({
      name: '基准',
      type: 'line',
      data: benchValues,
      smooth: true,
      showSymbol: false,
      lineStyle: { width: 1.5, color: '#909399', type: 'dashed' },
      itemStyle: { color: '#909399' },
      yAxisIndex: 0,
      z: 1,
    })
  }

  if (props.drawdown && ddValues.length > 0) {
    series.push({
      name: '回撤',
      type: 'line',
      data: ddValues,
      smooth: true,
      showSymbol: false,
      lineStyle: { width: 1, color: '#f56c6c' },
      itemStyle: { color: '#f56c6c' },
      yAxisIndex: 1,
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(245,108,108,0.4)' },
          { offset: 1, color: 'rgba(245,108,108,0.02)' },
        ]),
      },
      z: 0,
    })
  }

  return {
    title: props.title ? {
      text: props.title, textStyle: { fontSize: 13, color: textColor, fontWeight: 500 },
      left: 8, top: 4,
    } : undefined,
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? '#1d1d1d' : '#fff',
      borderColor: isDark ? '#333' : '#e0e0e0',
      textStyle: { color: textColor, fontSize: 12 },
    },
    legend: {
      data: ['策略净值', ...(benchValues ? ['基准'] : []), ...(props.drawdown ? ['回撤'] : [])],
      textStyle: { color: textColor, fontSize: 11 },
      top: 4, right: 8,
    },
    grid: { top: 40, left: 56, right: 56, bottom: 28 },
    xAxis: {
      type: 'category', data: dates,
      axisLabel: { color: textColor, fontSize: 10, rotate: dates.length > 20 ? 45 : 0 },
      axisLine: { lineStyle: { color: axisColor } },
    },
    yAxis: [
      {
        type: 'value', name: '净值',
        axisLabel: { color: textColor, fontSize: 10 },
        splitLine: { lineStyle: { color: axisColor, type: 'dashed' as const } },
        axisLine: { show: false },
      },
      props.drawdown ? {
        type: 'value', name: '回撤%',
        axisLabel: { color: textColor, fontSize: 10, formatter: '{value}%' },
        splitLine: { show: false },
        axisLine: { show: false },
        min: 'dataMin',
        max: 0,
      } : undefined,
    ].filter(Boolean),
    series,
    dataZoom: [
      { type: 'inside', xAxisIndex: 0, filterMode: 'none' },
      { type: 'slider', xAxisIndex: 0, height: 20, bottom: 0, borderColor: axisColor },
    ],
  }
}

function initChart() {
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value, props.dark ? 'dark' : undefined)
  chart.setOption(buildOption())
}

watch(() => [props.dailyValues, props.benchmark, props.dark, props.loading], () => {
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
