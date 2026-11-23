import { test, expect } from '@playwright/test'

const t = (page: import('@playwright/test').Page) => page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).currentTime)
test.describe('P1 真实演示媒体播放', () => {
  test.beforeEach(async ({ page }) => { await page.addInitScript(() => localStorage.removeItem('jindi.library.v1')) })
  test('loads real media, plays, time advances, duration is real', async ({ page }) => {
    await page.goto('/#/watch/d1/32?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    const d = await page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).duration); expect(d).toBeGreaterThan(23); expect(d).toBeLessThan(25)
    await page.getByTestId('toggle').click(); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing'); const t0 = await t(page); await page.waitForTimeout(1200); expect(await t(page)).toBeGreaterThan(t0 + 0.5)
  })
  test('seek by keys / click and playbackRate follows speed menu', async ({ page }) => {
    await page.goto('/#/watch/d1/7?paused'); await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused', { timeout: 10000 })
    await page.getByTestId('player').focus(); await page.keyboard.press('ArrowRight'); expect(await t(page)).toBeGreaterThan(9)
    await page.keyboard.press('ArrowLeft'); expect(await t(page)).toBeLessThan(1)
    const box = (await page.getByTestId('seek').boundingBox())!; await page.mouse.click(box.x + box.width * 0.5, box.y + 2); expect(Math.abs((await t(page)) - 12)).toBeLessThan(1.5)
    await page.getByTestId('speed').click(); await page.getByTestId('speed-2').click(); expect(await page.evaluate(() => (document.querySelector('video') as HTMLVideoElement).playbackRate)).toBe(2)
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
