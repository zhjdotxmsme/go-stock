/**
 * 会话管理：历史加载/保存、面板开合（含 VIP 校验）、滚动到底（自 FloatingAgentAssistant.vue 原样搬迁）。
 * messages/scrollbarRef/message/initDefaultExpanded 经 ctx 传入。
 */
import { ref, nextTick, onMounted } from 'vue'
import * as systemApi from '../../api/system'

export function useChatSession(ctx) {
  const { messages, scrollbarRef, message, initDefaultExpanded } = ctx

  const sessionId = ref('')
  const panelVisible = ref(false)
  const vipLevel = ref(0)
  const vipLoaded = ref(false)
  const vipLoading = ref(false)

  async function loadHistory() {
    try {
      const resp = (await systemApi.getAiAssistantSession('')).data
      if (resp?.sessionId) {
        sessionId.value = resp.sessionId
      }
      const list = resp?.messages
      if (Array.isArray(list) && list.length > 0) {
        messages.value = list.map(m => ({
          role: m.role ?? '',
          content: m.content ?? '',
          time: m.time ?? '',
          modelName: m.modelName ?? '',
          reasoning: m.reasoning ?? '',
          steps: m.steps ?? [],
          jsonMarkdown: m.jsonMarkdown ?? ''
        }))
        nextTick(() => {
          initDefaultExpanded()
        })
      }
    } catch (_) {
    }
  }

  function saveHistory() {
    if (messages.value.length === 0) return
    const list = messages.value.map(m => ({
      role: m.role,
      content: m.content,
      time: m.time ?? '',
      modelName: m.modelName ?? '',
      reasoning: m.reasoning ?? '',
      steps: m.steps ?? [],
      jsonMarkdown: m.jsonMarkdown ?? ''
    }))
    systemApi.saveAiAssistantSession(sessionId.value, list).catch(() => {})
  }

  function openPanel() {
    panelVisible.value = true
    if (!sessionId.value) {
      sessionId.value = Date.now().toString()
    }
    if (messages.value.length === 0) {
      messages.value = [
        {
          role: 'assistant',
          content: '我是 go-stock AI Agent 助手，可以帮您分析股票、查询市场数据、获取研究报告等。请问有什么可以帮您的？',
          time: new Date().toLocaleString(),
          modelName: '',
          reasoning: ''
        }
      ]
    }
    nextTick(() => {
      initDefaultExpanded()
      scrollToBottom()
    })
  }

  function closePanel() {
    panelVisible.value = false
  }

  async function ensureVipInfo() {
    if (vipLoaded.value || vipLoading.value) return
    vipLoading.value = true
    try {
      const res = (await systemApi.getSponsorInfo()).data
      const lvl = Number(res?.vipLevel ?? 0)
      vipLevel.value = Number.isNaN(lvl) ? 0 : lvl
    } catch (_) {
      vipLevel.value = 0
    } finally {
      vipLoaded.value = true
      vipLoading.value = false
    }
  }

  async function togglePanel() {
    if (!panelVisible.value) {
      await ensureVipInfo()
      if ((vipLevel.value ?? 0) < 2) {
        message.warning('go-stock AI Agent 助手功能仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
        return
      }
      openPanel()
    } else {
      closePanel()
    }
  }

  function scrollToBottom() {
    nextTick(() => {
      scrollbarRef.value?.scrollTo({ top: 99999, behavior: 'smooth' })
    })
  }

  onMounted(() => {
    loadHistory()
  })

  return {
    sessionId, panelVisible, vipLevel,
    loadHistory, saveHistory, openPanel, closePanel, togglePanel, scrollToBottom,
  }
}
