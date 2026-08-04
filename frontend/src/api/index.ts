/**
 * API 统一入口
 * 导出所有 API 模块
 */

export * as client from './client'
export * as stock from './stock'
export * as market from './market'
export * as system from './system'
export * as fund from './fund'
export * as commodity from './commodity'
export * as trade from './trade'

import stockApi from './stock'
import marketApi from './market'
import systemApi from './system'
import fundApi from './fund'
import commodityApi from './commodity'
import tradeApi from './trade'

/**
 * 统一的 API 对象
 * 使用方式:
 * import api from '@/api'
 * const result = await api.stock.getStockList()
 */
const api = {
  stock: stockApi,
  market: marketApi,
  system: systemApi,
  fund: fundApi,
  commodity: commodityApi,
  trade: tradeApi,
}

export default api
