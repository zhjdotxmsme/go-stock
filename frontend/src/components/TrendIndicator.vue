<template>
  <div class="trend-indicator" :class="trend">
    <n-icon size="28" :color="iconColor">
      <ArrowUp v-if="trend === 'up'" />
      <ArrowDown v-else-if="trend === 'down'" />
      <Minus v-else />
    </n-icon>
    <span class="trend-label" :style="{ color: iconColor }">{{ label }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ArrowUp, ArrowDown, Minus } from '@vicons/fa'

const props = defineProps({
  trend: { type: String, default: 'sideways' }, // up / down / sideways
})

const iconColor = computed(() => {
  if (props.trend === 'up') return '#18a058'
  if (props.trend === 'down') return '#d03050'
  return '#999'
})

const label = computed(() => {
  if (props.trend === 'up') return '上涨趋势'
  if (props.trend === 'down') return '下跌趋势'
  return '横盘震荡'
})
</script>

<style scoped>
.trend-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  background: var(--n-color);
}
.trend-indicator.up { border-left: 3px solid #18a058; }
.trend-indicator.down { border-left: 3px solid #d03050; }
.trend-indicator.sideways { border-left: 3px solid #999; }
.trend-label {
  font-size: 14px;
  font-weight: 500;
}
</style>
