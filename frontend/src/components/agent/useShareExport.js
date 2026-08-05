/**
 * AI 内容的复制/社区分享/导出图片（自 FloatingAgentAssistant.vue 原样搬迁）。
 * messages/darkTheme 经 ctx 传入（ref 共享引用）。
 */
import { ref, nextTick } from 'vue'
import { useMessage } from 'naive-ui'
import html2canvas from 'html2canvas'
import * as systemApi from '../../api/system'
import * as stockApi from '../../api/stock'

export function useShareExport(ctx) {
  const { messages, darkTheme } = ctx
  const message = useMessage()

  const shareLoading = ref(false)
  const exportImageKey = ref('')
  const shareTipVisible = ref(false)
  const shareTipText = ref('')

  async function copyAiContent(msg) {
    const text = (msg?.content ?? '').trim()
    if (!text) {
      message.warning('暂无可复制的 AI 正文内容')
      return
    }
    try {
      if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text)
        message.success('已复制 AI 回答内容')
      } else {
        const textarea = document.createElement('textarea')
        textarea.value = text
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
        message.success('已复制 AI 回答内容')
      }
    } catch (e) {
      message.error('复制失败，请手动选择文本')
    }
  }

  function shareTextToCommunity(text, title) {
    if (shareLoading.value) return
    shareLoading.value = true
    shareTipText.value = '正在分享到社区...'
    shareTipVisible.value = true
    systemApi.shareText(text, title)
      .then(({data: msg}) => {
        shareTipText.value = msg
        shareTipVisible.value = true
      })
      .catch((err) => {
        shareTipText.value = '分享失败: ' + (err?.message ?? err)
        shareTipVisible.value = true
      })
      .finally(() => {
        shareLoading.value = false
      })
  }

  function shareAiContent(msg) {
    const text = (msg?.content ?? '').trim()
    if (!text) {
      shareTipText.value = '暂无可分享的 AI 正文内容'
      shareTipVisible.value = true
      return
    }
    shareTextToCommunity(text, 'go-stock AI Agent助手')
  }

  function getLastAssistantContent() {
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const m = messages.value[i]
      if (m?.role === 'assistant') {
        const text = (m?.content ?? '').trim()
        if (text) return text
      }
    }
    return ''
  }

  function shareAiToCommunity() {
    const text = getLastAssistantContent()
    if (!text) {
      shareTipText.value = '暂无可分享的 AI 回复内容'
      shareTipVisible.value = true
      return
    }
    shareTextToCommunity(text, 'go-stock AI Agent助手')
  }

  async function exportAiReplyImage(assistantIndex, evt) {
    const msg = messages.value[assistantIndex]
    if (msg?.role !== 'assistant') return
    if (!(msg.content ?? '').trim()) {
      shareTipText.value = '暂无可导出的 AI 回答内容'
      shareTipVisible.value = true
      return
    }
    const editorId = 'agent-msg-' + assistantIndex
    const bubble = evt?.currentTarget?.closest?.('.msg-bubble')
    const key = String(assistantIndex)
    if (exportImageKey.value) return
    exportImageKey.value = key
    await nextTick()
    try {
      const target = document.getElementById(`${editorId}-preview-wrapper`) ||
        document.getElementById(`${editorId}-preview`) ||
        bubble?.querySelector('.md-editor-preview') ||
        null
      if (!target) {
        shareTipText.value = '未找到预览区域，请展开回答后重试'
        shareTipVisible.value = true
        return
      }
      const savedStyles = []
      const overflowParents = []
      let el = target.parentElement
      while (el && el !== document.body) {
        const style = getComputedStyle(el)
        if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
          savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
          overflowParents.push(el)
          el.style.overflow = 'visible'
          el.style.overflowY = 'visible'
          el.style.height = 'auto'
          el.style.maxHeight = 'none'
        }
        el = el.parentElement
      }
      const savedTargetStyle = { height: target.style.height, maxHeight: target.style.maxHeight, overflow: target.style.overflow, overflowY: target.style.overflowY }
      target.style.height = 'auto'
      target.style.maxHeight = 'none'
      target.style.overflow = 'visible'
      target.style.overflowY = 'visible'
      await nextTick()
      const canvas = await html2canvas(target, {
        useCORS: true,
        scale: 2,
        allowTaint: true,
        logging: false,
        backgroundColor: darkTheme.value ? '#1e1e1e' : '#ffffff'
      })
      target.style.height = savedTargetStyle.height
      target.style.maxHeight = savedTargetStyle.maxHeight
      target.style.overflow = savedTargetStyle.overflow
      target.style.overflowY = savedTargetStyle.overflowY
      savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
        el.style.overflow = overflow
        el.style.overflowY = overflowY
        el.style.height = height
        el.style.maxHeight = maxHeight
      })
      const dataUrl = canvas.toDataURL('image/png')
      const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
      const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')
      const result = (await stockApi.saveImage(`go-stock-agent-${safeTime}`, base64)).data
      if (result && !result.includes('异常') && !result.includes('无法')) {
        shareTipText.value = '已导出为 PNG 图片：' + result
      } else {
        shareTipText.value = result || '导出取消'
      }
      shareTipVisible.value = true
    } catch (e) {
      shareTipText.value = '导出图片失败: ' + (e?.message ?? e)
      shareTipVisible.value = true
    } finally {
      exportImageKey.value = ''
    }
  }

  return {
    shareLoading, exportImageKey, shareTipVisible, shareTipText,
    copyAiContent, shareTextToCommunity, shareAiContent,
    getLastAssistantContent, shareAiToCommunity, exportAiReplyImage,
  }
}
