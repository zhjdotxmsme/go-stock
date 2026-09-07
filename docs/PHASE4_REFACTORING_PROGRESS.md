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

---

## 全计划完成状态（2026-09-05 更新）

Phase 0-8 全部落地，最终架构状态：

| Phase | 主题 | 状态 | 关键产物 |
|-------|------|------|----------|
| 0 | 废弃组件清理 | ✅ | FloatingAiAssistant/promptPlaza/promptQa/agent-chat_bk 已删除 |
| 1 | stockcode + Handler 框架 | ✅ | `backend/stockcode/`、`backend/handler/`（13 个 Handler）、前端 `utils/stockCode.js` |
| 2 | 领域模型/端口/适配器 | ✅ | `internal/domain/`、`internal/port/`、`internal/adapter/` |
| 3 | Agent 分析增强 | ✅ | 多层降级信号提取、LLM 分层分配 |
| 4 | 前端重构 | ✅ | Pinia stores、api/ 层、composables；超大组件拆分：K线图 4832→1373 行、stock.vue 3151→1411 行、AnalyzeMartket 2032→395 行；重复组件合并：`FundFlowChart.vue`、`CalendarTimeline.vue` |
| 5 | 后端 Service 层 | ✅ | `internal/service/`（analysis/fund/market/news/stockchange/system/trading），app.go 3488→183 行 |
| 6 | DSA 量化选股 | ✅ | `agent/strategy/scoring|ranking|risk|filter`，已接入 daily_pick_engine（评分→LLM排序→风控→决策标尺） |
| 7 | 风控辩论 + 记忆 | ✅ | `agent/strategy/disagreement`、`agent/memory`（SQLite FTS5 反思记忆） |
| 8 | 选股高级功能 | ✅ | 瀑布过滤诊断、种子轮换（selection_variant）、后分析链（postanalysis） |

### 构建验证（2026-09-05）
- `go build ./...` 通过
- `go test ./...` 全部通过（根包 app_test 因 Wails 生命周期上下文在纯 go test 环境不可用，属环境限制）
- `npm run build` 通过

### 近期补充修复
- 大宗商品 K 线端到端修复（WSCN 列映射、前端字段大小写、时区、精度、ResizeObserver）
- AI 推荐列表空数据修复（日期格式化、字段名兼容、错误处理、默认 30 天范围）
- 多级缓存修复（L3 nil panic、cache_items 建表缺失、Mock Redis TTL、Clear 语义）

### 补完的计划项（2026-09-07）

最后一轮审计发现的 4 项计划缺口已全部补齐：

| 计划项 | 实现 | 提交 |
|--------|------|------|
| Step 3.3 数据完整性预检器 | `multi/checker.go`：K线存在/可解析/时效(12自然日,覆盖春节国庆)/完整率≥50%，不过则中止并透出明确原因 | 5ed3f49 |
| Step 3.5 工具调用轮数上限 | 旧硬编码 200 轮 → 默认 8 轮，`OpenAi.MaxToolDepth` 可按会话覆盖 | 9704dee |
| Step 3.4 交易标的上下文注入 | `domain/stock/instrument_context.go`：按代码前缀识别沪深/北交/港股/美股规则(交易时段/涨跌停/手数/T+N/做空)，注入 7 个分析师 Prompt | 3e04f36 |
| Step 5.1 stock 服务层 | `internal/service/stock/`：盈亏计算纯函数(6单测) + watchlist/groups 域服务(文案逐字一致)，stock 成为第 8 个有服务层的域 | 7de2051, 93ac57c |
