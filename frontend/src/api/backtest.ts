/**
 * 回测与数据同步相关 API
 * 封装 backtest.Service 与 service.DailyPickBacktestService 的 Wails 绑定调用
 */

import { callApi } from './client'
import * as BacktestService from '../../wailsjs/go/backtest/Service'
import * as DailyPickBacktestService from '../../wailsjs/go/service/DailyPickBacktestService'

// ========== 单次/批量回测 ==========

/**
 * 运行单次回测
 */
export async function runSingleBacktest(...args: any[]): Promise<any> {
  return callApi(BacktestService.RunSingleBacktest, ...args)
}

/**
 * 运行批量回测
 */
export async function runBatchBacktest(...args: any[]): Promise<any> {
  return callApi(BacktestService.RunBatchBacktest, ...args)
}

/**
 * 分页获取回测结果
 */
export async function getBacktestResults(...args: any[]): Promise<any> {
  return callApi(BacktestService.GetBacktestResults, ...args)
}

/**
 * 运行参数优化
 */
export async function runOptimization(...args: any[]): Promise<any> {
  return callApi(BacktestService.RunOptimization, ...args)
}

/**
 * 获取优化预设
 */
export async function getOptimizationPresets(): Promise<any> {
  return callApi(BacktestService.GetOptimizationPresets)
}

/**
 * 为每日选股运行回测
 */
export async function runBacktestForDailyPicks(...args: any[]): Promise<any> {
  return callApi(DailyPickBacktestService.RunBacktestForDailyPicks, ...args)
}

// ========== 历史数据同步 ==========

/**
 * 获取 K 线缓存统计
 */
export async function getKLineCacheStats(): Promise<any> {
  return callApi(BacktestService.GetKLineCacheStats)
}

/**
 * 启动历史数据同步
 */
export async function startHistoricalSync(...args: any[]): Promise<any> {
  return callApi(BacktestService.StartHistoricalSync, ...args)
}

/**
 * 获取同步进度
 */
export async function getSyncProgress(): Promise<any> {
  return callApi(BacktestService.GetSyncProgress)
}

/**
 * 获取种子导入状态
 */
export async function getSeedImportStatus(): Promise<any> {
  return callApi(BacktestService.GetSeedImportStatus)
}

/**
 * 运行种子导入
 */
export async function runSeedImport(...args: any[]): Promise<any> {
  return callApi(BacktestService.RunSeedImport, ...args)
}

/**
 * 获取最近一次种子导入输出
 */
export async function getLastSeedImportOutput(): Promise<any> {
  return callApi(BacktestService.GetLastSeedImportOutput)
}

export default {
  runSingleBacktest,
  runBatchBacktest,
  getBacktestResults,
  runOptimization,
  getOptimizationPresets,
  runBacktestForDailyPicks,
  getKLineCacheStats,
  startHistoricalSync,
  getSyncProgress,
  getSeedImportStatus,
  runSeedImport,
  getLastSeedImportOutput,
}
