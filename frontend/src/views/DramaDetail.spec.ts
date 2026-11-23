import { describe, expect, it } from 'vitest'
import { getDetail, groupEpisodes, epHot } from '@/mocks/provider'
describe('detail mock', () => {
  it('groups 80 episodes into 4 groups of 20', () => { const g = groupEpisodes(80); expect(g).toHaveLength(4); expect(g[3]).toMatchObject({ from: 61, to: 80 }); expect(groupEpisodes(45).at(-1)!.to).toBe(45) })
  it('returns detail, null for unknown id, throws in error mode', async () => {
    const d = await getDetail('d1'); expect(d!.cast).toHaveLength(4); expect(d!.seasons).toHaveLength(2)
    expect(await getDetail('zzz')).toBeNull(); await expect(getDetail('d1', 'error')).rejects.toThrow()
  })
  it('hot episode marker', () => { expect(epHot(6)).toBe(true); expect(epHot(7)).toBe(false) })
})
