import { describe, expect, it } from 'vitest'
import { DEFAULT_DANMAKU, filterDanmaku, genDanmaku, isBlocked, activeDanmaku, scheduleDanmaku, laneRange, LANES } from './danmaku'
const items = genDanmaku(32, 400)
describe('danmaku engine', () => {
  it('blocks keywords', () => { expect(isBlocked('这是剧透内容', ['剧透'])).toBe(true); expect(isBlocked('好看', ['剧透'])).toBe(false); expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, density: 'high' }).every((d) => !d.text.includes('广告'))).toBe(true) })
  it('disabled shows nothing', () => { expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, enabled: false })).toEqual([]) })
  it('density monotonic', () => { const c = (density: 'low'|'mid'|'high') => filterDanmaku(items, { ...DEFAULT_DANMAKU, density, blocked: [] }).length; expect(c('low')).toBeLessThan(c('mid')); expect(c('mid')).toBeLessThan(c('high')) })
  it('hot only', () => { expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, hotOnly: true, density: 'high' }).every((d) => d.hot)).toBe(true) })
  it('area restricts lanes', () => { const [lo, hi] = laneRange('top'); expect(filterDanmaku(items, { ...DEFAULT_DANMAKU, area: 'top', density: 'high' }).every((d) => d.lane >= lo && d.lane <= hi)).toBe(true); expect(laneRange('bottom')[1]).toBe(LANES - 1) })
  it('moves at the selected speed and can read positions in seek order', () => {
    const items = [{ id: 'mine', at: 10, text: 'x', likes: 0, lane: 0, mine: true }]
    const normal = scheduleDanmaku(items, DEFAULT_DANMAKU, 800, 600, () => 200)
    const fast = scheduleDanmaku(items, { ...DEFAULT_DANMAKU, speed: 2 }, 800, 600, () => 200)
    expect(activeDanmaku(normal, 14)[0].left).toBeCloseTo(280)
    expect(activeDanmaku(fast, 12)[0].left).toBeCloseTo(280)
    expect(activeDanmaku(normal, 30)).toEqual([])
    expect(activeDanmaku(normal, 10)[0].left).toBe(800)
  })

  it('reserves space for local submissions and drops crowded ordinary messages', () => {
    const item = (id: string, at: number, mine = false) => ({ id, at, text: id, likes: 0, lane: 0, mine })
    const items = [item('would collide', 0), item('mine', 1, true), item('also mine', 1, true), item('clear', 6), item('invalid timestamp', Infinity)]
    const plan = scheduleDanmaku(items, { ...DEFAULT_DANMAKU, density: 'high' }, 800, 26, () => 252)
    expect(plan.trajectories.map(({ item, startsAt }) => [item.id, startsAt])).toEqual([
      ['mine', 1], ['also mine', 3], ['clear', 6],
    ])
  })
})
