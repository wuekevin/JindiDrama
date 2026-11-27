import { readFileSync } from 'node:fs'
import type { Page } from '@playwright/test'
import { PNG } from 'pngjs'
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
          if (document.body.dataset.directoryOffline === '1') {
            throw new Error('苹果 CMS 目录暂不可用')
          }
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

test('CMS directory restores the retained page while the source stays offline', async ({ page }, testInfo) => {
  await setupCMSPages(page)
  await page.goto('/#/search?feed=today')
  const cards = page.getByTestId('result-card')
  const heading = page.getByRole('heading', { level: 1, name: '片源目录' })
  const next = page.getByRole('button', { name: '下一页' })
  const previous = page.getByRole('button', { name: '上一页' })
  const failure = page.getByRole('status').filter({ hasText: '当前显示已加载的第' })
  const setOffline = (offline: boolean) => page.locator('body').evaluate((element, offline) => {
    element.dataset.directoryOffline = offline ? '1' : '0'
  }, offline)

  await expect(cards).toHaveCount(6)
  await setOffline(true)
  await next.click()
  await expect(page).toHaveURL(/feed=today&page=2$/)
  await expect(failure).toContainText('当前显示已加载的第 1 页')
  await expect(cards).toHaveCount(6)
  await page.getByRole('button', { name: '返回第 1 页' }).click()
  await expect(page).toHaveURL(/feed=today$/)
  await expect(next).toBeVisible()
  await expect(failure).toHaveCount(0)
  await expect(heading).toBeFocused()
  expect(await cmsPageCalls(page)).toEqual([['', 1], ['', 2]])
  await page.screenshot({ path: testInfo.outputPath('cms-directory-offline-return.png') })

  await next.click()
  await expect(failure).toContainText('当前显示已加载的第 1 页')
  await setOffline(false)
  await page.getByRole('button', { name: '重试' }).click()
  await expect(cards.first()).toContainText('岸边灯塔')
  expect(await cmsPageCalls(page)).toEqual([['', 1], ['', 2], ['', 2], ['', 2]])

  await setOffline(true)
  await previous.click()
  await expect(failure).toContainText('当前显示已加载的第 2 页')
  await page.getByRole('button', { name: '返回第 2 页' }).click()
  await expect(page).toHaveURL(/feed=today&page=2$/)
  await expect(previous).toBeVisible()
  await expect(next).toHaveCount(0)
  await expect(failure).toHaveCount(0)
  await expect(cards.first()).toContainText('岸边灯塔')
  expect(await cmsPageCalls(page)).toEqual([['', 1], ['', 2], ['', 2], ['', 2], ['', 1]])
})

test('CMS keyword search retries the same page and shows local matches only on page one', async ({ page }, testInfo) => {
  await setupCMSPages(page)
  await page.goto(searchRoute('海岛'))
  const cards = page.getByTestId('result-card')
  const previous = page.getByRole('button', { name: '上一页' })
  await expect(cards.first()).toContainText('本地海岛短剧')
  await expect(cards.last()).toContainText('海岛来信')
  await page.getByRole('button', { name: '下一页' }).click()
  const fallback = page.getByRole('status').filter({ hasText: '当前显示已加载的第 1 页' })
  await expect(fallback).toContainText('第 2 页加载失败（苹果 CMS 目录暂不可用）')
  await expect(cards).toHaveCount(2)
  await expect(cards.first()).toContainText('本地海岛短剧')
  await expect(cards.last()).toContainText('海岛来信')
  await expect(page.getByRole('navigation', { name: '分页' }).locator('[aria-current=page]')).toHaveText('第 1 页')
  await expect(page.getByRole('button', { name: '返回第 1 页' })).toBeVisible()
  await expect(page).toHaveURL(/page=2$/)
  await page.screenshot({ path: testInfo.outputPath('cms-page-recovery-default.png') })
  const viewport = page.viewportSize()!
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((element) => element.style.setProperty('--user-zoom', '2.2'))
  await expect(fallback).toBeInViewport()
  await page.screenshot({ path: testInfo.outputPath('cms-page-recovery-220.png') })
  await page.locator('html').evaluate((element) => element.style.removeProperty('--user-zoom'))
  await page.setViewportSize(viewport)
  await page.getByRole('button', { name: '重试' }).click()
  await expect(cards).toHaveCount(1)
  await expect(cards.first()).toContainText('海岛归航')
  await expect(fallback).toHaveCount(0)
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

const wideGenre = '90071992547409931234'
type CMSGenreWindow = typeof window & {
  cmsGenreCalls: string[]
  releaseCMSGenrePage: () => void
  deleteCMSGenre: (genre: string) => void
  setNextCMSGenrePage: (outcome: 'empty' | 'empty-more' | 'error') => void
}

async function setupCMSGenres(page: Page, scenario: 'normal' | 'no-class' | 'retry-class' | 'late-page' = 'normal') {
  await page.addInitScript(({ wideGenre, scenario }) => {
    const row = (id: string, title: string, tags = ['剧情']) => ({
      id: 'apple-cms:0123456789abcdef0123456789abcdef:' + id,
      title, summary: '授权片源目录提供的短剧。', tags, episodeCount: 1, updatedTo: 1,
    })
    const allFirst = row('1', '未分类目录第一页')
    const allSecond = row('2', '未分类目录第二页')
    const first = row('3', '北海来信', ['海岛'])
    const second = row('4', '北海归航', ['海岛'])
    const other = row('5', '山中信号', ['海岛'])
    const calls: string[] = []
    const deleted = new Set<string>()
    let nextOutcome: Parameters<CMSGenreWindow['setNextCMSGenrePage']>[0] | undefined
    let classAttempts = 0
    let otherAttempts = 0
    Object.assign(window, {
      cmsGenreCalls: calls,
      deleteCMSGenre: (genre: string) => deleted.add(genre),
      setNextCMSGenrePage: (outcome: typeof nextOutcome) => { nextOutcome = outcome },
      go: { app: { App: {
      StorageReady: async () => false,
      ProviderName: async () => 'apple-cms',
      Capabilities: async () => ({ catalog: 'ready', search: 'ready' }),
      Home: async () => [allFirst],
      HomeFeed: async () => ({ catalogLabel: '片源目录', todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      LicensedCatalog: async () => [{ ...row('20', '本地海岛短剧'), id: 'local-island', license: 'QA' }],
      CatalogSearch: async () => { calls.push('search'); return [allFirst] },
      CatalogPage: async (_query: string, pg: number) => {
        calls.push('all:' + pg)
        return { entries: [pg === 1 ? allFirst : allSecond], page: pg, hasMore: pg === 1 }
      },
      CatalogGenres: async () => {
        calls.push('class')
        if (scenario === 'retry-class' && ++classAttempts === 1) throw new Error('fixture upstream URL')
        return scenario === 'no-class' ? [] : [
          { id: wideGenre, name: '海岛' }, { id: '7', name: '海岛' }, { id: '8', name: `海岛（ID ${wideGenre}）` },
        ].filter(({ id }) => !deleted.has(id))
      },
      CatalogGenrePage: async (genre: string, pg: number) => {
        calls.push('genre:' + genre + ':' + pg)
        const outcome = nextOutcome
        nextOutcome = undefined
        if (outcome === 'error') throw new Error('苹果 CMS 返回的短剧与请求分类不一致')
        if (outcome === 'empty' || deleted.has(genre)) return { entries: [], page: pg, hasMore: false }
        if (outcome === 'empty-more') return { entries: [], page: pg, hasMore: true }
        if (genre === '7' && scenario === 'late-page' && ++otherAttempts === 1) {
          throw new Error('苹果 CMS 返回的短剧与请求分类不一致')
        }
        if (scenario === 'late-page' && genre === wideGenre && pg === 2) {
          await new Promise<void>((resolve) => { Object.assign(window, { releaseCMSGenrePage: resolve }) })
        }
        return { entries: [genre === '7' ? other : pg === 1 ? first : second], page: pg, hasMore: genre === wideGenre && pg === 1 }
      },
      CatalogDrama: async (id: string) => {
        calls.push('detail:' + id)
        return [first, second, other].find((item) => item.id === id)
      },
      CatalogEpisodes: async (id: string) => [{ index: 1, available: true, dramaId: id }],
    } } } })
  }, { wideGenre, scenario })
}

const cmsGenreCalls = (page: Page) => page.evaluate(() => (window as CMSGenreWindow).cmsGenreCalls)

test('CMS classifications come from class and restore the selected ID and page through history', async ({ page }, testInfo) => {
  await setupCMSGenres(page)
  await page.goto('/#/home')
  await page.getByText('分类', { exact: true }).click()
  const heading = page.getByRole('heading', { name: '按分类浏览' })
  const select = page.getByRole('combobox', { name: '分类' })
  const cards = page.getByTestId('result-card')
  await expect(heading).toBeVisible()
  await expect(cards).toContainText('未分类目录第一页')
  await expect(select.locator('option')).toHaveText([
    '全部分类', `海岛（ID ${wideGenre} · 2）`, '海岛（ID 7）', `海岛（ID ${wideGenre}）`,
  ])
  expect(await cmsGenreCalls(page)).toEqual(['class', 'all:1'])

  await select.selectOption(wideGenre)
  await expect(page).toHaveURL(new RegExp('browse=topics&genre=' + wideGenre + '$'))
  await expect(cards).toContainText('北海来信')
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(heading).toBeFocused()
  await expect(page).toHaveURL(new RegExp('browse=topics&genre=' + wideGenre + '&page=2$'))
  await expect(select).toHaveValue(wideGenre)
  await expect(cards).toHaveCount(1)
  await expect(cards).toContainText('北海归航')
  await expect(page.getByRole('navigation', { name: '分页' })).toContainText('第 2 页')
  expect(await cmsGenreCalls(page)).toEqual(['class', 'all:1', 'class', `genre:${wideGenre}:1`, `genre:${wideGenre}:2`])
  await page.screenshot({ path: testInfo.outputPath('apple-cms-genre-page-2.png') })

  await select.selectOption('7')
  await expect(page).toHaveURL(/browse=topics&genre=7$/)
  await expect(cards).toContainText('山中信号')
  await page.goBack()
  await expect(cards).toContainText('北海归航')
  await expect(select).toHaveValue(wideGenre)
  await page.goForward()
  await expect(cards).toContainText('山中信号')
  await expect(select).toHaveValue('7')
  await page.goBack()
  await expect(cards).toContainText('北海归航')
  expect((await cmsGenreCalls(page)).filter((call) => call === 'class')).toHaveLength(6)
  await page.reload()
  await expect(cards).toContainText('北海归航')
  await expect(select).toHaveValue(wideGenre)
  expect(await cmsGenreCalls(page)).toEqual(['class', `genre:${wideGenre}:2`])
  await cards.getByTestId('view-detail').click()
  await expect(page).toHaveURL(/#\/catalog\/apple-cms%3A0123456789abcdef0123456789abcdef%3A4$/)
  await expect(page.getByRole('heading', { name: '北海归航' })).toBeVisible()
})

test('CMS without class supports all pages and recovers from unavailable ID routes', async ({ page }) => {
  await setupCMSGenres(page, 'no-class')
  await page.goto('/#/search?browse=topics&genre=7&page=2')
  await expect(page.getByTestId('empty')).toContainText('片源目录没有这个分类')
  await expect(page.getByRole('status').filter({ hasText: '片源未提供分类' })).toBeVisible()
  expect(await cmsGenreCalls(page)).toEqual(['class'])
  await page.getByRole('button', { name: '查看全部分类' }).click()
  await expect(page).toHaveURL(/browse=topics$/)
  await expect(page.getByRole('combobox', { name: '分类' })).toHaveValue('')
  await expect(page.getByTestId('result-card')).toContainText('未分类目录第一页')
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page.getByTestId('result-card')).toContainText('未分类目录第二页')
  expect(await cmsGenreCalls(page)).toEqual(['class', 'all:1', 'all:2'])
})

test('CMS invalid and expired classifications stay off the wire, and a failed class request retries', async ({ page }) => {
  await setupCMSGenres(page, 'retry-class')
  await page.goto('/#/search?browse=topics&genre=9')
  await expect(page.getByTestId('error')).toContainText('苹果 CMS 分类暂不可用')
  expect(await cmsGenreCalls(page)).toEqual(['class'])
  await page.getByRole('button', { name: '重试' }).click()
  await expect(page.getByTestId('empty')).toContainText('片源目录没有这个分类')
  for (const genre of ['0', '7&genre=9', '7%26t%3D9']) {
    await page.evaluate((genre) => { location.hash = '/search?browse=topics&genre=' + genre }, genre)
    await expect(page).toHaveURL(new RegExp('genre=' + genre + '$'))
    await expect(page.getByTestId('empty')).toContainText('片源目录没有这个分类')
  }
  expect(await cmsGenreCalls(page)).toEqual(['class', 'class'])
  await page.getByRole('button', { name: '查看全部分类' }).click()
  await expect(page.getByTestId('result-card')).toContainText('未分类目录第一页')
  expect(await cmsGenreCalls(page)).toEqual(['class', 'class', 'all:1'])
})

test('CMS classification retains its page on failure and discards it when the filter changes', async ({ page }) => {
  await setupCMSGenres(page)
  await page.goto('/#/search?browse=topics&genre=' + wideGenre)
  const cards = page.getByTestId('result-card')
  await expect(cards).toContainText('北海来信')
  const failNext = () => page.evaluate(() => (window as CMSGenreWindow).setNextCMSGenrePage('error'))
  await failNext()
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page.getByRole('status').filter({ hasText: '当前显示已加载的第 1 页' })).toContainText('第 2 页加载失败')
  await expect(cards).toContainText('北海来信')
  await failNext()
  await page.getByRole('combobox', { name: '分类' }).selectOption('7')
  await expect(page.getByTestId('error')).toContainText('苹果 CMS 分类暂不可用')
  await expect(cards).toHaveCount(0)
  await page.getByRole('button', { name: '重试' }).click()
  await expect(cards).toContainText('山中信号')
})

test('CMS switching classification discards a late page and retries a filter error', async ({ page }) => {
  await setupCMSGenres(page, 'late-page')
  await page.goto('/#/search?browse=topics&genre=' + wideGenre)
  const cards = page.getByTestId('result-card')
  await expect(cards).toContainText('北海来信')
  await page.getByRole('button', { name: '下一页' }).click()
  const initialCalls = ['class', `genre:${wideGenre}:1`, `genre:${wideGenre}:2`]
  await expect.poll(() => cmsGenreCalls(page)).toEqual(initialCalls)
  await expect(cards).toHaveCount(0)
  await page.getByRole('combobox', { name: '分类' }).selectOption('7')
  await expect(page.getByTestId('error')).toContainText('苹果 CMS 分类暂不可用')
  await expect(cards).toHaveCount(0)
  await page.getByRole('button', { name: '重试' }).click()
  await expect(cards).toContainText('山中信号')
  await page.evaluate(async () => {
    (window as CMSGenreWindow).releaseCMSGenrePage()
    await new Promise(requestAnimationFrame)
  })
  await expect(cards).toHaveCount(1)
  await expect(cards).toContainText('山中信号')
  await expect(page.getByRole('navigation', { name: '分页' })).toContainText('第 1 页')
  expect(await cmsGenreCalls(page)).toEqual([...initialCalls, 'class', 'genre:7:1', 'class', 'genre:7:1'])
})

test('CMS re-entry refreshes a deleted genre while remaining on the search view', async ({ page }) => {
  await setupCMSGenres(page)
  await page.goto('/#/search?browse=topics&genre=' + wideGenre)
  const cards = page.getByTestId('result-card')
  await expect(cards).toContainText('北海来信')
  await page.evaluate((id) => (window as CMSGenreWindow).deleteCMSGenre(id), wideGenre)
  const input = page.getByTestId('search-input')
  await input.fill('关键词')
  await input.press('Enter')
  await expect(cards).toContainText('未分类目录第一页')
  await page.goBack()
  await expect(page.getByTestId('empty')).toContainText('片源目录没有这个分类')
  await expect(page.locator(`option[value="${wideGenre}"]`)).toHaveCount(0)
  expect((await cmsGenreCalls(page)).filter((call) => call !== 'search')).toEqual([
    'class', `genre:${wideGenre}:1`, 'all:1', 'class',
  ])
})

test('CMS verifies empty pages and removes a deleted classification', async ({ page }) => {
  await setupCMSGenres(page)
  await page.goto('/#/search?browse=topics&genre=' + wideGenre)
  const cards = page.getByTestId('result-card')
  const next = page.getByRole('button', { name: '下一页' })
  await expect(cards).toContainText('北海来信')
  await page.evaluate(() => (window as CMSGenreWindow).setNextCMSGenrePage('empty'))
  await next.click()
  await expect(page.getByTestId('empty')).toContainText('本页暂无短剧')
  await page.getByRole('button', { name: '上一页' }).click()
  await expect(cards).toContainText('北海来信')
  await page.evaluate((id) => (window as CMSGenreWindow).deleteCMSGenre(id), wideGenre)
  await next.click()
  await expect(page.getByTestId('empty')).toContainText('片源目录没有这个分类')
  await expect(page.locator(`option[value="${wideGenre}"]`)).toHaveCount(0)
  expect(await cmsGenreCalls(page)).toEqual([
    'class', `genre:${wideGenre}:1`, `genre:${wideGenre}:2`, 'class',
    `genre:${wideGenre}:1`, `genre:${wideGenre}:2`, 'class',
  ])
})

test('CMS can advance beyond a page with only filtered titles', async ({ page }) => {
  await setupCMSGenres(page)
  await page.goto('/#/search?browse=topics&genre=' + wideGenre)
  const cards = page.getByTestId('result-card')
  const next = page.getByRole('button', { name: '下一页' })
  await expect(cards).toContainText('北海来信')
  await page.evaluate(() => (window as CMSGenreWindow).setNextCMSGenrePage('empty-more'))
  await next.click()
  await expect(page.getByTestId('empty')).toContainText('本页暂无短剧')
  await expect(page.getByRole('navigation', { name: '分页' })).toContainText('第 2 页')
  await next.click()
  await expect(cards).toContainText('北海归航')
  await expect(page.getByRole('navigation', { name: '分页' })).toContainText('第 3 页')
})

test('CMS retry refreshes a previously valid classification before sending a filter', async ({ page }) => {
  await setupCMSGenres(page)
  await page.goto('/#/search?browse=topics&genre=' + wideGenre)
  await expect(page.getByTestId('result-card')).toContainText('北海来信')
  await page.evaluate(() => (window as CMSGenreWindow).setNextCMSGenrePage('error'))
  await page.getByRole('button', { name: '下一页' }).click()
  const fallback = page.getByRole('status').filter({ hasText: '当前显示已加载的第 1 页' })
  await expect(fallback).toBeVisible()
  await page.evaluate((id) => (window as CMSGenreWindow).deleteCMSGenre(id), wideGenre)
  await page.getByRole('button', { name: '重试' }).click()
  await expect(page.getByTestId('empty')).toContainText('片源目录没有这个分类')
  expect(await cmsGenreCalls(page)).toEqual(['class', `genre:${wideGenre}:1`, `genre:${wideGenre}:2`, 'class'])
})

test('CMS covers use application paths and fall back without hiding detail or playback', async ({ page }, testInfo) => {
  const origin = 'http://127.0.0.1:45671'
  const coverURL = (id: number) => origin + '/catalog-cover/' + id.toString(16).padStart(32, '0')
  const approved = coverURL(1)
  const failed = coverURL(2)
  const leaseId = 'A'.repeat(26)
  const videoURL = origin + '/catalog-media/' + leaseId + '/0'
  const external = 'https://covers.example.test/photo.png?signature=fixture-only'
  const offscreen = coverURL(7)

  await page.addInitScript(({ approved, failed, external, offscreen, leaseId, videoURL }) => {
    const row = (id: number, title: string, cover?: string) => ({
      id: 'apple-cms:0123456789abcdef0123456789abcdef:' + id,
      title, cover, summary: '授权封面测试短剧。', tags: ['海岛'], episodeCount: 1, updatedTo: 1,
    })
    const valid = row(1, '港湾晚霞', approved)
    const broken = row(2, '夜航来信', failed)
    const entries = [
      valid, broken, row(3, '未经校验的封面', external), row(4, '无封面短剧'),
      row(5, '带参数的封面', approved + '?extra=1'), row(6, '外站封面', external.replace('https:', '')),
    ]
    const offscreenRow = row(7, '首页未渲染短剧', offscreen)
    const unsafePaths = [
      approved.replace('127.0.0.1', 'localhost'),
      approved.replace('127.0.0.1', 'user:pass@127.0.0.1'),
      approved.replace(':45671', ':65536'),
      approved.replace(':45671', ''),
      new URL(approved).pathname,
      approved + '#fragment',
    ].map((cover, index) => row(10 + index, '无效封面路径 ' + index, cover))
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      ProviderName: async () => 'apple-cms',
      Capabilities: async () => ({ catalog: 'ready', search: 'ready', playback: 'ready' }),
      Home: async () => [...entries, offscreenRow],
      HomeFeed: async () => ({ catalogLabel: '片源目录', todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      CatalogSearch: async () => entries,
      CatalogPage: async (_keyword: string, currentPage: number) => ({ entries: [...entries, ...unsafePaths], page: currentPage, hasMore: false }),
      CatalogGenres: async () => [{ id: '7', name: '海岛' }],
      CatalogGenrePage: async (_genre: string, currentPage: number) => ({
        entries: currentPage === 1 ? [valid] : [broken], page: currentPage, hasMore: currentPage === 1,
      }),
      CatalogDrama: async (id: string) => entries.find((entry) => entry.id === id),
      CatalogEpisodes: async (id: string) => [{ id: id + ':1', index: 1, available: true, dramaId: id }],
      CatalogPlayback: async (id: string, index: number) => {
        if (id !== broken.id || index !== 1) throw new Error('未知测试剧集')
        return {
          kind: 'authorized', episodeId: id + ':1', leaseId,
          expiresAt: new Date(Date.now() + 120_000).toISOString(),
          sources: [{ src: videoURL, type: 'video/webm' }],
        }
      },
    } } } })
  }, { approved, failed, external, offscreen, leaseId, videoURL })

  const image = new PNG({ width: 36, height: 54 })
  for (let index = 0; index < image.data.length; index += 4) {
    const row = Math.floor(index / 4 / image.width)
    const [r, g, b] = row < 26 ? [232, 131, 106] : row < 31 ? [255, 213, 145] : [39, 94, 121]
    image.data.set([r, g, b, 255], index)
  }
  const png = PNG.sync.write(image)
  const requests: string[] = []
  const externalRequests: string[] = []
  await page.route('**/catalog-cover/**', async (route) => {
    const url = route.request().url()
    requests.push(url)
    if (url === approved) {
      await route.fulfill({ contentType: 'image/png', headers: { 'Access-Control-Allow-Origin': '*' }, body: png })
    } else {
      await route.fulfill({ status: 502, body: 'Bad Gateway' })
    }
  })
  await page.route('**/covers.example.test/**', async (route) => {
    externalRequests.push(route.request().url())
    await route.abort()
  })

  const sample = readFileSync(new URL('../../public/demo/demo-1.webm', import.meta.url))
  await page.route(videoURL, async (route) => {
    const headers = { 'content-type': 'video/webm', 'accept-ranges': 'bytes' }
    const range = /^bytes=(\d+)-(\d*)$/.exec(route.request().headers()['range'] ?? '')
    if (range) {
      const start = Number(range[1])
      const end = range[2] ? Math.min(sample.length - 1, Number(range[2])) : sample.length - 1
      await route.fulfill({
        status: 206, headers: { ...headers, 'content-range': `bytes ${start}-${end}/${sample.length}` },
        body: sample.subarray(start, end + 1),
      })
      return
    }
    await route.fulfill({ headers, body: sample })
  })

  const byAlt = (alt: string) => page.getByAltText(alt, { exact: true })
  const placeholder = /^data:image\/svg\+xml/
  await page.goto('/#/home')
  const homeCover = byAlt('港湾晚霞')
  await expect(homeCover).toHaveAttribute('src', approved)
  await expect(homeCover).toHaveAttribute('loading', 'lazy')
  await expect(homeCover).toHaveAttribute('crossorigin', 'anonymous')
  await expect(homeCover).toHaveJSProperty('naturalWidth', 36)
  for (const title of ['夜航来信', '未经校验的封面', '无封面短剧', '带参数的封面', '外站封面']) {
    await expect(byAlt(title)).toHaveAttribute('src', placeholder)
  }
  expect(requests).toContain(approved)
  expect(requests).toContain(failed)
  expect(requests).not.toContain(offscreen)
  await page.screenshot({ path: testInfo.outputPath('apple-cms-cover-home.png') })
  const homeImage = await homeCover.elementHandle()
  await page.getByTestId('drama-card').filter({ hasText: '港湾晚霞' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('港湾晚霞')
  expect(await homeImage.getAttribute('src')).toBeNull()
  await expect(byAlt('港湾晚霞')).toHaveJSProperty('naturalWidth', 36)

  await page.goto(searchRoute('海岛'))
  const results = page.getByTestId('result-card')
  await expect(results.getByAltText('港湾晚霞')).toHaveJSProperty('naturalWidth', 36)
  const brokenCard = results.filter({ hasText: '夜航来信' })
  await expect(brokenCard.getByAltText('夜航来信')).toHaveAttribute('src', placeholder)
  await expect(results.getByAltText('未经校验的封面')).toHaveAttribute('src', placeholder)
  for (let index = 0; index < 6; index++) {
    await expect(results.getByAltText('无效封面路径 ' + index)).toHaveAttribute('src', placeholder)
  }
  await brokenCard.getByTestId('view-detail').click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('夜航来信')
  await expect(byAlt('夜航来信')).toHaveAttribute('src', placeholder)
  await expect.poll(() => byAlt('夜航来信').evaluate((image) => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
  await page.getByTestId('catalog-watch-now').click()
  const video = page.getByLabel('第1集视频')
  await expect(video).toHaveAttribute('src', videoURL)
  await expect.poll(() => video.evaluate((element) => (element as HTMLVideoElement).videoWidth)).toBeGreaterThan(0)
  await video.evaluate((element) => (element as HTMLVideoElement).play())
  await expect.poll(() => video.evaluate((element) => (element as HTMLVideoElement).currentTime)).toBeGreaterThan(0.1)

  await page.goto('/#/search?browse=topics&genre=7')
  await expect(results.getByAltText('港湾晚霞')).toHaveJSProperty('naturalWidth', 36)
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(results.getByAltText('夜航来信')).toHaveAttribute('src', placeholder)
  expect(externalRequests).toEqual([])
  expect([...new Set(requests)].sort()).toEqual([approved, failed])
})
