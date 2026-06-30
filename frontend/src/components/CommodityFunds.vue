<script setup>
import {ref, onMounted} from 'vue'
import {GetCommodityRegistry, GetCommodityQuote} from '../../wailsjs/go/main/App'

const funds = ref([])
const quotes = ref({})
const loading = ref(false)

async function loadData() {
  loading.value = true
  const registry = await GetCommodityRegistry()
  funds.value = registry.filter(item => item.assetType === 'etf')
  for (const item of funds.value) {
    try {
      const q = await GetCommodityQuote(item.code)
      if (q) {
        quotes.value[item.code] = q
      }
    } catch (e) {
      console.error('fund quote error', item.code, e)
    }
  }
  loading.value = false
}

function formatPrice(p) {
  if (p === undefined || p === null) return '--'
  return Number(p).toFixed(3)
}

function formatPct(p) {
  if (p === undefined || p === null) return '--'
  const t = Number(p).toFixed(2)
  return Number(p) >= 0 ? '+' + t + '%' : t + '%'
}

const columns = [
  {title: '名称', key: 'name'},
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
      :data="funds"
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
