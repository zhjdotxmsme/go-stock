/**
 * K线图生命周期管理 Composable
 * 管理图表初始化、销毁、子窗格清理
 */

import { ref } from 'vue'
import { LineSeries, HistogramSeries } from 'lightweight-charts'

/**
 * 安全移除系列
 * @param {*} api - 图表系列 API
 * @param {*} chart - 图表实例
 * @returns {null}
 */
export function removeSeriesSafe(api, chart) {
  if (!api || !chart) return null
  try {
    chart.removeSeries(api)
  } catch {
    /* ignore */
  }
  return null
}

/**
 * 提取 OHLCV 数据
 * @param {Array} rows - K线数据行
 * @param {Function} sortKeyFn - 排序键函数
 * @param {Function} toChartTimeFn - 转换为图表时间函数
 * @param {Function} parseNumStrFn - 解析数字字符串函数
 * @returns {Object} { times, opens, closes, highs, lows, vols, amplitudes }
 */
export function extractOHLCV(rows, sortKeyFn, toChartTimeFn, parseNumStrFn) {
  const sorted = [...(rows || [])].sort((a, b) => sortKeyFn(a.day) - sortKeyFn(b.day))
  const times = []
  const opens = []
  const closes = []
  const highs = []
  const lows = []
  const vols = []
  const amplitudes = []
  for (const r of sorted) {
    const t = toChartTimeFn(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    times.push(t)
    opens.push(o)
    closes.push(c)
    highs.push(h)
    lows.push(l)
    vols.push(Number.isFinite(v) ? v : 0)
    const rawAmp = parseNumStrFn(r.amplitude)
    amplitudes.push(Number.isFinite(rawAmp) ? rawAmp : (o > 0 ? (h - l) / o * 100 : NaN))
  }
  return { times, opens, closes, highs, lows, vols, amplitudes }
}

/**
 * 计算平均振幅
 * @param {Array<number>} amplitudes - 振幅数组
 * @param {number} period - 周期
 * @returns {number}
 */
export function avgAmplitude(amplitudes, period) {
  if (!amplitudes || amplitudes.length < period) return NaN
  let s = 0, cnt = 0
  for (let i = amplitudes.length - period; i < amplitudes.length; i++) {
    const v = amplitudes[i]
    if (Number.isFinite(v)) { s += v; cnt++ }
  }
  return cnt === period ? s / cnt : NaN
}

/**
 * 格式化量比
 * @param {number|string} v
 * @returns {string}
 */
export function formatVolumeRatio(v) {
  if (v == null || v === '' || v === '--') return '--'
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(2) : '--'
}

/**
 * 转换为线图数据格式
 * @param {Array<number>} times - 时间数组
 * @param {Array<number>} values - 值数组
 * @returns {Array<Object>}
 */
export function toLineData(times, values) {
  const arr = []
  for (let i = 0; i < times.length; i++) {
    const v = values[i]
    if (v != null && Number.isFinite(v)) arr.push({ time: times[i], value: v })
  }
  return arr
}

/**
 * 图表生命周期管理
 * @returns {Object}
 */
export function useChartLifecycle() {
  const chart = ref(null)
  const loading = ref(false)
  const loadingHistory = ref(false)
  const errorText = ref('')
  const activeDataSource = ref('')
  const mergedRawRowsVersion = ref(0)
  const hasMoreOlder = ref(true)

  // 指标系列引用存储
  const indicatorSeries = ref({
    // 移动平均
    ma5: null,
    ma10: null,
    ma20: null,
    ma30: null,
    ma60: null,
    ma120: null,
    ma250: null,
    ema5: null,
    ema10: null,
    ema20: null,
    ema30: null,
    ema60: null,
    ema120: null,
    ema250: null,

    // 布林带
    bollTop: null,
    bollMid: null,
    bollBottom: null,

    // 通道指标
    kama: null,
    keltnerMid: null,
    keltnerTop: null,
    keltnerBottom: null,
    donchianTop: null,
    donchianBottom: null,

    // 趋势指标
    supertrendUpper: null,
    supertrendLower: null,
    supertrendTrend: null,
    ichimokuBase: null,
    ichimokuConv: null,
    ichimokuSpanA: null,
    ichimokuSpanB: null,
    sar: null,

    // 成交量相关
    vwap: null,
    vwapBandsUpper: null,
    vwapBandsLower: null,
    obv: null,
    cmf: null,

    // MACD
    macdHist: null,
    macdDif: null,
    macdDea: null,

    // KDJ
    kdjK: null,
    kdjD: null,
    kdjJ: null,

    // 震荡指标
    rsi: null,
    atr: null,
    mfi: null,
    cci: null,
    williamsR: null,
    stochRsi: null,
    stochRsiD: null,
    aroonUp: null,
    aroonDown: null,
    cmo: null,
    forceIndex: null,

    // TTM 挤压
    ttmHist: null,
    ttmDots: null,

    // ADX
    adx: null,
    adxDiP: null,
    adxDiM: null,

    // 振幅
    avgAmp5: null,
    avgAmp10: null,
    avgAmp20: null,

    // AO (Awesome Oscillator)
    aoHist: null,
    aoLine: null,

    // 累积派发线
    adLine: null,

    // TRIX
    trixLine: null,
    trixSignal: null,

    // ROC
    rocLine: null,

    // CHOP (Choppiness Index)
    chopLine: null,

    // Elder-Ray
    elderBull: null,
    elderBear: null,

    // Chaikin Oscillator
    chaikinOscLine: null,

    // Mass Index
    massIndexLine: null,

    // Ulcer Index
    ulcerLine: null,

    // Coppock Curve
    coppockLine: null,

    // SMI (Stochastic Momentum Index)
    smiLine: null,
    smiSignal: null,

    // Signal Ratio
    signalRatioBullish: null,
    signalRatioBearish: null,
    signalRatioNet: null,

    // ZigZag
    zzDots: null,

    // SATS
    satsSpots: null,

    // Alligator
    alligatorJaw: null,
    alligatorTeeth: null,
    alligatorLips: null,

    // Fractal
    fractalUp: null,
    fractalDn: null,

    // Pivot Points
    pivotR3: null,
    pivotR2: null,
    pivotR1: null,
    pivotP: null,
    pivotS1: null,
    pivotS2: null,
    pivotS3: null,

    // DEMA
    dema: null,

    // TEMA
    tema: null,

    // Hull MA
    hull: null,

    // SMC 供需区
    smcObTop: null,
    smcObBot: null,
  })

  // 子线图通用配置
  const subLineOpts = {
    lineWidth: 1,
    lastValueVisible: false,
    priceLineVisible: false,
  }

  /**
   * 销毁所有子窗格和指标
   */
  function tearDownAllSubPanes() {
    if (!chart.value) return
    const ind = indicatorSeries.value

    // 安全移除所有指标系列
    ind.obv = removeSeriesSafe(ind.obv, chart.value)
    ind.macdHist = removeSeriesSafe(ind.macdHist, chart.value)
    ind.macdDif = removeSeriesSafe(ind.macdDif, chart.value)
    ind.macdDea = removeSeriesSafe(ind.macdDea, chart.value)
    ind.kdjK = removeSeriesSafe(ind.kdjK, chart.value)
    ind.kdjD = removeSeriesSafe(ind.kdjD, chart.value)
    ind.kdjJ = removeSeriesSafe(ind.kdjJ, chart.value)
    ind.rsi = removeSeriesSafe(ind.rsi, chart.value)
    ind.atr = removeSeriesSafe(ind.atr, chart.value)
    ind.mfi = removeSeriesSafe(ind.mfi, chart.value)
    ind.cci = removeSeriesSafe(ind.cci, chart.value)
    ind.ttmHist = removeSeriesSafe(ind.ttmHist, chart.value)
    ind.ttmDots = removeSeriesSafe(ind.ttmDots, chart.value)
    ind.adx = removeSeriesSafe(ind.adx, chart.value)
    ind.adxDiP = removeSeriesSafe(ind.adxDiP, chart.value)
    ind.adxDiM = removeSeriesSafe(ind.adxDiM, chart.value)
    ind.williamsR = removeSeriesSafe(ind.williamsR, chart.value)
    ind.stochRsi = removeSeriesSafe(ind.stochRsi, chart.value)
    ind.stochRsiD = removeSeriesSafe(ind.stochRsiD, chart.value)
    ind.cmf = removeSeriesSafe(ind.cmf, chart.value)
    ind.aroonUp = removeSeriesSafe(ind.aroonUp, chart.value)
    ind.aroonDown = removeSeriesSafe(ind.aroonDown, chart.value)
    ind.cmo = removeSeriesSafe(ind.cmo, chart.value)
    ind.forceIndex = removeSeriesSafe(ind.forceIndex, chart.value)
    ind.avgAmp5 = removeSeriesSafe(ind.avgAmp5, chart.value)
    ind.avgAmp10 = removeSeriesSafe(ind.avgAmp10, chart.value)
    ind.avgAmp20 = removeSeriesSafe(ind.avgAmp20, chart.value)
    ind.aoHist = removeSeriesSafe(ind.aoHist, chart.value)
    ind.aoLine = removeSeriesSafe(ind.aoLine, chart.value)
    ind.adLine = removeSeriesSafe(ind.adLine, chart.value)
    ind.trixLine = removeSeriesSafe(ind.trixLine, chart.value)
    ind.trixSignal = removeSeriesSafe(ind.trixSignal, chart.value)
    ind.rocLine = removeSeriesSafe(ind.rocLine, chart.value)
    ind.chopLine = removeSeriesSafe(ind.chopLine, chart.value)
    ind.elderBull = removeSeriesSafe(ind.elderBull, chart.value)
    ind.elderBear = removeSeriesSafe(ind.elderBear, chart.value)
    ind.chaikinOscLine = removeSeriesSafe(ind.chaikinOscLine, chart.value)
    ind.massIndexLine = removeSeriesSafe(ind.massIndexLine, chart.value)
    ind.ulcerLine = removeSeriesSafe(ind.ulcerLine, chart.value)
    ind.coppockLine = removeSeriesSafe(ind.coppockLine, chart.value)
    ind.smiLine = removeSeriesSafe(ind.smiLine, chart.value)
    ind.smiSignal = removeSeriesSafe(ind.smiSignal, chart.value)
    ind.signalRatioBullish = removeSeriesSafe(ind.signalRatioBullish, chart.value)
    ind.signalRatioBearish = removeSeriesSafe(ind.signalRatioBearish, chart.value)
    ind.signalRatioNet = removeSeriesSafe(ind.signalRatioNet, chart.value)

    // 移除主图指标系列
    // MA
    ind.ma5 = removeSeriesSafe(ind.ma5, chart.value)
    ind.ma10 = removeSeriesSafe(ind.ma10, chart.value)
    ind.ma20 = removeSeriesSafe(ind.ma20, chart.value)
    ind.ma30 = removeSeriesSafe(ind.ma30, chart.value)
    ind.ma60 = removeSeriesSafe(ind.ma60, chart.value)
    ind.ma120 = removeSeriesSafe(ind.ma120, chart.value)
    ind.ma250 = removeSeriesSafe(ind.ma250, chart.value)
    // EMA
    ind.ema5 = removeSeriesSafe(ind.ema5, chart.value)
    ind.ema10 = removeSeriesSafe(ind.ema10, chart.value)
    ind.ema20 = removeSeriesSafe(ind.ema20, chart.value)
    ind.ema30 = removeSeriesSafe(ind.ema30, chart.value)
    ind.ema60 = removeSeriesSafe(ind.ema60, chart.value)
    ind.ema120 = removeSeriesSafe(ind.ema120, chart.value)
    ind.ema250 = removeSeriesSafe(ind.ema250, chart.value)
    // BOLL
    ind.bollTop = removeSeriesSafe(ind.bollTop, chart.value)
    ind.bollMid = removeSeriesSafe(ind.bollMid, chart.value)
    ind.bollBottom = removeSeriesSafe(ind.bollBottom, chart.value)
    // KAMA
    ind.kama = removeSeriesSafe(ind.kama, chart.value)
    // Keltner
    ind.keltnerMid = removeSeriesSafe(ind.keltnerMid, chart.value)
    ind.keltnerTop = removeSeriesSafe(ind.keltnerTop, chart.value)
    ind.keltnerBottom = removeSeriesSafe(ind.keltnerBottom, chart.value)
    // Donchian
    ind.donchianTop = removeSeriesSafe(ind.donchianTop, chart.value)
    ind.donchianBottom = removeSeriesSafe(ind.donchianBottom, chart.value)
    // Supertrend
    ind.supertrendUpper = removeSeriesSafe(ind.supertrendUpper, chart.value)
    ind.supertrendLower = removeSeriesSafe(ind.supertrendLower, chart.value)
    ind.supertrendTrend = removeSeriesSafe(ind.supertrendTrend, chart.value)
    // Ichimoku
    ind.ichimokuBase = removeSeriesSafe(ind.ichimokuBase, chart.value)
    ind.ichimokuConv = removeSeriesSafe(ind.ichimokuConv, chart.value)
    ind.ichimokuSpanA = removeSeriesSafe(ind.ichimokuSpanA, chart.value)
    ind.ichimokuSpanB = removeSeriesSafe(ind.ichimokuSpanB, chart.value)
    // SAR
    ind.sar = removeSeriesSafe(ind.sar, chart.value)
    // VWAP
    ind.vwap = removeSeriesSafe(ind.vwap, chart.value)
    ind.vwapBandsUpper = removeSeriesSafe(ind.vwapBandsUpper, chart.value)
    ind.vwapBandsLower = removeSeriesSafe(ind.vwapBandsLower, chart.value)
    // ZigZag
    ind.zzDots = removeSeriesSafe(ind.zzDots, chart.value)
    // SATS
    ind.satsSpots = removeSeriesSafe(ind.satsSpots, chart.value)
    // Alligator
    ind.alligatorJaw = removeSeriesSafe(ind.alligatorJaw, chart.value)
    ind.alligatorTeeth = removeSeriesSafe(ind.alligatorTeeth, chart.value)
    ind.alligatorLips = removeSeriesSafe(ind.alligatorLips, chart.value)
    // Fractal
    ind.fractalUp = removeSeriesSafe(ind.fractalUp, chart.value)
    ind.fractalDn = removeSeriesSafe(ind.fractalDn, chart.value)
    // Pivot
    ind.pivotR3 = removeSeriesSafe(ind.pivotR3, chart.value)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2, chart.value)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1, chart.value)
    ind.pivotP = removeSeriesSafe(ind.pivotP, chart.value)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1, chart.value)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2, chart.value)
    ind.pivotS3 = removeSeriesSafe(ind.pivotS3, chart.value)
    // DEMA
    ind.dema = removeSeriesSafe(ind.dema, chart.value)
    // TEMA
    ind.tema = removeSeriesSafe(ind.tema, chart.value)
    // Hull MA
    ind.hull = removeSeriesSafe(ind.hull, chart.value)
    // SMC
    ind.smcObTop = removeSeriesSafe(ind.smcObTop, chart.value)
    ind.smcObBot = removeSeriesSafe(ind.smcObBot, chart.value)

    // 移除多余窗格（保留主窗格）
    while (chart.value.panes().length > 1) {
      chart.value.removePane(chart.value.panes().length - 1)
    }
    chart.value.panes()[0]?.setStretchFactor(1)
  }

  /**
   * 销毁图表
   */
  function destroyChart() {
    if (chart.value) {
      tearDownAllSubPanes()
      chart.value.remove()
      chart.value = null
    }
  }

  return {
    // 状态
    chart,
    loading,
    loadingHistory,
    errorText,
    activeDataSource,
    mergedRawRowsVersion,
    hasMoreOlder,
    indicatorSeries,

    // 常量
    subLineOpts,

    // 工具函数
    removeSeriesSafe,
    extractOHLCV,
    avgAmplitude,
    formatVolumeRatio,
    toLineData,

    // 方法
    tearDownAllSubPanes,
    destroyChart,
  }
}

export default useChartLifecycle
