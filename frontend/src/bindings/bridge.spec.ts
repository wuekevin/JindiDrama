import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { hasBridge, prime, readLibraryCache, restoreLibraryBackup, retryLibrarySave, saveLibrary, backupNow, saveSetting, LS, cacheStats, cacheSetLimit, logEvent } from './index'
const calls: string[] = []
const receipt = { writer: expect.stringMatching(/^[a-f0-9]{32}$/), revision: expect.any(Number) }
const fake = { StorageReady: async () => true, Favorites: async () => ['d9'], History: async () => [{ dramaId: 'd9', episode: 7, positionMs: 12000, durationMs: 24000, percent: 50, watchedAt: 5 }], SearchHistory: async () => ['替嫁'],
  GetSetting: async (k: string) => (k === 'app' ? '{"theme":"light"}' : ''), SaveSetting: async (k: string) => { calls.push('set:' + k) }, SaveLibraryChanges: async () => {}, SaveSearchHistory: async () => {}, BackupNow: async () => 'x' }
describe('wails bridge', () => {
  beforeEach(() => { localStorage.clear(); calls.length = 0 }); afterEach(() => { vi.unstubAllGlobals(); delete (window as any).go })
  it('no bridge in browser mode: falls back to localStorage', () => { expect(hasBridge()).toBe(false); saveSetting(LS.app, { a: 1 }); expect(localStorage.getItem(LS.app)).toBe('{"a":1}') })
  it('distinguishes a failed SQLite write with or without a persisted recovery journal', async () => {
    const SaveLibraryChanges = vi.fn()
      .mockRejectedValueOnce(new Error('locked'))
      .mockRejectedValueOnce(new Error('locked'))
      .mockResolvedValue(undefined)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const setItem = Storage.prototype.setItem
    let blockLibrary = false
    const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, key, value) {
      if (blockLibrary && key === LS.library) throw new DOMException('full', 'QuotaExceededError')
      return setItem.call(this, key, value)
    })
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    try {
      await prime()
      expect(await saveLibrary([], ['d9'])).toBe('pending')
      const durableJournal = localStorage.getItem(LS.library)
      expect(JSON.parse(durableJournal!)).toMatchObject({ pending: true })
      blockLibrary = true
      expect(await saveLibrary([], ['d2'])).toBe('unsaved')
      expect(localStorage.getItem(LS.library)).toBe(durableJournal)
      expect(await saveLibrary([], ['d3'])).toBe('saved')
      expect(readLibraryCache()).not.toHaveProperty('pending')
    } finally {
      storage.mockRestore()
      warn.mockRestore()
    }
  })

  it('detects when an earlier acknowledgement has since persisted a queued recovery receipt', async () => {
    let finishFirst!: () => void
    const firstWrite = new Promise<void>((resolve) => { finishFirst = resolve })
    const SaveLibraryChanges = vi.fn().mockReturnValueOnce(firstWrite).mockRejectedValueOnce(new Error('locked'))
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const setItem = Storage.prototype.setItem
    let skipNextJournalWrite = false
    const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, key, value) {
      if (skipNextJournalWrite && key === LS.library) {
        skipNextJournalWrite = false
        throw new DOMException('full', 'QuotaExceededError')
      }
      return setItem.call(this, key, value)
    })
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    try {
      await prime()
      const first = saveLibrary([], ['d1'])
      await vi.waitFor(() => expect(SaveLibraryChanges).toHaveBeenCalledTimes(1))
      skipNextJournalWrite = true
      const second = saveLibrary([], ['d2'])
      expect(JSON.parse(localStorage.getItem(LS.library)!).favorites).toEqual(['d1'])
      finishFirst()
      expect(await first).toBe('saved')
      expect(await second).toBe('pending')
      expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({ favorites: ['d2'], pending: true })
    } finally {
      storage.mockRestore()
      warn.mockRestore()
    }
  })

  it('reports a browser library write that could not be saved', async () => {
    const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('full', 'QuotaExceededError')
    })
    try {
      expect(await saveLibrary([], ['d1'])).toBe('unsaved')
    } finally {
      storage.mockRestore()
    }
    expect(await saveLibrary([], ['d1'])).toBe('saved')
  })
  it('retries the same receipt and preserves edits arriving while recovery is blocked', async () => {
    let finishRetry!: (state: { favorites: string[]; history: [] }) => void
    const held = new Promise<{ favorites: string[]; history: [] }>((resolve) => { finishRetry = resolve })
    let databaseFavorites = ['d9']
    const BackupNow = vi.fn().mockResolvedValue('/backup.db')
    const SaveLibraryChanges = vi.fn()
      .mockRejectedValueOnce(new Error('locked'))
      .mockReturnValueOnce(held)
      .mockResolvedValueOnce({ favorites: ['d3', 'd2', 'd9', 'd1', 'd4'], history: [] })
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.stubGlobal('go', { app: { App: {
      ...fake, SaveLibraryChanges, BackupNow, Favorites: async () => databaseFavorites, History: async () => [],
    } } })
    try {
      await prime()
      expect(await saveLibrary([], ['d2', 'd9'])).toBe('pending')
      const firstReceipt = SaveLibraryChanges.mock.calls[0][0].receipt
      databaseFavorites = ['d9', 'd1']
      const retry = retryLibrarySave([], ['d2', 'd9'])
      await vi.waitFor(() => expect(SaveLibraryChanges).toHaveBeenCalledTimes(2))
      expect(SaveLibraryChanges.mock.calls[1][0]).toMatchObject({ receipt: firstReceipt, addedFavorites: ['d2'] })
      const onReconciled = vi.fn()
      const second = saveLibrary([], ['d3', 'd2', 'd9'], { onReconciled })
      const backup = backupNow()
      expect(BackupNow).not.toHaveBeenCalled()
      finishRetry({ favorites: ['d2', 'd9', 'd1'], history: [] })
      expect(await retry).toBe('pending')
      expect(await second).toBe('saved')
      expect(onReconciled).toHaveBeenCalledOnce()
      expect(readLibraryCache()?.favorites).toEqual(['d3', 'd2', 'd9', 'd1', 'd4'])
      expect(await backup).toBe('/backup.db')
      expect(BackupNow).toHaveBeenCalledOnce()
      expect(SaveLibraryChanges.mock.calls[2][0]).toMatchObject({
        addedFavorites: ['d3'], removedFavorites: [], preserveNewerHistory: true,
      })
    } finally {
      finishRetry({ favorites: [], history: [] })
      warn.mockRestore()
    }
  })

  it('keeps a record unseen by the user when an acknowledgement is followed by another edit', async () => {
    const locked = new Error('locked')
    let failSecond!: (reason: Error) => void
    const paused = new Promise<never>((_, reject) => { failSecond = reject })
    const view = (favorites: string[]) => ({ favorites, history: [] })
    const SaveLibraryChanges = vi.fn()
      .mockRejectedValueOnce(locked)
      .mockResolvedValueOnce(view(['d2', 'd9']))
      .mockRejectedValueOnce(locked)
      .mockResolvedValueOnce(view(['d3', 'd2', 'd9', 'd1']))
      .mockReturnValueOnce(paused)
      .mockResolvedValueOnce(view(['d4', 'd3', 'd2', 'd9', 'd1']))
      .mockResolvedValueOnce(view(['d5', 'd4', 'd3', 'd2', 'd9', 'd1']))
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, History: async () => [] } } })
    try {
      await prime()
      await saveLibrary([], ['d2', 'd9'])
      expect(await retryLibrarySave([], ['d2', 'd9'])).toBe('saved')
      await saveLibrary([], ['d3', 'd2', 'd9'])
      const second = saveLibrary([], ['d4', 'd3', 'd2', 'd9'])
      await vi.waitFor(() => expect(SaveLibraryChanges).toHaveBeenCalledTimes(5))
      expect(readLibraryCache()?.favorites).toContain('d1')
      const newest = saveLibrary([], ['d5', 'd4', 'd3', 'd2', 'd9'], {
        before: view(['d4', 'd3', 'd2', 'd9']),
      })
      expect(readLibraryCache()?.favorites).toContain('d1')
      expect(readLibraryCache()?.batches?.at(-1)?.intent?.removedFavorites).toEqual([])
      failSecond(locked)
      expect(await second).toBe('pending')
      expect(await newest).toBe('saved')
      expect(SaveLibraryChanges.mock.lastCall?.[0]).toMatchObject({ addedFavorites: ['d5'], removedFavorites: [] })
      expect(readLibraryCache()?.favorites).toContain('d1')
    } finally {
      warn.mockRestore()
    }
  })

  it('recognizes a durable journal containing an already acknowledged prefix', async () => {
    const locked = new Error('locked')
    const SaveLibraryChanges = vi.fn()
      .mockRejectedValueOnce(locked).mockRejectedValueOnce(locked)
      .mockResolvedValueOnce({ favorites: ['d2'], history: [] }).mockRejectedValueOnce(locked)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, History: async () => [] } } })
    await prime()
    try {
      await saveLibrary([], ['d2'])
      await saveLibrary([], ['d3', 'd2'])
      const saved = localStorage.getItem(LS.library)
      const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new DOMException('full', 'QuotaExceededError')
      })
      try {
        expect(await retryLibrarySave([], ['d3', 'd2'])).toBe('pending')
        expect(readLibraryCache()?.batches).toHaveLength(1)
        expect(localStorage.getItem(LS.library)).toBe(saved)
      } finally {
        storage.mockRestore()
      }
    } finally {
      warn.mockRestore()
    }
  })

  it('does not discard an offline journal when retry still cannot open desktop storage', async () => {
    const StorageReady = vi.fn().mockResolvedValue(false)
    const SaveLibraryChanges = vi.fn()
    vi.stubGlobal('go', { app: { App: { ...fake, StorageReady, SaveLibraryChanges } } })
    await prime()
    expect(await saveLibrary([], ['d1'])).toBe('pending')
    const savedJournal = localStorage.getItem(LS.library)
    expect(await retryLibrarySave([], ['d1'])).toBe('pending')
    expect(localStorage.getItem(LS.library)).toBe(savedJournal)
    expect(SaveLibraryChanges).not.toHaveBeenCalled()
  })

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
  it('deletes SQLite history on the first change after loading', async () => {
    const SaveLibraryChanges = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    await prime()
    await saveLibrary([], ['d9'])
    expect(SaveLibraryChanges).toHaveBeenCalledExactlyOnceWith({ history: [], removed: ['d9'], receipt })
  })

  it('keeps imported rows and user edits made while the backup chooser was open', async () => {
    const original = (await fake.History())[0]
    const imported = { ...original, dramaId: 'd2' }
    const response = { state: { favorites: ['d9', 'd2'], history: [original, imported] }, favoritesAdded: 1, historyAdded: 1 }
    let finish!: () => void
    const restoreResult = new Promise<typeof response>((resolve) => { finish = () => resolve(response) })
    const RestoreLibraryBackup = vi.fn().mockReturnValue(restoreResult)
    const SaveLibraryChanges = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('go', { app: { App: { ...fake, RestoreLibraryBackup, SaveLibraryChanges } } })
    await prime()
    const restore = restoreLibraryBackup()
    await Promise.resolve()
    expect(RestoreLibraryBackup).toHaveBeenCalledOnce()
    const edit = saveLibrary([], [])
    finish()
    expect(await restore).toMatchObject({ state: { favorites: ['d2'], history: [{ dramaId: 'd2' }] } })
    await edit
    expect(SaveLibraryChanges).toHaveBeenCalledExactlyOnceWith({
      addedFavorites: [], removedFavorites: ['d9'], favoriteOrder: [], history: [], removed: ['d9'],
      preserveNewerHistory: true, receipt, readBack: true, legacyJournal: undefined,
    })
    expect(readLibraryCache()).not.toHaveProperty('pending')
    expect(readLibraryCache()?.history.map(({ dramaId }) => dramaId)).toEqual(['d2'])
  })

  it('retries an earlier edit when a subsequent edit persists after backup restore', async () => {
    const original = (await fake.History())[0]
    const imported = { ...original, dramaId: 'd2' }
    const response = { state: { favorites: ['d9', 'd2'], history: [original, imported] }, favoritesAdded: 1, historyAdded: 1 }
    let finish!: () => void
    const restoreResult = new Promise<typeof response>((resolve) => { finish = () => resolve(response) })
    const RestoreLibraryBackup = vi.fn().mockReturnValue(restoreResult)
    const SaveLibraryChanges = vi.fn().mockRejectedValueOnce(new Error('database locked')).mockResolvedValue(undefined)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.stubGlobal('go', { app: { App: { ...fake, RestoreLibraryBackup, SaveLibraryChanges } } })
    try {
      await prime()
      const restore = restoreLibraryBackup()
      await Promise.resolve()
      const first = saveLibrary([], ['d9'])
      const second = saveLibrary([], [])
      finish()
      await Promise.all([restore, first, second])
      expect(SaveLibraryChanges).toHaveBeenCalledTimes(3)
      expect(SaveLibraryChanges.mock.calls[1]).toEqual(SaveLibraryChanges.mock.calls[0])
      expect(SaveLibraryChanges.mock.calls[1]?.[0]).toMatchObject({ removed: ['d9'] })
      expect(SaveLibraryChanges.mock.calls[2]?.[0]).toMatchObject({ removedFavorites: ['d9'], readBack: true })
      expect(readLibraryCache()).toMatchObject({ favorites: ['d2'], history: [{ dramaId: 'd2' }] })
      expect(readLibraryCache()).not.toHaveProperty('pending')
    } finally {
      warn.mockRestore()
    }
  })

  it('replays pending deletes before SQLite hydration on restart', async () => {
    const SaveLibraryChanges = vi.fn().mockResolvedValue(undefined)
    const History = vi.fn().mockImplementationOnce(fake.History).mockResolvedValueOnce([])
    vi.stubGlobal('go', { app: { App: { ...fake, History, SaveLibraryChanges } } })
    localStorage.setItem(LS.library, JSON.stringify({ history: [], favorites: ['d9'], pending: true }))
    await prime()
    expect(SaveLibraryChanges).toHaveBeenCalledExactlyOnceWith({
      history: [], removed: ['d9'], receipt, readBack: true, legacyJournal: expect.any(String),
    })
    expect(JSON.parse(localStorage.getItem(LS.library)!)).toEqual({ history: [], favorites: ['d9'] })
  })

  it('replays the local catalog count after a failed count-only write', async () => {
    const row = { dramaId: 'local-d9', episode: 1, positionMs: 1000, durationMs: 1000, percent: 100, watchedAt: 5 }
    const updated = { ...row, episodeCountAtWatch: 2 }
    const SaveLibraryChanges = vi.fn().mockRejectedValueOnce(new Error('locked'))
      .mockResolvedValueOnce({ favorites: ['d9'], history: [updated] })
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    try {
      vi.stubGlobal('go', { app: { App: { ...fake, History: async () => [row], SaveLibraryChanges } } })
      await prime()
      const { history, favorites } = readLibraryCache()!
      expect(history[0]).not.toHaveProperty('episodeCountAtWatch')
      await saveLibrary([{ ...history[0], episodeCountAtWatch: 2 }], favorites)
      expect(readLibraryCache()?.pending).toBe(true)
      expect(SaveLibraryChanges).toHaveBeenNthCalledWith(1, expect.objectContaining({ history: [updated], receipt }))
      await prime()
      expect(SaveLibraryChanges).toHaveBeenNthCalledWith(2, expect.objectContaining({
        history: [updated], receipt, readBack: true,
      }))
      expect(readLibraryCache()).toMatchObject({ history: [{ episodeCountAtWatch: 2 }] })
      expect(readLibraryCache()).not.toHaveProperty('pending')
    } finally {
      warn.mockRestore()
    }
  })

  it('keeps the latest pending write when the same state is queued twice', async () => {
    let finishFirst!: () => void
    let finishSecond!: () => void
    const firstWrite = new Promise<void>((resolve) => { finishFirst = resolve })
    const secondWrite = new Promise<void>((resolve) => { finishSecond = resolve })
    const SaveLibraryChanges = vi.fn().mockReturnValueOnce(firstWrite).mockReturnValueOnce(secondWrite).mockResolvedValue(undefined)
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    await prime()
    const first = saveLibrary([], ['d9'])
    const second = saveLibrary([], [])
    const latest = saveLibrary([], ['d9'])
    finishFirst()
    await first
    const pending = JSON.parse(localStorage.getItem(LS.library)!)
    expect(pending).toMatchObject({ history: [], favorites: ['d9'], pending: true })
    expect(pending.batches).toMatchObject([
      { receipt, intent: { removedFavorites: ['d9'] } },
      { receipt, intent: { addedFavorites: ['d9'] } },
    ])
    finishSecond()
    await Promise.all([second, latest])
    expect(SaveLibraryChanges).toHaveBeenCalledTimes(3)
    expect(JSON.parse(localStorage.getItem(LS.library)!)).toEqual({ history: [], favorites: ['d9'] })
  })

  it('saves changes in order and recovers from a failed first write', async () => {
    let fail!: (err: Error) => void
    const firstWrite = new Promise<void>((_, reject) => { fail = reject })
    const SaveLibraryChanges = vi.fn().mockReturnValueOnce(firstWrite).mockResolvedValue(undefined)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    try {
      await prime()
      const first = saveLibrary([], ['d9'])
      const second = saveLibrary([{ dramaId: 'd2', episode: 1, percent: 1, watchedAt: 10 }], [])
      await Promise.resolve()
      expect(SaveLibraryChanges).toHaveBeenCalledTimes(1)
      fail(new Error('test rollback'))
      await Promise.all([first, second])
      expect(SaveLibraryChanges).toHaveBeenNthCalledWith(2, {
        favorites: [],
        history: [{ dramaId: 'd2', episode: 1, positionMs: 0, durationMs: 0, percent: 1, watchedAt: 10 }],
        removed: ['d9'], receipt,
      })
      expect(warn).toHaveBeenCalledOnce()
    } finally {
      warn.mockRestore()
    }
  })
  it('recovers offline edits without deleting rows that were absent in the startup cache', async () => {
    const cached = { dramaId: 'd1', episode: 2, percent: 40, watchedAt: 10 }
    const viewed = { dramaId: 'd2', episode: 3, percent: 50, watchedAt: 30 }
    const persisted = { dramaId: 'd9', episode: 1, percent: 42, watchedAt: 20, positionMs: 5000, durationMs: 12000 }
    localStorage.setItem(LS.library, JSON.stringify({ history: [cached], favorites: ['d1'] }))
    vi.stubGlobal('go', { app: { App: { ...fake, StorageReady: async () => false } } })
    await prime()
    await saveLibrary([viewed], ['d2'])
    expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({
      pending: true, batches: [{ receipt, intent: { addedFavorites: ['d2'], removedFavorites: ['d1'], updatedHistory: [viewed], removedHistory: ['d1'] } }],
    })

    const recovered = { ...viewed, positionMs: 0, durationMs: 0 }
    const SaveLibraryChanges = vi.fn().mockResolvedValue(undefined)
    const Favorites = vi.fn().mockResolvedValueOnce(['d1', 'd9']).mockResolvedValueOnce(['d2', 'd9', 'd3'])
    const History = vi.fn().mockResolvedValueOnce([{ ...cached, positionMs: 0, durationMs: 0 }, persisted]).mockResolvedValueOnce([recovered, persisted])
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, Favorites, History } } })
    await prime()
    expect(SaveLibraryChanges).toHaveBeenCalledExactlyOnceWith({
      addedFavorites: ['d2'], removedFavorites: ['d1'], removed: ['d1'], favoriteOrder: ['d2'],
      history: [recovered], preserveNewerHistory: true, receipt, readBack: true, legacyJournal: undefined,
    })
    expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({
      favorites: ['d2', 'd9', 'd3'], history: [{ dramaId: 'd2' }, { dramaId: 'd9' }],
    })
    expect(JSON.parse(localStorage.getItem(LS.library)!)).not.toHaveProperty('pending')
  })

  it('uses the same sparse journal after a failed startup recovery and another offline change', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const SaveLibraryChanges = vi.fn().mockRejectedValueOnce(new Error('locked'))
      .mockResolvedValueOnce({ favorites: ['d1', 'd9', 'd7'], history: await fake.History() })
      .mockResolvedValueOnce({ favorites: ['d3', 'd9', 'd7'], history: [] })
    const Favorites = vi.fn().mockResolvedValueOnce(['d9']).mockResolvedValueOnce(['d9', 'd7'])
    try {
      vi.stubGlobal('go', { app: { App: { ...fake, StorageReady: async () => false } } })
      await prime()
      await saveLibrary([], ['d1'])
      vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, Favorites } } })
      await prime()
      expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({ pending: true, favorites: ['d1', 'd9'] })
      await saveLibrary([], ['d3', 'd9'])
      await prime()
      expect(SaveLibraryChanges).toHaveBeenLastCalledWith({
        addedFavorites: ['d3'], removedFavorites: ['d1'], removed: ['d9'], favoriteOrder: ['d3', 'd9'],
        history: [], preserveNewerHistory: true, receipt, readBack: true, legacyJournal: undefined,
      })
      expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({ favorites: ['d3', 'd9', 'd7'] })
    } finally {
      warn.mockRestore()
    }
  })

  it('preserves library hydration when the independent search-history API fails', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    try {
      vi.stubGlobal('go', { app: { App: { ...fake, SearchHistory: vi.fn().mockRejectedValue(new Error('search failed')) } } })
      await prime()
      expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({ favorites: ['d9'], history: [{ dramaId: 'd9' }] })
      expect(localStorage.getItem(LS.app)).toBe('{"theme":"light"}')
    } finally {
      warn.mockRestore()
    }
  })

  it('does not apply a corrupt modern journal as a legacy full replacement', async () => {
    const SaveLibraryChanges = vi.fn()
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    localStorage.setItem(LS.library, JSON.stringify({ history: [], favorites: [], pending: true, intent: { removedHistory: 7 } }))
    await prime()
    expect(SaveLibraryChanges).not.toHaveBeenCalled()
    expect(JSON.parse(localStorage.getItem(LS.library)!)).toMatchObject({ favorites: ['d9'] })
  })

  it('uses the authoritative hydrated library when its WebView cache cannot be written', async () => {
    const SaveLibraryChanges = vi.fn().mockResolvedValue(undefined)
    localStorage.setItem(LS.library, JSON.stringify({ history: [], favorites: [] }))
    const setItem = Storage.prototype.setItem
    const cacheWrite = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, key, value) {
      if (key === LS.library) throw new DOMException('cache full', 'QuotaExceededError')
      setItem.call(this, key, value)
    })
    try {
      vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
      await prime()
      const loaded = readLibraryCache()!
      expect(loaded).toMatchObject({ favorites: ['d9'], history: [{ dramaId: 'd9' }] })
      await saveLibrary(loaded.history, ['d2', ...loaded.favorites])
      expect(SaveLibraryChanges).toHaveBeenCalledExactlyOnceWith({ favorites: ['d2', 'd9'], history: [], removed: [], receipt })
    } finally {
      cacheWrite.mockRestore()
    }
  })

  it('retains the original offline payload if recovery displays a newer DB copy but fails', async () => {
    const offline = { dramaId: 'd9', episode: 1, percent: 20, watchedAt: 10 }
    const newer = { dramaId: 'd9', episode: 4, percent: 35, positionMs: 0, durationMs: 0, watchedAt: 20 }
    const SaveLibraryChanges = vi.fn().mockRejectedValueOnce(new Error('locked')).mockResolvedValue(undefined)
    const History = vi.fn().mockResolvedValueOnce([newer]).mockResolvedValueOnce([])
      .mockResolvedValueOnce([{ ...offline, positionMs: 0, durationMs: 0 }])
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    try {
      vi.stubGlobal('go', { app: { App: { ...fake, StorageReady: async () => false } } })
      await prime()
      await saveLibrary([offline], [])
      vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, History } } })
      await prime()
      expect(readLibraryCache()).toMatchObject({ pending: true, history: [{ dramaId: newer.dramaId, episode: newer.episode, watchedAt: newer.watchedAt }], batches: [{ receipt, intent: { updatedHistory: [offline] } }] })
      await prime()
      expect(SaveLibraryChanges).toHaveBeenLastCalledWith(expect.objectContaining({
        history: [{ ...offline, positionMs: 0, durationMs: 0 }],
      }))
    } finally {
      warn.mockRestore()
    }
  })

  it('reuses the committed receipt when clearing the pending WebView journal fails', async () => {
    let saved = await fake.History()
    const acknowledged = new Map<string, number>()
    const SaveLibraryChanges = vi.fn(async ({ receipt: stamp, removed }: {
      receipt: { writer: string; revision: number }; removed: string[]
    }) => {
      if ((acknowledged.get(stamp.writer) ?? 0) >= stamp.revision) return
      acknowledged.set(stamp.writer, stamp.revision)
      saved = saved.filter((row) => !removed.includes(row.dramaId))
    })
    const setItem = Storage.prototype.setItem
    let blockClean = false
    const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, key, value) {
      if (blockClean && key === LS.library && !JSON.parse(value).pending) throw new DOMException('cache full', 'QuotaExceededError')
      setItem.call(this, key, value)
    })
    try {
      vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, History: async () => saved } } })
      await prime()
      blockClean = true
      await saveLibrary([], ['d9'])
      const stranded = JSON.parse(localStorage.getItem(LS.library)!)
      expect(stranded).toMatchObject({ pending: true, batches: [{ receipt, intent: { removedHistory: ['d9'] } }] })
      const later = { dramaId: 'd9', episode: 3, positionMs: 1400, durationMs: 10000, percent: 14, watchedAt: 80 }
      saved = [later]
      blockClean = false
      await prime()
      expect(SaveLibraryChanges.mock.calls[1][0].receipt).toEqual(SaveLibraryChanges.mock.calls[0][0].receipt)
      expect(saved).toEqual([later])
      expect(readLibraryCache()).toMatchObject({ history: [{ episode: 3, positionSec: 1.4 }] })
      expect(readLibraryCache()).not.toHaveProperty('pending')
    } finally {
      storage.mockRestore()
    }
  })

  it('hydrates the transaction snapshot after a duplicate without issuing another history read', async () => {
    const saved = await fake.History()
    const History = vi.fn().mockResolvedValueOnce(saved).mockRejectedValue(new Error('second read unavailable'))
    const SaveLibraryChanges = vi.fn().mockResolvedValue({ favorites: ['d9'], history: saved })
    localStorage.setItem(LS.library, JSON.stringify({
      favorites: [], history: [], pending: true, batches: [{
        receipt: { writer: 'a'.repeat(32), revision: 3 },
        intent: { addedFavorites: [], removedFavorites: ['d9'], favoriteOrder: [], updatedHistory: [], removedHistory: ['d9'] },
      }],
    }))
    vi.stubGlobal('go', { app: { App: { ...fake, History, SaveLibraryChanges } } })
    await prime()
    expect(History).toHaveBeenCalledOnce()
    expect(readLibraryCache()).toMatchObject({ favorites: ['d9'], history: [{ dramaId: 'd9', episode: 7 }] })
    expect(readLibraryCache()).not.toHaveProperty('pending')
  })

  it('reuses the legacy journal identity and keeps saving to SQLite when WebView writes are blocked', async () => {
    localStorage.setItem(LS.library, JSON.stringify({ favorites: ['d9'], history: [], pending: true }))
    const setItem = Storage.prototype.setItem
    const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, key, value) {
      if (key === LS.library) throw new DOMException('cache full', 'QuotaExceededError')
      setItem.call(this, key, value)
    })
    const SaveLibraryChanges = vi.fn().mockResolvedValueOnce({ favorites: ['d9'], history: [] })
      .mockResolvedValueOnce(null).mockResolvedValueOnce({ favorites: ['d2', 'd9'], history: [] })
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges } } })
    try {
      await prime()
      await saveLibrary([], ['d2', 'd9'])
      await prime()
      expect(SaveLibraryChanges).toHaveBeenCalledTimes(3)
      expect(SaveLibraryChanges.mock.calls[0][0].legacyJournal).toBe(SaveLibraryChanges.mock.calls[2][0].legacyJournal)
      expect(SaveLibraryChanges.mock.calls[1][0]).toMatchObject({ favorites: ['d2', 'd9'] })
      expect(readLibraryCache()).toMatchObject({ favorites: ['d2', 'd9'] })
    } finally {
      storage.mockRestore()
    }
  })

  it('does not combine a previously submitted receipt with edits made during a later outage', async () => {
    const stale = { writer: 'a'.repeat(32), revision: 3 }
    const saved = await fake.History()
    localStorage.setItem(LS.library, JSON.stringify({
      favorites: [], history: [], pending: true, batches: [{
        receipt: stale,
        intent: { addedFavorites: [], removedFavorites: ['d9'], favoriteOrder: [], updatedHistory: [], removedHistory: ['d9'] },
      }],
    }))
    vi.stubGlobal('go', { app: { App: { ...fake, StorageReady: async () => false } } })
    await prime()
    await saveLibrary([], ['d2'])
    await saveLibrary([{ dramaId: 'd2', episode: 1, percent: 10, watchedAt: 15 }], ['d2'])

    const pending = readLibraryCache()!
    expect(pending.batches).toHaveLength(2)
    expect(pending.batches?.[0].receipt).toEqual(stale)
    expect(pending.batches?.[1].intent).toMatchObject({ addedFavorites: ['d2'], removedFavorites: [], removedHistory: [] })
    const SaveLibraryChanges = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('go', { app: { App: { ...fake, SaveLibraryChanges, History: async () => saved } } })
    await prime()
    expect(SaveLibraryChanges.mock.calls.map(([change]) => change.receipt)).toEqual(pending.batches?.map((batch) => batch.receipt))
    expect(SaveLibraryChanges.mock.calls[1][0].removed).toEqual([])
  })

})
