import { isAppleCMSDramaId } from '@/catalogIdentity'
import { localId } from '@/localId'
import { loadRecoverableComments, restoreLegacyCommentBackup, stageRecoverableComments, type CommentLoad, type CommentSaveIssue, type CommentState } from './commentRecovery'
// Wails Go 绑定桥：桌面端数据写入本地 SQLite；浏览器预览（无 Go 运行时）退回 localStorage。
import type { DanmakuSubmission } from '@/player/danmakuDraft'
import { acknowledgeDanmaku, pendingDanmaku, retainDanmaku, UndurableDanmakuError } from './danmakuRecovery'
import type { DemoQualityId } from '@/player/demoMedia'
import { captureLibraryIntent, parseLibraryCache, replayLibraryBatch, type LibraryBatch, type LibraryCache, type LibraryReceipt, type LibraryState } from './libraryRecovery'
import type { Capability, CapabilityState, HistoryEntry } from '@/types'
import type { Episode, PlaybackTicket } from '../../../contracts/provider-contracts'
export type CatalogDrama = Pick<import('@/types').Drama, 'id' | 'title' | 'summary' | 'tags' | 'episodeCount' | 'updatedTo'> & { cover?: string }

export interface CatalogPage {
  entries: CatalogDrama[]
  page: number
  hasMore: boolean
}

export interface CatalogGenre {
  id: string
  name: string
}

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

interface SearchHistoryState {
  terms: string[]
  revision: number
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

export interface AppleCMSConfiguration {
  active: boolean
  savedURL: string
  managed: boolean
  canConfigure: boolean
  restartRequired: boolean
  readError?: string
}

interface GoApp {
  CatalogResumeIDs?(ids: string[]): Promise<string[]>
  AppleCMSConfiguration?(): Promise<AppleCMSConfiguration>
  TestAppleCMSConnection?(url: string): Promise<void>
  SaveAppleCMSURL?(url: string): Promise<AppleCMSConfiguration>
  ClearAppleCMSURL?(): Promise<AppleCMSConfiguration>
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
  LocalCommentsState?(dramaId: string, episode: number): Promise<CommentState>
  CompareAndSaveLocalComments?(dramaId: string, episode: number, expected: string, snapshot: string, writeId: string): Promise<string>
  SaveLocalComments(dramaId: string, episode: number, snapshot: string): Promise<void>
  LocalDataProfile?(): Promise<string>
  LocalDanmaku(dramaId: string, episode: number): Promise<LocalDanmaku[]>
  SendLocalDanmakuOnce?(dramaId: string, episode: number, offsetMs: number, text: string, requestId: string): Promise<LocalDanmaku>
  HomeFeed(): Promise<HomeFeed>
  Home?(): Promise<CatalogDrama[]>
  Capabilities?(): Promise<Partial<Record<Capability, CapabilityState>>>
  CatalogSearch?(query: string): Promise<CatalogDrama[]>
  CatalogPage?(query: string, page: number): Promise<CatalogPage>
  CatalogGenres?(): Promise<CatalogGenre[]>
  CatalogGenrePage?(genre: string, page: number): Promise<CatalogPage>
  CatalogDetail?(id: string): Promise<{ drama: CatalogDrama; episodes: Episode[]; genreId?: string }>
  CatalogDrama?(id: string): Promise<CatalogDrama>
  CatalogEpisodes?(id: string): Promise<Episode[]>
  CatalogPlayback?(id: string, episode: number): Promise<PlaybackTicket>
  ReleaseCatalogPlayback?(leaseId: string): Promise<void>

  Search(query: string): Promise<CatalogDrama[]>
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
  SearchHistoryState?(): Promise<SearchHistoryState>
  SaveSearchHistoryRevision?(state: SearchHistoryState): Promise<boolean>
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
export function hasAppleCMSSettings() {
  const a = api()
  return typeof a?.AppleCMSConfiguration === 'function' && typeof a.TestAppleCMSConnection === 'function' &&
    typeof a.SaveAppleCMSURL === 'function' && typeof a.ClearAppleCMSURL === 'function'
}

let appleCMSWrite = Promise.resolve()
let appleCMSWritesPending = 0
export const hasPendingAppleCMSWrite = () => appleCMSWritesPending > 0
export async function readAppleCMSConfiguration() {
  await appleCMSWrite
  return api()?.AppleCMSConfiguration?.()
}
export const testAppleCMSConnection = (url: string) => api()?.TestAppleCMSConnection?.(url)
export const saveAppleCMSURL = (url: string) => queueAppleCMSWrite(() => api()?.SaveAppleCMSURL?.(url))
export const clearAppleCMSURL = () => queueAppleCMSWrite(() => api()?.ClearAppleCMSURL?.())

function queueAppleCMSWrite(operation: () => Promise<AppleCMSConfiguration> | undefined) {
  appleCMSWritesPending++
  const write = appleCMSWrite.then(operation)
  appleCMSWrite = write.then(() => {}, () => {}).finally(() => { appleCMSWritesPending-- })
  return write
}

export const closeDesktopWindow = () => api()?.CloseWindow?.()
let windowCloseSettingWrite = Promise.resolve()
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
  catalogLabel?: string
  todayNew: string[]
  rankings: Record<'hot' | 'rise' | 'new', string[]>
}
export const exportDemoEpisode = (dramaId: string, episode: number, quality: DemoQualityId) => api()?.ExportDemoEpisode(dramaId, episode, quality)
export const exportDemoScreenshot = (dramaId: string, episode: number, quality: DemoQualityId, seconds: number, pngData: string) =>
  api()?.ExportDemoScreenshot?.(dramaId, episode, quality, seconds, pngData)
const episodeLikeStorageKey = (dramaId: string, episode: number) => `jindi.episode-like.${dramaId}.${episode}`
const episodeLikeWrites = new Map<string, Promise<void>>()
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

export function loadLocalComments(dramaId: string, episode: number): Promise<CommentLoad> | undefined {
  const app = api()
  if (!app) return
  const recovery = commentRecoveryBridge(app)
  if (recovery) return loadRecoverableComments(recovery, dramaId, episode, queueCommentWrite)
  return app.LocalComments(dramaId, episode).then((snapshot) => ({ snapshot }))
}

function commentRecoveryBridge(app: GoApp) {
  if (!app.LocalCommentsState || !app.CompareAndSaveLocalComments) return
  return {
    LocalCommentsState: app.LocalCommentsState.bind(app),
    CompareAndSaveLocalComments: app.CompareAndSaveLocalComments.bind(app),
  }
}

let localCommentWrites = Promise.resolve()
function queueCommentWrite(task: () => Promise<CommentSaveIssue | undefined>) {
  const write = localCommentWrites.then(task)
  // A failed snapshot must not block the rest of the queue.
  localCommentWrites = write.then(() => undefined, () => undefined)
  return write
}
export function saveLocalComments(dramaId: string, episode: number, snapshot: string) {
  const app = api()
  if (!app) return
  const recovery = commentRecoveryBridge(app)
  if (recovery) return queueCommentWrite(stageRecoverableComments(recovery, dramaId, episode, snapshot))
  return queueCommentWrite(async () => {
    await app.SaveLocalComments(dramaId, episode, snapshot)
    return undefined
  })
}

export function recoverLocalCommentBackup(dramaId: string, episode: number, key: string) {
  const app = api()
  const recovery = app && commentRecoveryBridge(app)
  if (!recovery) return Promise.resolve('failed' as const)
  return queueCommentWrite(() => restoreLegacyCommentBackup(recovery, dramaId, episode, key))
}

export const loadLocalDanmaku = (dramaId: string, episode: number) => api()?.LocalDanmaku(dramaId, episode)
const danmakuWrites = new Set<Promise<LocalDanmaku>>()
const activeDanmakuWrites = new Map<string, Promise<LocalDanmaku>>()
const danmakuProfiles = new WeakMap<GoApp, Promise<string | undefined>>()
const danmakuMemoryOnly = new Map<string, { app: GoApp; id: string; episode: number; submission: DanmakuSubmission }>()
export const hasPendingDanmakuWrite = () => danmakuMemoryOnly.size > 0
const danmakuReceiptTimeoutMs = 8_000

function waitForDanmakuResult<T>(call: Promise<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error('本地弹幕保存回执超时')), danmakuReceiptTimeoutMs)
    void call.then((result) => {
      window.clearTimeout(timer)
      resolve(result)
    }, (err) => {
      window.clearTimeout(timer)
      reject(err)
    })
  })
}

function danmakuProfile(app: GoApp) {
  const lookup = app.LocalDataProfile
  if (!lookup) return Promise.resolve(undefined)
  const cached = danmakuProfiles.get(app)
  if (cached) return cached
  const profile = waitForDanmakuResult(Promise.resolve().then(() => lookup.call(app)))
    .then((value) => /^[0-9a-f]{64}$/.test(value) ? value : undefined, () => undefined)
  danmakuProfiles.set(app, profile)
  void profile.then((value) => {
    if (!value && danmakuProfiles.get(app) === profile) danmakuProfiles.delete(app)
  })
  return profile
}

function trackDanmakuWrite(write: Promise<LocalDanmaku>) {
  danmakuWrites.add(write)
  const settle = () => danmakuWrites.delete(write)
  void write.then(settle, settle)
  return write
}

function writeDanmaku(app: GoApp, profile: string | undefined, id: string, episode: number, submission: DanmakuSubmission) {
  const send = app.SendLocalDanmakuOnce
  if (!send) return Promise.reject(new Error('本地弹幕发送需要新版桌面桥'))
  const { offsetMs, text, requestId } = submission
  const active = activeDanmakuWrites.get(requestId)
  if (active) return active
  const call = Promise.resolve().then(() => send.call(app, id, episode, offsetMs, text, requestId))
    .then((item) => {
      if (!item || item.offsetMs !== offsetMs || item.text !== text || !Number.isSafeInteger(item.id) || item.id < 1) {
        throw new Error('本地弹幕保存回执无效')
      }
      acknowledgeDanmaku(profile, id, episode, submission)
      danmakuMemoryOnly.delete(requestId)
      return item
    })
  const write = waitForDanmakuResult(call)
  activeDanmakuWrites.set(requestId, write)
  const settle = () => { if (activeDanmakuWrites.get(requestId) === write) activeDanmakuWrites.delete(requestId) }
  void write.then(settle, settle)
  return write
}

export function sendLocalDanmaku(id: string, episode: number, submission: DanmakuSubmission) {
  const app = api()
  if (!app) return
  if (!app.SendLocalDanmakuOnce) return Promise.reject(new Error('本地弹幕发送需要新版桌面桥'))
  danmakuMemoryOnly.set(submission.requestId, { app, id, episode, submission })
  return trackDanmakuWrite(danmakuProfile(app).then(async (profile) => {
    const retained = retainDanmaku(profile, id, episode, submission)
    if (retained) danmakuMemoryOnly.delete(submission.requestId)
    try {
      return await writeDanmaku(app, profile, id, episode, submission)
    } catch (error) {
      if (!retained && app.LocalDataProfile) throw new UndurableDanmakuError()
      throw error
    }
  }))
}

export async function recoverLocalDanmaku(
  id: string,
  episode: number,
  current: () => boolean,
  onSaved: (item: LocalDanmaku, entry: DanmakuSubmission) => void,
) {
  const app = api()
  if (!app?.SendLocalDanmakuOnce) return false
  const profile = await danmakuProfile(app)
  if (!current()) return false
  const stored = profile ? pendingDanmaku(profile, id, episode) : app.LocalDataProfile ? undefined : []
  let pending = !stored
  const queued = new Map(stored?.map((entry) => [entry.requestId, entry]))
  for (const [request, pending] of danmakuMemoryOnly) {
    if (pending.app === app && pending.id === id && pending.episode === episode) queued.set(request, pending.submission)
  }
  for (const entry of queued.values()) {
    if (!current()) break
    if (retainDanmaku(profile, id, episode, entry)) danmakuMemoryOnly.delete(entry.requestId)
    try {
      onSaved(await trackDanmakuWrite(writeDanmaku(app, profile, id, episode, entry)), entry)
    } catch {
      pending = true
    }
  }
  return pending
}

export const hasLicensedLibrary = () => typeof api()?.LicensedCatalog === 'function'
export const hasLicensedExport = () => typeof api()?.ExportLicensedEpisode === 'function'
export const exportLicensedEpisode = (dramaId: string, episode: number) => api()?.ExportLicensedEpisode?.(dramaId, episode)
export const readLicensedConfiguration = () => api()?.LicensedConfiguration?.()
export const chooseLicensedDirectory = () => api()?.ChooseLicensedDirectory?.()
export const clearLicensedDirectory = () => api()?.ClearLicensedDirectory?.()
export const loadLicensedCatalog = () => api()?.LicensedCatalog?.()
export const loadHomeFeed = () => api()?.HomeFeed()
export const loadProviderCatalog = () => api()?.Home?.()
export const loadProviderCapabilities = () => api()?.Capabilities?.()
export const hasProviderCapabilities = () => typeof api()?.Capabilities === 'function'
export const catalogResumeIDs = (ids: string[]) => api()?.CatalogResumeIDs?.(ids)
export const activeProviderName = () => api()?.ProviderName?.()
export const searchProvider = (query: string) => api()?.Search(query)
export const searchActiveCatalog = (query: string) => api()?.CatalogSearch?.(query)
export const loadCatalogPage = (query: string, page: number) => api()?.CatalogPage?.(query, page)
export const loadCatalogGenres = () => api()?.CatalogGenres?.()
export const loadCatalogGenrePage = (genre: string, page: number) => api()?.CatalogGenrePage?.(genre, page)
export const loadCatalogDrama = (id: string) => api()?.CatalogDrama?.(id)
export const hasCatalogPlayback = () => typeof api()?.CatalogPlayback === 'function'
export const resolveCatalogPlayback = (id: string, episode: number) => api()?.CatalogPlayback?.(id, episode)
export const releaseCatalogPlayback = (leaseId: string) => api()?.ReleaseCatalogPlayback?.(leaseId)
export const loadProviderEpisodes = (id: string) => api()?.Episodes(id)
export function loadProviderDetail(id: string, activeCatalog = false) {
  const app = api()
  if (!app) return
  if (activeCatalog) {
    const appleCMS = isAppleCMSDramaId(id)
    if (appleCMS && app.CatalogDetail) {
      return app.CatalogDetail(id).then(({ drama, episodes, genreId }) => [drama, episodes, genreId] as const)
    }
    if (!app.CatalogDrama || !app.CatalogEpisodes) return
    const requests = [app.CatalogDrama(id), app.CatalogEpisodes(id)] as const
    if (!appleCMS) return Promise.all(requests)
    // 库页的分批核验需要等到两项请求都结束，才可核验下一部短剧。
    return Promise.allSettled(requests).then(([drama, episodes]) => {
      if (drama.status === 'rejected') throw drama.reason
      if (episodes.status === 'rejected') throw episodes.reason
      return [drama.value, episodes.value] as const
    })
  }
  return Promise.all([app.Drama(id), app.Episodes(id)])
}
export const resolvePlayback = (dramaId: string, episode: number) => api()?.ResolvePlayback(dramaId, episode)
const warn = (e: unknown) => console.warn('[bridge]', e)

export const LS = { library: 'jindi.library.v1', search: 'jindi.search.v1', app: 'jindi.settings.v1', player: 'jindi.player.v1', auth: 'jindi.auth.v1' }
const SETTING_KEY: Record<string, string> = { [LS.app]: 'app', [LS.player]: 'player', [LS.auth]: 'auth_demo' }
const PENDING_SETTING_FIELD = '__jindiPendingSetting'
const LEGACY_PLAYER_PENDING = 'jindi.player.pending.v1'
const SEARCH_REVISION_CACHE = 'jindi.search.revision.v1'

interface LibrarySnapshot {
  favorites: string[]
  history: Map<string, HistoryRow>
}

let persistedLibrary: LibrarySnapshot | undefined
let libraryWrites = Promise.resolve()
let searchWrites = Promise.resolve(true)
let desktopSearch: string[] | undefined
let searchRevision = 0
const settingWrites = new Map<string, Promise<void>>()

function queueLibraryTask<T>(task: () => Promise<T>): Promise<T> {
  const queued = libraryWrites.then(task)
  libraryWrites = queued.then(() => undefined, () => undefined)
  return queued
}
function queuedWindowWrites() {
  return [windowCloseSettingWrite, appleCMSWrite, libraryWrites, searchWrites, localCommentWrites, ...danmakuWrites, ...settingWrites.values(), ...episodeLikeWrites.values()]
}

export async function waitForWindowWrites() {
  let pending = queuedWindowWrites()
  for (;;) {
    await Promise.allSettled(pending)
    const current = queuedWindowWrites()
    // Another queue can accept work while an earlier snapshot is settling.
    if (current.length === pending.length && current.every((write, index) => write === pending[index])) return
    pending = current
  }
}
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
  try {
    localStorage.setItem(LS.library, serialized)
    return true
  } catch {
    return false
  }
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
  return queueLibraryTask(async () => {
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

async function recoverLibrary(a: GoApp, database: LibraryState, pending: LibraryCache): Promise<LibrarySaveResult> {
  const batches = pendingBatches(pending)
  // Assign a receipt to a legacy journal before any new edits can append.
  if (!pending.batches) storeLibraryCache({ ...pending, batches })
  offlineDraft = undefined
  let confirmed = 0
  let current = database
  try {
    for (const batch of batches) {
      const snapshot = await a.SaveLibraryChanges(batchChanges(batch, current))
      confirmed++
      if (snapshot) {
        current = hydrateLibraryState(snapshot)
      } else {
        // Older bridges acknowledge without a snapshot. Keep the known
        // mutation if the subsequent refresh fails.
        current = replayLibraryBatch(current, batch)
        current = await readDesktopLibrary(a)
      }
    }
  } catch (e) {
    warn(e)
    persistedLibrary = undefined
  }

  // Saves arriving during recovery append to the same journal and wait for
  // the rebased database. Remove only the confirmed prefix.
  const remaining = (readLibraryCache()?.batches ?? batches).slice(confirmed)
  if (!remaining.length) {
    storeLibraryCache(current)
    return 'saved'
  }
  const state = remaining.reduce(replayLibraryBatch, current)
  return storeLibraryCache({ ...state, pending: true, batches: remaining }) ? 'pending' : currentJournalResult()
}

/** Hydrate desktop storage before stores are created. */
export async function prime(): Promise<void> {
  desktopSearch = undefined
  persistedLibrary = undefined
  desktopCache = undefined
  offlineDraft = undefined
  rebaseLibraryWrites = false
  const a = api()
  if (!a) return
  const storageReady = await primeDesktopStorage(a)
  if (a.WindowCloseAction) {
    try {
      const closeAction = await a.WindowCloseAction()
      const settings = JSON.parse(loadCachedSetting(LS.app) ?? '{}')
      if (closeAction) {
        writeCachedSetting(LS.app, JSON.stringify({ ...settings, closeAction }), Boolean(readPendingSetting(LS.app)))
      } else if (typeof settings?.closeAction === 'string') {
        await setWindowCloseSetting(settings.closeAction)
      }
    } catch (e) {
      warn(e)
    }
  }
  if (storageReady) {
    for (const [ls, key] of Object.entries(SETTING_KEY)) {
      try {
        const pending = readPendingSetting(ls)
        if (pending) queueSetting(a, ls, key, pending)
      } catch (e) {
        warn(e)
      }
    }
  }
}

async function primeDesktopStorage(a: GoApp): Promise<boolean> {
  try {
    if (!(await a.StorageReady())) return false
  } catch (e) {
    warn(e)
    return false
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
    const queued = searchWrites
    const state = a.SearchHistoryState ? await a.SearchHistoryState() : { terms: await a.SearchHistory(), revision: 0 }
    if (searchWrites === queued) {
      const cached = readSearchHistory()
      searchRevision = Math.max(searchRevision, state.revision, cached?.revision ?? 0)
      if (cached?.pending && cached.revision > state.revision) {
        desktopSearch = cached.terms
        void queueSearch(a, { terms: cached.terms, revision: cached.revision }, true)
      } else {
        desktopSearch = state.terms
        writeSearchHistory(state, false)
      }
    }
  } catch (e) {
    warn(e)
  }
  for (const [ls, key] of Object.entries(SETTING_KEY)) {
    try {
      if (readPendingSetting(ls)) continue
      const current = localStorage.getItem(ls)
      const queued = settingWrites.get(key)
      const value = await a.GetSetting(key)
      if (value && settingWrites.get(key) === queued && localStorage.getItem(ls) === current) {
        localStorage.setItem(ls, value)
      }
    } catch (e) {
      warn(e)
    }
  }
  return true
}

export type LibrarySaveResult = 'saved' | 'pending' | 'unsaved'

function journalResult(persisted: boolean): LibrarySaveResult {
  return persisted ? 'pending' : 'unsaved'
}

function matchesDurableJournal(current: LibraryCache, durable: LibraryCache): boolean {
  if (!current.batches) return JSON.stringify(current) === JSON.stringify(durable)
  if (durable.batches) {
    const savedBatches = new Set(durable.batches.map((batch) => JSON.stringify(batch)))
    return current.batches.every((batch) => savedBatches.has(JSON.stringify(batch)))
  }
  // A legacy journal may still be durable if writing its new receipt failed.
  const [legacy] = current.batches
  return current.batches.length === 1 && legacy.legacy === true &&
    JSON.stringify(legacy.intent ?? legacy.snapshot) ===
      JSON.stringify(durable.intent ?? { history: durable.history, favorites: durable.favorites })
}

function currentJournalResult(): LibrarySaveResult {
  const current = readLibraryCache()
  if (!current?.pending) return 'saved'
  try {
    const durable = parseLibraryCache(localStorage.getItem(LS.library))
    return journalResult(durable?.pending === true && matchesDurableJournal(current, durable))
  } catch {
    return 'unsaved'
  }
}

export const hasUnconfirmedLibrarySave = () => currentJournalResult() === 'unsaved'

export function retryLibrarySave(history: HistoryEntry[], favorites: string[]): Promise<LibrarySaveResult> {
  const a = api()
  if (!a) return saveLibrary(history, favorites)

  return queueLibraryTask<LibrarySaveResult>(async () => {
    if (!readLibraryCache()?.pending) return 'saved'
    try {
      if (!(await a.StorageReady())) return currentJournalResult()
      const database = await readDesktopLibrary(a)
      rebaseLibraryWrites = true
      const pending = readLibraryCache()
      return pending?.pending ? await recoverLibrary(a, database, pending) : 'saved'
    } catch (e) {
      warn(e)
      return currentJournalResult()
    }
  })
}

interface LibrarySaveEvents {
  before?: LibraryState
  onUnconfirmed?: () => void
  onReconciled?: () => void
}

export async function saveLibrary(
  history: HistoryEntry[], favorites: string[], events?: LibrarySaveEvents,
): Promise<LibrarySaveResult> {
  const next = { history, favorites }
  const a = api()
  if (!a) return storeLibraryCache(next) ? 'saved' : 'unsaved'

  const previous = readLibraryCache()
  // A queued SQLite acknowledgement can add records before the view renders
  // them. Only a change to the view the user edited can express deletion.
  const before = events?.before ?? previous
  const stage = (batch: LibraryBatch) => rebaseLibraryWrites && previous && events?.before
    ? replayLibraryBatch(previous, batch) : next
  const batches = pendingBatches(previous)
  const last = batches.at(-1)
  if (!persistedLibrary && last?.intent && offlineDraft
      && last.receipt.writer === offlineDraft.writer && last.receipt.revision === offlineDraft.revision) {
    const batch = { receipt: offlineDraft, intent: captureLibraryIntent({ ...before!, intent: last.intent }, next) }
    return journalResult(storeLibraryCache({
      ...stage(batch), pending: true, batches: [...batches.slice(0, -1), batch],
    }))
  }

  const receipt = nextLibraryReceipt()
  const batch = { receipt, intent: captureLibraryIntent(before, next) }
  const journal = [...batches, batch]
  const state = stage(batch)
  if (!persistedLibrary) {
    offlineDraft = receipt
    return journalResult(storeLibraryCache({ ...state, pending: true, batches: journal }))
  }
  return queueLibraryWrite(a, state, journal, receipt, events)
}

function acknowledgeLibraryWrite(receipt: LibraryReceipt, database?: LibraryState) {
  const latest = readLibraryCache()
  if (!latest?.batches) return
  const remaining = latest.batches.filter(({ receipt: batch }) =>
    batch.writer !== receipt.writer || batch.revision > receipt.revision)
  const state = database ? remaining.reduce(replayLibraryBatch, database)
    : { favorites: latest.favorites, history: latest.history }
  storeLibraryCache(remaining.length ? { ...state, pending: true, batches: remaining } : state)
}

async function writeRebasedLibrary(
  a: GoApp, previous: LibrarySnapshot, receipt: LibraryReceipt, onReconciled?: () => void,
) {
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
    acknowledgeLibraryWrite(batch.receipt, database)
    onReconciled?.()
  }
}

function queuedJournalResult(receipt: LibraryReceipt): LibrarySaveResult {
  try {
    const durable = parseLibraryCache(localStorage.getItem(LS.library))
    return journalResult(pendingBatches(durable).some(({ receipt: stored }) =>
      stored.writer === receipt.writer && stored.revision === receipt.revision))
  } catch {
    return 'unsaved'
  }
}

function queueLibraryWrite(
  a: GoApp, state: LibraryState, batches: LibraryBatch[], receipt: LibraryReceipt, events?: LibrarySaveEvents,
) {
  const committed = { history: state.history, favorites: state.favorites }
  if (!storeLibraryCache({ ...committed, pending: true, batches })) events?.onUnconfirmed?.()
  const pendingRows = state.history.map(toHistoryRow)
  const pendingFavorites = [...state.favorites]
  return queueLibraryTask<LibrarySaveResult>(async () => {
    const previous = persistedLibrary
    if (!previous) return queuedJournalResult(receipt)
    try {
      // Replay edits queued during restore in receipt order against the new
      // snapshot. An earlier failed edit stays pending until SQLite accepts it.
      if (rebaseLibraryWrites) {
        await writeRebasedLibrary(a, previous, receipt, events?.onReconciled)
        return 'saved'
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
      return 'saved'
    } catch (e) {
      warn(e)
      return queuedJournalResult(receipt)
    }
  })
}
function isSearchHistory(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((term) => typeof term === 'string')
}

function readCachedSearchRevision(): number {
  try {
    const revision = Number(localStorage.getItem(SEARCH_REVISION_CACHE))
    if (Number.isSafeInteger(revision) && revision > 0) return revision
  } catch { /* SQLite may still supply the version. */ }
  return 0
}

function readSearchHistory(): (SearchHistoryState & { pending: boolean }) | undefined {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(LS.search) ?? 'null')
    if (isSearchHistory(value)) return { terms: value, revision: readCachedSearchRevision(), pending: false }
    if (value && typeof value === 'object' && 'terms' in value && isSearchHistory(value.terms)
        && 'revision' in value && typeof value.revision === 'number'
        && Number.isSafeInteger(value.revision) && value.revision >= 0) {
      return { terms: value.terms, revision: value.revision, pending: 'pending' in value && value.pending === true }
    }
  } catch { /* Ignore unavailable or malformed cached history. */ }
}

export function loadCachedSearchHistory(): string[] | undefined {
  return (api() ? desktopSearch : undefined) ?? readSearchHistory()?.terms
}

function writeSearchHistory(state: SearchHistoryState, pending: boolean): boolean {
  try {
    localStorage.setItem(LS.search, JSON.stringify(pending ? { ...state, pending } : state.terms))
  } catch {
    return false
  }
  if (!pending && state.revision > 0) {
    try { localStorage.setItem(SEARCH_REVISION_CACHE, String(state.revision)) } catch { /* Advisory cache only. */ }
  }
  return true
}

function queueSearch(app: GoApp, state: SearchHistoryState, cachedForRecovery: boolean, userEdit = false): Promise<boolean> {
  const write: Promise<boolean> = searchWrites.then(async () => {
    if (searchWrites !== write) return true
    let snapshot = state
    let recoverable = cachedForRecovery
    try {
      if (app.SaveSearchHistoryRevision) {
        if (!(await app.SaveSearchHistoryRevision(snapshot))) {
          recoverable = false
          if (!userEdit || !app.SearchHistoryState || searchWrites !== write) return false
          const current = await app.SearchHistoryState()
          if (searchWrites !== write) return true
          searchRevision = Math.max(searchRevision, current.revision) + 1
          snapshot = { terms: snapshot.terms, revision: searchRevision }
          recoverable = writeSearchHistory(snapshot, true)
          if (!(await app.SaveSearchHistoryRevision(snapshot))) return false
        }
      } else {
        await app.SaveSearchHistory(snapshot.terms)
      }
    } catch (e) {
      warn(e)
      return recoverable
    }
    if (searchWrites === write && !writeSearchHistory(snapshot, false) && !recoverable) {
      try { localStorage.removeItem(LS.search) } catch { /* The local cache is unavailable. */ }
    }
    return true
  })
  searchWrites = write
  return write
}

export function saveSearch(terms: string[]): Promise<boolean> {
  const snapshot = [...terms]
  const app = api()
  if (!app) return Promise.resolve(writeSearchHistory({ terms: snapshot, revision: 0 }, false))
  desktopSearch = snapshot
  searchRevision = Math.max(searchRevision, readSearchHistory()?.revision ?? 0) + 1
  const state = { terms: snapshot, revision: searchRevision }
  const cachedForRecovery = writeSearchHistory(state, true)
  return queueSearch(app, state, cachedForRecovery, true)
}

function settingSnapshot(raw: string): Record<string, unknown> | undefined {
  try {
    const value = JSON.parse(raw)
    if (value && typeof value === 'object' && !Array.isArray(value)) return value
  } catch { /* Ignore malformed settings. */ }
}

export function loadCachedSetting(ls: string): string | null {
  const raw = localStorage.getItem(ls)
  if (!raw) return raw
  const value = settingSnapshot(raw)
  if (value?.[PENDING_SETTING_FIELD] !== true) return raw
  delete value[PENDING_SETTING_FIELD]
  return JSON.stringify(value)
}

function readPendingSetting(ls: string): string | undefined {
  const raw = localStorage.getItem(ls)
  const value = raw ? settingSnapshot(raw) : undefined
  if (value?.[PENDING_SETTING_FIELD] === true) {
    delete value[PENDING_SETTING_FIELD]
    return JSON.stringify(value)
  }
  if (ls === LS.player && localStorage.getItem(LEGACY_PLAYER_PENDING) === '1') {
    if (value && raw) return raw
    localStorage.removeItem(LEGACY_PLAYER_PENDING)
  }
}

function writeCachedSetting(ls: string, serialized: string, pending: boolean) {
  const snapshot = pending ? settingSnapshot(serialized) : undefined
  if (snapshot) snapshot[PENDING_SETTING_FIELD] = true
  localStorage.setItem(ls, snapshot ? JSON.stringify(snapshot) : serialized)
}

function queueSetting(app: GoApp, ls: string, key: string, serialized: string) {
  const write: Promise<void> = (settingWrites.get(key) ?? Promise.resolve()).then(async () => {
    if (settingWrites.get(key) !== write) return
    await app.SaveSetting(key, serialized)
    try {
      if (settingWrites.get(key) === write && readPendingSetting(ls) === serialized) {
        writeCachedSetting(ls, serialized, false)
        if (ls === LS.player) localStorage.removeItem(LEGACY_PLAYER_PENDING)
      }
    } catch { /* The desktop database already has this setting. */ }
  }).catch(warn)
  settingWrites.set(key, write)
}

export function saveSetting(lsKey: string, value: unknown) {
  const serialized = JSON.stringify(value)
  const key = SETTING_KEY[lsKey]
  const app = api()
  try { writeCachedSetting(lsKey, serialized, Boolean(app && key)) } catch { /* ignore */ }
  if (key && app) queueSetting(app, lsKey, key, serialized)
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
export class LibraryBackupPendingError extends Error {
  constructor() {
    super('收藏或观看历史尚未写入数据库。请先重试保存，再创建备份。')
  }
}

export function backupNow() {
  const app = api()
  if (!app) return Promise.reject(new Error('仅桌面端支持备份'))
  return queueLibraryTask(async () => {
    // The queue can finish with edits still in the recovery journal.
    if (readLibraryCache()?.pending) throw new LibraryBackupPendingError()
    return app.BackupNow()
  })
}

export type UpdateChannel = 'stable' | 'preview'
export interface UpdateResult { current: string; latest: string; hasUpdate: boolean; url: string; notes: string; notesTruncated?: boolean; noRelease: boolean; preview: boolean; channel: UpdateChannel; downloadName?: string }
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
