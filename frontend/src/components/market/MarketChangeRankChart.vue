<script setup>
import * as echarts from "echarts";
import { ref, watch } from "vue";

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: [Number, String], default: 350 },
  topStocks: { type: Array, default: () => [] },
  topIndustries: { type: Array, default: () => [] },
  topConcepts: { type: Array, default: () => [] },
  periodLabel: { type: String, default: '当日' }
})

const emit = defineEmits(['dimensionClick'])

const changeRankStockRef = ref(null)
const changeRankIndustryRef = ref(null)
const changeRankConceptRef = ref(null)

watch(() => [props.topStocks, props.topIndustries, props.topConcepts], () => {
  renderAllCharts()
}, { immediate: true, deep: true })

function renderAllCharts() {
  if (props.topStocks && props.topStocks.length > 0) {
    renderRankChart(changeRankStockRef, `${props.periodLabel}异动次数最多的股票`, props.topStocks, 'stock')
  }
  if (props.topIndustries && props.topIndustries.length > 0) {
    renderRankChart(changeRankIndustryRef, `${props.periodLabel}异动次数最多的行业`, props.topIndustries, 'industry')
  }
  if (props.topConcepts && props.topConcepts.length > 0) {
    renderRankChart(changeRankConceptRef, `${props.periodLabel}异动次数最多的概念`, props.topConcepts, 'concept')
  }
}

function renderRankChart(chartRef, title, items, dimension) {
  if (!chartRef.value || !items || items.length === 0) return

  const chart = echarts.init(chartRef.value)

  const names = items.map(d => d.name).reverse()
  const upValues = items.map(d => d.upCount).reverse()
  const downValues = items.map(d => d.downCount).reverse()

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
        let total = 0
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
          total += param.value
        })
        result += '<b>合计: ' + total + '</b><br/><span style="color:#888">点击查看按天趋势</span>'
        return result
      }
    },
    legend: {
      data: ['利好异动', '利空异动'],
      top: 25,
      textStyle: { color: props.darkTheme ? '#ccc' : '#333' }
    },
    grid: { left: '3%', right: '8%', bottom: '3%', top: 55, containLabel: true },
    xAxis: {
      type: 'value', name: '异动次数',
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
        name: '利好异动', type: 'bar', stack: 'total', data: upValues,
        itemStyle: { color: '#ef4444', borderRadius: [0, 0, 0, 0] },
        label: {
          show: true, position: 'insideRight', color: '#fff', fontSize: 10,
          formatter: function(params) { return params.value > 0 ? params.value : '' }
        }
      },
      {
        name: '利空异动', type: 'bar', stack: 'total', data: downValues,
        itemStyle: { color: '#22c55e', borderRadius: [0, 4, 4, 0] },
        label: {
          show: true, position: 'right', color: props.darkTheme ? '#ccc' : '#333', fontSize: 10,
          formatter: function(params) {
            const total = upValues[params.dataIndex] + downValues[params.dataIndex]
            return total > 0 ? total : ''
          }
        }
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
    <div ref="changeRankConceptRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="12">
    <div ref="changeRankStockRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="12">
    <div ref="changeRankIndustryRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
</template>
