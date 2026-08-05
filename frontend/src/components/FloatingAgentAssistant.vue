<template>
  <Transition name="fade">
    <div
      v-if="showButton"
      :class="['edge-trigger', { 'edge-trigger-busy': hasBackgroundTask }]"
      @click="togglePanel"
      :title="hasBackgroundTask ? 'go-stock AI Agent 助手正在后台分析...' : 'go-stock AI Agent 助手'"
    >
      <div class="edge-trigger-inner">
        <NIcon :component="SparklesOutline" size="18" />
        <span class="edge-trigger-text">AI助手</span>
        <div v-if="hasBackgroundTask" class="edge-trigger-badge" />
      </div>
    </div>
  </Transition>

  <Transition name="drawer-slide">
    <div v-if="panelVisible" class="drawer-wrap">
      <div class="drawer-mask" @click="closePanel" />
      <div class="drawer-panel" @click.stop>
        <NCard
          size="small"
          class="panel-card"
          :bordered="false"
          content-style="padding: 0; display: flex; flex-direction: column; min-height: 0; overflow: hidden;"
        >
          <template #header>
            <div class="panel-header">
              <span class="panel-title">go-stock AI Agent 助手</span>
              <div class="panel-actions">
                <NButton size="small" quaternary @click="startNewChat" title="开始新对话">
                  新对话
                </NButton>
                <NButton quaternary circle size="small" title="分享到社区" :loading="shareLoading" @click="shareAiToCommunity">
                  <template #icon>
                    <NIcon :component="ShareSocialOutline" />
                  </template>
                </NButton>
                <NButton quaternary circle size="small" title="关闭" @click="closePanel">
                  <template #icon>
                    <NIcon :component="CloseOutline" />
                  </template>
                </NButton>
              </div>
            </div>
          </template>

            <div class="chat-body">
            <Transition name="hint-fade">
              <div v-if="hintVisible" class="hint-bar">{{ hintText }}</div>
            </Transition>
            <div v-if="shareTipVisible" class="share-tip">
              <div class="share-tip-text">{{ shareTipText }}</div>
              <NButton size="tiny" quaternary class="share-tip-close" @click="shareTipVisible = false">关闭</NButton>
            </div>
            <NScrollbar ref="scrollbarRef" class="chat-scroll">
              <div class="message-list">
                <MessageBubble
                  v-for="(group, groupIndex) in messageGroups"
                  :key="group.id"
                  :group="group"
                  :group-index="groupIndex"
                  :theme="theme"
                  :reasoning-expanded-map="reasoningExpandedMap"
                  :expanded="isGroupExpanded(groupIndex)"
                  :is-stream-load="isStreamLoad"
                  :is-last-group="groupIndex === messageGroups.length - 1"
                  :share-loading="shareLoading"
                  :export-image-key="exportImageKey"
                  @toggle-group="toggleGroup"
                  @toggle-reasoning="toggleReasoning"
                  @copy="copyAiContent"
                  @export-image="exportAiReplyImage"
                  @share="shareAiContent"
                  @md-html-changed="onMdHtmlChanged"
                />
              </div>
            </NScrollbar>
            </div>

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
              :send-message="sendMessage"
              :abort-stream="abortStream"
              :on-user-prompt-change="onUserPromptChange"
            />
        </NCard>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeMount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NIcon, NScrollbar, useMessage } from 'naive-ui'
import {
  CloseOutline,
  SparklesOutline,
  PersonCircleOutline,
  CopyOutline,
  ShareSocialOutline,
  ImageOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  ChevronUpOutline
} from '@vicons/ionicons5'
import * as systemApi from '../api/system'
import 'md-editor-v3/lib/preview.css'
import MessageBubble from './agent/MessageBubble.vue'
import { onMdHtmlChanged } from './agent/codeCollapse'
import { useMessageGroups } from './agent/useMessageGroups'
import { useShareExport } from './agent/useShareExport'
import { useAgentOptions } from './agent/useAgentOptions'
import { useChatSession } from './agent/useChatSession'
import { useAgentStream } from './agent/useAgentStream'
import AgentChatFooter from './agent/AgentChatFooter.vue'


const route = useRoute()
const message = useMessage()

const showButton = computed(() => route.name !== 'agent')

const inputValue = ref('')
const messages = ref([])

// ---- 模型/提示词/模式选项（agent/useAgentOptions） ----
const {
  aiConfigOptions, aiConfigId, modelLabelForConfig,
  sysPromptTemplates, sysPromptOptions, sysPromptId,
  userPromptTemplates, userPromptOptions, userPromptId,
  thinkingMode, memoryMode, memoryCount, memoryCountOptions,
  agentMode, agentModeOptions, onUserPromptChange, loadPromptTemplates,
} = useAgentOptions({ inputValue, showHint })

const canSend = computed(() => !!inputValue.value.trim())
const scrollbarRef = ref(null)
const darkTheme = ref(false)
const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))
const hintVisible = ref(false)
const hintText = ref('')
let hintTimer = null

function showHint(text) {
  hintText.value = text
  hintVisible.value = true
  if (hintTimer) clearTimeout(hintTimer)
  hintTimer = setTimeout(() => { hintVisible.value = false }, 3000)
}

// ---- 消息分组与展开状态（agent/useMessageGroups） ----
const {
  messageGroups, expandedGroups, reasoningExpandedMap,
  isGroupExpanded, toggleGroup, initDefaultExpanded, ensureLatestGroupExpanded, toggleReasoning,
} = useMessageGroups({ messages })

// ---- 复制/分享/导出图片（agent/useShareExport） ----
const {
  shareLoading, exportImageKey, shareTipVisible, shareTipText,
  copyAiContent, shareTextToCommunity, shareAiContent,
  getLastAssistantContent, shareAiToCommunity, exportAiReplyImage,
} = useShareExport({ messages, darkTheme })

// ---- 会话管理与面板开合（agent/useChatSession） ----
const {
  sessionId, panelVisible, vipLevel,
  loadHistory, saveHistory, openPanel, closePanel, togglePanel, scrollToBottom,
} = useChatSession({ messages, scrollbarRef, message, initDefaultExpanded })

// ---- 流式对话与事件订阅（agent/useAgentStream） ----
const {
  isStreamLoad, sentFromFloating, isAborted,
  abortStream, sendMessage, startNewChat,
} = useAgentStream({
  messages, inputValue, sessionId, message,
  aiConfigId, aiConfigOptions, sysPromptId, memoryMode, memoryCount,
  thinkingMode, agentMode, modelLabelForConfig,
  shareTipText, shareTipVisible,
  saveHistory, scrollToBottom,
  ensureLatestGroupExpanded, messageGroups, reasoningExpandedMap,
})

const hasBackgroundTask = computed(() => isStreamLoad.value && sentFromFloating.value && !panelVisible.value)

watch(panelVisible, (v) => {
  if (v) {
    loadPromptTemplates()
    nextTick(scrollToBottom)
  }
})

onBeforeMount(() => {
  systemApi.getConfig().then(({data: result}) => {
    darkTheme.value = result.darkTheme
  })
})

onMounted(() => {
  loadPromptTemplates()
})


</script>

<style scoped>
.edge-trigger {
  position: fixed;
  top: 50%;
  right: 0;
  z-index: 9998;
  transform: translateY(-50%);
  width: 32px;
  height: 120px;
  border-radius: 12px 0 0 12px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: -2px 0 12px rgba(102, 126, 234, 0.4);
  transition: width 0.2s ease, box-shadow 0.2s ease;
}
.edge-trigger-busy {
  box-shadow: -4px 0 18px rgba(248, 113, 113, 0.8);
}
.edge-trigger:hover {
  width: 40px;
  box-shadow: -4px 0 16px rgba(102, 126, 234, 0.5);
}
.edge-trigger-inner {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.edge-trigger-text {
  font-size: 14px;
  writing-mode: vertical-rl;
  letter-spacing: 2px;
  line-height: 1;
  white-space: nowrap;
}
.edge-trigger-badge {
  position: absolute;
  top: 6px;
  left: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f97316;
  box-shadow: 0 0 6px rgba(248, 113, 113, 0.9);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.drawer-wrap {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
}
.drawer-wrap > * {
  pointer-events: auto;
}
.drawer-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  cursor: pointer;
}
.drawer-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 60vw;
  min-width: 320px;
  max-width: calc(100vw - 48px);
  background: var(--n-color-modal);
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-card {
  height: 100%;
  border-radius: 0;
  box-shadow: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.panel-card :deep(.n-card-header) {
  padding: 12px 16px;
  flex-shrink: 0;
}
.panel-card :deep(.n-card__content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.panel-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.panel-title {
  font-weight: 600;
  font-size: 16px;
}

.chat-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}
.hint-bar {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 8px 14px;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(118, 75, 162, 0.12) 100%);
  border: 1px solid rgba(102, 126, 234, 0.25);
  font-size: 13px;
  color: var(--n-text-color-2);
  text-align: center;
  line-height: 1.5;
}
.hint-fade-enter-active,
.hint-fade-leave-active {
  transition: opacity 0.3s, transform 0.3s;
}
.hint-fade-enter-from,
.hint-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.share-tip {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.04);
  border: 1px solid var(--n-border-color);
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.share-tip-text {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  text-align: left;
}
.share-tip-close {
  flex-shrink: 0;
}
.chat-scroll {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.chat-scroll :deep(.n-scrollbar-content) {
  min-height: 0;
}
.message-list {
  padding: 12px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message-group {
  border: 1px solid var(--n-border-color);
  border-radius: 12px;
  overflow: hidden;
  background: var(--n-color-modal);
}
.message-group-header {
  padding: 10px 14px;
  cursor: pointer;
  background: rgba(0, 0, 0, 0.02);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.message-group-header:hover {
  background: rgba(0, 0, 0, 0.04);
}
.message-group-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}
.message-group-title {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.message-group-time {
  font-size: 11px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}
.message-group-content {
  padding: 12px 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.message-group-content .message-item {
  padding: 0 14px;
}
.message-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
}
.message-item.user {
  align-items: flex-end;
}
.msg-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.assistant-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}
.user-avatar {
  background: linear-gradient(135deg, #34d399 0%, #22c55e 35%, #06b6d4 100%);
  color: #fff;
  box-shadow: 0 6px 14px rgba(34, 197, 94, 0.22);
  border: 1px solid rgba(255, 255, 255, 0.45);
}
.msg-bubble {
  max-width: 100%;
  width: 100%;
  box-sizing: border-box;
  padding: 8px 10px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
  display: flex;
  flex-direction: column;
}
.message-item.assistant .msg-bubble {
  background: var(--n-color-modal);
  border: 1px solid var(--n-border-color);
}
.message-item.user .msg-bubble {
  background: var(--n-color-primary);
  color: #fff;
  text-align: right;
}
.message-item.user .msg-content,
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  text-align: right;
}
.msg-content {
  white-space: normal;
  width: 100%;
  min-width: 0;
  flex: 1;
}
.msg-reasoning-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-steps-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.08) 0%, rgba(46, 139, 87, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-steps-header:hover {
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.14) 0%, rgba(46, 139, 87, 0.14) 100%);
}
.msg-steps-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-steps-count {
  font-size: 11px;
  background: var(--n-primary-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.msg-steps-content {
  padding: 10px 12px 10px 16px;
  max-height: 300px;
  overflow-y: auto;
}
.msg-step-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 4px 0;
  position: relative;
  font-size: 12px;
  color: var(--n-text-color-2);
  line-height: 1.5;
}
.msg-step-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 18px;
  bottom: -4px;
  width: 1px;
  background: var(--n-border-color);
}
.msg-step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--n-text-color-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
}
.msg-step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 4px rgba(230, 162, 60, 0.4);
}
.msg-step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 4px rgba(64, 158, 255, 0.4);
}
.msg-step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 4px rgba(103, 194, 58, 0.4);
}
.msg-step-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  text-align: left;
}
.msg-reasoning-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.08) 0%, rgba(118, 75, 162, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-reasoning-header:hover {
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(118, 75, 162, 0.12) 100%);
}
.msg-reasoning-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-reasoning-content {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  padding: 12px;
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
  text-align: left;
}
.msg-json-md-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-json-md-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.08) 0%, rgba(5, 150, 105, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-json-md-header:hover {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.14) 0%, rgba(5, 150, 105, 0.14) 100%);
}
.msg-json-md-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-json-md-content {
  padding: 12px;
  max-height: 300px;
  overflow-y: auto;
  text-align: left;
}
.msg-reasoning {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  background: var(--n-color-hover);
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
  border-left: 3px solid var(--n-primary-color);
}
.msg-bubble-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
  margin-top: 8px;
}
.msg-meta-row-assistant {
  flex: 1 1 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--n-text-color-3);
}
.msg-meta-row-assistant .msg-time {
  flex-shrink: 0;
}
.msg-model-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}
.msg-share-btn,
.msg-copy-btn,
.msg-export-img-btn,
.msg-toggle-btn {
  padding: 2px 10px;
  font-size: 12px;
  border-radius: 12px;
  color: var(--n-primary-color);
  background-color: var(--n-primary-color-suppl);
  border: 1px solid var(--n-primary-color);
  transition: color 0.2s, border-color 0.2s, background-color 0.2s;
}
.msg-share-btn:hover,
.msg-copy-btn:hover,
.msg-export-img-btn:hover,
.msg-toggle-btn:hover {
  border-color: var(--n-primary-color);
  background-color: var(--n-primary-color);
  color: #fff;
}
.message-item.user .msg-bubble .msg-share-btn,
.message-item.user .msg-bubble .msg-copy-btn,
.message-item.user .msg-bubble .msg-export-img-btn,
.message-item.user .msg-bubble .msg-toggle-btn {
  color: rgba(255, 255, 255, 0.92);
  background-color: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.65);
}
.message-item.user .msg-bubble .msg-share-btn:hover,
.message-item.user .msg-bubble .msg-copy-btn:hover,
.message-item.user .msg-bubble .msg-export-img-btn:hover,
.message-item.user .msg-bubble .msg-toggle-btn:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.95);
  background-color: rgba(255, 255, 255, 0.32);
}
.msg-content .msg-markdown {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.msg-content .msg-markdown :deep(.md-editor-preview-wrapper) {
  width: 100%;
}
.msg-content .msg-markdown :deep(.md-editor-preview) {
  font-size: 13px;
  line-height: 1.6;
  padding: 0 8px;
  width: 100%;
  box-sizing: border-box;
}
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  color: inherit;
}
.msg-loading {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.msg-meta {
  margin-top: 4px;
  font-size: 11px;
  color: var(--n-text-color-3);
  display: flex;
}
.msg-meta-user-inner {
  justify-content: flex-end;
  margin-top: 6px;
  margin-bottom: 0;
}
.message-item.user .msg-meta-user-inner {
  color: rgba(255, 255, 255, 0.78);
}


.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.drawer-slide-enter-active .drawer-mask,
.drawer-slide-leave-active .drawer-mask {
  transition: opacity 0.25s ease;
}
.drawer-slide-enter-active .drawer-panel,
.drawer-slide-leave-active .drawer-panel {
  transition: transform 0.25s ease;
}
.drawer-slide-enter-from .drawer-mask,
.drawer-slide-leave-to .drawer-mask {
  opacity: 0;
}
.drawer-slide-enter-from .drawer-panel,
.drawer-slide-leave-to .drawer-panel {
  transform: translateX(100%);
}
.drawer-slide-enter-to .drawer-mask,
.drawer-slide-leave-from .drawer-mask {
  opacity: 1;
}
.drawer-slide-enter-to .drawer-panel,
.drawer-slide-leave-from .drawer-panel {
  transform: translateX(0);
}
</style>

<style>
.msg-markdown .md-editor-code-block {
  position: relative;
}
.msg-markdown .md-editor-code-block pre {
  margin: 0;
}
.msg-markdown .md-editor-code-block .code-collapse-btn {
  position: absolute;
  top: 0;
  right: 0;
  z-index: 2;
  padding: 2px 8px;
  font-size: 11px;
  color: var(--n-text-color-3);
  background: var(--n-color-hover);
  border: 1px solid var(--n-border-color);
  border-radius: 0 4px 0 4px;
  cursor: pointer;
  user-select: none;
  opacity: 0;
  transition: opacity 0.2s;
}
.msg-markdown .md-editor-code-block:hover .code-collapse-btn {
  opacity: 1;
}
.msg-markdown .md-editor-code-block.code-collapsed pre {
  max-height: 80px;
  overflow: hidden;
}
.msg-markdown .md-editor-code-block.code-collapsed::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 40px;
  background: linear-gradient(transparent, var(--n-color));
  pointer-events: none;
}
</style>
