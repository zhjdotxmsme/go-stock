/**
 * 大宗商品 K 线字段兼容性测试
 * 验证 getField() 辅助函数能正确处理大小写两种字段名格式
 */

// 模拟 Go KLineBar 返回的小写字段数据（实际生产格式）
const mockLowerCaseBars = [
  { time: '2024-01-01T00:00:00Z', open: 2000, high: 2050, low: 1990, close: 2030, volume: 100000 },
  { time: '2024-01-02T00:00:00Z', open: 2030, high: 2080, low: 2020, close: 2070, volume: 120000 },
  { time: '2024-01-03T00:00:00Z', open: 2070, high: 2090, low: 2040, close: 2050, volume: 90000 },
]

// 模拟大写字段数据（旧格式/其他数据源）
const mockUpperCaseBars = [
  { Time: '2024-01-01T00:00:00Z', Open: 2000, High: 2050, Low: 1990, Close: 2030, Volume: 100000 },
  { Time: '2024-01-02T00:00:00Z', Open: 2030, High: 2080, Low: 2020, Close: 2070, Volume: 120000 },
  { Time: '2024-01-03T00:00:00Z', Open: 2070, High: 2090, Low: 2040, Close: 2050, Volume: 90000 },
]

// 与 CommodityPriceChart.vue 中相同的 getField 函数
function getField(b, key) {
  if (b[key] !== undefined && b[key] !== null) return b[key]
  const capKey = key.charAt(0).toUpperCase() + key.slice(1)
  return b[capKey]
}

// 与 CommodityPriceChart.vue 中相同的数据处理逻辑
function processBars(bars) {
  const lineData = []
  const volData = []
  for (let i = 0; i < bars.length; i++) {
    const b = bars[i]
    const timeMs = new Date(getField(b, 'time')).getTime()
    if (!Number.isFinite(timeMs)) continue
    const close = Number(getField(b, 'close'))
    if (!Number.isFinite(close)) continue
    const t = Math.floor(timeMs / 1000)
    lineData.push({ time: t, value: close })
    const prevClose = i > 0 ? Number(getField(bars[i - 1], 'close')) : close
    volData.push({
      time: t,
      value: Number(getField(b, 'volume')) || 0,
      color: close >= prevClose
        ? 'rgba(239,83,80,0.5)'
        : 'rgba(38,166,154,0.5)',
    })
  }
  return { lineData, volData }
}

// 与 CommodityPriceChart.vue 中相同的精度推断函数
function inferPricePrecision(bars) {
  if (!bars || bars.length === 0) return { precision: 2, minMove: 0.01 }
  const firstClose = Number(bars[0]?.close ?? bars[0]?.Close ?? 0)
  if (firstClose < 1) return { precision: 4, minMove: 0.0001 }
  if (firstClose < 10) return { precision: 3, minMove: 0.001 }
  return { precision: 2, minMove: 0.01 }
}

// 测试 1: 小写字段（Go 默认格式）
console.log('=== 测试 1: 小写字段（Go 默认 JSON 格式）===')
const result1 = processBars(mockLowerCaseBars)
console.log('lineData 数量:', result1.lineData.length)
console.log('第一条数据:', result1.lineData[0])
console.log('volData 数量:', result1.volData.length)
console.log('PASS:', result1.lineData.length === 3 && result1.volData.length === 3)

// 测试 2: 大写字段（旧格式兼容）
console.log('\n=== 测试 2: 大写字段（兼容旧格式）===')
const result2 = processBars(mockUpperCaseBars)
console.log('lineData 数量:', result2.lineData.length)
console.log('第一条数据:', result2.lineData[0])
console.log('PASS:', result2.lineData.length === 3 && result2.volData.length === 3)

// 测试 3: 两种格式结果一致
console.log('\n=== 测试 3: 两种格式结果一致 ===')
const matchTime = result1.lineData[0].time === result2.lineData[0].time
const matchValue = result1.lineData[0].value === result2.lineData[0].value
const matchVol = result1.volData[0].value === result2.volData[0].value
console.log('时间一致:', matchTime)
console.log('价格一致:', matchValue)
console.log('成交量一致:', matchVol)
console.log('PASS:', matchTime && matchValue && matchVol)

// 测试 4: 价格精度推断
console.log('\n=== 测试 4: 价格精度推断 ===')
const goldBars = [{ close: 2030.50 }]  // 黄金价格
const silverBars = [{ close: 23.45 }]  // 白银价格
const cryptoBars = [{ close: 0.0034 }] // 低价资产
console.log('黄金 (2030.50):', inferPricePrecision(goldBars))
console.log('白银 (23.45):', inferPricePrecision(silverBars))
console.log('低价 (0.0034):', inferPricePrecision(cryptoBars))
console.log('PASS:', 
  inferPricePrecision(goldBars).precision === 2 &&
  inferPricePrecision(silverBars).precision === 2 &&
  inferPricePrecision(cryptoBars).precision === 4
)

// 测试 5: 边界情况 - 空数据
console.log('\n=== 测试 5: 边界情况 ===')
const emptyResult = processBars([])
const invalidResult = processBars([{ time: 'invalid', close: NaN }])
console.log('空数组 lineData:', emptyResult.lineData.length)
console.log('无效数据 lineData:', invalidResult.lineData.length)
console.log('PASS:', emptyResult.lineData.length === 0 && invalidResult.lineData.length === 0)

// 汇总
console.log('\n=== 汇总 ===')
const allPass = 
  result1.lineData.length === 3 &&
  result2.lineData.length === 3 &&
  matchTime && matchValue && matchVol &&
  inferPricePrecision(goldBars).precision === 2 &&
  emptyResult.lineData.length === 0 &&
  invalidResult.lineData.length === 0

console.log(allPass ? '✅ 所有测试通过' : '❌ 部分测试失败')
process.exit(allPass ? 0 : 1)
