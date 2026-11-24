<template>
  <div class="smart-pipeline">
    <!-- 顶部工具栏 -->
    <n-space align="center" style="margin-bottom: 12px">
      <n-text strong style="font-size: 15px">智能流水线</n-text>
      <n-date-picker
        v-model:value="selectedDate"
        type="date"
        placeholder="选择日期"
        style="width: 150px"
        @update:value="onDateChange"
      />
      <n-button
        type="primary"
        :loading="running"
        :disabled="running"
        @click="startPipeline"
      >
        <template #icon><n-icon><PlayOutline /></n-icon></template>
        {{ running ? '分析中...' : '一键开始分析' }}
      </n-button>
      <n-button size="small" :loading="loading" @click="refresh">刷新</n-button>
      <n-tag v-if="status" size="small" :type="statusTagType">{{ status.message }}</n-tag>
    </n-space>

    <!-- 步骤条 -->
    <n-steps
      v-if="showSteps"
      :current="currentStep"
      size="small"
      style="margin-bottom: 16px"
    >
      <n-step
        v-for="step in steps"
        :key="step.key"
        :title="step.label"
        :description="step.description"
      />
    </n-steps>

    <!-- 错误提示 -->
    <n-alert v-if="error" type="error" :bordered="false" style="margin-bottom: 12px">
      {{ error }}
    </n-alert>

    <!-- 结果表格 -->
    <PipelineTable
      :rows="rows"
      :loading="loading"
      :expanded-code="expandedCode"
      @expand="onExpand"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { NIcon, useMessage } from 'naive-ui'
import { PlayOutline } from '@vicons/ionicons5'
import { format } from 'date-fns'
import PipelineTable from './PipelineTable.vue'
import {
  runPipeline,
  fetchPipelineResult,
  fetchPipelineStatus,
} from '../../api/analysisCenter'
import type { analysis } from '../../../wailsjs/go/models'
import type { RecommendRow } from './recommend'
import { errText } from './recommend'

const message = useMessage()

// ===== 状态 =====
const selectedDate = ref<number | null>(Date.now())
const running = ref(false)
const loading = ref(false)
const error = ref('')
const rows = ref<RecommendRow[]>([])
const steps = ref<analysis.PipelineStep[]>([])
const status = ref<analysis.PipelineStatus | null>(null)
const expandedCode = ref('')
let pollTimer: number | null = null

// ===== 计算属性 =====
const showSteps = computed(() => running.value || steps.value.length > 0)

const currentStep = computed(() => {
  if (!running.value) return steps.value.length
  // 运行中：根据状态推断当前步骤
  if (status.value?.step === 'data') return 1
  if (status.value?.step === 'scan') return 2
  if (status.value?.step === 'kronos') return 3
  if (status.value?.step === 'predict') return 4
  return 0
})

const statusTagType = computed(() => {
  if (!status.value) return 'default'
  if (status.value.error) return 'error'
  if (running.value) return 'warning'
  return 'success'
})

// ===== 方法 =====
function tradeDateStr(): string {
  return selectedDate.value ? format(new Date(selectedDate.value), 'yyyy-MM-dd') : ''
}

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    const result = await fetchPipelineResult(tradeDateStr(), 30)
    rows.value = result.rows || []
    steps.value = result.steps || []
    status.value = result.status
  } catch (e: unknown) {
    error.value = `加载失败：${errText(e)}`
  } finally {
    loading.value = false
  }
}

async function startPipeline() {
  if (running.value) return
  running.value = true
  error.value = ''
  try {
    // 后台异步执行：立即返回，进度与完成经 GetPipelineStatus 轮询获得
    await runPipeline(tradeDateStr())
  } catch (e: unknown) {
    running.value = false
    error.value = `启动失败：${errText(e)}`
    message.error(error.value)
    return
  }
  pollUntilDone()
}

function pollUntilDone() {
  pollTimer = window.setTimeout(async () => {
    if (!running.value) return // 组件已卸载或运行被终止
    try {
      status.value = await fetchPipelineStatus()
    } catch {
      // 忽略单次轮询错误，继续下一轮
    }
    if (!running.value) return
    if (status.value?.running) {
      pollUntilDone()
      return
    }
    running.value = false
    if (status.value?.error) {
      error.value = `运行失败：${status.value.error}`
      message.error(error.value)
    } else {
      message.success('流水线运行完成')
    }
    await refresh()
  }, 2000)
}

function onDateChange() {
  refresh()
}

function onExpand(code: string) {
  expandedCode.value = expandedCode.value === code ? '' : code
}

// ===== 生命周期 =====
onMounted(async () => {
  await refresh()
  // 切回本页时若有流水线正在跑（后端 khunter 状态），接管轮询直到完成
  if (status.value?.running) {
    running.value = true
    pollUntilDone()
  }
})

onUnmounted(() => {
  running.value = false
  if (pollTimer) window.clearTimeout(pollTimer)
})
</script>

<style scoped>
.smart-pipeline {
  padding: 8px 0;
}
</style>
