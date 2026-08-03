/**
 * 筹码分布计算与绘制 Composable
 */

import { ref, computed, watch, nextTick } from 'vue'
import { chipBarCostCenter, addChipVolumeKernel, calcChipDistribution } from '../chip'

/**
 * 绘制筹码 Canvas
 * @param {HTMLCanvasElement} canvas - Canvas 元素
 * @param {Array} chipItems - 筹码数据项
 * @param {Object} chipMeta - 筹码元数据
 * @param {boolean} darkTheme - 是否深色主题
 */
export function drawChipCanvas(canvas, chipItems, chipMeta, darkTheme) {
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const dpr = window.devicePixelRatio || 1
  const rect = canvas.getBoundingClientRect()
  const w = rect.width
  const h = rect.height
  canvas.width = w * dpr
  canvas.height = h * dpr
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, w, h)

  const isDark = darkTheme
  ctx.fillStyle = isDark ? '#141414' : '#ffffff'
  ctx.fillRect(0, 0, w, h)

  const items = chipItems
  if (!items.length) return
  const maxRatio = Math.max(...items.map((it) => it.ratio || 0), 1e-9)
  const barMaxW = w - 4
  const barH = Math.max(1, h / items.length)
  const cur = chipMeta.current || 0

  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    const y = i * barH
    const bw = Math.max(0, (it.ratio / maxRatio) * barMaxW)
    const isProfit = it.price <= cur
    if (isProfit) {
      ctx.fillStyle = isDark ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.6)'
    } else {
      ctx.fillStyle = isDark ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.6)'
    }
    ctx.fillRect(w - bw, y, bw, barH - 0.5)
  }

  if (cur > 0) {
    const minP = chipMeta.minPrice || 0
    const maxP = chipMeta.maxPrice || 0
    if (maxP > minP && cur >= minP && cur <= maxP) {
      const curY = ((cur - minP) / (maxP - minP)) * h
      ctx.strokeStyle = isDark ? '#fbbf24' : '#d97706'
      ctx.lineWidth = 1
      ctx.setLineDash([4, 3])
      ctx.beginPath()
      ctx.moveTo(0, curY)
      ctx.lineTo(w, curY)
      ctx.stroke()
      ctx.setLineDash([])
    }
  }
}

/**
 * 筹码分布管理
 * @param {Object} options
 * @param {Ref<Array>} options.mergedRawRows - K线数据
 * @param {Ref<boolean>} options.darkTheme - 深色主题
 * @returns {Object}
 */
export function useChipDistribution({ mergedRawRows, darkTheme }) {
  // 显示筹码分布开关
  const showChip = ref(false)
  // 分箱数
  const chipBins = ref(80)
  // Canvas 引用
  const chipCanvasRef = ref(null)
  // 筹码项数据
  const chipItems = ref([])
  // 筹码元数据
  const chipMeta = ref({ avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 })

  // 获利盘比例
  const profitRatio = computed(() => chipMeta.value.profitRatio ?? 0)

  // 平均成本
  const avgCost = computed(() => chipMeta.value.avgCost ?? 0)

  /**
   * 重新计算筹码分布
   */
  function recalculateChipDistribution() {
    const rows = mergedRawRows.value
    if (!rows || rows.length === 0) {
      chipItems.value = []
      chipMeta.value = { avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 }
      return
    }

    const N = rows.length
    // 成交量 Kernel：越新权重越高，按 vixFixedMethod/forwardWeighting 模拟衰减
    const volKernel = []
    for (let i = 0; i < N; i++) {
      volKernel.push(Math.exp(2 * ((i + 1) / N) - 2))
    }

    // 加权成交量汇总
    let totalWeightedVol = 0
    const weightedRows = rows.map((r, i) => {
      const v = Number(r.volume) || 0
      const w = volKernel[i]
      const wv = v * w
      totalWeightedVol += wv
      return { ...r, weightedVol: wv }
    })

    // 价格范围
    const allLows = rows.map(r => Number(r.low)).filter(Number.isFinite)
    const allHighs = rows.map(r => Number(r.high)).filter(Number.isFinite)
    const minP = Math.min(...allLows) * 0.98
    const maxP = Math.max(...allHighs) * 1.02

    // 建立分箱分布
    const bins = chipBins.value
    const binW = (maxP - minP) / bins || 1e-9
    const chipHistogram = new Array(bins).fill(0)

    for (let i = 0; i < weightedRows.length; i++) {
      const r = weightedRows[i]
      const o = Number(r.open)
      const h = Number(r.high)
      const l = Number(r.low)
      const c = Number(r.close)
      const wv = r.weightedVol
      if (![o, h, l, c].every(Number.isFinite) || wv <= 0) continue

      const costCenter = chipBarCostCenter(o, h, l, c, i)
      const distBars = 3 // 单根K 扩散到前后几个价格格
      addChipVolumeKernel(chipHistogram, costCenter, minP, binW, wv, distBars)
    }

    const dist = calcChipDistribution(chipHistogram, minP, binW, bins)
    chipItems.value = dist.items

    // 当前价 = 最后一根收盘价
    const cur = Number(rows[rows.length - 1]?.close) || 0
    chipMeta.value = {
      avgCost: dist.avgCost,
      profitRatio: dist.profitRatio(cur),
      current: cur,
      hoverDate: '',
      minPrice: minP,
      maxPrice: maxP,
    }

    nextTick(() => {
      drawChipCanvas(chipCanvasRef.value, chipItems.value, chipMeta.value, darkTheme.value)
    })
  }

  // 自动重计算
  watch(() => [mergedRawRows.value.length, showChip.value, darkTheme.value], () => {
    if (showChip.value) {
      recalculateChipDistribution()
    }
  }, { immediate: true, deep: true })

  return {
    // 状态
    showChip,
    chipBins,
    chipCanvasRef,
    chipItems,
    chipMeta,

    // 计算属性
    profitRatio,
    avgCost,

    // 方法
    recalculateChipDistribution,
    drawChipCanvas,
  }
}

export default useChipDistribution
