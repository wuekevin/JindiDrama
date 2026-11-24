import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import './styles/base.scss'
import { prime } from './bindings'
import { useAppSettings } from './stores/appSettings'

// 按窗口相对设计稿 1448×1086 的比例双向缩放（0.82–2.2）：放大变大、缩小变小，布局宽高随之重排。
const BASE = 1.05 // 整体基线放大 5%，让默认字号更易读
function applyZoom() {
  const z = Math.min(2.2, Math.max(0.82, BASE * Math.min(window.innerWidth / 1448, window.innerHeight / 1086)))
  document.documentElement.style.setProperty('--app-zoom', String(z))
}
applyZoom(); window.addEventListener('resize', applyZoom)
prime().finally(() => {
  const pinia = createPinia()
  const app = createApp(App).use(pinia).use(router)
  useAppSettings(pinia) // 启动即实例化：应用已保存的主题/缩放/字体
  app.mount('#app')
})
