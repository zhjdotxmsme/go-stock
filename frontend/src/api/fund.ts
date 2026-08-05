/**
 * 基金相关 API
 * 封装基金自选、基金K线、基金排名等调用
 */

import { callApi } from './client'
import * as FundHandler from '../../wailsjs/go/handler/FundHandler'

// ========== 基金自选 ==========

/**
 * 获取关注基金列表
 * Go: GetFollowedFund() []data.FollowedFund
 */
export async function getFollowedFund() {
  return callApi(FundHandler.GetFollowedFund)
}

/**
 * 关注基金
 * Go: FollowFund(code string) string
 * @param {string} code - 基金代码
 */
export async function followFund(code: string) {
  return callApi(FundHandler.FollowFund, code)
}

/**
 * 取消关注基金
 * Go: UnFollowFund(code string) string
 * @param {string} code - 基金代码
 */
export async function unFollowFund(code: string) {
  return callApi(FundHandler.UnFollowFund, code)
}

/**
 * 获取关注基金列表（分页）
 * Go: GetFollowedFundPaged(page, pageSize int, keyword string) data.FollowedFundPagedResult
 */
export async function getFollowedFundPaged(page: number, pageSize: number, keyword: string) {
  return callApi(FundHandler.GetFollowedFundPaged, page, pageSize, keyword)
}

// ========== 基金K线 ==========

/**
 * 获取基金K线数据
 * Go: GetFundKLine(code, name string, days int) data.KLineSourceResult
 */
export async function getFundKLine(code: string, name: string, days: number) {
  return callApi(FundHandler.GetFundKLine, code, name, days)
}

/**
 * 获取基金历史净值
 * Go: GetFundHistoryNetValue(code string, page int, startDate, endDate string) []data.FundHistoryNetValue
 */
export async function getFundHistoryNetValue(
  code: string,
  page: number,
  startDate: string,
  endDate: string
) {
  return callApi(FundHandler.GetFundHistoryNetValue, code, page, startDate, endDate)
}

// ========== 基金排名 ==========

/**
 * 获取基金排名
 * Go: GetFundRanking(type, order, startDate, endDate string, page, pageSize int) data.FundRankingResult
 */
export async function getFundRanking(
  type: string,
  order: string,
  startDate: string,
  endDate: string,
  page: number,
  pageSize: number
) {
  return callApi(FundHandler.GetFundRanking, type, order, startDate, endDate, page, pageSize)
}

/**
 * 获取基金Top10持仓
 * Go: GetFundTop10Holdings(code string) []data.FundHoldingStock
 */
export async function getFundTop10Holdings(code: string) {
  return callApi(FundHandler.GetFundTop10Holdings, code)
}

// ========== 基金搜索 ==========

/**
 * 搜索基金代码
 * Go: SearchFundCodes(keyword string) []data.FundSearchItem
 */
export async function searchFundCodes(keyword: string) {
  return callApi(FundHandler.SearchFundCodes, keyword)
}

/**
 * 获取基金列表
 * Go: GetfundList(keyword string) []data.FundBasic
 */
export async function getFundList(keyword: string) {
  return callApi(FundHandler.GetfundList, keyword)
}

export default {
  getFollowedFund,
  getFollowedFundPaged,
  followFund,
  unFollowFund,
  getFundKLine,
  getFundHistoryNetValue,
  getFundRanking,
  getFundTop10Holdings,
  searchFundCodes,
  getFundList,
}
