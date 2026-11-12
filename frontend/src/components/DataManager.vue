<script setup>
import { h, ref, onMounted, onUnmounted, computed } from 'vue'
import {
  getKLineCacheStats, getKLineHistory, startHistoricalSync, getSyncProgress,
  getSeedImportStatus, runSeedImport as runSeedImportApi, getLastSeedImportOutput,
} from '../api/backtest'
import { VacuumDatabase } from '../../wailsjs/go/handler/SystemHandler'
import {
  NAlert, NButton, NCard, NDataTable, NDivider,
  NGi, NGrid, NSpin, NStatistic, NTag, NText, NFlex,
  NInput, NInputNumber, NPopconfirm, NIcon, NEmpty, NCode,
  NCollapse, NCollapseItem, NPagination, NProgress, NDatePicker,
} from 'naive-ui'
import { useMessage } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'

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
    stats.value = await getKLineCacheStats()
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
const progressSummary = ref({ pending: 0, running: 0, done: 0, failed: 0, partial: 0 })
const hiddenSyncTasks = ref(0)
let progressTimer = null
let disposed = false

const hasActiveSync = computed(() => progressSummary.value.pending + progressSummary.value.running > 0)
const syncSummaryMetrics = computed(() => [
  { label: '待同步', key: 'pending' },
  { label: '进行中', key: 'running' },
  { label: '已完成', key: 'done' },
  { label: '失败', key: 'failed' },
  { label: '部分完成', key: 'partial' },
])

const syncStatusMeta = {
  pending: { label: '待同步', type: 'default' },
  running: { label: '进行中', type: 'info' },
  done: { label: '已完成', type: 'success' },
  failed: { label: '失败', type: 'error' },
  partial: { label: '部分完成', type: 'warning' },
  error: { label: '异常', type: 'error' },
}

const progressColumns = [
  { title: '股票代码', key: 'stockCode', width: 110, ellipsis: true },
  { title: '周期', key: 'period', width: 70, render: (row) => formatPeriod(row.period) },
  {
    title: '状态', key: 'status', width: 100,
    render(row) {
      const meta = syncStatusMeta[row.status] || { label: row.status || '未知', type: 'default' }
      return h(NTag, { type: meta.type, bordered: false, size: 'small' }, { default: () => meta.label })
    },
  },
  {
    title: '进度', key: 'progress', width: 180,
    render(row) {
      const percentage = Math.max(0, Math.min(100, Number(row.progress) || 0))
      const status = row.status === 'done' ? 'success' : row.status === 'failed' || row.status === 'error' ? 'error' : row.status === 'partial' ? 'warning' : 'default'
      return h(NProgress, {
        type: 'line', percentage, status, height: 14,
        indicatorPlacement: 'inside', processing: row.status === 'running',
      })
    },
  },
  { title: '错误信息', key: 'errorMsg', ellipsis: { tooltip: true } },
]

async function startSync() {
  syncLoading.value = true
  try {
    await startHistoricalSync(5)
    message.success('历史数据同步已启动，进度将自动刷新')
    await refreshProgress()
  } catch (e) {
    message.error('启动同步失败: ' + String(e))
  } finally {
    syncLoading.value = false
  }
}

async function refreshProgress(silent = false) {
  if (progressTimer) {
    clearTimeout(progressTimer)
    progressTimer = null
  }
  const resumeAfterError = hasActiveSync.value
  const wasActive = resumeAfterError
  if (!silent) progressLoading.value = true
  try {
    const res = await getSyncProgress()
    progressSummary.value = { pending: 0, running: 0, done: 0, failed: 0, partial: 0, ...res?.summary }
    progressItems.value = res?.tasks || []
    hiddenSyncTasks.value = res?.hiddenTasks || 0
    // Cache stats scans the full K-line table; refresh it only for explicit
    // refreshes or when an active run finishes, never on every 5s poll.
    if (!silent || (wasActive && !hasActiveSync.value)) await loadStats()
  } catch (e) {
    if (!silent) message.error('获取进度失败: ' + String(e))
  } finally {
    if (!silent) progressLoading.value = false
    if (!disposed && (hasActiveSync.value || resumeAfterError)) {
      progressTimer = setTimeout(() => refreshProgress(true), 5000)
    }
  }
}

// ── Historical K-line Browser ──
const historySearch = ref('')
const historyStartDate = ref(dateStr(new Date(Date.now() - 30 * 24 * 60 * 60 * 1000)))
const historyEndDate = ref(dateStr(new Date()))
const historyData = ref(null)
const historyPageSize = ref(20)
const historyLoading = ref(false)
const historyError = ref('')

function dateStr(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatPeriod(period) {
  return { day: '日线', '101': '日线', week: '周线', month: '月线' }[period] || period || '-'
}

function formatPrice(value) {
  const number = Number(value)
  return Number.isFinite(number) ? number.toFixed(2) : '-'
}

function formatNumber(value, maximumFractionDigits = 0) {
  const number = Number(value)
  return Number.isFinite(number)
    ? number.toLocaleString('zh-CN', { maximumFractionDigits })
    : '-'
}

const historyColumns = [
  { title: '股票代码', key: 'stockCode', width: 110, fixed: 'left' },
  { title: '股票名称', key: 'stockName', width: 110, fixed: 'left', render: (row) => row.stockName || '-' },
  { title: '交易日期', key: 'tradeDate', width: 105 },
  { title: '周期', key: 'period', width: 70, render: (row) => formatPeriod(row.period) },
  { title: '复权', key: 'adjusted', width: 65, render: (row) => row.adjusted ? '是' : '否' },
  { title: '开盘', key: 'open', width: 90, render: (row) => formatPrice(row.open) },
  { title: '最高', key: 'high', width: 90, render: (row) => formatPrice(row.high) },
  { title: '最低', key: 'low', width: 90, render: (row) => formatPrice(row.low) },
  { title: '收盘', key: 'close', width: 90, render: (row) => formatPrice(row.close) },
  { title: '成交量', key: 'volume', width: 120, render: (row) => formatNumber(row.volume) },
  { title: '成交额', key: 'amount', width: 140, render: (row) => formatNumber(row.amount, 2) },
]

async function loadHistory(page = 1) {
  historyLoading.value = true
  historyError.value = ''
  try {
    historyData.value = await getKLineHistory(
      historySearch.value.trim(),
      historyStartDate.value || '',
      historyEndDate.value || '',
      page,
      historyPageSize.value,
    )
  } catch (e) {
    historyError.value = String(e)
  } finally {
    historyLoading.value = false
  }
}

function changeHistoryPageSize(size) {
  historyPageSize.value = size
  loadHistory(1)
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
    seedStatus.value = await getSeedImportStatus()
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
    const output = await runSeedImportApi(
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
    seedImportOutput.value = await getLastSeedImportOutput()
  } catch (e) {
    message.error('获取上次输出失败: ' + String(e))
  }
}

// ── 数据库维护 ──
const vacuumLoading = ref(false)
const vacuumResult = ref('')

async function runVacuum() {
  vacuumLoading.value = true
  vacuumResult.value = ''
  try {
    vacuumResult.value = await VacuumDatabase()
    if (vacuumResult.value.includes('完成')) {
      message.success(vacuumResult.value)
    } else {
      message.error(vacuumResult.value)
    }
  } catch (e) {
    message.error('数据库压缩失败: ' + String(e))
  } finally {
    vacuumLoading.value = false
  }
}

// ── Lifecycle ──
onMounted(() => {
  refreshProgress()
  loadSeedStatus()
  loadHistory()
})

onUnmounted(() => {
  disposed = true
  if (progressTimer) clearTimeout(progressTimer)
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
        <n-button @click="refreshProgress()" :loading="progressLoading">
          刷新进度
        </n-button>
        <n-text v-if="hasActiveSync" depth="3" style="font-size: 12px">每 5 秒自动刷新</n-text>
      </n-flex>
      <n-grid :cols="5" :x-gap="12" :y-gap="12" style="margin-bottom: 16px">
        <n-gi v-for="metric in syncSummaryMetrics" :key="metric.key">
          <n-statistic :label="metric.label" :tabular-nums="true">
            {{ progressSummary[metric.key] || 0 }}
          </n-statistic>
        </n-gi>
      </n-grid>
      <n-text v-if="hiddenSyncTasks > 0" depth="3" style="display: block; margin-bottom: 8px; font-size: 12px">
        当前仅展示最近更新的 {{ progressItems.length }} 条任务，另有 {{ hiddenSyncTasks }} 条未完成任务。
      </n-text>
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

    <!-- 数据库维护 -->
    <n-card title="数据库维护" size="small" style="margin-top: 16px">
      <n-flex align="center" :size="12">
        <n-popconfirm @positive-click="runVacuum">
          <template #trigger>
            <n-button type="warning" :loading="vacuumLoading">
              压缩数据库(VACUUM)
            </n-button>
          </template>
          VACUUM 全程独占写锁、耗时与库大小成正比，执行期间其它写入会排队等待。请在非交易时段执行，确定继续？
        </n-popconfirm>
        <n-text v-if="vacuumResult" depth="2">{{ vacuumResult }}</n-text>
        <n-text v-else depth="3">回收已删除数据占用的磁盘空间；每天 02:00 自动清理历史大表后，文件体积不会自动缩小，需手动压缩。</n-text>
      </n-flex>
    </n-card>

    <n-divider />

    <!-- 历史 K 线数据 -->
    <n-card title="历史 K 线数据" size="small">
      <n-flex align="center" :size="8" style="margin-bottom: 12px">
        <n-input
          v-model:value="historySearch"
          placeholder="股票代码或名称（留空查看范围）"
          clearable
          style="width: 260px"
          @keyup.enter="loadHistory(1)"
        >
          <template #prefix>
            <n-icon :component="SearchOutline" />
          </template>
        </n-input>
        <n-date-picker
          v-model:formatted-value="historyStartDate"
          type="date"
          value-format="yyyy-MM-dd"
          clearable
          placeholder="开始日期"
          style="width: 145px"
        />
        <n-date-picker
          v-model:formatted-value="historyEndDate"
          type="date"
          value-format="yyyy-MM-dd"
          clearable
          placeholder="结束日期"
          style="width: 145px"
        />
        <n-button type="primary" size="small" @click="loadHistory(1)" :loading="historyLoading">搜索</n-button>
      </n-flex>
      <n-alert v-if="historyError" type="error" closable :bordered="false" style="margin-bottom: 8px" @close="historyError = ''">
        {{ historyError }}
      </n-alert>
      <n-data-table
        :columns="historyColumns"
        :data="historyData?.list || []"
        :loading="historyLoading"
        :bordered="true"
        :single-line="false"
        :scroll-x="1185"
        size="small"
        :max-height="480"
        :row-key="(row) => row.id"
      />
      <n-flex v-if="historyData && historyData.total > 0" justify="flex-end" style="margin-top: 12px">
        <n-pagination
          :page="historyData.page"
          :page-size="historyData.pageSize"
          :item-count="historyData.total"
          :page-slot="7"
          show-size-picker
          :page-sizes="[20, 50, 100]"
          @update:page="loadHistory"
          @update:page-size="changeHistoryPageSize"
        />
      </n-flex>
      <n-text v-if="!historyLoading && (!historyData || historyData.list.length === 0) && !historyError" depth="3" style="display: block; text-align: center; padding: 24px 0">
        暂无匹配的历史 K 线数据
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
