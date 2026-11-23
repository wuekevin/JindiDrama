import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { hasBridge, prime, saveLibrary, saveSetting, LS, cacheStats, cacheSetLimit, logEvent } from './index'
const calls: string[] = []
const fake = { StorageReady: async () => true, Favorites: async () => ['d9'], History: async () => [{ dramaId: 'd9', episode: 7, positionMs: 12000, durationMs: 24000, percent: 50, watchedAt: 5 }], SearchHistory: async () => ['替嫁'],
  GetSetting: async (k: string) => (k === 'app' ? '{"theme":"light"}' : ''), SaveSetting: async (k: string) => { calls.push('set:' + k) }, SaveFavorites: async () => { calls.push('fav') }, RecordHistory: async () => { calls.push('rec') }, RemoveHistory: async () => { calls.push('rm') }, ClearHistory: async () => {}, SaveSearchHistory: async () => {}, BackupNow: async () => 'x' }
describe('wails bridge', () => {
  beforeEach(() => { localStorage.clear(); calls.length = 0 }); afterEach(() => { delete (window as any).go })
  it('no bridge in browser mode: falls back to localStorage', () => { expect(hasBridge()).toBe(false); saveSetting(LS.app, { a: 1 }); expect(localStorage.getItem(LS.app)).toBe('{"a":1}') })
  it('prime hydrates localStorage from SQLite data', async () => {
    ;(window as any).go = { app: { App: fake } }; expect(hasBridge()).toBe(true); await prime()
    const lib = JSON.parse(localStorage.getItem(LS.library)!); expect(lib.favorites).toEqual(['d9']); expect(lib.history[0]).toMatchObject({ dramaId: 'd9', episode: 7, positionSec: 12 }); expect(localStorage.getItem(LS.app)).toBe('{"theme":"light"}')
  })
  it('cache + log calls go through Go bridge; browser mode is a safe no-op', async () => {
    expect(await cacheStats()).toBeNull(); await cacheSetLimit(2); logEvent('error', 'player', 'x')
    const sent: unknown[][] = []; ;(window as any).go = { app: { App: { ...fake, CacheStats: async () => ({ bytes: 5, files: 1, max: 9 }), CacheSetLimitMB: async (mb: number) => { sent.push(['limit', mb]) }, LogEvent: async (...a: unknown[]) => { sent.push(['log', ...a]) } } } }
    expect((await cacheStats())!.files).toBe(1); await cacheSetLimit(2); logEvent('warn', 'player', 'm'); await Promise.resolve()
    expect(sent).toEqual([['limit', 2048], ['log', 'warn', 'player', 'm']])
  })
  it('saveLibrary writes through (debounced) and diffs history', async () => {
    vi.useFakeTimers(); ;(window as any).go = { app: { App: fake } }
    saveLibrary([{ dramaId: 'a', episode: 1, percent: 1, watchedAt: 1 }], ['a']); await vi.advanceTimersByTimeAsync(400)
    expect(calls).toContain('fav'); expect(calls).toContain('rec'); vi.useRealTimers()
  })
})
