import { test, expect } from './fixtures'
import fs from 'node:fs'
test.describe('02 搜索结果', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/search?q=%E6%9B%BF%E5%AB%81'); await page.waitForSelector('[data-testid=result-card]') })
  test('screenshot + count', async ({ page }) => { await expect(page.getByTestId('result-count')).toContainText('5 部'); await page.screenshot({ path: '../design/rendered/02-搜索结果.png' }) })
  test('kind filter, status filter, empty state', async ({ page }) => {
    await page.getByTestId('kind-AI剧').click(); await expect(page.getByTestId('result-card')).toHaveCount(1)
    await page.getByTestId('status-finished').click(); await expect(page.getByTestId('empty')).toBeVisible()
  })
  test('sort by new puts d16 first', async ({ page }) => { await page.getByTestId('sort-new').click(); await expect(page.getByTestId('result-card').first()).toContainText('AI替嫁') })
  test('hot word search; history add/remove/clear', async ({ page }) => {
    await page.getByTestId('hot-word').nth(1).click(); await expect(page).toHaveURL(/q=/); await expect(page.getByTestId('history-item').first()).toContainText('替嫁新娘')
    await page.getByRole('button', { name: '删除替嫁新娘' }).click(); await expect(page.getByTestId('history-item').filter({ hasText: '替嫁新娘' })).toHaveCount(0)
    await page.getByTestId('clear-history').click(); await expect(page.getByTestId('history-item')).toHaveCount(0)
  })
  test('topbar suggestions & enter submit', async ({ page }) => {
    const i = page.getByTestId('search-input'); await i.fill('替嫁'); await i.focus(); await expect(page.getByRole('option')).toHaveCount(6)
    await page.getByRole('option').nth(1).click(); await expect(page).toHaveURL(/q=%E6%9B%BF%E5%AB%81%E6%96%B0%E5%A8%98/)
  })
  test('partial cast name suggests and finds the cast member’s works', async ({ page }) => {
    const input = page.getByTestId('search-input')
    await input.fill('萧景')
    const options = page.getByRole('option')
    await expect(options).toHaveCount(2)
    await expect(options.filter({ hasText: '暴君的替身医妃' })).toBeVisible()
    await input.press('Enter')
    await expect(page.getByTestId('result-card')).toHaveCount(2)
    await expect(page.getByTestId('result-card').filter({ hasText: '暴君的替身医妃' })).toBeVisible()
    await page.goto('/#/drama/d4')
    await expect(page.getByTestId('ep').first()).toBeVisible()
    await expect(page.getByRole('heading', { name: '演示主演' })).toHaveCount(0)
  })
  test('play / detail navigation', async ({ page }) => { await page.getByTestId('view-detail').first().click(); await expect(page).toHaveURL(/#\/drama\//); await page.goBack(); await page.getByTestId('play-now').first().click(); await expect(page).toHaveURL(/#\/watch\//) })
  test('error state', async ({ page }) => { await page.goto('/#/search?q=a&state=error'); await expect(page.getByTestId('error')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => {
    await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=result-card]')
    expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false)
    fs.mkdirSync('../design/rendered/responsive', { recursive: true }); await page.screenshot({ path: `../design/rendered/responsive/02-${w}x${h}.png` })
  })
})
