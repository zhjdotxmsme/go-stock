/**
 * 交易记录相关 API
 * 封装交易记录的增删改查等调用
 */

import { callApi } from './client'
import * as TradingRecordHandler from '../../wailsjs/go/handler/TradingRecordHandler'
import * as StockHandler from '../../wailsjs/go/handler/StockHandler'

// ========== 交易记录 ==========

/**
 * 获取交易记录列表
 * Go: GetTradingRecordList(query data.TradingRecordListQuery) data.TradingRecordPageData
 */
export async function getTradingRecordList(query: any) {
  return callApi(TradingRecordHandler.GetTradingRecordList, query)
}

/**
 * 添加交易记录
 * Go: AddTradingRecord(record data.TradingRecord) number
 */
export async function addTradingRecord(record: any) {
  return callApi(TradingRecordHandler.AddTradingRecord, record)
}

/**
 * 更新交易记录
 * Go: UpdateTradingRecord(record data.TradingRecord)
 */
export async function updateTradingRecord(record: any) {
  return callApi(TradingRecordHandler.UpdateTradingRecord, record)
}

/**
 * 删除交易记录
 * Go: DeleteTradingRecord(id number)
 */
export async function deleteTradingRecord(id: number) {
  return callApi(TradingRecordHandler.DeleteTradingRecord, id)
}

/**
 * 获取交易记录详情
 * Go: GetTradingRecordById(id number) data.TradingRecord
 */
export async function getTradingRecordById(id: number) {
  return callApi(TradingRecordHandler.GetTradingRecordById, id)
}

/**
 * 获取交易记录统计
 * Go: GetTradingRecordStatistics() data.TradingRecordStatistics
 */
export async function getTradingRecordStatistics() {
  return callApi(TradingRecordHandler.GetTradingRecordStatistics)
}

// ========== 辅助 ==========

/**
 * 检查频繁交易
 * Go: CheckFrequentTrading(code string) Record<string,any>
 */
export async function checkFrequentTrading(code: string) {
  return callApi(TradingRecordHandler.CheckFrequentTrading, code)
}

/**
 * 获取股票实时价格
 * Go: GetStockRealTimePrice(code string) Record<string,any>
 */
export async function getStockRealTimePrice(code: string) {
  return callApi(StockHandler.GetStockRealTimePrice, code)
}

// ========== 持仓明细 / 技术指标 / AI 每日总结 ==========

/**
 * 获取逐股持仓明细（FIFO 推算 + 实时现价）
 * Go: GetHoldingsDetail() []data.HoldingsPosition
 */
export async function getHoldingsDetail() {
  return callApi(TradingRecordHandler.GetHoldingsDetail)
}

/**
 * 获取单股全套技术指标与文字解读
 * Go: GetStockTechnicalIndicators(code string) *data.StockIndicatorsResult
 */
export async function getStockTechnicalIndicators(code: string) {
  return callApi(TradingRecordHandler.GetStockTechnicalIndicators, code)
}

/**
 * 流式生成持仓 AI 每日总结（结果通过 Wails 事件 eventName 推送，结束发 "DONE"，完成自动落库）
 * Go: SummarizeHoldings(aiConfigId int, eventName string)
 */
export async function summarizeHoldings(aiConfigId: number, eventName: string) {
  return callApi(TradingRecordHandler.SummarizeHoldings, aiConfigId, eventName)
}

/**
 * 中断进行中的持仓 AI 总结（不落库）
 * Go: AbortSummarizeHoldings()
 */
export async function abortSummarizeHoldings() {
  return callApi(TradingRecordHandler.AbortSummarizeHoldings)
}

/**
 * 持仓总结历史列表（按日期倒序，content 为预览）
 * Go: GetHoldingsSummaryList(page, pageSize int) *data.HoldingsSummaryPageData
 */
export async function getHoldingsSummaryList(page = 1, pageSize = 20) {
  return callApi(TradingRecordHandler.GetHoldingsSummaryList, page, pageSize)
}

/**
 * 获取单条持仓总结全文（含持仓快照）
 * Go: GetHoldingsSummaryDetail(id uint) *models.HoldingsDailySummary
 */
export async function getHoldingsSummaryDetail(id: number) {
  return callApi(TradingRecordHandler.GetHoldingsSummaryDetail, id)
}

// ========== AI 建议 / 单笔 AI 点评 ==========

/**
 * 获取股票最近一次 AI 推荐建议（止损/止盈/买入区间），用于交易日志自动带入
 * Go: GetAiAdviceForStock(stockCode, stockName string) *data.TradingAiAdvice
 */
export async function getAiAdviceForStock(stockCode: string, stockName = '') {
  return callApi(TradingRecordHandler.GetAiAdviceForStock, stockCode, stockName)
}

/**
 * 流式生成单笔交易日志的 AI 点评（结果通过 Wails 事件 eventName 推送，结束发 "DONE"，完成自动落库）
 * Go: GenerateTradeAiComment(id uint, aiConfigId int, eventName string)
 */
export async function generateTradeAiComment(id: number, aiConfigId: number, eventName: string) {
  return callApi(TradingRecordHandler.GenerateTradeAiComment, id, aiConfigId, eventName)
}

/**
 * 让 AI 基于最新技术指标直接给出建议止损/止盈价位（同步调用，耗时约 10~30 秒）
 * Go: AiSuggestPriceLevels(stockCode, stockName string, aiConfigId int) *data.TradingAiAdvice
 */
export async function aiSuggestPriceLevels(stockCode: string, stockName: string, aiConfigId: number) {
  return callApi(TradingRecordHandler.AiSuggestPriceLevels, stockCode, stockName, aiConfigId)
}

/**
 * 持仓深度分析数据包（本地聚合：直白关键价位 + 技术指标 + 资金流 + 板块 + 新闻 + 历史AI推荐）
 * Go: GetHoldingsDeepData() []*data.HoldingsDeepStock
 */
export async function getHoldingsDeepData() {
  return callApi(TradingRecordHandler.GetHoldingsDeepData)
}

/**
 * 流式生成持仓深度 AI 综合分析（结果通过 Wails 事件 eventName 推送，结束发 "DONE"，不落库）
 * Go: AnalyzeHoldingsDeep(aiConfigId int, eventName string)
 */
export async function analyzeHoldingsDeep(aiConfigId: number, eventName: string) {
  return callApi(TradingRecordHandler.AnalyzeHoldingsDeep, aiConfigId, eventName)
}

export default {
  getTradingRecordList,
  addTradingRecord,
  updateTradingRecord,
  deleteTradingRecord,
  getTradingRecordById,
  getTradingRecordStatistics,
  checkFrequentTrading,
  getStockRealTimePrice,
  getHoldingsDetail,
  getStockTechnicalIndicators,
  summarizeHoldings,
  abortSummarizeHoldings,
  getHoldingsSummaryList,
  getHoldingsSummaryDetail,
  getAiAdviceForStock,
  generateTradeAiComment,
  aiSuggestPriceLevels,
  getHoldingsDeepData,
  analyzeHoldingsDeep,
}
