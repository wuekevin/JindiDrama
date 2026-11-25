import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useLibraryStore } from './library'

describe('library store', () => {
  beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
  afterEach(() => vi.unstubAllGlobals())
  it('does not seed personal records for a new desktop library', () => {
    vi.stubGlobal('go', { app: { App: {} } })
    const store = useLibraryStore()
    expect(store.history).toEqual([])
    expect(store.favorites).toEqual([])
    store.toggleFav('d1')
    expect(store.history).toEqual([])
  })
  it('reads an older cache with missing history without importing demo records', () => {
    vi.stubGlobal('go', { app: { App: {} } })
    localStorage.setItem('jindi.library.v1', '{"favorites":["d1"]}')
    const store = useLibraryStore()
    expect(store.history).toEqual([])
    expect(store.favorites).toEqual(['d1'])
  })
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
