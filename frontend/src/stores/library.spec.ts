import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useLibraryStore } from './library'

describe('library store', () => {
  beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
  it('toggles favorites', () => {
    const s = useLibraryStore()
    expect(s.isFav('d1')).toBe(false)
    expect(s.toggleFav('d1')).toBe(true)
    expect(s.toggleFav('d1')).toBe(false)
  })
  it('records history, most recent first, no duplicates', () => {
    const s = useLibraryStore()
    s.record('d9', 3, 10); s.record('d9', 4, 20)
    expect(s.history.filter((h) => h.dramaId === 'd9')).toHaveLength(1)
    expect(s.history[0]).toMatchObject({ dramaId: 'd9', episode: 4, percent: 20 })
  })
})
