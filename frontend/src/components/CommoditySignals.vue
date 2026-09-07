<script setup>
import {ref, onMounted, onBeforeUnmount} from 'vue'
import {h} from 'vue'
import {useMessage, NTag} from 'naive-ui'
import * as commodityApi from '../api/commodity'
import {EventsOn, EventsOff, EventsEmit} from '../../wailsjs/runtime'

const message = useMessage()
const board = ref(null)
const loading = ref(false)
const error = ref('')

let timer = null

// 信号等级 → 中文 + 色调（红多绿空，国内习惯）
const labelMap = {
  bull:         {text: '多头',   type: 'error'},
  bear:         {text: '空头',   type: 'success'},
  transition:   {text: '中性',   type: 'default'},
  strong_bull:  {text: '强多',   type: 'error'},
  mild_bull:    {text: '偏多',   type: 'error'},
  neutral:      {text: '中性',   type: 'default'},
  mild_bear:    {text: '偏空',   type: 'success'},
  strong_bear:  {text: '强空',   type: 'success'},
  up:           {text: '向上突破', type: 'error'},
  down:         {text: '向下突破', type: 'success'},
  range:        {text: '区间震荡', type: 'default'},
  backwardation:{text: '贴水(偏多)', type: 'error'},
  contango:     {text: '升水(偏空)', type: 'success'},
  long_attack:  {text: '多头进攻', type: 'error'},
  short_cover:  {text: '空头回补', type: 'error'},
  short_attack: {text: '空头进攻', type: 'success'},
  long_exit:    {text: '多头离场', type: 'success'},
}

function sig(level) {
  if (!level) return {text: '—', type: 'default'}
  return labelMap[level] || {text: level, type: 'default'}
}

function sigTag(level) {
  const s = sig(level)
  if (s.text === '—') return h('span', {style: 'opacity:.4'}, '—')
  return h(NTag, {type: s.type, size: 'medium', round: true}, () => s.text)
}

function fmtPrice(p) {
  if (p === undefined || p === null) return '--'
  return Number(p).toFixed(2)
}

function fmtScore(s) {
  return (s >= 0 ? '+' : '') + Number(s).toFixed(3)
}

function verdictInfo(verdict) {
  if (verdict === '偏多') return {text: '偏多', type: 'error'}
  if (verdict === '偏空') return {text: '偏空', type: 'success'}
  return {text: verdict || '—', type: 'default'}
}

const columns = [
  {
    title: '品种',
    key: 'name',
    width: 160,
    render(row) {
      return h('div', null, [
        h('div', {style: 'font-weight:600'}, row.name),
        h('div', {style: 'font-size:12px;opacity:.65'}, `${row.market} · ${fmtPrice(row.price)}`),
      ])
    },
  },
  {title: '趋势', key: 'trend', width: 84, render: row => sigTag(row.trend)},
  {title: '动量', key: 'momentum', width: 84, render: row => sigTag(row.momentum)},
  {title: '突破', key: 'breakout', width: 96, render: row => sigTag(row.breakout)},
  {title: '期限', key: 'carry', width: 104, render: row => sigTag(row.carry)},
  {title: '持仓', key: 'oi', width: 96, render: row => sigTag(row.oi)},
  {
    title: '综合分',
    key: 'score',
    width: 190,
    render(row) {
      const score = Number(row.score) || 0
      const positive = score >= 0
      const widthPct = Math.min(1, Math.abs(score)) * 48
      const barColor = positive ? '#d03050' : '#18a058'
      return h('div', {style: 'display:flex;align-items:center;gap:8px'}, [
        h('div', {style: 'flex:1;position:relative;height:10px;border-radius:5px;background:rgba(128,128,128,.15);overflow:hidden'}, [
          h('div', {style: 'position:absolute;top:0;bottom:0;left:50%;width:1px;background:rgba(128,128,128,.45)'}),
          h('div', {
            style: positive
              ? `position:absolute;top:1px;bottom:1px;left:50%;width:${widthPct}%;background:${barColor};border-radius:3px`
              : `position:absolute;top:1px;bottom:1px;right:50%;width:${widthPct}%;background:${barColor};border-radius:3px`,
          }),
        ]),
        h('span', {style: 'font-size:12px;width:56px;text-align:right'}, fmtScore(row.score)),
      ])
    },
  },
  {
    title: '结论',
    key: 'verdict',
    width: 84,
    render(row) {
      const t = verdictInfo(row.verdict)
      return h(NTag, {type: t.type, size: 'medium', round: true}, () => t.text)
    },
  },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await commodityApi.getCommoditySignalBoard()
    board.value = res.data
  } catch (e) {
    console.error('signal board error', e)
    error.value = (e && e.message) || '信号板加载失败'
    if (board.value) {
      message.warning('信号板刷新失败，展示上次数据')
    } else {
      message.error('信号板加载失败')
    }
  } finally {
    loading.value = false
  }
}

function onRowClick(row) {
  EventsEmit('selectCommodityAsset', {code: row.code, name: row.name})
  EventsEmit('changeCommodityTab', {name: '行情总览'})
}

onMounted(() => {
  load()
  timer = setInterval(load, 120000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <n-space vertical>
    <template v-if="board">
      <div>
        <n-space align="center" :size="8">
          <n-text strong style="font-size: 14px">📊 全品种策略信号</n-text>
          <n-tag type="error" size="small" round>偏多 {{ board.stats.bull }}</n-tag>
          <n-tag size="small" round>中性 {{ board.stats.neutral }}</n-tag>
          <n-tag type="success" size="small" round>偏空 {{ board.stats.bear }}</n-tag>
          <n-text depth="3" style="font-size: 12px">更新：{{ board.fetchedAt }}</n-text>
          <n-button size="tiny" quaternary @click="load">刷新</n-button>
        </n-space>
        <n-text depth="3" style="font-size: 12px">
          信号 = 趋势(MA20/60) + 动量(20/60日) + 唐奇安突破 + 期限结构 + 持仓四象限（期货），按综合分强度排序；点击行查看 K 线。
        </n-text>
      </div>

      <n-data-table
          v-if="!loading || board"
          :columns="columns"
          :data="board.rows"
          :row-props="(row) => ({ onClick: () => !row.failed && onRowClick(row), style: row.failed ? 'opacity:.5' : 'cursor:pointer' })"
          :bordered="true"
          size="small"
      />

      <n-space v-if="error && !board" align="center">
        <n-text depth="3">{{ error }}</n-text>
        <n-button size="small" @click="load">重试</n-button>
      </n-space>
    </template>
    <n-spin v-else :show="loading">
      <div style="min-height: 120px"></div>
    </n-spin>
  </n-space>
</template>
