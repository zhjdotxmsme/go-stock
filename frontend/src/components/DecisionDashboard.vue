<template>
  <div class="decision-dashboard" v-if="hasStructuredData">
    <h4 class="dashboard-title">📊 决策仪表盘</h4>
    <div class="dashboard-grid">
      <!-- 评分环 -->
      <div class="db-card score-card" v-if="report.score > 0">
        <ScoreRing :score="report.score" />
      </div>

      <!-- 趋势方向 -->
      <div class="db-card trend-card" v-if="report.trend">
        <TrendIndicator :trend="report.trend" />
      </div>

      <!-- 买卖区间 -->
      <div class="db-card zone-card" v-if="report.entryZone">
        <PriceZoneCard type="entry" :low="report.entryZone.low" :high="report.entryZone.high" />
      </div>
      <div class="db-card zone-card" v-if="report.exitZone">
        <PriceZoneCard type="exit" :low="report.exitZone.low" :high="report.exitZone.high" />
      </div>

      <!-- 风险等级 -->
      <div class="db-card risk-card" v-if="report.riskLevel">
        <RiskBadge :level="report.riskLevel" />
      </div>

      <!-- D5 决策标尺 -->
      <div class="db-card scale-card" v-if="report.decisionSignal">
        <DecisionScaleBar :signal="report.decisionSignal" />
      </div>
    </div>

    <!-- 催化剂时间线 -->
    <CatalystTimeline v-if="report.catalysts && report.catalysts.length > 0" :items="report.catalysts" />

    <!-- 操作清单 -->
    <ActionChecklist v-if="report.checklist && report.checklist.length > 0" :items="report.checklist" />

    <!-- 多时间维度 -->
    <div v-if="hasTimeframeView" class="timeframe-section">
      <h5 class="tf-title"><n-icon size="16" color="#2080f0"><Clock /></n-icon> 多时间维度</h5>
      <div class="tf-grid">
        <div v-for="(view, period) in report.multiTimeframeView" :key="period" class="tf-item">
          <span class="tf-period">{{ period }}</span>
          <span class="tf-view">{{ view }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Clock } from '@vicons/fa'
import ScoreRing from './ScoreRing.vue'
import TrendIndicator from './TrendIndicator.vue'
import PriceZoneCard from './PriceZoneCard.vue'
import RiskBadge from './RiskBadge.vue'
import DecisionScaleBar from './DecisionScaleBar.vue'
import CatalystTimeline from './CatalystTimeline.vue'
import ActionChecklist from './ActionChecklist.vue'

const props = defineProps({
  report: { type: Object, default: () => ({}) },
})

const hasStructuredData = computed(() => {
  const r = props.report
  return (r.score && r.score > 0) || r.trend || r.entryZone || r.exitZone || r.riskLevel ||
    (r.checklist && r.checklist.length > 0) ||
    (r.catalysts && r.catalysts.length > 0)
})

const hasTimeframeView = computed(() => {
  const v = props.report.multiTimeframeView
  return v && Object.keys(v).length > 0
})
</script>

<style scoped>
.decision-dashboard {
  margin: 16px 0;
  padding: 16px;
  background: var(--n-color);
  border-radius: 10px;
  border: 1px solid #e5e5e5;
}
.dashboard-title {
  margin: 0 0 12px;
  font-size: 16px;
  font-weight: 600;
}
.dashboard-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: flex-start;
}
.db-card {
  flex: 0 0 auto;
}
.timeframe-section {
  margin-top: 16px;
}
.tf-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
.tf-grid {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.tf-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 14px;
  border-radius: 8px;
  background: #f5f5f5;
  min-width: 100px;
}
.tf-period {
  font-size: 12px;
  color: #999;
  font-weight: 500;
}
.tf-view {
  font-size: 13px;
  font-weight: 600;
}
</style>
