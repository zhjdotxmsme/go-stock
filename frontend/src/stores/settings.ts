/**
 * 设置相关状态 Store
 * 管理用户配置、AI配置、数据源设置等
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

/** 设置相关状态 */
export interface SettingsState {
  /** Tushare Token */
  tushareToken: string
  /** 推送设置 */
  localPushEnable: boolean
  dingPushEnable: boolean
  dingRobot: string
  /** 功能开关 */
  updateBasicInfoOnStart: boolean
  enableNews: boolean
  enableFund: boolean
  enableAgent: boolean
  enableDanmu: boolean
  /** 刷新间隔 (ms) */
  refreshInterval: number
  /** AI 配置 */
  aiConfigs: any[]
  activeAiConfigId: number
  /** 浏览器设置 */
  browserPath: string
  browserPoolSize: number
  /** 主题设置 */
  darkTheme: boolean
}

export const useSettingsStore = defineStore('settings', () => {
  // ========== 状态 ==========

  /** Tushare Token */
  const tushareToken = ref<string>('')

  /** 推送设置 */
  const localPushEnable = ref<boolean>(false)
  const dingPushEnable = ref<boolean>(false)
  const dingRobot = ref<string>('')

  /** 功能开关 */
  const updateBasicInfoOnStart = ref<boolean>(false)
  const enableNews = ref<boolean>(false)
  const enableFund = ref<boolean>(false)
  const enableAgent = ref<boolean>(false)
  const enableDanmu = ref<boolean>(false)

  /** 刷新间隔 (ms) */
  const refreshInterval = ref<number>(3000)

  /** AI 配置 */
  const aiConfigs = ref<any[]>([])
  const activeAiConfigId = ref<number>(0)

  /** 浏览器设置 */
  const browserPath = ref<string>('')
  const browserPoolSize = ref<number>(3)

  /** 主题设置 */
  const darkTheme = ref<boolean>(false)

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
  function loadSettings(settings: any): void {
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
  }

  /**
   * 导出设置对象
   */
  function exportSettings(): Record<string, any> {
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
    }
  }

  /**
   * 设置活跃 AI 配置
   */
  function setActiveAiConfig(id: number): void {
    activeAiConfigId.value = id
  }

  /**
   * 添加 AI 配置
   */
  function addAiConfig(config: any): void {
    aiConfigs.value.push(config)
  }

  /**
   * 删除 AI 配置
   */
  function removeAiConfig(id: number): void {
    const index = aiConfigs.value.findIndex(c => c.ID === id)
    if (index > -1) {
      aiConfigs.value.splice(index, 1)
    }
  }

  /**
   * 更新 AI 配置
   */
  function updateAiConfig(id: number, updates: any): void {
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
