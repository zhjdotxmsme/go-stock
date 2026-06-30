<template>
  <n-space vertical>
    <n-space>
      <n-input
        v-model:value="query"
        placeholder="输入查询内容，例如：分析股票、技术指标"
        style="width: 400px"
        clearable
      />
      <n-button type="primary" @click="loadRecommendations" :loading="loading">
        获取推荐
      </n-button>
    </n-space>
    
    <n-data-table
      v-if="recommendations.length > 0"
      :columns="columns"
      :data="recommendations"
      :pagination="false"
      :bordered="true"
    />
    
    <n-empty v-else-if="!loading" description="暂无推荐，输入查询内容后点击获取推荐" />
  </n-space>
</template>

<script setup>
import { ref, h, onMounted } from 'vue'
import { NButton, NSpace, NInput, NDataTable, NTag, NEmpty, useMessage, NIcon, NPopconfirm } from 'naive-ui'
import { FlashOutline } from '@vicons/ionicons5'
import { GetAllSkills, EnableSkill } from '../../wailsjs/go/main/App.js'

const message = useMessage()
const loading = ref(false)
const query = ref('')
const recommendations = ref([])
const allSkills = ref([])

const columns = [
  { title: '类型', key: 'type', width: 80,
    render(row) {
      const typeMap = { enable: '启用', create: '创建', merge: '合并' }
      return h(NTag, { type: row.type === 'enable' ? 'warning' : 'info' },
        { default: () => typeMap[row.type] || row.type })
    }
  },
  { title: '名称', key: 'name', width: 150 },
  { title: '原因', key: 'reason', ellipsis: { tooltip: true } },
  {
    title: '操作', key: 'actions', width: 100,
    render(row) {
      if (row.type === 'enable') {
        return h(NButton, {
          size: 'small', type: 'primary',
          onClick: () => handleEnable(row)
        }, {
          icon: () => h(NIcon, null, { default: () => h(FlashOutline) }),
          default: () => '启用'
        })
      }
      return null
    }
  }
]

// Client-side recommendation matching
function matchSkills(query) {
  const results = []
  const lower = query.toLowerCase()
  
  // Check for high-scoring disabled skills
  for (const s of allSkills.value) {
    if (!s.enable && s.avgScore > 0.7) {
      results.push({
        type: 'enable',
        skillId: s.id,
        name: s.name,
        reason: `评分 ${s.avgScore.toFixed(2)} 但未启用`,
        avgScore: s.avgScore
      })
    }
  }
  
  // Check if any skill matches the query
  let hasMatch = false
  for (const s of allSkills.value) {
    if (!s.enable) continue
    if (!s.triggerKeywords) { hasMatch = true; break }
    for (const kw of s.triggerKeywords.split(',')) {
      if (lower.includes(kw.trim().toLowerCase())) {
        hasMatch = true
        break
      }
    }
    if (hasMatch) break
  }
  
  if (!hasMatch && query.trim()) {
    results.unshift({
      type: 'create',
      name: '新技能',
      reason: `"${query}" 未匹配到任何已启用技能`
    })
  }
  
  return results.sort((a, b) => (b.avgScore || 0) - (a.avgScore || 0))
}

async function loadRecommendations() {
  loading.value = true
  try {
    allSkills.value = await GetAllSkills() || []
    recommendations.value = matchSkills(query.value)
  } catch (e) {
    message.error('获取推荐失败: ' + e)
  } finally {
    loading.value = false
  }
}

async function handleEnable(row) {
  try {
    const result = await EnableSkill(row.skillId, true)
    message.success(result)
    loadRecommendations()
  } catch (e) {
    message.error('启用失败: ' + e)
  }
}

onMounted(() => {
  loadRecommendations()
})
</script>
