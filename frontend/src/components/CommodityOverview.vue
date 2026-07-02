<script setup>
import {ref, onMounted, onBeforeUnmount} from 'vue'
import {GetCommodityQuote, GetCommodityRegistry} from '../../wailsjs/go/main/App'
import CommodityKlineChart from "./CommodityKlineChart.vue";

const selectedCode = ref('XAUUSD')
const selectedName = ref('现货黄金')
const quotes = ref({})
const errors = ref({})
const registry = ref([])
const loading = ref(false)
const mainAssets = ref([
  {code: 'XAUUSD', name: '现货黄金'},
  {code: 'XAGUSD', name: '现货白银'},
  {code: 'USCL', name: 'WTI原油'},
  {code: 'AU', name: '沪金'},
])

let timer = null

async function loadRegistry() {
  registry.value = await GetCommodityRegistry()
}

async function loadQuotes() {
  for (const asset of mainAssets.value) {
    try {
      const q = await GetCommodityQuote(asset.code)
      if (q) {
        quotes.value[asset.code] = q
        errors.value[asset.code] = ''
      }
    } catch (e) {
      console.error('quote error', asset.code, e)
      errors.value[asset.code] = e.message || e
    }
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

onMounted(() => {
  loadRegistry()
  loadQuotes()
  timer = setInterval(loadQuotes, 5000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <n-space vertical>
    <n-grid :cols="4" :x-gap="12" :y-gap="12">
      <n-grid-item v-for="asset in mainAssets" :key="asset.code">
        <n-card
            size="small"
            :class="{'border-blue-500': selectedCode === asset.code}"
            hoverable
            @click="selectAsset(asset.code, asset.name)"
        >
          <n-space justify="space-between" align="center">
            <div>
              <n-text depth="3">{{ asset.name }}</n-text>
              <div class="text-lg font-bold">{{ formatPrice(quotes[asset.code]?.Price) }}</div>
              <div v-if="errors[asset.code]" class="text-xs text-red-500">{{ errors[asset.code] }}</div>
            </div>
            <div :class="quotes[asset.code]?.ChangePct >= 0 ? 'text-red-500' : 'text-green-500'">
              {{ formatPct(quotes[asset.code]?.ChangePct) }}
            </div>
          </n-space>
        </n-card>
      </n-grid-item>
    </n-grid>

    <n-card :title="selectedName + ' K线图'" size="small">
      <CommodityKlineChart :code="selectedCode" :name="selectedName" period="day"/>
    </n-card>
  </n-space>
</template>
