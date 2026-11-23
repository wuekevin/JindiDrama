import { afterEach, describe, expect, it } from 'vitest'
import { backoffMs, demoSources, resolveMediaMode, MAX_AUTO_RETRY } from './usePlayer'
afterEach(() => { delete (window as any).__JD_MEDIA })
describe('player helpers', () => {
  it('exponential backoff', () => { expect([0, 1, 2].map(backoffMs)).toEqual([500, 1000, 2000]); expect(MAX_AUTO_RETRY).toBe(2) })
  it('demo sources rotate over 3 clips and offer webm + mp4', () => { const s = demoSources(4); expect(s.map((x) => x.type)).toEqual(['video/webm', 'video/mp4']); expect(demoSources(1).at(0)!.src).not.toBe(demoSources(2).at(0)!.src); expect(demoSources(3).at(0)!.src).toBe(demoSources(6).at(0)!.src) })
  it('media mode: setting and test override', () => { expect(resolveMediaMode('demo')).toBe('demo'); expect(resolveMediaMode('sim')).toBe('sim'); (window as any).__JD_MEDIA = 'sim'; expect(resolveMediaMode('demo')).toBe('sim') })
})
