/**
 * 股票相关状态 Store
 * 管理自选股、群组、实时数据等股票状态
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

/** 股票相关状态 */
export interface StockState {
  /** 自选股列表 */
  stockList: any[]
  /** 群组列表 */
  groupList: any[]
  /** 实时盈亏 */
  realtimeProfit: number
  /** 当前激活的群组 */
  activeGroupId: number
  activeGroupName: string
  /** 股票详情缓存 */
  stockDetailCache: Map<string, any>
}

export const useStockStore = defineStore('stock', () => {
  // ========== 状态 ==========

  /** 自选股列表 */
  const stockList = ref<any[]>([])

  /** 群组列表 */
  const groupList = ref<any[]>([])

  /** 实时盈亏 */
  const realtimeProfit = ref<number>(0)

  /** 当前激活的群组 */
  const activeGroupId = ref<number>(0)
  const activeGroupName = ref<string>('全部')

  /** 股票详情缓存 */
  const stockDetailCache = ref<Map<string, any>>(new Map())

  // ========== 计算属性 ==========

  /** 当前群组的股票列表 */
  const currentGroupStocks = computed(() => {
    if (activeGroupId.value === 0) {
      return stockList.value
    }
    // 这里需要根据 groupId 过滤，实际实现需要根据数据结构调整
    return stockList.value.filter(stock => stock.groupId === activeGroupId.value)
  })

  /** 总盈亏 */
  const totalProfit = computed(() => {
    return stockList.value.reduce((sum, stock) => sum + (stock.profit || 0), 0)
  })

  // ========== 方法 ==========

  /**
   * 设置股票列表
   */
  function setStockList(list: any[]): void {
    stockList.value = list
  }

  /**
   * 设置群组列表
   */
  function setGroupList(list: any[]): void {
    groupList.value = list
  }

  /**
   * 更新实时盈亏
   */
  function setRealtimeProfit(value: number): void {
    realtimeProfit.value = value
  }

  /**
   * 切换活跃群组
   */
  function setActiveGroup(groupId: number, groupName: string): void {
    activeGroupId.value = groupId
    activeGroupName.value = groupName
  }

  /**
   * 添加股票到自选
   */
  function addStock(stock: any): void {
    const exists = stockList.value.find(s => s.code === stock.code)
    if (!exists) {
      stockList.value.push(stock)
    }
  }

  /**
   * 从自选移除股票
   */
  function removeStock(code: string): void {
    const index = stockList.value.findIndex(s => s.code === code)
    if (index > -1) {
      stockList.value.splice(index, 1)
    }
  }

  /**
   * 更新股票价格
   */
  function updateStockPrice(code: string, price: number): void {
    const stock = stockList.value.find(s => s.code === code)
    if (stock) {
      stock.price = price
    }
  }

  /**
   * 缓存股票详情
   */
  function cacheStockDetail(code: string, detail: any): void {
    stockDetailCache.value.set(code, {
      data: detail,
      timestamp: Date.now(),
    })
  }

  /**
   * 获取缓存的股票详情
   */
  function getCachedStockDetail(code: string, maxAge: number = 5 * 60 * 1000): any {
    const cached = stockDetailCache.value.get(code)
    if (!cached) return null
    if (Date.now() - cached.timestamp > maxAge) {
      stockDetailCache.value.delete(code)
      return null
    }
    return cached.data
  }

  return {
    // 状态
    stockList,
    groupList,
    realtimeProfit,
    activeGroupId,
    activeGroupName,
    stockDetailCache,

    // 计算属性
    currentGroupStocks,
    totalProfit,

    // 方法
    setStockList,
    setGroupList,
    setRealtimeProfit,
    setActiveGroup,
    addStock,
    removeStock,
    updateStockPrice,
    cacheStockDetail,
    getCachedStockDetail,
  }
})

export default useStockStore
