/**
 * 页面操作埋点与报错上报（写入后端应用日志，[FRONTEND] 前缀）。
 *
 * 直接走 window.go 调用 SystemHandler.TrackEvent，不经 api 层，
 * 避免 client.ts ↔ tracker 循环依赖；上报本身失败时静默，防止递归。
 */

const LEVEL = { INFO: 'info', ERROR: 'error' }
const RESIZE_OBSERVER = 'ResizeObserver'

let currentRoute = 'unknown'
let lastKey = ''
let lastAt = 0

function emit(level, source, action, detail) {
  try {
    const handler = typeof window !== 'undefined' && window.go?.handler?.SystemHandler
    if (!handler?.TrackEvent) return
    handler.TrackEvent(level, String(source ?? ''), String(action ?? ''), String(detail ?? ''))
  } catch (_) {
    /* 上报失败静默 */
  }
}

function shouldDedupe(key) {
  const now = Date.now()
  if (key === lastKey && now - lastAt < 800) return true
  lastKey = key
  lastAt = now
  return false
}

/** 记录一次页面操作（info 级）。相同 source+action 800ms 内去重。 */
export function track(action, detail = '', source = '') {
  const key = source + '|' + action
  if (shouldDedupe(key)) return
  emit(LEVEL.INFO, source || currentRoute, action, detail)
}

/** 上报一次操作报错（error 级），含消息与截断后的堆栈。 */
export function reportError(source, err) {
  if (!err) return
  const message = err?.message ?? String(err)
  if (message.includes(RESIZE_OBSERVER)) return
  const stack = err?.stack ? String(err.stack).slice(0, 600) : ''
  emit(LEVEL.ERROR, source, message, stack)
}

/**
 * 安装全局埋点：Vue 渲染错误、window error、未捕获 Promise 拒绝、
 * 路由跳转、按钮点击。在 main.js 的 app 创建后调用一次。
 */
export function installGlobalTracking(app, router) {
  if (router) {
    router.afterEach((to) => {
      currentRoute = String(to.name ?? to.path ?? 'unknown')
      emit(LEVEL.INFO, 'router', 'navigate', to.fullPath)
    })
  }

  // Vue 渲染/生命周期错误（页面操作报错的主要来源）
  app.config.errorHandler = (err, _instance, info) => {
    if (err?.message?.includes(RESIZE_OBSERVER)) return
    console.error(err)
    emit(LEVEL.ERROR, 'vue:' + info, err?.message ?? String(err), err?.stack?.slice(0, 600) ?? '')
  }

  window.addEventListener('error', (event) => {
    if (event.message && event.message.includes(RESIZE_OBSERVER)) {
      event.preventDefault()
      return true
    }
    reportError('window', event.error ?? new Error(event.message))
  })

  window.addEventListener('unhandledrejection', (event) => {
    if (event.reason?.message?.includes(RESIZE_OBSERVER)) {
      event.preventDefault()
      return true
    }
    reportError('promise', event.reason)
  })

  // 按钮点击埋点（捕获阶段；naive-ui 按钮最终渲染为 <button>）
  document.addEventListener(
    'click',
    (event) => {
      const btn = event.target?.closest?.('button')
      if (!btn) return
      const text = (btn.textContent ?? '').trim().replace(/\s+/g, ' ').slice(0, 60)
      if (!text) return
      track('click:' + text, '', 'ui')
    },
    true,
  )

  // 手动上报出口（供控制台/异常兜底使用）
  window.__goStockReportError = reportError
  window.__goStockTrack = track
}
