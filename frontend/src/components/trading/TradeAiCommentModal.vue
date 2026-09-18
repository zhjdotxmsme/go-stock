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
  // 当前行（交易日志记录），用于展示标题、回填已有点评
  record: { type: Object, default: null },
})
const emit = defineEmits(['update:show', 'saved'])

const EVENT_NAME = 'tradingRecordAiComment'

const message = useMessage()
const notify = useNotification()

const aiConfigs = ref([])
const aiConfigId = ref(null)
const content = ref('')
const modelName = ref('')
const generating = ref(false)
const scrollRef = ref(null)

function loadAiConfigs() {
  if (aiConfigs.value.length > 0) return
  systemApi.getAiConfigs().then(({ data: res }) => {
    aiConfigs.value = res || []
    if (aiConfigs.value.length > 0) {
      aiConfigId.value = aiConfigs.value[0].ID
    }
  }).catch((e) => console.error('获取AI配置失败:', e))
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
    if (content.value.trim()) {
      notify.success({ title: 'AI点评完成', content: '单笔交易点评已生成并保存', duration: 3000 })
      emit('saved')
    } else {
      message.info('AI点评未生成内容')
    }
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
  if (!props.record || !props.record.ID) {
    message.warning('缺少交易日志记录，无法生成点评')
    return
  }
  if (!aiConfigId.value) {
    message.warning('请先选择AI模型服务配置（可在系统设置中添加）')
    return
  }
  content.value = ''
  modelName.value = ''
  generating.value = true
  EventsOff(EVENT_NAME)
  EventsOn(EVENT_NAME, onMsg)
  tradeApi.generateTradeAiComment(props.record.ID, aiConfigId.value, EVENT_NAME).catch((e) => {
    generating.value = false
    console.error('发起AI点评失败:', e)
  })
}

function handleClose() {
  if (generating.value) {
    // 后端流继续生成并落库，前端只是不再接收
    generating.value = false
    EventsOff(EVENT_NAME)
  }
  emit('update:show', false)
}

watch(() => props.show, (v) => {
  if (v) {
    content.value = String(props.record?.AiComment ?? props.record?.aiComment ?? '')
    modelName.value = ''
    loadAiConfigs()
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
    :title="'单笔交易 AI 点评 - ' + (record?.StockName || record?.stockName || '') + ' (' + (record?.StockCode || record?.stockCode || '') + ')'"
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
        <n-button type="primary" :loading="generating" @click="generate">
          {{ content ? '重新生成' : '生成点评' }}
        </n-button>
      </n-flex>
    </n-flex>

    <n-spin :show="generating" description="AI 正在结合行情、AI 推荐与技术指标点评该笔交易...">
      <div
        ref="scrollRef"
        style="max-height: calc(100vh - 320px); overflow-y: auto; text-align: left; border: 1px solid #e4e9f0; border-radius: 6px; padding: 4px 12px"
      >
        <div v-if="!content && !generating" style="color: #999; padding: 24px 0; text-align: center">
          该笔交易还没有 AI 点评，点击「生成点评」，AI 会结合交易数据、最近一次 AI 推荐与最新技术指标给出点评。
        </div>
        <MdPreview v-if="content" :model-value="content" theme="light" />
      </div>
    </n-spin>
  </n-modal>
</template>
