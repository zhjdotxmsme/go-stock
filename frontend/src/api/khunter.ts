/**
 * 狩猎场（KHunter）相关 API
 * 封装 KhunterHandler 的 Wails 绑定调用
 */

import { callApi } from './client'
import * as KhunterHandler from '../../wailsjs/go/handler/KhunterHandler'
import * as SystemHandler from '../../wailsjs/go/handler/SystemHandler'
import type { models, khunter } from '../../wailsjs/go/models'

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
/** 最新有数据的评分日 / 信号日（用于默认日期） */
export async function getKhunterMeta(): Promise<{ latestScoreDate?: string; latestSignalDate?: string; latestRunDate?: string }> {
  const r = await callApi(KhunterHandler.GetMeta)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? {}
}

export async function getScores(date: string): Promise<any[]> {
  const r = await callApi(KhunterHandler.GetScores, date)
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

export async function getRunSummaries(limit = 30): Promise<models.KhunterRunSummary[]> {
  const r = await callApi(KhunterHandler.GetRunSummaries, limit)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? []
}

export interface TechAnalysisSection {
  title: string
  text: string
}

export interface TechAnalysisProfile {
  trend?: string
  volume?: string
  position?: string
  momentum?: string
  sections?: TechAnalysisSection[]
}

/**
 * 任意个股的狩猎场同口径技术结论；tradeDate 为空表示今天。
 */
export async function getTechAnalysis(stockCode: string, tradeDate = ''): Promise<TechAnalysisProfile | null> {
  const r = await callApi(KhunterHandler.GetTechAnalysis, stockCode, tradeDate)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? null
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
 * 各策略半凯利建议仓位（配置缺失返回空数组）
 */
export async function getKellySuggestions(): Promise<any[]> {
  const r = await callApi(KhunterHandler.GetKellySuggestions)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? []
}

/**
 * 异步信号级回测（进度经 khunter:backtest_progress，结果经 khunter:backtest_done 推送）
 */
export async function runBacktest(codes: string[], startDate: string, endDate: string, holdingDays: number): Promise<any> {
  const r = await callApi(KhunterHandler.RunBacktestAsync, codes, startDate, endDate, holdingDays)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data
}

// ===== Task 11 新 API：研究流水线 / 观察列表 / 全链路回测 / 同步就绪诊断 =====

/**
 * 异步历史研究运行（只写研究记录、不写生产表）；
 * 结果经 khunter:research_progress 推送（result.readiness 携带同步诊断）
 */
export async function runResearchPipeline(tradeDate: string): Promise<any> {
  const r = await callApi(KhunterHandler.RunResearchPipelineAsync, tradeDate)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data
}

/**
 * 观察候选列表（status 为空返回全部）；观察不是买入推荐
 */
export async function getObservations(status: string): Promise<models.KhunterObservation[]> {
  const r = await callApi(KhunterHandler.GetObservations, status)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data ?? []
}

/** 日K同步就绪诊断（确认前置条件展示入口）；tradeDate 为空取最近交易日 */
export async function getKLineSyncReadiness(tradeDate: string): Promise<khunter.DataReadiness> {
  const r = await callApi(SystemHandler.GetKLineSyncReadiness, tradeDate)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data
}

/** 全链路回测输入凭证（四组共享，保证严格对照） */
export interface FullBacktestManifest {
  codes?: string[] | null
  startDate?: string
  endDate?: string
  costHash?: string
  initialCash?: number
  universe?: string
  useDataFix?: boolean
  useRiskFilter?: boolean
  useConfirmExit?: boolean
  approximate?: boolean
  notBacktestable?: boolean
}

/** 单笔成交（含滑点成交价，可复算净收益） */
export interface FullBacktestTrade {
  code?: string
  signalDate?: string
  entryDate?: string
  exitDate?: string
  entryPrice?: number
  exitPrice?: number
  shares?: number
  grossReturn?: number
  netReturn?: number
  holdingDays?: number
  exitReason?: string
}

/** 尾部风险与汇总指标 */
export interface FullBacktestMetrics {
  tradeCount?: number
  invalidCount?: number
  untradeableCount?: number
  zeroReturnCount?: number
  winRate?: number
  avgNetReturn?: number
  avgLoss?: number
  worstDecileAvg?: number
  maxSingleLoss?: number
  portfolioMaxDD?: number
  fiveDayAvgReturn?: number
  profitLossRatio?: number
  benchmarkReturn?: number
  excessReturn?: number
  sampleInsufficient?: boolean
}

/** 失败样本计数守恒：signalCount = confirmed + expired + invalid + untradeable + stillInWindow */
export interface FullBacktestReport {
  signalCount?: number
  confirmedCount?: number
  invalidCount?: number
  untradeableCount?: number
  expiredCount?: number
  stillInWindowCount?: number
  stillInWindowReasons?: string[]
  unfinished?: boolean
  tradeCount?: number
  trades?: FullBacktestTrade[]
  invalidReasons?: string[]
  untradeableReasons?: string[]
  portfolioEquity?: number[]
  limitations?: string[]
  metrics?: FullBacktestMetrics
}

export interface FullBacktestSplitRange {
  phase?: string
  warmupStart?: string
  signalStart?: string
  signalEnd?: string
  exitDataEnd?: string
}

/** 单组对照（变体）结果 */
export interface FullBacktestVariant {
  variant?: string
  manifest?: FullBacktestManifest
  report?: FullBacktestReport
  splits?: { split?: FullBacktestSplitRange; report?: FullBacktestReport }[]
  oosConfirmed?: number
  sampleInsufficient?: boolean
  stable?: boolean
  stableNote?: string
  unfinished?: boolean
  notBacktestableCodes?: string[]
}

export interface FullBacktestReplayEvent {
  date?: string
  variant?: string
  code?: string
  stage?: string
  detail?: string
  price?: number
  protectPrice?: number
  shares?: number
}

export interface FullBacktestCaseReplay {
  code?: string
  events?: FullBacktestReplayEvent[]
}

/** khunter.FullBacktestResult：全链路四组对照回测结果（khunter:full_backtest_done 事件的 result） */
export interface FullBacktestResult {
  startDate?: string
  endDate?: string
  allMarket?: boolean
  codes?: string[]
  costHash?: string
  splits?: FullBacktestSplitRange[]
  variants?: FullBacktestVariant[]
  caseReplays?: FullBacktestCaseReplay[]
  limitations?: string[]
}

/**
 * 异步全链路四组对照回测（绝不写生产狩猎池）；
 * codes 为空 = 全市场；结果经 khunter:full_backtest_done 推送
 */
export async function runFullBacktest(codes: string[], startDate: string, endDate: string): Promise<any> {
  const r = await callApi(KhunterHandler.RunFullBacktestAsync, codes, startDate, endDate)
  if (!r?.success) throw new Error(r?.message || '调用失败')
  return r.data
}

export default {
  runPipeline,
  getKhunterMeta,
  getScores,
  getHunting,
  getRunSummaries,
  getTechAnalysis,
  getRiskLevel,
  getKellySuggestions,
  runBacktest,
  runResearchPipeline,
  getObservations,
  getKLineSyncReadiness,
  runFullBacktest,
}
