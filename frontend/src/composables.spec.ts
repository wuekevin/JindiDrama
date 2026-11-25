import { describe, expect, it } from 'vitest'
import { historyDay, agoText, watchStatus, watchHours } from './composables'
const noon = new Date(2026, 10, 22, 12).getTime()
describe('history helpers', () => {
  it('buckets by local calendar day across midnight and month boundaries', () => {
    const now = new Date(2026, 10, 1, 0, 1).getTime()
    expect(historyDay(new Date(2026, 9, 31, 23, 59).getTime(), now)).toEqual({ age: 1, bucket: 'yesterday' })
    expect(historyDay(new Date(2026, 9, 30, 23, 59).getTime(), now)).toEqual({ age: 2, bucket: 'earlier' })
    expect(historyDay(new Date(2026, 10, 2, 12).getTime(), now)).toEqual({ age: 0, bucket: 'today' })
    expect(historyDay(noon, noon)).toEqual({ age: 0, bucket: 'today' })
  })
  it('formats the calendar age', () => {
    expect(agoText(0)).toBe('今天观看')
    expect(agoText(3)).toBe('3天前观看')
  })
  it('requires the final episode to reach 100% before marking the drama finished', () => {
    const drama = { episodeCount: 60 } as any
    const bookmark = { dramaId: 'x', episode: 60, percent: 100, watchedAt: 0 }
    expect(watchStatus(bookmark, drama)).toBe('done')
    expect(watchStatus({ ...bookmark, percent: 99 }, drama)).toBe('watching')
    expect(watchStatus({ ...bookmark, episode: 59 }, drama)).toBe('watching')
    expect(watchStatus(undefined, drama)).toBe('unwatched')
  })
  it('uses the recorded local episode count even when the live catalog changes', () => {
    const drama = { kind: '本地片库', episodeCount: 1 } as const
    const bookmark = { dramaId: 'local-x', episode: 2, percent: 100, watchedAt: 0 }
    expect(watchStatus({ ...bookmark, episodeCountAtWatch: 3 }, drama as any)).toBe('watching')
    expect(watchStatus({ ...bookmark, episodeCountAtWatch: 2 }, { ...drama, episodeCount: 0 } as any)).toBe('done')
    expect(watchStatus(bookmark, drama as any)).toBe('unknown')
    expect(watchStatus({ ...bookmark, percent: 99 }, drama as any)).toBe('watching')
  })
  it('hours positive', () => { expect(watchHours([{ dramaId: 'x', episode: 10, percent: 50, watchedAt: 0 }])).toBeCloseTo(1.1, 1) })
})
