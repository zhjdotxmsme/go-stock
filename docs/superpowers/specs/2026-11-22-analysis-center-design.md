# 统一分析中心设计（第一阶段：选股预测族）

> 状态：已确认（2026-11-22）。目标：把"选股预测"族的多个工具合并为一个入口、一个引擎；
> 回测族、市场资讯族在后续阶段按同样模式推进。
>
> 修订：2026-11-22 第二轮评审修正（因子不并入五维评分、daily_pick 保留为复盘数据源、
> 调度与前端嵌入的既有阻塞点）。见文末「修订记录」。

## 背景与问题

应用内选股预测相关工具入口分散、能力重叠：

| 现入口 | 路由 | 实质 |
|---|---|---|
| K线分析 | `/kline-analysis` | 单股技术分析 |
| 选股工作台 | `/analysis/workbench` | Tab 容器（形态/策略筛选，自有引擎） |
| 每日选股 | `/daily-pick` | DailyPick 引擎（扫描+评分+Kronos 因子+复盘调权） |
| 狩猎场 | `/khunter` | 观察→确认→退出 全链路（时点约束/数据闸门/风险过滤） |
| 股票推荐 | `/research/recommends` | AI 推荐列表（挂在"研究资讯"组） |

Kronos（AI 价格预测）集成现状不合理：

1. 预测接口挂在 `TradingRecordHandler`（交易记录处理器）名下，归属错误；
2. "AI 预测"点击后模型冷启动需 30 秒以上，超时无回显；
3. 作为评分因子只跑 top30、权重 0.05 硬编码，预测内容不可见、影响不可解释；
4. 预测只画在 K 线图上，与选股/狩猎场没有闭环。

## 目标形态

统一分析中心：单页 Tab 容器，khunter 作为推荐入口的唯一选股引擎，Kronos 作为统一预测服务。

**非目标（第一阶段）**：

- 选股工作台（ScreeningWorkbench）自有筛选引擎不合并，仅作为 Tab 嵌入；
- 回测族、市场资讯族不做；
- **不新建"预测质量"独立页**：`kronos_rolling_backtest` 第一阶段仍经 khunter「回测验证」Tab
  与交易记录页到达，独立页随第二阶段一并做；
- **不修改 khunter 五维评分公式**（见「Kronos 消费面 2」）；
- daily_pick 旧接口与引擎删除留给下一阶段（前置条件见「与 daily_pick 的关系」）。

## 架构与组件

### 后端

- **统一选股引擎 = khunter 流水线**（既有：点入时点扫描 → 五维评分 → 数据闸门 →
  观察 → 风险过滤 → 同步就绪确认 → 入场 → 退出），本次不做引擎级行为改动，
  `scorer.Score` 的五维加权公式与观察入选阈值保持不变。
- **Kronos 批量因子**：由 `daily_pick_kronos.go` 的内部模块改为 khunter 流水线内的
  独立旁路模块，**不参与五维评分、不改变观察入选**，只产出推荐视图用的因子分与明细
  （见「Kronos 消费面 2」）。
- **Kronos 因子配置**：独立文件 `config/khunter_kronos.json`
  （开关 / topN / predLen / 权重 / 权重上限），不并入
  `config/khunter_technical_weights.json`——后者是"策略命中分值 int 覆盖"
  （如 `"W底": 30`），与本处的小数权重不同量纲，混用会让两套语义都失效。
- **新增 `AnalysisCenterService` 门面**：暴露 推荐列表 / 预测明细 / 运行状态 / 异步预测任务
  四类读接口（不含运行触发与观察列表——这两项仍由既有 `KhunterHandler` 提供，前端狩猎场
  Tab 直连）；内部读 khunter 的观察与运行汇总表、写预测任务注册表；Kronos 预测接口从
  `TradingRecordHandler` 迁入本门面（旧方法保留为 deprecated 委托，签名不变）。
- **旧 daily_pick handler 保留**：签名与响应格式不变，第一阶段仍直连旧引擎（不转调门面），
  标记 deprecated，下一阶段删除。

### 与 daily_pick 的关系（第一阶段的真实边界）

khunter 的 `KhunterHunting` 只记录 `EntryPrice/ExitPrice/ExitReason`，**没有 T+1/3/5
收益字段**；`kronosFactorStats`（因子命中率）与策略自动调权（`daily_pick_weighting.go`）
的数据源全部是 `DailyPick` 表的回填收益。因此：

- **第一阶段 daily_pick 引擎与结果表继续运行并写入**（保留 15:30 选股、09:15 复盘两个
  cron）。它是当前唯一的复盘/调权数据源，停写即等于永久冻结自动调权。
- 前端不再把 daily_pick 作为入口（`/daily-pick` 重定向到分析中心推荐 Tab），
  接口保留到下一阶段删除。
- **删除 daily_pick 的前置条件**：khunter 侧实现观察/交易的 T+1、T+3、T+5 收益回填与
  Kronos 因子命中评估，`kronosFactorStats` 与自动调权改读 khunter 表。该工作属于第二阶段，
  不在本设计范围。

### 前端

- 新建 `/analysis`（分析中心）单页 Tab 容器，Tab 顺序按使用频率：
  **推荐 | 狩猎场 | 选股工作台 | K线分析**。
- Tab 使用 `display-directive="show"` 保留已挂载状态（naive-ui 默认 `if` 会卸载）；
  `KhunterPanel` 在 `onUnmounted` 做 `EventsOff`，卸载会丢运行进度事件。
- **嵌入前必须先修的既有阻塞点**（不改会让用户被踢出 Tab 容器）：
  - `ScreeningWorkbench.vue` 的 `watch(activeTab, …router.replace({name:'analysisWorkbench'}))`
    会在切换工作台内部 Tab 时跳到独立路由 → 改为 `emit('update:tab')` 与父容器双向绑定，
    或加 `embedded` prop 时跳过路由同步；
  - `kline-analysis.vue` 的 `router.push({ path: '/' })`（返回按钮）→ 嵌入时改为 `emit('back')`。
- `DailyPickPanel` **不改造为推荐视图**：推荐视图为新组件，读门面接口；`DailyPickPanel`
  随 `/daily-pick` 路由移除而不再挂载（组件代码与路由重定向保留到下一阶段）。
- 左侧菜单"分析工具"组收敛为 1 项"分析中心"；旧路由用 **路由 redirect**（前端是
  `createWebHashHistory`，不存在服务端 301）：`/daily-pick` → 推荐、
  `/khunter` → 狩猎场、`/kline-analysis` → K线分析、`/analysis/workbench` 保留直达但也归入
  分析中心 Tab。
- `/research/recommends` 保留路由、从菜单隐藏，**并在推荐视图页头放一个入口链接**，
  否则该页面无任何可达路径。

## Kronos 统一集成

定位：统一预测服务（后端托管生命周期），三个消费面：

### 1. 单股预测（交互面）

推荐视图、狩猎场观察、K线分析的上下文均可发起。

- 异步任务化：提交返回任务 ID，前端轮询；模型冷加载期任务保持 `pending` 并指数退避重试，
  重试窗口对齐 warmup 上限（8 分钟），不再 16 秒即判失败；
- 去重键 `(code, predLen, asOfDate, 自然日)`：仅对 `pending/running/done` 生效，
  `failed` 不占用键，用户可直接重试；
- 查询未知 ID 返回明确错误，不返回零值任务（否则前端无限轮询）；
- 任务表设 TTL（保留最近 N 条 / 24h），避免常驻进程内存只增不减；
- 结果叠加显示在 K 线图上（现有能力保留）。

### 2. 批量因子（排序面，非评分面）

- 每交易日流水线在**观察落库并提交之后**，异步对观察候选按评分取 top N
  批量预测（走既有 `/predict_batch`，不是逐只串行 HTTP）；**不阻塞确认/退出引擎**，
  也不改变当日观察名单。
- 因子分沿用既有口径（`daily_pick_kronos.go` 的 `kronosFactorScore`：预测涨幅按 ±10%
  截断映射，再按一致度向中性 50 收缩），保证与历史统计可比。
- **预测明细落库**：`ForecastJSON` 存完整预测（30 只 × predLen 根 K 线 ≈ 十几 KB，
  成本可忽略），推荐视图每只候选可展开曲线与因子贡献。
- 服务不可用 / 单只失败 → 该候选记 `DegradedReason` + 中性分 50，`HasFactor=false`。
- 权重只影响**推荐视图的排序分** `Score*(1-w) + FactorScore*w`，
  不进入 `scorer.Score` 的 Total，不影响观察入选与回测口径。

> 若要让因子进入评分，需要给 `scorer.FiveDimInput` 增加第六维并重跑
> `khunterreplay` 五案例对照，属于引擎改动，单独立项，不在第一阶段。

### 3. 滚动回测（验证面）

既有的 `kronos_rolling_backtest` 经 khunter「回测验证」Tab 与交易记录页到达，
支撑因子启停与权重决策；独立"预测质量"页在第二阶段。

服务治理：生命周期归后端（按需拉起 + 健康检查 + 状态徽标，lazy start 设置保留）；
服务离线时交互面提示"预测服务离线"，排序面降级中性分 + degraded 标记。

## 数据流

实际调度是三组互相独立的 cron，依赖只能靠"后一个任务跑时自查前一个的产物"：

| 时间 | 任务 | 产出 |
|---|---|---|
| 15:30 | `kline_sync`（`{"years":1}`） | 本地日 K 同步 |
| 15:30 | `daily_pick` 自动选股 + 写入 | 复盘/调权数据源（第一阶段保留） |
| 09:15 | `daily_pick` 自动复盘 | T+1/3/5 收益回填、调权系数 |
| 17:00 | `khunter_daily_pipeline` | 信号/评分/观察/确认/退出 + Kronos 批量因子 |

```
17:00 khunter 流水线
  → 时点扫描（形态策略）→ 五维评分（权重不变）→ 数据闸门 → 观察落库
  → 异步：观察 top N 批量 Kronos 预测 → 明细落库（失败记 degraded，不影响名单）
  → 确认/退出引擎（不被预测阻塞；同步未就绪时只观察，不变）
推荐视图 = 观察中/已确认候选，按 Score 与因子混分排序（可展开预测明细）
```

- `daily_pick` 结果表第一阶段**继续写入**（理由见「与 daily_pick 的关系」），
  新链路写 khunter 表（幂等 upsert 既有）。
- 权重配置：技术面权重覆盖仍读 `config/khunter_technical_weights.json`，
  但**该 overrides 目前未被接线**（`LoadTechnicalOverrides` 全仓无调用，
  `scorer.Score` 内部用的是零值 `TechnicalScorer{}`）。第一阶段不修，作为已知缺陷记录；
  Kronos 因子配置独立成 `config/khunter_kronos.json`。

## 错误处理

| 故障 | 行为 |
|---|---|
| Kronos 服务离线/冷启动中 | 交互面：任务挂起 + 状态提示，重试窗口对齐 warmup（8 分钟）；排序面：中性分 + degraded 标记，主链不阻塞 |
| Kronos 预测结果异常（缺 bars/summary） | 该候选因子记缺失原因，`HasFactor=false`，不混入排序分 |
| 批量预测部分候选失败 | 成功部分照常落库，失败写 `DegradedReason` 并计入运行汇总 message |
| 预测任务 `failed` | 不占用去重键，可立即重试；前端收到明确错误而非无限轮询 |
| 预测任务 ID 未知（进程重启等） | 返回 `ErrPredictJobNotFound`，前端提示"任务已失效，请重新提交" |
| 门面查询报错（DB 错误 / `db.Dao == nil`） | 返回错误，前端显示错误态；**不得**返回空列表伪装成"今日无候选" |
| 旧接口兼容 | `DailyPickHandler` 与 `TradingRecordHandler` 的 Kronos 方法签名/结构不变，直连旧实现 |
| 同步未就绪（既有） | 盘中只观察不确认（不变） |

## 测试策略

- 引擎：因子分计算与降级单测（纯函数 + 注入的假 predict）；批量预测不改观察名单的回归。
- 门面：因子混分排序单测（含无因子保持原分、degraded 不混入）；查询报错不吞的测试。
- 异步预测：提交→轮询→done；failed 后可重新提交；未知 ID 返回错误；冷启动重试时序。
- 前端：**只做纯函数单测**（`blendedScore` 等）。仓库无 `@vue/test-utils`/jsdom，
  不做组件挂载测试；Tab 容器与嵌入改造靠手工冒烟（安装版逐 Tab 点一遍）。
- 端到端：
  - `khunterreplay` 五案例对照，`khunter_kronos.json` 取 `enable:false` → 评分与结局计数
    与合并前逐项一致（证明主链未被触碰）；
  - 另跑一组 `enable:true` 对照，**只比对观察名单与结局计数不变**，确认因子未侵入入选项。

## 阶段划分

- **第一阶段（本设计）**：选股预测族统一分析中心 + Kronos 统一集成（因子为排序面）。
- 第二阶段：回测族（旧信号级回测 / 四组对照 / AI 推荐回测 → 回测中心）+
  "预测质量"独立页 + khunter 侧收益回填（解除 daily_pick 依赖）+ 因子入评分评估。
- 第三阶段：市场资讯族。选股工作台筛选引擎是否并入统一引擎，视第一阶段使用情况再定。

## 修订记录（2026-11-22 第二轮评审）

| 原设计表述 | 事实 | 修正 |
|---|---|---|
| 因子并入评分 → 影响观察入选 | `scorer.Score` 五维硬编码，无注入点；因子挂在观察落库之后 | 改为排序面因子，显式声明不改五维 |
| 权重并入 `khunter_technical_weights.json` | 该 overrides 未被接线，且是 int 策略分而非小数权重 | 独立 `khunter_kronos.json`，缺陷单独记录 |
| 停写 daily_pick 结果表 | khunter 无收益回填，停写即永久冻结调权 | 第一阶段继续写入，删除前置条件写入第二阶段 |
| 复盘对应 khunter 既有 RiskHistory | khunter 模型无 T+1/3/5 收益字段 | 同上，改为显式依赖 daily_pick |
| 「301 重定向」 | 前端 `createWebHashHistory` | 改为路由 redirect |
| `ScreeningWorkbench` 原样嵌入 | 内部 `router.replace` 会跳走 | 列入嵌入前必修（`embedded` prop） |
| keep-alive 缓存状态 | `n-tab-pane` 默认 `if` 卸载 | 改 `display-directive="show"` |
| 前端 node:test 组件测试 | 无 test-utils/jsdom | 改为纯函数测试 + 手工冒烟 |
| 预测明细落库（计划留空 ForecastJSON） | 与"黑盒变透明"承诺不符 | 明确落库完整预测 + `GetForecast` 读接口 |
| 批量因子设计 | 计划逐只串行、插在确认引擎之前 | 改 `/predict_batch` + 异步 goroutine，不阻塞 |
| 单链 `15:30 同步就绪 → 扫描` | 三组独立 cron，khunter 在 17:00 | 数据流按真实调度改写 |
| "预测质量"页 | 目标形态只列四 Tab | 明确第一阶段不做独立页 |
| `/research/recommends` 隐藏但可达 | 无入口 | 推荐视图页头加入口链接 |
| 门面"统一暴露 运行/观察/推荐/预测/状态" | 门面实际只读表，运行/观察仍走 `KhunterHandler` | 显式写明门面职责边界 |
| 异步任务：无论状态都复用去重键 | 失败后永远拿到同一个 failed 任务 | failed 释放键，可立即重试 |
| 异步任务：8×2s 重试上限 | `kronosWarmup` 超时 8 分钟，16s 必失败 | 重试窗口对齐 8 分钟，指数退避 |
| `GetPredictJob` 未命中返回零值 | 前端无限轮询 | 返回 `ErrPredictJobNotFound` |
