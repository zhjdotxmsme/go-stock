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
export async function getMarketNews(count: number = 20): Promise<any> {
  return callApi(App.GetMarketNews, count)
}

/**
 * 获取全球股指
 * @returns {Promise<ApiResult>}
 */
export async function getGlobalIndices(): Promise<any> {
  return callApi(App.GetGlobalStockIndices)
}

/**
 * 获取行业排名
 * @param {string} sort - 排序字段
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getIndustryRank(sort: string = '', count: number = 50): Promise<any> {
  return callApi(App.GetIndustryRank, sort, count)
}

/**
 * 获取个股资金流向
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getStockMoneyFlow(count: number = 50): Promise<any> {
  return callApi(App.GetStockMoneyFlow, count)
}

/**
 * 获取板块资金流向
 * @returns {Promise<ApiResult>}
 */
export async function getSectorMoneyFlow(): Promise<any> {
  return callApi(App.GetSectorMoneyFlow)
}

/**
 * 获取龙虎榜数据
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function getLongHuBang(date: string = ''): Promise<any> {
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
export async function summaryStockNews(stockCode: string, count: number, template: any, arg4: boolean, arg5: boolean, arg6: string, arg7: string): Promise<any> {
  return callApi(App.SummaryStockNews, stockCode, count, template, arg4, arg5, arg6, arg7)
}

/**
 * 中止股票新闻概括
 * @returns {Promise<ApiResult>}
 */
export async function abortSummaryStockNews(): Promise<any> {
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
export async function getStockChanges(changeTypes: any, pageIndex: number, pageSize: number): Promise<any> {
  return callApi(App.GetStockChanges, changeTypes, pageIndex, pageSize)
}

export async function getStockChangeHistory(query: any): Promise<any> {
  return callApi(App.GetStockChangeHistory, query)
}

export async function saveStockChangesToHistory(types: any): Promise<any> {
  return callApi(App.SaveStockChangesToHistory, types)
}

export async function getAllStockChangesWithPaging(pageSize: number): Promise<any> {
  return callApi(App.GetAllStockChangesWithPaging, pageSize)
}

/**
 * 获取异动统计
 * @returns {Promise<ApiResult>}
 */
export async function getChangeStats(): Promise<any> {
  return callApi(App.GetChangeStats)
}

// ========== 电报/新闻 ==========

/**
 * 获取财经电报
 * @param {number} count - 数量
 * @param {number} offset - 偏移
 * @returns {Promise<ApiResult>}
 */
export async function getTelegraph(count: number = 50, offset: number = 0): Promise<any> {
  return callApi(App.GetTelegraph, count, offset)
}

/**
 * 获取电报列表（按来源）
 * @param {string} source - 来源（财联社电报/新浪财经/外媒）
 * @returns {Promise<ApiResult>}
 */
export async function getTelegraphList(source: string): Promise<any> {
  return callApi(App.GetTelegraphList, source)
}

/**
 * 刷新电报列表
 * @param {string} source - 来源
 * @returns {Promise<ApiResult>}
 */
export async function refreshTelegraphList(source: string): Promise<any> {
  return callApi(App.ReFleshTelegraphList, source)
}

/**
 * 获取全球股指行情
 * @returns {Promise<ApiResult>}
 */
export async function getGlobalStockIndexes(): Promise<any> {
  return callApi(App.GlobalStockIndexes)
}

/**
 * 获取新闻列表
 * @param {Object} options - 查询选项
 * @returns {Promise<ApiResult>}
 */
export async function getNewsList(options: any = {}): Promise<any> {
  return callApi(App.GetNewsList, options)
}

// ========== 交易时间 ==========

/**
 * 检查是否交易时间
 * @returns {Promise<ApiResult>}
 */
export async function isTradingTime(): Promise<any> {
  return callApi(App.IsTradingTime)
}

/**
 * 检查是否港股交易时间
 * @returns {Promise<ApiResult>}
 */
export async function isHKTradingTime(): Promise<any> {
  return callApi(App.IsHKTradingTime)
}

/**
 * 检查是否美股交易时间
 * @returns {Promise<ApiResult>}
 */
export async function isUSTradingTime(): Promise<any> {
  return callApi(App.IsUSTradingTime)
}

// ========== 实时价格 ==========

/**
 * 获取实时行情
 * @param {string|Array} codes - 股票代码或代码数组
 * @returns {Promise<ApiResult>}
 */
export async function getRealtimeQuote(codes: string | string[]): Promise<any> {
  const codeList = Array.isArray(codes) ? codes : [codes]
  return callApi(App.GetRealtimeQuote, codeList.join(','))
}

// ========== 情绪分析 ==========

/**
 * 基于词频加权的情感分析
 * @param {string} text - 待分析文本
 * @returns {Promise<ApiResult>}
 */
export async function analyzeSentimentWithFreqWeight(text: string): Promise<any> {
  return callApi(App.AnalyzeSentimentWithFreqWeight, text)
}

// ========== 市场统计 ==========

/**
 * 获取今日市场统计数据
 * @returns {Promise<ApiResult>}
 */
export async function getTodayMarketStatistic(): Promise<any> {
  return callApi(App.GetTodayMarketStatistic)
}

/**
 * 获取最近N天市场统计数据
 * @param {number} days - 天数
 * @returns {Promise<ApiResult>}
 */
export async function getRecentDaysMarketStatistic(days: number): Promise<any> {
  return callApi(App.GetRecentDaysMarketStatistic, days)
}

/**
 * 获取每日涨跌统计
 * @param {number} days - 天数
 * @returns {Promise<ApiResult>}
 */
export async function getDailyChangeStats(days: number): Promise<any> {
  return callApi(App.GetDailyChangeStats, days)
}

/**
 * 获取涨跌类型每日统计
 * @param {number} days - 天数
 * @returns {Promise<ApiResult>}
 */
export async function getChangeTypeDailyStats(days: number): Promise<any> {
  return callApi(App.GetChangeTypeDailyStats, days)
}

/**
 * 获取涨跌排行
 * @param {number} limit - 限制数量
 * @param {number} days - 天数
 * @returns {Promise<ApiResult>}
 */
export async function getChangeRank(limit: number, days: number): Promise<any> {
  return callApi(App.GetChangeRank, limit, days)
}

/**
 * 获取每日维度统计
 * @param {string} field - 字段
 * @param {string} sort - 排序
 * @param {number} days - 天数
 * @returns {Promise<ApiResult>}
 */
export async function getDailyDimensionStats(field: string, sort: string, days: number): Promise<any> {
  return callApi(App.GetDailyDimensionStats, field, sort, days)
}

/**
 * 按日期获取类型统计
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function getTypeStatsByDate(date: string): Promise<any> {
  return callApi(App.GetTypeStatsByDate, date)
}

// ========== 板块/概念 ==========

/**
 * 获取板块列表
 * @returns {Promise<ApiResult>}
 */
export async function getAllBKCodes(): Promise<any> {
  return callApi(App.GetAllBKCodes)
}

/**
 * 获取概念列表
 * @returns {Promise<ApiResult>}
 */
export async function getAllConceptCodes(): Promise<any> {
  return callApi(App.GetAllConceptCodes)
}

/**
 * 按板块获取资金流列表
 * @param {string} code - 板块代码
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function getBKFundFlowListByDate(code: string, date: string): Promise<any> {
  return callApi(App.GetBKFundFlowListByDate, code, date)
}

/**
 * 获取板块资金流Top列表
 * @param {number} days - 天数
 * @param {number} limit - 限制数量
 * @returns {Promise<ApiResult>}
 */
export async function getBKFundFlowTopListByDate(days: number, limit: number): Promise<any> {
  return callApi(App.GetBKFundFlowTopListByDate, days, limit)
}

/**
 * 按概念获取资金流列表
 * @param {string} code - 概念代码
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function getConceptFundFlowListByDate(code: string, date: string): Promise<any> {
  return callApi(App.GetConceptFundFlowListByDate, code, date)
}

/**
 * 获取概念资金流Top列表
 * @param {number} days - 天数
 * @param {number} limit - 限制数量
 * @returns {Promise<ApiResult>}
 */
export async function getConceptFundFlowTopListByDate(days: number, limit: number): Promise<any> {
  return callApi(App.GetConceptFundFlowTopListByDate, days, limit)
}

// ========== 行业/个股资金 ==========

/**
 * 获取个股资金趋势(按日)
 * @param {string} code - 股票代码
 * @param {number} days - 天数
 * @returns {Promise<ApiResult>}
 */
export async function getStockMoneyTrendByDay(code: string, days: number): Promise<any> {
  return callApi(App.GetStockMoneyTrendByDay, code, days)
}

/**
 * 获取新浪资金排行
 * @param {string} sort - 排序方式
 * @returns {Promise<ApiResult>}
 */
export async function getMoneyRankSina(sort: string): Promise<any> {
  return callApi(App.GetMoneyRankSina, sort)
}

/**
 * 获取新浪行业资金排行
 * @param {string} fenlei - 分类
 * @param {string} sort - 排序方式
 * @returns {Promise<ApiResult>}
 */
export async function getIndustryMoneyRankSina(fenlei: string, sort: string): Promise<any> {
  return callApi(App.GetIndustryMoneyRankSina, fenlei, sort)
}

// ========== 新闻/资讯 ==========

/**
 * 按板块获取新闻
 * @param {string} sector - 板块
 * @param {number} page - 页码
 * @returns {Promise<ApiResult>}
 */
export async function getNewsBySector(sector: string, page: number): Promise<any> {
  return callApi(App.GetNewsBySector, sector, page)
}

/**
 * 获取板块列表（新闻用）
 * @returns {Promise<ApiResult>}
 */
export async function getSectors(): Promise<any> {
  return callApi(App.GetSectors)
}

/**
 * 获取个股相关新闻
 * @param {string} code - 股票代码
 * @param {number} page - 页码
 * @returns {Promise<ApiResult>}
 */
export async function getStockRelatedNews(code: string, page: number): Promise<any> {
  return callApi(App.GetStockRelatedNews, code, page)
}

// ========== 龙虎榜/研报/公告 ==========

/**
 * 获取龙虎榜排行
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function longTigerRank(date: string): Promise<any> {
  return callApi(App.LongTigerRank, date)
}

/**
 * 获取股票公告
 * @param {string} codes - 股票代码（逗号分隔）
 * @returns {Promise<ApiResult>}
 */
export async function stockNotice(codes: string): Promise<any> {
  return callApi(App.StockNotice, codes)
}

/**
 * 获取个股研报
 * @param {string} code - 股票代码
 * @returns {Promise<ApiResult>}
 */
export async function stockResearchReport(code: string): Promise<any> {
  return callApi(App.StockResearchReport, code)
}

/**
 * 获取行业研报
 * @param {string} code - 行业代码
 * @returns {Promise<ApiResult>}
 */
export async function industryResearchReport(code: string): Promise<any> {
  return callApi(App.IndustryResearchReport, code)
}

/**
 * 东方财富字典代码
 * @param {string} code - 代码
 * @returns {Promise<ApiResult>}
 */
export async function emDictCode(code: string): Promise<any> {
  return callApi(App.EMDictCode, code)
}

// ========== 热门/涨停 ==========

/**
 * 热门事件
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function hotEvent(count: number): Promise<any> {
  return callApi(App.HotEvent, count)
}

/**
 * 热门股票
 * @param {string} type - 类型
 * @returns {Promise<ApiResult>}
 */
export async function hotStock(type: string): Promise<any> {
  return callApi(App.HotStock, type)
}

/**
 * 热门话题
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function hotTopic(count: number): Promise<any> {
  return callApi(App.HotTopic, count)
}

/**
 * 涨停热榜
 * @param {string} type - 类型
 * @param {number} limit - 限制数量
 * @returns {Promise<ApiResult>}
 */
export async function getUplimitHot(type: string, limit: number): Promise<any> {
  return callApi(App.GetUplimitHot, type, limit)
}

/**
 * 热门策略
 * @returns {Promise<ApiResult>}
 */
export async function getHotStrategy(): Promise<any> {
  return callApi(App.GetHotStrategy)
}

// ========== 日历 ==========

/**
 * 财经日历
 * @returns {Promise<ApiResult>}
 */
export async function clsCalendar(): Promise<any> {
  return callApi(App.ClsCalendar)
}

/**
 * 投资日历时间线
 * @param {string} ym - 年月 (YYYY-MM)
 * @returns {Promise<ApiResult>}
 */
export async function investCalendarTimeLine(ym: string): Promise<any> {
  return callApi(App.InvestCalendarTimeLine, ym)
}

// ========== 交易日 ==========

/**
 * 判断是否交易日
 * @param {string} date - 日期
 * @returns {Promise<ApiResult>}
 */
export async function isTradingDay(date: string): Promise<any> {
  return callApi(App.IsTradingDay, date)
}

/**
 * 获取最近交易日
 * @returns {Promise<ApiResult>}
 */
export async function getLatestTradingDay(): Promise<any> {
  return callApi(App.GetLatestTradingDay)
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
  getStockChangeHistory,
  saveStockChangesToHistory,
  getAllStockChangesWithPaging,
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

  // 情绪分析
  analyzeSentimentWithFreqWeight,

  // 市场统计
  getTodayMarketStatistic,
  getRecentDaysMarketStatistic,
  getDailyChangeStats,
  getChangeTypeDailyStats,
  getChangeRank,
  getDailyDimensionStats,
  getTypeStatsByDate,

  // 板块/概念
  getAllBKCodes,
  getAllConceptCodes,
  getBKFundFlowListByDate,
  getBKFundFlowTopListByDate,
  getConceptFundFlowListByDate,
  getConceptFundFlowTopListByDate,

  // 行业/个股资金
  getStockMoneyTrendByDay,
  getMoneyRankSina,
  getIndustryMoneyRankSina,

  // 新闻/资讯
  getNewsBySector,
  getSectors,
  getStockRelatedNews,

  // 龙虎榜/研报/公告
  longTigerRank,
  stockNotice,
  stockResearchReport,
  industryResearchReport,
  emDictCode,

  // 热门/涨停
  hotEvent,
  hotStock,
  hotTopic,
  getUplimitHot,
  getHotStrategy,

  // 日历
  clsCalendar,
  investCalendarTimeLine,

  // 交易日
  isTradingDay,
  getLatestTradingDay,
}
