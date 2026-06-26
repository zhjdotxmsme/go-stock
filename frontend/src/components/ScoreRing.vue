<template>
  <div class="score-ring" :style="{ '--score': score, '--color': ringColor }">
    <svg viewBox="0 0 120 120" class="ring-svg">
      <circle cx="60" cy="60" r="50" fill="none" stroke="#e5e5e5" stroke-width="8" />
      <circle
        cx="60" cy="60" r="50"
        fill="none"
        :stroke="ringColor"
        stroke-width="8"
        stroke-linecap="round"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="dashOffset"
        transform="rotate(-90, 60, 60)"
        class="ring-fill"
      />
    </svg>
    <div class="score-label">
      <span class="score-value">{{ displayScore }}</span>
      <span class="score-unit">/10</span>
    </div>
    <div class="score-text">{{ label }}</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  score: { type: Number, default: 0 },
})

const circumference = 2 * Math.PI * 50

const displayScore = computed(() => {
  if (props.score <= 0) return '--'
  return props.score.toFixed(1)
})

const dashOffset = computed(() => {
  if (props.score <= 0) return circumference
  const pct = Math.min(props.score / 10, 1)
  return circumference * (1 - pct)
})

const ringColor = computed(() => {
  const s = props.score
  if (s <= 0) return '#999'
  if (s >= 8) return '#18a058'
  if (s >= 6) return '#2080f0'
  if (s >= 4) return '#f0a020'
  return '#d03050'
})

const label = computed(() => {
  const s = props.score
  if (s <= 0) return '未评估'
  if (s >= 8) return '强烈推荐'
  if (s >= 6) return '推荐'
  if (s >= 4) return '谨慎'
  return '规避'
})
</script>

<style scoped>
.score-ring {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  position: relative;
  width: 120px;
}
.ring-svg {
  width: 120px;
  height: 120px;
}
.ring-fill {
  transition: stroke-dashoffset 0.6s ease;
}
.score-label {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -60%);
  display: flex;
  align-items: baseline;
  gap: 2px;
}
.score-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--score-color, #333);
}
.score-unit {
  font-size: 12px;
  color: #999;
}
.score-text {
  margin-top: 4px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color);
}
</style>
