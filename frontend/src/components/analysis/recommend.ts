// 推荐视图纯函数：与服务端口径一致（后端 round2），便于单测。

export const ANALYSIS_TABS = ['recommend', 'khunter', 'workbench', 'kline', 'pipeline'] as const
export type AnalysisTab = (typeof ANALYSIS_TABS)[number]

/** 混分：w<=0 或缺失时保持原分；保留 2 位小数（与 Go round2 一致）。 */
export function blendedScore(score: number, factorScore: number, weight: number): number {
  if (!weight || weight <= 0) return Math.round(score * 100) / 100
  return Math.round((score * (1 - weight) + factorScore * weight) * 100) / 100
}

export function tabFromQuery(v: unknown): AnalysisTab {
  return ANALYSIS_TABS.includes(v as AnalysisTab) ? (v as AnalysisTab) : 'pipeline'
}

export interface RecommendRow {
  code: string
  name: string
  signalDate: string
  status: string
  dataStatus: string
  rejectReason?: string
  score: number
  factorScore?: number
  factorWeight?: number
  blendedScore: number
  hasFactor: boolean
  degraded?: boolean
  degradedReason?: string
}

export type SubtitleInput = Pick<RecommendRow, 'hasFactor' | 'degraded' | 'degradedReason' | 'rejectReason'>

/** 行内副标题：降级原因 / 数据闸门原因，二者互斥取一个非空。 */
export function rowSubtitle(row: SubtitleInput): string {
  if (row.hasFactor) return ''
  if (row.degraded) return row.degradedReason || '预测降级'
  return row.rejectReason || ''
}

/** 异常 → 可读文案。Wails 绑定抛出的值类型不确定，按 unknown 收窄。 */
export function errText(e: unknown): string {
  if (e instanceof Error) return e.message || e.name
  if (typeof e === 'string') return e
  if (e && typeof e === 'object' && 'message' in e) {
    const m = e.message
    if (typeof m === 'string' && m) return m
  }
  return '未知错误'
}

/** 门面状态：与后端 AnalysisCenterStatus 对齐（未启动时可选字段缺失）。 */
export interface AnalysisCenterStatus {
  kronosStatus: 'disabled' | 'offline' | 'loading' | 'online'
  pendingObservations: number
  factorEnabled: boolean
}

/** Kronos 状态徽标类型：仅 online 为 success。 */
export function kronosTagType(status: AnalysisCenterStatus | null): 'success' | 'warning' | 'default' {
  if (!status) return 'default'
  return status.kronosStatus === 'online' ? 'success' : 'warning'
}
