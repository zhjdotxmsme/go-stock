<script setup lang="ts">
import { computed } from 'vue'

interface Factor {
  name: string
  value: number
  color?: string
}

const props = withDefaults(defineProps<{
  factors: Factor[]
  max?: number
  height?: number
  width?: number
  showLabel?: boolean
  showValue?: boolean
}>(), {
  max: 1,
  height: 16,
  width: 120,
  showLabel: true,
  showValue: true,
})

function barColor(v: number): string {
  if (v >= 0.8) return '#18a058'
  if (v >= 0.5) return '#d48806'
  return '#909399'
}

const styledFactors = computed(() =>
  props.factors.map(f => ({
    ...f,
    pct: Math.min((f.value / props.max) * 100, 100),
    color: f.color || barColor(f.value),
  }))
)
</script>

<template>
  <div v-if="!factors?.length" style="color:#909399;font-size:11px">-</div>
  <div v-else class="factor-bar-container" :style="{ width: width + 'px' }">
    <div v-for="f in styledFactors" :key="f.name" class="factor-row" :style="{ height: height + 'px' }">
      <span v-if="showLabel" class="factor-label">{{ f.name }}</span>
      <div class="bar-track">
        <div class="bar-fill" :style="{ width: f.pct + '%', backgroundColor: f.color }" />
      </div>
      <span v-if="showValue" class="factor-value">{{ f.value.toFixed(2) }}</span>
    </div>
  </div>
</template>

<style scoped>
.factor-bar-container {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.factor-row {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
}
.factor-label {
  color: #909399;
  min-width: 24px;
  text-align: right;
  white-space: nowrap;
}
.bar-track {
  flex: 1;
  height: 6px;
  background-color: #f0f0f0;
  border-radius: 3px;
  overflow: hidden;
}
.bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.3s ease;
}
.factor-value {
  color: #606266;
  min-width: 32px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
</style>
