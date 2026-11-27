import fs from 'node:fs'
import { createServer } from 'node:http'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { PlayMode } from '../../src/player/preferences'

const id = 'catalog:qa/001'
const route = '/#/catalog/' + encodeURIComponent(id)
const cmsId = 'apple-cms:0123456789abcdef0123456789abcdef:90071992547409931234'
const cmsTitle = 'CMS 海岛来信'
const cmsRoute = '/#/catalog/' + encodeURIComponent(cmsId)
type HistoryRow = {
  dramaId: string; episode: number; percent: number; watchedAt: number
  positionMs: number; durationMs: number; episodeCountAtWatch: number
}
type LibraryChanges = {
  favorites?: string[]
  addedFavorites?: string[]
  removedFavorites?: string[]
  favoriteOrder?: string[]
  history: HistoryRow[]
  removed: string[]
}
type SavedDanmaku = { id: number; offsetMs: number; text: string }
type CatalogState = {
  id: string; provider: string; available: number[]
  playMode?: PlayMode
  dpi?: number
  shortcuts?: Record<string, string>
  holdPlaybackEpisode?: number
  holdResume?: boolean
  offlineId?: string
  home?: 'empty' | 'error'; localBookmarks?: boolean; format: 'webm' | 'hls'; expired?: boolean; offline?: boolean; cover?: string; detailDelayMs?: number; holdDetail?: boolean
}
const mediaURL = 'http://127.0.0.1:45671'
const hlsType = 'application/vnd.apple.mpegurl'
const hlsSample = fs.readFileSync(new URL('../fixtures/hls/blue.mpegts', import.meta.url))
const samples = {
  webm: fs.readFileSync(new URL('../../public/demo/demo-1.webm', import.meta.url)),
  mp4: fs.readFileSync(new URL('../../public/demo/demo-1.mp4', import.meta.url)),
}

async function fixture(page: Page, mode: 'ready' | 'blocked' | 'stale' | 'retry' | 'expires-live' | 'protected' | 'direct-url' | 'apple-cms' | 'apple-cms-empty' | 'apple-cms-hls' | 'apple-cms-hls-fallback' | 'apple-cms-hls-expires' | 'apple-cms-library') {
  await page.addInitScript(({ id, mediaURL, mode, cmsTitle, hlsType }) => {
    const cms = mode.startsWith('apple-cms')
    const hls = mode.startsWith('apple-cms-hls')
    const libraryFixture = mode === 'apple-cms-library'
    const desktop = window as unknown as {
      readDesktopCatalog(): Promise<CatalogState & { history: HistoryRow[]; favorites: string[] }>
      saveDesktopCatalog(changes: LibraryChanges): Promise<{ favorites: string[]; history: HistoryRow[] }>
      waitForCMSDetails(): Promise<void>
      readCMSComments(id: string, episode: number): Promise<string>
      saveCMSComments(id: string, episode: number, snapshot: string): Promise<void>
      readCMSDanmaku?(id: string, episode: number): Promise<SavedDanmaku[]>
      saveCMSDanmaku?(id: string, episode: number, offsetMs: number, text: string, requestId: string): Promise<SavedDanmaku>
    }
    if (hls || libraryFixture) {
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
    let activeLookups = 0
    const closeReplies: { id: string; unconfirmed: boolean }[] = []
    const testState = {
      calls, released, revoked, lookups, closeReplies, peakLookups: 0, failedLookups: 0, providerReads: 0, resumePending: false, release: () => {},
      sharedLink: '',
      openShare: (path: string) => events.get('jindi-open-share-link')?.(path),
      close: () => events.get('jindi-before-window-close')?.('close-1'),
      hide: () => events.get('jindi-before-window-hide')?.(''),
      cancelClose: () => events.get('jindi-window-close-canceled')?.('close-1'),
    }
    if (libraryFixture) {
      Object.assign(window, { runtime: {
        EventsOn: (name: string, handler: (attempt: string) => void) => events.set(name, handler),
        ClipboardSetText: (text: string) => { testState.sharedLink = text; return Promise.resolve(true) },
        EventsEmit: (_: string, id: string, unconfirmed: boolean) => closeReplies.push({ id, unconfirmed }),
      } })
    }
    Object.assign(window, { catalogTest: testState, go: { app: { App: {
      StorageReady: async () => libraryFixture,
      Favorites: async () => libraryFixture ? (await desktop.readDesktopCatalog()).favorites : [],
      History: async () => (await desktop.readDesktopCatalog()).history,
      SearchHistory: async () => [],
      LocalComments: (id: string, episode: number) => desktop.readCMSComments(id, episode),
      SaveLocalComments: (id: string, episode: number, snapshot: string) => desktop.saveCMSComments(id, episode, snapshot),
      LocalDanmaku: (id: string, episode: number) => desktop.readCMSDanmaku?.(id, episode) ?? [],
      SendLocalDanmakuOnce: (id: string, episode: number, offset: number, text: string, requestId: string) =>
        desktop.saveCMSDanmaku?.(id, episode, offset, text, requestId),
      GetSetting: async (key: string) => key === 'app'
        ? JSON.stringify({
          startPage: '上次观看',
          ...(libraryFixture ? await desktop.readDesktopCatalog().then(({ playMode, dpi, shortcuts }) => ({ playMode, dpi, shortcuts })) : {}),
        })
        : '',
      ProviderName: async () => {
        if (!libraryFixture) return cms ? 'apple-cms' : 'authorized-fixture'
        const { provider } = await desktop.readDesktopCatalog()
        testState.providerReads++
        return provider
      },
      CatalogResumeIDs: async (ids: string[]) => {
        if (!libraryFixture) return []
        const catalog = await desktop.readDesktopCatalog()
        if (catalog.holdResume) {
          testState.resumePending = true
          await new Promise<void>((resolve) => { testState.release = resolve })
          testState.resumePending = false
        }
        if (catalog.provider !== 'apple-cms') return []
        const prefix = catalog.id.slice(0, catalog.id.lastIndexOf(':') + 1)
        return ids.filter((candidate) => candidate.startsWith(prefix)).slice(0, 12)
      },
      LicensedCatalog: async () => libraryFixture && (await desktop.readDesktopCatalog()).localBookmarks
        ? [1, 2, 3].map((index) => ({ id: 'local-' + index, title: '本地样片' + index,
          tags: [], episodeCount: 3, updatedTo: 3, unavailableEpisodes: [] }))
        : [],
      Home: async () => {
        const state = libraryFixture ? (await desktop.readDesktopCatalog()).home : undefined
        if (state === 'error') throw new Error('目录暂不可用')
        return state === 'empty' || mode === 'apple-cms-empty' ? [] : [drama]
      },
      HomeFeed: async () => ({ catalogLabel: cms ? '片源目录' : undefined, todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      CatalogSearch: async (query: string) => query === '海岛' ? [drama] : [],
      CatalogPage: async (query: string, page: number) => ({
        entries: mode !== 'apple-cms-empty' && (!query || query === '海岛') ? [drama] : [],
        page,
        hasMore: false,
      }),
      Capabilities: async () => ({ catalog: 'ready', search: 'ready', playback: mode === 'blocked' ? 'not_authorized' : 'ready' }),
      CatalogDrama: async (requestedId: string) => {
        if (!libraryFixture) return drama
        lookups.push(requestedId)
        activeLookups++
        testState.peakLookups = Math.max(testState.peakLookups, activeLookups)
        try {
          const catalog = await desktop.readDesktopCatalog()
          if (catalog.holdDetail) await desktop.waitForCMSDetails()
          if (catalog.detailDelayMs) await new Promise((resolve) => setTimeout(resolve, catalog.detailDelayMs))
          if (catalog.offline || catalog.offlineId === requestedId) {
            testState.failedLookups++
            throw new Error('unavailable')
          }
          if (catalog.id !== requestedId) throw new Error('provider: 未找到')
          return { ...drama, id: requestedId, cover: catalog.cover, updatedTo: Math.max(0, ...catalog.available) }
        } finally {
          activeLookups--
        }
      },
      CatalogEpisodes: async (requestedId: string) => {
        if (!libraryFixture) return [1, 2, 3].map((index) => ({ id: 'episode-' + index, dramaId: id, index, available: cms && !hls ? index === 3 : index < 3 }))
        const catalog = await desktop.readDesktopCatalog()
        if (catalog.id !== requestedId) throw new Error('provider: 未找到')
        return [1, 2, 3].map((index) => ({
          id: 'episode-' + index, dramaId: requestedId, index, available: catalog.available.includes(index),
        }))
      },
      CatalogPlayback: async (_id: string, index: number) => {
        calls.push('catalog:' + _id + ':' + index)
        const sequence = ++attempt
        const leaseId = String.fromCharCode(64 + index) + String.fromCharCode(64 + sequence).repeat(25)
        if (mode === 'stale' && index === 1) await new Promise<void>((resolve) => { testState.release = resolve })
        const catalog = libraryFixture ? await desktop.readDesktopCatalog() : undefined
        if (catalog?.holdPlaybackEpisode === index) await new Promise<void>((resolve) => { testState.release = resolve })
        const expiresAt = (mode === 'retry' && sequence === 1) || catalog?.expired ? '2001-01-01T00:00:00Z'
          : new Date(Date.now() + (mode === 'apple-cms-hls-expires' ? 4_000 : mode === 'expires-live' ? 3_000 : 120_000)).toISOString()
        const format = !libraryFixture && index === 2 ? 'mp4' : 'webm'
        const path = mediaURL + '/catalog-media/' + leaseId
        const src = mode === 'direct-url' ? 'https://media.example.test/one.webm?upstream-secret=opaque-test' : path + '/0'
        const sources = [{ src, type: 'video/' + format }]
        if (hls && index === 1) {
          sources[0] = { src: path + '/0.m3u8', type: hlsType }
          if (mode !== 'apple-cms-hls') sources.push({ src: path + '/1', type: 'video/webm' })
        }
        if (catalog?.format === 'hls') sources[0] = { src: path + '/0.m3u8', type: hlsType }
        if (mode === 'expires-live' || libraryFixture && catalog?.format === 'webm') sources.push({ src: path + '/1', type: 'video/webm' })
        return { kind: 'authorized', episodeId: 'episode-' + index, expiresAt, leaseId, sources,
          restrictions: mode === 'protected' ? ['no-download'] : [] }
      },
      ReleaseCatalogPlayback: async (leaseId: string) => { released.push(leaseId) },
      ResolvePlayback: async () => { calls.push('legacy') },
      SaveLibraryChanges: async (changes: LibraryChanges) => {
        if (libraryFixture) return desktop.saveDesktopCatalog(changes)
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
    if ((mode.startsWith('apple-cms-hls') || mode === 'apple-cms-library') && (path.endsWith('.m3u8') || path.includes('/hls/'))) {
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
    const format = mode !== 'apple-cms-library' && path.startsWith('/catalog-media/B') ? 'mp4' : 'webm'
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
  await expect(page.getByRole('heading', { name: '远海来信' })).toBeVisible()
  await expect(page.getByTestId('fav')).toHaveCount(0)
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
  await video.evaluate(async (media: HTMLVideoElement) => {
    media.currentTime = media.duration - 0.25
    await media.play()
  })
  await expect(video).toHaveJSProperty('ended', true)
  await expect(page).toHaveURL(/\/play\/1$/)
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

test('CMS detail and playback use the combined bridge and retry without falling back to mixed data', async ({ page }) => {
  await fixture(page, 'apple-cms')
  await page.goto('/#/home')
  await expect(page.getByRole('region', { name: '片源目录' }).getByTestId('drama-card')).toContainText(cmsTitle)
  await page.evaluate(({ id, title }) => {
    const app = (window as unknown as { go: { app: { App: Record<string, unknown> } } }).go.app.App
    const detail = { calls: [] as string[], legacyCalls: [] as string[] }
    Object.assign(window, { combinedCMSDetail: detail })
    app.CatalogDrama = async () => { detail.legacyCalls.push('title'); throw new Error('legacy title') }
    app.CatalogEpisodes = async () => { detail.legacyCalls.push('episodes'); throw new Error('legacy episodes') }
    app.CatalogDetail = async (requested: string) => {
      detail.calls.push(requested)
      if (detail.calls.length === 1) throw new Error('片源暂不可用')
      return {
        drama: { id, title, tags: [], summary: '一致的剧目快照', episodeCount: 3, updatedTo: 3 },
        episodes: [1, 2, 3].map((index) => ({ id: 'episode-' + index, dramaId: id, index, available: index === 3 })),
      }
    }
  }, { id: cmsId, title: cmsTitle })
  await page.getByRole('region', { name: '片源目录' }).getByTestId('drama-card').click()
  await expect(page.getByTestId('error')).toBeVisible()
  await page.getByRole('button', { name: '重试' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(cmsTitle)
  await expect(page.getByRole('button', { name: '第2集' })).toBeDisabled()
  await page.getByRole('button', { name: '第3集' }).click()
  await expect.poll(() => durationMs(historyVideo(page, 3))).toBeGreaterThan(10_000)
  const detail = await page.evaluate(() => (window as unknown as {
    combinedCMSDetail: { calls: string[]; legacyCalls: string[] }
  }).combinedCMSDetail)
  expect(detail).toEqual({ calls: [cmsId, cmsId, cmsId], legacyCalls: [] })
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':3'])
})

test('CMS playback shows its rate limit and can retry after cooldown', async ({ page }) => {
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute)
  await page.evaluate(() => {
    type Desktop = { CatalogPlayback(id: string, episode: number): Promise<unknown> }
    const app = (window as unknown as { go: { app: { App: Desktop } } }).go.app.App
    const playback = app.CatalogPlayback.bind(app)
    let limited = true
    app.CatalogPlayback = async (id, episode) => {
      if (limited) {
        limited = false
        throw '苹果 CMS 请求过于频繁，请稍后重试'
      }
      return playback(id, episode)
    }
  })
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  await expect(page.getByTestId('error')).toContainText('苹果 CMS 请求过于频繁，请稍后重试')
  await expect(page.locator('video')).toHaveCount(0)
  await page.getByRole('button', { name: '重试' }).click()
  await expect.poll(() => durationMs(historyVideo(page, 1))).toBeGreaterThan(10_000)
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
  await video.evaluate((v: HTMLVideoElement) => {
    v.playbackRate = 0.25
    return v.play()
  })
  await expect(page.getByTestId('error')).toContainText('播放票据已过期')
  await expect(video).toHaveCount(0)
  await expect.poll(() => revoked(page)).toContain(objectURL)
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
  const prefix = mediaURL + '/catalog-media/' + 'A'.repeat(26) + '/'
  expect([...new Set(requests)]).toEqual([prefix + '0.m3u8', prefix + 'hls/0.m3u8', prefix + 'hls/1'])
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
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

async function cmsLibraryFixture(page: Page, initial: HistoryRow[] = [], initialFavorites: string[] = []) {
  const catalog: CatalogState = { id: cmsId, provider: 'apple-cms', available: [1, 2, 3], format: 'webm' }
  let history = initial
  let favorites = initialFavorites
  const writes: HistoryRow[][] = []
  await page.exposeFunction('readDesktopCatalog', () => ({ ...catalog, history, favorites }))
  await page.exposeFunction('saveDesktopCatalog', (changes: LibraryChanges) => {
    const ids = new Set([...changes.removed, ...changes.history.map(({ dramaId }) => dramaId)])
    history = [...changes.history, ...history.filter(({ dramaId }) => !ids.has(dramaId))]
    writes.push(changes.history)
    const removed = new Set(changes.removedFavorites)
    favorites = changes.favorites ?? [
      ...(changes.addedFavorites ?? []), ...favorites.filter((id) => !removed.has(id)),
    ]
    if (changes.favoriteOrder) {
      const positions = new Map(changes.favoriteOrder.map((id, index) => [id, index]))
      favorites.sort((a, b) => (positions.get(a) ?? Infinity) - (positions.get(b) ?? Infinity))
    }
    return { favorites, history }
  })
  const requests = await fixture(page, 'apple-cms-library')
  return { catalog, writes, requests, rows: () => history, favorites: () => favorites }
}

const historyVideo = (page: Page, episode: number) => page.getByLabel('第' + episode + '集视频')

test('CMS local comments start empty and persist independently for each episode', async ({ page }, testInfo) => {
  const snapshots = new Map<string, string>()
  await page.exposeFunction('readCMSComments', (id: string, episode: number) => snapshots.get(id + ':' + episode) ?? '')
  await page.exposeFunction('saveCMSComments', (id: string, episode: number, snapshot: string) => {
    snapshots.set(id + ':' + episode, snapshot)
  })
  const desktop = await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const open = page.getByRole('button', { name: '本机评论', exact: true })
  const panel = page.getByRole('complementary', { name: '本机评论' })
  await open.click()
  await expect(panel).toBeFocused()
  await expect(panel.getByTestId('comment-total')).toHaveText('(0)')
  await expect(panel.getByText('暂无评论')).toBeVisible()
  await expect(panel.getByTestId('csort-author')).toHaveCount(0)
  desktop.catalog.offline = true
  await panel.getByTestId('comment-input').fill('第 1 集的本机笔记')
  await panel.getByTestId('comment-post').click()
  await expect(panel.getByTestId('comment-total')).toHaveText('(1)')
  await expect.poll(() => snapshots.get(cmsId + ':1')).toContain('第 1 集的本机笔记')
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
  await panel.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('cms-local-comments.png') })
  await panel.press('Escape')
  await expect(open).toBeFocused()

  desktop.catalog.offline = false
  await page.getByRole('button', { name: '第2集' }).click()
  await open.click()
  await expect(panel.getByText('暂无评论')).toBeVisible()
  await expect(panel.getByTestId('comment-total')).toHaveText('(0)')
  await page.getByRole('button', { name: '第1集' }).click()
  await page.reload()
  await open.click()
  await expect(panel.getByTestId('comment')).toContainText('第 1 集的本机笔记')
  await expect(panel.getByTestId('comment-total')).toHaveText('(1)')
  expect(snapshots.has(cmsId + ':2')).toBe(false)
  await panel.getByTestId('more').click()
  await panel.getByTestId('delete').click()
  await expect(panel.getByTestId('comment-input')).toBeFocused()
  await expect(panel.getByText('暂无评论')).toBeVisible()
  await expect.poll(() => snapshots.get(cmsId + ':1')).toBe('[]')

  snapshots.set(cmsId + ':2', JSON.stringify([{ id: 'previous', text: '过往只读评论', mine: false }]))
  await page.getByRole('button', { name: '第2集' }).click()
  await open.click()
  const readonly = panel.getByTestId('comment').filter({ hasText: '过往只读评论' })
  await expect(readonly).toBeVisible()
  await expect(readonly.getByTestId('more')).toHaveCount(0)
})

const durationMs = (video: ReturnType<typeof historyVideo>) => video.evaluate((media: HTMLVideoElement) =>
  Math.round(media.duration * 1000),
)
const position = (video: ReturnType<typeof historyVideo>) => video.evaluate((media: HTMLVideoElement) =>
  media.seeking ? -1 : media.currentTime,
)

const startupCMSHistory: HistoryRow = {
  dramaId: cmsId, episode: 2, positionMs: 7345, durationMs: 24000, percent: 30,
  episodeCountAtWatch: 3, watchedAt: 100,
}

test('CMS shares a detail-only link and opens it for the configured source at 220% zoom', async ({ page }, testInfo) => {
  type ShareFixture = { catalogTest: { sharedLink: string; openShare(path: string): void } }
  await page.setViewportSize({ width: 1280, height: 860 })
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.dpi = 220
  await page.goto(cmsRoute)
  const share = page.getByTestId('share')
  await share.scrollIntoViewIfNeeded()
  await expect(share).toBeInViewport({ ratio: 0.95 })
  await share.click()
  await expect.poll(() => page.evaluate(() => (window as unknown as ShareFixture).catalogTest.sharedLink))
    .toBe('jindi://catalog/' + cmsId)
  await expect(page.getByText('分享链接已复制。对方需启用同一片源。')).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('cms-share-220.png') })

  const home = page.getByTestId('nav-home')
  await home.click()
  await expect(page.getByTestId('page-home')).toBeVisible()
  const openShare = (path: string) => page.evaluate((value) =>
    (window as unknown as ShareFixture).catalogTest.openShare(value), path)
  await openShare('/catalog/' + cmsId + '?token=fixture')
  await expect(page).toHaveURL(/\/home$/)
  await openShare('/catalog/' + cmsId)
  await expect(page.getByRole('heading', { name: cmsTitle })).toBeVisible()

  desktop.catalog.id = cmsId.replace('0123456789abcdef0123456789abcdef', 'f'.repeat(32))
  await openShare('/catalog/' + cmsId)
  await expect(page.getByText('没有找到这部短剧')).toBeVisible()
  expect(await calls(page)).toEqual([])
})

test('CMS detail offers playable same-source titles and ignores recommendations after navigation', async ({ page }, testInfo) => {
  await cmsLibraryFixture(page)
  await page.goto('/#/home')
  const relatedID = cmsId.replace(/:[0-9]+$/, ':42')
  await page.evaluate(({ id, relatedID, title }) => {
    const app = (window as unknown as { go: { app: { App: Record<string, unknown> } } }).go.app.App
    const card = (id: string, title: string, updatedTo = 3) => ({
      id, title, tags: ['海岛'], summary: '授权目录样本', episodeCount: 3, updatedTo,
    })
    const rows = [
      card(id, title),
      card(relatedID, '另一封海岛来信'),
      card(relatedID, '重复条目'),
      card(id.replace(/:[0-9]+$/, ':43'), '不可播作品', 0),
      card('apple-cms:' + 'f'.repeat(32) + ':44', '其他片源'),
      card(id.replace(/:[0-9]+$/, ':45'), '海边小镇', 2),
    ]
    const related = { requests: [] as Array<[string, number]>, finish: () => {} }
    Object.assign(window, { relatedCMS: related })
    app.CatalogDetail = async (requested: string) => ({
      drama: card(requested, requested === id ? title : '另一封海岛来信'),
      episodes: [{ id: 'episode-1', dramaId: requested, index: 1, available: true }],
      ...(requested === id ? { genreId: '7' } : {}),
    })
    app.CatalogGenrePage = async (genre: string, page: number) => {
      related.requests.push([genre, page])
      if (related.requests.length === 2) {
        await new Promise<void>((resolve) => { related.finish = resolve })
      }
      if (related.requests.length === 3) throw new Error('分类暂不可用')
      return { page, entries: rows, hasMore: false }
    }
  }, { id: cmsId, relatedID, title: cmsTitle })

  await page.getByRole('region', { name: '片源目录' }).getByTestId('drama-card').click()
  const section = page.getByRole('region', { name: '同类短剧' })
  const cards = section.getByTestId('guess-card')
  await expect(cards).toHaveCount(2)
  await expect(cards.first()).toContainText('另一封海岛来信')
  await expect(cards.last()).toContainText('已更新至第2集')
  await expect(page.getByRole('button', { name: '第1集' })).toBeEnabled()
  await page.screenshot({ path: testInfo.outputPath('cms-related-default.png'), fullPage: true })

  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((html) => { html.style.setProperty('--user-zoom', '2.2') })
  await expect(page.getByTestId('page-detail')).toHaveClass(/\bcompact\b/)
  await cards.first().scrollIntoViewIfNeeded()
  await expect(cards.first()).toBeInViewport({ ratio: 0.95 })
  await expect(cards.last()).toBeInViewport({ ratio: 0.95 })
  const width = await section.evaluate((element) => ({ inner: element.clientWidth, content: element.scrollWidth }))
  expect(width.content).toBeLessThanOrEqual(width.inner + 1)
  await page.screenshot({ path: testInfo.outputPath('cms-related-220.png') })

  await cards.first().click()
  await expect(page.getByRole('heading', { name: '另一封海岛来信' })).toBeVisible()
  await expect(section).toHaveCount(0)
  await page.goBack()
  await expect(page.getByRole('heading', { name: cmsTitle })).toBeVisible()
  const requests = () => page.evaluate(() => (window as unknown as { relatedCMS: { requests: Array<[string, number]> } }).relatedCMS.requests)
  await expect.poll(requests).toEqual([['7', 1], ['7', 1]])
  await page.evaluate((id) => { window.location.hash = '/catalog/' + encodeURIComponent(id) }, relatedID)
  await expect(page.getByRole('heading', { name: '另一封海岛来信' })).toBeVisible()
  await page.evaluate(async () => {
    (window as unknown as { relatedCMS: { finish(): void } }).relatedCMS.finish()
    await new Promise(requestAnimationFrame)
  })
  await expect(section).toHaveCount(0)
  expect(await requests()).toEqual([['7', 1], ['7', 1]])

  await page.goBack()
  await expect(page.getByRole('heading', { name: cmsTitle })).toBeVisible()
  await expect.poll(requests).toEqual([['7', 1], ['7', 1], ['7', 1]])
  await expect(page.getByRole('button', { name: '第1集' })).toBeEnabled()
  await expect(section).toHaveCount(0)
})

test('CMS detail shows supplied credits and hides missing data on the next title', async ({ page }, testInfo) => {
  await cmsLibraryFixture(page)
  await page.goto('/#/home')
  const nextID = cmsId.replace(/:[0-9]+$/, ':51')
  await page.evaluate(({ id, title }) => {
    const app = (window as unknown as { go: { app: { App: Record<string, unknown> } } }).go.app.App
    app.CatalogDetail = async (requested: string) => ({
      drama: { id: requested, title: requested === id ? title : '无演职信息的样片', tags: ['海岛'], summary: '授权目录样本', episodeCount: 1, updatedTo: 1 },
      episodes: [{ id: 'episode-1', dramaId: requested, index: 1, available: true }],
      ...(requested === id ? { cast: ['林舟', '雨岚', '长'.repeat(38)], directors: ['顾远'] } : {}),
    })
  }, { id: cmsId, title: cmsTitle })

  await page.getByRole('region', { name: '片源目录' }).getByTestId('drama-card').click()
  const cast = page.getByText('林舟、雨岚、' + '长'.repeat(38), { exact: true })
  await expect(cast).toBeVisible()
  await expect(page.getByText('顾远', { exact: true })).toBeVisible()
  await expect(page.getByText('演示主演')).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('cms-credits-default.png') })

  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((html) => { html.style.setProperty('--user-zoom', '2.2') })
  await expect(page.getByTestId('page-detail')).toHaveClass(/\bcompact\b/)
  await cast.scrollIntoViewIfNeeded()
  await expect(cast).toBeInViewport({ ratio: 0.95 })
  expect(await cast.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1)
  await page.screenshot({ path: testInfo.outputPath('cms-credits-220.png') })
  await page.evaluate((id) => { window.location.hash = '/catalog/' + encodeURIComponent(id) }, nextID)
  await expect(page.getByRole('heading', { name: '无演职信息的样片' })).toBeVisible()
  await expect(page.getByText('主演', { exact: true })).toHaveCount(0)
  await expect(page.getByText('导演', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '第1集' })).toBeEnabled()
})

test('CMS desktop startup restores the latest current-source bookmark paused', async ({ page }) => {
  const otherSource = 'apple-cms:ffffffffffffffffffffffffffffffff:90071992547409931234'
  await cmsLibraryFixture(page, [{ ...startupCMSHistory, dramaId: otherSource, watchedAt: 200 }, startupCMSHistory])

  await page.goto('/')
  await expect(page).toHaveURL(/\/play\/2$/)
  const video = historyVideo(page, 2)
  await expect.poll(() => position(video)).toBeCloseTo(7.345, 2)
  await expect(video).toHaveJSProperty('paused', true)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':2'])
  await page.getByTestId('nav-home').click()
  await expect(page.getByTestId('page-home')).toBeVisible()
})

test('CMS desktop startup returns home when the bookmarked episode is unavailable', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page, [startupCMSHistory])
  desktop.catalog.available = [1, 3]

  await page.goto('/')
  await expect(page.getByTestId('page-home')).toBeVisible()
  expect(await calls(page)).toEqual([])
  expect(desktop.rows()).toHaveLength(1)

  await page.goto(cmsRoute + '/play/2')
  await expect(page.getByText('所选分集暂不可播放。')).toBeVisible()
})

test('CMS desktop startup returns home if the initial media cannot load', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page, [startupCMSHistory])
  await page.route('**/catalog-media/**', (route) => route.fulfill({ status: 404 }))
  await page.goto('/')
  await expect.poll(() => calls(page)).toEqual(['catalog:' + cmsId + ':2'])
  await expect(page.getByTestId('page-home')).toBeVisible()
  expect(desktop.rows()).toEqual([startupCMSHistory])
})

test('a delayed CMS desktop startup cannot replace manual navigation', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page, [startupCMSHistory])
  desktop.catalog.holdResume = true
  await page.goto('/')
  await expect.poll(() => page.evaluate(() =>
    (window as unknown as { catalogTest: { resumePending: boolean } }).catalogTest.resumePending,
  )).toBe(true)
  await page.getByTestId('nav-history').click()
  await page.evaluate(() => (window as unknown as { catalogTest: { release(): void } }).catalogTest.release())
  await expect(page).toHaveURL(/\/history$/)
  const redirected = await page.waitForURL(/\/catalog\//, { timeout: 500 }).then(
    () => true,
    (error: Error) => { if (error.name !== 'TimeoutError') throw error; return false },
  )
  expect(redirected).toBe(false)
  await expect(page.getByTestId('page-history')).toBeVisible()
  expect(await calls(page)).toEqual([])
})

test('CMS shortcut bindings operate the native video and leave local editors alone', async ({ page }, testInfo) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.shortcuts = { playPause: 'K', fullscreen: 'G' }
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await expect.poll(() => durationMs(video)).toBeGreaterThan(1000)

  await page.keyboard.press('k')
  await expect(video).toHaveJSProperty('paused', false)
  await page.keyboard.press('k')
  await expect(video).toHaveJSProperty('paused', true)
  const before = await position(video)
  await page.keyboard.press('ArrowRight')
  await expect.poll(() => position(video)).toBeCloseTo(before + 10, 1)
  await page.keyboard.press('ArrowLeft')
  await expect.poll(() => position(video)).toBeCloseTo(before, 1)

  await page.keyboard.press('g')
  await expect.poll(() => page.evaluate(() => document.fullscreenElement?.classList.contains('stage'))).toBe(true)
  await expect(video).toHaveCSS('object-fit', 'contain')
  await expect.poll(() => video.evaluate((media) => {
    const bounds = media.getBoundingClientRect()
    return bounds.width <= window.innerWidth + 1 && bounds.height <= window.innerHeight + 1
  })).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('cms-shortcut-fullscreen.png') })
  await page.keyboard.press('g')
  await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBe(null)

  await page.getByRole('button', { name: '本机弹幕', exact: true }).click()
  const panel = page.getByTestId('danmaku-settings')
  const input = panel.getByTestId('dm-input')
  await input.focus()
  await page.keyboard.press('k')
  await page.keyboard.press('d')
  await page.keyboard.press('ArrowRight')
  await expect(input).toHaveValue('kd')
  await expect(panel.getByTestId('dm-enabled')).toHaveAttribute('aria-checked', 'true')
  await expect(video).toHaveJSProperty('paused', true)
  await expect.poll(() => position(video)).toBeCloseTo(before, 1)
  await input.press('Escape')
  await expect(panel).toHaveCount(0)
  await page.keyboard.press('d')
  await page.getByRole('button', { name: '本机弹幕', exact: true }).click()
  await expect(panel.getByTestId('dm-enabled')).toHaveAttribute('aria-checked', 'false')
  await page.keyboard.press('Escape')

  await page.keyboard.press('Control+ArrowRight')
  await expect(page).toHaveURL(/\/play\/2$/)
  await expect(historyVideo(page, 2)).toHaveJSProperty('paused', true)
  await page.keyboard.press('Control+ArrowLeft')
  await expect(page).toHaveURL(/\/play\/1$/)
})

test('CMS fullscreen and local controls remain usable at 220% zoom', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 860 })
  let commentSnapshot = ''
  await page.exposeFunction('readCMSComments', () => commentSnapshot)
  await page.exposeFunction('saveCMSComments', (_id: string, _episode: number, snapshot: string) => {
    commentSnapshot = snapshot
  })
  await page.exposeFunction('saveCMSDanmaku', (_id: string, _episode: number, offsetMs: number, text: string) =>
    ({ id: 1, offsetMs, text }),
  )
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.dpi = 220
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await expect.poll(() => durationMs(video)).toBeGreaterThan(1000)
  await expect(page.locator('html')).toHaveCSS('--user-zoom', '2.2')
  await expect.poll(() => video.evaluate((media) =>
    Math.round(media.getBoundingClientRect().height),
  )).toBeLessThanOrEqual(Math.ceil(860 * 0.65))
  await video.scrollIntoViewIfNeeded()
  await expect(video).toBeInViewport({ ratio: 0.99 })
  await page.keyboard.press('Space')
  await expect.poll(() => position(video)).toBeGreaterThan(0.2)
  await page.keyboard.press('Space')
  await expect(video).toHaveJSProperty('paused', true)
  await page.screenshot({ path: testInfo.outputPath('cms-window-220.png') })

  await page.keyboard.press('f')
  await expect.poll(() => video.evaluate((media) => {
    const stage = document.fullscreenElement
    if (!stage?.contains(media)) return null
    const box = media.getBoundingClientRect()
    return [box.x, box.y, box.width, box.height].map(Math.round)
  })).toEqual([0, 0, 1280, 860])
  await expect(video).toHaveCSS('object-fit', 'contain')
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth / media.videoHeight))
    .toBeCloseTo(9 / 16, 4)
  await page.screenshot({ path: testInfo.outputPath('cms-fullscreen-220.png') })
  await page.keyboard.press('f')
  await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBe(null)

  await page.getByRole('button', { name: '第2集', exact: true }).click()
  await expect(page).toHaveURL(/\/play\/2$/)
  await page.getByRole('button', { name: '本机评论', exact: true }).click()
  const comments = page.getByTestId('comment-panel')
  const commentInput = comments.getByTestId('comment-input')
  await commentInput.fill('高缩放评论')
  const publish = comments.getByTestId('comment-post')
  await expect(publish).toBeInViewport()
  await page.screenshot({ path: testInfo.outputPath('cms-comments-220.png') })
  await publish.click()
  await expect(comments.getByTestId('comment')).toContainText('高缩放评论')
  await expect.poll(() => commentSnapshot).toContain('高缩放评论')
  await comments.getByTestId('comment-close').click()
  await expect(comments).toHaveCount(0)
  await page.getByRole('button', { name: '本机弹幕', exact: true }).click()
  const panel = page.getByTestId('danmaku-settings')
  const danmaku = panel.getByTestId('dm-input')
  await danmaku.fill('高缩放弹幕')
  const send = panel.getByTestId('dm-send')
  await send.scrollIntoViewIfNeeded()
  await expect(send).toBeInViewport()
  await expect(danmaku).toBeInViewport()
  await page.screenshot({ path: testInfo.outputPath('cms-danmaku-220.png') })
  await send.click()
  await panel.getByText('本集本机弹幕').click()
  await expect(panel.getByRole('list', { name: '本机弹幕记录' })).toContainText('高缩放弹幕')
})

test('CMS overlays local danmaku at the native media position and keeps episodes isolated', async ({ page }, testInfo) => {
  const saved = new Map<string, SavedDanmaku[]>()
  await page.exposeFunction('readCMSDanmaku', (id: string, episode: number) => saved.get(id + ':' + episode) ?? [])
  await page.exposeFunction('saveCMSDanmaku', (id: string, episode: number, offsetMs: number, text: string) => {
    const item = { id: 1, offsetMs, text }
    saved.set(id + ':' + episode, [item])
    return item
  })
  const desktop = await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await expect.poll(() => durationMs(video)).toBeGreaterThan(1000)
  const seek = async (second: number) => {
    await video.evaluate((media: HTMLVideoElement, time) => { media.currentTime = time }, second)
    await expect.poll(() => position(video)).toBeCloseTo(second, 2)
  }
  await seek(3)
  const open = page.getByRole('button', { name: '本机弹幕', exact: true })
  await open.click()
  const panel = page.getByTestId('danmaku-settings')
  await expect(panel.getByTestId('dm-hot')).toHaveCount(0)
  desktop.catalog.offline = true
  await panel.getByTestId('dm-input').fill('苹果 CMS 的本机弹幕')
  await panel.getByTestId('dm-send').click()
  await expect.poll(() => saved.get(cmsId + ':1')).toEqual([{ id: 1, offsetMs: 3000, text: '苹果 CMS 的本机弹幕' }])
  await seek(4)
  await expect(page.getByTestId('danmaku-item')).toHaveText('苹果 CMS 的本机弹幕')
  await expect(page.getByTestId('danmaku-item')).toBeVisible()
  await video.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('cms-local-danmaku.png') })
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
  await panel.getByText('本集本机弹幕').click()
  await expect(panel.getByRole('list', { name: '本机弹幕记录' })).toContainText('苹果 CMS 的本机弹幕')

  desktop.catalog.offline = false
  await page.getByRole('button', { name: '第2集' }).click()
  await open.click()
  await expect(panel.getByTestId('dm-local-log')).toHaveCount(0)
  await expect(page.getByTestId('danmaku-item')).toHaveCount(0)
  await page.getByRole('button', { name: '第1集' }).click()
  await page.reload()
  await expect.poll(() => durationMs(video)).toBeGreaterThan(1000)
  await seek(4)
  await expect(page.getByTestId('danmaku-item')).toHaveText('苹果 CMS 的本机弹幕')
})

async function playHistoryVideo(video: ReturnType<typeof historyVideo>) {
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  const start = await position(video)
  await video.evaluate((media: HTMLVideoElement) => media.play())
  await expect.poll(() => position(video)).toBeGreaterThan(start + 0.1)
}

test('CMS bookmark persists through detail, history, refresh and the normal window-close handshake', async ({ page }, testInfo) => {
  const desktop = await cmsLibraryFixture(page)
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
  await page.goto('about:blank')
  await page.goto('/')
  await expect(page).toHaveURL(/\/play\/2$/)
  await expect.poll(() => position(video)).toBeCloseTo(closePosition, 2)
  await expect(video).toHaveJSProperty('paused', true)
})

test('CMS before-hide runtime event checkpoints sub-five-second playback without pausing', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
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
  const desktop = await cmsLibraryFixture(page, [bookmark])
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

async function failWithMediaError(video: ReturnType<typeof historyVideo>, options?: { resetPosition?: boolean; pauseFirst?: boolean; volume?: number; speed?: number }) {
  return video.evaluate(async (media: HTMLVideoElement, options) => {
    const probe = document.createElement('video')
    const error = await new Promise<MediaError | null>((resolve) => {
      probe.addEventListener('error', () => resolve(probe.error), { once: true })
      probe.src = 'data:video/webm;base64,AA=='
    })
    if (!error) throw new Error('Expected a real MediaError from the invalid media probe')
    Object.defineProperty(media, 'error', { value: error })
    if (options?.resetPosition) Object.defineProperty(media, 'currentTime', { value: 0 })
    const checkpoint = media.currentTime
    if (options?.pauseFirst) {
      media.pause()
      media.dispatchEvent(new Event('pause'))
    }
    if (options?.volume !== undefined) media.volume = options.volume
    if (options?.speed !== undefined) media.playbackRate = options.speed
    media.dispatchEvent(new Event('error'))
    return checkpoint
  }, options)
}

async function blockNextCMSAutoplay(page: Page) {
  await page.evaluate(() => {
    const nativePlay = HTMLMediaElement.prototype.play
    let rejected = false
    HTMLMediaElement.prototype.play = function () {
      if (!rejected && this.src.endsWith('/1')) {
        rejected = true
        return Promise.reject(new DOMException('Playback requires a user gesture', 'NotAllowedError'))
      }
      return nativePlay.call(this)
    }
  })
}

test('CMS media fallback captures the final played position and ignores events from old media', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  // The error handler must capture a real played position even without an earlier timeupdate sample.
  await first.evaluate((media: HTMLVideoElement) => {
    media.addEventListener('timeupdate', (event) => event.stopImmediatePropagation(), { capture: true })
  })
  await playHistoryVideo(first)
  const duration = await durationMs(first)
  expect(desktop.rows()).toEqual([])
  // Keep the restored frame still while checking the exact position. The
  // playing scenario below separately checks automatic resumption.
  await blockNextCMSAutoplay(page)
  const checkpoint = await failWithMediaError(first)
  expect(checkpoint).toBeGreaterThan(0.1)
  await expect(first).toHaveAttribute('src', /\/1$/)
  await expect(first).toHaveJSProperty('paused', true)
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

test('CMS retries truncated network streams without losing the saved position', async ({ page }) => {
  const bookmark = { dramaId: cmsId, episode: 1, percent: 20, watchedAt: 1000,
    positionMs: 5000, durationMs: 24000, episodeCountAtWatch: 3 }
  const desktop = await cmsLibraryFixture(page, [bookmark])
  let recovered = false
  const requested: string[] = []
  const server = createServer((request, response) => {
    const path = request.url ?? ''
    requested.push(path)
    const sample = samples.webm
    const range = /^bytes=(\d+)-(\d*)$/.exec(request.headers.range ?? '')
    const start = Number(range?.[1] ?? 0)
    const end = range?.[2] ? Math.min(Number(range[2]), sample.length - 1) : sample.length - 1
    const body = sample.subarray(start, end + 1)
    response.setHeader('Content-Type', 'video/webm')
    response.setHeader('Content-Length', body.length)
    response.setHeader('Accept-Ranges', 'bytes')
    if (range) {
      response.statusCode = 206
      response.setHeader('Content-Range', `bytes ${start}-${end}/${sample.length}`)
    }
    if (request.method === 'HEAD') {
      response.end()
      return
    }
    if (recovered && path.endsWith('/1')) {
      response.end(body)
      return
    }
    response.write(body.subarray(0, Math.min(512, Math.floor(body.length / 2))), () => response.destroy())
  })
  try {
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject)
      server.listen(0, '127.0.0.1', resolve)
    })
    const address = server.address()
    if (!address || typeof address === 'string') throw new Error('Expected a TCP media fixture')
    await page.goto(cmsRoute)
    await page.evaluate((port) => {
      type Ticket = { sources: { src: string }[] }
      type Desktop = { CatalogPlayback(id: string, episode: number): Promise<Ticket> }
      const app = (window as unknown as { go: { app: { App: Desktop } } }).go.app.App
      const playback = app.CatalogPlayback.bind(app)
      app.CatalogPlayback = async (id, episode) => {
        const ticket = await playback(id, episode)
        for (const source of ticket.sources) source.src = `http://127.0.0.1:${port}${new URL(source.src).pathname}`
        return ticket
      }
    }, address.port)

    await page.getByRole('button', { name: '第1集', exact: true }).click()
    await expect(page.getByTestId('error')).toContainText('媒体加载失败')
    const beforeRetry = [...requested]
    expect(beforeRetry.some((path) => path.endsWith('/0'))).toBe(true)
    expect(beforeRetry.some((path) => path.endsWith('/1'))).toBe(true)
    expect(desktop.rows()).toEqual([bookmark])

    recovered = true
    await page.getByRole('button', { name: '重试' }).click()
    const video = historyVideo(page, 1)
    await expect(video).toHaveAttribute('src', /\/1$/)
    const retryPath = new URL((await video.getAttribute('src'))!).pathname
    expect(beforeRetry).not.toContain(retryPath)
    await expect.poll(() => position(video)).toBeCloseTo(5, 1)
    expect(await calls(page)).toEqual([1, 1].map((episode) => 'catalog:' + cmsId + ':' + episode))
    await playHistoryVideo(video)
    await page.getByRole('link', { name: '返回详情' }).click()
    await expect.poll(() => desktop.rows()[0].positionMs).toBeGreaterThan(5000)
  } finally {
    await new Promise<void>((resolve) => {
      server.close(() => resolve())
      server.closeAllConnections()
    })
  }
})

for (const scenario of ['playing', 'paused', 'blocked'] as const) {
  test(`CMS fallback preserves playback intent when ${scenario}`, async ({ page }, testInfo) => {
    const desktop = await cmsLibraryFixture(page)
    await page.goto(cmsRoute + '/play/1')
    const video = historyVideo(page, 1)
    await playHistoryVideo(video)
    await video.evaluate((media: HTMLVideoElement) => {
      media.currentTime = 6
      media.muted = true
    })
    await expect.poll(() => position(video)).toBeGreaterThanOrEqual(6)
    if (scenario === 'paused') {
      await video.evaluate((media: HTMLVideoElement) => media.pause())
    }
    if (scenario === 'blocked') await blockNextCMSAutoplay(page)
    const checkpoint = await failWithMediaError(video)
    await expect(video).toHaveAttribute('src', /\/1$/)
    if (scenario === 'playing') {
      await expect(video).toHaveJSProperty('paused', false)
      await expect(video).toHaveJSProperty('muted', true)
      await expect.poll(() => position(video)).toBeGreaterThan(checkpoint + 0.15)
    } else {
      await expect.poll(() => position(video)).toBeCloseTo(checkpoint, 1)
      await expect(video).toHaveJSProperty('paused', true)
      if (scenario === 'blocked') {
        const notice = page.getByRole('status').filter({ hasText: '已切换到备用源，请点击播放。' })
        await expect(notice).toBeVisible()
        await page.screenshot({ path: testInfo.outputPath('cms-fallback-manual.png') })
        await playHistoryVideo(video)
        await expect(notice).toHaveCount(0)
      }
    }
    expect(desktop.rows()[0].episode).toBe(1)
    expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
  })
}

test('CMS picture-in-picture follows source changes and returns inline on navigation', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    let active: HTMLVideoElement | null = null
    Object.defineProperties(document, {
      pictureInPictureEnabled: { configurable: true, value: true },
      pictureInPictureElement: { get: () => active },
      exitPictureInPicture: { value: async () => {
        const previous = active
        active = null
        previous?.dispatchEvent(new Event('leavepictureinpicture'))
      } },
    })
    Object.defineProperties(HTMLVideoElement.prototype, {
      webkitSupportsPresentationMode: { configurable: true, value: () => false },
      requestPictureInPicture: { configurable: true, value: async function (this: HTMLVideoElement) {
        active = this
        this.dispatchEvent(new Event('enterpictureinpicture'))
      } },
    })
  })
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  const pip = page.getByTestId('catalog-pip')
  await pip.click()
  await expect(pip).toHaveAttribute('aria-pressed', 'true')
  expect(await first.evaluate((media) => document.pictureInPictureElement === media)).toBe(true)
  await pip.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('cms-pip-default.png') })

  await page.evaluate(() => document.exitPictureInPicture())
  await expect(pip).toHaveAttribute('aria-pressed', 'false')
  await page.evaluate(() => Object.defineProperty(document, 'pictureInPictureEnabled', { value: false }))
  await pip.click()
  await expect(page.getByText('当前环境不支持画中画', { exact: true })).toBeVisible()
  await expect(pip).toHaveAttribute('aria-pressed', 'false')
  await page.evaluate(() => Object.defineProperty(document, 'pictureInPictureEnabled', { value: true }))
  await pip.click()
  await expect(pip).toHaveAttribute('aria-pressed', 'true')
  await first.dispatchEvent('error')
  await expect(first).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  await expect(pip).toHaveAttribute('aria-pressed', 'false')
  expect(await page.evaluate(() => document.pictureInPictureElement)).toBeNull()

  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((html) => html.style.setProperty('--user-zoom', '2.2'))
  await pip.focus()
  await expect(pip).toBeInViewport()
  await expect(pip).toBeFocused()
  await expect(page.getByText('当前环境不支持画中画', { exact: true })).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('cms-pip-220.png') })
  await pip.press('Enter')
  await expect(pip).toHaveAttribute('aria-pressed', 'true')
  await page.getByRole('link', { name: '返回详情' }).click()
  await expect(pip).toHaveCount(0)
  expect(await page.evaluate(() => document.pictureInPictureElement)).toBeNull()
})

test('CMS picture-in-picture closes a timed-out WebKit transition for an old episode', async ({ page }) => {
  await page.addInitScript(() => {
    const modes = new WeakMap<HTMLVideoElement, string>()
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: false })
    Object.defineProperties(HTMLVideoElement.prototype, {
      webkitPresentationMode: { configurable: true, get(this: HTMLVideoElement) { return modes.get(this) ?? 'inline' } },
      webkitSupportsPresentationMode: { configurable: true, value: () => true },
      webkitSetPresentationMode: { configurable: true, value(this: HTMLVideoElement, mode: string) {
        const change = () => {
          modes.set(this, mode)
          this.dispatchEvent(new Event('webkitpresentationmodechanged'))
        }
        if (mode === 'picture-in-picture') {
          document.body.dataset.pipPending = 'true'
          window.addEventListener('finish-cms-pip', change, { once: true })
        } else {
          change()
          document.body.dataset.pipClosed = 'true'
        }
      } },
    })
  })
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  await expect.poll(() => durationMs(historyVideo(page, 1))).toBeGreaterThan(1000)
  const pip = page.getByTestId('catalog-pip')
  await pip.click()
  await expect(pip).toBeDisabled()
  await expect(page.locator('body')).toHaveAttribute('data-pip-pending', 'true')
  await expect(page.getByText('无法切换画中画，请稍后重试', { exact: true })).toBeVisible()
  await expect(pip).toBeEnabled()
  await page.getByRole('button', { name: '第2集' }).click()
  await expect.poll(() => durationMs(historyVideo(page, 2))).toBeGreaterThan(1000)
  await page.evaluate(() => window.dispatchEvent(new Event('finish-cms-pip')))
  await expect(page.locator('body')).toHaveAttribute('data-pip-closed', 'true')
  await expect(pip).toBeEnabled()
  await expect(pip).toHaveAttribute('aria-pressed', 'false')
})

test('CMS manual source choice preserves pause, playback and fallback to an earlier source', async ({ page }, testInfo) => {
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute)
  await page.evaluate(() => {
    type Playback = (id: string, episode: number) => Promise<{ sources: { quality?: string; routeName?: string }[] }>
    const desktop = window as unknown as { go: { app: { App: { CatalogPlayback: Playback } } } }
    const runtime = desktop.go.app.App
    const resolve = runtime.CatalogPlayback.bind(runtime)
    runtime.CatalogPlayback = async (id, episode) => {
      const ticket = await resolve(id, episode)
      ticket.sources[0].quality = 'hd'
      ticket.sources[0].routeName = '主线路'
      return ticket
    }
  })
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  const video = historyVideo(page, 1)
  const source = page.getByRole('combobox', { name: '播放源' })
  await expect.poll(() => durationMs(video)).toBeGreaterThan(10_000)
  await expect(source).toBeVisible({ timeout: 5000 })
  await expect(source.locator('option')).toHaveText(['线路 1 · 主线路 · 高清', '线路 2'])
  await expect(video).toHaveJSProperty('paused', true)
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = 7 })
  await expect.poll(() => position(video)).toBe(7)
  await expect(video).toHaveJSProperty('seeking', false)
  await source.selectOption('1')
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => position(video)).toBe(7)
  await expect(video).toHaveJSProperty('paused', true)
  await source.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('cms-source-default.png') })

  await video.evaluate(async (media: HTMLVideoElement) => { media.muted = true; await media.play() })
  await expect.poll(() => position(video)).toBeGreaterThan(7.2)
  await source.selectOption('0')
  await expect(video).toHaveAttribute('src', /\/0$/)
  await expect(video).toHaveJSProperty('paused', false)
  await expect(video).toHaveJSProperty('muted', true)
  await expect.poll(() => position(video)).toBeGreaterThan(7.2)
  await source.selectOption('1')
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect(video).toHaveJSProperty('paused', false)
  await video.dispatchEvent('error')
  await expect(video).toHaveAttribute('src', /\/0$/)
  await expect(video).toHaveJSProperty('paused', false)
  await expect.poll(() => position(video)).toBeGreaterThan(7.2)
  await expect(source.locator('option').nth(1)).toBeDisabled()

  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((html) => html.style.setProperty('--user-zoom', '2.2'))
  await source.focus()
  await expect(source).toBeInViewport()
  await expect(source).toBeFocused()
  await page.screenshot({ path: testInfo.outputPath('cms-source-220.png') })
  await video.dispatchEvent('error')
  await expect(page.getByTestId('error')).toContainText('媒体加载失败')
  await expect(source).toHaveCount(0)
})

test('CMS displays a single named source and fits long names at 220% zoom', async ({ page }, testInfo) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.dpi = 220
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.goto(cmsRoute)
  await page.evaluate(() => {
    type Playback = (id: string, episode: number) => Promise<{ sources: { routeName?: string }[] }>
    const desktop = window as unknown as { go: { app: { App: { CatalogPlayback: Playback } } } }
    const runtime = desktop.go.app.App
    const resolve = runtime.CatalogPlayback.bind(runtime)
    runtime.CatalogPlayback = async (id, episode) => {
      const ticket = await resolve(id, episode)
      ticket.sources[0].routeName = episode === 1 ? '主线路' : '长'.repeat(32)
      if (episode === 1) ticket.sources.splice(1)
      return ticket
    }
  })
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  await expect(page.getByText('播放源 · 主线路', { exact: true })).toBeVisible()
  const source = page.getByRole('combobox', { name: '播放源' })
  await expect(source).toHaveCount(0)

  await page.getByRole('button', { name: '第2集', exact: true }).click()
  await expect(source.locator('option')).toHaveText(['线路 1 · ' + '长'.repeat(32), '线路 2'])
  await source.focus()
  await source.scrollIntoViewIfNeeded()
  const bounds = await source.evaluate((control) => {
    const rect = control.getBoundingClientRect()
    return { left: rect.left, right: rect.right, viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth }
  })
  expect(bounds.left).toBeGreaterThanOrEqual(0)
  expect(bounds.right).toBeLessThanOrEqual(bounds.viewportWidth)
  expect(bounds.documentWidth).toBeLessThanOrEqual(bounds.viewportWidth)
  await expect(source).toBeInViewport({ ratio: 0.95 })
  await page.screenshot({ path: testInfo.outputPath('cms-source-long-220.png') })
})

test('CMS keeps the chosen named route across manual and automatic episode changes', async ({ page }, testInfo) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.playMode = '连续播放（自动下一集）'
  await page.goto(cmsRoute)
  await page.evaluate(() => {
    type Playback = (id: string, episode: number) => Promise<{
      sources: { src: string; type: string; routeName?: string }[]
    }>
    const desktop = window as unknown as { go: { app: { App: { CatalogPlayback: Playback } } } }
    const runtime = desktop.go.app.App
    const resolve = runtime.CatalogPlayback.bind(runtime)
    let episodeTwoVisits = 0
    runtime.CatalogPlayback = async (id, episode) => {
      const ticket = await resolve(id, episode)
      const visit = episode === 2 ? ++episodeTwoVisits : 0
      ticket.sources[0].routeName = visit === 2 ? '备用线' : '主线路'
      ticket.sources[1].routeName = episode === 3 || visit === 3 ? '其他线路' : '备用线'
      if (episode === 3) {
        const backup = ticket.sources[1]
        ticket.sources.push({ ...backup, src: backup.src.replace(/\/1$/, '/2'), routeName: '备用线' })
      }
      return ticket
    }
  })
  const source = page.getByRole('combobox', { name: '播放源' })
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  await source.selectOption('1')
  await expect(first).toHaveAttribute('src', /\/1$/)

  await page.getByRole('button', { name: '第2集', exact: true }).click()
  const second = historyVideo(page, 2)
  await expect(second).toHaveAttribute('src', /\/1$/)
  await expect(second).toHaveJSProperty('paused', true)
  await playHistoryVideo(second)
  await second.evaluate((media: HTMLVideoElement) => {
    media.muted = true
    media.currentTime = media.duration - 0.45
  })

  await expect(page).toHaveURL(/\/play\/3$/)
  const third = historyVideo(page, 3)
  await expect(third).toHaveAttribute('src', /\/2$/)
  await expect(source).toHaveValue('2')
  await expect.poll(() => position(third)).toBeGreaterThan(0.1)
  await expect(third).toHaveJSProperty('muted', true)
  await source.scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('cms-route-preference.png') })

  await third.dispatchEvent('error')
  await expect(third).toHaveAttribute('src', /\/0$/)
  await page.getByRole('button', { name: '第2集', exact: true }).click()
  await expect(second).toHaveAttribute('src', /\/0$/)
  await expect(source.locator('option')).toHaveText(['线路 1 · 备用线', '线路 2 · 备用线'])
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  await expect(first).toHaveAttribute('src', /\/1$/)

  await page.getByRole('button', { name: '第2集', exact: true }).click()
  await expect(second).toHaveAttribute('src', /\/0$/)
  await expect(source.locator('option')).toHaveText(['线路 1 · 主线路', '线路 2 · 其他线路'])
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  await expect(first).toHaveAttribute('src', /\/1$/)

  await page.getByRole('link', { name: '返回详情' }).click()
  await page.getByRole('button', { name: '第1集', exact: true }).click()
  await expect(first).toHaveAttribute('src', /\/0$/)
})

test('CMS fallback preserves a newer paused seek after manual source selection', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  const source = page.getByRole('combobox', { name: '播放源' })
  const seekWhilePaused = async (seconds: number) => {
    await expect(video).toHaveJSProperty('paused', true)
    await video.evaluate((media: HTMLVideoElement, seconds) => new Promise<void>((resolve) => {
      media.addEventListener('seeked', () => resolve(), { once: true })
      media.currentTime = seconds
    }), seconds)
    await expect.poll(() => position(video)).toBe(seconds)
  }
  await expect.poll(() => durationMs(video)).toBeGreaterThan(10_000)
  await seekWhilePaused(7)
  await source.selectOption('1')
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => position(video)).toBe(7)
  await seekWhilePaused(12)
  await video.dispatchEvent('error')
  await expect(video).toHaveAttribute('src', /\/0$/)
  await expect.poll(() => position(video)).toBe(12)
  await expect(video).toHaveJSProperty('paused', true)
  expect(desktop.rows()).toEqual([])
})

test('CMS fallback stays paused when playback is cancelled during a manual switch', async ({ page }) => {
  await cmsLibraryFixture(page)
  await page.addInitScript(() => {
    const play = HTMLMediaElement.prototype.play
    HTMLMediaElement.prototype.play = function () {
      if (!this.getAttribute('src')?.endsWith('/1')) return play.call(this)
      this.dataset.playRequested = 'true'
      return new Promise<void>((_, reject) => {
        this.addEventListener('pause', () => reject(new DOMException('Playback was paused', 'AbortError')), { once: true })
      })
    }
  })
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  const source = page.getByRole('combobox', { name: '播放源' })
  await playHistoryVideo(video)
  await source.selectOption('1')
  await expect(video).toHaveAttribute('data-play-requested', 'true')
  await video.evaluate((media: HTMLVideoElement) => {
    media.dispatchEvent(new Event('pause'))
    media.dispatchEvent(new Event('error'))
  })
  await expect(video).toHaveAttribute('src', /\/0$/)
  await expect(source.locator('option').nth(1)).toBeDisabled()
  await expect.poll(() => durationMs(video)).toBeGreaterThan(10_000)
  await expect(video).toHaveJSProperty('paused', true)
  await expect(page.getByText('已切换到备用源，请点击播放。')).toHaveCount(0)
})

test('CMS returns to the previous source when the selected media request fails', async ({ page }) => {
  await cmsLibraryFixture(page)
  await page.route(mediaURL + '/catalog-media/' + 'A'.repeat(26) + '/1', (route) => route.fulfill({ status: 404 }))
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  const source = page.getByRole('combobox', { name: '播放源' })
  await expect.poll(() => durationMs(video)).toBeGreaterThan(10_000)
  const failedResponse = page.waitForResponse((response) =>
    response.url() === mediaURL + '/catalog-media/' + 'A'.repeat(26) + '/1' && response.status() === 404)
  await source.selectOption('1')
  await failedResponse
  await expect(source.locator('option').nth(1)).toBeDisabled()
  await expect(video).toHaveAttribute('src', /\/0$/)
  await expect.poll(() => durationMs(video)).toBeGreaterThan(10_000)
  await expect(video).toHaveJSProperty('paused', true)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
})

test('CMS native volume carries across a fallback and a manual episode change', async ({ page }) => {
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  // A decoder can fail before the queued volumechange event is delivered.
  await failWithMediaError(first, { volume: 0.23 })
  await expect(first).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  await expect(first).toHaveJSProperty('volume', 0.23)

  await first.evaluate((media: HTMLVideoElement) => { media.volume = 0 })
  await page.getByRole('button', { name: '第2集' }).click()
  const second = historyVideo(page, 2)
  await expect.poll(() => durationMs(second)).toBeGreaterThan(1000)
  await expect(second).toHaveJSProperty('volume', 0)
})

test('CMS stalled playback switches sources even when the media element emits no error', async ({ page }) => {
  await page.addInitScript(() => {
    window.setTimeout = new Proxy(window.setTimeout, {
      apply(schedule, browser, args) {
        if (args[1] === 20000) args[1] = 950
        return Reflect.apply(schedule, browser, args)
      },
    })
  })
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await playHistoryVideo(video)
  await video.evaluate((media: HTMLVideoElement) => {
    media.pause()
    media.currentTime = 4
  })
  await expect.poll(() => position(video)).toBeCloseTo(4, 2)
  await page.waitForTimeout(1050)
  await expect(video).toHaveAttribute('src', /\/0$/)
  await playHistoryVideo(video)
  const checkpoint = await video.evaluate((media: HTMLVideoElement) => {
    const frozen = media.currentTime
    Object.defineProperty(media, 'currentTime', { get: () => frozen })
    return frozen
  })
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect(video).toHaveJSProperty('paused', false)
  await expect.poll(() => position(video)).toBeGreaterThan(checkpoint + 0.15)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
})

test('CMS shares supported native rates across a fallback and a manual episode change', async ({ page }) => {
  await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  await failWithMediaError(first, { speed: 1.5 })
  await expect(first).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  await expect(first).toHaveJSProperty('playbackRate', 1.5)
  await expect(first).toHaveJSProperty('defaultPlaybackRate', 1.5)
  await first.evaluate(async (media: HTMLVideoElement) => {
    const changed = new Promise<void>((resolve) => media.addEventListener('ratechange', () => resolve(), { once: true }))
    media.playbackRate = 3
    await changed
  })
  await page.getByRole('button', { name: '第2集' }).click()
  const second = historyVideo(page, 2)
  await playHistoryVideo(second)
  await expect(second).toHaveJSProperty('playbackRate', 1.5)
})

test('CMS HLS recovery retains the latest rate before the native event is delivered', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  await page.addInitScript(() => {
    const create = URL.createObjectURL
    URL.createObjectURL = (object) => {
      if (object instanceof MediaSource) Object.assign(window, { cmsMediaSource: object })
      return create(object)
    }
  })
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  const original = await video.getAttribute('src')
  expect(original).toMatch(/^blob:/)
  const recoveredRate = await video.evaluate((media: HTMLVideoElement) => {
    const source = (window as unknown as { cmsMediaSource: MediaSource }).cmsMediaSource
    media.playbackRate = 1.5
    source.dispatchEvent(new Event('sourceclose'))
    return media.playbackRate
  })
  expect(recoveredRate).toBe(1.5)
  await expect.poll(() => video.getAttribute('src')).not.toBe(original)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await expect(video).toHaveJSProperty('playbackRate', 1.5)
})

test('CMS media fallback keeps the sampled bookmark when a failed decoder resets time', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await playHistoryVideo(video)
  await video.evaluate((media: HTMLVideoElement) => {
    media.pause()
    media.currentTime = 7
  })
  await expect.poll(() => desktop.rows()[0]?.positionMs).toBe(7000)
  expect(await failWithMediaError(video, { resetPosition: true })).toBe(0)
  await expect(video).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => position(video)).toBeCloseTo(7, 2)
  expect(desktop.rows()[0]?.positionMs).toBe(7000)
})

test('CMS completion survives a paused scrub and only offers an available next episode', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.playMode = '单集播放'
  await page.goto(cmsRoute + '/play/1')
  const video = historyVideo(page, 1)
  await playHistoryVideo(video)
  const duration = await durationMs(video)
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = media.duration - 0.1 })
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ percent: 100, positionMs: duration, episode: 1 })
  await expect(video).toHaveJSProperty('ended', true)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
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

test('CMS continuous play saves completion and decodes the adjacent episode through an alternate source', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  const secondPrefix = mediaURL + '/catalog-media/' + 'B'.repeat(26) + '/'
  await page.route(secondPrefix + '0', (route) => route.fulfill({ status: 404 }))
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  await expect(first).toHaveJSProperty('paused', true)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
  desktop.catalog.format = 'webm'
  const completedDuration = await durationMs(first)
  await first.evaluate((media: HTMLVideoElement) => {
    media.volume = 0.41
    media.playbackRate = 1.25
    media.muted = true
  })
  await playHistoryVideo(first)

  await expect(page).toHaveURL(/\/play\/2$/)
  const second = historyVideo(page, 2)
  await expect(second).toHaveAttribute('src', secondPrefix + '1')
  await expect.poll(() => second.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await expect(second).toHaveJSProperty('volume', 0.41)
  await expect(second).toHaveJSProperty('playbackRate', 1.25)
  await expect(second).toHaveJSProperty('muted', true)
  await expect.poll(() => position(second)).toBeGreaterThan(0.1)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({
    dramaId: cmsId, episode: 1, percent: 100, positionMs: completedDuration,
  })
  expect(await calls(page)).toEqual([1, 2].map((index) => 'catalog:' + cmsId + ':' + index))
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26)])
})

test('CMS keeps prior completion when the automatic next episode fails before a checkpoint', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  desktop.catalog.format = 'webm'
  await playHistoryVideo(first)

  const second = historyVideo(page, 2)
  await expect.poll(() => position(second)).toBeGreaterThan(0.35)
  await expect(second).toHaveAttribute('src', /\/0$/)
  expect(await second.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  expect(await failWithMediaError(second, { pauseFirst: true })).toBeLessThan(5)

  await expect(second).toHaveAttribute('src', /\/1$/)
  await expect.poll(() => position(second)).toBe(0)
  await expect(second).toHaveJSProperty('paused', true)
  expect(desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  expect(await calls(page)).toEqual([1, 2].map((index) => 'catalog:' + cmsId + ':' + index))

  await playHistoryVideo(second)
  await second.evaluate((media: HTMLVideoElement) => media.pause())
  await expect.poll(() => desktop.rows()[0]?.episode).toBe(2)
})

test('CMS continuous play offers manual playback if the browser refuses automatic start', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  await page.addInitScript(() => {
    const play = HTMLMediaElement.prototype.play
    let blocked = false
    HTMLMediaElement.prototype.play = function () {
      if (!blocked && this.getAttribute('aria-label') === '第2集视频') {
        blocked = true
        return Promise.reject(new DOMException('Blocked by autoplay policy', 'NotAllowedError'))
      }
      return play.call(this)
    }
  })
  await page.goto(cmsRoute + '/play/1')
  await playHistoryVideo(historyVideo(page, 1))

  const notice = page.getByText('已切换到下一集，请点击播放。', { exact: true })
  await expect(notice).toBeVisible()
  const second = historyVideo(page, 2)
  await expect(second).toHaveAttribute('src', /^blob:/)
  await expect.poll(() => second.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBe(64)
  await expect(second).toHaveJSProperty('paused', true)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  await playHistoryVideo(second)
  await expect(notice).toHaveCount(0)
  expect(await calls(page)).toEqual([1, 2].map((index) => 'catalog:' + cmsId + ':' + index))
})

test('CMS continuous play stops at a missing adjacent episode', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  desktop.catalog.available = [1, 3]
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await playHistoryVideo(first)
  await expect(first).toHaveJSProperty('ended', true)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  await expect(page).toHaveURL(/\/play\/1$/)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
})

test('CMS seeking straight to the end while playing neither completes nor advances the episode', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await playHistoryVideo(first)
  await first.evaluate((media: HTMLVideoElement) => media.pause())
  await expect.poll(() => desktop.rows()[0]?.episode).toBe(1)
  await playHistoryVideo(first)
  await first.evaluate((media: HTMLVideoElement) => new Promise<void>((resolve) => {
    media.addEventListener('ended', () => resolve(), { once: true })
    media.currentTime = media.duration
  }))
  await expect(page).toHaveURL(/\/play\/1$/)
  expect(desktop.rows()[0]?.percent).toBeLessThan(100)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
})

test('CMS continuous play keeps completion when the next ticket has expired and retries manually', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  await page.goto(cmsRoute + '/play/1')
  await playHistoryVideo(historyVideo(page, 1))
  desktop.catalog.expired = true

  await expect(page).toHaveURL(/\/play\/2$/)
  await expect(page.getByTestId('error')).toContainText('无法播放下一集')
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  expect(desktop.requests.every((address) => address.startsWith(mediaURL + '/catalog-media/' + 'A'.repeat(26)))).toBe(true)
  expect(await calls(page)).toEqual([1, 2].map((index) => 'catalog:' + cmsId + ':' + index))

  desktop.catalog.expired = false
  desktop.catalog.format = 'webm'
  await page.getByRole('button', { name: '重试' }).click()
  const second = historyVideo(page, 2)
  await expect.poll(() => second.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await expect(second).toHaveJSProperty('paused', true)
  expect(desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  expect(await calls(page)).toEqual([1, 2, 2].map((index) => 'catalog:' + cmsId + ':' + index))
})

test('CMS manual selection while the next ticket is pending leaves the selected episode paused', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  desktop.catalog.holdPlaybackEpisode = 2
  await page.goto(cmsRoute + '/play/1')
  await playHistoryVideo(historyVideo(page, 1))
  desktop.catalog.format = 'webm'
  await expect.poll(() => calls(page)).toEqual([1, 2].map((index) => 'catalog:' + cmsId + ':' + index))
  await page.getByRole('button', { name: '第3集', exact: true }).click()

  const third = historyVideo(page, 3)
  await expect.poll(() => third.evaluate((media: HTMLVideoElement) => media.videoWidth)).toBeGreaterThan(0)
  await page.evaluate(() => (window as Window & { catalogTest?: { release(): void } }).catalogTest?.release())
  await expect.poll(() => released(page)).toEqual(['A'.repeat(26), 'B'.repeat(26)])
  await expect(third).toHaveJSProperty('paused', true)
  expect(desktop.requests.every((address) => !address.startsWith(mediaURL + '/catalog-media/' + 'B'.repeat(26)))).toBe(true)
  expect(desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  expect(await calls(page)).toEqual([1, 2, 3].map((index) => 'catalog:' + cmsId + ':' + index))
})

test('CMS single episode loop plays again from zero under the same ticket', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  desktop.catalog.playMode = '单集循环'
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await expect.poll(() => durationMs(first)).toBeGreaterThan(1000)
  const restartedAt = first.evaluate((media: HTMLVideoElement) => new Promise<number>((resolve) => {
    const ended = () => media.addEventListener('playing', () => resolve(media.currentTime), { once: true })
    media.addEventListener('ended', ended, { once: true })
  }))
  await playHistoryVideo(first)
  expect(await restartedAt).toBeLessThan(0.15)
  await expect.poll(() => position(first)).toBeGreaterThan(0.1)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  await expect(page).toHaveURL(/\/play\/1$/)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
  expect(await released(page)).toEqual([])
})

test('CMS loop stays ended when an active replay is scrubbed directly to the end', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.playMode = '单集循环'
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await playHistoryVideo(first)
  const restartedAt = await first.evaluate((media: HTMLVideoElement) => new Promise<number>((resolve) => {
    const ended = () => media.addEventListener('playing', () => resolve(media.currentTime), { once: true })
    media.addEventListener('ended', ended, { once: true })
    media.currentTime = media.duration - 0.25
  }))
  expect(restartedAt).toBeLessThan(0.15)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  await expect.poll(() => position(first)).toBeGreaterThan(0.1)

  await first.evaluate((media: HTMLVideoElement) => new Promise<void>((resolve) => {
    media.addEventListener('ended', () => resolve(), { once: true })
    media.currentTime = media.duration
  }))
  await expect(first).toHaveJSProperty('ended', true)
  await expect(first).toHaveJSProperty('paused', true)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
})

test('CMS close at the end saves completion and cancels pending automatic advance', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.format = 'hls'
  await page.goto(cmsRoute + '/play/1')
  const first = historyVideo(page, 1)
  await first.evaluate((media: HTMLVideoElement) => {
    const { close } = (window as unknown as { catalogTest: { close(): void } }).catalogTest
    media.addEventListener('ended', close, { capture: true, once: true })
  })
  await playHistoryVideo(first)
  await expect(first).toHaveJSProperty('ended', true)
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 1, percent: 100 })
  await expect(page).toHaveURL(/\/play\/1$/)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':1'])
})

test('CMS history remains manageable offline and after a source or episode changes', async ({ page }) => {
  const obsolete = 'apple-cms:ffffffffffffffffffffffffffffffff:90071992547409931234'
  const row = (dramaId: string, episode: number): HistoryRow => ({
    dramaId, episode, positionMs: 5000, percent: 20, durationMs: 24000, episodeCountAtWatch: 3, watchedAt: Date.now(),
  })
  const desktop = await cmsLibraryFixture(page, [row(cmsId, 2), row(obsolete, 1)])
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
  await cmsLibraryFixture(page, records)
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

test('CMS favorites persist through refresh and use the earliest available episode and then the saved episode', async ({ page }, testInfo) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.available = [3]
  const cover = mediaURL + '/catalog-cover/' + '1'.repeat(32)
  desktop.catalog.cover = cover
  await page.route(cover, (route) => route.fulfill({ status: 410 }))

  await page.goto(cmsRoute)
  const favorite = page.getByTestId('fav')
  await expect(favorite).toHaveAttribute('aria-pressed', 'false')
  await favorite.click()
  await expect(favorite).toHaveText('已收藏')
  await expect(favorite).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(desktop.favorites).toEqual([cmsId])

  await page.getByTestId('nav-favorites').click()
  await expect(page).toHaveURL(/\/favorites$/)
  const card = page.getByTestId('fav-card')
  await expect(card).toContainText(cmsTitle)
  await page.reload()
  await expect(card).toContainText(cmsTitle)
  const image = card.getByRole('img', { name: cmsTitle })
  await expect(image).toHaveAttribute('crossorigin', 'anonymous')
  await expect(image).toHaveAttribute('src', /^data:image\/svg\+xml/)
  await page.screenshot({ path: testInfo.outputPath('apple-cms-favorites.png') })
  await card.getByTestId('fav-play').click()
  await expect(page).toHaveURL(/\/play\/3$/)
  const video = historyVideo(page, 3)
  await playHistoryVideo(video)
  await video.evaluate((media: HTMLVideoElement) => {
    media.pause()
    media.currentTime = 5
  })
  await expect.poll(() => desktop.rows()[0]).toMatchObject({ episode: 3, positionMs: 5000 })
  desktop.catalog.available = [2, 3]

  await page.getByTestId('nav-favorites').click()
  await expect(card.getByTestId('fav-play')).toHaveText('继续观看')
  await card.getByTestId('fav-play').click()
  await expect(page).toHaveURL(/\/play\/3$/)
  await expect.poll(() => position(video)).toBeCloseTo(5, 2)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':3', 'catalog:' + cmsId + ':3'])

  desktop.catalog.available = [2]
  await page.getByTestId('nav-favorites').click()
  await expect(card).toContainText('上次观看的分集不可用')
  await card.getByRole('button', { name: '选集' }).click()
  await expect(page).toHaveURL(/\/catalog\/[^/]+$/)
  await expect(page.getByRole('heading', { name: cmsTitle })).toBeVisible()
  await expect(page.getByTestId('catalog-continue')).toHaveCount(0)
  expect(await calls(page)).toHaveLength(2)
})

test('CMS detail can toggle favorites while playback is unavailable', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page)
  desktop.catalog.available = []
  await page.goto(cmsRoute)
  const favorite = page.getByTestId('fav')
  await expect(page.getByText('当前剧目播放暂不可用。')).toBeVisible()
  await expect(page.getByTestId('catalog-watch-now')).toHaveCount(0)
  await favorite.click()
  await expect.poll(desktop.favorites).toEqual([cmsId])
  await page.getByTestId('nav-favorites').click()
  const card = page.getByTestId('fav-card')
  await expect(card).toContainText('当前剧目播放暂不可用')
  await card.getByRole('button', { name: '查看详情' }).click()
  await expect(favorite).toHaveText('已收藏')
  await favorite.click()
  await expect.poll(desktop.favorites).toEqual([])
  await expect(favorite).toHaveText('收藏')
  expect(await calls(page)).toEqual([])
})

const cmsFavoriteReference = (id: string) => id.split(':').slice(1).join(':')
const cmsFavoriteLabel = (id: string) => '苹果 CMS 短剧 · ' + cmsFavoriteReference(id)

for (const [path, item, play] of [
  ['favorites', 'fav-card', 'fav-play'], ['history', 'history-item', 'resume'],
] as const) {
  test(`CMS ${path} rechecks a failed bookmark without leaving the page`, async ({ page }, testInfo) => {
    const desktop = await cmsLibraryFixture(page, [{
      dramaId: cmsId, episode: 3, percent: 25, watchedAt: Date.now(),
      positionMs: 6000, durationMs: 24000, episodeCountAtWatch: 3,
    }], [cmsId])
    desktop.catalog.available = [3]
    desktop.catalog.offline = true
    await page.goto('/#/' + path)

    const row = page.getByTestId(item)
    const retry = row.getByRole('button', { name: '重试核验' })
    await expect(retry).toBeEnabled()
    await expect(row).toContainText('当前片源不可用')
    await expect(row.getByTestId(play)).toHaveCount(0)
    await row.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath('cms-' + path + '-retry.png') })

    desktop.catalog.offline = false
    desktop.catalog.detailDelayMs = 500
    await retry.click()
    await expect(row.getByRole('button', { name: '正在核验…' })).toBeDisabled()
    await expect(row.getByTestId(play)).toHaveText('继续观看')
    await expect(retry).toHaveCount(0)
    expect(desktop.favorites()).toEqual([cmsId])
    expect(desktop.rows()[0]).toMatchObject({ episode: 3, positionMs: 6000 })
    expect(desktop.writes).toHaveLength(0)
    expect(await page.evaluate(() => (window as unknown as {
      catalogTest: { lookups: string[] }
    }).catalogTest.lookups)).toEqual([cmsId, cmsId])
    await row.getByTestId(play).click()
    await expect(page).toHaveURL(/\/play\/3$/)
    await expect.poll(() => position(historyVideo(page, 3))).toBeCloseTo(6, 2)
  })
}

async function holdCMSDetails(page: Page, catalog: CatalogState) {
  let release!: () => void
  const pending = new Promise<void>((resolve) => { release = resolve })
  await page.exposeFunction('waitForCMSDetails', () => pending)
  catalog.holdDetail = true
  return release
}

test('CMS favorites stay removable when the source is disabled, switched, or the title disappears', async ({ page }) => {
  const obsolete = 'apple-cms:ffffffffffffffffffffffffffffffff:90071992547409931234'
  const other = 'apple-cms:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee:90071992547409931234'
  const desktop = await cmsLibraryFixture(page, [], [cmsId, obsolete, other])
  desktop.catalog.provider = 'authorized-fixture'
  await page.goto('/#/favorites')
  const cards = page.getByTestId('fav-card')
  const unavailable = cards.filter({ hasText: '当前片源不可用' })
  await expect(unavailable).toHaveCount(3)
  await expect(unavailable.getByRole('button', { name: '重试核验' })).toHaveCount(0)
  const reads = () => page.evaluate(() => (window as unknown as { catalogTest: { lookups: string[] } }).catalogTest.lookups)
  expect(await reads()).toEqual([])
  await expect(page.getByTestId('fav-play')).toHaveCount(0)
  const chosen = unavailable.filter({ hasText: cmsFavoriteReference(other) })
  const retained = unavailable.filter({ hasText: cmsFavoriteReference(obsolete) })
  const choose = chosen.getByRole('button', { name: cmsFavoriteLabel(other), exact: true })
  await expect(choose).toBeDisabled()
  await chosen.getByTestId('fav-more').click()
  await expect(chosen.getByRole('button', { name: '查看详情' })).toHaveCount(0)
  await page.getByTestId('batch-toggle').click()
  await choose.click()
  await expect(choose).toHaveAttribute('aria-pressed', 'true')
  await expect(retained.getByRole('button', { name: cmsFavoriteLabel(obsolete), exact: true })).toHaveAttribute('aria-pressed', 'false')
  await page.getByTestId('batch-unfav').click()
  await page.getByRole('button', { name: '确认取消' }).click()
  await expect.poll(desktop.favorites).toEqual([cmsId, obsolete])

  desktop.catalog.provider = 'apple-cms'
  desktop.catalog.id = obsolete
  await page.reload()
  await expect(cards.filter({ hasText: cmsTitle })).toBeVisible()
  await expect(unavailable).toHaveCount(1)
  await expect(unavailable.getByTestId('fav-play')).toHaveCount(0)
  await expect(unavailable.getByRole('button', { name: '重试核验' })).toHaveCount(0)
  await unavailable.getByTestId('fav-unfav').click()
  await expect.poll(desktop.favorites).toEqual([obsolete])

  desktop.catalog.id = cmsId
  await page.reload()
  await expect(unavailable).toHaveCount(1)
  await expect(page.getByTestId('fav-play')).toHaveCount(0)
  await page.getByTestId('batch-toggle').click()
  await unavailable.getByRole('button', { name: cmsFavoriteLabel(obsolete), exact: true }).click()
  await page.getByTestId('batch-unfav').click()
  await page.getByRole('button', { name: '确认取消' }).click()
  await expect.poll(desktop.favorites).toEqual([])
  await page.reload()
  await expect(cards).toHaveCount(0)
  expect(await calls(page)).toEqual([])
})

test('CMS favorites check the latest twelve and leave unchecked cards selectable', async ({ page }) => {
  const favorites = Array.from({ length: 25 }, (_, index) =>
    'apple-cms:0123456789abcdef0123456789abcdef:' + (index + 1),
  )
  const desktop = await cmsLibraryFixture(page, [], favorites)
  desktop.catalog.detailDelayMs = 250
  await page.goto('/#/favorites')
  const cards = page.getByTestId('fav-card')
  const loadMore = page.getByRole('button', { name: '核验更多片源收藏' })
  const reads = () => page.evaluate(() => (window as unknown as { catalogTest: { lookups: string[] } }).catalogTest.lookups)
  await expect(cards).toHaveCount(25)
  await expect(loadMore).toBeEnabled()
  expect(await reads()).toEqual(favorites.slice(0, 12))
  await expect(cards.nth(12)).toContainText('尚未核验片源')
  await expect(cards.nth(12).getByTestId('fav-play')).toHaveCount(0)

  await loadMore.click()
  await expect(loadMore).toBeEnabled()
  expect(await reads()).toEqual(favorites.slice(0, 24))
  await expect(cards.nth(23)).toContainText('当前片源不可用')
  const last = cards.last()
  await expect(last).toContainText('尚未核验片源')
  await page.getByTestId('edit-manage').click()
  await last.getByRole('button', { name: cmsFavoriteLabel(favorites[24]), exact: true }).click()
  await page.getByTestId('batch-unfav').click()
  await page.getByRole('button', { name: '确认取消' }).click()
  await expect.poll(desktop.favorites).toEqual(favorites.slice(0, 24))
  await expect(loadMore).toHaveCount(0)
  expect(await reads()).toEqual(favorites.slice(0, 24))
  expect(await page.evaluate(() => (window as unknown as {
    catalogTest: { peakLookups: number }
  }).catalogTest.peakLookups)).toBe(3)
})

test('CMS favorites drop a hidden selection and dismiss its confirmation when verification changes the watch status', async ({ page }) => {
  const bookmark: HistoryRow = {
    dramaId: cmsId, episode: 1, percent: 100, watchedAt: Date.now(),
    positionMs: 24000, durationMs: 24000, episodeCountAtWatch: 1,
  }
  const desktop = await cmsLibraryFixture(page, [bookmark], [cmsId])
  const release = await holdCMSDetails(page, desktop.catalog)
  try {
    await page.goto('/#/favorites')
    await page.getByTestId('ftab-done').click()
    const card = page.getByTestId('fav-card')
    await expect(card).toContainText('已看过')
    await page.getByTestId('edit-manage').click()
    await card.getByRole('button', { name: cmsFavoriteLabel(cmsId), exact: true }).click()
    await page.getByTestId('batch-unfav').click()
    const dialog = page.getByRole('dialog', { name: '取消收藏' })
    await expect(dialog).toBeVisible()

    release()
    await expect(card).toHaveCount(0)
    await expect(dialog).toHaveCount(0)
    await expect(page.getByTestId('batchbar')).toContainText('已选 0 部')
    await expect(page.getByTestId('batch-unfav')).toBeDisabled()
    await expect(page.getByRole('button', { name: '完成', exact: true })).toBeFocused()
    await page.getByTestId('ftab-watching').click()
    await expect(card).toContainText(cmsTitle)
    await expect(card.getByRole('button', { name: cmsTitle, exact: true })).toHaveAttribute('aria-pressed', 'false')
    expect(desktop.favorites()).toEqual([cmsId])
    expect(desktop.writes).toHaveLength(0)
  } finally {
    release()
  }
})

test('CMS library views share three lookup slots and skip requests queued by an unmounted page', async ({ page }) => {
  const saved = Array.from({ length: 5 }, (_, i) => 'apple-cms:0123456789abcdef0123456789abcdef:' + (i + 1))
  const desktop = await cmsLibraryFixture(page, [{
    dramaId: cmsId, episode: 1, percent: 20, watchedAt: Date.now(),
    positionMs: 5000, durationMs: 24000, episodeCountAtWatch: 3,
  }], saved)
  const release = await holdCMSDetails(page, desktop.catalog)
  const readState = () => page.evaluate(() => (window as unknown as {
    catalogTest: { lookups: string[]; peakLookups: number; providerReads: number }
  }).catalogTest)
  try {
    await page.goto('/#/favorites')
    await expect.poll(async () => (await readState()).lookups).toEqual(saved.slice(0, 3))
    const first = await readState()

    await page.getByTestId('nav-history').click()
    await expect(page.getByTestId('page-history')).toBeVisible()
    await expect.poll(async () => (await readState()).providerReads).toBeGreaterThan(first.providerReads)
    const history = await readState()

    await page.getByTestId('nav-favorites').click()
    await expect(page.getByTestId('page-favorites')).toBeVisible()
    await expect.poll(async () => (await readState()).providerReads).toBeGreaterThan(history.providerReads)
    expect((await readState()).lookups).toEqual(saved.slice(0, 3))
    release()
    await expect(page.getByTestId('fav-card').filter({ hasText: '当前片源不可用' })).toHaveCount(5)
    const final = await readState()
    expect(final.lookups).toEqual([...saved.slice(0, 3), ...saved])
    expect(final.peakLookups).toBe(3)
  } finally {
    release()
  }
})


test('CMS home resumes a verified bookmark even if the main catalog request fails', async ({ page }, testInfo) => {
  const row = (dramaId: string, watchedAt: number): HistoryRow => ({
    dramaId, episode: 3, percent: 20, watchedAt, positionMs: 5000, durationMs: 24000, episodeCountAtWatch: 3,
  })
  const now = Date.now()
  const obsolete = Array.from({ length: 13 }, (_, index) =>
    row('apple-cms:ffffffffffffffffffffffffffffffff:' + (index + 1), now - index))
  const desktop = await cmsLibraryFixture(page, [...obsolete, row(cmsId, now - 1000)])
  desktop.catalog.home = 'error'
  desktop.catalog.available = [3]
  const cover = mediaURL + '/catalog-cover/' + '2'.repeat(32)
  desktop.catalog.cover = cover
  await page.route(cover, (route) => route.fulfill({ status: 410 }))

  await page.goto('/#/home')
  const resume = page.getByTestId('resume-card')
  await expect(page.getByTestId('error')).toBeVisible()
  await expect(resume).toHaveCount(1)
  await expect(resume).toContainText(cmsTitle)
  await expect(resume).toContainText('第3集')
  await expect(resume.getByRole('img')).toHaveAttribute('src', /^data:image\/svg\+xml/)
  await expect.poll(() => calls(page)).toEqual([])
  await page.screenshot({ path: testInfo.outputPath('apple-cms-home-resume.png') })
  await resume.click()
  await expect(page).toHaveURL(/\/play\/3$/)
  const video = historyVideo(page, 3)
  await expect.poll(() => position(video)).toBeCloseTo(5, 2)
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':3'])
})

test('CMS home only offers a completed bookmark when the next episode is available', async ({ page }) => {
  const local = [1, 2, 3].map((index) => ({
    dramaId: 'local-' + index, episode: 1, percent: 50, watchedAt: Date.now(),
    positionMs: 12000, durationMs: 24000, episodeCountAtWatch: 3,
  }))
  const desktop = await cmsLibraryFixture(page, [{
    dramaId: cmsId, episode: 1, percent: 100, watchedAt: Date.now(),
    positionMs: 24000, durationMs: 24000, episodeCountAtWatch: 3,
  }, ...local])
  desktop.catalog.localBookmarks = true
  desktop.catalog.home = 'empty'
  desktop.catalog.available = [1, 2]
  await page.goto('/#/home')
  await expect(page.getByText('暂无片源')).toBeVisible()
  const resume = page.getByTestId('resume-card')
  await expect(resume).toContainText('下一集 · 第2集')
  await resume.click()
  await expect(page).toHaveURL(/\/play\/2$/)
  await expect(historyVideo(page, 2)).toBeVisible()
  expect(await calls(page)).toEqual(['catalog:' + cmsId + ':2'])
  await page.goBack()
  await expect(page.getByTestId('page-home')).toBeVisible()

  desktop.catalog.available = [1, 3]
  desktop.catalog.localBookmarks = false
  delete desktop.catalog.home
  await page.reload()
  await expect.poll(() => page.evaluate(() => (window as unknown as {
    catalogTest: { lookups: string[] }
  }).catalogTest.lookups)).toEqual([cmsId])
  await expect(page.getByText('暂无可继续观看的短剧')).toBeVisible()
  await expect(resume).toHaveCount(0)
  expect(await calls(page)).toEqual([])
})


test('retrying a failed CMS home rechecks bookmarks once the connection recovers', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page, [{
    dramaId: cmsId, episode: 3, percent: 25, watchedAt: Date.now(),
    positionMs: 6000, durationMs: 24000, episodeCountAtWatch: 3,
  }])
  desktop.catalog.home = 'error'
  desktop.catalog.offline = true
  const lookups = () => page.evaluate(() => (window as unknown as {
    catalogTest: { lookups: string[] }
  }).catalogTest.lookups)
  await page.goto('/#/home')
  const error = page.getByTestId('error')
  await expect(error).toBeVisible()
  await expect.poll(() => page.evaluate(() => (window as unknown as {
    catalogTest: { failedLookups: number }
  }).catalogTest.failedLookups)).toBe(1)
  desktop.catalog.offline = false
  delete desktop.catalog.home
  await error.getByRole('button', { name: '重试' }).click()
  await expect(page.getByTestId('resume-card')).toContainText(cmsTitle)
  await expect.poll(lookups).toEqual([cmsId, cmsId])
  expect(await calls(page)).toEqual([])
})

test('CMS home retries only failed resume checks while the directory remains available', async ({ page }, testInfo) => {
  const missing = cmsId.replace(/:[0-9]+$/, ':99')
  const row = (dramaId: string, watchedAt: number): HistoryRow => ({
    dramaId, episode: 1, percent: 25, watchedAt, positionMs: 6000, durationMs: 24000, episodeCountAtWatch: 3,
  })
  const now = Date.now()
  const desktop = await cmsLibraryFixture(page, [row(cmsId, now), row(missing, now - 1), row('local-1', now - 2)])
  desktop.catalog.localBookmarks = true
  desktop.catalog.offlineId = cmsId
  desktop.catalog.available = [1]
  await page.goto('/#/home')
  const section = page.getByRole('region', { name: '继续观看' })
  const retry = section.getByRole('button', { name: '重试核验' })
  const local = section.getByTestId('resume-card').filter({ hasText: '本地样片1' })
  const lookups = () => page.evaluate(() => (window as unknown as {
    catalogTest: { lookups: string[] }
  }).catalogTest.lookups)

  await expect(page.getByRole('heading', { name: '片源目录' })).toBeVisible()
  await expect(local).toBeVisible()
  await expect(section.getByRole('list', { name: '片源核验失败' })).toContainText(cmsTitle)
  await expect(retry).toBeEnabled()
  await expect.poll(lookups).toEqual([cmsId, missing])
  await section.screenshot({ path: testInfo.outputPath('cms-home-retry.png') })

  delete desktop.catalog.offlineId
  desktop.catalog.detailDelayMs = 500
  await retry.focus()
  await retry.press('Enter')
  await expect(section.getByRole('button', { name: '正在核验…' })).toBeDisabled()
  await expect(local).toBeVisible()
  const recovered = section.getByTestId('resume-card').filter({ hasText: cmsTitle })
  await expect(recovered).toContainText('25%')
  await expect(retry).toHaveCount(0)
  await expect(section.getByRole('button', { name: '查看更多' })).toBeFocused()
  await expect.poll(lookups).toEqual([cmsId, missing, cmsId])
  expect(await calls(page)).toEqual([])
  await recovered.click()
  await expect(page).toHaveURL(/\/play\/1$/)
  await expect.poll(() => position(historyVideo(page, 1))).toBeCloseTo(6, 2)
})

test('CMS home keeps the history link focused when retry confirms a title is unavailable', async ({ page }) => {
  const desktop = await cmsLibraryFixture(page, [{
    dramaId: cmsId, episode: 1, percent: 25, watchedAt: Date.now(),
    positionMs: 6000, durationMs: 24000, episodeCountAtWatch: 3,
  }])
  desktop.catalog.home = 'empty'
  desktop.catalog.offlineId = cmsId
  await page.goto('/#/home')
  const section = page.getByRole('region', { name: '继续观看' })
  const retry = section.getByRole('button', { name: '重试核验' })
  await expect(section.getByRole('list', { name: '片源核验失败' })).toContainText(cmsId.split(':').at(-1)!)
  await expect(retry).toBeEnabled()

  delete desktop.catalog.offlineId
  desktop.catalog.id = cmsId.replace(/:[0-9]+$/, ':99')
  await retry.focus()
  await retry.press('Enter')
  await expect(section.getByText('暂无可继续观看的短剧')).toBeVisible()
  await expect(retry).toHaveCount(0)
  await expect(section.getByRole('button', { name: '查看更多' })).toBeFocused()
  expect(desktop.rows()[0]).toMatchObject({ dramaId: cmsId, positionMs: 6000 })
  expect(desktop.writes).toHaveLength(0)
})
