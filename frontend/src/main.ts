import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import './styles/base.scss'
import { prime } from './bindings'

// 窗口 > 1448×1086 时按比例放大（zoom），小于时保持 1:1 并滚动内容区。
function applyZoom() {
  const z = Math.min(2, Math.max(1, Math.min(window.innerWidth / 1448, window.innerHeight / 1086)))
  document.documentElement.style.setProperty('--app-zoom', String(z))
}
applyZoom(); window.addEventListener('resize', applyZoom)
prime().finally(() => createApp(App).use(createPinia()).use(router).mount('#app'))
