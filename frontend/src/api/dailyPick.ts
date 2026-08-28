/**
 * 每日选股相关 API
 * 封装 DailyPickHandler 的 Wails 绑定调用
 */

import { callApi } from './client'
import * as DailyPickHandler from '../../wailsjs/go/handler/DailyPickHandler'

/**
 * 同步执行每日选股
 */
export async function runDailyPick(tradeDate: string, topN: number): Promise<any> {
  return callApi(DailyPickHandler.RunDailyPick, tradeDate, topN)
}

/**
 * 异步执行每日选股（进度经 dailyPickProgress 事件推送）
 */
export async function runDailyPickAsync(tradeDate: string, topN: number): Promise<any> {
  return callApi(DailyPickHandler.RunDailyPickAsync, tradeDate, topN)
}

/**
 * 执行次日复盘
 */
export async function runDailyReview(reviewDate: string, pickDate: string): Promise<any> {
  return callApi(DailyPickHandler.RunDailyReview, reviewDate, pickDate)
}

/**
 * 分页查询选股结果
 */
export async function getDailyPicks(query: any): Promise<any> {
  return callApi(DailyPickHandler.GetDailyPicks, query)
}

/**
 * 选股统计
 */
export async function getDailyPickStats(): Promise<any> {
  return callApi(DailyPickHandler.GetDailyPickStats)
}

/**
 * 更新选股备注
 */
export async function updateDailyPickRemarks(id: number, remarks: string): Promise<any> {
  return callApi(DailyPickHandler.UpdateDailyPickRemarks, id, remarks)
}

/**
 * 复盘趋势序列
 */
export async function getReviewTrend(limit: number): Promise<any> {
  return callApi(DailyPickHandler.GetReviewTrend, limit)
}

export default {
  runDailyPick,
  runDailyPickAsync,
  runDailyReview,
  getDailyPicks,
  getDailyPickStats,
  updateDailyPickRemarks,
  getReviewTrend,
}
