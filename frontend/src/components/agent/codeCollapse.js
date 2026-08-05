/**
 * Markdown 渲染后处理：长代码块/JSON 代码块自动折叠（自 FloatingAgentAssistant.vue 原样搬迁）。
 * 直接操作渲染出的 DOM（.msg-markdown .md-editor-code-block），与组件位置无关。
 */
import { nextTick } from 'vue'

export function onMdHtmlChanged() {
  nextTick(() => {
    document.querySelectorAll('.msg-markdown .md-editor-code-block').forEach(block => {
      if (block.querySelector('.code-collapse-btn')) return
      const codeEl = block.querySelector('code')
      if (!codeEl) return
      const lang = (codeEl.className || '').toLowerCase()
      const isJson = lang.includes('json') || lang.includes('language-json')
      const text = codeEl.textContent || ''
      const lineCount = text.split('\n').length
      if (!isJson && lineCount <= 8) return

      block.classList.add('code-collapsed')
      const btn = document.createElement('span')
      btn.className = 'code-collapse-btn'
      btn.textContent = '展开'
      btn.addEventListener('click', (e) => {
        e.stopPropagation()
        const collapsed = block.classList.toggle('code-collapsed')
        btn.textContent = collapsed ? '展开' : '收起'
      })
      block.appendChild(btn)
    })
  })
}
