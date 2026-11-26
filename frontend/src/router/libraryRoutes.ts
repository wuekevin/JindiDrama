import { watchStatus } from '@/composables'
import type { DramaDetail } from '@/mocks/provider'
import type { Drama, HistoryEntry } from '@/types'

export function savedMediaIssue(drama: Drama, episode: number) {
  if (drama.kind === '本地片库' && !drama.updatedTo) return 'library'
  if (!Number.isInteger(episode) || episode < 1 || episode > drama.updatedTo) return 'episode'
}

export function unfinishedBookmark(drama: Drama, history: HistoryEntry) {
  return history.percent >= 0 && history.percent < 100 && !savedMediaIssue(drama, history.episode)
}

export function continuationEpisode(drama: Drama, history: HistoryEntry) {
  if (unfinishedBookmark(drama, history)) return history.episode
  if (history.percent !== 100 || watchStatus(history, drama) !== 'watching') return
  const episode = history.episode + 1
  if (!savedMediaIssue(drama, history.episode) && !savedMediaIssue(drama, episode)) return episode
}

export function availableCatalogEpisode(detail: DramaDetail | undefined | null, episode: number) {
  return detail?.episodes.find((item) => item.index === episode && item.available && item.id)
}

export function firstAvailableCatalogEpisode(detail: DramaDetail) {
  let first: number | undefined
  for (const episode of detail.episodes) {
    if (episode.available && episode.id && (first === undefined || episode.index < first)) first = episode.index
  }
  return first
}

export function catalogBookmarkEpisode(detail: DramaDetail, history: HistoryEntry) {
  if (!detail.appleCMS || !detail.catalogPlayback || !availableCatalogEpisode(detail, history.episode)) return
  const next = continuationEpisode(detail.drama, history)
  return next !== undefined && availableCatalogEpisode(detail, next) ? next : history.episode
}

export function detailRoute(id: string, activeCatalog = false) {
  if (activeCatalog) return `/catalog/${encodeURIComponent(id)}`
  return id.startsWith('local-') ? { name: 'licensed', query: { id } } : `/drama/${encodeURIComponent(id)}`
}

export function catalogPlaybackRoute(id: string, episode: number) {
  return { name: 'catalog-play', params: { id, episode } }
}

export function playbackRoute(id: string, episode: number, autoplay = false) {
  return id.startsWith('local-')
    ? { name: 'licensed', query: { id, ep: String(episode), autoplay: autoplay ? '1' : undefined } }
    : `/watch/${id}/${episode}`
}

type ResumeEntry = { dramaId: string; episode: number }
let automatic: ResumeEntry[] = []

export function beginStartupPlayback(entries: ResumeEntry[]) {
  automatic = entries
  return startupPlaybackTarget()
}

function startupPlaybackTarget() {
  const next = automatic[0]
  if (!next) return '/home'
  if (!next.dramaId.startsWith('local-')) automatic = []
  return playbackRoute(next.dramaId, next.episode)
}

export function retryStartupPlayback(id: string, episode: number) {
  const active = automatic[0]
  if (active?.dramaId !== id || active.episode !== episode) return
  automatic.shift()
  return startupPlaybackTarget()
}

export function clearStartupPlayback() {
  automatic = []
}
