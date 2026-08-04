import {createMemoryHistory, createRouter, createWebHashHistory, createWebHistory} from 'vue-router'

import stockView from '../components/stock.vue'
import settingsView from '../components/settings.vue'
import aboutView from "../components/about.vue";
import fundView from "../components/fund.vue";
import marketView from "../components/market.vue";
import agentChat from "../components/agent-chat.vue"
import klineAnalysis from "../components/kline-analysis.vue"
import backtestPanel from "../components/BacktestPanel.vue"
import dataManager from "../components/DataManager.vue"
import dailyPickPanel from "../components/DailyPickPanel.vue"
import commodityView from "../components/commodity.vue"
import newsView from '../components/NewsPage.vue'

// 研究中心拆分后的独立页面组件
import researchReport from "../components/researchReport.vue"
import aiRecommendStocksList from "../components/aiRecommendStocksList.vue"
import stockChangesMonitor from "../components/stockChangesMonitor.vue"
import uplimitLadder from "../components/uplimitLadder.vue"
import promptTemplateList from "../components/promptTemplateList.vue"
import allStockList from "../components/allStockList.vue"
import SelectStock from "../components/SelectStock.vue"

// 系统管理组件
import cronTaskManager from "../components/cron-task-manager.vue"
import TradingRecordManager from "../components/TradingRecordManager.vue"
import mcpServerManager from "../components/mcp-server-manager.vue"
import skillManager from "../components/skill-manager.vue"

const routes = [
    // 核心页面
    { path: '/', component: stockView, name: 'stock' },
    { path: '/fund', component: fundView, name: 'fund' },
    { path: '/market', component: marketView, name: 'market' },
    { path: '/agent', component: agentChat, name: 'agent' },
    { path: '/settings', component: settingsView, name: 'settings' },
    { path: '/about', component: aboutView, name: 'about' },

    // 分析工具
    { path: '/kline-analysis', component: klineAnalysis, name: 'klineAnalysis' },
    { path: '/backtest', component: backtestPanel, name: 'backtest' },
    { path: '/daily-pick', component: dailyPickPanel, name: 'dailyPick' },

    // 数据 & 资讯
    { path: '/data-manager', component: dataManager, name: 'data-manager' },
    { path: '/commodity', component: commodityView, name: 'commodity' },
    { path: '/news', component: newsView, name: 'news' },

    // 研究中心 — 拆分为独立路由（原 researchIndex.vue 12 子 tab）
    { path: '/research/reports', component: researchReport, name: 'researchReports' },
    { path: '/research/recommends', component: aiRecommendStocksList, name: 'researchRecommends' },
    { path: '/research/changes', component: stockChangesMonitor, name: 'researchChanges' },
    { path: '/research/uplimit', component: uplimitLadder, name: 'researchUplimit' },
    { path: '/research/prompts', component: promptTemplateList, name: 'researchPrompts' },
    { path: '/analysis/pattern', component: allStockList, name: 'analysisPattern' },
    { path: '/analysis/screening', component: SelectStock, name: 'analysisScreening' },

    // 系统管理
    { path: '/system/cron', component: cronTaskManager, name: 'systemCron' },
    { path: '/system/trading', component: TradingRecordManager, name: 'systemTrading' },
    { path: '/system/mcp', component: mcpServerManager, name: 'systemMcp' },
    { path: '/system/skills', component: skillManager, name: 'systemSkills' },
]

const router = createRouter({
    //history: createWebHistory(),
    history: createWebHashHistory(),
    routes,
})

export default router
