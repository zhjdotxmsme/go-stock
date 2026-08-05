<script setup>
// AI 分析结果弹窗（自 stock.vue 原样抽离）
// mdEditorRef/mdPreviewRef/aiResultScrollRef/tipsRef 为父组件持有的 ref（导出功能要用），
// 通过函数 ref 桥接写回，挂载/卸载时序与模板字符串 ref 一致。
import { computed, ref } from 'vue'
import { MdEditor, MdPreview } from 'md-editor-v3'
// preview.css相比style.css少了编辑器那部分样式
//import 'md-editor-v3/lib/preview.css';
import 'md-editor-v3/lib/style.css'
import { ExportPDF } from '@vavt/v3-extension'
import '@vavt/v3-extension/lib/asset/ExportPDF.css'
import MultiAgentResult from '../MultiAgentResult.vue'

const show = defineModel('show', { type: Boolean })
const enableTools = defineModel('enableTools', { type: Boolean })
const thinkingMode = defineModel('thinkingMode', { type: Boolean })
const strategyCode = defineModel('strategyCode', { type: String })

const props = defineProps({
  data: { type: Object, required: true },
  multiAgentState: { type: Object, required: true },
  strategies: { type: Array, required: true },
  aiConfigs: { type: Array, required: true },
  sysPromptOptions: { type: Array, required: true },
  userPromptOptions: { type: Array, required: true },
  // 父组件持有的 DOM/组件 ref（exportReport 使用）
  mdEditorRef: { type: Object, required: true },
  mdPreviewRef: { type: Object, required: true },
  aiResultScrollRef: { type: Object, required: true },
  tipsRef: { type: Object, required: true },
  // 操作函数（父组件注入）
  aiReCheckStock: { type: Function, required: true },
  saveAsImage: { type: Function, required: true },
  copyToClipboard: { type: Function, required: true },
  saveAsMarkdown: { type: Function, required: true },
  saveAsWord: { type: Function, required: true },
  share: { type: Function, required: true },
})

const toolbars = [0];

const handleProgress = (progress) => {
  //console.log(`Export progress: ${progress.ratio * 100}%`);
};
const enableEditor = ref(false)

const theme = computed(() => {
  return props.data.darkTheme ? 'dark' : 'light'
})
</script>

<template>
  <n-modal transform-origin="center" v-model:show="show" preset="card" style="width: 800px;max-width: calc(100vw - 32px);"
           :title="'['+data.name+']AI分析'">
    <!-- Multi-agent structured view -->
    <div v-if="multiAgentState.active" class="multi-agent-container">
      <MultiAgentResult :state="multiAgentState" />
    </div>
    <!-- Legacy flat markdown view (always rendered for export access) -->
    <div v-show="!multiAgentState.active" style="height: 440px;max-height: 60vh;">
      <n-spin size="small" :show="data.loading && !data.airesult">
        <MdEditor v-if="enableEditor" :toolbars="toolbars" :ref="(el) => mdEditorRef.value = el" style="height: 440px;text-align: left"
                  :modelValue="data.airesult" :theme="theme">
          <template #defToolbars>
            <ExportPDF :file-name="data.name+'['+data.code+']AI分析报告'" style="text-align: left"
                       :modelValue="data.airesult" @onProgress="handleProgress"/>
          </template>
        </MdEditor>
        <div v-if="!enableEditor" :ref="(el) => aiResultScrollRef.value = el" style="height: 440px;text-align: left;overflow-y: auto;">
          <MdPreview :ref="(el) => mdPreviewRef.value = el" :modelValue="data.airesult" :theme="theme"/>
        </div>
      </n-spin>
    </div>
    <template #footer>
      <n-flex justify="space-between" :ref="(el) => tipsRef.value = el">
        <n-text type="info" v-if="data.time">
          <n-tag v-if="data.modelName" type="warning" round :title="data.chatId" :bordered="false">
            {{ data.modelName }}
          </n-tag>
          {{ data.time }}
        </n-text>
        <n-text type="success" v-if="data.analysisStatus">{{ data.analysisStatus }}</n-text>
        <n-text type="error">*AI分析结果仅供参考，请以实际行情为准。投资需谨慎，风险自担。</n-text>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="left" style="margin-bottom: 10px">
        <n-switch v-model:value="enableTools" :round="false">
          <template #checked>
            工具调用
          </template>
          <template #unchecked>
            非工具调用
          </template>
        </n-switch>
        <n-switch v-model:value="thinkingMode" :round="false">
          <template #checked>
            思考模式
          </template>
          <template #unchecked>
            非思考模式
          </template>
        </n-switch>
        <n-gradient-text type="error" style="margin-left: 10px">
          *AI函数工具调用可以增强AI获取数据的能力,但会消耗更多tokens。
        </n-gradient-text>
      </n-flex>
      <n-flex justify="left" style="margin-bottom: 10px">
        <n-select style="width: 240px" v-model:value="strategyCode" :options="strategies"
                  placeholder="选择分析策略（默认全维度）" clearable/>
      </n-flex>
      <n-flex justify="space-between" style="margin-bottom: 10px">
        <n-select style="width: 31%" v-model:value="data.aiConfigId" label-field="name" value-field="ID"
                  :options="aiConfigs" placeholder="请选择AI模型服务配置"/>
        <n-select style="width: 31%" v-model:value="data.sysPromptId" label-field="name" value-field="ID"
                  :options="sysPromptOptions" placeholder="请选择系统提示词"/>
        <n-select style="width: 31%" v-model:value="data.question" label-field="name" value-field="content"
                  :options="userPromptOptions" placeholder="请选择用户提示词"/>
      </n-flex>
      <n-flex justify="right">
        <n-input v-model:value="data.question" style="text-align: left" clearable
                 type="textarea"
                 :show-count="true"
                 placeholder="请输入您的问题:例如{{stockName}}[{{stockCode}}]分析和总结"
                 :autosize="{
              minRows: 2,
              maxRows: 5
            }"
        />
        <!--        <n-button size="tiny" type="error" @click="enableEditor=!enableEditor">编辑/预览</n-button>-->
        <n-button size="tiny" type="warning" @click="aiReCheckStock(data.name,data.code)">开始AI分析</n-button>
        <n-button size="tiny" type="info" @click="saveAsImage(data.name,data.code)">保存为图片</n-button>
        <n-button size="tiny" type="success" @click="copyToClipboard">复制到剪切板</n-button>
        <n-button size="tiny" type="primary" @click="saveAsMarkdown">保存为Markdown文件</n-button>
        <n-button size="tiny" type="primary" @click="saveAsWord">保存为Word文件</n-button>
        <n-button size="tiny" type="error" @click="share(data.code,data.name)">分享到项目社区</n-button>
      </n-flex>
    </template>
  </n-modal>
</template>

<style scoped>
.md-editor-preview h3 {
  text-align: center !important;
}

.md-editor-preview p {
  text-align: left !important;
}
</style>
