<template>
  <div class="risk-badge" :class="level">
    <n-icon size="16">
      <ExclamationCircle v-if="level === 'high'" />
      <Info v-else-if="level === 'medium'" />
      <CheckCircle v-else />
    </n-icon>
    <span class="risk-label">{{ label }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ExclamationCircle, Info, CheckCircle } from '@vicons/fa'

const props = defineProps({
  level: { type: String, default: 'medium' }, // low / medium / high
})

const label = computed(() => {
  if (props.level === 'high') return '高风险'
  if (props.level === 'medium') return '中等风险'
  return '低风险'
})
</script>

<style scoped>
.risk-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 16px;
  font-size: 13px;
  font-weight: 500;
}
.risk-badge.low { background: #e8f8e8; color: #18a058; }
.risk-badge.medium { background: #fff3e0; color: #f0a020; }
.risk-badge.high { background: #fce8e8; color: #d03050; }
</style>
