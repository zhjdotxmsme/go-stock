// 验证 K 线时间轴归一化（toChartTime）与刻度文案（formatTickTime）。
//
// 修复背景：东财年K 的 day 字段只有 4 位年份（"2025"），原实现三个正则都不匹配，
// 返回 null；而 useIndicatorSync.extractOHLCV 对 null 直接 `if (t === null) continue`
// 整行丢弃 —— 年K 图上一根柱子都没有、x 轴也取不到年份。同样的问题还存在于
// 紧凑 8 位日期（YYYYMMDD）与 "YYYY-MM" 周期形。
//
// 运行：node --import ./tests/register-ts-ext.mjs tests/check-kline-time.mjs
import { toChartTime, formatTickTime } from '../src/components/kline/time.ts'
// TickMarkType 是运行时枚举（chart 的 tickMarkFormatter 回调拿到的就是它），
// 直接从上游包导入，避免经由 time.ts 二次 re-export。
import { TickMarkType } from 'lightweight-charts'

const results = []
const assert = (name, actual, expected) => {
  const ok = actual === expected
  results.push(ok)
  console.log(`${ok ? '✓' : '✗'} ${name}${ok ? '' : `: expected ${expected}, got ${actual}`}`)
}
const zhYear = (s) => `${s}年`
const zhMonth = (ym) => `${ym.slice(0, 4)}年${Number(ym.slice(5, 7))}月`
const zhDay = (md) => `${md.slice(0, 2)}/${md.slice(2, 4)}`

// ---- 年K：只给 4 位年份，必须能解析出「该年」的时间戳 ----
for (const y of [1990, 2000, 2010, 2020, 2024, 2025, 2026]) {
  const sec = toChartTime(String(y))
  assert(`年K ${y} 可解析`, sec !== null, true)
  assert(`年K ${y} 刻度年份`, formatTickTime(sec, TickMarkType.Year), zhYear(String(y)))
  assert(`年K ${y} 刻度月份`, formatTickTime(sec, TickMarkType.Month), zhMonth(`${y}-01`))
}

// ---- 月K 的紧凑周期形 "YYYY-MM"（原本同样返回 null）----
for (const ym of ['1990-01', '2025-09', '2026-01', '2026-12']) {
  const sec = toChartTime(ym)
  assert(`${ym} 可解析`, sec !== null, true)
  assert(`${ym} 刻度月份`, formatTickTime(sec, TickMarkType.Month), zhMonth(ym))
}

// ---- 紧凑 8 位日期 YYYYMMDD（原本同样返回 null）----
for (const ds of ['19900102', '20250916', '20260915', '20260217']) {
  const sec = toChartTime(ds)
  assert(`${ds} 可解析`, sec !== null, true)
  assert(`${ds} 刻度日`, formatTickTime(sec, TickMarkType.DayOfMonth), zhDay(ds.slice(4)))
}

// ---- 回归：原有格式必须不变 ----
assert('纯日期 YYYY-MM-DD', formatTickTime(toChartTime('2026-09-15'), TickMarkType.DayOfMonth), zhDay('0915'))
assert('14 位时间戳 150000', formatTickTime(toChartTime('20260915150000'), TickMarkType.DayOfMonth), zhDay('0915'))
assert('14 位时间戳 155959', formatTickTime(toChartTime('20260915155959'), TickMarkType.DayOfMonth), zhDay('0915'))
assert('12 位时间戳 150000', formatTickTime(toChartTime('20260915150000'), TickMarkType.DayOfMonth), zhDay('0915'))
assert('12 位时间戳 155959', formatTickTime(toChartTime('20260915155959'), TickMarkType.DayOfMonth), zhDay('0915'))
assert('ISO 带时区', formatTickTime(toChartTime('2026-09-15 15:59:59+08:00'), TickMarkType.DayOfMonth), zhDay('0915'))
assert('ISO UTC Z', formatTickTime(toChartTime('2026-09-15T07:00:00Z'), TickMarkType.DayOfMonth), zhDay('0915'))

// ---- 无效输入 ----
assert('空字符串', toChartTime(''), null)
assert('undefined', toChartTime(undefined), null)
assert('垃圾字符串', toChartTime('abcdef'), null)
assert('非法月份', toChartTime('20251340'), null)
assert('5 位数字不算年份', toChartTime('20250'), null)

const failed = results.filter((x) => !x).length
console.log(failed ? `\n${failed} 项失败` : '\n全部通过')
process.exitCode = failed ? 1 : 0
