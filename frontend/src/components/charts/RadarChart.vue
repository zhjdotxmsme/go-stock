<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts'

export interface Indicator { name: string; max: number }

const props = withDefaults(defineProps<{
  indicators: Indicator[]
  data: number[]
  dark?: boolean
  loading?: boolean
  height?: number
}>(), {
  dark: false,
  loading: false,
  height: 280,
})

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function buildOption(): echarts.EChartsOption {
  const isDark = props.dark
  const textColor = isDark ? '#a0a0a0' : '#666'

  if (!props.indicators?.length || !props.data?.length) {
    return {
      title: {
        text: '暂无数据', left: 'center', top: 'center',
        textStyle: { color: textColor, fontSize: 14, fontWeight: 'normal' },
      },
    }
  }

  return {
    tooltip: {
      backgroundColor: isDark ? '#1d1d1d' : '#fff',
      borderColor: isDark ? '#333' : '#e0e0e0',
      textStyle: { color: textColor, fontSize: 12 },
    },
    radar: {
      indicator: props.indicators.map(ind => ({ name: ind.name, max: ind.max })),
      center: ['50%', '50%'],
      radius: '65%',
      axisName: { color: textColor, fontSize: 11 },
      splitArea: {
        areaStyle: {
          color: isDark
            ? ['rgba(84,112,198,0.02)', 'rgba(84,112,198,0.05)', 'rgba(84,112,198,0.02)']
            : ['rgba(64,158,255,0.02)', 'rgba(64,158,255,0.05)', 'rgba(64,158,255,0.02)'],
        },
      },
      axisLine: { lineStyle: { color: isDark ? '#333' : '#ddd' } },
      splitLine: { lineStyle: { color: isDark ? '#333' : '#ddd' } },
    },
    series: [{
      type: 'radar',
      data: [{ value: props.data }],
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
          { offset: 0, color: isDark ? 'rgba(84,112,198,0.3)' : 'rgba(64,158,255,0.25)' },
          { offset: 1, color: isDark ? 'rgba(84,112,198,0.05)' : 'rgba(64,158,255,0.05)' },
        ]),
      },
      lineStyle: { color: isDark ? '#5470c6' : '#409eff', width: 2 },
      itemStyle: { color: isDark ? '#5470c6' : '#409eff' },
    }],
  }
}

function initChart() {
  if (!chartRef.value) return
  chart?.dispose()
  chart = echarts.init(chartRef.value, props.dark ? 'dark' : undefined)
  chart.setOption(buildOption())
}

watch(() => [props.indicators, props.data, props.dark, props.loading], () => {
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
