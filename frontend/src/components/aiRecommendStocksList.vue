<script setup>
import {computed, h, nextTick, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import * as systemApi from "../api/system";
import * as marketApi from "../api/market";
import {EventsOn, EventsOff} from "../../wailsjs/runtime";
import {FlashOutline} from "@vicons/ionicons5";
import {NAvatar, NButton, NCard, NEllipsis, NGi, NGrid, NGridItem, NNumberAnimation, NSpace, NSwitch, NTag, NText, useMessage, useNotification} from "naive-ui";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import sparkLine from "./stockSparkLine.vue"
import {format} from "date-fns";

const notify = useNotification()
const vipLevel=ref("2");
const isValidVip=ref(true) // 是否会员：VIP功能已免费开放

// ── L1 Overview Stats ──
const statsRef = ref(null) // AiRecommendStats

onBeforeMount(()=> {
  systemApi.getConfig().then(({data: result}) => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })
})
onMounted(() => {
  systemApi.getAiRecommendStats().then(({data: s}) => statsRef.value = s)

  systemApi.getAiConfigs().then(({data: res}) => {
    aiConfigs.value = res
    recommendConfigId.value = res?.[0]?.ID ?? null
  })

  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    order: "desc",
    keyword: paginationReactive.keyword,
    startDate: paginationReactive.range[0],
    endDate: paginationReactive.range[1]
  }).then((data) => {
    console.log( data)
    dataRef.value = data.data
    paginationReactive.page = 1
    paginationReactive.pageCount = data.pageCount
    paginationReactive.itemCount = data.total
    loadingRef.value = false
  })
})

// ── AI 推荐（LLM 聊天工具调用写入 ai_recommend_stocks）──
const aiConfigs = ref([])
const recommendConfigId = ref(null)
const recommendCount = ref(5)
const recommendRunning = ref(false)
const recommendOutput = ref('')
const recommendScrollRef = ref(null)

function startAiRecommend() {
  if (recommendRunning.value) return
  if (!recommendConfigId.value) {
    message.warning('请先选择AI模型服务配置')
    return
  }
  recommendRunning.value = true
  recommendOutput.value = ''
  // 复用 SummaryStockNews 带工具链路：后端系统提示会强制 LLM 最后调用
  // CreateAiRecommendStocks 工具把推荐结果写入本页列表。
  const question = `请根据当前时间，结合市场新闻、板块热点和资金流向，推荐 ${recommendCount.value} 只值得关注的股票，逐只说明推荐理由（基本面/消息面/技术面），并调用 CreateAiRecommendStocks 工具保存推荐记录。`
  marketApi.summaryStockNews(question, recommendConfigId.value, null, true, false, "aiRecommendStocks", "")
}

function stopAiRecommend() {
  marketApi.abortSummaryStockNews()
}

function scrollRecommendBottom() {
  nextTick(() => {
    requestAnimationFrame(() => {
      const el = recommendScrollRef.value
      if (el) el.scrollTop = el.scrollHeight
    })
  })
}

EventsOn("aiRecommendStocks", async (msg) => {
  if (msg === "DONE") {
    recommendRunning.value = false
    message.success('AI 推荐完成，列表已刷新')
    handleSearch()
    systemApi.getAiRecommendStats().then(({data: s}) => statsRef.value = s)
    return
  }
  if (msg.content || msg.reasoning_content || msg.extraContent) {
    if (msg.content) recommendOutput.value += msg.content
    if (msg.reasoning_content) recommendOutput.value += msg.reasoning_content
    if (msg.extraContent) recommendOutput.value += msg.extraContent
    scrollRecommendBottom()
  }
})

onUnmounted(() => {
  EventsOff("aiRecommendStocks")
})
const message = useMessage()
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const editorDataRef = reactive({
  show: false,
  loading: false,
  darkTheme: false,
  chatId: "",
  modelName: "",
  CreatedAt: "",
  stockName: "",
  stockCode: "",
  question: "",
  content: "",
})
const dataRef = ref([])
const loadingRef = ref(true)

// StockClosePrice          string     `json:"StockClosePrice" md:"推荐时股票收盘价格"`
// StockPrePrice            string     `json:"stockPrePricePrice" md:"前一交易日股票价格"`
// RecommendReason          string     `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
// RecommendBuyPrice        string     `json:"recommendBuyPrice" md:"ai建议买入价"`
// RecommendStopProfitPrice string     `json:"recommendStopProfitPrice" md:"ai建议止盈价"`
// RecommendStopLossPrice   string     `json:"recommendStopLossPrice" md:"ai建议止损价"`
// RiskRemarks              string     `json:"riskRemarks" md:"风险提示"`
// Remarks                  string     `json:"remarks" md:"备注"`
const columnsRef = ref([
  {
    title: '推荐模型',
    key: 'modelName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.modelName })
    }
  },
  {
    title: '评级',
    key: 'rating',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.rating || '-' })
    }
  },
  {
    title: '推荐时间',
    key: 'dataTime',
    render(row, index) {
      //2026-01-14T22:13:27.2693252+08:00 格式化为常用时间格式
      return row.CreatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '板块概念',
    key: 'bkName'
  },
  {
    title: '股票名称',
    key: 'stockName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockName })
    }
  },
  {
    title: '股票代码',
    key: 'stockCode'
  },
  {
    title: '最新分时',
    key: 'stockCode',
    render(row, index) {
      return h(sparkLine, { idSuffix:row.ID, stockName: row.stockName, stockCode: row.stockCode, lastPrice: row.stockCurrentPrice, openPrice: row.stockPrePrice, tooltip: true }, )
    }
  },
  {
    title: '最新',
    key: 'stockCurrentPrice',
    minWidth: 120,
    render(row, index) {

      let diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrePrice))/ Number(row.stockPrePrice)*100).toFixed(2)

      if(Number(row.stockCurrentPrice)< Number(row.stockPrePrice)) {
        return [h(NText, { type: "success", bordered: false }, { default: () => row.stockCurrentPrice+` |  ${diff}%` })]
      } else {
        return [h(NText, { type: "error" , bordered: false}, { default: () => row.stockCurrentPrice+` |  ${diff}%` })]
      }
    }
  },
  {
    title: '推荐时',
    key: 'stockPrice',
    render(row, index) {

      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.stockPrice })
      }

      let diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrice))/ Number(row.stockPrice)*100).toFixed(2)
      let flagStr="暂平"
      let flag="info"
      if(Number(row.stockCurrentPrice)>Number(row.stockPrice)) {
        flagStr="暂赢 "+diff+"%"
        flag="error"
      }else if(Number(row.stockCurrentPrice)===Number(row.stockPrice)){
        flagStr="暂平"
        flag="info"
      }else{
        flagStr="暂亏 "+ diff+"%"
        flag="success"
      }

      return [h(NText, { type: "info" }, { default: () => row.stockPrice }),h(NTag, { type: flag,size: "tiny", bordered: false }, { default: () => flagStr })]
    }
  },
  {
    title: '昨收',
    key: 'stockPrePrice',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockPrePrice })
    }
  },
  {
    title: '开仓价',
    key: 'recommendBuyPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })
      }


      if(row.recommendBuyPrice.includes("-")){
        let prices= row.recommendBuyPrice.split("-")
        if(Number(row.stockCurrentPrice)>=Number(prices[0])&&Number(row.stockCurrentPrice)<=Number(prices[1])){
          return [h(NText, { type: "success" }, { default: () => row.recommendBuyPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })]
        }
      }
      if(row.recommendBuyPriceMin&&row.recommendBuyPriceMax&&Number(row.stockCurrentPrice)<Number(row.recommendBuyPriceMax)&&Number(row.stockCurrentPrice)>Number(row.recommendBuyPriceMin)){
        return [h(NText, { type: "success" }, { default: () => row.recommendBuyPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })]
      }
      return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })

    }
  },
  {
    title: '止盈价',
    key: 'recommendStopProfitPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
      }
      if(row.recommendStopProfitPrice.includes("-")){
        let prices= row.recommendStopProfitPrice.split("-")
        if(Number(row.stockCurrentPrice)>=Number(prices[0])&&Number(row.stockCurrentPrice)<=Number(prices[1])){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopProfitPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }
      if(row.recommendStopProfitPriceMin&&Number(row.stockCurrentPrice)>row.recommendStopProfitPriceMin){
        return [h(NText, { type: "success" }, { default: () => row.recommendStopProfitPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
      }

      return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
    }
  },
  {
    title: '止损价',
    key: 'recommendStopLossPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })
      }
      if(row.recommendStopLossPrice.includes("-")){
        let prices= row.recommendStopLossPrice.split("-")
        if(Number(row.stockCurrentPrice)<=Number(prices[0])){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopLossPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }else{
        let prices=row.recommendStopLossPrice
        if(Number(row.stockCurrentPrice)<=Number(prices)){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopLossPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }
      return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })

    }
  },
  {
    title: '信号',
    key: 'signal',
    width: 100,
    render(row) {
      const cur = Number(row.stockCurrentPrice)
      const badges = []

      // buy zone check
      const buyMin = Number(row.recommendBuyPriceMin)
      const buyMax = Number(row.recommendBuyPriceMax)
      if (buyMin && buyMax && cur >= buyMin && cur <= buyMax) {
        badges.push(h(NTag, { type: 'error', size: 'tiny', bordered: false }, () => '买入'))
      } else if (row.recommendBuyPrice && String(row.recommendBuyPrice).includes('-')) {
        const parts = String(row.recommendBuyPrice).split('-')
        if (cur >= Number(parts[0]) && cur <= Number(parts[1])) {
          badges.push(h(NTag, { type: 'error', size: 'tiny', bordered: false }, () => '买入'))
        }
      }

      // tp zone check
      const tpMin = Number(row.recommendStopProfitPriceMin)
      if (tpMin && cur >= tpMin) {
        badges.push(h(NTag, { type: 'warning', size: 'tiny', bordered: false }, () => '止盈'))
      } else if (row.recommendStopProfitPrice && String(row.recommendStopProfitPrice).includes('-')) {
        const parts = String(row.recommendStopProfitPrice).split('-')
        if (cur >= Number(parts[0]) && cur <= Number(parts[1])) {
          badges.push(h(NTag, { type: 'warning', size: 'tiny', bordered: false }, () => '止盈'))
        }
      }

      // sl zone check
      const sl = Number(row.recommendStopLossPrice)
      if (sl && cur <= sl) {
        badges.push(h(NTag, { type: 'success', size: 'tiny', bordered: false }, () => '止损'))
      } else if (row.recommendStopLossPrice && String(row.recommendStopLossPrice).includes('-')) {
        const parts = String(row.recommendStopLossPrice).split('-')
        if (cur <= Number(parts[0])) {
          badges.push(h(NTag, { type: 'success', size: 'tiny', bordered: false }, () => '止损'))
        }
      }

      if (badges.length === 0) {
        return h(NTag, { type: 'default', size: 'tiny', bordered: false }, () => '持有')
      }
      return badges
    }
  },
  {
    title: '推荐理由',
    key: 'recommendReason',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '风险提示',
    key: 'riskRemarks',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '备注',
    key: 'remarks',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '监控预警',
    key: 'enableAlert',
    width: 80,
    render(row, index) {
      return h(NSwitch, {
        value: row.enableAlert,
        onUpdateValue: (newValue) => toggleAlert(row, newValue)
      })
    }
  },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NTag,
          {
            strong: true,
            tertiary: true,
            //size: 'small',
            type: 'warning', // 橙色按钮
            onClick: () => showDetail(row)
          },
          { default: () => '查看' }
      ),h(NTag, { strong: true,
        tertiary: true, type: 'error',  onClick: () => deleteAiRecommendStocks(row.ID) }, { default: () => '删除' })]
    }
  },
])
const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: "",
  enableAlert: null, // null 表示全部，true 表示已开启，false 表示未开启
  range: [
    new Date(new Date().getTime() - 3 * 24 * 60 * 60 * 1000), // 前3天
    new Date() // 当天
  ],
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const enableAlertOptions = [
  { label: '全部', value: null },
  { label: '已开启预警', value: true },
  { label: '未开启预警', value: false }
]

const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
  /** 传给 K 线组件的多单价位（与 StockLightweightKlineChart v-model 同步） */
  longEntryPrice: '',
  longStopLossPrice: '',
  longTakeProfitPrice: '',
})

const totalCount = computed(() => {
  if (!statsRef.value) return 0
  let sum = 0
  for (const m of statsRef.value.byModel) sum += m.count
  return sum
})

const theme = computed(() => {
  return editorDataRef.darkTheme ? 'dark' : 'light'
})


function query({
                 page,
                 pageSize = 10,
                 order = 'desc',
                 keyword = "",
                 startDate = "",
                 endDate = "",
                 enableAlert = null
               }) {
  return new Promise((resolve) => {

    systemApi.getAiRecommendStocksList({
      "page": page,
      "pageSize": pageSize,
      "modelName":keyword,
      "stockName":keyword,
      "stockCode":keyword,
      "bkName":keyword,
      "startDate": startDate,
      "endDate": endDate,
      "enableAlert": enableAlert
    }).then(({data: res}) => {
      const pagedData =res.list
      const total = res.total
      const pageCount =res.totalPages
      resolve({
        pageCount,
        data: pagedData,
        total
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]), // Format date to string
      endDate: formatDate(paginationReactive.range[1]), // Format date to string
      enableAlert: paginationReactive.enableAlert
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: paginationReactive?.page ?? 1,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1]),
      enableAlert: paginationReactive.enableAlert
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = data.page
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function formatDate(dateString) {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  // const hours = String(date.getHours()).padStart(2, '0')
  // const minutes = String(date.getMinutes()).padStart(2, '0')
  // const seconds = String(date.getSeconds()).padStart(2, '0')
  //return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  return `${year}-${month}-${day}`
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}

/** 推荐价可能为区间 "a-b"，取左侧作为图上开仓/止损/止盈线参考价 */
function recommendRangeToSinglePrice(p) {
  if (p == null || String(p).trim() === '') return ''
  const s = String(p).trim()
  const i = s.indexOf('-')
  if (i > 0) return s.slice(0, i).trim()
  return s
}

function showDetail(row) {
  if(vipLevel.value===""|| Number(vipLevel.value) <=0){
    notify.warning({content: '未开通VIP或者已经过期'})
    return
  }
  modalDataRef.title = row.stockName
  modalDataRef.content = row.recommendReason
  modalDataRef.riskRemarks = row.riskRemarks
  modalDataRef.stockCode = getStockCode(row.stockCode)
  modalDataRef.stockName = row.stockName
  modalDataRef.visible = true
  modalDataRef.remarks = row.remarks
  modalDataRef.longEntryPrice = recommendRangeToSinglePrice(row.recommendBuyPrice)
  modalDataRef.longStopLossPrice = recommendRangeToSinglePrice(row.recommendStopLossPrice)
  modalDataRef.longTakeProfitPrice = recommendRangeToSinglePrice(row.recommendStopProfitPrice)
}
function rowProps(row) {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      showDetail(row)
    }
  }
}
function deleteAiRecommendStocks(id) {
  systemApi.deleteAiRecommendStocks(id).then(({data: res}) => {
    notify.info({content: res, duration: 2000})
    handleSearch()
  })
}

function toggleAlert(row, newEnableAlert) {
  systemApi.updateAiRecommendStocksAlert(row.ID, newEnableAlert).then(({data: res}) => {
    notify.info({content: res, duration: 2000})
    // 更新本地数据
    row.enableAlert = newEnableAlert
  })
}

</script>

<template>
  <!-- AI 推荐：LLM 聊天工具调用写入 -->
  <n-card size="small" class="ai-recommend-card">
    <template #header>
      <n-space align="center">
        <n-text strong>AI 推荐股票</n-text>
        <n-text depth="3" style="font-size:12px">由 LLM 分析市场新闻后，通过工具调用把推荐写入下方列表</n-text>
      </n-space>
    </template>
    <n-space vertical>
      <n-space align="center">
        <n-select v-model:value="recommendConfigId" label-field="name" value-field="ID"
                  :options="aiConfigs" placeholder="选择AI模型服务配置" style="width:280px" />
        <n-text depth="3" style="font-size:12px">推荐数量</n-text>
        <n-input-number v-model:value="recommendCount" :min="1" :max="20" style="width:90px" />
        <n-button type="primary" :loading="recommendRunning" @click="startAiRecommend">
          <template #icon><n-icon><FlashOutline /></n-icon></template>开始推荐
        </n-button>
        <n-button :disabled="!recommendRunning" @click="stopAiRecommend">停止</n-button>
      </n-space>
      <n-alert type="info" :bordered="false" style="font-size:12px">
        点击「开始推荐」后，AI 会分析当前市场新闻并生成推荐；后端系统提示会强制 LLM 最后调用
        <n-text code>CreateAiRecommendStocks</n-text> 工具，把推荐结果保存到本页列表，生成完成后自动刷新。
      </n-alert>
      <div v-if="recommendOutput" ref="recommendScrollRef" class="recommend-output">{{ recommendOutput }}</div>
    </n-space>
  </n-card>

  <!-- L1 Overview Dashboard -->
  <n-space vertical v-if="statsRef" class="l1-dashboard">
    <n-grid :cols="4" :x-gap="12">
      <n-grid-item>
        <n-card size="small" title="模型表现">
          <n-table size="small" :single-line="false">
            <thead><tr><th>模型</th><th>推荐数</th><th>胜率</th><th>平均收益</th></tr></thead>
            <tbody>
              <tr v-for="m in statsRef.byModel" :key="m.modelName">
                <td><n-text>{{ m.modelName }}</n-text></td>
                <td><n-number-animation :from="0" :to="m.count" /></td>
                <td><n-text :type="m.winRate >= 50 ? 'error' : 'success'">{{ m.winRate }}%</n-text></td>
                <td><n-text :type="m.avgReturn >= 0 ? 'error' : 'success'">{{ m.avgReturn }}%</n-text></td>
              </tr>
            </tbody>
          </n-table>
        </n-card>
      </n-grid-item>
      <n-grid-item>
        <n-card size="small" title="板块分布">
          <n-table size="small" :single-line="false">
            <thead><tr><th>板块</th><th>推荐数</th></tr></thead>
            <tbody>
              <tr v-for="s in statsRef.bySector" :key="s.bkName">
                <td><n-tag size="tiny">{{ s.bkName }}</n-tag></td>
                <td><n-number-animation :from="0" :to="s.count" /></td>
              </tr>
            </tbody>
          </n-table>
        </n-card>
      </n-grid-item>
      <n-grid-item>
        <n-card size="small" title="每日推荐">
          <n-table size="small" :single-line="false">
            <thead><tr><th>日期</th><th>数量</th></tr></thead>
            <tbody>
              <tr v-for="d in statsRef.dailyCount" :key="d.date">
                <td>{{ d.date }}</td>
                <td><n-number-animation :from="0" :to="d.count" /></td>
              </tr>
            </tbody>
          </n-table>
        </n-card>
      </n-grid-item>
      <n-grid-item>
        <n-card size="small" title="总计">
          <n-text style="font-size:2em;font-weight:700">
            <n-number-animation :from="0" :to="totalCount" />
          </n-text>
          <n-text depth="3">条推荐</n-text>
        </n-card>
      </n-grid-item>
    </n-grid>
  </n-space>

  <n-input-group>
    <n-date-picker  v-model:value="paginationReactive.range" type="daterange"   style="width: 40%"/>
    <n-select v-model:value="paginationReactive.enableAlert" :options="enableAlertOptions" placeholder="预警状态" style="width: 15%" clearable />
    <n-input clearable placeholder="输入关键词搜索" v-model:value="paginationReactive.keyword"/>
    <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
      搜索
    </n-button>
  </n-input-group>
        <n-data-table
            remote
            size="small"
            :columns="columnsRef"
            :data="dataRef"
            :loading="loadingRef"
            :pagination="paginationReactive"
            :row-key="(rowData)=>rowData.ID"
            @update:page="handlePageChange"
            flex-height
            style="height: calc(100vh - 210px);margin-top: 10px"
        />

  <n-modal v-model:show="modalDataRef.visible" :title="modalDataRef.title" preset="card" style="max-width: 1400px;">
    <n-gradient-text :size="16" type="warning">{{modalDataRef.remarks}}</n-gradient-text>
    <n-card size="small">
      <StockLightweightKlineChart
        style="width: 100%;"
        :code="modalDataRef.stockCode"
        :chart-height="350"
        :stock-name="modalDataRef.stockName"
        :dark-theme="editorDataRef.darkTheme"
        v-model:long-entry-price="modalDataRef.longEntryPrice"
        v-model:long-stop-loss-price="modalDataRef.longStopLossPrice"
        v-model:long-take-profit-price="modalDataRef.longTakeProfitPrice"
      />
    </n-card>
    <n-card size="small">
    <n-text type="info">{{modalDataRef.content}}</n-text>
    <n-divider><n-gradient-text type="error">风险提示</n-gradient-text></n-divider>
    <n-text type="error">{{modalDataRef.riskRemarks}}</n-text>
    </n-card>
  </n-modal>
</template>

<style scoped>
.ai-recommend-card {
  margin-bottom: 12px;
}
.recommend-output {
  max-height: 240px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 13px;
  line-height: 1.7;
  color: #606266;
  background: rgba(128, 128, 128, 0.06);
  border-radius: 6px;
  padding: 10px 12px;
}
.l1-dashboard {
  margin-bottom: 12px;
}
.l1-dashboard .n-card {
  height: 100%;
}
.l1-dashboard .n-table {
  font-size: 12px;
}
</style>