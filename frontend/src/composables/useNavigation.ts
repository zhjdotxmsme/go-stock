/**
 * 导航相关逻辑 Composable
 * 从 App.vue 提取的导航逻辑，提供统一的导航管理
 */

import { h, ref } from 'vue'
import type { Ref } from 'vue'
import { useRouter } from 'vue-router'
import { EventsEmit, Quit, Hide, WindowFullscreen, WindowUnfullscreen } from '../../wailsjs/runtime'
import { NIcon } from 'naive-ui'
import { createMenuOptions } from '../config/navigation'
import { useStockStore } from '../stores'
import { GetGroupList } from '../../wailsjs/go/main/App'

// ========== 图标导入 - 集中管理（多源图标库） ==========

// Ionicons5
import {
  AnalyticsOutline,
  BarChartSharp,
  BonfireOutline,
  DiamondOutline,
  ExpandOutline,
  Flag,
  FlameSharp,
  FlaskOutline,
  GlobeOutline,
  NewspaperOutline,
  NewspaperSharp,
  PowerOutline,
  Pulse,
  ServerOutline,
  SettingsOutline,
  SparklesOutline,
  StarOutline,
  Star,
  StatsChartOutline,
  TimeOutline,
  WarningOutline,
  FlashOutline,
  SearchOutline,
  Notifications,
} from '@vicons/ionicons5'

// Tabler 图标
import { ReportSearch, ReportAnalytics, ReportMoney, TrendingUp, Prompt } from '@vicons/tabler'

// Ant Design 图标
import { MoneyCollectOutlined, NotificationFilled, StockOutlined } from '@vicons/antd'

// Fluent UI 图标
import { SlideHide24Filled, BoxSearch20Regular } from '@vicons/fluent'

// Material 图标
import { LocalFireDepartmentRound } from '@vicons/material'

// FA 图标
import { Dragon, FirefoxBrowser, Gripfire, Robot } from '@vicons/fa'

/**
 * 图标映射表
 * 集中管理所有导航图标，便于统一替换和维护
 */
export const NAV_ICONS: Record<string, any> = {
  // Ionicons5
  StarOutline,
  Star,
  NewspaperOutline,
  NewspaperSharp,
  BarChartSharp,
  AnalyticsOutline,
  Flag,
  Pulse,
  DiamondOutline,
  StatsChartOutline,
  FlaskOutline,
  GlobeOutline,
  BonfireOutline,
  SparklesOutline,
  TimeOutline,
  WarningOutline,
  FlameSharp,
  ReportSearch,
  ReportAnalytics,
  MoneyCollectOutlined,
  ServerOutline,
  FlashOutline,
  SettingsOutline,
  ExpandOutline,
  SlideHide24Filled,
  PowerOutline,
  SearchOutline,
  Notifications,
  // Tabler
  ReportMoney,
  TrendingUp,
  Prompt,
  // Material
  LocalFireDepartmentRound,
  // Fluent
  BoxSearch20Regular,
  // Ant Design
  NotificationFilled,
  StockOutlined,
  // FA
  Dragon,
  FirefoxBrowser,
  Gripfire,
  Robot,
}

/**
 * 图标渲染辅助函数
 */
export function renderIcon(icon: any) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

/** useNavigation 返回值 */
export interface UseNavigationReturn {
  activeKey: Ref<string>
  isFullscreen: Ref<boolean>
  menuOptions: Ref<any[]>
  toggleFullscreen: () => void
  renderIcon: typeof renderIcon
  loadDynamicMenus: () => Promise<void>
  applyConfigVisibility: (config: any) => void
}

/**
 * 导航管理 Composable
 * @returns {Object} 导航相关状态和方法
 */
export function useNavigation(): UseNavigationReturn {
  const router = useRouter()
  const stockStore = useStockStore()
  const activeKey = ref<string>('stock')
  const isFullscreen = ref<boolean>(false)

  /**
   * 切换全屏状态
   */
  function toggleFullscreen(): void {
    activeKey.value = 'full'
    if (isFullscreen.value) {
      WindowUnfullscreen()
    } else {
      WindowFullscreen()
    }
    isFullscreen.value = !isFullscreen.value
  }

  /**
   * 创建菜单配置 (注入依赖)
   */
  const menuOptions = ref<any[]>(createMenuOptions({
    activeKey,
    router,
    EventsEmit,
    renderIcon,
    isFullscreen,
    Quit,
    Hide,
    toggleFullscreen,
    icons: NAV_ICONS,
  }))

  /**
   * 加载动态群组菜单
   * 从 GetGroupList 获取用户自定义群组，注入到 stock 菜单的 children 中
   */
  async function loadDynamicMenus(): Promise<void> {
    try {
      const groupList = await GetGroupList()
      stockStore.setGroupList(groupList)

      // 动态注入群组子菜单到 stock 菜单
      menuOptions.value.forEach((item: any) => {
        if (item.key === 'stock') {
          const dynamicChildren = groupList.map((group: any) => ({
            label: () =>
              h(
                'a',
                {
                  href: '#',
                  type: 'info',
                  onClick: () => {
                    router.push({
                      name: 'stock',
                      query: {
                        groupName: group.name,
                        groupId: group.ID,
                      },
                    })
                    setTimeout(() => {
                      EventsEmit('changeTab', group)
                    }, 100)
                  },
                  to: {
                    name: 'stock',
                    query: {
                      groupName: group.name,
                      groupId: group.ID,
                    },
                  },
                },
                { default: () => group.name }
              ),
            key: group.ID,
          }))
          item.children.push(...dynamicChildren)
        }
      })
    } catch (err) {
      console.error('GetGroupList error:', err)
    }
  }

  /**
   * 根据 GetConfig 结果控制菜单显隐
   * @param {Object} config - GetConfig 返回的配置对象
   */
  function applyConfigVisibility(config: any): void {
    menuOptions.value.forEach((item: any) => {
      if (item.key === 'fund') {
        item.show = config.enableFund
      }
      if (item.key === 'agent') {
        item.show = config.enableAgent
      }
      // 技能管理菜单由配置控制显隐
      if (item.key === 'systemSkills') {
        item.show = config.enableAgent ?? true
      }
    })
  }

  return {
    // 状态
    activeKey,
    isFullscreen,
    menuOptions,

    // 方法
    toggleFullscreen,
    renderIcon,
    loadDynamicMenus,
    applyConfigVisibility,
  }
}

export default useNavigation
