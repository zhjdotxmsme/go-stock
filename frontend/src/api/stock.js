/**
 * 股票相关 API
 * 封装所有股票相关的 Wails Go 绑定调用
 */

import { callApi } from './client'
import * as App from '../../wailsjs/go/main/App'

// ========== 自选股相关 ==========

/**
 * 获取股票实时信息
 * Go: Greet(stockCode string) *data.StockInfo
 * @param {string} stockCode - 股票代码
 */
export async function greet(stockCode) {
  return callApi(App.Greet, stockCode)
}

/**
 * 关注股票
 * Go: Follow(stockCode string) string
 * @param {string} stockCode - 股票代码
 */
export async function follow(stockCode) {
  return callApi(App.Follow, stockCode)
}

/**
 * 取消关注
 * Go: UnFollow(stockCode string) string
 * @param {string} stockCode - 股票代码
 */
export async function unFollow(stockCode) {
  return callApi(App.UnFollow, stockCode)
}

/**
 * 获取自选股列表
 * Go: GetFollowList(groupId int) *[]data.FollowedStock
 * @param {number} groupId - 群组ID
 */
export async function getFollowList(groupId) {
  return callApi(App.GetFollowList, groupId)
}

/**
 * 获取股票列表
 * Go: GetStockList(key string) []data.StockBasic
 * @param {string} key - 搜索关键词
 */
export async function getStockList(key = '') {
  return callApi(App.GetStockList, key)
}

// ========== 群组相关 ==========

/**
 * 获取群组列表
 * Go: GetGroupList() []data.Group
 */
export async function getGroupList() {
  return callApi(App.GetGroupList)
}

/**
 * 添加群组
 * Go: AddGroup(group data.Group) string
 * @param {Object} group - 群组对象
 */
export async function addGroup(group) {
  return callApi(App.AddGroup, group)
}

/**
 * 删除群组
 * Go: RemoveGroup(groupId int) string
 * @param {number} groupId - 群组ID
 */
export async function removeGroup(groupId) {
  return callApi(App.RemoveGroup, groupId)
}

/**
 * 添加股票到群组
 * Go: AddStockGroup(groupId int, stockCode string) string
 * @param {number} groupId - 群组ID
 * @param {string} stockCode - 股票代码
 */
export async function addStockToGroup(groupId, stockCode) {
  return callApi(App.AddStockGroup, groupId, stockCode)
}

/**
 * 从群组移除股票
 * Go: RemoveStockGroup(code, name string, groupId int) string
 * @param {string} code - 股票代码
 * @param {string} name - 股票名称
 * @param {number} groupId - 群组ID
 */
export async function removeStockGroup(code, name, groupId) {
  return callApi(App.RemoveStockGroup, code, name, groupId)
}

/**
 * 更新群组排序
 * Go: UpdateGroupSort(id int, newSort int) bool
 * @param {number} id - 群组ID
 * @param {number} newSort - 新排序值
 */
export async function updateGroupSort(id, newSort) {
  return callApi(App.UpdateGroupSort, id, newSort)
}

/**
 * 初始化群组排序
 * Go: InitializeGroupSort() bool
 */
export async function initializeGroupSort() {
  return callApi(App.InitializeGroupSort)
}

// ========== K 线相关 ==========

/**
 * 获取股票 K 线数据
 * Go: GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData
 * @param {string} stockCode - 股票代码
 * @param {string} stockName - 股票名称
 * @param {number} days - 天数
 */
export async function getStockKLine(stockCode, stockName, days) {
  return callApi(App.GetStockKLine, stockCode, stockName, days)
}

/**
 * 获取分钟价格线数据
 * Go: GetStockMinutePriceLineData(stockCode, stockName string) map[string]any
 * @param {string} stockCode - 股票代码
 * @param {string} stockName - 股票名称
 */
export async function getStockMinutePriceLineData(stockCode, stockName) {
  return callApi(App.GetStockMinutePriceLineData, stockCode, stockName)
}

/**
 * 获取东财 K 线数据
 * Go: GetStockEastMoneyKLine(stockCode string, period string, count int) *[]data.KLineData
 */
export async function getEastMoneyKLine(stockCode, period = 'day', count = 100) {
  return callApi(App.GetStockEastMoneyKLine, stockCode, period, count)
}

/**
 * 获取 K 线数据（带降级）
 */
export async function getStockKLineWithFallback(stockCode, period = 'day', count = 100) {
  return callApi(App.GetStockKLineWithFallback, stockCode, period, count)
}

// ========== 交易设置 ==========

/**
 * 设置成本价和持仓量
 * Go: SetCostPriceAndVolume(stockCode string, price float64, volume int64) string
 */
export async function setCostPriceAndVolume(stockCode, price, volume) {
  return callApi(App.SetCostPriceAndVolume, stockCode, price, volume)
}

/**
 * 设置预警涨跌幅
 * Go: SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string
 * @param {number} val - 涨跌幅值
 * @param {number} alarmPrice - 预警价格
 * @param {string} stockCode - 股票代码
 */
export async function setAlarmChangePercent(val, alarmPrice, stockCode) {
  return callApi(App.SetAlarmChangePercent, val, alarmPrice, stockCode)
}

/**
 * 设置股票排序
 * Go: SetStockSort(sort int64, stockCode string)
 * @param {number} sort - 排序值
 * @param {string} stockCode - 股票代码
 */
export async function setStockSort(sort, stockCode) {
  return callApi(App.SetStockSort, sort, stockCode)
}

/**
 * 设置 AI 定时任务
 * Go: SetStockAICron(cronText, stockCode string)
 * @param {string} cronText - cron 表达式
 * @param {string} stockCode - 股票代码
 */
export async function setStockAICron(cronText, stockCode) {
  return callApi(App.SetStockAICron, cronText, stockCode)
}

/**
 * 设置交易价格
 * Go: SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string
 */
export async function setTradingPrice(stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice) {
  return callApi(App.SetTradingPrice, stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
}

// ========== AI 分析 ==========

/**
 * 发起新的 AI 聊天流
 * Go: NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools, think bool, agentMode, strategyCode string)
 */
export async function newChatStream(stock, stockCode, question, aiConfigId, sysPromptId, enableTools, think, agentMode, strategyCode) {
  return callApi(App.NewChatStream, stock, stockCode, question, aiConfigId, sysPromptId, enableTools, think, agentMode, strategyCode)
}

/**
 * 获取 AI 响应结果
 * Go: GetAIResponseResult(stock string) *models.AIResponseResult
 */
export async function getAIResponseResult(stock) {
  return callApi(App.GetAIResponseResult, stock)
}

/**
 * 获取有效赞助商 VIP 信息
 * Go: GetEffectiveSponsorVip() map[string]any
 */
export async function getEffectiveSponsorVip() {
  return callApi(App.GetEffectiveSponsorVip)
}

// ========== 保存/分享 ==========

/**
 * 保存图片
 * Go: SaveImage(name, base64Data string) string
 */
export async function saveImage(name, base64Data) {
  return callApi(App.SaveImage, name, base64Data)
}

/**
 * 保存 Word 文件
 * Go: SaveWordFile(filename string, base64Data string) string
 */
export async function saveWordFile(filename, base64Data) {
  return callApi(App.SaveWordFile, filename, base64Data)
}

/**
 * 发送钉钉消息（按类型）
 * Go: SendDingDingMessageByType(message string, stockCode string, msgType int) string
 */
export async function sendDingDingMessageByType(message, stockCode, msgType) {
  return callApi(App.SendDingDingMessageByType, message, stockCode, msgType)
}

// ========== 技术指标 ==========

/**
 * 获取筹码分布
 */
export async function getChipDistribution(stockCode) {
  return callApi(App.GetChipDistribution, stockCode)
}

/**
 * 获取公司信息
 */
export async function getCompanyInfo(stockCode) {
  return callApi(App.GetTdxCompanyInfo, stockCode)
}

// ========== 搜索 ==========

/**
 * 搜索股票
 */
export async function searchStock(keyword) {
  return callApi(App.SearchStock, keyword)
}

export default {
  // 自选股
  greet,
  follow,
  unFollow,
  getFollowList,
  getStockList,

  // 群组
  getGroupList,
  addGroup,
  removeGroup,
  addStockToGroup,
  removeStockGroup,
  updateGroupSort,
  initializeGroupSort,

  // K 线
  getStockKLine,
  getStockMinutePriceLineData,
  getEastMoneyKLine,
  getStockKLineWithFallback,

  // 交易设置
  setCostPriceAndVolume,
  setAlarmChangePercent,
  setStockSort,
  setStockAICron,
  setTradingPrice,

  // AI 分析
  newChatStream,
  getAIResponseResult,
  getEffectiveSponsorVip,

  // 保存/分享
  saveImage,
  saveWordFile,
  sendDingDingMessageByType,

  // 技术指标
  getChipDistribution,
  getCompanyInfo,

  // 搜索
  searchStock,
}
