/**
 * AI 智能体会话 Store —— 会话与流式的唯一真源。
 *
 * 设计要点（见 .openteams/specs/2026-09-10-agent-chat-ux-design.html 第 4/5 节）：
 * 1. `agent-message` 事件在本 store 内**只注册一次且永不注销**。
 *    旧实现里两个宿主各自 EventsOn/EventsOff，而 Wails 的 EventsOff(name) 是按事件名
 *    清空所有监听者（delete(listeners, name)）—— 打开过 /agent 页再离开会把侧边浮窗的
 *    监听一起干掉。集中到 store 后该缺陷在结构上不可能再发生。
 * 2. 纯逻辑放 components/agent/agentStreamCore.js，本文件只管状态、编排与副作用。
 * 3. UI 能力（消息提示、滚动容器）由宿主通过 registerNotifier / registerScroller 注入，
 *    因此 store 不依赖 naive-ui，也不关心自己跑在页面还是浮窗里。
 */
import { computed, nextTick, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { EventsOn } from '../../wailsjs/runtime'
import * as systemApi from '../api/system'
import {
  applyChunkToAssistant,
  currentStepSummary,
  finalizeAssistantMessage,
  isDoneChunk,
  lastAssistantContent,
  newAssistantMessage,
  newUserMessage,
} from '../components/agent/agentStreamCore.js'
import { loadPrefs, savePrefs } from '../components/agent/agentPrefs.js'

/** 流式无响应看门狗：超过该时长没有收到任何 chunk 即判定超时 */
const STREAM_TIMEOUT_MS = 120000

/** 默认系统提示词为空时的兜底会话数 */
const MEMORY_COUNT_OPTIONS = [
  { label: '1 条', value: 1 },
  { label: '2 条', value: 2 },
  { label: '3 条', value: 3 },
  { label: '4 条', value: 4 },
  { label: '5 条', value: 5 },
  { label: '10 条', value: 10 },
]

const AGENT_MODE_OPTIONS = [
  { label: '🤖 自动选择', value: 'auto' },
  { label: '⚡ 快速模式', value: 'react' },
  { label: '🧠 规划模式', value: 'plan_execute' },
]

const GREETING =
  '我是 go-stock AI Agent 助手，可以帮您分析股票、查询市场数据、获取研究报告等。请问有什么可以帮您的？'

export const useAgentStore = defineStore('agent', () => {
  // ---------------------------------------------------------------- 状态
  const messages = ref([])
  const sessionId = ref('')
  const sessions = ref([])
  const inputValue = ref('')
  const panelVisible = ref(false)

  const isStreamLoad = ref(false)
  const isAborted = ref(false)
  /** 已中断的助手消息索引，用于气泡角标（不持久化） */
  const abortedIndexes = ref(new Set())

  const shareTipVisible = ref(false)
  const shareTipText = ref('')

  // 顶部轻提示（模型/模式切换时的建议文案）
  const hintVisible = ref(false)
  const hintText = ref('')
  let hintTimer = null
  function showHint(text) {
    hintText.value = text
    hintVisible.value = true
    if (hintTimer) clearTimeout(hintTimer)
    hintTimer = setTimeout(() => {
      hintVisible.value = false
    }, 3000)
  }

  // 首选项（持久化在 localStorage: agent.prefs）
  const prefs = loadPrefs()
  const aiConfigOptions = ref([])
  const aiConfigId = ref(prefs.aiConfigId)
  const sysPromptTemplates = ref([])
  const sysPromptId = ref(prefs.sysPromptId)
  const userPromptTemplates = ref([])
  const userPromptId = ref(prefs.userPromptId)
  const thinkingMode = ref(prefs.thinkingMode)
  const memoryMode = ref(prefs.memoryMode)
  const memoryCount = ref(prefs.memoryCount)
  const agentMode = ref(prefs.agentMode)
  const memoryCountOptions = MEMORY_COUNT_OPTIONS
  const agentModeOptions = AGENT_MODE_OPTIONS

  const sysPromptOptions = computed(() =>
    sysPromptTemplates.value.map((t) => ({ label: t.name ?? '', value: t.ID ?? t.id }))
  )
  const userPromptOptions = computed(() =>
    userPromptTemplates.value.map((t) => ({ label: t.name ?? '', value: t.ID ?? t.id }))
  )

  // 展开状态
  const expandedGroups = ref(new Set())
  const reasoningExpandedMap = ref({})

  // ------------------------------------------------- 依赖注入（宿主提供）
  let notifier = null
  const scrollers = new Set()

  /** 注册消息提示能力（naive-ui useMessage）；未注册时降级到 console，绝不静默 */
  function registerNotifier(api) {
    notifier = api
  }
  function registerScroller(fn) {
    if (typeof fn !== 'function') return () => {}
    scrollers.add(fn)
    return () => scrollers.delete(fn)
  }

  function warn(text) {
    if (notifier?.warning) notifier.warning(text)
    else console.warn('[agent]', text)
  }
  function fail(text) {
    if (notifier?.error) notifier.error(text)
    else console.error('[agent]', text)
  }
  function ok(text) {
    if (notifier?.success) notifier.success(text)
    else console.info('[agent]', text)
  }

  function showTip(text) {
    shareTipText.value = text
    shareTipVisible.value = true
  }

  function scrollToBottom() {
    nextTick(() => {
      scrollers.forEach((fn) => {
        try {
          fn()
        } catch (e) {
          console.warn('[agent] scroller failed', e)
        }
      })
    })
  }

  // ---------------------------------------------------------------- 派生
  const messageGroups = computed(() => {
    const groups = []
    let current = null
    for (let i = 0; i < messages.value.length; i++) {
      const msg = messages.value[i]
      if (msg.role === 'user') {
        if (current) groups.push(current)
        current = { id: i, userMsg: msg, userIndex: i, assistantMsg: null, assistantIndex: -1 }
      } else if (msg.role === 'assistant' && current) {
        current.assistantMsg = msg
        current.assistantIndex = i
      }
    }
    if (current) groups.push(current)
    return groups
  })

  const latestSteps = computed(() => {
    const last = messages.value[messages.value.length - 1]
    return currentStepSummary(last?.steps)
  })

  function isGroupExpanded(groupIndex) {
    return expandedGroups.value.has(groupIndex)
  }
  function toggleGroup(groupIndex) {
    const next = new Set(expandedGroups.value)
    if (next.has(groupIndex)) next.delete(groupIndex)
    else next.add(groupIndex)
    expandedGroups.value = next
  }
  function toggleReasoning(index) {
    reasoningExpandedMap.value = {
      ...reasoningExpandedMap.value,
      [index]: !reasoningExpandedMap.value[index],
    }
  }
  /** 首次进入时展开最后一组 */
  function initDefaultExpanded() {
    if (messageGroups.value.length > 0 && expandedGroups.value.size === 0) {
      expandedGroups.value = new Set([messageGroups.value.length - 1])
    }
  }
  /** 新一轮提问时确保该组展开 */
  function ensureLatestGroupExpanded() {
    if (messageGroups.value.length === 0) return
    const last = messageGroups.value.length - 1
    const next = new Set(expandedGroups.value)
    next.add(last)
    expandedGroups.value = next
  }

  /** 某条消息是否被中断（气泡角标用） */
  function isMessageAborted(index) {
    return abortedIndexes.value.has(index)
  }

  // ------------------------------------------------------ 首选项读写
  function persistPrefs() {
    savePrefs({
      aiConfigId: aiConfigId.value,
      sysPromptId: sysPromptId.value,
      userPromptId: userPromptId.value,
      thinkingMode: thinkingMode.value,
      memoryMode: memoryMode.value,
      memoryCount: memoryCount.value,
      agentMode: agentMode.value,
    })
  }

  function modelLabelForConfig(configId) {
    const opts = aiConfigOptions.value
    if (!opts?.length) return ''
    const id = configId != null ? Number(configId) : Number(opts[0].value)
    const found = opts.find((o) => Number(o.value) === id)
    return found?.label != null ? String(found.label) : ''
  }

  // ------------------------------------------------------ 选项加载
  let optionsLoaded = false
  async function loadOptions() {
    if (optionsLoaded) return
    const { data, error } = await systemApi.getAiConfigs()
    if (error) {
      fail('AI 模型配置加载失败：' + (error.message || error))
      return
    }
    const list = Array.isArray(data) ? data : []
    aiConfigOptions.value = list.map((c, index) => {
      const id = c.ID != null ? Number(c.ID) : c.id != null ? Number(c.id) : index
      const name = c.name ?? c.Name ?? ''
      const modelName = c.modelName ?? c.ModelName ?? ''
      return { label: name + (modelName ? ' [' + modelName + ']' : ''), value: id }
    })
    optionsLoaded = true
    if (!aiConfigOptions.value.length) return
    const wanted = aiConfigId.value != null ? Number(aiConfigId.value) : null
    const valid = wanted != null && aiConfigOptions.value.some((o) => o.value === wanted)
    aiConfigId.value = valid ? wanted : aiConfigOptions.value[0].value
    persistPrefs()
  }

  async function loadPromptTemplates() {
    const { data, error } = await systemApi.getPromptTemplates('', '')
    if (error) {
      fail('提示词模板加载失败：' + (error.message || error))
      return
    }
    const list = Array.isArray(data) ? data : []
    sysPromptTemplates.value = list.filter((t) => t.type === '模型系统Prompt')
    userPromptTemplates.value = list.filter((t) => t.type === '模型用户Prompt')
  }

  function onUserPromptChange(id) {
    if (!id) return
    const t = userPromptTemplates.value.find((x) => (x.ID ?? x.id) === id)
    if (t?.content) inputValue.value = t.content
  }

  // ------------------------------------------------------ 会话读写
  function mapHistoryMessage(m) {
    return {
      role: m.role ?? '',
      content: m.content ?? '',
      time: m.time ?? '',
      modelName: m.modelName ?? '',
      reasoning: m.reasoning ?? '',
      rawReasoning: '',
      rawContent: '',
      steps: Array.isArray(m.steps) ? m.steps : [],
      jsonMarkdown: m.jsonMarkdown ?? '',
    }
  }

  async function loadHistory(id) {
    const { data, error } = await systemApi.getAiAssistantSession(id ?? '')
    if (error) {
      // 历史读不到不应阻塞对话，但必须让用户知道
      warn('会话历史加载失败：' + (error.message || error))
      return
    }
    const resp = data
    if (resp?.sessionId) sessionId.value = resp.sessionId
    const list = resp?.messages
    if (Array.isArray(list) && list.length > 0) {
      messages.value = list.map(mapHistoryMessage)
      nextTick(initDefaultExpanded)
    }
  }

  async function loadSessions(limit = 50) {
    const { data, error } = await systemApi.listAiAssistantSessions(limit)
    if (error) {
      warn('会话列表加载失败：' + (error.message || error))
      return
    }
    sessions.value = Array.isArray(data) ? data : []
  }

  async function switchSession(id) {
    if (!id || id === sessionId.value) return
    if (isStreamLoad.value) {
      warn('当前回答正在生成，请先中断或等待完成')
      return
    }
    sessionId.value = id
    messages.value = []
    expandedGroups.value = new Set()
    reasoningExpandedMap.value = {}
    abortedIndexes.value = new Set()
    await loadHistory(id)
    scrollToBottom()
  }

  async function deleteSession(id) {
    if (!id) return
    const { error } = await systemApi.deleteAiAssistantSession(id)
    if (error) {
      fail('删除会话失败：' + (error.message || error))
      return
    }
    ok('会话已删除')
    sessions.value = sessions.value.filter((s) => s.sessionId !== id)
    if (id === sessionId.value) {
      messages.value = []
      sessionId.value = Date.now().toString()
      ensureGreeting()
    }
  }

  function saveHistory() {
    if (messages.value.length === 0) return
    const list = messages.value.map((m) => ({
      role: m.role,
      content: m.content,
      time: m.time ?? '',
      modelName: m.modelName ?? '',
      reasoning: m.reasoning ?? '',
      steps: Array.isArray(m.steps) ? m.steps : [],
      jsonMarkdown: m.jsonMarkdown ?? '',
    }))
    // 不再静默吞错：失败时明确提示（旧实现是 .catch(() => {})）
    systemApi.saveAiAssistantSession(sessionId.value, list).then(({ error }) => {
      if (error) {
        showTip('本次会话未能保存：' + (error.message || error))
      } else {
        loadSessions()
      }
    })
  }

  function ensureGreeting() {
    if (messages.value.length > 0) return
    messages.value = [
      { role: 'assistant', content: GREETING, time: new Date().toLocaleString(), modelName: '', reasoning: '', steps: [] },
    ]
  }

  function startNewChat() {
    if (isStreamLoad.value) {
      warn('当前有回答正在生成，请先中断或等待完成')
      return
    }
    messages.value = []
    sessionId.value = Date.now().toString()
    expandedGroups.value = new Set()
    reasoningExpandedMap.value = {}
    abortedIndexes.value = new Set()
    ensureGreeting()
  }

  function openPanel() {
    panelVisible.value = true
    if (!sessionId.value) sessionId.value = Date.now().toString()
    ensureGreeting()
    loadSessions()
    nextTick(() => {
      initDefaultExpanded()
      scrollToBottom()
    })
  }
  function closePanel() {
    panelVisible.value = false
  }
  function togglePanel() {
    if (panelVisible.value) closePanel()
    else openPanel()
  }

  // ------------------------------------------------------ 流式与中断
  let formatTimer = null
  let streamWatchdog = null

  function startFormatTimer() {
    stopFormatTimer()
    // 流式过程中周期性把 raw 文本规整为 Markdown（沿用原有 1.5s 轮询）
    formatTimer = setInterval(() => {
      const last = messages.value[messages.value.length - 1]
      if (last && last.role === 'assistant') finalizeAssistantMessage(last)
    }, 1500)
  }
  function stopFormatTimer() {
    if (formatTimer) {
      clearInterval(formatTimer)
      formatTimer = null
    }
  }
  function armWatchdog() {
    clearWatchdog()
    streamWatchdog = setTimeout(() => {
      streamWatchdog = null
      if (!isStreamLoad.value) return
      isAborted.value = true
      isStreamLoad.value = false
      stopFormatTimer()
      fail('AI 长时间无响应，已结束本次回答')
      systemApi.abortChatWithAgent()
    }, STREAM_TIMEOUT_MS)
  }
  function clearWatchdog() {
    if (streamWatchdog) {
      clearTimeout(streamWatchdog)
      streamWatchdog = null
    }
  }

  function finalizeLast() {
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') finalizeAssistantMessage(last)
  }

  /** 中断：真正取消后端上下文，并让迟到 chunk 失效 */
  function abortStream(showTipMessage = true) {
    if (!isStreamLoad.value) return
    isAborted.value = true
    isStreamLoad.value = false
    stopFormatTimer()
    clearWatchdog()
    finalizeLast()
    const lastIndex = messages.value.length - 1
    if (messages.value[lastIndex]?.role === 'assistant') {
      const next = new Set(abortedIndexes.value)
      next.add(lastIndex)
      abortedIndexes.value = next
    }
    if (showTipMessage) showTip('已中断本次 AI 回答')
    systemApi.abortChatWithAgent().then(({ error }) => {
      if (error) fail('中断请求失败：' + (error.message || error))
    })
  }

  function sendMessage() {
    // 流式中再次发送 = 中断并发送（与按钮/回车语义一致）
    if (isStreamLoad.value) abortStream(false)

    const text = inputValue.value.trim()
    if (!text) {
      warn('请输入你的问题')
      return
    }

    const configId = aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
    messages.value.push(newUserMessage(text))
    messages.value.push(newAssistantMessage(modelLabelForConfig(configId)))
    inputValue.value = ''
    isStreamLoad.value = true
    isAborted.value = false
    startFormatTimer()
    armWatchdog()
    saveHistory()
    nextTick(() => {
      ensureLatestGroupExpanded()
      const lastGroup = messageGroups.value[messageGroups.value.length - 1]
      if (lastGroup) {
        reasoningExpandedMap.value = {
          ...reasoningExpandedMap.value,
          [lastGroup.assistantIndex]: true,
          ['j-' + lastGroup.assistantIndex]: true,
        }
      }
      scrollToBottom()
    })
    systemApi
      .chatWithAgent(
        text,
        configId,
        sysPromptId.value,
        memoryMode.value,
        memoryCount.value,
        thinkingMode.value,
        agentMode.value === 'auto' ? '' : agentMode.value,
        sessionId.value // 第 8 个参数：旧实现缺失，导致记忆无法按会话隔离
      )
      .then(({ error }) => {
        if (error) {
          isStreamLoad.value = false
          stopFormatTimer()
          clearWatchdog()
          fail('发送失败：' + (error.message || error))
        }
      })
  }

  /** 后端流式事件入口 */
  function onAgentMessage(msg) {
    // 中断后迟到的 chunk 一律丢弃（旧实现会继续往最后一条消息里追加）
    if (isAborted.value) return

    if (isDoneChunk(msg)) {
      isStreamLoad.value = false
      stopFormatTimer()
      clearWatchdog()
      finalizeLast()
      saveHistory()
      nextTick(scrollToBottom)
      const last = messages.value[messages.value.length - 1]
      if (msg.content === 'agent-DONE' && last?.role === 'assistant' && last.content) {
        const user = messages.value[messages.value.length - 2]
        systemApi.saveAIResponseResult('agent', '市场分析', last.content, sessionId.value, user?.content ?? '', aiConfigId.value)
      }
      return
    }

    if (String(msg?.role || '').toLowerCase() !== 'assistant') return
    const last = messages.value[messages.value.length - 1]
    if (!last || last.role !== 'assistant') return
    if (applyChunkToAssistant(last, msg)) {
      armWatchdog() // 收到数据即重置看门狗
      nextTick(scrollToBottom)
    }
  }

  // -------------------------------------------- 事件订阅（全局单例，永不注销）
  let subscribed = false
  function ensureSubscribed() {
    if (subscribed) return
    subscribed = true
    EventsOn('agent-message', onAgentMessage)
  }

  /** 宿主挂载时调用：注册依赖 + 拉取选项 + 恢复会话。幂等，先到者执行。 */
  let bootstrapped = false
  async function bootstrap() {
    ensureSubscribed()
    if (bootstrapped) return
    bootstrapped = true
    await loadOptions()
    await loadHistory(sessionId.value)
    loadSessions()
  }

  // 首选项变更即持久化（模型/提示词/思考/记忆/模式）
  watch(
    [aiConfigId, sysPromptId, userPromptId, thinkingMode, memoryMode, memoryCount, agentMode],
    () => persistPrefs()
  )

  // 模式切换建议（沿用原 useAgentOptions 的提示语义）
  watch(agentMode, (val) => {
    if (val === 'react') showHint('⚡ 快速模式推荐使用DeepSeek最新版')
    else if (val === 'plan_execute') showHint('🧠 规划模式推荐使用GLM最新版')
  })

  // 换模型时按模型特性自动调整模式/思考，并给出提示（沿用原行为）
  watch(aiConfigId, (val) => {
    const label = modelLabelForConfig(val).toLowerCase()
    const labelCompact = label.replace(/[\s_-]/g, '')
    if (label.includes('deepseek-chat')) {
      agentMode.value = 'plan_execute'
      thinkingMode.value = false
      showHint('deepseek-chat 已使用规划模式并关闭思考模式')
    } else if (label.includes('deepseek')) {
      showHint('⚡ DeepSeek模型推荐使用快速模式')
    } else if (labelCompact.includes('glm5.1')) {
      agentMode.value = 'plan_execute'
      thinkingMode.value = true
      showHint('GLM 5.1 已使用规划模式并开启思考模式')
    } else if (label.includes('glm')) {
      showHint('🧠 GLM模型推荐使用规划模式')
    }
  })

  return {
    // 状态
    messages, sessionId, sessions, inputValue, panelVisible,
    isStreamLoad, isAborted, shareTipVisible, shareTipText,
    hintVisible, hintText, showHint,
    // 首选项
    aiConfigOptions, aiConfigId, sysPromptId, userPromptId,
    thinkingMode, memoryMode, memoryCount, agentMode,
    memoryCountOptions, agentModeOptions, sysPromptOptions, userPromptOptions,
    // 派生
    messageGroups, latestSteps,
    // 展开状态（宿主需要透传给 MessageBubble；漏导出会让气泡渲染直接抛错）
    expandedGroups, reasoningExpandedMap,
    isGroupExpanded, toggleGroup, toggleReasoning, initDefaultExpanded, ensureLatestGroupExpanded,
    isMessageAborted, modelLabelForConfig,
    // 依赖注入
    registerNotifier, registerScroller,
    // 会话
    bootstrap, loadOptions, loadPromptTemplates, onUserPromptChange,
    loadHistory, loadSessions, switchSession, deleteSession,
    saveHistory, startNewChat, openPanel, closePanel, togglePanel,
    // 流式
    sendMessage, abortStream, scrollToBottom,
    // 供分享模块复用
    lastAssistantContent: () => lastAssistantContent(messages.value),
  }
})
