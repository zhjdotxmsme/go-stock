/**
 * 市场状态 Composable
 * 处理交易时间检查和市场状态更新
 */

import { ref, onMounted, onBeforeUnmount } from 'vue'
import { IsTradingTime, IsHKTradingTime, IsUSTradingTime, WindowSetTitle } from '../../wailsjs/go/main/App'
import { useAppStore } from '../stores'

/**
 * 市场状态管理
 * @param {Object} options - 配置选项
 * @param {number} options.interval - 更新间隔（毫秒），默认 30 秒
 */
export function useMarketStatus(options = {}) {
  const { interval = 30000 } = options

  const appStore = useAppStore()
  const marketStatus = ref('')
  let statusTimer = null

  /**
   * 更新市场状态
   */
  async function updateMarketStatus() {
    try {
      const [cn, hk, us] = await Promise.all([
        IsTradingTime().catch(() => false),
        IsHKTradingTime().catch(() => false),
        IsUSTradingTime().catch(() => false),
      ])

      const parts = []
      parts.push(cn ? 'A股交易中' : 'A股休市')
      parts.push(hk ? '港股交易中' : '港股休市')
      parts.push(us ? '美股交易中' : '美股休市')
      marketStatus.value = parts.join(' | ')

      // 更新窗口标题
      WindowSetTitle(
        `go-stock ${marketStatus.value} ${appStore.officialStatement} 「${appStore.currentMotto}」 [数据来源于网络，仅供参考；投资有风险，入市需谨慎]`
      )
    } catch (err) {
      console.error('Failed to update market status:', err)
    }
  }

  /**
   * 启动定时更新
   */
  function startAutoUpdate() {
    // 立即执行一次
    updateMarketStatus()

    // 设置定时器
    if (statusTimer) {
      clearInterval(statusTimer)
    }
    statusTimer = setInterval(updateMarketStatus, interval)
  }

  /**
   * 停止定时更新
   */
  function stopAutoUpdate() {
    if (statusTimer) {
      clearInterval(statusTimer)
      statusTimer = null
    }
  }

  // 组件挂载时启动
  onMounted(() => {
    startAutoUpdate()
  })

  // 组件卸载时停止
  onBeforeUnmount(() => {
    stopAutoUpdate()
  })

  return {
    marketStatus,
    updateMarketStatus,
    startAutoUpdate,
    stopAutoUpdate,
  }
}

export default useMarketStatus
