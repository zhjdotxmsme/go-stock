<script setup>
import * as echarts from "echarts";
import { ref, watch } from "vue";

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: [Number, String], default: 350 },
  topStocks: { type: Array, default: () => [] },
  topIndustries: { type: Array, default: () => [] },
  topConcepts: { type: Array, default: () => [] }
})

const emit = defineEmits(['dimensionClick'])

const bullBearStockUpRef = ref(null)
const bullBearStockDownRef = ref(null)
const bullBearIndustryUpRef = ref(null)
const bullBearIndustryDownRef = ref(null)
const bullBearConceptUpRef = ref(null)
const bullBearConceptDownRef = ref(null)

watch(() => [props.topStocks, props.topIndustries, props.topConcepts], () => {
  renderAllCharts()
}, { immediate: true, deep: true })

function renderAllCharts() {
  if (props.topStocks && props.topStocks.length > 0) {
    const upStocks = [...props.topStocks].sort((a, b) => b.upCount - a.upCount).slice(0, 15)
    const downStocks = [...props.topStocks].sort((a, b) => b.downCount - a.downCount).slice(0, 15)
    renderBullBearChart(bullBearStockUpRef, '利好异动最多的股票', upStocks, 'up', 'stock')
    renderBullBearChart(bullBearStockDownRef, '利空异动最多的股票', downStocks, 'down', 'stock')
  }
  if (props.topIndustries && props.topIndustries.length > 0) {
    const upIndustries = [...props.topIndustries].sort((a, b) => b.upCount - a.upCount).slice(0, 15)
    const downIndustries = [...props.topIndustries].sort((a, b) => b.downCount - a.downCount).slice(0, 15)
    renderBullBearChart(bullBearIndustryUpRef, '利好异动最多的行业', upIndustries, 'up', 'industry')
    renderBullBearChart(bullBearIndustryDownRef, '利空异动最多的行业', downIndustries, 'down', 'industry')
  }
  if (props.topConcepts && props.topConcepts.length > 0) {
    const upConcepts = [...props.topConcepts].sort((a, b) => b.upCount - a.upCount).slice(0, 15)
    const downConcepts = [...props.topConcepts].sort((a, b) => b.downCount - a.downCount).slice(0, 15)
    renderBullBearChart(bullBearConceptUpRef, '利好异动最多的概念', upConcepts, 'up', 'concept')
    renderBullBearChart(bullBearConceptDownRef, '利空异动最多的概念', downConcepts, 'down', 'concept')
  }
}

function renderBullBearChart(chartRefVal, title, items, direction, dimension) {
  if (!chartRefVal.value || !items || items.length === 0) return

  const chart = echarts.init(chartRefVal.value)

  const names = items.map(d => d.name).reverse()
  const mainValues = direction === 'up'
    ? items.map(d => d.upCount).reverse()
    : items.map(d => d.downCount).reverse()
  const subValues = direction === 'up'
    ? items.map(d => d.downCount).reverse()
    : items.map(d => d.upCount).reverse()

  const mainColor = direction === 'up' ? '#ef4444' : '#22c55e'
  const subColor = direction === 'up' ? '#22c55e' : '#ef4444'
  const mainLabel = direction === 'up' ? '利好次数' : '利空次数'
  const subLabel = direction === 'up' ? '利空次数' : '利好次数'

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: title,
      left: 'center',
      textStyle: {
        color: props.darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        const d = items[items.length - 1 - idx]
        if (d) {
          result += `<b>利好: ${d.upCount} 利空: ${d.downCount} 合计: ${d.count}</b><br/>`
          result += '<span style="color:#888">点击查看按天趋势</span>'
        }
        return result
      }
    },
    legend: {
      data: [mainLabel, subLabel],
      top: 25,
      textStyle: { color: props.darkTheme ? '#ccc' : '#333' }
    },
    grid: { left: '3%', right: '8%', bottom: '3%', top: 55, containLabel: true },
    xAxis: {
      type: 'value', name: '次数',
      axisLabel: { color: props.darkTheme ? '#999' : '#666' },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
      splitLine: { lineStyle: { color: props.darkTheme ? '#333' : '#eee' } }
    },
    yAxis: {
      type: 'category',
      data: names,
      axisLabel: {
        color: props.darkTheme ? '#999' : '#666',
        fontSize: 11, width: 80, overflow: 'truncate'
      },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } }
    },
    series: [
      {
        name: mainLabel, type: 'bar', data: mainValues,
        itemStyle: { color: mainColor, borderRadius: [0, 4, 4, 0] },
        label: {
          show: true, position: 'right', color: props.darkTheme ? '#ccc' : '#333', fontSize: 10,
          formatter: function(params) { return params.value > 0 ? params.value : '' }
        }
      },
      {
        name: subLabel, type: 'bar', data: subValues,
        itemStyle: { color: subColor, opacity: 0.4 }
      }
    ]
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series') {
      const clickedName = names[params.dataIndex]
      if (clickedName) {
        emit('dimensionClick', dimension, clickedName)
      }
    }
  })
}
</script>

<template>
  <n-gi span="8">
    <div ref="bullBearStockUpRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="8">
    <div ref="bullBearIndustryUpRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="8">
    <div ref="bullBearConceptUpRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="8">
    <div ref="bullBearStockDownRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="8">
    <div ref="bullBearIndustryDownRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="8">
    <div ref="bullBearConceptDownRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
</template>
