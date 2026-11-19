export function observationLabel(row: { status?: string; dataStatus?: string; rejectReason?: string }): string {
  if (row.dataStatus === 'intraday') return '盘中观察'
  if (row.dataStatus === 'invalid') return '数据无效'
  if (row.dataStatus === 'not_backtestable') return '不可回测'
  if (row.status === '已确认') return '已确认'
  if (row.status === '已过期') return '观察过期'
  if (row.rejectReason?.includes('跳空')) return '过度跳空'
  return '观察中'
}

export function huntingStatusType(status?: string): 'success' | 'info' | 'default' {
  if (status === '持仓中') return 'success'
  if (status === '已退出') return 'info'
  return 'default'
}

export function backtestSampleWarning(row: { confirmedCount?: number }): string {
  return (row.confirmedCount ?? 0) < 100
    ? '样本外确认交易少于100笔，样本不足，不用于替换默认策略'
    : ''
}
