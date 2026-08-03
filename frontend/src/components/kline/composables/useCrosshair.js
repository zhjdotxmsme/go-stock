/**
 * K线图十字光标交互 Composable
 * 处理时间格式化、行查找、面板数据计算
 */

import { computed, ref } from 'vue'
import { CN_TZ, DAILY_LIKE_KLT, CLR_RISE, CLR_FALL } from '../constants'
import { chartTimeToUtcMs, toChartTime } from '../time'
import {
  extractYmdDatePart,
  parseNumStr,
  formatPrice2,
  formatPctField,
  formatSigned2,
  formatVolumeCn,
  formatAmountCn,
} from '../format'
import { avgAmplitude, formatVolumeRatio } from './useChartLifecycle'

/**
 * 格式化十字光标时间
 * @param {number} time - 图表时间
 * @param {string} activeKlt - 当前K线类型
 * @returns {string}
 */
export function formatCrosshairTime(time, activeKlt) {
  const ms = chartTimeToUtcMs(time)
  if (!Number.isFinite(ms)) return ''
  const d = new Date(ms)
  const loc = 'zh-CN'
  const minuteLike = !DAILY_LIKE_KLT.has(activeKlt)
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

/**
 * 按图表时间查找原始数据行
 * @param {number|null} t - 图表时间
 * @param {Array} mergedRawRows - 合并后的K线数据
 * @param {Function} toChartTimeFn - 转换为图表时间函数
 * @returns {Object|null}
 */
export function findRawRowByChartTime(t, mergedRawRows, toChartTimeFn) {
  if (t === undefined || t === null) return null
  const targetMs = chartTimeToUtcMs(t)
  if (Number.isFinite(targetMs)) {
    for (const r of mergedRawRows) {
      const ct = toChartTimeFn(r.day)
      const ms = chartTimeToUtcMs(ct)
      if (Number.isFinite(ms) && ms === targetMs) return r
    }
  }
  for (const r of mergedRawRows) {
    const ct = toChartTimeFn(r.day)
    if (ct === t) return r
  }
  return null
}

/**
 * 十字光标管理
 * @param {Object} options
 * @param {Ref<Array>} options.mergedRawRows - 合并的K线数据
 * @param {Ref<string>} options.activeKlt - 当前K线类型
 * @param {Ref<boolean>} options.darkTheme - 深色主题
 * @returns {Object}
 */
export function useCrosshair({ mergedRawRows, activeKlt, darkTheme }) {
  // 悬停的原始数据行
  const hoverRawRow = ref(null)
  // 默认最新数据行
  const defaultLatestRawRow = ref(null)

  /**
   * 同步默认最新面板行（mergedRawRows 按时间升序，取最后一根）
   */
  function syncDefaultLatestPanelRow() {
    const rows = mergedRawRows.value
    if (!rows.length) {
      defaultLatestRawRow.value = null
      return
    }
    defaultLatestRawRow.value = rows[rows.length - 1]
  }

  /**
   * 格式化面板标题日期
   * @param {Object} r - K线数据行
   * @returns {string}
   */
  function formatPanelTitleDay(r) {
    const dailyLike = DAILY_LIKE_KLT.has(activeKlt.value)
    if (dailyLike) {
      const ymd = extractYmdDatePart(String(r.day || '').replace(/\//g, '-'))
      if (/^\d{4}-\d{2}-\d{2}$/.test(ymd)) return ymd
    }
    const t = toChartTime(r.day)
    if (t == null) return String(r.day || '').trim() || '--'
    return formatCrosshairTime(t, activeKlt.value)
  }

  /**
   * 十字光标面板数据（计算属性）
   */
  const crosshairPanel = computed(() => {
    const r = hoverRawRow.value ?? defaultLatestRawRow.value
    if (!r) return null

    const chgPct = parseNumStr(r.changePercent)
    const sign = chgPct > 0 ? 1 : chgPct < 0 ? -1 : 0
    const neu = darkTheme.value ? '#94a3b8' : '#64748b'
    const ohlcC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
    const chgC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
    const showLatestTag = !hoverRawRow.value && defaultLatestRawRow.value
    const titleDay = formatPanelTitleDay(r)
    const curDay = String(r.day || '').replace(/\//g, '-')
    const curIdx = mergedRawRows.value.findIndex(x => String(x.day || '').replace(/\//g, '-') === curDay)

    // 计算振幅序列
    const amps = []
    for (let i = 0; i <= curIdx; i++) {
      const row = mergedRawRows.value[i]
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

    // 计算平均振幅
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

  return {
    // 状态
    hoverRawRow,
    defaultLatestRawRow,

    // 计算属性
    crosshairPanel,

    // 方法
    formatCrosshairTime,
    findRawRowByChartTime,
    syncDefaultLatestPanelRow,
    formatPanelTitleDay,
  }
}

export default useCrosshair
