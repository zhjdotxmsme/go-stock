<template>
  <div class="price-zone-card" :class="type">
    <div class="zone-header">
      <n-icon size="16" :color="iconColor">
        <ArrowRight v-if="type === 'entry'" />
        <ArrowLeft v-else />
      </n-icon>
      <span class="zone-title">{{ title }}</span>
    </div>
    <div class="zone-prices" v-if="showPrices">
      <span class="price-low">¥{{ formatPrice(low) }}</span>
      <span class="price-sep">—</span>
      <span class="price-high">¥{{ formatPrice(high) }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ArrowRight, ArrowLeft } from '@vicons/fa'

const props = defineProps({
  type: { type: String, default: 'entry' }, // entry / exit
  low: { type: Number, default: 0 },
  high: { type: Number, default: 0 },
})

const title = computed(() => props.type === 'entry' ? '买入区间' : '卖出区间')
const iconColor = computed(() => props.type === 'entry' ? '#18a058' : '#d03050')
const showPrices = computed(() => props.low > 0 && props.high > 0)

function formatPrice(v) {
  if (!v) return '--'
  return v.toFixed(2)
}
</script>

<style scoped>
.price-zone-card {
  display: inline-flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--n-color);
  min-width: 140px;
}
.price-zone-card.entry { border-left: 3px solid #18a058; }
.price-zone-card.exit { border-left: 3px solid #d03050; }
.zone-header {
  display: flex;
  align-items: center;
  gap: 6px;
}
.zone-title {
  font-size: 13px;
  font-weight: 600;
}
.zone-prices {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.price-low { color: #18a058; }
.price-high { color: #d03050; }
.price-sep { color: #999; font-size: 14px; }
</style>
