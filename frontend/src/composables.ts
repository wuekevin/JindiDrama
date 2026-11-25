import type { Drama, HistoryEntry } from '@/types'
export function watchStatus(history: HistoryEntry | undefined, drama: Drama): 'unwatched' | 'watching' | 'done' | 'unknown' {
  if (!history) return 'unwatched'
  if (history.percent < 100) return 'watching'
  if (drama.kind === '本地片库') {
    if (history.episodeCountAtWatch === undefined) return 'unknown'
    return history.episode === history.episodeCountAtWatch ? 'done' : 'watching'
  }
  return drama.episodeCount > 0 && history.episode >= drama.episodeCount ? 'done' : 'watching'
}
export function dayBucket(ts: number, now = Date.now()): 'today' | 'yesterday' | 'earlier' {
  const s = new Date(now); s.setHours(0, 0, 0, 0); const t0 = s.getTime()
  return ts >= t0 ? 'today' : ts >= t0 - 86400e3 ? 'yesterday' : 'earlier'
}
export function agoText(ts: number, now = Date.now()): string { const d = Math.floor((now - ts) / 86400e3); return d <= 0 ? '今天观看' : `${d}天前观看` }
export function watchHours(hs: HistoryEntry[]): number {
  const minutes = hs.reduce((total, entry) => {
    if (entry.dramaId.startsWith('local-')) {
      return total + (entry.positionSec ?? (entry.durationSec ?? 0) * entry.percent / 100) / 60
    }
    return total + (entry.episode - 1 + entry.percent / 100) * 6.7
  }, 0)
  return +(minutes / 60).toFixed(1)
}
