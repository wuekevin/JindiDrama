import fs from 'node:fs'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'

const id = 'catalog:qa/001'
const route = '/#/catalog/' + encodeURIComponent(id)
const mediaURL = 'https://media.example.test'
const samples = {
  webm: fs.readFileSync(new URL('../../public/demo/demo-1.webm', import.meta.url)),
  mp4: fs.readFileSync(new URL('../../public/demo/demo-1.mp4', import.meta.url)),
}

async function fixture(page: Page, mode: 'ready' | 'blocked' | 'stale' | 'retry' | 'expires-live' | 'protected') {
  await page.addInitScript(({ id, mediaURL, mode }) => {
    let attempt = 0
    const calls: string[] = []
    const testState = { calls, release: () => {} }
    Object.assign(window, { catalogTest: testState, go: { app: { App: {
      StorageReady: async () => false,
      ProviderName: async () => 'authorized-fixture',
      Capabilities: async () => ({ catalog: 'ready', playback: mode === 'blocked' ? 'not_authorized' : 'ready' }),
      CatalogDrama: async () => ({ id, title: '远海来信', tags: [], summary: '授权播放测试目录', updatedTo: 2, episodeCount: 3 }),
      CatalogEpisodes: async () => [1, 2, 3].map((index) => ({ id: 'episode-' + index, dramaId: id, index, available: index < 3 })),
      CatalogPlayback: async (_id: string, index: number) => {
        calls.push('catalog:' + _id + ':' + index)
        attempt++
        if (mode === 'stale' && index === 1) await new Promise<void>((resolve) => { testState.release = resolve })
        const expiresAt = mode === 'retry' && attempt === 1 ? '2001-01-01T00:00:00Z'
          : new Date(Date.now() + (mode === 'expires-live' ? 3_000 : 120_000)).toISOString()
        const format = index === 2 ? 'mp4' : 'webm'
        const sources = [
          { src: mediaURL + '/episode-' + index + '.' + format + '?ticket=opaque-test', type: 'video/' + format },
        ]
        if (mode === 'expires-live') sources.push({ src: mediaURL + '/fallback.webm', type: 'video/webm' })
        return { kind: 'authorized', episodeId: 'episode-' + index, expiresAt, sources,
          restrictions: mode === 'protected' ? ['no-download'] : [] }
      },
      ResolvePlayback: async () => { calls.push('legacy') },
      SaveLibraryChanges: async () => { calls.push('library') },
    } } } })
  }, { id, mediaURL, mode })
  const requests: string[] = []
  await page.route(mediaURL + '/**', async (interception) => {
    const requestURL = interception.request().url()
    requests.push(requestURL)
    const format = new URL(requestURL).pathname.endsWith('.mp4') ? 'mp4' : 'webm'
    const sample = samples[format]
    const range = /^bytes=(\d+)-(\d*)$/.exec(interception.request().headers()['range'] ?? '')
    const headers = { 'content-type': 'video/' + format, 'access-control-allow-origin': '*', 'accept-ranges': 'bytes' }
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
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain('opaque-test')
})

test('episode change discards the earlier ticket before any stale media request', async ({ page }) => {
  const requests = await fixture(page, 'stale')
  await page.goto(route + '/play/1')
  await expect.poll(() => calls(page)).toEqual(['catalog:' + id + ':1'])
  await page.getByRole('button', { name: '第2集' }).click()
  await expect(page.getByLabel('第2集视频')).toHaveAttribute('src', /episode-2/)
  await page.evaluate(() => (window as Window & { catalogTest?: { release(): void } }).catalogTest?.release())
  await expect.poll(() => requests.length).toBeGreaterThan(0)
  await expect(page.getByLabel('第2集视频')).toHaveAttribute('src', /episode-2/)
  await expect.poll(() => page.getByLabel('第2集视频').evaluate((v) => (v as HTMLVideoElement).duration)).toBeGreaterThan(10)
  expect(requests.every((url) => url.includes('episode-2'))).toBe(true)
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
  await expect(page.getByLabel('第1集视频')).toHaveAttribute('src', /episode-1/)
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
  expect(requests.every((url) => url.includes('episode-1'))).toBe(true)
})

test('a protected ticket does not expose a stream to the browser', async ({ page }) => {
  const requests = await fixture(page, 'protected')
  await page.goto(route + '/play/1')
  await expect(page.getByTestId('error')).toContainText('无法取得可用的播放源')
  await expect(page.locator('video')).toHaveCount(0)
  expect(requests).toEqual([])
})
