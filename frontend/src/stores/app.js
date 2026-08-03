/**
 * 应用全局状态 Store
 * 管理应用级别的状态：主题、加载状态、全屏、窗口相关等
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAppStore = defineStore('app', () => {
  // ========== 状态 ==========

  /** 加载状态 */
  const loading = ref(true)
  const loadingMsg = ref('加载数据中...')

  /** 启用功能 */
  const enableNews = ref(false)
  const enableFund = ref(false)
  const enableAgent = ref(false)

  /** 主题设置 */
  const enableDarkTheme = ref(null)

  /** 应用内容 */
  const content = ref('未经授权,禁止商业目的!\n\n数据来源于网络,仅供参考;投资有风险,入市需谨慎')
  const officialStatement = ref('')

  /** 窗口状态 */
  const isFullscreen = ref(false)

  /** 市场状态 */
  const marketStatus = ref('')

  /** 投资格言 */
  const investmentMottos = [
    '投资有风险，入市需谨慎',
    '别人贪婪我恐惧，别人恐惧我贪婪',
    '股市有风险，投资需谨慎',
    '不要把所有鸡蛋放在一个篮子里',
    '时间是优秀企业的朋友',
    '买股票就是买公司',
    '市场短期是投票机，长期是称重机',
    '保住本金是投资的第一要务',
    '在别人恐慌时贪婪，在别人贪婪时恐慌',
    '风险来自于你不知道自己在做什么',
    '价格是你付出的，价值是你得到的',
    '投资最重要的品质是耐心',
    '机会总是留给有准备的人',
    '知行合一，方能致远',
    '顺势而为，逆势而思',
    '投资是一场马拉松，不是百米冲刺',
    '独立思考是投资成功的关键',
    '市场永远在波动，但价值终将回归',
    '控制风险比追求收益更重要',
    '学习是最好的投资',
  ]
  const currentMotto = ref(investmentMottos[Math.floor(Math.random() * investmentMottos.length)])

  // ========== 计算属性 ==========

  const isDark = computed(() => enableDarkTheme.value)

  // ========== 方法 ==========

  /**
   * 刷新投资格言
   */
  function refreshMotto() {
    currentMotto.value = investmentMottos[Math.floor(Math.random() * investmentMottos.length)]
  }

  /**
   * 设置加载状态
   */
  function setLoading(value, message = null) {
    loading.value = value
    if (message !== null) {
      if (message === 'done') {
        loadingMsg.value = '加载完成...'
        loading.value = false
      } else {
        loadingMsg.value = message
      }
    }
  }

  /**
   * 设置市场状态
   */
  function setMarketStatus(status) {
    marketStatus.value = status
  }

  /**
   * 设置官方声明
   */
  function setOfficialStatement(statement) {
    officialStatement.value = statement
  }

  /**
   * 切换暗色主题
   */
  function toggleDarkTheme() {
    enableDarkTheme.value = !enableDarkTheme.value
  }

  /**
   * 切换全屏
   */
  function toggleFullscreen() {
    isFullscreen.value = !isFullscreen.value
  }

  return {
    // 状态
    loading,
    loadingMsg,
    enableNews,
    enableFund,
    enableAgent,
    enableDarkTheme,
    content,
    officialStatement,
    isFullscreen,
    marketStatus,
    investmentMottos,
    currentMotto,

    // 计算属性
    isDark,

    // 方法
    refreshMotto,
    setLoading,
    setMarketStatus,
    setOfficialStatement,
    toggleDarkTheme,
    toggleFullscreen,
  }
})

export default useAppStore
