// Wails Go 绑定桥：桌面端数据写入本地 SQLite；浏览器预览（无 Go 运行时）退回 localStorage。
import type { HistoryEntry } from '@/types'
interface GoApp {
  StorageReady(): Promise<boolean>; Favorites(): Promise<string[]>; SaveFavorites(ids: string[]): Promise<void>
  History(): Promise<{ dramaId: string; episode: number; positionMs: number; durationMs: number; percent: number; watchedAt: number }[]>
  RecordHistory(h: unknown): Promise<void>; RemoveHistory(ids: string[]): Promise<void>; ClearHistory(): Promise<void>
  SearchHistory(): Promise<string[]>; SaveSearchHistory(q: string[]): Promise<void>
  CacheStats(): Promise<{ bytes: number; files: number; max: number }>; CacheClear(): Promise<void>; CacheSetLimitMB(mb: number): Promise<void>
  AppVersion(): Promise<string>; CheckUpdate(): Promise<UpdateResult>; OpenReleasePage(url: string): Promise<void>
  LogEvent(level: string, component: string, msg: string): Promise<void>; DiagnosticsTail(n: number): Promise<string[]>
  GetSetting(k: string): Promise<string>; SaveSetting(k: string, v: string): Promise<void>; BackupNow(): Promise<string>
}
const api = (): GoApp | null => ((window as any).go?.app?.App as GoApp | undefined) ?? null
export const hasBridge = () => api() !== null
const warn = (e: unknown) => console.warn('[bridge]', e)

export const LS = { library: 'jindi.library.v1', search: 'jindi.search.v1', app: 'jindi.settings.v1', player: 'jindi.player.v1', auth: 'jindi.auth.v1' }
const SETTING_KEY: Record<string, string> = { [LS.app]: 'app', [LS.player]: 'player', [LS.auth]: 'auth_demo' }

/** 启动时把 SQLite 中的数据灌入 localStorage 缓存，随后各 store 按原有逻辑初始化。 */
export async function prime(): Promise<void> {
  const a = api(); if (!a) return
  try {
    if (!(await a.StorageReady())) return
    const [fav, hist, sh] = await Promise.all([a.Favorites(), a.History(), a.SearchHistory()])
    const empty = fav.length === 0 && hist.length === 0
    const seeded = localStorage.getItem('jindi.seeded') === '1'
    if (!empty || seeded) { // 数据库已有数据（或用户已清空过）→ 以数据库为准
      const history: HistoryEntry[] = hist.map((h) => ({ dramaId: h.dramaId, episode: h.episode, percent: h.percent, watchedAt: h.watchedAt, positionSec: h.positionMs / 1000, durationSec: h.durationMs / 1000 }))
      localStorage.setItem(LS.library, JSON.stringify({ history, favorites: fav }))
      localStorage.setItem(LS.search, JSON.stringify(sh))
    } else localStorage.removeItem(LS.library)
    for (const [ls, key] of Object.entries(SETTING_KEY)) { const v = await a.GetSetting(key); if (v) localStorage.setItem(ls, v) }
    localStorage.setItem('jindi.seeded', '1')
  } catch (e) { warn(e) }
}

let lastHist = new Map<string, string>()
let timer: number | undefined
export function saveLibrary(history: HistoryEntry[], favorites: string[]) {
  try { localStorage.setItem(LS.library, JSON.stringify({ history, favorites })) } catch { /* ignore */ }
  const a = api(); if (!a) return
  clearTimeout(timer)
  timer = window.setTimeout(async () => {
    try {
      await a.SaveFavorites(favorites)
      const now = new Map(history.map((h) => [h.dramaId, JSON.stringify(h)]))
      const removed = [...lastHist.keys()].filter((k) => !now.has(k)); if (removed.length) await a.RemoveHistory(removed)
      for (const h of history) if (lastHist.get(h.dramaId) !== now.get(h.dramaId)) await a.RecordHistory({ dramaId: h.dramaId, episode: h.episode, positionMs: Math.round((h.positionSec ?? 0) * 1000), durationMs: Math.round((h.durationSec ?? 0) * 1000), percent: h.percent, watchedAt: h.watchedAt })
      lastHist = now
    } catch (e) { warn(e) }
  }, 250)
}
export function saveSearch(q: string[]) { try { localStorage.setItem(LS.search, JSON.stringify(q)) } catch { /* ignore */ } api()?.SaveSearchHistory(q).catch(warn) }
export function saveSetting(lsKey: string, value: unknown) {
  const s = JSON.stringify(value); try { localStorage.setItem(lsKey, s) } catch { /* ignore */ }
  const k = SETTING_KEY[lsKey]; if (k) api()?.SaveSetting(k, s).catch(warn)
}
export const cacheStats = () => api()?.CacheStats() ?? Promise.resolve(null)
export const cacheClear = () => api()?.CacheClear() ?? Promise.resolve()
export const cacheSetLimit = (gb: number) => api()?.CacheSetLimitMB(Math.round(gb * 1024)) ?? Promise.resolve()
/** 故障上报：Go 侧统一脱敏后落盘；浏览器模式仅输出到控制台。 */
export function logEvent(level: 'info' | 'warn' | 'error', component: string, msg: string) { const a = api(); if (a) a.LogEvent(level, component, msg).catch(warn); else console.debug(`[${level}] ${component}: ${msg}`) }
export const diagnostics = () => api()?.DiagnosticsTail(100) ?? Promise.resolve([] as string[])
export const backupNow = () => api()?.BackupNow() ?? Promise.reject(new Error('仅桌面端支持备份'))

export interface UpdateResult { current: string; latest: string; hasUpdate: boolean; url: string; notes: string; noRelease: boolean }
export const appVersion = () => api()?.AppVersion() ?? Promise.resolve('v0.1.0')
/** 检查更新：仅桌面端可用（浏览器预览返回 null）。 */
export const checkUpdate = (): Promise<UpdateResult | null> => api()?.CheckUpdate() ?? Promise.resolve(null)
export const openReleasePage = (url: string) => api()?.OpenReleasePage(url) ?? Promise.resolve()
