<script setup>
/**
 * Agent 聊天面板（共享）—— /agent 页与侧边「AI 助手」浮窗共用同一套实现。
 *
 * 设计（spec 第 4 节）：
 * - 面板不知道自己跑在页面还是抽屉里，只按 density 调整信息密度；
 * - 全部状态来自 stores/agent.ts（单一真源），面板不持有会话数据；
 * - 面板自己注册 notifier 与滚动容器，宿主只负责外层容器与页面级 chrome。
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NButton, NScrollbar, useMessage } from 'naive-ui'
import { storeToRefs } from 'pinia'
import MessageBubble from './MessageBubble.vue'
import AgentChatFooter from './AgentChatFooter.vue'
import { useShareExport } from './useShareExport'
import { onMdHtmlChanged } from './codeCollapse'
import { useAgentStore } from '../../stores/agent'

const props = defineProps({
  /** 'page'：页面全宽，默认展开各分区；'panel'：侧边抽屉，紧凑折叠 */
  density: { type: String, default: 'panel' },
})

const agentStore = useAgentStore()
const message = useMessage()
const scrollbarRef = ref(null)
const darkTheme = ref(false)
let unregisterScroller = null

const {
  messages, inputValue, panelVisible,
  isStreamLoad, shareTipVisible, shareTipText, hintVisible, hintText,
  aiConfigOptions, aiConfigId, sysPromptId, userPromptId,
  thinkingMode, memoryMode, memoryCount, agentMode,
  memoryCountOptions, agentModeOptions, sysPromptOptions, userPromptOptions,
  messageGroups, reasoningExpandedMap,
} = storeToRefs(agentStore)

const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))
const canSend = computed(() => !!inputValue.value.trim())

/**
 * 展开默认值（按 density 区分）：
 * - page：所有分组与分区默认展开（页面够宽，过程信息直接可见）
 * - panel：只展开最新一组，各分区默认折叠（抽屉空间小）
 * 用户手动点过的项记在 store 的覆盖表里，优先于这里的默认值。
 */
const isPage = computed(() => props.density === 'page')
function groupExpanded(groupIndex) {
  const isLatest = groupIndex === messageGroups.value.length - 1
  return agentStore.isGroupExpanded(groupIndex, isPage.value || isLatest)
}
function onToggleGroup(groupIndex) {
  agentStore.toggleGroup(groupIndex, isPage.value || groupIndex === messageGroups.value.length - 1)
}
function onToggleSection(key) {
  agentStore.toggleReasoning(key, isPage.value)
}

// 面板自己持有分享/导出（依赖具体 DOM 节点做截图）
const {
  shareLoading, exportImageKey,
  copyAiContent, shareAiContent, shareAiToCommunity, exportAiReplyImage,
} = useShareExport({
  messages,
  darkTheme,
  tipVisible: shareTipVisible,
  tipText: shareTipText,
})

onMounted(() => {
  agentStore.registerNotifier(message)
  unregisterScroller = agentStore.registerScroller(() => {
    scrollbarRef.value?.scrollTo({ top: 99999, behavior: 'smooth' })
  })
  // 主题只影响消息气泡渲染，随系统配置读取一次即可
  import('../../api/system').then(({ getConfig }) => {
    getConfig().then(({ data }) => {
      darkTheme.value = data?.darkTheme ?? false
    })
  })
})

onBeforeUnmount(() => {
  if (unregisterScroller) unregisterScroller()
})

// 宿主可用的面板能力（浮窗头部的「分享到社区」按钮即通过面板实例调用）
defineExpose({
  scrollbarRef,
  shareLoading,
  shareAiToCommunity,
  scrollToBottom: () => agentStore.scrollToBottom(),
})
</script>

<template>
  <div class="agent-panel" :class="`agent-panel--${props.density}`">
    <Transition name="hint-fade">
      <div v-if="hintVisible" class="hint-bar">{{ hintText }}</div>
    </Transition>
    <div v-if="shareTipVisible" class="share-tip">
      <div class="share-tip-text">{{ shareTipText }}</div>
      <NButton size="tiny" quaternary class="share-tip-close" @click="shareTipVisible = false">关闭</NButton>
    </div>

    <NScrollbar ref="scrollbarRef" class="agent-panel-scroll">
      <div class="message-list">
        <MessageBubble
          v-for="(group, groupIndex) in messageGroups"
          :key="group.id"
          :group="group"
          :group-index="groupIndex"
          :theme="theme"
          :reasoning-expanded-map="reasoningExpandedMap"
          :expanded="groupExpanded(groupIndex)"
          :is-stream-load="isStreamLoad"
          :is-last-group="groupIndex === messageGroups.length - 1"
          :aborted="agentStore.isMessageAborted(group.assistantIndex)"
          :density="props.density"
          :share-loading="shareLoading"
          :export-image-key="exportImageKey"
          @toggle-group="onToggleGroup"
          @toggle-reasoning="onToggleSection"
          @copy="copyAiContent"
          @export-image="exportAiReplyImage"
          @share="shareAiContent"
          @md-html-changed="onMdHtmlChanged"
        />
      </div>
    </NScrollbar>

    <AgentChatFooter
      v-model:ai-config-id="aiConfigId"
      v-model:sys-prompt-id="sysPromptId"
      v-model:user-prompt-id="userPromptId"
      v-model:thinking-mode="thinkingMode"
      v-model:memory-mode="memoryMode"
      v-model:memory-count="memoryCount"
      v-model:agent-mode="agentMode"
      v-model:input-value="inputValue"
      :ai-config-options="aiConfigOptions"
      :sys-prompt-options="sysPromptOptions"
      :user-prompt-options="userPromptOptions"
      :memory-count-options="memoryCountOptions"
      :agent-mode-options="agentModeOptions"
      :can-send="canSend"
      :is-stream-load="isStreamLoad"
      :send-message="agentStore.sendMessage"
      :abort-stream="agentStore.abortStream"
      :on-user-prompt-change="agentStore.onUserPromptChange"
    />
  </div>
</template>

<style scoped>
.agent-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  background: var(--n-color-modal);
}

.agent-panel-scroll {
  flex: 1 1 auto;
  min-height: 0;
}

.message-list {
  padding: 12px 16px 4px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 页面模式：更宽松的留白，内容区居中限宽，长文本更好读 */
.agent-panel--page .message-list {
  padding: 16px 24px 8px;
  max-width: 1100px;
  margin: 0 auto;
  width: 100%;
}

.hint-bar {
  flex: 0 0 auto;
  padding: 6px 12px;
  font-size: 12px;
  text-align: center;
  color: var(--n-text-color-3);
  background: var(--n-color-embedded);
}

.share-tip {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  font-size: 12px;
  background: var(--n-color-embedded);
  color: var(--n-text-color-2);
}

.share-tip-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
}

.hint-fade-enter-active,
.hint-fade-leave-active {
  transition: opacity 0.25s ease;
}
.hint-fade-enter-from,
.hint-fade-leave-to {
  opacity: 0;
}
</style>
