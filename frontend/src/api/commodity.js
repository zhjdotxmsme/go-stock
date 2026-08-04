/**
 * 大宗商品相关 API
 * 封装大宗商品行情、K线、分析等调用
 */

import { callApi } from './client'
import * as App from '../../wailsjs/go/main/App'

// ========== 行情 ==========

/**
 * 获取大宗商品行情
 * Go: GetCommodityQuote(code string) datasource.QuoteData
 */
export async function getCommodityQuote(code) {
  return callApi(App.GetCommodityQuote, code)
}

/**
 * 获取国际大宗商品行情
 * Go: GetCommodityQuoteIntl(code string) datasource.QuoteData
 */
export async function getCommodityQuoteIntl(code) {
  return callApi(App.GetCommodityQuoteIntl, code)
}

/**
 * 获取可交易大宗商品列表
 * Go: GetTradableCommodities() []models.CommodityAsset
 */
export async function getTradableCommodities() {
  return callApi(App.GetTradableCommodities)
}

/**
 * 获取大宗商品注册表
 * Go: GetCommodityRegistry() any
 */
export async function getCommodityRegistry() {
  return callApi(App.GetCommodityRegistry)
}

/**
 * 获取宏观经济指标
 * Go: GetMacroIndicatorsEnhanced() any
 */
export async function getMacroIndicatorsEnhanced() {
  return callApi(App.GetMacroIndicatorsEnhanced)
}

// ========== K线 ==========

/**
 * 获取大宗商品K线数据
 * Go: GetCommodityKLine(code, period string, count int) []datasource.KLineBar
 */
export async function getCommodityKLine(code, period, count) {
  return callApi(App.GetCommodityKLine, code, period, count)
}

/**
 * 获取国际大宗商品K线数据
 * Go: GetCommodityKLineIntl(code, period string, count int) []datasource.KLineBar
 */
export async function getCommodityKLineIntl(code, period, count) {
  return callApi(App.GetCommodityKLineIntl, code, period, count)
}

// ========== 分析 ==========

/**
 * 获取大宗商品基本面
 * Go: GetCommodityFundamentals(code string) string
 */
export async function getCommodityFundamentals(code) {
  return callApi(App.GetCommodityFundamentals, code)
}

/**
 * 获取大宗商品相关性
 * Go: GetCommodityCorrelation(code1, code2 string) string
 */
export async function getCommodityCorrelation(code1, code2) {
  return callApi(App.GetCommodityCorrelation, code1, code2)
}

/**
 * 获取大宗商品报告
 * Go: GetCommodityReport(code, lang string) string
 */
export async function getCommodityReport(code, lang) {
  return callApi(App.GetCommodityReport, code, lang)
}

/**
 * 获取大宗商品技术指标
 * Go: GetCommodityTechnicals(code, period string) string
 */
export async function getCommodityTechnicals(code, period) {
  return callApi(App.GetCommodityTechnicals, code, period)
}

/**
 * 大宗商品AI分析流
 * Go: NewCommodityAnalysisStream(code, period, question string, aiConfigId int)
 */
export async function newCommodityAnalysisStream(code, period, question, aiConfigId) {
  return callApi(App.NewCommodityAnalysisStream, code, period, question, aiConfigId)
}

export default {
  getCommodityQuote,
  getCommodityQuoteIntl,
  getTradableCommodities,
  getCommodityRegistry,
  getMacroIndicatorsEnhanced,
  getCommodityKLine,
  getCommodityKLineIntl,
  getCommodityFundamentals,
  getCommodityCorrelation,
  getCommodityReport,
  getCommodityTechnicals,
  newCommodityAnalysisStream,
}
