import {createApp} from 'vue'
import {createPinia} from 'pinia'
import naive from 'naive-ui'
import App from './App.vue'
import router from './router/router'
import {installGlobalTracking} from './utils/tracker'
// 引入组件库的少量全局样式变量
import 'tdesign-vue-next/es/style/index.css';

const app = createApp(App)

// 全局埋点与报错上报：Vue 错误、window error、未捕获 Promise、路由跳转、
// 按钮点击，统一经 SystemHandler.TrackEvent 写入后端应用日志（[FRONTEND] 前缀）。
installGlobalTracking(app, router)

// 注册 Pinia 状态管理
const pinia = createPinia()
app.use(pinia)

app.use(router)
app.use(naive)
app.mount('#app')