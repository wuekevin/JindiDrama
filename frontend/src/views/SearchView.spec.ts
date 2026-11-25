import { describe, expect, it, vi } from 'vitest'
import { byId } from '@/mocks/data'
import { getDetail, searchDramas } from '@/mocks/provider'

describe('searchDramas', () => {
  it('matches keyword and sorts by heat', async () => {
    const r = await searchDramas('替嫁'); expect(r.length).toBe(5)
    expect(r.map((d) => d.heat)).toEqual([...r.map((d) => d.heat)].sort((a, b) => b - a))
  })
  it('filters by kind and status', async () => {
    expect((await searchDramas('替嫁', { kind: 'AI剧' })).every((d) => d.kind === 'AI剧')).toBe(true)
    expect((await searchDramas('替嫁', { status: 'finished' })).every((d) => d.finished)).toBe(true)
  })
  it('uses the Go bridge for desktop search and retains UI filters', async () => {
    const Search = vi.fn().mockResolvedValue([{ id: 'd1' }, { id: 'd16' }])
    vi.stubGlobal('go', { app: { App: { Search } } })
    try {
      const result = await searchDramas(' 替嫁 ', { kind: 'AI剧' })
      expect(Search).toHaveBeenCalledWith('替嫁')
      expect(result.map(({ id }) => id)).toEqual(['d16'])
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('empty result for unknown keyword; error mode throws', async () => {
    expect(await searchDramas('不存在的剧xyz')).toEqual([])
    await expect(searchDramas('x', {}, 'error')).rejects.toThrow()
  })
  it('keeps actor links scoped to the exact demo cast, even with mixed provider results', async () => {
    const local = { ...byId('d2')!, id: 'local-namesake', kind: '本地片库' as const, title: '苏晚宁' }
    const card = (id: string) => id === local.id ? local : byId(id)
    const Search = vi.fn().mockResolvedValue([{ id: local.id }, { id: 'd1' }, { id: 'd3' }])
    const ProviderName = vi.fn().mockResolvedValue('mock')
    vi.stubGlobal('go', { app: { App: { Search, ProviderName } } })
    try {
      expect((await searchDramas('苏晚宁', {}, 'normal', card)).map(({ id }) => id)).toEqual(['d1', 'd3', local.id])
      expect((await searchDramas('苏晚宁', { castOnly: true }, 'normal', card)).map(({ id }) => id)).toEqual(['d1', 'd3'])
      ProviderName.mockResolvedValue('authorized')
      expect(await searchDramas('苏晚宁', { castOnly: true }, 'normal', card)).toEqual([])
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('does not attach demo cast to details from a different provider with the same ID', async () => {
    const ProviderName = vi.fn().mockResolvedValue('authorized')
    vi.stubGlobal('go', { app: { App: { ProviderName, Drama: async () => byId('d1'), Episodes: async () => [] } } })
    try {
      expect((await getDetail('d1'))?.cast).toEqual([])
      ProviderName.mockResolvedValue('mock')
      expect((await getDetail('d1'))?.cast.map(({ name }) => name)).toContain('苏晚宁')
    } finally {
      vi.unstubAllGlobals()
    }
  })
})
