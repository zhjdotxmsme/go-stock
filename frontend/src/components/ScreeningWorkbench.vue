<script setup>
// 选股工作台：合并原「形态选股」「指标选股」两个入口。
// - 形态条件筛选：东财粗筛 + 本地信号引擎「本地验证」列（allStockList）
// - 组合策略与问财：追涨/抄底等组合 + 自然语言筛选（SelectStock）
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import allStockList from './allStockList.vue'
import SelectStock from './SelectStock.vue'

const route = useRoute()
const router = useRouter()

const activeTab = ref(route.query.tab === 'strategy' ? 'strategy' : 'pattern')

watch(activeTab, (v) => {
  router.replace({ name: 'analysisWorkbench', query: { tab: v } })
})

watch(() => route.query.tab, (v) => {
  const t = v === 'strategy' ? 'strategy' : 'pattern'
  if (activeTab.value !== t) activeTab.value = t
})
</script>

<template>
  <n-tabs v-model:value="activeTab" type="line" animated>
    <n-tab-pane name="pattern" tab="形态条件筛选">
      <allStockList v-if="activeTab === 'pattern'" />
    </n-tab-pane>
    <n-tab-pane name="strategy" tab="组合策略与问财">
      <SelectStock v-if="activeTab === 'strategy'" />
    </n-tab-pane>
  </n-tabs>
</template>

<style scoped>
</style>
