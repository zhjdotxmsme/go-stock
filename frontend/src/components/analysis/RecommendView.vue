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

    <!-- 预测结果弹窗 -->
    <n-modal v-model:show="showForecastModal" preset="card" :title="forecastTitle" style="width: 480px">
      <n-spin :show="forecastLoading">
        <n-alert v-if="forecastError" type="error" :bordered="false">{{ forecastError }}</n-alert>
        <template v-else-if="forecast">
          <div class="forecast-summary">
            <div class="forecast-direction" :class="forecast.direction === 'up' ? 'dir-up' : 'dir-down'">
              {{ forecast.direction === 'up' ? '上涨' : '下跌' }}
              {{ forecast.changePct > 0 ? '+' : '' }}{{ forecast.changePct?.toFixed(1) }}%
            </div>
            <div class="forecast-grid">
              <div class="fg-row"><span>预测期末</span><span>{{ forecast.predEnd?.toFixed(2) }}</span></div>
              <div class="fg-row"><span>区间</span><span>{{ forecast.predLow?.toFixed(2) }} ~ {{ forecast.predHigh?.toFixed(2) }}</span></div>
              <div class="fg-row"><span>置信度</span><span>{{ forecast.confidence?.toFixed(0) }}/100</span></div>
              <div class="fg-row"><span>预测天数</span><span>{{ forecast.predLen }} 天</span></div>
            </div>
          </div>
          <n-divider style="margin: 12px 0" />
          <div class="forecast-bars">
            <div class="bars-title">预测 K 线（{{ forecastBars.length }} 根）</div>
            <div class="bars-list">
              <div v-for="bar in forecastBars" :key="bar.date" class="bar-row">
                <span class="bar-date">{{ bar.date }}</span>
                <span class="bar-ohlc">O:{{ bar.open?.toFixed(2) }} H:{{ bar.high?.toFixed(2) }} L:{{ bar.low?.toFixed(2) }} C:{{ bar.close?.toFixed(2) }}</span>
              </div>
            </div>
          </div>
        </template>
      </n-spin>
    </n-modal>
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
import type { data } from '../../../wailsjs/go/models'

defineEmits<{ (e: 'open-research-recommends'): void }>()

const rows = ref<RecommendRow[]>([])
const status = ref<AnalysisCenterStatus | null>(null)
const loading = ref(false)
const loadError = ref('')
const message = useMessage()
const pollTimers = new Map<string, number>()

// 预测结果弹窗
const showForecastModal = ref(false)
const forecastLoading = ref(false)
const forecastError = ref('')
const forecast = ref<data.KronosForecast | null>(null)
const forecastBars = ref<data.KronosBar[]>([])
const forecastTitle = ref('')

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
    loadError.value = `加载失败：${errText(e)}`
  } finally {
    loading.value = false
  }
}

async function expandForecast(row: RecommendRow) {
  forecastTitle.value = `${row.name} (${row.code}) 预测明细`
  showForecastModal.value = true
  forecastLoading.value = true
  forecastError.value = ''
  forecast.value = null
  forecastBars.value = []
  try {
    const raw = await fetchForecast(row.code, row.signalDate)
    if (!raw) {
      forecastError.value = '该候选暂无落库预测明细'
      return
    }
    const parsed: data.KronosPrediction = JSON.parse(raw)
    forecast.value = parsed.summary || null
    forecastBars.value = parsed.bars || []
  } catch (e: unknown) {
    forecastError.value = `解析预测明细失败：${errText(e)}`
  } finally {
    forecastLoading.value = false
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
      // 自动展示结果
      if (j.resultJson) {
        try {
          const parsed: data.KronosPrediction = JSON.parse(j.resultJson)
          forecastTitle.value = `${j.code} 预测结果`
          showForecastModal.value = true
          forecast.value = parsed.summary || null
          forecastBars.value = parsed.bars || []
        } catch {
          // 解析失败不弹窗
        }
      }
      return
    }
    if (j.status === 'failed') {
      message.error(`预测失败：${j.err || '未知错误'}`)
      return
    }
  } catch (e: unknown) {
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

<style scoped>
.forecast-summary {
  text-align: center;
}
.forecast-direction {
  font-size: 28px;
  font-weight: 700;
  margin-bottom: 12px;
}
.dir-up { color: #ef5350; }
.dir-down { color: #26a69a; }
.forecast-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.fg-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  color: var(--muted, #666);
}
.forecast-bars {
  margin-top: 8px;
}
.bars-title {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 6px;
}
.bars-list {
  max-height: 200px;
  overflow-y: auto;
}
.bar-row {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  padding: 2px 0;
  border-bottom: 1px solid var(--border, #eee);
}
.bar-date {
  color: var(--muted, #999);
}
.bar-ohlc {
  font-family: monospace;
}
</style>
