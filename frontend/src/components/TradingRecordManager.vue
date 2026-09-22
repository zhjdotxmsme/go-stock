<script setup>
import { h, onMounted, onUnmounted, ref, reactive } from 'vue'
import * as tradeApi from '../api/trade'
import * as stockApi from '../api/stock'
import * as systemApi from '../api/system'
import * as fundApi from '../api/fund'
import {
  NButton,
  NDataTable,
  NDatePicker,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NModal,
  NNumberAnimation,
  NSelect,
  NSpace,
  NStatistic,
  NTag,
  NText,
  NTooltip,
  NAutoComplete,
  useMessage,
  useNotification
} from 'naive-ui'
import sparkLine from "./stockSparkLine.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import StockIndicatorsModal from "./trading/StockIndicatorsModal.vue";
import HoldingsAiSummaryModal from "./trading/HoldingsAiSummaryModal.vue";
import HoldingsSummaryHistoryModal from "./trading/HoldingsSummaryHistoryModal.vue";
import HoldingsDeepAnalysisModal from "./trading/HoldingsDeepAnalysisModal.vue";
import TradeAiCommentModal from "./trading/TradeAiCommentModal.vue";
import { normalizeStockCode } from '../utils/stockCode'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'

const message = useMessage()
const notify = useNotification()

const showKlineModal = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
const longStopLossPrice = ref(0)
const longTakeProfitPrice = ref(0)
const costPrice = ref(0)
const darkTheme = ref(false)

const showIndicatorsModal = ref(false)
const indicatorsCode = ref('')
const indicatorsName = ref('')
const showAiSummaryModal = ref(false)
const showSummaryHistory = ref(false)
const showAiCommentModal = ref(false)
const aiCommentRecord = ref(null)
const showDeepAnalysisModal = ref(false)

const dataRef = ref([])
const loadingRef = ref(true)
const statisticsRef = ref(null)
const refreshTimer = ref(null)

const showAddModal = ref(false)
const showEditModal = ref(false)

const formData = reactive({
  ID: 0,
  StockCode: '',
  StockName: '',
  Direction: '买入',
  Price: 0,
  Volume: 0,
  Amount: 0,
  TradingTime: Date.now(),
  Reason: '',
  StopLossPrice: 0,
  TakeProfitPrice: 0,
  Fee: 0,
  MarketValue: 0,
  Mindset: '',
})

const directionOptions = [
  { label: '全部', value: '' },
  { label: '买入', value: '买入' },
  { label: '卖出', value: '卖出' }
]

// ---- 股票模糊搜索（与「关注股票」同一数据源）：
// stock_basic + 指数 + 港股 + 美股 + 全量快照聚合而来，代码或名称都能搜到；
// 选中候选后自动带出代码与名称。旧实现依赖 all_stock_info 全量快照表，
// 未跑过快照同步时该表为空，下拉永远没有候选，等于「搜不出股票」。 ----
const stockList = ref([])          // 后端聚合的可搜索股票全集
const stockSearchText = ref('')    // 搜索输入框文本
const stockSearchOptions = ref([]) // 下拉候选

function loadStockList() {
  stockApi.getStockList('').then(({data: result}) => {
    stockList.value = Array.isArray(result) ? result : []
  }).catch(err => {
    console.error('加载股票列表失败:', err)
  })
}

function toStockOption(item) {
  return {
    label: `${item.name} - ${item.ts_code}`,
    value: item.ts_code,
    name: item.name,
  }
}

// ---- 场内基金候选（ETF/LOF，可在交易所买卖，行情与盈亏按股票链路计算）----
const fundCandidates = ref([]) // 最近一次基金搜索结果
let fundSearchTimer = null
let fundSearchSeq = 0

/** 场内基金代码：50/51/52/56/58 沪市，15/16/18 深市 */
function isExchangeFundCode(code) {
  return /^(50|51|52|56|58|15|16|18)\d{4}$/.test(String(code || ''))
}

function toFundOption(item) {
  return {
    label: `【基金】${item.name} - ${item.code}${item.type ? `（${item.type}）` : ''}`,
    value: 'fund:' + item.code,
    name: item.name,
    isFund: true,
  }
}

/** 客户端模糊匹配：名称或代码包含关键词即命中（与自选股搜索逻辑一致，避免逐键请求后端） */
function filterStockOptions(value) {
  const q = String(value || '').trim().toLowerCase()
  if (!q) {
    stockSearchOptions.value = []
    fundCandidates.value = []
    return
  }
  const stockOpts = stockList.value
    .filter(item =>
      (item.name && String(item.name).toLowerCase().includes(q)) ||
      (item.ts_code && String(item.ts_code).toLowerCase().includes(q))
    )
    .slice(0, 50)
    .map(toStockOption)
  stockSearchOptions.value = stockOpts

  // 基金搜索（东财接口，防抖避免逐键请求）
  if (fundSearchTimer) clearTimeout(fundSearchTimer)
  const seq = ++fundSearchSeq
  fundSearchTimer = setTimeout(() => {
    fundApi.searchFundCodes(q).then(({data: items}) => {
      if (seq !== fundSearchSeq) return
      const list = (items || []).filter(item => isExchangeFundCode(item.code))
      fundCandidates.value = list
      if (list.length > 0) {
        stockSearchOptions.value = stockOpts.concat(list.slice(0, 8).map(toFundOption))
      }
    }).catch((err) => {
      console.error('搜索基金失败:', err)
    })
  }, 350)
}

/** 选中候选：带出代码与名称，并自动拉取实时价格填入「价格」；
 * 场内基金候选带 fund: 前缀，代码存裸 6 位（后端行情/收盘快照按场内基金规则补市场前缀） */
// naive-ui NAutoComplete 的 @select 载荷就是候选的 value（interface.d.ts:
// OnSelect = (value: string) => void），此处 value 即候选的 value。
function handleStockSelect(value) {
  if (String(value).startsWith('fund:')) {
    const code = String(value).slice(5)
    const hit = fundCandidates.value.find(item => String(item.code) === code)
    if (!hit) return
    formData.StockCode = code
    formData.StockName = hit.name || ''
    stockSearchText.value = formData.StockName ? `${formData.StockName} - ${code}` : code
    fetchStockPrice(code)
    fetchAiAdvice(code, formData.StockName)
    return
  }
  const hit = stockList.value.find(item => item.ts_code === value)
  if (!hit) return
  formData.StockCode = normalizeStockCode(hit.ts_code)
  formData.StockName = hit.name || ''
  stockSearchText.value = hit.name ? `${hit.name} - ${hit.ts_code}` : hit.ts_code
  fetchStockPrice(formData.StockCode)
  fetchAiAdvice(formData.StockCode, formData.StockName)
}

/** 把最近一次 AI 推荐的止损/止盈带入表单（仅当前为空时带入，不覆盖手填值） */
function applyAiAdvice(advice) {
  let applied = false
  if (!Number(formData.StopLossPrice) && advice.stopLossPrice > 0) {
    formData.StopLossPrice = Number(advice.stopLossPrice.toFixed(2))
    applied = true
  }
  if (!Number(formData.TakeProfitPrice) && advice.takeProfitPrice > 0) {
    formData.TakeProfitPrice = Number(advice.takeProfitPrice.toFixed(2))
    applied = true
  }
  return applied
}

function adviceSummaryText(advice) {
  const parts = []
  if (advice.stopLossPrice > 0) parts.push('止损 ' + advice.stopLossPrice)
  if (advice.takeProfitMin > 0 && advice.takeProfitMax > advice.takeProfitMin) {
    parts.push(`止盈区间 ${advice.takeProfitMin} ~ ${advice.takeProfitMax}（已带入下限）`)
  } else if (advice.takeProfitMin > 0) {
    parts.push('止盈 ' + advice.takeProfitMin)
  }
  return parts.join('，')
}

/** 查询该股最近一次 AI 推荐建议，自动带入止损/止盈 */
function fetchAiAdvice(stockCode, stockName) {
  tradeApi.getAiAdviceForStock(stockCode, stockName).then(({data: advice}) => {
    if (!advice) {
      message.info('该股暂无 AI 推荐记录，止损/止盈价需手动填写（或点击「AI建议」由 AI 直接给出）')
      return
    }
    if (applyAiAdvice(advice)) {
      const summary = adviceSummaryText(advice)
      message.success(`已带入AI建议（${advice.dataTime || ''}）${summary ? '：' + summary : ''}`)
    }
  }).catch((err) => {
    console.error('获取AI建议失败:', err)
  })
}

// ---- AI 一键建议止损/止盈（实时调 AI，覆盖表单现值） ----
const aiSuggesting = ref(false)
const formAiConfigs = ref([])
const formAiConfigId = ref(null)

function loadFormAiConfigs() {
  if (formAiConfigs.value.length > 0) return Promise.resolve()
  return systemApi.getAiConfigs().then(({data: res}) => {
    formAiConfigs.value = res || []
    if (formAiConfigs.value.length > 0) {
      formAiConfigId.value = formAiConfigId.value || formAiConfigs.value[0].ID
    }
  }).catch((e) => console.error('获取AI配置失败:', e))
}

/** 点击「AI建议」：AI 基于最新技术指标给出止损/止盈并覆盖填入表单 */
function aiSuggestForForm() {
  if (!formData.StockCode) {
    message.warning('请先选择股票')
    return
  }
  if (aiSuggesting.value) return
  aiSuggesting.value = true
  loadFormAiConfigs().then(() => {
    if (!formAiConfigId.value) {
      message.warning('请先在系统设置中添加AI模型服务配置')
      aiSuggesting.value = false
      return
    }
    return tradeApi.aiSuggestPriceLevels(formData.StockCode, formData.StockName, formAiConfigId.value)
      .then(({data: advice}) => {
        if (!advice || (!(advice.stopLossPrice > 0) && !(advice.takeProfitPrice > 0))) {
          message.warning('AI 未给出有效价位建议，请稍后重试或手动填写')
          return
        }
        if (advice.stopLossPrice > 0) formData.StopLossPrice = Number(advice.stopLossPrice.toFixed(2))
        if (advice.takeProfitPrice > 0) formData.TakeProfitPrice = Number(advice.takeProfitPrice.toFixed(2))
        const summary = adviceSummaryText(advice)
        message.success(`AI建议已填入${summary ? '：' + summary : ''}${advice.reason ? '（' + advice.reason + '）' : ''}`)
      })
  }).catch((err) => {
    console.error('AI建议失败:', err)
    message.error(err?.message || 'AI建议失败，请检查AI模型服务配置')
  }).finally(() => {
    aiSuggesting.value = false
  })
}

function fetchStockPrice(stockCode) {
  const code = normalizeStockCode(stockCode)
  if (!code) return
  tradeApi.getStockRealTimePrice(code).then(({data: res}) => {
    if (res && res.code === 0 && res.price > 0) {
      formData.Price = res.price
    }
  }).catch(err => {
    console.error('获取股票价格失败:', err)
  })
}

/** 打开弹窗时把已有代码/名称回填到搜索框，便于对照或重新搜索 */
function syncStockSearchTextFromForm() {
  const code = String(formData.StockCode || '').trim()
  const name = String(formData.StockName || '').trim()
  stockSearchText.value = code ? (name ? `${name} - ${code}` : code) : ''
  stockSearchOptions.value = []
}

/** 提交前统一代码格式（sh600519 / sz000001 / hk00700 / usAAPL），并按代码补齐缺失的名称 */
function normalizeFormStock() {
  formData.StockCode = normalizeStockCode(formData.StockCode)
  if (!formData.StockName && formData.StockCode) {
    const hit = stockList.value.find(item =>
      item.ts_code && normalizeStockCode(item.ts_code) === formData.StockCode
    )
    if (hit) formData.StockName = hit.name || ''
  }
}

/** 当前自然月 [月初 0 点, 月末当日]（供日期区间选择与 formatDate 查询） */
function currentMonthDateRange() {
  const now = new Date()
  // return [
  //   new Date(now.getFullYear(), now.getMonth(), 1),
  //   new Date(now.getFullYear(), now.getMonth() + 1, 0)
  // ]
  return null
}

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: '',
  direction: '',
  range: currentMonthDateRange(),
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

function formatDate(dateVal) {
  const date = new Date(dateVal)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatAmount(n) {
  return Number(n).toFixed(2)
}

function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim().toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.HK')) return 'hk' + c.slice(0, -3).toLowerCase()
  // 场内基金：50/51/52/56/58 沪市，15/16/18 深市
  if (/^(50|51|52|56|58)/.test(c)) return 'sh' + c.toLowerCase()
  if (/^(15|16|18)/.test(c)) return 'sz' + c.toLowerCase()
  // 不带后缀的代码，根据规则添加前缀
  if (c.startsWith('6')) return 'sh' + c.toLowerCase()
  if (c.startsWith('0') || c.startsWith('3')) return 'sz' + c.toLowerCase()
  if (c.startsWith('8') || c.startsWith('9')) return 'bj' + c.toLowerCase()
  return c.toLowerCase()
}

function openKlineChart(row) {
  klineStockCode.value = toEastMoneyCode(row.StockCode)
  klineStockName.value = row.StockName || ''
  showKlineModal.value = true
  longStopLossPrice.value = row.StopLossPrice || 0
  longTakeProfitPrice.value = row.TakeProfitPrice || 0
  costPrice.value = row.Price || 0
}

function openIndicators(row) {
  indicatorsCode.value = toEastMoneyCode(row.StockCode)
  indicatorsName.value = row.StockName || ''
  showIndicatorsModal.value = true
}



function formatRowTradingTime(row) {
  const t = row.TradingTime
  if (t == null || t === '') return '-'
  // 后端 TradingTime 为本地时区 time.Time，Wails 序列化为带偏移的 RFC3339
  // （如 2026-09-21T10:30:00+08:00），直接 new Date 即可得到正确瞬间；
  // 旧实现截断后拼 " UTC" 再加双重偏移补偿，UTC+8 下显示会偏一天
  const date = new Date(t)
  if (isNaN(date.getTime())) return String(t)
  const pad = (n) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

/** 统一列表行字段（Wails/JSON 可能为 PascalCase），供表格渲染与刷新使用 */
function normalizeTradingRecordRow(row) {
  if (!row || typeof row !== 'object') return row
  const closePrice = Number(row.closePrice ?? row.ClosePrice ?? 0)
  const profitAmount = Number(row.profitAmount ?? row.ProfitAmount ?? 0)
  const profitPercent = Number(row.profitPercent ?? row.ProfitPercent ?? 0)
  const aiComment = String(row.AiComment ?? row.aiComment ?? '')
  return { ...row, closePrice, profitAmount, profitPercent, aiComment }
}

function query({ page, pageSize = 12, keyword = '', direction = '', startDate = '', endDate = '' }) {
  return new Promise((resolve, reject) => {
    tradeApi.getTradingRecordList({
      page,
      pageSize,
      keyword,
      direction,
      startDate,
      endDate
    })
      .then(({data: res, error}) => {
        if (!res) {
          throw new Error(error?.message || '查询交易日志失败')
        }
        const raw = res.list ?? []
        const list = raw.map(normalizeTradingRecordRow)
        const total = res.total ?? 0
        const pageCount = res.totalPages ?? 1
        resolve({
          pageCount,
          data: list,
          total,
          page
        })
      })
      .catch(reject)
  })
}

let silentRefreshFailing = false

/** 定时静默刷新当前列表与统计，不占用 loadingRef，避免与上次请求重叠时整页停更；
 * 连续失败期间只提示一次，恢复成功后自动重置 */
function silentRefreshCurrentPage() {
  query({
    page: paginationReactive.page,
    pageSize: paginationReactive.pageSize,
    keyword: paginationReactive.keyword,
    direction: paginationReactive.direction,
    startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
    endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
  })
    .then((data) => {
      silentRefreshFailing = false
      dataRef.value = data.data
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
    })
    .catch((e) => {
      if (!silentRefreshFailing) {
        silentRefreshFailing = true
        message.warning(e?.message || '交易日志自动刷新失败，将自动重试')
      }
    })
  fetchStatistics()
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      keyword: paginationReactive.keyword,
      direction: paginationReactive.direction,
      startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
      endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
    })
      .then((data) => {
        dataRef.value = data.data
        paginationReactive.page = currentPage
        paginationReactive.pageCount = data.pageCount
        paginationReactive.itemCount = data.total
        loadingRef.value = false
      })
      .catch((e) => {
        message.error(e?.message || '加载交易日志失败')
        loadingRef.value = false
      })
  }
}

function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: 1,
      pageSize: paginationReactive.pageSize,
      keyword: paginationReactive.keyword,
      direction: paginationReactive.direction,
      startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
      endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
    })
      .then((data) => {
        dataRef.value = data.data
        paginationReactive.page = 1
        paginationReactive.pageCount = data.pageCount
        paginationReactive.itemCount = data.total
        loadingRef.value = false
      })
      .catch((e) => {
        message.error(e?.message || '加载交易日志失败')
        loadingRef.value = false
      })
  }
  fetchStatistics()
}

function fetchStatistics() {
  tradeApi.getTradingRecordStatistics()
    .then(({data: res}) => {
      console.log('统计数据返回:', res)
      if (res) {
        statisticsRef.value = res
      }
    })
    .catch((e) => {
      console.error('获取统计数据失败:', e)
    })
}

function resetFilter() {
  paginationReactive.keyword = ''
  paginationReactive.direction = ''
  paginationReactive.range = currentMonthDateRange()
  handleSearch()
}

function openAddModal() {
  Object.assign(formData, {
    ID: 0,
    StockCode: '',
    StockName: '',
    Direction: '买入',
    Price: 0,
    Volume: 0,
    Amount: 0,
    TradingTime: Date.now(),
    Reason: '',
    StopLossPrice: 0,
    TakeProfitPrice: 0,
    Fee: 0,
    MarketValue: 0,
    Mindset: '',
  })
  stockSearchText.value = ''
  stockSearchOptions.value = []
  loadStockList()
  showAddModal.value = true
}

function openEditModal(row) {
  Object.assign(formData, row)
  formData.TradingTime = new Date(row.TradingTime).getTime()
  syncStockSearchTextFromForm()
  loadStockList()
  // 止损/止盈为空时尝试带入最近一次 AI 推荐建议
  if (!Number(formData.StopLossPrice) || !Number(formData.TakeProfitPrice)) {
    fetchAiAdvice(formData.StockCode, formData.StockName)
  }
  showEditModal.value = true
}

/** 打开单笔 AI 点评弹窗 */
function openAiComment(row) {
  aiCommentRecord.value = row
  showAiCommentModal.value = true
}

function handleAdd() {
  normalizeFormStock()
  if (!formData.StockCode) {
    message.warning('请先在「股票」中搜索代码或名称并选择，带出代码与名称后再添加')
    return
  }
  const run = () => {
    formData.Amount = formData.Price * formData.Volume
    tradeApi.addTradingRecord({
      ...formData,
      TradingTime: new Date(formData.TradingTime)
    })
      .then(() => {
        message.success('添加交易日志成功')
        showAddModal.value = false
        handleSearch()
      })
      .catch((e) => {
        message.error(e?.message || '添加交易日志失败')
      })
  }

  if (formData.Direction === '买入' && formData.StockCode) {
    tradeApi.checkFrequentTrading(formData.StockCode)
      .then(({data: res}) => {
        console.log('检查频繁交易结果:', res)
        const canTrade = res.canTrade
        const msg = res.msg
        if (!canTrade) {
          message.warning(msg)
          return
        }
        run()
      })
      .catch((e) => {
        console.error('检查频繁交易失败:', e)
        run()
      })
  } else {
    run()
  }
}

function handleUpdate() {
  normalizeFormStock()
  if (!formData.StockCode) {
    message.warning('请先在「股票」中搜索代码或名称并选择，带出代码与名称后再保存')
    return
  }
  formData.Amount = formData.Price * formData.Volume
  tradeApi.updateTradingRecord({
    ...formData,
    TradingTime: new Date(formData.TradingTime)
  })
    .then(() => {
      message.success('更新交易日志成功')
      showEditModal.value = false
      handleSearch()
    })
    .catch((e) => {
      message.error(e?.message || '更新交易日志失败')
    })
}

function deleteTradingRecord(id) {
  tradeApi.deleteTradingRecord(id)
    .then(() => {
      notify.info({ content: '删除成功', duration: 2000 })
      handleSearch()
    })
    .catch((e) => {
      message.error(e?.message || '删除交易日志失败')
    })
}

const columnsRef = ref([
  {
    title: '股票代码',
    key: 'StockCode',
    render(row) {
      return h(NText, { type: 'info' }, { default: () => row.StockCode })
    }
  },
  {
    title: '股票名称',
    key: 'StockName',
    render(row) {
      return h(NText, { type: 'info' }, { default: () => row.StockName })
    }
  },
  {
    title: '方向',
    key: 'Direction',
    width: 80,
    render(row) {
      return h(
        NTag,
        { type: row.Direction === '买入' ? 'error' : 'success', size: 'small', round: true, bordered: false },
        { default: () => row.Direction }
      )
    }
  },
  {
    title: '价格',
    key: 'Price',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => formatAmount(row.Price) })
    }
  },
  {
    title: '数量',
    key: 'Volume',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => String(row.Volume) })
    }
  },
  {
    title: '金额',
    key: 'Amount',
    width: 120,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => formatAmount(row.Amount) })
    }
  },
  {
    title: '收盘/最新价',
    key: 'closePrice',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => formatAmount(row.closePrice) })
    }
  },
  {
    title: '盈亏额',
    key: 'profitAmount',
    width: 100,
    render(row) {
      const color = row.profitAmount > 0 ? 'error' : row.profitAmount < 0 ? 'success' : 'info'
      return h(NText, { type: color }, { default: () => formatAmount(row.profitAmount) })
    }
  },
  {
    title: '收益率',
    key: 'profitPercent',
    width: 100,
    render(row) {
      const color = row.profitPercent > 0 ? 'error' : row.profitPercent < 0 ? 'success' : 'info'
      const prefix = row.profitPercent > 0 ? '+' : ''
      return h(NText, { type: color }, { default: () => prefix + row.profitPercent?.toFixed(2) + '%' })
    }
  },
  {
    title: '时间',
    key: 'TradingTime',
    width: 180,
    render(row) {
      return formatRowTradingTime(row)
    }
  },
  {
    title: '止损价',
    key: 'StopLossPrice',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, {
        default: () => (row.StopLossPrice > 0 ? formatAmount(row.StopLossPrice) : '-')
      })
    }
  },
  {
    title: '止盈价',
    key: 'TakeProfitPrice',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, {
        default: () => (row.TakeProfitPrice > 0 ? formatAmount(row.TakeProfitPrice) : '-')
      })
    }
  },
  {
    title: 'AI点评',
    key: 'aiComment',
    width: 180,
    ellipsis: { tooltip: true },
    render(row) {
      if (!row.aiComment) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      return h(NText, { type: 'info' }, { default: () => row.aiComment.replace(/[#*`|\->]/g, ' ').replace(/\s+/g, ' ').trim() })
    }
  },
  {
    title: '买入信号',
    key: 'signalSnapshot',
    width: 170,
    render(row) {
      const raw = row.SignalSnapshot ?? row.signalSnapshot
      if (!raw) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      let arr = []
      try {
        arr = JSON.parse(raw)
      } catch (e) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      if (!Array.isArray(arr) || !arr.length) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      const tagType = (d) => ({ bullish: 'error', bearish: 'success', warning: 'warning' })[d] || 'info'
      const tags = arr.slice(0, 3).map((sig) =>
        h(NTooltip, { trigger: 'hover' }, {
          trigger: () => h(NTag, {
            size: 'small', bordered: false, type: tagType(sig.direction),
            style: 'margin-right: 2px; cursor: help'
          }, { default: () => sig.name }),
          default: () => `${sig.category}｜${sig.tip}`
        })
      )
      if (arr.length > 3) {
        tags.push(h(NText, { depth: 3 }, { default: () => `+${arr.length - 3}` }))
      }
      return tags
    }
  },
  {
    title: '交易理由',
    key: 'Reason',
    ellipsis: { tooltip: true }
  },
  {
    title: '操作',
    width: 320,
    render(row) {
      return [
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'success',
            onClick: () => openIndicators(row)
          },
          { default: () => '技术指标' }
        ),
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'info',
            onClick: () => openKlineChart(row)
          },
          { default: () => 'K线' }
        ),
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'primary',
            onClick: () => openAiComment(row)
          },
          { default: () => 'AI点评' }
        ),
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'warning',
            onClick: () => openEditModal(row)
          },
          { default: () => '编辑' }
        ),
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'error',
            onClick: () => deleteTradingRecord(row.ID)
          },
          { default: () => '删除' }
        )
      ]
    }
  }
])

onMounted(() => {
  // 获取主题配置
  systemApi.getConfig().then(({data: result}) => {
    if (result.darkTheme) {
      darkTheme.value = true
    }
  })

  loadingRef.value = true
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    keyword: paginationReactive.keyword,
    direction: paginationReactive.direction,
    startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
    endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
  })
    .then((data) => {
      dataRef.value = data.data
      paginationReactive.page = 1
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
    .catch((e) => {
      message.error(e?.message || '加载交易日志失败')
      loadingRef.value = false
    })
  fetchStatistics()
  // 定时刷新收盘/最新价与盈亏：不抢 loading，避免请求进行中时跳过后续刷新
  refreshTimer.value = setInterval(() => {
    silentRefreshCurrentPage()
  }, 1000 * 10)
  // 股票基础数据由后端异步加载，加载完成后刷新可搜索股票全集
  loadStockList()
  EventsOn('loadingDone', () => loadStockList())
})

onUnmounted(() => {
  // 清除定时器
  if (refreshTimer.value) {
    clearInterval(refreshTimer.value)
  }
  if (fundSearchTimer) {
    clearTimeout(fundSearchTimer)
  }
  EventsOff('loadingDone')
})
</script>

<template>
  <n-input-group>
    <n-date-picker v-model:value="paginationReactive.range" type="daterange" style="width: 40%" />
    <n-select
      v-model:value="paginationReactive.direction"
      :options="directionOptions"
      placeholder="交易方向"
      style="width: 15%"
      clearable
    />
    <n-input clearable placeholder="股票代码 / 名称" v-model:value="paginationReactive.keyword" />
    <n-button type="primary" ghost @click="handleSearch">搜索</n-button>
    <n-button @click="resetFilter">重置</n-button>
    <n-button type="primary" ghost @click="openAddModal">添加记录</n-button>
  </n-input-group>

  <n-flex justify="end" align="center" style="margin-top: 10px">
    <n-button type="primary" secondary @click="showAiSummaryModal = true">AI 分析持仓</n-button>
    <n-button type="warning" secondary @click="showDeepAnalysisModal = true">深度分析</n-button>
    <n-button secondary @click="showSummaryHistory = true">历史总结</n-button>
  </n-flex>

  <n-grid :cols="6" :x-gap="12" style="margin-top: 12px; padding: 12px; border-radius: 4px">
    <n-grid-item>
      <n-statistic label="持仓金额(元)">
        <n-number-animation :from="0" :to="statisticsRef?.holdingsAmount || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="持仓市值(元)">
        <n-number-animation :from="0" :to="statisticsRef?.currentValue || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="总买入(元)">
        <n-number-animation :from="0" :to="statisticsRef?.totalBuyAmount || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="总卖出(元)">
        <n-number-animation :from="0" :to="statisticsRef?.totalSellAmount || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="总收益(元)">
        <n-text :type="statisticsRef?.totalProfit > 0 ? 'error' : 'success'">
          <n-number-animation :from="0" :to="statisticsRef?.totalProfit || 0" :precision="2"  />
        </n-text>
      </n-statistic>
    </n-grid-item>
    <n-grid-item>

      <n-statistic label="收益率">
        <n-text :type="statisticsRef?.profitRate > 0 ? 'error' : 'success'">
          <n-number-animation :from="0" :to="statisticsRef?.profitRate || 0" :precision="2"  />%
        </n-text>
      </n-statistic>

    </n-grid-item>
  </n-grid>

  <n-data-table
    remote
    size="small"
    :columns="columnsRef"
    :data="dataRef"
    :loading="loadingRef"
    :pagination="paginationReactive"
    :row-key="(rowData) => rowData.ID"
    @update:page="handlePageChange"
    flex-height
    style="height: calc(100vh - 310px); margin-top: 10px"
  />

  <n-modal v-model:show="showAddModal" preset="card" title="添加交易日志" style="width: 820px;max-width: calc(100vw - 32px);">
    <n-form label-placement="top" size="small">
      <n-grid :cols="3" :x-gap="12" :y-gap="2">
        <n-grid-item :span="3">
          <n-form-item label="股票/场内基金（输入代码或名称模糊搜索，选中后自动带出代码与名称）">
            <n-auto-complete
              v-model:value="stockSearchText"
              :options="stockSearchOptions"
              placeholder="输入股票代码或名称，如 600519 / 茅台 / 000001.SH"
              :input-props="{ autocomplete: 'disabled' }"
              clearable
              @update:value="filterStockOptions"
              @select="handleStockSelect"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="股票代码">
            <n-input v-model:value="formData.StockCode" placeholder="选中上方候选后自动填充" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="股票名称">
            <n-input v-model:value="formData.StockName" placeholder="选中上方候选后自动填充" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易方向">
            <n-select
              v-model:value="formData.Direction"
              :options="[
                { label: '买入', value: '买入' },
                { label: '卖出', value: '卖出' }
              ]"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="价格">
            <n-input-number v-model:value="formData.Price" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="成交数量">
            <n-input-number v-model:value="formData.Volume" :min="1" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易时间">
            <n-date-picker v-model:value="formData.TradingTime" type="datetime" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止损价">
            <n-input-number v-model:value="formData.StopLossPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止盈价">
            <n-input-number v-model:value="formData.TakeProfitPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="手续费">
            <n-input-number v-model:value="formData.Fee" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="AI 建议（基于最新技术面与历史AI推荐）">
            <n-button type="primary" ghost :loading="aiSuggesting" @click="aiSuggestForForm">
              AI建议止损/止盈（约10-30秒）
            </n-button>
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易理由">
            <n-input v-model:value="formData.Reason" type="textarea" placeholder="请输入交易理由" :rows="4"  style="text-align: left" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易心态/感悟/复盘/备注">
            <n-input v-model:value="formData.Mindset" type="textarea" placeholder="请输入交易心态" :rows="5" style="text-align: left" />
          </n-form-item>
        </n-grid-item>
      </n-grid>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showAddModal = false">取消</n-button>
        <n-button type="primary" @click="handleAdd">添加</n-button>
      </n-space>
    </template>
  </n-modal>

  <n-modal v-model:show="showEditModal" preset="card" title="编辑交易日志" style="width: 820px;max-width: calc(100vw - 32px);">
    <n-form label-placement="top" size="small">
      <n-grid :cols="3" :x-gap="12" :y-gap="2">
        <n-grid-item :span="3">
          <n-form-item label="股票/场内基金（输入代码或名称模糊搜索，选中后自动带出代码与名称）">
            <n-auto-complete
              v-model:value="stockSearchText"
              :options="stockSearchOptions"
              placeholder="输入股票代码或名称，如 600519 / 茅台 / 000001.SH"
              :input-props="{ autocomplete: 'disabled' }"
              clearable
              @update:value="filterStockOptions"
              @select="handleStockSelect"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="股票代码">
            <n-input v-model:value="formData.StockCode" placeholder="选中上方候选后自动填充" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="股票名称">
            <n-input v-model:value="formData.StockName" placeholder="选中上方候选后自动填充" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易方向">
            <n-select
              v-model:value="formData.Direction"
              :options="[
                { label: '买入', value: '买入' },
                { label: '卖出', value: '卖出' }
              ]"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="价格">
            <n-input-number v-model:value="formData.Price" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="成交数量">
            <n-input-number v-model:value="formData.Volume" :min="1" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易时间">
            <n-date-picker v-model:value="formData.TradingTime" type="datetime" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止损价">
            <n-input-number v-model:value="formData.StopLossPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止盈价">
            <n-input-number v-model:value="formData.TakeProfitPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="手续费">
            <n-input-number v-model:value="formData.Fee" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="AI 建议（基于最新技术面与历史AI推荐）">
            <n-button type="primary" ghost :loading="aiSuggesting" @click="aiSuggestForForm">
              AI建议止损/止盈（约10-30秒）
            </n-button>
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易理由">
            <n-input v-model:value="formData.Reason" type="textarea" placeholder="请输入交易理由" :rows="2" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易心态">
            <n-input v-model:value="formData.Mindset" type="textarea" placeholder="请输入交易心态" :rows="2" />
          </n-form-item>
        </n-grid-item>
      </n-grid>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showEditModal = false">取消</n-button>
        <n-button type="primary" @click="handleUpdate">更新</n-button>
      </n-space>
    </template>
  </n-modal>

  <n-modal v-model:show="showKlineModal" preset="card" :title="'K线 - ' + klineStockName" style="width: 95vw; max-width: 1400px">
    <StockLightweightKlineChart
      :code="klineStockCode"
      :stock-name="klineStockName"
      :chart-height="500"
      :dark-theme="darkTheme"
      :longStopLossPrice="longStopLossPrice"
      :longTakeProfitPrice="longTakeProfitPrice"
      :costPrice="costPrice"
    />
  </n-modal>

  <StockIndicatorsModal v-model:show="showIndicatorsModal" :code="indicatorsCode" :name="indicatorsName" />
  <HoldingsAiSummaryModal v-model:show="showAiSummaryModal" />
  <HoldingsSummaryHistoryModal v-model:show="showSummaryHistory" />
  <TradeAiCommentModal v-model:show="showAiCommentModal" :record="aiCommentRecord" @saved="silentRefreshCurrentPage" />
  <HoldingsDeepAnalysisModal v-model:show="showDeepAnalysisModal" />
</template>

<style scoped></style>