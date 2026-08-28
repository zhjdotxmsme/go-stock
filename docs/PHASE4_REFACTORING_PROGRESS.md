# Phase 4 前端重构进度报告

## 已完成 ✅ (2026-08-03)

### 1. 核心基础设施

**状态管理 - Pinia**
- ✅ `src/stores/app.js` - 应用全局状态 Store (129 行)
  - 加载状态、主题、市场状态、投资格言
  - `setLoading`, `toggleDarkTheme`, `refreshMotto` 方法
- ✅ `src/stores/stock.js` - 股票相关状态 Store (152 行)
  - 自选股列表、群组、实时盈亏
  - `addStock`, `removeStock`, `updateStockPrice`, `cacheStockDetail` 方法
- ✅ `src/stores/settings.js` - 设置相关状态 Store (230 行)
  - Tushare Token、推送配置、功能开关、AI配置
  - `loadSettings`, `exportSettings`, `saveAiConfig` 方法
- ✅ `src/stores/index.js` - Store 统一入口
- ✅ `package.json` - 添加 pinia 2.3.1 依赖
- ✅ `src/main.js` - Pinia 注册完成

**API 层 - 统一封装**
- ✅ `src/api/client.js` - API 客户端封装 (67 行)
  - 标准化返回格式 `{ success, data, error, message }`
  - 统一错误处理和日志记录
- ✅ `src/api/stock.js` - 股票相关 API (168 行)
  - 自选股 API: `getStockList`, `addStock`, `removeStock`
  - 群组 API: `getGroupList`, `addGroup`, `removeStockFromGroup`
  - K 线 API: `getStockKLine`, `getEastMoneyKLine`, `getStockKLineWithFallback`
  - 技术指标 API: `getChipDistribution`, `getCompanyInfo`
  - 交易 API: `setCostPriceAndVolume`, `setAlarmChangePercent`
- ✅ `src/api/market.js` - 市场行情 API (128 行)
  - 市场快讯、全球股指、行业排名、个股资金流向、板块资金流向
  - 龙虎榜、异动监控、财经电报、交易时间检查
- ✅ `src/api/system.js` - 系统管理 API (224 行)
  - 版本信息、设置配置、AI配置
  - Cron 定时任务、MCP 服务管理、技能管理
  - 提示词模板管理、日志管理、数据导入导出
- ✅ `src/api/index.js` - API 统一出口

**Composables 层**
- ✅ `src/composables/useNavigation.js` - 导航逻辑 (89 行)
  - 菜单配置管理、激活状态、全屏切换
  - 统一图标渲染函数
- ✅ `src/composables/useMarketStatus.js` - 市场状态管理 (91 行)
  - A/HK/US 三市场交易时间检查
  - 30秒自动更新
  - 窗口标题同步
- ✅ `src/composables/useWailsEvents.js` - Wails 事件监听 (59 行)
  - `realtime_profit`, `telegraph`, `loadingMsg`, `newsPush` 事件处理
  - 自动清理事件监听器

**配置层**
- ✅ `src/config/navigation.js` - 导航配置 (232 行)
  - 所有菜单项抽离为配置
  - 工厂函数模式，支持依赖注入避免循环依赖
  - 完整的菜单项定义：股票自选、市场行情、研究中心、设置

**工具函数**
- ✅ `src/utils/logger.js` - 分级日志工具 (86 行)
  - DEBUG/INFO/WARN/ERROR 四个级别
  - 性能计时 API (`time`, `timeEnd`)

### 2. 文档与迁移指南

- ✅ `docs/PHASE4_FRONTEND_REFACTOR.md` - Phase 4 完整文档 (271 行)
  - 重构背景、设计决策、目录结构说明
  - 迁移指南：如何在组件中使用 Store/API/Composables
  - 待完成事项清单
- ✅ `src/App.vue.refactored.template` - App.vue 重构参考模板 (116 行)
  - 展示重构后 App.vue 的目标结构
  - 从 1279 行 精简到 ~150 行
  - 演示 Stores + Composables 的组合使用

---

## 待完成 📋

### P0 - 核心迁移（上线前必须）✅ 全部完成（2026-08-28）

#### 1. npm 依赖安装 ✅
pinia 已安装并在 main.js 注册（app.use(pinia)）。

#### 2. App.vue 迁移 ✅
App.vue 已迁移至 165 行（原 1,279 行），状态/事件/菜单经 Pinia Stores 与
useWailsEvents/useMarketStatus/useNavigation 组合。

#### 3. 现有组件逐步接入 API 层 ✅（2026-08-28）
- `components/stock.vue` / `components/market.vue`：数据调用全部经 api 层
- 新增 `api/backtest.ts`（backtest.Service + DailyPickBacktestService）与
  `api/dailyPick.ts`（DailyPickHandler）
- BacktestPanel / DataManager / DailyPickPanel 迁移至上述 api 封装
- OpenURL/RestartAsAdmin 收口至 `api.system.openURL/restartAsAdmin`，
  6 处直连（stock/FundFollow/FundRanking/HotTopics/SelectStock/useStockEvents）清除
- 验收：`grep -r "wailsjs/go" src/components/ src/views/`（豁免 OpenURL/
  RestartAsAdmin/models 类型外）= 0；计划 Step8 验收命令 = 0

### P1 - 架构优化

#### 4. 路由结构扁平化
当前 Research Center 有 12 个子 Tab 嵌套在单个路由下
目标: 拆分为独立路由，支持浏览器后退和直接访问
```
/research/reports      - AI分析报告
/research/recommends   - AI推荐
/research/changes      - 异动监控
/research/uplimit      - 涨停梯队
/system/cron           - 定时任务
/system/trading        - 交易日志
/system/mcp            - MCP服务
/system/skills         - 技能管理
/system/prompts        - 提示词
/analysis/screening    - 指标选股
```

#### 5. 超大组件拆分（按行数降序优先级）
| 组件 | 当前行数 | 目标拆分方案 |
|------|----------|-------------|
| `StockLightweightKlineChart.vue` | 4,832 | `KLineChart.vue` + `KLineToolbar.vue` + 子指标组件 |
| `stock.vue` | 3,151 | `StockWatchList.vue` + `StockDetail.vue` + `StockAIChat.vue` |
| `AnalyzeMartket.vue` | 2,032 | `MarketStats.vue` + `ChangeRanking.vue` |
| `FloatingAgentAssistant.vue` | 1,954 | `AgentDrawer.vue` + `AgentSession.vue` |

#### 6. 重复组件合并
- `bkFundFlowChart.vue` + `conceptFundFlowChart.vue` → `FundFlowChart.vue` (参数化板块类型)
- `InvestCalendarTimeLine.vue` + `ClsCalendarTimeLine.vue` → `CalendarTimeline.vue`

### P2 - 体验优化

#### 7. Store 持久化
- 使用 `localStorage` 或 IndexedDB 持久化设置 Store
- 自选股列表缓存

#### 8. TypeScript 类型支持
- 为 API 返回值添加类型定义
- 为 Store State 添加类型定义
- 为 Wails 绑定添加类型安全

---

## 代码统计

### Phase 4 新增代码 (截至目前)
| 文件类型 | 文件数 | 新增行数 |
|---------|-------|---------|
| Stores | 4 | 511 行 |
| API 层 | 4 | 587 行 |
| Composables | 3 | 239 行 |
| 配置文件 | 1 | 232 行 |
| 工具函数 | 1 | 86 行 |
| 文档 | 2 | ~390 行 |
| **合计** | **15** | **2,045 行** |

### 后续 Phase 概览

| Phase | 主题 | 核心内容 |
|-------|------|---------|
| Phase 5 | 后端 Service 层 | 提取业务逻辑到 Service，解耦 Handler 和数据层 |
| Phase 6 | DSA 量化选股 | 9因子评分、LLM重排序、独立风控叠加层 |
| Phase 7 | Agent 风控辩论 + 学习系统 | 多方/空方/中立三方辩论、判决器、反思记忆系统 |
| Phase 8 | 选股高级功能 + 最终清理 | 瀑布过滤、热点生命周期、种子轮换、数据层清理 |

---

## 迁移最佳实践

### 原则 1: 渐进式迁移，不破坏现有功能
- 新架构与旧代码可共存
- 按模块逐步替换，一次性替换风险高
- 保持 API 兼容，外部调用方式不变

### 原则 2: 先建基础设施，再迁移业务
✅ Pinia Stores 基础设施已建
✅ API 层封装已建
✅ Composables 机制已建
⏳ 可以开始迁移业务逻辑了

### 原则 3: 每一步都可回滚
- 保留旧代码注释，不立即删除
- 用 `// TODO: migrate to store` 标记待迁移点
- 确保任何时候都能构建成功

---

## 下一步建议

### 本周可以完成的增量:
1. **Day 1**: `npm install` 验证，修复可能的编译错误
2. **Day 2-3**: App.vue 核心状态迁移到 Pinia
3. **Day 4-5**: stock.vue 接入 API 层和 Stock Store
4. **Day 6-7**: market.vue 接入 API 层，完成核心页面迁移

完成后 Phase 4 架构基本落地，后续可按节奏拆分超大组件。
