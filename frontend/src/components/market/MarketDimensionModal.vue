<script setup>
import * as echarts from "echarts";
import { ref, watch, nextTick } from "vue";

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: [Number, String], default: 350 },
  show: { type: Boolean, default: false },
  dimension: { type: String, default: '' },
  dimensionName: { type: String, default: '' },
  title: { type: String, default: '' },
  data: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:show'])

const dimensionDetailChartRef = ref(null)

watch(() => [props.show, props.data], ([newShow, newData]) => {
  if (newShow && newData && newData.length > 0) {
    nextTick(() => {
      renderChart()
    })
  }
}, { immediate: true })

function renderChart() {
  if (!dimensionDetailChartRef.value) return

  const chart = echarts.init(dimensionDetailChartRef.value)

  if (props.dimension === 'date') {
    renderDateTypeChart(chart)
  } else {
    renderDimensionDetailChart(chart)
  }
}

function renderDimensionDetailChart(chart) {
  const data = props.data
  const dates = data.map(d => d.changeDate)
  const upCounts = data.map(d => d.upCount)
  const downCounts = data.map(d => d.downCount)
  const totalCounts = data.map(d => d.totalCount)

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: props.title,
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
        return result
      }
    },
    legend: {
      data: ['利好异动', '利空异动', '总异动数'],
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
        type: 'value', name: '次数', position: 'left',
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
      {
        name: '利好异动', type: 'bar', stack: 'direction', data: upCounts,
        itemStyle: { color: '#ef4444' }
      },
      {
        name: '利空异动', type: 'bar', stack: 'direction', data: downCounts,
        itemStyle: { color: '#22c55e' }
      },
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
}

function renderDateTypeChart(chart) {
  const data = props.data
  const typeNames = data.map(d => d.typeName).reverse()
  const upValues = data.map(d => d.upCount).reverse()
  const downValues = data.map(d => d.downCount).reverse()

  const option = {
    darkMode: props.darkTheme,
    title: {
      text: props.title,
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
        result += '<b>合计: ' + total + '</b>'
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
      type: 'value', name: '次数',
      axisLabel: { color: props.darkTheme ? '#999' : '#666' },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } },
      splitLine: { lineStyle: { color: props.darkTheme ? '#333' : '#eee' } }
    },
    yAxis: {
      type: 'category',
      data: typeNames,
      axisLabel: {
        color: props.darkTheme ? '#999' : '#666',
        fontSize: 11, width: 100, overflow: 'truncate'
      },
      axisLine: { lineStyle: { color: props.darkTheme ? '#444' : '#ccc' } }
    },
    series: [
      {
        name: '利好异动', type: 'bar', stack: 'total', data: upValues,
        itemStyle: { color: '#ef4444' }
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
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="title"
    style="width: 800px;max-width: calc(100vw - 32px);"
    :mask-closable="true"
    @update:show="emit('update:show', $event)"
  >
    <div ref="dimensionDetailChartRef" style="width: 100%;height: 450px;--wails-draggable:no-drag"></div>
  </n-modal>
</template>
