/**
 * AI 分析结果导出与分享：图片 / Markdown / Word / 剪贴板 / 社区分享，及报告组装辅助。
 * 自 stock.vue 原样搬迁；依赖经 ctx 传入（ref 共享引用）。
 */
import { h, nextTick } from 'vue'
import { NAvatar } from 'naive-ui'
import { Environment } from '../../../wailsjs/runtime'
import html2canvas from 'html2canvas'
import { asBlob } from 'html-docx-js-typescript'
import * as stockApi from '../../api/stock'
import * as systemApi from '../../api/system'

export function useExportReport(ctx) {
  const { mdPreviewRef, mdEditorRef, aiResultScrollRef, tipsRef, message, notify, icon } = ctx

  function saveAsImage(name, code) {
    const previewEl = mdPreviewRef.value?.$el || mdEditorRef.value?.$el
    const element = previewEl?.querySelector('.md-editor-preview-wrapper') ||
                    previewEl?.querySelector('.md-editor-preview') ||
                    document.querySelector('.md-editor-preview')
    if (!element) {
      message.error('无法找到分析结果元素')
      return
    }
    const savedStyles = []
    let el = element.parentElement
    while (el && el !== document.body) {
      const style = getComputedStyle(el)
      if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
        savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
        el.style.overflow = 'visible'
        el.style.overflowY = 'visible'
        el.style.height = 'auto'
        el.style.maxHeight = 'none'
      }
      el = el.parentElement
    }
    const savedTargetStyle = { height: element.style.height, maxHeight: element.style.maxHeight, overflow: element.style.overflow, overflowY: element.style.overflowY }
    element.style.height = 'auto'
    element.style.maxHeight = 'none'
    element.style.overflow = 'visible'
    element.style.overflowY = 'visible'
    nextTick(async () => {
      const isDark = document.documentElement.getAttribute('theme-mode') === 'dark'
      try {
        const canvas = await html2canvas(element, {
          useCORS: true,
          scale: 2,
          allowTaint: true,
          logging: false,
          backgroundColor: isDark ? '#1e1e1e' : '#ffffff'
        })
        element.style.height = savedTargetStyle.height
        element.style.maxHeight = savedTargetStyle.maxHeight
        element.style.overflow = savedTargetStyle.overflow
        element.style.overflowY = savedTargetStyle.overflowY
        savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
          el.style.overflow = overflow
          el.style.overflowY = overflowY
          el.style.height = height
          el.style.maxHeight = maxHeight
        })
        const dataUrl = canvas.toDataURL('image/png')
        const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
        const {data: result} = await stockApi.saveImage(name + '[' + code + ']AI分析', base64)
        if (result && !result.includes('异常') && !result.includes('无法')) {
          message.success('已导出为 PNG 图片：' + result)
        } else {
          message.info(result || '导出取消')
        }
      } catch (e) {
        element.style.height = savedTargetStyle.height
        element.style.maxHeight = savedTargetStyle.maxHeight
        element.style.overflow = savedTargetStyle.overflow
        element.style.overflowY = savedTargetStyle.overflowY
        savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
          el.style.overflow = overflow
          el.style.overflowY = overflowY
          el.style.height = height
          el.style.maxHeight = maxHeight
        })
        message.error('导出图片失败: ' + (e?.message ?? e))
      }
    })
  }
  
  async function copyToClipboard() {
    try {
      await navigator.clipboard.writeText(data.airesult);
      message.success('分析结果已复制到剪切板');
    } catch (err) {
      message.error('复制失败: ' + err);
    }
  }
  
  function agentTitle(role) {
    const map = { fundamental: '基本面', technical: '技术面', sentiment: '情绪面', news: '新闻面', synthesis: '综合' }
    return map[role] || role
  }
  
  // buildFullReport constructs a complete markdown report from multi-agent state for export.
  function buildFullReport(state) {
    let md = `# AI 多维度分析报告\n\n`
    md += `> 生成时间：${new Date().toLocaleString()}\n\n`
  
    // Analyst reports
    const roles = ['fundamental', 'technical', 'sentiment', 'news']
    const roleLabels = { fundamental: '基本面分析', technical: '技术面分析', sentiment: '情绪面分析', news: '新闻面分析' }
  
    for (const role of roles) {
      const content = state.reports[role]
      if (content) {
        md += `## 📊 ${roleLabels[role]}\n\n${content}\n\n`
      }
    }
  
    // Debate
    if (state.debates && state.debates.length > 0) {
      md += `## ⚖️ 多空辩论\n\n`
      for (const d of state.debates) {
        const sideLabel = d.side === 'bull' ? '看多方' : '看空方'
        md += `### ${sideLabel} 第${d.round}轮\n\n${d.argument}\n\n`
      }
    }
  
    // Final report
    const fr = state.finalReport
    if (fr) {
      md += `## 📝 最终结论\n\n`
      md += `**总体评级**：${fr.overallRating || '待定'}\n\n`
      if (fr.conclusion) md += `${fr.conclusion}\n\n`
      if (fr.catalysts && fr.catalysts.length > 0) {
        md += `### 催化剂\n\n${fr.catalysts.map(c => `- ${c}`).join('\n')}\n\n`
      }
      if (fr.riskFactors && fr.riskFactors.length > 0) {
        md += `### 风险因素\n\n${fr.riskFactors.map(r => `- ${r}`).join('\n')}\n\n`
      }
    }
  
    md += `---\n*本报告由 go-stock AI 多维度分析系统生成，仅供参考，不构成投资建议。*\n`
    return md
  }
  
  function scrollToAiResultBottom() {
    nextTick(() => {
      requestAnimationFrame(() => {
        const el = aiResultScrollRef.value
        if (el) {
          el.scrollTop = el.scrollHeight
        }
      })
    })
  }
  
  function saveAsMarkdown() {
    systemApi.saveAsMarkdown(data.code, data.name).then(({data: result}) => {
      message.success(result)
    })
  }
  
  function saveAsMarkdown_old() {
    const blob = new Blob([data.airesult], {type: 'text/markdown;charset=utf-8'});
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `${data.name}[${data.code}]-${data.time}ai-analysis-result.md`;
    link.click();
    URL.revokeObjectURL(link.href);
    link.remove()
  }
  
  function getHtml(ref) {
    if (ref.value) {
      // 获取 MdPreview 组件的根元素
      const rootElement = ref.value.$el;
      // 获取 HTML 内容
      return rootElement.innerHTML;
    } else {
      console.error('mdPreviewRef is not yet available');
      return "";
    }
  }
  
  // 导出文档
  async function saveAsWord() {
    // 将富文本内容拼接为一个完整的html
    const html = getHtml(mdPreviewRef)
    const tipsHtml = getHtml(tipsRef)
    const value = `
           ${html}
           <hr>
           <div style="font-size: 12px;color: red">
           ${tipsHtml}
            </div>
  <br>
  本报告由go-stock项目生成：
  <p>
  <a href="https://github.com/ArvinLovegood/go-stock">
  AI赋能股票分析：自选股行情获取，成本盈亏展示，涨跌报警推送，市场整体/个股情绪分析，K线技术指标分析等。数据全部保留在本地。支持DeepSeek，OpenAI， Ollama，LMStudio，AnythingLLM，硅基流动，火山方舟，阿里云百炼等平台或模型。
  </a></p>
  `
    // landscape就是横着的，portrait是竖着的，默认是竖屏portrait。
    const blob = await asBlob(value, {orientation: 'portrait'})
    const {platform} = await Environment()
    switch (platform) {
      case 'windows':
        const a = document.createElement('a')
        a.href = URL.createObjectURL(blob)
        a.download = `${data.name}[${data.code}]-ai-analysis-result.docx`;
        a.click()
        // 下载后将标签移除
        URL.revokeObjectURL(a.href);
        a.remove()
        break
      default:
        const arrayBuffer = await blob.arrayBuffer()
        const uint8Array = new Uint8Array(arrayBuffer)
        const binary = uint8Array.reduce((data, byte) => data + String.fromCharCode(byte), '')
        const base64 = btoa(binary)
        await stockApi.saveWordFile(`${data.name}[${data.code}]-ai-analysis-result.docx`, base64).then(({data: result}) => {
          message.success(result)
        })
    }
  }
  
  function share(code, name) {
    systemApi.shareAnalysis(code, name).then(({data: msg}) => {
      //message.info(msg)
      notify.info({
        avatar: () =>
            h(NAvatar, {
              size: 'small',
              round: false,
              src: icon.value
            }),
        title: '分享到社区',
        duration: 1000 * 30,
        content: () => {
          return h('div', {
            style: {
              'text-align': 'left',
              'font-size': '14px',
            }
          }, {default: () => msg})
        },
      })
    })
  }

  return {
    saveAsImage, copyToClipboard, agentTitle, buildFullReport, scrollToAiResultBottom,
    saveAsMarkdown, saveAsMarkdown_old, getHtml, saveAsWord, share,
  }
}
