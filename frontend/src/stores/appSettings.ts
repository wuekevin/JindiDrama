import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { isPlaybackSpeed, normalizePlayMode, type PlayMode } from '@/player/preferences'
import { normalizeDemoQuality, type DemoQuality } from '@/player/demoMedia'
import { loadCachedSetting, saveSetting, setWindowCloseSetting, LS, windowTrayAvailable } from '@/bindings'
const KEY = LS.app
export const CLOSE_TO_TRAY = '隐藏到菜单栏'
const windowsTray = /Windows/.test(navigator.userAgent)
export const CLOSE_TO_TRAY_LABEL = windowsTray ? '隐藏到系统托盘' : CLOSE_TO_TRAY
export const TRAY_UNAVAILABLE_MESSAGE = windowsTray ? '系统托盘暂不可用，已最小化窗口' : '菜单栏暂不可用，已最小化窗口'
export interface AppSettings { autostart: boolean; startPage: string; minimize: string; closeAction: string; defaultQuality: DemoQuality; playMode: PlayMode; decode: 'hw' | 'sw'; defaultSpeed: number; dlQuality: DemoQuality; dlConcurrent: number; cacheLimitGB: number; cachePath: string; mediaSource: 'demo' | 'sim'; theme: 'dark' | 'light' | 'system'; dpi: number; fontSize: string; shortcuts: Record<string, string> }
export const DEFAULT_SHORTCUTS: Record<string, string> = { playPause: '空格', forward: '→', back: '←', next: 'Ctrl + →', prev: 'Ctrl + ←', fullscreen: 'F', danmaku: 'D', screenshot: 'Ctrl + S' }
export const SHORTCUT_LABELS: Record<string, string> = { playPause: '播放 / 暂停', forward: '快进 10 秒', back: '快退 10 秒', next: '下一集', prev: '上一集', fullscreen: '全屏 / 退出全屏', danmaku: '开启 / 关闭弹幕', screenshot: '截图（保存到本地）' }
export const DEFAULTS: AppSettings = { autostart: false, startPage: '上次观看', minimize: '显示在任务栏', closeAction: '退出程序', defaultQuality: '高清', playMode: '连续播放（自动下一集）', decode: 'hw', defaultSpeed: 1, mediaSource: 'demo', dlQuality: '高清', dlConcurrent: 3, cacheLimitGB: 2, cachePath: 'JindiDrama/Cache/Downloads', theme: 'dark', dpi: 100, fontSize: '标准', shortcuts: { ...DEFAULT_SHORTCUTS } }
function recordableKey(key: string, modified: boolean) {
  return !!key && !['Control', 'Shift', 'Alt', 'Meta', 'Tab', 'Escape', 'Dead', 'Process', 'Unidentified'].includes(key) && (key !== 'Enter' || modified)
}
export function normalizeKey(e: { key: string; code?: string; ctrlKey: boolean; shiftKey: boolean; altKey: boolean; metaKey: boolean; isComposing?: boolean }): string | null {
  const modified = e.ctrlKey || e.metaKey || e.altKey || e.shiftKey
  const controlLetter = (e.ctrlKey || e.metaKey) && /^[\u0000-\u001f]$/.test(e.key) && /^Key[A-Z]$/.test(e.code ?? '')
  const key = controlLetter ? e.code!.slice(3) : e.key
  if (e.isComposing || !recordableKey(key, modified)) return null
  const k = key === ' ' ? '空格' : key === 'ArrowRight' ? '→' : key === 'ArrowLeft' ? '←' : key === 'ArrowUp' ? '↑' : key === 'ArrowDown' ? '↓' : key.length === 1 ? key.toUpperCase() : key
  return [e.ctrlKey || e.metaKey ? 'Ctrl' : '', e.altKey ? 'Alt' : '', e.shiftKey ? 'Shift' : '', k].filter(Boolean).join(' + ')
}

function restoreShortcuts(saved: Record<string, unknown> = {}) {
  const shortcuts: Record<string, string> = {}
  const used = new Set<string>()
  for (const action of Object.keys(DEFAULT_SHORTCUTS)) {
    const old = saved[action]
    if (typeof old !== 'string') continue
    // WebKit 旧版曾将 Ctrl + 字母记录成 ASCII 控制字符。
    const combo = old.startsWith('Ctrl + ')
      ? old.replace(/[\u0001-\u001a]$/, (key) => String.fromCharCode(key.charCodeAt(0) + 64))
      : old
    if (combo === '') {
      shortcuts[action] = ''
      continue
    }
    const parts = combo.split(' + ')
    if (!recordableKey(parts.at(-1) ?? '', parts.length > 1) || used.has(combo)) continue
    shortcuts[action] = combo
    used.add(combo)
  }
  for (const [action, combo] of Object.entries(DEFAULT_SHORTCUTS)) {
    if (action in shortcuts) continue
    shortcuts[action] = used.has(combo) ? '' : combo
  }
  return shortcuts
}
export const useAppSettings = defineStore('appSettings', () => {
  let init: AppSettings = { ...DEFAULTS, shortcuts: { ...DEFAULT_SHORTCUTS } }
  try { const r = loadCachedSetting(KEY); if (r) { const o = JSON.parse(r); init = { ...init, ...o, shortcuts: restoreShortcuts(o.shortcuts ?? {}) } } } catch { /* ignore */ }
  if (!['最小化到任务栏', '退出程序', CLOSE_TO_TRAY].includes(init.closeAction)) init.closeAction = '退出程序'
  if (!isPlaybackSpeed(init.defaultSpeed)) init.defaultSpeed = DEFAULTS.defaultSpeed
  init.playMode = normalizePlayMode(init.playMode)
  init.defaultQuality = normalizeDemoQuality(init.defaultQuality)
  init.dlQuality = normalizeDemoQuality(init.dlQuality)
  const s = ref<AppSettings>(init)
  const canCloseToTray = ref(false)
  windowTrayAvailable()?.then((supported) => { canCloseToTray.value = supported }).catch(() => {})
  watch(s, () => { saveSetting(LS.app, s.value); applyTheme() }, { deep: true })
  function applyTheme() {
    const sys = typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: light)').matches
    document.documentElement.dataset.theme = s.value.theme === 'system' ? (sys ? 'light' : 'dark') : s.value.theme
    document.documentElement.style.setProperty('--ui-font-scale', s.value.fontSize === '大' ? '1.08' : s.value.fontSize === '小' ? '0.94' : '1')
    document.documentElement.style.setProperty('--user-zoom', String(s.value.dpi / 100))
  }
  /** 返回冲突的动作名（若该组合已被占用） */
  function rebind(action: string, combo: string): string | null { const dup = Object.entries(s.value.shortcuts).find(([a, c]) => a !== action && c === combo); if (dup) return SHORTCUT_LABELS[dup[0]]; s.value.shortcuts[action] = combo; return null }
  async function setCloseAction(action: string) {
    await setWindowCloseSetting(action)
    s.value.closeAction = action
  }
  const reset = () => { s.value = { ...DEFAULTS, shortcuts: { ...DEFAULT_SHORTCUTS } } }
  applyTheme() // 启动即应用已保存的主题/缩放/字体，而不是等到打开设置页
  return { s, canCloseToTray, setCloseAction, rebind, reset, applyTheme }
})
