<script setup>
import { ref, onMounted } from 'vue'
import * as marketApi from '../api/market'

const sectors = ref([])
const activeSector = ref('ai')
const sectorNews = ref(null)
const loading = ref(false)
const error = ref('')

async function loadSectors() {
  try {
    sectors.value = (await marketApi.getSectors()).data
  } catch (e) {
    console.error('load sectors error:', e)
  }
}

async function loadNews() {
  loading.value = true
  error.value = ''
  try {
    sectorNews.value = (await marketApi.getNewsBySector(activeSector.value, 30)).data
  } catch (e) {
    error.value = e.message || String(e)
    sectorNews.value = null
  } finally {
    loading.value = false
  }
}

function switchSector(id) {
  activeSector.value = id
  loadNews()
}

function openURL(url) {
  if (url) window.open(url, '_blank')
}

onMounted(() => {
  loadSectors()
  loadNews()
})
</script>

<template>
  <div>
    <!-- 赛道 Tab 导航 -->
    <n-tabs type="line" animated :value="activeSector" @update:value="switchSector">
      <n-tab-pane v-for="s in sectors" :key="s.id" :tab="s.name" :name="s.id" />
    </n-tabs>

    <!-- 内容区 -->
    <n-spin :show="loading">
      <n-empty v-if="!loading && !sectorNews" description="暂无数据" />
      <div v-if="error" class="text-red-500 text-sm mb-2">{{ error }}</div>

      <template v-if="sectorNews">
        <!-- 今日要点 -->
        <n-card v-if="sectorNews.highlights && sectorNews.highlights.length" title="📌 今日要点" size="small" class="mb-4">
          <n-list>
            <n-list-item v-for="(item, i) in sectorNews.highlights" :key="i">
              <n-thing :title="item.title" :description="item.summary">
                <template #footer>
                  <n-space :size="12">
                    <n-tag size="tiny">{{ item.source }}</n-tag>
                    <n-text depth="3" class="text-xs">{{ item.time }}</n-text>
                    <n-button v-if="item.url" text size="tiny" type="primary" @click="openURL(item.url)">
                      原文
                    </n-button>
                  </n-space>
                </template>
              </n-thing>
            </n-list-item>
          </n-list>
        </n-card>

        <!-- 时间线新闻 -->
        <n-card title="时间线" size="small">
          <n-list v-if="sectorNews.news && sectorNews.news.length">
            <n-list-item v-for="(item, i) in sectorNews.news" :key="i">
              <n-thing :title="item.title" :description="item.summary">
                <template #footer>
                  <n-space :size="12">
                    <n-tag size="tiny">{{ item.source }}</n-tag>
                    <n-text depth="3" class="text-xs">{{ item.time }}</n-text>
                    <n-button v-if="item.url" text size="tiny" type="primary" @click="openURL(item.url)">
                      原文
                    </n-button>
                    <n-tag v-for="(stock, si) in (item.relatedStocks || [])" :key="si" size="tiny" type="info">
                      {{ stock }}
                    </n-tag>
                  </n-space>
                </template>
              </n-thing>
            </n-list-item>
          </n-list>
          <n-empty v-else description="该赛道暂无相关新闻" />
        </n-card>
      </template>
    </n-spin>
  </div>
</template>
