<script setup>
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { NButton, NFlex, NModal, NSelect, NSpin, NTag, NTooltip, useMessage } from 'naive-ui'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import { EventsOff, EventsOn } from '../../../wailsjs/runtime'
import * as tradeApi from '../../api/trade'
import * as systemApi from '../../api/system'

const props = defineProps({
  show: Boolean,
})
const emit = defineEmits(['update:show'])

const EVENT_NAME = 'holdingsDeepAnalysis'

const message = useMessage()

const aiConfigs = ref([])
const aiConfigId = ref(null)
const deepData = ref([])
const holdingsSignals = ref({})
const loadingData = ref(false)
const generating = ref(false)
const aiContent = ref('')
const scrollRef = ref(null)

// 信号方向 → 标签颜色（红涨绿跌：多头红、空头绿）
const signalTagType = (d) => ({ bullish: 'error', bearish: 'success', warning: 'warning' })[d] || 'info'

function loadHoldingsSignals() {
  holdingsSignals.value = {}
  tradeApi.getHoldingsSignals().then(({ data: res }) => {
    holdingsSignals.value = res || {}
  }).catch((e) => console.error('获取持仓信号失败:', e))
}

const fmt = (v) => (typeof v === 'number' && isFinite(v) ? v.toFixed(2) : '-')
const fmtPct = (v) => (typeof v === 'number' && isFinite(v) ? (v >= 0 ? '+' : '') + v.toFixed(2) + '%' : '-')

function loadAiConfigs() {
  if (aiConfigs.value.length > 0) return
  systemApi.getAiConfigs().then(({ data: res }) => {
    aiConfigs.value = res || []
    if (aiConfigs.value.length > 0) {
      aiConfigId.value = aiConfigs.value[0].ID
    }
  }).catch((e) => console.error('获取AI配置失败:', e))
}

function loadDeepData() {
  loadingData.value = true
  deepData.value = []
  tradeApi.getHoldingsDeepData().then(({ data: res, error }) => {
    if (!res) throw new Error(error?.message || '获取持仓深度数据失败')
    deepData.value = res || []
  }).catch((e) => {
    console.error('获取持仓深度数据失败:', e)
    message.error(e?.message || '获取持仓深度数据失败')
  }).finally(() => {
    loadingData.value = false
  })
}

function scrollToBottom() {
  nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function onMsg(msg) {
  if (msg === 'DONE') {
    generating.value = false
    EventsOff(EVENT_NAME)
    if (!aiContent.value.trim()) message.info('AI 深度分析未生成内容')
    return
  }
  if (!msg || typeof msg !== 'object') return
  if (typeof msg.content === 'string' && msg.content) {
    aiContent.value += msg.content
  }
  if (typeof msg.extraContent === 'string' && msg.extraContent) {
    aiContent.value += msg.extraContent
  }
  scrollToBottom()
}

function generate() {
  if (!deepData.value.length) {
    message.warning('暂无持仓深度数据，请先在交易日志中添加买入记录')
    return
  }
  if (!aiConfigId.value) {
    message.warning('请先选择AI模型服务配置（可在系统设置中添加）')
    return
  }
  aiContent.value = ''
  generating.value = true
  EventsOff(EVENT_NAME)
  EventsOn(EVENT_NAME, onMsg)
  tradeApi.analyzeHoldingsDeep(aiConfigId.value, EVENT_NAME).catch((e) => {
    generating.value = false
    console.error('发起AI深度分析失败:', e)
  })
}

function handleClose() {
  if (generating.value) {
    generating.value = false
    EventsOff(EVENT_NAME)
  }
  emit('update:show', false)
}

watch(() => props.show, (v) => {
  if (v) {
    aiContent.value = ''
    loadAiConfigs()
    loadDeepData()
    loadHoldingsSignals()
  } else {
    handleClose()
  }
})

onUnmounted(() => {
  EventsOff(EVENT_NAME)
})
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    title="持仓深度分析"
    style="width: 960px; max-width: calc(100vw - 32px)"
    @update:show="handleClose"
  >
    <n-flex justify="space-between" align="center" style="margin-bottom: 10px">
      <n-select
        v-model:value="aiConfigId"
        label-field="name"
        value-field="ID"
        :options="aiConfigs"
        placeholder="请选择AI模型服务配置"
        style="width: 300px"
        :disabled="generating"
      />
      <n-button type="primary" :loading="generating" @click="generate">AI 综合分析</n-button>
    </n-flex>

    <n-spin :show="loadingData">
      <!-- 每只持仓的本地多维数据卡片 -->
      <div v-if="!deepData.length && !loadingData" style="color: #999; padding: 24px 0; text-align: center">
        暂无持仓数据。请先在交易日志中添加买入记录。
      </div>
      <div
        v-for="s in deepData"
        :key="s.stockCode"
        style="border: 1px solid #e4e9f0; border-radius: 8px; padding: 10px 14px; margin-bottom: 10px; text-align: left"
      >
        <n-flex justify="space-between" align="center">
          <strong>{{ s.stockName }}（{{ s.stockCode }}）</strong>
          <n-flex align="center" :size="6">
            <n-tag size="small" :type="(s.changePercent ?? 0) >= 0 ? 'error' : 'success'" round>
              现价 {{ fmt(s.currentPrice) }}（{{ fmtPct(s.changePercent) }}）
            </n-tag>
            <n-tag size="small" :type="(s.profitPercent ?? 0) >= 0 ? 'error' : 'success'" round>
              持仓 {{ fmtPct(s.profitPercent) }}
            </n-tag>
            <n-tag size="small" type="info" round>仓位 {{ fmt(s.positionPct) }}%</n-tag>
          </n-flex>
        </n-flex>

        <!-- 信号徽章：本地信号引擎判定，悬停查看说明 -->
        <n-flex v-if="(holdingsSignals[s.stockCode] || []).length" align="center" :size="4" style="margin-top: 6px">
          <span style="font-size: 12px; color: #999">信号：</span>
          <n-tooltip v-for="sig in holdingsSignals[s.stockCode]" :key="sig.key" trigger="hover">
            <template #trigger>
              <n-tag size="small" :type="signalTagType(sig.direction)" :bordered="false" style="cursor: help">
                {{ sig.name }}
              </n-tag>
            </template>
            <span style="display: inline-block; max-width: 260px">{{ sig.category }}｜{{ sig.tip }}</span>
          </n-tooltip>
        </n-flex>

        <div style="margin-top: 8px; font-size: 13px; line-height: 1.8">
          <div v-if="s.levels">
            <strong>压力位：</strong><span style="color: #d03050">{{ fmt(s.levels.resistance?.[0]?.price) }}</span>
            <span v-if="s.levels.resistance?.[1]">、{{ fmt(s.levels.resistance[1].price) }}</span>
            <span style="color: #999">（{{ s.levels.resistance?.[0]?.label }}）</span>
            ｜ <strong>支撑位：</strong><span style="color: #18a058">{{ fmt(s.levels.support?.[0]?.price) }}</span>
            <span v-if="s.levels.support?.[1]">、{{ fmt(s.levels.support[1].price) }}</span>
            <span style="color: #999">（{{ s.levels.support?.[0]?.label }}）</span>
          </div>
          <div v-if="s.levels">
            <strong>买点参考：</strong>{{ (s.levels.buyPoints || []).map(b => `${fmt(b.price)}（${b.label}）`).join('；') || '-' }}
          </div>
          <div v-if="s.levels">
            <strong>卖出/止损：</strong>{{ (s.levels.sellPoints || []).map(b => `${fmt(b.price)}（${b.label}）`).join('；') || '-' }}
          </div>
          <div v-if="s.indicators">
            <strong>技术面：</strong>趋势{{ s.indicators.trend }}；MACD {{ s.indicators.macdSignal }}；RSI14 {{ fmt(s.indicators.rsiValue) }}（{{ s.indicators.rsiStatus }}）；KDJ {{ s.indicators.kdjSignal }}；布林{{ s.indicators.bollStatus }}
            <span v-if="s.indicators.gapStatus">；{{ s.indicators.gapStatus }}</span>
          </div>
          <div><strong>资金面：</strong>{{ s.capitalSummary || '-' }}</div>
          <div v-if="s.kronos?.summary" style="color: #7c5cd6">
            <strong>Kronos预测：</strong>未来{{ s.kronos.summary.predLen }}日
            <span :style="{ color: s.kronos.summary.direction === 'up' ? '#d03050' : '#18a058' }">
              {{ s.kronos.summary.direction === 'up' ? '上涨' : '下跌' }} {{ fmtPct(s.kronos.summary.changePct) }}
            </span>
            ｜期末 {{ fmt(s.kronos.summary.predEnd) }}｜区间 {{ fmt(s.kronos.summary.predLow) }} ~ {{ fmt(s.kronos.summary.predHigh) }}
            ｜一致度 {{ fmt(s.kronos.summary.confidence) }}/100
            <span style="color: #999">（模型推演，非投资建议）</span>
          </div>
          <div v-if="s.industryFlow || s.industry">
            <strong>板块：</strong>{{ s.industryFlow || '所属行业 ' + s.industry }}
          </div>
          <div v-if="s.news && s.news.length">
            <strong>最近新闻：</strong>
            <div v-for="(n, i) in s.news" :key="i" style="color: #666">· {{ n.title }}（{{ n.time }} {{ n.source }}）</div>
          </div>
        </div>
      </div>
    </n-spin>

    <div style="margin-top: 6px">
      <n-spin :show="generating" description="AI 正在结合技术、资金、板块与新闻生成综合分析...">
        <div
          ref="scrollRef"
          style="max-height: calc(100vh - 520px); min-height: 120px; overflow-y: auto; text-align: left; border: 1px solid #e4e9f0; border-radius: 6px; padding: 4px 12px"
        >
          <div v-if="!aiContent && !generating" style="color: #999; padding: 16px 0; text-align: center">
            点击「AI 综合分析」，AI 将基于上方多维数据逐股深度解读并给出组合综合结论。
          </div>
          <MdPreview v-if="aiContent" :model-value="aiContent" theme="light" />
        </div>
      </n-spin>
    </div>
  </n-modal>
</template>
