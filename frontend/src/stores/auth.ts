import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
// 演示账号：仅表示“本地演示会话”，不持有任何令牌，不代表官方红果账号（能力门禁 account_login = not_authorized）
import { saveSetting, LS } from '@/bindings'
const KEY = LS.auth
export const useAuthStore = defineStore('auth', () => {
  let init = { demo: false, name: '' }
  try { const r = localStorage.getItem(KEY); if (r) init = JSON.parse(r) } catch { /* ignore */ }
  const demo = ref(init.demo); const name = ref(init.name)
  watch([demo, name], () => saveSetting(LS.auth, { demo: demo.value, name: name.value }))
  const loginDemo = (phone?: string) => { demo.value = true; name.value = phone ? `演示用户${phone.slice(-4)}` : '演示用户' }
  const logout = () => { demo.value = false; name.value = '' }
  return { demo, name, loginDemo, logout }
})
export const DEMO_CODE = '123456'
export const isValidPhone = (p: string) => /^1[3-9]\d{9}$/.test(p)
export const isValidCode = (c: string) => /^\d{6}$/.test(c)
