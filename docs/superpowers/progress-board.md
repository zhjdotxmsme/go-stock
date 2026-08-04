# go-stock 重构进度看板

> 自动生成于 2026-08-04，基于 `feat/multi-agent-analysis` 分支实际仓库状态。
> 关联方案：`go-stock-全面重构方案.md`（v2.1）

---

## 一、总体进度

| 维度 | 当前值 | 目标值 | 状态 |
|------|--------|--------|------|
| `app.go` 行数 | 3,488 | < 200 | ❌ 尚未开始缩减 |
| `backend/data/` 文件数 | 126 | < 15（按子包拆分后） | ❌ 仍是上帝包 |
| 后端 Handler 拆分 | 1/9 | 9/9 | 🟡 仅 StockHandler 启动 |
| 前端 API 抽象层 | ✅ 完成 | 完成 | ✅ |
| 前端 Pinia 状态管理 | 4 stores | 覆盖核心模块 | 🟡 基础框架完成 |
| 股票代码归一化 | `backend/stockcode/` 已建 | 全链路统一 | 🟡 后端完成，前端已引入工具 |
| 大宗商品 AI 专家路由 | ✅ 完成 | 完成 | ✅ |

**综合评估**：Phase 0 完成，Phase 1-4 并行推进中，前端 Phase 4 进展最快。

---

## 二、Phase 状态

### Phase 0：大宗商品 AI 专家路由重构 ✅ 已完成

| 任务 | 状态 | 备注 |
|------|------|------|
| `CommodityContext` 新增 Category/AssetType | ✅ | `backend/agent/commodity/types.go` |
| `CategoryExpert` 接口 + 路由注册 | ✅ | `expert.go` |
| 3 通用专家（Technical/Macro/Sentiment） | ✅ | |
| 5 品种专属专家 | ✅ | monetary / safehaven / oil_supply / oil_geo / fund_tracking |
| 删除旧 correlation_expert / supply_expert | ✅ | |
| `CommodityEngine` 动态专家选择 | ✅ | |
| `Synthesis` 动态适配专家数量 | ✅ | |
| 新增 FRED VIX/经济日历等数据源 | ✅ | `fred_api.go` |
| 前端动态专家展示 | ✅ | `CommodityAnalysis.vue` |
| `go build` 验收 | ✅ | |

---

### Phase 1：基础设施 🟡 部分完成

| 任务 | 状态 | 备注 |
|------|------|------|
| 新建 `backend/stockcode/` 归一化包 | ✅ | `normalize.go` / `convert.go` / `normalize_test.go` |
| 新建 `backend/internal/domain/` 领域模型 | ✅ | stock/fund/commodity/analysis/market/system |
| 新建 `backend/internal/port/` 接口层 | ❌ | 尚未创建 |
| 新建 `backend/handler/` 框架 | 🟡 | 仅 `stock_handler.go`（245 行） |
| 新建 `frontend/src/utils/stockCode.js` | ✅ | |
| 删除 6 个废弃组件 | 🟡 | `allStockInfoList.vue`、`researchIndex.vue` 已删除；`promptPlaza`、`promptQa` 等仍存在 |
| `go build` 验收 | 🟡 | 当前可构建，但依赖旧架构 |

**阻塞项**：`internal/port/` 和 `internal/service/` 尚未建立，导致 Phase 2/3 缺乏 Clean Architecture 基础。

---

### Phase 2：股票核心模块 🟡 部分开始

| 任务 | 状态 | 备注 |
|------|------|------|
| Router 层归一化 | ✅ | commit `2601849` |
| KLineStore 候选查询 + 历史迁移 | 🟡 | 代码已归一化，未确认 DB 迁移脚本 |
| 数据源适配器迁入 `adapter/datasource/` | ❌ | 仍在 `backend/data/` |
| 股票 Service 层建立 | ❌ | `internal/service/` 未创建 |
| `StockHandler` 拆分 | 🟡 | 已启动，仅拆出股票部分 |
| 股票自选/K线/搜索功能验收 | 🟡 | 功能仍通过旧代码运行 |

**关键进展**：`free-stockdb` 已作为高优先级数据源接入 Router（多个相关 commit）。

---

### Phase 3：Agent 与分析 🟡 部分提前启动

| 任务 | 状态 | 备注 |
|------|------|------|
| Agent tools 拆分（5,292 + 2,760 行 → 10 文件） | ❌ | 尚未开始 |
| Agent 通过 port 接口获取数据 | ❌ | 依赖 Phase 1 port 层 |
| 回测/选股 Service 层建立 | ❌ | |
| `AnalysisHandler` 拆分 | ❌ | |
| T3 多层降级信号提取 | ✅ | commit `89972c6` |
| T4 LLM 双层分配优化 | ✅ | commit `89972c6` |
| T5 数据完整性预检器 | ❌ | |
| T6 交易标的上下文注入 | ❌ | |
| T7 工具调用计数器 | ❌ | |

---

### Phase 4：前端重构 🟡 actively 进行中

| 任务 | 状态 | 备注 |
|------|------|------|
| 引入 Pinia 建立 `stores/` | ✅ | `app.js` / `index.js` / `settings.js` / `stock.js` |
| 建立 `api/` 层 | ✅ | 8 个模块文件 |
| 拆分超大组件 | 🟡 | `AnalyzeMartket` 已拆出 7 个 market/ 子组件；`stock.vue` / `StockLightweightKlineChart.vue` / `FloatingAgentAssistant.vue` 仍较大 |
| 前端导航重构 | 🟡 | `useNavigation.js` / `navigation.js` 已创建；Research Center 拆分未完全落地 |
| 渐进 TypeScript | 🟡 | `tsconfig.json` 已添加（未提交） |
| `App.vue` 瘦身 | ✅ | 165 行（原臃肿状态） |
| KLine 逻辑提取 composables | ✅ | 核心逻辑 + 25 子面板指标 |
| 合并重复组件 | ✅ | `bkFundFlowChart` + `conceptFundFlowChart` → `FundFlowChart` |

**未提交改动**：
- `frontend/tsconfig.json`（新增）
- `frontend/src/components/market/`（7 个拆分组件）
- `AnalyzeMartket.vue`、`useNavigation.js`、`router.js`、`vite.config.js` 等修改中

---

### Phase 5：剩余模块 + 清理 ❌ 未启动

| 任务 | 状态 |
|------|------|
| Market/Fund/Commodity/News/System Handler 拆分 | ❌ |
| `app.go` 缩减到 < 200 行 | ❌ |
| `backend/data/` 清空（仅保留兼容别名） | ❌ |
| VIP 策略调整 | ❌ |
| 全量功能测试 | ❌ |

---

### Phase 6：DSA 量化选股增强 ❌ 未启动

| 任务 | 状态 |
|------|------|
| D1 9 因子量化评分系统 | ❌ |
| D2 LLM 二次排序 | ❌ |
| D3 独立风控叠加层 | ❌ |
| D12 扩展 Pick 模型 | ❌ |
| D5 5 档决策标尺 | ❌ |

---

### Phase 7：Agent 风控辩论 + 学习系统 ❌ 未启动

| 任务 | 状态 |
|------|------|
| T1 三方风控辩论 | ❌ |
| D4 风控否决/降级状态机 | ❌ |
| D6 11 类分歧分类 | ❌ |
| T2 反思记忆系统 | ❌ |

---

### Phase 8：选股高级功能 ❌ 未启动

| 任务 | 状态 |
|------|------|
| D7 36 项硬过滤器 | ❌ |
| D8 5 阶段热点生命周期 | ❌ |
| D9 种子化选股旋转 | ❌ |
| D10 可插拔后分析链 | ❌ |
| D11 4 模式 Agent 编排 | ❌ |

---

## 三、关键文件状态

### 后端

| 文件/目录 | 行数/状态 | 说明 |
|-----------|----------|------|
| `app.go` | 3,488 行 | 仍承担绝大部分 Wails 绑定 |
| `backend/data/` | 126 个 Go 文件 | 上帝包，未拆分 |
| `backend/data/data_tools_wrapper.go` | 5,292 行 | Agent tools 尚未拆分 |
| `backend/data/tools.go` | 2,760 行 | 同上 |
| `backend/stockcode/` | 3 个文件 | 已建立 |
| `backend/internal/domain/` | 7 个领域模型 | 已建立 |
| `backend/internal/port/` | 不存在 | 待建 |
| `backend/internal/service/` | 不存在 | 待建 |
| `backend/handler/stock_handler.go` | 245 行 | 唯一拆出的 Handler |

### 前端

| 文件/目录 | 行数/状态 | 说明 |
|-----------|----------|------|
| `frontend/src/App.vue` | 165 行 | 已大幅瘦身 |
| `frontend/src/components/stock.vue` | 3,121 行 | 待拆分 |
| `frontend/src/components/StockLightweightKlineChart.vue` | 4,832 行 | 待拆分 |
| `frontend/src/components/FloatingAgentAssistant.vue` | 1,943 行 | 待拆分 |
| `frontend/src/components/AnalyzeMartket.vue` | 395 行 | 已部分拆分 |
| `frontend/src/api/` | 8 个文件 | 已建立 |
| `frontend/src/stores/` | 4 个 store | 已建立 |
| `frontend/src/composables/` | 3 个文件 | 已启动 |
| `frontend/src/utils/stockCode.js` | 已创建 | 已建立 |
| `frontend/tsconfig.json` | 未提交 | TS 迁移准备 |

---

## 四、最近 10 个 Commit

```
7b0ec8a refactor(frontend): merge bkFundFlowChart + conceptFundFlowChart into FundFlowChart
e546e83 refactor(frontend): migrate all .vue files from Wails bindings to API abstraction layer
35db788 refactor(frontend): Phase 4 P0 - API layer migration & App.vue slim
7631121 refactor(kline): extract 25 sub-pane indicators to composable
ef6a9b2 refactor(kline): extract core logic to composables
2ef65e9 docs(phase4): add App.vue incremental migration guide
be35d99 feat(frontend): add navigation config and useNavigation composable
3e985b4 fix(wails): add missing commodity API bindings
a09c59c feat(frontend): complete phase 4 refactoring infrastructure
a4d0df1 feat(frontend): phase 4 refactoring foundation
```

**趋势**：前端 Phase 4 重构是最近主要投入方向。

---

## 五、当前工作区未提交改动

```
 M frontend/auto-imports.d.ts
 M frontend/components.d.ts
 M frontend/package-lock.json
 M frontend/package.json
 M frontend/src/components/AnalyzeMartket.vue
 D frontend/src/components/allStockInfoList.vue
 D frontend/src/components/researchIndex.vue
 M frontend/src/composables/useNavigation.js
 M frontend/src/config/navigation.js
 M frontend/src/router/router.js
 M frontend/vite.config.js
?? .zcode/
?? frontend/src/components/market/
?? frontend/tsconfig.json
```

---

## 六、下一步建议

### 高优先级

1. **完成 Phase 1 基础设施收尾**
   - 创建 `backend/internal/port/`（datasource/repository/notification 接口）
   - 创建 `backend/internal/service/` 目录结构
   - 完成剩余废弃组件删除：`promptPlaza.vue`、`promptQa.vue`、`agent-chat_bk.vue`、`FloatingAiAssistant.vue`

2. **推进 Handler 拆分**
   - 按方案拆分 System/Market/Agent/Analysis/Commodity/Fund/News/Notification Handler
   - 每拆一个 Handler，同步从 `app.go` 迁移对应方法

3. **提交当前前端改动**
   - `tsconfig.json`、market/ 拆分组件、导航配置等已完成工作建议先提交，避免工作区堆积过大

### 中优先级

4. **启动 `backend/data/` 适配器迁移**
   - 先迁移数据源适配器（eastmoney/sina/tdx 等）到 `adapter/datasource/`
   - 保持旧包兼容别名，渐进替换

5. **Agent tools 拆分**
   - 将 `data_tools_wrapper.go` 和 `tools.go` 拆分为 `agent/tools/*.go`

### 低优先级 / 后续阶段

6. DSA 量化选股增强（Phase 6）
7. Agent 风控辩论 + 反思记忆（Phase 7）
8. 选股高级功能（Phase 8）

---

## 七、更新记录

| 日期 | 更新内容 |
|------|----------|
| 2026-08-04 | 初版，基于 `feat/multi-agent-analysis` 分支实际状态 |
