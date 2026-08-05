/**
 * 十字线 / 默认信息面板：时间格式化、原始行查找、最新行同步、面板数据组装。
 * 自 StockLightweightKlineChart.vue 原样搬迁；mergedRawRows 原为组件模块级 let，
 * 现经 ctx.getMergedRawRows() 调用时取值，语义一致。
 */
import { computed } from 'vue'
import { parseNumStr, formatPrice2, formatVolumeCn, formatAmountCn, formatPctField, formatSigned2 } from '../format'
import { chartTimeToUtcMs, toChartTime, extractYmdDatePart } from '../time'
import { CLR_RISE, CLR_FALL, DAILY_LIKE_KLT, CN_TZ } from '../constants'

export function createCrosshairPanel(ctx) {
  const { props, activeKlt, hoverRawRow, defaultLatestRawRow, getMergedRawRows } = ctx

  
  
  
  
  
  function avgAmplitude(amplitudes, period) {
    if (!amplitudes || amplitudes.length < period) return NaN
    let s = 0, cnt = 0
    for (let i = amplitudes.length - period; i < amplitudes.length; i++) {
      const v = amplitudes[i]
      if (Number.isFinite(v)) { s += v; cnt++ }
    }
    return cnt === period ? s / cnt : NaN
  }
  
  function formatVolumeRatio(v) {
    if (v == null || v === '' || v === '--') return '--'
    const n = Number(v)
    return Number.isFinite(n) ? n.toFixed(2) : '--'
  }
  
  
  /** 单根 K 的近似「成本中枢」：优先日 VWAP（成交额/量），否则典型价，夹在 [L,H] */
  
  
  
  
  
  
  function formatCrosshairTime(time) {
    const ms = chartTimeToUtcMs(time)
    if (!Number.isFinite(ms)) return ''
    const d = new Date(ms)
    const loc = 'zh-CN'
    const minuteLike = !DAILY_LIKE_KLT.has(activeKlt.value)
    if (minuteLike) {
      return new Intl.DateTimeFormat(loc, {
        timeZone: CN_TZ,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      }).format(d)
    }
    return new Intl.DateTimeFormat(loc, {
      timeZone: CN_TZ,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(d)
  }
  
  function findRawRowByChartTime(t) {
    const mergedRawRows = getMergedRawRows()
    if (t === undefined || t === null) return null
    const targetMs = chartTimeToUtcMs(t)
    if (Number.isFinite(targetMs)) {
      for (const r of mergedRawRows) {
        const ct = toChartTime(r.day)
        const ms = chartTimeToUtcMs(ct)
        if (Number.isFinite(ms) && ms === targetMs) return r
      }
    }
    for (const r of mergedRawRows) {
      const ct = toChartTime(r.day)
      if (ct === t) return r
    }
    return null
  }
  
  /** mergedRawRows 按时间升序，取最后一根作为面板默认展示 */
  function syncDefaultLatestPanelRow() {
    const mergedRawRows = getMergedRawRows()
    const rows = mergedRawRows
    if (!rows.length) {
      defaultLatestRawRow.value = null
      return
    }
    defaultLatestRawRow.value = rows[rows.length - 1]
  }
  
  function formatPanelTitleDay(r) {
    const dailyLike = DAILY_LIKE_KLT.has(activeKlt.value)
    if (dailyLike) {
      const ymd = extractYmdDatePart(String(r.day || '').replace(/\//g, '-'))
      if (/^\d{4}-\d{2}-\d{2}$/.test(ymd)) return ymd
    }
    const t = toChartTime(r.day)
    if (t == null) return String(r.day || '').trim() || '--'
    return formatCrosshairTime(t)
  }
  
  const crosshairPanel = computed(() => {
    const mergedRawRows = getMergedRawRows()
    const r = hoverRawRow.value ?? defaultLatestRawRow.value
    if (!r) return null
    const chgPct = parseNumStr(r.changePercent)
    const sign = chgPct > 0 ? 1 : chgPct < 0 ? -1 : 0
    const neu = props.darkTheme ? '#94a3b8' : '#64748b'
    const ohlcC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
    const chgC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
    const showLatestTag = !hoverRawRow.value && defaultLatestRawRow.value
    const titleDay = formatPanelTitleDay(r)
    const curDay = String(r.day || '').replace(/\//g, '-')
    const curIdx = mergedRawRows.findIndex(x => String(x.day || '').replace(/\//g, '-') === curDay)
    const amps = []
    for (let i = 0; i <= curIdx; i++) {
      const row = mergedRawRows[i]
      const rawAmp = parseNumStr(row.amplitude)
      const o = Number(row.open), h = Number(row.high), l = Number(row.low)
      if (Number.isFinite(rawAmp)) {
        amps.push(rawAmp)
      } else if (Number.isFinite(o) && o > 0 && Number.isFinite(h) && Number.isFinite(l)) {
        amps.push((h - l) / o * 100)
      } else {
        amps.push(NaN)
      }
    }
    let amp5 = '--', amp10 = '--', amp20 = '--'
    if (curIdx >= 0) {
      const a5 = avgAmplitude(amps, 5)
      const a10 = avgAmplitude(amps, 10)
      const a20 = avgAmplitude(amps, 20)
      if (Number.isFinite(a5)) amp5 = a5.toFixed(2) + '%'
      if (Number.isFinite(a10)) amp10 = a10.toFixed(2) + '%'
      if (Number.isFinite(a20)) amp20 = a20.toFixed(2) + '%'
    }
    return {
      title: showLatestTag ? `${titleDay} · 最新` : titleDay,
      open: formatPrice2(r.open),
      close: formatPrice2(r.close),
      high: formatPrice2(r.high),
      low: formatPrice2(r.low),
      changePercent: formatPctField(r.changePercent),
      changeValue: formatSigned2(r.changeValue),
      volume: formatVolumeCn(r.volume),
      amount: formatAmountCn(r.amount),
      amplitude: formatPctField(r.amplitude),
      avgAmp5: amp5,
      avgAmp10: amp10,
      avgAmp20: amp20,
      turnoverRate: formatPctField(r.turnoverRate),
      volumeRatio: formatVolumeRatio(r.volumeRatio),
      cOpenClose: ohlcC,
      cHigh: CLR_RISE,
      cLow: CLR_FALL,
      cChg: chgC,
      cNeu: neu,
    }
  })

  return { formatCrosshairTime, findRawRowByChartTime, syncDefaultLatestPanelRow, crosshairPanel }
}
