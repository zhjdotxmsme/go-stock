<script setup>
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { NButton, NFlex, NModal, NSelect, NSpin, NTag, useMessage, useNotification } from 'naive-ui'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import { EventsOff, EventsOn } from '../../../wailsjs/runtime'
import * as tradeApi from '../../api/trade'
import * as systemApi from '../../api/system'

const props = defineProps({
  show: Boolean,
})
const emit = defineEmits(['update:show'])

const EVENT_NAME = 'tradingHoldingsSummary'

const message = useMessage()
const notify = useNotification()

const aiConfigs = ref([])
const aiConfigId = ref(null)
const content = ref('')
const modelName = ref('')
const generating = ref(false)
const hasToday = ref(false)
const scrollRef = ref(null)

function todayStr() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function loadAiConfigs() {
  if (aiConfigs.value.length > 0) return
  systemApi.getAiConfigs().then(({ data: res }) => {
    aiConfigs.value = res || []
    if (aiConfigs.value.length > 0) {
      aiConfigId.value = aiConfigs.value[0].ID
    }
  }).catch((e) => console.error('获取AI配置失败:', e))
}

function checkTodaySummary() {
  tradeApi.getHoldingsSummaryList(1, 20).then(({ data: res }) => {
    const today = (res?.list || []).find((i) => i.summaryDate === todayStr())
    if (!today) return
    return tradeApi.getHoldingsSummaryDetail(today.id).then(({ data: detail }) => {
      if (detail) {
        hasToday.value = true
        content.value = detail.content
        modelName.value = detail.modelName || ''
      }
    })
  }).catch((e) => console.error('查询当日总结失败:', e))
}

function scrollToBottom() {
  nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function onMsg(msg) {
  if (msg === 'DONE') {
    // DONE 在正常完成与被中断时都会收到：以库里当天是否有总结为准，
    // 有内容视为完成（后端已在发 DONE 前落库），无内容视为中断
    generating.value = false
    EventsOff(EVENT_NAME)
    if (content.value.trim()) {
      notify.success({ title: 'AI总结完成', content: '持仓每日总结已生成并保存', duration: 3000 })
    } else {
      message.info('AI总结已中断')
    }
    hasToday.value = false
    checkTodaySummary()
    return
  }
  if (!msg || typeof msg !== 'object') return
  if (typeof msg.content === 'string' && msg.content) {
    content.value += msg.content
  }
  if (typeof msg.extraContent === 'string' && msg.extraContent) {
    content.value += msg.extraContent
  }
  if (msg.model) {
    modelName.value = msg.model
  }
  scrollToBottom()
}

function generate() {
  if (!aiConfigId.value) {
    message.warning('请先选择AI模型服务配置（可在系统设置中添加）')
    return
  }
  content.value = ''
  hasToday.value = false
  modelName.value = ''
  generating.value = true
  EventsOff(EVENT_NAME)
  EventsOn(EVENT_NAME, onMsg)
  tradeApi.summarizeHoldings(aiConfigId.value, EVENT_NAME).catch((e) => {
    console.error('发起AI总结失败:', e)
  })
}

function abort() {
  tradeApi.abortSummarizeHoldings().catch(() => {})
}

function handleClose() {
  if (generating.value) {
    generating.value = false
    EventsOff(EVENT_NAME)
    abort()
  }
  emit('update:show', false)
}

watch(() => props.show, (v) => {
  if (v) {
    content.value = ''
    hasToday.value = false
    modelName.value = ''
    loadAiConfigs()
    checkTodaySummary()
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
    :title="'持仓AI每日总结 - ' + todayStr()"
    style="width: 880px; max-width: calc(100vw - 32px)"
    @update:show="handleClose"
  >
    <n-flex justify="space-between" align="center" style="margin-bottom: 10px">
      <n-select
        v-model:value="aiConfigId"
        label-field="name"
        value-field="ID"
        :options="aiConfigs"
        placeholder="请选择AI模型服务配置"
        style="width: 320px"
        :disabled="generating"
      />
      <n-flex align="center">
        <n-tag v-if="modelName" size="small" type="info" round>{{ modelName }}</n-tag>
        <n-button v-if="generating" type="warning" secondary @click="abort">中断</n-button>
        <n-button type="primary" :loading="generating" @click="generate">
          {{ hasToday ? '重新生成（覆盖今日）' : '开始生成' }}
        </n-button>
      </n-flex>
    </n-flex>

    <n-spin :show="generating" description="AI正在分析持仓，请稍候...">
      <div
        ref="scrollRef"
        style="max-height: calc(100vh - 320px); overflow-y: auto; text-align: left; border: 1px solid #e4e9f0; border-radius: 6px; padding: 4px 12px"
      >
        <div v-if="!content && !generating" style="color: #999; padding: 24px 0; text-align: center">
          今日还没有持仓总结，点击「开始生成」让 AI 分析你的全部持仓。
        </div>
        <MdPreview v-if="content" :model-value="content" theme="light" />
      </div>
    </n-spin>
  </n-modal>
</template>
