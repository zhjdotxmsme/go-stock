<script setup>
// AI Agent 助手输入区（自 FloatingAgentAssistant.vue 原样抽离：模板 + 样式零改动）
// 8 个 v-model 直通父组件状态；sendMessage/abortStream/onUserPromptChange 为函数 props。
const aiConfigId = defineModel('aiConfigId')
const sysPromptId = defineModel('sysPromptId')
const userPromptId = defineModel('userPromptId')
const thinkingMode = defineModel('thinkingMode', { type: Boolean })
const memoryMode = defineModel('memoryMode', { type: Boolean })
const memoryCount = defineModel('memoryCount', { type: Number })
const agentMode = defineModel('agentMode', { type: String })
const inputValue = defineModel('inputValue', { type: String })

defineProps({
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
      <NInput
        v-model:value="inputValue"
        type="textarea"
        placeholder="输入消息，回车发送..."
        :autosize="{ minRows: 2, maxRows: 4 }"
        :disabled="isStreamLoad"
        @keydown.enter.exact.prevent="sendMessage"
      />
      <NButton
        v-if="isStreamLoad"
        type="warning"
        quaternary
        class="chat-footer-abort"
        @click="abortStream(true)"
      >
        中断
      </NButton>
      <NButton
        type="primary"
        :loading="isStreamLoad"
        :disabled="isStreamLoad || !canSend"
        @click="sendMessage"
      >
        发送
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
