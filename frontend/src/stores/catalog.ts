import { defineStore } from 'pinia'
import { ref } from 'vue'
import { mockProvider, type HomePayload } from '@/mocks/provider'

export const useCatalogStore = defineStore('catalog', () => {
  const status = ref<'idle'|'loading'|'ready'|'empty'|'error'>('idle')
  const error = ref('')
  const home = ref<HomePayload | null>(null)
  let token = 0
  /** mode 由 URL ?state=empty|error|loading 驱动，仅用于演示与测试 */
  async function loadHome(mode: 'normal'|'empty'|'error'|'loading' = 'normal') {
    const my = ++token // 取消过期请求
    status.value = 'loading'; error.value = ''
    if (mode === 'loading') return
    try {
      const data = await mockProvider.home(mode === 'normal' ? 'normal' : mode)
      if (my !== token) return
      home.value = data
      status.value = data.todayNew.length === 0 && data.banners.length === 0 ? 'empty' : 'ready'
    } catch (e) {
      if (my !== token) return
      error.value = (e as Error).message; status.value = 'error'
    }
  }
  return { status, error, home, loadHome }
})
