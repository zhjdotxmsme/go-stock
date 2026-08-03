/**
 * 指标信号评估系统 Composable
 * 基于技术指标计算多空信号
 */

import { ref, computed } from 'vue'
import { extractOHLCV } from './useChartLifecycle'
import {
  smaValues,
  emaFinite,
  bollingerBands,
  vwapValues,
  demaValues,
  temaValues,
  kamaValues,
  hullMaValues,
  keltnerChannelValues,
  supertrendValues,
  rsiValues,
  macdBundle,
  kdjBundle,
  cciValues,
  williamsRValues,
  stochRsiValues,
  aroonValues,
  cmfValues,
  mfiValues,
  atrValues,
  adxValues,
} from '../calc'

/**
 * 获取数组最后一个有效值
 * @param {Array<number|null|undefined>} arr
 * @returns {number|null}
 */
export function last(arr) {
  for (let i = arr.length - 1; i >= 0; i--) {
    if (arr[i] != null && Number.isFinite(arr[i])) return arr[i]
  }
  return null
}

/**
 * 获取数组倒数第二个有效值
 * @param {Array<number|null|undefined>} arr
 * @returns {number|null}
 */
export function prev(arr) {
  let cnt = 0
  for (let i = arr.length - 1; i >= 0; i--) {
    if (arr[i] != null && Number.isFinite(arr[i])) {
      cnt++
      if (cnt === 2) return arr[i]
    }
  }
  return null
}

/**
 * 信号类型
 * @typedef {'bullish'|'bearish'|'neutral'|'oscillating'} SignalType
 */

/**
 * 单个指标信号
 * @typedef {Object} IndicatorSignal
 * @property {string} name - 指标名称
 * @property {SignalType} signal - 信号类型
 */

/**
 * 计算趋势指标信号
 * @param {Object} params
 * @param {Array<number>} params.closes - 收盘价
 * @param {Array<number>} params.highs - 最高价
 * @param {Array<number>} params.lows - 最低价
 * @param {Array<number>} params.vols - 成交量
 * @returns {Array<IndicatorSignal>}
 */
export function evaluateTrendSignals({ closes, highs, lows, vols }) {
  const signals = []
  const n = closes.length

  // MA - 移动平均线
  const m5 = smaValues(closes, 5)
  const m10 = smaValues(closes, 10)
  const m20 = smaValues(closes, 20)
  const m60 = smaValues(closes, 60)
  const v5 = last(m5), v10 = last(m10), v20 = last(m20), v60 = last(m60)
  if (v5 != null && v10 != null && v20 != null && v60 != null) {
    if (v5 > v10 && v10 > v20 && v20 > v60) signals.push({ name: 'MA', signal: 'bullish' })
    else if (v5 < v10 && v10 < v20 && v20 < v60) signals.push({ name: 'MA', signal: 'bearish' })
    else if ((v5 > v20 && v10 < v60) || (v5 < v20 && v10 > v60)) signals.push({ name: 'MA', signal: 'oscillating' })
    else signals.push({ name: 'MA', signal: 'neutral' })
  }

  // EMA - 指数移动平均
  const e12 = emaFinite(closes, 12)
  const e21 = emaFinite(closes, 21)
  const ve12 = last(e12), ve21 = last(e21)
  if (ve12 != null && ve21 != null) {
    if (ve12 > ve21) signals.push({ name: 'EMA', signal: 'bullish' })
    else if (ve12 < ve21) signals.push({ name: 'EMA', signal: 'bearish' })
    else signals.push({ name: 'EMA', signal: 'neutral' })
  }

  // BOLL - 布林带
  const { upper: bollU, mid: bollM, lower: bollL } = bollingerBands(closes, 20, 2)
  const bollVU = last(bollU), bollVM = last(bollM), bollVL = last(bollL), bollC = closes[n - 1]
  if (bollVU != null && bollVM != null && bollVL != null) {
    if (bollC > bollVU) signals.push({ name: 'BOLL', signal: 'bullish' })
    else if (bollC < bollVL) signals.push({ name: 'BOLL', signal: 'bearish' })
    else if (bollC > bollVM) signals.push({ name: 'BOLL', signal: 'oscillating' })
    else signals.push({ name: 'BOLL', signal: 'neutral' })
  }

  // VWAP - 成交量加权平均价
  const vw = vwapValues(highs, lows, closes, vols, 20)
  const vwV = last(vw), vwC = closes[n - 1]
  if (vwV != null) {
    if (vwC > vwV) signals.push({ name: 'VWAP', signal: 'bullish' })
    else if (vwC < vwV) signals.push({ name: 'VWAP', signal: 'bearish' })
    else signals.push({ name: 'VWAP', signal: 'neutral' })
  }

  // DEMA - 双指数移动平均
  const d = demaValues(closes, 21)
  const dV = last(d), dC = closes[n - 1]
  if (dV != null) {
    if (dC > dV) signals.push({ name: 'DEMA', signal: 'bullish' })
    else if (dC < dV) signals.push({ name: 'DEMA', signal: 'bearish' })
    else signals.push({ name: 'DEMA', signal: 'neutral' })
  }

  // TEMA - 三重指数移动平均
  const t = temaValues(closes, 21)
  const tV = last(t), tC = closes[n - 1]
  if (tV != null) {
    if (tC > tV) signals.push({ name: 'TEMA', signal: 'bullish' })
    else if (tC < tV) signals.push({ name: 'TEMA', signal: 'bearish' })
    else signals.push({ name: 'TEMA', signal: 'neutral' })
  }

  // KAMA - 考夫曼自适应移动平均
  const k = kamaValues(closes, 10, 2, 30)
  const kV = last(k), kC = closes[n - 1], kPv = prev(k)
  if (kV != null && kPv != null) {
    if (kC > kV && kV > kPv) signals.push({ name: 'KAMA', signal: 'bullish' })
    else if (kC < kV && kV < kPv) signals.push({ name: 'KAMA', signal: 'bearish' })
    else signals.push({ name: 'KAMA', signal: 'neutral' })
  }

  // HullMA
  const h = hullMaValues(closes, 9)
  const hV = last(h), hPv = prev(h)
  if (hV != null && hPv != null) {
    if (hV > hPv) signals.push({ name: 'HullMA', signal: 'bullish' })
    else if (hV < hPv) signals.push({ name: 'HullMA', signal: 'bearish' })
    else signals.push({ name: 'HullMA', signal: 'neutral' })
  }

  // Keltner - 肯特纳通道
  const { upper: kU, mid: kM, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
  const kVU = last(kU), kVL = last(kL), kC = closes[n - 1]
  if (kVU != null && kVL != null) {
    if (kC > kVU) signals.push({ name: 'Keltner', signal: 'bullish' })
    else if (kC < kVL) signals.push({ name: 'Keltner', signal: 'bearish' })
    else signals.push({ name: 'Keltner', signal: 'oscillating' })
  }

  // SuperTrend
  const { supertrend: stVal, direction: stDir } = supertrendValues(highs, lows, closes, 10, 3)
  const stD = last(stDir)
  if (stD != null) {
    if (stD === 1) signals.push({ name: 'SuperTrend', signal: 'bullish' })
    else if (stD === -1) signals.push({ name: 'SuperTrend', signal: 'bearish' })
    else signals.push({ name: 'SuperTrend', signal: 'neutral' })
  }

  return signals
}

/**
 * 计算震荡指标信号
 * @param {Object} params
 * @param {Array<number>} params.closes - 收盘价
 * @param {Array<number>} params.highs - 最高价
 * @param {Array<number>} params.lows - 最低价
 * @param {Array<number>} params.vols - 成交量
 * @returns {Array<IndicatorSignal>}
 */
export function evaluateOscillatorSignals({ closes, highs, lows, vols }) {
  const signals = []

  // RSI - 相对强弱指标
  const r = rsiValues(closes, 14)
  const rV = last(r)
  if (rV != null) {
    if (rV > 70) signals.push({ name: 'RSI', signal: 'bearish' })
    else if (rV < 30) signals.push({ name: 'RSI', signal: 'bullish' })
    else signals.push({ name: 'RSI', signal: 'neutral' })
  }

  // MACD
  const { dif: mDif, dea: mDea, hist: mHist } = macdBundle(closes)
  const mDifV = last(mDif), mDeaV = last(mDea), mHistV = last(mHist)
  if (mDifV != null && mDeaV != null && mHistV != null) {
    if (mDifV > mDeaV && mHistV > 0) signals.push({ name: 'MACD', signal: 'bullish' })
    else if (mDifV < mDeaV && mHistV < 0) signals.push({ name: 'MACD', signal: 'bearish' })
    else signals.push({ name: 'MACD', signal: 'neutral' })
  }

  // KDJ
  const { K: kdjK, D: kdjD, J: kdjJ } = kdjBundle(highs, lows, closes, 9)
  const kK = last(kdjK), kD = last(kdjD), kJ = last(kdjJ)
  if (kK != null && kD != null && kJ != null) {
    if (kK > 80 && kD > 80) signals.push({ name: 'KDJ', signal: 'bearish' })
    else if (kK < 20 && kD < 20) signals.push({ name: 'KDJ', signal: 'bullish' })
    else if (kK > kD) signals.push({ name: 'KDJ', signal: 'bullish' })
    else signals.push({ name: 'KDJ', signal: 'neutral' })
  }

  // CCI - 顺势指标
  const cc = cciValues(highs, lows, closes, 20)
  const ccV = last(cc)
  if (ccV != null) {
    if (ccV > 100) signals.push({ name: 'CCI', signal: 'bullish' })
    else if (ccV < -100) signals.push({ name: 'CCI', signal: 'bearish' })
    else signals.push({ name: 'CCI', signal: 'neutral' })
  }

  // Williams %R - 威廉指标
  const wr = williamsRValues(highs, lows, closes, 14)
  const wrV = last(wr)
  if (wrV != null) {
    if (wrV > -20) signals.push({ name: 'WilliamsR', signal: 'bearish' })
    else if (wrV < -80) signals.push({ name: 'WilliamsR', signal: 'bullish' })
    else signals.push({ name: 'WilliamsR', signal: 'neutral' })
  }

  // StochRSI - 随机相对强弱指标
  const { stochK: srK, stochD: srD } = stochRsiValues(closes, 14, 14, 3, 3)
  const srKV = last(srK), srDV = last(srD)
  if (srKV != null && srDV != null) {
    if (srKV > 0.8 && srDV > 0.8) signals.push({ name: 'StochRSI', signal: 'bearish' })
    else if (srKV < 0.2 && srDV < 0.2) signals.push({ name: 'StochRSI', signal: 'bullish' })
    else if (srKV > srDV) signals.push({ name: 'StochRSI', signal: 'bullish' })
    else signals.push({ name: 'StochRSI', signal: 'neutral' })
  }

  return signals
}

/**
 * 计算成交量指标信号
 * @param {Object} params
 * @param {Array<number>} params.closes - 收盘价
 * @param {Array<number>} params.highs - 最高价
 * @param {Array<number>} params.lows - 最低价
 * @param {Array<number>} params.vols - 成交量
 * @returns {Array<IndicatorSignal>}
 */
export function evaluateVolumeSignals({ closes, highs, lows, vols }) {
  const signals = []

  // CMF - 蔡金资金流量
  const cmf = cmfValues(highs, lows, closes, vols, 20)
  const cmfV = last(cmf)
  if (cmfV != null) {
    if (cmfV > 0.2) signals.push({ name: 'CMF', signal: 'bullish' })
    else if (cmfV < -0.2) signals.push({ name: 'CMF', signal: 'bearish' })
    else signals.push({ name: 'CMF', signal: 'neutral' })
  }

  // MFI - 资金流量指标
  const m = mfiValues(highs, lows, closes, vols, 14)
  const mV = last(m)
  if (mV != null) {
    if (mV > 80) signals.push({ name: 'MFI', signal: 'bearish' })
    else if (mV < 20) signals.push({ name: 'MFI', signal: 'bullish' })
    else signals.push({ name: 'MFI', signal: 'neutral' })
  }

  return signals
}

/**
 * 计算趋势强度信号
 * @param {Object} params
 * @param {Array<number>} params.closes - 收盘价
 * @param {Array<number>} params.highs - 最高价
 * @param {Array<number>} params.lows - 最低价
 * @returns {Array<IndicatorSignal>}
 */
export function evaluateStrengthSignals({ closes, highs, lows }) {
  const signals = []

  // Aroon - 阿隆指标
  const { up: aUp, down: aDown } = aroonValues(highs, lows, 25)
  const aUpV = last(aUp), aDownV = last(aDown)
  if (aUpV != null && aDownV != null) {
    if (aUpV > 70 && aDownV < 30) signals.push({ name: 'Aroon', signal: 'bullish' })
    else if (aDownV > 70 && aUpV < 30) signals.push({ name: 'Aroon', signal: 'bearish' })
    else signals.push({ name: 'Aroon', signal: 'neutral' })
  }

  // ATR - 平均真实波幅（仅作为强度参考）
  const at = atrValues(highs, lows, closes, 14)
  const atV = last(at), atPv = prev(at)
  if (atV != null && atPv != null) {
    if (atV > atPv * 1.5) signals.push({ name: 'ATR', signal: 'bullish' }) // 波动率放大
    else signals.push({ name: 'ATR', signal: 'neutral' })
  }

  // ADX - 平均趋势指数
  const { adx: ad, diPlus: dp, diMinus: dm } = adxValues(highs, lows, closes, 14)
  const adV = last(ad), dpV = last(dp), dmV = last(dm)
  if (adV != null && dpV != null && dmV != null) {
    if (adV > 25 && dpV > dmV) signals.push({ name: 'ADX', signal: 'bullish' })
    else if (adV > 25 && dpV < dmV) signals.push({ name: 'ADX', signal: 'bearish' })
    else signals.push({ name: 'ADX', signal: 'neutral' })
  }

  return signals
}

/**
 * 信号评估系统
 * @param {Object} options
 * @param {Ref<Array>} options.mergedRawRows - K线数据
 * @param {Function} options.sortKeyFn - 排序键函数
 * @param {Function} options.toChartTimeFn - 图表时间转换函数
 * @param {Function} options.parseNumStrFn - 数字解析函数
 * @returns {Object}
 */
export function useSignalEvaluation({ mergedRawRows, sortKeyFn, toChartTimeFn, parseNumStrFn }) {
  const showSignalPanel = ref(false)

  /**
   * 评估所有指标信号
   * @param {number|null} endIdx - 截止索引（用于历史信号评估）
   * @returns {Array<IndicatorSignal>}
   */
  function evaluateIndicatorSignals(endIdx = null) {
    const rows = mergedRawRows.value
    if (!rows || rows.length < 2) return []

    // 截取到 endIdx（含），使指标计算基于该 K 线位置的数据
    const sliced = endIdx != null && endIdx >= 0 && endIdx < rows.length - 1
      ? rows.slice(0, endIdx + 1)
      : rows

    const { times, opens, closes, highs, lows, vols } = extractOHLCV(
      sliced,
      sortKeyFn,
      toChartTimeFn,
      parseNumStrFn
    )
    const n = times.length
    if (n < 2) return []

    return [
      ...evaluateTrendSignals({ closes, highs, lows, vols }),
      ...evaluateOscillatorSignals({ closes, highs, lows, vols }),
      ...evaluateVolumeSignals({ closes, highs, lows, vols }),
      ...evaluateStrengthSignals({ closes, highs, lows }),
    ]
  }

  /**
   * 聚合所有信号为综合评分
   * @param {Array<IndicatorSignal>} signals - 信号列表
   * @returns {Object} { bullish: number, bearish: number, neutral: number, total: number, score: number }
   */
  function aggregateSignals(signals) {
    const bullish = signals.filter(s => s.signal === 'bullish').length
    const bearish = signals.filter(s => s.signal === 'bearish').length
    const neutral = signals.filter(s => s.signal === 'neutral' || s.signal === 'oscillating').length
    const total = signals.length || 1
    const score = ((bullish - bearish) / total) * 100 // -100 ~ +100

    return {
      bullish,
      bearish,
      neutral,
      total,
      score,
      verdict: score > 20 ? 'bullish' : score < -20 ? 'bearish' : 'neutral',
    }
  }

  // 最新综合信号
  const latestSignals = computed(() => evaluateIndicatorSignals())
  const signalSummary = computed(() => aggregateSignals(latestSignals.value))

  return {
    // 状态
    showSignalPanel,

    // 计算属性
    latestSignals,
    signalSummary,

    // 方法
    evaluateIndicatorSignals,
    evaluateTrendSignals,
    evaluateOscillatorSignals,
    evaluateVolumeSignals,
    evaluateStrengthSignals,
    aggregateSignals,
  }
}

export default useSignalEvaluation
