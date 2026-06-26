<template>
  <div class="catalyst-timeline" v-if="items.length > 0">
    <h5 class="tl-title"><n-icon size="16" color="#2080f0"><Rocket /></n-icon> 催化剂</h5>
    <n-timeline>
      <n-timeline-item
        v-for="(item, i) in items"
        :key="i"
        :content="item"
        :type="'info'"
        :time="''"
      />
    </n-timeline>
  </div>
</template>

<script setup>
import { Rocket } from '@vicons/fa'

defineProps({
  items: { type: Array, default: () => [] },
})
</script>

<style scoped>
.catalyst-timeline {
  margin: 8px 0;
}
.tl-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
</style>
