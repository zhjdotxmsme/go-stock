<script setup>

import * as marketApi from "../api/market";
import {onMounted, onUnmounted, ref, watch, nextTick} from "vue";
import MarketUpDownChart from "./market/MarketUpDownChart.vue";
import MarketDailyChart from "./market/MarketDailyChart.vue";
import MarketChangeStatsChart from "./market/MarketChangeStatsChart.vue";
import MarketChangeRankChart from "./market/MarketChangeRankChart.vue";
import MarketBullBearRankChart from "./market/MarketBullBearRankChart.vue";
import MarketTreemapChart from "./market/MarketTreemapChart.vue";
import MarketDimensionModal from "./market/MarketDimensionModal.vue";

const { name,darkTheme,kDays ,chartHeight} = defineProps({
  name: {
    type: String,
    default: ''
  },
  kDays: {
    type: Number,
    default: 14
  },
  chartHeight: {
    type: Number,
    default: 500
  },
  darkTheme: {
    type: Boolean,
    default: false
  }
})
const common = ref([])
const america = ref([])
const europe = ref([])
const asia = ref([])
const mainIndex = ref([])
const chinaIndex = ref([])
const other = ref([])
const globalStockIndexes = ref(null)
const showTreemap = ref(false);
const showDailyChart = ref(false);
const showChangeStats = ref(false);
const showChangeRank = ref(false);
const changeRankDays = ref(1);
const showBullBearRank = ref(false);
const bullBearDays = ref(1);
const showDimensionModal = ref(false);
const dimensionModalTitle = ref('');
const triggerAreas=ref(["main","extra","arrow"])

// Data refs for child components
const todayMarketData = ref([])
const dailyMarketData = ref([])
const changeStatsData = ref([])
const changeTypeData = ref([])
const changeRankResult = ref(null)
const bullBearResult = ref(null)
const dimensionModalData = ref([])
let currentDimension = ''
let currentDimensionName = ''

let handleChartInterval=null
let handleIndexInterval=null

onMounted(() => {
  handleChart()
  handleDailyChart()
  handleChangeRank()
  getIndex()
  handleChartInterval=setInterval(function () {
    handleChart()
  }, 1000 * 60)

  handleIndexInterval=setInterval(function () {
    getIndex()
  }, 1000 * 10)
})

onUnmounted(()=>{
  clearInterval(handleChartInterval)
  clearInterval(handleIndexInterval)
})

watch(showTreemap, (newVal) => {
  if (newVal) {
    nextTick(() => {
      // Treemap component handles its own data loading
    })
  }
})

watch(showDailyChart, (newVal) => {
  if (newVal) {
    handleDailyChart()
  }
})

watch(showChangeStats, (newVal) => {
  if (newVal) {
    handleChangeStats()
  }
})

watch(showChangeRank, (newVal) => {
  if (newVal) {
    handleChangeRank()
  }
})

watch(changeRankDays, () => {
  handleChangeRank()
})

watch(showBullBearRank, (newVal) => {
  if (newVal) {
    handleBullBearRank()
  }
})

watch(bullBearDays, () => {
  handleBullBearRank()
})

watch(showDimensionModal, (newVal) => {
  if (newVal) {
    handleDimensionDetail()
  }
})

function getIndex() {
  marketApi.getGlobalStockIndexes().then(({data: res}) => {
    globalStockIndexes.value = res
    common.value = res["common"]
    america.value = res["america"]
    europe.value = res["europe"]
    asia.value = res["asia"]
    other.value = res["other"]
    mainIndex.value=asia.value.filter(function (item) {
      return ['上海',"深圳","香港","台湾","北京","东京","首尔","纽约","纳斯达克"].includes(item.location)
    }).concat(america.value.filter(function (item) {
      return ['上海',"深圳","香港","台湾","北京","东京","首尔","纽约","纳斯达克"].includes(item.location)
    }))

    chinaIndex.value=asia.value.filter(function (item) {
      return ['上海',"深圳","香港","台湾","北京"].includes(item.location)
    })

  })
}

async function handleChart(){
  try {
    const data = (await marketApi.getTodayMarketStatistic()).data
    if (data && data.length > 0) {
      todayMarketData.value = data
    }
  } catch (error) {
    console.error('获取市场统计数据失败:', error)
  }
}

async function handleDailyChart() {
  try {
    const data = (await marketApi.getRecentDaysMarketStatistic(30)).data
    if (data && data.length > 0) {
      dailyMarketData.value = data
    }
  } catch (error) {
    console.error('获取历史市场统计数据失败:', error)
  }
}

async function handleChangeStats() {
  try {
    const [dailyStats, typeStats] = await Promise.all([
      marketApi.getDailyChangeStats(30),
      marketApi.getChangeTypeDailyStats(30)
    ]).then(([r1, r2]) => [r1.data, r2.data])
    if (dailyStats && dailyStats.length > 0) {
      changeStatsData.value = dailyStats
    }
    if (typeStats && typeStats.length > 0) {
      changeTypeData.value = typeStats
    }
  } catch (error) {
    console.error('获取异动统计数据失败:', error)
  }
}

async function handleChangeRank() {
  try {
    const days = changeRankDays.value
    const result = (await marketApi.getChangeRank(days, 20)).data
    if (result) {
      const hasData = (result.topStocks && result.topStocks.length > 0) ||
        (result.topIndustries && result.topIndustries.length > 0) ||
        (result.topConcepts && result.topConcepts.length > 0)
      if (days === 1 && !hasData) {
        const isTrading = (await marketApi.isTradingTime()).data
        if (!isTrading) {
          changeRankDays.value = 3
          return
        }
      }
      changeRankResult.value = result
    }
  } catch (error) {
    console.error('获取异动排行数据失败:', error)
  }
}

async function handleBullBearRank() {
  try {
    const days = bullBearDays.value
    const result = (await marketApi.getChangeRank(days, 20)).data
    if (result) {
      const hasData = (result.topStocks && result.topStocks.length > 0) ||
        (result.topIndustries && result.topIndustries.length > 0) ||
        (result.topConcepts && result.topConcepts.length > 0)
      if (days === 1 && !hasData) {
        const isTrading = (await marketApi.isTradingTime()).data
        if (!isTrading) {
          bullBearDays.value = 3
          return
        }
      }
      bullBearResult.value = result
    }
  } catch (error) {
    console.error('获取利好利空排行数据失败:', error)
  }
}

function openDimensionDetail(dimension, name) {
  currentDimension = dimension
  currentDimensionName = name
  const labels = { stock: '股票', industry: '行业', concept: '概念', type: '异动类型' }
  dimensionModalTitle.value = `${name} - 近30日${labels[dimension] || ''}异动趋势`
  showDimensionModal.value = true
}

async function handleDimensionDetail() {
  if (!currentDimension || !currentDimensionName) return
  try {
    if (currentDimension === 'date') {
      const data = (await marketApi.getTypeStatsByDate(currentDimensionName)).data
      if (data && data.length > 0) {
        dimensionModalData.value = data
      }
    } else {
      const data = (await marketApi.getDailyDimensionStats(currentDimension, currentDimensionName, 30)).data
      if (data && data.length > 0) {
        dimensionModalData.value = data
      }
    }
  } catch (error) {
    console.error('获取维度详情数据失败:', error)
  }
}

function onDimensionClick(dimension, name) {
  openDimensionDetail(dimension, name)
}

const changeRankPeriodLabel = () => {
  const days = changeRankDays.value
  return days === 1 ? '当日' : `近${days}日`
}
</script>

<template>
  <n-collapse :trigger-areas="triggerAreas" :default-expanded-names="['1']" display-directive="show">
    <n-collapse-item  name="1" >
      <template #header>
          <n-flex>
              <n-tag size="small" :bordered="false" v-for="(item, index) in mainIndex" :type="item.zdf>0?'error':'success'">
                <n-flex>
                  <n-image :width="20" :src="item.img" />
                  <n-text style="font-size: 14px" :type="item.zdf>0?'error':'success'">{{item.name}}&nbsp;{{item.zxj}}</n-text>
                  <n-number-animation :precision="2" :from="0" :to="item.zdf" style="font-size: 14px"/>
                  <n-text style="margin-left: -12px;font-size: 14px" :type="item.zdf>0?'error':'success'">%</n-text>
                </n-flex>
              </n-tag>
          </n-flex>
      </template>
      <template #header-extra>
        主要股指
      </template>
      <n-flex justify="end" style="margin-bottom: 4px">
        <n-button-group size="tiny">
          <n-button :type="changeRankDays===1?'primary':'default'" @click="changeRankDays=1">当日</n-button>
          <n-button :type="changeRankDays===3?'primary':'default'" @click="changeRankDays=3">近3日</n-button>
          <n-button :type="changeRankDays===5?'primary':'default'" @click="changeRankDays=5">近5日</n-button>
          <n-button :type="changeRankDays===10?'primary':'default'" @click="changeRankDays=10">近10日</n-button>
        </n-button-group>
      </n-flex>
      <n-grid :cols="24" :y-gap="0">
        <MarketUpDownChart :dark-theme="darkTheme" :chart-height="chartHeight" :data="todayMarketData" />
        <MarketChangeRankChart
          :dark-theme="darkTheme"
          :chart-height="chartHeight"
          :top-stocks="changeRankResult?.topStocks || []"
          :top-industries="changeRankResult?.topIndustries || []"
          :top-concepts="changeRankResult?.topConcepts || []"
          :period-label="changeRankPeriodLabel()"
          @dimension-click="onDimensionClick"
        />
      </n-grid>
      <n-flex justify="center" style="margin: 8px 0" :wrap="false">
        <n-button text @click="showTreemap = !showTreemap" :type="showTreemap?'primary':''">
          {{ showTreemap ? '隐藏热词' : '查看热词' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showDailyChart = !showDailyChart" :type="showDailyChart?'primary':''">
          {{ showDailyChart ? '隐藏按天分析' : '按天涨跌/涨跌停分析' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showChangeStats = !showChangeStats" :type="showChangeStats?'primary':''">
          {{ showChangeStats ? '隐藏异动分析' : '历史异动分析' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showChangeRank = !showChangeRank" :type="showChangeRank?'primary':''">
          {{ showChangeRank ? '隐藏异动排行' : '异动排行' }}
        </n-button>
        <n-divider vertical />
        <n-button text @click="showBullBearRank = !showBullBearRank" :type="showBullBearRank?'primary':''">
          {{ showBullBearRank ? '隐藏利好/利空排行' : '利好/利空排行' }}
        </n-button>
      </n-flex>
      <n-collapse-transition :show="showTreemap">
        <MarketTreemapChart :dark-theme="darkTheme" :chart-height="chartHeight" :name="name" />
      </n-collapse-transition>
      <n-collapse-transition :show="showDailyChart">
        <n-grid :cols="24" :y-gap="0">
          <MarketDailyChart :dark-theme="darkTheme" :chart-height="chartHeight" :data="dailyMarketData" />
        </n-grid>
      </n-collapse-transition>
      <n-collapse-transition :show="showChangeStats">
        <n-grid :cols="24" :y-gap="0">
          <MarketChangeStatsChart
            :dark-theme="darkTheme"
            :chart-height="chartHeight"
            :data="[...changeStatsData, ...changeTypeData]"
            @dimension-click="onDimensionClick"
          />
        </n-grid>
      </n-collapse-transition>
      <n-collapse-transition :show="showChangeRank">
        <n-grid :cols="24" :y-gap="0">
          <MarketChangeRankChart
            :dark-theme="darkTheme"
            :chart-height="chartHeight"
            :top-stocks="changeRankResult?.topStocks || []"
            :top-industries="changeRankResult?.topIndustries || []"
            :top-concepts="changeRankResult?.topConcepts || []"
            :period-label="changeRankPeriodLabel()"
            @dimension-click="onDimensionClick"
          />
        </n-grid>
      </n-collapse-transition>
      <n-collapse-transition :show="showBullBearRank">
        <n-flex justify="end" style="margin-bottom: 4px">
          <n-button-group size="tiny">
            <n-button :type="bullBearDays===1?'primary':'default'" @click="bullBearDays=1">当日</n-button>
            <n-button :type="bullBearDays===3?'primary':'default'" @click="bullBearDays=3">近3日</n-button>
            <n-button :type="bullBearDays===5?'primary':'default'" @click="bullBearDays=5">近5日</n-button>
            <n-button :type="bullBearDays===10?'primary':'default'" @click="bullBearDays=10">近10日</n-button>
            <n-button :type="bullBearDays===30?'primary':'default'" @click="bullBearDays=30">近30日</n-button>
          </n-button-group>
        </n-flex>
        <n-grid :cols="24" :y-gap="0">
          <MarketBullBearRankChart
            :dark-theme="darkTheme"
            :chart-height="chartHeight"
            :top-stocks="bullBearResult?.topStocks || []"
            :top-industries="bullBearResult?.topIndustries || []"
            :top-concepts="bullBearResult?.topConcepts || []"
            @dimension-click="onDimensionClick"
          />
        </n-grid>
      </n-collapse-transition>
    </n-collapse-item>
  </n-collapse>
  <MarketDimensionModal
    v-model:show="showDimensionModal"
    :title="dimensionModalTitle"
    :dark-theme="darkTheme"
    :dimension="currentDimension"
    :dimension-name="currentDimensionName"
    :data="dimensionModalData"
  />
</template>

<style scoped>

</style>
