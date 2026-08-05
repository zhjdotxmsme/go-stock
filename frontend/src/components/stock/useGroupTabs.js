/**
 * 自选股分组标签：新增/重命名/删除分组、组内删股、公告/研报跳转。
 * 自 stock.vue 原样搬迁；依赖经 ctx 传入，updateData 由组件注入。
 */
import { ref, watch } from 'vue'
import * as stockApi from '../../api/stock'

export function useGroupTabs(ctx) {
  const {
    router, dialog, message, stocks, followList, groupList, modalShow6,
    klineAutoCloseTimer, data, currentGroupId, updateData,
  } = ctx

  const addTabModel = ref({
    name: '',
    sort: 1,
  })
  const addTabPane = ref(false)
  
  function addTab() {
    addTabPane.value = true
  }
  
  function saveTabPane() {
    stockApi.addGroup(addTabModel.value).then(({data: result}) => {
      message.info(result)
      addTabPane.value = false
      stockApi.getGroupList().then(({data: result}) => {
        groupList.value = result
      })
    })
  }
  
  function AddStockGroupInfo(groupId, code, name) {
    if (code.startsWith("gb_")) {
      code = "us" + code.replace("gb_", "").toLowerCase()
    }
    stockApi.addStockToGroup(groupId, code).then(({data: result}) => {
      message.info(result)
      stockApi.getGroupList().then(({data: result}) => {
        groupList.value = result
      })
    })
  
  }
  
  function updateTab(name) {
    stocks.value = []
    const tabId= Number(name)
    currentGroupId.value = tabId;
    stockApi.getFollowList(tabId).then(({data: result}) => {
      followList.value = result
  
      for (const followedStock of result) {
        if (followedStock.StockCode.startsWith("us")) {
          followedStock.StockCode = "gb_" + followedStock.StockCode.replace("us", "").toLowerCase()
        }
        stocks.value.push(followedStock.StockCode)
        stockApi.greet(followedStock.StockCode).then(({data: result}) => {
          updateData(result)
        })
      }
      //monitor()
      message.destroyAll()
    })
  }
  
  function delTab(groupId) {
    let infos = groupList.value = groupList.value.filter(item => item.ID === Number(groupId))
    dialog.create({
      title: '删除分组',
      type: 'warning',
      content: '确定要删除[' + infos[0].name + ']分组吗？分组数据将不能恢复哟！',
      positiveText: '确定',
      negativeText: '取消',
      onPositiveClick: () => {
        stockApi.removeGroup(Number(groupId)).then(({data: result}) => {
          message.info(result)
          stockApi.getGroupList().then(({data: result}) => {
            groupList.value = result
          })
        })
      }
    })
  }
  
  function delStockGroup(code, name, groupId) {
    stockApi.removeStockGroup(code, name, groupId).then(({data: result}) => {
      updateTab(groupId)
      message.info(result)
    })
  }
  
  function searchNotice(stockCode) {
    router.push({
      name: 'market',
      query: {
        name: '公司公告',
        stockCode: stockCode,
      },
    })
  }
  
  function searchStockReport(stockCode) {
    router.push({
      name: 'market',
      query: {
        name: '个股研报',
        stockCode: stockCode,
      },
    })
  }
  
  // 监听多周期 K 线模态框关闭，清除定时器
  watch(modalShow6, (newVal) => {
    if (!newVal && klineAutoCloseTimer.value) {
      clearTimeout(klineAutoCloseTimer.value)
      klineAutoCloseTimer.value = null
    }
  })

  return {
    addTabModel, addTabPane, addTab, saveTabPane, AddStockGroupInfo,
    updateTab, delTab, delStockGroup, searchNotice, searchStockReport,
  }
}
