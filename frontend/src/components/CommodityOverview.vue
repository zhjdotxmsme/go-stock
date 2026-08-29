<script setup>
import {ref, computed, onMounted, onBeforeUnmount} from 'vue'
import * as commodityApi from '../api/commodity'
import CommodityKlineChart from './CommodityKlineChart.vue'

const selectedCode = ref('XAUUSD')
const selectedName = ref('现货黄金(纽约)')
const quotes = ref({})
const errors = ref({})
const registry = ref([])
const macro = ref(null)
const macroLoading = ref(false)

let quoteTimer = null
let macroTimer = null

// 按分类分组
const categoryOrder = ['贵金属', '能源', '基金']

const groupedAssets = computed(() => {
  const groups = {}
  for (const cat of categoryOrder) {
    groups[cat] = registry.value.filter(a => a.category === cat && a.isTradable)
  }
  return groups
})

async function loadRegistry() {
  registry.value = (await commodityApi.getCommodityRegistry()).data
}

async function loadQuotes() {
  // 只加载可交易标的的报价。
  // 并行加载：串行时任何一个慢源（如 Yahoo 限流后的 PowerShell 降级，
  // 单次可达 15~30s）都会把后续所有品种的报价卡住，页面长时间无值。
  const tradable = registry.value.filter(a => a.isTradable)
  await Promise.all(tradable.map(async (asset) => {
    try {
      const q = (await commodityApi.getCommodityQuote(asset.code)).data
      if (q) {
        quotes.value[asset.code] = q
        errors.value[asset.code] = ''
      }
    } catch (e) {
      console.error('quote error', asset.code, e)
      errors.value[asset.code] = (e && e.message) || e
    }
  }))
}

async function loadMacro() {
  macroLoading.value = true
  try {
    macro.value = (await commodityApi.getMacroIndicatorsEnhanced()).data
  } catch (e) {
    console.error('macro indicators error', e)
  } finally {
    macroLoading.value = false
  }
}

function selectAsset(code, name) {
  selectedCode.value = code
  selectedName.value = name
}

function formatPrice(p) {
  if (p === undefined || p === null) return '--'
  return Number(p).toFixed(2)
}

function formatPct(p) {
  if (p === undefined || p === null) return '--'
  const t = Number(p).toFixed(2)
  return Number(p) > 0 ? '+' + t + '%' : t + '%'
}

function formatYield(v) {
  if (v === undefined || v === null || v === 0) return '--'
  return Number(v).toFixed(2) + '%'
}

function getCategoryEmoji(cat) {
  const map = {'贵金属': '🥇', '能源': '🛢️', '基金': '📊', '宏观': '💰'}
  return map[cat] || '📋'
}

const yieldCurveLabel = computed(() => {
  if (!macro.value) return ''
  const map = {normal: '正常', inverted: '倒挂', steep: '陡峭'}
  return map[macro.value.yieldCurve] || ''
})

const yieldCurveType = computed(() => {
  if (!macro.value) return 'default'
  const map = {normal: 'success', inverted: 'error', steep: 'warning'}
  return map[macro.value.yieldCurve] || 'default'
})

const colsMap = {
  '贵金属': 4,
  '能源': 3,
  '基金': 2,
}

onMounted(() => {
  loadRegistry()
  loadQuotes()
  loadMacro()
  quoteTimer = setInterval(loadQuotes, 5000)
  macroTimer = setInterval(loadMacro, 30000)
})

onBeforeUnmount(() => {
  if (quoteTimer) clearInterval(quoteTimer)
  if (macroTimer) clearInterval(macroTimer)
})
</script>

<template>
  <n-space vertical>
    <!-- 可交易标的分组展示 -->
    <template v-for="cat in categoryOrder" :key="cat">
      <template v-if="groupedAssets[cat] && groupedAssets[cat].length > 0">
        <n-text strong style="font-size: 14px">{{ getCategoryEmoji(cat) }} {{ cat }}</n-text>
        <n-grid :cols="colsMap[cat] || 3" :x-gap="12" :y-gap="12">
          <n-grid-item v-for="asset in groupedAssets[cat]" :key="asset.code">
            <n-card
                size="small"
                :class="{'border-blue-500': selectedCode === asset.code}"
                hoverable
                @click="selectAsset(asset.code, asset.name)"
            >
              <n-space justify="space-between" align="center">
                <div>
                  <n-text depth="3" style="font-size: 12px">{{ asset.name }}</n-text>
                  <div class="text-lg font-bold">{{ formatPrice(quotes[asset.code]?.Price) }}</div>
                  <div v-if="errors[asset.code]" class="text-xs text-red-500">{{ errors[asset.code] }}</div>
                </div>
                <div :class="quotes[asset.code]?.ChangePct >= 0 ? 'text-red-500' : 'text-green-500'" style="font-size: 13px">
                  {{ formatPct(quotes[asset.code]?.ChangePct) }}
                </div>
              </n-space>
            </n-card>
          </n-grid-item>
        </n-grid>
      </template>
    </template>

    <!-- 宏观指标 -->
    <n-text strong style="font-size: 14px">💰 宏观指标</n-text>
    <n-spin :show="macroLoading && !macro">
      <n-card size="small">
        <template v-if="macro">
          <n-grid :cols="4" :x-gap="16" :y-gap="12">
            <!-- 美元指数 + 欧元/美元 + ETF -->
            <n-grid-item>
              <n-space vertical size="small">
                <n-text depth="3" style="font-size: 12px">美元指数 DXY</n-text>
                <n-text strong style="font-size: 18px">{{ formatPrice(macro.dxy) }}</n-text>
              </n-space>
            </n-grid-item>
            <n-grid-item>
              <n-space vertical size="small">
                <n-text depth="3" style="font-size: 12px">欧元/美元 EURUSD</n-text>
                <n-text strong style="font-size: 18px">{{ formatPrice(macro.eurusd) }}</n-text>
              </n-space>
            </n-grid-item>
            <n-grid-item>
              <n-space vertical size="small">
                <n-text depth="3" style="font-size: 12px">20+年美债ETF TLT</n-text>
                <n-text strong style="font-size: 18px">{{ formatPrice(macro.tltPrice) }}
                  <n-text :class="macro.tltChangePct >= 0 ? 'text-red-500' : 'text-green-500'" style="font-size: 12px">
                    {{ formatPct(macro.tltChangePct) }}
                  </n-text>
                </n-text>
              </n-space>
            </n-grid-item>
            <n-grid-item>
              <n-space vertical size="small">
                <n-text depth="3" style="font-size: 12px">TIPS债券ETF TIP</n-text>
                <n-text strong style="font-size: 18px">{{ formatPrice(macro.tipPrice) }}
                  <n-text :class="macro.tipChangePct >= 0 ? 'text-red-500' : 'text-green-500'" style="font-size: 12px">
                    {{ formatPct(macro.tipChangePct) }}
                  </n-text>
                </n-text>
              </n-space>
            </n-grid-item>
          </n-grid>

          <n-divider style="margin: 12px 0 8px 0">美债收益率</n-divider>

          <n-grid :cols="4" :x-gap="16" :y-gap="8">
            <n-grid-item v-for="item in [
              {label: '2Y', value: macro.us2yr},
              {label: '5Y', value: macro.us5yr},
              {label: '7Y', value: macro.us7yr},
              {label: '10Y', value: macro.us10yr},
            ]" :key="item.label">
              <n-space vertical align="center" size="0">
                <n-text depth="3" style="font-size: 12px">美国{{ item.label }}期</n-text>
                <n-text strong style="font-size: 16px">{{ formatYield(item.value) }}</n-text>
              </n-space>
            </n-grid-item>
          </n-grid>
          <n-grid :cols="2" :x-gap="16" :y-gap="8" style="margin-top: 4px">
            <n-grid-item>
              <n-space vertical align="center" size="0">
                <n-text depth="3" style="font-size: 12px">美国30年期</n-text>
                <n-text strong style="font-size: 16px">{{ formatYield(macro.us30yr) }}</n-text>
              </n-space>
            </n-grid-item>
            <n-grid-item>
              <n-space vertical align="center" size="0">
                <n-text depth="3" style="font-size: 12px">收益率曲线</n-text>
                <n-tag :type="yieldCurveType" size="medium" round>{{ yieldCurveLabel }}</n-tag>
              </n-space>
            </n-grid-item>
          </n-grid>

          <n-divider style="margin: 12px 0 8px 0">TIPS 实际利率</n-divider>

          <n-grid :cols="4" :x-gap="16" :y-gap="8">
            <n-grid-item v-for="item in [
              {label: '5Y TIPS', value: macro.tips5y},
              {label: '10Y TIPS', value: macro.tips10y},
              {label: '20Y TIPS', value: macro.tips20y},
              {label: '30Y TIPS', value: macro.tips30y},
            ]" :key="item.label">
              <n-space vertical align="center" size="0">
                <n-text depth="3" style="font-size: 12px">{{ item.label }}</n-text>
                <n-text strong style="font-size: 16px">{{ formatYield(item.value) }}</n-text>
              </n-space>
            </n-grid-item>
          </n-grid>

          <n-divider style="margin: 12px 0 8px 0">盈亏平衡通胀率</n-divider>

          <n-grid :cols="2" :x-gap="16" :y-gap="8">
            <n-grid-item>
              <n-space vertical align="center" size="0">
                <n-text depth="3" style="font-size: 12px">5年盈亏平衡通胀</n-text>
                <n-text strong style="font-size: 16px">{{ formatYield(macro.breakEven5y) }}</n-text>
              </n-space>
            </n-grid-item>
            <n-grid-item>
              <n-space vertical align="center" size="0">
                <n-text depth="3" style="font-size: 12px">10年盈亏平衡通胀</n-text>
                <n-text strong style="font-size: 16px">{{ formatYield(macro.breakEven10y) }}</n-text>
              </n-space>
            </n-grid-item>
          </n-grid>
        </template>
        <template v-else-if="!macroLoading">
          <n-text depth="3">宏观指标加载失败</n-text>
        </template>
      </n-card>
    </n-spin>

    <!-- K线图 -->
    <n-card :title="selectedName + ' K线图'" size="small">
      <CommodityKlineChart :code="selectedCode" :name="selectedName" period="day"/>
    </n-card>
  </n-space>
</template>
