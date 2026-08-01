<script setup>
import {computed, h, onBeforeUnmount, onMounted, ref} from "vue";
import {
  AddPrompt,
  DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  GetMultiAgentPrompts,
  UpdateMultiAgentPrompt,
  SendDingDingMessageByType,
  UpdateConfig,
  CheckSponsorCode,
  FetchAiModels,
  FetchAiModelInfo
} from "../../wailsjs/go/main/App";
import {NButton, NTag, NTooltip, NIcon, useMessage} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";

const message = useMessage()

const formRef = ref(null)
const formValue = ref({
  ID: 1,
  tushareToken: '',
  iwencaiApiKey: '',
  emApiKey: '',
  dingPush: {
    enable: false,
    dingRobot: ''
  },
  wechatPush: {
    enable: false,
    robot: ''
  },
  feishuPush: {
    enable: false,
    robot: ''
  },
  telegramPush: {
    enable: false,
    botToken: '',
    chatID: ''
  },
  emailPush: {
    enable: false,
    smtpHost: '',
    smtpPort: 587,
    smtpUser: '',
    smtpPass: '',
    to: ''
  },
  localPush: {
    enable: true,
  },
  updateBasicInfoOnStart: false,
  refreshInterval: 1,
  openAI: {
    enable: false,
    aiConfigs: [], // AI配置列表
    prompt: '你是一位拥有20年经验的顶级股票投资大师，精通价值投资、趋势交易、量化分析。你擅长结合宏观经济、行业周期和基本面进行全方位分析，对A股、港股、美股有深刻理解。秉持"风险控制第一"原则，分析时请调用工具获取实时数据（行情、财务、新闻、资金流向），不得凭记忆编造数据。给出明确的操作建议：强烈看多/看多/持有/看空/强烈看空，并附上关键数据支撑和风险提示。',
    questionTemplate: '请对 {{stockName}}({{stockCode}}) 进行全面分析，涵盖基本面、技术面、资金面和消息面。给出综合评级和操作建议。',
    crawlTimeOut: 30,
    kDays: 30,
    httpProxy:"",
    httpProxyEnabled:false,
  },
  enableDanmu: false,
  freeStockDBEnable: false,
  freeStockDBPath: '',
  freeStockDBAddr: '127.0.0.1:7899',
  freeStockDBAutoStart: false,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enablePushNews: true,
  enableOnlyPushRedNews: true,
  sponsorCode: "",
  httpProxy:"",
  httpProxyEnabled:false,
  enableAgent: false,
  qgqpBId: '',
  updateChannel: 'release',
  promptPlazaApiBase: '',
  quickThinkModelId: null,
  deepThinkModelId: null,
})

// 添加一个新的AI配置到列表
function addAiConfig() {
  formValue.value.openAI.aiConfigs.push(new data.AIConfig({
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-reasoner',
    temperature: 0.1,
    maxTokens: 8192,
    timeOut: 6000,
    httpProxy:"",
    httpProxyEnabled:false,
    thinking: true,
    deepModelName: '',
  }));
}

// 从列表中移除一个AI配置
function removeAiConfig(index) {
  const originalCount = formValue.value.openAI.aiConfigs.length;
  // 使用filter创建新数组确保响应式更新
  formValue.value.openAI.aiConfigs = formValue.value.openAI.aiConfigs.filter((_, i) => i !== index);
}

const updateChannelOptions = [
  { label: 'Release（稳定版）', value: 'release' },
  { label: 'Pre-release（预发布版）', value: 'pre' },
  { label: 'Dev（开发版）', value: 'dev' },
]

async function fetchAiModels(aiConfig) {
  if (!aiConfig.baseUrl || !aiConfig.apiKey) {
    message.warning('请先填写接口地址和 apiKey')
    return
  }
  if (aiConfig._loadingModels) {
    return
  }
  aiConfig._loadingModels = true
  try {
    const list = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey)
    const options = (list || []).map(id => ({ label: id, value: id }))
    aiConfig._modelOptions = options
    if (!aiConfig.modelName && options.length > 0) {
      aiConfig.modelName = options[0].value
      onModelNameChange(aiConfig, aiConfig.modelName)
    }
    if (!options.length) {
      message.warning('未从接口获取到可用模型，请检查地址和 apiKey')
    }
  } catch (e) {
    console.error('FetchAiModels error', e)
    message.error('获取模型列表失败，请检查接口地址和 apiKey')
  } finally {
    aiConfig._loadingModels = false
  }
}


const promptTemplates = ref([])
const aiConfigExpandedNames = ref([])

const aiConfigOptions = computed(() => {
  return (formValue.value.openAI.aiConfigs || []).map(config => ({
    label: config.name || `AI 配置 #${formValue.value.openAI.aiConfigs.indexOf(config) + 1}`,
    value: config.ID,
  }))
})

const aiPlatformOptions = [
  { label: 'DeepSeek (https://api.deepseek.com)', value: 'https://api.deepseek.com' },
  { label: '硅基流动 (https://api.siliconflow.cn/v1)', value: 'https://api.siliconflow.cn/v1' },
  { label: '智谱AI(GLM) (https://open.bigmodel.cn/api/paas/v4)', value: 'https://open.bigmodel.cn/api/paas/v4' },
  { label: '字节豆包(火山引擎) (https://ark.cn-beijing.volces.com/api/v3)', value: 'https://ark.cn-beijing.volces.com/api/v3' },
  { label: '阿里云百炼 (https://dashscope.aliyuncs.com/compatible-mode/v1)', value: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { label: 'Moonshot(月之暗面) (https://api.moonshot.cn/v1)', value: 'https://api.moonshot.cn/v1' },
  { label: '腾讯混元 (https://api.hunyuan.cloud.tencent.com/v1)', value: 'https://api.hunyuan.cloud.tencent.com/v1' },
  { label: '讯飞星火 (https://spark-api-open.xf-yun.com/v1)', value: 'https://spark-api-open.xf-yun.com/v1' },
  { label: '零一万物 (https://api.lingyiwanwu.com/v1)', value: 'https://api.lingyiwanwu.com/v1' },
  { label: 'MiniMax (https://api.minimax.chat/v1)', value: 'https://api.minimax.chat/v1' },
  { label: '小米MiMo TokenPlan (https://token-plan-cn.xiaomimimo.com/v1)', value: 'https://token-plan-cn.xiaomimimo.com/v1' },
  { label: '小米MiMo (https://api.xiaomimimo.com/v1)', value: 'https://api.xiaomimimo.com/v1' },
  { label: '腾讯云TokenHub (https://tokenhub.tencentmaas.com/v1)', value: 'https://tokenhub.tencentmaas.com/v1' },
  { label: 'OpenAI (https://api.openai.com/v1)', value: 'https://api.openai.com/v1' },
  { label: 'Azure OpenAI (https://YOUR_RESOURCE.openai.azure.com)', value: 'https://YOUR_RESOURCE.openai.azure.com' },
  { label: 'OpenRouter (https://openrouter.ai/api/v1)', value: 'https://openrouter.ai/api/v1' },
  { label:'Ollama (http://localhost:11434/v1)', value: 'http://localhost:11434/v1' },
  { label: 'Anthropic Claude (https://api.anthropic.com)', value: 'https://api.anthropic.com' },
  { label: 'OpenAI 兼容接口 (自定义)', value: 'https://你的接口地址/v1' },
]

function getPlatformName(baseUrl) {
  if (!baseUrl) return ''
  const platform = aiPlatformOptions.find(opt => opt.value === baseUrl)
  if (platform) {
    const idx = platform.label.indexOf(' (')
    return idx > 0 ? platform.label.substring(0, idx) : platform.label
  }
  return ''
}

function onBaseUrlChange(aiConfig, newBaseUrl) {
  const platformName = getPlatformName(newBaseUrl)
  if (platformName && aiConfig.name && !aiConfig.name.startsWith(platformName)) {
    aiConfig.name = platformName + '-' + aiConfig.name
  } else if (platformName && !aiConfig.name) {
    aiConfig.name = platformName
  }
}

function onModelNameChange(aiConfig, newModelName) {
  if (!newModelName) return
  const platformName = getPlatformName(aiConfig.baseUrl)
  const baseName = platformName || 'AI'
  
  if (!aiConfig.name) {
    aiConfig.name = baseName + '-' + newModelName
  } else if (aiConfig.name === platformName) {
    aiConfig.name = platformName + '-' + newModelName
  } else {
    const parts = aiConfig.name.split('-')
    if (parts.length >= 2 && parts[0] === platformName) {
      parts[parts.length - 1] = newModelName
      aiConfig.name = parts.join('-')
    } else if (!aiConfig.name.endsWith(newModelName)) {
      aiConfig.name = aiConfig.name + '-' + newModelName
    }
  }

  fetchModelInfo(aiConfig, newModelName)
}

async function fetchModelInfo(aiConfig, modelName) {
  if (!modelName || !aiConfig.baseUrl) return
  try {
    const info = await FetchAiModelInfo(aiConfig.baseUrl, aiConfig.apiKey || '', modelName)
    if (info && info.maxTokens > 0) {
      aiConfig.maxTokens = info.maxTokens
      const sourceLabel = info.source === 'api' ? 'API' : '内置数据'
      message.success(`已自动设置 ${modelName} 的 MaxTokens 为 ${info.maxTokens}（来源：${sourceLabel}）`)
    }
  } catch (e) {
    console.error('FetchAiModelInfo error', e)
  }
}

onMounted(() => {
  GetConfig().then(res => {
    formValue.value.ID = res.ID
    formValue.value.tushareToken = res.tushareToken
    formValue.value.iwencaiApiKey = res.iwencaiApiKey || ''
    formValue.value.emApiKey = res.emApiKey || ''
    formValue.value.dingPush = {
      enable: res.dingPushEnable,
      dingRobot: res.dingRobot
    }
    formValue.value.wechatPush = {
      enable: res.wechatPushEnable,
      robot: res.wechatRobot
    }
    formValue.value.feishuPush = {
      enable: res.feishuPushEnable,
      robot: res.feishuRobot
    }
    formValue.value.telegramPush = {
      enable: res.telegramPushEnable,
      botToken: res.telegramBotToken,
      chatID: res.telegramChatID
    }
    formValue.value.emailPush = {
      enable: res.emailPushEnable,
      smtpHost: res.emailSmtpHost,
      smtpPort: res.emailSmtpPort || 587,
      smtpUser: res.emailSmtpUser,
      smtpPass: res.emailSmtpPass,
      to: res.emailTo
    }
    formValue.value.localPush = {
      enable: res.localPushEnable,
    }
    formValue.value.updateBasicInfoOnStart = res.updateBasicInfoOnStart
    formValue.value.refreshInterval = res.refreshInterval
    // 加载AI配置
    formValue.value.openAI = {
      enable: res.openAiEnable,
      aiConfigs: res.aiConfigs || [],
      prompt: res.prompt ? res.prompt : '你是一位拥有20年经验的顶级股票投资大师，精通价值投资、趋势交易、量化分析。你擅长结合宏观经济、行业周期和基本面进行全方位分析，对A股、港股、美股有深刻理解。秉持"风险控制第一"原则，分析时请调用工具获取实时数据（行情、财务、新闻、资金流向），不得凭记忆编造数据。给出明确的操作建议：强烈看多/看多/持有/看空/强烈看空，并附上关键数据支撑和风险提示。',
      questionTemplate: res.questionTemplate ? res.questionTemplate : '请对 {{stockName}}({{stockCode}}) 进行全面分析，涵盖基本面、技术面、资金面和消息面。给出综合评级和操作建议。',
      crawlTimeOut: res.crawlTimeOut,
      kDays: res.kDays,
      httpProxy:"",
      httpProxyEnabled:false,
    }


    formValue.value.enableDanmu = res.enableDanmu
    formValue.value.browserPath = res.browserPath
    formValue.value.freeStockDBEnable = res.freeStockDBEnable
    formValue.value.freeStockDBPath = res.freeStockDBPath
    formValue.value.freeStockDBAddr = res.freeStockDBAddr || '127.0.0.1:7899'
    formValue.value.freeStockDBAutoStart = res.freeStockDBAutoStart
    formValue.value.enableNews = res.enableNews
    formValue.value.darkTheme = res.darkTheme
    formValue.value.enableFund = res.enableFund
    formValue.value.enablePushNews = res.enablePushNews
    formValue.value.enableOnlyPushRedNews = res.enableOnlyPushRedNews
    formValue.value.sponsorCode = res.sponsorCode
    formValue.value.httpProxy=res.httpProxy;
    formValue.value.httpProxyEnabled=res.httpProxyEnabled;
    formValue.value.enableAgent = res.enableAgent;
    formValue.value.qgqpBId = res.qgqpBId;
    formValue.value.updateChannel = res.updateChannel || 'release';
    formValue.value.promptPlazaApiBase = res.promptPlazaApiBase || '';
    formValue.value.quickThinkModelId = res.quickThinkModelId || null;
    formValue.value.deepThinkModelId = res.deepThinkModelId || null;
  })

  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res
  })
  loadMultiAgentPrompts()
})
onBeforeUnmount(() => {
  message.destroyAll()
})

function saveConfig() {
  console.log('开始保存设置', formValue.value);
  // 构建配置时，包含aiConfigs列表
  let config = new data.SettingConfig({
    ID: formValue.value.ID,
    dingPushEnable: formValue.value.dingPush.enable,
    dingRobot: formValue.value.dingPush.dingRobot,
    wechatPushEnable: formValue.value.wechatPush.enable,
    wechatRobot: formValue.value.wechatPush.robot,
    feishuPushEnable: formValue.value.feishuPush.enable,
    feishuRobot: formValue.value.feishuPush.robot,
    telegramPushEnable: formValue.value.telegramPush.enable,
    telegramBotToken: formValue.value.telegramPush.botToken,
    telegramChatID: formValue.value.telegramPush.chatID,
    emailPushEnable: formValue.value.emailPush.enable,
    emailSmtpHost: formValue.value.emailPush.smtpHost,
    emailSmtpPort: formValue.value.emailPush.smtpPort,
    emailSmtpUser: formValue.value.emailPush.smtpUser,
    emailSmtpPass: formValue.value.emailPush.smtpPass,
    emailTo: formValue.value.emailPush.to,
    localPushEnable: formValue.value.localPush.enable,
    updateBasicInfoOnStart: formValue.value.updateBasicInfoOnStart,
    refreshInterval: formValue.value.refreshInterval,
    openAiEnable: formValue.value.openAI.enable,
    aiConfigs: formValue.value.openAI.aiConfigs,
    // 序列化aiConfigs列表以传递给后端
    tushareToken: formValue.value.tushareToken,
    iwencaiApiKey: formValue.value.iwencaiApiKey,
    emApiKey: formValue.value.emApiKey,
    prompt: formValue.value.openAI.prompt,
    questionTemplate: formValue.value.openAI.questionTemplate,
    crawlTimeOut: formValue.value.openAI.crawlTimeOut,
    kDays: formValue.value.openAI.kDays,
    enableDanmu: formValue.value.enableDanmu,
    browserPath: formValue.value.browserPath,
    freeStockDBEnable: formValue.value.freeStockDBEnable,
    freeStockDBPath: formValue.value.freeStockDBPath,
    freeStockDBAddr: formValue.value.freeStockDBAddr,
    freeStockDBAutoStart: formValue.value.freeStockDBAutoStart,
    enableNews: formValue.value.enableNews,
    darkTheme: formValue.value.darkTheme,
    enableFund: formValue.value.enableFund,
    enablePushNews: formValue.value.enablePushNews,
    enableOnlyPushRedNews: formValue.value.enableOnlyPushRedNews,
    sponsorCode: formValue.value.sponsorCode,
    httpProxy:formValue.value.httpProxy,
    httpProxyEnabled:formValue.value.httpProxyEnabled,
    enableAgent: formValue.value.enableAgent,
    qgqpBId: formValue.value.qgqpBId,
    updateChannel: formValue.value.updateChannel,
    promptPlazaApiBase: formValue.value.promptPlazaApiBase,
    quickThinkModelId: formValue.value.quickThinkModelId,
    deepThinkModelId: formValue.value.deepThinkModelId,
  })

  if (config.sponsorCode) {
    CheckSponsorCode(config.sponsorCode).then(res => {
      if (!res.code) {
        message.warning(res.msg || '赞助码验证失败')
      }
    })
  }

  UpdateConfig(config).then(res => {
    if (res === '保存成功！') {
      message.success(res)
    } else {
      message.error(res)
    }
    EventsEmit("updateSettings", config);
  })
}


function getHeight() {
  return document.documentElement.clientHeight
}

function sendTestNotice() {
  let markdown = "### go-stock test\n" + new Date()
  let msg = '{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"go-stock' + new Date() + '",' +
      '         "text": "' + markdown + '"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'

  SendDingDingMessageByType(msg, "test-" + new Date().getTime(), 1).then(res => {
    message.info(res)
  })
}

function sendTestNotification(channel) {
  const fn = window['go']?.['main']?.['App']?.['SendTestNotification']
  if (typeof fn !== 'function') {
    message.warning('测试通知接口尚未绑定，请重新生成 Wails 绑定后重试')
    return
  }
  fn(channel).then(res => {
    message.info(res)
  }).catch(err => {
    message.error('发送测试通知失败: ' + err)
  })
}

function exportConfig() {
  ExportConfig().then(res => {
    message.info(res)
  })
}

function importConfig() {
  let input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    let file = e.target.files[0];
    let reader = new FileReader();
    reader.onload = (e) => {
      let config = JSON.parse(e.target.result);
      formValue.value.ID = config.ID
      formValue.value.tushareToken = config.tushareToken
      formValue.value.iwencaiApiKey = config.iwencaiApiKey || ''
      formValue.value.emApiKey = config.emApiKey || ''
      formValue.value.dingPush = {
        enable: config.dingPushEnable,
        dingRobot: config.dingRobot
      }
      formValue.value.wechatPush = {
        enable: config.wechatPushEnable,
        robot: config.wechatRobot
      }
      formValue.value.feishuPush = {
        enable: config.feishuPushEnable,
        robot: config.feishuRobot
      }
      formValue.value.telegramPush = {
        enable: config.telegramPushEnable,
        botToken: config.telegramBotToken,
        chatID: config.telegramChatID
      }
      formValue.value.emailPush = {
        enable: config.emailPushEnable,
        smtpHost: config.emailSmtpHost,
        smtpPort: config.emailSmtpPort || 587,
        smtpUser: config.emailSmtpUser,
        smtpPass: config.emailSmtpPass,
        to: config.emailTo
      }
      formValue.value.localPush = {
        enable: config.localPushEnable,
      }
      formValue.value.updateBasicInfoOnStart = config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval
      // 导入AI配置
      formValue.value.openAI = {
        enable: config.openAiEnable,
        aiConfigs: config.aiConfigs || [],
        prompt: config.prompt,
        questionTemplate: config.questionTemplate,
        crawlTimeOut: config.crawlTimeOut,
        kDays: config.kDays
      }
      formValue.value.enableDanmu = config.enableDanmu
      formValue.value.browserPath = config.browserPath
      formValue.value.freeStockDBEnable = config.freeStockDBEnable
      formValue.value.freeStockDBPath = config.freeStockDBPath
      formValue.value.freeStockDBAddr = config.freeStockDBAddr || '127.0.0.1:7899'
      formValue.value.freeStockDBAutoStart = config.freeStockDBAutoStart
      formValue.value.enableNews = config.enableNews
      formValue.value.darkTheme = config.darkTheme
      formValue.value.enableFund = config.enableFund
      formValue.value.enablePushNews = config.enablePushNews
      formValue.value.enableOnlyPushRedNews = config.enableOnlyPushRedNews
      formValue.value.sponsorCode = config.sponsorCode
      formValue.value.httpProxy=config.httpProxy
      formValue.value.httpProxyEnabled=config.httpProxyEnabled
      formValue.value.enableAgent = config.enableAgent
      formValue.value.qgqpBId = config.qgqpBId
      formValue.value.updateChannel = config.updateChannel || 'release'
      formValue.value.quickThinkModelId = config.quickThinkModelId || null
      formValue.value.deepThinkModelId = config.deepThinkModelId || null
    };
    reader.readAsText(file);
  };
  input.click();
}


window.onerror = function (event, source, lineno, colno, error) {
  EventsEmit("frontendError", {
    page: "settings.vue",
    message: event,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null
  });
  return true;
};

const showManagePromptsModal = ref(false)
const promptTypeOptions = [
  {label: "模型系统Prompt", value: '模型系统Prompt'},
  {label: "模型用户Prompt", value: '模型用户Prompt'},]
const formPromptRef = ref(null)
const formPrompt = ref({
  ID: 0,
  Name: '',
  Content: '',
  Type: '',
})

function managePrompts() {
  formPrompt.value.ID = 0
  showManagePromptsModal.value = true
}

function savePrompt() {
  AddPrompt(formPrompt.value).then(res => {
    message.success(res)
    GetPromptTemplates("", "").then(res => {
      promptTemplates.value = res
    })
    showManagePromptsModal.value = false
  })
}

function editPrompt(prompt) {
  formPrompt.value.ID = prompt.ID
  formPrompt.value.Name = prompt.name
  formPrompt.value.Content = prompt.content
  formPrompt.value.Type = prompt.type
  showManagePromptsModal.value = true
}

function deletePrompt(ID) {
  DelPrompt(ID).then(res => {
    message.success(res)
    GetPromptTemplates("", "").then(res => {
      promptTemplates.value = res
    })
  })
}

// --- 多智能体提示词管理 ---
const multiAgentPromptList = ref([])
const editingPromptRoleKey = ref('')
const editingPromptContent = ref('')

const multiAgentPromptColumns = [
  { title: '角色', key: 'name', width: 140 },
  { title: 'RoleKey', key: 'roleKey', width: 180 },
  { title: '内容预览', key: 'content',
    render(row) {
      const preview = row.content ? row.content.substring(0, 80) + (row.content.length > 80 ? '...' : '') : ''
      return h('span', { style: 'font-size:12px;color:#666' }, preview)
    }
  },
  { title: '操作', key: 'actions', width: 120,
    render(row) {
      return h(NButton, {
        size: 'tiny',
        type: 'primary',
        secondary: true,
        onClick: () => startEditPrompt(row)
      }, () => '编辑')
    }
  }
]

function loadMultiAgentPrompts() {
  GetMultiAgentPrompts().then(res => {
    multiAgentPromptList.value = res || []
  }).catch(e => {
    console.error('loadMultiAgentPrompts error', e)
  })
}

function startEditPrompt(row) {
  editingPromptRoleKey.value = row.roleKey
  editingPromptContent.value = row.content
}

function cancelEditPrompt() {
  editingPromptRoleKey.value = ''
  editingPromptContent.value = ''
}

function saveMultiAgentPrompt() {
  if (!editingPromptRoleKey.value) return
  const item = multiAgentPromptList.value.find(p => p.roleKey === editingPromptRoleKey.value)
  const name = item ? item.name : editingPromptRoleKey.value
  UpdateMultiAgentPrompt(editingPromptRoleKey.value, name, editingPromptContent.value).then(res => {
    message.success(res)
    editingPromptRoleKey.value = ''
    editingPromptContent.value = ''
    loadMultiAgentPrompts()
  }).catch(e => {
    message.error('保存失败: ' + e)
  })
}
</script>

<template>
  <n-flex justify="left" style="text-align: left; --wails-draggable:no-drag">
    <n-form ref="formRef" :label-placement="'left'" :label-align="'left'">
      <n-space vertical size="large">
        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '基础设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
<!--            <n-form-item-gi :span="10" label="Tushare Token：" path="tushareToken">
              <n-input type="text" placeholder="Tushare api token" v-model:value="formValue.tushareToken" clearable/>
            </n-form-item-gi>-->
            <n-form-item-gi :span="4" label="启动时更新基础信息：" path="updateBasicInfoOnStart">
              <n-switch v-model:value="formValue.updateBasicInfoOnStart"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="数据刷新间隔：" path="refreshInterval">
              <n-input-number v-model:value="formValue.refreshInterval" placeholder="请输入数据刷新间隔(秒)">
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="暗黑主题：" path="darkTheme">
              <n-switch v-model:value="formValue.darkTheme"/>
            </n-form-item-gi>
            <!-- 更新通道（自动更新已禁用） -->
            <!-- <n-form-item-gi :span="8" label="更新通道：" path="updateChannel">
              <n-select v-model:value="formValue.updateChannel" :options="updateChannelOptions" />
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    更新通道说明：<br>
                    <b>Release（稳定版）</b>：仅接收正式发布版本，稳定性最高<br>
                    <b>Pre-release（预发布版）</b>：包含预发布版本，可提前体验新功能<br>
                    <b>Dev（开发版）</b>：包含所有可用版本，获取最新开发进度
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi> -->
            <n-form-item-gi :span="10" label="浏览器安装路径：" path="browserPath">
              <n-input type="text" placeholder="浏览器安装路径" v-model:value="formValue.browserPath" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="本地数据引擎：" path="freeStockDBEnable">
              <n-switch v-model:value="formValue.freeStockDBEnable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="引擎程序路径：" path="freeStockDBPath">
              <n-input type="text" placeholder="stockdb.exe 完整路径" v-model:value="formValue.freeStockDBPath" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="引擎地址：" path="freeStockDBAddr">
              <n-input type="text" placeholder="127.0.0.1:7899" v-model:value="formValue.freeStockDBAddr" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="自动拉起：" path="freeStockDBAutoStart">
              <n-switch v-model:value="formValue.freeStockDBAutoStart"/>
            </n-form-item-gi>
           <n-form-item-gi :span="3" label="指数基金：" path="enableFund">
              <n-switch v-model:value="formValue.enableFund"/>
            </n-form-item-gi>
            <!--      <n-form-item-gi :span="3" label="AI智能体：" path="enableAgent">
                   <n-switch v-model:value="formValue.enableAgent"/>
                 </n-form-item-gi>-->
            <n-form-item-gi :span="11" label="东财唯一标识：" path="qgqpBId">
              <n-input type="text" placeholder="东财唯一标识" v-model:value="formValue.qgqpBId" clearable/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    打开浏览器,访问东财网站，<br>
                    按F12打开开发人员工具-》网络面板，<br>
                    随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="问财API密钥：" path="iwencaiApiKey">
              <n-input type="password" placeholder="同花顺问财开放平台API Key" v-model:value="formValue.iwencaiApiKey" clearable show-password-on="click"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    访问同花顺问财开放平台：<br>
                    <a href="https://open.iwencai.com" target="_blank" style="color: #63e2b7">https://www.iwencai.com/skillhub</a><br>
                    注册并登录后，在控制台获取API Key。<br>
                    配置后可使用问财智能选股、行情查询、研报搜索等功能。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="东财AI密钥：" path="emApiKey">
              <n-input type="password" placeholder="东方财富AI SaaS API Key" v-model:value="formValue.emApiKey" clearable show-password-on="click"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    访问东方财富妙想AI平台获取API Key。
                    https://ai.eastmoney.com/mxClaw<br>
                    配置后可使用个股业绩点评功能。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="赞助码：" path="sponsorCode">
              <n-input-group>
                <n-input :show-count="true" placeholder="联系作者QQ或微信获取，激活VIP功能" v-model:value="formValue.sponsorCode">
                </n-input>
                <n-button type="success" secondary strong
                          @click="CheckSponsorCode(formValue.sponsorCode).then((res) => {message.warning(res.msg)})">验证
                </n-button>
                <n-popover trigger="hover" placement="top">
                  <template #trigger>
                    <n-icon color="#0e7a0d" size="20">
                      <HelpCircleFilledIcon />
                    </n-icon>
                  </template>
                  <n-gradient-text :type="'warning'">
                    <div style="max-width: 400px;text-align: left">
                      赞助码获取方式：<br>
                      联系作者获取赞助码，激活VIP功能<br>
                      享受更多高级功能和优先支持
                    </div>
                  </n-gradient-text>
                </n-popover>
              </n-input-group>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="提示词广场地址：" path="promptPlazaApiBase">
              <n-input type="text" placeholder="http://go-stock.sparkmemory.top:1918/api" v-model:value="formValue.promptPlazaApiBase" clearable/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    提示词广场服务接口地址<br>
                    默认: http://go-stock.sparkmemory.top:1918/api<br>
                    如已部署提示词广场服务，可修改为实际地址
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '通知设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="3" label="钉钉推送：" path="dingPush.enable">
              <n-switch v-model:value="formValue.dingPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="本地推送：" path="localPush.enable">
              <n-switch v-model:value="formValue.localPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="弹幕功能：" path="enableDanmu">
              <n-switch v-model:value="formValue.enableDanmu"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="显示滚动快讯：" path="enableNews">
              <n-switch v-model:value="formValue.enableNews"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="市场资讯提醒：" path="enablePushNews">
              <n-switch v-model:value="formValue.enablePushNews"/>
            </n-form-item-gi>
            <n-form-item-gi v-if="formValue.enablePushNews" :span="4" label="只提醒红字或关注个股的新闻：" path="enableOnlyPushRedNews">
              <n-switch v-model:value="formValue.enableOnlyPushRedNews"/>
            </n-form-item-gi>

            <n-form-item-gi :span="22" v-if="formValue.dingPush.enable" label="钉钉机器人接口地址："
                            path="dingPush.dingRobot">
              <n-input placeholder="请输入钉钉机器人接口地址" v-model:value="formValue.dingPush.dingRobot"/>
              <n-button type="primary" @click="sendTestNotice">发送测试通知</n-button>
            </n-form-item-gi>

            <n-form-item-gi :span="3" label="企业微信推送：" path="wechatPush.enable">
              <n-switch v-model:value="formValue.wechatPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="22" v-if="formValue.wechatPush.enable" label="企业微信机器人 Webhook："
                            path="wechatPush.robot">
              <n-input placeholder="企业微信机器人 Webhook 地址" v-model:value="formValue.wechatPush.robot"/>
              <n-button type="primary" @click="sendTestNotification('wechat')">发送测试</n-button>
            </n-form-item-gi>

            <n-form-item-gi :span="3" label="飞书推送：" path="feishuPush.enable">
              <n-switch v-model:value="formValue.feishuPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="22" v-if="formValue.feishuPush.enable" label="飞书机器人 Webhook："
                            path="feishuPush.robot">
              <n-input placeholder="飞书机器人 Webhook 地址" v-model:value="formValue.feishuPush.robot"/>
              <n-button type="primary" @click="sendTestNotification('feishu')">发送测试</n-button>
            </n-form-item-gi>

            <n-form-item-gi :span="3" label="Telegram 推送：" path="telegramPush.enable">
              <n-switch v-model:value="formValue.telegramPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="11" v-if="formValue.telegramPush.enable" label="Bot Token："
                            path="telegramPush.botToken">
              <n-input placeholder="Telegram Bot Token" v-model:value="formValue.telegramPush.botToken"/>
            </n-form-item-gi>
            <n-form-item-gi :span="11" v-if="formValue.telegramPush.enable" label="Chat ID："
                            path="telegramPush.chatID">
              <n-input placeholder="Telegram Chat ID" v-model:value="formValue.telegramPush.chatID"/>
              <n-button type="primary" @click="sendTestNotification('telegram')">发送测试</n-button>
            </n-form-item-gi>

            <n-form-item-gi :span="3" label="邮件推送：" path="emailPush.enable">
              <n-switch v-model:value="formValue.emailPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="11" v-if="formValue.emailPush.enable" label="SMTP 服务器："
                            path="emailPush.smtpHost">
              <n-input placeholder="SMTP Host" v-model:value="formValue.emailPush.smtpHost"/>
            </n-form-item-gi>
            <n-form-item-gi :span="5" v-if="formValue.emailPush.enable" label="SMTP 端口："
                            path="emailPush.smtpPort">
              <n-input-number v-model:value="formValue.emailPush.smtpPort" placeholder="587"/>
            </n-form-item-gi>
            <n-form-item-gi :span="11" v-if="formValue.emailPush.enable" label="SMTP 用户名："
                            path="emailPush.smtpUser">
              <n-input placeholder="SMTP 用户名" v-model:value="formValue.emailPush.smtpUser"/>
            </n-form-item-gi>
            <n-form-item-gi :span="11" v-if="formValue.emailPush.enable" label="SMTP 密码："
                            path="emailPush.smtpPass">
              <n-input type="password" placeholder="SMTP 密码" v-model:value="formValue.emailPush.smtpPass"
                       show-password-on="click"/>
            </n-form-item-gi>
            <n-form-item-gi :span="22" v-if="formValue.emailPush.enable" label="收件人邮箱："
                            path="emailPush.to">
              <n-input placeholder="收件人邮箱地址" v-model:value="formValue.emailPush.to"/>
              <n-button type="primary" @click="sendTestNotification('email')">发送测试</n-button>
            </n-form-item-gi>

          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'AI设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left;">
            <n-form-item-gi :span="24" label="AI诊股：" path="openAI.enable">
              <n-switch v-model:value="formValue.openAI.enable"/>
            </n-form-item-gi>

            <n-form-item-gi :span="6" v-if="formValue.openAI.enable" label="Crawler Timeout(秒)"
                            title="资讯采集超时时间(秒)" path="openAI.crawlTimeOut">
              <n-input-number min="30" step="1" v-model:value="formValue.openAI.crawlTimeOut"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" v-if="formValue.openAI.enable" title="天数越多消耗tokens越多"
                            label="日K线数据(天)" path="openAI.kDays">
              <n-input-number min="30" step="1" max="60" v-model:value="formValue.openAI.kDays"/>
            </n-form-item-gi>
            <n-form-item-gi :span="2" label="爬虫http代理" path="httpProxyEnabled">
              <n-switch v-model:value="formValue.httpProxyEnabled"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" v-if="formValue.httpProxyEnabled" title="http代理地址"
                            label="http代理地址" path="httpProxy">
              <n-input type="text" placeholder="爬虫http代理地址" v-model:value="formValue.httpProxy" clearable/>
            </n-form-item-gi>


            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">默认提示词设置</n-divider>
            </n-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="默认系统提示词" path="openAI.prompt">
              <n-input v-model:value="formValue.openAI.prompt" type="textarea" :show-count="true"
                       placeholder="请输入系统提示词" :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="默认个股分析提示词"
                            path="openAI.questionTemplate">
              <n-input v-model:value="formValue.openAI.questionTemplate" type="textarea" :show-count="true"
                       placeholder="请输入个股分析提示词:例如{{stockName}}[{{stockCode}}]分析和总结"
                       :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">AI模型服务配置</n-divider>
            </n-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-space vertical>
                <n-collapse v-model:expanded-names="aiConfigExpandedNames" accordion>
                  <n-collapse-item v-for="(aiConfig, index) in formValue.openAI.aiConfigs" :key="index" :name="String(index)">
                    <template #header>
                      <n-flex justify="space-between" align="center" style="width: 100%;">
                        <n-text>{{ aiConfig.name || `AI 配置 #${index + 1}` }}</n-text>
                        <n-text depth="3" style="font-size: 12px;">{{ aiConfig.modelName || '未选择模型' }}</n-text>
                      </n-flex>
                    </template>
                    <template #header-extra>
                      <n-button type="error" size="tiny" ghost @click.stop="removeAiConfig(index)" style="margin-right: 8px;">删除</n-button>
                    </template>
                    <n-grid :cols="24" :x-gap="24">
                      <n-form-item-gi :span="24" hidden label="配置ID" :path="`openAI.aiConfigs[${index}].ID`">
                        <n-input type="text" placeholder="配置ID" v-model:value="aiConfig.ID" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="配置名称" :path="`openAI.aiConfigs[${index}].name`">
                        <n-input type="text" placeholder="配置名称" v-model:value="aiConfig.name" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="接口地址" :path="`openAI.aiConfigs[${index}].baseUrl`">
                        <n-select
                          v-model:value="aiConfig.baseUrl"
                          :options="aiPlatformOptions"
                          filterable
                          tag
                          clearable
                          placeholder="选择或输入AI接口地址"
                          @update:value="(val) => onBaseUrlChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="令牌(apiKey)" :path="`openAI.aiConfigs[${index}].apiKey`">
                        <n-input type="password" placeholder="apiKey" v-model:value="aiConfig.apiKey" clearable
                                 show-password-on="click"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="模型名称" :path="`openAI.aiConfigs[${index}].modelName`">
                        <n-select
                          v-model:value="aiConfig.modelName"
                          :options="aiConfig._modelOptions || []"
                          filterable
                          tag
                          :loading="aiConfig._loadingModels"
                          placeholder="点击获取模型列表或手动输入"
                          @click="fetchAiModels(aiConfig)"
                          @update:value="(val) => onModelNameChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="深度模型" :path="`openAI.aiConfigs[${index}].deepModelName`">
                        <n-select
                          v-model:value="aiConfig.deepModelName"
                          :options="aiConfig._modelOptions || []"
                          filterable
                          tag
                          :loading="aiConfig._loadingModels"
                          clearable
                          placeholder="留空则使用快速模型(降级)"
                          @click="fetchAiModels(aiConfig)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="Temperature" :path="`openAI.aiConfigs[${index}].temperature`">
                        <n-input-number placeholder="temperature" v-model:value="aiConfig.temperature" :step="0.1"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="MaxTokens" :path="`openAI.aiConfigs[${index}].maxTokens`">
                        <n-input-number placeholder="maxTokens" v-model:value="aiConfig.maxTokens"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="Timeout(秒)" :path="`openAI.aiConfigs[${index}].timeOut`">
                        <n-input-number min="60" step="1" placeholder="超时(秒)" v-model:value="aiConfig.timeOut"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="深度思考">
                        <n-switch v-model:value="aiConfig.thinking"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              启用深度思考模式：<br>
                              适用于 DeepSeek-Reasoner、MiMo-V2.5-Pro 等支持推理的模型。<br>
                              如使用普通模型请关闭此选项
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="http代理" :path="`openAI.aiConfigs[${index}].httpProxyEnabled`">
                        <n-switch v-model:value="aiConfig.httpProxyEnabled"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" v-if="aiConfig.httpProxyEnabled" title="http代理地址" :path="`openAI.aiConfigs[${index}].httpProxy`">
                        <n-input type="text" placeholder="http代理地址" v-model:value="aiConfig.httpProxy" clearable/>
                      </n-form-item-gi>
                    </n-grid>
                  </n-collapse-item>
                </n-collapse>
                <n-divider />
                <h3>⚡ 双LLM模型配置</h3>
                <n-space vertical>
                  <n-form-item label="快速模型 (Quick Think)" description="用于分析师分析和研究员辩论，建议选轻量模型降低成本">
                    <n-select v-model:value="formValue.quickThinkModelId" :options="aiConfigOptions" clearable placeholder="默认AI配置" />
                  </n-form-item>
                  <n-form-item label="深度模型 (Deep Think)" description="用于总结决策，建议选推理能力强的模型。留空则降级为快速模型">
                    <n-select v-model:value="formValue.deepThinkModelId" :options="aiConfigOptions" clearable placeholder="使用快速模型(降级)" />
                  </n-form-item>
                </n-space>
                <n-button type="primary" dashed @click="addAiConfig" style="width: 100%;">+ 添加AI配置</n-button>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-collapse arrow-placement="right" :default-expanded-names="[]">
                <n-collapse-item title="🧠 Agent 提示词管理" name="agent-prompts">
                  <n-data-table
                    :columns="multiAgentPromptColumns"
                    :data="multiAgentPromptList"
                    :bordered="false"
                    :single-line="false"
                    size="small"
                  />
                  <n-space v-if="editingPromptRoleKey" style="margin-top: 12px">
                    <n-input type="textarea" v-model:value="editingPromptContent" :autosize="{ minRows: 6, maxRows: 16 }" style="width: 100%" />
                    <n-space justify="end">
                      <n-button size="small" @click="cancelEditPrompt">取消</n-button>
                      <n-button size="small" type="primary" @click="saveMultiAgentPrompt">保存</n-button>
                    </n-space>
                  </n-space>
                </n-collapse-item>
              </n-collapse>
            </n-gi>

            <n-gi :span="24">
              <n-divider/>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-space justify="center">
<!--                  <n-button type="warning" @click="managePrompts">管理提示词模板</n-button>-->
                  <n-button type="primary" strong @click="saveConfig">保存设置</n-button>
                  <n-button type="info" @click="exportConfig">导出配置</n-button>
                  <n-button type="error" @click="importConfig">导入配置</n-button>
                </n-space>

<!--                <n-flex justify="start" style="margin-top: 10px" v-if="promptTemplates.length > 0">-->
<!--                  <n-tag :bordered="false" type="warning">提示词模板:</n-tag>-->
<!--                  <n-tag size="medium" secondary v-for="prompt in promptTemplates" closable-->
<!--                         @close="deletePrompt(prompt.ID)" @click="editPrompt(prompt)" :title="prompt.content"-->
<!--                         :type="prompt.type === '模型系统Prompt' ? 'success' : 'info'" :bordered="false">{{-->
<!--                      prompt.name-->
<!--                    }}-->
<!--                  </n-tag>-->
<!--                </n-flex>-->
              </n-space>
            </n-gi>

          </n-grid>
        </n-card>
      </n-space>
    </n-form>
  </n-flex>

  <n-modal v-model:show="showManagePromptsModal" closable :mask-closable="false">
    <n-card style="width: 800px; height: 600px; text-align: left" :bordered="false"
            :title="(formPrompt.ID > 0 ? '修改' : '添加') + '提示词'" size="huge" role="dialog" aria-modal="true">
      <n-form ref="formPromptRef" :label-placement="'left'" :label-align="'left'">
        <n-form-item label="名称">
          <n-input v-model:value="formPrompt.Name" placeholder="请输入提示词名称"/>
        </n-form-item>
        <n-form-item label="类型">
          <n-select v-model:value="formPrompt.Type" :options="promptTypeOptions" placeholder="请选择提示词类型"/>
        </n-form-item>
        <n-form-item label="内容">
          <n-input v-model:value="formPrompt.Content" type="textarea" :show-count="true" placeholder="请输入prompt"
                   :autosize="{ minRows: 12, maxRows: 12, }"/>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-flex justify="end">
          <n-button type="primary" @click="savePrompt">保存</n-button>
          <n-button type="warning" @click="showManagePromptsModal = false">取消</n-button>
        </n-flex>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.cardHeaderClass {
  font-size: 16px;
  font-weight: bold;
  color: red;
}
</style>