<script setup>
import { NButton, NFlex, NText, NTooltip } from 'naive-ui'
import { indicatorTips } from './indicators/tips'
import { SHOW_CHIP_TOOLBAR_BUTTON } from './constants'

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  indicators: { type: Object, required: true },
})

const emit = defineEmits(['toggle'])

const categories = [
  {
    name: '📈趋势',
    color: '#ef4444',
    bg: 'rgba(239,68,68,0.08)',
    items: [
      { key: 'ma', label: 'MA' },
      { key: 'ema', label: 'EMA' },
      { key: 'kama', label: 'KAMA' },
      { key: 'supertrend', label: 'STrend' },
      { key: 'sar', label: 'SAR' },
      { key: 'ichimoku', label: 'Ichi' },
      { key: 'aroon', label: 'Aroon' },
      { key: 'dema', label: 'DEMA' },
      { key: 'sats', label: 'SATS' },
      { key: 'alligator', label: 'Gator' },
      { key: 'hullMa', label: 'Hull' },
      { key: 'tema', label: 'TEMA' },
    ]
  },
  {
    name: '🎢波动',
    color: '#f59e0b',
    bg: 'rgba(245,158,11,0.08)',
    items: [
      { key: 'boll', label: 'BOLL' },
      { key: 'keltner', label: 'Kelt' },
      { key: 'donchian', label: 'Donch' },
      { key: 'atr', label: 'ATR' },
      { key: 'avgAmp', label: '均幅' },
      { key: 'ttmSqueeze', label: 'TTM' },
      { key: 'zigzag', label: 'ZigZag' },
      { key: 'fractal', label: 'Fractal' },
      { key: 'massIndex', label: 'Mass' },
      { key: 'smc', label: 'SMC' },
    ]
  },
  {
    name: '💫动量',
    color: '#3b82f6',
    bg: 'rgba(59,130,246,0.08)',
    items: [
      { key: 'macd', label: 'MACD' },
      { key: 'kdj', label: 'KDJ' },
      { key: 'rsi', label: 'RSI' },
      { key: 'cci', label: 'CCI' },
      { key: 'williamsR', label: 'W%R' },
      { key: 'stochRsi', label: 'SRSI' },
      { key: 'cmo', label: 'CMO' },
      { key: 'ao', label: 'AO' },
      { key: 'trix', label: 'TRIX' },
      { key: 'roc', label: 'ROC' },
      { key: 'smi', label: 'SMI' },
      { key: 'coppock', label: 'Coppck' },
    ]
  },
  {
    name: '📊量价',
    color: '#10b981',
    bg: 'rgba(16,185,129,0.08)',
    items: [
      { key: 'obv', label: 'OBV' },
      { key: 'vwap', label: 'VWAP' },
      { key: 'mfi', label: 'MFI' },
      { key: 'cmf', label: 'CMF' },
      { key: 'forceIndex', label: 'FI' },
      { key: 'ad', label: 'A/D' },
      { key: 'chaikinOsc', label: 'ChkOsc' },
      { key: 'vwapBands', label: 'VWBnd' },
    ]
  },
  {
    name: '📏强度',
    color: '#8b5cf6',
    bg: 'rgba(139,92,246,0.08)',
    items: [
      { key: 'adx', label: 'ADX' },
      { key: 'pivot', label: 'Pivot' },
      { key: 'chop', label: 'CHOP' },
      { key: 'elderRay', label: 'Elder' },
      { key: 'ulcerIndex', label: 'Ulcer' },
      { key: 'signalRatio', label: '信号比' },
    ]
  },
]

function onToggle(key) {
  emit('toggle', key)
}
</script>

<template>
  <div class="lw-kline-sidebar">
    <div class="lw-kline-sidebar__inner">
      <NFlex vertical :size="6">
        <div
          v-for="cat in categories"
          :key="cat.name"
          class="lw-kline-sidebar__section"
        >
          <NText
            depth="3"
            :style="{
              fontSize: '13px',
              fontWeight: 700,
              display: 'block',
              marginBottom: '4px',
              padding: '2px 6px',
              background: cat.bg,
              borderRadius: '4px',
              borderLeft: `3px solid ${cat.color}`,
              color: cat.color
            }"
          >
            {{ cat.name }}
          </NText>
          <NFlex :size="4" wrap style="row-gap: 4px">
            <NTooltip
              v-for="item in cat.items"
              :key="item.key"
              :delay="500"
              placement="right-start"
            >
              <template #trigger>
                <NButton
                  size="tiny"
                  :type="indicators[item.key] ? 'primary' : 'default'"
                  :secondary="!indicators[item.key]"
                  @click="onToggle(item.key)"
                >
                  {{ item.label }}
                </NButton>
              </template>
              <span style="white-space: pre-line; text-align: left">{{ indicatorTips[item.key] }}</span>
            </NTooltip>
            <NButton
              v-if="cat.name === '📏强度' && SHOW_CHIP_TOOLBAR_BUTTON"
              size="tiny"
              :type="indicators.chip ? 'primary' : 'default'"
              :secondary="!indicators.chip"
              @click="onToggle('chip')"
            >
              筹码
            </NButton>
          </NFlex>
        </div>
      </NFlex>
    </div>
  </div>
</template>
