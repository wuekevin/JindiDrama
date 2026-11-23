import { describe, expect, it } from 'vitest'
import { DEFAULT_DANMAKU, filterDanmaku, genDanmaku, isBlocked, place, laneRange, LANES } from './danmaku'
const items = genDanmaku(32, 400)
describe('danmaku engine', () => {
  it('blocks keywords', () => { expect(isBlocked('这是剧透内容', ['剧透'])).toBe(true); expect(isBlocked('好看', ['剧透'])).toBe(false); expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, density: 'high' }).every((d) => !d.text.includes('广告'))).toBe(true) })
  it('disabled shows nothing', () => { expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, enabled: false })).toEqual([]) })
  it('density monotonic', () => { const c = (density: 'low'|'mid'|'high') => filterDanmaku(items, { ...DEFAULT_DANMAKU, density, blocked: [] }).length; expect(c('low')).toBeLessThan(c('mid')); expect(c('mid')).toBeLessThan(c('high')) })
  it('hot only', () => { expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, hotOnly: true, density: 'high' }).every((d) => d.hot)).toBe(true) })
  it('area restricts lanes', () => { const [lo, hi] = laneRange('top'); expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, area: 'top', density: 'high' }).every((d) => d.lane >= lo && d.lane <= hi)).toBe(true); expect(laneRange('bottom')[1]).toBe(LANES - 1) })
  it('place: moves with time and speed', () => { const it1 = [{ id: 'a', at: 10, text: 'x', likes: 0, lane: 0, mine: true }]; expect(place(it1, 14, DEFAULT_DANMAKU)[0].x).toBeCloseTo(0.5); expect(place(it1, 12, DEFAULT_DANMAKU)[0].x).toBeCloseTo(0.25); expect(place(it1, 12, { ...DEFAULT_DANMAKU, speed: 2 })[0].x).toBeCloseTo(0.5); expect(place(it1, 30, DEFAULT_DANMAKU)).toEqual([]) })
})
