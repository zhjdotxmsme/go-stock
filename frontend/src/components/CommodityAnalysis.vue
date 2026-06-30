<script setup>
import {ref, onMounted} from 'vue'
import {
  GetCommodityRegistry,
  GetCommodityTechnicals,
  GetCommodityFundamentals,
  GetCommodityCorrelation,
  GetCommodityReport,
} from '../../wailsjs/go/main/App'

const registry = ref([])
const selectedCode = ref('XAUUSD')
const selectedModes = ref(['technical'])
const period = ref('day')
const result = ref('')
const loading = ref(false)
const secondaryCodes = ref('XAGUSD,USCL')

const modeOptions = [
  {label: '技术面', value: 'technical'},
  {label: '基本面', value: 'fundamental'},
  {label: '关联分析', value: 'correlation'},
]

const periodOptions = [
  {label: '日线', value: 'day'},
  {label: '周线', value: 'week'},
]

async function loadRegistry() {
  registry.value = await GetCommodityRegistry()
}

function assetOptions() {
  return registry.value.map(item => ({label: item.name, value: item.code}))
}

async function runAnalysis() {
  if (selectedModes.value.length === 0) return
  loading.value = true
  result.value = ''
  try {
    const parts = []
    for (const mode of selectedModes.value) {
      if (mode === 'technical') {
        const r = await GetCommodityTechnicals(selectedCode.value, period.value)
        parts.push('## 技术面分析\n' + r)
      } else if (mode === 'fundamental') {
        const r = await GetCommodityFundamentals(selectedCode.value)
        parts.push('## 基本面分析\n' + r)
      } else if (mode === 'correlation') {
        const r = await GetCommodityCorrelation(selectedCode.value, secondaryCodes.value)
        parts.push('## 关联分析\n' + r)
      }
    }
    if (selectedModes.value.length > 1) {
      const report = await GetCommodityReport(selectedCode.value, '周报')
      parts.push('## 综合报告\n' + report)
    }
    result.value = parts.join('\n\n')
  } catch (e) {
    result.value = '分析失败: ' + (e.message || e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadRegistry()
})
</script>

<template>
  <n-space vertical>
    <n-space align="center">
      <n-select v-model:value="selectedCode" :options="assetOptions()" style="width: 160px"/>
      <n-select v-model:value="period" :options="periodOptions" style="width: 120px"/>
      <n-checkbox-group v-model:value="selectedModes" :options="modeOptions"/>
      <n-button type="primary" :loading="loading" @click="runAnalysis">AI 分析</n-button>
    </n-space>

    <n-space v-if="selectedModes.includes('correlation')" align="center">
      <n-text depth="3">关联品种（逗号分隔）:</n-text>
      <n-input v-model:value="secondaryCodes" style="width: 240px"/>
    </n-space>

    <n-spin :show="loading">
      <n-card v-if="result" size="small">
        <div class="whitespace-pre-wrap">{{ result }}</div>
      </n-card>
    </n-spin>
  </n-space>
</template>
