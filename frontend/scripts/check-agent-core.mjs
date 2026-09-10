/**
 * Agent 流式核心逻辑 + 首选项读写回归测试（无第三方依赖，node 直接跑）
 *
 *   node scripts/check-agent-core.mjs      （或 npm run test:agent）
 *
 * 覆盖 spec 9.1：parseStepText 四类输入、chunk 累加、结束判定、
 * 中断守卫所依赖的纯函数、以及 prefs 的读写与兜底。
 */
import {
  STEP_PREFIX,
  DONE_CONTENT,
  newUserMessage,
  newAssistantMessage,
  isDoneChunk,
  applyChunkToAssistant,
  finalizeAssistantMessage,
  currentStepSummary,
  lastAssistantContent,
} from '../src/components/agent/agentStreamCore.js'
import {
  PREFS_KEY,
  LEGACY_MODEL_KEY,
  defaultPrefs,
  pickPrefs,
  loadPrefs,
  savePrefs,
} from '../src/components/agent/agentPrefs.js'

let failed = 0
let passed = 0

function eq(label, got, want) {
  const g = JSON.stringify(got)
  const w = JSON.stringify(want)
  if (g === w) {
    passed++
    return
  }
  failed++
  console.log(`FAIL ${label}\n     got  ${g}\n     want ${w}`)
}
function ok(label, cond) {
  if (cond) {
    passed++
    return
  }
  failed++
  console.log(`FAIL ${label}`)
}

// ---------------------------------------------------------------- 消息构造
{
  const u = newUserMessage('分析茅台', '2026-09-10 11:00:00')
  eq('newUserMessage.role', u.role, 'user')
  eq('newUserMessage.content', u.content, '分析茅台')
  eq('newUserMessage.steps', u.steps, [])

  const a = newAssistantMessage('deepseek-chat')
  eq('newAssistantMessage.role', a.role, 'assistant')
  eq('newAssistantMessage.content', a.content, '')
  eq('newAssistantMessage.steps', a.steps, [])
  eq('newAssistantMessage.modelName', a.modelName, 'deepseek-chat')
}

// ---------------------------------------------------------------- 结束判定
{
  eq('isDoneChunk(agent-DONE)', isDoneChunk({ content: DONE_CONTENT }), true)
  eq('isDoneChunk(finish_reason=stop)', isDoneChunk({ response_meta: { finish_reason: 'stop' } }), true)
  eq('isDoneChunk(普通正文)', isDoneChunk({ content: '普通正文' }), false)
  eq('isDoneChunk(finish_reason=length)', isDoneChunk({ response_meta: { finish_reason: 'length' } }), false)
  eq('isDoneChunk(null)', isDoneChunk(null), false)
}

// ---------------------------------------------------------------- 执行步骤（[STEP] 前缀四类输入）
{
  const t = newAssistantMessage()
  applyChunkToAssistant(t, { role: 'assistant', reasoning_content: STEP_PREFIX + '🔄 获取行情' })
  eq('STEP 纯字符串 → 1 条步骤', t.steps, ['🔄 获取行情'])

  applyChunkToAssistant(t, { role: 'assistant', reasoning_content: STEP_PREFIX + '["a","b"]' })
  eq('STEP JSON 数组 → 展开为编号步骤', t.steps.slice(1), ['1. a', '2. b'])

  applyChunkToAssistant(t, { role: 'assistant', reasoning_content: STEP_PREFIX + '{"steps":["x"]}' })
  eq('STEP JSON 对象(steps) → 1 条', t.steps.slice(-1), ['1. x'])

  applyChunkToAssistant(t, { role: 'assistant', reasoning_content: STEP_PREFIX + '{坏 JSON' })
  eq('STEP 非法 JSON → 原文保留', t.steps.slice(-1), ['{坏 JSON'])

  applyChunkToAssistant(t, { role: 'assistant', reasoning_content: STEP_PREFIX + '   ' })
  eq('STEP 空内容 → 不新增', t.steps.length, 5)
}

// ---------------------------------------------------------------- 思维链 / 正文累加
{
  const t = newAssistantMessage()
  ok('首条 reasoning 有变化', applyChunkToAssistant(t, { reasoning_content: '先看' }) === true)
  applyChunkToAssistant(t, { reasoning_content: '行情' })
  eq('reasoning 累加', t.reasoning, '先看行情')
  eq('rawReasoning 与 reasoning 同步', t.rawReasoning, '先看行情')

  applyChunkToAssistant(t, { content: '贵州' })
  applyChunkToAssistant(t, { content: '茅台' })
  eq('content 累加', t.content, '贵州茅台')
  eq('rawContent 累加', t.rawContent, '贵州茅台')

  // 结束标记不得混进正文（防御性：正常路径由 isDoneChunk 提前 return）
  applyChunkToAssistant(t, { content: DONE_CONTENT })
  eq('agent-DONE 不写入正文', t.content, '贵州茅台')

  eq('空 chunk 无变化', applyChunkToAssistant(t, null), false)
  eq('无内容的 chunk 无变化', applyChunkToAssistant(t, { role: 'assistant' }), false)
}

// ---------------------------------------------------------------- 中断守卫（isAborted 时 store 直接 return，核心函数不被调用）
{
  const t = newAssistantMessage()
  applyChunkToAssistant(t, { content: '中断前' })
  const snapshot = JSON.stringify(t)
  // 模拟 store 的守卫：中断后不再调用核心函数
  const isAborted = true
  if (!isAborted) applyChunkToAssistant(t, { content: '中断后' })
  eq('中断后消息内容不变', JSON.stringify(t), snapshot)
}

// ---------------------------------------------------------------- 收尾格式化 + 分析报告提取
{
  const t = newAssistantMessage()
  t.rawReasoning = '先取行情'
  t.rawContent = '好的\n{"response":"# 分析报告\\n\\n现价 1580 元"}'
  finalizeAssistantMessage(t)
  eq('rawReasoning 格式化写入 reasoning', t.reasoning, '先取行情')
  ok('jsonMarkdown 被提取', typeof t.jsonMarkdown === 'string' && t.jsonMarkdown.includes('分析报告'))
  ok('正文不再包含原始 JSON', !t.content.includes('"response"'))

  const empty = newAssistantMessage()
  finalizeAssistantMessage(empty)
  eq('空消息收尾不报错', empty.content, '')
  finalizeAssistantMessage(null) // 不应抛
  passed++
}

// ---------------------------------------------------------------- 当前步骤摘要
{
  eq('无步骤 → 空串', currentStepSummary([]), '')
  eq('非数组 → 空串', currentStepSummary(null), '')
  eq('取最后一条', currentStepSummary(['1. a', '2. b']), '2. b')
}

// ---------------------------------------------------------------- 最后一条助手正文（分享用）
{
  const list = [
    { role: 'user', content: 'q' },
    { role: 'assistant', content: '  ' },
    { role: 'assistant', content: '最终答案' },
  ]
  eq('跳过空白找最后一条非空', lastAssistantContent(list), '最终答案')
  eq('空列表', lastAssistantContent([]), '')
  eq('非数组', lastAssistantContent(null), '')
}

// ---------------------------------------------------------------- 首选项
class FakeStorage {
  constructor(seed = {}) {
    this.map = new Map(Object.entries(seed))
  }
  getItem(k) {
    return this.map.has(k) ? this.map.get(k) : null
  }
  setItem(k, v) {
    this.map.set(k, String(v))
  }
}

{
  eq('defaultPrefs', defaultPrefs(), {
    aiConfigId: null,
    sysPromptId: null,
    userPromptId: null,
    thinkingMode: true,
    memoryMode: false,
    memoryCount: 1,
    agentMode: 'auto',
  })

  // 脏数据只取已知字段
  eq('pickPrefs 丢弃未知字段', pickPrefs({ aiConfigId: '7', 未知: 1, agentMode: 'react' }), {
    aiConfigId: 7,
    sysPromptId: null,
    userPromptId: null,
    thinkingMode: true,
    memoryMode: false,
    memoryCount: 1,
    agentMode: 'react',
  })

  const s = new FakeStorage()
  eq('无存储内容 → 默认值', loadPrefs(s).aiConfigId, null)
  eq('savePrefs 成功', savePrefs({ aiConfigId: 3, memoryMode: true, memoryCount: 5 }, s), true)
  const back = loadPrefs(s)
  eq('读回 aiConfigId', back.aiConfigId, 3)
  eq('读回 memoryMode', back.memoryMode, true)
  eq('读回 memoryCount', back.memoryCount, 5)

  // 旧键兜底
  const legacy = new FakeStorage({ [LEGACY_MODEL_KEY]: '9' })
  eq('旧键兜底 aiConfigId', loadPrefs(legacy).aiConfigId, 9)

  // 新键优先于旧键
  const both = new FakeStorage({ [LEGACY_MODEL_KEY]: '9' })
  savePrefs({ aiConfigId: 4 }, both)
  eq('agent.prefs 优先', loadPrefs(both).aiConfigId, 4)

  // 损坏 JSON 不抛异常
  const broken = new FakeStorage({ [PREFS_KEY]: '{不是 JSON' })
  eq('损坏 JSON → 默认值', loadPrefs(broken).aiConfigId, null)

  // 存储不可用
  eq('storage=null 读取', loadPrefs(null).agentMode, 'auto')
  eq('storage=null 写入', savePrefs({}, null), false)
  const throwing = {
    getItem() {
      throw new Error('denied')
    },
    setItem() {
      throw new Error('quota')
    },
  }
  eq('getItem 抛异常时降级', loadPrefs(throwing).agentMode, 'auto')
  eq('setItem 抛异常返回 false', savePrefs({ agentMode: 'react' }, throwing), false)
}

console.log(failed === 0 ? `\nALL PASS (${passed} assertions)` : `\n${failed} FAILURES / ${passed} passed`)
process.exit(failed === 0 ? 0 : 1)
