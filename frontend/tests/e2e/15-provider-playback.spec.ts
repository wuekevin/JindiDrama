import fs from 'node:fs'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'

const id = 'catalog:qa/001'
const route = '/#/catalog/' + encodeURIComponent(id)
const cmsId = 'apple-cms:0123456789abcdef0123456789abcdef:90071992547409931234'
const cmsTitle = 'CMS 海岛来信'
const mediaURL = 'http://127.0.0.1:45671'
const samples = {
  webm: fs.readFileSync(new URL('../../public/demo/demo-1.webm', import.meta.url)),
  mp4: fs.readFileSync(new URL('../../public/demo/demo-1.mp4', import.meta.url)),
}

async function fixture(page: Page, mode: 'ready' | 'blocked' | 'stale' | 'retry' | 'expires-live' | 'protected' | 'direct-url' | 'apple-cms' | 'apple-cms-empty') {
  await page.addInitScript(({ id, mediaURL, mode, cmsTitle }) => {
    const cms = mode === 'apple-cms' || mode === 'apple-cms-empty'
    const drama = { id, title: cms ? cmsTitle : '远海来信', tags: cms ? ['海岛'] : [], summary: '授权播放测试目录', updatedTo: cms ? 3 : 2, episodeCount: 3 }
    let attempt = 0
    const calls: string[] = []
    const released: string[] = []
    const testState = { calls, released, release: () => {} }
    Object.assign(window, { catalogTest: testState, go: { app: { App: {
      StorageReady: async () => false,
      ProviderName: async () => cms ? 'apple-cms' : 'authorized-fixture',
      Home: async () => mode === 'apple-cms-empty' ? [] : [drama],
      HomeFeed: async () => ({ catalogLabel: cms ? '片源目录' : undefined, todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      CatalogSearch: async (query: string) => query === '海岛' ? [drama] : [],
      Capabilities: async () => ({ catalog: 'ready', search: 'ready', playback: mode === 'blocked' ? 'not_authorized' : 'ready' }),
      CatalogDrama: async () => drama,
      CatalogEpisodes: async () => [1, 2, 3].map((index) => ({ id: 'episode-' + index, dramaId: id, index, available: cms ? index === 3 : index < 3 })),
      CatalogPlayback: async (_id: string, index: number) => {
        calls.push('catalog:' + _id + ':' + index)
        const sequence = ++attempt
        const leaseId = String.fromCharCode(64 + index) + String.fromCharCode(64 + sequence).repeat(25)
        if (mode === 'stale' && index === 1) await new Promise<void>((resolve) => { testState.release = resolve })
        const expiresAt = mode === 'retry' && sequence === 1 ? '2001-01-01T00:00:00Z'
          : new Date(Date.now() + (mode === 'expires-live' ? 3_000 : 120_000)).toISOString()
        const format = index === 2 ? 'mp4' : 'webm'
        const path = mediaURL + '/catalog-media/' + leaseId
        const src = mode === 'direct-url' ? 'https://media.example.test/one.webm?upstream-secret=opaque-test' : path + '/0'
        const sources = [{ src, type: 'video/' + format }]
        if (mode === 'expires-live') sources.push({ src: path + '/1', type: 'video/webm' })
        return { kind: 'authorized', episodeId: 'episode-' + index, expiresAt, leaseId, sources,
          restrictions: mode === 'protected' ? ['no-download'] : [] }
      },
      ReleaseCatalogPlayback: async (leaseId: string) => { released.push(leaseId) },
      ResolvePlayback: async () => { calls.push('legacy') },
      SaveLibraryChanges: async () => { calls.push('library') },
    } } } })
  }, { id: mode.startsWith('apple-cms') ? cmsId : id, mediaURL, mode, cmsTitle })
  const requests: string[] = []
  await page.route('https://media.example.test/**', async (request) => { requests.push(request.request().url()); await request.abort() })
  await page.route(mediaURL + '/**', async (interception) => {
    const requestURL = interception.request().url()
    requests.push(requestURL)
    const format = new URL(requestURL).pathname.startsWith('/catalog-media/B') ? 'mp4' : 'webm'
    const sample = samples[format]
    const range = /^bytes=(\d+)-(\d*)$/.exec(interception.request().headers()['range'] ?? '')
    const headers = { 'content-type': 'video/' + format, 'accept-ranges': 'bytes' }
    if (range) {
      const start = Number(range[1])
      const end = range[2] ? Math.min(sample.length - 1, Number(range[2])) : sample.length - 1
      await interception.fulfill({ status: 206, headers: { ...headers, 'content-range': `bytes ${start}-${end}/${sample.length}` }, body: sample.subarray(start, end + 1) })
    } else {
      await interception.fulfill({ status: 200, headers, body: sample })
    }
  })
  return requests
}

const calls = (page: Page) => page.evaluate(() => (window as Window & { catalogTest?: { calls: string[] } }).catalogTest?.calls)
const released = (page: Page) => page.evaluate(() => (window as Window & { catalogTest?: { released: string[] } }).catalogTest?.released)

test('authorized Provider detail opens actual playback, with pause and seek', async ({ page }, testInfo) => {
  const requests = await fixture(page, 'ready')
  await page.goto(route)
  await expect(page.getByTestId('ep').last()).toBeDisabled()
  await page.getByTestId('catalog-watch-now').click()
  await expect(page).toHaveURL(/\/play\/1$/)
  const video = page.getByLabel('第1集视频')
  await expect.poll(() => video.evaluate((v) => (v as HTMLVideoElement).duration)).toBeGreaterThan(10)
  await video.evaluate((v) => (v as HTMLVideoElement).play())
  await expect.poll(() => video.evaluate((v) => (v as HTMLVideoElement).currentTime)).toBeGreaterThan(0.1)
  await video.evaluate((v) => {
    const media = v as HTMLVideoElement
    media.pause()
    media.currentTime = 5
  })
  await expect.poll(() => video.evaluate((v) => (v as HTMLVideoElement).paused && (v as HTMLVideoElement).currentTime)).toBeCloseTo(5, 0)
  await page.screenshot({ path: testInfo.outputPath('provider-playback.png') })
  await page.getByRole('link', { name: '返回详情' }).click()
  await expect(page.locator('video')).toHaveCount(0)
  expect(requests.length).toBeGreaterThan(0)
  expect(await calls(page)).toEqual(['catalog:' + id + ':1'])
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain('opaque-test')
})

test('episode change discards the earlier ticket before any stale media request', async ({ page }) => {
  const requests = await fixture(page, 'stale')
  await page.goto(route + '/play/1')
  await expect.poll(() => calls(page)).toEqual(['catalog:' + id + ':1'])
  await page.getByRole('button', { name: '第2集' }).click()
  await expect(page.getByLabel('第2集视频')).toHaveAttribute('src', /catalog-media\/B/)
  await page.evaluate(() => (window as Window & { catalogTest?: { release(): void } }).catalogTest?.release())
  await expect.poll(() => requests.length).toBeGreaterThan(0)
  await expect(page.getByLabel('第2集视频')).toHaveAttribute('src', /catalog-media\/B/)
  await expect.poll(() => page.getByLabel('第2集视频').evaluate((v) => (v as HTMLVideoElement).duration)).toBeGreaterThan(10)
  expect(requests.every((url) => url.includes('/catalog-media/B'))).toBe(true)
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
})

test('blocked capability does not resolve a direct play URL', async ({ page }) => {
  const requests = await fixture(page, 'blocked')
  await page.goto(route + '/play/1')
  await expect(page.getByTestId('error')).toContainText('播放暂不可用')
  expect(await calls(page)).toEqual([])
  expect(requests).toEqual([])
})

test('an expired ticket sends no request and retry obtains a fresh ticket', async ({ page }) => {
  const requests = await fixture(page, 'retry')
  await page.goto(route + '/play/1')
  await expect(page.getByTestId('error')).toContainText('无法取得可用的播放源')
  expect(requests).toEqual([])
  await expect(page.getByTestId('error')).not.toContainText('opaque-test')
  await page.getByRole('button', { name: '重试' }).click()
  await expect(page.getByLabel('第1集视频')).toHaveAttribute('src', /catalog-media\/A/)
  await expect.poll(() => requests.length).toBeGreaterThan(0)
  expect(await calls(page)).toEqual(['catalog:' + id + ':1', 'catalog:' + id + ':1'])
})

test('ticket expiry removes the current stream and suppresses alternate sources', async ({ page }) => {
  const requests = await fixture(page, 'expires-live')
  await page.goto(route + '/play/1')
  await expect(page.getByLabel('第1集视频')).toBeVisible()
  await expect.poll(() => requests.length).toBeGreaterThan(0)
  await expect(page.getByTestId('error')).toContainText('播放票据已过期')
  await expect(page.locator('video')).toHaveCount(0)
  expect(requests.every((url) => url.endsWith('/0'))).toBe(true)
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
})

test('a protected ticket does not expose a stream to the browser', async ({ page }) => {
  const requests = await fixture(page, 'protected')
  await page.goto(route + '/play/1')
  await expect(page.getByTestId('error')).toContainText('无法取得可用的播放源')
  await expect(page.locator('video')).toHaveCount(0)
  expect(requests).toEqual([])
})

test('the WebView refuses a raw signed upstream URL in a ticket', async ({ page }) => {
  const requests = await fixture(page, 'direct-url')
  await page.goto(route + '/play/1')
  await expect(page.getByTestId('error')).toContainText('无法取得可用的播放源')
  expect(requests).toEqual([])
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
})

test('CMS home and search open the same title and play its explicitly numbered episode', async ({ page }, testInfo) => {
  const requests = await fixture(page, 'apple-cms')
  await page.goto('/#/home')
  const catalog = page.getByRole('region', { name: '片源目录' })
  await expect(catalog.getByRole('heading')).toHaveText('片源目录')
  await expect(catalog.getByTestId('drama-card')).toContainText(cmsTitle)
  await expect(catalog.getByText('NEW', { exact: true })).toHaveCount(0)
  await expect(page.getByText('今日上新', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('region', { name: '排行榜' })).toHaveCount(0)
  await expect(page.getByTestId('nav-rank')).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('apple-cms-home.png') })
  await catalog.getByTestId('drama-card').click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(cmsTitle)
  const detailURL = page.url()
  await page.goBack()
  await catalog.getByTestId('all-today-new').click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('片源目录')
  await expect(page.getByTestId('result-card')).toContainText(cmsTitle)
  const search = page.getByTestId('search-input')
  await search.fill('海岛')
  await search.press('Enter')
  const result = page.getByTestId('result-card')
  await expect(result).toContainText(cmsTitle)
  await result.getByTestId('view-detail').click()
  await expect(page).toHaveURL(detailURL)
  await expect(page.getByRole('button', { name: '第1集' })).toBeDisabled()
  await expect(page.getByRole('button', { name: '第2集' })).toBeDisabled()
  await page.getByRole('button', { name: '第3集' }).click()
  await expect(page).toHaveURL(/\/play\/3$/)
  const video = page.getByLabel('第3集视频')
  await expect(video).toHaveAttribute('src', /^http:\/\/127\.0\.0\.1:\d+\/catalog-media\//)
  await expect.poll(() => video.evaluate((v) => (v as HTMLVideoElement).duration)).toBeGreaterThan(10)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':3'])
  expect(requests.length).toBeGreaterThan(0)
  expect(requests.every((url) => url.startsWith(mediaURL + '/catalog-media/'))).toBe(true)
})

test('empty CMS home keeps the catalog heading and a catalog-specific empty state', async ({ page }) => {
  await fixture(page, 'apple-cms-empty')
  await page.goto('/#/home')
  const catalog = page.getByRole('region', { name: '片源目录' })
  await expect(catalog.getByRole('heading', { level: 2 })).toHaveText('片源目录')
  await expect(catalog.getByTestId('empty')).toHaveText('暂无片源')
  await expect(page.getByText('暂无推荐内容')).toHaveCount(0)
  await expect(page.getByText('稍后再来看看吧')).toHaveCount(0)
})
