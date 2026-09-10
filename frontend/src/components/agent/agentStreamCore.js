/**
 * Agent 流式消息的纯逻辑：不依赖 Vue / Pinia / 网络 / DOM，node 可直接单测。
 *
 * 约定：状态变更「就地写入传入的普通对象」，由 stores/agent.ts 负责把这些对象
 * 放进响应式容器。这样核心逻辑可以在 node 里用普通对象断言，不必起 Vue 环境。
 */
import { formatMarkdown, parseStepText } from './markdownFormat.js'

/** 执行步骤前缀，与后端 backend/handler/agent_handler.go 的约定一致 */
export const STEP_PREFIX = '[STEP]'

/** 后端在流结束时额外补发的结束标记 */
export const DONE_CONTENT = 'agent-DONE'

/** 新建一条用户消息 */
export function newUserMessage(text, time = new Date().toLocaleString()) {
  return {
    role: 'user',
    content: text,
    time,
    modelName: '',
    reasoning: '',
    steps: [],
  }
}

/** 新建一条待填充的助手消息（raw* 为流式原始累积，content/reasoning 为格式化结果） */
export function newAssistantMessage(modelName = '', time = new Date().toLocaleString()) {
  return {
    role: 'assistant',
    content: '',
    rawContent: '',
    time,
    modelName,
    reasoning: '',
    rawReasoning: '',
    steps: [],
    jsonMarkdown: '',
  }
}

/**
 * 是否为「流结束」chunk。
 * 后端两种收尾都会出现：`content === 'agent-DONE'`，或 `response_meta.finish_reason === 'stop'`。
 */
export function isDoneChunk(msg) {
  if (!msg) return false
  if (msg.content === DONE_CONTENT) return true
  return msg?.response_meta?.finish_reason === 'stop'
}

/**
 * 把一条流式 chunk 累加到最后一条助手消息上。
 * - reasoning_content 以 [STEP] 开头 → 解析为执行步骤
 * - 其余 reasoning_content → 思维链
 * - content → 正文（结束标记除外，避免混进正文）
 * @returns {boolean} 是否产生了变化
 */
export function applyChunkToAssistant(target, msg) {
  if (!target || !msg) return false
  let changed = false

  const rc = msg.reasoning_content
  if (rc) {
    if (rc.startsWith(STEP_PREFIX)) {
      const stepText = rc.slice(STEP_PREFIX.length).trim()
      if (stepText) {
        if (!Array.isArray(target.steps)) target.steps = []
        target.steps.push(...parseStepText(stepText))
        changed = true
      }
    } else {
      target.rawReasoning = (target.rawReasoning || '') + rc
      target.reasoning = target.rawReasoning
      changed = true
    }
  }

  // 结束标记由 isDoneChunk 处理，这里再兜一层，保证核心单独使用也不会污染正文
  if (msg.content && msg.content !== DONE_CONTENT) {
    target.rawContent = (target.rawContent || '') + msg.content
    target.content = target.rawContent
    changed = true
  }

  return changed
}

/**
 * 流式收尾：把累积的原始文本做一次 Markdown 规整，
 * 并把从 JSON 响应体里提取出的「分析报告」写入 jsonMarkdown。
 */
export function finalizeAssistantMessage(target) {
  if (!target) return
  if (target.rawContent) {
    const fmt = formatMarkdown(target.rawContent)
    target.content = fmt.content
    if (fmt.jsonMarkdown) target.jsonMarkdown = fmt.jsonMarkdown
  }
  if (target.rawReasoning) {
    target.reasoning = formatMarkdown(target.rawReasoning).content
  }
}

/**
 * 气泡顶部「当前步骤」摘要：取最后一条步骤。
 * 折叠状态下也能看出 AI 正在做什么。
 */
export function currentStepSummary(steps) {
  if (!Array.isArray(steps) || steps.length === 0) return ''
  const last = steps[steps.length - 1]
  return typeof last === 'string' ? last : ''
}

/** 读取会话消息里的助手正文（从后往前找第一条非空），供分享使用 */
export function lastAssistantContent(messages) {
  if (!Array.isArray(messages)) return ''
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i]
    if (m?.role === 'assistant') {
      const text = String(m.content ?? '').trim()
      if (text) return text
    }
  }
  return ''
}

/**
 * 解析某个折叠区的展开状态：显式覆盖优先，未覆盖时用传入的默认值。
 *
 * 页面模式（density='page'）希望默认展开各分区，侧边浮窗（'panel'）希望默认折叠，
 * 但用户手动点开关后必须记住 —— 用「覆盖表 + 默认值」表达，而不是把默认值写死进表。
 *
 * @param {Object} overrideMap 形如 { 3: true, 'r-3': false } 的覆盖表
 * @param {string|number} key 折叠区标识
 * @param {boolean} fallback 未覆盖时的默认值
 */
export function resolveExpanded(overrideMap, key, fallback) {
  const v = (overrideMap ?? {})[key]
  if (v === undefined || v === null) return !!fallback
  return !!v
}
