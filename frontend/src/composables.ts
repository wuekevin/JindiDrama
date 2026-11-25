import type { Drama, HistoryEntry } from '@/types'
export const isFinished = (h: HistoryEntry | undefined, d: Drama) => !!h && d.episodeCount > 0 && h.episode >= d.episodeCount && h.percent >= 100
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
