import { defineStore } from 'pinia'
import { nextTick, ref, watch } from 'vue'
import type { HistoryEntry } from '@/types'
import { DEFAULT_HISTORY } from '@/mocks/provider'
import { hasBridge, readLibraryCache, restoreLibraryBackup, retryLibrarySave, saveLibrary, type LibrarySaveResult } from '@/bindings'
import { useToastStore } from '@/stores/toast'
import type { LibraryState } from '@/bindings/libraryRecovery'

function load(): { history: HistoryEntry[]; favorites: string[] } {
  const cached = readLibraryCache()
  if (cached) return cached
  if (hasBridge()) return { history: [], favorites: [] }
  return { history: DEFAULT_HISTORY, favorites: ['d2', 'd3', 'd4', 'd5', 'd6', 'd7', 'd8', 'd10', 'd11', 'd12', 'd9', 'd1'].filter((x) => x !== 'd1') }
}
export const useLibraryStore = defineStore('library', () => {
  const init = load()
  const history = ref<HistoryEntry[]>(init.history)
  const favorites = ref<string[]>(init.favorites)
  const toast = useToastStore()
  const viewSnapshot = (): LibraryState => ({
    favorites: [...favorites.value],
    history: history.value.map((row) => ({ ...row })),
  })
  let displayedLibrary = viewSnapshot()
  let applyingRestore = false
  let latestSave: Promise<LibrarySaveResult> | undefined
  let nextSave = 0
  let lastReported = 0
  let displayedResult: LibrarySaveResult = 'saved'
  let warning: number | undefined
  const retrying = ref(false)
  const retryAction = { label: '重试保存', run: () => void retrySave(), busy: () => retrying.value }

  function reportSave(result: LibrarySaveResult) {
    if (result === displayedResult) return
    displayedResult = result
    if (warning !== undefined) toast.dismiss(warning)
    warning = undefined
    if (result === 'pending') {
      warning = toast.push('收藏或历史已暂存在本机，重启应用后会重试写入。', 'info', 0, retryAction)
    } else if (result === 'unsaved') {
      warning = toast.push('收藏或历史无法确认保存，关闭应用后可能丢失。', 'error', 0, retryAction)
    }
  }

  async function applyLibraryState(state: LibraryState) {
    applyingRestore = true
    favorites.value = state.favorites
    history.value = state.history
    displayedLibrary = viewSnapshot()
    await nextTick()
    applyingRestore = false
  }

  async function applyCachedLibrary() {
    await nextTick()
    const current = readLibraryCache()
    if (current && hasBridge()) await applyLibraryState(current)
  }

  async function retrySave() {
    if (retrying.value) return
    retrying.value = true
    const revision = ++nextSave
    try {
      const result = await retryLibrarySave(history.value, favorites.value)
      await applyCachedLibrary()
      if (revision >= lastReported) {
        lastReported = revision
        reportSave(result)
      }
    } finally {
      retrying.value = false
    }
  }

  if (hasBridge() && readLibraryCache()?.pending) reportSave('pending')
  watch([history, favorites], async () => {
    if (applyingRestore) return
    const revision = ++nextSave
    const report = (result: LibrarySaveResult) => {
      if (revision < lastReported) return
      lastReported = revision
      reportSave(result)
    }
    const before = displayedLibrary
    displayedLibrary = viewSnapshot()
    const write = saveLibrary(displayedLibrary.history, displayedLibrary.favorites, {
      before,
      onUnconfirmed: () => report('unsaved'),
      onReconciled: () => { void applyCachedLibrary() },
    })
    latestSave = write
    report(await write)
  }, { deep: true })
  async function notifySaved(message: string, duration?: number) {
    const previous = latestSave
    await nextTick()
    const write = latestSave
    if (!write || write === previous) return
    if (await write === 'saved' && latestSave === write) toast.push(message, 'success', duration)
  }

  const isFav = (id: string) => favorites.value.includes(id)
  function toggleFav(id: string) { favorites.value = isFav(id) ? favorites.value.filter((x) => x !== id) : [id, ...favorites.value]; return isFav(id) }
  function record(dramaId: string, episode: number, percent: number, positionSec?: number, durationSec?: number, episodeCountAtWatch?: number) {
    history.value = [{ dramaId, episode, percent, watchedAt: Date.now(), positionSec, durationSec, episodeCountAtWatch }, ...history.value.filter((h) => h.dramaId !== dramaId)]
  }
  const removeFavs = (ids: string[]) => { favorites.value = favorites.value.filter((x) => !ids.includes(x)) }
  const addFav = (id: string) => { if (!isFav(id)) favorites.value = [id, ...favorites.value] }
  const removeHistory = (ids: string[]) => { history.value = history.value.filter((h) => !ids.includes(h.dramaId)) }
  const clearHistory = () => { history.value = [] }
  async function restoreMissingFromBackup() {
    const result = await restoreLibraryBackup()
    if (!result) return null
    await applyLibraryState(result.state)
    return { favorites: result.favoritesAdded, history: result.historyAdded }
  }
  return { history, favorites, isFav, toggleFav, record, removeFavs, addFav, removeHistory, clearHistory, restoreMissingFromBackup, notifySaved }
})
