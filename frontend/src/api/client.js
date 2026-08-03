/**
 * API 客户端基础封装
 * 统一处理 Wails Go 绑定调用，提供统一的错误处理和日志
 */

import { logger } from '../utils/logger'

/**
 * API 调用结果封装
 * @typedef {Object} ApiResult
 * @property {boolean} success - 是否成功
 * @property {*} data - 返回数据
 * @property {Error} error - 错误对象
 * @property {string} message - 错误消息
 */

/**
 * 创建 API 调用结果
 * @param {*} data - 成功数据
 * @param {Error} error - 错误对象
 * @returns {ApiResult} API 结果
 */
function createResult(data, error = null) {
  return {
    success: !error,
    data,
    error,
    message: error?.message || '',
  }
}

/**
 * 调用 Wails Go 绑定方法
 * @param {Function} method - Wails 绑定方法
 * @param {...*} args - 方法参数
 * @returns {Promise<ApiResult>} API 结果
 */
export async function callApi(method, ...args) {
  try {
    // logger.debug(`[API] Calling ${method.name}`, ...args)
    const result = await method(...args)
    // logger.debug(`[API] ${method.name} success`, result)
    return createResult(result)
  } catch (error) {
    logger.error(`[API] ${method?.name || 'unknown'} error`, error)
    return createResult(null, error)
  }
}

/**
 * 创建 API 代理，自动绑定错误处理
 * @param {Object} bindings - Wails 绑定对象
 * @returns {Object} 封装后的 API 对象
 */
export function createApiClient(bindings) {
  const client = {}
  for (const [name, method] of Object.entries(bindings)) {
    if (typeof method === 'function') {
      client[name] = async (...args) => callApi(method, ...args)
    }
  }
  return client
}

export default {
  callApi,
  createApiClient,
}
