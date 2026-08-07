# go-stock 重构进度看板

> 更新于 2026-08-05（第三次），基于 `feat/multi-agent-analysis` 分支实际仓库状态（HEAD: 53aa29e）。
> 关联方案：`go-stock-全面重构方案.md`（v2.1）

---

## 一、总体进度

| 维度 | 当前值 | 目标值 | 状态 |
|------|--------|--------|------|
| `app.go` 行数 | **184**（原 3,488） | < 200 | ✅ |
| Wails 绑定 | 214 方法直连 11 个 handler | 直连 | ✅ |
| Service/Port/Adapter 层 | 7 service 切片 + sqlite repository（4 实体组）+ datasource Router | 全域迁移 | ✅ 主要域完成（trading/stockchange/fund/analysis/news/system/market） |
| 前端超大组件 | 最大 1,516 行 | < 1,500 | ✅ |
| 废弃功能清理 | 弹幕 + 提示词广场全清 | 完成 | ✅ |
| Phase 6/7/8 功能 | 14 个包落地 + **已接入生产管线（A1-A5）** | 落地+接线 | ✅ |
| `backend/data/` 上帝包 | 137 文件 | < 15 | ❌ 长尾（适配器以包装方式落地，未物理搬迁） |

---

## 二、Phase 6-8 接线状态（本轮核心成果）

| 接线 | 内容 | 状态 |
|------|------|------|
| A1 | 选股引擎：D7 硬过滤 → D1 九因子 → D3 风控标记（PickEnhanceConfig 可开关，失败降级） | ✅ |
| A2 | D2 LLM 排序（模型链取 AI 配置，无配置静默跳过）→ D10 后分析计入 FinalScore；D9 旋转默认关 | ✅ |
| A3 | multi 引擎 full/specialist：D6 分歧分类 → 合成引导 → T1 风控辩论 → D4 否决钳位；**standard 逐字节不变**；`agentMode` 参数生效 | ✅ |
| A4 | D5 决策标尺透出（decisionSignal/Action/Label）+ 前端 DecisionScaleBar | ✅ |
| A5 | T2 记忆注入 7 分析师 Prompt（无记忆时 prompt 不变）+ `ReflectOnAnalysis` 绑定 | ✅ |

**生产数据流**：
- 每日选股：候选池 → 硬过滤 → 九因子评分 → LLM 重排（可选）→ 后分析 → 风控标记 → D12 全字段落库
- 多 Agent（full 模式）：7 分析师（记忆注入）→ D6 分类 → 多空辩论 → 合成（引导注入）→ 三方风控辩论 → D4 否决 → 决策标尺

---

## 三、架构现状

```
main (app.go 184 行，生命周期入口)
 ├─ app_lifecycle.go (466) 启动 cron/数据同步
 ├─ app_monitor.go (529) 价格监控
 └─ app_tradingtime.go (262) 交易时间
backend/handler/ (14 文件，11 handler，Wails 直连)
backend/internal/
 ├─ domain/ (7 领域 + decision_scale)
 ├─ port/ (datasource + repository 接口)
 ├─ adapter/
 │   ├─ repository/sqlite/ (stock+fund, GORM/委托混合)
 │   └─ datasource/ (Router + 5 K线源 + 腾讯行情, fallback 链与原版一致)
 └─ service/ (trading/stockchange/fund/market 4 个切片)
backend/agent/
 ├─ strategy/ (scoring/ranking/risk/filter/disagreement/postanalysis 6 包)
 ├─ multi/ (引擎 + mode + enhance + risk_debate)
 └─ memory/ (SQLite FTS5 反思记忆)
```

---

## 四、遗留项（按优先级）

1. **⚠️ Wails 环境实测（最高优先级）**：本轮 30+ commit 改动量极大，build/单测全过但未实际运行。回归清单：
   - 每日选股全流程（新管线：过滤→评分→LLM排序→风控）
   - 个股 AI 分析 standard 与 full 模式（full 有风控辩论）
   - K线指标开关、多单拖拽、分组拖拽
   - 浮动助手流式对话、AI 弹窗导出
   - MCP/Cron/Skill 管理页（修复过的 4 个调用）
   - ReflectOnAnalysis 调用一次（验证反思记忆闭环）
2. 剩余 service 切片：✅ 已完成（analysis 40b4590 / news 40b4590 / system 4f58182）；SettingConfig 有意不 repo 化（全局副作用）
3. `backend/data/` 上帝包物理拆分（长尾，适配器已包装主要数据源）
4. ~~VIP 策略调整~~ ✅ 已移除（267af56）：`EffectiveSponsorVipLevel` 恒返回 (2,true)，全部功能免费；外媒新闻同步不再限 VIP2；赞助码仅用于展示
5. specialist 模式 skills 挂点（占位待 SkillRouter）
6. 预存在问题（历史遗留，非本轮引入）：`go vet ./backend/data` 测试文件报错、`TestCheckStockBaseInfo` 失败、`backend/models` 测试 import cycle、`backend/agent/tools` 测试需运行时 DB、GOOS=darwin/linux 交叉编译 yahoo_finance_api.go 失败

---

## 五、本轮提交（2026-08-05 第三次）

```
53aa29e feat(adapter): datasource Router with priority fallback
f81e1e1 feat(agent): A4 decision scale + A5 reflection memory wiring
50eb4df feat(agent): A3 wire T1/D6/D4 into multi engine
5faec67 feat(daily-pick): A2 wire D2 LLM ranking + D10 + D9
4819671 feat(daily-pick): A1 wire D7 filter + D1 scoring + D3 risk
```

---

## 六、更新记录

| 日期 | 更新内容 |
|------|----------|
| 2026-08-04 | 初版 |
| 2026-08-05 | Handler 全量接线；service 切片；tools.go 拆分；前端三大组件拆分 |
| 2026-08-05(2) | app.go 达标 184 行；Wails 直连 handler；Phase 6-8 全部 14 个功能包落地 |
| 2026-08-05(3) | Phase 6-8 生产接线完成（A1-A5）；datasource 适配器落地 |
