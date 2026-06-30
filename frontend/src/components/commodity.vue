<script setup>
import {ref, onBeforeUnmount} from "vue";
import {EventsOn, EventsOff} from "../../wailsjs/runtime";
import {useRoute} from 'vue-router'
import CommodityOverview from "./CommodityOverview.vue";
import CommodityFutures from "./CommodityFutures.vue";
import CommodityFunds from "./CommodityFunds.vue";
import CommodityAnalysis from "./CommodityAnalysis.vue";

const nowTab = ref("行情总览")
const route = useRoute()

nowTab.value = route.query.name || '行情总览'

EventsOn("changeCommodityTab", async (msg) => {
  nowTab.value = msg.name
})

onBeforeUnmount(() => {
  EventsOff("changeCommodityTab")
})
</script>

<template>
  <n-card>
    <n-tabs type="line" animated v-model:value="nowTab" style="--wails-draggable:no-drag">
      <n-tab-pane name="行情总览" display-directive="show">
        <CommodityOverview/>
      </n-tab-pane>
      <n-tab-pane name="商品期货" display-directive="show">
        <CommodityFutures/>
      </n-tab-pane>
      <n-tab-pane name="商品基金" display-directive="show">
        <CommodityFunds/>
      </n-tab-pane>
      <n-tab-pane name="AI分析" display-directive="show">
        <CommodityAnalysis/>
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>
