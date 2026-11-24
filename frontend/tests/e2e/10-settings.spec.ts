import { test, expect } from './fixtures'
test.describe('10 应用设置', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/settings'); await page.evaluate(() => { localStorage.removeItem('jindi.settings.v1'); localStorage.removeItem('jindi.player.v1') }); await page.reload(); await page.waitForSelector('[data-testid=page-settings]') })
  test('screenshot', async ({ page }) => { await expect(page.locator('#sec-keys')).toBeVisible(); await page.screenshot({ path: '../design/rendered/10-应用设置.png' }) })
  test('toggles & selects persist across reload', async ({ page }) => {
    await page.getByTestId('autostart').click(); await page.getByTestId('close-action').selectOption('退出程序'); await page.getByTestId('default-quality').selectOption('蓝光 4K'); await page.getByTestId('decode-sw').check(); await page.getByTestId('dl-concurrent').selectOption('5')
    await page.reload(); await expect(page.getByTestId('autostart')).toHaveAttribute('aria-checked', 'true'); await expect(page.getByTestId('close-action')).toHaveValue('退出程序'); await expect(page.getByTestId('default-quality')).toHaveValue('蓝光 4K'); await expect(page.getByTestId('decode-sw')).toBeChecked(); await expect(page.getByTestId('dl-concurrent')).toHaveValue('5')
  })
  test('theme switch applies data-theme and persists', async ({ page }) => { await page.getByTestId('theme-light').click(); await expect(page.locator('html')).toHaveAttribute('data-theme', 'light'); await page.reload(); await expect(page.locator('html')).toHaveAttribute('data-theme', 'light'); await page.getByTestId('theme-dark').click(); await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark') })
  test('DPI slider scales UI', async ({ page }) => { const w0 = (await page.locator('.nav').boundingBox())!.width; await page.getByTestId('dpi').fill('120'); await expect.poll(async () => (await page.locator('.nav').boundingBox())!.width).toBeGreaterThan(w0 * 1.1) })
  test('danmaku settings are shared with player settings', async ({ page }) => { await page.getByTestId('s-dm-size').fill('22'); await page.goto('/#/watch/d1/32?panel=danmaku&paused&t=100'); await expect(page.getByTestId('dm-size')).toHaveValue('22') })
  test('cache: change path, clear with confirm', async ({ page }) => {
    await page.getByTestId('change-path').click(); await expect(page.getByTestId('cache-path')).not.toHaveValue('JindiDrama/Cache/Downloads')
    await page.getByTestId('clear-cache').click(); await page.getByTestId('modal-confirm').click(); await expect(page.getByTestId('cache-size')).toContainText('0.0 GB'); await expect(page.getByTestId('clear-cache')).toBeDisabled()
  })
  test('shortcut rebind: accept, conflict rejected, reset', async ({ page }) => {
    await page.getByTestId('key-danmaku').click(); await page.keyboard.press('B'); await expect(page.getByTestId('key-danmaku')).toHaveText('B')
    await page.getByTestId('key-fullscreen').click(); await page.keyboard.press('B'); await expect(page.getByTestId('toast').last()).toContainText('冲突'); await expect(page.getByTestId('key-fullscreen')).toHaveText('F')
    await page.reload(); await expect(page.getByTestId('key-danmaku')).toHaveText('B'); await page.getByTestId('reset-keys').click(); await expect(page.getByTestId('key-danmaku')).toHaveText('D')
  })
  test('Esc cancels key recording', async ({ page }) => { await page.getByTestId('key-danmaku').click(); await expect(page.getByTestId('key-danmaku')).toContainText('按下'); await page.keyboard.press('Escape'); await expect(page.getByTestId('key-danmaku')).toHaveText('D') })
  test('check update in browser preview reports unsupported (no fake result)', async ({ page }) => { await page.getByTestId('check-update').click(); await expect(page.getByTestId('toast')).toContainText('检查更新'); await expect(page.getByTestId('version')).toContainText('v0.1.0') })
  test('cache limit selectable, persists; diagnostics copy handled', async ({ page }) => {
    await page.getByTestId('cache-limit').selectOption('10'); await expect(page.getByTestId('toast').last()).toContainText('10 GB'); await page.reload(); await expect(page.getByTestId('cache-limit')).toHaveValue('10')
    await page.getByTestId('copy-diag').click(); await expect(page.getByTestId('toast').last()).toContainText(/诊断信息已复制|无法写入/)
  })
  test('left nav scrolls to section', async ({ page }) => { await page.getByTestId('snav-about').click(); await expect(page.getByTestId('snav-about')).toHaveClass(/on/) })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=page-settings]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/10-${w}x${h}.png` }) })
  test('saved light theme is applied on startup without opening settings', async ({ page }) => {
    await page.getByTestId('theme-light').click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await page.goto('/#/home'); await page.reload()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  })
  test('close-action offers only implemented options; legacy tray value migrates to 退出程序', async ({ page }) => {
    await expect(page.getByTestId('close-action').locator('option')).toHaveText(['最小化到任务栏', '退出程序'])
    await expect(page.getByText('最小化到托盘')).toHaveCount(0)
    await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ closeAction: '最小化到托盘' })))
    await page.reload()
    await expect(page.getByTestId('close-action')).toHaveValue('退出程序')
  })
  test('autostart toggle works in browser preview (no desktop bridge)', async ({ page }) => {
    await page.getByTestId('autostart').click()
    await expect(page.getByTestId('autostart')).toHaveAttribute('aria-checked', 'true')
  })
})
