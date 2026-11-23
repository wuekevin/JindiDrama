import { test, expect } from './fixtures'
import fs from 'node:fs'
test.describe('03 短剧详情', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/drama/d1'); await page.waitForSelector('[data-testid=ep]') })
  test('screenshot', async ({ page }) => { await expect(page.locator('h1')).toContainText('替嫁成了千亿总裁'); await page.screenshot({ path: '../design/rendered/03-短剧详情.png' }) })
  test('history resumes to group 21-40 with current ep 32', async ({ page }) => { await expect(page.getByTestId('ep').filter({ hasText: '32' }).first()).toHaveClass(/cur/); await expect(page.getByTestId('continue')).toContainText('第32集') })
  test('group switch & episode click goes to player', async ({ page }) => { await page.getByTestId('ep-group').first().click(); await page.getByTestId('ep').nth(4).click(); await expect(page).toHaveURL(/#\/watch\/d1\/5$/) })
  test('unreleased episode blocked with toast', async ({ page }) => { await page.getByTestId('ep').last().click(); await expect(page.getByTestId('toast')).toContainText('尚未更新'); await expect(page).toHaveURL(/#\/drama\/d1/) })
  test('favorite toggle', async ({ page }) => { const b = page.getByTestId('fav'); const t = await b.innerText(); await b.click(); expect(await b.innerText()).not.toBe(t) })
  test('expand summary', async ({ page }) => { const before = await page.getByTestId('summary').innerText(); await page.getByTestId('expand').click(); expect((await page.getByTestId('summary').innerText()).length).toBeGreaterThan(before.length) })
  test('clips tab, season select, shuffle, guess nav', async ({ page }) => {
    await page.getByTestId('tab-clips').click(); await expect(page.getByTestId('clip')).toHaveCount(3)
    await page.getByTestId('season').nth(1).click(); await expect(page.getByTestId('season').nth(1)).toHaveClass(/on/)
    const first = await page.getByTestId('guess-card').first().innerText(); await page.getByTestId('shuffle').click(); expect(await page.getByTestId('guess-card').first().innerText()).not.toBe(first)
    await page.getByTestId('guess-card').first().click(); await expect(page).toHaveURL(/#\/drama\/d\d+/)
  })
  test('not found & error', async ({ page }) => { await page.goto('/#/drama/nope'); await expect(page.getByTestId('empty')).toBeVisible(); await page.goto('/#/drama/d1?state=error'); await expect(page.getByTestId('error')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => {
    await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=ep]')
    expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false)
    await page.screenshot({ path: `../design/rendered/responsive/03-${w}x${h}.png` })
  })
})
