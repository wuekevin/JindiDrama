import { test, expect } from './fixtures'

const providerId = 'catalog:2026/001'
const providerDrama = {
  id: providerId, title: '独立目录 · 海边来信', tags: ['海岛'], summary: '目录服务提供的海岛故事。',
  updatedTo: 1, episodeCount: 2,
}
const collidingDrama = { ...providerDrama, id: 'd1', title: '目录中的另一个 d1' }
const prefixedDrama = { ...providerDrama, id: 'local-foreign', title: 'Provider 新剧' }
const licensed = { ...prefixedDrama, title: '独立本地片库 · 海岛', license: 'QA' }

async function setupProvider(page: import('@playwright/test').Page, authorized: boolean, explicitName = true, rankOnly = false) {
  await page.addInitScript(({ authorized, explicitName, rankOnly, providerDrama, collidingDrama, prefixedDrama, licensed }) => {
    const calls: string[] = []
    Object.assign(window, { providerCatalogCalls: calls, go: { app: { App: {
      StorageReady: async () => false,
      ...(explicitName ? { ProviderName: async () => 'catalog-fixture' } : {}),
      Capabilities: async () => ({
        catalog: authorized ? 'ready' : 'not_authorized',
        search: authorized ? 'ready' : 'not_authorized',
        playback: 'not_authorized',
      }),
      LicensedCatalog: async () => [licensed],
      Home: async () => { calls.push('home'); return [collidingDrama] },
      HomeFeed: async () => {
        calls.push('feed')
        return { todayNew: rankOnly ? [] : [prefixedDrama.id], rankings: { hot: [providerDrama.id, collidingDrama.id], rise: [], new: [] } }
      },
      CatalogSearch: async (query: string) => { calls.push('search:' + query); return [providerDrama, collidingDrama] },
      CatalogDrama: async (id: string) => {
        calls.push('detail:' + id)
        return [providerDrama, collidingDrama, prefixedDrama].find((drama) => drama.id === id)
      },
      CatalogEpisodes: async (id: string) => {
        calls.push('episodes:' + id)
        return [
          { index: 1, available: true, dramaId: id },
          { index: 2, available: false, dramaId: id },
        ]
      },
      ResolvePlayback: async () => { calls.push('resolve'); throw new Error('playback unavailable') },
    } } } })
  }, { authorized, explicitName, rankOnly, providerDrama, collidingDrama, prefixedDrama, licensed })
}

test('ready desktop catalog renders its own ranking, search and detail metadata', async ({ page }, testInfo) => {
  await setupProvider(page, true)
  await page.goto('/#/home')
  const ranks = page.getByTestId('rank-item')
  await expect(ranks).toHaveCount(2)
  await expect(ranks.first()).toContainText(providerDrama.title)
  await expect(ranks.last()).toContainText(collidingDrama.title)
  await expect(page.getByTestId('hero')).toHaveCount(0)
  await expect(page.getByTestId('resume-card')).toHaveCount(0)
  await expect(page.getByTestId('drama-card')).toContainText(prefixedDrama.title)
  await page.getByTestId('drama-card').click()
  await expect(page).toHaveURL(/#\/catalog\/local-foreign$/)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(prefixedDrama.title)
  await page.goBack()
  await expect(ranks.first()).not.toContainText('热度')
  await page.getByTestId('full-ranking').click()
  await expect(page.getByTestId('result-card').first()).toContainText(providerDrama.title)
  await page.goto('/#/search?q=海岛')
  const items = page.getByTestId('result-card')
  await expect(items).toHaveCount(3)
  await expect(items.first()).toContainText(licensed.title)
  await expect(items.nth(1)).toContainText(providerDrama.summary)
  await expect(items.last()).toContainText(collidingDrama.title)
  await expect(items.nth(1).getByTestId('play-now')).toHaveCount(0)
  await items.nth(1).getByTestId('view-detail').click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(providerDrama.title)
  await page.reload()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(providerDrama.title)
  await expect(page.getByTestId('summary')).toHaveText(providerDrama.summary)
  await expect(page.getByTestId('ep')).toHaveCount(2)
  await expect(page.getByTestId('ep').first()).toBeDisabled()
  await expect(page.getByTestId('tab-clips')).toHaveCount(0)
  await expect(page.getByTestId('watch-now')).toHaveCount(0)
  await expect(page.getByTestId('guess-card')).toHaveCount(0)
  await expect(page.getByText('独播')).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('provider-detail.png') })
  await page.goto('/#/watch/d1/1')
  await expect(page.getByTestId('empty')).toContainText('没有找到这部短剧')
  await page.goto('/#/watch/local-foreign/1')
  await expect(page).toHaveURL(/#\/drama\/local-foreign$/)
  await expect(page.getByTestId('empty')).toContainText('没有找到这部短剧')
  await page.goto('/#/catalog/d1')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(collidingDrama.title)
  await expect(page.locator('video')).toHaveCount(0)
  const calls = await page.evaluate(() => (window as Window & { providerCatalogCalls?: string[] }).providerCatalogCalls)
  expect(calls).toContain('detail:d1')
  expect(calls?.filter((call) => call === 'detail:d1')).toHaveLength(1)
  expect(calls).not.toContain('resolve')
})

test('unauthorized desktop catalog does not load metadata or play media, including without a provider name', async ({ page }) => {
  await setupProvider(page, false, false)
  await page.goto('/#/home')
  await expect(page.getByTestId('error')).toContainText('目录暂不可用')
  const input = page.getByTestId('search-input')
  await input.fill('海岛')
  await expect(page.getByRole('option', { name: licensed.title })).toBeVisible()
  await input.press('Enter')
  await expect(page.getByRole('status').filter({ hasText: '主目录暂不可用' })).toBeVisible()
  await expect(page.getByTestId('result-card')).toContainText(licensed.title)
  await page.getByTestId('kind-本地片库').click()
  await expect(page.getByTestId('result-card')).toContainText(licensed.title)
  await page.getByTestId('view-detail').click()
  await expect(page).toHaveURL(/#\/licensed\?id=local-foreign$/)
  await page.goto('/#/watch/d1/1')
  await expect(page.getByTestId('empty')).toContainText('没有找到这部短剧')
  await page.goto('/#/catalog/d1')
  await expect(page.getByTestId('error')).toContainText('目录暂不可用')
  await expect(page.locator('video')).toHaveCount(0)
  const calls = await page.evaluate(() => (window as Window & { providerCatalogCalls?: string[] }).providerCatalogCalls)
  expect(calls).toEqual([])
})

test('a provider with only ranked titles still has a ready home', async ({ page }) => {
  await setupProvider(page, true, true, true)
  await page.goto('/#/home')
  await expect(page.getByTestId('rank-item')).toHaveCount(2)
  await expect(page.getByTestId('hero')).toHaveCount(0)
  await expect(page.getByTestId('drama-card')).toHaveCount(0)
})
