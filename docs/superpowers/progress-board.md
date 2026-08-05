# go-stock 重构进度看板

> 更新于 2026-08-05，基于 `feat/multi-agent-analysis` 分支实际仓库状态（HEAD: ff1ce59）。
> 关联方案：`go-stock-全面重构方案.md`（v2.1）

---

## 一、总体进度

| 维度 | 当前值 | 目标值 | 状态 |
|------|--------|--------|------|
| `app.go` 行数 | 2,145（+app_common.go 346） | < 200 | 🟡 已从 3,488 降 38%，方法全部委托，剩余为生命周期/监控内部逻辑 |
| `backend/data/` 文件数 | 137 | < 15（按子包拆分后） | ❌ 仍是上帝包，但 tools.go 已拆分 |
| 后端 Handler 拆分 | 11/11 | 11 | ✅ 9 域 handler + trading + stockchange，App 方法全部委托 |
| Service/Port/Adapter 层 | trading 垂直切片完成 | 全域迁移 | 🟡 模板已确立 |
| 前端超大组件 | 最大 1,516 行 | < 1,500 | ✅ 三大组件（4,832/3,121/1,954）全部拆分完成 |
| 前端 API 抽象层 | ✅ 完成（TS） | 完成 | ✅ |
| 股票代码归一化 | `backend/stockcode/` 已建 | 全链路统一 | 🟡 后端完成，前端已引入工具 |
| 大宗商品 AI 专家路由 | ✅ 完成 | 完成 | ✅ |

---

## 二、Phase 状态

### Phase 0：大宗商品 AI 专家路由重构 ✅ 已完成

### Phase 1：基础设施 🟡 大部分完成

| 任务 | 状态 | 备注 |
|------|------|------|
| `backend/stockcode/` 归一化包 | ✅ | |
| `backend/internal/domain/` 领域模型 | ✅ | 7 个领域 |
| `backend/internal/port/` 接口层 | ✅ | datasource + repository |
| `backend/handler/` 全部拆分 | ✅ | 11 个 handler 文件，App 委托接线完成 |
| `frontend/src/utils/stockCode.js` | ✅ | |
| 删除废弃组件 | 🟡 | 弹幕已删；promptPlaza/promptQa/agent-chat_bk/FloatingAiAssistant 仍存在 |

### Phase 2：股票核心模块 🟡

| 任务 | 状态 | 备注 |
|------|------|------|
| Router 层归一化 | ✅ | |
| 数据源适配器迁入 `adapter/datasource/` | ❌ | 仍在 `backend/data/` |
| 股票 Service 层 | ❌ | trading 域已建模板，stock 域未迁移 |
| StockHandler 拆分 | ✅ | 含 AllStockInfo/实时价格 |

### Phase 3：Agent 与分析 🟡

| 任务 | 状态 | 备注 |
|------|------|------|
| `tools.go` 拆分（2,760 → 581 + 10 域文件） | ✅ | 94 个工具字节级一致 |
| Agent tools 迁移到 `agent/tools/` | ❌ | 拆分完成，未迁包 |
| AnalysisHandler 拆分 | ✅ | |
| T3 多层降级信号提取 / T4 LLM 双层分配 | ✅ | |
| T5/T6/T7 | ❌ | |

### Phase 4：前端重构 🟢 接近完成

| 任务 | 状态 | 备注 |
|------|------|------|
| Pinia stores / api/ 层 / TypeScript | ✅ | |
| `StockLightweightKlineChart.vue` 拆分 | ✅ | 4,832 → 1,372（5 composables + series.js） |
| `stock.vue` 拆分 | ✅ | 3,121 → 1,442（7 script 块 + 2 弹窗子组件，删弹幕） |
| `FloatingAgentAssistant.vue` 拆分 | ✅ | 1,954 → 862（7 composables + AgentChatFooter） |
| 孤儿 composable 清理 | ✅ | 删除 6 个未接线的前序重写版 |
| 前端导航重构 | 🟡 | 配置已建，Research Center 拆分未完全落地 |
| 合并重复组件 | ✅ | FundFlowChart |

### Phase 5：剩余模块 + 清理 🟡 进行中

| 任务 | 状态 | 备注 |
|------|------|------|
| 全部 Handler 拆分 | ✅ | 含 trading/stockchange 两个新增 |
| `app.go` 缩减 | 🟡 | 2,145 行；剩余为 domReady/价格监控/InitCronTasks 等内部逻辑 |
| `backend/data/` 清空 | ❌ | |
| VIP 策略调整 | ❌ | |
| 全量功能测试 | ❌ | 拆分后建议在 Wails 环境实测（见风险节） |

### Phase 6-8：DSA 量化选股 / 风控辩论 / 选股高级功能 ❌ 未启动

---

## 三、关键文件状态

### 后端

| 文件/目录 | 状态 |
|-----------|------|
| `app.go` | 2,145 行（原 3,488），方法全部委托 handler |
| `backend/handler/` | 14 个文件（11 handler + 3 个 admin 平台文件） |
| `backend/data/tools.go` | 581 行（原 2,760），+10 个 tools_*.go 域文件 |
| `backend/data/stock_data_api.go` | 3,066 行，data 包最大文件，待处理 |
| `backend/internal/service/trading/` | 首个垂直切片（含测试） |
| `backend/internal/adapter/repository/sqlite/` | StockRepository 实现（trading 部分，15 个方法 TODO 占位） |

### 前端

| 文件/目录 | 状态 |
|-----------|------|
| `stock.vue` | 1,442 行（原 3,121） |
| `StockLightweightKlineChart.vue` | 1,372 行（原 4,832） |
| `FloatingAgentAssistant.vue` | 862 行（原 1,954） |
| `cron-task-manager.vue` | 1,516 行，现最大组件 |
| `components/stock/` `components/kline/` `components/agent/` | 拆分产物目录 |

---

## 四、本轮提交（2026-08-05）

```
ff1ce59 refactor(frontend): split FloatingAgentAssistant.vue 1821 -> 862 lines
ea7cd04 refactor(frontend): split stock.vue 3121 -> 1442 lines, remove danmaku feature
5c3ccf7 refactor(frontend): split StockLightweightKlineChart.vue 4556 -> 1372 lines
5dba10a refactor(data): split 2760-line tools.go into 10 domain files
be0142c feat(service): first Handler->Service->Port<-Adapter vertical slice (trading)
20b1952 refactor(handler): complete remaining wiring and remove dead code
1ed9474 refactor(handler): wire App methods to delegate to all 9 handlers
```

---

## 五、风险与待验证项

1. **运行时未实测**：所有拆分均通过 build/字节级比对验证，但未在 Wails 环境实际运行。重点回归点：K线指标开关/信号评估、多单拖拽、分组拖拽排序、AI 弹窗导出、浮动助手流式对话。
2. **`go vet ./backend/data` 预存在报错**（测试文件 import 问题），非本轮引入。
3. **`TestCheckStockBaseInfo` 预存在失败**（Wails EventsEmit 无生命周期 ctx）。
4. `vue3-danmaku` 依赖仍在 package.json（弹幕代码已删），待 `npm uninstall`。
5. cron entry 双 map 兼容逻辑（`removeLegacyCronEntry`）待 InitCronTasks 委托后移除。

## 六、下一步建议

1. **Wails 环境实测回归**（拆分规模大，优先验证运行时）
2. 继续 service 切片迁移：fund / news / stockchange / stock（复用 trading 模板，注意 `backend/internal` 不可被 main 包 import，需 handler 内装配）
3. 数据源适配器迁移到 `adapter/datasource/`
4. `app.go` 内部逻辑收口：InitCronTasks 委托 systemHandler、价格监控提取
5. 删除剩余废弃组件（promptPlaza/promptQa/agent-chat_bk/FloatingAiAssistant）
6. Phase 6-8 功能增强

---

## 七、更新记录

| 日期 | 更新内容 |
|------|----------|
| 2026-08-04 | 初版 |
| 2026-08-05 | Handler 全量接线完成；service 首个切片；tools.go 拆分；前端三大组件拆分完成 |
