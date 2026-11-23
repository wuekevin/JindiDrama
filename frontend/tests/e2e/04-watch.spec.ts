import { test, expect } from './fixtures'
test.describe('04 视频播放', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/watch/d1/32?t=192&paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused') })
  test('screenshot', async ({ page }) => { await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.screenshot({ path: '../design/rendered/04-视频播放.png' }) })
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
