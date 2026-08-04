<script setup>
import * as echarts from "echarts";
import { ref, watch } from "vue";

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: [Number, String], default: 350 },
  data: { type: Array, default: () => [] }
})

const chartRef = ref(null)
const limitChartRef = ref(null)

watch(() => props.data, (newData) => {
  if (newData && newData.length > 0) {
    renderUpDownChart(newData)
    renderLimitChart(newData)
  }
}, { immediate: true })

function renderUpDownChart(data) {
  if (!chartRef.value || !data || data.length === 0) return

  const chart = echarts.init(chartRef.value)

  const times = data.map(d => d.dataTime)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const ratios = data.map(d => d.upRatio.toFixed(2))
  const upDownRatios = data.map(d => d.upDownRatio.toFixed(2))

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: '涨跌家数比',
      left: 'center',
      textStyle: {
        color: props.darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">红盘率: ${d.upRatio.toFixed(1)}%</span><br/>`
          result += `<span style="color:#666">情绪指标: ${d.upDownRatio.toFixed(2)} (${d.sentimentDesc || ''})</span>`
        }
        return result
      }
    },
    legend: {
      data: ['上涨家数', '下跌家数', '红盘率(%)', '情绪指标'],
      top: 25,
      textStyle: { color: props.darkTheme ? '#ccc' : '#333' }
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: { color: props.darkTheme ? '#999' : '#666', rotate: 45 },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } }
    },
    yAxis: [
      {
        type: 'value', name: '家数', position: 'left',
        axisLabel: { color: props.darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
        splitLine: { lineStyle: { color: props.darkTheme ? '#333' : '#eee' } }
      },
      {
        type: 'value', name: '红盘率(%)', position: 'right', min: 0, max: 100,
        axisLabel: { color: props.darkTheme ? '#999' : '#666', formatter: '{value}%' },
        axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
        splitLine: { show: false }
      },
      {
        type: 'value', name: '情绪指标', position: 'right', offset: 60,
        axisLabel: { color: props.darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
        splitLine: { show: false }
      }
    ],
    series: [
      { name: '上涨家数', type: 'bar', stack: 'total', data: upCounts, itemStyle: { color: '#ef4444' } },
      { name: '下跌家数', type: 'bar', stack: 'total', data: downCounts, itemStyle: { color: '#22c55e' } },
      {
        name: '红盘率(%)', type: 'line', yAxisIndex: 1, data: ratios, smooth: true,
        lineStyle: { color: '#f59e0b', width: 2 }, itemStyle: { color: '#f59e0b' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.3)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.05)' }
          ])
        },
        markLine: {
          silent: true,
          data: [{ yAxis: 50, name: '平衡线', lineStyle: { color: '#888', type: 'dashed' } }]
        }
      },
      {
        name: '情绪指标', type: 'line', yAxisIndex: 2, data: upDownRatios, smooth: true,
        lineStyle: { color: '#8b5cf6', width: 2 }, itemStyle: { color: '#8b5cf6' },
        markLine: {
          silent: true,
          data: [
            { yAxis: 1, name: '平衡线', lineStyle: { color: '#8b5cf6', type: 'dashed' } },
            { yAxis: 2, name: '极强线', lineStyle: { color: '#ef4444', type: 'dotted' } },
            { yAxis: 0.5, name: '冰点线', lineStyle: { color: '#22c55e', type: 'dotted' } }
          ]
        }
      }
    ]
  }

  chart.setOption(option)
}

function renderLimitChart(data) {
  if (!limitChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(limitChartRef.value)

  const times = data.map(d => d.dataTime)
  const limitUps = data.map(d => d.limitUp)
  const limitDowns = data.map(d => d.limitDown)
  const ratios = data.map(d => d.limitRatio.toFixed(2))

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: '涨跌停家数比',
      left: 'center',
      textStyle: {
        color: props.darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">涨跌停比: ${d.limitRatio.toFixed(2)}</span><br/>`
        }
        return result
      }
    },
    legend: {
      data: ['涨停家数', '跌停家数', '涨跌停比'],
      top: 25,
      textStyle: { color: props.darkTheme ? '#ccc' : '#333' }
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: { color: props.darkTheme ? '#999' : '#666', rotate: 45 },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } }
    },
    yAxis: [
      {
        type: 'value', name: '家数', position: 'left',
        axisLabel: { color: props.darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
        splitLine: { lineStyle: { color: props.darkTheme ? '#333' : '#eee' } }
      },
      {
        type: 'value', name: '涨跌停比', position: 'right',
        axisLabel: { color: props.darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
        splitLine: { show: false }
      }
    ],
    series: [
      { name: '涨停家数', type: 'bar', stack: 'total', data: limitUps, itemStyle: { color: '#ef4444' } },
      { name: '跌停家数', type: 'bar', stack: 'total', data: limitDowns, itemStyle: { color: '#22c55e' } },
      {
        name: '涨跌停比', type: 'line', yAxisIndex: 1, data: ratios, smooth: true,
        lineStyle: { color: '#f59e0b', width: 2 }, itemStyle: { color: '#f59e0b' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(245, 158, 11, 0.3)' },
            { offset: 1, color: 'rgba(245, 158, 11, 0.05)' }
          ])
        },
        markLine: {
          silent: true,
          data: [{ yAxis: 1, name: '平衡线', lineStyle: { color: '#888', type: 'dashed' } }]
        }
      }
    ]
  }

  chart.setOption(option)
}
</script>

<template>
  <n-gi span="8">
    <div ref="chartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="8">
    <div ref="limitChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
</template>
