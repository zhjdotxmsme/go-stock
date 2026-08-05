/**
 * Agent 配置选项：模型配置、系统/用户提示词、思考/记忆/Agent 模式（自 FloatingAgentAssistant.vue 原样搬迁）。
 * inputValue/showHint 经 ctx 传入。
 */
import { ref, computed, watch, onMounted } from 'vue'
import * as systemApi from '../../api/system'

const STORAGE_KEY_MODEL_ID = 'go-stock-agent-last-model-id'

export function useAgentOptions(ctx) {
  const { inputValue, showHint } = ctx

  const aiConfigOptions = ref([])
  const aiConfigId = ref(null)

  function modelLabelForConfig(configId) {
    const opts = aiConfigOptions.value
    if (!opts?.length) return ''
    const id = configId != null ? Number(configId) : Number(opts[0].value)
    const found = opts.find(o => Number(o.value) === id)
    return found?.label != null ? String(found.label) : ''
  }

  const sysPromptTemplates = ref([])
  const sysPromptOptions = computed(() =>
    sysPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
  )
  const sysPromptId = ref(null)

  const userPromptTemplates = ref([])
  const userPromptOptions = computed(() =>
    userPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
  )
  const userPromptId = ref(null)
  const thinkingMode = ref(true)
  const memoryMode = ref(false)
  const memoryCount = ref(1)
  const memoryCountOptions = [
    { label: '1 条', value: 1 },
    { label: '2 条', value: 2 },
    { label: '3 条', value: 3 },
    { label: '4 条', value: 4 },
    { label: '5 条', value: 5 },
    { label: '10 条', value: 10 },
  ]
  const agentMode = ref('auto')
  const agentModeOptions = [
    { label: '🤖 自动选择', value: 'auto' },
    { label: '⚡ 快速模式', value: 'react' },
    { label: '🧠 规划模式', value: 'plan_execute' },
  ]

  watch(agentMode, (val) => {
    if (val === 'react') showHint('⚡ 快速模式推荐使用DeepSeek最新版')
    else if (val === 'plan_execute') showHint('🧠 规划模式推荐使用GLM最新版')
  })

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

  function onUserPromptChange(id) {
    if (!id) return
    const t = userPromptTemplates.value.find(x => (x.ID ?? x.id) === id)
    if (t?.content) inputValue.value = t.content
  }

  function loadPromptTemplates() {
    systemApi.getPromptTemplates('', '').then(({data: res}) => {
      const list = Array.isArray(res) ? res : []
      sysPromptTemplates.value = list.filter(t => t.type === '模型系统Prompt')
      userPromptTemplates.value = list.filter(t => t.type === '模型用户Prompt')
    })
  }

  onMounted(() => {
    systemApi.getAiConfigs().then(({data: res}) => {
      const list = Array.isArray(res) ? res : []
      aiConfigOptions.value = list.map((c, index) => {
        const id = c.ID != null ? Number(c.ID) : (c.id != null ? Number(c.id) : index)
        const name = c.name ?? c.Name ?? ''
        const modelName = c.modelName ?? c.ModelName ?? ''
        return {
          label: name + (modelName ? ' [' + modelName + ']' : ''),
          value: id
        }
      })
      if (aiConfigOptions.value.length) {
        const lastModelId = localStorage.getItem(STORAGE_KEY_MODEL_ID)
        if (lastModelId) {
          const foundId = Number(lastModelId)
          const isValid = aiConfigOptions.value.some(opt => opt.value === foundId)
          aiConfigId.value = isValid ? foundId : aiConfigOptions.value[0].value
        } else {
          aiConfigId.value = aiConfigOptions.value[0].value
        }
      }
    })
  })

  watch(aiConfigId, (newId) => {
    if (newId != null) {
      localStorage.setItem(STORAGE_KEY_MODEL_ID, String(newId))
    }
  })

  return {
    aiConfigOptions, aiConfigId, modelLabelForConfig,
    sysPromptTemplates, sysPromptOptions, sysPromptId,
    userPromptTemplates, userPromptOptions, userPromptId,
    thinkingMode, memoryMode, memoryCount, memoryCountOptions,
    agentMode, agentModeOptions, onUserPromptChange, loadPromptTemplates,
  }
}
