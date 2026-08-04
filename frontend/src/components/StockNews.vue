<script setup>
import { ref, watch } from 'vue'
import * as marketApi from '../api/market'

const props = defineProps({
  code: { type: String, required: true },
})

const news = ref([])
const loading = ref(false)

async function load() {
  if (!props.code) return
  loading.value = true
  try {
    news.value = (await marketApi.getStockRelatedNews(props.code, 20)).data
  } catch (e) {
    console.error('stock news error:', e)
    news.value = []
  } finally {
    loading.value = false
  }
}

watch(() => props.code, load, { immediate: true })

function openURL(url) {
  if (url) window.open(url, '_blank')
}
</script>

<template>
  <n-spin :show="loading">
    <n-empty v-if="!loading && news.length === 0" description="暂无相关资讯" />
    <n-list v-else>
      <n-list-item v-for="(item, i) in news" :key="i">
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
  </n-spin>
</template>
