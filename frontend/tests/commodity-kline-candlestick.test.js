/**
 * CommodityKlineChart 蜡烛图字段兼容性测试
 * 验证 barField() 辅助函数和 timeToChart() 时间格式
 */

// 模拟 Go KLineBar 返回的小写字段数据
const mockLowerCaseBars = [
  { time: '2024-01-01T00:00:00Z', open: 2000, high: 2050, low: 1990, close: 2030, volume: 100000 },
  { time: '2024-01-02T00:00:00Z', open: 2030, high: 2080, low: 2020, close: 2070, volume: 120000 },
  { time: '2024-01-03T00:00:00Z', open: 2070, high: 2090, low: 2040, close: 2050, volume: 90000 },
]

// 与 CommodityKlineChart.vue 中相同的 barField 函数
function barField(b, key) {
  if (!b) return undefined
  const lower = b[key]
  if (lower !== undefined && lower !== null) return lower
  return b[key.charAt(0).toUpperCase() + key.slice(1)]
}

// 与 CommodityKlineChart.vue 中相同的 timeToChart 函数（BusinessDay 格式）
function timeToChart(t) {
  if (!t) return null
  const d = new Date(t)
  if (Number.isNaN(d.getTime())) return null
  return {
    year: d.getUTCFullYear(),
    month: d.getUTCMonth() + 1,
    day: d.getUTCDate(),
  }
}

// 测试 1: barField 小写字段
console.log('=== 测试 1: barField 小写字段 ===')
const b = mockLowerCaseBars[0]
console.log('open:', barField(b, 'open'), '=== 2000 ?', barField(b, 'open') === 2000)
console.log('close:', barField(b, 'close'), '=== 2030 ?', barField(b, 'close') === 2030)
console.log('volume:', barField(b, 'volume'), '=== 100000 ?', barField(b, 'volume') === 100000)
console.log('PASS:', barField(b, 'open') === 2000 && barField(b, 'close') === 2030 && barField(b, 'volume') === 100000)

// 测试 2: barField 大写字段兼容
console.log('\n=== 测试 2: barField 大写字段兼容 ===')
const upperB = { Time: '2024-01-01T00:00:00Z', Open: 2000, Close: 2030, Volume: 100000 }
console.log('open:', barField(upperB, 'open'), '=== 2000 ?', barField(upperB, 'open') === 2000)
console.log('close:', barField(upperB, 'close'), '=== 2030 ?', barField(upperB, 'close') === 2030)
console.log('PASS:', barField(upperB, 'open') === 2000 && barField(upperB, 'close') === 2030)

// 测试 3: timeToChart BusinessDay 格式
console.log('\n=== 测试 3: timeToChart BusinessDay 格式 ===')
const td = timeToChart('2024-01-15T10:30:00Z')
console.log('结果:', JSON.stringify(td))
console.log('year:', td.year, '=== 2024 ?', td.year === 2024)
console.log('month:', td.month, '=== 1 ?', td.month === 1)
console.log('day:', td.day, '=== 15 ?', td.day === 15)
console.log('PASS:', td.year === 2024 && td.month === 1 && td.day === 15)

// 测试 4: 时间边界 - UTC 日期不偏移
console.log('\n=== 测试 4: UTC 日期不偏移 ===')
// 2024-01-01 23:59 UTC 应该仍然是 1月1日
const lateDate = timeToChart('2024-01-01T23:59:59Z')
console.log('23:59 UTC:', JSON.stringify(lateDate))
console.log('PASS:', lateDate.day === 1)
// 2024-01-02 00:01 UTC 应该是 1月2日
const earlyDate = timeToChart('2024-01-02T00:01:00Z')
console.log('00:01 UTC:', JSON.stringify(earlyDate))
console.log('PASS:', earlyDate.day === 2)

// 测试 5: 完整蜡烛数据处理
console.log('\n=== 测试 5: 完整蜡烛数据处理 ===')
const candles = []
const volumes = []
const CLR_RISE = '#ef5350'
const CLR_FALL = '#26a69a'
for (const bar of mockLowerCaseBars) {
  const open = Number(barField(bar, 'open'))
  const close = Number(barField(bar, 'close'))
  const high = Number(barField(bar, 'high'))
  const low = Number(barField(bar, 'low'))
  const time = timeToChart(barField(bar, 'time'))
  if (!time || !Number.isFinite(open) || !Number.isFinite(close)) continue
  candles.push({ time, open, close, high, low })
  volumes.push({
    time,
    value: Number(barField(bar, 'volume')) || 0,
    color: close >= open ? CLR_RISE + '80' : CLR_FALL + '80',
  })
}
console.log('蜡烛数量:', candles.length, '=== 3 ?', candles.length === 3)
console.log('成交量数量:', volumes.length, '=== 3 ?', volumes.length === 3)
console.log('第一根蜡烛:', JSON.stringify(candles[0]))
console.log('PASS:', candles.length === 3 && volumes.length === 3)

// 测试 6: 无效数据过滤
console.log('\n=== 测试 6: 无效数据过滤 ===')
const invalidBars = [
  { time: 'invalid-date', open: 2000, close: 2030 },
  { time: '2024-01-01T00:00:00Z', open: NaN, close: 2030 },
  { time: '2024-01-01T00:00:00Z', open: 2000, close: undefined },
]
let validCount = 0
for (const bar of invalidBars) {
  const open = Number(barField(bar, 'open'))
  const close = Number(barField(bar, 'close'))
  const time = timeToChart(barField(bar, 'time'))
  if (time && Number.isFinite(open) && Number.isFinite(close)) validCount++
}
console.log('有效数据:', validCount, '=== 0 ?', validCount === 0)
console.log('PASS:', validCount === 0)

// 汇总
console.log('\n=== 汇总 ===')
const allPass =
  barField(b, 'open') === 2000 &&
  barField(upperB, 'open') === 2000 &&
  td.year === 2024 &&
  lateDate.day === 1 &&
  candles.length === 3 &&
  validCount === 0

console.log(allPass ? '✅ 所有测试通过' : '❌ 部分测试失败')
process.exit(allPass ? 0 : 1)
