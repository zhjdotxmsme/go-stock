/**
 * Wails 事件监听 Composable
 * 从 App.vue 提取的事件监听逻辑
 */

import { ref, onBeforeUnmount } from 'vue'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime'
import { useAppStore, useStockStore } from '../stores'
import { h } from 'vue'
import { createDiscreteApi, darkTheme, lightTheme, NText } from 'naive-ui'

/**
 * 注册 Wails 事件监听器
 * 自动在组件卸载时清理监听器
 * @param {Object} options - 可选配置
 * @param {Object} options.themeRef - 主题 ref（用于 createDiscreteApi）
 * @param {Function} options.getEnableNews - 获取 enableNews 的函数
 */
export function useWailsEvents(options = {}) {
  const appStore = useAppStore()
  const stockStore = useStockStore()

  // 电报数据 ref（默认空数组，与 App.vue 保持一致）
  const telegraph = ref([])

  // newsPush 事件引用（需要延迟注册）
  let newsPushRegistered = false

  /**
   * 注册 newsPush 事件监听
   * 使用 createDiscreteApi 在组件树外创建通知
   * @param {Ref} enableDarkTheme - 主题 ref
   */
  function registerNewsPush(enableDarkTheme) {
    if (newsPushRegistered) return
    newsPushRegistered = true

    const { notification } = createDiscreteApi(['notification'], {
      configProviderProps: {
        theme: enableDarkTheme?.value ? darkTheme : lightTheme,
        max: 3,
      },
    })

    EventsOn('newsPush', (data) => {
      if (data.isRed) {
        notification.create({
          title: data.time,
          content: () => h('div', {
            type: 'error',
            style: {
              'text-align': 'left',
              'font-size': '14px',
              color: '#f67979',
            },
          }, { default: () => data.content }),
          meta: () => h(NText, { type: 'warning' }, { default: () => data.source }),
          duration: 1000 * 40,
        })
      } else {
        notification.create({
          title: data.time,
          content: () => h('div', {
            type: 'info',
            style: {
              'text-align': 'left',
              'font-size': '14px',
              color: data.source === 'go-stock' ? '#F98C24' : '#549EC8',
            },
          }, { default: () => data.content }),
          meta: () => h(NText, { type: 'warning' }, { default: () => data.source }),
          duration: 1000 * 30,
        })
      }
    })
  }

  // 事件处理器映射（不含 newsPush，需要延迟注册）
  const eventHandlers = {
    // 实时盈亏更新
    realtime_profit: (data) => {
      stockStore.setRealtimeProfit(data)
    },

    // 电报更新
    telegraph: (data) => {
      telegraph.value = data
    },

    // 加载状态更新
    loadingMsg: (data) => {
      if (data === 'done') {
        appStore.setLoading(false, '加载完成...')
        EventsEmit('loadingDone', 'app')
      } else {
        appStore.setLoading(true, data)
      }
    },
  }

  // 注册所有事件监听器（不含 newsPush）
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
    if (newsPushRegistered) {
      EventsOff('newsPush')
      newsPushRegistered = false
    }
  }

  // 自动在组件卸载时清理
  onBeforeUnmount(() => {
    unregisterEvents()
  })

  return {
    registerEvents,
    unregisterEvents,
    registerNewsPush,
    telegraph,
  }
}

export default useWailsEvents
