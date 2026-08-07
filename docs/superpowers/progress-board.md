# go-stock 重构进度看板

> 更新于 2026-08-05（终版），基于 `feat/multi-agent-analysis` 分支实际仓库状态（HEAD: fe81869）。
> 关联方案：`go-stock-全面重构方案.md`（v2.1）。**方案 Phase 0-8 已全部执行完毕。**

---

## 一、总体进度

| 维度 | 当前值 | 目标值 | 状态 |
|------|--------|--------|------|
| `app.go` 行数 | **184**（原 3,488，-95%） | < 200 | ✅ |
| Wails 绑定 | 214 方法直连 11 个 handler | 直连 | ✅ |
| Service/Port/Adapter 层 | 7 service 切片 + sqlite repository + datasource Router | 主要域覆盖 | ✅ |
| 前端超大组件 | 最大 1,516 行（原 4,832） | < 1,500 | ✅ |
| 废弃功能清理 | 弹幕 + 提示词广场 + 28 死 api + 6 孤儿 composable | 完成 | ✅ |
| VIP 策略 | 已移除，全部功能免费 | 移除 | ✅ |
| Phase 6/7/8 功能 | 14 个包落地 + 生产接线（A1-A5） | 落地+接线 | ✅ |
| 测试 | 19 个测试包全绿（新增 ~200 用例） | — | ✅ |
| `backend/data/` 上帝包 | 137 文件 | < 15 | ⚠️ 长尾未做（适配器已包装主路径） |

---

## 二、Phase 完成清单

### Phase 0：大宗商品 AI 专家路由 ✅（前期已完成）

### Phase 1-5：架构重构 ✅

- `app.go` 3,488 → 184 行；生命周期拆分为 app_lifecycle/app_monitor/app_tradingtime
- 11 个 handler 全部拆分并由 main.go 直连绑定；前端 api 层迁移至 handler 命名空间
- Service 垂直切片 ×7：trading / stockchange / fund / analysis / news / system / market
- Port 接口：datasource + repository（stock/fund/analysis/news/system 5 组）
- Adapter：sqlite repository（GORM/委托混合）+ datasource Router（5 K 线源 fallback 链与原版一致 + 腾讯行情）
- `tools.go` 2,760 → 581 行 + 10 域文件（94 工具字节一致）
- 前端三大组件拆分（4,832→1,372 / 3,121→1,442 / 1,954→862）+ DecisionScaleBar 新组件
- 修复 4 个预存坏 api 调用（MCP×2/Cron/Skill）；删除 28 个死包装

### Phase 6：DSA 量化选股 ✅ 落地+接线

| 项 | 位置 | 接线 |
|---|---|---|
| D1 九因子评分 | `strategy/scoring/` | ✅ 选股管线（A1） |
| D2 LLM 二次排序 | `strategy/ranking/` | ✅ A2（无 AI 配置静默跳过） |
| D3 风控叠加 | `strategy/risk/` | ✅ A1（标记不剔除） |
| D5 决策标尺 | `domain/analysis/` | ✅ A4（FinalReport 透出 + DecisionScaleBar） |
| D12 68 字段模型 | `models/daily_pick.go` +62 字段 | ✅ AutoMigrate |
| D9 种子旋转 | `scoring/selection_variant.go` | ✅ A2（默认关） |

### Phase 7：Agent 风控 + 学习 ✅ 落地+接线

| 项 | 位置 | 接线 |
|---|---|---|
| T1 三方风控辩论 | `multi/risk_debate/` | ✅ A3（full/specialist 模式） |
| D4 风控否决状态机 | `risk/override.go` | ✅ A3（合成后钳位） |
| D6 11 类分歧分类 | `strategy/disagreement/` | ✅ A3（注入合成引导） |
| T2 反思记忆 | `agent/memory/`（SQLite FTS5） | ✅ A5（Prompt 注入 + ReflectOnAnalysis 绑定） |
| T3/T4 信号提取/LLM 分配 | — | ✅（前期已完成） |

### Phase 8：选股高级功能 ✅ 落地+接线

| 项 | 位置 | 接线 |
|---|---|---|
| D7 硬过滤器 | `strategy/filter/`（38 参数+瀑布诊断） | ✅ A1 |
| D8 热点生命周期 | `service/market/hotspot.go` | ✅ 包可用（选股管线复用热度数据） |
| D10 后分析链 | `strategy/postanalysis/` | ✅ A2（计入 FinalScore） |
| D11 4 模式编排 | `multi/mode.go` | ✅（`agentMode` Wails 参数生效；standard=原管线逐字一致） |

**生产数据流**：
- 每日选股：候选池 → D7 硬过滤 → D1 九因子 → D2 LLM 重排 → D10 后分析 → D3 风控标记 → D12 全字段落库
- 多 Agent full 模式：7 分析师（T2 记忆注入）→ D6 分类 → 多空辩论 → 合成（引导注入）→ T1 风控辩论 → D4 否决 → D5 标尺

---

## 三、⚠️ 待办：实测回归（最高优先级）

所有改动经 build + ~200 单元测试 + 字节级比对验证，但**未在 Wails 环境实际运行**。回归清单：

1. 每日选股全流程（新管线：过滤→评分→LLM 排序→风控标记→D12 字段落库展示）
2. 个股 AI 分析：standard 模式（应与旧版一致）与 full 模式（风控辩论/否决/分歧引导）
3. K线页：指标开关、副图、多单拖拽、筹码分布
4. 自选股：分组拖拽、成本/交易价弹窗、AI 弹窗导出/分享
5. 浮动助手：流式对话、中断、历史恢复（VIP 门已移除，所有人可用）
6. MCP/Cron/Skill 管理页（修复过的 4 个调用）
7. ReflectOnAnalysis 调用一次（反思记忆闭环）
8. VIP 移除后：基金排行/选股/交易日志的 K线查看、AI 推荐股操作

## 四、已知预存在问题（历史遗留，非本轮引入）

- `go vet ./backend/data` 测试文件报错；`TestCheckStockBaseInfo` 失败
- `backend/models` 测试 import cycle；`backend/agent/tools` 测试需运行时 DB
- GOOS=darwin/linux 交叉编译 `yahoo_finance_api.go` 失败；app_linux.go 存在重复定义副本

## 五、长尾项（价值低，未做）

1. `backend/data/` 137 文件物理拆分（适配器已包装主要数据源，纯搬迁风险大于收益）
2. specialist 模式 skills 挂点（需 SkillRouter 设计）
3. 文档手册同步（使用手册中提示词广场/VIP 描述已过时）

---

## 六、更新记录

| 日期 | 更新内容 |
|------|----------|
| 2026-08-04 | 初版 |
| 2026-08-05 | Handler 全量接线；service 切片；tools.go 拆分；前端三大组件拆分（42 commits） |
| 2026-08-05(2) | app.go 达标 184 行；Wails 直连 handler；Phase 6-8 全部 14 个功能包落地 |
| 2026-08-05(3) | Phase 6-8 生产接线（A1-A5）；datasource 适配器 |
| 2026-08-05(终) | VIP 策略移除；analysis/news/system 切片收尾；**方案 Phase 0-8 全部完成** |
