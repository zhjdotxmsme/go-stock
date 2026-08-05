/**
 * 筹码分布（获利盘/成本分布直方图）面板的数据计算与 canvas 绘制。
 * 自 StockLightweightKlineChart.vue 原样搬迁；mergedRawRows 原为组件模块级 let，
 * 现经 ctx.getMergedRawRows() 调用时取值，语义一致。
 */
import { nextTick, ref } from 'vue'
import { sortKey, extractYmdDatePart } from '../time'
import { chipBarCostCenter, addChipVolumeKernel, calcChipDistribution } from '../chip'

export function useChipOverlay(ctx) {
  const { props, getMergedRawRows, hoverRawRow, defaultLatestRawRow } = ctx

  const showChip = ref(false)
  const chipBins = ref(80)
  const chipCanvasRef = ref(null)
  const chipItems = ref([])
  const chipMeta = ref({ avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 })

  let chipUpdateTimer = null

  function drawChipCanvas() {
    const canvas = chipCanvasRef.value
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
    const isDark = props.darkTheme
    ctx.fillStyle = isDark ? '#141414' : '#ffffff'
    ctx.fillRect(0, 0, w, h)
    const items = chipItems.value
    if (!items.length) return
    const maxRatio = Math.max(...items.map((it) => it.ratio || 0), 1e-9)
    const barMaxW = w - 4
    const barH = Math.max(1, h / items.length)
    const cur = chipMeta.value.current || 0
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
      const minP = chipMeta.value.minPrice || 0
      const maxP = chipMeta.value.maxPrice || 0
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

  function toggleChip() {
    const mergedRawRows = getMergedRawRows()
    showChip.value = !showChip.value
    if (showChip.value) {
      updateChipFromHover()
    }
  }
  
  function updateChipFromHover() {
    if (!showChip.value || !mergedRawRows.length) {
      chipItems.value = []
      return
    }
    if (chipUpdateTimer) return
    chipUpdateTimer = setTimeout(() => {
      chipUpdateTimer = null
      doUpdateChip()
    }, 30)
  }
  
  function doUpdateChip() {
    if (!showChip.value || !mergedRawRows.length) {
      chipItems.value = []
      return
    }
    const r = hoverRawRow.value ?? defaultLatestRawRow.value
    let rows = mergedRawRows
    if (r) {
      const sk = sortKey(r.day)
      let hi = mergedRawRows.length
      for (let i = 0; i < mergedRawRows.length; i++) {
        if (sortKey(mergedRawRows[i].day) > sk) { hi = i; break }
      }
      rows = hi > 0 ? mergedRawRows.slice(0, hi) : mergedRawRows
    }
    if (!rows.length) {
      chipItems.value = []
      return
    }
    const result = calcChipDistribution(rows, chipBins.value)
    chipItems.value = result.items
    chipMeta.value = {
      avgCost: result.avgCost,
      profitRatio: result.profitRatio,
      current: result.current,
      hoverDate: r ? extractYmdDatePart(String(r.day || '').replace(/\//g, '-')) : '',
      minPrice: result.minPrice || 0,
      maxPrice: result.maxPrice || 0,
    }
    nextTick(() => drawChipCanvas())
  }

  return {
    showChip, chipBins, chipCanvasRef, chipItems, chipMeta,
    toggleChip, updateChipFromHover, drawChipCanvas,
  }
}
