/**
 * 导航相关逻辑 Composable
 * 从 App.vue 提取的导航逻辑，提供统一的导航管理
 */

import { h, ref } from 'vue'
import { useRouter } from 'vue-router'
import { EventsEmit, Quit, Hide, WindowFullscreen, WindowUnfullscreen } from '../../wailsjs/runtime'
import { NIcon } from 'naive-ui'
import { createMenuOptions } from '../config/navigation'

// 图标导入 - 集中管理（多源图标库）
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
  StatsChartOutline,
  TimeOutline,
  WarningOutline,
  FlashOutline,
} from '@vicons/ionicons5'

// Tabler 图标
import { ReportSearch, ReportAnalytics } from '@vicons/tabler'

// Ant Design 图标
import { MoneyCollectOutlined } from '@vicons/antd'

// Fluent UI 图标
import { SlideHide24Filled } from '@vicons/fluent'

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
