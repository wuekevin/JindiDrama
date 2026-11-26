import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'

const providerId = 'catalog:2026/001'
const providerDrama = {
  id: providerId, title: '独立目录 · 海边来信', tags: ['海岛'], summary: '目录服务提供的海岛故事。',
  updatedTo: 1, episodeCount: 2,
}
const collidingDrama = { ...providerDrama, id: 'd1', title: '目录中的另一个 d1' }
const prefixedDrama = { ...providerDrama, id: 'local-foreign', title: 'Provider 新剧' }
const licensed = { ...prefixedDrama, title: '独立本地片库 · 海岛', license: 'QA' }

async function setupProvider(page: Page, authorized: boolean, explicitName = true, rankOnly = false) {
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
      CatalogPage: async () => { calls.push('page'); throw new Error('unexpected pagination binding') },
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
  expect(calls).not.toContain('page')
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

interface CMSPageFixture {
  calls: [string, number][]
  release: () => void
}
type CMSWindow = Window & { cmsPageFixture?: CMSPageFixture }

async function setupCMSPages(page: Page) {
  await page.addInitScript(() => {
    const row = (id: number, title: string) => ({
      id: 'apple-cms:0123456789abcdef0123456789abcdef:' + id,
      title, summary: '授权片源目录提供的短剧。', tags: ['剧情'], episodeCount: 12, updatedTo: 12,
    })
    const state: CMSPageFixture = { calls: [], release: () => {} }
    let failed = false
    Object.assign(window, { cmsPageFixture: state, go: { app: { App: {
      StorageReady: async () => false,
      ProviderName: async () => 'apple-cms',
      Capabilities: async () => ({ catalog: 'ready', search: 'ready' }),
      Home: async () => [row(1, '潮汐来信')],
      HomeFeed: async () => ({ catalogLabel: '片源目录', todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      LicensedCatalog: async () => [{ ...row(20, '本地海岛短剧'), id: 'local-island', license: 'QA' }],
      CatalogSearch: async () => [],
      CatalogPage: async (keyword: string, page: number) => {
        state.calls.push([keyword, page])
        let entries: ReturnType<typeof row>[] = []
        if (!keyword) {
          entries = page === 1 ? Array.from({ length: 6 }, (_, index) => row(index + 1, '潮汐来信' + (index ? ' ' + (index + 1) : '')))
            : [row(7, '岸边灯塔'), row(8, '归航时刻'), row(9, '港口旧事')]
        } else if (keyword === '海岛') {
          if (page === 2 && !failed) {
            failed = true
            throw new Error('苹果 CMS 目录暂不可用')
          }
          entries = [page === 1 ? row(10, '海岛来信') : row(11, '海岛归航')]
        } else if (keyword === '慢速' && page === 2) {
          await new Promise<void>((resolve) => { state.release = resolve })
          entries = [row(12, '过期的第二页')]
        } else if (keyword !== '无结果' || page === 1) {
          entries = [row(13, keyword + '的第一页')]
        }
        return { entries, page, hasMore: page === 1 && keyword !== '新词' }
      },
    } } } })
  })
}

const cmsPageCalls = (page: Page) => page.evaluate(() => (window as CMSWindow).cmsPageFixture?.calls)
const searchRoute = (keyword: string, page?: number) => '/#/search?q=' + encodeURIComponent(keyword) + (page ? '&page=' + page : '')

test('CMS directory fetches the selected page and restores it through browser history', async ({ page }, testInfo) => {
  await setupCMSPages(page)
  await page.goto('/#/home')
  const section = page.getByRole('region', { name: '片源目录' })
  await expect(section.getByTestId('drama-card')).toContainText('潮汐来信')
  expect(await cmsPageCalls(page)).toEqual([])
  await section.getByRole('button', { name: '查看更多' }).click()
  const heading = page.getByRole('heading', { level: 1, name: '片源目录' })
  const cards = page.getByTestId('result-card')
  const next = page.getByRole('button', { name: '下一页' })
  const previous = page.getByRole('button', { name: '上一页' })
  await expect(heading).toBeVisible()
  await expect(cards).toHaveCount(6)
  await expect(next).toBeVisible()
  await next.focus()
  await next.press('Enter')
  await expect(page).toHaveURL(/feed=today&page=2$/)
  await expect(heading).toBeFocused()
  await expect.poll(() => page.locator('#content').evaluate((element) => element.scrollTop)).toBe(0)
  await expect(cards).toHaveCount(3)
  await expect(cards.first()).toContainText('岸边灯塔')
  await expect(page.getByTestId('result-count')).toHaveText('本页共 3 部相关内容')
  await expect(page.getByRole('navigation', { name: '分页' }).locator('[aria-current=page]')).toHaveText('第 2 页')
  await expect(next).toHaveCount(0)
  await expect(previous).toBeVisible()
  expect(await cmsPageCalls(page)).toEqual([['', 1], ['', 2]])
  await page.screenshot({ path: testInfo.outputPath('apple-cms-directory-page-2.png') })

  await page.goBack()
  await expect(cards).toHaveCount(6)
  await expect(page).toHaveURL(/feed=today$/)
  await page.goForward()
  await expect(cards.first()).toContainText('岸边灯塔')
  expect(await cmsPageCalls(page)).toEqual([['', 1], ['', 2], ['', 1], ['', 2]])
  await page.reload()
  await expect(cards.first()).toContainText('岸边灯塔')
  await expect(heading).toBeFocused()
  expect(await cmsPageCalls(page)).toEqual([['', 2]])
  await previous.click()
  await expect(cards).toHaveCount(6)
  await expect(page).toHaveURL(/feed=today$/)
})

test('CMS keyword search retries the same page and shows local matches only on page one', async ({ page }) => {
  await setupCMSPages(page)
  await page.goto(searchRoute('海岛'))
  const cards = page.getByTestId('result-card')
  const previous = page.getByRole('button', { name: '上一页' })
  await expect(cards.first()).toContainText('本地海岛短剧')
  await expect(cards.last()).toContainText('海岛来信')
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page.getByTestId('error')).toContainText('苹果 CMS 目录暂不可用')
  await expect(cards).toHaveCount(0)
  await expect(previous).toBeVisible()
  await page.getByRole('button', { name: '重试' }).click()
  await expect(cards).toHaveCount(1)
  await expect(cards.first()).toContainText('海岛归航')
  await expect(page.getByRole('heading', { level: 1 })).toBeFocused()
  expect(await cmsPageCalls(page)).toEqual([['海岛', 1], ['海岛', 2], ['海岛', 2]])

  const input = page.getByTestId('search-input')
  await input.fill('新词')
  await input.press('Enter')
  await expect(cards.first()).toContainText('新词的第一页')
  await expect(input).toBeFocused()
  await page.goBack()
  await expect(cards.first()).toContainText('海岛归航')
  await expect(page.getByRole('heading', { level: 1 })).toBeFocused()

  await previous.click()
  await expect(cards.first()).toContainText('本地海岛短剧')
  expect(await cmsPageCalls(page)).toEqual([['海岛', 1], ['海岛', 2], ['海岛', 2], ['新词', 1], ['海岛', 2], ['海岛', 1]])
})

test('CMS changes keyword during a page request and discards the late result', async ({ page }) => {
  await setupCMSPages(page)
  await page.goto(searchRoute('慢速'))
  await page.getByRole('button', { name: '下一页' }).click()
  await expect.poll(() => cmsPageCalls(page)).toEqual([['慢速', 1], ['慢速', 2]])
  const input = page.getByTestId('search-input')
  await input.fill('新词')
  await input.press('Enter')
  const cards = page.getByTestId('result-card')
  await expect(cards.first()).toContainText('新词的第一页')
  await page.evaluate(async () => {
    (window as CMSWindow).cmsPageFixture?.release()
    await new Promise(requestAnimationFrame)
  })
  await expect(cards).toHaveCount(1)
  await expect(cards.first()).toContainText('新词的第一页')
  await expect(page.getByRole('button', { name: '下一页' })).toHaveCount(0)
  expect(new URLSearchParams(new URL(page.url()).hash.split('?')[1]).get('page')).toBeNull()
  expect(await cmsPageCalls(page)).toEqual([['慢速', 1], ['慢速', 2], ['新词', 1]])
})

test('empty CMS pages can go back and unsafe hash page values resolve to page one', async ({ page }) => {
  await setupCMSPages(page)
  await page.goto(searchRoute('无结果', 2))
  await expect(page.getByTestId('empty')).toContainText('本页暂无短剧')
  const previous = page.getByRole('button', { name: '上一页' })
  await expect(previous).toBeVisible()
  await expect(page.getByRole('button', { name: '下一页' })).toHaveCount(0)
  await previous.click()
  await expect(page.getByTestId('result-card')).toContainText('无结果的第一页')
  const invalid = ['0', '-1', '100001', '1e2', '2&page=3']
  for (const [index, value] of invalid.entries()) {
    await page.goto(searchRoute('无结果') + '&page=' + value)
    await expect.poll(() => cmsPageCalls(page)).toHaveLength(3 + index)
    await expect(page.getByRole('navigation', { name: '分页' })).toContainText('第 1 页')
  }
  expect(await cmsPageCalls(page)).toEqual([['无结果', 2], ...Array.from({ length: 6 }, () => ['无结果', 1])])
})
