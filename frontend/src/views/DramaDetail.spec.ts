import { describe, expect, it, vi } from 'vitest'
import { getDetail, groupEpisodes, epHot } from '@/mocks/provider'
describe('detail mock', () => {
  it('groups 80 episodes into 4 groups of 20', () => { const g = groupEpisodes(80); expect(g).toHaveLength(4); expect(g[3]).toMatchObject({ from: 61, to: 80 }); expect(groupEpisodes(45).at(-1)!.to).toBe(45) })
  it('returns detail, null for unknown id, throws in error mode', async () => {
    const d = await getDetail('d1'); expect(d!.cast).toHaveLength(4); expect(d!.seasons).toHaveLength(2)
    expect(await getDetail('zzz')).toBeNull(); await expect(getDetail('d1', 'error')).rejects.toThrow()
  })
  it('uses desktop detail and episode availability from the provider', async () => {
    const Drama = vi.fn().mockResolvedValue({ title: '桌面目录标题' })
    const Episodes = vi.fn().mockResolvedValue([{ index: 1, available: false }])
    vi.stubGlobal('go', { app: { App: { Drama, Episodes } } })
    try {
      const result = await getDetail('d1')
      expect(Drama).toHaveBeenCalledWith('d1')
      expect(Episodes).toHaveBeenCalledWith('d1')
      expect(result?.drama.title).toBe('桌面目录标题')
      expect(result?.episodes).toEqual([{ index: 1, available: false }])
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('hot episode marker', () => { expect(epHot(6)).toBe(true); expect(epHot(7)).toBe(false) })
})
