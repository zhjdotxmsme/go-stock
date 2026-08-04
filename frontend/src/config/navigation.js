/**
 * 导航配置 - 从 App.vue 中提取的菜单配置
 * Phase 4 重构：消除研究中心臃肿父菜单，拆分为独立路由页面
 * 作为工厂函数提供，接受依赖注入以避免循环依赖
 */

import { h } from 'vue'
import { RouterLink } from 'vue-router'

// 图标名称映射 (避免在配置文件中直接导入大量图标)
// 实际渲染时由 useNavigation composable 处理图标映射
export const ICON_NAMES = {
  // Ionicons5
  StarOutline: 'StarOutline',
  Star: 'Star',
  NewspaperOutline: 'NewspaperOutline',
  NewspaperSharp: 'NewspaperSharp',
  BarChartSharp: 'BarChartSharp',
  AnalyticsOutline: 'AnalyticsOutline',
  Flag: 'Flag',
  Pulse: 'Pulse',
  DiamondOutline: 'DiamondOutline',
  StatsChartOutline: 'StatsChartOutline',
  FlaskOutline: 'FlaskOutline',
  AlarmOutline: 'AlarmOutline',
  SearchOutline: 'SearchOutline',
  TimeOutline: 'TimeOutline',
  ServerOutline: 'ServerOutline',
  FlashOutline: 'FlashOutline',
  SettingsOutline: 'SettingsOutline',
  ExpandOutline: 'ExpandOutline',
  SparklesOutline: 'SparklesOutline',
  // Tabler
  ReportAnalytics: 'ReportAnalytics',
  ReportSearch: 'ReportSearch',
  ReportMoney: 'ReportMoney',
  TrendingUp: 'TrendingUp',
  Prompt: 'Prompt',
  // Fluent
  SlideHide24Filled: 'SlideHide24Filled',
  BoxSearch20Regular: 'BoxSearch20Regular',
  // Material
  LocalFireDepartmentRound: 'LocalFireDepartmentRound',
  // Ant Design
  MoneyCollectOutlined: 'MoneyCollectOutlined',
  NotificationFilled: 'NotificationFilled',
  StockOutlined: 'StockOutlined',
  // FA
  Dragon: 'Dragon',
  FirefoxBrowser: 'FirefoxBrowser',
  Gripfire: 'Gripfire',
  Robot: 'Robot',
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
 * @param {Function} deps.toggleFullscreen - 全屏切换函数
 * @param {Object} deps.icons - 图标映射对象
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
    toggleFullscreen,
    icons
  } = deps

  return [
    // 1. 股票自选
    {
      label: () =>
        h(
          RouterLink,
          {
            to: {
              name: 'stock',
              query: { groupName: '全部', groupId: 0 },
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
                    query: { groupName: '全部', groupId: 0 },
                  })
                  EventsEmit('changeTab', { ID: 0, name: '全部' })
                },
              },
              { default: () => '全部' }
            ),
          key: 0,
        },
        // 动态群组子菜单由 useNavigation.loadDynamicMenus() 注入
      ],
    },
    // 2. 市场行情
    {
      label: () =>
        h(
          RouterLink,
          {
            href: '#',
            to: { name: 'market', params: {} },
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
        // market1 - 市场快讯
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '市场快讯' } },
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
        // market2 - 全球股指
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '全球股指' } },
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
        // market3 - 重大指数
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '重大指数' } },
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
        // market4 - 行业排名
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '行业排名' } },
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
        // market5 - 个股资金流向
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '个股资金流向' } },
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
        // market5_1 - 板块资金流向
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '板块资金流向' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '板块资金流向' })
                },
              },
              { default: () => '板块资金流向' }
            ),
          key: 'market5_1',
          icon: renderIcon(icons.ReportMoney),
        },
        // market5_2 - 概念资金流向
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '概念资金流向' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '概念资金流向' })
                },
              },
              { default: () => '概念资金流向' }
            ),
          key: 'market5_2',
          icon: renderIcon(icons.TrendingUp),
        },
        // market6 - 龙虎榜
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '龙虎榜' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '龙虎榜' })
                },
              },
              { default: () => '龙虎榜' }
            ),
          key: 'market6',
          icon: renderIcon(icons.Dragon),
        },
        // market7 - 个股研报
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '个股研报' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '个股研报' })
                },
              },
              { default: () => '个股研报' }
            ),
          key: 'market7',
          icon: renderIcon(icons.StockOutlined),
        },
        // market8 - 公司公告
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '公司公告' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '公司公告' })
                },
              },
              { default: () => '公司公告' }
            ),
          key: 'market8',
          icon: renderIcon(icons.NotificationFilled),
        },
        // market9 - 行业研究
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '行业研究' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '行业研究' })
                },
              },
              { default: () => '行业研究' }
            ),
          key: 'market9',
          icon: renderIcon(icons.ReportSearch),
        },
        // market10 - 当前热门
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '当前热门' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '当前热门' })
                },
              },
              { default: () => '当前热门' }
            ),
          key: 'market10',
          icon: renderIcon(icons.Gripfire),
        },
        // market11 - 名站优选
        {
          label: () =>
            h(
              RouterLink,
              {
                href: '#',
                to: { name: 'market', query: { name: '名站优选' } },
                onClick: () => {
                  activeKey.value = 'market'
                  EventsEmit('changeMarketTab', { ID: 0, name: '名站优选' })
                },
              },
              { default: () => '名站优选' }
            ),
          key: 'market11',
          icon: renderIcon(icons.FirefoxBrowser),
        },
      ],
    },
    // 3. K线分析
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'klineAnalysis' },
            onClick: () => {
              activeKey.value = 'klineAnalysis'
            },
          },
          { default: () => 'K线分析' }
        ),
      key: 'klineAnalysis',
      icon: renderIcon(icons.StatsChartOutline),
    },
    // 4. 回测验证
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'backtest' },
            onClick: () => {
              activeKey.value = 'backtest'
            },
          },
          { default: () => '回测验证' }
        ),
      key: 'backtest',
      icon: renderIcon(icons.AlarmOutline),
    },
    // 5. 每日选股
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'dailyPick' },
            onClick: () => {
              activeKey.value = 'dailyPick'
            },
          },
          { default: () => '每日选股' }
        ),
      key: 'dailyPick',
      icon: renderIcon(icons.TrendingUp),
    },
    // 6. 数据管理
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'data-manager' },
            onClick: () => {
              activeKey.value = 'data-manager'
            },
          },
          { default: () => '数据管理' }
        ),
      key: 'data-manager',
      icon: renderIcon(icons.DiamondOutline),
    },
    // 7. 基金自选 (show 由 applyConfigVisibility 控制)
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'fund', query: { name: '基金自选' } },
            onClick: () => {
              activeKey.value = 'fund'
            },
          },
          { default: () => '基金自选' }
        ),
      show: false,
      key: 'fund',
      icon: renderIcon(icons.SparklesOutline),
      children: [
        {
          label: () =>
            h(
              RouterLink,
              {
                to: { name: 'fund', query: { name: '基金自选' } },
                onClick: () => {
                  activeKey.value = 'fund'
                  EventsEmit('changeFundTab', { name: '基金自选' })
                },
              },
              { default: () => '基金自选' }
            ),
          key: 'fundFollow',
          icon: renderIcon(icons.StarOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: { name: 'fund', query: { name: '基金排行' } },
                onClick: () => {
                  activeKey.value = 'fund'
                  EventsEmit('changeFundTab', { name: '基金排行' })
                },
              },
              { default: () => '基金排行' }
            ),
          key: 'fundRanking',
          icon: renderIcon(icons.TrendingUp),
        },
      ],
    },
    // 8. 大宗商品
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'commodity', query: { name: '行情总览' } },
            onClick: () => {
              activeKey.value = 'commodity'
            },
          },
          { default: () => '大宗商品' }
        ),
      key: 'commodity',
      icon: renderIcon(icons.DiamondOutline),
      children: [
        {
          label: () =>
            h(
              RouterLink,
              {
                to: { name: 'commodity', query: { name: '行情总览' } },
                onClick: () => {
                  activeKey.value = 'commodity'
                  EventsEmit('changeCommodityTab', { name: '行情总览' })
                },
              },
              { default: () => '行情总览' }
            ),
          key: 'commodityOverview',
          icon: renderIcon(icons.StatsChartOutline),
        },
        {
          label: () =>
            h(
              RouterLink,
              {
                to: { name: 'commodity', query: { name: 'AI分析' } },
                onClick: () => {
                  activeKey.value = 'commodity'
                  EventsEmit('changeCommodityTab', { name: 'AI分析' })
                },
              },
              { default: () => 'AI分析' }
            ),
          key: 'commodityAnalysis',
          icon: renderIcon(icons.SparklesOutline),
        },
      ],
    },
    // 9. 投资资讯
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'news' },
          onClick: () => { activeKey.value = 'news' },
        }, { default: () => '投资资讯' }),
      key: 'news',
      icon: renderIcon(icons.NewspaperOutline),
    },
    // 10. Ai智能体 (show 由 applyConfigVisibility 控制)
    {
      label: () =>
        h(
          RouterLink,
          {
            to: {
              name: 'agent',
              query: { name: 'Ai智能体' },
            },
            onClick: () => {
              activeKey.value = 'agent'
            },
          },
          { default: () => 'Ai智能体' }
        ),
      key: 'agent',
      show: false,
      icon: renderIcon(icons.Robot),
    },
    // ===== 研究中心拆分 → 独立扁平菜单项（原 12 子 tab）=====
    // 11. AI分析报告
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'researchReports' },
          onClick: () => { activeKey.value = 'researchReports' },
        }, { default: () => 'AI分析报告' }),
      key: 'researchReports',
      icon: renderIcon(icons.ReportAnalytics),
    },
    // 12. 股票推荐
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'researchRecommends' },
          onClick: () => { activeKey.value = 'researchRecommends' },
        }, { default: () => '股票推荐' }),
      key: 'researchRecommends',
      icon: renderIcon(icons.Star),
    },
    // 13. 异动监控
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'researchChanges' },
          onClick: () => { activeKey.value = 'researchChanges' },
        }, { default: () => '异动监控' }),
      key: 'researchChanges',
      icon: renderIcon(icons.TrendingUp),
    },
    // 14. 涨停梯队
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'researchUplimit' },
          onClick: () => { activeKey.value = 'researchUplimit' },
        }, { default: () => '涨停梯队' }),
      key: 'researchUplimit',
      icon: renderIcon(icons.LocalFireDepartmentRound),
    },
    // 15. 提示词
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'researchPrompts' },
          onClick: () => { activeKey.value = 'researchPrompts' },
        }, { default: () => '提示词' }),
      key: 'researchPrompts',
      icon: renderIcon(icons.Prompt),
    },
    // 16. 形态选股
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'analysisPattern' },
          onClick: () => { activeKey.value = 'analysisPattern' },
        }, { default: () => '形态选股' }),
      key: 'analysisPattern',
      icon: renderIcon(icons.SearchOutline),
    },
    // 17. 指标选股
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'analysisScreening' },
          onClick: () => { activeKey.value = 'analysisScreening' },
        }, { default: () => '指标选股' }),
      key: 'analysisScreening',
      icon: renderIcon(icons.BoxSearch20Regular),
    },
    // 18. 定时任务
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'systemCron' },
          onClick: () => { activeKey.value = 'systemCron' },
        }, { default: () => '定时任务' }),
      key: 'systemCron',
      icon: renderIcon(icons.TimeOutline),
    },
    // 19. 交易日志
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'systemTrading' },
          onClick: () => { activeKey.value = 'systemTrading' },
        }, { default: () => '交易日志' }),
      key: 'systemTrading',
      icon: renderIcon(icons.MoneyCollectOutlined),
    },
    // 20. MCP服务
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'systemMcp' },
          onClick: () => { activeKey.value = 'systemMcp' },
        }, { default: () => 'MCP服务' }),
      key: 'systemMcp',
      icon: renderIcon(icons.ServerOutline),
    },
    // 21. 技能管理
    {
      label: () =>
        h(RouterLink, {
          to: { name: 'systemSkills' },
          onClick: () => { activeKey.value = 'systemSkills' },
        }, { default: () => '技能管理' }),
      key: 'systemSkills',
      icon: renderIcon(icons.FlashOutline),
    },
    // ===== 原有辅助菜单 =====
    // 22. 设置
    {
      label: () =>
        h(
          RouterLink,
          {
            to: {
              name: 'settings',
              query: { name: '设置' },
            },
            onClick: () => {
              activeKey.value = 'settings'
            },
          },
          { default: () => '设置' }
        ),
      key: 'settings',
      icon: renderIcon(icons.SettingsOutline),
    },
    // 23. 全屏 (隐藏)
    {
      show: false,
      label: () => h('a', {
        href: '#',
        onClick: toggleFullscreen,
        title: '全屏 Ctrl+F 退出全屏 Esc',
      }, { default: () => (isFullscreen.value ? '取消全屏' : '全屏') }),
      key: 'full',
      icon: renderIcon(icons.ExpandOutline),
    },
    // 24. 隐藏至托盘区
    {
      label: () => h('a', {
        href: '#',
        onClick: Hide,
      }, { default: () => '隐藏至托盘区' }),
      key: 'hide',
      icon: renderIcon(icons.SlideHide24Filled),
    },
    // 25. 退出程序
    {
      label: () => h('a', {
        href: '#',
        onClick: Quit,
      }, { default: () => '退出程序' }),
      key: 'exit',
      icon: renderIcon(icons.PowerOutline),
    },
  ]
}
