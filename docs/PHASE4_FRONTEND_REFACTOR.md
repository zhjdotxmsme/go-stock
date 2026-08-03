# Phase 4: 前端重构

## 概述

Phase 4 旨在将 `App.vue` 中的大量代码拆分为模块化结构，引入状态管理（Pinia）和统一的 API 层，提升代码可维护性和可扩展性。

## 重构进度

### ✅ 已完成

#### 1. 导航配置拆分
**文件**: `src/config/navigation.js`
- 提取 `menuOptions` 配置为独立模块
- 采用工厂函数模式，支持依赖注入避免循环依赖
- 集中管理所有导航图标和菜单项

**文件**: `src/composables/useNavigation.js`
- 导航逻辑 Composable
- 封装图标渲染、菜单创建、全屏切换等逻辑

#### 2. Pinia 状态管理建立
**文件**: `src/stores/app.js`
- 应用全局状态：加载状态、主题、市场状态、投资格言等
- 封装加载控制、主题切换、市场状态更新方法

**文件**: `src/stores/stock.js`
- 股票相关状态：自选股列表、群组、实时盈亏
- 封装股票增删、群组切换、价格更新、缓存管理等方法

**文件**: `src/stores/settings.js`
- 设置相关状态：Tushare Token、推送配置、功能开关、AI配置、浏览器设置
- 封装设置导入导出、AI配置管理等方法

#### 3. API 层建立
**文件**: `src/api/client.js`
- 统一 API 调用封装
- 标准化错误处理和日志记录
- 提供 `callApi` 和 `createApiClient` 工具函数

**文件**: `src/api/stock.js`
- 自选股相关 API
- 群组管理 API
- K 线数据 API
- 技术指标 API
- 交易相关 API

**文件**: `src/api/market.js`
- 市场行情 API
- 异动监控 API
- 电报/新闻 API
- 交易时间 API
- 实时价格 API

**文件**: `src/api/system.js`
- 版本信息 API
- 设置配置 API
- AI 配置 API
- Cron 定时任务 API
- MCP 服务 API
- 技能管理 API
- 提示词模板 API

**文件**: `src/api/index.js`
- API 统一入口
- 便于导入和使用

#### 4. 工具函数
**文件**: `src/utils/logger.js`
- 统一分级日志工具
- 支持 DEBUG/INFO/WARN/ERROR 级别
- 支持性能计时

### 📋 待完成

#### 5. main.js Pinia 注册
需要在 `main.js` 中注册 Pinia 插件：
```javascript
import { createPinia } from 'pinia'
app.use(createPinia())
```

#### 6. App.vue 简化
将 `App.vue` 中的状态和方法替换为 Pinia Store 和 Composables：
- `loading`, `loadingMsg` → `useAppStore()`
- `groupList`, `realtimeProfit` → `useStockStore()`
- `menuOptions` → `useNavigation()`
- 事件监听器移至 Composables

#### 7. 路由结构优化
当前 Research Center 有 12 个子 Tab，需要拆分为独立路由：
- `/research/reports` - AI分析报告
- `/research/recommends` - AI推荐
- `/research/changes` - 异动监控
- `/research/uplimit` - 涨停梯队
- `/system/cron` - 定时任务
- `/system/trading` - 交易日志
- `/system/mcp` - MCP服务
- `/system/skills` - 技能管理
- `/system/prompts` - 提示词
- `/analysis/screening` - 指标选股

#### 8. 超大组件拆分
按行数优先级拆分：
1. `StockLightweightKlineChart.vue` (4,832行) → `KLineChart.vue` + `KLineToolbar.vue` + 指标子组件
2. `stock.vue` (3,151行) → `StockWatchList.vue` + `StockDetail.vue` + `StockAIChat.vue`
3. `AnalyzeMartket.vue` (2,032行) → `MarketStats.vue` + `ChangeRanking.vue`
4. `FloatingAgentAssistant.vue` (1,954行) → `AgentDrawer.vue` + `AgentSession.vue`

#### 9. 重复组件合并
- `bkFundFlowChart.vue` + `conceptFundFlowChart.vue` → `FundFlowChart.vue` (参数化板块类型)
- `InvestCalendarTimeLine.vue` + `ClsCalendarTimeLine.vue` → `CalendarTimeline.vue`

## 目录结构

### 重构后目录
```
frontend/src/
├── api/                      # API 层
│   ├── client.js             # API 客户端封装
│   ├── stock.js              # 股票相关 API
│   ├── market.js             # 市场行情相关 API
│   ├── system.js             # 系统管理相关 API
│   └── index.js              # API 统一入口
├── composables/              # 组合式函数
│   └── useNavigation.js      # 导航逻辑
├── config/                   # 配置文件
│   └── navigation.js         # 导航配置
├── stores/                   # Pinia 状态管理
│   ├── app.js                # 应用全局状态
│   ├── stock.js              # 股票相关状态
│   ├── settings.js           # 设置相关状态
│   └── index.js              # Store 统一入口
├── utils/                    # 工具函数
│   ├── logger.js             # 日志工具
│   └── stockCode.js          # 股票代码工具
└── components/               # 组件 (待拆分)
```

## 迁移指南

### 在组件中使用 Store
```javascript
import { useAppStore, useStockStore, useSettingsStore } from '@/stores'

const appStore = useAppStore()
const stockStore = useStockStore()
const settingsStore = useSettingsStore()

// 使用状态
console.log(appStore.loading)
console.log(stockStore.stockList)

// 调用方法
appStore.setLoading(false)
stockStore.addStock({ code: 'sh600519', name: '贵州茅台' })
```

### 在组件中使用 API
```javascript
import api from '@/api'

// 获取股票列表
const result = await api.stock.getStockList()
if (result.success) {
  console.log(result.data)
}

// 获取市场新闻
const newsResult = await api.market.getMarketNews(50)
```

### 使用导航 Composable
```javascript
import { useNavigation } from '@/composables/useNavigation'

const { activeKey, menuOptions, toggleFullscreen } = useNavigation()
```

## 关键设计决策

### 1. 为什么选择 Pinia？
- Vue 官方推荐，替代 Vuex
- 完整的 TypeScript 支持
- 更简洁的 API，更少的样板代码
- 支持热模块替换
- 支持时间旅行调试

### 2. API 层设计原则
- 统一的错误处理：所有调用返回 `{ success, data, error }` 格式
- 语义化命名：`getStockList`、`addStock`、`removeStock`
- 模块化拆分：按领域（stock/market/system）拆分

### 3. Store 设计原则
- 按领域拆分：app/stock/settings，而不是按功能
- 单一职责：每个 Store 管理特定领域的状态
- 避免循环依赖：Store 之间避免相互引用

## 后续计划

Phase 4 完成后，将继续：
- Phase 5: 后端 Service 层建立
- Phase 6: DSA 量化选股增强
- Phase 7: Agent 风控辩论 + 学习系统
- Phase 8: 选股高级功能 + 最终清理
