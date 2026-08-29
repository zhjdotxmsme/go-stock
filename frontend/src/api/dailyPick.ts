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
  const r = await callApi(DailyPickHandler.RunDailyPick, tradeDate, topN)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
}

/**
 * 异步执行每日选股（进度经 dailyPickProgress 事件推送）
 */
export async function runDailyPickAsync(tradeDate: string, topN: number): Promise<any> {
  const r = await callApi(DailyPickHandler.RunDailyPickAsync, tradeDate, topN)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
}

/**
 * 执行次日复盘
 */
export async function runDailyReview(reviewDate: string, pickDate: string): Promise<any> {
  const r = await callApi(DailyPickHandler.RunDailyReview, reviewDate, pickDate)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
}

/**
 * 分页查询选股结果
 */
export async function getDailyPicks(query: any): Promise<any> {
  const r = await callApi(DailyPickHandler.GetDailyPicks, query)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
}

/**
 * 选股统计
 */
export async function getDailyPickStats(): Promise<any> {
  const r = await callApi(DailyPickHandler.GetDailyPickStats)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
}

/**
 * 更新选股备注
 */
export async function updateDailyPickRemarks(id: number, remarks: string): Promise<any> {
  const r = await callApi(DailyPickHandler.UpdateDailyPickRemarks, id, remarks)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
}

/**
 * 复盘趋势序列
 */
export async function getReviewTrend(limit: number): Promise<any> {
  const r = await callApi(DailyPickHandler.GetReviewTrend, limit)
  if (!r?.success) throw new Error(r?.message || "调用失败")
  return r.data
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
