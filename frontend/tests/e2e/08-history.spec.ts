import { test, expect } from './fixtures'
test.describe('08 观看历史', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/history'); await page.evaluate(() => localStorage.removeItem('jindi.library.v1')); await page.reload(); await page.waitForSelector('[data-testid=history-item]') })
  test('screenshot + groups + stats', async ({ page }) => { await expect(page.getByTestId('group-today')).toContainText('3部'); await expect(page.getByTestId('group-yesterday')).toContainText('4部'); await expect(page.getByTestId('stat-total')).toHaveText('12'); await page.screenshot({ path: '../design/rendered/08-观看历史.png' }) })
  test('status tabs filter', async ({ page }) => { await page.getByTestId('htab-done').click(); await expect(page.getByTestId('history-item')).toHaveCount(4); await expect(page.getByTestId('resume')).toHaveCount(0); await page.getByTestId('htab-watching').click(); await expect(page.getByTestId('done-badge')).toHaveCount(0) })
  test('side filters combine (genre + time)', async ({ page }) => { await page.getByTestId('fg-甜宠').click(); const n1 = await page.getByTestId('history-item').count(); await page.getByTestId('fw-今天').click(); expect(await page.getByTestId('history-item').count()).toBeLessThanOrEqual(n1); await page.getByTestId('fw-全部').click(); await page.getByTestId('fg-全部').click(); await expect(page.getByTestId('history-item')).toHaveCount(12) })
  test('resume restores exact progress', async ({ page }) => {
    await page.getByTestId('resume').first().click(); await expect(page).toHaveURL(/#\/watch\/d1\/32/)
    await expect(page.locator('.time')).toContainText('02:'); const seek = page.getByTestId('seek'); await expect(seek).toHaveAttribute('aria-valuenow', /^(3[89]|40|41)$/)
  })
  test('continue-latest goes to most recent unfinished', async ({ page }) => { await page.getByTestId('continue-latest').click(); await expect(page).toHaveURL(/#\/watch\/d1\/32/) })
  test('delete single record via menu', async ({ page }) => { await page.getByTestId('h-more').first().click(); await page.getByTestId('h-delete').click(); await expect(page.getByTestId('stat-total')).toHaveText('11') })
  test('batch delete with confirmation', async ({ page }) => {
    await page.getByTestId('batch-toggle').first().click(); await page.getByTestId('hcheck').nth(0).click(); await page.getByTestId('hcheck').nth(1).click(); await expect(page.getByTestId('batchbar')).toContainText('已选 2'); await page.getByTestId('batch-delete').click(); await page.getByTestId('modal-confirm').click(); await expect(page.getByTestId('stat-total')).toHaveText('10')
  })
  test('clear all needs second confirmation; cancel keeps; confirm empties and persists', async ({ page }) => {
    await page.getByTestId('clear-all').click(); await page.getByTestId('modal').getByRole('button', { name: '取消', exact: true }).click(); await expect(page.getByTestId('stat-total')).toHaveText('12')
    await page.getByTestId('clear-all').click(); await page.getByTestId('modal-confirm').click(); await expect(page.getByTestId('empty')).toBeVisible(); await page.reload(); await expect(page.getByTestId('empty')).toBeVisible()
  })
  test('list view toggle & sort by progress', async ({ page }) => { await page.getByTestId('view-list').click(); await page.getByTestId('sort').click(); await expect(page.getByTestId('sort')).toContainText('按观看进度') })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=history-item]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/08-${w}x${h}.png` }) })
})
