/**
 * 狩猎场（KHunter）相关 API
 * 封装 KhunterHandler 的 Wails 绑定调用
 */

import { callApi } from './client'
import * as KhunterHandler from '../../wailsjs/go/handler/KhunterHandler'

/**
 * 异步执行狩猎场流水线（结果经 khunter:progress 事件推送）
 */
export async function runPipeline(tradeDate: string): Promise<any> {
  const r = await callApi(KhunterHandler.RunPipelineAsync, tradeDate)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data
}

/**
 * 五维评分排行
 */
export async function getScores(date: string): Promise<any[]> {
  const r = await callApi(KhunterHandler.GetScores, date)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? []
}

/**
 * 策略信号
 */
export async function getSignals(date: string): Promise<any[]> {
  const r = await callApi(KhunterHandler.GetSignals, date)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? []
}

/**
 * 狩猎场追踪列表（status: 追踪中 / 已移除 / 空=全部）
 */
export async function getHunting(status: string): Promise<any[]> {
  const r = await callApi(KhunterHandler.GetHunting, status)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? []
}

/**
 * 当前风险档位
 */
export async function getRiskLevel(): Promise<any> {
  const r = await callApi(KhunterHandler.GetRiskLevel)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? null
}

/**
 * 异步信号级回测（进度经 khunter:backtest_progress，结果经 khunter:backtest_done 推送）
 */
export async function runBacktest(codes: string[], startDate: string, endDate: string, holdingDays: number): Promise<any> {
  const r = await callApi(KhunterHandler.RunBacktestAsync, codes, startDate, endDate, holdingDays)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data
}

export default {
  runPipeline,
  getScores,
  getSignals,
  getHunting,
  getRiskLevel,
  runBacktest,
}
