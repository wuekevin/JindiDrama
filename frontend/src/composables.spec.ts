import { describe, expect, it } from 'vitest'
import { dayBucket, agoText, isFinished, watchHours } from './composables'
const noon = new Date(2026, 10, 22, 12).getTime()
describe('history helpers', () => {
  it('buckets by calendar day', () => { expect(dayBucket(noon - 3600e3, noon)).toBe('today'); expect(dayBucket(noon - 13 * 3600e3, noon)).toBe('yesterday'); expect(dayBucket(noon - 3 * 86400e3, noon)).toBe('earlier') })
  it('ago text', () => { expect(agoText(noon, noon)).toBe('今天观看'); expect(agoText(noon - 3 * 86400e3, noon)).toBe('3天前观看') })
  it('finished only at last episode with >=95%', () => { const d = { episodeCount: 60 } as any; expect(isFinished({ dramaId: 'x', episode: 60, percent: 100, watchedAt: 0 }, d)).toBe(true); expect(isFinished({ dramaId: 'x', episode: 59, percent: 100, watchedAt: 0 }, d)).toBe(false); expect(isFinished(undefined, d)).toBe(false) })
  it('hours positive', () => { expect(watchHours([{ dramaId: 'x', episode: 10, percent: 50, watchedAt: 0 }])).toBeCloseTo(1.1, 1) })
})
