# Interpretability 界面可解释性优化 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化三个界面的数据可解释性：每日选股（DailyPickPanel）、回测面板（BacktestPanel）、AI推荐股票列表（aiRecommendStocksList）。采用三层信息架构（概览仪表盘 → 增强对比列表 → 个股深度），让用户从宏观到微观逐步理解数据。

**Architecture:** 统一的三层信息层级模型。L1（概览图表）+ L2（增强列表）同页面上下布局，L3（个股深度）通过底部抽屉展开。图表基于 ECharts v5 + vue-echarts v7，K线图保留现有 Lightweight Charts。所有数据优先从现有后端 API 聚合，仅 aiRecommendStats 需新增后端接口。

**Tech Stack:** Vue 3, NaiveUI, ECharts v5, vue-echarts v7, Lightweight Charts v5, Go 1.26, GORM/SQLite

---

## 文件变更总览

| 文件 | 操作 | 说明 |
|------|------|------|
| `frontend/package.json` | 修改 | 新增 `vue-echarts` 依赖 |
| `frontend/src/components/charts/TrendLine.vue` | **新建** | 通用折线图组件 |
| `frontend/src/components/charts/DistributionHist.vue` | **新建** | 分布直方图组件 |
| `frontend/src/components/charts/RadarChart.vue` | **新建** | 因子雷达图组件 |
| `frontend/src/components/charts/BarCompare.vue` | **新建** | 对比柱状图组件 |
| `frontend/src/components/charts/EquityCurve.vue` | **新建** | 净值曲线组件 |
| `frontend/src/components/charts/MonthlyHeatmap.vue` | **新建** | 月度热力图组件 |
| `frontend/src/components/charts/FactorBar.vue` | **新建** | 内联因子水平条组件 |
| `frontend/src/components/charts/index.ts` | **新建** | 图表组件统一导出 |
| `frontend/src/components/DailyPickPanel.vue` | 修改 | L2 列精简+因子条+条件着色 → L1 概览图表 → L3 底部抽屉 |
| `frontend/src/components/DailyPickDetail.vue` | **新建** | 每日选股详情底部抽屉组件 |
| `frontend/src/components/BacktestPanel.vue` | 修改 | L1 净值曲线+指标卡片 → L2 增强对比 → 策略回测增强 |
| `frontend/src/components/aiRecommendStocksList.vue` | 修改 | L1 概览仪表盘 → L2 分组折叠+信号徽章 → L3 K线增强 |
| `backend/data/backtest/engine.go` | 修改 | `Result` 新增 `DailyValues []float64` 字段 |
| `backend/data/backtest/service.go` | 修改 | `RunSingleBacktest` 填充净值序列 |
| `backend/data/ai_recommend_stocks_api.go` | 修改 | 新增 `GetAiRecommendStats()` 聚合接口 |

---

## Phase 1: 基础设施

### Task 1.1: 安装 vue-echarts 依赖 + 创建图表组件目录

**Files:**
- Modify: `frontend/package.json`
- **New**: `frontend/src/components/charts/`
- **New**: `frontend/src/components/charts/index.ts`

- [ ] **Step 1: 安装依赖**

  ```bash
  cd frontend && npm install vue-echarts@^7.0.0
  ```

- [ ] **Step 2: 创建目录结构**

  ```
  frontend/src/components/charts/
    index.ts
  ```

  `index.ts` 统一导出所有图表组件：
  ```ts
  export { default as TrendLine } from './TrendLine.vue'
  export { default as DistributionHist } from './DistributionHist.vue'
  export { default as RadarChart } from './RadarChart.vue'
  export { default as BarCompare } from './BarCompare.vue'
  export { default as EquityCurve } from './EquityCurve.vue'
  export { default as MonthlyHeatmap } from './MonthlyHeatmap.vue'
  export { default as FactorBar } from './FactorBar.vue'
  ```

---

### Task 1.2: TrendLine.vue — 通用趋势折线图

**Files:**
- **New**: `frontend/src/components/charts/TrendLine.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Props {
    data: Array<{ date: string; value: number }>
    title?: string
    xKey?: string    // default: 'date'
    yKey?: string    // default: 'value'
    dark?: boolean
    loading?: boolean
    empty?: boolean
    height?: number  // default: 240
    smooth?: boolean // 平滑曲线, default: true
    areaStyle?: boolean // 填充面积, default: false
    markLine?: { label: string; value: number } // 参考线
    yUnit?: string   // 纵轴单位, default: '%'
  }
  ```

- [ ] **Step 2: ECharts 配置**

  - X 轴: `category` 类型，标签旋转 45° 防重叠
  - Y 轴: `value` 类型，format `{value}%`
  - Tooltip: trigger `axis`，显示日期+数值
  - dark 模式: 切换 ECharts `dark` theme
  - 空态: `graphic` 层绘制 "暂无数据"
  - 加载态: `showLoading()` + `hideLoading()`
  - 响应式: `ResizeObserver` 监听容器尺寸

- [ ] **Step 3: Emit**

  - `dataPointClick: { date: string; value: number }` — 点击数据点

---

### Task 1.3: DistributionHist.vue — 分布直方图

**Files:**
- **New**: `frontend/src/components/charts/DistributionHist.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Props {
    data: number[]              // 数值列表
    buckets?: number            // 分桶数, default: 20
    title?: string
    dark?: boolean
    loading?: boolean
    empty?: boolean
    height?: number
    showStats?: boolean         // 显示均值/中位数线, default: true
  }
  ```

- [ ] **Step 2: 逻辑**

  - 前端将 `data` 分桶（`d3-array` 的 `bin` 逻辑或手动实现）
  - 均值线、中位数线（`markLine`）
  - 超过 ±3σ 的数据点标记为异常值

---

### Task 1.4: RadarChart.vue — 因子雷达图

**Files:**
- **New**: `frontend/src/components/charts/RadarChart.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Indicator { name: string; max: number }
  interface Props {
    indicators: Indicator[]       // 因子定义: [{ name: '量比', max: 1 }]
    data: number[]               // 对应分值
    dark?: boolean
    loading?: boolean
    height?: number              // default: 280
  }
  ```

---

### Task 1.5: BarCompare.vue — 对比柱状图

**Files:**
- **New**: `frontend/src/components/charts/BarCompare.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Series {
    name: string
    data: number[]
    color?: string
  }
  interface Props {
    categories: string[]          // X 轴分类名称
    series: Series[]              // 系列数据
    title?: string
    dark?: boolean
    loading?: boolean
    empty?: boolean
    height?: number
    horizontal?: boolean          // 横向柱状图
  }
  ```

---

### Task 1.6: EquityCurve.vue — 净值曲线（叠加基准）

**Files:**
- **New**: `frontend/src/components/charts/EquityCurve.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Props {
    dailyValues: Array<{ date: string; value: number }>  // 策略净值
    benchmark?: Array<{ date: string; value: number }>   // 基准净值
    drawdown?: boolean     // 显示回撤区域, default: true
    title?: string
    dark?: boolean
    loading?: boolean
    height?: number        // default: 300
    initialCapital?: number // default: 1 (净值归一)
  }
  ```

- [ ] **Step 2: 特性**

  - 双 Y 轴（左侧净值，右侧回撤%）
  - 基准净值灰色虚线叠加
  - 回撤区域用红色填充 `areaStyle`
  - 区域缩放 `dataZoom`

---

### Task 1.7: MonthlyHeatmap.vue — 月度收益热力图

**Files:**
- **New**: `frontend/src/components/charts/MonthlyHeatmap.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Props {
    data: Array<{ year: number; month: number; value: number }>
    title?: string
    dark?: boolean
    loading?: boolean
    height?: number
  }
  ```

- [ ] **Step 2: ECharts 配置**

  - `calendar` 坐标系
  - 颜色范围: 红（正）→ 白（零）→ 绿（负）

---

### Task 1.8: FactorBar.vue — 内联因子水平条形图

**Files:**
- **New**: `frontend/src/components/charts/FactorBar.vue`

- [ ] **Step 1: 组件 Props**

  ```ts
  interface Factor { name: string; value: number; color?: string }
  interface Props {
    factors: Factor[]
    max?: number           // 最大值, default: 1
    height?: number        // default: 16
    width?: number         // default: 120
    showLabel?: boolean    // 显示因子名称, default: true
    showValue?: boolean    // 显示数值, default: true
  }
  ```

- [ ] **Step 2: 渲染**

  - 使用 ECharts 极简水平条或纯 CSS（推荐纯 CSS，更轻量）
  - 颜色随值变化: >=0.8 绿, 0.5-0.8 黄, <0.5 灰
  - 内嵌在表格单元格中使用

---

## Phase 2: DailyPickPanel 优化

### Task 2.1: L2 列精简 + 因子条

**Files:**
- Modify: `frontend/src/components/DailyPickPanel.vue`

- [ ] **Step 1: 重定义 columns**

  默认展示列（精简到 8 列）:
  1. 排名（`rank`）
  2. 股票名称（`stockName` + `stockCode` 合并）
  3. 综合评分（彩色徽章 `NTag`，≥80 绿，60-79 黄，<60 灰）
  4. 涨跌幅（条件着色，`NText` 红绿）
  5. 次日收益（箭头 + 颜色梯度）
  6. 潜在盈亏（`nextMaxReturn` / `nextMaxDrawdown` 合并列）
  7. 选股理由（截断 30 字，点击展开）
  8. 备注/操作

- [ ] **Step 2: 评分列重构**

  当前 `renderScore` 只显示数字。改为彩色徽章 + 迷你因子条：

  ```html
  <span>
    <n-tag :type="score >= 80 ? 'success' : score >= 60 ? 'warning' : 'info'" size="small">
      {{ score.toFixed(1) }}
    </n-tag>
    <FactorBar v-if="expanded" :factors="rowFactors(row)" />
  </span>
  ```

  因子条显示 6 因子：`volumeFactor`, `maFactor`, `rsiFactor`, `macdFactor`, `priceFactor`, `turnoverFactor`。

- [ ] **Step 3: 次日收益列用颜色渐变**

  `nextReturn` >= 3% 深绿 `#18a058`，>=1% 浅绿，<=-1% 浅红，<=-3% 深红 `#d03050`。正收益加 `+` 前缀。

- [ ] **Step 4: 选股理由列可展开**

  默认显示前 30 字 + `...`，点击展开全文。使用 `NEllipsis` + `expand-trigger="click"`。

- [ ] **Step 5: 可切换扩展列**

  在表格上方添加「显示更多列」开关，切换时显示/隐藏：
  - 技术指标组 (MA5/10/20, RSI14, MACD, KDJ)
  - 因子得分组 (6 因子数值)
  - 迷你走势列 (sparkLine 组件)

---

### Task 2.2: L2 迷你走势 + 条件着色

**Files:**
- Modify: `frontend/src/components/DailyPickPanel.vue`

- [ ] **Step 1: 每行添加迷你走势**

  在「股票名称」列或新列中，使用 `stockSparkLine` 组件（已有）展示近 5 日价格走势。数据从 `ClosePrice` + 历史 K 线（如有）构建。

- [ ] **Step 2: 条件着色统一**

  - 涨跌幅列: `NText` type=success/error
  - 评分列: `NTag` type=success/warning/info
  - 次日收益: 自定义颜色渐变的 `NText`

---

### Task 2.3: L1 胜率趋势线 + 时间切换

**Files:**
- Modify: `frontend/src/components/DailyPickPanel.vue`（追加到统计卡片上方）

- [ ] **Step 1: 调用 `GetReviewTrend(limit)`**

  创建 computed 属性 `winRateData`，调用 `GetReviewTrend(activeDays)` 获取数据，时间切换（7/30/90/全部）改变 `activeDays`。

- [ ] **Step 2: 渲染 TrendLine**

  在统计卡片上方添加 TrendLine 组件：

  ```html
  <n-card size="small" title="胜率趋势">
    <TrendLine
      :data="winRateData"
      :dark="darkTheme"
      :loading="loading"
      height="200"
      area-style
      title="每日胜率"
    />
  </n-card>
  ```

- [ ] **Step 3: 时间选择器**

  ```html
  <n-radio-group v-model:value="activeDays" @update:value="refreshWinRate">
    <n-radio-button :value="7">7天</n-radio-button>
    <n-radio-button :value="30">30天</n-radio-button>
    <n-radio-button :value="90">90天</n-radio-button>
    <n-radio-button :value="0">全部</n-radio-button>
  </n-radio-group>
  ```

---

### Task 2.4: L1 收益分布 + 评分散点 + 策略对比

**Files:**
- Modify: `frontend/src/components/DailyPickPanel.vue`

- [ ] **Step 1: 收益分布直方图**

  `DistributionHist` 数据来自 `GetDailyPicks` 的 `nextReturn` 字段。日期范围联动时间选择器。

- [ ] **Step 2: 评分散点图**

  `BarCompare` 数据来自 `GetDailyPicks` 的 `score` + `nextReturn`（X=评分，Y=次日收益）。

- [ ] **Step 3: 策略表现柱状图**

  `BarCompare` 数据来自 `GetDailyPicks` 按 `strategyName` 分组聚合胜率。

- [ ] **Step 4: 布局**

  使用 NaiveUI `n-grid` 排列四个图表（2×2 或按需 4 列）：

  ```
  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │  胜率趋势线   │ │  收益分布    │ │  评分散点    │ │  策略对比    │
  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
  ```

  小屏幕自动折行。

---

### Task 2.5: L3 底部抽屉 + 因子雷达 + 指标解读

**Files:**
- **New**: `frontend/src/components/DailyPickDetail.vue`
- Modify: `frontend/src/components/DailyPickPanel.vue`

- [ ] **Step 1: 创建底部抽屉组件**

  使用 NaiveUI `n-drawer` + `n-drawer-content`，placement=bottom，高度 60vh：

  ```html
  <n-drawer v-model:show="showDetail" placement="bottom" :height="'60vh'">
    <n-drawer-content :title="selectedPick.stockName">
      <RadarChart :indicators="factorIndicators" :data="factorValues" />
      <n-divider />
      <StockLightweightKlineChart :code="stockCode" :stock-name="stockName" />
      <n-divider />
      <n-text>{{ selectedPick.reason }}</n-text>
    </n-drawer-content>
  </n-drawer>
  ```

- [ ] **Step 2: 因子雷达**

  `RadarChart` 展示 6 因子得分。

- [ ] **Step 3: K线图**

  复用 `StockLightweightKlineChart`，传入股票代码。买入日标记（用现有 `longEntryPrice` 机制）。

- [ ] **Step 4: 技术指标解读**

  对 `rsi14` / `macd` / `ma5` / `ma10` / `ma20` 等字段做硬编码范围解读：
  - RSI: <30 超卖, 30-50 偏弱, 50-70 偏强, >70 超买
  - MACD > 0: 多头, < 0: 空头
  - 当前价 > MA5 > MA10 > MA20: 多头排列

  用 `n-descriptions` 列表展示。

- [ ] **Step 5: 表格行绑定**

  `DailyPickPanel.vue` 的表格每行绑定 `onClick` → `openDetail(row)`。

---

## Phase 3: BacktestPanel 优化

### Task 3.1: 后端 Result 增加净值序列

**Files:**
- Modify: `backend/data/backtest/engine.go`

- [ ] **Step 1: `Result` 新增 `DailyValues` 字段**

  ```go
  type Result struct {
      // ... existing fields
      DailyValues  []float64 `json:"dailyValues"` // 每日归一化净值
  }
  ```

- [ ] **Step 2: `Engine.Run` 中填充 DailyValues**

  循环中记录每日收盘价净值：`dailyValues[i] = (bar.Close - entry) / entry`。基准净值同理存入 `BenchmarkValues`。

---

### Task 3.2: L1 单次回测净值曲线

**Files:**
- Modify: `frontend/src/components/BacktestPanel.vue`

- [ ] **Step 1: 后端返回净值序列后，前端渲染 EquityCurve**

  单次回测结果区域新增：

  ```html
  <n-card title="净值曲线" size="small">
    <EquityCurve
      :daily-values="singleResult.dailyValues"
      :benchmark="singleResult.benchmarkValues"
      :dark="darkTheme"
      height="280"
    />
  </n-card>
  ```

- [ ] **Step 2: 关键指标卡片组**

  当前 3 个指标（总收益/最大回撤/Alpha）改为 NaiveUI `n-statistic` 卡片布局，添加颜色编码（正收益绿/负收益红）。

---

### Task 3.3: L1 批量回测图表

**Files:**
- Modify: `frontend/src/components/BacktestPanel.vue`

- [ ] **Step 1: 净值曲线**

  从 `batchResult` 的每笔交易收益率构建累计净值曲线。使用 `EquityCurve`。

- [ ] **Step 2: 月度热力图**

  从 `batchResult` 交易数据按年月聚合收益，用 `MonthlyHeatmap`。

- [ ] **Step 3: 盈亏散点图**

  用 `BarCompare` 或独立散点图展示每笔交易按时间分布。

---

### Task 3.4: L2 策略回测增强

**Files:**
- Modify: `frontend/src/components/BacktestPanel.vue`

- [ ] **Step 1: 收益率渐变着色**

  在 `strategyColumns` 的 `totalReturn` 列渲染中，用 JavaScript 计算颜色插值：-20% → `#d03050`，0% → `#fff`，+20% → `#18a058`。颜色作为 `style` 应用。

- [ ] **Step 2: 迷你净值线**

  每行新增列，用 `sparkLine` 组件或 ECharts 迷你图渲染。数据来自 `RunBacktestForDailyPicks` 返回结果（如果后端返回净值序列）或模拟。

- [ ] **Step 3: 新增收益风险比列**

  `(r.totalReturn / Math.abs(r.maxDrawdown)).toFixed(2)`，条件着色 > 2 绿色，>1 黄色，<1 红色。

---

## Phase 4: aiRecommendStocksList 优化

### Task 4.1: 新增 GetAiRecommendStats 后端接口

**Files:**
- Modify: `backend/data/ai_recommend_stocks_api.go`

- [ ] **Step 1: 新增方法**

  ```go
  // GetAiRecommendStats 返回按模型、板块、日期聚合的统计数据
  type AiRecommendStats struct {
      ByModel    []ModelStat  `json:"byModel"`
      BySector   []SectorStat `json:"bySector"`
      DailyCount []DailyCount `json:"dailyCount"`
  }

  type ModelStat struct {
      ModelName string  `json:"modelName"`
      WinRate   float64 `json:"winRate"`
      AvgReturn float64 `json:"avgReturn"`
      Count     int     `json:"count"`
  }

  type SectorStat struct {
      BkName string `json:"bkName"`
      Count  int    `json:"count"`
  }

  type DailyCount struct {
      Date  string `json:"date"`
      Count int    `json:"count"`
  }

  func (s *AiRecommendStocksService) GetAiRecommendStats() (*AiRecommendStats, error) {
      // 查询所有记录，按 modelName 分组统计胜率（当前价 vs 推荐价）
      // 按 bkName 分组计数
      // 按 dataTime 日期分组计数
  }
  ```

- [ ] **Step 2: 注册 Wails 绑定**

  在 `app.go` 中将 `GetAiRecommendStats` 暴露给前端。

---

### Task 4.2: L1 概览仪表盘

**Files:**
- Modify: `frontend/src/components/aiRecommendStocksList.vue`

- [ ] **Step 1: 请求统计数据**

  在 `onMounted` 中调用 `GetAiRecommendStats()`。

- [ ] **Step 2: 渲染仪表盘**

  在表格上方添加：

  ```
  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐
  │ 模型成功率(柱状) │ │ 平均收益率(柱状) │ │ 板块分布(饼图)  │ │ 推荐趋势(折线)  │
  └────────────────┘ └────────────────┘ └────────────────┘ └────────────────┘
  ```

  `BarCompare` × 2（模型成功率 + 平均收益率）
  `TrendLine` × 1（推荐趋势）
  板块分布用 ECharts 饼图 `pie` 系列。

---

### Task 4.3: L2 分组折叠列 + 价格信号徽章

**Files:**
- Modify: `frontend/src/components/aiRecommendStocksList.vue`

- [ ] **Step 1: 列重定义**

  默认显示组：
  1. 模型 + 评级（合并）
  2. 股票名称/代码
  3. 当前价 | 涨跌幅（条件着色）
  4. 推荐价 diff（百分比标记）
  5. **价格信号徽章**（新渲染逻辑）
  6. 推荐理由摘要
  7. 操作

- [ ] **Step 2: 价格信号徽章渲染**

  ```ts
  function renderSignal(row) {
    const current = Number(row.stockCurrentPrice)
    const buyMin = row.recommendBuyPriceMin
    const buyMax = row.recommendBuyPriceMax
    const tp = row.recommendStopProfitPriceMin
    const sl = row.recommendStopLossPrice

    if (current >= buyMin && current <= buyMax) return h(NTag, { type: 'success' }, 'B')
    if (tp && current >= tp) return h(NTag, { type: 'warning' }, 'TP')
    if (sl && current <= sl) return h(NTag, { type: 'error' }, 'SL')
    return h(NTag, { type: 'default' }, 'HOLD')
  }
  ```

- [ ] **Step 3: 可展开分组**

  表头增加分组切换按钮「价格信号」「技术信息」「风控」，点击切换对应组列的可见性。

---

### Task 4.4: L3 K线图标记增强 + 历史时间线

**Files:**
- Modify: `frontend/src/components/aiRecommendStocksList.vue`
- Modify: `frontend/src/components/StockLightweightKlineChart.vue`

- [ ] **Step 1: K线图价位线增强**

  在 `showDetail` modal 的 K线图中：
  - 买入价线：绿色虚线 `longEntryPrice`
  - 止盈价线：橙色虚线 `longTakeProfitPrice`
  - 止损价线：红色虚线 `longStopLossPrice`
  - 需要确认 `StockLightweightKlineChart` 是否已支持多价位线

- [ ] **Step 2: 推荐历史时间线**

  在 modal 中新增「历史推荐」区域，用 `TrendLine` 或简单列表展示该股票此前被 AI 推荐的时间点和涨跌幅。

---

## Phase 5: 收尾

### Task 5.1: 暗色主题 + 响应式 + QA

**Files:**
- Modify: `frontend/src/components/DailyPickPanel.vue`
- Modify: `frontend/src/components/BacktestPanel.vue`
- Modify: `frontend/src/components/aiRecommendStocksList.vue`

- [ ] **Step 1: 暗色主题验证**

  - 所有图表组件读取 `dark` prop 切换 ECharts theme
  - 切换系统/设置 dark 模式后仪表盘自动刷新

- [ ] **Step 2: 响应式适配**

  - 图表容器监听 resize，调用 `chart.resize()`
  - Wails 窗口缩放不会导致图表变形

- [ ] **Step 3: 加载/空态/错误态覆盖**

  - 每个 L1 图表组件初始状态显示 loading spinner
  - 无数据时显示 "暂无数据" 占位
  - API 错误时显示错误提示（`n-alert type="error"`）

- [ ] **Step 4: QA 场景验证**

  验证以下场景：
  1. 每日选股 - 切换 30 天时间范围 → 趋势线、分布图联动更新
  2. 每日选股 - 点击某行 → 底部抽屉出现，因子雷达正确渲染
  3. 回测面板 - 单次回测完成 → 净值曲线显示，基准叠加正确
  4. 回测面板 - 策略回测完成 → 渐变着色正确，收益风险比列显示
  5. AI 推荐 - 模型成功率柱状图按模型分组正确
  6. AI 推荐 - 价格达到止损 → 徽章显示为 SL
  7. 全局 - 切换暗色主题 → 所有图表颜色适配

---

## 依赖关系

```
P1 (基础设施)
├── D1 → D2 → D3 → D4 → D5
├── B1 → B2 → B3 → B4
└── A1 → A2 → A3 → A4
         └── P5 (收尾)
```

P1 是所有 Phase 的前置依赖。D1-D5（每日选股）和 A1-A4（AI推荐）任务间有强依赖关系，必须按顺序完成。B1-B4（回测面板）大部分独立，B1后端改变不阻塞前端任务。P5 收尾最后执行。

---

## 验收检查清单

- [ ] 每日选股：默认列从 17 减至 8
- [ ] 每日选股：因子水平条形图显示 6 因子分值
- [ ] 每日选股：次日收益列红绿渐变着色
- [ ] 每日选股：5 个 L1 图表正确渲染
- [ ] 每日选股：时间切换器联动所有 L1 图表
- [ ] 每日选股：点击行弹出底部抽屉
- [ ] 每日选股：抽屉内因子雷达 + K线图 + 指标解读
- [ ] 回测面板：单次回测显示净值曲线
- [ ] 回测面板：策略回测收益率渐变着色
- [ ] 回测面板：策略回测每行迷你净值线
- [ ] 回测面板：批量回测月度热力图
- [ ] AI 推荐：L1 概览仪表盘 4 个图表
- [ ] AI 推荐：L2 分组折叠列可切换
- [ ] AI 推荐：价格信号徽章 B/TP/SL/HOLD
- [ ] AI 推荐：K线图显示 3 条价位线
- [ ] 全局：暗色模式切换后所有图表适配
- [ ] 全局：窗口缩放图表自适应
- [ ] 全局：所有图表覆盖加载态、空态、错误态
