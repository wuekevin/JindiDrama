import { localId } from '@/localId'
// Wails Go 绑定桥：桌面端数据写入本地 SQLite；浏览器预览（无 Go 运行时）退回 localStorage。
import type { DemoQualityId } from '@/player/demoMedia'
import { captureLibraryIntent, parseLibraryCache, replayLibraryBatch, type LibraryBatch, type LibraryCache, type LibraryReceipt, type LibraryState } from './libraryRecovery'
import type { HistoryEntry } from '@/types'
import type { Episode, PlaybackTicket } from '../../../contracts/provider-contracts'
type CatalogDrama = Pick<import('@/types').Drama, 'id' | 'title' | 'summary' | 'tags' | 'episodeCount' | 'updatedTo'>

export interface LocalDanmaku { id: number; offsetMs: number; text: string }

interface HistoryRow {
  dramaId: string
  episode: number
  positionMs: number
  durationMs: number
  percent: number
  watchedAt: number
  episodeCountAtWatch?: number
}

interface LibraryChanges {
  favorites?: string[]
  history: HistoryRow[]
  removed: string[]
  addedFavorites?: string[]
  removedFavorites?: string[]
  favoriteOrder?: string[]
  preserveNewerHistory?: boolean
  receipt?: LibraryReceipt
  legacyJournal?: string
  readBack?: boolean
}

export interface LicensedDrama extends CatalogDrama {
  license: string
  allowExport?: boolean
}

export interface LicensedConfiguration {
  startupDirectory: string
  activeDirectory: string
  savedDirectory: string
  managed: boolean
  canConfigure: boolean
  restartRequired: boolean
  readError?: string
}

interface GoApp {
  LicensedConfiguration?(): Promise<LicensedConfiguration>
  ChooseLicensedDirectory?(): Promise<LicensedConfiguration>
  ClearLicensedDirectory?(): Promise<LicensedConfiguration>
  LicensedCatalog?(): Promise<LicensedDrama[]>
  ExportDemoEpisode(dramaId: string, episode: number, quality: DemoQualityId): Promise<boolean>
  ExportLicensedEpisode?(dramaId: string, episode: number): Promise<boolean>
  ExportDemoScreenshot?(dramaId: string, episode: number, quality: DemoQualityId, seconds: number, pngData: string): Promise<boolean>
  LocalEpisodeLiked?(dramaId: string, episode: number): Promise<boolean>
  SetLocalEpisodeLiked?(dramaId: string, episode: number, liked: boolean): Promise<void>
  LocalComments(dramaId: string, episode: number): Promise<string>
  SaveLocalComments(dramaId: string, episode: number, snapshot: string): Promise<void>
  LocalDanmaku(dramaId: string, episode: number): Promise<LocalDanmaku[]>
  SendLocalDanmaku(dramaId: string, episode: number, offsetMs: number, text: string): Promise<LocalDanmaku>
  HomeFeed(): Promise<HomeFeed>

  Search(query: string): Promise<Pick<CatalogDrama, "id" | "title">[]>
  Drama(id: string): Promise<CatalogDrama>
  Episodes(id: string): Promise<Episode[]>
  ResolvePlayback(dramaId: string, episode: number): Promise<PlaybackTicket>

  CloseWindow?(): Promise<boolean>
  WindowCloseAction?(): Promise<string>
  SetWindowCloseAction?(action: string): Promise<void>
  WindowTrayAvailable?(): Promise<boolean>
  HideWindowToTray?(): Promise<boolean>
  StorageReady(): Promise<boolean>
  MarkFrontendReady(): Promise<void>
  Favorites(): Promise<string[]>
  History(): Promise<HistoryRow[]>
  RestoreLibraryBackup?(): Promise<{ state: { favorites: string[]; history: HistoryRow[] }; favoritesAdded: number; historyAdded: number } | null>
  SaveLibraryChanges(changes: LibraryChanges): Promise<{ favorites: string[]; history: HistoryRow[] } | null>
  SearchHistory(): Promise<string[]>; SaveSearchHistory(q: string[]): Promise<void>
  CacheStats(): Promise<CacheStats>; CacheClear(): Promise<void>; CacheSetLimitMB(mb: number): Promise<void>
  ChooseCacheDirectory?(): Promise<CacheStats | null>
  ResetCacheDirectory?(): Promise<CacheStats>
  AutostartEnabled(): Promise<boolean>; SetAutostart(on: boolean): Promise<void>
  ProviderName?(): Promise<string>
  AppVersion(): Promise<string>; CheckUpdate(): Promise<UpdateResult>; OpenReleasePage(url: string): Promise<void>
  DownloadUpdate?(releaseURL: string): Promise<boolean>
  CancelUpdateDownload?(): Promise<void>
  UpdateChannel(): Promise<UpdateChannel>; SetUpdateChannel(channel: UpdateChannel): Promise<void>
  LogEvent(level: string, component: string, msg: string): Promise<void>; DiagnosticsTail(n: number): Promise<string[]>
  GetSetting(k: string): Promise<string>; SaveSetting(k: string, v: string): Promise<void>; BackupNow(): Promise<string>
}
const api = (): GoApp | null => ((window as any).go?.app?.App as GoApp | undefined) ?? null
export const markFrontendReady = () => api()?.MarkFrontendReady?.()

export const hasBridge = () => api() !== null
export const closeDesktopWindow = () => api()?.CloseWindow?.()
let windowCloseSettingWrite = Promise.resolve()
export const waitForWindowCloseSetting = () => windowCloseSettingWrite
export function setWindowCloseSetting(action: string) {
  const a = api()
  const save = a?.SetWindowCloseAction?.bind(a)
  if (!save) return
  const write = windowCloseSettingWrite.then(() => save(action))
  windowCloseSettingWrite = write.catch(warn)
  return write
}
export const windowTrayAvailable = () => api()?.WindowTrayAvailable?.()
export const hideWindowToTray = () => api()?.HideWindowToTray?.()
interface HomeFeed {
  todayNew: string[]
  rankings: Record<'hot' | 'rise' | 'new', string[]>
}
export const exportDemoEpisode = (dramaId: string, episode: number, quality: DemoQualityId) => api()?.ExportDemoEpisode(dramaId, episode, quality)
export const exportDemoScreenshot = (dramaId: string, episode: number, quality: DemoQualityId, seconds: number, pngData: string) =>
  api()?.ExportDemoScreenshot?.(dramaId, episode, quality, seconds, pngData)
const episodeLikeStorageKey = (dramaId: string, episode: number) => `jindi.episode-like.${dramaId}.${episode}`
const episodeLikeWrites = new Map<string, Promise<void>>()
export const waitForEpisodeLikeWrites = () => Promise.all(episodeLikeWrites.values())
export async function loadEpisodeLike(dramaId: string, episode: number) {
  const key = episodeLikeStorageKey(dramaId, episode)
  await episodeLikeWrites.get(key)
  const app = api()
  if (app?.LocalEpisodeLiked && app.SetLocalEpisodeLiked) return app.LocalEpisodeLiked(dramaId, episode)
  return localStorage.getItem(key) === '1'
}
export function saveEpisodeLike(dramaId: string, episode: number, liked: boolean) {
  const key = episodeLikeStorageKey(dramaId, episode)
  const previous = episodeLikeWrites.get(key) ?? Promise.resolve()
  const write = previous.then(async () => {
    const app = api()
    if (app?.LocalEpisodeLiked && app.SetLocalEpisodeLiked) {
      await app.SetLocalEpisodeLiked(dramaId, episode, liked)
      return
    }
    if (liked) localStorage.setItem(key, '1')
    else localStorage.removeItem(key)
  })
  const pending = write.catch(warn)
  episodeLikeWrites.set(key, pending)
  void pending.then(() => {
    if (episodeLikeWrites.get(key) === pending) episodeLikeWrites.delete(key)
  })
  return write
}

export const loadLocalComments = (dramaId: string, episode: number) => api()?.LocalComments(dramaId, episode)
export const saveLocalComments = (dramaId: string, episode: number, snapshot: string) =>
  api()?.SaveLocalComments(dramaId, episode, snapshot)

export const loadLocalDanmaku = (dramaId: string, episode: number) => api()?.LocalDanmaku(dramaId, episode)
export const sendLocalDanmaku = (dramaId: string, episode: number, offsetMs: number, text: string) =>
  api()?.SendLocalDanmaku(dramaId, episode, offsetMs, text)

export const hasLicensedLibrary = () => typeof api()?.LicensedCatalog === 'function'
export const hasLicensedExport = () => typeof api()?.ExportLicensedEpisode === 'function'
export const exportLicensedEpisode = (dramaId: string, episode: number) => api()?.ExportLicensedEpisode?.(dramaId, episode)
export const readLicensedConfiguration = () => api()?.LicensedConfiguration?.()
export const chooseLicensedDirectory = () => api()?.ChooseLicensedDirectory?.()
export const clearLicensedDirectory = () => api()?.ClearLicensedDirectory?.()
export const loadLicensedCatalog = () => api()?.LicensedCatalog?.()
export const loadHomeFeed = () => api()?.HomeFeed()
export const activeProviderName = () => api()?.ProviderName?.()
export const searchProvider = (query: string) => api()?.Search(query)
export const loadProviderEpisodes = (id: string) => api()?.Episodes(id)
export function loadProviderDetail(id: string) {
  const app = api()
  return app ? Promise.all([app.Drama(id), app.Episodes(id)]) : undefined
}
export const resolvePlayback = (dramaId: string, episode: number) => api()?.ResolvePlayback(dramaId, episode)
const warn = (e: unknown) => console.warn('[bridge]', e)

export const LS = { library: 'jindi.library.v1', search: 'jindi.search.v1', app: 'jindi.settings.v1', player: 'jindi.player.v1', auth: 'jindi.auth.v1' }
const SETTING_KEY: Record<string, string> = { [LS.app]: 'app', [LS.player]: 'player', [LS.auth]: 'auth_demo' }

interface LibrarySnapshot {
  favorites: string[]
  history: Map<string, HistoryRow>
}

let persistedLibrary: LibrarySnapshot | undefined
let libraryWrites = Promise.resolve()
export const waitForLibraryWrites = () => libraryWrites
let desktopCache: string | undefined
let libraryWriter: string | undefined
let writerRevision = 0
let offlineDraft: LibraryReceipt | undefined
let rebaseLibraryWrites = false

function nextLibraryReceipt(): LibraryReceipt {
  libraryWriter ??= localId()
  return { writer: libraryWriter, revision: ++writerRevision }
}

export function readLibraryCache(): LibraryCache | undefined {
  try {
    return parseLibraryCache(api() && desktopCache !== undefined ? desktopCache : localStorage.getItem(LS.library))
  } catch {
    return undefined
  }
}

function storeLibraryCache(cache: LibraryCache) {
  const serialized = JSON.stringify(cache)
  if (api()) desktopCache = serialized
  try { localStorage.setItem(LS.library, serialized) } catch { /* local cache is optional when SQLite is ready */ }
}

function fromHistoryRow(row: HistoryRow): HistoryEntry {
  return {
    dramaId: row.dramaId, episode: row.episode, percent: row.percent, watchedAt: row.watchedAt,
    positionSec: row.positionMs / 1000, durationSec: row.durationMs / 1000,
    ...(row.episodeCountAtWatch === undefined ? {} : { episodeCountAtWatch: row.episodeCountAtWatch }),
  }
}

function sameHistoryRow(previous: HistoryRow | undefined, next: HistoryRow): boolean {
  return previous !== undefined && previous.episode === next.episode && previous.percent === next.percent
    && previous.watchedAt === next.watchedAt && previous.positionMs === next.positionMs
    && previous.durationMs === next.durationMs && previous.episodeCountAtWatch === next.episodeCountAtWatch
}

function toHistoryRow(entry: HistoryEntry): HistoryRow {
  return {
    dramaId: entry.dramaId, episode: entry.episode, percent: entry.percent, watchedAt: entry.watchedAt,
    positionMs: Math.round((entry.positionSec ?? 0) * 1000),
    durationMs: Math.round((entry.durationSec ?? 0) * 1000),
    ...(entry.episodeCountAtWatch === undefined ? {} : { episodeCountAtWatch: entry.episodeCountAtWatch }),
  }
}

async function readDesktopLibrary(a: GoApp): Promise<LibraryState> {
  const [favorites, rows] = await Promise.all([a.Favorites(), a.History()])
  return hydrateLibraryState({ favorites, history: rows })
}

function hydrateLibraryState({ favorites, history }: { favorites: string[]; history: HistoryRow[] }): LibraryState {
  persistedLibrary = { favorites, history: new Map(history.map((row) => [row.dramaId, row])) }
  return { favorites, history: history.map(fromHistoryRow) }
}

export const canRestoreLibraryBackup = () => typeof api()?.RestoreLibraryBackup === 'function'

export function restoreLibraryBackup() {
  const app = api()
  const restore = libraryWrites.then(async () => {
    if (!persistedLibrary || readLibraryCache()?.pending) {
      throw new Error('本地收藏或历史尚未写入。请重新启动应用，等补写完成后再恢复。')
    }
    const result = await app?.RestoreLibraryBackup?.()
    if (!result) return null
    const database = hydrateLibraryState(result.state)
    rebaseLibraryWrites = true
    const batches = pendingBatches(readLibraryCache())
    const state = batches.reduce(replayLibraryBatch, database)
    storeLibraryCache(batches.length ? { ...state, pending: true, batches } : state)
    return { state, favoritesAdded: result.favoritesAdded, historyAdded: result.historyAdded }
  })
  libraryWrites = restore.then(() => undefined, () => undefined)
  return restore
}

function pendingBatches(previous: LibraryCache | undefined): LibraryBatch[] {
  if (!previous?.pending) return []
  if (previous.batches) return previous.batches
  const receipt = nextLibraryReceipt()
  return previous.intent ? [{ receipt, legacy: true, intent: previous.intent }]
    : [{ receipt, legacy: true, snapshot: { history: previous.history, favorites: previous.favorites } }]
}

function batchChanges(batch: LibraryBatch, database: LibraryState): LibraryChanges {
  const acknowledgement = { receipt: batch.receipt, readBack: true,
    legacyJournal: batch.legacy ? JSON.stringify(batch.intent ? { intent: batch.intent } : { snapshot: batch.snapshot }) : undefined }
  if (batch.intent) {
    const intent = batch.intent
    return {
      history: intent.updatedHistory.map(toHistoryRow),
      removed: intent.removedHistory,
      addedFavorites: intent.addedFavorites,
      removedFavorites: intent.removedFavorites,
      favoriteOrder: intent.favoriteOrder,
      preserveNewerHistory: true,
      ...acknowledgement,
    }
  }
  const ids = new Set(batch.snapshot.history.map((row) => row.dramaId))
  const changes: LibraryChanges = {
    history: batch.snapshot.history.map(toHistoryRow)
      .filter((row) => !sameHistoryRow(persistedLibrary?.history.get(row.dramaId), row)),
    removed: database.history.filter((row) => !ids.has(row.dramaId)).map((row) => row.dramaId),
    ...acknowledgement,
  }
  if (JSON.stringify(database.favorites) !== JSON.stringify(batch.snapshot.favorites)) {
    changes.favorites = batch.snapshot.favorites
  }
  return changes
}

async function recoverLibrary(a: GoApp, database: LibraryState, pending: LibraryCache) {
  const batches = pendingBatches(pending)
  let confirmed = 0
  let current = database
  try {
    for (const batch of batches) {
      const snapshot = await a.SaveLibraryChanges(batchChanges(batch, current))
      confirmed++
      // The packaged bridge returns the authoritative transaction snapshot.
      // Older bridge implementations can refresh after the acknowledgement.
      current = snapshot ? hydrateLibraryState(snapshot) : await readDesktopLibrary(a)
    }
    storeLibraryCache(current)
  } catch (e) {
    warn(e)
    const remaining = batches.slice(confirmed)
    if (remaining.length) {
      storeLibraryCache({ ...remaining.reduce(replayLibraryBatch, current), pending: true, batches: remaining })
    } else {
      storeLibraryCache(current)
    }
    persistedLibrary = undefined
  }
}

/** Hydrate desktop storage before stores are created. */
export async function prime(): Promise<void> {
  persistedLibrary = undefined
  desktopCache = undefined
  offlineDraft = undefined
  rebaseLibraryWrites = false
  const a = api()
  if (!a) return
  await primeDesktopStorage(a)
  try {
    if (!a.WindowCloseAction) return
    const closeAction = await a.WindowCloseAction()
    const settings = JSON.parse(localStorage.getItem(LS.app) ?? '{}')
    if (closeAction) {
      localStorage.setItem(LS.app, JSON.stringify({ ...settings, closeAction }))
    } else if (typeof settings?.closeAction === 'string') {
      await setWindowCloseSetting(settings.closeAction)
    }
  } catch (e) {
    warn(e)
  }
}

async function primeDesktopStorage(a: GoApp) {
  try {
    if (!(await a.StorageReady())) return
  } catch (e) {
    warn(e)
    return
  }

  try {
    const database = await readDesktopLibrary(a)
    const pending = readLibraryCache()
    if (pending?.pending) await recoverLibrary(a, database, pending)
    else storeLibraryCache(database)
  } catch (e) {
    warn(e)
  }

  try {
    const search = await a.SearchHistory()
    localStorage.setItem(LS.search, JSON.stringify(search))
  } catch (e) {
    warn(e)
  }
  try {
    for (const [ls, key] of Object.entries(SETTING_KEY)) { const v = await a.GetSetting(key); if (v) localStorage.setItem(ls, v) }
  } catch (e) {
    warn(e)
  }
}

export function saveLibrary(history: HistoryEntry[], favorites: string[]) {
  const next = { history, favorites }
  const a = api()
  if (!a) {
    storeLibraryCache(next)
    return
  }
  const previous = readLibraryCache()
  const batches = pendingBatches(previous)
  const last = batches.at(-1)
  if (!persistedLibrary && last?.intent && offlineDraft
      && last.receipt.writer === offlineDraft.writer && last.receipt.revision === offlineDraft.revision) {
    const intent = captureLibraryIntent({ ...previous!, intent: last.intent }, next)
    storeLibraryCache({ ...next, pending: true, batches: [...batches.slice(0, -1), { receipt: offlineDraft, intent }] })
    return
  }

  const receipt = nextLibraryReceipt()
  const intent = captureLibraryIntent(previous && { history: previous.history, favorites: previous.favorites }, next)
  const journal = [...batches, { receipt, intent }]
  if (!persistedLibrary) {
    offlineDraft = receipt
    storeLibraryCache({ ...next, pending: true, batches: journal })
    return
  }
  return queueLibraryWrite(a, next, journal, receipt)
}

function acknowledgeLibraryWrite(receipt: LibraryReceipt) {
  const latest = readLibraryCache()
  if (!latest?.batches) return
  const remaining = latest.batches.filter(({ receipt: batch }) =>
    batch.writer !== receipt.writer || batch.revision > receipt.revision)
  const { favorites, history } = latest
  storeLibraryCache(remaining.length ? { favorites, history, pending: true, batches: remaining } : { favorites, history })
}

async function writeRebasedLibrary(a: GoApp, previous: LibrarySnapshot, receipt: LibraryReceipt) {
  let database = { favorites: previous.favorites, history: [...previous.history.values()].map(fromHistoryRow) }
  const batches = readLibraryCache()?.batches ?? []
  for (const batch of batches) {
    if (batch.receipt.writer !== receipt.writer || batch.receipt.revision > receipt.revision) continue
    const snapshot = await a.SaveLibraryChanges(batchChanges(batch, database))
    if (snapshot) {
      database = hydrateLibraryState(snapshot)
    } else {
      database = replayLibraryBatch(database, batch)
      persistedLibrary = { favorites: database.favorites, history: new Map(database.history.map((entry) => [entry.dramaId, toHistoryRow(entry)])) }
    }
    acknowledgeLibraryWrite(batch.receipt)
  }
}

function queueLibraryWrite(a: GoApp, state: LibraryState, batches: LibraryBatch[], receipt: LibraryReceipt) {
  const committed = { history: state.history, favorites: state.favorites }
  storeLibraryCache({ ...committed, pending: true, batches })
  const pendingRows = state.history.map(toHistoryRow)
  const pendingFavorites = [...state.favorites]
  libraryWrites = libraryWrites.then(async () => {
    const previous = persistedLibrary
    if (!previous) return
    try {
      // Replay edits queued during restore in receipt order against the new
      // snapshot. An earlier failed edit stays pending until SQLite accepts it.
      if (rebaseLibraryWrites) {
        await writeRebasedLibrary(a, previous, receipt)
        return
      }
      const next = { favorites: pendingFavorites, history: new Map(pendingRows.map((row) => [row.dramaId, row])) }
      const changes: LibraryChanges = {
        history: pendingRows.filter((row) => !sameHistoryRow(previous.history.get(row.dramaId), row)),
        removed: [...previous.history.keys()].filter((id) => !next.history.has(id)),
        receipt,
      }
      if (JSON.stringify(previous.favorites) !== JSON.stringify(next.favorites)) changes.favorites = next.favorites
      await a.SaveLibraryChanges(changes)
      persistedLibrary = next
      acknowledgeLibraryWrite(receipt)
    } catch (e) {
      warn(e)
    }
  })
  return libraryWrites
}
export function saveSearch(q: string[]) { try { localStorage.setItem(LS.search, JSON.stringify(q)) } catch { /* ignore */ } api()?.SaveSearchHistory(q).catch(warn) }
export function saveSetting(lsKey: string, value: unknown) {
  const s = JSON.stringify(value); try { localStorage.setItem(lsKey, s) } catch { /* ignore */ }
  const k = SETTING_KEY[lsKey]; if (k) api()?.SaveSetting(k, s).catch(warn)
}
export interface CacheStats { bytes: number; files: number; max: number; path: string; custom?: boolean; fallback?: boolean }
export const cacheStats = () => api()?.CacheStats() ?? Promise.resolve(null)
export const cacheClear = () => api()?.CacheClear() ?? Promise.resolve()
export const cacheSetLimit = (gb: number) => api()?.CacheSetLimitMB(Math.round(gb * 1024)) ?? Promise.resolve()
export const hasCacheDirectoryPicker = () => !!(api()?.ChooseCacheDirectory && api()?.ResetCacheDirectory)
export const chooseCacheDirectory = () => api()?.ChooseCacheDirectory?.()
export const resetCacheDirectory = () => api()?.ResetCacheDirectory?.()
/** 故障上报：Go 侧统一脱敏后落盘；浏览器模式仅输出到控制台。 */
export function logEvent(level: 'info' | 'warn' | 'error', component: string, msg: string) { const a = api(); if (a) a.LogEvent(level, component, msg).catch(warn); else console.debug(`[${level}] ${component}: ${msg}`) }
export const diagnostics = () => api()?.DiagnosticsTail(100) ?? Promise.resolve([] as string[])
export const backupNow = () => api()?.BackupNow() ?? Promise.reject(new Error('仅桌面端支持备份'))

export type UpdateChannel = 'stable' | 'preview'
export interface UpdateResult { current: string; latest: string; hasUpdate: boolean; url: string; notes: string; noRelease: boolean; preview: boolean; channel: UpdateChannel; downloadName?: string }
export const getUpdateChannel = () => api()?.UpdateChannel?.() ?? Promise.resolve(null)
export const setUpdateChannel = (channel: UpdateChannel) =>
  api()?.SetUpdateChannel?.(channel) ?? Promise.reject(new Error('桌面更新渠道不可用'))
export const appVersion = () => api()?.AppVersion() ?? Promise.resolve('v0.1.0')
/** 检查更新：仅桌面端可用（浏览器预览返回 null）。 */
export const checkUpdate = (): Promise<UpdateResult | null> => api()?.CheckUpdate() ?? Promise.resolve(null)
export const openReleasePage = (url: string) => api()?.OpenReleasePage(url) ?? Promise.resolve()
export const supportsUpdateDownload = () => !!(api()?.DownloadUpdate && api()?.CancelUpdateDownload)
export const downloadUpdate = (url: string) => api()?.DownloadUpdate?.(url) ?? Promise.resolve(false)
export const cancelUpdateDownload = () => api()?.CancelUpdateDownload?.() ?? Promise.resolve()

/** 开机启动：仅桌面端有效；浏览器预览返回 null / 不报错。 */
export const autostartEnabled = (): Promise<boolean | null> => api()?.AutostartEnabled() ?? Promise.resolve(null)
export const setAutostart = (on: boolean): Promise<void> => api()?.SetAutostart(on) ?? Promise.resolve()
