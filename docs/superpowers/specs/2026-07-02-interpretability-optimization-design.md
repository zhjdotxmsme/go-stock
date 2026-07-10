# 回测与推荐界面可解释性优化方案

## 概述

优化 go-stock 三个核心界面的数据可解释性：每日选股（DailyPickPanel）、回测面板（BacktestPanel）、AI推荐股票列表（aiRecommendStocksList）。核心问题是复杂数据以原始数字平铺在表格中，缺少信息层级和视觉引导。

## 核心架构：三层信息层级

所有三个界面采用统一的信息架构模型：

```
┌──────────────────────────────────────────┐
│  Layer 1: 概览仪表盘                      │
│  趋势图 | 分布图 | 汇总统计 | 全局上下文    │
│  ─────────────────────────────────────    │
│  【固定显示在页面顶部，不依赖用户操作】       │
└──────────────────┬───────────────────────┘
                   │ 点击图表数据点 / 列表行
                   ▼
┌──────────────────────────────────────────┐
│  Layer 2: 增强对比列表                    │
│  迷你图 | 条件着色 | 因子条 | 横向比较     │
│  ─────────────────────────────────────    │
│  【点击行 → Layer 3 展开】                 │
└──────────────────┬───────────────────────┘
                   │ 点击行
                   ▼
┌──────────────────────────────────────────┐
│  Layer 3: 个股深度                        │
│  K线+信号 | 因子雷达 | 完整理由 | 交易明细  │
│  ─────────────────────────────────────    │
│  【底部抽屉/弹窗，关闭返回 Layer 2】        │
└──────────────────────────────────────────┘
```

**交互统一规则**：
- L1 ↔ L2：同页面上下布局，无导航操作，数据联动（选择日期范围 → L2 自动过滤）
- L2 → L3：点击表格行 → 底部抽屉（面板从下弹起，保留 L2 上下文）
- L3 返回：点击抽屉外部 / 关闭按钮 → 抽屉收起，回到 L2
- 三个界面使用完全一致的导航模式

每一层通过展开/点击向下钻取，用户自行控制信息深度。

---

## 一、每日选股 (DailyPickPanel)

### Layer 1: 概览仪表盘

在现有统计卡片上方追加：

| 组件 | 类型 | 数据源 | 说明 |
|---|---|---|---|
| 胜率趋势线 | 折线图 | `GetReviewTrend(N)` — 已存在 | 近 N 天每日胜率 + 7日移动平均 |
| 收益分布 | 直方图 | `GetDailyPicks(tradeDate)` 的 `nextReturn` 字段 — 前端聚合 | 次日收益率分布，标注均值/中位数 |
| 评分散布 | 散点图 | `GetDailyPicks(tradeDate)` 的 `score` + `nextReturn` — 前端聚合 | 当日各股评分 vs 次日收益 |
| 策略表现对比 | 柱状图 | `GetDailyPicks()` 按 `strategyCode` 分组 — 前端聚合 | 各策略近期胜率对比 |

**时间范围切换**：7天 / 30天 / 90天 / 全部。影响所有 L1 图表的数据范围。

**数据流向**：所有 L1 图表数据 **前端聚合** — 从现有后端 API 拉取原始数据后由前端 computed 计算图表所需格式。无需新增后端接口。

### Layer 2: 增强对比列表

对现有 17 列表格做以下改造：

**精简默认列**（从 17 列缩至 8 列）：
1. 排名
2. 股票名称
3. 综合评分 + 因子条形图（6 因子水平叠加条）
4. 涨跌幅（条件着色）
5. 次日收益（箭头 + 颜色梯度 + 与均值偏差标记）
6. 最大收益/最大回撤（合并为单列 "潜在盈亏"）
7. 选股理由摘要（30 字截断，可展开）
8. 操作（备注等）

**折叠的扩展列**（可切换显示）：
- 技术指标组（MA5/10/20, RSI14, MACD, KDJ）
- 因子得分明细（量比/均线/RSI/MACD/价格/换手率 — 带水平条形图）
- 迷你日K走势（sparkLine 组件展示近 5 日）

**视觉增强**：
- 综合评分：彩色徽章，≥80 绿，60-79 黄，<60 灰
- 因子列：`Volume ████████░░ 0.82` 水平条，长度即分值
- 次日收益：>=3% 深绿，>=1% 浅绿，<=-1% 浅红，<=-3% 深红
- 选股理由：高亮关键词（技术形态/放量/突破等）

### Layer 3: 个股深度

点击某行 → 底部弹出详情面板，包含：

- **K线图**：复用 StockLightweightKlineChart，标记买入日
- **因子雷达图**：六因子（量/均线/RSI/MACD/价格/换手率）雷达图，直观展示强弱项
- **技术指标解读**：MA/KDJ/BOLL 当前值 + 自动文本解读（如"RSI 54.2 中性偏强"）
- **选股理由原文**
- **同类历史推荐**：该股票在此前被推荐时的表现记录

---

## 二、回测面板 (BacktestPanel)

### Layer 1: 概览仪表盘

**单次回测结果** 改造：

当前展示三个指标数字 + 入场/出场价格。改为：
- 净值曲线图（持有期间每日净值，叠加沪深300基准）— 数据来自 `RunSingleBacktest` 返回的中间 K 线数据，需要后端在 `Result` 中新增 `DailyValues []float64` 字段
- 回撤曲线 — 同上，从净值计算
- 关键指标卡片组（收益率/Alpha/最大回撤/夏普/胜率）
- 滑点警告视觉标记

**批量回测结果** 改造：

当前仅一张指标表。改为：
- 净值曲线 + 基准对比 — 数据来自 `RunBatchBacktest` 每笔交易的收益率序列，前端计算累计净值
- 月度收益热力图（每月的正/负收益强度）— 前端按月份聚合每笔交易收益
- 交易盈亏散点图（每笔交易按时间分布，标记盈亏）— 数据来自 `GetBacktestResults` 分页查询结果
- 指标卡片组

**策略回测** 增加：
- 多股收益对比柱状图（各选股按收益率排序）— 直接从 `RunBacktestForDailyPicks` 返回结果前端渲染
- 持有期收益分布箱线图 — 同上

### Layer 2: 增强对比列表

策略回测结果表格增强：

- 收益率列：渐变着色（-20% 深红 → 0 白 → +20% 深绿）
- 最大回撤列：>=15% 红色高亮，>=8% 黄色警告
- 新增列：收益风险比（TotalReturn / MaxDrawdown）
- 每行添加迷你净值曲线（50px 宽 sparkline）
- 新增列：是否跑赢基准（标签 + 色块）

### Layer 3: 个股深度

点击策略回测某行 → 弹窗展示：

- 详细成交过程（入场日→出场日的每日价格路径）
- 净值曲线 + 基准对比
- 止损/止盈触发位置标记
- 滑点警告详细解释
- Alpha / Beta 分解

---

## 三、AI推荐股票列表 (aiRecommendStocksList)

### Layer 1: 概览仪表盘

当前无概览区。在表格上方新增：

| 组件 | 类型 | 数据源 | 说明 |
|---|---|---|---|
| 各模型成功率 | 柱状图 | `GetAiRecommendStocksList` 所有记录 — 前端按 `modelName` 分组聚合 | 每个 AI 模型的推荐胜率（当前价 vs 推荐价） |
| 各模型平均收益 | 柱状图 | 同上 | 每个模型的平均收益率 |
| 板块分布 | 饼图 | 同上 `bkName` 字段 | 推荐股票的板块/概念分布 |
| 推荐时间趋势 | 折线图 | 同上按 `CreatedAt` 日期聚合 | 每日推荐数量变化 |

**注意**：当前 `GetAiRecommendStocksList` 仅返回分页数据。L1 图表需要全量历史数据（或额外聚合接口），建议新加 `GetAiRecommendStats()` 后端接口返回按模型/板块/日期聚合的统计数据。

### Layer 2: 增强列表

当前 18 列信息过载。改造为分组折叠列：

**默认显示组**（核心信息）：
1. 推荐模型 + 评级（合并为单列）
2. 股票名称/代码
3. 当前价 | 涨跌幅（条件着色）
4. 推荐价（带 diff 百分比标记）
5. 价格信号状态（B/TP/SL 视觉徽章）
6. 推荐理由摘要
7. 操作

**价格信号视觉徽章**（替代当前复杂的文字比较）：
- 当前价在买入区间 → 绿色 `B` 徽章
- 当前价 ≥ 止盈价 → 橙色 `TP` 徽章
- 当前价 ≤ 止损价 → 红色 `SL` 徽章
- 正常持仓 → 灰色 `HOLD` 徽章

**可展开分组**（点击表头切换）：
- 价格信号组（开仓价/止盈价/止损价/昨收 — 带价位线视觉）
- 技术信息组（分时图、板块代码）
- 风控组（风险提示、预警开关）

### Layer 3: 个股深度

在现有 modal（K线图 + 推荐理由）基础上增强：

- K线图标记改进：买入线 B（绿虚线）、止盈线 TP（橙虚线）、止损线 SL（红虚线），线型颜色区分
- 推荐后收益率走势 mini chart（推荐日→现在的价格变化）
- 风险收益比可视化条（收益/回撤比例条）
- 推荐依据的板块/概念表现
- 同一股票历史推荐时间线（过去被推荐的时间和表现）

---

## 技术方案

| 需求 | 方案 |
|---|---|
| 图表库 | ECharts v5（Apache 2.0 协议，免费）✅ 已在 package.json |
| K线图 | 保留现有 Lightweight Charts，只增强标记层 |
| 迷你图 | 现有 sparkLine 组件（已用 ECharts）+ ECharts 迷你图 |
| Vue 集成 | vue-echarts v7.x（需 `npm install vue-echarts` 新增依赖） |
| 状态管理 | 组件内 reactive + computed，不引入 Pinia |
| 暗色主题 | ECharts 注册 `dark` 主题 + 读取现有 `darkTheme` 配置切换 |

### 新增依赖

```bash
npm install vue-echarts@^7.0.0
```

`echarts` 已存在（^5.6.0），只需安装 wrapper。

### 图表组件复用

新增以下可复用图表组件到 `frontend/src/components/charts/`：

```
frontend/src/components/charts/
  TrendLine.vue          — 通用趋势折线图（支持 dark/light 切换）
  DistributionHist.vue   — 分布直方图（标注均值/中位数）
  RadarChart.vue         — 因子雷达图（支持多组对比）
  BarCompare.vue         — 对比柱状图（支持多维度）
  EquityCurve.vue        — 净值曲线（叠加基准，含回撤区域）
  MonthlyHeatmap.vue     — 月度收益热力图
  FactorBar.vue          — 因子水平条形图（小型 inline，用于表格内）
```

### 通用组件约定

所有图表组件遵循以下约定：
- **Prop**: `dark: boolean` — 切换暗色主题
- **Prop**: `loading: boolean` — 显示加载骨架屏
- **Prop**: `empty: boolean` — 无数据时展示空状态图
- **Emit**: `dataPointClick(payload)` — 点击数据点时触发，用于 L1 → L2 联动
- **响应式**: 监听容器 resize，调用 `chart.resize()`
- **空态**: ECharts `graphic` 层展示 "暂无数据" 文本
- **加载态**: ECharts `showLoading()` + spinner

### ECharts 注册策略

`vue-echarts` + 按需 `registerMap`/`registerTheme`，不全局注册所有图表类型以减少包体积。

---

## 实施顺序与任务拆解

### Phase 1: 基础设施（1 个任务）

| ID | 任务 | 文件 | 描述 |
|----|------|------|------|
| P1 | 安装 vue-echarts 依赖 + 创建图表组件目录 | `frontend/package.json` + `frontend/src/components/charts/` | `npm install vue-echarts@^7`，新建目录，创建 `charts/index.ts` 统一导出 |
| P1.1 | `TrendLine.vue` | `charts/TrendLine.vue` | 通用折线图组件，Prop: data, xKey, yKey, title, dark, loading, empty |
| P1.2 | `DistributionHist.vue` | `charts/DistributionHist.vue` | 直方图组件，Prop: data, valueKey, title, meanLabel, medianLabel |
| P1.3 | `RadarChart.vue` | `charts/RadarChart.vue` | 雷达图组件，Prop: indicators, data, max |
| P1.4 | `BarCompare.vue` | `charts/BarCompare.vue` | 对比柱状图组件，Prop: categories, series |
| P1.5 | `EquityCurve.vue` | `charts/EquityCurve.vue` | 净值曲线组件，含基准叠加、回撤区域填充 |
| P1.6 | `MonthlyHeatmap.vue` | `charts/MonthlyHeatmap.vue` | 月度热力图组件 |
| P1.7 | `FactorBar.vue` | `charts/FactorBar.vue` | 内联水平条形图组件，用于表格单元格内 |

**验收标准**：每个图表组件独立运行，支持 dark/light 切换，加载态/空态展示正确。可在 Storybook 或单独页面测试。

### Phase 2: DailyPickPanel 优化（5 个任务，按 L2→L1→L3 顺序）

| ID | 任务 | 交付物 | 验收标准 |
|----|------|--------|---------|
| D1 | **L2 列精简 + 因子条** | `DailyPickPanel.vue` | 默认列从 17 减至 8；因子得分列使用 FactorBar 渲染；评分列使用彩色徽章 |
| D2 | **L2 条件着色 + 迷你图** | `DailyPickPanel.vue` | 次日收益列红绿渐变色；每行添加 sparkLine 迷你走势；选股理由可展开 |
| D3 | **L1 胜率趋势线 + 时间切换** | `DailyPickPanel.vue` + `GetReviewTrend()` | 折线图展示近 7/30/90 天胜率趋势；时间切换器触发图表数据刷新 |
| D4 | **L1 收益分布 + 评分散点 + 策略对比** | `DailyPickPanel.vue` | 三个图表正确渲染；与时间选择器联动 |
| D5 | **L3 底部抽屉 + 因子雷达 + 指标解读** | 新组件 `DailyPickDetail.vue` + `RadarChart` | 点击行弹出底部抽屉；展示因子雷达图；技术指标自动解读文本 |

### Phase 3: BacktestPanel 优化（4 个任务）

| ID | 任务 | 交付物 | 验收标准 |
|----|------|--------|---------|
| B1 | **后端: 单次回测返回净值序列** | `backend/data/backtest/engine.go` `Result` 新增 `DailyValues []float64` | RunSingleBacktest 返回值包含每日净值数组 |
| B2 | **L1 单次回测: 净值曲线 + 回撤曲线** | `BacktestPanel.vue` + `EquityCurve` | 单次回测结果区域展示净值曲线（叠加基准）、回撤曲线 |
| B3 | **L1 批量回测: 净值曲线 + 月度热力图 + 盈亏散点** | `BacktestPanel.vue` | 批量回测结果包含净值曲线、月度热力图、散点图 |
| B4 | **L2 策略回测: 对比着色 + 迷你净值线 + 收益风险比** | `BacktestPanel.vue` | 渐变着色、每行迷你净值线、新增收益风险比列 |

### Phase 4: aiRecommendStocksList 优化（4 个任务）

| ID | 任务 | 交付物 | 验收标准 |
|----|------|--------|---------|
| A1 | **后端: 新增 `GetAiRecommendStats()` 聚合接口** | `backend/data/ai_recommend_stocks_api.go` | 返回按模型/板块/日期聚合的统计数据 |
| A2 | **L1 概览仪表盘** | `aiRecommendStocksList.vue` | 模型成功率柱状图 + 板块分布饼图 + 推荐趋势线 |
| A3 | **L2 分组折叠列 + 价格信号徽章** | `aiRecommendStocksList.vue` | 列分组折叠可切换；价格信号以 B/TP/SL 徽章显示 |
| A4 | **L3 K线图标记增强 + 历史时间线** | `aiRecommendStocksList.vue` + modal | K线图显示买入/止盈/止损价位线；展示同一股票历史推荐时间线 |

### Phase 5: 收尾（1 个任务）

| ID | 任务 | 交付物 | 验收标准 |
|----|------|--------|---------|
| F1 | 暗色主题全量验证 + 响应式适配 + QA | 所有修改过的面板 | 切换深色/浅色主题后所有图表正确渲染；窗口缩放图表自适应；表格滚动流畅 |

---

## 非功能要求

- 图表首次渲染 ≤ 500ms（ECharts 实例化 + 数据注入）
- 表格滚动保持 60fps（已有虚拟滚动）
- ECharts 按需注册，不全局引入所有图表类型
- 暗色主题适配：ECharts 注册 `dark` 主题，所有图表组件通过 `props.dark` 切换
- 所有图表组件需覆盖以下状态：（1）正常渲染 （2）加载中（loading spinner） （3）空数据（"暂无数据"） （4）错误（error callback）

### QA 验证场景

| 场景 | 操作 | 预期 |
|------|------|------|
| 胜率趋势线 | 切换到 90 天 | 图表正确重绘，X 轴显示日期 |
| 收益分布图 | 切换日期 | 分布柱状图更新，均值线重新计算 |
| 底部抽屉 | 点击 L2 某行 | 抽屉从底部弹出，不遮挡表格标题 |
| 暗色切换 | 系统/设置切换 dark 模式 | 所有图表切换为暗色背景+浅色文字 |
| 空数据 | 日期无选股结果 | 图表显示 "暂无数据" 占位 |
| 价格信号徽章 | 当前价格触及止损 | 显示红色 SL 徽章 |
| 净值曲线 | 批量回测完成 | 曲线展示每日净值变化，基准叠加对比 |
