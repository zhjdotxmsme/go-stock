/**
 * 股票代码 → 简称 内存注册表（启动后拉一次 GetStockList，与后端 ResolveStockNames 口径对齐）。
 */
import { shallowRef } from 'vue'
import * as stockApi from '../api/stock'
import { normalizeStockCode, toTushareCode } from '../utils/stockCode'

const loaded = shallowRef(false)
const byNorm = shallowRef(new Map())
let loadingPromise = null

function register(map, code, name) {
  const n = (name || '').trim()
  if (!n || !code) return
  const norm = normalizeStockCode(String(code))
  if (norm) map.set(norm, n)
  const ts = toTushareCode(code)
  if (ts) map.set('ts:' + ts.toUpperCase(), n)
}

export async function ensureStockNameRegistry() {
  if (loaded.value) return byNorm.value
  if (loadingPromise) return loadingPromise
  loadingPromise = (async () => {
    try {
      const { data } = await stockApi.getStockList('')
      const map = new Map()
      for (const item of data || []) {
        register(map, item.ts_code, item.name)
      }
      byNorm.value = map
      loaded.value = true
      return map
    } finally {
      loadingPromise = null
    }
  })()
  return loadingPromise
}

export function lookupStockName(code) {
  if (!code || !loaded.value) return ''
  const norm = normalizeStockCode(String(code))
  if (norm && byNorm.value.has(norm)) return byNorm.value.get(norm)
  const ts = toTushareCode(code)
  if (ts && byNorm.value.has('ts:' + ts.toUpperCase())) {
    return byNorm.value.get('ts:' + ts.toUpperCase())
  }
  return ''
}

/** 批量回填缺失名称（mutate rows） */
export function fillRowNames(rows, codeKey = 'code', nameKey = 'name') {
  if (!rows?.length) return rows
  for (const row of rows) {
    const ck = row[codeKey] ?? row.stockCode ?? row.StockCode
    if (!ck) continue
    if (!(row[nameKey] || row.stockName || row.StockName || '').trim()) {
      const n = lookupStockName(ck)
      if (n) row[nameKey] = n
    }
  }
  return rows
}
