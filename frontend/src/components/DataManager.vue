<script setup>
import { h, ref, onMounted, computed } from 'vue'
import {
  GetKLineCacheStats, StartHistoricalSync, GetSyncProgress,
  GetSeedImportStatus, RunSeedImport, GetLastSeedImportOutput,
} from '../../wailsjs/go/backtest/Service'
import {
  GetAllStockInfoList, BatchDeleteAllStockInfo,
} from '../../wailsjs/go/main/App'
import {
  NAlert, NButton, NCard, NDataTable, NDivider,
  NGi, NGrid, NSpin, NStatistic, NTag, NText, NFlex,
  NInput, NInputNumber, NPopconfirm, NIcon, NEmpty, NCode,
  NCollapse, NCollapseItem,
} from 'naive-ui'
import { useMessage } from 'naive-ui'
import { SearchOutline, TrashOutline } from '@vicons/ionicons5'

const message = useMessage()

// ── Cache Stats ──
const statsLoading = ref(false)
const stats = ref(null)
const statsError = ref('')

const displayStats = computed(() => {
  const s = stats.value
  if (!s) return null
  const dr = s.dateRange || {}
  return {
    totalBars: s.totalBars ?? 0,
    uniqueStocks: s.uniqueStocks ?? 0,
    dateRange: dr.min && dr.max ? `${dr.min} ~ ${dr.max}` : '-',
    lastSync: s.lastSyncTime || '-',
  }
})

const metricCards = computed(() => {
  const d = displayStats.value
  if (!d) return []
  return [
    { label: 'K线总条数', value: d.totalBars, suffix: '' },
    { label: '覆盖股票数', value: d.uniqueStocks, suffix: '只' },
    { label: '数据日期范围', value: d.dateRange, suffix: '' },
    { label: '最近同步时间', value: d.lastSync, suffix: '' },
  ]
})

async function loadStats() {
  statsLoading.value = true
  statsError.value = ''
  try {
    stats.value = await GetKLineCacheStats()
  } catch (e) {
    statsError.value = String(e)
  } finally {
    statsLoading.value = false
  }
}

// ── Sync Controls ──
const syncLoading = ref(false)
const progressLoading = ref(false)
const progressItems = ref([])

const progressColumns = [
  { title: '股票代码', key: 'stockCode', width: 110, ellipsis: true },
  { title: '周期', key: 'period', width: 70 },
  {
    title: '状态', key: 'status', width: 90,
    render(row) {
      const type = row.status === 'done' ? 'success' : row.status === 'error' ? 'error' : 'info'
      return h(NTag, { type, bordered: false, size: 'small' }, { default: () => row.status })
    },
  },
  { title: '进度', key: 'progress', width: 80 },
  { title: '错误信息', key: 'error', ellipsis: { tooltip: true } },
]

async function startSync() {
  syncLoading.value = true
  try {
    await StartHistoricalSync(5)
    message.success('历史数据同步已启动，稍后刷新进度查看')
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

// ── Stock Browser ──
const stockSearch = ref('')
const stockList = ref([])
const stockLoading = ref(false)
const stockError = ref('')
const selectedStockIds = ref([])

const stockColumns = [
  { type: 'selection', width: 40 },
  { title: '代码', key: 'ts_code', width: 120 },
  { title: '名称', key: 'name', width: 130 },
  { title: '市场', key: 'market', width: 70 },
  { title: '上市日期', key: 'list_date', width: 100 },
]

async function searchStocks() {
  if (!stockSearch.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }
  stockLoading.value = true
  stockError.value = ''
  try {
    const res = await GetAllStockInfoList({ keyword: stockSearch.value, page: 1, pageSize: 50 })
    stockList.value = res?.list || []
  } catch (e) {
    stockError.value = String(e)
  } finally {
    stockLoading.value = false
  }
}

async function batchDeleteStocks() {
  if (selectedStockIds.value.length === 0) {
    message.warning('请先选择要删除的股票')
    return
  }
  try {
    await BatchDeleteAllStockInfo(selectedStockIds.value)
    message.success(`已删除 ${selectedStockIds.value.length} 只股票信息`)
    selectedStockIds.value = []
    await searchStocks()
  } catch (e) {
    message.error('删除失败: ' + String(e))
  }
}

// ── Seed Import ──
const seedStatus = ref(null)
const seedStatusLoading = ref(false)
const seedImportRunning = ref(false)
const seedImportOutput = ref('')
const seedForm = ref({
  startDate: '',
  endDate: '',
  limit: 0,
})

async function loadSeedStatus() {
  seedStatusLoading.value = true
  try {
    seedStatus.value = await GetSeedImportStatus()
  } catch (e) {
    message.error('获取种子导入状态失败: ' + String(e))
  } finally {
    seedStatusLoading.value = false
  }
}

async function runSeedImport() {
  seedImportRunning.value = true
  seedImportOutput.value = ''
  try {
    const output = await RunSeedImport(
      '',
      seedForm.value.startDate,
      seedForm.value.endDate,
      seedForm.value.limit,
    )
    seedImportOutput.value = output
    message.success('种子导入完成')
    await loadSeedStatus()
  } catch (e) {
    seedImportOutput.value = String(e)
    message.error('种子导入失败')
  } finally {
    seedImportRunning.value = false
  }
}

async function refreshSeedOutput() {
  try {
    seedImportOutput.value = await GetLastSeedImportOutput()
  } catch (e) {
    message.error('获取上次输出失败: ' + String(e))
  }
}

// ── Lifecycle ──
onMounted(() => {
  loadStats()
  refreshProgress()
  loadSeedStatus()
})
</script>

<template>
  <div class="data-manager">
    <!-- 缓存统计 -->
    <n-grid :cols="24" :x-gap="16" :y-gap="16">
      <n-gi :span="24">
        <n-card title="K线缓存统计" size="small">
          <n-spin :show="statsLoading">
            <template v-if="statsError">
              <n-alert type="error" closable @close="statsError = ''">
                <template #header>加载失败</template>
                {{ statsError }}
              </n-alert>
            </template>
            <template v-else-if="displayStats">
              <n-grid :cols="4" :x-gap="16" :y-gap="12">
                <n-gi v-for="m in metricCards" :key="m.label">
                  <n-statistic :label="m.label" :tabular-nums="true">
                    {{ m.value }}
                    <template v-if="m.suffix" #suffix>
                      <n-text depth="3" style="font-size: 14px">{{ m.suffix }}</n-text>
                    </template>
                  </n-statistic>
                </n-gi>
              </n-grid>
            </template>
            <template v-else>
              <n-empty description="暂无数据" style="padding: 20px" />
            </template>
          </n-spin>
        </n-card>
      </n-gi>
    </n-grid>

    <!-- 同步控制 -->
    <n-card title="历史数据同步" size="small" style="margin-top: 16px">
      <n-flex align="center" :size="12" style="margin-bottom: 12px">
        <n-button type="primary" @click="startSync" :loading="syncLoading">
          启动5年历史数据同步
        </n-button>
        <n-button @click="refreshProgress" :loading="progressLoading">
          刷新进度
        </n-button>
      </n-flex>
      <n-data-table
        :columns="progressColumns"
        :data="progressItems"
        :bordered="true"
        :single-line="false"
        size="small"
        :loading="progressLoading"
        :max-height="320"
      />
      <n-text v-if="!progressLoading && progressItems.length === 0" depth="3" style="display: block; text-align: center; padding: 24px 0">
        暂无同步任务
      </n-text>
    </n-card>

    <n-divider />

    <!-- 个股数据浏览 -->
    <n-card title="个股数据浏览" size="small">
      <n-flex align="center" :size="8" style="margin-bottom: 12px">
        <n-input
          v-model:value="stockSearch"
          placeholder="输入股票代码或名称搜索..."
          clearable
          style="width: 280px"
          @keyup.enter="searchStocks"
        >
          <template #prefix>
            <n-icon :component="SearchOutline" />
          </template>
        </n-input>
        <n-button type="primary" size="small" @click="searchStocks" :loading="stockLoading">搜索</n-button>
        <n-popconfirm
          v-if="selectedStockIds.length > 0"
          @positive-click="batchDeleteStocks"
        >
          <template #trigger>
            <n-button size="small" type="error">
              <template #icon>
                <n-icon :component="TrashOutline" />
              </template>
              删除选中 ({{ selectedStockIds.length }})
            </n-button>
          </template>
          确定要删除选中的 {{ selectedStockIds.length }} 只股票信息吗？
        </n-popconfirm>
      </n-flex>
      <n-alert v-if="stockError" type="error" closable :bordered="false" style="margin-bottom: 8px">
        {{ stockError }}
      </n-alert>
      <n-data-table
        :columns="stockColumns"
        :data="stockList"
        :loading="stockLoading"
        :bordered="true"
        :single-line="false"
        size="small"
        :max-height="400"
        @update:checked-row-keys="selectedStockIds = $event"
        row-key="id"
      />
      <n-text v-if="!stockLoading && stockList.length === 0 && !stockError" depth="3" style="display: block; text-align: center; padding: 24px 0">
        输入关键词搜索个股数据
      </n-text>
    </n-card>

    <!-- 本地种子导入 -->
    <n-card title="本地种子数据导入" size="small" style="margin-top: 16px">
      <n-spin :show="seedStatusLoading">
        <template v-if="seedStatus">
          <n-grid :cols="2" :x-gap="16" :y-gap="12">
            <!-- 种子数据统计 -->
            <n-gi>
              <n-statistic label="种子 K 线条数" :tabular-nums="true">
                {{ seedStatus.seedBars ?? 0 }}
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="种子覆盖股票" :tabular-nums="true">
                {{ seedStatus.seedStocks ?? 0 }}
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="数据库路径">
                <n-text depth="3" style="font-size: 13px">{{ seedStatus.dbPath || '-' }}</n-text>
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="Python 环境">
                <n-tag v-if="seedStatus.pythonFound" type="success" bordered size="small">已检测</n-tag>
                <n-tag v-else type="error" bordered size="small">未找到</n-tag>
                <n-text v-if="seedStatus.pythonFound" depth="3" style="font-size: 12px; margin-left: 6px">
                  {{ seedStatus.pythonPath }}
                </n-text>
              </n-statistic>
            </n-gi>
            <n-gi :span="2">
              <n-statistic label="种子数据状态">
                <n-tag v-if="seedStatus.hasSeedData" type="success" bordered size="small">已导入</n-tag>
                <n-tag v-else type="warning" bordered size="small">未导入</n-tag>
              </n-statistic>
            </n-gi>
            <n-gi :span="2">
              <n-text depth="3" style="font-size: 13px">
                脚本路径：<n-code :code="seedStatus.scriptPath || '未找到'" language="bash" />
              </n-text>
            </n-gi>
          </n-grid>
        </template>
      </n-spin>

      <n-divider />

      <n-flex align="center" :size="8" style="margin-bottom: 12px">
        <n-input
          v-model:value="seedForm.startDate"
          placeholder="开始日期 YYYYMMDD (默认 20100101)"
          clearable
          style="width: 200px"
        />
        <n-input
          v-model:value="seedForm.endDate"
          placeholder="结束日期 YYYYMMDD (默认昨日)"
          clearable
          style="width: 200px"
        />
        <n-input-number
          v-model:value="seedForm.limit"
          :min="0"
          :max="10000"
          placeholder="限制股票数 (0=全部)"
          style="width: 160px"
          clearable
        />
        <n-button
          type="primary"
          @click="runSeedImport"
          :loading="seedImportRunning"
          :disabled="seedStatusLoading"
        >
          执行种子导入
        </n-button>
        <n-button @click="loadSeedStatus" :loading="seedStatusLoading">
          刷新状态
        </n-button>
      </n-flex>

      <n-collapse>
        <n-collapse-item title="查看导入日志" name="output">
          <n-code
            v-if="seedImportOutput"
            :code="seedImportOutput"
            language="bash"
            style="display: block; max-height: 300px; overflow: auto; white-space: pre-wrap; font-size: 12px; padding: 8px; background: #f5f5f5; border-radius: 4px;"
          />
          <n-text v-else depth="3">
            暂无导入日志，执行导入后在此查看输出
          </n-text>
        </n-collapse-item>
      </n-collapse>
    </n-card>

    <n-card title="数据指引" size="small" style="margin-top: 16px">
      <n-flex vertical :size="8">
        <n-text depth="3">
          • K线数据通过东方财富等数据源实时获取后自动缓存到本地数据库，无需手动导入
        </n-text>
        <n-text depth="3">
          • 如需批量补齐历史K线数据，可使用「历史数据同步」功能，系统会自动检测缺失区间并补充
        </n-text>
        <n-text depth="3">
          • 外部种子脚本：<n-code code="python scripts/history_seed/baostock_seed.py --db-path <data-dir>/stock.db" language="bash" />
        </n-text>
        <n-text depth="3">
          • 数据文件位于应用数据目录下的 <n-code code="stock.db" language="bash" />，请勿手动修改
        </n-text>
      </n-flex>
    </n-card>
  </div>
</template>

<style scoped>
.data-manager {
  padding: 8px;
  --wails-draggable: no-drag;
}
</style>
