/**
 * 设置相关状态 Store
 * 管理用户配置、AI配置、数据源设置等
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useSettingsStore = defineStore('settings', () => {
  // ========== 状态 ==========

  /** Tushare Token */
  const tushareToken = ref('')

  /** 推送设置 */
  const localPushEnable = ref(false)
  const dingPushEnable = ref(false)
  const dingRobot = ref('')

  /** 功能开关 */
  const updateBasicInfoOnStart = ref(false)
  const enableNews = ref(false)
  const enableFund = ref(false)
  const enableAgent = ref(false)
  const enableDanmu = ref(false)

  /** 刷新间隔 (ms) */
  const refreshInterval = ref(3000)

  /** AI 配置 */
  const aiConfigs = ref([])
  const activeAiConfigId = ref(0)

  /** 浏览器设置 */
  const browserPath = ref('')
  const browserPoolSize = ref(3)

  /** 主题设置 */
  const darkTheme = ref(false)

  /** 升级提示 */
  const sponsorCode = ref('')

  // ========== 计算属性 ==========

  /** 当前激活的 AI 配置 */
  const currentAiConfig = computed(() => {
    return aiConfigs.value.find(c => c.ID === activeAiConfigId.value) || aiConfigs.value[0] || null
  })

  /** 是否配置了 Tushare */
  const hasTushareConfig = computed(() => tushareToken.value && tushareToken.value.length > 0)

  /** 是否配置了 AI */
  const hasAiConfig = computed(() => aiConfigs.value.length > 0)

  // ========== 方法 ==========

  /**
   * 加载设置
   */
  function loadSettings(settings) {
    if (!settings) return

    // 基础设置
    if (settings.tushareToken !== undefined) tushareToken.value = settings.tushareToken
    if (settings.localPushEnable !== undefined) localPushEnable.value = settings.localPushEnable
    if (settings.dingPushEnable !== undefined) dingPushEnable.value = settings.dingPushEnable
    if (settings.dingRobot !== undefined) dingRobot.value = settings.dingRobot
    if (settings.updateBasicInfoOnStart !== undefined) updateBasicInfoOnStart.value = settings.updateBasicInfoOnStart
    if (settings.refreshInterval !== undefined) refreshInterval.value = settings.refreshInterval

    // 功能开关
    if (settings.enableNews !== undefined) enableNews.value = settings.enableNews
    if (settings.enableFund !== undefined) enableFund.value = settings.enableFund
    if (settings.enableAgent !== undefined) enableAgent.value = settings.enableAgent
    if (settings.enableDanmu !== undefined) enableDanmu.value = settings.enableDanmu

    // 浏览器设置
    if (settings.browserPath !== undefined) browserPath.value = settings.browserPath
    if (settings.browserPoolSize !== undefined) browserPoolSize.value = settings.browserPoolSize

    // 主题设置
    if (settings.darkTheme !== undefined) darkTheme.value = settings.darkTheme

    // AI 配置
    if (settings.aiConfigs !== undefined) aiConfigs.value = settings.aiConfigs
    if (settings.activeAiConfigId !== undefined) activeAiConfigId.value = settings.activeAiConfigId

    // 赞助码
    if (settings.sponsorCode !== undefined) sponsorCode.value = settings.sponsorCode
  }

  /**
   * 导出设置对象
   */
  function exportSettings() {
    return {
      tushareToken: tushareToken.value,
      localPushEnable: localPushEnable.value,
      dingPushEnable: dingPushEnable.value,
      dingRobot: dingRobot.value,
      updateBasicInfoOnStart: updateBasicInfoOnStart.value,
      refreshInterval: refreshInterval.value,
      enableNews: enableNews.value,
      enableFund: enableFund.value,
      enableAgent: enableAgent.value,
      enableDanmu: enableDanmu.value,
      browserPath: browserPath.value,
      browserPoolSize: browserPoolSize.value,
      darkTheme: darkTheme.value,
      aiConfigs: aiConfigs.value,
      activeAiConfigId: activeAiConfigId.value,
      sponsorCode: sponsorCode.value,
    }
  }

  /**
   * 设置活跃 AI 配置
   */
  function setActiveAiConfig(id) {
    activeAiConfigId.value = id
  }

  /**
   * 添加 AI 配置
   */
  function addAiConfig(config) {
    aiConfigs.value.push(config)
  }

  /**
   * 删除 AI 配置
   */
  function removeAiConfig(id) {
    const index = aiConfigs.value.findIndex(c => c.ID === id)
    if (index > -1) {
      aiConfigs.value.splice(index, 1)
    }
  }

  /**
   * 更新 AI 配置
   */
  function updateAiConfig(id, updates) {
    const config = aiConfigs.value.find(c => c.ID === id)
    if (config) {
      Object.assign(config, updates)
    }
  }

  return {
    // 状态
    tushareToken,
    localPushEnable,
    dingPushEnable,
    dingRobot,
    updateBasicInfoOnStart,
    refreshInterval,
    enableNews,
    enableFund,
    enableAgent,
    enableDanmu,
    browserPath,
    browserPoolSize,
    darkTheme,
    aiConfigs,
    activeAiConfigId,
    sponsorCode,

    // 计算属性
    currentAiConfig,
    hasTushareConfig,
    hasAiConfig,

    // 方法
    loadSettings,
    exportSettings,
    setActiveAiConfig,
    addAiConfig,
    removeAiConfig,
    updateAiConfig,
  }
})

export default useSettingsStore
