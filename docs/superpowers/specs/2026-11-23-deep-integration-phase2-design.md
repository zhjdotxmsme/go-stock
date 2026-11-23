# 深度整合与 Kronos 优化设计（阶段 2：数据闭环）

> 状态：已确认（2026-11-23）。路线：**闭环先行 → 效果可解释 → 引擎统一 → 回测中心**。
> 本设计详细覆盖阶段 2；阶段 3-5 为路线级描述，各自进入实施前单独评审成设计。
> 前置：阶段 1（统一分析中心壳 + Kronos 排序因子旁路 + 异步预测任务）已交付，
> 回放五案例对照 DIFF COUNT 0 已验证（见 2026-11-22-analysis-center-design.md「与 daily_pick 的关系」）。

## 背景与问题

阶段 1 之后，"深度整合"的阻塞点全部指向同一个地基——**khunter 侧没有收益回填**：

1. Kronos 因子命中率、策略自动调权的数据源全部是 `daily_picks` 表的回填收益，
   停写 daily_pick 即等于永久冻结调权与因子评估；
2. Kronos 预测"准不准"无从量化——批量预测明细已落库（`khunter_kronos_predictions`），
   但没有任何对账机制回答"预测期末实际涨跌 vs 预测方向"；
3. daily_pick 引擎与 khunter 流水线双引擎并存，口径分裂（尾盘买入 vs 确认成交）。

## 路线图

| 阶段 | 内容 | 交付判据 |
|---|---|---|
| **2 数据闭环（本设计）** | khunter 收益回填 + Kronos 因子命中评估 + 调权切源 + daily_pick 退役 | khunter 表自含复盘数据；调权不再依赖 `daily_picks`；daily_pick 引擎/cron/入口全移除后全量回归 + 五案例回放对照仍 DIFF 0 |
| **3 Kronos 效果与可解释** | 命中率/超额收益统计看板、预测明细可展开推理依据、因子入评分评估（scorer 第六维 + 五案例对照重跑） | 每个推荐可回答"因子为什么给这个分、历史上这类预测准不准"；第六维经回放对照可证伪 |
| **4 引擎统一** | 选股工作台（ScreeningWorkbench）筛选引擎并入 khunter 流水线 | 单一可信口径，工作台 Tab 改读统一引擎 |
| **5 回测中心** | 回测族统一（旧信号级回测 / 四组对照 / AI 推荐回测） | 按阶段 1 同款模式收敛 |

## 阶段 2 详细设计

### 2.1 收益回填（khunter 侧自建复盘数据）

- `KhunterHunting` 扩展字段：`ReturnT1`、`Return3D`、`Return5D`（% ，基准=**确认日真实成交价
  `EntryPrice`**，可执行口径）、`Reviewed bool`。
  - 与 daily_pick 口径的差异是有意的：daily_pick 是"尾盘买入"代理口径（基准=推荐日收盘），
    khunter 是"确认才买"的真实链路（基准=EntryPrice）。两者不追求数值相等，追求各自自洽。
  - 窗口定义（移植 `daily_pick_review.go` 的口径）：T+1 = EnterDate 后第 1 个交易日收盘；
    3D/5D = 第 3/5 个交易日收盘。存在才回填，未到期不填（0 值=未回填），
    到期后重复运行自动补齐。
- 新 cron 任务类型 **`khunter_review`**（09:15，对齐原 daily_pick 复盘时点，此时 T+1 收盘数据已可用）：
  对未回填（`Reviewed=false` 或窗口未补齐）的狩猎记录执行回填。
  - 持仓中：只回填已到期窗口；
  - 已退出：三个窗口照常回填（学习价值不随退出消失）；
  - `历史待核`（旧版本迁移记录，非真实持仓）：不回填，避免假样本污染统计；
  - 交易日序列复用 khunter 确认引擎的既有交易日逻辑，不新造日历。
- 接线点：`agent/cron_task_api.go` 的 `GetTaskTypes` + `executeTaskByType`、
  `system_handler.go` 的自动创建（仿 `khunter_daily_pipeline` 模式）。

### 2.2 Kronos 因子命中评估

- `khunter_kronos_predictions` 扩展：`Hit string`（hit/miss/"" ，对照**预测期末**——
  signal_date 后 predLen 个交易日的实际收盘 vs 预测方向，口径与 `DailyPick.KronosHit` 一致：
  方向对即 hit）。
- `khunter_review` 任务在回填收益的同时，对已到对账期的预测记录回写 `Hit`；
  预测明细缺失（降级记录）跳过，不计入样本。
- 因子统计（现 `daily_pick_kronos.go` 的 `kronosFactorStats`，样本=已回填 KronosHit 的
  daily_picks 行）**切源**：改读 `khunter_kronos_predictions.Hit` + 狩猎回填收益，
  输出命中率 / 平均超额收益。这是阶段 3 调权与第六维的数据地基。

### 2.3 调权切源

> **修订记录（2026-11-23 全分支评审）**：原设计写"策略胜率调权迁移到 khunter"——
> 实现裁剪为**只迁移 Kronos 因子调权**（命中率驱动，源自 `daily_pick_kronos.go`
> 的 `kronosFactorMultiplier`），策略级胜率调权（`daily_pick_weighting.go`）不迁移：
> khunter 的 `scorer.Score` 为五维固定加权，无策略权重注入点，迁移无可挂之处。
> `KhunterHunting.ReturnT1/3D/5D` 在本阶段无消费者，是阶段 3 效果看板的数据地基。
> 口径差异提示：daily_pick 跳过无方向的预测记录，khunter 的 judgePrediction 在
> `Summary.Direction` 缺失时按 ChangePct 符号兜底推导方向并计入样本——两侧样本构成
> 有微小差异，命中率跨链路对比时需注意。

- `daily_pick_weighting.go` 的策略自动调权（胜率→调权系数）迁移到 khunter 包：
  khunter 批量因子的混分权重从"配置固定值"升级为"复盘驱动的自动调权"
  （应用点在阶段 1 的混分/落库路径，只改系数来源，不改混分公式），
  数据源从 `daily_picks` 回填收益切换为 `KhunterHunting` 回填收益
  （口径对齐：T+1 可执行收益胜率；样本不足时系数=1，行为不变）。
- 迁移后 khunter 侧调权与 daily_pick 侧调权**并存双跑一个交易日周期**，
  对比两口径系数差异在容忍范围内，再执行 2.4 删除。

### 2.4 daily_pick 退役（范围修订 2026-11-23：只拆自动化，引擎保留）

**删除清单（本阶段）**：
- `app_lifecycle.go` 两个 daily_pick 定时任务（15:30 自动选股、09:15 自动复盘——
  复盘职责由 `khunter_review` 取代）；
- daily_pick 的"无人值守自动写入"路径（若有其他 cron/启动触发点一并移除）。

**保留（显式降级为手动研究工具）**：
- DailyPick 引擎、`DailyPickHandler`、SelectStock.vue 手动选股链路、`daily_picks` 表写入——
  选股工作台的选股功能依赖它们（`SelectStock.vue` ← `ScreeningWorkbench.vue`，
  是分析中心活 Tab）；
- 它们不再承担调权/因子统计数据源职责（2.2/2.3 已切 khunter），
  仅作为用户手动选股工具存在。

**并入阶段 4**：SelectStock 迁移到统一引擎后，DailyPick 引擎与 `daily_picks`
表写入的彻底删除随引擎统一完成。

**时序约束（硬）**：2.1-2.3 部署并跨至少一个完整交易日周期验证 cron 实跑
（khunter_review 09:15 回填正确、调权系数生效且回放对照 DIFF 0）之后，
方可执行 2.4 删除自动化。删除是收紧操作，不回滚假设。

**部署门控清单（2026-11-23 全分支评审补充）**：
1. 接线已上线、自动化保留（d6766a29 恢复）的状态下部署；
2. 首日 09:15 后查日志 `khunter_review 完成：…`（非零回填/对账数即实跑成功）；
3. 次日 17:00 流水线日志查 `Kronos 因子调权 命中率系数 …`（enable 翻真且样本 ≥20 时）；
4. 首周内用 `review_verify_test.go` 对库副本重跑复核，并核对交叉验证中
   前复权回溯导致的不一致是否随除权落地消失；
5. 全部通过后，才合入"删除 daily_pick 自动化"的后续 commit（即还原
   `350836a9` 中 app_lifecycle.go 的拆除 hunk）。
6. 兼容性提示：新版本库在新列上写过后，**回退老版本二进制写入的行进不了复盘**
   （hit 为 NULL 不匹配 `hit = ''` 谓词）；如需回退老版本，
   先执行 `UPDATE khunter_kronos_predictions SET hit='' WHERE hit IS NULL`。

### 2.5 验证

1. **回填正确性**：用现有 `daily_picks` 回填数据做交叉验证——同一 (code, 日期) 的
   **窗口收盘价**两边必须一致（收益值因基准不同允许系统性差异，不比收益值）。
2. **退役后回归**：`go build ./...`、`go vet ./backend/...`、定向包测试全绿
   （`backend/data` 的存量 live 网络用例环境性失败除外，已有记录）。
3. **回放对照**：退役后重跑 `khunterreplay` 五案例，与退役前输出 DIFF COUNT 0
   （证明删除未触碰引擎主链）。

## 数据流（阶段 2 完成后）

| 时间 | 任务 | 产出 |
|---|---|---|
| 15:30 | `kline_sync` | 本地日 K 同步 |
| 17:00 | `khunter_daily_pipeline` | 信号/评分/观察/确认/退出 + Kronos 批量因子 |
| 09:15 | `khunter_review`（新） | 狩猎记录 T+1/3/5 回填 + Kronos 预测 Hit 对账 + 调权系数刷新 |

```
17:00 khunter 流水线（不变）
09:15 khunter_review
  → 未回填狩猎记录：窗口收盘价回填 → ReturnT1/3D/5D → Reviewed
  → 到期预测记录：signal_date+predLen 实际收盘 vs 预测方向 → Hit
  → 因子统计刷新（命中率/平均超额）→ 调权系数（样本不足=1）
推荐视图 = 混分排序（不变）+ 可展开因子明细（阶段 3 加推理依据展示）
```

## 错误处理

| 故障 | 行为 |
|---|---|
| 窗口内某交易日 K 线缺失 | 该窗口不回填（保持 0），记运行汇总 message；不阻塞其他记录 |
| 预测明细为空（降级记录） | 跳过 Hit 对账，不计入样本；运行汇总注明跳过数 |
| 回填部分记录失败 | 成功部分照常提交，失败记录保持未回填，下次运行重试（幂等） |
| 重复触发 khunter_review | 已回填记录/窗口跳过，未到期窗口不填，结果与单次执行一致 |
| 交易日序列取不到 | 整体报错返回，不半提交 |
| `db.Dao == nil` | 返回错误，不吞 |

## 测试策略

- 回填：窗口计算纯函数单测（T+1/3/5 交易日映射、缺口跳过、到期判定）；
  注入假 K 线数据的回填流程测试。
- Hit 对账：方向判定纯函数单测（涨/跌/平边界、缺明细跳过、未到期不对账）。
- 调权切源：khunter 回填收益 → 系数的纯函数单测，与 daily_pick 口径双跑对比测试。
- cron：`khunter_review` 任务类型注册、执行分发、自动创建的单测。
- 端到端：2.5 的交叉验证 + 五案例回放对照 + 定向包回归。

## 非目标（阶段 2 明确不做）

- 不改 `scorer.Score` 五维加权公式，因子仍只在排序面（第六维属阶段 3）；
- 不做"预测质量"独立页与推理依据展示（阶段 3）；
- 不合并选股工作台引擎（阶段 4）、不做回测中心（阶段 5）；
- 不删 `daily_picks` 表（保留只读历史数据）；
- 不动阶段 1 已交付的分析中心壳、推荐视图、异步预测任务（已验证，回放对照守着）。
