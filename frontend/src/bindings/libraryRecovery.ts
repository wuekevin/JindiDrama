import type { HistoryEntry } from '@/types'

export interface LibraryState {
  favorites: string[]
  history: HistoryEntry[]
}

export interface LibraryIntent {
  addedFavorites: string[]
  removedFavorites: string[]
  favoriteOrder: string[]
  updatedHistory: HistoryEntry[]
  removedHistory: string[]
}

export interface LibraryReceipt {
  writer: string
  revision: number
}

export type LibraryBatch = { receipt: LibraryReceipt; legacy?: true } & (
  { intent: LibraryIntent; snapshot?: never } | { intent?: never; snapshot: LibraryState }
)

export interface LibraryCache extends LibraryState {
  pending?: true
  batches?: LibraryBatch[]
  intent?: LibraryIntent
}

const validID = (id: unknown): id is string => typeof id === 'string' && id.length > 0
const validIDs = (ids: unknown): ids is string[] => Array.isArray(ids) && ids.every(validID)
function isHistoryEntry(entry: HistoryEntry): boolean {
  return Boolean(entry && validID(entry.dramaId) && Number.isInteger(entry.episode) && entry.episode > 0
    && Number.isFinite(entry.percent) && Number.isFinite(entry.watchedAt))
}

function validReceipt(receipt: LibraryReceipt | undefined): receipt is LibraryReceipt {
  return Boolean(receipt && typeof receipt.writer === 'string' && /^[a-f0-9]{32}$/.test(receipt.writer)
    && Number.isSafeInteger(receipt.revision) && receipt.revision > 0)
}

function validIntent(intent: LibraryIntent | undefined): intent is LibraryIntent {
  return Boolean(intent && validIDs(intent.addedFavorites) && validIDs(intent.removedFavorites)
    && validIDs(intent.favoriteOrder) && Array.isArray(intent.updatedHistory)
    && intent.updatedHistory.every(isHistoryEntry) && validIDs(intent.removedHistory))
}

function validBatch(batch: LibraryBatch): boolean {
  if (!batch || !validReceipt(batch.receipt)) return false
  if ('intent' in batch) return validIntent(batch.intent)
  const snapshot = batch.snapshot
  return Boolean(snapshot && validIDs(snapshot.favorites)
    && Array.isArray(snapshot.history) && snapshot.history.every(isHistoryEntry))
}

export function parseLibraryCache(raw: string | null): LibraryCache | undefined {
  try {
    const value = JSON.parse(raw ?? 'null') as LibraryCache | null
    if (!value || typeof value !== 'object') return
    const favorites = Array.isArray(value.favorites) ? value.favorites.filter(validID) : []
    const history = Array.isArray(value.history) ? value.history.filter(isHistoryEntry) : []
    if (value.pending !== true) return { favorites, history }
    if ('batches' in value) {
      return Array.isArray(value.batches) && value.batches.length > 0 && value.batches.every(validBatch)
        ? { favorites, history, pending: true, batches: value.batches }
        : { favorites, history }
    }
    const { intent } = value
    if (!('intent' in value) || validIntent(intent)) return { favorites, history, pending: true, intent }
    return { favorites, history }
  } catch {
    return undefined
  }
}

function sameHistoryEntry(previous: HistoryEntry | undefined, next: HistoryEntry): boolean {
  return previous !== undefined && previous.episode === next.episode && previous.percent === next.percent
    && previous.watchedAt === next.watchedAt && (previous.positionSec ?? 0) === (next.positionSec ?? 0)
    && (previous.durationSec ?? 0) === (next.durationSec ?? 0)
}

export function captureLibraryIntent(previous: LibraryCache | undefined, next: LibraryState): LibraryIntent {
  const before = previous ?? { favorites: [], history: [] }
  const intent = previous?.intent
  const favorites = new Set(next.favorites)
  const previousFavorites = new Set(before.favorites)
  const addedFavorites = new Set(intent?.addedFavorites)
  const removedFavorites = new Set(intent?.removedFavorites)
  for (const id of before.favorites) {
    if (favorites.has(id)) continue
    addedFavorites.delete(id)
    removedFavorites.add(id)
  }
  for (const id of next.favorites) {
    if (previousFavorites.has(id)) continue
    removedFavorites.delete(id)
    addedFavorites.add(id)
  }

  const oldHistory = new Map(before.history.map((entry) => [entry.dramaId, entry]))
  const newHistory = new Set(next.history.map(({ dramaId }) => dramaId))
  const updatedHistory = new Map(intent?.updatedHistory.map((item) => [item.dramaId, item]))
  const removedHistory = new Set(intent?.removedHistory)
  for (const old of before.history) {
    if (newHistory.has(old.dramaId)) continue
    updatedHistory.delete(old.dramaId)
    removedHistory.add(old.dramaId)
  }
  for (const entry of next.history) {
    if (sameHistoryEntry(oldHistory.get(entry.dramaId), entry)) continue
    removedHistory.delete(entry.dramaId)
    updatedHistory.set(entry.dramaId, entry)
  }
  return {
    addedFavorites: next.favorites.filter((id) => addedFavorites.has(id)),
    removedFavorites: [...removedFavorites],
    favoriteOrder: JSON.stringify(before.favorites) === JSON.stringify(next.favorites)
      ? intent?.favoriteOrder ?? []
      : next.favorites,
    updatedHistory: [...updatedHistory.values()],
    removedHistory: [...removedHistory],
  }
}

export function replayLibraryBatch(database: LibraryState, batch: LibraryBatch): LibraryState {
  const intent = batch.intent
  if (!intent) return batch.snapshot

  const history = new Map(database.history.map((entry) => [entry.dramaId, entry]))
  for (const id of intent.removedHistory) history.delete(id)
  for (const entry of intent.updatedHistory) {
    const saved = history.get(entry.dramaId)
    if (!saved || saved.watchedAt <= entry.watchedAt) history.set(entry.dramaId, entry)
  }

  return {
    history: [...history.values()].sort((left, right) => right.watchedAt - left.watchedAt),
    favorites: mergeFavoriteOrder(database.favorites, intent),
  }
}

// Keep untouched entries in SQLite order. Place re-additions next to their
// preceding visible favorite, which preserves undo at its original position.
function mergeFavoriteOrder(current: string[], intent: LibraryIntent): string[] {
  const added = new Set(intent.addedFavorites)
  const changed = new Set([...intent.removedFavorites, ...added])
  const result = current.filter((id) => !changed.has(id))
  let anchor = ''
  for (const id of [...intent.favoriteOrder, ...intent.addedFavorites]) {
    if (added.has(id) && !result.includes(id)) result.splice(result.indexOf(anchor) + 1, 0, id)
    if (result.includes(id)) anchor = id
  }
  return result
}
