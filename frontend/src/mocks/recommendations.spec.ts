import { effectScope, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import type { Drama } from '@/types'
import { DRAMAS } from './data'
import { rankRelatedDramas, useRelatedDramas } from './recommendations'

const candidate = (id: string, extra: Partial<Drama> = {}): Drama => ({
  ...DRAMAS[0], id, kind: '真人剧', tags: ['都市', '甜宠'], heat: 10, updatedTo: 1, ...extra,
})

describe('related dramas', () => {
  it('ranks type, tag overlap and heat and excludes duplicates and unavailable dramas', () => {
    const source = candidate('source')
    const list = [source, candidate('unavailable', { updatedTo: 0 }), candidate('other-type', { kind: '漫剧', heat: 100 }),
      candidate('same-type', { tags: ['古装'], heat: 100 }), candidate('matches'), candidate('matches-hot', { heat: 200 }),
      candidate('matches')]
    expect(rankRelatedDramas(source, list).map(({ id }) => id)).toEqual(['matches-hot', 'matches', 'same-type', 'other-type'])
  })

  it('cycles distinct playable batches and resets when the source changes', () => {
    const scope = effectScope()
    scope.run(() => {
      const first = candidate('a')
      const source = ref<Drama | undefined>(first)
      const rows = [first, candidate('b'), candidate('c'), candidate('d'), candidate('e'), candidate('f')]
      const { cards, canShuffle, shuffle } = useRelatedDramas(source, 2, rows)
      const ids = () => cards.value.map(({ id }) => id)
      expect(canShuffle.value).toBe(true)
      for (const batch of [['b','c'], ['d','e'], ['f','b'], ['c','d'], ['e','f'], ['b','c']]) {
        expect(ids()).toEqual(batch)
        shuffle()
      }
      expect(ids()).toEqual(['d','e'])
      source.value = undefined
      expect(ids()).toEqual([])
      source.value = rows[1]
      expect(ids()).toEqual(['a','c'])
    })
    scope.stop()
  })

  it('disables shuffle when all candidates fit and shows no cards without a source', () => {
    const scope = effectScope()
    scope.run(() => {
      const source = ref<Drama | undefined>(candidate('source'))
      const { cards, canShuffle, shuffle } = useRelatedDramas(source, 6, [source.value!, candidate('only')])
      expect(canShuffle.value).toBe(false)
      shuffle()
      expect(cards.value.map(({ id }) => id)).toEqual(['only'])
      source.value = undefined
      expect(cards.value).toEqual([])
    })
    scope.stop()
  })
})
