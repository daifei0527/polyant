import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'

import App from './App.vue'
import router from './router'
import { useAdminStore } from './stores/admin'
import './styles/index.scss'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 刷新后用 token 重新校验会话并恢复用户信息（失败则登出，下次导航跳回登录）。
// 不阻塞挂载——路由守卫已从 sessionStorage 同步读到 userLevel。
useAdminStore().restoreSession()

app.mount('#app')
