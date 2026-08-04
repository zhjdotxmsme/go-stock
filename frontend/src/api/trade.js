/**
 * 交易记录相关 API
 * 封装交易记录的增删改查等调用
 */

import { callApi } from './client'
import * as App from '../../wailsjs/go/main/App'

// ========== 交易记录 ==========

/**
 * 获取交易记录列表
 * Go: GetTradingRecordList(query data.TradingRecordListQuery) data.TradingRecordPageData
 */
export async function getTradingRecordList(query) {
  return callApi(App.GetTradingRecordList, query)
}

/**
 * 添加交易记录
 * Go: AddTradingRecord(record data.TradingRecord) number
 */
export async function addTradingRecord(record) {
  return callApi(App.AddTradingRecord, record)
}

/**
 * 更新交易记录
 * Go: UpdateTradingRecord(record data.TradingRecord)
 */
export async function updateTradingRecord(record) {
  return callApi(App.UpdateTradingRecord, record)
}

/**
 * 删除交易记录
 * Go: DeleteTradingRecord(id number)
 */
export async function deleteTradingRecord(id) {
  return callApi(App.DeleteTradingRecord, id)
}

/**
 * 获取交易记录详情
 * Go: GetTradingRecordById(id number) data.TradingRecord
 */
export async function getTradingRecordById(id) {
  return callApi(App.GetTradingRecordById, id)
}

/**
 * 获取交易记录统计
 * Go: GetTradingRecordStatistics() data.TradingRecordStatistics
 */
export async function getTradingRecordStatistics() {
  return callApi(App.GetTradingRecordStatistics)
}

// ========== 辅助 ==========

/**
 * 检查频繁交易
 * Go: CheckFrequentTrading(code string) Record<string,any>
 */
export async function checkFrequentTrading(code) {
  return callApi(App.CheckFrequentTrading, code)
}

/**
 * 获取股票实时价格
 * Go: GetStockRealTimePrice(code string) Record<string,any>
 */
export async function getStockRealTimePrice(code) {
  return callApi(App.GetStockRealTimePrice, code)
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
}
