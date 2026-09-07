/**
 * 大宗商品相关 API
 * 封装大宗商品行情、K线、分析等调用
 */

import { callApi } from './client'
import * as CommodityHandler from '../../wailsjs/go/handler/CommodityHandler'
import * as AgentHandler from '../../wailsjs/go/handler/AgentHandler'

// ========== 行情 ==========

/**
 * 获取大宗商品行情
 * Go: GetCommodityQuote(code string) datasource.QuoteData
 */
export async function getCommodityQuote(code: string) {
  return callApi(CommodityHandler.GetCommodityQuote, code)
}

/**
 * 获取国际大宗商品行情
 * Go: GetCommodityQuoteIntl(code string) datasource.QuoteData
 */
export async function getCommodityQuoteIntl(code: string) {
  return callApi(CommodityHandler.GetCommodityQuoteIntl, code)
}

/**
 * 获取可交易大宗商品列表
 * Go: GetTradableCommodities() []models.CommodityAsset
 */
export async function getTradableCommodities() {
  return callApi(CommodityHandler.GetTradableCommodities)
}

/**
 * 获取大宗商品注册表
 * Go: GetCommodityRegistry() any
 */
export async function getCommodityRegistry() {
  return callApi(CommodityHandler.GetCommodityRegistry)
}

/**
 * 获取宏观经济指标
 * Go: GetMacroIndicatorsEnhanced() any
 */
export async function getMacroIndicatorsEnhanced() {
  return callApi(CommodityHandler.GetMacroIndicatorsEnhanced)
}

// ========== 期货盘面 / 策略信号 ==========

/**
 * 期货盘面数据（持仓/增仓/期限结构/库存/COT/金银比/ATR/季节性）
 * Go: GetCommodityFuturesPanel() data.CommodityFuturesPanel
 */
export async function getCommodityFuturesPanel() {
  return callApi(CommodityHandler.GetCommodityFuturesPanel)
}

/**
 * 全品种策略信号排行（趋势/动量/突破/carry/持仓象限 + 综合分）
 * Go: GetCommoditySignalBoard() data.CommoditySignalBoard
 */
export async function getCommoditySignalBoard() {
  return callApi(CommodityHandler.GetCommoditySignalBoard)
}

// ========== K线 ==========

/**
 * 获取大宗商品K线数据
 * Go: GetCommodityKLine(code, period string, count int) []datasource.KLineBar
 */
export async function getCommodityKLine(code: string, period: string, count: number) {
  return callApi(CommodityHandler.GetCommodityKLine, code, period, count)
}

/**
 * 获取国际大宗商品K线数据
 * Go: GetCommodityKLineIntl(code, period string, count int) []datasource.KLineBar
 */
export async function getCommodityKLineIntl(code: string, period: string, count: number) {
  return callApi(CommodityHandler.GetCommodityKLineIntl, code, period, count)
}

// ========== 分析 ==========

/**
 * 获取大宗商品基本面
 * Go: GetCommodityFundamentals(code string) string
 */
export async function getCommodityFundamentals(code: string) {
  return callApi(CommodityHandler.GetCommodityFundamentals, code)
}

/**
 * 获取大宗商品相关性
 * Go: GetCommodityCorrelation(code1, code2 string) string
 */
export async function getCommodityCorrelation(code1: string, code2: string) {
  return callApi(CommodityHandler.GetCommodityCorrelation, code1, code2)
}

/**
 * 获取大宗商品报告
 * Go: GetCommodityReport(code, lang string) string
 */
export async function getCommodityReport(code: string, lang: string) {
  return callApi(CommodityHandler.GetCommodityReport, code, lang)
}

/**
 * 获取大宗商品技术指标
 * Go: GetCommodityTechnicals(code, period string) string
 */
export async function getCommodityTechnicals(code: string, period: string) {
  return callApi(CommodityHandler.GetCommodityTechnicals, code, period)
}

/**
 * 大宗商品AI分析流
 * Go: NewCommodityAnalysisStream(code, period, question string, aiConfigId int)
 */
export async function newCommodityAnalysisStream(
  code: string,
  period: string,
  question: string,
  aiConfigId: number
) {
  return callApi(AgentHandler.NewCommodityAnalysisStream, code, period, question, aiConfigId)
}

export default {
  getCommodityQuote,
  getCommodityQuoteIntl,
  getTradableCommodities,
  getCommodityRegistry,
  getMacroIndicatorsEnhanced,
  getCommodityFuturesPanel,
  getCommoditySignalBoard,
  getCommodityKLine,
  getCommodityKLineIntl,
  getCommodityFundamentals,
  getCommodityCorrelation,
  getCommodityReport,
  getCommodityTechnicals,
  newCommodityAnalysisStream,
}
