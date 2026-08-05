/**
 * Agent 流式对话：发送/中断/新对话、格式化轮询、agent-message 事件订阅（自 FloatingAgentAssistant.vue 原样搬迁）。
 * 状态与跨块函数经 ctx 传入（ref 共享引用 / 函数注入）。
 */
import { ref, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { EventsOff, EventsOn } from '../../../wailsjs/runtime'
import * as systemApi from '../../api/system'
import { formatMarkdown, parseStepText } from './markdownFormat'

const AGENT_EVENT = 'agent-message'

export function useAgentStream(ctx) {
  const {
    messages, inputValue, sessionId, message,
    aiConfigId, aiConfigOptions, sysPromptId, memoryMode, memoryCount,
    thinkingMode, agentMode, modelLabelForConfig,
    shareTipText, shareTipVisible,
    saveHistory, scrollToBottom,
    ensureLatestGroupExpanded, messageGroups, reasoningExpandedMap,
  } = ctx

  const isStreamLoad = ref(false)
  const sentFromFloating = ref(false)
  const isAborted = ref(false)
  let formatTimer = null

  function abortStream(showTip = true) {
    if (!isStreamLoad.value) return
    isAborted.value = true
    isStreamLoad.value = false
    stopFormatTimer()
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      if (last.rawContent) {
        const fmt = formatMarkdown(last.rawContent)
        last.content = fmt.content
        if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
      }
      if (last.rawReasoning) {
        const fmt = formatMarkdown(last.rawReasoning)
        last.reasoning = fmt.content
      }
    }
    if (showTip) {
      shareTipText.value = '已中断本次 AI 回答'
      shareTipVisible.value = true
    }
    systemApi.abortChatWithAgent()
  }

  function sendMessage() {
    if (isStreamLoad.value) {
      abortStream(false)
    }
    const text = inputValue.value.trim()
    if (!text) {
      message.warning('请输入你的问题')
      return
    }

    messages.value.push({
      role: 'user',
      content: text,
      time: new Date().toLocaleString(),
      modelName: '',
      reasoning: '',
      steps: []
    })
    const configId = aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
    const modelName = modelLabelForConfig(configId)
    messages.value.push({
      role: 'assistant',
      content: '',
      rawContent: '',
      time: new Date().toLocaleString(),
      modelName,
      reasoning: '',
      rawReasoning: '',
      steps: [],
      jsonMarkdown: ''
    })
    inputValue.value = ''
    isStreamLoad.value = true
    isAborted.value = false
    sentFromFloating.value = true
    startFormatTimer()
    saveHistory()
    nextTick(() => {
      ensureLatestGroupExpanded()
      const lastGroup = messageGroups.value[messageGroups.value.length - 1]
      if (lastGroup) {
        reasoningExpandedMap.value = {
          ...reasoningExpandedMap.value,
          [lastGroup.assistantIndex]: true,
          ['j-' + lastGroup.assistantIndex]: true
        }
      }
      scrollToBottom()
    })
    systemApi.chatWithAgent(text, configId, sysPromptId.value, memoryMode.value, memoryCount.value, thinkingMode.value, agentMode.value === 'auto' ? '' : agentMode.value)
  }

  function startNewChat() {
    if (isStreamLoad.value) {
      message.warning('当前有回答正在生成，请先中断或等待完成')
      return
    }
    messages.value = []
    sessionId.value = Date.now().toString()
  }

  function startFormatTimer() {
    stopFormatTimer()
    formatTimer = setInterval(() => {
      const last = messages.value[messages.value.length - 1]
      if (last && last.role === 'assistant') {
        if (last.rawContent) {
          const fmt = formatMarkdown(last.rawContent)
          last.content = fmt.content
          if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
        }
        if (last.rawReasoning) {
          const fmt = formatMarkdown(last.rawReasoning)
          last.reasoning = fmt.content
        }
      }
    }, 1500)
  }

  function stopFormatTimer() {
    if (formatTimer) {
      clearInterval(formatTimer)
      formatTimer = null
    }
  }

  function onAgentMessage(msg) {
    if (isAborted.value) return

    if (msg.content === 'agent-DONE' || (msg?.response_meta?.finish_reason === 'stop')) {
      isStreamLoad.value = false
      sentFromFloating.value = false
      isAborted.value = false
      stopFormatTimer()
      const last = messages.value[messages.value.length - 1]
      if (last && last.role === 'assistant') {
        if (last.rawContent) {
          const fmt = formatMarkdown(last.rawContent)
          last.content = fmt.content
          if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
        }
        if (last.rawReasoning) {
          const fmt = formatMarkdown(last.rawReasoning)
          last.reasoning = fmt.content
        }
      }
      saveHistory()
      nextTick(scrollToBottom)
      if (msg.content === 'agent-DONE' && last && last.role === 'assistant' && last.content) {
        const user = messages.value[messages.value.length - 2]
        systemApi.saveAIResponseResult("agent","市场分析", last.content, sessionId.value,user.content, aiConfigId.value)
      }
      return
    }

    const roleLower = String(msg?.role || '').toLowerCase()
    if (roleLower !== 'assistant') {
      return
    }

    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      if (msg?.reasoning_content) {
        const rc = msg.reasoning_content
        if (rc.startsWith('[STEP]')) {
          const stepText = rc.replace(/^\[STEP\]/, '').trim()
          if (stepText) {
            if (!last.steps) last.steps = []
            const parsed = parseStepText(stepText)
            last.steps.push(...parsed)
          }
        } else {
          last.rawReasoning = (last.rawReasoning || '') + rc
          last.reasoning = last.rawReasoning
        }
      }
      if (msg?.content) {
        last.rawContent = (last.rawContent || '') + msg.content
        last.content = last.rawContent
      }
      nextTick(scrollToBottom)
    }
  }

  onMounted(() => {
    EventsOn(AGENT_EVENT, onAgentMessage)
  })

  onBeforeUnmount(() => {
    EventsOff(AGENT_EVENT)
  })

  return {
    isStreamLoad, sentFromFloating, isAborted,
    abortStream, sendMessage, startNewChat,
  }
}
