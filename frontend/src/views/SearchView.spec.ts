import { describe, expect, it } from 'vitest'
import { searchDramas } from '@/mocks/provider'

describe('searchDramas', () => {
  it('matches keyword and sorts by heat', async () => {
    const r = await searchDramas('替嫁'); expect(r.length).toBe(5)
    expect(r.map((d) => d.heat)).toEqual([...r.map((d) => d.heat)].sort((a, b) => b - a))
  })
  it('filters by kind and status', async () => {
    expect((await searchDramas('替嫁', { kind: 'AI剧' })).every((d) => d.kind === 'AI剧')).toBe(true)
    expect((await searchDramas('替嫁', { status: 'finished' })).every((d) => d.finished)).toBe(true)
  })
  it('empty result for unknown keyword; error mode throws', async () => {
    expect(await searchDramas('不存在的剧xyz')).toEqual([])
    await expect(searchDramas('x', {}, 'error')).rejects.toThrow()
  })
})
