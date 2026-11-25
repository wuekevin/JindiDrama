import { readFile } from 'node:fs/promises'
import { test, expect } from '@playwright/test'

const t = (page: import('@playwright/test').Page) => page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).currentTime)

test('quick switches and unavailable episodes preserve the real playback bookmark', async ({ page }) => {
  const bookmark = { dramaId: 'd2', episode: 80, percent: 99, positionSec: 23.8, durationSec: 24, watchedAt: Date.now() - 8 * 86_400_000 }
  await page.addInitScript((saved) => localStorage.setItem('jindi.library.v1', JSON.stringify({ favorites: [], history: [saved] })), bookmark)
  const state = page.getByTestId('player')
  const saved = () => page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!).history[0])
  await page.goto('/#/watch/d2/79?paused')
  await expect(state).toHaveAttribute('data-state', 'paused')
  await page.goto('/#/watch/d2/81?paused')
  await expect(page.getByTestId('player-error')).toContainText('第81集尚未更新')
  expect(await saved()).toEqual(bookmark)
  await page.goto('/#/watch/d2/78?paused')
  await expect(state).toHaveAttribute('data-state', 'resolving')
  await page.goto('/#/watch/d2/80?paused')
  await expect(state).toHaveAttribute('data-state', 'paused')
  await expect.poll(() => t(page)).toBeCloseTo(23.8, 1)
  await expect(page.getByTestId('video')).toHaveJSProperty('seeking', false)
  await page.getByTestId('player-back').click()
  expect(await saved()).toEqual(bookmark)
  await page.getByTestId('nav-history').click()
  const row = page.getByTestId('history-item').filter({ hasText: '暴君的替身医妃' })
  await expect(row.getByTestId('resume')).toBeVisible()
  await expect(row.getByTestId('done-badge')).toHaveCount(0)
  await page.screenshot({ path: '../design/rendered/08-末集未看完.png' })

  await page.goto('/#/watch/d2/80?paused')
  await expect(state).toHaveAttribute('data-state', 'paused')
  await page.goto('/#/watch/d2/80?paused&t=0')
  await expect(state).toHaveAttribute('data-state', 'paused')
  await expect.poll(() => t(page)).toBe(0)
  await state.focus()
  await page.keyboard.press('ArrowRight')
  await expect.poll(() => t(page)).toBeCloseTo(10, 1)
  await page.keyboard.press('ArrowLeft')
  await expect.poll(() => t(page)).toBe(0)
  const durationSec = await page.getByTestId('video').evaluate((media: HTMLVideoElement) => media.duration)
  await page.getByTestId('player-back').click()
  await expect.poll(saved).toMatchObject({ episode: 80, positionSec: 0, durationSec })

  await page.goto('/#/watch/d2/80?paused&t=12')
  await expect(state).toHaveAttribute('data-state', 'paused')
  await state.press('ArrowRight')
  await state.press('ArrowRight')
  await expect.poll(() => t(page)).toBeGreaterThan(durationSec - 0.05)
  await expect(state).toHaveAttribute('data-state', 'paused')
  await page.getByTestId('player-back').click()
  await expect.poll(saved).toMatchObject({ episode: 80, percent: 99, durationSec })
})

test('opens the second season directly and resumes its real video from a saved position', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('jindi.library.v1', JSON.stringify({
    favorites: [],
    history: [{ dramaId: 'd17', episode: 2, percent: 25, positionSec: 6, durationSec: 24, watchedAt: Date.now() }],
  })))
  await page.goto('/#/drama/d17')
  await expect(page.locator('[data-testid=season][aria-current=page]')).toContainText('第2季')
  await page.getByTestId('continue').click()
  await expect(page).toHaveURL(/#\/watch\/d17\/2$/)
  await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing')
  await expect.poll(() => t(page), { timeout: 3000 }).toBeGreaterThan(5.7)
  await expect.poll(() => page.getByTestId('video').evaluate((el: HTMLVideoElement) => el.currentSrc)).toMatch(/\/demo-3\./)
})

test.describe('P1 真实演示媒体播放', () => {
  test.beforeEach(async ({ page }) => { await page.addInitScript(() => localStorage.removeItem('jindi.library.v1')) })
  test('loads real media, plays, time advances, duration is real', async ({ page }) => {
    await page.goto('/#/watch/d1/32?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    const d = await page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).duration); expect(d).toBeGreaterThan(23); expect(d).toBeLessThan(25)
    await page.getByTestId('toggle').click()
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing')
    const t0 = await t(page)
    await expect.poll(() => t(page), { timeout: 5000 }).toBeGreaterThan(t0 + 0.5)
  })
  test('keeps the same real media across watch, comments and danmaku layouts', async ({ page }, testInfo) => {
    await page.goto('/#/watch/d1/7?t=6&paused=1')
    const video = page.getByTestId('video')
    const player = page.getByTestId('player')
    await expect(player).toHaveAttribute('data-state', 'paused')
    await expect.poll(() => t(page)).toBeGreaterThan(5.5)
    const pausedAt = await t(page)
    await video.evaluate((element: HTMLVideoElement) => { element.dataset.panelContinuity = 'original' })
    await page.getByTestId('speed').click()
    await page.getByTestId('speed-2').click()
    await page.getByTestId('volume').evaluate((input: HTMLInputElement) => {
      input.value = '35'
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    const checkMedia = async (paused: boolean) => {
      await expect(video).toHaveAttribute('data-panel-continuity', 'original')
      await expect(player).toHaveAttribute('data-state', paused ? 'paused' : 'playing')
      expect(await video.evaluate((element: HTMLVideoElement) => ({
        paused: element.paused, rate: element.playbackRate, volume: element.volume,
      }))).toEqual({ paused, rate: 2, volume: 0.35 })
    }

    await page.getByTestId('fullscreen').click()
    await expect.poll(() => page.evaluate(() => document.fullscreenElement?.getAttribute('data-testid'))).toBe('player')
    await page.getByTestId('quality').click()
    await expect(page.getByTestId('q-高清')).toBeVisible()
    await page.getByTestId('to-comments').click()
    await expect(page.getByTestId('page-comments')).toBeVisible()
    await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBeNull()
    await expect(page.getByTestId('q-高清')).toHaveCount(0)
    await checkMedia(true)
    expect(await t(page)).toBeCloseTo(pausedAt, 1)
    await page.screenshot({ path: testInfo.outputPath('comments-video-continuity.png') })

    await page.getByTestId('comment-close').click()
    await expect(page.getByTestId('page-watch')).toBeVisible()
    await checkMedia(true)
    expect(await t(page)).toBeCloseTo(pausedAt, 1)

    await page.getByTestId('open-danmaku-settings').click()
    await expect(page.getByTestId('page-danmaku')).toBeVisible()
    await checkMedia(true)
    expect(await t(page)).toBeCloseTo(pausedAt, 1)
    await page.getByTestId('danmaku-back').click()
    await expect(page.getByTestId('page-watch')).toBeVisible()
    await checkMedia(true)

    await page.getByTestId('to-comments').click()
    const beforeContinue = await t(page)
    await page.getByTestId('continue').click()
    await expect(page.getByTestId('page-watch')).toBeVisible()
    await expect(page).not.toHaveURL(/paused/)
    await checkMedia(false)
    expect(await t(page)).toBeGreaterThanOrEqual(beforeContinue - 0.05)
    await expect.poll(() => t(page)).toBeGreaterThan(beforeContinue + 0.35)

    await page.getByTestId('to-comments').click()
    await expect(page.getByTestId('page-comments')).toBeVisible()
    await checkMedia(false)
    const beforeDanmaku = await t(page)
    await page.getByTestId('open-danmaku-settings').click()
    await expect(page.getByTestId('page-danmaku')).toBeVisible()
    await checkMedia(false)
    expect(await t(page)).toBeGreaterThanOrEqual(beforeDanmaku - 0.05)
    await expect.poll(() => t(page)).toBeGreaterThan(beforeDanmaku + 0.2)
    await page.getByTestId('danmaku-back').click()
    await expect(page.getByTestId('page-watch')).toBeVisible()
    await checkMedia(false)
  })
  test('requests real video picture-in-picture and closes it on navigation', async ({ page }) => {
    await page.addInitScript(() => {
      let active: HTMLVideoElement | null = null
      Object.defineProperties(document, {
        pictureInPictureEnabled: { value: true },
        pictureInPictureElement: { get: () => active },
        exitPictureInPicture: { value: async () => {
          const previous = active
          active = null
          previous?.dispatchEvent(new Event('leavepictureinpicture'))
        } },
      })
      Object.defineProperty(HTMLVideoElement.prototype, 'requestPictureInPicture', { value: async function (this: HTMLVideoElement) {
        active = this
        this.dispatchEvent(new Event('enterpictureinpicture'))
      } })
    })
    await page.goto('/#/watch/d1/7?paused')
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
    const toggle = page.getByTestId('pip')
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-pressed', 'true')
    expect(await page.evaluate(() => document.pictureInPictureElement === document.querySelector('video'))).toBe(true)
    await page.getByTestId('to-comments').click()
    await expect(page.getByTestId('page-comments')).toBeVisible()
    await expect(toggle).toHaveText('退出画中画')
    await page.getByTestId('comment-close').click()
    await expect(page.getByTestId('page-watch')).toBeVisible()
    await expect(toggle).toHaveAttribute('aria-pressed', 'true')
    const video = page.getByTestId('video')
    await video.evaluate((media: HTMLVideoElement) => media.play())
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing')
    await video.evaluate((media: HTMLVideoElement) => media.pause())
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
    await page.evaluate(() => document.exitPictureInPicture())
    await expect(toggle).toHaveAttribute('aria-pressed', 'false')
    await toggle.click()
    await expect(toggle).toHaveText('退出画中画')
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-pressed', 'false')
    await toggle.click()
    await page.getByTestId('next-ep').click()
    await expect(page).toHaveURL(/#\/watch\/d1\/8$/)
    expect(await page.evaluate(() => document.pictureInPictureElement)).toBeNull()
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', /playing|paused/)
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-pressed', 'true')
    await page.getByTestId('player-back').click()
    await expect(page).toHaveURL(/#\/drama\/d1$/)
    expect(await page.evaluate(() => document.pictureInPictureElement)).toBeNull()
  })
  test('seek by keys / click and playbackRate follows speed menu', async ({ page }) => {
    await page.goto('/#/watch/d1/7?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    await page.getByTestId('player').focus(); await page.keyboard.press('ArrowRight'); expect(await t(page)).toBeGreaterThan(9)
    await page.keyboard.press('ArrowLeft'); expect(await t(page)).toBeLessThan(1)
    const box = (await page.getByTestId('seek').boundingBox())!; await page.mouse.click(box.x + box.width * 0.5, box.y + 2); expect(Math.abs((await t(page)) - 12)).toBeLessThan(1.5)
    await page.getByTestId('speed').click(); await page.getByTestId('speed-2').click(); expect(await page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).playbackRate)).toBe(2)
  })
  test('screenshot shortcut saves a PNG of the decoded video frame', async ({ page }, testInfo) => {
    await page.goto('/#/watch/d1/7?paused')
    const player = page.getByTestId('player')
    await expect(player).toHaveAttribute('data-state', 'paused')
    await expect.poll(() => page.getByTestId('video').evaluate((v: HTMLVideoElement) => v.readyState)).toBeGreaterThanOrEqual(2)
    await page.screenshot({ path: testInfo.outputPath('player-screenshot-control.png') })
    await player.focus()
    const [download] = await Promise.all([page.waitForEvent('download'), page.keyboard.press('Control+s')])
    expect(download.suggestedFilename()).toBe('JindiDrama-d1-7.png')
    const data = await readFile((await download.path())!)
    expect(data.subarray(0, 8).toString('hex')).toBe('89504e470d0a1a0a')
    expect([data.readUInt32BE(16), data.readUInt32BE(20)]).toEqual([360, 640])
  })
  test('ended state offers replay', async ({ page }) => {
    await page.goto('/#/watch/d1/32?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    await page.evaluate(() => { const v = document.querySelector('video') as HTMLVideoElement; v.currentTime = v.duration - 0.3; return v.play() }); await expect(page.getByTestId('ended')).toBeVisible({ timeout: 8000 })
    await page.getByTestId('ended').getByText('重新播放').click(); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing')
  })
  test('episode switch swaps source and cancels stale load', async ({ page }) => {
    await page.goto('/#/watch/d1/21?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    const src1 = await page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).currentSrc)
    await page.getByTestId('ep').nth(1).click(); await page.getByTestId('ep').nth(2).click() // 快速连点：只保留最后一次
    await expect(page).toHaveURL(/watch\/d1\/23/); await expect(page.getByTestId('player')).toHaveAttribute('data-state', /playing|paused/, { timeout: 10000 })
    expect(await page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).currentSrc)).not.toBe(src1)
  })
  test('resume: leaving saves exact position, re-entering restores it', async ({ page }) => {
    await page.goto('/#/watch/d2/5?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    await page.getByTestId('player').focus(); await page.keyboard.press('ArrowRight'); await page.keyboard.press('ArrowRight'); const saved = await t(page); expect(saved).toBeGreaterThan(19)
    await page.getByTestId('player-back').click(); await expect(page).toHaveURL(/#\/drama\/d2/)
    await expect.poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!).history.find((x: any) => x.dramaId === 'd2').episode)).toBe(5)
    const h = await page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!).history.find((x: any) => x.dramaId === 'd2')); expect(Math.abs(h.positionSec - saved)).toBeLessThan(0.5)
    await page.goto('/#/watch/d2/5?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 }); expect(Math.abs((await t(page)) - saved)).toBeLessThan(1)
  })
  test('failure: auto retry with backoff then error; manual retry recovers', async ({ page }) => {
    let blocked = true; let hits = 0
    await page.route('**/demo/demo-*', (r) => { hits++; return blocked ? r.abort() : r.continue() })
    await page.goto('/#/watch/d1/5?paused'); await expect(page.getByTestId('player-error')).toBeVisible({ timeout: 15000 }); await expect(page.getByTestId('player-error')).toContainText('已自动重试'); expect(hits).toBeGreaterThanOrEqual(3)
    blocked = false; await page.getByTestId('retry').click(); await expect(page.getByTestId('player')).toHaveAttribute('data-state', /playing|paused/, { timeout: 10000 })
  })
  test('settings: switching 播放源 to 模拟画面 removes <video>', async ({ page }) => {
    await page.goto('/#/settings'); await page.getByTestId('media-source').selectOption('sim'); await page.goto('/#/watch/d1/32?paused'); await expect(page.getByTestId('player')).toBeVisible(); await expect(page.locator('video')).toHaveCount(0)
  })
})

for (const { preference, media, outcome } of [
  { preference: '连续播放（自动下一集）', media: 'demo', outcome: 'next' },
  { preference: '单集播放', media: 'demo', outcome: 'ended' },
  { preference: '单集循环', media: 'demo', outcome: 'loop' },
  { preference: '连续播放（自动下一集）', media: 'sim', outcome: 'next' },
  { preference: '单集播放', media: 'sim', outcome: 'ended' },
  { preference: '单集循环', media: 'sim', outcome: 'loop' },
]) {
  test(`playback ending follows ${preference} in ${media} mode`, async ({ page }) => {
    await page.addInitScript(({ preference, media }) => {
      localStorage.setItem('jindi.settings.v1', JSON.stringify({ playMode: preference, mediaSource: media, defaultSpeed: 2 }))
    }, { preference, media })
    const start = media === 'sim' ? 900 : 23.7
    await page.goto(`/#/watch/d1/31?paused&t=${start}`)
    const player = page.getByTestId('player')
    await expect(player).toHaveAttribute('data-state', 'paused')
    await page.getByTestId('toggle').click()
    if (outcome === 'next') {
      await expect(page).toHaveURL(/#\/watch\/d1\/32$/)
      await expect(player).toHaveAttribute('data-state', 'playing')
    } else if (outcome === 'ended') {
      await expect(page.getByTestId('ended')).toBeVisible()
      await expect(page).toHaveURL(/#\/watch\/d1\/31\?/)
    } else {
      await expect.poll(async () => media === 'demo'
        ? (await t(page)) < 8
        : (await player.getByTestId('seek').getAttribute('aria-valuenow')) === '0').toBe(true)
      await expect(player).toHaveAttribute('data-state', 'playing')
      await expect(page).toHaveURL(/#\/watch\/d1\/31\?/)
    }
  })
}

test('default speed setting applies to the next video and manual speed still persists', async ({ page }) => {
  await page.goto('/#/settings')
  await page.getByTestId('default-speed').fill('1.75')
  await page.goto('/#/watch/d1/32?paused')
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect(page.locator('video')).toHaveJSProperty('playbackRate', 1.75)
  await expect(page.getByTestId('speed')).toContainText('1.75x')
  await page.getByTestId('speed').click()
  await page.getByTestId('speed-1.5').click()
  await page.reload()
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect(page.locator('video')).toHaveJSProperty('playbackRate', 1.5)
})

test('changing quality switches the decoded resolution and preserves playhead and playback state', async ({ page }, testInfo) => {
  await page.goto('/#/watch/d1/31?paused&t=8')
  const player = page.getByTestId('player')
  const video = page.locator('video')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await expect(video).toHaveJSProperty('videoHeight', 640)

  await page.getByTestId('quality').click()
  await expect(page.getByTestId('q-流畅')).toHaveText('流畅 240×426')
  await expect(page.getByTestId('q-高清')).toHaveText('高清 360×640')
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState >= 2 && !media.seeking)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('quality-menu.png') })
  await page.getByTestId('q-流畅').click()
  await expect(video).toHaveJSProperty('videoWidth', 240)
  await expect(video).toHaveJSProperty('videoHeight', 426)
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect(video).toHaveJSProperty('paused', true)
  expect(Math.abs((await t(page)) - 8)).toBeLessThan(0.5)

  await page.getByTestId('toggle').click()
  await expect.poll(() => t(page)).toBeGreaterThan(8.1)
  const beforeSwitch = await t(page)
  await page.getByTestId('quality').click()
  await page.getByTestId('q-高清').click()
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await expect(player).toHaveAttribute('data-state', 'playing')
  await expect(video).toHaveJSProperty('paused', false)
  await expect.poll(() => t(page)).toBeGreaterThan(beforeSwitch)
})

test('default quality selects the video and a manual choice persists after reload', async ({ page }) => {
  await page.goto('/#/settings')
  await page.getByTestId('default-quality').selectOption('流畅')
  await page.goto('/#/watch/d1/32?paused')
  const video = page.locator('video')
  await expect(video).toHaveJSProperty('videoWidth', 240)
  await page.getByTestId('quality').click()
  await page.getByTestId('q-高清').click()
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await page.reload()
  await expect(video).toHaveJSProperty('videoWidth', 360)
})

test('switching quality during resolution ignores old metadata and retains legacy resume progress', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('jindi.library.v1', JSON.stringify({
    favorites: [],
    history: [{ dramaId: 'd1', episode: 31, percent: 50, watchedAt: Date.now() }],
  })))
  let releaseOriginal!: () => void
  const holdOriginal = new Promise<void>((resolve) => { releaseOriginal = resolve })
  await page.route('**/demo/demo-2.*', async (route) => {
    await holdOriginal
    await route.abort().catch(() => {})
  })
  await page.goto('/#/watch/d1/31?paused', { waitUntil: 'domcontentloaded' })
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'resolving')

  const status = await page.evaluate(async () => {
    const button = (name: string) => document.querySelector<HTMLButtonElement>(`[data-testid="${name}"]`)!
    button('quality').click()
    await Promise.resolve()
    button('q-流畅').click()
    await Promise.resolve()
    document.querySelector('video')!.dispatchEvent(new Event('loadedmetadata'))
    await Promise.resolve()
    return document.querySelector('[data-testid="player"]')!.getAttribute('data-state')
  }).finally(releaseOriginal)
  expect(status).toBe('resolving')
  await expect(page.locator('video')).toHaveJSProperty('videoWidth', 240)
  await expect(player).toHaveAttribute('data-state', 'paused')
  expect(Math.abs((await t(page)) - 12)).toBeLessThan(0.5)
})

test('uses the selected MP4 variant when WebM is unavailable', async ({ page }) => {
  await page.route('**/demo/*.webm', (route) => route.abort())
  await page.goto('/#/watch/d1/31?paused&t=8')
  const video = page.locator('video')
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect(video).toHaveJSProperty('videoWidth', 360)
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentSrc)).toMatch(/demo-2\.mp4$/)
  await page.getByTestId('quality').click()
  await page.getByTestId('q-流畅').click()
  await expect(video).toHaveJSProperty('videoWidth', 240)
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentSrc)).toMatch(/demo-2-sd\.mp4$/)
  expect(Math.abs((await t(page)) - 8)).toBeLessThan(0.5)
})

test('all download entries save the configured MP4 independently of playback quality', async ({ page }) => {
  await page.goto('/#/settings')
  await page.getByTestId('download-quality').selectOption('流畅')
  await page.goto('/#/watch/d1/32?paused')
  await expect(page.locator('video')).toHaveJSProperty('videoWidth', 360)

  const expected = await readFile(new URL('../../public/demo/demo-3-sd.mp4', import.meta.url))
  const assertDownload = async (button: import('@playwright/test').Locator) => {
    const download = page.waitForEvent('download')
    await button.click()
    const file = await download
    expect(file.suggestedFilename()).toBe('JindiDrama-demo-d1-32-sd.mp4')
    expect(await readFile(await file.path())).toEqual(expected)
  }
  const playbackButtons = page.getByRole('button', { name: '下载', exact: true })
  await expect(playbackButtons).toHaveCount(2)
  for (const button of await playbackButtons.all()) await assertDownload(button)

  await page.getByTestId('rtab-comments').click()
  const commentPage = page.getByTestId('page-comments')
  await expect(commentPage).toBeVisible()
  await assertDownload(commentPage.getByRole('button', { name: '下载', exact: true }))
})
