/**
 * 导航相关逻辑 Composable
 * 从 App.vue 提取的导航逻辑，提供统一的导航管理
 */

import { h, ref } from 'vue'
import { useRouter } from 'vue-router'
import { EventsEmit, Quit, Hide, WindowFullscreen, WindowUnfullscreen } from '../../wailsjs/runtime'
import { NIcon } from 'naive-ui'
import { createMenuOptions } from '../config/navigation'

// 图标导入 - 集中管理
import {
  AlarmOutline,
  AnalyticsOutline,
  BarChartSharp,
  BonfireOutline,
  DiamondOutline,
  ExpandOutline,
  Flag,
  FlaskOutline,
  GlobeOutline,
  NewspaperOutline,
  NewspaperSharp,
  Notifications,
  报告
  ReportSearch,
  ReportAnalytics,
  MoneyCollectOutlined,
  ServerOutline,
  SettingsOutline,
  SparklesOutline,
  StarOutline,
  StatsChartOutline,
  TimeOutline,
  WarningOutline,
} from '@vicons/ionicons5'

/**
 * 图标映射表
 * 集中管理所有导航图标，便于统一替换和维护
 */
export const NAV_ICONS = {
  StarOutline,
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
  ReportSearch,
  ReportAnalytics,
  MoneyCollectOutlined,
  ServerOutline,
}

/**
 * 图标渲染辅助函数
 */
export function renderIcon(icon) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

/**
 * 导航管理 Composable
 * @returns {Object} 导航相关状态和方法
 */
export function useNavigation() {
  const router = useRouter()
  const activeKey = ref('stock')
  const isFullscreen = ref(false)

  /**
   * 切换全屏状态
   */
  function toggleFullscreen() {
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
  const menuOptions = ref(createMenuOptions({
    activeKey,
    router,
    EventsEmit,
    renderIcon,
    isFullscreen,
    Quit,
    Hide,
    icons: NAV_ICONS,
  }))

  return {
    // 状态
    activeKey,
    isFullscreen,
    menuOptions,

    // 方法
    toggleFullscreen,
    renderIcon,
  }
}

export default useNavigation
