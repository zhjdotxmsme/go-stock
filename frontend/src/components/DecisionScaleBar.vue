<template>
  <div class="scale-bar">
    <div class="scale-title">决策标尺</div>
    <div class="scale-bands">
      <div
        v-for="band in bands"
        :key="band.key"
        class="scale-band"
        :class="{ active: band.key === signal }"
        :style="band.key === signal ? { background: band.color, borderColor: band.color } : {}"
      >
        {{ band.label }}
      </div>
    </div>
  </div>
</template>

<script setup>
// D5 决策标尺展示（A4）：5 档静态刻度，高亮后端透出的当前档位。
// 红=买入侧、绿=卖出侧（A 股配色习惯），观望为中性灰。
defineProps({
  signal: { type: String, default: '' }, // strong_buy / buy / watch / reduce / sell
})

const bands = [
  { key: 'strong_buy', label: '强烈买入', color: '#d03050' },
  { key: 'buy', label: '买入', color: '#f0a020' },
  { key: 'watch', label: '观望', color: '#888' },
  { key: 'reduce', label: '减仓', color: '#70c0a8' },
  { key: 'sell', label: '卖出', color: '#18a058' },
]
</script>

<style scoped>
.scale-bar {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.scale-title {
  font-size: 12px;
  color: #999;
  font-weight: 500;
}
.scale-bands {
  display: flex;
  gap: 4px;
}
.scale-band {
  padding: 4px 10px;
  border-radius: 6px;
  border: 1px solid #e0e0e0;
  font-size: 12px;
  color: #999;
  background: #f5f5f5;
  white-space: nowrap;
}
.scale-band.active {
  color: #fff;
  font-weight: 600;
}
</style>
