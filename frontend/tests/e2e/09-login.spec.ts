import { test, expect } from './fixtures'
test.describe('09 账号登录', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/login'); await page.evaluate(() => localStorage.removeItem('jindi.auth.v1')); await page.reload(); await page.waitForSelector('[data-testid=page-login]') })
  test('screenshot + demo badge visible (no official-login claim)', async ({ page }) => { await expect(page.getByTestId('demo-badge')).toContainText('演示模式'); await expect(page.locator('body')).not.toContainText('红果账号登录'); await page.screenshot({ path: '../design/rendered/09-账号登录.png' }) })
  test('validation: empty submit shows all errors', async ({ page }) => { await page.getByTestId('submit').click(); await expect(page.getByTestId('err-phone')).toBeVisible(); await expect(page.getByTestId('err-code')).toBeVisible(); await expect(page.getByTestId('err-agree')).toBeVisible() })
  test('invalid phone blocks get-code; valid starts countdown', async ({ page }) => {
    await page.getByTestId('phone').fill('123'); await page.getByTestId('get-code').click(); await expect(page.getByTestId('err-phone')).toBeVisible()
    await page.getByTestId('phone').fill('13800138000'); await page.getByTestId('get-code').click(); await expect(page.getByTestId('get-code')).toContainText('重新获取('); await expect(page.getByTestId('get-code')).toBeDisabled()
  })
  test('wrong code counts down attempts then locks', async ({ page }) => {
    await page.getByTestId('phone').fill('13800138000'); await page.getByTestId('agree').check()
    for (let i = 4; i >= 0; i--) { await page.getByTestId('code').fill('000000'); await page.getByTestId('submit').click(); await expect(page.getByTestId('err-form')).toContainText(i > 0 ? `还可尝试 ${i} 次` : '尝试次数过多') }
    await expect(page.getByTestId('submit')).toBeDisabled()
  })
  test('correct demo code → demo session, logout returns to form', async ({ page }) => {
    await page.getByTestId('phone').fill('13800138000'); await page.getByTestId('code').fill('123456'); await page.getByTestId('agree').check(); await page.getByTestId('submit').click()
    await expect(page.getByTestId('logged-in')).toContainText('演示用户8000'); await page.reload(); await expect(page.getByTestId('logged-in')).toBeVisible()
    await page.getByTestId('logout').click(); await expect(page.getByTestId('phone')).toBeVisible()
  })
  test('qr state machine: waiting → scanned → confirmed', async ({ page }) => {
    await page.getByTestId('tab-qr').click(); await expect(page.getByTestId('sim-confirm')).toBeDisabled(); await page.getByTestId('sim-scan').click(); await expect(page.getByTestId('qr-overlay')).toContainText('已扫码')
    await page.getByTestId('sim-confirm').click(); await expect(page.getByTestId('logged-in')).toBeVisible()
  })
  test('qr expiry and refresh', async ({ page }) => { await page.getByTestId('tab-qr').click(); await page.getByTestId('sim-expire').click(); await expect(page.getByTestId('qr-overlay')).toContainText('已过期'); await page.getByTestId('qr-refresh').click(); await expect(page.getByTestId('qr-overlay')).toHaveCount(0) })
  test('session expired banner', async ({ page }) => { await page.goto('/#/login?state=expired'); await expect(page.getByTestId('expired')).toContainText('已过期') })
  test('keyboard: Enter submits form', async ({ page }) => { await page.getByTestId('phone').focus(); await page.keyboard.press('Enter'); await expect(page.getByTestId('err-phone')).toBeVisible() })
  test('avatar opens login', async ({ page }) => { await page.goto('/#/home?static'); await page.getByTestId('avatar').click(); await expect(page).toHaveURL(/#\/login/) })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=page-login]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/09-${w}x${h}.png` }) })
})
