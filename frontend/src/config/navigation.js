/**
 * 导航配置 - 从 App.vue 中提取的菜单配置
 * Phase 4 重构：消除研究中心臃肿父菜单，拆分为独立路由页面
 * 作为工厂函数提供，接受依赖注入以避免循环依赖
 *
 * 菜单梳理（2026-09-10）：底部横向菜单原来有 24 个一级项，靠 naive-ui 的
 * responsive 折叠成「···」，既难找又挤压空间。现按功能收敛为 6 个一级分组：
 *   自选股 / 市场行情 / 分析工具 / 研究资讯 / AI 能力 / 系统管理
 * 每个分组的子项保持原有的跳转与 EventsEmit 行为不变。
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

/** 一级分组 key（供 applyConfigVisibility / 高亮使用） */
export const MENU_GROUP_KEYS = {
  STOCK: 'stock',
  MARKET: 'market',
  ANALYSIS: 'analysis',
  RESEARCH: 'research',
  AI: 'ai',
  SYSTEM: 'system',
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

  /**
   * 生成一个「跳转到路由 + 可选触发子页面 tab 事件」的子项
   * @param {Object} cfg
   * @param {string} cfg.title 显示文案
   * @param {string} cfg.groupKey 所属一级分组 key（用于高亮）
   * @param {Object} cfg.to router 目标
   * @param {Function} [cfg.onClick] 额外点击行为（EventsEmit 等）
   * @param {any} [cfg.icon] 子项图标
   * @param {any} [cfg.key] 子项 key，默认用 to.name
   */
  function routeItem(cfg) {
    const { title, groupKey, to, onClick, icon, key } = cfg
    return {
      key: key ?? to.name,
      icon: icon ? renderIcon(icon) : undefined,
      label: () =>
        h(
          RouterLink,
          {
            to,
            onClick: () => {
              activeKey.value = groupKey
              if (onClick) onClick()
            },
          },
          { default: () => title }
        ),
    }
  }

  /** 生成一个「跳转到某个路由并切换其内部 tab」的 market 子项 */
  function marketItem(title, key, icon) {
    return routeItem({
      title,
      key,
      groupKey: MENU_GROUP_KEYS.MARKET,
      icon,
      to: { name: 'market', query: { name: title } },
      onClick: () => EventsEmit('changeMarketTab', { ID: 0, name: title }),
    })
  }

  /** 生成一个纯动作子项（全屏/隐藏/退出） */
  function actionItem(title, key, icon, onClick) {
    return {
      key,
      icon: renderIcon(icon),
      label: () => h('a', { href: '#', onClick }, { default: () => title }),
    }
  }

  return [
    // ===== 1. 自选股 =====
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
              activeKey.value = MENU_GROUP_KEYS.STOCK
            },
          },
          { default: () => '自选股' }
        ),
      key: MENU_GROUP_KEYS.STOCK,
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
                  activeKey.value = MENU_GROUP_KEYS.STOCK
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
        // 动态群组子菜单由 useNavigation.loadDynamicMenus() 插入到「全部」之后
        routeItem({
          title: '基金自选',
          key: 'fundFollow',
          groupKey: MENU_GROUP_KEYS.STOCK,
          icon: icons.StarOutline,
          to: { name: 'fund', query: { name: '基金自选' } },
          onClick: () => EventsEmit('changeFundTab', { name: '基金自选' }),
        }),
        routeItem({
          title: '基金排行',
          key: 'fundRanking',
          groupKey: MENU_GROUP_KEYS.STOCK,
          icon: icons.TrendingUp,
          to: { name: 'fund', query: { name: '基金排行' } },
          onClick: () => EventsEmit('changeFundTab', { name: '基金排行' }),
        }),
      ],
    },

    // ===== 2. 市场行情（原「市场行情」13 项 + 大宗商品 + 投资资讯）=====
    {
      label: () =>
        h(
          RouterLink,
          {
            href: '#',
            to: { name: 'market', params: {} },
            onClick: () => {
              activeKey.value = MENU_GROUP_KEYS.MARKET
              EventsEmit('changeMarketTab', { ID: 0, name: '市场快讯' })
            },
          },
          { default: () => '市场行情' }
        ),
      key: MENU_GROUP_KEYS.MARKET,
      icon: renderIcon(icons.NewspaperOutline),
      children: [
        marketItem('市场快讯', 'market1', icons.NewspaperSharp),
        marketItem('全球股指', 'market2', icons.BarChartSharp),
        marketItem('重大指数', 'market3', icons.AnalyticsOutline),
        marketItem('行业排名', 'market4', icons.Flag),
        marketItem('个股资金流向', 'market5', icons.Pulse),
        marketItem('板块资金流向', 'market5_1', icons.ReportMoney),
        marketItem('概念资金流向', 'market5_2', icons.TrendingUp),
        marketItem('龙虎榜', 'market6', icons.Dragon),
        marketItem('当前热门', 'market10', icons.Gripfire),
        marketItem('名站优选', 'market11', icons.FirefoxBrowser),
        marketItem('个股研报', 'market7', icons.StockOutlined),
        marketItem('公司公告', 'market8', icons.NotificationFilled),
        marketItem('行业研究', 'market9', icons.ReportSearch),
        routeItem({
          title: '大宗商品·行情总览',
          key: 'commodityOverview',
          groupKey: MENU_GROUP_KEYS.MARKET,
          icon: icons.DiamondOutline,
          to: { name: 'commodity', query: { name: '行情总览' } },
          onClick: () => EventsEmit('changeCommodityTab', { name: '行情总览' }),
        }),
        routeItem({
          title: '大宗商品·AI分析',
          key: 'commodityAnalysis',
          groupKey: MENU_GROUP_KEYS.MARKET,
          icon: icons.SparklesOutline,
          to: { name: 'commodity', query: { name: 'AI分析' } },
          onClick: () => EventsEmit('changeCommodityTab', { name: 'AI分析' }),
        }),
        routeItem({
          title: '投资资讯',
          key: 'news',
          groupKey: MENU_GROUP_KEYS.MARKET,
          icon: icons.NewspaperOutline,
          to: { name: 'news' },
        }),
      ],
    },

    // ===== 3. 分析工具 =====
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'klineAnalysis' },
            onClick: () => {
              activeKey.value = MENU_GROUP_KEYS.ANALYSIS
            },
          },
          { default: () => '分析工具' }
        ),
      key: MENU_GROUP_KEYS.ANALYSIS,
      icon: renderIcon(icons.StatsChartOutline),
      children: [
        routeItem({
          title: 'K线分析',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.StatsChartOutline,
          to: { name: 'klineAnalysis' },
        }),
        routeItem({
          title: '形态选股',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.SearchOutline,
          to: { name: 'analysisPattern' },
        }),
        routeItem({
          title: '指标选股',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.BoxSearch20Regular,
          to: { name: 'analysisScreening' },
        }),
        routeItem({
          title: '每日选股',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.TrendingUp,
          to: { name: 'dailyPick' },
        }),
        routeItem({
          title: '回测验证',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.AlarmOutline,
          to: { name: 'backtest' },
        }),
      ],
    },

    // ===== 4. 研究资讯 =====
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'researchReports' },
            onClick: () => {
              activeKey.value = MENU_GROUP_KEYS.RESEARCH
            },
          },
          { default: () => '研究资讯' }
        ),
      key: MENU_GROUP_KEYS.RESEARCH,
      icon: renderIcon(icons.ReportAnalytics),
      children: [
        routeItem({
          title: 'AI分析报告',
          groupKey: MENU_GROUP_KEYS.RESEARCH,
          icon: icons.ReportAnalytics,
          to: { name: 'researchReports' },
        }),
        routeItem({
          title: '股票推荐',
          groupKey: MENU_GROUP_KEYS.RESEARCH,
          icon: icons.Star,
          to: { name: 'researchRecommends' },
        }),
        routeItem({
          title: '异动监控',
          groupKey: MENU_GROUP_KEYS.RESEARCH,
          icon: icons.TrendingUp,
          to: { name: 'researchChanges' },
        }),
        routeItem({
          title: '涨停梯队',
          groupKey: MENU_GROUP_KEYS.RESEARCH,
          icon: icons.LocalFireDepartmentRound,
          to: { name: 'researchUplimit' },
        }),
        routeItem({
          title: '提示词',
          groupKey: MENU_GROUP_KEYS.RESEARCH,
          icon: icons.Prompt,
          to: { name: 'researchPrompts' },
        }),
      ],
    },

    // ===== 5. AI 能力（Ai智能体/技能管理 由 applyConfigVisibility 控制）=====
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'agent', query: { name: 'Ai智能体' } },
            onClick: () => {
              activeKey.value = MENU_GROUP_KEYS.AI
            },
          },
          { default: () => 'AI 能力' }
        ),
      key: MENU_GROUP_KEYS.AI,
      icon: renderIcon(icons.Robot),
      children: [
        routeItem({
          title: 'Ai智能体',
          key: 'agentChat',
          groupKey: MENU_GROUP_KEYS.AI,
          icon: icons.Robot,
          to: { name: 'agent', query: { name: 'Ai智能体' } },
        }),
        routeItem({
          title: '技能管理',
          key: 'systemSkills',
          groupKey: MENU_GROUP_KEYS.AI,
          icon: icons.FlashOutline,
          to: { name: 'systemSkills' },
        }),
        routeItem({
          title: 'MCP服务',
          key: 'systemMcp',
          groupKey: MENU_GROUP_KEYS.AI,
          icon: icons.ServerOutline,
          to: { name: 'systemMcp' },
        }),
      ],
    },

    // ===== 6. 系统管理 =====
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'settings', query: { name: '设置' } },
            onClick: () => {
              activeKey.value = MENU_GROUP_KEYS.SYSTEM
            },
          },
          { default: () => '系统管理' }
        ),
      key: MENU_GROUP_KEYS.SYSTEM,
      icon: renderIcon(icons.SettingsOutline),
      children: [
        routeItem({
          title: '设置',
          groupKey: MENU_GROUP_KEYS.SYSTEM,
          icon: icons.SettingsOutline,
          to: { name: 'settings', query: { name: '设置' } },
        }),
        routeItem({
          title: '数据管理',
          groupKey: MENU_GROUP_KEYS.SYSTEM,
          icon: icons.DiamondOutline,
          to: { name: 'data-manager' },
        }),
        routeItem({
          title: '定时任务',
          groupKey: MENU_GROUP_KEYS.SYSTEM,
          icon: icons.TimeOutline,
          to: { name: 'systemCron' },
        }),
        routeItem({
          title: '交易日志',
          groupKey: MENU_GROUP_KEYS.SYSTEM,
          icon: icons.MoneyCollectOutlined,
          to: { name: 'systemTrading' },
        }),
        actionItem(
          '全屏（Ctrl+F，Esc 退出）',
          'full',
          icons.ExpandOutline,
          toggleFullscreen
        ),
        actionItem('隐藏至托盘区', 'hide', icons.SlideHide24Filled, Hide),
        actionItem('退出程序', 'exit', icons.PowerOutline, Quit),
      ],
    },
  ]
}
