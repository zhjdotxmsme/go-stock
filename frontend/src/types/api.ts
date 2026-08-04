/** API 调用结果封装 */
export interface ApiResult<T = any> {
  success: boolean
  data: T | null
  error: Error | null
  message: string
}

/** 分页查询结果 */
export interface PaginatedResult<T = any> {
  list: T[]
  total: number
  page: number
  pageSize: number
}
