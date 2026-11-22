<script setup>
// 选股工作台：合并原「形态选股」「指标选股」两个入口。
// - 形态条件筛选：东财粗筛 + 本地信号引擎「本地验证」列（allStockList）
// - 组合策略与问财：追涨/抄底等组合 + 自然语言筛选（SelectStock）
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import allStockList from './allStockList.vue'
import SelectStock from './SelectStock.vue'

const props = defineProps({
  // 嵌入分析中心 Tab 壳时为 true：内部 tab 切换只通知父容器，不改路由
  embedded: { type: Boolean, default: false },
})
const emit = defineEmits(['update:tab'])

const route = useRoute()
const router = useRouter()

const activeTab = ref(route.query.tab === 'strategy' ? 'strategy' : 'pattern')

watch(activeTab, (v) => {
  if (props.embedded) {
    // 嵌入时不改父路由，否则会被踢出 Tab 容器
    emit('update:tab', v)
    return
  }
  router.replace({ name: 'analysisWorkbench', query: { tab: v } })
})

watch(() => route.query.tab, (v) => {
  if (props.embedded) return
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
