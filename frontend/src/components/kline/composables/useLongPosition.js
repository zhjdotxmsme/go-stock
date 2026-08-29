/**
 * TradingView 风格「多单」：开仓 / 止损 / 止盈 价位线（显示、拖拽、点击取价、输入框联动）。
 * 自 StockLightweightKlineChart.vue 原样搬迁；chart / candleSeries 原为组件模块级 let，
 * 现经 ctx.getChart()/ctx.getCandleSeries() 调用时取值，语义一致。
 */
import { computed, nextTick, ref, watch } from 'vue'
import { LineStyle } from 'lightweight-charts'
import { parseNumStr } from '../format'
import { CLR_RISE, CLR_FALL } from '../constants'

export function useLongPosition(ctx) {
  const { props, emit, getChart, getCandleSeries, chartContainerRef, defaultLatestRawRow } = ctx

  /** TradingView 风格「多单」：开仓 / 止损 / 止盈 价位线 */
  const showLongPosition = ref(false)
  const longEntryStr = ref('')
  const longStopStr = ref('')
  const longTakeProfitStr = ref('')
  const longCostStr = ref('')
  /** 在 K 线主区点击，按顺序写入开仓 → 止损 → 止盈（再点回到开仓） */
  const longClickPickEnabled = ref(false)
  const longClickNextField = ref('entry')
  /** 点过开仓/止损/止盈输入框后，下一次主图点击写入对应价位（blur 延迟清除以兼容「先失焦后 click」） */
  const longFocusedPriceField = ref(null)
  /** 由 props 写入价位时抑制 emit，避免与 v-model 循环 */
  const suppressLongPriceEmit = ref(false)

  /** 多单标注：createPriceLine 返回的句柄，需在 dispose / 重绘前 remove */
  let longPositionPriceLines = []
  /** 各价位线句柄，用于命中与拖动时 applyOptions */
  let longLineByKind = { entry: null, stop: null, takeProfit: null }
  /** 正在拖动某条多单价位线时禁止 watch 里整表重建线条 */
  let longPositionDragActive = false
  let longDragKind = null
  /** 命中线后抑制一次「图上点击设价」，避免拖/点冲突 */
  let longSuppressChartClick = false
  let longPaneDragEl = null
  /** window 上拖动用监听器是否已挂（避免重复解绑 / 重复 up） */
  let longDragWindowListenersOn = false
  /** 拖动过程中最近一次指针 Y，用于松手后恢复「可拖」光标 */
  let longLastPointerClientY = null
  /** 垂直方向命中容差（px） */
  const LONG_PRICE_LINE_HIT_PX = 12
  /** 输入框 blur 后延迟清除「待图表取价」状态（ms），需大于 click 相对 blur 的间隔 */
  const LONG_FOCUS_BLUR_CLEAR_MS = 600
  let longFocusBlurTimer = null

  function clearLongPositionPriceLines() {
    const chart = getChart()
    const candleSeries = getCandleSeries()
    longLineByKind = { entry: null, stop: null, takeProfit: null }
    if (!candleSeries) {
      longPositionPriceLines = []
      return
    }
    for (const pl of longPositionPriceLines) {
      try {
        candleSeries.removePriceLine(pl)
      } catch {
        /* ignore */
      }
    }
    longPositionPriceLines = []
  }
  
  function syncLongPositionPriceLines() {
    const candleSeries = getCandleSeries()
    console.log('[DEBUG syncLongPositionPriceLines] called, showLongPosition:', showLongPosition.value, 'candleSeries:', !!candleSeries)
    clearLongPositionPriceLines()
    if (!showLongPosition.value || !candleSeries) {
      console.log('[DEBUG syncLongPositionPriceLines] early return, showLongPosition:', showLongPosition.value, 'candleSeries:', !!candleSeries)
      return
    }
    const entry = parseNumStr(longEntryStr.value)
    const stop = parseNumStr(longStopStr.value)
    const tp = parseNumStr(longTakeProfitStr.value)
    const cost = parseNumStr(longCostStr.value)
    console.log('[DEBUG syncLongPositionPriceLines] entry:', entry, 'stop:', stop, 'tp:', tp, 'cost:', cost)
  
    // 如果没有任何价格信息，直接返回
    if (!Number.isFinite(entry) && !Number.isFinite(cost)) {
      console.log('[DEBUG syncLongPositionPriceLines] no valid price, returning')
      return
    }
  
    const pushLine = (price, kind, partial) => {
      const pl = candleSeries.createPriceLine({
        price,
        lineWidth: 2,
        axisLabelVisible: true,
        ...partial,
      })
      longPositionPriceLines.push(pl)
      longLineByKind[kind] = pl
    }
    if (Number.isFinite(entry)) {
      pushLine(entry, 'entry', {
        color: '#3b82f6',
        lineStyle: LineStyle.Solid,
        title: '开仓',
      })
    }
    if (Number.isFinite(cost)) {
      pushLine(cost, 'cost', {
        color: '#f59e0b',
        lineStyle: LineStyle.Dashed,
        title: '成本',
      })
    }
    if (Number.isFinite(stop)) {
      pushLine(stop, 'stop', {
        color: CLR_FALL,
        lineStyle: LineStyle.Dashed,
        title: '止损',
      })
    }
    if (Number.isFinite(tp)) {
      pushLine(tp, 'takeProfit', {
        color: CLR_RISE,
        lineStyle: LineStyle.Dashed,
        title: '止盈',
      })
    }
    console.log('[DEBUG syncLongPositionPriceLines] done, created', longPositionPriceLines.length, 'lines')
  }
  
  function getLongDragPaneElement() {
    const chart = getChart()
    if (!chart) return chartContainerRef.value
    return chart.panes()[0]?.getHTMLElement() ?? chartContainerRef.value
  }
  
  function longPaneLocalYFromClient(clientY) {
    const el = getLongDragPaneElement()
    if (!el) return null
    const r = el.getBoundingClientRect()
    const y = clientY - r.top
    return Number.isFinite(y) ? y : null
  }
  
  function refreshLongPriceLineCursorFromCrosshair(param) {
    const paneEl = getLongDragPaneElement()
    if (!paneEl) return
    if (longPositionDragActive) {
      paneEl.style.cursor = 'grabbing'
      return
    }
    if (
      !showLongPosition.value ||
      !longLineByKind.entry ||
      param.point === undefined ||
      (param.paneIndex != null && param.paneIndex !== 0)
    ) {
      paneEl.style.cursor = ''
      return
    }
    const kind = hitTestLongPriceLineKind(param.point.y)
    paneEl.style.cursor = kind ? 'grab' : ''
  }
  
  function clearLongPriceLinePaneCursor() {
    if (longPositionDragActive) return
    const paneEl = getLongDragPaneElement()
    if (paneEl) paneEl.style.cursor = ''
  }
  
  /** @returns {'entry'|'stop'|'takeProfit'|null} */
  function hitTestLongPriceLineKind(localY) {
    const candleSeries = getCandleSeries()
    if (!candleSeries || !showLongPosition.value || localY == null) return null
    let best = null
    let bestDist = LONG_PRICE_LINE_HIT_PX + 1
    for (const kind of ['entry', 'stop', 'takeProfit']) {
      const line = longLineByKind[kind]
      if (!line) continue
      const p = Number(line.options().price)
      if (!Number.isFinite(p)) continue
      const ly = candleSeries.priceToCoordinate(p)
      if (ly == null) continue
      const cy = Number(ly)
      if (!Number.isFinite(cy)) continue
      const d = Math.abs(cy - localY)
      if (d <= LONG_PRICE_LINE_HIT_PX && d < bestDist) {
        bestDist = d
        best = kind
      }
    }
    return best
  }
  
  function detachLongDragWindowListeners() {
    if (!longDragWindowListenersOn) return
    longDragWindowListenersOn = false
    window.removeEventListener('pointermove', onLongDragWindowMove, true)
    window.removeEventListener('pointerup', onLongDragWindowUp, true)
    window.removeEventListener('pointercancel', onLongDragWindowUp, true)
  }
  
  function onLongDragWindowMove(e) {
    const candleSeries = getCandleSeries()
    if (!longPositionDragActive || !longDragKind || !candleSeries) return
    longLastPointerClientY = e.clientY
    const y = longPaneLocalYFromClient(e.clientY)
    if (y == null) return
    const raw = candleSeries.coordinateToPrice(y)
    const price = raw == null ? NaN : Number(raw)
    if (!Number.isFinite(price)) return
    const s = price.toFixed(2)
    const line = longLineByKind[longDragKind]
    if (line) {
      try {
        line.applyOptions({ price })
      } catch {
        /* ignore */
      }
    }
    if (longDragKind === 'entry') longEntryStr.value = s
    else if (longDragKind === 'stop') longStopStr.value = s
    else longTakeProfitStr.value = s
  }
  
  function onLongDragWindowUp() {
    if (!longDragWindowListenersOn) return
    const was = longPositionDragActive
    detachLongDragWindowListeners()
    longPositionDragActive = false
    longDragKind = null
    if (was) syncLongPositionPriceLines()
    const paneEl = getLongDragPaneElement()
    if (paneEl) {
      if (
        longLastPointerClientY != null &&
        showLongPosition.value &&
        longLineByKind.entry
      ) {
        const ly = longPaneLocalYFromClient(longLastPointerClientY)
        const kind = ly != null ? hitTestLongPriceLineKind(ly) : null
        paneEl.style.cursor = kind ? 'grab' : ''
      } else {
        paneEl.style.cursor = ''
      }
    }
    longLastPointerClientY = null
    setTimeout(() => {
      longSuppressChartClick = false
    }, 0)
  }
  
  function onLongPriceLinePointerDownCapture(e) {
    const candleSeries = getCandleSeries()
    if (!showLongPosition.value || !candleSeries) return
    if (e.pointerType === 'mouse' && e.button !== 0) return
    const y = longPaneLocalYFromClient(e.clientY)
    if (y == null) return
    const kind = hitTestLongPriceLineKind(y)
    if (!kind) return
    longLastPointerClientY = e.clientY
    longSuppressChartClick = true
    longPositionDragActive = true
    longDragKind = kind
    const paneElGrab = getLongDragPaneElement()
    if (paneElGrab) paneElGrab.style.cursor = 'grabbing'
    try {
      e.preventDefault()
    } catch {
      /* ignore */
    }
    e.stopPropagation()
    detachLongDragWindowListeners()
    longDragWindowListenersOn = true
    window.addEventListener('pointermove', onLongDragWindowMove, true)
    window.addEventListener('pointerup', onLongDragWindowUp, true)
    window.addEventListener('pointercancel', onLongDragWindowUp, true)
  }
  
  function attachLongPriceLineDragListeners() {
    detachLongPriceLinePaneListener()
    longPaneDragEl = getLongDragPaneElement()
    if (longPaneDragEl) {
      longPaneDragEl.addEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
    }
  }
  
  function detachLongPriceLinePaneListener() {
    if (longPaneDragEl) {
      longPaneDragEl.removeEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
      longPaneDragEl = null
    }
  }
  
  const longPositionStats = computed(() => {
    const entry = parseNumStr(longEntryStr.value)
    const stop = parseNumStr(longStopStr.value)
    const tp = parseNumStr(longTakeProfitStr.value)
    if (!Number.isFinite(entry)) return null
    const risk = Number.isFinite(stop) ? entry - stop : NaN
    const reward = Number.isFinite(tp) ? tp - entry : NaN
    const rr =
      Number.isFinite(risk) && risk > 0 && Number.isFinite(reward)
        ? reward / risk
        : NaN
    return {
      riskPts: risk,
      rewardPts: reward,
      riskRr: rr,
      riskPct: Number.isFinite(risk) && entry !== 0 ? (risk / entry) * 100 : NaN,
      rewardPct: Number.isFinite(reward) && entry !== 0 ? (reward / entry) * 100 : NaN,
    }
  })
  
  const longPositionHint = computed(() => {
    if (!showLongPosition.value) return ''
    if (!Number.isFinite(parseNumStr(longEntryStr.value))) {
      return '输入开仓价后显示线；止损低于开仓、止盈高于开仓为典型多单'
    }
    const s = longPositionStats.value
    if (!s) return ''
    const parts = []
    if (Number.isFinite(s.riskPts)) parts.push(`风险幅度 ${s.riskPts.toFixed(2)}`)
    if (Number.isFinite(s.rewardPts)) parts.push(`目标幅度 ${s.rewardPts.toFixed(2)}`)
    if (Number.isFinite(s.riskRr) && s.riskRr > 0) parts.push(`盈亏比 1 : ${s.riskRr.toFixed(2)}`)
    parts.push('单击线条可拖动改价')
    return parts.join(' · ')
  })
  
  function toggleLongPosition() {
    showLongPosition.value = !showLongPosition.value
    if (!showLongPosition.value) {
      longClickPickEnabled.value = false
      clearLongFocusedPriceField()
      clearLongPriceLinePaneCursor()
    }
    syncLongPositionPriceLines()
  }
  
  function fillLongEntryFromLatestClose() {
    const r = defaultLatestRawRow.value
    if (!r) return
    const c = parseNumStr(r.close)
    if (!Number.isFinite(c)) return
    longEntryStr.value = c.toFixed(2)
    showLongPosition.value = true
    syncLongPositionPriceLines()
  }
  
  const longClickNextLabel = computed(() => {
    const m = { entry: '开仓价', stop: '止损价', takeProfit: '止盈价' }
    return m[longClickNextField.value] || '开仓价'
  })
  
  const longFocusChartHint = computed(() => {
    const k = longFocusedPriceField.value
    if (!k) return ''
    const m = { entry: '开仓', stop: '止损', takeProfit: '止盈' }
    return `已选「${m[k] || ''}」：请在 K 线主图（非成交量）点击纵轴位置写入价格`
  })
  
  function cancelLongFocusBlurTimer() {
    if (longFocusBlurTimer != null) {
      clearTimeout(longFocusBlurTimer)
      longFocusBlurTimer = null
    }
  }
  
  function onLongPriceInputFocus(kind) {
    cancelLongFocusBlurTimer()
    longFocusedPriceField.value = kind
  }
  
  function onLongPriceInputBlur() {
    cancelLongFocusBlurTimer()
    longFocusBlurTimer = setTimeout(() => {
      longFocusBlurTimer = null
      longFocusedPriceField.value = null
    }, LONG_FOCUS_BLUR_CLEAR_MS)
  }
  
  function clearLongFocusedPriceField() {
    cancelLongFocusBlurTimer()
    longFocusedPriceField.value = null
  }
  
  function applyLongPriceFromChartByField(kind, price) {
    if (!Number.isFinite(price)) return
    const s = price.toFixed(2)
    showLongPosition.value = true
    if (kind === 'entry') longEntryStr.value = s
    else if (kind === 'stop') longStopStr.value = s
    else if (kind === 'takeProfit') longTakeProfitStr.value = s
    clearLongFocusedPriceField()
    syncLongPositionPriceLines()
  }
  
  function toggleLongClickPick() {
    longClickPickEnabled.value = !longClickPickEnabled.value
    if (longClickPickEnabled.value) {
      showLongPosition.value = true
      longClickNextField.value = 'entry'
    }
    syncLongPositionPriceLines()
  }
  
  function resetLongClickSequence() {
    longClickNextField.value = 'entry'
  }
  
  function applyLongClickPrice(price) {
    if (!Number.isFinite(price)) return
    const s = price.toFixed(2)
    const step = longClickNextField.value
    if (step === 'entry') {
      longEntryStr.value = s
      longClickNextField.value = 'stop'
    } else if (step === 'stop') {
      longStopStr.value = s
      longClickNextField.value = 'takeProfit'
    } else {
      longTakeProfitStr.value = s
      longClickNextField.value = 'entry'
    }
    syncLongPositionPriceLines()
  }

  function pricePropToInputStr(v) {
    if (v == null) return ''
    return String(v).trim()
  }

  watch(
    () => [props.longEntryPrice, props.longStopLossPrice, props.longTakeProfitPrice, props.costPrice],
    ([e, s, t, c]) => {
      console.log('[DEBUG props watch] triggered, e:', e, 's:', s, 't:', t, 'c:', c, 'candleSeries:', !!getCandleSeries())
      let needReleaseSuppress = false
      if (e !== undefined) {
        const se = pricePropToInputStr(e)
        console.log('[DEBUG props watch] entry: propVal=', e, 'converted=', se, 'current longEntryStr=', longEntryStr.value)
        if (se !== longEntryStr.value) {
          suppressLongPriceEmit.value = true
          needReleaseSuppress = true
          longEntryStr.value = se
        }
      }
      if (s !== undefined) {
        const ss = pricePropToInputStr(s)
        if (ss !== longStopStr.value) {
          suppressLongPriceEmit.value = true
          needReleaseSuppress = true
          longStopStr.value = ss
        }
      }
      if (t !== undefined) {
        const st = pricePropToInputStr(t)
        if (st !== longTakeProfitStr.value) {
          suppressLongPriceEmit.value = true
          needReleaseSuppress = true
          longTakeProfitStr.value = st
        }
      }
      if (c !== undefined) {
        const sc = pricePropToInputStr(c)
        if (sc !== longCostStr.value) {
          suppressLongPriceEmit.value = true
          needReleaseSuppress = true
          longCostStr.value = sc
        }
      }
      if (needReleaseSuppress) {
        nextTick(() => {
          suppressLongPriceEmit.value = false
        })
      }
      const hasPrice = Number.isFinite(parseNumStr(longEntryStr.value)) || Number.isFinite(parseNumStr(longStopStr.value)) || Number.isFinite(parseNumStr(longTakeProfitStr.value)) || Number.isFinite(parseNumStr(longCostStr.value))
      console.log('[DEBUG props watch] hasPrice:', hasPrice, 'showLongPosition:', showLongPosition.value)
      if (hasPrice) {
        showLongPosition.value = true
        nextTick(() => {
          console.log('[DEBUG props watch] nextTick: calling syncLongPositionPriceLines, candleSeries:', !!getCandleSeries())
          syncLongPositionPriceLines()
        })
      }
    },
    { immediate: true },
  )

  watch(longEntryStr, (v) => {
    console.log('[DEBUG longEntryStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longEntryPrice:', props.longEntryPrice)
    if (suppressLongPriceEmit.value) return
    emit('update:longEntryPrice', v)
  })
  watch(longStopStr, (v) => {
    console.log('[DEBUG longStopStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longStopLossPrice:', props.longStopLossPrice)
    if (suppressLongPriceEmit.value) return
    emit('update:longStopLossPrice', v)
  })
  watch(longTakeProfitStr, (v) => {
    console.log('[DEBUG longTakeProfitStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longTakeProfitPrice:', props.longTakeProfitPrice)
    if (suppressLongPriceEmit.value) return
    emit('update:longTakeProfitPrice', v)
  })
  watch(longCostStr, (v) => {
    console.log('[DEBUG longCostStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.costPrice:', props.costPrice)
    if (suppressLongPriceEmit.value) return
    emit('update:costPrice', v)
  })

  watch(
    [showLongPosition, longEntryStr, longStopStr, longTakeProfitStr],
    () => {
      console.log('[DEBUG priceLines watch] triggered, showLongPosition:', showLongPosition.value, 'longEntryStr:', longEntryStr.value)
      if (longPositionDragActive) return
      syncLongPositionPriceLines()
    },
  )

  watch(showLongPosition, (newVal) => {
    console.log('[DEBUG showLongPosition watch] changed to:', newVal)
    if (newVal && getCandleSeries()) {
      nextTick(() => syncLongPositionPriceLines())
    }
  })

  /** dispose 时复位拖动相关的模块级标记（原 disposeChart 内联赋值） */
  function resetLongDragState() {
    longPositionDragActive = false
    longDragKind = null
    longSuppressChartClick = false
  }

  /** 图表点击处理器读取：命中价位线后抑制一次「图上点击设价」 */
  function isLongSuppressChartClick() {
    return longSuppressChartClick
  }

  return {
    showLongPosition, longEntryStr, longStopStr, longTakeProfitStr, longCostStr,
    longClickPickEnabled, longClickNextField, longFocusedPriceField, suppressLongPriceEmit,
    longPositionStats, longPositionHint, longClickNextLabel, longFocusChartHint,
    toggleLongPosition, fillLongEntryFromLatestClose, onLongPriceInputFocus, onLongPriceInputBlur,
    toggleLongClickPick, resetLongClickSequence,
    syncLongPositionPriceLines, clearLongPositionPriceLines,
    refreshLongPriceLineCursorFromCrosshair, clearLongPriceLinePaneCursor,
    attachLongPriceLineDragListeners, detachLongPriceLinePaneListener, detachLongDragWindowListeners,
    cancelLongFocusBlurTimer, applyLongPriceFromChartByField, applyLongClickPrice,
    pricePropToInputStr, resetLongDragState, isLongSuppressChartClick,
  }
}
