import { defineStore } from 'pinia'
import { nextTick, ref, watch } from 'vue'
import type { HistoryEntry } from '@/types'
import { DEFAULT_HISTORY } from '@/mocks/provider'
import { hasBridge, readLibraryCache, restoreLibraryBackup, saveLibrary, type LibrarySaveResult } from '@/bindings'
import { useToastStore } from '@/stores/toast'

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
  let applyingRestore = false
  let latestSave: Promise<LibrarySaveResult> | undefined
  let nextSave = 0
  let lastReported = 0
  let displayedResult: LibrarySaveResult = 'saved'
  let warning: number | undefined

  function reportSave(result: LibrarySaveResult) {
    if (result === displayedResult) return
    displayedResult = result
    if (warning !== undefined) toast.dismiss(warning)
    warning = undefined
    if (result === 'pending') {
      warning = toast.push('收藏或历史已暂存在本机，重启应用后会重试写入。', 'info', 0)
    } else if (result === 'unsaved') {
      warning = toast.push('收藏或历史无法确认保存，关闭应用后可能丢失。', 'error', 0)
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
    const write = saveLibrary(history.value, favorites.value, () => report('unsaved'))
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
    applyingRestore = true
    favorites.value = result.state.favorites
    history.value = result.state.history
    await nextTick()
    applyingRestore = false
    return { favorites: result.favoritesAdded, history: result.historyAdded }
  }
  return { history, favorites, isFav, toggleFav, record, removeFavs, addFav, removeHistory, clearHistory, restoreMissingFromBackup, notifySaved }
})
