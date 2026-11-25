import { readFile } from 'node:fs/promises'
import { expect, test, type Locator, type Page } from '@playwright/test'

const drama = {
  id: 'local-landscape', title: '南风纪事', summary: '自有版权的风景片段', tags: ['风景'],
  license: '作者自有', updatedTo: 2, episodeCount: 2,
}

test('global desktop search suggests and opens active licensed dramas', async ({ page }, testInfo) => {
  const pageErrors: string[] = []
  page.on('pageerror', (error) => pageErrors.push(error.message))
  await page.addInitScript((entry) => {
    const other = { ...entry, id: 'local-river', title: '江畔纪事' }
    localStorage.setItem('jindi.library.v1', JSON.stringify({
      favorites: [], history: [{ dramaId: entry.id, episode: 1, percent: 33, watchedAt: Date.now(), positionSec: 8, durationSec: 24 }],
    }))
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      SaveSearchHistory: async () => undefined,
      LicensedCatalog: async () => [entry, other],
      Search: async (query: string) => [
        ...[entry, other].filter(({ title, tags, summary }) => [title, summary, ...tags].some((value) => value.includes(query))),
        { id: 'd1', title: '替嫁千金' },
      ],
      Episodes: async (id: string) => [{ id: id + '-1', dramaId: id, index: 1, available: true }],
      ResolvePlayback: async (id: string, index: number) => ({
        kind: 'licensed', episodeId: id + '-' + index,
        sources: [{ src: `/_licensed/media/${id}/${index}/clip.mp4`, type: 'video/mp4' }],
      }),
    } } } })
  }, drama)
  await serveLicensedClip(page)
  await page.goto('/#/search')
  const input = page.getByTestId('search-input')
  await input.fill('风景')
  await expect(page.getByRole('option', { name: '南风纪事' })).toBeVisible()
  await input.press('Enter')
  const cards = page.getByTestId('result-card')
  await expect(cards).toHaveCount(3)
  await page.getByTestId('sort-new').click()
  await expect(cards.nth(1)).toContainText('南风纪事')
  await expect(cards.nth(2)).toContainText('江畔纪事')
  await page.getByTestId('status-finished').click()
  await expect(cards.filter({ hasText: '南风纪事' })).toHaveCount(0)
  await page.getByTestId('kind-本地片库').click()
  await expect(cards).toHaveCount(2)
  const local = cards.filter({ hasText: '南风纪事' })
  await expect(local).toContainText('共2集')
  await expect(local).not.toContainText(/万热度|已完结|连载中/)
  await page.screenshot({ path: testInfo.outputPath('licensed-search.png') })
  await local.getByTestId('view-detail').click()
  await expect(page).toHaveURL(/#\/licensed\?id=local-landscape$/)
  await expect(page.getByText('许可说明 · 作者自有')).toBeVisible()
  await page.goBack()
  await local.getByTestId('play-now').click()
  await expect(page).toHaveURL(/#\/licensed\?id=local-landscape&ep=1&autoplay=1$/)
  const video = page.locator('video')
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await expect(video).toHaveJSProperty('paused', false)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(8)
  expect(pageErrors).toEqual([])
})

async function serveLicensedClip(page: Page) {
  const clip = await readFile(new URL('../../public/demo/demo-1.mp4', import.meta.url))
  await page.route('**/_licensed/media/**', (route) => {
    const range = /^bytes=(\d+)-(\d*)$/.exec(route.request().headers().range ?? '')
    const headers = { 'Accept-Ranges': 'bytes', 'Content-Type': 'video/mp4' }
    if (!range) return route.fulfill({ headers, body: clip })
    const begin = Number(range[1])
    const end = Math.min(range[2] ? Number(range[2]) : clip.length - 1, clip.length - 1)
    if (begin > end) return route.fulfill({ status: 416, headers: { ...headers, 'Content-Range': `bytes */${clip.length}` } })
    return route.fulfill({
      status: 206, headers: { ...headers, 'Content-Range': `bytes ${begin}-${end}/${clip.length}` }, body: clip.subarray(begin, end + 1),
    })
  })
}

for (const scenario of ['clamped', 'no-ranges'] as const) {
  test(`licensed automatic resume handles ${scenario} without hanging or overwriting an unopened bookmark`, async ({ page }) => {
    await page.addInitScript(({ entry, scenario }) => {
      const bookmark = { dramaId: entry.id, episode: 1, positionSec: 6, durationSec: 24, percent: 25, watchedAt: Date.now() }
      localStorage.setItem('jindi.library.v1', JSON.stringify({ favorites: [], history: [bookmark] }))
      const prototype = HTMLMediaElement.prototype
      const time = Object.getOwnPropertyDescriptor(prototype, 'currentTime')!
      Object.defineProperty(prototype, 'currentTime', {
        ...time,
        set(this: HTMLMediaElement, value: number) {
          this.dataset.requestedSeek = String(value)
          if (scenario === 'clamped') time.set!.call(this, Math.max(8, value))
        },
      })
      if (scenario === 'no-ranges') {
        Object.defineProperty(prototype, 'seekable', { get: () => ({ length: 0 }) })
        const play = prototype.play
        prototype.play = function () {
          this.muted = true
          return play.call(this)
        }
      } else {
        const seekable = Object.getOwnPropertyDescriptor(prototype, 'seekable')!.get!
        Object.defineProperty(prototype, 'seekable', {
          get(this: HTMLMediaElement) {
            const ranges = seekable.call(this) as TimeRanges
            return ranges.length ? { length: 1, start: () => 8, end: () => ranges.end(0) } : ranges
          },
        })
      }
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LicensedCatalog: async () => [entry],
        Episodes: async () => [{ id: entry.id + '-1', dramaId: entry.id, index: 1, available: true }],
        ResolvePlayback: async () => ({
          kind: 'licensed', episodeId: entry.id + '-1',
          sources: [{ src: `/_licensed/media/${entry.id}/1/clip.mp4`, type: 'video/mp4' }],
        }),
      } } } })
    }, { entry: drama, scenario })
    await serveLicensedClip(page)
    await page.goto(`/#/licensed?id=${drama.id}&ep=1${scenario === 'no-ranges' ? '&autoplay=1' : ''}`)
    const video = page.locator('video')
    await expect(video).toHaveJSProperty('videoWidth', 360)
    if (scenario === 'no-ranges') {
      await expect(video).toHaveJSProperty('paused', false)
      await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(0)
      expect(await video.getAttribute('data-requested-seek')).toBeNull()
    } else {
      await expect(video).toHaveJSProperty('currentTime', 8)
      await expect(video).toHaveJSProperty('seeking', false)
      await page.goto(`/#/licensed?id=${drama.id}`)
      await expect(video).toHaveCount(0)
      const stored = await page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!).history[0])
      expect(stored).toMatchObject({ positionSec: 6, durationSec: 24, percent: 25 })
    }
  })
}

test('licensed playback retires a delayed WebKit picture-in-picture window when the episode changes', async ({ page }) => {
  await page.addInitScript((entry) => {
    const mode = new WeakMap<HTMLVideoElement, string>()
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: false })
    Object.defineProperties(HTMLVideoElement.prototype, {
      webkitPresentationMode: { configurable: true, get(this: HTMLVideoElement) { return mode.get(this) ?? 'inline' } },
      webkitSupportsPresentationMode: { configurable: true, value: () => true },
      webkitSetPresentationMode: { configurable: true, value(this: HTMLVideoElement, next: string) {
        if (next !== 'inline') {
          document.body.dataset.pipRequested = 'true'
          window.addEventListener('finish-pip-enter', () => {
            mode.set(this, next)
            this.dispatchEvent(new Event('webkitpresentationmodechanged'))
          }, { once: true })
          return
        }
        mode.set(this, next)
        this.dispatchEvent(new Event('webkitpresentationmodechanged'))
        document.body.dataset.pipClosed = 'true'
      } },
    })
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      LicensedCatalog: async () => [entry],
      Episodes: async () => [1, 2].map((index) => ({ id: entry.id + '-' + index, dramaId: entry.id, index, available: true })),
      ResolvePlayback: async (id: string, index: number) => ({
        kind: 'licensed', episodeId: id + '-' + index,
        sources: [{ src: `/_licensed/media/${id}/${index}/clip.mp4`, type: 'video/mp4' }],
      }),
    } } } })
  }, drama)
  await serveLicensedClip(page)
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  await expect(page.locator('video')).toHaveJSProperty('videoWidth', 360)
  const button = page.getByRole('button', { name: '画中画', exact: true })
  await button.click()
  await expect(page.locator('body')).toHaveAttribute('data-pip-requested', 'true')
  await page.getByRole('button', { name: '第 2 集' }).click()
  await expect(page.locator('video')).toHaveAttribute('aria-label', '第 2 集播放器')
  await page.evaluate(() => window.dispatchEvent(new Event('finish-pip-enter')))
  await expect(page.locator('body')).toHaveAttribute('data-pip-closed', 'true')
  await expect(button).toHaveAttribute('aria-pressed', 'false')
})

test('desktop browses and plays licensed entries', async ({ page }, testInfo) => {
  await page.addInitScript((entry) => {
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      LicensedCatalog: async () => [entry],
      Episodes: async () => [1, 2].map((index) => ({
        id: entry.id + '-' + index, dramaId: entry.id, index, available: true,
      })),
      ResolvePlayback: async (id: string, index: number) => ({
        kind: 'licensed', episodeId: id + '-' + index,
        sources: [{ src: `/_licensed/media/${id}/${index}/clip.mp4`, type: 'video/mp4' }],
      }),
    } } } })
  }, drama)
  await page.route('**/_licensed/media/**', (route) => route.fulfill({
    path: route.request().url().includes('/2/') ? 'public/demo/demo-2-sd.mp4' : 'public/demo/demo-1.mp4',
    contentType: 'video/mp4',
  }))
  await page.goto('/#/search')
  await page.getByTestId('tab-library').click()
  await expect(page).toHaveURL(/#\/licensed/)
  const search = page.getByRole('searchbox', { name: '搜索本地片库' })
  await search.fill('无匹配')
  await expect(page.getByText('未找到匹配的短剧')).toBeVisible()
  await search.fill('风景')
  await page.getByRole('button', { name: /南风纪事/ }).click()
  await expect(page.getByText('许可说明 · 作者自有')).toBeVisible()
  await page.getByRole('button', { name: '第 1 集' }).click()
  const video = page.locator('video')
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await video.evaluate((media: HTMLVideoElement) => media.play())
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(0.25)
  await page.getByRole('button', { name: '第 2 集' }).click()
  await expect(video).toHaveJSProperty('videoWidth', 240)
  await page.screenshot({ path: testInfo.outputPath('licensed-library.png') })
})

for (const failure of [false, true]) {
  test(failure ? 'desktop reports invalid library configuration' : 'desktop distinguishes an unconfigured library', async ({ page }) => {
    await page.addInitScript((invalid) => {
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LicensedCatalog: async () => {
          if (invalid) throw '授权片库清单格式错误或没有条目'
          return []
        },
      } } } })
    }, failure)
    await page.goto('/#/licensed')
    await expect(page.getByText(failure ? '授权片库清单格式错误或没有条目' : '尚未配置本地片库')).toBeVisible()
    if (failure) await expect(page.getByRole('alert')).toContainText('请重启应用')
  })
}

test('licensed export requires opt-in and sends the selected episode to desktop', async ({ page }, testInfo) => {
  const calls: [string, number][] = []
  await page.exposeFunction('saveLicensedTestEpisode', async (id: string, index: number) => {
    calls.push([id, index])
    if (calls.length === 1) return false
    if (calls.length === 2) throw new Error('目标文件已存在，请使用其他文件名')
    return true
  })
  await page.addInitScript((entry) => {
    const desktop = window as unknown as {
      saveLicensedTestEpisode(id: string, index: number): Promise<boolean>
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      LicensedCatalog: async () => [entry, { ...entry, id: 'local-copy', title: '授权副本', allowExport: true }],
      Episodes: async (id: string) => [1, 2].map((index) => ({ id: id + '-' + index, dramaId: id, index, available: true })),
      ResolvePlayback: async (id: string, index: number) => ({
        kind: 'licensed', episodeId: id + '-' + index,
        sources: [{ src: `/_licensed/media/${id}/${index}/clip.mp4`, type: 'video/mp4' }],
      }),
      ExportLicensedEpisode: desktop.saveLicensedTestEpisode,
    } } } })
  }, drama)
  await serveLicensedClip(page)
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  await expect(page.locator('video')).toHaveJSProperty('videoWidth', 360)
  const save = page.getByRole('button', { name: '保存本集' })
  await expect(save).toHaveCount(0)
  await page.getByRole('button', { name: /授权副本/ }).click()
  await expect(save).toHaveCount(0)
  await page.getByRole('button', { name: '第 2 集' }).click()
  await expect(save).toBeEnabled()
  await expect(page.locator('video')).toHaveJSProperty('videoWidth', 360)
  await page.screenshot({ path: testInfo.outputPath('licensed-export.png') })
  await save.click()
  await expect.poll(() => calls.length).toBe(1)
  await expect(save).toBeEnabled()
  await expect(page.getByText('本地片库视频已保存')).toHaveCount(0)
  await save.click()
  await expect(page.getByText(/目标文件已存在/)).toBeVisible()
  await page.getByRole('button', { name: '第 1 集' }).click()
  await save.click()
  await expect(page.getByText('本地片库视频已保存')).toBeVisible()
  expect(calls).toEqual([['local-copy', 2], ['local-copy', 2], ['local-copy', 1]])
})

type HistoryRow = {
  dramaId: string; episode: number; positionMs: number; durationMs: number; percent: number; watchedAt: number
}

test('licensed favorites and exact resume survive a restart and work from global pages', async ({ page }, testInfo) => {
  const saved = { favorites: [] as string[], history: [] as HistoryRow[] }
  let episodeCount = 2
  await page.exposeFunction('readLicensedTestState', () => ({ ...saved, episodeCount }))
  await page.exposeFunction('saveLicensedTestState', (changes: { favorites?: string[]; history: HistoryRow[]; removed: string[] }) => {
    if (changes.favorites) saved.favorites = changes.favorites
    const replaced = new Set([...changes.removed, ...changes.history.map(({ dramaId }) => dramaId)])
    saved.history = [...changes.history, ...saved.history.filter(({ dramaId }) => !replaced.has(dramaId))]
  })
  await page.addInitScript((entry) => {
    const desktop = window as unknown as {
      readLicensedTestState(): Promise<{ episodeCount: number; favorites: string[]; history: HistoryRow[] }>
      saveLicensedTestState(changes: unknown): Promise<void>
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => true,
      Favorites: async () => (await desktop.readLicensedTestState()).favorites,
      History: async () => (await desktop.readLicensedTestState()).history,
      SearchHistory: async () => [],
      HomeFeed: async () => ({ todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
      GetSetting: async (key: string) => key === 'app' ? '{"startPage":"上次观看"}' : '',
      SaveLibraryChanges: desktop.saveLicensedTestState,
      LicensedCatalog: async () => {
        const { episodeCount } = await desktop.readLicensedTestState()
        return episodeCount ? [{ ...entry, episodeCount, updatedTo: episodeCount }] : []
      },
      Episodes: async () => {
        const { episodeCount } = await desktop.readLicensedTestState()
        return Array.from({ length: episodeCount }, (_, i) => ({
          id: entry.id + '-' + (i + 1), dramaId: entry.id, index: i + 1, available: true,
        }))
      },
      ResolvePlayback: async (id: string, index: number) => ({
        kind: 'licensed', episodeId: id + '-' + index,
        sources: [{ src: `/_licensed/media/${id}/${index}/clip.mp4`, type: 'video/mp4' }],
      }),
    } } } })
  }, drama)
  await serveLicensedClip(page)
  await page.goto('/#/licensed?id=local-landscape&ep=2')
  const video = page.locator('video')
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await page.getByTestId('licensed-library').getByRole('button', { name: '收藏', exact: true }).click()
  await expect.poll(() => saved.favorites).toEqual(['local-landscape'])
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.seekable.length)).toBeGreaterThan(0)
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = 8 })
  await expect(video).toHaveJSProperty('currentTime', 8)
  await expect.poll(() => saved.history[0]?.positionMs).toBe(8000)
  expect(saved.history[0]?.episode).toBe(2)
  await page.getByRole('button', { name: '第 1 集' }).click()
  await expect(video).toHaveAttribute('aria-label', '第 1 集播放器')
  await expect(video).toHaveJSProperty('currentTime', 0)
  expect(saved.history[0]?.episode).toBe(2)

  await page.goto('/#/favorites')
  await page.reload()
  const card = page.getByTestId('fav-card').filter({ hasText: '南风纪事' })
  await expect(card).toBeVisible()
  await card.getByTestId('fav-play').click()
  await expect(page).toHaveURL(/licensed\?id=local-landscape&ep=2/)
  await expect(video).toHaveJSProperty('currentTime', 8)

  await page.goto('/#/history')
  await page.getByTestId('history-item').filter({ hasText: '南风纪事' }).getByTestId('resume').click()
  await expect(video).toHaveJSProperty('currentTime', 8)
  await page.goto('/#/home')
  await page.getByTestId('resume-card').filter({ hasText: '南风纪事' }).click()
  await expect(video).toHaveJSProperty('currentTime', 8)
  await page.goto('/#/')
  await page.reload()
  await expect(page).toHaveURL(/licensed\?id=local-landscape&ep=2/)
  await expect(video).toHaveJSProperty('currentTime', 8)
  const historyPage = page.getByTestId('page-history')
  const tail = await video.evaluate((media: HTMLVideoElement) => media.duration - 0.3)
  await video.evaluate((media: HTMLVideoElement, position) => { media.currentTime = position }, tail)
  await expect.poll(() => saved.history[0]?.positionMs / 1000).toBeCloseTo(tail, 2)
  expect(saved.history[0]?.percent).toBeLessThan(100)
  await video.evaluate((media: HTMLVideoElement) => { media.muted = true; return media.play() })
  await expect.poll(() => saved.history[0]?.percent).toBe(100)
  await page.goto('/#/history')
  await expect(historyPage.getByTestId('done-badge')).toBeVisible()
  await historyPage.getByTestId('done-badge').click()
  await expect(video).toHaveJSProperty('currentTime', 0)

  saved.history[0] = { ...saved.history[0], percent: 50, positionMs: 0, durationMs: 0 }
  await page.reload()
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => Math.abs(media.currentTime - media.duration / 2))).toBeLessThan(0.08)
  saved.history[0] = { ...saved.history[0], percent: 99, positionMs: 23800 }
  await page.goto('/#/')
  await page.reload()
  await expect(page).toHaveURL(/licensed\?id=local-landscape&ep=2/)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(23.8, 1)
  saved.history[0] = { ...saved.history[0], percent: 100, positionMs: 24000 }
  await page.reload()
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await expect(video).toHaveJSProperty('currentTime', 0)

  const validBookmark = { ...saved.history[0], percent: 20, positionMs: 12000, durationMs: 60000 }
  saved.history[0] = validBookmark
  await page.reload()
  await expect(video).toHaveJSProperty('currentTime', 12)
  await expect(video).toHaveJSProperty('seeking', false)

  const oldBookmark = { ...saved.history[0], percent: 67, positionMs: 40000, durationMs: 60000 }
  saved.history[0] = oldBookmark
  await page.reload()
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime), { timeout: 3000 }).toBeCloseTo(16, 1)
  await expect(video).toHaveJSProperty('seeking', false)
  await page.goto('/#/licensed?id=local-landscape')
  await expect(video).toHaveCount(0)
  expect(saved.history[0]).toEqual(oldBookmark)
  await page.goto('/#/licensed?id=local-landscape&ep=2')
  await expect(video).toHaveJSProperty('currentTime', 16)
  await expect(video).toHaveJSProperty('seeking', false)
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = 18 })
  await expect.poll(() => saved.history[0]).toMatchObject({ positionMs: 18000, durationMs: 24000 })

  saved.history[0] = { ...saved.history[0], percent: 75, positionMs: 40000, durationMs: 0 }
  await page.reload()
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(18, 1)

  await page.goto('/#/licensed?id=local-landscape&ep=99')
  await expect(page.getByRole('alert')).toContainText('该分集已不可用')
  await expect(video).toHaveCount(0)
  await page.goto('/#/licensed?id=local-landscape&ep=')
  await expect(page.getByRole('alert')).toContainText('该分集已不可用')
  await page.goto('/#/licensed?id=local-landscape')
  await expect(page.getByRole('alert')).toHaveCount(0)

  saved.history[0] = validBookmark
  episodeCount = 1
  await page.goto('/#/favorites')
  await page.reload()
  await expect(card).toContainText('上次观看的分集不可用')
  await card.getByRole('button', { name: '选集' }).click()
  await expect(page).toHaveURL(/licensed\?id=local-landscape$/)
  await expect(page.getByRole('button', { name: '第 1 集' })).toBeVisible()
  await expect(video).toHaveCount(0)

  await page.goto('/#/history')
  const shorter = historyPage.getByTestId('history-item').filter({ hasText: '南风纪事' })
  await expect(shorter).toContainText('该分集不可用')
  await expect(historyPage.getByTestId('continue-latest')).toBeDisabled()
  await shorter.getByRole('button', { name: '选集' }).click()
  await expect(page).toHaveURL(/licensed\?id=local-landscape$/)
  expect(saved.history).toEqual([validBookmark])

  episodeCount = 0
  await page.goto('/#/favorites')
  await page.reload()
  const unavailable = page.getByTestId('fav-card').filter({ hasText: '本地短剧 · landscape' })
  await expect(unavailable).toContainText('本地媒体不可用')
  await expect(unavailable).not.toContainText('0集')
  await page.screenshot({ path: testInfo.outputPath('unavailable-favorites.png') })
  await unavailable.getByTestId('fav-play').click()
  await expect(page).toHaveURL(/#\/licensed$/)
  await expect(page.getByText('尚未配置本地片库')).toBeVisible()
  await page.goto('/#/history')
  const history = page.getByTestId('history-item').filter({ hasText: '本地短剧 · landscape' })
  await expect(history).toContainText('本地媒体不可用')
  await expect(history).not.toContainText('共0集')
  await page.screenshot({ path: testInfo.outputPath('unavailable-history.png') })
  await history.getByRole('button', { name: '查看片库' }).click()
  await expect(page).toHaveURL(/#\/licensed$/)
  expect(saved.history).toEqual([validBookmark])

  episodeCount = 2
  await page.goto('/#/history')
  await page.reload()
  await historyPage.getByTestId('resume').click()
  await expect(page).toHaveURL(/licensed\?id=local-landscape&ep=2$/)
  await expect(video).toHaveJSProperty('currentTime', 12)

  episodeCount = 0
  await page.goto('/#/favorites')
  await page.reload()
  await unavailable.getByTestId('fav-unfav').click()
  await expect.poll(() => saved.favorites).toEqual([])
  await page.goto('/#/history')
  await expect(history).toBeVisible()
  await history.getByTestId('h-more').click()
  await history.getByTestId('h-delete').click()
  await expect.poll(() => saved.history).toEqual([])
})

test('startup skips a licensed episode whose media cannot be opened', async ({ page }) => {
  await page.addInitScript((entry) => {
    const older = { ...entry, id: 'local-older', title: '可播放剧集' }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => true,
      Favorites: async () => [],
      History: async () => [entry, older].map((item, i) => ({
        dramaId: item.id, episode: 1, positionMs: 5000, durationMs: 24000, percent: 20, watchedAt: Date.now() - i * 1000,
      })),
      SearchHistory: async () => [],
      GetSetting: async (key: string) => key === 'app' ? '{"startPage":"上次观看"}' : '',
      LicensedCatalog: async () => [entry, older],
      Episodes: async (id: string) => [{ id: id + '-1', dramaId: id, index: 1, available: true }],
      ResolvePlayback: async (id: string) => ({
        kind: 'licensed', episodeId: id + '-1',
        sources: [{ src: `/_licensed/media/${id}/1/clip.mp4`, type: 'video/mp4' }],
      }),
    } } } })
  }, drama)
  await page.route('**/_licensed/media/local-landscape/**', (route) => route.fulfill({ status: 404 }))
  await page.route('**/_licensed/media/local-older/**', (route) => route.fulfill({
    path: 'public/demo/demo-1.mp4', contentType: 'video/mp4',
  }))
  await page.goto('/#/')
  await expect(page).toHaveURL(/licensed\?id=local-older&ep=1/)
  await expect(page.locator('video')).toHaveJSProperty('videoWidth', 360)
})

async function setupPlaybackPreferences(page: Page, mode: string, { locked = false, blockedAutoplay = false, delayedEpisodes = false, blockedLoop = false } = {}) {
  await page.addInitScript(({ entry, playMode, locked, blockedAutoplay, delayedEpisodes, blockedLoop }) => {
    if (!localStorage.getItem('jindi.settings.v1')) {
      localStorage.setItem('jindi.settings.v1', JSON.stringify({ playMode, defaultSpeed: 1.75 }))
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      CacheStats: async () => ({ bytes: 0, max: 1024, path: '/tmp', files: 0 }),
      AppVersion: async () => 'v0.1.0',
      AutostartEnabled: async () => false,
      SaveSetting: async () => {},
      LicensedCatalog: async () => [entry],
      Episodes: async () => {
        if (delayedEpisodes) {
          await new Promise<void>((resolve) => { Object.assign(window, { releaseTestEpisodes: resolve }) })
        }
        return [1, 2].map((index) => ({
          id: entry.id + '-' + index, dramaId: entry.id, index, available: !(locked && index === 2),
        }))
      },
      ResolvePlayback: async (id: string, index: number) => ({
        kind: 'licensed', episodeId: id + '-' + index,
        sources: [{ src: `/_licensed/media/${id}/${index}/clip.mp4`, type: 'video/mp4' }],
      }),
    } } } })
    if (blockedAutoplay || blockedLoop) {
      const play = HTMLMediaElement.prototype.play
      HTMLMediaElement.prototype.play = function () {
        const blockedNext = blockedAutoplay && this.currentSrc.includes('/2/') && this.paused && !this.muted
        if (blockedNext || (blockedLoop && this.currentTime < 0.1)) {
          return Promise.reject(new DOMException('Autoplay blocked', 'NotAllowedError'))
        }
        return play.call(this)
      }
    }
  }, { entry: drama, playMode: mode, locked, blockedAutoplay, delayedEpisodes, blockedLoop })
  await serveLicensedClip(page)
}

async function finishLicensedEpisode(video: Locator) {
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.seekable.length)).toBeGreaterThan(0)
  await video.evaluate((media: HTMLVideoElement) => {
    media.currentTime = media.duration - 0.3
    return media.play()
  })
}

test('licensed playback uses the Settings speed and keeps a native speed choice after reloading', async ({ page }) => {
  await setupPlaybackPreferences(page, '单集播放')
  await page.goto('/#/settings')
  await page.getByTestId('default-speed').fill('1.25')
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  const video = page.locator('video')
  await expect(video).toHaveJSProperty('playbackRate', 1.25)
  await expect(video).toHaveJSProperty('paused', true)
  await video.evaluate((media: HTMLVideoElement) => { media.playbackRate = 1.5 })
  await expect.poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('jindi.player.v1') ?? '{}').speed)).toBe(1.5)
  await page.reload()
  await expect(video).toHaveJSProperty('playbackRate', 1.5)
})

for (const mode of ['单集播放', '单集循环', '连续播放（自动下一集）']) {
  test(`licensed playback finishes with ${mode}`, async ({ page }) => {
    await setupPlaybackPreferences(page, mode)
    await page.goto('/#/licensed?id=local-landscape&ep=1')
    const video = page.locator('video')
    await expect(video).toHaveJSProperty('playbackRate', 1.75)
    await finishLicensedEpisode(video)
    if (mode === '单集播放') {
      await expect(video).toHaveJSProperty('ended', true)
      await expect(page).toHaveURL(/ep=1$/)
    } else if (mode === '单集循环') {
      await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeLessThan(3)
      await expect(video).toHaveJSProperty('paused', false)
      await expect(page).toHaveURL(/ep=1$/)
    } else {
      await expect(page).toHaveURL(/ep=2$/)
      await expect(video).toHaveJSProperty('paused', false)
      await expect(video).toHaveJSProperty('playbackRate', 1.75)
      await finishLicensedEpisode(video)
      await expect(video).toHaveJSProperty('ended', true)
      await expect(page).toHaveURL(/ep=2$/)
    }
  })
}

test('licensed continuous playback stays on the ended video if the next episode is unavailable', async ({ page }) => {
  await setupPlaybackPreferences(page, '连续播放（自动下一集）', { locked: true })
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  await expect(page.getByRole('button', { name: '第 2 集' })).toBeDisabled()
  const video = page.locator('video')
  await finishLicensedEpisode(video)
  await expect(video).toHaveJSProperty('ended', true)
  await expect(page).toHaveURL(/ep=1$/)
})

test('licensed next episode offers manual playback when autoplay is denied', async ({ page }, testInfo) => {
  await setupPlaybackPreferences(page, '连续播放（自动下一集）', { blockedAutoplay: true })
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  const video = page.locator('video')
  await finishLicensedEpisode(video)
  await expect(page).toHaveURL(/ep=2$/)
  await expect(page.getByTestId('licensed-library').getByRole('status')).toContainText('已切换到下一集，请点击播放。')
  await expect(video).toHaveJSProperty('paused', true)
  await page.screenshot({ path: testInfo.outputPath('autoplay-paused.png') })
  await video.evaluate((media: HTMLVideoElement) => { media.muted = true; return media.play() })
  await expect(page.getByText('已切换到下一集，请点击播放。')).toHaveCount(0)
})

test('licensed continuous playback returns to the completed episode if the next file fails', async ({ page }) => {
  await setupPlaybackPreferences(page, '连续播放（自动下一集）')
  await page.route('**/_licensed/media/local-landscape/2/**', (route) => route.fulfill({ status: 404 }))
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  const video = page.locator('video')
  await finishLicensedEpisode(video)
  await expect(page.getByText('下一集媒体无法播放，已停在本集结束处。')).toBeVisible()
  await expect(page).toHaveURL(/ep=1$/)
  await expect(video).toHaveJSProperty('paused', true)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.duration - media.currentTime)).toBeLessThan(0.1)
  const entry = await page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!).history[0])
  expect(entry).toMatchObject({ episode: 1, percent: 100 })
})

test('licensed continuous playback waits for the episode list before advancing', async ({ page }) => {
  await setupPlaybackPreferences(page, '连续播放（自动下一集）', { delayedEpisodes: true })
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  const video = page.locator('video')
  await finishLicensedEpisode(video)
  await expect(video).toHaveJSProperty('ended', true)
  await expect(page).toHaveURL(/ep=1$/)
  await page.evaluate(() => (window as unknown as { releaseTestEpisodes(): void }).releaseTestEpisodes())
  await expect(page).toHaveURL(/ep=2$/)
  await expect(video).toHaveJSProperty('paused', false)
})

test('denied single-episode replay keeps history completed when the user leaves', async ({ page }) => {
  await setupPlaybackPreferences(page, '单集循环', { blockedLoop: true })
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  await finishLicensedEpisode(page.locator('video'))
  await expect(page.getByText('请点击播放重新开始本集。')).toBeVisible()
  await page.getByTestId('nav-history').click()
  await expect(page.getByTestId('history-item')).toBeVisible()
  const entry = await page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!).history[0])
  expect(entry).toMatchObject({ episode: 1, percent: 100 })
})

test('ending licensed playback does not interrupt a pending page navigation', async ({ page }) => {
  await setupPlaybackPreferences(page, '连续播放（自动下一集）')
  let releaseSettings!: () => void
  const holdSettings = new Promise<void>((resolve) => { releaseSettings = resolve })
  let settingsRequested!: () => void
  const requestStarted = new Promise<void>((resolve) => { settingsRequested = resolve })
  await page.route('**/assets/SettingsView-*.js', async (route) => {
    settingsRequested()
    await holdSettings
    await route.continue()
  })
  await page.goto('/#/licensed?id=local-landscape&ep=1')
  const video = page.locator('video')
  try {
    await page.getByTestId('nav-settings').click()
    await requestStarted
    await finishLicensedEpisode(video)
    await expect(video).toHaveJSProperty('ended', true)
    await expect(page).toHaveURL(/ep=1$/)
  } finally {
    releaseSettings()
  }
  await expect(page).toHaveURL(/#\/settings$/)
})
