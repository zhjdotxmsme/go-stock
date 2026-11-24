<template>
  <n-tabs v-model:value="tab" type="line" animated display-directive="show" @update:value="syncQuery">
    <n-tab-pane name="pipeline" tab="智能流水线">
      <SmartPipeline />
    </n-tab-pane>
    <n-tab-pane name="recommend" tab="推荐">
      <RecommendView @open-research-recommends="openResearchRecommends" />
    </n-tab-pane>
    <n-tab-pane name="khunter" tab="狩猎场">
      <KhunterPanel />
    </n-tab-pane>
    <n-tab-pane name="workbench" tab="选股工作台">
      <ScreeningWorkbench embedded @update:tab="syncQuery" />
    </n-tab-pane>
    <n-tab-pane name="kline" tab="K线分析">
      <KlineAnalysis embedded @back="syncQuery('recommend')" />
    </n-tab-pane>
  </n-tabs>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SmartPipeline from './SmartPipeline.vue'
import RecommendView from './RecommendView.vue'
import KhunterPanel from '../KhunterPanel.vue'
import ScreeningWorkbench from '../ScreeningWorkbench.vue'
import KlineAnalysis from '../kline-analysis.vue'
import { ANALYSIS_TABS, tabFromQuery, type AnalysisTab } from './recommend'

const route = useRoute()
const router = useRouter()
const tab = ref<AnalysisTab>(tabFromQuery(route.query.tab))

watch(() => route.query.tab, (v) => {
  const t = tabFromQuery(v)
  if (t !== tab.value) tab.value = t
})

function syncQuery(name: string) {
  // 工作台内部 tab（pattern/strategy）不是分析中心 Tab，忽略之，否则切换内部 Tab
  // 会被 tabFromQuery 回落成 recommend 而把用户踢出工作台
  if (!ANALYSIS_TABS.includes(name as AnalysisTab)) return
  const t = tabFromQuery(name)
  if (route.query.tab === t) return
  router.replace({ query: { ...route.query, tab: t } })
}

function openResearchRecommends() {
  router.push({ name: 'researchRecommends' })
}
</script>
