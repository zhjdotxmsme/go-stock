/**
 * 全站统一的代码展示与「名称 + 代码」文案（查询/表格/选择器共用）。
 */
import { normalizeStockCode, toTushareCode } from './stockCode'
import { lookupStockName, ensureStockNameRegistry } from '../composables/useStockNameRegistry'

/** 界面展示用代码：A 股等为 600519.SH，内部仍用 sh600519 */
export function displayStockCode(code) {
  if (!code) return ''
  const norm = normalizeStockCode(String(code))
  if (norm) {
    const ts = toTushareCode(norm)
    if (ts) return ts
  }
  return String(code).trim()
}

/** 名称 (展示码) — 缺名称时尝试注册表回填 */
export function formatStockLabel(code, name) {
  let n = (name || '').trim()
  if (!n && code) {
    n = lookupStockName(code)
  }
  const dc = displayStockCode(code)
  if (n && dc) return `${n} (${dc})`
  if (n) return n
  return dc || '-'
}

export function resolveDisplayName(code, name) {
  const n = (name || '').trim()
  if (n) return n
  return lookupStockName(code) || ''
}

export async function prepareStockDisplay() {
  await ensureStockNameRegistry()
}

export { ensureStockNameRegistry, lookupStockName, fillRowNames } from '../composables/useStockNameRegistry'
