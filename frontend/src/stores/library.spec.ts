import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useLibraryStore } from './library'
import { useToastStore } from './toast'
import { prime } from '@/bindings'
import { nextTick } from 'vue'

describe('library store', () => {
  beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
  afterEach(() => vi.unstubAllGlobals())
  it('does not seed personal records for a new desktop library', () => {
    vi.stubGlobal('go', { app: { App: {} } })
    const store = useLibraryStore()
    expect(store.history).toEqual([])
    expect(store.favorites).toEqual([])
    store.toggleFav('d1')
    expect(store.history).toEqual([])
  })
  it('reads an older cache with missing history without importing demo records', () => {
    vi.stubGlobal('go', { app: { App: {} } })
    localStorage.setItem('jindi.library.v1', '{"favorites":["d1"]}')
    const store = useLibraryStore()
    expect(store.history).toEqual([])
    expect(store.favorites).toEqual(['d1'])
  })
  it('toggles favorites', () => {
    const s = useLibraryStore()
    expect(s.isFav('d1')).toBe(false)
    expect(s.toggleFav('d1')).toBe(true)
    expect(s.toggleFav('d1')).toBe(false)
  })
  it('shows a failure during a burst of saves and clears it when a newer save completes', async () => {
    let failFirst!: (error: Error) => void
    let finishSecond!: () => void
    const first = new Promise<void>((_, reject) => { failFirst = reject })
    const second = new Promise<void>((resolve) => { finishSecond = resolve })
    const SaveLibraryChanges = vi.fn().mockReturnValueOnce(first).mockReturnValueOnce(second)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.stubGlobal('go', { app: { App: {
      StorageReady: async () => true, Favorites: async () => [], History: async () => [],
      SearchHistory: async () => [], GetSetting: async () => '', SaveLibraryChanges,
    } } })
    try {
      await prime()
      const store = useLibraryStore()
      const toast = useToastStore()
      store.record('d1', 1, 10)
      await vi.waitFor(() => expect(SaveLibraryChanges).toHaveBeenCalledTimes(1))
      store.record('d1', 1, 20)
      await nextTick()
      failFirst(new Error('locked'))
      await vi.waitFor(() => expect(SaveLibraryChanges).toHaveBeenCalledTimes(2))
      expect(toast.items.map(({ text }) => text)).toEqual(['收藏或历史已暂存在本机，重启应用后会重试写入。'])
      finishSecond()
      await vi.waitFor(() => expect(toast.items).toHaveLength(0))
    } finally {
      warn.mockRestore()
    }
  })
  it('warns immediately when the recovery journal cannot be written and SQLite is still waiting', async () => {
    let finish!: () => void
    const delayed = new Promise<void>((resolve) => { finish = resolve })
    const SaveLibraryChanges = vi.fn().mockReturnValue(delayed)
    vi.stubGlobal('go', { app: { App: {
      StorageReady: async () => true, Favorites: async () => [], History: async () => [],
      SearchHistory: async () => [], GetSetting: async () => '', SaveLibraryChanges,
    } } })
    await prime()
    const write = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('full', 'QuotaExceededError')
    })
    try {
      const store = useLibraryStore()
      const toast = useToastStore()
      store.record('d1', 1, 10)
      await nextTick()
      expect(toast.items.map(({ text }) => text)).toEqual(['收藏或历史无法确认保存，关闭应用后可能丢失。'])
      await vi.waitFor(() => expect(SaveLibraryChanges).toHaveBeenCalledTimes(1))
      finish()
      await vi.waitFor(() => expect(toast.items).toHaveLength(0))
    } finally {
      write.mockRestore()
    }
  })
  it('records history, most recent first, no duplicates', () => {
    const s = useLibraryStore()
    s.record('d9', 3, 10); s.record('d9', 4, 20)
    expect(s.history.filter((h) => h.dramaId === 'd9')).toHaveLength(1)
    expect(s.history[0]).toMatchObject({ dramaId: 'd9', episode: 4, percent: 20 })
  })
})
