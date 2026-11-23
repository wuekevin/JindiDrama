import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCatalogStore } from './catalog'

describe('catalog store', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('loads home data', async () => { const s = useCatalogStore(); await s.loadHome('normal'); expect(s.status).toBe('ready'); expect(s.home!.todayNew).toHaveLength(6); expect(s.home!.rankings.hot).toHaveLength(5) })
  it('reports empty', async () => { const s = useCatalogStore(); await s.loadHome('empty'); expect(s.status).toBe('empty') })
  it('reports error', async () => { const s = useCatalogStore(); await s.loadHome('error'); expect(s.status).toBe('error'); expect(s.error).toContain('重试') })
})
