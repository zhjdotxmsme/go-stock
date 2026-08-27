<script setup>
import {computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch} from 'vue'
import api from '../api'
import * as stockApi from '../api/stock'
import * as systemApi from '../api/system'
import {
  OpenURL,
} from '../../wailsjs/go/handler/SystemHandler'
import {
  NFlex,
  NForm,
  NFormItem,
  NInputNumber,
  NText,
  useDialog,
  useMessage,
  useNotification
} from 'naive-ui'
import {
  Environment,
  EventsEmit,
  EventsOff,
  WindowFullscreen,
  WindowUnfullscreen
} from '../../wailsjs/runtime'
import {Add,} from '@vicons/ionicons5'

import {keys, padStart} from "lodash";
import {useRoute, useRouter} from 'vue-router'
import MoneyTrend from "./moneyTrend.vue";
import StockNews from "./StockNews.vue";
import StockSparkLine from "./stockSparkLine.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import StockCostModal from "./stock/StockCostModal.vue";
import StockAiModal from "./stock/StockAiModal.vue";
import { useDraggableTabs } from './stock/useDraggableTabs'
import { useFenshiChart } from './stock/useFenshiChart'
import { useEchartsKline } from './stock/echartsKline'
import { useExportReport } from './stock/exportReport'
import { useAiCheck } from './stock/useAiCheck'
import { useGroupTabs } from './stock/useGroupTabs'
import { useStockEvents } from './stock/useStockEvents'

const route = useRoute()
const router = useRouter()

const dialog = useDialog()

const kLineChartRef = ref(null);
const kLineChartRef2 = ref(null);


const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const aiResultScrollRef = ref(null)
const tipsRef = ref(null)
const message = useMessage()
const notify = useNotification()
const stocks = ref([])
const results = ref({})
const stockList = ref([])
const followList = ref([])
const groupList = ref([])
const options = ref([])
const modalShow = ref(false)
const modalShow2 = ref(false)
const modalShow3 = ref(false)
const modalShow4 = ref(false)
const modalShow5 = ref(false)
const modalShow6 = ref(false)
const modalShowNews = ref(false)
const newsCode = ref('')
const newsName = ref('')

function showNews(code, name) {
  newsCode.value = code
  newsName.value = name
  modalShowNews.value = true
}

const lwKlineCode = ref('')
const lwKlineName = ref('')
const currentStockTradingPrice = ref({
  stockCode: '',
  costPrice: 0,
  entryPrice: 0,
  takeProfitPrice: 0,
  stopLossPrice: 0,
})
const klineAutoCloseTimer = ref(null)
const addBTN = ref(true)
const enableTools = ref(true)
const thinkingMode = ref(true)
const formModel = ref({
  name: "",
  code: "",
  costPrice: 0.000,
  volume: 0,
  alarm: 0,
  alarmPrice: 0,
  sort: 999,
  cron: "",
  entryPrice: 0,
  takeProfitPrice: 0,
  stopLossPrice: 0,
})

const promptTemplates = ref([])
const aiConfigs = ref([])
const sysPromptOptions = ref([])
const userPromptOptions = ref([])
const strategyCode = ref('')
const strategies = ref([])
const data = reactive({
  modelName: "",
  chatId: "",
  question: "",
  sysPromptId: null,
  aiConfigId: null,
  name: "",
  code: "",
  fenshiURL: "",
  kURL: "",
  resultText: "Please enter your name below 👇",
  fullscreen: false,
  airesult: "",
  openAiEnable: false,
  loading: true,
  analysisStatus: "",
  darkTheme: false,
  changePercent: 0
})
const feishiInterval = ref(null)
const aiAnalysisTimeout = ref(null)

// Multi-agent streaming state
const multiAgentState = reactive({
  active: false,
  currentPhase: '',
  phaseLabel: '',
  phases: {},
  reports: {},
  ratings: {},
  debates: [],
  finalReport: null,
  done: false,
})


const currentGroupId = ref(0)


const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');

const sortedResults = computed(() => {
  const sortedKeys = keys(results.value).sort();
  const sortedObject = {};
  sortedKeys.forEach(key => {
    sortedObject[key] = results.value[key];
  });
  return sortedObject
});

const groupResults = computed(() => {
  const group = {}
  if (currentGroupId.value === 0) {
    return sortedResults.value
  } else {
    for (const key in sortedResults.value) {
      if (stocks.value.includes(sortedResults.value[key]['股票代码'])) {
        group[key] = sortedResults.value[key]
      }
    }
    return group
  }
})
const showPopover = ref(false)

// ---- 分时图 / echarts K线 / 导出分享 / AI检查 / 分组标签 / 拖拽（components/stock/） ----
const { clearFeishi, showFsChart, showFenshi, handleFeishi } = useFenshiChart({
  kLineChartRef2, data, modalShow2, feishiInterval,
})
const { handleKLine } = useEchartsKline({ kLineChartRef, data })
const {
  saveAsImage, copyToClipboard, agentTitle, buildFullReport, scrollToAiResultBottom,
  saveAsMarkdown, saveAsMarkdown_old, getHtml, saveAsWord, share,
} = useExportReport({
  mdPreviewRef, mdEditorRef, aiResultScrollRef, tipsRef, message, notify, icon,
})
const { aiReCheckStock, aiCheckStock } = useAiCheck({
  message, modalShow4, enableTools, thinkingMode, strategyCode, data, aiAnalysisTimeout,
})
const { cleanupDraggableTabs, initDraggableTabs } = useDraggableTabs({
  stockApi, message, groupList,
})

// ---- 分组标签（components/stock/useGroupTabs） ----
const {
  addTabModel, addTabPane, addTab, saveTabPane, AddStockGroupInfo,
  updateTab, delTab, delStockGroup, searchNotice, searchStockReport,
} = useGroupTabs({
  router, dialog, message, stocks, followList, groupList, modalShow6,
  klineAutoCloseTimer, data, currentGroupId, updateData,
})

// ---- 事件编排（components/stock/useStockEvents，内部注册 onBeforeMount） ----
useStockEvents({
  route, message, notify, stockList, groupList, options, addBTN,
  promptTemplates, aiConfigs, sysPromptOptions, userPromptOptions, strategies,
  aiAnalysisTimeout, multiAgentState, currentGroupId, icon, data,
  fetchGroupList, updateData, buildFullReport, scrollToAiResultBottom, updateTab,
})
// 监听分组列表变化，重新初始化拖拽
const unwatch = watch(groupList, () => {
  nextTick(() => {
    initDraggableTabs();
  });
});

// 在组件卸载时清理监听器
onBeforeUnmount(() => {
  unwatch();
});
onMounted(() => {
  nextTick(() => {
    initDraggableTabs();
  });

  message.loading("Loading...")
  stockApi.getFollowList(currentGroupId.value).then(({data: result}) => {

    followList.value = result
    for (const followedStock of result) {
      if (followedStock.StockCode.startsWith("us")) {
        followedStock.StockCode = "gb_" + followedStock.StockCode.replace("us", "").toLowerCase()
      }
      if (!stocks.value.includes(followedStock.StockCode)) {
        stocks.value.push(followedStock.StockCode)
      }
      stockApi.greet(followedStock.StockCode).then(({data: greetResult}) => {
        updateData(greetResult)
      })
    }
    //monitor()
    message.destroyAll()
  })

  systemApi.getVersionInfo().then(({data: res}) => {
    icon.value = res.icon
  })
})

onBeforeUnmount(() => {
  // //console.log(`the component is now unmounted.`)
  //clearInterval(ticker.value)
  message.destroyAll()
  notify.destroyAll()
  clearInterval(feishiInterval.value)
  // 清理 AI 分析超时定时器
  if (aiAnalysisTimeout.value) {
    clearTimeout(aiAnalysisTimeout.value)
    aiAnalysisTimeout.value = null
  }
  // 清理多周期 K 线自动关闭定时器
  if (klineAutoCloseTimer.value) {
    clearTimeout(klineAutoCloseTimer.value)
    klineAutoCloseTimer.value = null
  }

  EventsOff("refresh")
  EventsOff("showSearch")
  EventsOff("stock_price")
  EventsOff("refreshFollowList")
  EventsOff("newChatStream")
  EventsOff("changeTab")
  EventsOff("updateVersion")
  EventsOff("updateNeedAdmin")
  EventsOff("warnMsg")
  EventsOff("loadingDone")

  cleanupDraggableTabs()

})

//判断是否是A股交易时间
function isTradingTime() {
  const now = new Date();
  const day = now.getDay(); // 获取星期几，0表示周日，1-6表示周一至周六
  if (day >= 1 && day <= 5) { // 周一至周五
    const hours = now.getHours();
    const minutes = now.getMinutes();
    const totalMinutes = hours * 60 + minutes;
    const startMorning = 9 * 60 + 15; // 上午9点15分换算成分钟数
    const endMorning = 11 * 60 + 30; // 上午11点30分换算成分钟数
    const startAfternoon = 13 * 60; // 下午13点换算成分钟数
    const endAfternoon = 15 * 60; // 下午15点换算成分钟数
    if ((totalMinutes >= startMorning && totalMinutes < endMorning) ||
        (totalMinutes >= startAfternoon && totalMinutes < endAfternoon)) {
      return true;
    }
  }
  return false;
}

// 添加一个获取分组列表的函数，用于处理初始化逻辑
function fetchGroupList() {
  stockApi.initializeGroupSort().then(({data: initResult}) => {
    if (initResult) {
      stockApi.getGroupList().then(({data: result}) => {
        groupList.value = result
        if (route.query.groupId) {
          message.success("切换分组:" + route.query.groupName)
          currentGroupId.value = Number(route.query.groupId)
        }
      })
    } else {
      message.error("初始化分组序号失败")
    }
  })
}

function AddStock() {
  if (!data?.code) {
    message.error("请输入有效股票代码");
    return;
  }
  if (!stocks.value.includes(data.code)) {
    stockApi.follow(data.code).then(({data: result}) => {
      if (result === "关注成功") {
        if (data.code.startsWith("us")) {
          data.code = "gb_" + data.code.replace("us", "").toLowerCase()
        }
        stocks.value.push(data.code)
        message.success(result)
        stockApi.getFollowList(currentGroupId.value).then(({data}) => {
          followList.value = data
        })
        monitor();
      } else {
        message.error(result)
      }
    })
  } else {
    message.error("已经关注了")
  }
}


function removeMonitor(code, name, key) {
  //console.log("removeMonitor",name,code,key)
  stocks.value.splice(stocks.value.indexOf(code), 1)
  //console.log("removeMonitor-key",key)
  //console.log("removeMonitor-v",results.value[key])

  delete results.value[key]
  //console.log("removeMonitor-v",results.value[key])

  stockApi.unFollow(code).then(({data: result}) => {
    message.success(result)
    monitor()
  })
}


function getStockList(value) {


  // //console.log("getStockList",value)
  let result;
  result = stockList.value.filter(item => item.name.includes(value) || item.ts_code.includes(value))
  options.value = result.map(item => {
    return {
      label: item.name + " - " + item.ts_code,
      value: item.ts_code
    }
  })
  if (value && value.indexOf("-") <= 0) {
    data.code = value
  }

  //console.log("getStockList-options",data.code)

  if (data.code) {
    let findId = data.code
    if (findId.startsWith("us")) {
      findId = "gb_" + findId.replace("us", "").toLowerCase()
    }
    blinkBorder(findId)
  }


}

function blinkBorder(findId) {
  // 获取要滚动到的元素
  let element = document.getElementById(findId);
  //console.log("blinkBorder",findId,element)
  if (element) {
    // 滚动到该元素
    element.scrollIntoView({behavior: 'smooth'});
    const pelement = document.getElementById(findId + '_gi');
    if (pelement) {
      // 添加闪烁效果
      pelement.classList.add('blink-border');
      // 3秒后移除闪烁效果
      setTimeout(() => {
        pelement.classList.remove('blink-border');
      }, 1000 * 5);
    } else {
      console.error(`Element with ID ${findId}_gi not found`);
    }
  }
}

async function updateData(result) {
  ////console.log("stock_price",result['日期'],result['时间'],result['股票代码'],result['股票名称'],result['当前价格'],result['盘前盘后'])

  if (result["当前价格"] <= 0) {
    result["当前价格"] = result["卖一报价"]
  }

  if (result.changePercent > 0) {
    result.type = "error"
    result.color = "#E88080"
  } else if (result.changePercent < 0) {
    result.type = "success"
    result.color = "#63E2B7"
  } else {
    result.type = "default"
    result.color = "#FFFFFF"
  }

  if (result.profitAmount > 0) {
    result.profitType = "error"
  } else if (result.profitAmount < 0) {
    result.profitType = "success"
  }
  if (result["当前价格"]) {
    // if (result.alarmChangePercent > 0 && Math.abs(result.changePercent) >= result.alarmChangePercent) {
    //   SendMessage(result, 1)
    // }

    // if (result.alarmPrice > 0 && result["当前价格"] >= result.alarmPrice) {
    //   SendMessage(result, 2)
    // }

    // if (result.costPrice > 0 && result["当前价格"] >= result.costPrice) {
    //   SendMessage(result, 3)
    // }

    checkPriceLineAlerts(result)
  }

  // result.key=result.sort
  results.value = Object.fromEntries(
      Object.entries(results.value).filter(
          ([key]) => !key.includes(result["股票代码"])
      ));

  result.key = GetSortKey(result.sort, result["股票代码"])
  results.value[result.key] = result
  if (!stocks.value.includes(result["股票代码"])) {
    delete results.value[result.key]
  }
}


async function monitor() {
  if (stocks.value && stocks.value.length === 0) {
    showPopover.value = true
  }
  for (let code of stocks.value) {
    stockApi.greet(code).then(({data: result}) => {
      updateData(result)
    })
  }
}


function GetSortKey(sort, code) {
  return padStart(sort, 8, '0') + "_" + code
}

function onSelect(item) {
  ////console.log("onSelect",item)

  if (item.indexOf("-") > 0) {
    item = item.split("-")[1].toLowerCase()
  }
  if (item.indexOf(".") > 0) {
    data.code = item.split(".")[1].toLowerCase() + item.split(".")[0]
  }

}

function openCenteredWindow(url, width, height) {
  const left = (window.screen.width - width) / 2;
  const top = (window.screen.height - height) / 2;
  Environment().then(env => {
    switch (env.platform) {
      case 'windows':
        window.open(
            url,
            'centeredWindow',
            `width=${width},height=${height},left=${left},top=${top},location=no,menubar=no,toolbar=no,display=standalone`
        )
        break
      default :
        OpenURL(url)
        break
    }
  })


  //
  // return window.open(
  //     url,
  //     'centeredWindow',
  //     `width=${width},height=${height},left=${left},top=${top}`
  // );
}

function search(code, name) {
  setTimeout(() => {
    //window.open("https://xueqiu.com/S/"+code)
    //window.open("https://www.cls.cn/stock?code="+code)
    //window.open("https://quote.eastmoney.com/"+code+".html")
    //window.open("https://finance.sina.com.cn/realstock/company/"+code+"/nc.shtml")
    //window.open("https://www.iwencai.com/unifiedwap/result?w=" + name)
    //window.open("https://www.iwencai.com/chat/?question="+code)

    openCenteredWindow("https://www.iwencai.com/unifiedwap/result?w=" + name, 1000, 800)

  }, 500)
}

function handleLongEntryPriceUpdate(newPrice) {
  console.log('[DEBUG handleLongEntryPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.entryPrice = newPrice
  console.log('[DEBUG handleLongEntryPriceUpdate] after assignment, entryPrice:', currentStockTradingPrice.value.entryPrice)
  saveTradingPriceToBackend()
}

function handleLongStopLossPriceUpdate(newPrice) {
  console.log('[DEBUG handleLongStopLossPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.stopLossPrice = newPrice
  saveTradingPriceToBackend()
}

function handleLongTakeProfitPriceUpdate(newPrice) {
  console.log('[DEBUG handleLongTakeProfitPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.takeProfitPrice = newPrice
  saveTradingPriceToBackend()
}

function handleCostPriceUpdate(newPrice) {
  console.log('[DEBUG handleCostPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.costPrice = newPrice
  saveTradingPriceToBackend()
}

function saveTradingPriceToBackend() {
  console.log('[DEBUG saveTradingPriceToBackend] called, stockCode:', currentStockTradingPrice.value.stockCode)
  if (!currentStockTradingPrice.value.stockCode) {
    console.log('[DEBUG saveTradingPriceToBackend] early return - no stockCode')
    return
  }
  const emCode = currentStockTradingPrice.value.stockCode
  const code = fromEastMoneyCode(emCode)
  if (!code) {
    console.warn('[saveTradingPriceToBackend] 无法转换股票代码:', emCode)
    return
  }
  const entryPrice = Number(currentStockTradingPrice.value.entryPrice) || 0
  const takeProfitPrice = Number(currentStockTradingPrice.value.takeProfitPrice) || 0
  const stopLossPrice = Number(currentStockTradingPrice.value.stopLossPrice) || 0
  const costPrice = Number(currentStockTradingPrice.value.costPrice) || 0
  console.log('[DEBUG saveTradingPriceToBackend] calling SetTradingPrice with:', code, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
  stockApi.setTradingPrice(
    code,
    entryPrice,
    takeProfitPrice,
    stopLossPrice,
    costPrice
  ).then(({data: result}) => {
    console.log('[DEBUG saveTradingPriceToBackend] SetTradingPrice result:', result)
    if (result === '设置成功') {
      const emCode = currentStockTradingPrice.value.stockCode
      const internalCode = code
      const followItem = followList.value.find(item => item.StockCode === internalCode || item.StockCode === emCode)
      if (followItem) {
        followItem.EntryPrice = entryPrice
        followItem.TakeProfitPrice = takeProfitPrice
        followItem.StopLossPrice = stopLossPrice
        console.log('[DEBUG saveTradingPriceToBackend] updated followList item')
      }
    }
  }).catch(err => {
    console.error('[DEBUG saveTradingPriceToBackend] SetTradingPrice error:', err)
  })
}

function setStock(code, name) {
  let res = followList.value.filter(item => item.StockCode === code)
  ////console.log("res:",res)
  formModel.value.name = name
  formModel.value.code = code
  formModel.value.volume = res[0].Volume ? res[0].Volume : 0
  formModel.value.costPrice = res[0].CostPrice
  formModel.value.alarm = res[0].AlarmChangePercent
  formModel.value.alarmPrice = res[0].AlarmPrice
  formModel.value.sort = res[0].Sort
  formModel.value.cron = res[0].Cron
  formModel.value.entryPrice = res[0].EntryPrice || 0
  formModel.value.takeProfitPrice = res[0].TakeProfitPrice || 0
  formModel.value.stopLossPrice = res[0].StopLossPrice || 0
  modalShow.value = true
}



function showMoney(code, name) {
  data.code = code
  data.name = name
  modalShow5.value = true
}

/** 新浪/应用内代码转为东方财富接口常用格式（如 600519.SH） */
function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  if (lower.startsWith('gb_')) return lower.slice(3).toUpperCase() + '.US'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return c + '.SH'
    if (d === '0' || d === '3') return c + '.SZ'
    if (d === '8' || d === '9') return c + '.BJ'
    return c + '.SZ'
  }
  // 纯字母代码视为美股（如 AAPL → AAPL.US）
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

/** 东方财富格式转回应用内部代码格式（如 000001.SZ → sh000001） */
function fromEastMoneyCode(emCode) {
  if (!emCode) return ''
  const c = String(emCode).trim().toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3)
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3)
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3)
  if (c.endsWith('.HK')) return 'hk' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.US')) return 'us' + c.slice(0, -3).toLowerCase()
  return c.toLowerCase()
}

function goKlineAnalysis(code, name) {
  const em = toEastMoneyCode(code)
  if (!em) {
    message.warning('当前代码暂不支持技术分析')
    return
  }
  router.push({ path: '/kline-analysis', query: { code: em, name: name || '' } })
}

async function showLightweightKline(code, name) {
  const em = toEastMoneyCode(code)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  lwKlineCode.value = em
  lwKlineName.value = name || ''

  // 刷新自选列表，确保获取最新的交易价格数据
  try {
    const {data: list} = await stockApi.getFollowList(currentGroupId.value)
    followList.value = list || []
  } catch (e) {
    console.error('[showLightweightKline] 刷新自选列表失败:', e)
  }

  // 从自选列表中获取交易价格
  // lwKlineCode 格式为 000001.SZ，followList 中的 StockCode 格式为 sh000001
  // 需要进行格式转换来匹配
  let followListCode = code
  if (code.startsWith('sh') || code.startsWith('sz') || code.startsWith('bj') || code.startsWith('hk')) {
    // 如果是 sh000001 格式，转换为东方财富格式
    const market = code.slice(0, 2).toUpperCase()
    const stockNum = code.slice(2)
    followListCode = stockNum + '.' + market
  }

  const stockInfo = followList.value.find(item => item.StockCode === code || item.StockCode === followListCode)
  if (stockInfo) {
    currentStockTradingPrice.value.stockCode = lwKlineCode.value  // 使用东方财富格式
    currentStockTradingPrice.value.costPrice = stockInfo.CostPrice || 0
    currentStockTradingPrice.value.entryPrice = stockInfo.EntryPrice || 0
    currentStockTradingPrice.value.takeProfitPrice = stockInfo.TakeProfitPrice || 0
    currentStockTradingPrice.value.stopLossPrice = stockInfo.StopLossPrice || 0
  } else {
    currentStockTradingPrice.value.stockCode = lwKlineCode.value
    currentStockTradingPrice.value.costPrice = 0
    currentStockTradingPrice.value.entryPrice = 0
    currentStockTradingPrice.value.takeProfitPrice = 0
    currentStockTradingPrice.value.stopLossPrice = 0
  }

  modalShow6.value = true
}

function showK(code, name) {
  data.code = code
  data.name = name
  data.kURL = 'http://image.sinajs.cn/newchart/daily/n/' + data.code + '.gif' + "?t=" + Date.now()
  if (code.startsWith('hk')) {
    data.kURL = 'http://image.sinajs.cn/newchart/hk_stock/daily/' + data.code.replace("hk", "") + '.gif' + "?t=" + Date.now()
  }
  if (code.startsWith('gb_')) {
    data.kURL = 'http://image.sinajs.cn/newchart/usstock/daily/' + data.code.replace("gb_", "") + '.gif' + "?t=" + Date.now()
  }
  modalShow3.value = true
  //https://image.sinajs.cn/newchart/usstock/daily/dji.gif
  //https://image.sinajs.cn/newchart/hk_stock/daily/06030.gif?1740729404273
}


function updateCostPriceAndVolumeNew(code, price, volume, alarm, formModel) {
  if (formModel.sort) {
    stockApi.setStockSort(formModel.sort, code).then(({data: result}) => {
      //message.success(result)
    })
  }
  if (formModel.cron) {
    stockApi.setStockAICron(formModel.cron, code).then(({data: result}) => {
      //message.success(result)
    })
  }

  if (alarm || formModel.alarmPrice) {
    stockApi.setAlarmChangePercent(alarm, formModel.alarmPrice, code).then(({data: result}) => {
      //message.success(result)
    })
  }

  // 保存交易价格（开仓价、止盈价、止损价、成本价）
  if (formModel.entryPrice || formModel.takeProfitPrice || formModel.stopLossPrice || formModel.costPrice) {
    stockApi.setTradingPrice(code, formModel.entryPrice || 0, formModel.takeProfitPrice || 0, formModel.stopLossPrice || 0, formModel.costPrice || 0).then(({data: result}) => {
      //message.success(result)
    })
  }

  stockApi.setCostPriceAndVolume(code, price, volume).then(({data: result}) => {
    modalShow.value = false
    message.success(result)
    stockApi.getFollowList(currentGroupId.value).then(({data: result}) => {
      followList.value = result
      stocks.value = []
      for (const followedStock of result) {
        if (!stocks.value.includes(followedStock.StockCode)) {
          stocks.value.push(followedStock.StockCode)
        }
      }
      monitor()
      message.destroyAll()
    })
  })
}

function fullscreen() {
  if (data.fullscreen) {
    WindowUnfullscreen()
  } else {
    WindowFullscreen()
  }
  data.fullscreen = !data.fullscreen
}


//type 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
function SendMessage(result, type) {
  let typeName = getTypeName(type)
  let img = 'http://image.sinajs.cn/newchart/min/n/' + result["股票代码"] + '.gif' + "?t=" + Date.now()
  let markdown = "### go-stock [" + typeName + "]\n\n" +
      "### " + result["股票名称"] + "(" + result["股票代码"] + ")\n" +
      "- 当前价格: " + result["当前价格"] + "  " + result.changePercent + "%\n" +
      "- 最高价: " + result["今日最高价"] + "  " + result.highRate + "\n" +
      "- 最低价: " + result["今日最低价"] + "  " + result.lowRate + "\n" +
      "- 昨收价: " + result["昨日收盘价"] + "\n" +
      "- 今开价: " + result["今日开盘价"] + "\n" +
      "- 成本价: " + result.costPrice + "  " + result.profit + "%  " + result.profitAmount + " ¥\n" +
      "- 成本数量: " + result.costVolume + "股\n" +
      "- 日期: " + result["日期"] + "  " + result["时间"] + "\n\n" +
      "![image](" + img + ")\n"
  let title = result["股票名称"] + "(" + result["股票代码"] + ") " + result["当前价格"] + " " + result.changePercent

  let msg = '{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"[' + typeName + "]" + title + '",' +
      '         "text": "' + markdown + '"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'
  // SendDingDingMessage(msg,result["股票代码"])
  stockApi.sendDingDingMessageByType(msg, result["股票代码"], type)
}

const priceLineAlertCache = new Map()

function checkPriceLineAlerts(result) {
  const code = result["股票代码"]
  const price = result["当前价格"]
  if (!price || price <= 0) return

  const followedStock = followList.value.find(s => {
    const sCode = s.StockCode || ''
    return sCode === code || sCode === 'sh' + code || sCode === 'sz' + code ||
           sCode === code.replace('sh', '').replace('sz', '') ||
           (sCode.length > 2 && code.length > 2 && sCode.includes(code.slice(2)))
  })

  if (!followedStock) return

  const alerts = []
  let triggeredType = 0
  if (followedStock.EntryPrice > 0) {
    const diff = ((price - followedStock.EntryPrice) / followedStock.EntryPrice * 100).toFixed(2)
    alerts.push(`开仓价: ${followedStock.EntryPrice} (${diff >= 0 ? '+' : ''}${diff}%)`)
  }
  if (followedStock.TakeProfitPrice > 0) {
    if (price >= followedStock.TakeProfitPrice) {
      alerts.push(`止盈价: ${followedStock.TakeProfitPrice} ⚠️ 已触及`)
      triggeredType = 4
    } else {
      const diff = ((followedStock.TakeProfitPrice - price) / followedStock.TakeProfitPrice * 100).toFixed(2)
      alerts.push(`止盈价: ${followedStock.TakeProfitPrice} (距离 ${diff}%)`)
    }
  }
  if (followedStock.StopLossPrice > 0) {
    if (price <= followedStock.StopLossPrice) {
      alerts.push(`止损价: ${followedStock.StopLossPrice} ⚠️ 已触及`)
      triggeredType = 5
    } else {
      const diff = ((price - followedStock.StopLossPrice) / followedStock.StopLossPrice * 100).toFixed(2)
      alerts.push(`止损价: ${followedStock.StopLossPrice} (+${diff}%)`)
    }
  }

  if (alerts.length === 0) return

  const cacheKey = `${code}_${price}`
  if (priceLineAlertCache.get(cacheKey)) return

  const notifyKey = `${code}_notify`
  const lastNotify = priceLineAlertCache.get(notifyKey) || 0
  const now = Date.now()
  if (now - lastNotify < 60000) return

  priceLineAlertCache.set(cacheKey, true)
  priceLineAlertCache.set(notifyKey, now)

  const stockName = followedStock.Name || followedStock.StockName || result["股票名称"] || code
  const stockCodeDisplay = code.length > 6 ? code : code.toUpperCase()

  // notify.info({
  //   avatar: () => h(NAvatar, { size: 'small', round: false, src: icon.value }),
  //   title: `📈 ${stockName} (${stockCodeDisplay})`,
  //   duration: 5000,
  //   meta: `当前价: ${price}`,
  //   content: () => h('div', { style: { 'text-align': 'left', 'font-size': '13px' } },
  //     alerts.map(a => h('div', { style: { 'margin-bottom': '4px' } }, a))
  //   ),
  // })

  if (triggeredType > 0) {
    const msg = `### 📈 价位线预警\n\n### ${stockName} (${stockCodeDisplay})\n\n- 当前价格: ${price}\n- 预警类型: ${triggeredType === 4 ? '止盈触及' : '止损触及'}\n- 开仓价: ${followedStock.EntryPrice || '-'}\n- 止盈价: ${followedStock.TakeProfitPrice || '-'}\n- 止损价: ${followedStock.StopLossPrice || '-'}`;
    stockApi.sendDingDingMessageByType(msg, code, triggeredType)
  }
}


function getTypeName(type) {
  switch (type) {
    case 1:
      return "涨跌报警"
    case 2:
      return "股价报警"
    case 3:
      return "成本价报警"
    default:
      return ""
  }
}

//获取高度
function getHeight() {
  return document.documentElement.clientHeight
}

window.onerror = function (msg, source, lineno, colno, error) {
  // 将错误信息发送给后端
  EventsEmit("frontendError", {
    page: "stock.vue",
    message: msg,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null,
    data: data,
    results: results,
    followList: followList,
    stockList: stockList,
    stocks: stocks,
    formModel: formModel,
  });
  message.error("发生错误:" + msg)
  return true;
};


</script>

<template>
  <n-tabs type="card" style="--wails-draggable:no-drag" animated addable :data-currentGroupId="currentGroupId"
          :value="String(currentGroupId)" @add="addTab" @update:value="updateTab" placement="top" @close="(key)=>{delTab(key)}">

    <n-tab-pane closable name="0" :tab="'全部'">
      <n-grid :x-gap="8" :cols="3" :y-gap="8">
        <n-gi :id="result['股票代码']+'_gi'" v-for="result in sortedResults" style="margin-left: 2px;">
          <n-card :data-sort="result.sort" :id="result['股票代码']" :data-code="result['股票代码']" :bordered="true"
                  :title="result['股票名称']" :closable="false"
                  @close="removeMonitor(result['股票代码'],result['股票名称'],result.key)">
            <n-grid :cols="1" :y-gap="6">
              <n-gi>
                <n-text :type="result.type">
                  <n-number-animation :duration="1000" :precision="2" :from="result['上次当前价格']"
                                      :to="Number(result['当前价格'])"/>
                  <n-tag size="small" :type="result.type" :bordered="false" v-if="result['盘前盘后']>0">
                    ({{ result['盘前盘后'] }} {{ result['盘前盘后涨跌幅'] }}%)
                  </n-tag>
                </n-text>
                <n-text style="padding-left: 10px;" :type="result.type">
                  <n-number-animation :duration="1000" :precision="3" :from="0" :to="result.changePercent"/>
                  %
                </n-text>&nbsp;
                <n-text size="small" v-if="result.costVolume>0" :type="result.type">
                  <n-number-animation :duration="1000" :precision="2" :from="0" :to="result.profitAmountToday"/>
                </n-text>
              </n-gi>
            </n-grid>
            <n-grid :cols="2" :y-gap="4" :x-gap="4">
              <n-gi>
                <n-text :type="'info'">{{ "最高 " + result["今日最高价"] + " " + result.highRate }}%</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "最低 " + result["今日最低价"] + " " + result.lowRate }}%</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "昨收 " + result["昨日收盘价"] }}</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "今开 " + result["今日开盘价"] }}</n-text>
              </n-gi>
            </n-grid>
            <n-collapse accordion v-if="result['买一报价']>0">
              <n-collapse-item title="盘口" name="1" v-if="result['买一报价']>0">
                <template #header-extra>
                  <n-flex justify="space-between">
                    <n-text :type="'info'">{{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}</n-text>
                    <n-text :type="'info'">{{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}</n-text>
                  </n-flex>
                </template>
                <n-grid :cols="2" :y-gap="4" :x-gap="4">
                  <n-gi v-if="result['买一报价']>0">
                    <n-text :type="'info'">{{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖一报价']>0">
                    <n-text :type="'info'">{{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买二报价']>0">
                    <n-text :type="'info'">{{ "买二 " + result["买二报价"] + '(' + result["买二申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖二报价']>0">
                    <n-text :type="'info'">{{ "卖二 " + result["卖二报价"] + '(' + result["卖二申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买三报价']>0">
                    <n-text :type="'info'">{{ "买三 " + result["买三报价"] + '(' + result["买三申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖三报价']>0">
                    <n-text :type="'info'">{{ "买三 " + result["卖三报价"] + '(' + result["卖三申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买四报价']>0">
                    <n-text :type="'info'">{{ "买四 " + result["买四报价"] + '(' + result["买四申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖四报价']>0">
                    <n-text :type="'info'">{{ "卖四 " + result["卖四报价"] + '(' + result["卖四申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买五报价']>0">
                    <n-text :type="'info'">{{ "买五 " + result["买五报价"] + '(' + result["买五申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖五报价']>0">
                    <n-text :type="'info'">{{ "卖五 " + result["卖五报价"] + '(' + result["卖五申报"] + ")" }}</n-text>
                  </n-gi>
                </n-grid>
              </n-collapse-item>
            </n-collapse>
            <template #header-extra>

              <n-tag size="small" :bordered="false">{{ result['股票代码'] }}</n-tag>&nbsp;
              <n-button size="tiny" secondary type="primary"
                        @click="removeMonitor(result['股票代码'],result['股票名称'],result.key)">
                取消关注
              </n-button>&nbsp;

              <n-button size="tiny" v-if="data.openAiEnable" secondary type="warning"
                        @click="aiCheckStock(result['股票名称'],result['股票代码'])">
                AI分析
              </n-button>
            </template>
            <template #footer>
              <n-flex vertical :size="8">
                <n-flex justify="center">
                  <n-text :type="'info'">{{ result["日期"] + " " + result["时间"] }}</n-text>
                  <n-tag size="small" v-if="result.volume>0" :type="result.profitType">{{ result.volume + "股" }}</n-tag>
                  <n-tag size="small" v-if="result.costPrice>0" :type="result.profitType">
                    {{
                      "成本:" + result.costPrice + "*" + result.costVolume + " " + result.profit + "%" + " ( " + result.profitAmount + " ¥ )"
                    }}
                  </n-tag>
                </n-flex>
                <n-flex justify="center">
                  <n-button size="tiny" type="primary" secondary
                            @click="showLightweightKline(result['股票代码'],result['股票名称'])">
                    多周期K线
                  </n-button>
                </n-flex>
              </n-flex>
            </template>
            <template #action>
              <n-flex justify="left">
                <n-button size="tiny" type="warning" @click="setStock(result['股票代码'],result['股票名称'])"> 成本
                </n-button>
                <n-button size="tiny" type="error"
                          @click="showFenshi(result['股票代码'],result['股票名称'],result.changePercent)"> 分时
                </n-button>
                <n-button size="tiny" type="error" @click="showK(result['股票代码'],result['股票名称'])"> 日K</n-button>
                <n-button size="tiny" type="error" v-if="result['买一报价']>0"
                          @click="showMoney(result['股票代码'],result['股票名称'])"> 资金
                </n-button>
                <n-button size="tiny" type="success" @click="search(result['股票代码'],result['股票名称'])"> 详情
                </n-button>
                <n-button v-if="result['买一报价']>0" size="tiny" type="success"
                          @click="searchNotice(result['股票代码'])"> 公告
                </n-button>
                <n-button v-if="result['买一报价']>0" size="tiny" type="success"
                          @click="searchStockReport(result['股票代码'])"> 研报
                </n-button>
                <n-button size="tiny" type="info"
                          @click="goKlineAnalysis(result['股票代码'], result['股票名称'])"> 技术分析
                </n-button>
                <n-flex justify="right">
                  <n-dropdown trigger="click" :options="groupList" key-field="ID" label-field="name"
                              @select="(groupId) => AddStockGroupInfo(groupId,result['股票代码'],result['股票名称'])">
                    <n-button type="warning" size="tiny">设置分组</n-button>
                  </n-dropdown>
                </n-flex>
              </n-flex>
            </template>
          </n-card>
        </n-gi>
      </n-grid>
    </n-tab-pane>
    <n-tab-pane closable v-for="group in groupList" :group-id="group.ID" :name="String(group.ID)" :tab="group.name">
      <n-grid :x-gap="8" :cols="3" :y-gap="8">
        <n-gi :id="result['股票代码']+'_gi'" v-for="result in groupResults" style="margin-left: 2px;">
          <n-card :data-sort="result.sort" :id="result['股票代码']" :data-code="result['股票代码']" :bordered="true"
                  :title="result['股票名称']" :closable="false"
                  @close="removeMonitor(result['股票代码'],result['股票名称'],result.key)">
            <n-grid :cols="12" :y-gap="6">
              <n-gi :span="6">
                <n-text :type="result.type">
                  <n-number-animation :duration="1000" :precision="2" :from="result['上次当前价格']"
                                      :to="Number(result['当前价格'])"/>
                  <n-tag size="small" :type="result.type" :bordered="false" v-if="result['盘前盘后']>0">
                    ({{ result['盘前盘后'] }} {{ result['盘前盘后涨跌幅'] }}%)
                  </n-tag>
                </n-text>
                <n-text style="padding-left: 10px;" :type="result.type">
                  <n-number-animation :duration="1000" :precision="3" :from="0" :to="result.changePercent"/>
                  %
                </n-text>&nbsp;
                <n-text size="small" v-if="result.costVolume>0" :type="result.type">
                  <n-number-animation :duration="1000" :precision="2" :from="0" :to="result.profitAmountToday"/>
                </n-text>
              </n-gi>
              <n-gi :span="6">
                <stock-spark-line :last-price="Number(result['当前价格'])" :open-price="Number(result['昨日收盘价'])"
                                  :stock-code="result['股票代码']" :stock-name="result['股票名称']"></stock-spark-line>
              </n-gi>
            </n-grid>
            <n-grid :cols="2" :y-gap="4" :x-gap="4">
              <n-gi>
                <n-text :type="'info'">{{ "最高 " + result["今日最高价"] + " " + result.highRate }}%</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "最低 " + result["今日最低价"] + " " + result.lowRate }}%</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "昨收 " + result["昨日收盘价"] }}</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "今开 " + result["今日开盘价"] }}</n-text>
              </n-gi>
            </n-grid>
            <n-collapse accordion v-if="result['买一报价']>0">
              <n-collapse-item title="盘口" name="1" v-if="result['买一报价']>0">
                <template #header-extra>
                  <n-flex justify="space-between">
                    <n-text :type="'info'">{{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}</n-text>
                    <n-text :type="'info'">{{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}</n-text>
                  </n-flex>
                </template>
                <n-grid :cols="2" :y-gap="4" :x-gap="4">
                  <n-gi v-if="result['买一报价']>0">
                    <n-text :type="'info'">{{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖一报价']>0">
                    <n-text :type="'info'">{{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买二报价']>0">
                    <n-text :type="'info'">{{ "买二 " + result["买二报价"] + '(' + result["买二申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖二报价']>0">
                    <n-text :type="'info'">{{ "卖二 " + result["卖二报价"] + '(' + result["卖二申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买三报价']>0">
                    <n-text :type="'info'">{{ "买三 " + result["买三报价"] + '(' + result["买三申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖三报价']>0">
                    <n-text :type="'info'">{{ "买三 " + result["卖三报价"] + '(' + result["卖三申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买四报价']>0">
                    <n-text :type="'info'">{{ "买四 " + result["买四报价"] + '(' + result["买四申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖四报价']>0">
                    <n-text :type="'info'">{{ "卖四 " + result["卖四报价"] + '(' + result["卖四申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买五报价']>0">
                    <n-text :type="'info'">{{ "买五 " + result["买五报价"] + '(' + result["买五申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖五报价']>0">
                    <n-text :type="'info'">{{ "卖五 " + result["卖五报价"] + '(' + result["卖五申报"] + ")" }}</n-text>
                  </n-gi>
                </n-grid>
              </n-collapse-item>
            </n-collapse>
            <template #header-extra>

              <n-tag size="small" :bordered="false">{{ result['股票代码'] }}</n-tag>&nbsp;
              <n-button size="tiny" secondary type="primary"
                        @click="removeMonitor(result['股票代码'],result['股票名称'],result.key)">
                取消关注
              </n-button>&nbsp;

              <n-button size="tiny" v-if="data.openAiEnable" secondary type="warning"
                        @click="aiCheckStock(result['股票名称'],result['股票代码'])">
                AI分析
              </n-button>&nbsp;
              <n-button size="tiny" secondary type="info"
                        @click="showNews(result['股票代码'], result['股票名称'])">
                资讯
              </n-button>
              <n-button secondary type="error" size="tiny"
                        @click="delStockGroup(result['股票代码'],result['股票名称'],group.ID)">移出分组
              </n-button>
            </template>
            <template #footer>
              <n-flex vertical :size="8">
                <n-flex justify="center">
                  <n-text :type="'info'">{{ result["日期"] + " " + result["时间"] }}</n-text>
                  <n-tag size="small" v-if="result.volume>0" :type="result.profitType">{{ result.volume + "股" }}</n-tag>
                  <n-tag size="small" v-if="result.costPrice>0" :type="result.profitType">
                    {{
                      "成本:" + result.costPrice + "*" + result.costVolume + " " + result.profit + "%" + " ( " + result.profitAmount + " ¥ )"
                    }}
                  </n-tag>
                </n-flex>
                <n-flex justify="center">
                  <n-button size="tiny" type="primary" secondary
                            @click="showLightweightKline(result['股票代码'],result['股票名称'])">
                    多周期K线
                  </n-button>
                </n-flex>
              </n-flex>
            </template>
            <template #action>
              <n-flex justify="left">
                <n-button size="tiny" type="warning" @click="setStock(result['股票代码'],result['股票名称'])"> 成本
                </n-button>
                <n-button size="tiny" type="error"
                          @click="showFenshi(result['股票代码'],result['股票名称'],result.changePercent)"> 分时
                </n-button>
                <n-button size="tiny" type="error" @click="showK(result['股票代码'],result['股票名称'])"> 日K</n-button>
                <n-button size="tiny" type="error" v-if="result['买一报价']>0"
                          @click="showMoney(result['股票代码'],result['股票名称'])"> 资金
                </n-button>
                <n-button size="tiny" type="success" @click="search(result['股票代码'],result['股票名称'])"> 详情
                </n-button>
                <n-button v-if="result['买一报价']>0" size="tiny" type="success"
                          @click="searchNotice(result['股票代码'])"> 公告
                </n-button>
                <n-button v-if="result['买一报价']>0" size="tiny" type="success"
                          @click="searchStockReport(result['股票代码'])"> 研报
                </n-button>
                <n-button size="tiny" type="info"
                          @click="goKlineAnalysis(result['股票代码'], result['股票名称'])"> 技术分析
                </n-button>
                <n-flex justify="right">
                  <n-dropdown trigger="click" :options="groupList" key-field="ID" label-field="name"
                              @select="(groupId) => AddStockGroupInfo(groupId,result['股票代码'],result['股票名称'])">
                    <n-button type="warning" size="tiny">设置分组</n-button>
                  </n-dropdown>
                </n-flex>
              </n-flex>
            </template>
          </n-card>
        </n-gi>
      </n-grid>
    </n-tab-pane>
  </n-tabs>

  <div style="position: fixed;bottom: 18px;right:5px;z-index: 10;width: 400px">
    <!--    <n-card :bordered="false">-->
    <n-input-group>
      <!--        <n-button  type="error" @click="addBTN=!addBTN" > <n-icon :component="Search"/>&nbsp;<n-text  v-if="addBTN">隐藏</n-text></n-button>-->

      <n-auto-complete v-model:value="data.name" v-if="addBTN"
                       :input-props="{
                                autocomplete: 'disabled',
                              }"
                       :options="options"
                       placeholder="股票指数名称/代码"
                       clearable @update-value="getStockList" :on-select="onSelect"/>

      <n-popover trigger="manual" :show="showPopover">
        <template #trigger>
          <n-button type="primary" @click="AddStock" v-if="addBTN">
            <n-icon :component="Add"/> &nbsp;关注
          </n-button>
        </template>
        <span>输入股票名称/代码关键词开始吧~~~</span>
      </n-popover>
    </n-input-group>
    <!--    </n-card>-->
  </div>
  <StockCostModal v-model:show="modalShow" :form-model="formModel" @save="updateCostPriceAndVolumeNew"/>

  <n-modal v-model:show="addTabPane" title="添加分组" style="width: 400px;text-align: left" :preset="'card'">
    <n-form
        :model="addTabModel"
        size="medium"
        label-placement="left"
    >
      <n-grid :cols="2">
        <n-form-item-gi label="分组名称:" path="name" :span="5">
          <n-input v-model:value="addTabModel.name" style="width: 100%" placeholder="请输入分组名称"/>
        </n-form-item-gi>
        <n-form-item-gi label="分组排序:" path="sort" :span="5">
          <n-input-number v-model:value="addTabModel.sort" style="width: 100%" min="0"
                          placeholder="请输入分组排序值"></n-input-number>
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="saveTabPane">
          保存
        </n-button>
        <n-button type="warning" @click="addTabPane=false">
          取消
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="modalShow2" :title="data.name+' '+ data.changePercent+'%'" style="width: 1000px;max-width: calc(100vw - 32px);"
           :preset="'card'" @after-enter="handleFeishi" @after-leave="clearFeishi">
    <!--    <n-image :src="data.fenshiURL" />-->
    <div ref="kLineChartRef2" style="width: 100%; height: 500px;"></div>
  </n-modal>
  <n-modal v-model:show="modalShow3" :title="data.name" style="width: 1000px;max-width: calc(100vw - 32px);" :preset="'card'"
           @after-enter="handleKLine">
    <!--    <n-image :src="data.kURL" />-->
    <div ref="kLineChartRef" style="width: 100%; height: 500px;"></div>
  </n-modal>

  <StockAiModal v-model:show="modalShow4" :data="data" :multi-agent-state="multiAgentState"
                :strategies="strategies" :ai-configs="aiConfigs" :sys-prompt-options="sysPromptOptions"
                :user-prompt-options="userPromptOptions"
                v-model:enable-tools="enableTools" v-model:thinking-mode="thinkingMode" v-model:strategy-code="strategyCode"
                :md-editor-ref="mdEditorRef" :md-preview-ref="mdPreviewRef"
                :ai-result-scroll-ref="aiResultScrollRef" :tips-ref="tipsRef"
                :ai-re-check-stock="aiReCheckStock" :save-as-image="saveAsImage"
                :copy-to-clipboard="copyToClipboard" :save-as-markdown="saveAsMarkdown"
                :save-as-word="saveAsWord" :share="share"/>
  <n-modal v-model:show="modalShow5" :title="data.name+'资金趋势'" style="width: 1000px;max-width: calc(100vw - 32px);" :preset="'card'">
    <money-trend :code="data.code" :name="data.name" :days="360" :dark-theme="data.darkTheme"
                 :chart-height="500"></money-trend>
  </n-modal>
  <n-modal
    v-model:show="modalShow6"
    :title="(lwKlineName || '') + ' — 多周期K线'"
    preset="card"
    style="width: min(1100px, 96vw); max-width: 96vw; box-sizing: border-box"
    :content-style="{
      maxHeight: 'min(85vh, 820px)',
      overflowY: 'auto',
      overflowX: 'hidden',
      minWidth: 0,
      boxSizing: 'border-box',
    }"
  >
    <stock-lightweight-kline-chart
      v-if="modalShow6"
      :key="'lightweight-' + lwKlineCode"
      :code="lwKlineCode"
      :stock-name="lwKlineName"
      :dark-theme="data.darkTheme"
      :chart-height="500"
      :long-entry-price="currentStockTradingPrice.entryPrice"
      :long-stop-loss-price="currentStockTradingPrice.stopLossPrice"
      :long-take-profit-price="currentStockTradingPrice.takeProfitPrice"
      :cost-price="currentStockTradingPrice.costPrice"
      @update:longEntryPrice="handleLongEntryPriceUpdate"
      @update:longStopLossPrice="handleLongStopLossPriceUpdate"
      @update:longTakeProfitPrice="handleLongTakeProfitPriceUpdate"
      @update:costPrice="handleCostPriceUpdate"
    />
  </n-modal>
  <n-modal v-model:show="modalShowNews" :title="newsName + ' - 关联资讯'" style="width: 800px;max-width: calc(100vw - 32px);" :preset="'card'">
    <StockNews :code="newsCode" />
  </n-modal>
</template>

<style scoped>
/* 添加闪烁效果的CSS类 */
.blink-border {
  animation: blink-border 1s linear infinite;
  border: 4px solid transparent;
}

@keyframes blink-border {
  0% {
    border-color: red;
  }
  50% {
    border-color: transparent;
  }
  100% {
    border-color: red;
  }
}

/* 所有标签的通用样式 */
:deep(.n-tabs-nav .n-tabs-tab) {
  position: relative;
  cursor: pointer;
}

/* 可拖拽标签的样式 */
:deep(.n-tabs-nav .n-tabs-tab[draggable="true"]) {
  user-select: none;
  cursor: move;
}

.tab-drag-over {
  background-color: #e6f7ff !important;
  border: 2px dashed #1890ff !important;
  transform: scale(1.02);
  transition: all 0.2s ease;
  z-index: 10;
}

.tab-drag-over::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: -1;
}

.tab-dragging {
  opacity: 0.5;
}
</style>
