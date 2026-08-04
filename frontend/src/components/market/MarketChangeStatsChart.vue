<script setup>
import * as echarts from "echarts";
import { ref, watch } from "vue";

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: [Number, String], default: 350 },
  data: { type: Array, default: () => [] }
})

const emit = defineEmits(['dimensionClick'])

const changeStatsChartRef = ref(null)
const changeTypeChartRef = ref(null)

watch(() => props.data, (newData) => {
  if (newData && newData.length > 0) {
    renderChangeStatsChart(newData)
    renderChangeTypeChart(newData)
  }
}, { immediate: true })

function renderChangeStatsChart(data) {
  if (!changeStatsChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(changeStatsChartRef.value)

  const dates = data.map(d => d.changeDate)
  const totalCounts = data.map(d => d.totalCount)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const limitUps = data.map(d => d.limitUp)
  const limitDowns = data.map(d => d.limitDown)

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: '近30日异动统计趋势',
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
          result += `<span style="color:#666">封涨停: ${d.limitUp} 封跌停: ${d.limitDown}</span>`
        }
        return result
      }
    },
    legend: {
      data: ['上涨异动', '下跌异动', '封涨停', '封跌停', '总异动数'],
      top: 25,
      textStyle: { color: props.darkTheme ? '#ccc' : '#333' }
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: {
      type: 'category',
      data: dates,
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
        type: 'value', name: '总异动数', position: 'right',
        axisLabel: { color: props.darkTheme ? '#999' : '#666' },
        axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
        splitLine: { show: false }
      }
    ],
    series: [
      { name: '上涨异动', type: 'bar', stack: 'direction', data: upCounts, itemStyle: { color: '#ef4444' } },
      { name: '下跌异动', type: 'bar', stack: 'direction', data: downCounts, itemStyle: { color: '#22c55e' } },
      { name: '封涨停', type: 'bar', data: limitUps, itemStyle: { color: '#f97316' } },
      { name: '封跌停', type: 'bar', data: limitDowns, itemStyle: { color: '#06b6d4' } },
      {
        name: '总异动数', type: 'line', yAxisIndex: 1, data: totalCounts, smooth: true,
        lineStyle: { color: '#8b5cf6', width: 2 }, itemStyle: { color: '#8b5cf6' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(139, 92, 246, 0.3)' },
            { offset: 1, color: 'rgba(139, 92, 246, 0.05)' }
          ])
        }
      }
    ]
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series' && params.name) {
      emit('dimensionClick', 'date', params.name)
    }
  })
}

function renderChangeTypeChart(data) {
  if (!changeTypeChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(changeTypeChartRef.value)

  const dateSet = [...new Set(data.map(d => d.changeDate))].sort()

  const upTypes = ['封涨停板', '打开涨停板', '火箭发射', '快速反弹', '大笔买入', '有大买盘', '竞价上涨', '高开5日线', '向上缺口', '60日新高', '60日大幅上涨']
  const downTypes = ['封跌停板', '打开跌停板', '高台跳水', '加速下跌', '大笔卖出', '有大卖盘', '竞价下跌', '低开5日线', '向下缺口', '60日新低', '60日大幅下跌']

  const typeColorMap = {
    '封涨停板': '#ef4444',
    '封跌停板': '#22c55e',
    '打开涨停板': '#f97316',
    '打开跌停板': '#06b6d4',
    '火箭发射': '#dc2626',
    '快速反弹': '#f59e0b',
    '高台跳水': '#10b981',
    '加速下跌': '#14b8a6',
    '大笔买入': '#e11d48',
    '大笔卖出': '#059669',
    '有大买盘': '#db2777',
    '有大卖盘': '#0d9488',
    '竞价上涨': '#f43f5e',
    '竞价下跌': '#0891b2',
    '高开5日线': '#fb923c',
    '低开5日线': '#2dd4bf',
    '向上缺口': '#f87171',
    '向下缺口': '#34d399',
    '60日新高': '#c026d3',
    '60日新低': '#0ea5e9',
    '60日大幅上涨': '#a855f7',
    '60日大幅下跌': '#38bdf8',
  }

  const upSeries = upTypes.filter(typeName => data.some(d => d.typeName === typeName)).map(typeName => {
    const typeData = dateSet.map(date => {
      const found = data.find(d => d.changeDate === date && d.typeName === typeName)
      return found ? found.count : 0
    })
    return {
      name: typeName, type: 'bar', stack: 'up',
      emphasis: { focus: 'series' },
      data: typeData,
      itemStyle: { color: typeColorMap[typeName] || '#ef4444' }
    }
  })

  const downSeries = downTypes.filter(typeName => data.some(d => d.typeName === typeName)).map(typeName => {
    const typeData = dateSet.map(date => {
      const found = data.find(d => d.changeDate === date && d.typeName === typeName)
      return found ? found.count : 0
    })
    return {
      name: typeName, type: 'bar', stack: 'down',
      emphasis: { focus: 'series' },
      data: typeData,
      itemStyle: { color: typeColorMap[typeName] || '#22c55e' }
    }
  })

  const series = [...upSeries, ...downSeries]

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: '近30日异动类型分布(利好↑/利空↓)',
      left: 'center',
      textStyle: {
        color: props.darkTheme ? '#ccc' : '#333',
        fontSize: 14
      }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    legend: {
      type: 'scroll',
      top: 25,
      textStyle: { color: props.darkTheme ? '#ccc' : '#333', fontSize: 11 },
      pageIconColor: props.darkTheme ? '#aaa' : '#333',
      pageTextStyle: { color: props.darkTheme ? '#aaa' : '#333' }
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: {
      type: 'category',
      data: dateSet,
      axisLabel: { color: props.darkTheme ? '#999' : '#666', rotate: 45 },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } }
    },
    yAxis: {
      type: 'value', name: '次数',
      axisLabel: { color: props.darkTheme ? '#999' : '#666' },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
      splitLine: { lineStyle: { color: props.darkTheme ? '#333' : '#eee' } }
    },
    series: series
  }

  chart.setOption(option)
  chart.off('click')
  chart.on('click', function(params) {
    if (params.componentType === 'series' && params.seriesName) {
      emit('dimensionClick', 'type', params.seriesName)
    }
  })
}
</script>

<template>
  <n-gi span="12">
    <div ref="changeStatsChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
  <n-gi span="12">
    <div ref="changeTypeChartRef" style="width: 100%;height: auto;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}" ></div>
  </n-gi>
</template>
