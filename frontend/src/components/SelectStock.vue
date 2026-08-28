<script setup lang="ts">
import {h, onBeforeMount, onMounted, onUnmounted, ref, reactive, computed} from 'vue'
import * as stockApi from "../api/stock";
import * as marketApi from "../api/market";
import * as systemApi from "../api/system";

import {useMessage, NText, NTag, NButton, NPopconfirm, NCard, NTooltip, NSpace, NEllipsis} from 'naive-ui'
import {Environment} from "../../wailsjs/runtime"
import {BookmarkOutline, TrashOutline, CreateOutline, AddOutline, FlashOutline, TrendingUpOutline, TrendingDownOutline, GitBranchOutline} from "@vicons/ionicons5";
import {EventsEmit} from "../../wailsjs/runtime";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";

interface ComboIndicator {
  name: string
  explanation: string
}

interface StrategyCombo {
  name: string
  description: string
  query: string
  category: string
  indicators: ComboIndicator[]
  icon: any
}

const strategyCombos: StrategyCombo[] = [
  {
    name: '追涨组合',
    description: '捕捉强势上涨趋势，适合顺势交易',
    query: '均线多头排列 放量突破 连涨放量',
    category: '趋势跟踪',
    icon: TrendingUpOutline,
    indicators: [
      { name: '均线多头排列', explanation: '短期均线（5日）在长期均线（20日）上方，且都向上发散，表明处于强势上涨趋势中' },
      { name: '放量突破', explanation: '成交量显著放大且股价突破关键阻力位（均线或前高），资金大量涌入，突破有效性强' },
      { name: '连涨放量', explanation: '连续多日上涨且成交量持续放大，上涨有资金支撑，趋势健康' },
    ],
  },
  {
    name: '抄底组合',
    description: '寻找超跌反弹机会，适合左侧交易',
    query: '低位资金净流入 连跌14天 早晨之星',
    category: '反转交易',
    icon: FlashOutline,
    indicators: [
      { name: '低位资金净流入', explanation: '股价处于阶段低位但资金持续流入，可能有主力在悄悄吸筹' },
      { name: '连跌14天', explanation: '股价连续下跌14个交易日，处于极度超卖状态，反弹概率增大' },
      { name: '早晨之星', explanation: '三根K线组合：大阴线→十字星→大阳线，经典见底反转信号' },
    ],
  },
  {
    name: '逃顶组合',
    description: '识别见顶信号及时离场，适合风险控制',
    query: '高位资金净流出 乌云盖顶 人气排名下降',
    category: '风险管理',
    icon: TrendingDownOutline,
    indicators: [
      { name: '高位资金净流出', explanation: '股价处于阶段高位但资金持续流出，主力可能在出货，需要警惕' },
      { name: '乌云盖顶', explanation: '大阳线后出现高开低走的大阴线，经典的见顶反转K线形态' },
      { name: '人气排名下降', explanation: '市场关注度/热度在下降，说明资金正在撤离该股' },
    ],
  },
  {
    name: '金叉组合',
    description: '捕捉技术指标金叉信号，适合短线交易',
    query: 'MACD金叉 KDJ金叉',
    category: '短线信号',
    icon: GitBranchOutline,
    indicators: [
      { name: 'MACD金叉', explanation: '快线（DIF）上穿慢线（DEA），短期趋势转强，是常见的买入信号' },
      { name: 'KDJ金叉', explanation: 'K线上穿D线，尤其在超卖区（20以下）金叉时反弹信号更强' },
    ],
  },
  {
    name: '放量强稳组合',
    description: '放量上涨且趋势稳健，适合稳健型投资者',
    query: '放量上攻 均线多头排列 强势多方炮',
    category: '稳健投资',
    icon: FlashOutline,
    indicators: [
      { name: '放量上攻', explanation: '成交量放大伴随股价上涨，上涨动能充足，是真金白银推动的上涨' },
      { name: '均线多头排列', explanation: '短期均线在长期均线上方且向上发散，整体处于强势上涨趋势' },
      { name: '强势多方炮', explanation: '两阳夹一阴且阳线实体大、阴线实体小，是上涨中继的强势信号' },
    ],
  },
  {
    name: '超跌反弹组合',
    description: '连续下跌后的反弹机会，适合激进型投资者',
    query: '连跌8天 曙光初现 低位资金净流入',
    category: '反转交易',
    icon: FlashOutline,
    indicators: [
      { name: '连跌8天', explanation: '连续8个交易日下跌，处于明显超卖状态' },
      { name: '曙光初现', explanation: '大跌后出现大阳线且深入前一根阴线实体，是见底反转信号' },
      { name: '低位资金净流入', explanation: '低位有资金持续买入，说明有资金认可当前价位' },
    ],
  },
]

const message = useMessage()
const search = ref('')
const columns = ref([])
const dataList = ref([])
const hotStrategy = ref([])
const customStrategies = ref([])
const traceInfo = ref('')
const tableScrollX = ref(2800)
const leftTab = ref('hot')
const showSaveModal = ref(false)
const darkTheme = ref(false)
const klineModalShow = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
let klineAutoCloseTimer = null
const useAIConfig = ref(false)
const aiLoading = ref(false)
const batchFollowing = ref(false)
const batchFollowProgress = ref('')

function displayAIPickResult(picks) {
  if (!picks || picks.length === 0) {
    traceInfo.value = 'AI 配置选股无符合条件的结果'
    columns.value = []
    dataList.value = []
    return
  }
  columns.value = [
    {title: '排名', key: 'rank', width: 60},
    {title: '股票代码', key: 'stockCode', width: 100},
    {title: '股票名称', key: 'stockName', width: 120, ellipsis: {tooltip: true}},
    {title: '评分', key: 'score', width: 80, sorter: (a, b) => a.score - b.score},
    {title: '策略', key: 'strategyName', width: 100},
    {title: '得分原因', key: 'reason', width: 400, ellipsis: {tooltip: true}},
  ]
  dataList.value = picks.map((p, i) => ({
    rank: i + 1,
    stockCode: p.StockCode,
    stockName: p.StockName,
    score: p.Score,
    strategyName: p.StrategyName,
    reason: p.Reason,
    SECURITY_CODE: p.StockCode,
    SECURITY_SHORT_NAME: p.StockName,
  }))
  traceInfo.value = `AI 配置选股结果（共 ${picks.length} 只）`
}

const saveForm = reactive({
  id: 0,
  name: '',
  query: '',
  description: '',
  sortOrder: 0,
})

const paginationProps = computed(() => ({
  pageSize: 10,
  prefix: ({itemCount}) => h('span', {style: 'margin-right: 8px'}, [
    '共找到 ',
    h(NTag, {type: 'info', bordered: false, size: 'small'}, {default: () => itemCount}),
    ' 只股',
  ]),
}))

function calculateTableWidth(cols) {
  let totalWidth = 0;
  cols.forEach(col => {
    if (col.children && col.children.length > 0) {
      let childrenWidth = 0;
      col.children.forEach(child => {
        childrenWidth += child.width || child.minWidth || 100;
      });
      totalWidth += Math.max(col.width || col.minWidth || 200, childrenWidth);
    } else {
      totalWidth += col.width || col.minWidth || 120;
    }
  });
  totalWidth += 100;
  return Math.max(totalWidth, 1200);
}

async function Search() {
  if (!search.value) {
    message.warning('请输入选股指标或者要求')
    return
  }

  if (useAIConfig.value) {
    aiLoading.value = true
    try {
      const res = (await stockApi.aiConfiguredStockPick(search.value, 10)).data
      displayAIPickResult(res)
    } catch (e) {
      message.error('AI 配置选股失败: ' + e)
    } finally {
      aiLoading.value = false
    }
    return
  }

  const loading = message.loading("正在获取选股数据...", {duration: 0});
  stockApi.searchStock(search.value).then(({data: res}) => {
    loading.destroy()
    if (res.code == 100) {
      traceInfo.value = res.data.traceInfo.showText
      columns.value = res.data.result.columns.filter(item => !item.hiddenNeed && (item.title != "市场码" && item.title != "市场简称")).map(item => {
        if (item.children) {
          return {
            title: item.title + (item.unit ? '[' + item.unit + ']' : ''),
            key: item.key,
            resizable: true,
            minWidth: 200,
            ellipsis: {tooltip: true},
            children: item.children.filter(item => !item.hiddenNeed).map(item => {
              return {
                title: item.dateMsg,
                key: item.key,
                minWidth: 100,
                resizable: true,
                ellipsis: {tooltip: true},
                sorter: (row1, row2) => {
                  if (isNumeric(row1[item.key]) && isNumeric(row2[item.key])) {
                    return row1[item.key] - row2[item.key];
                  } else {
                    return 'default'
                  }
                },
              }
            })
          }
        } else {
          return {
            title: item.title + (item.unit ? '[' + item.unit + ']' : ''),
            key: item.key,
            resizable: true,
            minWidth: 120,
            ellipsis: {tooltip: true},
            sorter: (row1, row2) => {
              if (isNumeric(row1[item.key]) && isNumeric(row2[item.key])) {
                return row1[item.key] - row2[item.key];
              } else {
                return 'default'
              }
            },
          }
        }
      })
      columns.value.push({
        title: '操作',
        key: 'actions',
        width: 130,
        fixed: 'right',
        render: (row) => {
          return h('div', {style: 'display:flex;gap:4px;'}, [
            h(
              NButton,
              {
                size: 'tiny',
                type: 'info',
                onClick: () => showStockKline(row)
              },
              {default: () => 'K线'}
            ),
            h(
              NButton,
              {
                strong: true,
                tertiary: true,
                size: 'small',
                type: 'warning',
                style: 'font-size: 14px; padding: 0 10px;',
                onClick: () => handleFollow(row)
              },
              {default: () => '关注'}
            )
          ])
        }
      });
      dataList.value = res.data.result.dataList
      tableScrollX.value = calculateTableWidth(columns.value);
    } else {
      if (res.msg) {
        message.error(res.msg)
      }
      if (res.message) {
        message.error(res.message)
      }
    }
  }).catch(err => {
    message.error(err)
  })
}

function toEastMoneyCode(stockCode, marketShortName) {
  const m = (marketShortName || '').toUpperCase()
  if (m === 'SH' || m === 'SZ' || m === 'BJ') return stockCode + '.' + m
  if (m === 'HK') return stockCode + '.HK'
  if (m === 'US') return stockCode + '.US'
  if (/^(6|5)/.test(stockCode)) return stockCode + '.SH'
  if (/^(8|4)/.test(stockCode)) return stockCode + '.BJ'
  // 纯字母代码视为美股（如 AAPL）
  if (/^[a-zA-Z]+$/.test(stockCode)) return stockCode.toUpperCase() + '.US'
  return stockCode + '.SZ'
}

function showStockKline(row) {
  const stockCode = row.SECURITY_CODE
  const stockName = row.SECURITY_SHORT_NAME
  const em = toEastMoneyCode(stockCode, row.MARKET_SHORT_NAME)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  klineStockCode.value = em
  klineStockName.value = stockName || ''
  klineModalShow.value = true
  if (klineAutoCloseTimer) clearTimeout(klineAutoCloseTimer)
}

function handleFollow(row) {
  let code = row.MARKET_SHORT_NAME.toLowerCase() + row.SECURITY_CODE
  stockApi.follow(code).then(({data: result}) => {
    if (result === "关注成功") {
      message.success(result)
    } else {
      message.error(result)
    }
  });
}

async function batchFollow() {
  const stocks = dataList.value
  if (!stocks || stocks.length === 0) {
    message.warning('没有可关注的股票')
    return
  }
  batchFollowing.value = true
  let success = 0
  let fail = 0
  for (let i = 0; i < stocks.length; i++) {
    const row = stocks[i]
    const code = (row.SECURITY_CODE || row.stockCode)
    const market = (row.MARKET_SHORT_NAME || 'SZ').toLowerCase()
    batchFollowProgress.value = `正在关注 (${i+1}/${stocks.length}): ${row.SECURITY_SHORT_NAME || row.stockName}`
    try {
      const result = (await stockApi.follow(market + code)).data
      if (result === "关注成功") {
        success++
      } else {
        fail++
      }
    } catch {
      fail++
    }
  }
  batchFollowing.value = false
  batchFollowProgress.value = ''
  message.success(`批量关注完成：成功 ${success} 只，失败 ${fail} 只`)
}

function onComboClick(combo: StrategyCombo) {
  search.value = combo.query
  Search()
}

function isNumeric(value) {
  return !isNaN(parseFloat(value)) && isFinite(value);
}

onBeforeMount(() => {
  systemApi.getConfig().then(({data: result}) => {
    if (result.darkTheme) darkTheme.value = true
  })
  marketApi.getHotStrategy().then(({data: res}) => {
    if (res.code == 1) {
      hotStrategy.value = res.data
      search.value = hotStrategy.value[0].question
      Search()
    }
  }).catch(err => {
    message.error(err)
  })
  loadCustomStrategies()
})

function loadCustomStrategies() {
  stockApi.getAllCustomStrategies().then(({data: res}) => {
    customStrategies.value = res || []
  }).catch(err => {
    message.error(err)
  })
}

function DoSearch(question) {
  search.value = question
  Search()
}

function openSaveModal(isEdit = false, strategy = null) {
  if (isEdit && strategy) {
    saveForm.id = strategy.id
    saveForm.name = strategy.name
    saveForm.query = strategy.query
    saveForm.description = strategy.description || ''
    saveForm.sortOrder = strategy.sortOrder || 0
  } else {
    saveForm.id = 0
    saveForm.name = ''
    saveForm.query = search.value
    saveForm.description = ''
    saveForm.sortOrder = 0
  }
  showSaveModal.value = true
}

function handleSaveStrategy() {
  if (!saveForm.name.trim()) {
    message.warning('请输入策略名称')
    return
  }
  if (!saveForm.query.trim()) {
    message.warning('请输入选股条件')
    return
  }
  stockApi.saveCustomStrategy({
    id: saveForm.id || 0,
    name: saveForm.name,
    query: saveForm.query,
    description: saveForm.description,
    sortOrder: saveForm.sortOrder,
  }).then(res => {
    message.success(res)
    showSaveModal.value = false
    loadCustomStrategies()
  }).catch(err => {
    message.error(err)
  })
}

function handleDeleteStrategy(id) {
  stockApi.deleteCustomStrategy(id).then(({data: res}) => {
    message.success(res)
    loadCustomStrategies()
  }).catch(err => {
    message.error(err)
  })
}

function openCenteredWindow(url, width, height) {
  const left = (window.screen.width - width) / 2;
  const top = (window.screen.height - height) / 2;
  Environment().then(env => {
    switch (env.platform) {
      case 'windows':
        window.open(
            url,
            'centeredWindow',
            `width=${width},height=${height},left=${left},top=${top},location=no,menubar=no,toolbar=no,display=standalone`
        )
        break
      default:
        systemApi.openURL(url)
    }
  })
}
</script>

<template>
  <n-grid :cols="24" style="max-height: calc(100vh - 165px)">
    <n-gi :span="4">
      <n-tabs v-model:value="leftTab" type="segment" size="small" style="margin-bottom: 4px;">
        <n-tab name="hot">热门策略</n-tab>
        <n-tab name="combo">推荐组合</n-tab>
        <n-tab name="custom">我的策略</n-tab>
      </n-tabs>

      <n-list bordered style="text-align: left;" hoverable clickable v-show="leftTab==='hot'">
        <n-scrollbar style="max-height: calc(100vh - 210px);">
          <n-list-item v-for="item in hotStrategy" :key="item.rank" @click="DoSearch(item.question)">
            <n-ellipsis line-clamp="1" :tooltip="true">
              <n-tag size="small" :bordered="false" type="info">#{{ item.rank }}</n-tag>
              <n-text type="warning">{{ item.question }}</n-text>
              <template #tooltip>
                <div style="text-align: center;max-width: 180px">
                  <n-text type="warning">{{ item.question }}</n-text>
                </div>
              </template>
            </n-ellipsis>
          </n-list-item>
        </n-scrollbar>
      </n-list>

      <div v-show="leftTab==='combo'">
        <n-scrollbar style="max-height: calc(100vh - 210px);">
          <n-space vertical size="small" style="padding: 2px">
            <n-card
              v-for="combo in strategyCombos"
              :key="combo.name"
              size="small"
              hoverable
              @click="onComboClick(combo)"
              style="cursor: pointer;"
            >
              <template #header>
                <n-space align="center" size="small">
                  <n-icon :component="combo.icon" size="18"/>
                  <n-text strong>{{ combo.name }}</n-text>
                  <n-tag size="tiny" :bordered="false" type="info">{{ combo.category }}</n-tag>
                </n-space>
              </template>
              <n-text depth="2" style="font-size: 12px;">{{ combo.description }}</n-text>
              <template #footer>
                <n-space size="small">
                  <n-tooltip v-for="ind in combo.indicators" :key="ind.name" trigger="hover">
                    <template #trigger>
                      <n-tag size="tiny" :bordered="false" type="warning">{{ ind.name }}</n-tag>
                    </template>
                    <span>{{ ind.explanation }}</span>
                  </n-tooltip>
                </n-space>
              </template>
            </n-card>
          </n-space>
        </n-scrollbar>
      </div>

      <div v-show="leftTab==='custom'">
        <n-scrollbar style="max-height: calc(100vh - 250px);">
          <n-list bordered hoverable clickable v-if="customStrategies.length > 0">
            <n-list-item v-for="item in customStrategies" :key="item.id">
              <template #suffix>
                <n-flex :size="2" align="center">
                  <n-button text type="info" size="small" @click.stop="openSaveModal(true, item)">
                    <template #icon><n-icon :component="CreateOutline"/></template>
                  </n-button>
                  <n-popconfirm @positive-click="handleDeleteStrategy(item.id)">
                    <template #trigger>
                      <n-button text type="error" size="small" @click.stop>
                        <template #icon><n-icon :component="TrashOutline"/></template>
                      </n-button>
                    </template>
                    确定删除策略「{{ item.name }}」吗？
                  </n-popconfirm>
                </n-flex>
              </template>
              <div @click="DoSearch(item.query)" style="cursor: pointer;">
                <n-ellipsis line-clamp="1" :tooltip="true">
                  <n-tag size="small" :bordered="false" type="success">
                    <template #icon><n-icon :component="BookmarkOutline" size="12"/></template>
                  </n-tag>
                  <n-text strong>{{ item.name }}</n-text>
                  <template #tooltip>
                    <div style="max-width: 200px">
                      <div><n-text strong>{{ item.name }}</n-text></div>
                      <div v-if="item.description" style="margin-top:2px"><n-text depth="3">{{ item.description }}</n-text></div>
                      <div style="margin-top:2px"><n-text type="warning">{{ item.query }}</n-text></div>
                    </div>
                  </template>
                </n-ellipsis>
                <n-ellipsis line-clamp="1" style="margin-top: 2px;">
                  <n-text depth="3" style="font-size: 12px;">{{ item.query }}</n-text>
                </n-ellipsis>
              </div>
            </n-list-item>
          </n-list>
          <n-empty v-else description="暂无自定义策略" style="margin-top: 40px;"/>
        </n-scrollbar>
        <n-button block dashed type="primary" size="small" @click="openSaveModal(false)" style="margin-top: 4px;">
          <template #icon><n-icon :component="AddOutline"/></template>
          添加策略
        </n-button>
      </div>
    </n-gi>
    <n-gi :span="20">
      <div style="--wails-draggable:no-drag">
        <n-space align="center" style="margin-bottom: 4px">
          <n-switch v-model:value="useAIConfig" size="small" />
          <n-text depth="3" style="font-size: 12px;">AI配置选股</n-text>
          <n-tag v-if="useAIConfig" type="warning" size="tiny">消耗token</n-tag>
        </n-space>
        <n-input-group style="text-align: left">
          <n-input :rows="1" clearable v-model:value="search" placeholder="请输入选股指标或者要求" @keyup.enter="Search"/>
          <n-button type="primary" @click="Search" :loading="aiLoading" :disabled="aiLoading">
            {{ useAIConfig ? 'AI配置选股' : '搜索A股' }}
          </n-button>
          <n-button type="warning" @click="openSaveModal(false)" :disabled="!search">
            <template #icon><n-icon :component="BookmarkOutline" size="16"/></template>
            保存策略
          </n-button>
        </n-input-group>
      </div>
      <div v-if="traceInfo" style="margin: 5px 0; --wails-draggable:no-drag; display: flex; align-items: center; gap: 8px;">
        <n-ellipsis line-clamp="1" :tooltip="true" style="flex: 1;">
          <n-text type="info" :bordered="false">选股条件：</n-text>
          <n-text type="warning" :bordered="true">{{ traceInfo }}</n-text>
          <template #tooltip>
            <div style="text-align: center;max-width: 580px">
              <n-text type="warning">{{ traceInfo }}</n-text>
            </div>
          </template>
        </n-ellipsis>
        <n-button
          v-if="dataList.length > 0"
          size="tiny"
          type="warning"
          :loading="batchFollowing"
          :disabled="batchFollowing"
          @click="batchFollow"
        >
          <template #icon><n-icon :component="BookmarkOutline" size="14"/></template>
          {{ batchFollowing ? batchFollowProgress : '批量关注' }}
        </n-button>
      </div>
      <n-data-table
          :striped="true"
          flex-height
          size="small"
          :columns="columns"
          :data="dataList"
          :pagination="paginationProps"
          :scroll-x="tableScrollX"
          style="height: calc(100vh - 240px)"
          :render-cell="(value, rowData, column) => {
        if(column.key=='SECURITY_CODE'||column.key=='SERIAL'){
          return h(NText, { type: 'info',border: false }, { default: () => `${value}` })
        }
        if (isNumeric(value)) {
          let type='info';
          if (Number(value)<0){
            type='success';
          }
          if(Number(value)>=0&&Number(value)<=5){
            type='warning';
          }
          if (Number(value)>5){
            type='error';
          }
            return h(NText, { type: type }, { default: () => `${value}` })
        }else{
            if(column.key=='SECURITY_SHORT_NAME'){
              return h(NText, { type: 'info',bordered: false ,size:'small',onClick:()=>{
               openCenteredWindow(`https://quote.eastmoney.com/${rowData.MARKET_SHORT_NAME}${rowData.SECURITY_CODE}.html#fullScreenChart`,1240,700)
              }}, { default: () => `${value}` })
            }else{
              return h(NText, { type: 'info' }, { default: () => `${value}` })
            }
          }
      }"
      />
    </n-gi>
  </n-grid>

  <n-modal v-model:show="showSaveModal" preset="dialog" :title="saveForm.id ? '编辑策略' : '保存策略'" positive-text="保存" negative-text="取消"
           @positive-click="handleSaveStrategy" style="width: 500px;">
    <n-form label-placement="left" label-width="80">
      <n-form-item label="策略名称">
        <n-input v-model:value="saveForm.name" placeholder="请输入策略名称"/>
      </n-form-item>
      <n-form-item label="选股条件">
        <n-input v-model:value="saveForm.query" type="textarea" :rows="3" placeholder="请输入选股条件"/>
      </n-form-item>
      <n-form-item label="策略描述">
        <n-input v-model:value="saveForm.description" type="textarea" :rows="2" placeholder="可选，对策略的简要说明"/>
      </n-form-item>
    </n-form>
  </n-modal>

  <n-modal
    v-model:show="klineModalShow"
    :title="(klineStockName || '') + ' - ' + klineStockCode + ' K线图'"
    preset="card"
    style="width: 1400px;max-width: calc(100vw - 32px);"
    :mask-closable="true"
  >
    <StockLightweightKlineChart
      v-if="klineModalShow && klineStockCode"
      :key="klineStockCode"
      :code="klineStockCode"
      :stock-name="klineStockName"
      :dark-theme="darkTheme"
      :chart-height="460"
    />
  </n-modal>
</template>

<style scoped>
</style>
