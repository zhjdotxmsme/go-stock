<script setup>
import {h, onBeforeMount, onMounted, ref, reactive} from 'vue'
import * as stockApi from "../api/stock"
import * as systemApi from "../api/system"
import {NButton, NInput, NTag, NText, NIcon, NTooltip, NPopover, useMessage, useNotification, NDataTable, NSpace, NPagination} from "naive-ui";
import {HelpCircleOutline} from "@vicons/ionicons5";
import sparkLine from "./stockSparkLine.vue"
import klineChart from "./KLineChart.vue"
import KLineChart from "./KLineChart.vue";

// 形态筛选选项元数据：分组 + 每项说明（悬停显示）。
// 与「K线分析」页技术指标的区别：这里是把指标/形态「信号化」为布尔筛选条件，
// 用于从全市场过滤个股；K线分析页的指标是画在单只股票图上的连续分析工具。
const filterGroups = [
  {
    name: '指标信号',
    desc: '由技术指标（MACD/KDJ/均线）派生的信号筛选',
    items: [
      {key: 'MACD_GOLDEN_FORK', label: 'MACD金叉', tip: '快线DIF上穿慢线DEA，短期动能转强，常见买入信号；零轴上方的金叉强度更高'},
      {key: 'KDJ_GOLDEN_FORK', label: 'KDJ金叉', tip: 'K线上穿D线，短线反弹信号；在20以下超卖区发生的金叉可靠性更高'},
      {key: 'BREAKUP_MA_5DAYS', label: '向上突破5日均线', tip: '收盘价站上5日均线，短线转强的初步信号'},
      {key: 'LONG_AVG_ARRAY', label: '均线多头排列', tip: '短/中/长期均线自上而下依次排列且向上发散，上升趋势的典型结构'},
      {key: 'SHORT_AVG_ARRAY', label: '均线空头排列', tip: '短/中/长期均线自下而上依次排列且向下发散，下降趋势，回避为主'},
    ],
  },
  {
    name: 'K线形态',
    desc: '对单根/多根K线价格组合的模式识别（非指标）',
    items: [
      {key: 'ONE_DAYANG_LINE', label: '一根大阳线', tip: '单日收出实体明显的大阳线，买方力量强劲'},
      {key: 'TWO_DAYANG_LINES', label: '两根大阳线', tip: '连续两根大阳线，强势上攻形态'},
      {key: 'UPPER_4DAYS', label: '四串阳', tip: '连续4根阳线稳步推升，多头持续占优'},
      {key: 'UPPER_8DAYS', label: '八仙过海(八连阳)', tip: '连续8日收阳，极端强势；高位出现时也需防物极必反'},
      {key: 'UPPER_9DAYS', label: '九阳神功(九连阳)', tip: '连续9日收阳的极端强势形态，注意高位滞涨风险'},
      {key: 'DOWN_7DAYS', label: '七仙女下凡(七连阴)', tip: '连续7日收阴，严重超卖，可关注反弹机会'},
      {key: 'RISE_SUN', label: '旭日东升', tip: '下跌中阴线之后高开高走收大阳线，收盘超过前阴开盘价，见底反转信号'},
      {key: 'POWER_FULGUN', label: '强势多方炮', tip: '两阳夹一阴：阳线实体大、阴线实体小，上涨中继的强势信号'},
      {key: 'RESTORE_JUSTICE', label: '拨云见日', tip: '连续下跌后出现带量长阳收复失地，底部反转形态'},
      {key: 'MORNING_STAR', label: '早晨之星', tip: '大阴线→星线→大阳线的三根组合，经典底部反转信号'},
      {key: 'FIRST_DAWN', label: '曙光初现', tip: '大阴线后的大阳线深入前阴线实体一半以上，见底信号'},
      {key: 'REVERSING_HAMMER', label: '倒转锤头', tip: '下跌末端出现上影线长、实体小的K线，试探性反攻信号'},
      {key: 'PREGNANT', label: '身怀六甲', tip: '大K线后紧跟的小K线实体被完全包含，趋势减速/反转预警'},
      {key: 'NARROW_FINISH', label: '窄幅整理', tip: '股价在小范围横盘收敛、波动率压缩，常预示即将选择方向（变盘）'},
      {key: 'BLACK_CLOUD_TOPS', label: '乌云盖顶', tip: '大阳线后高开低走的大阴线深入阳线实体，经典顶部反转信号'},
      {key: 'EVENING_STAR', label: '黄昏之星', tip: '大阳线→星线→大阴线的三根组合，顶部反转信号'},
      {key: 'SHOOTING_STAR', label: '射击之星', tip: '上涨末端出现上影线长、实体小的K线，冲高回落的见顶预警'},
      {key: 'BEARISH_ENGULFING', label: '穿头破脚', tip: '大阴线完全吞没前一根阳线实体，见顶反转信号（看跌）'},
    ],
  },
  {
    name: '量价',
    desc: '成交量与价格配合关系筛选',
    items: [
      {key: 'BREAK_THROUGH', label: '放量突破', tip: '成交量显著放大且价格突破关键阻力（均线/前高），资金涌入，突破有效性高'},
      {key: 'UPPER_LARGE_VOLUME', label: '连涨放量', tip: '连续上涨且成交量持续放大，上涨有资金支撑，趋势健康'},
      {key: 'UPSIDE_VOLUME', label: '放量上攻', tip: '价涨量增，真金白银推动的上涨，动能充足'},
      {key: 'DOWN_NARROW_VOLUME', label: '下跌无量', tip: '下跌过程中成交量萎缩，抛压衰竭，接近阶段底部的特征'},
      {key: 'HEAVEN_RULE', label: '天量法则', tip: '出现阶段天量成交、剧烈换手，常预示短期拐点（可能是突破也可能是顶部）'},
    ],
  },
  {
    name: '资金流',
    desc: '主力资金流向筛选（资金面数据，非技术指标）',
    items: [
      {key: 'LOW_FUNDS_INFLOW', label: '低位资金净流入', tip: '股价处于阶段低位但主力资金持续流入，疑似主力吸筹'},
      {key: 'HIGH_FUNDS_OUTFLOW', label: '高位资金净流出', tip: '股价处于阶段高位但主力资金持续流出，警惕主力出货'},
    ],
  },
]

const rankGroups = [
  {key: 'UPP_DAYS', name: '人气排名连涨', tip: '市场人气（关注度）排名连续N天上升，热度在积聚', options: [3, 5, 7]},
  {key: 'CONCERN_RANK_7DAYS', name: '7日关注排名', tip: '近7日市场关注榜排名进入前N名（情绪面数据）', options: [10, 50, 100]},
  {key: 'UPNDAY', name: '连涨天数', tip: '收盘价连续N天上涨', options: [3, 5, 8]},
  {key: 'DOWNNDAY', name: '连跌天数', tip: '收盘价连续N天下跌，超卖反弹观察池', options: [3, 5, 8, 10, 14]},
]

const notify = useNotification()
const message = useMessage()

const editorDataRef = reactive({
  darkTheme: false
})

onBeforeMount(() => {
  systemApi.getConfig().then(({data: result}) => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })
})

onMounted(() => {
  console.log('stock-list mounted')
  loadStocks(1, paginationReactive.pageSize)
})

const dataRef = ref([])
const loadingRef = ref(false)
const columnsRef = ref([
  // {
  //   title: '数据时间',
  //   key: 'MAX_TRADE_DATE',
  //   width: 120,
  // },
  {
    title: '股票代码',
    key: 'SECUCODE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.SECUCODE })
    }
  },
  {
    title: '股票名称',
    key: 'SECURITY_NAME_ABBR',
    width: 100,
    render(row) {
      return h(NText, { type: "success" }, { default: () => row.SECURITY_NAME_ABBR })
    }
  },
  {
    title: '最新价',
    key: 'NEW_PRICE',
    width: 100,
    render(row) {
      const price = row.NEW_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  {
    title: '涨跌幅(%)',
    key: 'CHANGE_RATE',
    width: 100,
    render(row) {
      const rate = toNumber(row.CHANGE_RATE, 0)
      const type = rate >= 0 ? 'error' : 'success'
      const sign = rate >= 0 ? '+' : ''
      return h(NText, { type: type }, { default: () => `${sign}${rate.toFixed(2)}%` })
    }
  },
  {
    title: '分时图',
    key: 'sparkline',
    width: 120,
    render(row) {
      return h(sparkLine, {
        idSuffix: row.SECUCODE,
        stockName: row.SECURITY_NAME_ABBR,
        stockCode: row.SECUCODE,
        lastPrice: row.NEW_PRICE,
        openPrice: row.PRE_CLOSE_PRICE,
        tooltip: true
      })
    }
  },
  {
    title: '最高价',
    key: 'HIGH_PRICE',
    width: 100,
    render(row) {
      const price = row.HIGH_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  {
    title: '最低价',
    key: 'LOW_PRICE',
    width: 100,
    render(row) {
      const price = row.LOW_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  // {
  //   title: '前收价',
  //   key: 'PRE_CLOSE_PRICE',
  //   width: 100,
  //   render(row) {
  //     return h(NText, { type: "info" }, { default: () => row.PRE_CLOSE_PRICE.toFixed(2) })
  //   }
  // },
  {
    title: '成交量',
    key: 'VOLUME',
    width: 120,
    render(row) {
      const volume = toNumber(row.VOLUME, 0)
      let displayVolume = volume
      if (volume >= 100000000) {
        displayVolume = (volume / 100000000).toFixed(2) + '亿'
      } else if (volume >= 10000) {
        displayVolume = (volume / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayVolume })
    }
  },
  {
    title: '成交额',
    key: 'DEAL_AMOUNT',
    width: 120,
    render(row) {
      const amount = toNumber(row.DEAL_AMOUNT, 0)
      let displayAmount = amount
      if (amount >= 100000000) {
        displayAmount = (amount / 100000000).toFixed(2) + '亿'
      } else if (amount >= 10000) {
        displayAmount = (amount / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayAmount })
    }
  },
  {
    title: '换手率 (%)',
    key: 'TURNOVERRATE',
    width: 80,
    render(row) {
      const rate = row.TURNOVERRATE
      return h(NText, { type: "info" }, { default: () => isNumeric(rate) ? rate : '-' })
    }
  },
  {
    title: '量比',
    key: 'VOLUME_RATIO',
    width: 80,
    render(row) {
      const ratio = row.VOLUME_RATIO
      return h(NText, { type: "info" }, { default: () => isNumeric(ratio) ? ratio : '-' })
    }
  },
  {
    title: '所属行业',
    key: 'INDUSTRY',
    width: 100,
    render(row) {
      return h(NTag, { type: "primary", size: "small" }, { default: () => row.INDUSTRY })
    }
  },
  {
    title: '所属概念',
    key: 'CONCEPT',
    width: 100,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      if(typeof row.CONCEPT === 'string'){
        return h(NTag, { type: "info", size: "small" ,style: "margin-right: 4px;" }, { default: () => row.CONCEPT })
      }else{
        if (!row.CONCEPT || row.CONCEPT.length === 0) {
          return h(NText, { type: "secondary" }, { default: () => '无' })
        }
        return row.CONCEPT.map(concept =>
            h(NTag, { type: "info", size: "small", style: "margin-right: 4px;" }, { default: () => concept })
        )
      }
    }
  },
  // {
  //   title: '交易所',
  //   key: 'MARKET',
  //   width: 100,
  //   render(row) {
  //     return h(NTag, { type: "warning", size: "small" }, { default: () => row.MARKET })
  //   }
  // },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NButton,
          {
            secondary: true,
            size: 'small',
            type: 'warning', // 橙色按钮
            onClick: () => showKline(row)
          },
          { default: () => '日K' }
      ),]
    }
  },
])

const paginationReactive = reactive({
  keyword:"",
  page: 1,
  pageCount: 1,
  pageSize: 9,
  itemCount: 0,
  prefix({ itemCount }) {
    return `${itemCount} 只股票`
  }
})
const optionsReactive= reactive([
  {
    label: '全部',
    value: ''
  },
 ])

function loadStocks(page, pageSize) {
  if (!loadingRef.value) {
    loadingRef.value = true
    stockApi.getAllStocks(page, pageSize, paginationReactive.keyword, technicalIndicatorReactive).then(({data: res}) => {
      console.log(res)
      if (res && res.result && res.result.data) {
        dataRef.value = res.result.data
        paginationReactive.page = page
        paginationReactive.pageCount = Math.ceil(res.result.count / pageSize)
        paginationReactive.itemCount = res.result.count
      } else {
        dataRef.value = []
        paginationReactive.page = 1
        paginationReactive.pageCount = 1
        paginationReactive.itemCount = 0
        message.error('获取股票数据失败')
      }
      loadingRef.value = false
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
      loadingRef.value = false
    })
  }
}
function handleReset(){
  for (const g of filterGroups) {
    for (const item of g.items) {
      technicalIndicatorReactive[item.key] = false
    }
  }
  for (const g of rankGroups) {
    technicalIndicatorReactive[g.key] = 0
  }
}

function handlePageSizeChange(pageSize) {
  paginationReactive.pageSize = pageSize
  loadStocks(1, pageSize)
}
function handleSearch() {
  loadStocks(1, paginationReactive.pageSize)
}
function handleUpdateVal(value) {
  console.log('handleUpdateVal', value)
  if (value === '') {
    optionsReactive.splice(1, optionsReactive.length - 1)
  } else {
    stockApi.getAllStockInfoList({
      searchKeyWord: value
    }).then(({data: res}) => {
      console.log('GetAllStockInfoList result:', res)
      if (res  && res.list) {
        optionsReactive.splice(1, optionsReactive.length - 1)
        optionsReactive.push(...res.list.map(item => {
          return {
            label: item.SECURITY_NAME_ABBR,
            value: item.SECURITY_NAME_ABBR,
            obj: item,
          }
        }))
      }
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
    })
  }
}
const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
})
function showKline(row) {
  console.log('showKline', row)
  modalDataRef.title = row.SECURITY_NAME_ABBR
  modalDataRef.stockCode = getStockCode(row.SECUCODE)
  modalDataRef.stockName = row.SECURITY_NAME_ABBR
  modalDataRef.visible = true
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}
const technicalIndicatorReactive = reactive({
  MACD_GOLDEN_FORK: false,
  KDJ_GOLDEN_FORK: false,
  BREAK_THROUGH: false,
  LOW_FUNDS_INFLOW: false,
  HIGH_FUNDS_OUTFLOW: false,
  BREAKUP_MA_5DAYS: false,
  LONG_AVG_ARRAY: false,
  SHORT_AVG_ARRAY: false,
  UPPER_LARGE_VOLUME: false,
  DOWN_NARROW_VOLUME: false,
  ONE_DAYANG_LINE: false,
  TWO_DAYANG_LINES: false,
  RISE_SUN: false,
  POWER_FULGUN: false,
  RESTORE_JUSTICE: false,
  DOWN_7DAYS: false,
  UPPER_8DAYS:false,
  UPPER_9DAYS:false,
  UPPER_4DAYS:false,
  HEAVEN_RULE:false,
  UPSIDE_VOLUME: false,
  BEARISH_ENGULFING: false,
  REVERSING_HAMMER: false,
  SHOOTING_STAR: false,
  EVENING_STAR: false,
  FIRST_DAWN: false,
  PREGNANT: false,
  BLACK_CLOUD_TOPS: false,
  MORNING_STAR: false,
  NARROW_FINISH: false,
  UPP_DAYS:0,
  CONCERN_RANK_7DAYS:0,
  UPNDAY:0,
  DOWNNDAY :0,
})

function handlePageChange(currentPage) {
  loadStocks(currentPage, paginationReactive.pageSize)
}

// 判断是否是数字
const isNumeric = (value) => {
  if (value === null || value === undefined || value === '') {
    return false
  }
  return !isNaN(Number(value))
}

// 安全转换数字
const toNumber = (value, defaultValue = 0) => {
  const num = Number(value)
  return isNaN(num) ? defaultValue : num
}

</script>

<template>
    <n-space justify="start">
      <n-card size="small" :bordered="false"  style="text-align: left">
        <div style="display: flex; align-items: center; margin-bottom: 2px;">
          <n-text depth="3" style="font-size: 12px;">形态筛选条件</n-text>
          <n-popover trigger="hover" style="max-width: 420px;">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" size="14" style="margin-left: 4px; cursor: help; opacity: 0.6;"/>
            </template>
            <div style="font-size: 12px; line-height: 1.7;">
              <b>形态选股 vs 技术分析指标的区别：</b><br/>
              本页的选项是<b>筛选条件（布尔信号）</b>——回答「全市场哪些股票此刻出现了这个信号」，用于从几千只股票中过滤出候选；<br/>
              K线分析页的技术指标（MA/MACD/KDJ/BOLL/RSI 等）是<b>连续分析工具</b>——画在单只股票的K线图上，回答「这只股票当前处于什么状态」。<br/>
              两者关系：MACD金叉、均线排列是把指标「信号化」后的筛选条件；乌云盖顶、早晨之星等是对K线价格本身的模式识别（不属于指标）；人气排名、资金流属于情绪面/资金面数据。
            </div>
          </n-popover>
        </div>
        <div v-for="group in filterGroups" :key="group.name" style="margin-bottom: 2px;">
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-text depth="3" style="font-size: 12px; margin-right: 2px; cursor: help; border-bottom: 1px dashed #888;">{{ group.name }}</n-text>
            </template>
            <span>{{ group.desc }}</span>
          </n-tooltip>
          <n-tooltip v-for="opt in group.items" :key="opt.key" trigger="hover">
            <template #trigger>
              <n-checkbox size="small" v-model:checked="technicalIndicatorReactive[opt.key]">{{ opt.label }}</n-checkbox>
            </template>
            <span style="display: inline-block; max-width: 260px;">{{ opt.tip }}</span>
          </n-tooltip>
        </div>
      </n-card>
      <n-card size="small" :bordered="false"  style="text-align: left">
        <n-space align="center" size="small" wrap>
          <template v-for="group in rankGroups" :key="group.key">
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-text depth="3" style="font-size: 12px; cursor: help; border-bottom: 1px dashed #888;">{{ group.name }}</n-text>
              </template>
              <span>{{ group.tip }}</span>
            </n-tooltip>
            <n-radio-group size="small" :name="group.key" v-model:value="technicalIndicatorReactive[group.key]">
              <n-radio v-for="v in group.options" :key="v" :value="v">
                {{ group.key === 'CONCERN_RANK_7DAYS' ? `前${v}名` : `${v}天及以上` }}
              </n-radio>
            </n-radio-group>
            <n-divider vertical/>
          </template>
        </n-space>
      </n-card>

    </n-space>
    <n-input-group>
<!--    <n-input clearable placeholder="输入股票名称" v-model:value="paginationReactive.keyword"/>-->
      <n-auto-complete
          v-model:value="paginationReactive.keyword"
          :input-props="{
            autocomplete: 'disabled',
          }"
          :options="optionsReactive"
          placeholder="输入搜索关键词"
          clearable
          @input="handleUpdateVal"
          @select="(value) => {
            paginationReactive.keyword = value
            handleSearch()
          }"
      />
    <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
      搜索
    </n-button>
      <n-button @click="handleReset">重置</n-button>

    </n-input-group>
    <!-- 数据表格 -->
    <n-data-table
      remote
      size="small"
      :columns="columnsRef"
      :data="dataRef"
      :loading="loadingRef"
      :pagination="paginationReactive"
      :row-key="(rowData) => rowData.SECUCODE"
      flex-height
      style="height: calc(100vh - 380px);margin-top: 10px"
      @update:page="handlePageChange"
    />
    
    <!-- 分页控件 -->
<!--    <div style="margin-top: 16px; display: flex; justify-content: center;">-->
<!--      <n-pagination-->
<!--        v-model:page="paginationReactive.page"-->
<!--        v-model:page-size="paginationReactive.pageSize"-->
<!--        :page-count="paginationReactive.pageCount"-->
<!--        :item-count="paginationReactive.itemCount"-->
<!--        :page-sizes="[10, 20, 50, 100]"-->
<!--        show-size-picker-->
<!--        show-quick-jumper-->
<!--        @update:page="handlePageChange"-->
<!--        @update:page-size="handlePageSizeChange"-->
<!--      />-->
<!--    </div>-->

  <n-modal v-model:show="modalDataRef.visible" :title="modalDataRef.title" preset="card" style="width: 850px;max-width: calc(100vw - 32px);">
    <n-card size="small">
      <KLineChart style="width: 100%;max-width: 800px;" :code="getStockCode(modalDataRef.stockCode)" :chart-height="500" :stock-name="modalDataRef.stockName" :k-days="30" :dark-theme="editorDataRef.darkTheme"></KLineChart>
    </n-card>
  </n-modal>
</template>

<style scoped>
</style>
