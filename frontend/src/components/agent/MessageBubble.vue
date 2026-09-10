<script setup>
import { MdPreview } from 'md-editor-v3'
import { NButton, NIcon, NSpin } from 'naive-ui'
import {
  PersonCircleOutline, SparklesOutline, ChevronDownOutline, ChevronForwardOutline,
  ChevronUpOutline, CopyOutline, ImageOutline, ShareSocialOutline,
} from '@vicons/ionicons5'

const props = defineProps({
  group: { type: Object, required: true },
  groupIndex: { type: Number, required: true },
  theme: { type: String, default: 'light' },
  // 容错：展开状态缺失时退化为「全部折叠」，而不是让整个消息列表渲染崩掉
  reasoningExpandedMap: { type: Object, default: () => ({}) },
  expanded: { type: Boolean, default: false },
  isStreamLoad: { type: Boolean, default: false },
  isLastGroup: { type: Boolean, default: false },
  /** 该条回答是否被手动中断（显示角标；不持久化） */
  aborted: { type: Boolean, default: false },
  /** 'page' | 'panel'：页面模式显示更多细节 */
  density: { type: String, default: 'panel' },
  shareLoading: { type: Boolean, default: false },
  exportImageKey: { type: String, default: '' },
})

const emit = defineEmits([
  'toggle-group', 'toggle-reasoning',
  'copy', 'export-image', 'share',
  'md-html-changed',
])

function getStepDotClass(step) {
  if (step.startsWith('✅')) return 'step-done'
  if (step.startsWith('❌')) return 'step-error'
  if (step.startsWith('⏳')) return 'step-running'
  return 'step-pending'
}

function onToggleGroup() {
  emit('toggle-group', props.groupIndex)
}

function onToggleReasoning(key) {
  emit('toggle-reasoning', key)
}

function onCopy() {
  emit('copy', props.group.assistantMsg)
}

function onExportImage(e) {
  emit('export-image', props.group.assistantIndex, e)
}

function onShare() {
  emit('share', props.group.assistantMsg)
}

function onMdHtmlChanged() {
  emit('md-html-changed')
}
</script>

<template>
  <div class="message-group">
    <div class="message-group-header" @click="onToggleGroup">
      <div class="message-group-summary">
        <NIcon :component="expanded ? ChevronDownOutline : ChevronForwardOutline" size="16" />
        <span class="message-group-title">{{ group.userMsg.content.slice(0, 50) }}{{ group.userMsg.content.length > 50 ? '...' : '' }}</span>
        <span class="message-group-time">{{ group.userMsg.time }}</span>
      </div>
    </div>
    <div v-show="expanded" class="message-group-content">
      <div :class="['message-item', group.userMsg.role]">
        <div class="msg-avatar user-avatar">
          <NIcon :component="PersonCircleOutline" size="18" />
        </div>
        <div class="msg-bubble">
          <div class="msg-content">
            <div v-if="group.userMsg.time" class="msg-meta msg-meta-user-inner">
              <span class="msg-time">{{ group.userMsg.time }}</span>
            </div>
            <MdPreview
              :theme="theme"
              :style="{ textAlign: 'right' }"
              v-if="group.userMsg.content"
              :model-value="group.userMsg.content"
              :editor-id="'agent-msg-' + group.userIndex"
              class="msg-markdown"
            />
          </div>
        </div>
      </div>
      <div v-if="group.assistantMsg" :class="['message-item', 'assistant']">
        <div class="msg-avatar assistant-avatar">
          <NIcon :component="SparklesOutline" size="20" />
        </div>
        <div class="msg-bubble">
          <div class="msg-content">
            <div v-if="group.assistantMsg.steps && group.assistantMsg.steps.length > 0" class="msg-steps-wrapper">
              <div class="msg-steps-header" @click="onToggleReasoning(group.assistantIndex)">
                <NIcon :component="reasoningExpandedMap[group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-steps-title">📋 执行步骤</span>
                <span class="msg-steps-count">{{ group.assistantMsg.steps.length }}</span>
              </div>
              <div v-show="reasoningExpandedMap[group.assistantIndex]" class="msg-steps-content">
                <div v-for="(step, si) in group.assistantMsg.steps" :key="si" class="msg-step-item">
                  <div class="msg-step-dot" :class="getStepDotClass(step)"></div>
                  <span class="msg-step-text">{{ step }}</span>
                </div>
              </div>
            </div>
            <div v-if="group.assistantMsg.reasoning" class="msg-reasoning-wrapper">
              <div class="msg-reasoning-header" @click="onToggleReasoning('r-' + group.assistantIndex)">
                <NIcon :component="reasoningExpandedMap['r-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-reasoning-title">💭 思考过程</span>
              </div>
              <div v-show="reasoningExpandedMap['r-' + group.assistantIndex]" class="msg-reasoning-content">
                <MdPreview
                  :theme="theme"
                  :style="{ textAlign: 'left' }"
                  :model-value="group.assistantMsg.reasoning"
                  :editor-id="'agent-reasoning-' + group.assistantIndex"
                  class="msg-markdown"
                />
              </div>
            </div>
            <div v-if="group.assistantMsg.jsonMarkdown" class="msg-json-md-wrapper">
              <div class="msg-json-md-header" @click="onToggleReasoning('j-' + group.assistantIndex)">
                <NIcon :component="reasoningExpandedMap['j-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-json-md-title">📊 分析报告</span>
              </div>
              <div v-show="reasoningExpandedMap['j-' + group.assistantIndex]" class="msg-json-md-content">
                <MdPreview
                  :theme="theme"
                  :style="{ textAlign: 'left' }"
                  :model-value="group.assistantMsg.jsonMarkdown"
                  :editor-id="'agent-json-md-' + group.assistantIndex"
                  class="msg-markdown"
                  @onHtmlChanged="onMdHtmlChanged"
                />
              </div>
            </div>
            <MdPreview
              :theme="theme"
              :style="{ textAlign: 'left' }"
              :model-value="group.assistantMsg.content || '...'"
              :editor-id="'agent-msg-' + group.assistantIndex"
              class="msg-markdown"
              @onHtmlChanged="onMdHtmlChanged"
            />
            <div v-if="isStreamLoad && isLastGroup && !group.assistantMsg.content" class="msg-loading">
              <NSpin size="small" />
              <span>思考中...</span>
            </div>
            <div class="msg-bubble-actions">
              <div v-if="group.assistantMsg.modelName || group.assistantMsg.time || aborted" class="msg-meta-row-assistant">
                <span v-if="group.assistantMsg.modelName" class="msg-model-name" :title="group.assistantMsg.modelName">{{ group.assistantMsg.modelName }}</span>
                <span v-if="group.assistantMsg.time" class="msg-time">{{ group.assistantMsg.time }}</span>
                <span v-if="aborted" class="msg-aborted-tag" title="本次回答被手动中断，内容可能不完整">已中断</span>
              </div>
              <NButton quaternary size="tiny" class="msg-toggle-btn" @click="onToggleGroup">
                <template #icon>
                  <NIcon :component="expanded ? ChevronUpOutline : ChevronDownOutline" />
                </template>
                {{ expanded ? '收起' : '展开' }}
              </NButton>
              <NButton quaternary size="tiny" class="msg-copy-btn" @click="onCopy">
                <template #icon>
                  <NIcon :component="CopyOutline" />
                </template>
                复制
              </NButton>
              <NButton
                quaternary
                size="tiny"
                class="msg-export-img-btn"
                :loading="exportImageKey === String(group.assistantIndex)"
                title="导出为图片"
                @click="onExportImage"
              >
                <template #icon>
                  <NIcon :component="ImageOutline" />
                </template>
                导出图
              </NButton>
              <NButton quaternary size="tiny" class="msg-share-btn" :loading="shareLoading" @click="onShare">
                <template #icon>
                  <NIcon :component="ShareSocialOutline" />
                </template>
                分享
              </NButton>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 消息区样式：原先挂在 FloatingAgentAssistant.vue 的 scoped 块里，
   但那些选择器作用于本组件内部元素，scoped 不会下传，导致消息区一直无样式渲染。
   迁到本组件后既生效，也能被 /agent 页与侧边浮窗共用。 */
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
</style>
