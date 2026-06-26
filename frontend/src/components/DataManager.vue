<script setup>
import { ref, reactive, onMounted } from 'vue'
import { GetKLineCacheStats, StartHistoricalSync, GetSyncProgress } from '../../wailsjs/go/backtest/Service'
import {
  NAlert, NButton, NCard, NCode, NDataTable, NDivider,
  NGi, NGrid, NSpin, NStatistic, NTag, NText
} from 'naive-ui'
import { useMessage } from 'naive-ui'

const message = useMessage()

const statsLoading = ref(false)
const stats = ref(null)
const statsError = ref('')

const syncLoading = ref(false)
const progressLoading = ref(false)
const progressItems = ref([])

const progressColumns = [
  { title: '股票代码', key: 'stockCode', width: 120 },
  { title: '周期', key: 'period', width: 80 },
  { title: '状态', key: 'status', width: 100,
    render(row) {
      const type = row.status === 'done' ? 'success' : row.status === 'error' ? 'error' : 'info'
      return h(NTag, { type, bordered: false }, { default: () => row.status })
    }
  },
  { title: '错误信息', key: 'error', ellipsis: { tooltip: true } },
]

async function loadStats() {
  statsLoading.value = true
  statsError.value = ''
  try {
    stats.value = await GetKLineCacheStats()
  } catch (e) {
    statsError.value = String(e)
    message.error('获取缓存统计失败: ' + String(e))
  } finally {
    statsLoading.value = false
  }
}

async function startSync() {
  syncLoading.value = true
  try {
    await StartHistoricalSync(5)
    message.success('历史数据同步已启动')
  } catch (e) {
    message.error('启动同步失败: ' + String(e))
  } finally {
    syncLoading.value = false
  }
}

async function refreshProgress() {
  progressLoading.value = true
  try {
    progressItems.value = await GetSyncProgress()
  } catch (e) {
    message.error('获取进度失败: ' + String(e))
  } finally {
    progressLoading.value = false
  }
}

onMounted(() => {
  loadStats()
})
</script>

<template>
  <div class="data-manager">
    <n-grid :cols="24" :x-gap="16" :y-gap="16">
      <n-gi :span="8">
        <n-card title="K-Line Cache Stats" size="small">
          <n-spin :show="statsLoading">
            <template v-if="statsError">
              <n-alert type="error" closable @close="statsError = ''">
                <template #header>加载失败</template>
                {{ statsError }}
              </n-alert>
            </template>
            <template v-else-if="stats">
              <n-grid :cols="2" :x-gap="12" :y-gap="12">
                <n-gi>
                  <n-statistic label="Total Bars" :value="stats.TotalBars" />
                </n-gi>
                <n-gi>
                  <n-statistic label="Unique Stocks" :value="stats.UniqueStocks" />
                </n-gi>
                <n-gi>
                  <n-statistic label="Date Range" :value="stats.DateRange || '-'" />
                </n-gi>
                <n-gi>
                  <n-statistic label="Last Sync" :value="stats.LastSyncTime || '-'" />
                </n-gi>
              </n-grid>
            </template>
            <template v-else>
              <div style="padding: 20px 0; text-align: center">
                <n-text depth="3">Loading...</n-text>
              </div>
            </template>
          </n-spin>
        </n-card>
      </n-gi>

      <n-gi :span="16">
        <n-card title="Sync Controls" size="small">
          <div style="margin-bottom: 12px; display: flex; gap: 12px">
            <n-button type="primary" @click="startSync" :loading="syncLoading">
              启动历史数据同步
            </n-button>
            <n-button @click="refreshProgress" :loading="progressLoading">
              刷新进度
            </n-button>
          </div>
          <n-data-table
            :columns="progressColumns"
            :data="progressItems"
            :bordered="true"
            :single-line="false"
            size="small"
            :loading="progressLoading"
          />
        </n-card>
      </n-gi>
    </n-grid>

    <n-divider />

    <n-card title="Info" size="small">
      <n-text depth="3">
        Run the Baostock seed script to populate K-line data:
      </n-text>
      <br><br>
      <n-code :code="`python scripts/history_seed/baostock_seed.py --db-path <path>`" language="bash" />
      <br><br>
      <n-alert type="info" :bordered="false">
        Tip: go-stock must be run first to initialize the database before executing the seed script.
      </n-alert>
    </n-card>
  </div>
</template>

<style scoped>
.data-manager {
  padding: 8px;
  --wails-draggable: no-drag;
}
</style>
