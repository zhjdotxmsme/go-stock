/**
 * API 统一入口
 * 导出所有 API 模块
 */

export * as client from './client'
export * as stock from './stock'
export * as market from './market'
export * as system from './system'

import stockApi from './stock'
import marketApi from './market'
import systemApi from './system'

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
}

export default api
