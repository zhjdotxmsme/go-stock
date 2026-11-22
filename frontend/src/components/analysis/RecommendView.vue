<template>
  <div>
    <n-space align="center" style="margin-bottom: 8px">
      <n-text>推荐视图（观察候选 × Kronos 因子混分）</n-text>
      <n-tag v-if="status" size="small" :type="kronosTagType(status)">{{ status.kronosStatus }}</n-tag>
      <n-tag v-if="status && !status.factorEnabled" size="small" type="default">因子未启用</n-tag>
      <n-tag v-if="status" size="small">待确认 {{ status.pendingObservations }}</n-tag>
      <n-button size="small" :loading="loading" @click="refresh">刷新</n-button>
      <n-button text size="small" @click="$emit('open-research-recommends')">AI 推荐列表 →</n-button>
    </n-space>
    <n-alert v-if="loadError" type="error" :bordered="false" style="margin-bottom: 8px">{{ loadError }}</n-alert>
    <n-data-table :columns="columns" :data="rows" :loading="loading" size="small" />
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, onUnmounted, ref } from 'vue'
import { NButton, NTag, NText, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  fetchRecommendations, fetchAnalysisStatus, fetchForecast, submitPredictJob, fetchPredictJob,
} from '../../api/analysisCenter'
import { rowSubtitle, kronosTagType, errText, type RecommendRow, type AnalysisCenterStatus } from './recommend'

defineEmits<{ (e: 'open-research-recommends'): void }>()

const rows = ref<RecommendRow[]>([])
const status = ref<AnalysisCenterStatus | null>(null)
const loading = ref(false)
const loadError = ref('')
const message = useMessage()
const pollTimers = new Map<string, number>()

const columns: DataTableColumns<RecommendRow> = [
  { title: '代码', key: 'code', width: 96 },
  { title: '名称', key: 'name', width: 96 },
  { title: '信号日', key: 'signalDate', width: 110 },
  { title: '状态', key: 'status', width: 80 },
  { title: '评分', key: 'score', width: 72 },
  {
    title: 'Kronos', key: 'factor', width: 130,
    render(row) {
      if (row.hasFactor) {
        return h(NTag, { size: 'tiny', type: 'info' },
          { default: () => `${row.factorScore} × ${row.factorWeight}` })
      }
      const sub = rowSubtitle(row)
      return sub ? h(NText, { depth: 3 }, { default: () => sub }) : h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: '综合', key: 'blendedScore', width: 80,
    sorter: (a, b) => a.blendedScore - b.blendedScore, defaultSortOrder: 'descend' as const,
  },
  {
    title: '操作', key: 'op', width: 150,
    render(row) {
      return h('span', [
        h(NButton, { size: 'tiny', onClick: () => runPredict(row.code) },
          { default: () => 'AI预测' }),
        row.hasFactor
          ? h(NButton, {
              size: 'tiny', text: true, style: 'margin-left:8px',
              onClick: () => expandForecast(row),
            }, { default: () => '明细' })
          : null,
      ])
    },
  },
]

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    rows.value = await fetchRecommendations()
    status.value = await fetchAnalysisStatus()
  } catch (e: unknown) {
    // 门面返回 error 时必须显式报错，不能把空列表显示成"今天没候选"
    loadError.value = `加载失败：${errText(e)}`
  } finally {
    loading.value = false
  }
}

async function expandForecast(row: RecommendRow) {
  try {
    const raw = await fetchForecast(row.code, row.signalDate)
    if (!raw) {
      message.warning('该候选暂无落库预测明细')
      return
    }
    // 明细用 modal/抽屉展示预测曲线；最小实现为 JSON 预览
    message.info(`${row.code} 预测明细已加载（${raw.length} 字节）`)
  } catch (e: unknown) {
    message.error(`读取明细失败：${errText(e)}`)
  }
}

async function runPredict(code: string) {
  try {
    const job = await submitPredictJob(code, 5, '')
    message.info('预测任务已提交（模型冷启动需数分钟），请稍后查看')
    pollJob(job.id)
  } catch (e: unknown) {
    message.error(`提交失败：${errText(e)}`)
  }
}

async function pollJob(id: string) {
  try {
    const j = await fetchPredictJob(id)
    if (j.status === 'done') {
      message.success(`${j.code} 预测完成`)
      return
    }
    if (j.status === 'failed') {
      message.error(`预测失败：${j.err || '未知错误'}`)
      return
    }
  } catch (e: unknown) {
    // 未知 ID / 任务过期：停止轮询并提示，绝不无限循环
    message.error(`预测任务已失效：${errText(e)}`)
    return
  }
  const t = window.setTimeout(() => {
    pollTimers.delete(id)
    pollJob(id)
  }, 2000)
  pollTimers.set(id, t)
}

onMounted(refresh)
onUnmounted(() => {
  for (const t of pollTimers.values()) window.clearTimeout(t)
  pollTimers.clear()
})
</script>
