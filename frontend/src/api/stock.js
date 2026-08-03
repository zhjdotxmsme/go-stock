/**
 * 股票相关 API
 * 封装所有股票相关的 Wails Go 绑定调用
 */

import { callApi } from './client'
import * as StockHandler from '../../wailsjs/go/handler/StockHandler'
import * as App from '../../wailsjs/go/main/App'

// ========== 自选股相关 ==========

/**
 * 获取股票列表
 * @returns {Promise<ApiResult>} 股票列表
 */
export async function getStockList() {
  return callApi(App.GetStockList)
}

/**
 * 添加股票到自选
 * @param {string} code - 股票代码
 * @param {string} name - 股票名称
 * @param {number} groupId - 群组ID
 * @returns {Promise<ApiResult>}
 */
export async function addStock(code, name, groupId = 0) {
  return callApi(App.Follow, code, name, groupId)
}

/**
 * 移除自选股票
 * @param {string} code - 股票代码
 * @returns {Promise<ApiResult>}
 */
export async function removeStock(code) {
  return callApi(App.UnFollow, code)
}

/**
 * 获取自选股列表
 * @returns {Promise<ApiResult>}
 */
export async function getFollowList() {
  return callApi(App.GetFollowList)
}

// ========== 群组相关 ==========

/**
 * 获取群组列表
 * @returns {Promise<ApiResult>}
 */
export async function getGroupList() {
  return callApi(App.GetGroupList)
}

/**
 * 添加群组
 * @param {string} name - 群组名称
 * @param {number} sort - 排序
 * @returns {Promise<ApiResult>}
 */
export async function addGroup(name, sort = 0) {
  return callApi(App.AddGroup, name, sort)
}

/**
 * 删除群组
 * @param {number} groupId - 群组ID
 * @returns {Promise<ApiResult>}
 */
export async function removeGroup(groupId) {
  return callApi(App.RemoveGroup, groupId)
}

/**
 * 添加股票到群组
 * @param {string} code - 股票代码
 * @param {number} groupId - 群组ID
 * @returns {Promise<ApiResult>}
 */
export async function addStockToGroup(code, groupId) {
  return callApi(App.AddStockGroup, code, groupId)
}

/**
 * 从群组移除股票
 * @param {string} code - 股票代码
 * @param {number} groupId - 群组ID
 * @returns {Promise<ApiResult>}
 */
export async function removeStockFromGroup(code, groupId) {
  return callApi(App.RemoveStockGroup, code, groupId)
}

/**
 * 获取群组股票列表
 * @param {number} groupId - 群组ID
 * @returns {Promise<ApiResult>}
 */
export async function getGroupStockList(groupId) {
  return callApi(App.GetGroupStockList, groupId)
}

/**
 * 更新群组排序
 * @param {Array} groups - 群组数组
 * @returns {Promise<ApiResult>}
 */
export async function updateGroupSort(groups) {
  return callApi(App.UpdateGroupSort, groups)
}

// ========== K 线相关 ==========

/**
 * 获取股票 K 线数据
 * @param {string} code - 股票代码
 * @param {string} period - 周期: day/week/month/minute
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getStockKLine(code, period = 'day', count = 100) {
  return callApi(App.GetStockKLine, code, period, count)
}

/**
 * 获取分钟 K 线数据
 * @param {string} code - 股票代码
 * @returns {Promise<ApiResult>}
 */
export async function getStockMinuteKLine(code) {
  return callApi(App.GetStockMinutePriceLineData, code)
}

/**
 * 获取东财 K 线数据
 * @param {string} code - 股票代码
 * @param {string} period - 周期
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getEastMoneyKLine(code, period = 'day', count = 100) {
  return callApi(App.GetStockEastMoneyKLine, code, period, count)
}

/**
 * 获取 K 线数据（带降级）
 * @param {string} code - 股票代码
 * @param {string} period - 周期
 * @param {number} count - 数量
 * @returns {Promise<ApiResult>}
 */
export async function getStockKLineWithFallback(code, period = 'day', count = 100) {
  return callApi(App.GetStockKLineWithFallback, code, period, count)
}

// ========== 技术指标 ==========

/**
 * 获取筹码分布
 * @param {string} code - 股票代码
 * @returns {Promise<ApiResult>}
 */
export async function getChipDistribution(code) {
  return callApi(App.GetChipDistribution, code)
}

/**
 * 获取公司信息
 * @param {string} code - 股票代码
 * @returns {Promise<ApiResult>}
 */
export async function getCompanyInfo(code) {
  return callApi(App.GetTdxCompanyInfo, code)
}

// ========== 交易相关 ==========

/**
 * 设置成本价和持仓量
 * @param {string} code - 股票代码
 * @param {number} costPrice - 成本价
 * @param {number} volume - 持仓量
 * @returns {Promise<ApiResult>}
 */
export async function setCostPriceAndVolume(code, costPrice, volume) {
  return callApi(App.SetCostPriceAndVolume, code, costPrice, volume)
}

/**
 * 设置预警涨跌幅
 * @param {string} code - 股票代码
 * @param {number} changePercent - 涨跌幅百分比
 * @returns {Promise<ApiResult>}
 */
export async function setAlarmChangePercent(code, changePercent) {
  return callApi(App.SetAlarmChangePercent, code, changePercent)
}

/**
 * 设置股票排序
 * @param {string} code - 股票代码
 * @param {number} sort - 排序值
 * @returns {Promise<ApiResult>}
 */
export async function setStockSort(code, sort) {
  return callApi(App.SetStockSort, code, sort)
}

// ========== 搜索 ==========

/**
 * 搜索股票
 * @param {string} keyword - 关键词
 * @returns {Promise<ApiResult>}
 */
export async function searchStock(keyword) {
  return callApi(App.SearchStock, keyword)
}

// 导出所有 API
export default {
  // 自选股
  getStockList,
  addStock,
  removeStock,
  getFollowList,

  // 群组
  getGroupList,
  addGroup,
  removeGroup,
  addStockToGroup,
  removeStockFromGroup,
  getGroupStockList,
  updateGroupSort,

  // K 线
  getStockKLine,
  getStockMinuteKLine,
  getEastMoneyKLine,
  getStockKLineWithFallback,

  // 技术指标
  getChipDistribution,
  getCompanyInfo,

  // 交易
  setCostPriceAndVolume,
  setAlarmChangePercent,
  setStockSort,

  // 搜索
  searchStock,
}
