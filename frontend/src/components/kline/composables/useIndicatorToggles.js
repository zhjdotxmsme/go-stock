/**
 * 指标开关：48 个 show* ref 的 toggle 函数、侧边栏状态映射 indicators、key->toggle 映射。
 * 自 StockLightweightKlineChart.vue 原样搬迁。
 */
import { computed } from 'vue'
import { makeToggle } from '../indicators/toggle'

export function createIndicatorToggles(shows, syncIndicators) {
  const {
    showMA,
    showBOLL,
    showOBV,
    showMACD,
    showKDJ,
    showRSI,
    showATR,
    showVWAP,
    showMFI,
    showKAMA,
    showKeltner,
    showSupertrend,
    showEMA,
    showIchimoku,
    showCCI,
    showTTMSqueeze,
    showSAR,
    showDonchian,
    showADX,
    showWilliamsR,
    showStochRSI,
    showCMF,
    showAroon,
    showCMO,
    showForceIndex,
    showPivot,
    showDEMA,
    showZigZag,
    showSATS,
    showAvgAmp,
    showAlligator,
    showAO,
    showHullMA,
    showAD,
    showTRIX,
    showROC,
    showFractal,
    showCHOP,
    showElderRay,
    showChaikinOsc,
    showVWAPBands,
    showMassIndex,
    showUlcerIndex,
    showCoppock,
    showTEMA,
    showSMI,
    showSignalRatio,
    showSMC,
    showChip,
  } = shows

  const toggleMA = makeToggle(showMA, syncIndicators)
  const toggleBOLL = makeToggle(showBOLL, syncIndicators)
  const toggleOBV = makeToggle(showOBV, syncIndicators)
  const toggleMACD = makeToggle(showMACD, syncIndicators)
  const toggleKDJ = makeToggle(showKDJ, syncIndicators)
  const toggleRSI = makeToggle(showRSI, syncIndicators)
  const toggleATR = makeToggle(showATR, syncIndicators)
  const toggleVWAP = makeToggle(showVWAP, syncIndicators)
  const toggleMFI = makeToggle(showMFI, syncIndicators)
  const toggleKAMA = makeToggle(showKAMA, syncIndicators)
  const toggleKeltner = makeToggle(showKeltner, syncIndicators)
  const toggleSupertrend = makeToggle(showSupertrend, syncIndicators)
  const toggleEMA = makeToggle(showEMA, syncIndicators)
  const toggleIchimoku = makeToggle(showIchimoku, syncIndicators)
  const toggleCCI = makeToggle(showCCI, syncIndicators)
  const toggleTTMSqueeze = makeToggle(showTTMSqueeze, syncIndicators)
  const toggleSAR = makeToggle(showSAR, syncIndicators)
  const toggleDonchian = makeToggle(showDonchian, syncIndicators)
  const toggleADX = makeToggle(showADX, syncIndicators)
  const toggleWilliamsR = makeToggle(showWilliamsR, syncIndicators)
  const toggleStochRSI = makeToggle(showStochRSI, syncIndicators)
  const toggleCMF = makeToggle(showCMF, syncIndicators)
  const toggleAroon = makeToggle(showAroon, syncIndicators)
  const toggleCMO = makeToggle(showCMO, syncIndicators)
  const toggleForceIndex = makeToggle(showForceIndex, syncIndicators)
  const togglePivot = makeToggle(showPivot, syncIndicators)
  const toggleDEMA = makeToggle(showDEMA, syncIndicators)
  const toggleZigZag = makeToggle(showZigZag, syncIndicators)
  const toggleSATS = makeToggle(showSATS, syncIndicators)
  const toggleAvgAmp = makeToggle(showAvgAmp, syncIndicators)
  const toggleAlligator = makeToggle(showAlligator, syncIndicators)
  const toggleAO = makeToggle(showAO, syncIndicators)
  const toggleHullMA = makeToggle(showHullMA, syncIndicators)
  const toggleAD = makeToggle(showAD, syncIndicators)
  const toggleTRIX = makeToggle(showTRIX, syncIndicators)
  const toggleROC = makeToggle(showROC, syncIndicators)
  const toggleFractal = makeToggle(showFractal, syncIndicators)
  const toggleCHOP = makeToggle(showCHOP, syncIndicators)
  const toggleElderRay = makeToggle(showElderRay, syncIndicators)
  const toggleChaikinOsc = makeToggle(showChaikinOsc, syncIndicators)
  const toggleVWAPBands = makeToggle(showVWAPBands, syncIndicators)
  const toggleMassIndex = makeToggle(showMassIndex, syncIndicators)
  const toggleUlcerIndex = makeToggle(showUlcerIndex, syncIndicators)
  const toggleCoppock = makeToggle(showCoppock, syncIndicators)
  const toggleTEMA = makeToggle(showTEMA, syncIndicators)
  const toggleSMI = makeToggle(showSMI, syncIndicators)
  const toggleSignalRatio = makeToggle(showSignalRatio, syncIndicators)
  const toggleSMC = makeToggle(showSMC, syncIndicators)

  const indicators = computed(() => ({
    ma: showMA.value, boll: showBOLL.value, obv: showOBV.value,
    macd: showMACD.value, kdj: showKDJ.value, rsi: showRSI.value,
    atr: showATR.value, vwap: showVWAP.value, mfi: showMFI.value,
    kama: showKAMA.value, keltner: showKeltner.value, supertrend: showSupertrend.value,
    ema: showEMA.value, ichimoku: showIchimoku.value, cci: showCCI.value,
    ttmSqueeze: showTTMSqueeze.value, sar: showSAR.value, donchian: showDonchian.value,
    adx: showADX.value, williamsR: showWilliamsR.value, stochRsi: showStochRSI.value,
    cmf: showCMF.value, aroon: showAroon.value, cmo: showCMO.value,
    forceIndex: showForceIndex.value, pivot: showPivot.value, dema: showDEMA.value,
    zigzag: showZigZag.value, sats: showSATS.value, avgAmp: showAvgAmp.value,
    alligator: showAlligator.value, ao: showAO.value, hullMa: showHullMA.value,
    ad: showAD.value, trix: showTRIX.value, roc: showROC.value,
    fractal: showFractal.value, chop: showCHOP.value, elderRay: showElderRay.value,
    chaikinOsc: showChaikinOsc.value, vwapBands: showVWAPBands.value,
    massIndex: showMassIndex.value, ulcerIndex: showUlcerIndex.value,
    coppock: showCoppock.value, tema: showTEMA.value, smi: showSMI.value,
    signalRatio: showSignalRatio.value, smc: showSMC.value, chip: showChip.value,
  }))

  const indicatorToggleMap = {
    ma: toggleMA, boll: toggleBOLL, obv: toggleOBV, macd: toggleMACD,
    kdj: toggleKDJ, rsi: toggleRSI, atr: toggleATR, vwap: toggleVWAP,
    mfi: toggleMFI, kama: toggleKAMA, keltner: toggleKeltner,
    supertrend: toggleSupertrend, ema: toggleEMA, ichimoku: toggleIchimoku,
    cci: toggleCCI, ttmSqueeze: toggleTTMSqueeze, sar: toggleSAR,
    donchian: toggleDonchian, adx: toggleADX, williamsR: toggleWilliamsR,
    stochRsi: toggleStochRSI, cmf: toggleCMF, aroon: toggleAroon,
    cmo: toggleCMO, forceIndex: toggleForceIndex, pivot: togglePivot,
    dema: toggleDEMA, zigzag: toggleZigZag, sats: toggleSATS,
    avgAmp: toggleAvgAmp, alligator: toggleAlligator, ao: toggleAO,
    hullMa: toggleHullMA, ad: toggleAD, trix: toggleTRIX,
    roc: toggleROC, fractal: toggleFractal, chop: toggleCHOP,
    elderRay: toggleElderRay, chaikinOsc: toggleChaikinOsc,
    vwapBands: toggleVWAPBands, massIndex: toggleMassIndex,
    ulcerIndex: toggleUlcerIndex, coppock: toggleCoppock, tema: toggleTEMA,
    smi: toggleSMI, signalRatio: toggleSignalRatio, smc: toggleSMC,
  }

  return { indicators, indicatorToggleMap }
}
