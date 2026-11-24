<template>
  <div v-if="visible" class="pipeline-detail">
    <div class="detail-header">
      <n-text strong>{{ detail?.name || code }}</n-text>
      <n-button size="tiny" quaternary @click="emit('close')">收起</n-button>
    </div>
    <n-spin :show="loading">
      <n-alert v-if="error" type="error" :bordered="false" style="margin-bottom: 12px">
        {{ error }}
      </n-alert>

      <template v-if="detail">
        <!-- 三列详情 -->
        <n-grid :cols="3" :x-gap="16" :y-gap="12" style="margin-bottom: 16px">
          <!-- 技术面 -->
          <n-gi>
            <n-card size="small" title="技术面" class="detail-card">
              <template v-if="detail.tech">
                <div class="tech-facts">
                  <div class="fact-item">
                    <span class="fact-label">趋势</span>
                    <span class="fact-value">{{ detail.tech.trend || '数据不足' }}</span>
                  </div>
                  <div class="fact-item">
                    <span class="fact-label">量能</span>
                    <span class="fact-value">{{ detail.tech.volume || '数据不足' }}</span>
                  </div>
                  <div class="fact-item">
                    <span class="fact-label">位置</span>
                    <span class="fact-value">{{ detail.tech.position || '数据不足' }}</span>
                  </div>
                  <div class="fact-item">
                    <span class="fact-label">动能</span>
                    <span class="fact-value">{{ detail.tech.momentum || '数据不足' }}</span>
                  </div>
                </div>
                <n-divider style="margin: 8px 0" />
                <div v-for="sec in detail.tech.sections" :key="sec.title" class="tech-section">
                  <div class="section-title">{{ sec.title }}</div>
                  <div class="section-text">{{ sec.text }}</div>
                </div>
              </template>
              <n-empty v-else description="暂无技术数据" size="small" />
            </n-card>
          </n-gi>

          <!-- 因子面 -->
          <n-gi>
            <n-card size="small" title="因子面" class="detail-card">
              <template v-if="detail.factor">
                <div class="factor-stats">
                  <div class="stat-row">
                    <span class="stat-label">Kronos 得分</span>
                    <span class="stat-value">{{ detail.factor.factorScore?.toFixed(1) || '—' }}</span>
                  </div>
                  <div class="stat-row">
                    <span class="stat-label">权重</span>
                    <span class="stat-value">{{ detail.factor.weight?.toFixed(2) || '—' }}</span>
                  </div>
                  <div class="stat-row">
                    <span class="stat-label">贡献度</span>
                    <span class="stat-value">{{ detail.factor.contribution?.toFixed(1) || '—' }}</span>
                  </div>
                </div>
                <n-alert
                  v-if="detail.factor.degradedReason"
                  type="warning"
                  :bordered="false"
                  style="margin-top: 8px"
                >
                  {{ detail.factor.degradedReason }}
                </n-alert>
              </template>
              <n-empty v-else description="暂无因子数据" size="small" />
            </n-card>
          </n-gi>

          <!-- AI面 -->
          <n-gi>
            <n-card size="small" title="AI面" class="detail-card">
              <template v-if="detail.ai">
                <template v-if="detail.ai.forecast">
                  <div class="ai-forecast">
                    <div class="forecast-main" :class="forecastDirectionClass">
                      {{ forecastDirectionText }}
                      {{ detail.ai.forecast.changePct > 0 ? '+' : '' }}{{ detail.ai.forecast.changePct?.toFixed(1) }}%
                    </div>
                    <div class="forecast-details">
                      <div class="detail-row">
                        <span>预测期末</span>
                        <span>{{ detail.ai.forecast.predEnd?.toFixed(2) }}</span>
                      </div>
                      <div class="detail-row">
                        <span>区间</span>
                        <span>{{ detail.ai.forecast.predLow?.toFixed(2) }} ~ {{ detail.ai.forecast.predHigh?.toFixed(2) }}</span>
                      </div>
                      <div class="detail-row">
                        <span>置信度</span>
                        <span>{{ detail.ai.forecast.confidence?.toFixed(0) }}/100</span>
                      </div>
                      <div class="detail-row">
                        <span>预测天数</span>
                        <span>{{ detail.ai.forecast.predLen }} 天</span>
                      </div>
                    </div>
                  </div>
                </template>
                <n-alert
                  v-else-if="detail.ai.degraded"
                  type="warning"
                  :bordered="false"
                >
                  {{ detail.ai.degradedReason || 'AI 预测降级' }}
                </n-alert>
                <n-empty v-else description="暂无 AI 预测" size="small" />
              </template>
              <n-empty v-else description="暂无 AI 数据" size="small" />
            </n-card>
          </n-gi>
        </n-grid>

        <!-- K线技术图 -->
        <n-card size="small" title="K线技术图" class="kline-card">
          <KlineAnalysis
            v-if="detail.code"
            :code="detail.code"
            :stockName="detail.name"
            :embedded="true"
            :hideToolbar="true"
            :chartHeight="320"
          />
        </n-card>
      </template>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import KlineAnalysis from '../kline-analysis.vue'
import { fetchPipelineDetail } from '../../api/analysisCenter'
import type { analysis } from '../../../wailsjs/go/models'
import { errText } from './recommend'

const props = defineProps<{
  code: string
  signalDate: string
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const detail = ref<analysis.PipelineDetail | null>(null)
const loading = ref(false)
const error = ref('')

const forecastDirectionClass = computed(() => {
  if (!detail.value?.ai?.forecast) return ''
  return detail.value.ai.forecast.direction === 'up' ? 'direction-up' : 'direction-down'
})

const forecastDirectionText = computed(() => {
  if (!detail.value?.ai?.forecast) return ''
  return detail.value.ai.forecast.direction === 'up' ? '上涨' : '下跌'
})

// 代码直接传内部格式（sh600519 等），格式转换由 kline-analysis 内部完成

async function loadDetail() {
  if (!props.code || !props.visible) return
  loading.value = true
  error.value = ''
  try {
    detail.value = await fetchPipelineDetail(props.code, props.signalDate)
  } catch (e: unknown) {
    error.value = `加载详情失败：${errText(e)}`
  } finally {
    loading.value = false
  }
}

watch(() => [props.code, props.visible], () => {
  if (props.visible) {
    loadDetail()
  }
}, { immediate: true })
</script>

<style scoped>
.pipeline-detail {
  padding: 12px;
  background: var(--surface, #f8f9fa);
  border-radius: 8px;
  margin-top: 8px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.detail-card {
  min-height: 200px;
}

.tech-facts {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fact-item {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.fact-label {
  color: var(--muted, #666);
}

.fact-value {
  text-align: right;
  max-width: 65%;
}

.tech-section {
  margin-bottom: 8px;
}

.section-title {
  font-weight: 600;
  font-size: 12px;
  margin-bottom: 2px;
}

.section-text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted, #555);
}

.factor-stats {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
}

.stat-label {
  color: var(--muted, #666);
}

.stat-value {
  font-weight: 600;
}

.ai-forecast {
  text-align: center;
}

.forecast-main {
  font-size: 24px;
  font-weight: 700;
  margin-bottom: 12px;
}

.direction-up {
  color: #ef5350;
}

.direction-down {
  color: #26a69a;
}

.forecast-details {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted, #666);
}

.kline-card {
  margin-top: 8px;
}

.kline-card :deep(.kline-search-bar) {
  display: none;
}

.kline-card :deep(.kline-tech-card) {
  display: none;
}
</style>
