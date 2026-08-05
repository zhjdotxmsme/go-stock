<script setup>
import { onBeforeMount, onMounted, ref } from 'vue'
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
const contentStyle = ref('')

let mottoTimer = null

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

  // 设置内容区域样式
  contentStyle.value = 'max-height: calc(92vh);overflow: hidden'

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
              <n-flex>
                <n-grid x-gap="12" :cols="1">
                  <n-gi>
                    <n-spin :show="appStore.loading">
                      <template #description>
                        {{ appStore.loadingMsg }}
                      </template>
                      <n-marquee :speed="100" style="position: relative;top:0;z-index: 19;width: 100%"
                                 v-if="(telegraph.length>0)&&(appStore.enableNews)">
                        <n-tag type="warning" v-for="item in telegraph" style="margin-right: 10px">
                          {{ item }}
                        </n-tag>
                      </n-marquee>
                      <n-scrollbar :style="contentStyle">
                        <n-skeleton v-if="appStore.loading" height="calc(100vh)" />
                        <RouterView/>
                      </n-scrollbar>
                    </n-spin>
                  </n-gi>
                  <n-gi style="position: fixed;bottom:0;z-index: 9;width: 100%;">
                    <n-card size="small" style="--wails-draggable:no-drag">
                      <n-menu style="font-size: 18px;"
                              v-model:value="activeKey"
                              mode="horizontal"
                              :options="menuOptions"
                              responsive
                      />
                    </n-card>
                  </n-gi>
                </n-grid>
              </n-flex>
            </n-watermark>
          </n-dialog-provider>
        </n-modal-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>
<style>

</style>
