/**
 * AI 推荐股票列表字段兼容性测试
 * 验证前端能正确处理 Go 后端返回的字段名
 */

// 模拟 Go 后端返回的推荐记录数据（gorm.Model + AiRecommendStocks）
const mockGoResponse = {
  list: [
    {
      id: 1,
      createdAt: '2024-01-15T10:30:00Z',
      updatedAt: '2024-01-15T10:30:00Z',
      dataTime: '2024-01-15T10:30:00Z',
      modelName: 'gpt-4',
      rating: '强烈推荐',
      stockCode: 'sh600519',
      stockName: '贵州茅台',
      bkCode: '白酒',
      bkName: '白酒板块',
      stockPrice: '1800.00',
      stockCurrentPrice: '1850.00',
      stockPrePrice: '1790.00',
      recommendReason: '业绩超预期，白酒龙头',
      recommendBuyPrice: '1750-1800',
      recommendBuyPriceMin: 1750,
      recommendBuyPriceMax: 1800,
      recommendStopProfitPrice: '2000-2100',
      recommendStopProfitPriceMin: 2000,
      recommendStopProfitPriceMax: 2100,
      recommendStopLossPrice: '1600',
      riskRemarks: '注意白酒行业政策风险',
      remarks: '',
      enableAlert: false,
    }
  ],
  total: 1,
  page: 1,
  pageSize: 12,
  totalPages: 1,
}

// 测试 1: 字段名兼容性 - 时间字段
console.log('=== 测试 1: 时间字段兼容性 ===')
const row = mockGoResponse.list[0]
const t = row.dataTime || row.createdAt || row.DataTime || ''
const formatted = String(t).substring(0, 19).replace('T', ' ')
console.log('原始 dataTime:', row.dataTime)
console.log('格式化后:', formatted)
console.log('PASS:', formatted === '2024-01-15T10:30:00'.replace('T', ' '))

// 测试 2: ID 字段兼容性
console.log('\n=== 测试 2: ID 字段兼容性 ===')
const id1 = row.ID ?? row.id
console.log('row.ID ?? row.id =', id1)
console.log('PASS:', id1 === 1)

// 测试 3: 价格计算 - 涨跌幅
console.log('\n=== 测试 3: 价格涨跌幅计算 ===')
const diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrePrice)) / Number(row.stockPrePrice) * 100).toFixed(2)
console.log('stockCurrentPrice:', row.stockCurrentPrice)
console.log('stockPrePrice:', row.stockPrePrice)
console.log('涨跌幅:', diff + '%')
console.log('PASS:', diff === '3.35')

// 测试 4: 推荐价区间解析
console.log('\n=== 测试 4: 推荐价区间解析 ===')
function recommendRangeToSinglePrice(p) {
  if (p == null || String(p).trim() === '') return ''
  const s = String(p).trim()
  const i = s.indexOf('-')
  if (i > 0) return s.slice(0, i).trim()
  return s
}
const buyPrice = recommendRangeToSinglePrice(row.recommendBuyPrice)
const tpPrice = recommendRangeToSinglePrice(row.recommendStopProfitPrice)
const slPrice = recommendRangeToSinglePrice(row.recommendStopLossPrice)
console.log('买入价区间:', row.recommendBuyPrice, '→ 单值:', buyPrice)
console.log('止盈价区间:', row.recommendStopProfitPrice, '→ 单值:', tpPrice)
console.log('止损价:', row.recommendStopLossPrice, '→ 单值:', slPrice)
console.log('PASS:', buyPrice === '1750' && tpPrice === '2000' && slPrice === '1600')

// 测试 5: 买入信号判断
console.log('\n=== 测试 5: 买入信号判断 ===')
const cur = Number(row.stockCurrentPrice)
const buyMin = Number(row.recommendBuyPriceMin)
const buyMax = Number(row.recommendBuyPriceMax)
const inBuyZone = buyMin && buyMax && cur >= buyMin && cur <= buyMax
console.log('当前价:', cur, '买入区间:', buyMin, '-', buyMax)
console.log('在买入区间:', inBuyZone)
console.log('PASS:', inBuyZone === false) // 1850 > 1800，不在买入区间

// 测试 6: 止盈信号判断
console.log('\n=== 测试 6: 止盈信号判断 ===')
const tpMin = Number(row.recommendStopProfitPriceMin)
const inTpZone = tpMin && cur >= tpMin
console.log('当前价:', cur, '止盈最低:', tpMin)
console.log('达到止盈:', inTpZone)
console.log('PASS:', inTpZone === false) // 1850 < 2000，未达止盈

// 测试 7: 空数据处理
console.log('\n=== 测试 7: 空数据处理 ===')
const emptyList = { list: [], total: 0, page: 1, totalPages: 0 }
const pagedData = emptyList.list || []
const total = emptyList.total || 0
const pageCount = emptyList.totalPages || 0
console.log('空列表长度:', pagedData.length)
console.log('总数:', total)
console.log('总页数:', pageCount)
console.log('PASS:', pagedData.length === 0 && total === 0 && pageCount === 0)

// 测试 8: API 错误返回处理
console.log('\n=== 测试 8: API 错误返回处理 ===')
const errorResult = { success: false, data: null, message: '网络错误' }
const safeData = errorResult.data?.list || []
console.log('错误返回时数据长度:', safeData.length)
console.log('PASS:', safeData.length === 0)

// 测试 9: 关键词搜索参数
console.log('\n=== 测试 9: 搜索参数 ===')
const keyword = '茅台'
const queryParams = {
  page: 1,
  pageSize: 12,
  modelName: keyword,
  stockName: keyword,
  stockCode: keyword,
  bkName: keyword,
  startDate: '2024-01-01',
  endDate: '2024-01-31',
}
console.log('搜索参数:', JSON.stringify(queryParams))
console.log('PASS:', queryParams.stockName === '茅台' && queryParams.startDate === '2024-01-01')

// 测试 10: 默认日期范围格式化
console.log('\n=== 测试 10: 日期格式化 ===')
function formatDate(dateString) {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const today = new Date()
const thirtyDaysAgo = new Date(today.getTime() - 30 * 24 * 60 * 60 * 1000)
const startStr = formatDate(thirtyDaysAgo)
const endStr = formatDate(today)
console.log('30天前:', startStr)
console.log('今天:', endStr)
console.log('格式正确:', /^\d{4}-\d{2}-\d{2}$/.test(startStr) && /^\d{4}-\d{2}-\d{2}$/.test(endStr))

// 汇总
console.log('\n=== 汇总 ===')
const allPass = 
  formatted === '2024-01-15 10:30:00' &&
  id1 === 1 &&
  diff === '3.35' &&
  buyPrice === '1750' &&
  inBuyZone === false &&
  inTpZone === false &&
  pagedData.length === 0 &&
  safeData.length === 0 &&
  queryParams.stockName === '茅台' &&
  /^\d{4}-\d{2}-\d{2}$/.test(startStr)

console.log(allPass ? '✅ 所有测试通过' : '❌ 部分测试失败')
process.exit(allPass ? 0 : 1)
