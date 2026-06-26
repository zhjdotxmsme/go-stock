<template>
  <div class="action-checklist" v-if="items.length > 0">
    <h5 class="cl-title"><n-icon size="16" color="#2080f0"><List /></n-icon> 操作清单</h5>
    <n-checkbox-group v-model:value="completedActions">
      <div v-for="(item, i) in items" :key="i" class="checklist-row">
        <n-checkbox :value="i" :disabled="false">
          <span class="check-action" :class="{ done: completedActions.includes(i) }">
            {{ item.action }}
          </span>
        </n-checkbox>
        <n-tag v-if="item.priority === 'high'" size="tiny" type="error">高优</n-tag>
        <n-tag v-else-if="item.priority === 'low'" size="tiny" type="info">低优</n-tag>
      </div>
    </n-checkbox-group>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { List } from '@vicons/fa'

const props = defineProps({
  items: { type: Array, default: () => [] },
})

const completedActions = ref(
  props.items.map((item, i) => item.isCompleted ? i : -1).filter(i => i >= 0)
)
</script>

<style scoped>
.action-checklist {
  margin: 8px 0;
}
.cl-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
.checklist-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
}
.check-action {
  font-size: 13px;
  transition: color 0.2s;
}
.check-action.done {
  text-decoration: line-through;
  color: #999;
}
</style>
