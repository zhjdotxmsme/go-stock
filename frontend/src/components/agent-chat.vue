<template>
  <div class="agent-page">
    <AgentSessionList />
    <div class="agent-page-main">
      <div class="agent-page-head">
        <span class="agent-page-title">AI 智能体</span>
        <span class="agent-page-sub">会话与侧边「AI 助手」共用；切换页面不会丢失</span>
        <div class="agent-page-actions">
          <NButton size="small" quaternary :disabled="isStreamLoad" @click="onClearHistory">
            清空当前会话
          </NButton>
        </div>
      </div>
      <AgentChatPanel density="page" class="agent-page-panel" />
    </div>
  </div>
</template>

<script setup>
/**
 * /agent（菜单「AI 能力 → Ai智能体」）页面。
 *
 * 由 TDesign t-chat 实现改为复用 AgentChatPanel：
 * - 旧实现把「执行步骤 / 思维链 / 分析报告」塞进一个默认折叠且无标题的 Collapse；
 * - 旧实现流式期间禁用输入框、自定义「发送」按钮点了静默返回；
 * - 旧实现 onStop 不调后端中断，且 onBeforeUnmount 的 EventsOff('agent-message')
 *   会把侧边浮窗的监听一并清空（Wails 按事件名清空所有监听者）。
 * 以上问题在改用共享面板后一并消除；会话与流式状态来自 stores/agent.ts。
 */
import { onMounted } from 'vue'
import { NButton, useDialog } from 'naive-ui'
import { storeToRefs } from 'pinia'
import AgentChatPanel from '../components/agent/AgentChatPanel.vue'
import AgentSessionList from '../components/agent/AgentSessionList.vue'
import { useAgentStore } from '../stores/agent'

const agentStore = useAgentStore()
const dialog = useDialog()
const { isStreamLoad } = storeToRefs(agentStore)

function onClearHistory() {
  dialog.warning({
    title: '清空当前会话',
    content: '将清空当前会话的消息（左侧历史会话仍保留），确定继续？',
    positiveText: '清空',
    negativeText: '取消',
    onPositiveClick: () => {
      // 开一段新会话（旧实现是直接 chatList = []，没有任何确认）
      agentStore.startNewChat()
    },
  })
}

onMounted(() => {
  // 浮窗与本页共用 store：bootstrap 幂等，先到者拉取，后到者复用（不会覆盖内存中的会话）
  agentStore.bootstrap()
})
</script>

<style scoped>
.agent-page {
  display: flex;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  text-align: left;
}
.agent-page-main {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.agent-page-head {
  flex: 0 0 auto;
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.agent-page-title {
  font-size: 15px;
  font-weight: 600;
}
.agent-page-sub {
  font-size: 12px;
  color: var(--n-text-color-3);
}
.agent-page-actions {
  margin-left: auto;
}
.agent-page-panel {
  flex: 1 1 auto;
  min-height: 0;
}
</style>
