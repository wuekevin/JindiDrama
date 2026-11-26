import { watchStatus } from '@/composables'
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

export function detailRoute(id: string, activeCatalog = false) {
  if (activeCatalog) return `/catalog/${encodeURIComponent(id)}`
  return id.startsWith('local-') ? { name: 'licensed', query: { id } } : `/drama/${encodeURIComponent(id)}`
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
