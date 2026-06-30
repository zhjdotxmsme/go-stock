<script setup>
import {ref, onMounted} from 'vue'
import {GetCommodityRegistry, GetCommodityQuote} from '../../wailsjs/go/main/App'

const futures = ref([])
const quotes = ref({})
const loading = ref(false)

async function loadData() {
  loading.value = true
  const registry = await GetCommodityRegistry()
  futures.value = registry.filter(item => item.assetType === 'futures')
  for (const item of futures.value) {
    try {
      const q = await GetCommodityQuote(item.code)
      if (q) quotes.value[item.code] = q
    } catch (e) {
      console.error('futures quote error', item.code, e)
    }
  }
  loading.value = false
}

function formatPrice(p) {
  if (p === undefined || p === null) return '--'
  return Number(p).toFixed(2)
}

function formatPct(p) {
  if (p === undefined || p === null) return '--'
  const t = Number(p).toFixed(2)
  return Number(p) >= 0 ? '+' + t + '%' : t + '%'
}

const columns = [
  {title: '品种', key: 'name'},
  {title: '代码', key: 'code'},
  {title: '最新价', key: 'price'},
  {title: '涨跌幅', key: 'changePct'},
]

onMounted(() => {
  loadData()
})
</script>

<template>
  <n-data-table
      :columns="columns"
      :data="futures"
      :loading="loading"
      size="small"
      :single-line="false"
  >
    <template #default-price="{row}">
      {{ formatPrice(quotes[row.code]?.Price) }}
    </template>
    <template #default-changePct="{row}">
      <span :class="quotes[row.code]?.ChangePct >= 0 ? 'text-red-500' : 'text-green-500'">
        {{ formatPct(quotes[row.code]?.ChangePct) }}
      </span>
    </template>
  </n-data-table>
</template>
