import { test, expect } from './fixtures'
test.describe('05 评论互动', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/watch/d1/32?panel=comments&t=192&paused'); await page.waitForSelector('[data-testid=comment]') })
  test('screenshot', async ({ page }) => { await expect(page.getByTestId('comment-total')).toContainText('3,217'); await page.screenshot({ path: '../design/rendered/05-评论互动.png' }) })
  test('post validation: empty disabled, >500 disabled with counter', async ({ page }) => {
    const post = page.getByTestId('comment-post'); await expect(post).toBeDisabled()
    await page.getByTestId('comment-input').fill('a'.repeat(501)); await expect(post).toBeDisabled(); await expect(page.getByTestId('comment-count')).toHaveClass(/over/)
  })
  test('post, appears on top as local demo, delete own', async ({ page }) => {
    await page.getByTestId('comment-input').fill('测试评论'); await page.getByTestId('comment-post').click()
    await expect(page.getByTestId('toast')).toContainText('不会同步'); await expect(page.getByTestId('comment').first()).toContainText('测试评论'); await expect(page.getByTestId('comment-total')).toContainText('3,218')
    await page.getByTestId('more').first().click(); await page.getByTestId('delete').click(); await expect(page.getByTestId('comment').filter({ hasText: '测试评论' })).toHaveCount(0)
  })
  test('like toggles count', async ({ page }) => { const b = page.getByTestId('like').nth(1); const t = await b.innerText(); await b.click(); await expect(b).toHaveAttribute('aria-pressed', 'true'); expect(await b.innerText()).not.toBe(t); await b.click(); expect(await b.innerText()).toBe(t) })
  test('expand replies and send reply', async ({ page }) => {
    await page.getByTestId('toggle-replies').first().click(); await expect(page.getByTestId('replies').first()).toBeVisible()
    await page.getByTestId('reply-btn').first().click(); await page.getByTestId('reply-input').fill('我也觉得'); await page.getByTestId('reply-send').click(); await expect(page.getByTestId('replies').first()).toContainText('我也觉得')
  })
  test('sort hot reorders; author filter', async ({ page }) => { await page.getByTestId('csort-hot').click(); await expect(page.getByTestId('comment').first()).toContainText('晚风也温柔'); await page.getByTestId('csort-author').click(); await expect(page.getByTestId('comment').first()).toContainText('作者') })
  test('load more adds comments', async ({ page }) => { await expect(page.getByTestId('comment')).toHaveCount(4); await page.getByTestId('load-more').click(); await expect(page.getByTestId('comment')).toHaveCount(8) })
  test('close returns to player', async ({ page }) => { await page.getByTestId('comment-close').click(); await expect(page.getByTestId('page-watch')).toBeVisible() })
  test('Esc closes drawer', async ({ page }) => { await page.getByTestId('comment-input').focus(); await page.keyboard.press('Escape'); await expect(page.getByTestId('page-watch')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=comment]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/05-${w}x${h}.png` }) })
})
