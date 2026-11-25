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
function localCalendarDay(timestamp: number): number {
  const date = new Date(timestamp)
  return Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()) / 86_400_000
}

export function historyDay(timestamp: number, now = Date.now()) {
  const age = Math.max(0, localCalendarDay(now) - localCalendarDay(timestamp))
  const bucket = age === 0 ? 'today' : age === 1 ? 'yesterday' : 'earlier'
  return { age, bucket } as const
}

export function agoText(age: number): string {
  return age === 0 ? '今天观看' : `${age}天前观看`
}

export function watchHours(hs: HistoryEntry[]): number {
  const minutes = hs.reduce((total, entry) => {
    if (entry.dramaId.startsWith('local-')) {
      return total + (entry.positionSec ?? (entry.durationSec ?? 0) * entry.percent / 100) / 60
    }
    return total + (entry.episode - 1 + entry.percent / 100) * 6.7
  }, 0)
  return +(minutes / 60).toFixed(1)
}
