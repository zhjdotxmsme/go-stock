# go-stock 重构进度看板

> 更新于 2026-08-05（第二次），基于 `feat/multi-agent-analysis` 分支实际仓库状态（HEAD: 1d2f664）。
> 关联方案：`go-stock-全面重构方案.md`（v2.1）

---

## 一、总体进度

| 维度 | 当前值 | 目标值 | 状态 |
|------|--------|--------|------|
| `app.go` 行数 | **184**（原 3,488） | < 200 | ✅ 达标；生命周期拆至 app_lifecycle/app_monitor/app_tradingtime |
| Wails 绑定 | 214 方法直连 11 个 handler | 直连 | ✅ 前端已迁移至 handler 命名空间 |
| `backend/data/` 文件数 | 137 | < 15 | ❌ 仍是上帝包（tools.go 已拆 10 域文件） |
| Service/Port/Adapter 层 | 3 个垂直切片（trading/stockchange/fund） | 全域迁移 | 🟡 模板确立，逐步迁移中 |
| 前端超大组件 | 最大 1,516 行 | < 1,500 | ✅ 三大组件全部拆分 |
| 废弃功能清理 | 弹幕 + 提示词广场全清 | 完成 | ✅ |
| Phase 6/7/8 功能包 | 14 个新包全部落地（带测试） | 落地+接线 | 🟡 **包已完成，生产接线未做** |

---

## 二、Phase 状态

### Phase 0-4 ✅ 完成或接近完成
- Phase 0 商品专家路由 ✅；Phase 1 基础设施 ✅（port/domain/stockcode/handler 全齐）
- Phase 3 tools.go 拆分 ✅（2,760→581+10 域文件，94 工具字节一致）
- Phase 4 前端 ✅（三大组件拆分、Pinia、api 层 TS、广场/弹幕清理、28 个死 api 包装删除、4 个坏调用修复）

### Phase 5：清理 🟢 大部分完成
- app.go 3,488 → **184 行**；app_common.go 346 → 20 行
- 剩余：数据源适配器迁移 ❌、VIP 策略调整 ❌、全量功能测试 ❌

### Phase 6：DSA 量化选股 🟡 包完成，未接线

| 任务 | 状态 | 位置 |
|------|------|------|
| D1 9 因子评分 | ✅ 包完成 | `backend/agent/strategy/scoring/`（13 文件 1,517 行，14 测试） |
| D2 LLM 二次排序 | ✅ 包完成 | `backend/agent/strategy/ranking/`（1,023 行，13 测试） |
| D3 风控叠加 | ✅ 包完成 | `backend/agent/strategy/risk/`（17 检查+7 板块桶，10 测试） |
| D5 决策标尺 | ✅ 包完成 | `backend/internal/domain/analysis/decision_scale.go` |
| D12 68 字段 Pick 模型 | ✅ | `backend/models/daily_pick.go` +62 字段（AutoMigrate） |
| D9 种子化旋转 | ✅ 包完成 | `scoring/selection_variant.go` |

### Phase 7：Agent 风控辩论 + 学习 🟡 包完成，未接线

| 任务 | 状态 | 位置 |
|------|------|------|
| D4 风控否决状态机 | ✅ | `risk/override.go`（veto/downgrade，7 测试） |
| D6 11 类分歧分类 | ✅ | `backend/agent/strategy/disagreement/` |
| T1 三方风控辩论 | ✅ | `backend/agent/multi/risk_debate/`（695 行，8 测试） |
| T2 反思记忆 | ✅ | `backend/agent/memory/`（SQLite FTS5+LIKE 降级，6 测试） |
| T3/T4 | ✅ | 历史提交已完成 |

### Phase 8：选股高级功能 🟡 包完成，未接线

| 任务 | 状态 | 位置 |
|------|------|------|
| D7 硬过滤器 | ✅ | `backend/agent/strategy/filter/`（38 参数+瀑布诊断） |
| D8 热点生命周期 | ✅ | `backend/internal/service/market/hotspot.go`（5 阶段+5 角色） |
| D10 后分析链 | ✅ | `backend/agent/strategy/postanalysis/` |
| D11 4 模式编排 | ✅ | `multi/mode.go`（standard=现有管线逐字一致，T1/技能挂点预留） |

---

## 三、关键遗留（按优先级）

1. **⚠️ 生产接线（最高优先级）**：Phase 6-8 的 14 个包全部是独立纯函数/注入式实现，**尚未接入任何生产调用链**：
   - D1/D2/D3/D7/D9/D10 → `daily_pick_engine.go` 选股管线
   - T1/D6/D4 → multi 引擎（D11 已留 RiskDebateHook 挂点）
   - D5 → 合成输出与前端 DecisionDashboard
   - T2 → Agent Prompt 注入
2. **⚠️ Wails 环境实测**：本轮改动量极大（20+ commit），build/单测/字节比对全过，但未实际运行。重点回归：K线指标、多单拖拽、AI 弹窗、浮动助手流式对话、MCP/Cron/Skill 管理页（修复过的 4 个调用）。
3. 数据源适配器迁移 `adapter/datasource/`（❌ 未启动）
4. 剩余 service 切片（news/system/analysis/market 等，模板已确立）
5. 预存在问题：`go vet ./backend/data` 测试文件报错、`TestCheckStockBaseInfo` 失败、`backend/models` 测试 import cycle、`backend/agent/tools` 测试需运行时 DB、GOOS=darwin/linux 交叉编译 yahoo_finance_api.go 失败——均为历史遗留。

---

## 四、本轮提交（2026-08-05 第二次，Phase 6-8）

```
1d2f664 feat(agent): D11 4-mode orchestration with stage budget control
65d3dd7 feat(strategy): D10 pluggable post-analysis chain
773247e feat(strategy): D7 hard filters + D8 hotspot lifecycle
33a688b feat(agent): T2 reflection memory system (SQLite FTS5)
3b8b70e feat(agent): T1 three-way risk debate + risk judge with veto
b2a4ebb feat(strategy): D4 risk override state machine + D6 disagreement classification
75a6413 feat(models): D12 extend DailyPick 62 fields + D9 seeded rotation
d04b5dd feat(ranking): D2 LLM re-ranking with model-chain fallback
03e9f10 feat(risk): D3 risk overlay + D5 decision scale
9ce104e feat(scoring): D1 9-factor quantitative scoring system
1650df9 fix(frontend): repair 4 broken api wrappers, remove 28 dead ones
5439b18 refactor(wails): bind all 11 handlers directly (app.go -> 184 lines)
```

---

## 五、更新记录

| 日期 | 更新内容 |
|------|----------|
| 2026-08-04 | 初版 |
| 2026-08-05 | Handler 全量接线；service 切片；tools.go 拆分；前端三大组件拆分 |
| 2026-08-05(2) | app.go 达标 184 行；Wails 直连 handler；Phase 6-8 全部 14 个功能包落地（未接线） |
