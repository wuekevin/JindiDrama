import { test, expect } from './fixtures'
test.describe('07 我的收藏', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/favorites'); await page.evaluate(() => localStorage.removeItem('jindi.library.v1')); await page.reload(); await page.waitForSelector('[data-testid=fav-card]') })
  test('screenshot + count', async ({ page }) => { await expect(page.getByTestId('fav-count')).toContainText('11'); await page.screenshot({ path: '../design/rendered/07-我的收藏.png' }) })
  test('tabs filter (在追 / 已看过 / 漫剧 / 真人剧)', async ({ page }) => {
    await page.getByTestId('ftab-done').click(); for (const c of await page.getByTestId('fav-card').all()) await expect(c).toContainText('100%')
    await page.getByTestId('ftab-manju').click(); await expect(page.getByTestId('fav-card')).toHaveCount(2)
    await page.getByTestId('ftab-watching').click(); expect(await page.getByTestId('fav-card').count()).toBeGreaterThan(0)
  })
  test('sort by heat and genre filter', async ({ page }) => { await page.getByTestId('sort').selectOption('heat'); await expect(page.getByTestId('fav-card').first()).toContainText('禁欲总裁'); await page.getByTestId('genre').selectOption('玄幻'); await expect(page.getByTestId('fav-card')).toHaveCount(2) })
  test('unfavorite single updates count', async ({ page }) => { await page.getByTestId('fav-unfav').first().click(); await expect(page.getByTestId('fav-count')).toContainText('10'); await expect(page.getByTestId('toast')).toContainText('取消收藏') })
  test('batch mode: select, confirm modal, remove', async ({ page }) => {
    await page.getByTestId('batch-toggle').click(); await expect(page.getByTestId('batchbar')).toBeVisible(); await expect(page.getByTestId('batch-unfav')).toBeDisabled()
    await page.getByTestId('fav-card').nth(0).locator('.cover').click(); await page.getByTestId('fav-card').nth(1).locator('.cover').click(); await expect(page.getByTestId('batchbar')).toContainText('已选 2')
    await page.getByTestId('batch-unfav').click(); await page.getByTestId('modal-confirm').click(); await expect(page.getByTestId('fav-count')).toContainText('9')
  })
  test('select all then cancel keeps data', async ({ page }) => { await page.getByTestId('edit-manage').click(); await page.getByTestId('select-all').click(); await expect(page.getByTestId('batchbar')).toContainText('已选 11'); await page.getByTestId('batch-unfav').click(); await page.getByTestId('modal').getByRole('button', { name: '取消', exact: true }).click(); await expect(page.getByTestId('fav-count')).toContainText('11') })
  test('empty state after removing all, persists reload', async ({ page }) => {
    await page.getByTestId('batch-toggle').click(); await page.getByTestId('select-all').click(); await page.getByTestId('batch-unfav').click(); await page.getByTestId('modal-confirm').click(); await expect(page.getByTestId('empty')).toBeVisible()
    await page.reload(); await expect(page.getByTestId('empty')).toBeVisible()
  })
  test('list view, play and detail navigation', async ({ page }) => { await page.getByTestId('view-list').click(); await expect(page.getByTestId('fav-grid')).toHaveClass(/list/); await page.getByTestId('fav-play').first().click(); await expect(page).toHaveURL(/#\/watch\//); await page.goBack(); await page.getByTestId('fav-card').first().locator('.cover').click(); await expect(page).toHaveURL(/#\/drama\//) })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=fav-card]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/07-${w}x${h}.png` }) })
})
