import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { saveSetting, LS } from '@/bindings'
const KEY = LS.app
export interface AppSettings { autostart: boolean; startPage: string; minimize: string; closeAction: string; defaultQuality: string; playMode: string; decode: 'hw' | 'sw'; defaultSpeed: number; dlQuality: string; dlConcurrent: number; cacheLimitGB: number; cachePath: string; mediaSource: 'demo' | 'sim'; theme: 'dark' | 'light' | 'system'; dpi: number; fontSize: string; shortcuts: Record<string, string> }
export const DEFAULT_SHORTCUTS: Record<string, string> = { playPause: '空格', forward: '→', back: '←', next: 'Ctrl + →', prev: 'Ctrl + ←', fullscreen: 'F', danmaku: 'D', screenshot: 'Ctrl + S' }
export const SHORTCUT_LABELS: Record<string, string> = { playPause: '播放 / 暂停', forward: '快进 10 秒', back: '快退 10 秒', next: '下一集', prev: '上一集', fullscreen: '全屏 / 退出全屏', danmaku: '开启 / 关闭弹幕', screenshot: '截图（保存到本地）' }
export const DEFAULTS: AppSettings = { autostart: false, startPage: '上次观看', minimize: '显示在任务栏', closeAction: '退出程序', defaultQuality: '高清 1080P', playMode: '连续播放（自动下一集）', decode: 'hw', defaultSpeed: 1, mediaSource: 'demo', dlQuality: '高清 1080P', dlConcurrent: 3, cacheLimitGB: 2, cachePath: 'JindiDrama/Cache/Downloads', theme: 'dark', dpi: 100, fontSize: '标准', shortcuts: { ...DEFAULT_SHORTCUTS } }
export function normalizeKey(e: { key: string; ctrlKey: boolean; shiftKey: boolean; altKey: boolean; metaKey: boolean }): string | null {
  if (['Control', 'Shift', 'Alt', 'Meta'].includes(e.key)) return null
  const k = e.key === ' ' ? '空格' : e.key === 'ArrowRight' ? '→' : e.key === 'ArrowLeft' ? '←' : e.key === 'ArrowUp' ? '↑' : e.key === 'ArrowDown' ? '↓' : e.key.length === 1 ? e.key.toUpperCase() : e.key
  return [e.ctrlKey || e.metaKey ? 'Ctrl' : '', e.altKey ? 'Alt' : '', e.shiftKey ? 'Shift' : '', k].filter(Boolean).join(' + ')
}
export const useAppSettings = defineStore('appSettings', () => {
  let init: AppSettings = { ...DEFAULTS, shortcuts: { ...DEFAULT_SHORTCUTS } }
  try { const r = localStorage.getItem(KEY); if (r) { const o = JSON.parse(r); init = { ...init, ...o, shortcuts: { ...DEFAULT_SHORTCUTS, ...(o.shortcuts ?? {}) } } } } catch { /* ignore */ }
  if (!['最小化到任务栏', '退出程序'].includes(init.closeAction)) init.closeAction = '退出程序' // 旧版“最小化到托盘”没有对应实现
  const s = ref<AppSettings>(init)
  watch(s, () => { saveSetting(LS.app, s.value); applyTheme() }, { deep: true })
  function applyTheme() {
    const sys = typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: light)').matches
    document.documentElement.dataset.theme = s.value.theme === 'system' ? (sys ? 'light' : 'dark') : s.value.theme
    document.documentElement.style.setProperty('--ui-font-scale', s.value.fontSize === '大' ? '1.08' : s.value.fontSize === '小' ? '0.94' : '1')
    document.documentElement.style.setProperty('--user-zoom', String(s.value.dpi / 100))
  }
  /** 返回冲突的动作名（若该组合已被占用） */
  function rebind(action: string, combo: string): string | null { const dup = Object.entries(s.value.shortcuts).find(([a, c]) => a !== action && c === combo); if (dup) return SHORTCUT_LABELS[dup[0]]; s.value.shortcuts[action] = combo; return null }
  const reset = () => { s.value = { ...DEFAULTS, shortcuts: { ...DEFAULT_SHORTCUTS } } }
  applyTheme() // 启动即应用已保存的主题/缩放/字体，而不是等到打开设置页
  return { s, rebind, reset, applyTheme }
})
