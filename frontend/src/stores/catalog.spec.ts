import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCatalogStore } from './catalog'

describe('catalog store', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('loads home data', async () => { const s = useCatalogStore(); await s.loadHome('normal'); expect(s.status).toBe('ready'); expect(s.home!.todayNew).toHaveLength(6); expect(s.home!.rankings.hot).toHaveLength(5) })
  it('uses desktop feed ordering and local poster metadata', async () => {
    const HomeFeed = vi.fn().mockResolvedValue({
      todayNew: ['d6', 'd2'],
      rankings: { hot: ['d9', 'd8'], rise: [], new: ['d3'] },
    })
    vi.stubGlobal('go', { app: { App: { HomeFeed } } })
    try {
      const store = useCatalogStore()
      await store.loadHome()
      expect(HomeFeed).toHaveBeenCalledOnce()
      expect(store.status).toBe('ready')
      expect(store.home?.todayNew.map(({ id }) => id)).toEqual(['d6', 'd2'])
      expect(store.home?.rankings.hot.map(({ id }) => id)).toEqual(['d9', 'd8'])
      expect(store.home?.todayNew[0].cover).toMatch(/^data:image/)
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('reports empty', async () => { const s = useCatalogStore(); await s.loadHome('empty'); expect(s.status).toBe('empty') })
  it('reports error', async () => { const s = useCatalogStore(); await s.loadHome('error'); expect(s.status).toBe('error'); expect(s.error).toContain('重试') })
})
