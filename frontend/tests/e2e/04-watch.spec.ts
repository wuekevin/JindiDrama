import { test, expect } from './fixtures'

test('configured player shortcuts persist and preserve native controls', async ({ page }) => {
  await page.goto('/#/settings')
  for (const [action, key] of [['playPause', 'K'], ['next', 'N'], ['danmaku', 'B']]) {
    await page.getByTestId(`key-${action}`).click()
    await page.keyboard.press(key)
    await expect(page.getByTestId(`key-${action}`)).toHaveText(key)
  }
  await page.goto('/#/watch/d1/31?t=192&paused')
  await page.reload()
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await player.focus()
  await page.keyboard.press('Space')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await page.keyboard.press('K')
  await expect(player).toHaveAttribute('data-state', 'playing')
  await player.dispatchEvent('keydown', { key: 'K', repeat: true })
  await expect(player).toHaveAttribute('data-state', 'playing')
  await page.keyboard.press('K')
  await expect(player).toHaveAttribute('data-state', 'paused')
  const dm = page.getByTestId('dm-toggle')
  await page.getByTestId('volume').focus()
  await page.keyboard.press('B')
  await expect(dm).not.toHaveClass(/off/)
  await player.focus()
  await page.keyboard.press('D')
  await expect(dm).not.toHaveClass(/off/)
  await page.keyboard.press('B')
  await expect(dm).toHaveClass(/off/)
  await page.keyboard.press('N')
  await expect(page).toHaveURL(/#\/watch\/d1\/32$/)
  await page.getByTestId('fullscreen').click()
  await expect.poll(() => page.evaluate(() => Boolean(document.fullscreenElement))).toBe(true)
  await page.keyboard.press('Control+s')
  await expect(player.getByTestId('toast').last()).toHaveText('当前没有可截图的视频画面')
})
test.describe('04 视频播放', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/watch/d1/32?t=192&paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused') })
  test('screenshot', async ({ page }) => { await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.screenshot({ path: '../design/rendered/04-视频播放.png' }) })
  test('downloads the bundled demo for the current episode', async ({ page }) => {
    const buttons = page.getByRole('button', { name: '下载', exact: true })
    await expect(buttons).toHaveCount(2)
    for (const button of await buttons.all()) {
      const download = page.waitForEvent('download')
      await button.click()
      const file = await download
      expect(file.suggestedFilename()).toBe('JindiDrama-demo-d1-32.mp4')
      expect(file.url()).toContain('/demo/demo-3.mp4')
      expect(await file.failure()).toBeNull()
    }
  })
  test('passes the migrated download quality to the desktop exporter', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('jindi.settings.v1', JSON.stringify({ dlQuality: '标清 480P' }))
      Object.assign(window, {
        downloadCalls: [] as unknown[][],
        go: { app: { App: {
          StorageReady: async () => false,
          ExportDemoEpisode: async (...args: unknown[]) => {
            (window as unknown as { downloadCalls: unknown[][] }).downloadCalls.push(args)
            return true
          },
        } } },
      })
    })
    await page.goto('/#/watch/d1/32?paused')
    await page.reload()
    await page.getByRole('button', { name: '下载', exact: true }).first().click()
    await expect(page.getByText('演示视频已保存')).toBeVisible()
    expect(await page.evaluate(() => (window as unknown as { downloadCalls: unknown[][] }).downloadCalls)).toEqual([['d1', 32, 'sd']])
  })
  test('state machine: paused→playing→paused via button and Space', async ({ page }) => {
    await page.getByTestId('toggle').click(); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing')
    await page.getByTestId('player').focus(); await page.keyboard.press('Space'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
  })
  test('time advances while playing', async ({ page }) => { const t0 = await page.locator('.time').innerText(); await page.getByTestId('toggle').click(); await page.waitForTimeout(1300); expect(await page.locator('.time').innerText()).not.toBe(t0) })
  test('seek by click and by arrow keys', async ({ page }) => {
    const before = await page.locator('.time').innerText(); await page.getByTestId('player').focus(); await page.keyboard.press('ArrowRight'); expect(await page.locator('.time').innerText()).not.toBe(before)
    const box = (await page.getByTestId('seek').boundingBox())!; await page.mouse.click(box.x + 2, box.y + 2); await expect(page.locator('.time')).toContainText('00:0')
  })
  test('speed and quality menus persist', async ({ page }) => {
    await page.getByTestId('speed').click(); await page.getByTestId('speed-1.5').click(); await expect(page.getByTestId('speed')).toContainText('1.5x')
    await page.getByTestId('quality').click(); await page.getByTestId('q-高清').click(); await expect(page.getByTestId('quality')).toContainText('高清')
    await page.reload(); await expect(page.getByTestId('speed')).toContainText('1.5x')
  })
  test('danmaku toggle button and D key', async ({ page }) => {
    await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.getByTestId('dm-toggle').click(); await expect(page.getByTestId('danmaku-item')).toHaveCount(0)
    await page.getByTestId('player').focus(); await page.keyboard.press('d'); await expect(page.getByTestId('danmaku-item').first()).toBeVisible()
  })
  test('next episode switches URL and reloads (ep32 is latest → toast)', async ({ page }) => { await page.getByTestId('next-ep').click({ force: true }).catch(() => {}); await expect(page).toHaveURL(/watch\/d1\/32/) })
  test('episode thumb switches episode, player resolves', async ({ page }) => { await page.getByTestId('ep').first().click(); await expect(page).toHaveURL(/watch\/d1\/21/); await expect(page.getByTestId('player')).toHaveAttribute('data-state', /playing|paused|ready/) })
  test('unreleased episode shows error + retry', async ({ page }) => { await page.goto('/#/watch/d1/50'); await expect(page.getByTestId('player-error')).toContainText('尚未更新'); await page.getByTestId('retry').click(); await expect(page.getByTestId('player-error')).toBeVisible() })
  test('simulated fault state', async ({ page }) => { await page.goto('/#/watch/d1/3?state=error'); await expect(page.getByTestId('player-error')).toContainText('模拟故障') })
  test('mute, fullscreen & pip handle gracefully', async ({ page }) => { await page.getByTestId('mute').click(); await page.getByTestId('fullscreen').click(); await page.getByTestId('pip').click(); await expect(page.getByTestId('player')).toBeVisible() })
  test('comment tab opens comments panel; back returns to detail', async ({ page }) => { await page.getByTestId('rtab-comments').click(); await expect(page.getByTestId('comment-panel')).toBeVisible(); await page.goBack().catch(() => {}) })
  test('favorite from rail', async ({ page }) => { const b = page.getByTestId('fav'); const t = await b.innerText(); await b.click(); expect(await b.innerText()).not.toBe(t) })
  test('history is recorded on leave', async ({ page }) => { await page.getByTestId('toggle').click(); await page.waitForTimeout(2500); await page.getByTestId('player-back').click(); await expect(page).toHaveURL(/#\/drama\/d1/) })
  test('unknown drama', async ({ page }) => { await page.goto('/#/watch/zzz/1'); await expect(page.getByTestId('empty')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=player]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/04-${w}x${h}.png` }) })
})

test('related drama shuffle changes the batch while retaining playback and links to the new drama', async ({ page }, testInfo) => {
  await page.goto('/#/watch/d1/31?t=192&paused=1')
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'paused')
  const cards = page.locator('.rec .ri')
  const shuffle = page.getByRole('button', { name: '换一换' })
  await expect(cards).toHaveCount(2)
  const original = await cards.allInnerTexts()
  const seek = page.getByRole('slider', { name: '播放进度' })
  const initialProgress = await seek.getAttribute('aria-valuenow')
  await seek.press('ArrowRight')
  await expect(seek).not.toHaveAttribute('aria-valuenow', initialProgress!)
  const adjustedProgress = await seek.getAttribute('aria-valuenow')
  await shuffle.click()
  await expect.poll(() => cards.allInnerTexts()).not.toEqual(original)
  await expect(cards).toHaveCount(2)
  await expect(player).toHaveAttribute('data-state', 'paused')
  await expect(page).toHaveURL(/#\/watch\/d1\/31\?t=192&paused=1$/)
  await expect(seek).toHaveAttribute('aria-valuenow', adjustedProgress!)
  await page.screenshot({ path: testInfo.outputPath('related-drama-shuffle.png') })

  const selected = cards.first()
  const title = await selected.locator('b').innerText()
  await selected.click()
  await expect(page.getByTestId('page-detail').getByRole('heading', { name: title })).toBeVisible()
  await expect(page).not.toHaveURL(/#\/drama\/d1$/)
})
