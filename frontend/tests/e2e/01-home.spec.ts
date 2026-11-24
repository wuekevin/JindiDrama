import { test, expect } from './fixtures'
import fs from 'node:fs'

test.describe('01 首页推荐', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/home?static'); await page.waitForSelector('[data-testid=hero]') })
  test('structure & screenshot 1448x1086', async ({ page }) => {
    await expect(page.getByTestId('drama-card')).toHaveCount(6)
    await page.evaluate(() => document.fonts.ready)
    fs.mkdirSync('../design/rendered', { recursive: true })
    await page.screenshot({ path: '../design/rendered/01-首页推荐.png' })
  })
  test('hero carousel arrows & dots', async ({ page }) => {
    const title = () => page.locator('.hero .title').innerText()
    const t0 = await title(); await page.getByTestId('hero-next').click(); expect(await title()).not.toBe(t0)
    await page.getByTestId('hero-prev').click(); expect(await title()).toBe(t0)
  })
  test('hero favorite toggles with toast', async ({ page }) => {
    const btn = page.getByTestId('hero-fav'); const before = await btn.innerText(); await btn.click()
    await expect(page.getByTestId('toast')).toBeVisible(); expect(await btn.innerText()).not.toBe(before)
  })
  test('rank tab switch', async ({ page }) => { await page.getByTestId('rank-rise').click(); await expect(page.getByTestId('rank-item').first()).toContainText('离婚后') })
  test('top search navigates to results', async ({ page }) => { await page.getByTestId('search-input').fill('替嫁'); await page.keyboard.press('Enter'); await expect(page).toHaveURL(/#\/search\?q=/) })
  test('sidebar nav to favorites and back', async ({ page }) => { await page.getByTestId('nav-favorites').click(); await expect(page).toHaveURL(/favorites/); await page.getByTestId('nav-home').click(); await expect(page).toHaveURL(/home/) })
  test('card opens detail; resume opens player', async ({ page }) => { await page.getByTestId('drama-card').first().click(); await expect(page).toHaveURL(/#\/drama\//); await page.goBack(); await page.getByTestId('resume-card').first().click(); await expect(page).toHaveURL(/#\/watch\/d1\/32/) })
  test('error & empty states', async ({ page }) => {
    await page.goto('/#/home?state=error'); await expect(page.getByTestId('error')).toBeVisible()
    await page.goto('/#/home?state=empty'); await expect(page.getByTestId('empty')).toBeVisible()
  })
  test('keyboard: tab reaches first nav item', async ({ page }) => { await page.keyboard.press('Tab'); await expect(page.locator(':focus')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) {
    test(`responsive ${w}x${h} no horizontal overflow`, async ({ page }) => {
      await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=hero]')
      const over = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)
      expect(over).toBe(false)
      fs.mkdirSync('../design/rendered/responsive', { recursive: true })
      await page.screenshot({ path: `../design/rendered/responsive/01-${w}x${h}.png` })
    })
  }
  test('shrinking the window scales content down with no overflow (min window 1280x860)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 860 })
    await page.goto('/#/home'); await page.reload()
    await expect(page.getByTestId('hero')).toBeVisible()
    const m = await page.evaluate(() => ({ z: Number(getComputedStyle(document.documentElement).getPropertyValue('--app-zoom')), sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth, sh: document.documentElement.scrollHeight, ch: document.documentElement.clientHeight }))
    expect(m.z).toBeLessThan(1); expect(m.sw).toBeLessThanOrEqual(m.cw); expect(m.sh).toBeLessThanOrEqual(m.ch)
  })
})
