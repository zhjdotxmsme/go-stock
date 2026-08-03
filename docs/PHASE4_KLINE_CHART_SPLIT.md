# Phase 4: StockLightweightKlineChart.vue 组件拆分计划

> 目标：将 4832 行的 K线图单体组件拆分为模块化、可维护的架构

## 当前状态分析

**StockLightweightKlineChart.vue: 4832 行**

主要功能模块：
1. **指标切换状态** (行 64-113) - 40+ 个技术指标的显示开关
2. **筹码分布** (行 114-130) - 筹码计算和绘制
3. **多头仓位模拟** (行 119-130) - 仓位计算器
4. **图表核心逻辑** (行 280-450) - 数据提取、绘制函数
5. **子窗格管理** (行 400-453) - tearDownAllSubPanes, syncSubPaneIndicators
6. **40+ 技术指标计算** (行 459-1320) - syncIndicators 巨型函数
7. **K线类型切换** (行 1321-2037) - 分时、日K、周K、月K等
8. **十字光标交互** (行 2038-2160) - 格式化、行查找、面板渲染
9. **信号评估系统** (行 2159-2400) - evaluateIndicatorSignals 巨型函数
10. **图表生命周期** - onMounted, onBeforeUnmount, watchers
11. **Template** - 工具栏、图表容器、仓位面板、指标设置弹窗

## 拆分策略

### 分层架构

```
components/kline/
├── composables/
│   ├── useChartLifecycle.js    # 图表生命周期管理
│   ├── useIndicators.js        # 40+ 技术指标计算与渲染
│   ├── useCrosshair.js         # 十字光标交互
│   ├── useChipDistribution.js  # 筹码分布计算与绘制
│   ├── usePositionSim.js       # 仓位模拟器
│   └── useSignalEvaluation.js  # 指标信号评估系统
├── components/
│   ├── KlineToolbar.vue         # K线类型 + 指标切换工具栏
│   ├── KlineChartContainer.vue  # 图表容器 + Crosshair
│   ├── ChipDistribution.vue     # 筹码分布面板
│   ├── PositionSimPanel.vue     # 仓位模拟面板
│   └── IndicatorSettings.vue    # 指标设置弹窗
├── constants.ts                 # 已有常量定义
├── format.ts                    # 已有格式化函数
├── time.ts                      # 已有时间处理
├── calc.ts                      # 已有计算工具
└── StockLightweightKlineChart.vue # 根组件 (重构后约 200 行)
```

## 拆分步骤 (增量式，每步可独立验证)

### Step 1: 建立 kline 目录结构 (P0)
- ✅ 已有基础目录
- ✅ 已有 constants.ts, format.ts, time.ts, calc.ts
- ⏳ 创建 composables/ 子目录
- ⏳ 创建 components/ 子目录

**提交**: `refactor(kline): setup directory structure for component split`

### Step 2: 提取 useChartLifecycle composable (P0)
- 图表初始化 (initChart)
- 图表销毁 (tearDownAllSubPanes)
- 数据源切换
- 数据加载状态

**文件**: `components/kline/composables/useChartLifecycle.js`

**提交**: `refactor(kline): extract chart lifecycle composable`

### Step 3: 提取 useIndicators composable (P0 - 最高优先级)
将 `syncIndicators()` 巨型函数 (862 行!) 拆分：
- MA, EMA, BOLL, VWAP, DEMA, TEMA, KAMA, HullMA
- MACD, KDJ, RSI, CCI, ATR, OBV, MFI, CMF
- Supertrend, Keltner, Ichimoku, SAR, Donchian, ADX
- WilliamsR, StochRSI, Aroon, CMO, ForceIndex, Pivot
- ZigZag, SATS, AvgAmp, Alligator, AO, AD, TRIX, ROC
- Fractal, CHOP, ElderRay, ChaikinOsc, VWAPBands
- MassIndex, UlcerIndex, Coppock, SMI, SignalRatio, SMC

**文件**: `components/kline/composables/useIndicators.js`
**策略**: 按指标类别分组导出

**提交**: `refactor(kline): extract indicators calculation to composable`

### Step 4: 提取 useCrosshair composable (P1)
- formatCrosshairTime
- findRawRowByChartTime
- syncDefaultLatestPanelRow
- formatPanelTitleDay
- crosshairPanel computed

**文件**: `components/kline/composables/useCrosshair.js`

**提交**: `refactor(kline): extract crosshair interaction composable`

### Step 5: 提取 useSignalEvaluation composable (P1)
将 `evaluateIndicatorSignals()` 巨型函数拆分：
- 趋势信号 (MA, EMA, BOLL, Supertrend)
- 震荡指标 (RSI, KDJ, CCI, WilliamsR)
- 成交量信号 (OBV, MFI, CMF)
- 综合评分

**文件**: `components/kline/composables/useSignalEvaluation.js`

**提交**: `refactor(kline): extract signal evaluation system`

### Step 6: 提取 useChipDistribution composable (P1)
- drawChipCanvas
- chipBins, chipCanvasRef, chipItems, chipMeta
- 筹码计算逻辑

**文件**: `components/kline/composables/useChipDistribution.js`

**提交**: `refactor(kline): extract chip distribution composable`

### Step 7: 提取 usePositionSim composable (P2)
- 多头仓位计算
- 入场/止损/止盈逻辑
- 成本计算

**文件**: `components/kline/composables/usePositionSim.js`

**提交**: `refactor(kline): extract position simulator composable`

### Step 8: 拆分子组件 - KlineToolbar (P1)
将 template 中的工具栏拆分：
- K线类型选择 (分时, 日K, 周K, 月K, 季K, 年K)
- 指标快捷切换按钮
- 指标设置弹窗触发

**文件**: `components/kline/components/KlineToolbar.vue`

**提交**: `refactor(kline): split KlineToolbar component`

### Step 9: 拆分子组件 - ChipDistribution + PositionSimPanel (P2)
- ChipDistribution.vue - 筹码分布面板
- PositionSimPanel.vue - 仓位模拟面板

**提交**: `refactor(kline): split chip and position subcomponents`

### Step 10: 根组件清理 (P0)
重构完成后，StockLightweightKlineChart.vue 将包含：
- 导入所有 composables 和子组件
- Props + Emits 定义
- 组合式 API 调用
- 简化的 template

**预期行数**: ~200 行 (减少 95%!)

## 风险控制

1. **每一步都运行 build**，确保没有破坏
2. **保持 props 接口不变**，父组件无需修改
3. **功能完全等效**，不改变原有行为
4. **可随时回滚**：通过 git 恢复原始组件

## 预期收益

| 指标 | 拆分前 | 拆分后 |
|------|--------|--------|
| 根组件行数 | 4832 | ~200 |
| 最大函数行数 | 862 (syncIndicators) | <100 |
| 可测试性 | ❌ 单体难以测试 | ✅ 每个 composable 可独立测试 |
| 可维护性 | ❌ 神对象 | ✅ 单一职责 + 模块化 |
| 代码复用 | ❌ 逻辑内联 | ✅ composables 可复用于其他图表 |
| Git 协作 | ❌ 频繁冲突 | ✅ 独立文件并行开发 |

## 依赖关系图

```
StockLightweightKlineChart.vue (根)
├── useChartLifecycle.js       (无依赖)
├── useIndicators.js           (依赖 calc.ts, constants.ts)
│   └── indicators/tips.ts, indicators/toggle.ts
├── useCrosshair.js            (依赖 format.ts, time.ts)
├── useSignalEvaluation.js     (依赖 useIndicators)
├── useChipDistribution.js     (依赖 chip.ts, calc.ts)
├── usePositionSim.js          (无依赖)
├── KlineToolbar.vue           (UI 组件)
├── ChipDistribution.vue       (UI 组件，依赖 useChipDistribution)
└── PositionSimPanel.vue       (UI 组件，依赖 usePositionSim)
```

## 优先级说明

- **P0**: 必须完成 - 核心逻辑拆分，直接影响可维护性
- **P1**: 应该完成 - 交互和信号系统
- **P2**: 可以完成 - 锦上添花的 UI 拆分
