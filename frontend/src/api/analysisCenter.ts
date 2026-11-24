/**
 * 统一分析中心 API：封装 AnalysisCenterHandler 的 Wails 绑定。
 * 门面方法签名为 (value, error)，Go 侧返回 error 时绑定 Promise 直接 reject。
 */

import {
  GetRecommendations,
  GetAnalysisStatus,
  GetForecast,
  SubmitPredictJob,
  GetPredictJob,
  RunPipeline,
  GetPipelineStatus,
  GetPipelineResult,
  GetPipelineDetail,
} from '../../wailsjs/go/handler/AnalysisCenterHandler'
import type { khunter, analysis } from '../../wailsjs/go/models'
import type { RecommendRow, AnalysisCenterStatus } from '../components/analysis/recommend'

export async function fetchRecommendations(tradeDate = '', topN = 30): Promise<RecommendRow[]> {
  return GetRecommendations(tradeDate, topN)
}

export async function fetchAnalysisStatus(): Promise<AnalysisCenterStatus> {
  const s = await GetAnalysisStatus()
  // 生成类的 kronosStatus 是 string，收窄为门面约定的状态联合
  return { ...s, kronosStatus: s.kronosStatus as AnalysisCenterStatus['kronosStatus'] }
}

export async function fetchForecast(code: string, signalDate: string): Promise<string> {
  return GetForecast(code, signalDate)
}

export async function submitPredictJob(code: string, predLen = 5, asOfDate = ''): Promise<khunter.PredictJob> {
  return SubmitPredictJob(code, predLen, asOfDate)
}

export async function fetchPredictJob(id: string): Promise<khunter.PredictJob> {
  return GetPredictJob(id)
}

// ========== 智能流水线 ==========

export async function runPipeline(tradeDate: string): Promise<void> {
  return RunPipeline(tradeDate)
}

export async function fetchPipelineStatus(): Promise<analysis.PipelineStatus> {
  return GetPipelineStatus()
}

export async function fetchPipelineResult(tradeDate: string, topN = 30): Promise<analysis.PipelineResult> {
  return GetPipelineResult(tradeDate, topN)
}

export async function fetchPipelineDetail(code: string, signalDate: string): Promise<analysis.PipelineDetail> {
  return GetPipelineDetail(code, signalDate)
}
