# Phase 4: App.vue 增量迁移计划

> 目标：将 App.vue 从 1279 行的单体文件重构为基于 Pinia + Composables 的模块化架构

## 当前状态

- ✅ `src/stores/app.js` - 全局应用状态 (loading, theme, mottos)
- ✅ `src/stores/stock.js` - 股票自选状态 (watchlist, groups, realtimeProfit)
- ✅ `src/stores/settings.js` - 设置状态
- ✅ `src/composables/useNavigation.js` - 导航逻辑 + 菜单配置
- ✅ `src/composables/useMarketStatus.js` - 市场状态 + 窗口标题
- ✅ `src/composables/useWailsEvents.js` - 事件监听管理
- ✅ `src/config/navigation.js` - 菜单配置工厂

## 迁移步骤

### Step 1: 状态迁移到 Pinia Stores (当前: ⏳ 进行中)

将以下状态从 App.vue 迁移到对应 Store：

| 状态变量 | 目标 Store | 优先级 |
|---------|-----------|--------|
| `loading`, `loadingMsg` | `appStore` | P0 |
| `enableDarkTheme` | `settingsStore` | P0 |
| `enableNews`, `enableFund`, `enableAgent` | `settingsStore` | P1 |
| `content` (免责声明) | `appStore` | P1 |
| `officialStatement` | `appStore` | P0 |
| `investmentMottos`, `currentMotto` | `appStore` | P1 |
| `marketStatus` | `appStore` | P0 |
| `realtimeProfit` | `stockStore` | P0 |
| `telegraph` | `marketStore` (新建) | P2 |
| `groupList` | `stockStore` | P1 |
| `isFullscreen`, `activeKey` | `useNavigation` | ✅ Done |

### Step 2: 逻辑迁移到 Composables

| 函数 | 目标 Composable | 优先级 |
|-----|----------------|--------|
| `updateMarketStatus()` | `useMarketStatus` | ✅ Done |
| `refreshMotto()` | `appStore` action | P1 |
| `toggleFullscreen()` | `useNavigation` | ✅ Done |
| `renderIcon()` | `useNavigation` | ✅ Done |
| 事件监听 (realtime_profit, telegraph, etc.) | `useWailsEvents` | ✅ Done |
| 菜单配置 (menuOptions) | `useNavigation` | ✅ Done |

### Step 3: 逐步替换 inline 逻辑

保持 template 不变，仅将 script setup 中的逻辑逐步替换：

1. **导入新增依赖**
   ```javascript
   import { useAppStore, useStockStore, useSettingsStore } from './stores'
   import { useNavigation } from './composables/useNavigation'
   import { useMarketStatus } from './composables/useMarketStatus'
   import { useWailsEvents } from './composables/useWailsEvents'
   ```

2. **替换 navigation 相关**
   ```javascript
   // 旧代码
   const isFullscreen = ref(false)
   const activeKey = ref('stock')
   const menuOptions = ref([...])
   
   // 新代码
   const { activeKey, isFullscreen, menuOptions, toggleFullscreen } = useNavigation()
   ```

3. **替换 market status 相关**
   ```javascript
   // 旧代码
   const marketStatus = ref('')
   let marketStatusTimer = null
   function updateMarketStatus() { ... }
   
   // 新代码
   const { marketStatus } = useMarketStatus()
   ```

4. **替换事件监听**
   ```javascript
   // 旧代码
   EventsOn("realtime_profit", (data) => { realtimeProfit.value = data })
   // ...
   
   // 新代码
   const { registerEvents } = useWailsEvents()
   onMounted(() => registerEvents())
   ```

5. **替换 Pinia 状态**
   ```javascript
   // 旧代码
   const loading = ref(true)
   const realtimeProfit = ref(0)
   
   // 新代码
   const appStore = useAppStore()
   const stockStore = useStockStore()
   // 使用 appStore.loading, appStore.loadingMsg, stockStore.realtimeProfit
   ```

### Step 4: 清理未使用的导入

完成替换后，移除不再需要的 icon 和 API 导入。

## 风险控制

1. **每一步替换后运行 build**，确保没有破坏
2. **保留旧代码作为注释**，直到新代码验证通过
3. **可随时回滚**：通过 git checkout 恢复原始 App.vue

## 预期收益

| 指标 | 重构前 | 重构后 |
|-----|-------|-------|
| App.vue 行数 | 1279 | ~200 |
| 状态管理 | 分散 inline | 集中 Pinia |
| 可测试性 | ❌ 难以单独测试 | ✅ 每个 composable 可测试 |
| 可维护性 | ❌ 单体文件 | ✅ 模块化 + 职责清晰 |
| 代码复用 | ❌ 复制粘贴 | ✅ composables 可复用 |

## 相关文件

- `frontend/src/stores/` - Pinia 状态管理
- `frontend/src/composables/` - 逻辑复用
- `frontend/src/config/navigation.js` - 菜单配置
- `frontend/src/api/` - API 层封装
