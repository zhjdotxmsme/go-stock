<script setup>
// AI Agent 助手输入区（自 FloatingAgentAssistant.vue 原样抽离：模板 + 样式零改动）
// 8 个 v-model 直通父组件状态；sendMessage/abortStream/onUserPromptChange 为函数 props。
//
// 交互修复（spec A）：流式期间**不再禁用输入框**（旧实现 :disabled="isStreamLoad"，
// 让用户在整个回答过程中打不了字）。改为：可继续输入草稿；有草稿时主按钮变
// 「中断并发送」（点击=先中断当前回答再发出新问题，与 sendMessage 语义一致），
// 无草稿时主按钮禁用并给出独立「中断」按钮。
import { computed } from 'vue'

const aiConfigId = defineModel('aiConfigId')
const sysPromptId = defineModel('sysPromptId')
const userPromptId = defineModel('userPromptId')
const thinkingMode = defineModel('thinkingMode', { type: Boolean })
const memoryMode = defineModel('memoryMode', { type: Boolean })
const memoryCount = defineModel('memoryCount', { type: Number })
const agentMode = defineModel('agentMode', { type: String })
const inputValue = defineModel('inputValue', { type: String })

const props = defineProps({
  aiConfigOptions: { type: Array, required: true },
  sysPromptOptions: { type: Array, required: true },
  userPromptOptions: { type: Array, required: true },
  memoryCountOptions: { type: Array, required: true },
  agentModeOptions: { type: Array, required: true },
  canSend: { type: Boolean, required: true },
  isStreamLoad: { type: Boolean, required: true },
  sendMessage: { type: Function, required: true },
  abortStream: { type: Function, required: true },
  onUserPromptChange: { type: Function, required: true },
})

/** 有草稿的流式态下，主按钮承担「中断并发送」语义 */
const sendInterrupts = computed(() => props.isStreamLoad && props.canSend)
const sendLabel = computed(() => {
  if (sendInterrupts.value) return '中断并发送'
  if (props.isStreamLoad) return '生成中…'
  return '发送'
})
const sendDisabled = computed(() => {
  if (sendInterrupts.value) return false
  if (props.isStreamLoad) return true
  return !props.canSend
})

function onSendClick() {
  props.sendMessage()
}
</script>

<template>
  <div class="chat-footer">
    <div class="chat-footer-row">
      <NSelect
        v-model:value="aiConfigId"
        :options="aiConfigOptions"
        size="small"
        filterable
        to="body"
        placement="top-start"
        placeholder="选择模型"
        :consistent-menu-width="false"
        :menu-props="{ style: { zIndex: 10002 } }"
        class="chat-footer-select"
      />
      <NSelect
        v-model:value="sysPromptId"
        :options="sysPromptOptions"
        size="small"
        clearable
        to="body"
        placement="top-start"
        placeholder="系统提示词"
        :consistent-menu-width="false"
        :menu-props="{ style: { zIndex: 10002 } }"
        class="chat-footer-prompt"
      />
      <NSelect
        v-model:value="userPromptId"
        :options="userPromptOptions"
        size="small"
        clearable
        to="body"
        placement="top-start"
        placeholder="用户提示词"
        :consistent-menu-width="false"
        :menu-props="{ style: { zIndex: 10002 } }"
        class="chat-footer-prompt"
        @update:value="onUserPromptChange"
      />
      <div class="chat-footer-thinking">
        <span class="chat-footer-thinking-label">思考模式</span>
        <NSwitch v-model:value="thinkingMode" size="small" />
      </div>
      <div class="chat-footer-memory">
        <span class="chat-footer-thinking-label">记忆模式</span>
        <NSwitch v-model:value="memoryMode" size="small" />
        <NSelect
          v-if="memoryMode"
          v-model:value="memoryCount"
          :options="memoryCountOptions"
          size="small"
          :consistent-menu-width="false"
          to="body"
          placement="top-start"
          :menu-props="{ style: { zIndex: 10002 } }"
          class="chat-footer-memory-count"
        />
      </div>
      <div class="chat-footer-agent-mode">
        <NSelect
          v-model:value="agentMode"
          :options="agentModeOptions"
          size="small"
          to="body"
          placement="top-start"
          placeholder="Agent模式"
          :consistent-menu-width="false"
          :menu-props="{ style: { zIndex: 10002 } }"
          class="chat-footer-agent-mode-select"
        />
      </div>
    </div>
    <div class="chat-footer-input">
      <!-- 流式期间不禁用：允许先打草稿，避免"整个回答过程打不了字" -->
      <NInput
        v-model:value="inputValue"
        type="textarea"
        :placeholder="isStreamLoad ? '可继续输入；回车将中断当前回答并发送' : '输入消息，回车发送...'"
        :autosize="{ minRows: 2, maxRows: 4 }"
        @keydown.enter.exact.prevent="onSendClick"
      />
      <NButton
        v-if="isStreamLoad"
        type="warning"
        quaternary
        class="chat-footer-abort"
        title="中断当前回答（会取消后端生成）"
        @click="abortStream(true)"
      >
        中断
      </NButton>
      <NButton
        type="primary"
        :loading="isStreamLoad && !sendInterrupts"
        :disabled="sendDisabled"
        :title="sendInterrupts ? '中断当前回答并立即发送新问题' : ''"
        @click="onSendClick"
      >
        {{ sendLabel }}
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.chat-footer {
  flex-shrink: 0;
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color);
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--n-color-modal);
}
.chat-footer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.chat-footer-select {
  flex: 1;
  min-width: 0;
}
.chat-footer-select .n-select {
  width: 100%;
}
.chat-footer-prompt {
  flex: 0 0 120px;
  min-width: 0;
}
.chat-footer-prompt .n-select {
  width: 100%;
}
.chat-footer-thinking {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-thinking-label {
  font-size: 12px;
  color: var(--n-text-color-2);
}
.chat-footer-memory {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-memory-count {
  width: 70px;
}
.chat-footer-agent-mode-select {
  width: 120px;
}
.chat-footer-memory-count .n-select {
  width: 100%;
}
.chat-footer-input {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}
.chat-footer-input .n-input {
  flex: 1;
  min-width: 0;
}
.chat-footer-input .n-input :deep(textarea) {
  text-align: left;
}
.chat-footer-input .n-button {
  flex-shrink: 0;
}
.chat-footer-abort {
  color: #f97316;
}
</style>

<style>
body > div:has(.n-select-menu) {
  z-index: 10002 !important;
}
</style>
