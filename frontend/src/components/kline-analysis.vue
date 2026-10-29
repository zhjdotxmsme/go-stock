<script setup>
import * as stockApi from '../api/stock'
import * as systemApi from '../api/system'
import { pinyin } from 'pinyin-pro'
import { EventsOn } from '../../wailsjs/runtime'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import { NAutoComplete, NButton, NFlex, NText, NInputGroup } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { onBeforeMount, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const message = useMessage()
const route = useRoute()
const router = useRouter()
const searchQuery = ref('')
const selectedCode = ref('000001.SH')
const selectedName = ref('上证指数')
const stockList = ref([])
const options = ref([])
const isFromQuery = ref(false)
const darkTheme = ref(false)
const chartHeight = ref(window.innerHeight - 230)
const recentStocks = ref([])
const unsupportedCode = ref(false)
let stockChangeHandler = null

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

// 检索归一化：全角转半角，并去掉空格、连字符等分隔符，保证「连打」能命中
// 例：五 粮 液 → 五粮液、中巨芯-U → 中巨芯u、万  科Ａ → 万科a
function normalizeSearchKey(s) {
  return String(s || '')
    .replace(/[\uff01-\uff5e]/g, ch => String.fromCharCode(ch.charCodeAt(0) - 0xfee0))
    .replace(/[^\p{L}\p{N}]/gu, '')
    .toLowerCase()
}

// 繁→简（常用公司/港股名用字）：pinyin-pro 的多音词组词典只按简体建索引，
// 不转换会把「恒生銀行」的行读成 xíng（yx）而不是 háng（yh）
const TRAD_TO_SIMP = {
  銀: '银', 業: '业', 發: '发', 長: '长', 慶: '庆', 廈: '厦', 門: '门', 樂: '乐', 娛: '娱',
  東: '东', 鑰: '钥', 給: '给', 側: '侧', 祕: '秘', 魯: '鲁', 國: '国', 華: '华', 興: '兴',
  際: '际', 藥: '药', 證: '证', 險: '险', 產: '产', 團: '团', 島: '岛', 灣: '湾', 龍: '龙',
  萬: '万', 亞: '亚', 恆: '恒', 豐: '丰', 實: '实', 創: '创', 營: '营', 聯: '联', 匯: '汇',
  滙: '汇', 財: '财', 貿: '贸', 運: '运', 輸: '输', 電: '电', 鋼: '钢', 鐵: '铁', 銅: '铜',
  鋁: '铝', 輕: '轻', 車: '车', 馬: '马', 醫: '医', 學: '学', 農: '农', 縣: '县', 鎮: '镇',
  廣: '广', 寧: '宁', 寶: '宝', 麗: '丽', 潤: '润', 貴: '贵', 陽: '阳', 蘇: '苏', 閩: '闽',
  雲: '云', 煙: '烟', 臺: '台', 網: '网', 訊: '讯', 軟: '软', 數: '数', 據: '据', 機: '机',
  構: '构', 認: '认', 議: '议', 記: '记', 務: '务', 勢: '势', 億: '亿', 債: '债', 儲: '储',
  權: '权', 錢: '钱', 濟: '济', 眾: '众', 競: '竞', 總: '总', 會: '会',
}

function toSimplified(s) {
  let out = ''
  for (const ch of String(s || '')) out += TRAD_TO_SIMP[ch] || ch
  return out
}

// 股票名称拼音首字母（gzmt → 贵州茅台）：pinyin-pro 带词组词典，可正确处理多音字（银行→yh、重庆→cq）
const pinyinCache = new Map()

function initialsOf(name) {
  const key = String(name || '')
  if (!key) return ''
  if (pinyinCache.has(key)) return pinyinCache.get(key)
  let py = ''
  try {
    py = pinyin(toSimplified(key), { pattern: 'first', toneType: 'none', type: 'array' }).join('')
  } catch (_) {
    py = ''
  }
  const normalized = normalizeSearchKey(py)
  pinyinCache.set(key, normalized)
  return normalized
}

// 单条索引（归一化名称 + 拼音首字母），幂等：已建好就直接返回
function ensureItemIndex(item) {
  if (!item || item.nameKey !== undefined) return
  item.nameKey = normalizeSearchKey(item.name)
  item.pyInit = initialsOf(item.name)
}

let indexBuilt = false

// 分片建立检索索引，避免一次性转换上万条名称卡住界面
function buildPinyinIndex() {
  indexBuilt = false
  let i = 0
  const step = () => {
    const end = Math.min(i + 1500, stockList.value.length)
    for (; i < end; i++) ensureItemIndex(stockList.value[i])
    if (i < stockList.value.length) setTimeout(step, 0)
    else indexBuilt = true
  }
  step()
}

function findStockList(val) {
  if (!val || !val.trim()) {
    options.value = []
    return
  }
  // 分片建索引尚未完成时先补齐剩余条目，避免搜索落在这个时间窗口里漏检
  if (!indexBuilt) {
    for (const item of stockList.value) ensureItemIndex(item)
    indexBuilt = true
  }
  const q = val.trim().toLowerCase()
  const qn = normalizeSearchKey(val)
  const filtered = stockList.value.filter(item =>
    item.name.toLowerCase().includes(q) ||
    item.ts_code.toLowerCase().includes(q) ||
    (qn && ((item.nameKey && item.nameKey.includes(qn)) ||
      (item.pyInit && item.pyInit.includes(qn))))
  ).slice(0, 30)
  options.value = filtered.map(item => ({
    label: item.name + ' - ' + item.ts_code,
    value: item.ts_code,
  }))
}

function handleSearch(value) {
  const emCode = toEastMoneyCode(value)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  const found = stockList.value.find(item => item.ts_code === value)
  selectedName.value = found ? found.name : ''
  addToRecent(value, selectedName.value)
}

function addToRecent(code, name) {
  const list = recentStocks.value.filter(s => s.code !== code)
  list.unshift({ code, name })
  if (list.length > 10) list.length = 10
  recentStocks.value = list
  try {
    localStorage.setItem('kline-recent-stocks', JSON.stringify(list))
  } catch {}
}

function loadRecentStocks() {
  try {
    const raw = localStorage.getItem('kline-recent-stocks')
    if (raw) recentStocks.value = JSON.parse(raw)
  } catch {}
}

function selectRecent(code, name) {
  const emCode = toEastMoneyCode(code)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  selectedName.value = name
  addToRecent(code, name)
}

function updateChartHeight() {
  chartHeight.value = Math.max(400, window.innerHeight - 230)
}

onBeforeMount(() => {
  stockApi.getStockList('').then(({data: result}) => {
    stockList.value = result || []
    buildPinyinIndex()
  }).catch(err => { console.error('GetStockList error:', err) })
  systemApi.getConfig().then(({data: result}) => {
    darkTheme.value = !!result.darkTheme
  }).catch(err => { console.error('GetConfig error:', err) })
})

onMounted(() => {
  loadRecentStocks()
  updateChartHeight()
  window.addEventListener('resize', updateChartHeight)

  // 从 query 参数自动加载股票（来自 stock.vue 的跳转）
  if (route.query.code) {
    selectedCode.value = route.query.code
    selectedName.value = route.query.name || ''
    isFromQuery.value = true
    searchQuery.value = selectedName.value
    addToRecent(route.query.code, selectedName.value)
  }

  stockChangeHandler = (data) => {
    if (data && data.ts_code) {
      const emCode = toEastMoneyCode(data.ts_code)
      if (!emCode) {
        unsupportedCode.value = true
        return
      }
      unsupportedCode.value = false
      selectedCode.value = emCode
      selectedName.value = data.name || ''
      addToRecent(data.ts_code, data.name || '')
    }
  }
  EventsOn('klineSelectStock', stockChangeHandler)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateChartHeight)
})

// 监听路由 query 变化（从 stock.vue 切换不同股票时）
watch(() => route.query.code, (newCode) => {
  if (newCode && newCode !== selectedCode.value) {
    selectedCode.value = newCode
    selectedName.value = route.query.name || ''
    isFromQuery.value = true
    searchQuery.value = selectedName.value
    addToRecent(newCode, selectedName.value)
  }
})

function goBack() {
  router.push({ path: '/' })
}
</script>

<template>
  <div class="kline-analysis-page" :class="{ 'kline-analysis-page--dark': darkTheme }">
    <div class="kline-title-bar">
      <NButton v-if="isFromQuery" size="tiny" quaternary @click="goBack" style="margin-right: 8px; padding: 0 4px">⟵ 返回</NButton>
      <NText :depth="darkTheme ? 1 : 3" style="font-size: 15px; font-weight: 700">{{ selectedName }}&nbsp;</NText>
      <NText depth="3" style="font-size: 13px">{{ selectedCode }}</NText>
    </div>
    <StockLightweightKlineChart
      :key="selectedCode"
      :code="selectedCode"
      :stockName="selectedName"
      :darkTheme="darkTheme"
      :chartHeight="chartHeight"
      :realtimeIntervalMs="60000"
    />

    <div class="kline-search-bar">
      <n-input-group>
        <n-auto-complete
          v-model:value="searchQuery"
          :options="options"
          placeholder="股票名称/代码搜索..."
          clearable
          :on-select="handleSearch"
          @update:value="findStockList"
        />
        <n-button type="primary">
          🔍
        </n-button>
      </n-input-group>
      <NFlex v-if="unsupportedCode" align="center" :size="6" style="margin-top: 4px">
        <NText type="warning" style="font-size: 12px">该股票暂不支持K线图</NText>
      </NFlex>
      <div v-if="recentStocks.length && !selectedCode" class="recent-stocks">
        <NText depth="3" style="font-size: 11px; white-space: nowrap">最近:</NText>
        <n-button
          v-for="s in recentStocks.slice(0, 6)"
          :key="s.code"
          size="tiny"
          secondary
          @click="selectRecent(s.code, s.name)"
        >
          {{ s.name }}
        </n-button>
      </div>
    </div>

  </div>
</template>

<style scoped>
.kline-analysis-page {
  width: 100%;
  padding: 4px 8px;
  box-sizing: border-box;
  --wails-draggable: no-drag;
  position: relative;
}
.kline-analysis-page--dark {
  background: #0a0a0a;
  color: #e2e8f0;
}
.kline-title-bar {
  padding: 2px 0 4px 0;
}
.kline-search-bar {
  position: fixed;
  bottom: calc(var(--app-bottombar-h, 68px) + 14px);
  right: 12px;
  z-index: 10;
  width: 320px;
}
.recent-stocks {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  flex-wrap: wrap;
}
</style>
