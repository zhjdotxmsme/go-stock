<script setup>
/**
 * 会话侧栏（仅 /agent 页使用）：新建 / 切换 / 删除。
 * 数据与动作全部来自 stores/agent.ts。
 */
import { computed, onMounted } from 'vue'
import { NButton, NEmpty, NIcon, NPopconfirm, NScrollbar, useMessage } from 'naive-ui'
import { AddOutline, ChatbubbleEllipsesOutline, TrashOutline } from '@vicons/ionicons5'
import { storeToRefs } from 'pinia'
import { useAgentStore } from '../../stores/agent'

const agentStore = useAgentStore()
const message = useMessage()
const { sessions, sessionId, isStreamLoad } = storeToRefs(agentStore)

const rows = computed(() =>
  (sessions.value ?? []).map((s) => ({
    id: s.sessionId,
    title: (s.title ?? '').trim() || '未命名会话',
    count: s.messageCount ?? 0,
    time: formatTime(s.updatedAt),
    active: s.sessionId === sessionId.value,
  }))
)

function formatTime(value) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  if (sameDay) return `${hh}:${mm}`
  return `${d.getMonth() + 1}/${d.getDate()} ${hh}:${mm}`
}

async function onNew() {
  agentStore.startNewChat()
  await agentStore.loadSessions()
  message.success('已开启新对话')
}

async function onSwitch(id) {
  await agentStore.switchSession(id)
}

async function onDelete(id) {
  await agentStore.deleteSession(id)
}

onMounted(() => {
  agentStore.loadSessions()
})
</script>

<template>
  <aside class="session-side">
    <div class="session-side-head">
      <NButton type="primary" size="small" block @click="onNew">
        <template #icon>
          <NIcon :component="AddOutline" />
        </template>
        新对话
      </NButton>
    </div>

    <NScrollbar class="session-side-list">
      <NEmpty v-if="rows.length === 0" size="small" description="暂无历史会话" style="margin-top: 24px" />
      <div
        v-for="row in rows"
        :key="row.id"
        class="session-item"
        :class="{ 'session-item--active': row.active }"
        @click="onSwitch(row.id)"
      >
        <div class="session-item-main">
          <NIcon :component="ChatbubbleEllipsesOutline" size="14" class="session-item-icon" />
          <span class="session-item-title" :title="row.title">{{ row.title }}</span>
        </div>
        <div class="session-item-meta">
          <span class="session-item-time">{{ row.count }} 条 · {{ row.time }}</span>
          <NPopconfirm @positive-click="onDelete(row.id)">
            <template #trigger>
              <NButton
                text
                size="tiny"
                class="session-item-del"
                :disabled="isStreamLoad"
                @click.stop
              >
                <template #icon>
                  <NIcon :component="TrashOutline" />
                </template>
              </NButton>
            </template>
            删除该会话？此操作不可恢复。
          </NPopconfirm>
        </div>
      </div>
    </NScrollbar>
  </aside>
</template>

<style scoped>
.session-side {
  width: 240px;
  flex: 0 0 240px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-right: 1px solid var(--n-border-color);
  background: var(--n-color-embedded);
}
.session-side-head {
  flex: 0 0 auto;
  padding: 12px;
  border-bottom: 1px solid var(--n-border-color);
}
.session-side-list {
  flex: 1 1 auto;
  min-height: 0;
}
.session-item {
  padding: 8px 10px 8px 12px;
  cursor: pointer;
  border-left: 3px solid transparent;
  transition: background 0.15s ease;
}
.session-item:hover {
  background: var(--n-color-hover);
}
.session-item--active {
  border-left-color: var(--n-color-target);
  background: var(--n-color-hover);
}
.session-item-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.session-item-icon {
  flex-shrink: 0;
  color: var(--n-text-color-3);
}
.session-item-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.session-item-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-top: 2px;
  padding-left: 20px;
}
.session-item-time {
  font-size: 11px;
  color: var(--n-text-color-3);
}
.session-item-del {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.session-item:hover .session-item-del {
  opacity: 1;
}
</style>
