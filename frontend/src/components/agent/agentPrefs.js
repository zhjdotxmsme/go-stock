/**
 * Agent 首选项（模型 / 提示词 / 思考 / 记忆 / 模式）的读写。
 * 纯逻辑 + 可注入 storage，便于 node 单测；默认落 localStorage。
 */

export const PREFS_KEY = 'agent.prefs'

/** 旧版本单独存模型 ID 的键，读取时作为兜底，避免用户已选的模型丢失 */
export const LEGACY_MODEL_KEY = 'go-stock-agent-last-model-id'

/** 默认首选项（与历史默认值保持一致） */
export function defaultPrefs() {
  return {
    aiConfigId: null,
    sysPromptId: null,
    userPromptId: null,
    thinkingMode: true,
    memoryMode: false,
    memoryCount: 1,
    agentMode: 'auto',
  }
}

/** 只保留已知字段，避免把脏数据写回存储 */
export function pickPrefs(source) {
  const base = defaultPrefs()
  if (!source || typeof source !== 'object') return base
  const out = { ...base }
  if (source.aiConfigId != null) out.aiConfigId = Number(source.aiConfigId)
  if (source.sysPromptId != null) out.sysPromptId = source.sysPromptId
  if (source.userPromptId != null) out.userPromptId = source.userPromptId
  if (typeof source.thinkingMode === 'boolean') out.thinkingMode = source.thinkingMode
  if (typeof source.memoryMode === 'boolean') out.memoryMode = source.memoryMode
  if (source.memoryCount != null) out.memoryCount = Number(source.memoryCount)
  if (typeof source.agentMode === 'string' && source.agentMode) out.agentMode = source.agentMode
  return out
}

/**
 * 读取首选项。
 * @param {Storage} [storage] 默认 localStorage；传入 null/不可用时返回默认值
 */
export function loadPrefs(storage = globalThis.localStorage) {
  const prefs = defaultPrefs()
  if (!storage) return prefs
  let parsed = null
  try {
    const raw = storage.getItem(PREFS_KEY)
    if (raw) parsed = JSON.parse(raw)
  } catch {
    parsed = null
  }
  const merged = pickPrefs(parsed)
  // agent.prefs 尚未写入过时，回落到旧键
  if (merged.aiConfigId == null) {
    try {
      const legacy = storage.getItem(LEGACY_MODEL_KEY)
      if (legacy != null && legacy !== '') merged.aiConfigId = Number(legacy)
    } catch {
      /* ignore */
    }
  }
  return merged
}

/**
 * 写入首选项。
 * @returns {boolean} 是否写入成功（存储不可用/超限时返回 false，不抛异常）
 */
export function savePrefs(prefs, storage = globalThis.localStorage) {
  if (!storage) return false
  try {
    storage.setItem(PREFS_KEY, JSON.stringify(pickPrefs(prefs)))
    return true
  } catch {
    return false
  }
}
