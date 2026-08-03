/**
 * Wails 事件监听 Composable
 * 从 App.vue 提取的事件监听逻辑
 */

import { onBeforeUnmount } from 'vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { useAppStore, useStockStore } from '../stores'

/**
 * 注册 Wails 事件监听器
 * 自动在组件卸载时清理监听器
 */
export function useWailsEvents() {
  const appStore = useAppStore()
  const stockStore = useStockStore()

  // 事件处理器映射
  const eventHandlers = {
    // 实时盈亏更新
    realtime_profit: (data) => {
      stockStore.setRealtimeProfit(data)
    },

    // 电报更新
    telegraph: (data) => {
      // TODO: 移至 market store
      console.log('[EVENT] telegraph', data)
    },

    // 加载状态更新
    loadingMsg: (data) => {
      if (data === 'done') {
        appStore.setLoading(false, '加载完成...')
      } else {
        appStore.setLoading(true, data)
      }
    },

    // 新闻推送
    newsPush: (data) => {
      console.log('[EVENT] newsPush', data)
    },
  }

  // 注册所有事件监听器
  function registerEvents() {
    Object.entries(eventHandlers).forEach(([event, handler]) => {
      EventsOn(event, handler)
    })
  }

  // 清理所有事件监听器
  function unregisterEvents() {
    Object.keys(eventHandlers).forEach((event) => {
      EventsOff(event)
    })
  }

  // 自动在组件卸载时清理
  onBeforeUnmount(() => {
    unregisterEvents()
  })

  return {
    registerEvents,
    unregisterEvents,
  }
}

export default useWailsEvents
