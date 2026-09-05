<script setup lang="ts">
/**
 * 统一财经日历时间轴组件（合并自 InvestCalendarTimeLine + ClsCalendarTimeLine）
 * source="invest" 财联社投资日历（按月分页，支持加载更多）
 * source="cls"    财经日历（事件/经济数据，含星级与前值/预测/公布值）
 *
 * 两种数据源的形状不同，这里在组件内归一化后渲染，对外只暴露 source prop。
 */
import { nextTick, onBeforeMount, ref } from 'vue'
import * as marketApi from '../api/market'
import { addMonths, format, parse } from 'date-fns'
import { zhCN } from 'date-fns/locale'

import { useMessage } from 'naive-ui'
import { Star48Filled } from '@vicons/fluent'

const props = defineProps({
  source: { type: String, default: 'invest' }, // 'invest' | 'cls'
})

const today = new Date()
const formattedDate = format(today, 'yyyy-MM-dd')
const formattedYM = format(today, 'yyyy-MM')

const list = ref([])
const message = useMessage()

/** 统一条目模型 { key, title, star, tags, economic } */
function normalizeDay(raw) {
  if (props.source === 'cls') {
    return {
      day: raw.calendar_day,
      week: raw.week || '',
      entries: (raw.items || []).map((l) => ({
        key: l.id,
        title: l.title,
        star: Math.max(l.event?.star ?? 0, l.economic?.star ?? 0),
        tags: [
          ...(l.event ? ['事件'] : []),
          ...(l.economic ? ['数据'] : []),
        ],
        economic: l.economic || null,
      })),
    }
  }
  return {
    day: raw.date,
    week: getweekday(raw.date),
    entries: (raw.list || []).map((l) => ({
      key: l.article_id,
      title: l.title,
      star: l.like_count ?? 0,
      tags: [],
      economic: null,
    })),
  }
}

function goBackToday() {
  setTimeout(() => {
    nextTick(() => {
      const elementById = document.getElementById(formattedDate)
      if (elementById) {
        elementById.scrollIntoView({ behavior: 'auto', block: 'start' })
      }
    })
  }, 500)
}

async function fetchList(ym: string) {
  if (props.source === 'cls') {
    const { data: res } = await marketApi.clsCalendar()
    return res || []
  }
  const { data: res } = await marketApi.investCalendarTimeLine(ym)
  return res || []
}

onBeforeMount(async () => {
  list.value = await fetchList(formattedYM)
  goBackToday()
})

function loadMore() {
  if (props.source !== 'invest' || list.value.length === 0) return
  const lastDay = parse(list.value[list.value.length - 1].date, 'yyyy-MM-dd', new Date())
  const ym = format(addMonths(lastDay, 1), 'yyyy-MM')
  marketApi.investCalendarTimeLine(ym).then(({ data: res }) => {
    if (!res || res.length === 0) {
      message.warning('没有更多数据了')
      return
    }
    list.value.push(...res)
  })
}

function getweekday(date) {
  if (!date) return ''
  let day = parse(date, 'yyyy-MM-dd', new Date())
  if (Number.isNaN(day.getTime())) return ''
  return format(day, 'EEEE', { locale: zhCN })
}
</script>

<template>
  <n-list bordered style="max-height: calc(100vh - 230px); text-align: left">
    <n-scrollbar style="max-height: calc(100vh - 230px)">
      <n-list-item v-for="item in list" :id="item.day" :key="item.day">
        <n-thing :title="item.day + ' ' + item.week">
          <n-list :bordered="false" hoverable>
            <n-list-item v-for="(l, i) in item.entries" :key="l.key">
              <n-flex justify="space-between">
                <n-text :type="item.day === formattedDate ? 'warning' : 'info'">
                  {{ i + 1 }}# {{ l.title }}
                  <n-tag v-for="t in l.tags" :key="t" size="small" round
                         :type="t === '事件' ? 'success' : 'error'">{{ t }}</n-tag>
                </n-text>
                <n-rate v-if="l.star > 0" readonly :default-value="l.star">
                  <n-icon :component="Star48Filled" />
                </n-rate>
              </n-flex>
              <n-flex v-if="l.economic">
                <n-tag type="warning" :bordered="false" size="small">公布：{{ l.economic.actual }}</n-tag>
                <n-tag type="warning" :bordered="false" size="small">预测：{{ l.economic.consensus }}</n-tag>
                <n-tag type="warning" :bordered="false" size="small">前值：{{ l.economic.front }}</n-tag>
              </n-flex>
            </n-list-item>
          </n-list>
        </n-thing>
      </n-list-item>
      <n-list-item v-if="list.length === 0">
        <n-text type="info">没有数据</n-text>
      </n-list-item>
      <n-list-item v-else style="text-align: center">
        <n-button-group>
          <n-button v-if="source === 'invest'" strong secondary type="info" @click="loadMore">加载更多</n-button>
          <n-button strong secondary type="warning" @click="goBackToday">回到今天</n-button>
        </n-button-group>
      </n-list-item>
    </n-scrollbar>
  </n-list>
</template>
