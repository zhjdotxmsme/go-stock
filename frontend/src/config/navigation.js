/**
 * 导航配置 - 从 App.vue 中提取的菜单配置
 * 作为工厂函数提供，接受依赖注入以避免循环依赖
 */

import { h } from 'vue'
import { RouterLink } from 'vue-router'

// 图标名称映射 (避免在配置文件中直接导入大量图标)
// 实际渲染时由 useNavigation composable 处理图标映射
export const ICON_NAMES = {
  StarOutline: 'StarOutline',
  NewspaperOutline: 'NewspaperOutline',
  NewspaperSharp: 'NewspaperSharp',
  BarChartSharp: 'BarChartSharp',
  AnalyticsOutline: 'AnalyticsOutline',
  Flag: 'Flag',
  Pulse: 'Pulse',
  DiamondOutline: 'DiamondOutline',
  StatsChartOutline: 'StatsChartOutline',
  FlaskOutline: 'FlaskOutline',
  GlobeOutline: 'GlobeOutline',
  BonfireOutline: 'BonfireOutline',
  SparklesOutline: 'SparklesOutline',
  Robot: 'Robot',
  FlameSharp: 'FlameSharp',
  ReportSearch: 'ReportSearch',
  ReportAnalytics: 'ReportAnalytics',
  MoneyCollectOutlined: 'MoneyCollectOutlined',
  ServerOutline: 'ServerOutline',
  FlashOutline: 'FlashOutline',
  SettingsOutline: 'SettingsOutline',
  ExpandOutline: 'ExpandOutline',
  SlideHide24Filled: 'SlideHide24Filled',
  PowerOutline: 'PowerOutline',
}

/**
 * 创建菜单配置
 * @param {Object} deps - 依赖注入
 * @param {Object} deps.activeKey - 响应式 activeKey ref
 * @param {Object} deps.router - vue-router 实例
 * @param {Function} deps.EventsEmit - 事件发射函数
 * @param {Function} deps.renderIcon - 图标渲染函数
 * @param {Object} deps.isFullscreen - 全屏状态 ref
 * @param {Function} deps.Quit - 退出函数
 * @param {Function} deps.Hide - 隐藏函数
 * @returns {Array} 菜单配置数组
 */
export function createMenuOptions(deps) {
  const {
    activeKey,
    router,
    EventsEmit,
    renderIcon,
    isFullscreen,
    Quit,
    Hide,
    icons
  } = deps

  return [
    {
      label: () =>
        h(
          RouterLink,
          {
            to: {
              name: 'stock',
              query: {
                groupName: '全部',
                groupId: 0,
              },
              params: {},
            },
            onClick: () => {
              activeKey.value = 'stock'
            },
          },
          { default: () => '股票自选' }
        ),
      key: 'stock',
      icon: renderIcon(icons.StarOutline),
      children: [
        {
          label: () =>
            h(
              'a',
              {
                href: '#',
                type: 'info',
                onClick: () => {
                  activeKey.value = 'stock'
                  router.push({
                    name: 'stock',
                    query: {
                      groupName: '全部',
                      groupId: 0,
                    },
                  })
                  EventsEmit('changeTab', { ID: 0, name: '全部' })
                },
                to: {
                  name: 'stock',
                  query: {
                    groupName: '全部',
                    groupId: 0,
                  },
                },
              },
              { default: () => '全部' }
            ),
          key: 0,
        },
      ],
    },
    {
      label: () =>
        h(
          RouterLink,
          {
            href: '#',
            to: {
              name: 'market',
              params: {},
            },
            onClick: () => {
              activeKey.value = 'market'
              EventsEmit('changeMarketTab', { ID: 0, name: '市场快讯' })
            },
          },
          { default: () => '市场行情' }
        ),
      key: 'market',
      icon: renderIcon(icons.NewspaperOutline),
      children: [
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '市场快讯',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '市场快讯' })
                },
              },
              { default: () => '市场快讯' }
            ),
          key: 'market1',
          icon: renderIcon(icons.NewspaperSharp),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '全球股指',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '全球股指' })
                },
              },
              { default: () => '全球股指' }
            ),
          key: 'market2',
          icon: renderIcon(icons.BarChartSharp),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '重大指数',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '重大指数' })
                },
              },
              { default: () => '重大指数' }
            ),
          key: 'market3',
          icon: renderIcon(icons.AnalyticsOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '行业排名',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '行业排名' })
                },
              },
              { default: () => '行业排名' }
            ),
          key: 'market4',
          icon: renderIcon(icons.Flag),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '个股资金流向',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '个股资金流向' })
                },
              },
              { default: () => '个股资金流向' }
            ),
          key: 'market5',
          icon: renderIcon(icons.Pulse),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '板块资金流向',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '板块资金流向' })
                },
              },
              { default: () => '板块资金流向' }
            ),
          key: 'market6',
          icon: renderIcon(icons.DiamondOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: {
                  name: 'market',
                  query: {
                    name: '龙虎榜',
                  },
                },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '龙虎榜' })
                },
              },
              { default: () => '龙虎榜' }
            ),
          key: 'market7',
          icon: renderIcon(icons.StatsChartOutline),
        },
      ],
    },
    {
      label: () =>
        h(
          RouterLink,
          {
            to: {
              name: 'research',
            },
            onClick: () => {
              activeKey.value = 'research'
              setTimeout(() => {
                EventsEmit('changeResearchTab', { ID: 0, name: 'AI分析报告' })
              }, 100)
            },
          },
          { default: () => '研究中心' }
        ),
      key: 'research',
      icon: renderIcon(icons.FlaskOutline),
      children: [
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 0, name: 'AI分析报告' })
                  }, 100)
                },
              },
              { default: () => 'AI分析报告' }
            ),
          key: 'aiAnalysisReport',
          icon: renderIcon(icons.ReportSearch),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 1, name: '异动监控' })
                  }, 100)
                },
              },
              { default: () => '异动监控' }
            ),
          key: 'stockChanges',
          icon: renderIcon(icons.WarningOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 2, name: '涨停梯队' })
                  }, 100)
                },
              },
              { default: () => '涨停梯队' }
            ),
          key: 'dailyLimit',
          icon: renderIcon(icons.FlameSharp),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 3, name: '选股策略' })
                  }, 100)
                },
              },
              { default: () => '选股策略' }
            ),
          key: 'screening',
          icon: renderIcon(icons.ReportAnalytics),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 4, name: '定时任务' })
                  }, 100)
                },
              },
              { default: () => '定时任务' }
            ),
          key: 'cronTasks',
          icon: renderIcon(icons.TimeOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 5, name: '交易日志' })
                  }, 100)
                },
              },
              { default: () => '交易日志' }
            ),
          key: 'tradingJournal',
          icon: renderIcon(icons.MoneyCollectOutlined),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 6, name: '提示词' })
                  }, 100)
                },
              },
              { default: () => '提示词' }
            ),
          key: 'prompts',
          icon: renderIcon(icons.SparklesOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 7, name: 'MCP服务' })
                  }, 100)
                },
              },
              { default: () => 'MCP服务' }
            ),
          key: 'mcpServers',
          icon: renderIcon(icons.ServerOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: {
                  name: 'research',
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit('changeResearchTab', { ID: 8, name: '技能管理' })
                  }, 100)
                },
              },
              { default: () => '技能管理' }
            ),
          key: 'skills',
          icon: renderIcon(icons.FlashOutline),
          show: true,
        },
      ],
    },
    {
      label: () =>
        h(
          RouterLink,
          {
            to: {
              name: 'settings',
              query: {
                name: '设置',
              },
              onClick: () => {
                activeKey.value = 'settings'
              },
            },
          },
          { default: () => '设置' }
        ),
      key: 'settings',
      icon: renderIcon(icons.SettingsOutline),
    },
    {
      show: false,
      label: () =>
        h(
          'a',
          {
            href: '#',
            onClick: () => {},
            title: '全屏 Ctrl+F 退出全屏 Esc',
          },
          { default: () => (isFullscreen.value ? '取消全屏' : '全屏') }
        ),
      key: 'full',
      icon: renderIcon(icons.ExpandOutline),
    },
    {
      label: () =>
        h(
          'a',
          {
            href: '#',
            onClick: Hide,
          },
          { default: () => '隐藏至托盘区' }
        ),
      key: 'hide',
      icon: renderIcon(icons.SlideHide24Filled),
    },
    {
      label: () =>
        h(
          'a',
          {
            href: '#',
            onClick: Quit,
          },
          { default: () => '退出程序' }
        ),
      key: 'exit',
      icon: renderIcon(icons.PowerOutline),
    },
  ]
}
