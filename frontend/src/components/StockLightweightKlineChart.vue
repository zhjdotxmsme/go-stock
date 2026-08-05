<script setup>
import * as stockApi from '../api/stock'
import {
  CandlestickSeries,
  createChart,
  HistogramSeries,
} from 'lightweight-charts'
import { NButton, NFlex, NInput, NSpin, NText, NTooltip } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  eastMoneyDayToUnixSeconds, eastMoneyKlineFieldToUnixSeconds,
  formatTickTime, mergeKlineRows, mergeRefreshWithLatest,
  extractYmdDatePart, barSecondsForMinuteKlt,
} from './kline/time'

import {
  DAILY_LIKE_KLT,
  HISTORY_PAGE_SIZE, BARS_BEFORE_LOAD_MORE, DEFAULT_VISIBLE_BARS,
  DEFAULT_RIGHT_LOGICAL_GAP, SHOW_CHIP_TOOLBAR_BUTTON, INTERVALS,
} from './kline/constants'
import { createIndicatorSync } from './kline/composables/useIndicatorSync'
import { useLongPosition } from './kline/composables/useLongPosition'
import { useChipOverlay } from './kline/composables/useChipOverlay'
import { createIndicatorToggles } from './kline/composables/useIndicatorToggles'
import { createCrosshairPanel } from './kline/composables/useCrosshairPanel'
import { toSeriesData } from './kline/series'

const props = defineProps({
  code: { type: String, default: '' },
  stockName: { type: String, default: '' },
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: Number, default: 400 },
  /** 定时拉取当前周期最新 K 线，毫秒；0 关闭；默认 60 秒 */
  realtimeIntervalMs: { type: Number, default: 1000*60 },
  /** 多单开仓价；传入则与内部输入同步，未传入（undefined）时不向父组件 emit */
  longEntryPrice: { type: [String, Number], default: undefined },
  /** 多单止损价 */
  longStopLossPrice: { type: [String, Number], default: undefined },
  /** 多单止盈价 */
  longTakeProfitPrice: { type: [String, Number], default: undefined },
  /** 成本价 */
  costPrice: { type: [String, Number], default: undefined },
})

const emit = defineEmits([
  'update:longEntryPrice',
  'update:longStopLossPrice',
  'update:longTakeProfitPrice',
  'update:costPrice',
])
const chartContainerRef = ref(null)
/** 十字线当前 K 对应的原始行（东财字段） */
const hoverRawRow = ref(null)
/** 无十字线时展示：当前数据中时间最新的一根 K 线 */
const defaultLatestRawRow = ref(null)
const activeKlt = ref('101')
const showMA = ref(false)
const showBOLL = ref(false)
const showOBV = ref(false)
const showMACD = ref(false)
const showKDJ = ref(false)
const showRSI = ref(false)
const showATR = ref(false)
const showVWAP = ref(false)
const showMFI = ref(false)
const showKAMA = ref(false)
const showKeltner = ref(false)
const showSupertrend = ref(false)
const showEMA = ref(false)
const showIchimoku = ref(false)
const showCCI = ref(false)
const showTTMSqueeze = ref(false)
const showSAR = ref(false)
const showDonchian = ref(false)
const showADX = ref(false)
const showWilliamsR = ref(false)
const showStochRSI = ref(false)
const showCMF = ref(false)
const showAroon = ref(false)
const showCMO = ref(false)
const showForceIndex = ref(false)
const showPivot = ref(false)
const showDEMA = ref(false)
const showZigZag = ref(false)
const showSATS = ref(false)
const showAvgAmp = ref(false)
const showAlligator = ref(false)
const showAO = ref(false)
const showHullMA = ref(false)
const showAD = ref(false)
const showTRIX = ref(false)
const showROC = ref(false)
const showFractal = ref(false)
const showCHOP = ref(false)
const showElderRay = ref(false)
const showChaikinOsc = ref(false)
const showVWAPBands = ref(false)
const showMassIndex = ref(false)
const showUlcerIndex = ref(false)
const showCoppock = ref(false)
const showTEMA = ref(false)
const showSMI = ref(false)
const showSignalRatio = ref(false)
const showSMC = ref(false)
const loading = ref(false)
const loadingHistory = ref(false)
const errorText = ref('')
const activeDataSource = ref('')

let chart = null
let candleSeries = null
let volSeries = null
let pollTimer = null
/** 已合并的后端原始 K 线（按时间升序） */
let mergedRawRows = []
/** 每次 mergedRawRows 变更后递增，供 computed 感知变化 */
const mergedRawRowsVersion = ref(0)
const hasMoreOlder = ref(true)
let loadOlderDebounceTimer = null
/** Wails/WebView 下可见区回调偶发不触发，用轻量轮询兜底 */
let historyVisiblePollTimer = null
let logicalRangeHandler = null
let visibleTimeRangeHandler = null
let crosshairMoveHandler = null
let chartClickHandler = null
/** 上一次请求更早 K 线使用的 end，用于识别「重叠返回」避免误判无更多数据 */
let lastOlderHistoryEndTried = ''
/** >0 时表示由代码在改时间轴（fitContent / setData / setVisibleLogicalRange），不触发分页加载 */
let programmaticRangeDepth = 0

// ---- 指标同步（搬迁自原组件的巨型函数，见 kline/composables/useIndicatorSync） ----
const { syncIndicators, evaluateIndicatorSignals, resetIndicatorHandles } = createIndicatorSync({
  getChart: () => chart,
  getCandleSeries: () => candleSeries,
  getMergedRawRows: () => mergedRawRows,
  mergedRawRowsVersion,
  hoverRawRow,
  showMA, showBOLL, showOBV, showMACD, showKDJ, showRSI, showATR, showVWAP, showMFI, showKAMA, showKeltner, showSupertrend, showEMA, showIchimoku, showCCI, showTTMSqueeze, showSAR, showDonchian, showADX, showWilliamsR, showStochRSI, showCMF, showAroon, showCMO, showForceIndex, showPivot, showDEMA, showZigZag, showSATS, showAvgAmp, showAlligator, showAO, showHullMA, showAD, showTRIX, showROC, showFractal, showCHOP, showElderRay, showChaikinOsc, showVWAPBands, showMassIndex, showUlcerIndex, showCoppock, showTEMA, showSMI, showSignalRatio, showSMC,
})

// ---- 多单价位（见 kline/composables/useLongPosition） ----
const {
  showLongPosition, longEntryStr, longStopStr, longTakeProfitStr, longCostStr,
  longClickPickEnabled, longClickNextField, longFocusedPriceField,
  longPositionStats, longPositionHint, longClickNextLabel, longFocusChartHint,
  toggleLongPosition, fillLongEntryFromLatestClose, onLongPriceInputFocus, onLongPriceInputBlur,
  toggleLongClickPick, resetLongClickSequence,
  syncLongPositionPriceLines, clearLongPositionPriceLines,
  refreshLongPriceLineCursorFromCrosshair, clearLongPriceLinePaneCursor,
  attachLongPriceLineDragListeners, detachLongPriceLinePaneListener, detachLongDragWindowListeners,
  cancelLongFocusBlurTimer, applyLongPriceFromChartByField, applyLongClickPrice,
  pricePropToInputStr, resetLongDragState, isLongSuppressChartClick,
} = useLongPosition({
  props,
  emit,
  getChart: () => chart,
  getCandleSeries: () => candleSeries,
  chartContainerRef,
  defaultLatestRawRow,
})

// ---- 筹码分布（见 kline/composables/useChipOverlay） ----
const {
  showChip, chipBins, chipCanvasRef, chipItems, chipMeta,
  toggleChip, updateChipFromHover, drawChipCanvas,
} = useChipOverlay({
  props,
  getMergedRawRows: () => mergedRawRows,
  hoverRawRow,
  defaultLatestRawRow,
})


import { indicatorTips } from './kline/indicators/tips'
import KlineIndicatorSidebar from './kline/KlineIndicatorSidebar.vue'
// ---- 十字线信息面板（见 kline/composables/useCrosshairPanel） ----
const { formatCrosshairTime, findRawRowByChartTime, syncDefaultLatestPanelRow, crosshairPanel } = createCrosshairPanel({
  props,
  activeKlt,
  hoverRawRow,
  defaultLatestRawRow,
  getMergedRawRows: () => mergedRawRows,
})


const indicatorSignalSummary = computed(() => {
  // 依赖 mergedRawRowsVersion 以感知 mergedRawRows 变更
  void mergedRawRowsVersion.value
  // 根据 hoverRawRow 计算当前光标对应的 K 线索引
  const hoveredRow = hoverRawRow.value
  let endIdx = null
  if (hoveredRow) {
    const curDay = String(hoveredRow.day || '').replace(/\//g, '-')
    const idx = mergedRawRows.findIndex(x => String(x.day || '').replace(/\//g, '-') === curDay)
    if (idx >= 0) endIdx = idx
  }
  const sigs = evaluateIndicatorSignals(endIdx)
  if (!sigs.length) return null
  const counts = { bullish: 0, bearish: 0, neutral: 0, oscillating: 0 }
  for (const s of sigs) {
    if (counts[s.signal] !== undefined) counts[s.signal]++
  }
  const total = sigs.length
  return {
    total,
    bullish: counts.bullish,
    bearish: counts.bearish,
    neutral: counts.neutral,
    oscillating: counts.oscillating,
    bullishPct: total > 0 ? Math.round((counts.bullish / total) * 100) : 0,
    bearishPct: total > 0 ? Math.round((counts.bearish / total) * 100) : 0,
    neutralPct: total > 0 ? Math.round((counts.neutral / total) * 100) : 0,
    oscillatingPct: total > 0 ? Math.round((counts.oscillating / total) * 100) : 0,
    signals: sigs,
  }
})


function chartThemeOptions(isDark) {
  const minuteLike = !DAILY_LIKE_KLT.has(activeKlt.value)
  return {
    layout: {
      background: { type: 'solid', color: isDark ? '#141414' : '#ffffff' },
      textColor: isDark ? '#cbd5e1' : '#334155',
    },
    grid: {
      vertLines: { color: isDark ? '#27272a' : '#f1f5f9' },
      horzLines: { color: isDark ? '#27272a' : '#f1f5f9' },
    },
    crosshair: { mode: 1 },
    rightPriceScale: { borderColor: isDark ? '#3f3f46' : '#e2e8f0' },
    localization: {
      locale: 'zh-CN',
      dateFormat: 'yyyy-MM-dd',
      timeFormatter: (t) => formatCrosshairTime(t),
    },
    timeScale: {
      borderColor: isDark ? '#3f3f46' : '#e2e8f0',
      timeVisible: minuteLike,
      secondsVisible: false,
      tickMarkFormatter: (t, tickMarkType) => formatTickTime(t, tickMarkType),
    },
  }
}

/** 根据当前最旧一根 K 线的 day 字段生成东财 end 参数 */
function formatEastMoneyEndFromOldest(oldestDayField, klt) {
  const s = String(oldestDayField || '').trim()
  const dailyOnly = DAILY_LIKE_KLT.has(klt)
  if (dailyOnly) {
    const dateStr = extractYmdDatePart(s.replace(/\//g, '-'))
    if (!/^\d{4}-\d{2}-\d{2}$/.test(dateStr)) return ''
    const noon = Date.parse(`${dateStr}T12:00:00+08:00`)
    if (!Number.isFinite(noon)) return ''
    return formatYmdCompactShanghai(noon - 86400000)
  }
  const sec = eastMoneyKlineFieldToUnixSeconds(s)
  if (sec == null) return ''
  const back = barSecondsForMinuteKlt(klt)
  return formatYmdHmsCompactShanghai((sec - back) * 1000)
}

function applySeriesFromRaw() {
  if (!candleSeries || !volSeries) return
  const { candles, volumes } = toSeriesData(mergedRawRows)
  candleSeries.setData(candles)
  volSeries.setData(volumes)
  syncIndicators()
  syncLongPositionPriceLines()
}

function withProgrammaticTimeRange(fn) {
  programmaticRangeDepth++
  try {
    return fn()
  } finally {
    programmaticRangeDepth--
  }
}

/** 默认只展开最近若干根，避免 fitContent 全量挤在一起；仍可向左拖看更早 */
function applyDefaultVisibleRange() {
  if (!chart || !mergedRawRows.length) return
  const n = mergedRawRows.length
  const vis = Math.min(DEFAULT_VISIBLE_BARS, n)
  const from = Math.max(0, n - vis)
  const to = n - 1 + DEFAULT_RIGHT_LOGICAL_GAP
  chart.timeScale().setVisibleLogicalRange({ from, to })
}


function clearPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function setupPoll() {
  clearPoll()
  if (props.realtimeIntervalMs > 0 && props.code) {
    pollTimer = setInterval(refreshLatestPoll, props.realtimeIntervalMs)
  }
}

function disposeChart() {
  clearPoll()
  if (loadOlderDebounceTimer) {
    clearTimeout(loadOlderDebounceTimer)
    loadOlderDebounceTimer = null
  }
  stopHistoryVisiblePoll()
  detachLongDragWindowListeners()
  detachLongPriceLinePaneListener()
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = null
  resetLongDragState()
  if (chart && logicalRangeHandler) {
    chart.timeScale().unsubscribeVisibleLogicalRangeChange(logicalRangeHandler)
    logicalRangeHandler = null
  }
  if (chart && visibleTimeRangeHandler) {
    chart.timeScale().unsubscribeVisibleTimeRangeChange(visibleTimeRangeHandler)
    visibleTimeRangeHandler = null
  }
  if (chart && crosshairMoveHandler) {
    chart.unsubscribeCrosshairMove(crosshairMoveHandler)
    crosshairMoveHandler = null
  }
  if (chart && chartClickHandler) {
    chart.unsubscribeClick(chartClickHandler)
    chartClickHandler = null
  }
  hoverRawRow.value = null
  defaultLatestRawRow.value = null
  if (chart) {
    try {
      const pe = chart.panes()[0]?.getHTMLElement()
      if (pe?.style) pe.style.cursor = ''
    } catch {
      /* ignore */
    }
    clearLongPositionPriceLines()
    chart.remove()
    chart = null
    candleSeries = null
    volSeries = null
  }
  resetIndicatorHandles()
}

function scheduleLoadOlderDebounced() {
  if (loadOlderDebounceTimer) clearTimeout(loadOlderDebounceTimer)
  loadOlderDebounceTimer = setTimeout(() => {
    loadOlderDebounceTimer = null
    loadOlderHistory()
  }, 280)
}

/** 根据当前可见逻辑区间判断是否需要加载更早 K 线（与 subscribe 共用 + 轮询兜底） */
function tryScheduleLoadOlderFromVisibleRange(range) {
  if (!chart || !candleSeries) return
  if (programmaticRangeDepth > 0 || loadingHistory.value || !hasMoreOlder.value) return
  const lr = range ?? chart?.timeScale().getVisibleLogicalRange()
  if (!lr) return
  const info = candleSeries.barsInLogicalRange(lr)
  if (!info || typeof info.barsBefore !== 'number') return
  if (info.barsBefore < BARS_BEFORE_LOAD_MORE) {
    scheduleLoadOlderDebounced()
  }
}

function onVisibleLogicalRangeChanged(range) {
  if (!range || !candleSeries) return
  if (programmaticRangeDepth > 0) return
  tryScheduleLoadOlderFromVisibleRange(range)
}

/** 分钟线等：部分 WebView 下逻辑区间回调稀疏，时间区间在拖动时更可靠 */
function onVisibleTimeRangeChanged() {
  if (programmaticRangeDepth > 0 || !candleSeries) return
  tryScheduleLoadOlderFromVisibleRange(null)
}

function startHistoryVisiblePoll() {
  stopHistoryVisiblePoll()
  historyVisiblePollTimer = setInterval(() => {
    tryScheduleLoadOlderFromVisibleRange(null)
  }, 400)
}

function stopHistoryVisiblePoll() {
  if (historyVisiblePollTimer) {
    clearInterval(historyVisiblePollTimer)
    historyVisiblePollTimer = null
  }
}

async function loadOlderHistory() {
  if (
    loadingHistory.value ||
    !hasMoreOlder.value ||
    !mergedRawRows.length ||
    !props.code ||
    !chart ||
    !candleSeries
  ) {
    return
  }
  const kltSnap = activeKlt.value
  const codeSnap = props.code
  const oldest = mergedRawRows[0]
  const end = formatEastMoneyEndFromOldest(oldest.day, kltSnap)
  if (!end) {
    hasMoreOlder.value = false
    return
  }
  loadingHistory.value = true
  const logical = chart.timeScale().getVisibleLogicalRange()
  const beforeCount = mergedRawRows.length
  try {
    const result = (await stockApi.getStockKLinePageWithFallback(
      codeSnap,
      props.stockName || '',
      kltSnap,
      HISTORY_PAGE_SIZE,
      end,
    )).data
    if (kltSnap !== activeKlt.value || codeSnap !== props.code) return
    const src = result?.source || ''
    if (src) activeDataSource.value = src
    const raw = result?.data
    const inc = Array.isArray(raw) ? raw : []
    if (!inc.length) {
      hasMoreOlder.value = false
      lastOlderHistoryEndTried = ''
      return
    }
    const merged = mergeKlineRows(mergedRawRows, inc)
    const added = merged.length - beforeCount
    if (added <= 0) {
      if (end === lastOlderHistoryEndTried) {
        hasMoreOlder.value = false
      } else {
        lastOlderHistoryEndTried = end
      }
      return
    }
    lastOlderHistoryEndTried = ''
    mergedRawRows = merged
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      if (logical) {
        chart.timeScale().setVisibleLogicalRange({
          from: logical.from + added,
          to: logical.to + added,
        })
      }
    })
  } catch {
    /* 网络/桥接异常：保留 hasMoreOlder，用户可继续拖动重试 */
  } finally {
    loadingHistory.value = false
  }
}

async function refreshLatestPoll() {
  if (!props.code || !candleSeries) return
  const kltSnap = activeKlt.value
  const codeSnap = props.code
  try {
    const meta = INTERVALS.find((x) => x.klt === kltSnap) || INTERVALS[0]
    const result = (await stockApi.getStockKLineWithFallback(
      codeSnap,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )).data
    if (codeSnap !== props.code || activeKlt.value !== kltSnap) return
    const src = result?.source || ''
    if (src) activeDataSource.value = src
    const raw = result?.data
    const list = Array.isArray(raw) ? raw : []
    if (!list.length) return
    mergedRawRows = mergeRefreshWithLatest(mergedRawRows, list)
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    withProgrammaticTimeRange(() => applySeriesFromRaw())
  } catch {
    /* 静默，避免打断看盘 */
  }
}

function ensureChart() {
  if (!chartContainerRef.value || chart) return
  chart = createChart(chartContainerRef.value, {
    autoSize: true,
    height: props.chartHeight,
    ...chartThemeOptions(props.darkTheme),
  })
  candleSeries = chart.addSeries(CandlestickSeries, {
    upColor: '#ef5350',
    downColor: '#26a69a',
    borderVisible: false,
    wickUpColor: '#ef5350',
    wickDownColor: '#26a69a',
  })
  volSeries = chart.addSeries(
    HistogramSeries,
    {
      priceFormat: { type: 'volume' },
      priceScaleId: 'vol',
      color: 'rgba(38, 166, 154, 0.35)',
    },
    0,
  )
  chart.priceScale('vol').applyOptions({
    scaleMargins: { top: 0.82, bottom: 0 },
  })
  candleSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.06, bottom: 0.22 },
  })
  logicalRangeHandler = onVisibleLogicalRangeChanged
  chart.timeScale().subscribeVisibleLogicalRangeChange(logicalRangeHandler)
  visibleTimeRangeHandler = onVisibleTimeRangeChanged
  chart.timeScale().subscribeVisibleTimeRangeChange(visibleTimeRangeHandler)
  startHistoryVisiblePoll()
  crosshairMoveHandler = (param) => {
    if (param.point === undefined) {
      hoverRawRow.value = null
      clearLongPriceLinePaneCursor()
      if (showChip.value) updateChipFromHover()
      return
    }
    refreshLongPriceLineCursorFromCrosshair(param)
    if (param.time === undefined) {
      hoverRawRow.value = null
      if (showChip.value) updateChipFromHover()
      return
    }
    const bar = param.seriesData.get(candleSeries)
    if (!bar) {
      hoverRawRow.value = null
      if (showChip.value) updateChipFromHover()
      return
    }
    hoverRawRow.value = findRawRowByChartTime(param.time)
    if (showChip.value) updateChipFromHover()
  }
  chart.subscribeCrosshairMove(crosshairMoveHandler)
  chartClickHandler = (param) => {
    if (isLongSuppressChartClick()) return
    if (!candleSeries || !param.point) return
    if (param.paneIndex != null && param.paneIndex !== 0) return
    if (param.hoveredSeries && param.hoveredSeries !== candleSeries) return
    const y = param.point.y
    const raw = candleSeries.coordinateToPrice(y)
    const price = raw == null ? NaN : Number(raw)
    if (!Number.isFinite(price)) return
    const focusKind = longFocusedPriceField.value
    if (focusKind === 'entry' || focusKind === 'stop' || focusKind === 'takeProfit') {
      applyLongPriceFromChartByField(focusKind, price)
      return
    }
    if (!longClickPickEnabled.value || !showLongPosition.value) return
    applyLongClickPrice(price)
  }
  chart.subscribeClick(chartClickHandler)
  syncLongPositionPriceLines()
  nextTick(() => attachLongPriceLineDragListeners())
}

async function loadData() {
  if (!props.code) {
    errorText.value = '未设置股票代码'
    mergedRawRows = []
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    hasMoreOlder.value = true
    lastOlderHistoryEndTried = ''
    candleSeries?.setData([])
    volSeries?.setData([])
    syncLongPositionPriceLines()
    chipItems.value = []
    chipMeta.value = { avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 }
    return
  }
  loading.value = true
  errorText.value = ''
  mergedRawRows = []
  mergedRawRowsVersion.value++
  syncDefaultLatestPanelRow()
  hasMoreOlder.value = true
  lastOlderHistoryEndTried = ''
  try {
    const meta = INTERVALS.find((x) => x.klt === activeKlt.value) || INTERVALS[0]
    const result = (await stockApi.getStockKLineWithFallback(
      props.code,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )).data
    const src = result?.source || ''
    activeDataSource.value = src
    const raw = result?.data
    const list = Array.isArray(raw) ? raw : []
    ensureChart()
    mergedRawRows = mergeKlineRows([], list)
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    const { candles } = toSeriesData(mergedRawRows)
    if (!candles.length) {
      errorText.value =
        '暂无 K 线数据（如 600519.SH、000001.SZ、00700.HK、AAPL.US）'
      candleSeries?.setData([])
      volSeries?.setData([])
      syncIndicators()
      syncLongPositionPriceLines()
      return
    }
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      applyDefaultVisibleRange()
    })

    if (showChip.value) {
      updateChipFromHover()
    } else {
      chipItems.value = []
    }
  } catch (e) {
    errorText.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

function onSelectKlt(klt) {
  activeKlt.value = klt
}

// ---- 指标开关（见 kline/composables/useIndicatorToggles） ----
const { indicators, indicatorToggleMap } = createIndicatorToggles(
  { showMA, showBOLL, showOBV, showMACD, showKDJ, showRSI, showATR, showVWAP, showMFI, showKAMA, showKeltner, showSupertrend, showEMA, showIchimoku, showCCI, showTTMSqueeze, showSAR, showDonchian, showADX, showWilliamsR, showStochRSI, showCMF, showAroon, showCMO, showForceIndex, showPivot, showDEMA, showZigZag, showSATS, showAvgAmp, showAlligator, showAO, showHullMA, showAD, showTRIX, showROC, showFractal, showCHOP, showElderRay, showChaikinOsc, showVWAPBands, showMassIndex, showUlcerIndex, showCoppock, showTEMA, showSMI, showSignalRatio, showSMC, showChip },
  syncIndicators,
)

function onIndicatorToggle(key) {
  if (key === 'chip') {
    toggleChip()
    return
  }
  const fn = indicatorToggleMap[key]
  if (fn) fn()
}






onMounted(() => {
  console.log('[DEBUG onMounted] starting')
  nextTick(() => {
    console.log('[DEBUG onMounted] nextTick callback')
    console.log('[DEBUG onMounted] current longEntryStr:', longEntryStr.value, 'showLongPosition:', showLongPosition.value)
    ensureChart()
    console.log('[DEBUG onMounted] after ensureChart, candleSeries:', !!candleSeries)
    loadData()
    console.log('[DEBUG onMounted] after loadData call')
    setupPoll()
  })
})

onBeforeUnmount(() => {
  disposeChart()
})

watch(
  () => props.code,
  () => {
    hoverRawRow.value = null
    loadData()
    setupPoll()
  },
)

watch(activeKlt, () => {
  hoverRawRow.value = null
  chart?.applyOptions(chartThemeOptions(props.darkTheme))
  loadData()
  setupPoll()
})

watch(
  () => props.darkTheme,
  (d) => {
    chart?.applyOptions(chartThemeOptions(d))
    if (showChip.value) nextTick(() => drawChipCanvas())
  },
)

watch(
  () => props.chartHeight,
  (h) => {
    chart?.applyOptions({ height: h })
    if (showChip.value) nextTick(() => drawChipCanvas())
  },
)

watch(
  () => props.realtimeIntervalMs,
  () => setupPoll(),
)


</script>

<template>
  <div class="lw-kline-root" :class="{ 'lw-kline--dark': darkTheme }">
    <div class="lw-kline-body">
      <KlineIndicatorSidebar :dark-theme="darkTheme" :indicators="indicators" @toggle="onIndicatorToggle" />
      <div class="lw-kline-main">
        <NFlex :size="6" wrap style="row-gap: 4px; align-items: center">
          <NText depth="3" style="font-size: 12px; margin-right: 2px">周期</NText>
          <NButton
            v-for="it in INTERVALS"
            :key="it.klt"
            size="tiny"
            :type="activeKlt === it.klt ? 'primary' : 'default'"
            :secondary="activeKlt !== it.klt"
            @click="onSelectKlt(it.klt)"
          >
            {{ it.label }}
          </NButton>
          <span style="width: 12px" />
          <NText depth="3" style="font-size: 12px; margin-right: 2px">多单</NText>
          <NButton
            size="tiny"
            :type="showLongPosition ? 'primary' : 'default'"
            :secondary="!showLongPosition"
            @click="toggleLongPosition"
          >
            价位线
          </NButton>
          <NInput
            v-model:value="longEntryStr"
            size="tiny"
            placeholder="开仓"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('entry')"
            @blur="onLongPriceInputBlur"
          />
          <NInput
            v-model:value="longStopStr"
            size="tiny"
            placeholder="止损"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('stop')"
            @blur="onLongPriceInputBlur"
          />
          <NInput
            v-model:value="longTakeProfitStr"
            size="tiny"
            placeholder="止盈"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('takeProfit')"
            @blur="onLongPriceInputBlur"
          />
          <NButton size="tiny" secondary @click="fillLongEntryFromLatestClose">
            最新收盘
          </NButton>
          <NButton
            size="tiny"
            :type="longClickPickEnabled ? 'primary' : 'default'"
            :secondary="!longClickPickEnabled"
            @click="toggleLongClickPick"
          >
            设置价位线
          </NButton>
          <NButton
            v-if="longClickPickEnabled"
            size="tiny"
            quaternary
            @click="resetLongClickSequence"
          >
            重置
          </NButton>
          <NText
            v-if="longFocusChartHint"
            depth="3"
            class="lw-kline-longpos-focus-hint"
          >
            {{ longFocusChartHint }}
          </NText>
          <NText
            v-if="longClickPickEnabled && showLongPosition"
            depth="3"
            class="lw-kline-longpos-click-hint"
          >
            点击K线设置{{ longClickNextLabel }}
          </NText>
          <NText v-if="longPositionHint" depth="3" class="lw-kline-longpos-hint">
            {{ longPositionHint }}
          </NText>
        </NFlex>
        <div
          class="lw-kline-crosshair-strip"
          :class="{ 'lw-kline-crosshair-strip--dark': darkTheme }"
        >
          <template v-if="crosshairPanel">
            <div class="lw-kline-crosshair-strip__head">
              <span class="lw-kline-crosshair-strip__date">{{ crosshairPanel.title }}</span>
            </div>
            <div class="lw-kline-crosshair-strip__grid">
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">开盘</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cOpenClose }">{{
                  crosshairPanel.open
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">收盘</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cOpenClose }">{{
                  crosshairPanel.close
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">最高</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cHigh }">{{
                  crosshairPanel.high
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">最低</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cLow }">{{
                  crosshairPanel.low
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">涨跌幅</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.changePercent
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">涨跌额</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.changeValue
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">成交量</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.volume
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">成交额</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.amount
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">振幅</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.amplitude
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">均幅5</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.avgAmp5
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">均幅10</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.avgAmp10
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">均幅20</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.avgAmp20
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">换手率</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.turnoverRate
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">量比</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.volumeRatio
                }}</span>
              </span>
            </div>
          </template>
          <NText v-else depth="3" style="font-size: 11px; line-height: 1.5">
            {{ loading ? '加载中…' : '暂无 K 线数据' }}
          </NText>
        </div>
        <div
          v-if="indicatorSignalSummary"
          class="lw-kline-signal-summary"
          :class="{ 'lw-kline-signal-summary--dark': darkTheme }"
        >
          <div class="lw-kline-signal-summary__head">
            <span class="lw-kline-signal-summary__title">指标信号汇总</span>
            <span class="lw-kline-signal-summary__total">共 {{ indicatorSignalSummary.total }} 项</span>
          </div>
          <div class="lw-kline-signal-summary__bar">
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--bullish" :style="{ width: indicatorSignalSummary.bullishPct + '%' }"></div>
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--bearish" :style="{ width: indicatorSignalSummary.bearishPct + '%' }"></div>
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--oscillating" :style="{ width: indicatorSignalSummary.oscillatingPct + '%' }"></div>
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--neutral" :style="{ width: indicatorSignalSummary.neutralPct + '%' }"></div>
          </div>
          <div class="lw-kline-signal-summary__legend">
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--bullish"></span>
              看多 {{ indicatorSignalSummary.bullish }} ({{ indicatorSignalSummary.bullishPct }}%)
            </span>
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--bearish"></span>
              看空 {{ indicatorSignalSummary.bearish }} ({{ indicatorSignalSummary.bearishPct }}%)
            </span>
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--oscillating"></span>
              震荡 {{ indicatorSignalSummary.oscillating }} ({{ indicatorSignalSummary.oscillatingPct }}%)
            </span>
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--neutral"></span>
              中性 {{ indicatorSignalSummary.neutral }} ({{ indicatorSignalSummary.neutralPct }}%)
            </span>
          </div>
          <div class="lw-kline-signal-summary__tags">
            <span
              v-for="s in indicatorSignalSummary.signals"
              :key="s.name"
              class="lw-kline-signal-summary__tag"
              :class="'lw-kline-signal-summary__tag--' + s.signal"
            >{{ s.name }}</span>
          </div>
        </div>
        <NText v-if="errorText" type="error" style="font-size: 12px">{{ errorText }}</NText>
        <div class="lw-kline-chart-wrap">
          <div
            ref="chartContainerRef"
            class="lw-kline-chart"
            :style="{ height: chartHeight-110 + 'px', minHeight: chartHeight-110 + 'px' }"
          />
          <div
            v-if="showChip"
            class="lw-chip"
            :class="{ 'lw-chip--dark': darkTheme }"
            :style="{ height: chartHeight-110 + 'px', minHeight: chartHeight-110 + 'px' }"
          >
            <div class="lw-chip__head">
              <span class="lw-chip__title">筹码分布</span>
              <span v-if="chipMeta.hoverDate" class="lw-chip__meta">
                {{ chipMeta.hoverDate }}
              </span>
              <span v-if="chipItems.length" class="lw-chip__meta">
                均成本 {{ chipMeta.avgCost.toFixed(2) }} · 获利
                {{ (chipMeta.profitRatio * 100).toFixed(1) }}%
              </span>
            </div>
            <div v-if="!chipItems.length" class="lw-chip__empty">
              {{ mergedRawRows.length ? '移动鼠标到K线查看' : '暂无K线数据' }}
            </div>
            <canvas
              v-show="chipItems.length"
              ref="chipCanvasRef"
              class="lw-chip__canvas"
            />
          </div>
        </div>
        <NFlex align="center" :size="8" class="lw-kline-hint-row">
          <NText depth="3" class="lw-kline-hint-text">
            {{ stockName || code }} ·
            {{ 
              realtimeIntervalMs > 0
                ? `每 ${Math.round(realtimeIntervalMs / 1000)} 秒刷新`
                : '切换周期后加载'
            }}
            · 按住拖动查看左侧历史时会自动加载更早 K 线
            <span v-if="activeDataSource" class="lw-kline-source-tag" :class="{ 'lw-kline-source-tag--fallback': activeDataSource !== 'eastmoney' && activeDataSource !== 'tdx-mac' && activeDataSource !== 'tdx-mac-ex' }">
              {{ activeDataSource === 'eastmoney' ? '东方财富' : activeDataSource === 'tdx-mac' ? '通达信MAC' : activeDataSource === 'tdx-mac-ex' ? '通达信MAC扩展' : activeDataSource === 'sina' ? '新浪财经' : activeDataSource === 'tencent' ? '腾讯财经' : activeDataSource === 'tdx' ? '通达信' : activeDataSource }}
            </span>
          </NText>
          <NSpin v-if="loading || loadingHistory" size="small" />
        </NFlex>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lw-kline-root {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  --wails-draggable: no-drag;
}
.lw-kline-body {
  display: flex;
  width: 100%;
  gap: 8px;
  align-items: stretch;
}
.lw-kline-sidebar {
  flex: 0 0 auto;
  width: 140px;
  min-width: 120px;
}
.lw-kline--dark .lw-kline-sidebar {
  border-color: #3f3f46;
}
.lw-kline-sidebar__inner {
  min-width: 0;
  position: sticky;
  top: 0;
}
.lw-kline-sidebar__section {
  margin-bottom: 6px;
}
.lw-kline-main {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.lw-kline-hint-row {
  min-width: 0;
  max-width: 100%;
}
.lw-kline-hint-text {
  font-size: 12px;
  min-width: 0;
  flex: 1 1 auto;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.lw-kline-longpos-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 200px;
  overflow-wrap: anywhere;
}
.lw-kline-longpos-click-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 220px;
  color: #0ea5e9;
  overflow-wrap: anywhere;
}
.lw-kline--dark .lw-kline-longpos-click-hint {
  color: #38bdf8;
}
.lw-kline-longpos-focus-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 220px;
  color: #d97706;
  overflow-wrap: anywhere;
}
.lw-kline--dark .lw-kline-longpos-focus-hint {
  color: #fbbf24;
}
.lw-kline-crosshair-strip {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  overflow-x: auto;
  overflow-y: hidden;
}
.lw-kline-crosshair-strip--dark {
  border-color: #3f3f46;
  background: #18181b;
}
.lw-kline-crosshair-strip__head {
  margin-bottom: 4px;
}
.lw-kline-crosshair-strip__date {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-kline-crosshair-strip--dark .lw-kline-crosshair-strip__date {
  color: #f1f5f9;
}
.lw-kline-kv {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
  flex-shrink: 0;
}
.lw-kline-crosshair-strip__grid {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  column-gap: 14px;
  row-gap: 4px;
  font-size: 11px;
  min-width: 0;
}
.lw-kline-crosshair-strip__k {
  color: #64748b;
  white-space: nowrap;
}
.lw-kline-crosshair-strip--dark .lw-kline-crosshair-strip__k {
  color: #94a3b8;
}
.lw-kline-crosshair-strip__v {
  font-variant-numeric: tabular-nums;
  min-width: 0;
}
.lw-kline-chart {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  position: relative;
  touch-action: none;
  box-sizing: border-box;
}

.lw-kline-chart-wrap {
  width: 100%;
  display: flex;
  gap: 10px;
  align-items: stretch;
  min-width: 0;
}
.lw-kline--dark .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #27272a;
}
.lw-kline-root:not(.lw-kline--dark) .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #e2e8f0;
}

.lw-chip {
  width: 160px;
  flex: 0 0 160px;
  border-radius: 4px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  box-sizing: border-box;
  padding: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.lw-chip--dark {
  border-color: #27272a;
  background: #141414;
}
.lw-chip__head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin-bottom: 6px;
  flex-wrap: wrap;
}
.lw-chip__title {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-chip--dark .lw-chip__title {
  color: #f1f5f9;
}
.lw-chip__meta {
  font-size: 11px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lw-chip--dark .lw-chip__meta {
  color: #94a3b8;
}
.lw-chip__empty {
  font-size: 11px;
  color: #64748b;
}
.lw-chip--dark .lw-chip__empty {
  color: #94a3b8;
}
.lw-chip__canvas {
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
  display: block;
}
.lw-kline-source-tag {
  display: inline-block;
  font-size: 10px;
  line-height: 1;
  padding: 2px 5px;
  border-radius: 3px;
  background: #e0f2fe;
  color: #0369a1;
  vertical-align: middle;
  margin-left: 4px;
}
.lw-kline--dark .lw-kline-source-tag {
  background: #1e3a5f;
  color: #7dd3fc;
}
.lw-kline-source-tag--fallback {
  background: #fef3c7;
  color: #b45309;
}
.lw-kline--dark .lw-kline-source-tag--fallback {
  background: #422006;
  color: #fbbf24;
}
.lw-kline-signal-summary {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}
.lw-kline-signal-summary--dark {
  border-color: #3f3f46;
  background: #18181b;
}
.lw-kline-signal-summary__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.lw-kline-signal-summary__title {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__title {
  color: #f1f5f9;
}
.lw-kline-signal-summary__total {
  font-size: 11px;
  color: #64748b;
  white-space: nowrap;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__total {
  color: #94a3b8;
}
.lw-kline-signal-summary__bar {
  display: flex;
  height: 8px;
  border-radius: 4px;
  overflow: hidden;
  background: #e2e8f0;
  margin-bottom: 6px;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__bar {
  background: #3f3f46;
}
.lw-kline-signal-summary__bar-seg {
  height: 100%;
  min-width: 0;
  transition: width 0.3s ease;
}
.lw-kline-signal-summary__bar-seg--bullish { background: #ef4444; }
.lw-kline-signal-summary__bar-seg--bearish { background: #22c55e; }
.lw-kline-signal-summary__bar-seg--oscillating { background: #f59e0b; }
.lw-kline-signal-summary__bar-seg--neutral { background: #94a3b8; }
.lw-kline-signal-summary__legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  margin-bottom: 6px;
  font-size: 11px;
  color: #334155;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__legend {
  color: #cbd5e1;
}
.lw-kline-signal-summary__legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
.lw-kline-signal-summary__dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.lw-kline-signal-summary__dot--bullish { background: #ef4444; }
.lw-kline-signal-summary__dot--bearish { background: #22c55e; }
.lw-kline-signal-summary__dot--oscillating { background: #f59e0b; }
.lw-kline-signal-summary__dot--neutral { background: #94a3b8; }
.lw-kline-signal-summary__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.lw-kline-signal-summary__tag {
  display: inline-block;
  font-size: 10px;
  line-height: 1;
  padding: 3px 6px;
  border-radius: 3px;
  white-space: nowrap;
}
.lw-kline-signal-summary__tag--bullish {
  background: #fef2f2;
  color: #dc2626;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--bullish {
  background: #450a0a;
  color: #fca5a5;
}
.lw-kline-signal-summary__tag--bearish {
  background: #f0fdf4;
  color: #16a34a;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--bearish {
  background: #052e16;
  color: #86efac;
}
.lw-kline-signal-summary__tag--oscillating {
  background: #fffbeb;
  color: #d97706;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--oscillating {
  background: #451a03;
  color: #fcd34d;
}
.lw-kline-signal-summary__tag--neutral {
  background: #f1f5f9;
  color: #475569;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--neutral {
  background: #1e293b;
  color: #94a3b8;
}
</style>
