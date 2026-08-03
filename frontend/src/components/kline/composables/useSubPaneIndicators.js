/**
 * 子窗格指标渲染 Composable
 * 处理 25+ 技术指标的计算和图表渲染
 * 重构自 862 行的 syncSubPaneIndicators 巨型函数
 */

import { ref } from 'vue'
import { LineSeries, HistogramSeries } from 'lightweight-charts'
import {
  obvValues,
  macdBundle,
  kdjBundle,
  rsiValues,
  atrValues,
  mfiValues,
  cciValues,
  ttmSqueezeValues,
  adxValues,
  williamsRValues,
  stochRsiValues,
  cmfValues,
  aroonValues,
  cmoValues,
  forceIndexValues,
  smaValues,
  aoValues,
  adValues,
  trixValues,
  emaLeadingNull,
  rocValues,
  chopValues,
  elderRayValues,
  chaikinOscValues,
  massIndexValues,
  ulcerIndexValues,
  coppockValues,
  smiValues,
  emaFinite,
} from '../calc'
import { toLineData } from './useChartLifecycle'

// 子线图通用配置
export const subLineOpts = {
  lineWidth: 1,
  lastValueVisible: false,
  priceLineVisible: false,
}

/**
 * 指标注册器
 * 每个指标包含：名称、计算函数、渲染函数
 */
export function createIndicatorRegistry() {
  return {
    obv: {
      name: 'OBV',
      calc: (closes, highs, lows, vols) => ({ obv: obvValues(closes, vols) }),
      render: (chart, paneIdx, data, ind) => {
        ind.obv = chart.addSeries(
          LineSeries,
          {
            color: '#22c55e',
            lineWidth: 1,
            title: 'OBV',
            lastValueVisible: true,
            priceLineVisible: false,
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        ind.obv.setData(toLineData(data.times, data.obv))
      },
    },

    macd: {
      name: 'MACD',
      calc: (closes, highs, lows, vols) => macdBundle(closes),
      render: (chart, paneIdx, data, ind) => {
        const { dif, dea, hist } = data
        ind.macdHist = chart.addSeries(
          HistogramSeries,
          {
            priceFormat: { type: 'price', precision: 4, minMove: 0.0001 },
            priceScaleId: 'macd',
          },
          paneIdx,
        )
        ind.macdDif = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f59e0b',
            title: 'DIF',
            priceScaleId: 'macd',
          },
          paneIdx,
        )
        ind.macdDea = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#6366f1',
            title: 'DEA',
            priceScaleId: 'macd',
          },
          paneIdx,
        )
        const histData = []
        for (let i = 0; i < data.times.length; i++) {
          const hv = hist[i]
          if (hv == null || !Number.isFinite(hv)) continue
          histData.push({
            time: data.times[i],
            value: hv,
            color: hv >= 0
              ? 'rgba(239, 83, 80, 0.55)'
              : 'rgba(38, 166, 154, 0.55)',
          })
        }
        ind.macdHist.setData(histData)
        ind.macdDif.setData(toLineData(data.times, dif))
        ind.macdDea.setData(toLineData(data.times, dea))
      },
    },

    kdj: {
      name: 'KDJ',
      calc: (closes, highs, lows, vols) => kdjBundle(highs, lows, closes, 9),
      render: (chart, paneIdx, data, ind) => {
        const { K, D, J } = data
        ind.kdjK = chart.addSeries(
          LineSeries,
          { ...subLineOpts, color: '#f59e0b', title: 'K' },
          paneIdx,
        )
        ind.kdjD = chart.addSeries(
          LineSeries,
          { ...subLineOpts, color: '#3b82f6', title: 'D' },
          paneIdx,
        )
        ind.kdjJ = chart.addSeries(
          LineSeries,
          { ...subLineOpts, color: '#a855f7', title: 'J' },
          paneIdx,
        )
        ind.kdjK.setData(toLineData(data.times, K))
        ind.kdjD.setData(toLineData(data.times, D))
        ind.kdjJ.setData(toLineData(data.times, J))
      },
    },

    rsi: {
      name: 'RSI',
      calc: (closes, highs, lows, vols) => ({ rsi: rsiValues(closes, 14) }),
      render: (chart, paneIdx, data, ind) => {
        ind.rsi = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#d946ef',
            title: 'RSI14',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.rsi.setData(toLineData(data.times, data.rsi))
      },
    },

    atr: {
      name: 'ATR',
      calc: (closes, highs, lows, vols) => ({ atr: atrValues(highs, lows, closes, 14) }),
      render: (chart, paneIdx, data, ind) => {
        ind.atr = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#06b6d4',
            title: 'ATR14',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.atr.setData(toLineData(data.times, data.atr))
      },
    },

    mfi: {
      name: 'MFI',
      calc: (closes, highs, lows, vols) => ({ mfi: mfiValues(highs, lows, closes, vols, 14) }),
      render: (chart, paneIdx, data, ind) => {
        ind.mfi = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f97316',
            title: 'MFI14',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.mfi.setData(toLineData(data.times, data.mfi))
      },
    },

    cci: {
      name: 'CCI',
      calc: (closes, highs, lows, vols) => ({ cci: cciValues(highs, lows, closes, 20) }),
      render: (chart, paneIdx, data, ind) => {
        ind.cci = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#eab308',
            title: 'CCI20',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.cci.setData(toLineData(data.times, data.cci))
      },
    },

    ttmSqueeze: {
      name: 'TTM Squeeze',
      calc: (closes, highs, lows, vols) => ttmSqueezeValues(highs, lows, closes),
      render: (chart, paneIdx, data, ind) => {
        const { squeeze, momentum } = data
        ind.ttmHist = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.ttmDots = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#6366f1',
            lineWidth: 0,
            pointMarkersVisible: true,
            pointMarkersRadius: 2,
            title: 'SQZ',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        const histData = []
        const dotData = []
        for (let i = 0; i < data.times.length; i++) {
          const mv = momentum[i]
          if (mv != null && Number.isFinite(mv)) {
            histData.push({
              time: data.times[i],
              value: mv,
              color: mv >= 0
                ? (squeeze[i] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.4)')
                : (squeeze[i] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.4)'),
            })
            dotData.push({
              time: data.times[i],
              value: 0,
              color: squeeze[i] ? '#eab308' : '#22c55e',
            })
          }
        }
        ind.ttmHist.setData(histData)
        ind.ttmDots.setData(dotData)
      },
    },

    adx: {
      name: 'ADX',
      calc: (closes, highs, lows, vols) => adxValues(highs, lows, closes, 14),
      render: (chart, paneIdx, data, ind) => {
        const { adx, diP, diM } = data
        ind.adxDiP = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#22c55e',
            lineWidth: 1,
            title: '+DI',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.adxDiM = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#ef4444',
            lineWidth: 1,
            title: '-DI',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.adx = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#3b82f6',
            lineWidth: 2,
            title: 'ADX',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.adxDiP.setData(toLineData(data.times, diP))
        ind.adxDiM.setData(toLineData(data.times, diM))
        ind.adx.setData(toLineData(data.times, adx))
      },
    },

    williamsR: {
      name: 'Williams %R',
      calc: (closes, highs, lows, vols) => ({ wr: williamsRValues(highs, lows, closes, 14) }),
      render: (chart, paneIdx, data, ind) => {
        ind.williamsR = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#8b5cf6',
            title: 'W%R14',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.williamsR.setData(toLineData(data.times, data.wr))
      },
    },

    stochRsi: {
      name: 'StochRSI',
      calc: (closes, highs, lows, vols) => stochRsiValues(closes, 14, 14, 3, 3),
      render: (chart, paneIdx, data, ind) => {
        const { k, d } = data
        ind.stochRsi = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#06b6d4',
            title: 'StochRSI K',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.stochRsiD = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f59e0b',
            title: 'StochRSI D',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.stochRsi.setData(toLineData(data.times, k))
        ind.stochRsiD.setData(toLineData(data.times, d))
      },
    },

    cmf: {
      name: 'CMF',
      calc: (closes, highs, lows, vols) => ({ cmf: cmfValues(highs, lows, closes, vols, 20) }),
      render: (chart, paneIdx, data, ind) => {
        ind.cmf = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#14b8a6',
            title: 'CMF20',
            priceFormat: { type: 'price', precision: 3, minMove: 0.001 },
          },
          paneIdx,
        )
        ind.cmf.setData(toLineData(data.times, data.cmf))
      },
    },

    aroon: {
      name: 'Aroon',
      calc: (closes, highs, lows, vols) => aroonValues(highs, lows, 25),
      render: (chart, paneIdx, data, ind) => {
        const { up, down } = data
        ind.aroonUp = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#22c55e',
            title: 'Aroon Up',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.aroonDown = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#ef4444',
            title: 'Aroon Down',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.aroonUp.setData(toLineData(data.times, up))
        ind.aroonDown.setData(toLineData(data.times, down))
      },
    },

    cmo: {
      name: 'CMO',
      calc: (closes, highs, lows, vols) => ({ cmo: cmoValues(closes, 14) }),
      render: (chart, paneIdx, data, ind) => {
        ind.cmo = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#8b5cf6',
            title: 'CMO14',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.cmo.setData(toLineData(data.times, data.cmo))
      },
    },

    forceIndex: {
      name: 'Force Index',
      calc: (closes, highs, lows, vols) => ({ fi: forceIndexValues(closes, vols, 13) }),
      render: (chart, paneIdx, data, ind) => {
        ind.forceIndex = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f97316',
            title: 'FI13',
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        ind.forceIndex.setData(toLineData(data.times, data.fi))
      },
    },

    avgAmp: {
      name: 'Average Amplitude',
      calc: (closes, highs, lows, vols, amplitudes) => ({
        aa5: smaValues(amplitudes, 5),
        aa10: smaValues(amplitudes, 10),
        aa20: smaValues(amplitudes, 20),
      }),
      render: (chart, paneIdx, data, ind) => {
        const { aa5, aa10, aa20 } = data
        ind.avgAmp5 = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f59e0b',
            title: '均幅5',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.avgAmp10 = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#3b82f6',
            title: '均幅10',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.avgAmp20 = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#a855f7',
            title: '均幅20',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.avgAmp5.setData(toLineData(data.times, aa5))
        ind.avgAmp10.setData(toLineData(data.times, aa10))
        ind.avgAmp20.setData(toLineData(data.times, aa20))
      },
    },

    ao: {
      name: 'AO',
      calc: (closes, highs, lows, vols) => ({ ao: aoValues(highs, lows) }),
      render: (chart, paneIdx, data, ind) => {
        const { ao } = data
        ind.aoHist = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.aoLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#3b82f6',
            title: 'AO',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        const aoHistData = []
        for (let i = 0; i < data.times.length; i++) {
          const v = ao[i]
          if (v != null && Number.isFinite(v)) {
            aoHistData.push({
              time: data.times[i],
              value: v,
              color: v >= 0
                ? (i > 0 && ao[i - 1] != null && v > ao[i - 1] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.35)')
                : (i > 0 && ao[i - 1] != null && v < ao[i - 1] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.35)'),
            })
          }
        }
        ind.aoHist.setData(aoHistData)
        ind.aoLine.setData(toLineData(data.times, ao))
      },
    },

    ad: {
      name: 'A/D Line',
      calc: (closes, highs, lows, vols) => ({ ad: adValues(highs, lows, closes, vols) }),
      render: (chart, paneIdx, data, ind) => {
        ind.adLine = chart.addSeries(
          LineSeries,
          {
            color: '#22c55e',
            lineWidth: 1,
            title: 'A/D',
            lastValueVisible: true,
            priceLineVisible: false,
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        ind.adLine.setData(toLineData(data.times, data.ad))
      },
    },

    trix: {
      name: 'TRIX',
      calc: (closes, highs, lows, vols) => {
        const trix = trixValues(closes, 15)
        const signal = emaLeadingNull(trix, 9)
        return { trix, signal }
      },
      render: (chart, paneIdx, data, ind) => {
        const { trix, signal } = data
        ind.trixLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#3b82f6',
            lineWidth: 2,
            title: 'TRIX',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.trixSignal = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#ef4444',
            title: 'Signal',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.trixLine.setData(toLineData(data.times, trix))
        ind.trixSignal.setData(toLineData(data.times, signal))
      },
    },

    roc: {
      name: 'ROC',
      calc: (closes, highs, lows, vols) => ({ roc: rocValues(closes, 12) }),
      render: (chart, paneIdx, data, ind) => {
        ind.rocLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#d946ef',
            title: 'ROC12',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.rocLine.setData(toLineData(data.times, data.roc))
      },
    },

    chop: {
      name: 'CHOP',
      calc: (closes, highs, lows, vols) => ({ chop: chopValues(highs, lows, closes, 14) }),
      render: (chart, paneIdx, data, ind) => {
        ind.chopLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f97316',
            title: 'CHOP',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.chopLine.setData(toLineData(data.times, data.chop))
      },
    },

    elderRay: {
      name: 'Elder Ray',
      calc: (closes, highs, lows, vols) => elderRayValues(highs, lows, closes, 13),
      render: (chart, paneIdx, data, ind) => {
        const { bullPower, bearPower } = data
        ind.elderBull = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.elderBear = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        const bullData = []
        const bearData = []
        for (let i = 0; i < data.times.length; i++) {
          const bv = bullPower[i]
          const brv = bearPower[i]
          if (bv != null && Number.isFinite(bv)) {
            bullData.push({
              time: data.times[i],
              value: bv,
              color: bv >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(239, 68, 68, 0.35)',
            })
          }
          if (brv != null && Number.isFinite(brv)) {
            bearData.push({
              time: data.times[i],
              value: brv,
              color: brv >= 0 ? 'rgba(34, 197, 94, 0.7)' : 'rgba(34, 197, 94, 0.35)',
            })
          }
        }
        ind.elderBull.setData(bullData)
        ind.elderBear.setData(bearData)
      },
    },

    chaikinOsc: {
      name: "Chaikin's Oscillator",
      calc: (closes, highs, lows, vols) => ({ co: chaikinOscValues(highs, lows, closes, vols, 3, 10) }),
      render: (chart, paneIdx, data, ind) => {
        ind.chaikinOscLine = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        const coData = []
        for (let i = 0; i < data.times.length; i++) {
          const v = data.co[i]
          if (v != null && Number.isFinite(v)) {
            coData.push({
              time: data.times[i],
              value: v,
              color: v >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(34, 197, 94, 0.7)',
            })
          }
        }
        ind.chaikinOscLine.setData(coData)
      },
    },

    massIndex: {
      name: 'Mass Index',
      calc: (closes, highs, lows, vols) => ({ mi: massIndexValues(highs, lows) }),
      render: (chart, paneIdx, data, ind) => {
        ind.massIndexLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#f59e0b',
            title: 'Mass',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.massIndexLine.setData(toLineData(data.times, data.mi))
      },
    },

    ulcerIndex: {
      name: 'Ulcer Index',
      calc: (closes, highs, lows, vols) => ({ ui: ulcerIndexValues(closes) }),
      render: (chart, paneIdx, data, ind) => {
        ind.ulcerLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#ef4444',
            title: 'Ulcer',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.ulcerLine.setData(toLineData(data.times, data.ui))
      },
    },

    coppock: {
      name: 'Coppock Curve',
      calc: (closes, highs, lows, vols) => ({ cp: coppockValues(closes) }),
      render: (chart, paneIdx, data, ind) => {
        ind.coppockLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#8b5cf6',
            title: 'Coppock',
            priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
          },
          paneIdx,
        )
        ind.coppockLine.setData(toLineData(data.times, data.cp))
      },
    },

    smi: {
      name: 'SMI',
      calc: (closes, highs, lows, vols) => smiValues(highs, lows, closes),
      render: (chart, paneIdx, data, ind) => {
        const { smi: smiData, signal: smiSig } = data
        ind.smiLine = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#3b82f6',
            lineWidth: 2,
            title: 'SMI',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.smiSignal = chart.addSeries(
          LineSeries,
          {
            ...subLineOpts,
            color: '#ef4444',
            title: 'Signal',
            priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
          },
          paneIdx,
        )
        ind.smiLine.setData(toLineData(data.times, smiData))
        ind.smiSignal.setData(toLineData(data.times, smiSig))
      },
    },

    signalRatio: {
      name: 'Signal Ratio',
      calc: (closes, highs, lows, vols, amplitudes) => {
        const times = Array.from({ length: closes.length }, (_, i) => i)
        const bullishArr = new Array(times.length).fill(null)
        const bearishArr = new Array(times.length).fill(null)
        const netArr = new Array(times.length).fill(null)

        const ma5 = smaValues(closes, 5)
        const ma10 = smaValues(closes, 10)
        const ma20 = smaValues(closes, 20)
        const ma60 = smaValues(closes, 60)
        const ema12 = emaFinite(closes, 12)

        for (let i = 0; i < times.length; i++) {
          const m5 = ma5[i], m10 = ma10[i], m20 = ma20[i], m60 = ma60[i], e12 = ema12[i]
          const c = closes[i]

          if ([m5, m10, m20, m60, e12, c].every(x => x != null && Number.isFinite(x))) {
            let bullish = 0, bearish = 0
            if (m5 > m10 && m10 > m20 && m20 > m60) bullish++
            else if (m5 < m10 && m10 < m20 && m20 < m60) bearish++
            if (c > m20) bullish++
            else bearish++

            bullishArr[i] = bullish
            bearishArr[i] = bearish
            netArr[i] = bullish - bearish
          }
        }
        return { bullishArr, bearishArr, netArr }
      },
      render: (chart, paneIdx, data, ind) => {
        const { bullishArr, bearishArr, netArr } = data
        ind.signalRatioBullish = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        ind.signalRatioBearish = chart.addSeries(
          HistogramSeries,
          {
            priceLineVisible: false,
            lastValueVisible: false,
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        ind.signalRatioNet = chart.addSeries(
          LineSeries,
          {
            color: '#3b82f6',
            lineWidth: 2,
            title: 'Net',
            priceLineVisible: false,
            priceFormat: { type: 'price', precision: 0, minMove: 1 },
          },
          paneIdx,
        )
        const bullData = []
        const bearData = []
        const netData = []
        for (let i = 0; i < data.times.length; i++) {
          const b = bullishArr[i], be = bearishArr[i], n = netArr[i]
          if (b != null && Number.isFinite(b)) {
            bullData.push({ time: data.times[i], value: b, color: 'rgba(34, 197, 94, 0.7)' })
          }
          if (be != null && Number.isFinite(be)) {
            bearData.push({ time: data.times[i], value: be, color: 'rgba(239, 68, 68, 0.7)' })
          }
          if (n != null && Number.isFinite(n)) {
            netData.push({ time: data.times[i], value: n })
          }
        }
        ind.signalRatioBullish.setData(bullData)
        ind.signalRatioBearish.setData(bearData)
        ind.signalRatioNet.setData(netData)
      },
    },
  }
}

/**
 * 子窗格指标管理
 */
export function useSubPaneIndicators() {
  const registry = createIndicatorRegistry()

  // 指标显示开关
  const showOBV = ref(false)
  const showMACD = ref(false)
  const showKDJ = ref(false)
  const showRSI = ref(false)
  const showATR = ref(false)
  const showMFI = ref(false)
  const showCCI = ref(false)
  const showTTMSqueeze = ref(false)
  const showADX = ref(false)
  const showWilliamsR = ref(false)
  const showStochRSI = ref(false)
  const showCMF = ref(false)
  const showAroon = ref(false)
  const showCMO = ref(false)
  const showForceIndex = ref(false)
  const showAvgAmp = ref(false)
  const showAO = ref(false)
  const showAD = ref(false)
  const showTRIX = ref(false)
  const showROC = ref(false)
  const showCHOP = ref(false)
  const showElderRay = ref(false)
  const showChaikinOsc = ref(false)
  const showMassIndex = ref(false)
  const showUlcerIndex = ref(false)
  const showCoppock = ref(false)
  const showSMI = ref(false)
  const showSignalRatio = ref(false)

  /**
   * 获取启用的指标列表
   */
  function getEnabledIndicators() {
    const enabled = []
    if (showOBV.value) enabled.push('obv')
    if (showMACD.value) enabled.push('macd')
    if (showKDJ.value) enabled.push('kdj')
    if (showRSI.value) enabled.push('rsi')
    if (showATR.value) enabled.push('atr')
    if (showMFI.value) enabled.push('mfi')
    if (showCCI.value) enabled.push('cci')
    if (showTTMSqueeze.value) enabled.push('ttmSqueeze')
    if (showADX.value) enabled.push('adx')
    if (showWilliamsR.value) enabled.push('williamsR')
    if (showStochRSI.value) enabled.push('stochRsi')
    if (showCMF.value) enabled.push('cmf')
    if (showAroon.value) enabled.push('aroon')
    if (showCMO.value) enabled.push('cmo')
    if (showForceIndex.value) enabled.push('forceIndex')
    if (showAvgAmp.value) enabled.push('avgAmp')
    if (showAO.value) enabled.push('ao')
    if (showAD.value) enabled.push('ad')
    if (showTRIX.value) enabled.push('trix')
    if (showROC.value) enabled.push('roc')
    if (showCHOP.value) enabled.push('chop')
    if (showElderRay.value) enabled.push('elderRay')
    if (showChaikinOsc.value) enabled.push('chaikinOsc')
    if (showMassIndex.value) enabled.push('massIndex')
    if (showUlcerIndex.value) enabled.push('ulcerIndex')
    if (showCoppock.value) enabled.push('coppock')
    if (showSMI.value) enabled.push('smi')
    if (showSignalRatio.value) enabled.push('signalRatio')
    return enabled
  }

  /**
   * 同步子窗格指标
   * @param {Object} chart - 图表实例
   * @param {Array} times - 时间数组
   * @param {Array} closes - 收盘价数组
   * @param {Array} highs - 最高价数组
   * @param {Array} lows - 最低价数组
   * @param {Array} vols - 成交量数组
   * @param {Array} amplitudes - 振幅数组
   * @param {Object} indicatorSeries - 指标系列引用对象
   * @param {Function} tearDownFn - 清理函数
   */
  function syncSubPaneIndicators(
    chart,
    times,
    closes,
    highs,
    lows,
    vols,
    amplitudes,
    indicatorSeries,
    tearDownFn
  ) {
    if (!chart) return

    const enabled = getEnabledIndicators()
    if (enabled.length === 0) return

    // 设置主图拉伸因子
    chart.panes()[0]?.setStretchFactor(3)

    let paneIdx = 1
    for (const key of enabled) {
      const indicator = registry[key]
      if (!indicator) continue

      // 计算指标数据
      const data = indicator.calc(closes, highs, lows, vols, amplitudes)
      // 添加 times 供渲染函数使用
      data.times = times
      // 渲染指标到图表
      indicator.render(chart, paneIdx, data, indicatorSeries)
      paneIdx++
    }
  }

  /**
   * 批量设置指标显示状态
   * @param {Object} states - 状态对象 { obv: true, macd: false, ... }
   */
  function setIndicatorStates(states) {
    Object.entries(states).forEach(([key, value]) => {
      const refKey = `show${key.charAt(0).toUpperCase() + key.slice(1)}`
      if (typeof this[refKey] !== 'undefined') {
        this[refKey].value = value
      }
    })
  }

  return {
    // 指标显示状态
    showOBV,
    showMACD,
    showKDJ,
    showRSI,
    showATR,
    showMFI,
    showCCI,
    showTTMSqueeze,
    showADX,
    showWilliamsR,
    showStochRSI,
    showCMF,
    showAroon,
    showCMO,
    showForceIndex,
    showAvgAmp,
    showAO,
    showAD,
    showTRIX,
    showROC,
    showCHOP,
    showElderRay,
    showChaikinOsc,
    showMassIndex,
    showUlcerIndex,
    showCoppock,
    showSMI,
    showSignalRatio,

    // 方法
    syncSubPaneIndicators,
    getEnabledIndicators,
    setIndicatorStates,
    createIndicatorRegistry,
    subLineOpts,
  }
}

export default useSubPaneIndicators
