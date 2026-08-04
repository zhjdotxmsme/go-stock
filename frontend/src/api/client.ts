/**
 * API 客户端基础封装
 * 统一处理 Wails Go 绑定调用，提供统一的错误处理和日志
 */

import { logger } from '../utils/logger'
import type { ApiResult } from '../types/api'

/**
 * 创建 API 调用结果
 * @param {*} data - 成功数据
 * @param {Error} error - 错误对象
 * @returns {ApiResult} API 结果
 */
function createResult<T = any>(data: T | null, error: Error | null = null): ApiResult<T> {
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
export async function callApi<T = any>(
  method: (...args: any[]) => Promise<T>,
  ...args: any[]
): Promise<ApiResult<T>> {
  try {
    // logger.debug(`[API] Calling ${method.name}`, ...args)
    const result = await method(...args)
    // logger.debug(`[API] ${method.name} success`, result)
    return createResult<T>(result)
  } catch (error) {
    logger.error(`[API] ${method?.name || 'unknown'} error`, error)
    return createResult<T>(null, error as Error)
  }
}

/** Wails 绑定方法类型 */
type WailsMethod = (...args: any[]) => Promise<any>

/** 封装后的 API 客户端方法类型 */
export type ApiClientMethod<M extends WailsMethod> = (
  ...args: Parameters<M>
) => Promise<ApiResult<Awaited<ReturnType<M>>>>

/** 封装后的 API 客户端类型 */
export type ApiClient<B> = {
  [K in keyof B]: B[K] extends WailsMethod ? ApiClientMethod<B[K]> : never
}

/**
 * 创建 API 代理，自动绑定错误处理
 * @param {Object} bindings - Wails 绑定对象
 * @returns {Object} 封装后的 API 对象
 */
export function createApiClient<B extends Record<string, WailsMethod>>(
  bindings: B
): ApiClient<B> {
  const client: Record<string, any> = {}
  for (const [name, method] of Object.entries(bindings)) {
    if (typeof method === 'function') {
      client[name] = async (...args: any[]) => callApi(method, ...args)
    }
  }
  return client as ApiClient<B>
}

export default {
  callApi,
  createApiClient,
}
