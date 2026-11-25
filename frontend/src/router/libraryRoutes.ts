import type { Drama } from '@/types'

export function savedLocalMediaIssue(drama: Drama, episode: number) {
  if (drama.kind !== '本地片库') return
  if (!drama.updatedTo) return 'library'
  if (episode > drama.updatedTo) return 'episode'
}

export function detailRoute(id: string) {
  return id.startsWith('local-') ? { name: 'licensed', query: { id } } : `/drama/${id}`
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
