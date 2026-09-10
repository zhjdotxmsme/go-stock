<script setup>
import { nextTick, onBeforeMount, onBeforeUnmount, onMounted, ref } from 'vue'
import { EventsEmit } from '../wailsjs/runtime'
import { darkTheme, lightTheme, dateZhCN, zhCN } from 'naive-ui'
import { GetConfig, GetVersionInfo } from '../wailsjs/go/handler/SystemHandler'
import FloatingAgentAssistant from './components/FloatingAgentAssistant.vue'

// ========== Composables & Stores ==========
import { useAppStore } from './stores'
import { useNavigation } from './composables/useNavigation'
import { useMarketStatus } from './composables/useMarketStatus'
import { useWailsEvents } from './composables/useWailsEvents'

const appStore = useAppStore()

// 导航管理：菜单配置、activeKey、全屏
const { activeKey, menuOptions, loadDynamicMenus, applyConfigVisibility } = useNavigation()

// 市场状态：交易时间检查 + 窗口标题更新（60秒间隔）
const marketStatus = useMarketStatus({ interval: 60000 })

// Wails 事件：realtime_profit、telegraph、loadingMsg
const { registerEvents, registerNewsPush, telegraph } = useWailsEvents()

// ========== 局部状态 ==========
const containerRef = ref({})
const bottomBarRef = ref(null)
/**
 * 底部菜单栏实测高度，写入 --app-bottombar-h。
 * 页面内的右下角浮动组件用它把自己抬到菜单栏之上，避免相互遮挡。
 */
const bottomBarHeight = ref(68)

let mottoTimer = null
let bottomBarObserver = null

// ========== 8 秒加载超时保护 ==========
setTimeout(() => {
  if (appStore.loading) {
    appStore.setLoading(false, '加载完成...')
    EventsEmit('loadingDone', 'app')
  }
}, 8000)

// ========== 全局错误上报 ==========
window.onerror = function (msg, source, lineno, colno, error) {
  EventsEmit('frontendError', {
    page: 'App.vue',
    message: msg,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null,
  })
  return true
}

// ========== 生命周期 ==========

onBeforeMount(() => {
  // 获取版本信息 → 设置官方声明
  GetVersionInfo()
    .then((result) => {
      if (result.officialStatement) {
        appStore.content = result.officialStatement + '\n\n' + appStore.content
      }
      appStore.setOfficialStatement(result.officialStatement || '')
    })
    .catch((err) => console.error('GetVersionInfo error:', err))

  // 加载动态群组菜单
  loadDynamicMenus()

  // 获取配置 → 设置功能开关（基金/智能体/暗色主题）
  GetConfig()
    .then((res) => {
      applyConfigVisibility(res)

      if (res.darkTheme) {
        appStore.enableDarkTheme = darkTheme
      } else {
        appStore.enableDarkTheme = null
      }
    })
    .catch((err) => console.error('GetConfig error:', err))
})

onMounted(() => {
  // 注册 Wails 事件监听（realtime_profit、telegraph、loadingMsg）
  registerEvents()

  // 测量底部菜单栏实际高度（菜单项换行/字体变化时自动更新）
  measureBottomBar()
  if (typeof ResizeObserver !== 'undefined') {
    bottomBarObserver = new ResizeObserver(() => measureBottomBar())
    nextTick(() => {
      if (bottomBarRef.value) bottomBarObserver.observe(bottomBarRef.value)
    })
  }

  // 获取配置 → 启用新闻推送通知
  GetConfig()
    .then((res) => {
      if (res.enableNews) {
        appStore.enableNews = true
        registerNewsPush(appStore.enableDarkTheme)
      }
    })
    .catch((err) => console.error('GetConfig(onMounted) error:', err))

  // 60 秒定时刷新格言
  mottoTimer = setInterval(() => {
    appStore.refreshMotto()
  }, 60000)
})

/** 读取底部菜单栏高度并写入 CSS 变量 */
function measureBottomBar() {
  const el = bottomBarRef.value
  if (el && el.offsetHeight > 0) {
    bottomBarHeight.value = el.offsetHeight
  }
}

onBeforeUnmount(() => {
  if (bottomBarObserver) {
    bottomBarObserver.disconnect()
    bottomBarObserver = null
  }
  if (mottoTimer) {
    clearInterval(mottoTimer)
    mottoTimer = null
  }
})
</script>

<template>
  <n-config-provider ref="containerRef" :theme="appStore.enableDarkTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-notification-provider>
        <n-modal-provider>
          <n-dialog-provider>
            <n-watermark
                :content="''"
                cross
                selectable
                :font-size="16"
                :line-height="16"
                :width="500"
                :height="400"
                :x-offset="50"
                :y-offset="150"
                :rotate="-15"
            >
              <FloatingAgentAssistant />
              <!--
                app-shell：内容区 + 底部菜单栏 的纵向 flex 布局。
                底部菜单栏不再使用 position:fixed 浮层（旧实现会盖住内容区最后 ~10vh
                以及页面右下角的浮动组件），改为占据真实布局空间，彻底消除互相覆盖。
              -->
              <div class="app-shell" :style="{ '--app-bottombar-h': bottomBarHeight + 'px' }">
                <div class="app-content">
                  <n-spin :show="appStore.loading">
                    <template #description>
                      {{ appStore.loadingMsg }}
                    </template>
                    <n-marquee v-if="(telegraph.length>0)&&(appStore.enableNews)"
                               :speed="100" class="app-marquee">
                      <n-tag type="warning" v-for="item in telegraph" style="margin-right: 10px">
                        {{ item }}
                      </n-tag>
                    </n-marquee>
                    <n-scrollbar class="app-scroll">
                      <n-skeleton v-if="appStore.loading" height="calc(100vh)" />
                      <RouterView/>
                    </n-scrollbar>
                  </n-spin>
                </div>
                <div ref="bottomBarRef" class="app-bottom-bar">
                  <n-card size="small" style="--wails-draggable:no-drag">
                    <n-menu style="font-size: 18px;"
                            v-model:value="activeKey"
                            mode="horizontal"
                            :options="menuOptions"
                            responsive
                    />
                  </n-card>
                </div>
              </div>
            </n-watermark>
          </n-dialog-provider>
        </n-modal-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>
<style>
/*
 * 底部菜单栏实测高度（App.vue 用 ResizeObserver 实时写入 --app-bottombar-h）。
 * 这里是首屏渲染前的兜底值，也供页面内 position:fixed 的右下角浮动组件
 * （自选关注框 / AI总结按钮 / K线搜索框）把自己抬到菜单栏之上，避免互相覆盖。
 */
:root {
  --app-bottombar-h: 68px;
}

/* ===== 应用外壳：内容区 + 底部菜单栏（真实布局，非浮层） ===== */
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.app-content {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

/* n-spin 的内部结构需要撑满，内容区才能正确滚动 */
.app-content > .n-spin-container,
.app-content .n-spin-content {
  height: 100%;
  min-height: 0;
}

.app-content .n-spin-content {
  display: flex;
  flex-direction: column;
}

.app-marquee {
  position: relative;
  flex: 0 0 auto;
  width: 100%;
  z-index: 19;
}

.app-scroll {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
}

/*
 * naive-ui 默认给加载中的内容加 pointer-events:none。本应用把 loading 用在
 * 后台数据刷新上（A股/港股/美股基础信息），一旦为 true，整个内容区都会「点不动」，
 * 点击还会穿透到下方的固定元素上，表现为「按钮点了没反应」。
 * 这里放开指针事件，只保留视觉上的变暗提示。
 */
.app-content .n-spin-content--spinning {
  pointer-events: all;
}

.app-bottom-bar {
  flex: 0 0 auto;
  width: 100%;
}
</style>
