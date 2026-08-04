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
  reasoningExpandedMap: { type: Object, required: true },
  expanded: { type: Boolean, default: false },
  isStreamLoad: { type: Boolean, default: false },
  isLastGroup: { type: Boolean, default: false },
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
              <div v-if="group.assistantMsg.modelName || group.assistantMsg.time" class="msg-meta-row-assistant">
                <span v-if="group.assistantMsg.modelName" class="msg-model-name" :title="group.assistantMsg.modelName">{{ group.assistantMsg.modelName }}</span>
                <span v-if="group.assistantMsg.time" class="msg-time">{{ group.assistantMsg.time }}</span>
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
