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
export async function greet(stockCode: string): Promise<any> {
  return callApi(App.Greet, stockCode)
}

/**
 * 关注股票
 * Go: Follow(stockCode string) string
 * @param {string} stockCode - 股票代码
 */
export async function follow(stockCode: string): Promise<any> {
  return callApi(App.Follow, stockCode)
}

/**
 * 取消关注
 * Go: UnFollow(stockCode string) string
 * @param {string} stockCode - 股票代码
 */
export async function unFollow(stockCode: string): Promise<any> {
  return callApi(App.UnFollow, stockCode)
}

/**
 * 获取自选股列表
 * Go: GetFollowList(groupId int) *[]data.FollowedStock
 * @param {number} groupId - 群组ID
 */
export async function getFollowList(groupId: number): Promise<any> {
  return callApi(App.GetFollowList, groupId)
}

/**
 * 获取股票列表
 * Go: GetStockList(key string) []data.StockBasic
 * @param {string} key - 搜索关键词
 */
export async function getStockList(key: string = ''): Promise<any> {
  return callApi(App.GetStockList, key)
}

// ========== 群组相关 ==========

/**
 * 获取群组列表
 * Go: GetGroupList() []data.Group
 */
export async function getGroupList(): Promise<any> {
  return callApi(App.GetGroupList)
}

/**
 * 添加群组
 * Go: AddGroup(group data.Group) string
 * @param {Object} group - 群组对象
 */
export async function addGroup(group: any): Promise<any> {
  return callApi(App.AddGroup, group)
}

/**
 * 删除群组
 * Go: RemoveGroup(groupId int) string
 * @param {number} groupId - 群组ID
 */
export async function removeGroup(groupId: number): Promise<any> {
  return callApi(App.RemoveGroup, groupId)
}

/**
 * 添加股票到群组
 * Go: AddStockGroup(groupId int, stockCode string) string
 * @param {number} groupId - 群组ID
 * @param {string} stockCode - 股票代码
 */
export async function addStockToGroup(groupId: number, stockCode: string): Promise<any> {
  return callApi(App.AddStockGroup, groupId, stockCode)
}

/**
 * 从群组移除股票
 * Go: RemoveStockGroup(code, name string, groupId int) string
 * @param {string} code - 股票代码
 * @param {string} name - 股票名称
 * @param {number} groupId - 群组ID
 */
export async function removeStockGroup(code: string, name: string, groupId: number): Promise<any> {
  return callApi(App.RemoveStockGroup, code, name, groupId)
}

/**
 * 更新群组排序
 * Go: UpdateGroupSort(id int, newSort int) bool
 * @param {number} id - 群组ID
 * @param {number} newSort - 新排序值
 */
export async function updateGroupSort(id: number, newSort: number): Promise<any> {
  return callApi(App.UpdateGroupSort, id, newSort)
}

/**
 * 初始化群组排序
 * Go: InitializeGroupSort() bool
 */
export async function initializeGroupSort(): Promise<any> {
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
export async function getStockKLine(stockCode: string, stockName: string, days: number): Promise<any> {
  return callApi(App.GetStockKLine, stockCode, stockName, days)
}

/**
 * 获取分钟价格线数据
 * Go: GetStockMinutePriceLineData(stockCode, stockName string) map[string]any
 * @param {string} stockCode - 股票代码
 * @param {string} stockName - 股票名称
 */
export async function getStockMinutePriceLineData(stockCode: string, stockName: string): Promise<any> {
  return callApi(App.GetStockMinutePriceLineData, stockCode, stockName)
}

/**
 * 获取东财 K 线数据
 * Go: GetStockEastMoneyKLine(stockCode string, period string, count int) *[]data.KLineData
 */
export async function getEastMoneyKLine(stockCode: string, period: string = 'day', count: number = 100): Promise<any> {
  return callApi(App.GetStockEastMoneyKLine, stockCode, period, count)
}

/**
 * 获取 K 线数据（带降级）
 */
export async function getStockKLineWithFallback(stockCode: string, period: string = 'day', count: number = 100): Promise<any> {
  return callApi(App.GetStockKLineWithFallback, stockCode, period, count)
}

// ========== 交易设置 ==========

/**
 * 设置成本价和持仓量
 * Go: SetCostPriceAndVolume(stockCode string, price float64, volume int64) string
 */
export async function setCostPriceAndVolume(stockCode: string, price: number, volume: number): Promise<any> {
  return callApi(App.SetCostPriceAndVolume, stockCode, price, volume)
}

/**
 * 设置预警涨跌幅
 * Go: SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string
 * @param {number} val - 涨跌幅值
 * @param {number} alarmPrice - 预警价格
 * @param {string} stockCode - 股票代码
 */
export async function setAlarmChangePercent(val: number, alarmPrice: number, stockCode: string): Promise<any> {
  return callApi(App.SetAlarmChangePercent, val, alarmPrice, stockCode)
}

/**
 * 设置股票排序
 * Go: SetStockSort(sort int64, stockCode string)
 * @param {number} sort - 排序值
 * @param {string} stockCode - 股票代码
 */
export async function setStockSort(sort: number, stockCode: string): Promise<any> {
  return callApi(App.SetStockSort, sort, stockCode)
}

/**
 * 设置 AI 定时任务
 * Go: SetStockAICron(cronText, stockCode string)
 * @param {string} cronText - cron 表达式
 * @param {string} stockCode - 股票代码
 */
export async function setStockAICron(cronText: string, stockCode: string): Promise<any> {
  return callApi(App.SetStockAICron, cronText, stockCode)
}

/**
 * 设置交易价格
 * Go: SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string
 */
export async function setTradingPrice(stockCode: string, entryPrice: number, takeProfitPrice: number, stopLossPrice: number, costPrice: number): Promise<any> {
  return callApi(App.SetTradingPrice, stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
}

// ========== AI 分析 ==========

/**
 * 发起新的 AI 聊天流
 * Go: NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools, think bool, agentMode, strategyCode string)
 */
export async function newChatStream(stock: string, stockCode: string, question: string, aiConfigId: number, sysPromptId: any, enableTools: boolean, think: boolean, agentMode: string, strategyCode: string): Promise<any> {
  return callApi(App.NewChatStream, stock, stockCode, question, aiConfigId, sysPromptId, enableTools, think, agentMode, strategyCode)
}

/**
 * 获取 AI 响应结果
 * Go: GetAIResponseResult(stock string) *models.AIResponseResult
 */
export async function getAIResponseResult(stock: string): Promise<any> {
  return callApi(App.GetAIResponseResult, stock)
}

/**
 * 获取有效赞助商 VIP 信息
 * Go: GetEffectiveSponsorVip() map[string]any
 */
export async function getEffectiveSponsorVip(): Promise<any> {
  return callApi(App.GetEffectiveSponsorVip)
}

// ========== 保存/分享 ==========

/**
 * 保存图片
 * Go: SaveImage(name, base64Data string) string
 */
export async function saveImage(name: string, base64Data: string): Promise<any> {
  return callApi(App.SaveImage, name, base64Data)
}

/**
 * 保存 Word 文件
 * Go: SaveWordFile(filename string, base64Data string) string
 */
export async function saveWordFile(filename: string, base64Data: string): Promise<any> {
  return callApi(App.SaveWordFile, filename, base64Data)
}

/**
 * 发送钉钉消息（按类型）
 * Go: SendDingDingMessageByType(message string, stockCode string, msgType int) string
 */
export async function sendDingDingMessageByType(message: string, stockCode: string, msgType: number): Promise<any> {
  return callApi(App.SendDingDingMessageByType, message, stockCode, msgType)
}

// ========== 技术指标 ==========

/**
 * 获取筹码分布
 */
export async function getChipDistribution(stockCode: string): Promise<any> {
  return callApi(App.GetChipDistribution, stockCode)
}

/**
 * 获取公司信息
 */
export async function getCompanyInfo(stockCode: string): Promise<any> {
  return callApi(App.GetTdxCompanyInfo, stockCode)
}

// ========== 搜索 ==========

/**
 * 搜索股票
 */
export async function searchStock(keyword: string): Promise<any> {
  return callApi(App.SearchStock, keyword)
}

// ========== 东财K线分页 ==========

/**
 * 获取东财K线数据（分页）
 * Go: GetStockEastMoneyKLinePage(code, period string, count, fields int)
 */
export async function getStockEastMoneyKLinePage(code: string, period: string, count: number, fields: number): Promise<any> {
  return callApi(App.GetStockEastMoneyKLinePage, code, period, count, fields)
}

/**
 * 获取K线数据分页（带降级）
 * Go: GetStockKLinePageWithFallback
 */
export async function getStockKLinePageWithFallback(...args: any[]): Promise<any> {
  return callApi(App.GetStockKLinePageWithFallback, ...args)
}

// ========== 全量股票 ==========

/**
 * 获取全量股票（分页）
 * Go: GetAllStocks(page, pageSize int, key string, indicators map[string]bool)
 */
export async function getAllStocks(page: number, pageSize: number, key: string, indicators: any): Promise<any> {
  return callApi(App.GetAllStocks, page, pageSize, key, indicators)
}

/**
 * 获取全量股票信息列表
 * Go: GetAllStockInfoList(query data.StockInfoListQuery) data.StockInfoPageData
 */
export async function getAllStockInfoList(query: any): Promise<any> {
  return callApi(App.GetAllStockInfoList, query)
}

/**
 * 获取全量股票信息（按ID）
 * Go: GetAllStockInfoById(id int) data.AllStockInfo
 */
export async function getAllStockInfoById(id: number): Promise<any> {
  return callApi(App.GetAllStockInfoById, id)
}

/**
 * 获取全量市场列表
 * Go: GetAllMarkets() []string
 */
export async function getAllMarkets(): Promise<any> {
  return callApi(App.GetAllMarkets)
}

/**
 * 获取全量行业列表
 * Go: GetAllIndustries() []string
 */
export async function getAllIndustries(): Promise<any> {
  return callApi(App.GetAllIndustries)
}

/**
 * 获取全量概念列表
 * Go: GetAllConcepts() []string
 */
export async function getAllConcepts(): Promise<any> {
  return callApi(App.GetAllConcepts)
}

/**
 * 获取群组股票列表
 * Go: GetGroupStockList(groupId int) []data.GroupStock
 */
export async function getGroupStockList(groupId: number): Promise<any> {
  return callApi(App.GetGroupStockList, groupId)
}

/**
 * 获取通用K线数据
 * Go: GetStockCommonKLine(code, name string, days int)
 */
export async function getStockCommonKLine(code: string, name: string, days: number): Promise<any> {
  return callApi(App.GetStockCommonKLine, code, name, days)
}

// ========== 集合竞价 ==========

/**
 * 获取集合竞价数据
 * Go: GetTdxCallAuction(code string, market, type int)
 */
export async function getTdxCallAuction(code: string, market: number, type: number): Promise<any> {
  return callApi(App.GetTdxCallAuction, code, market, type)
}

// ========== 自定义策略 ==========

/**
 * 获取所有自定义策略
 * Go: GetAllCustomStrategies() []*data.CustomStrategy
 */
export async function getAllCustomStrategies(): Promise<any> {
  return callApi(App.GetAllCustomStrategies)
}

/**
 * 保存自定义策略
 * Go: SaveCustomStrategy(strategy *data.CustomStrategy) error
 */
export async function saveCustomStrategy(strategy: any): Promise<any> {
  return callApi(App.SaveCustomStrategy, strategy)
}

/**
 * 删除自定义策略
 * Go: DeleteCustomStrategy(id int) error
 */
export async function deleteCustomStrategy(id: number): Promise<any> {
  return callApi(App.DeleteCustomStrategy, id)
}

/**
 * AI配置选股
 * Go: AIConfiguredStockPick(code string, count int)
 */
export async function aiConfiguredStockPick(code: string, count: number): Promise<any> {
  return callApi(App.AIConfiguredStockPick, code, count)
}

// ========== 数据管理 ==========

/**
 * 批量删除全量股票信息
 * Go: BatchDeleteAllStockInfo() string
 */
export async function batchDeleteAllStockInfo(): Promise<any> {
  return callApi(App.BatchDeleteAllStockInfo)
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
  getStockEastMoneyKLinePage,
  getStockKLinePageWithFallback,
  getStockCommonKLine,

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
  aiConfiguredStockPick,

  // 保存/分享
  saveImage,
  saveWordFile,
  sendDingDingMessageByType,

  // 技术指标
  getChipDistribution,
  getCompanyInfo,

  // 搜索
  searchStock,

  // 全量股票
  getAllStocks,
  getAllStockInfoList,
  getAllStockInfoById,
  getAllMarkets,
  getAllIndustries,
  getAllConcepts,
  getGroupStockList,

  // 集合竞价
  getTdxCallAuction,

  // 自定义策略
  getAllCustomStrategies,
  saveCustomStrategy,
  deleteCustomStrategy,

  // 数据管理
  batchDeleteAllStockInfo,
}
