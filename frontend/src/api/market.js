/**
 * 市场行情相关 API
 * 封装行情、新闻、资金流等市场相关调用
 */

import { callApi } from './client'
import * as App from '../../wailsjs/go/main/App'

// ========== 市场行情 ==========

/**
 * 获取市场快讯
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getMarketNews(count = 20) {
  return callApi(App.GetMarketNews, count)
}

/**
 * 获取全球股指
 * @returns {Promise<ApiResult>}
 */
export async function getGlobalIndices() {
  return callApi(App.GetGlobalStockIndices)
}

/**
 * 获取行业排名
 * @param {string} sort - 排序字段
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getIndustryRank(sort = '', count = 50) {
  return callApi(App.GetIndustryRank, sort, count)
}

/**
 * 获取个股资金流向
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getStockMoneyFlow(count = 50) {
  return callApi(App.GetStockMoneyFlow, count)
}

/**
 * 获取板块资金流向
 * @returns {Promise<ApiResult>}
 */
export async function getSectorMoneyFlow() {
  return callApi(App.GetSectorMoneyFlow)
}

/**
 * 获取龙虎榜数据
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function getLongHuBang(date = '') {
  return callApi(App.GetLongHuBang, date)
}

/**
 * 概括股票新闻 (AI 总结)
 * @param {string} stockCode
 * @param {number} count
 * @param {any} template
 * @param {boolean} arg4
 * @param {boolean} arg5
 * @param {string} arg6
 * @param {string} arg7
 * @returns {Promise<ApiResult>}
 */
export async function summaryStockNews(stockCode, count, template, arg4, arg5, arg6, arg7) {
  return callApi(App.SummaryStockNews, stockCode, count, template, arg4, arg5, arg6, arg7)
}

/**
 * 中止股票新闻概括
 * @returns {Promise<ApiResult>}
 */
export async function abortSummaryStockNews() {
  return callApi(App.AbortSummaryStockNews)
}

// ========== 异动监控 ==========

/**
 * 获取股票异动列表
 * @param {Object} options - 查询选项
 * @param {number} options.page - 页码
 * @param {number} options.pageSize - 每页数量
 * @param {string} options.type - 异动类型
 * @returns {Promise<ApiResult>}
 */
export async function getStockChanges(options = {}) {
  const { page = 1, pageSize = 50, type = '' } = options
  return callApi(App.GetStockChanges, page, pageSize, type)
}

/**
 * 获取异动统计
 * @returns {Promise<ApiResult>}
 */
export async function getChangeStats() {
  return callApi(App.GetChangeStats)
}

// ========== 电报/新闻 ==========

/**
 * 获取财经电报
 * @param {number} count - 数量
 * @param {number} offset - 偏移
 * @returns {Promise<ApiResult>}
 */
export async function getTelegraph(count = 50, offset = 0) {
  return callApi(App.GetTelegraph, count, offset)
}

/**
 * 获取电报列表（按来源）
 * @param {string} source - 来源（财联社电报/新浪财经/外媒）
 * @returns {Promise<ApiResult>}
 */
export async function getTelegraphList(source) {
  return callApi(App.GetTelegraphList, source)
}

/**
 * 刷新电报列表
 * @param {string} source - 来源
 * @returns {Promise<ApiResult>}
 */
export async function refreshTelegraphList(source) {
  return callApi(App.ReFleshTelegraphList, source)
}

/**
 * 获取全球股指行情
 * @returns {Promise<ApiResult>}
 */
export async function getGlobalStockIndexes() {
  return callApi(App.GlobalStockIndexes)
}

/**
 * 获取新闻列表
 * @param {Object} options - 查询选项
 * @returns {Promise<ApiResult>}
 */
export async function getNewsList(options = {}) {
  return callApi(App.GetNewsList, options)
}

// ========== 交易时间 ==========

/**
 * 检查是否交易时间
 * @returns {Promise<ApiResult>}
 */
export async function isTradingTime() {
  return callApi(App.IsTradingTime)
}

/**
 * 检查是否港股交易时间
 * @returns {Promise<ApiResult>}
 */
export async function isHKTradingTime() {
  return callApi(App.IsHKTradingTime)
}

/**
 * 检查是否美股交易时间
 * @returns {Promise<ApiResult>}
 */
export async function isUSTradingTime() {
  return callApi(App.IsUSTradingTime)
}

// ========== 实时价格 ==========

/**
 * 获取实时行情
 * @param {string|Array} codes - 股票代码或代码数组
 * @returns {Promise<ApiResult>}
 */
export async function getRealtimeQuote(codes) {
  const codeList = Array.isArray(codes) ? codes : [codes]
  return callApi(App.GetRealtimeQuote, codeList.join(','))
}

export default {
  // 市场行情
  getMarketNews,
  getGlobalIndices,
  getIndustryRank,
  getStockMoneyFlow,
  getSectorMoneyFlow,
  getLongHuBang,

  // 异动监控
  getStockChanges,
  getChangeStats,

  // 电报/新闻
  getTelegraph,
  getTelegraphList,
  refreshTelegraphList,
  getNewsList,

  // 交易时间
  isTradingTime,
  isHKTradingTime,
  isUSTradingTime,

  // AI 新闻概括
  summaryStockNews,
  abortSummaryStockNews,

  // 实时价格
  getRealtimeQuote,

  // 全球股指
  getGlobalStockIndexes,
}
