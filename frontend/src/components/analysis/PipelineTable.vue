<template>
  <div class="pipeline-table">
    <n-data-table
      :columns="columns"
      :data="rows"
      :loading="loading"
      size="small"
      :row-key="(r: RecommendRow) => r.code"
    />
    <!-- 展开详情面板 -->
    <PipelineDetail
      v-if="expandedCode"
      :code="expandedCode"
      :signal-date="expandedSignalDate"
      :visible="true"
      @close="emit('expand', '')"
    />
  </div>
</template>

<script setup lang="ts">
import { h, ref, watch, computed } from 'vue'
import { NButton, NTag, NText, NIcon } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ChevronDownOutline, ChevronUpOutline } from '@vicons/ionicons5'
import PipelineDetail from './PipelineDetail.vue'
import type { RecommendRow } from './recommend'
import { rowSubtitle } from './recommend'

const props = defineProps<{
  rows: RecommendRow[]
  loading: boolean
  expandedCode: string
}>()

const emit = defineEmits<{
  (e: 'expand', code: string): void
}>()

// 找到展开行的 signalDate
const expandedSignalDate = computed(() => {
  const row = props.rows.find(r => r.code === props.expandedCode)
  return row?.signalDate || ''
})

const columns: DataTableColumns<RecommendRow> = [
  {
    title: '代码',
    key: 'code',
    width: 96,
    render(row) {
      return h('span', [
        h(
          NButton,
          {
            size: 'tiny',
            quaternary: true,
            onClick: () => emit('expand', row.code),
          },
          {
            icon: () =>
              h(NIcon, null, {
                default: () =>
                  props.expandedCode === row.code
                    ? h(ChevronUpOutline)
                    : h(ChevronDownOutline),
              }),
          }
        ),
        h('span', { style: 'margin-left: 4px' }, row.code),
      ])
    },
  },
  { title: '名称', key: 'name', width: 96 },
  { title: '信号日', key: 'signalDate', width: 110 },
  { title: '状态', key: 'status', width: 80 },
  { title: '评分', key: 'score', width: 72 },
  {
    title: 'Kronos',
    key: 'factor',
    width: 130,
    render(row) {
      if (row.hasFactor) {
        return h(
          NTag,
          { size: 'tiny', type: 'info' },
          { default: () => `${row.factorScore} × ${row.factorWeight}` }
        )
      }
      const sub = rowSubtitle(row)
      return sub
        ? h(NText, { depth: 3 }, { default: () => sub })
        : h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: '综合',
    key: 'blendedScore',
    width: 80,
    sorter: (a, b) => a.blendedScore - b.blendedScore,
    defaultSortOrder: 'descend',
  },
  {
    title: '操作',
    key: 'op',
    width: 100,
    render(row) {
      return h(
        NButton,
        {
          size: 'tiny',
          onClick: () => emit('expand', row.code),
        },
        { default: () => (props.expandedCode === row.code ? '收起' : '详情') }
      )
    },
  },
]
</script>

<style scoped>
.pipeline-table {
  width: 100%;
}
</style>
