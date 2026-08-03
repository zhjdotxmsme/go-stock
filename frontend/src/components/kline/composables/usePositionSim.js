/**
 * 仓位模拟器 Composable
 * 处理多头/空头仓位计算、入场止损止盈管理
 */

import { ref, computed } from 'vue'

// 常量
const LONG_PRICE_LINE_HIT_PX = 12
const LONG_FOCUS_BLUR_CLEAR_MS = 600

/**
 * 多头仓位管理
 * @param {Object} options
 * @param {Ref<number>} options.lastClose - 最新收盘价
 * @returns {Object}
 */
export function useLongPosition({ lastClose }) {
  // 显示多头仓位
  const showLongPosition = ref(false)

  // 价格字段
  const longEntryStr = ref('')
  const longStopStr = ref('')
  const longTakeProfitStr = ref('')
  const longCostStr = ref('')

  // 点击选取启用
  const longClickPickEnabled = ref(false)
  // 下一个要填充的字段
  const longClickNextField = ref('entry')
  // 当前聚焦的价格字段
  const longFocusedPriceField = ref(null)
  // 抑制价格变化时的 emit
  const suppressLongPriceEmit = ref(false)

  // 数值化的价格
  const longEntry = computed(() => Number(longEntryStr.value) || 0)
  const longStop = computed(() => Number(longStopStr.value) || 0)
  const longTakeProfit = computed(() => Number(longTakeProfitStr.value) || 0)
  const longCost = computed(() => Number(longCostStr.value) || 0)

  // 盈亏计算
  const longPnL = computed(() => {
    const entry = longEntry.value
    if (!entry || !lastClose.value) return 0
    return lastClose.value - entry
  })

  const longPnLPct = computed(() => {
    const entry = longEntry.value
    if (!entry) return 0
    return (longPnL.value / entry) * 100
  })

  // 风险回报比
  const riskRewardRatio = computed(() => {
    const entry = longEntry.value
    const stop = longStop.value
    const tp = longTakeProfit.value
    if (!entry || !stop || !tp || stop >= entry || tp <= entry) return 0
    const risk = entry - stop
    const reward = tp - entry
    return reward / risk
  })

  // 1R 盈亏
  const long1R = computed(() => {
    const entry = longEntry.value
    const stop = longStop.value
    if (!entry || !stop || stop >= entry) return 0
    return entry - stop
  })

  /**
   * 设置价格字段（点击K线图时调用）
   * @param {number} price - 价格
   */
  function setLongPriceField(price) {
    if (!longClickPickEnabled.value) return
    const p = price.toFixed(2)
    switch (longClickNextField.value) {
      case 'entry':
        longEntryStr.value = p
        longClickNextField.value = 'stop'
        break
      case 'stop':
        longStopStr.value = p
        longClickNextField.value = 'takeProfit'
        break
      case 'takeProfit':
        longTakeProfitStr.value = p
        longClickNextField.value = 'cost'
        break
      case 'cost':
        longCostStr.value = p
        longClickNextField.value = 'entry'
        break
    }
  }

  /**
   * 重置多头仓位
   */
  function resetLongPosition() {
    longEntryStr.value = ''
    longStopStr.value = ''
    longTakeProfitStr.value = ''
    longCostStr.value = ''
    longClickNextField.value = 'entry'
    longFocusedPriceField.value = null
  }

  /**
   * 基于当前价格快速设置
   */
  function quickSetFromCurrent() {
    if (!lastClose.value) return
    const cur = lastClose.value
    longEntryStr.value = cur.toFixed(2)
    longStopStr.value = (cur * 0.95).toFixed(2) // 5% 止损
    longTakeProfitStr.value = (cur * 1.10).toFixed(2) // 10% 止盈
    longCostStr.value = cur.toFixed(2)
  }

  return {
    // 状态
    showLongPosition,
    longEntryStr,
    longStopStr,
    longTakeProfitStr,
    longCostStr,
    longClickPickEnabled,
    longClickNextField,
    longFocusedPriceField,
    suppressLongPriceEmit,

    // 计算属性
    longEntry,
    longStop,
    longTakeProfit,
    longCost,
    longPnL,
    longPnLPct,
    riskRewardRatio,
    long1R,

    // 方法
    setLongPriceField,
    resetLongPosition,
    quickSetFromCurrent,

    // 常量
    LONG_PRICE_LINE_HIT_PX,
    LONG_FOCUS_BLUR_CLEAR_MS,
  }
}

export default useLongPosition
