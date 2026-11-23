import { test, expect } from './fixtures'
test.describe('06 弹幕设置', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/watch/d1/32?panel=danmaku&t=192&paused'); await page.waitForSelector('[data-testid=danmaku-settings]'); await page.evaluate(() => localStorage.removeItem('jindi.player.v1')); await page.reload(); await page.waitForSelector('[data-testid=danmaku-settings]') })
  test('screenshot', async ({ page }) => { await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.screenshot({ path: '../design/rendered/06-弹幕设置.png' }) })
  test('master switch hides all danmaku', async ({ page }) => { await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.getByTestId('dm-enabled').click(); await expect(page.getByTestId('danmaku-item')).toHaveCount(0); await page.getByTestId('dm-enabled').click(); await expect(page.getByTestId('danmaku-item').first()).toBeVisible() })
  test('opacity / font size sliders change rendering', async ({ page }) => {
    await page.getByTestId('dm-size').fill('24'); await expect(page.getByTestId('danmaku-item').first()).toHaveCSS('font-size', '24px')
    await page.getByTestId('dm-opacity').fill('40'); await expect(page.getByTestId('danmaku-item').first()).toHaveCSS('opacity', '0.4')
  })
  test('area top confines danmaku to upper half', async ({ page }) => {
    await page.getByTestId('area-top').click(); const box = (await page.getByTestId('danmaku-layer').boundingBox())!
    for (const el of await page.getByTestId('danmaku-item').all()) { const b = (await el.boundingBox())!; expect(b.y + b.height / 2).toBeLessThanOrEqual(box.y + box.height / 2 + 2) }
  })
  test('hot only reduces count', async ({ page }) => { const n = await page.getByTestId('danmaku-item').count(); await page.getByTestId('dm-hot').click(); expect(await page.getByTestId('danmaku-item').count()).toBeLessThanOrEqual(n) })
  test('blocked keyword add/remove & validation', async ({ page }) => {
    await page.getByTestId('kw-add-btn').click(); await page.getByTestId('kw-confirm').click(); await expect(page.getByTestId('toast').last()).toContainText('不能为空')
    await page.getByTestId('kw-input').fill('剧透'); await page.getByTestId('kw-confirm').click(); await expect(page.getByTestId('toast').last()).toContainText('已存在')
    await page.getByTestId('kw-input').fill('太甜'); await page.getByTestId('kw-confirm').click(); await expect(page.getByTestId('kw').filter({ hasText: '太甜' })).toHaveCount(1)
    await expect(page.getByTestId('danmaku-item').filter({ hasText: '太甜' })).toHaveCount(0)
    await page.getByRole('button', { name: '移除太甜' }).click(); await expect(page.getByTestId('kw').filter({ hasText: '太甜' })).toHaveCount(0)
  })
  test('send danmaku appears as mine; blocked/empty rejected', async ({ page }) => {
    await page.getByTestId('dm-send').click(); await expect(page.getByTestId('toast').last()).toContainText('请输入')
    await page.getByTestId('dm-input').fill('这是广告'); await page.getByTestId('dm-send').click(); await expect(page.getByTestId('toast').last()).toContainText('屏蔽')
    await page.getByTestId('dm-input').fill('我的弹幕测试'); await page.getByTestId('dm-input').press('Enter'); await expect(page.getByTestId('danmaku-item').filter({ hasText: '我的弹幕测试' })).toBeVisible()
  })
  test('settings persist after reload', async ({ page }) => { await page.getByTestId('dm-size').fill('20'); await page.reload(); await expect(page.getByTestId('dm-size')).toHaveValue('20') })
  test('back returns to player', async ({ page }) => { await page.getByTestId('danmaku-back').click(); await expect(page.getByTestId('page-watch')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=danmaku-settings]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/06-${w}x${h}.png` }) })
})
