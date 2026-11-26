import fs from 'node:fs'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'

const id = 'catalog:qa/001'
const route = '/#/catalog/' + encodeURIComponent(id)
const cmsId = 'apple-cms:0123456789abcdef0123456789abcdef:90071992547409931234'
const cmsTitle = 'CMS 海岛来信'
const cmsRoute = '/#/catalog/' + encodeURIComponent(cmsId)
type HistoryRow = {
  dramaId: string; episode: number; percent: number; watchedAt: number
  positionMs: number; durationMs: number; episodeCountAtWatch: number
}
type CatalogState = {
  id: string; provider: string; available: number[]
  format: 'webm' | 'hls'; expired?: boolean; offline?: boolean
}
const mediaURL = 'http://127.0.0.1:45671'
const hlsType = 'application/vnd.apple.mpegurl'
const hlsSample = fs.readFileSync(new URL('../fixtures/hls/blue.mpegts', import.meta.url))
const samples = {
  webm: fs.readFileSync(new URL('../../public/demo/demo-1.webm', import.meta.url)),
  mp4: fs.readFileSync(new URL('../../public/demo/demo-1.mp4', import.meta.url)),
}

async function fixture(page: Page, mode: 'ready' | 'blocked' | 'stale' | 'retry' | 'expires-live' | 'protected' | 'direct-url' | 'apple-cms' | 'apple-cms-empty' | 'apple-cms-hls' | 'apple-cms-hls-fallback' | 'apple-cms-hls-expires' | 'apple-cms-history') {
  await page.addInitScript(({ id, mediaURL, mode, cmsTitle, hlsType }) => {
    const cms = mode.startsWith('apple-cms')
    const hls = mode.startsWith('apple-cms-hls')
    const historyFixture = mode === 'apple-cms-history'
    const desktop = window as unknown as {
      readDesktopCatalog(): Promise<CatalogState & { history: HistoryRow[] }>
      saveDesktopCatalog(changes: { history: HistoryRow[]; removed: string[] }): Promise<{ favorites: string[]; history: HistoryRow[] }>
    }
    if (hls || historyFixture) {
      // Chromium on macOS can use the system HLS decoder. Exercise MSE here.
      const canPlayType = HTMLMediaElement.prototype.canPlayType
      HTMLMediaElement.prototype.canPlayType = function (type) {
        return type === hlsType ? '' : canPlayType.call(this, type)
      }
    }
    const drama = { id, title: cms ? cmsTitle : '远海来信', tags: cms ? ['海岛'] : [], summary: '授权播放测试目录', updatedTo: cms && !hls ? 3 : 2, episodeCount: 3 }
    let attempt = 0
    const calls: string[] = []
    const released: string[] = []
    const revoked: string[] = []
    if (hls) {
      const revokeObjectURL = URL.revokeObjectURL
      URL.revokeObjectURL = (address) => {
        revoked.push(address)
        revokeObjectURL.call(URL, address)
      }
    }
    const events = new Map<string, (attempt: string) => void>()
    const lookups: string[] = []
    const closeReplies: { id: string; unconfirmed: boolean }[] = []
    const testState = {
      calls, released, revoked, lookups, closeReplies, release: () => {},
      close: () => events.get('jindi-before-window-close')?.('close-1'),
      hide: () => events.get('jindi-before-window-hide')?.(''),
      cancelClose: () => events.get('jindi-window-close-canceled')?.('close-1'),
    }
    if (historyFixture) {
      Object.assign(window, { runtime: {
        EventsOn: (name: string, handler: (attempt: string) => void) => events.set(name, handler),
        EventsEmit: (_: string, id: string, unconfirmed: boolean) => closeReplies.push({ id, unconfirmed }),
      } })
    }
    Object.assign(window, { catalogTest: testState, go: { app: { App: {
      StorageReady: async () => historyFixture,
      Favorites: async () => [],
      History: async () => (await desktop.readDesktopCatalog()).history,
      SearchHistory: async () => [],
      GetSetting: async (key: string) => key === 'app' ? '{"startPage":"上次观看"}' : '',
      ProviderName: async () => historyFixture ? (await desktop.readDesktopCatalog()).provider : cms ? 'apple-cms' : 'authorized-fixture',
      Home: async () => mode === 'apple-cms-empty' ? [] : [drama],
      HomeFeed: async () => ({ catalogLabel: cms ? '片源目录' : undefined, todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      CatalogSearch: async (query: string) => query === '海岛' ? [drama] : [],
      CatalogPage: async (query: string, page: number) => ({
        entries: mode !== 'apple-cms-empty' && (!query || query === '海岛') ? [drama] : [],
        page,
        hasMore: false,
      }),
      Capabilities: async () => ({ catalog: 'ready', search: 'ready', playback: mode === 'blocked' ? 'not_authorized' : 'ready' }),
      CatalogDrama: async (requestedId: string) => {
        if (!historyFixture) return drama
        lookups.push(requestedId)
        const catalog = await desktop.readDesktopCatalog()
        if (catalog.offline) throw new Error('unavailable')
        if (catalog.id !== requestedId) throw 'provider: 未找到'
        return { ...drama, id: requestedId, updatedTo: Math.max(0, ...catalog.available) }
      },
      CatalogEpisodes: async (requestedId: string) => {
        if (!historyFixture) return [1, 2, 3].map((index) => ({ id: 'episode-' + index, dramaId: id, index, available: cms && !hls ? index === 3 : index < 3 }))
        const catalog = await desktop.readDesktopCatalog()
        if (catalog.id !== requestedId) throw 'provider: 未找到'
        return [1, 2, 3].map((index) => ({
          id: 'episode-' + index, dramaId: requestedId, index, available: catalog.available.includes(index),
        }))
      },
      CatalogPlayback: async (_id: string, index: number) => {
        calls.push('catalog:' + _id + ':' + index)
        const sequence = ++attempt
        const leaseId = String.fromCharCode(64 + index) + String.fromCharCode(64 + sequence).repeat(25)
        if (mode === 'stale' && index === 1) await new Promise<void>((resolve) => { testState.release = resolve })
        const catalog = historyFixture ? await desktop.readDesktopCatalog() : undefined
        const expiresAt = (mode === 'retry' && sequence === 1) || catalog?.expired ? '2001-01-01T00:00:00Z'
          : new Date(Date.now() + (mode === 'apple-cms-hls-expires' ? 4_000 : mode === 'expires-live' ? 3_000 : 120_000)).toISOString()
        const format = !historyFixture && index === 2 ? 'mp4' : 'webm'
        const path = mediaURL + '/catalog-media/' + leaseId
        const src = mode === 'direct-url' ? 'https://media.example.test/one.webm?upstream-secret=opaque-test' : path + '/0'
        const sources = [{ src, type: 'video/' + format }]
        if (hls && index === 1) {
          sources[0] = { src: path + '/0.m3u8', type: hlsType }
          if (mode !== 'apple-cms-hls') sources.push({ src: path + '/1', type: 'video/webm' })
        }
        if (catalog?.format === 'hls') sources[0] = { src: path + '/0.m3u8', type: hlsType }
        if (mode === 'expires-live' || historyFixture && catalog?.format === 'webm') sources.push({ src: path + '/1', type: 'video/webm' })
        return { kind: 'authorized', episodeId: 'episode-' + index, expiresAt, leaseId, sources,
          restrictions: mode === 'protected' ? ['no-download'] : [] }
      },
      ReleaseCatalogPlayback: async (leaseId: string) => { released.push(leaseId) },
      ResolvePlayback: async () => { calls.push('legacy') },
      SaveLibraryChanges: async (changes: { history: HistoryRow[]; removed: string[] }) => {
        if (historyFixture) return desktop.saveDesktopCatalog(changes)
        calls.push('library')
      },
    } } } })
  }, { id: mode.startsWith('apple-cms') ? cmsId : id, mediaURL, mode, cmsTitle, hlsType })
  const requests: string[] = []
  await page.route('https://media.example.test/**', async (request) => { requests.push(request.request().url()); await request.abort() })
  await page.route(mediaURL + '/**', async (interception) => {
    const requestURL = interception.request().url()
    requests.push(requestURL)
    const path = new URL(requestURL).pathname
    if ((mode.startsWith('apple-cms-hls') || mode === 'apple-cms-history') && (path.endsWith('.m3u8') || path.includes('/hls/'))) {
      const origin = interception.request().headers()['origin']
      const headers: Record<string, string> = origin ? { 'access-control-allow-origin': origin } : {}
      if (mode === 'apple-cms-hls-fallback') {
        await interception.fulfill({ status: 404, headers })
      } else if (path.endsWith('/0.m3u8') && !path.includes('/hls/')) {
        const body = `#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=32000\n${path.replace(/0[.]m3u8$/, 'hls/0.m3u8')}\n`
        await interception.fulfill({ headers: { ...headers, 'content-type': hlsType }, body })
      } else if (path.endsWith('.m3u8')) {
        const body = `#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:2\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:2,\n${path.replace(/0[.]m3u8$/, '1')}\n#EXT-X-ENDLIST\n`
        await interception.fulfill({ headers: { ...headers, 'content-type': hlsType }, body })
      } else {
        await interception.fulfill({ headers: { ...headers, 'content-type': 'video/mp2t' }, body: hlsSample })
      }
      return
    }
    const format = mode !== 'apple-cms-history' && path.startsWith('/catalog-media/B') ? 'mp4' : 'webm'
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
const revoked = (page: Page) => page.evaluate(() => (window as Window & { catalogTest?: { revoked: string[] } }).catalogTest?.revoked)

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
  expect(await page.evaluate(() => localStorage.getItem('jindi.library.v1') ?? '')).not.toContain(id)
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


test('CMS HLS decodes Chromium video under one lease, then switches episodes and exits', async ({ page }) => {
  const requests = await fixture(page, 'apple-cms-hls')
  await page.goto('/#/catalog/' + encodeURIComponent(cmsId) + '/play/1')
  const video = page.getByLabel('第1集视频')
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState)).toBeGreaterThanOrEqual(2)
  await expect(video).toHaveAttribute('src', /^blob:/)
  const objectURL = await video.getAttribute('src')
  expect(await video.evaluate((v: HTMLVideoElement) => v.videoWidth)).toBe(64)
  await video.evaluate((v: HTMLVideoElement) => v.play())
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(0.1)
  await video.evaluate((v: HTMLVideoElement) => {
    v.pause()
    v.currentTime = 1.25
  })
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.paused && !v.seeking && v.currentTime)).toBeCloseTo(1.25)
  const hlsRoot = mediaURL + '/catalog-media/' + 'A'.repeat(26) + '/'
  expect([...new Set(requests)]).toEqual([hlsRoot + '0.m3u8', hlsRoot + 'hls/0.m3u8', hlsRoot + 'hls/1'])
  const hlsRequestCount = requests.length

  await page.getByRole('button', { name: '第2集' }).click()
  await expect(video).toHaveCount(0)
  await expect.poll(() => revoked(page)).toContain(objectURL)
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
  await expect.poll(() => page.getByLabel('第2集视频').evaluate((v: HTMLVideoElement) => v.duration)).toBeGreaterThan(10)
  expect(requests.slice(hlsRequestCount).every((address) => address.startsWith(mediaURL + '/catalog-media/' + 'B'.repeat(26) + '/'))).toBe(true)
  await page.getByRole('link', { name: '返回详情' }).click()
  await expect(page.locator('video')).toHaveCount(0)
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26), 'B'.repeat(26)])
})

test('a fatal HLS response falls back to a direct source under the same lease', async ({ page }) => {
  const requests = await fixture(page, 'apple-cms-hls-fallback')
  await page.goto('/#/catalog/' + encodeURIComponent(cmsId) + '/play/1')
  const video = page.getByLabel('第1集视频')
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.videoWidth)).toBeGreaterThan(0)
  const prefix = mediaURL + '/catalog-media/' + 'A'.repeat(26) + '/'
  expect(requests).toContain(prefix + '0.m3u8')
  expect(requests).toContain(prefix + '1')
  expect(await released(page)).toEqual([])
})

test('HLS ticket expiry destroys playback and suppresses its direct alternate', async ({ page }) => {
  const requests = await fixture(page, 'apple-cms-hls-expires')
  await page.goto('/#/catalog/' + encodeURIComponent(cmsId) + '/play/1')
  const video = page.getByLabel('第1集视频')
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.videoWidth)).toBe(64)
  const objectURL = await video.getAttribute('src')
  await video.evaluate((v: HTMLVideoElement) => v.play())
  await expect(page.getByTestId('error')).toContainText('播放票据已过期')
  await expect(video).toHaveCount(0)
  await expect.poll(() => revoked(page)).toContain(objectURL)
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
  const prefix = mediaURL + '/catalog-media/' + 'A'.repeat(26) + '/'
  expect([...new Set(requests)]).toEqual([prefix + '0.m3u8', prefix + 'hls/0.m3u8', prefix + 'hls/1'])
})


test('switching episodes while hls.js loads discards the earlier stream', async ({ page }) => {
  const requests = await fixture(page, 'apple-cms-hls')
  let allowChunk = () => {}
  const chunkGate = new Promise<void>((resolve) => { allowChunk = resolve })
  const chunkRequested = page.waitForRequest('**/assets/hls-*.js')
  await page.route('**/assets/hls-*.js', async (request) => {
    await chunkGate
    await request.continue()
  })
  try {
    await page.goto('/#/catalog/' + encodeURIComponent(cmsId) + '/play/1')
    await chunkRequested
    await page.getByRole('button', { name: '第2集' }).click()
    await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
    await expect.poll(() => page.getByLabel('第2集视频').evaluate((v: HTMLVideoElement) => v.duration)).toBeGreaterThan(10)
    const chunkReady = page.waitForResponse('**/assets/hls-*.js')
    allowChunk()
    await (await chunkReady).finished()
    await page.evaluate(async () => {
      // Drain microtasks after the module download and the Vue update.
      await new Promise(requestAnimationFrame)
      await new Promise(requestAnimationFrame)
    })
    expect(requests.length).toBeGreaterThan(0)
    expect(requests.every((address) => address.startsWith(mediaURL + '/catalog-media/' + 'B'.repeat(26) + '/'))).toBe(true)
    await expect(page.getByLabel('第1集视频')).toHaveCount(0)
  } finally {
    allowChunk()
  }
})

async function cmsHistoryFixture(page: Page, initial: HistoryRow[] = []) {
  const catalog: CatalogState = { id: cmsId, provider: 'apple-cms', available: [1, 2, 3], format: 'webm' }
  let history = initial
  const writes: HistoryRow[][] = []
  await page.exposeFunction('readDesktopCatalog', () => ({ ...catalog, history }))
  await page.exposeFunction('saveDesktopCatalog', (changes: { history: HistoryRow[]; removed: string[] }) => {
    const ids = new Set([...changes.removed, ...changes.history.map(({ dramaId }) => dramaId)])
    history = [...changes.history, ...history.filter(({ dramaId }) => !ids.has(dramaId))]
    writes.push(changes.history)
    return { favorites: [], history }
  })
  const requests = await fixture(page, 'apple-cms-history')
  return { catalog, writes, requests, rows: () => history }
}

const historyVideo = (page: Page, episode: number) => page.getByLabel('第' + episode + '集视频')
const durationMs = (video: ReturnType<typeof historyVideo>) => video.evaluate((media: HTMLVideoElement) =>
  Math.round(media.duration * 1000),
)
const position = (video: ReturnType<typeof historyVideo>) => video.evaluate((media: HTMLVideoElement) =>
  media.seeking ? -1 : media.currentTime,
)

async function playHistoryVideo(video: ReturnType<typeof historyVideo>) {
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  const start = await position(video)
  await video.evaluate((media: HTMLVideoElement) => media.play())
  await expect.poll(() => position(video)).toBeGreaterThan(start + 0.1)
}

test('CMS bookmark persists through detail, history, refresh and the normal window-close handshake', async ({ page }, testInfo) => {
  const desktop = await cmsHistoryFixture(page)
  await page.goto(cmsRoute + '/play/2')
  const video = historyVideo(page, 2)
  await playHistoryVideo(video)
  const duration = await durationMs(video)
  await video.evaluate((media: HTMLVideoElement) => {
    media.pause()
    media.currentTime = 5
  })
  await expect.poll(() => desktop.rows()[0]).toMatchObject({
    dramaId: cmsId, episode: 2, positionMs: 5000, percent: 20, durationMs: duration, episodeCountAtWatch: 3,
  })
  await page.getByRole('link', { name: '返回详情' }).click()
  const resume = page.getByTestId('catalog-continue')
  await expect(resume).toContainText('第2集')
  await page.screenshot({ path: testInfo.outputPath('apple-cms-history-detail.png') })
  await page.getByTestId('catalog-watch-now').click()
  await expect(page).toHaveURL(/\/play\/1$/)
  await expect.poll(() => historyVideo(page, 1).evaluate((media: HTMLVideoElement) => media.duration)).toBeGreaterThan(10)
  await page.getByRole('link', { name: '返回详情' }).click()
  await resume.click()
  await expect.poll(() => position(video)).toBeCloseTo(5, 2)

  await page.getByTestId('nav-history').click()
  const row = page.getByTestId('history-item')
  await expect(row).toContainText(cmsTitle)
  await expect(page.getByText(/累计观看.*0.*小时/)).toBeVisible()
  await expect(row.getByTestId('resume')).toHaveText('继续观看')
  await page.screenshot({ path: testInfo.outputPath('apple-cms-history.png') })
  const writes = desktop.writes.length
  await row.getByTestId('resume').click()
  await expect.poll(() => position(video)).toBeCloseTo(5, 2)
  await page.reload()
  await expect.poll(() => position(video)).toBeCloseTo(5, 2)
  await expect(video).toHaveJSProperty('paused', true)
  expect(desktop.writes).toHaveLength(writes)

  await playHistoryVideo(video)
  const beforeRefresh = await position(video)
  await page.reload()
  await expect.poll(() => position(video)).toBeGreaterThanOrEqual(beforeRefresh - 0.001)
  await expect(video).toHaveJSProperty('paused', true)
  await expect.poll(() => desktop.rows()[0]?.positionMs).toBeGreaterThan(5000)

  await playHistoryVideo(video)
  const closePosition = await video.evaluate((media: HTMLVideoElement) => {
    const state = (window as unknown as { catalogTest: { close(): void } }).catalogTest
    state.close()
    state.close()
    return media.currentTime
  })
  await expect.poll(() => desktop.rows()[0]?.positionMs).toBe(Math.round(closePosition * 1000))
  await expect.poll(() => page.evaluate(() => (window as unknown as {
    catalogTest: { closeReplies: unknown[] }
  }).catalogTest.closeReplies)).toEqual([{ id: 'close-1', unconfirmed: false }])
  await expect(video).toHaveJSProperty('paused', true)
  expect(JSON.stringify(desktop.rows())).not.toMatch(/catalog-media|127\.0\.0\.1|lease/)
  await page.goto('/#/')
  await expect(page).toHaveURL(/\/home$/)
  await expect(page.locator('video')).toHaveCount(0)
})

test('CMS before-hide runtime event checkpoints sub-five-second playback without pausing', async ({ page }) => {
  const desktop = await cmsHistoryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await playHistoryVideo(video)
  expect(desktop.rows()).toEqual([])

  const checkpoint = await video.evaluate((media: HTMLVideoElement) => {
    const { hide } = (window as unknown as { catalogTest: { hide(): void } }).catalogTest
    hide()
    return media.currentTime
  })
  expect(checkpoint).toBeGreaterThan(0)
  expect(checkpoint).toBeLessThan(5)
  await expect.poll(() => (desktop.rows()[0]?.positionMs ?? 0) / 1000).toBeCloseTo(checkpoint, 1)
  await expect(video).toHaveJSProperty('paused', false)
  await expect.poll(() => position(video)).toBeGreaterThan(checkpoint + 0.1)
  expect(await page.evaluate(() => (window as unknown as {
    catalogTest: { closeReplies: unknown[] }
  }).catalogTest.closeReplies)).toEqual([])
})

test('CMS expiry and a shorter replacement preserve a saved bookmark until the user watches', async ({ page }) => {
  const bookmark: HistoryRow = {
    dramaId: cmsId, episode: 1, positionMs: 5000, percent: 20, durationMs: 24000, episodeCountAtWatch: 3, watchedAt: Date.now(),
  }
  const desktop = await cmsHistoryFixture(page, [bookmark])
  desktop.catalog.expired = true
  await page.goto(cmsRoute + '/play/1')
  await expect(page.getByTestId('error')).toContainText('无法取得可用的播放源')
  expect(desktop.requests).toEqual([])
  expect(desktop.rows()).toEqual([bookmark])
  desktop.catalog.expired = false
  desktop.catalog.format = 'hls'
  await page.getByRole('button', { name: '重试' }).click()
  const video = historyVideo(page, 1)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBe(64)
  await expect.poll(() => position(video)).toBeCloseTo(5 / 24 * 2, 1)
  await expect(video).toHaveJSProperty('paused', true)
  await page.getByRole('link', { name: '返回详情' }).click()
  expect(desktop.rows()).toEqual([bookmark])
  expect(desktop.writes).toEqual([])
})

async function failWithMediaError(video: ReturnType<typeof historyVideo>, resetPosition = false) {
  return video.evaluate(async (media: HTMLVideoElement, reset) => {
    const probe = document.createElement('video')
    const error = await new Promise<MediaError | null>((resolve) => {
      probe.addEventListener('error', () => resolve(probe.error), { once: true })
      probe.src = 'data:video/webm;base64,AA=='
    })
    if (!error) throw new Error('Expected a real MediaError from the invalid media probe')
    Object.defineProperty(media, 'error', { value: error })
    if (reset) Object.defineProperty(media, 'currentTime', { value: 0 })
    const checkpoint = media.currentTime
    media.dispatchEvent(new Event('error'))
    return checkpoint
  }, resetPosition)
}

test('CMS media fallback captures the final played position and ignores events from old media', async ({ page }) => {
  const desktop = await cmsHistoryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  // The error handler must capture a real played position even without an earlier timeupdate sample.
  await first.evaluate((media: HTMLVideoElement) => {
    media.addEventListener('timeupdate', (event) => event.stopImmediatePropagation(), { capture: true })
  })
  await playHistoryVideo(first)
  const duration = await durationMs(first)
  expect(desktop.rows()).toEqual([])
  const checkpoint = await failWithMediaError(first)
  expect(checkpoint).toBeGreaterThan(0.1)
  await expect(first).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => position(first)).toBeCloseTo(checkpoint, 1)
  await expect.poll(() => (desktop.rows()[0]?.positionMs ?? 0) / 1000).toBeCloseTo(checkpoint, 1)
  await playHistoryVideo(first)
  await first.evaluate((media: HTMLVideoElement) => Object.assign(window, { priorCMSMedia: media }))

  await page.getByRole('button', { name: '第2集' }).click()
  const second = historyVideo(page, 2)
  await expect.poll(() => second.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await page.evaluate(() => {
    const prior = (window as unknown as { priorCMSMedia: HTMLVideoElement }).priorCMSMedia
    for (const type of ['playing', 'seeked', 'pause', 'timeupdate', 'ended', 'error']) prior.dispatchEvent(new Event(type))
  })
  expect(desktop.rows()[0]).toMatchObject({ episode: 1, durationMs: duration })
  await expect(second).toHaveJSProperty('currentTime', 0)
  await playHistoryVideo(second)
  await second.evaluate((media: HTMLVideoElement) => { media.currentTime = 9 })
  await expect.poll(() => position(second)).toBeGreaterThanOrEqual(9)
  await page.getByRole('link', { name: '返回详情' }).click()
  await expect.poll(() => desktop.rows()[0]?.episode).toBe(2)
  expect(desktop.rows()[0].positionMs).toBeGreaterThanOrEqual(9000)
})

test('CMS media fallback keeps the sampled bookmark when a failed decoder resets time', async ({ page }) => {
  const desktop = await cmsHistoryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await playHistoryVideo(video)
  await video.evaluate((media: HTMLVideoElement) => {
    media.pause()
    media.currentTime = 7
  })
  await expect.poll(() => desktop.rows()[0]?.positionMs).toBe(7000)
  expect(await failWithMediaError(video, true)).toBe(0)
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => position(video)).toBeCloseTo(7, 2)
  expect(desktop.rows()[0]?.positionMs).toBe(7000)
})

test('CMS completion survives a paused scrub and only offers an available next episode', async ({ page }) => {
  const desktop = await cmsHistoryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await playHistoryVideo(video)
  const duration = await durationMs(video)
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = media.duration - 0.1 })
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ percent: 100, positionMs: duration, episode: 1 })
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = 3 })
  await expect.poll(() => position(video)).toBeCloseTo(3, 2)
  await page.getByRole('link', { name: '返回详情' }).click()
  await expect(page.getByTestId('catalog-continue')).toContainText('看下一集')
  expect(desktop.rows()[0]?.percent).toBe(100)

  await page.getByTestId('nav-history').click()
  const row = page.getByTestId('history-item')
  await expect(row.getByTestId('resume')).toHaveText('看下一集')
  desktop.catalog.available = [1, 3]
  await row.getByTestId('resume').click()
  await expect(page.getByTestId('error')).toContainText('所选分集暂不可播放')
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
  await page.getByTestId('nav-history').click()
  await expect(row.getByTestId('resume')).toHaveText('重播本集')
  await expect(page.getByTestId('continue-latest')).toBeDisabled()
  await row.getByTestId('h-more').click()
  await row.getByRole('button', { name: '查看详情' }).click()
  await expect(page.getByTestId('catalog-continue')).toContainText('重播本集')
  await page.getByTestId('catalog-continue').click()
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await expect(video).toHaveJSProperty('currentTime', 0)
  await expect(video).toHaveJSProperty('paused', true)
  expect(desktop.rows()[0]?.percent).toBe(100)

  await page.getByRole('button', { name: '第3集' }).click()
  const last = historyVideo(page, 3)
  await playHistoryVideo(last)
  await last.evaluate((media: HTMLVideoElement) => { media.currentTime = media.duration - 0.1 })
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ percent: 100, episode: 3 })
  await page.getByTestId('nav-history').click()
  await expect(page.getByTestId('stat-done')).toHaveText('1')
  await expect(row.getByTestId('done-badge')).toBeVisible()
})

test('CMS history remains manageable offline and after a source or episode changes', async ({ page }) => {
  const obsolete = 'apple-cms:ffffffffffffffffffffffffffffffff:90071992547409931234'
  const row = (dramaId: string, episode: number): HistoryRow => ({
    dramaId, episode, positionMs: 5000, percent: 20, durationMs: 24000, episodeCountAtWatch: 3, watchedAt: Date.now(),
  })
  const desktop = await cmsHistoryFixture(page, [row(cmsId, 2), row(obsolete, 1)])
  desktop.catalog.provider = 'authorized-fixture'
  await page.goto('/#/history')
  const rows = page.getByTestId('history-item')
  const unavailable = rows.filter({ hasText: '当前片源不可用' })
  await expect(unavailable).toHaveCount(2)
  await expect(page.getByRole('button', { name: '核验更多片源记录' })).toHaveCount(0)
  expect(await page.evaluate(() => (window as unknown as { catalogTest: { lookups: string[] } }).catalogTest.lookups)).toEqual([])
  await expect(page.getByTestId('stat-total')).toHaveText('2')
  await expect(page.getByTestId('continue-latest')).toBeDisabled()

  desktop.catalog.provider = 'apple-cms'
  desktop.catalog.offline = true
  await page.reload()
  await expect(unavailable).toHaveCount(2)
  await expect(page.getByTestId('resume')).toHaveCount(0)

  desktop.catalog.offline = false
  desktop.catalog.available = [1, 3]
  await page.reload()
  const availableTitle = rows.filter({ hasText: cmsTitle })
  await expect(availableTitle).toContainText('该分集不可用')
  await expect(availableTitle.getByTestId('resume')).toHaveCount(0)
  await availableTitle.getByRole('button', { name: '选集' }).click()
  await expect(page.getByRole('heading', { name: cmsTitle })).toBeVisible()
  await expect(page.getByTestId('catalog-continue')).toHaveCount(0)
  expect(await calls(page)).toEqual([])

  desktop.catalog.id = obsolete
  await page.getByTestId('nav-history').click()
  await expect(rows.filter({ hasText: cmsTitle }).getByTestId('resume')).toHaveText('继续观看')
  await expect(unavailable).toHaveCount(1)
  await unavailable.getByTestId('h-more').click()
  await expect(unavailable.getByRole('button', { name: '查看详情' })).toHaveCount(0)
  await unavailable.getByTestId('h-delete').click()
  await expect.poll(() => desktop.rows().map(({ dramaId }) => dramaId)).toEqual([obsolete])
  await page.getByTestId('resume').click()
  await expect.poll(() => position(historyVideo(page, 1))).toBeCloseTo(5, 2)
  expect(await calls(page)).toEqual(['catalog:' + obsolete + ':1'])
})

test('CMS history checks a bounded batch before offering to check older records', async ({ page }) => {
  const now = Date.now()
  const records = Array.from({ length: 25 }, (_, index): HistoryRow => ({
    dramaId: 'apple-cms:0123456789abcdef0123456789abcdef:' + (index + 1),
    episode: 1, positionMs: 5000, percent: 20, durationMs: 24000, episodeCountAtWatch: 3,
    watchedAt: now - index,
  }))
  await cmsHistoryFixture(page, records)
  await page.goto('/#/history')
  const loadMore = page.getByRole('button', { name: '核验更多片源记录' })
  const lookups = () => page.evaluate(() => (window as unknown as { catalogTest: { lookups: string[] } }).catalogTest.lookups)
  await expect(page.getByTestId('history-item')).toHaveCount(25)
  await expect(loadMore).toBeEnabled()
  expect(await lookups()).toEqual(records.slice(0, 12).map(({ dramaId }) => dramaId))
  await loadMore.click()
  await expect(loadMore).toBeEnabled()
  expect(await lookups()).toEqual(records.slice(0, 24).map(({ dramaId }) => dramaId))
})
