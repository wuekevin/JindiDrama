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

test('global topbar keeps search and window controls usable at 150% zoom', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    const keys: string[] = []
    Object.assign(window, { runtime: {
      WindowMinimise: () => keys.push('min'),
      WindowToggleMaximise: () => keys.push('max'),
      Quit: () => keys.push('close'),
    } })
    Object.defineProperty(window, '__jdTopbarActions', { value: keys })
  })
  const topbar = page.locator('.topbar')
  async function checkLayout() {
    const geometry = await topbar.evaluate((header) => {
      const [tabs, search, actions] = ['.tabs', '.search', '.right'].map((selector) =>
        header.querySelector(selector)!.getBoundingClientRect())
      return {
        tabsEnd: tabs.right, searchStart: search.left, searchEnd: search.right,
        actionsStart: actions.left, actionsEnd: actions.right, viewport: window.innerWidth,
      }
    })
    expect(geometry.tabsEnd).toBeLessThanOrEqual(geometry.searchStart)
    expect(geometry.searchEnd).toBeLessThanOrEqual(geometry.actionsStart)
    expect(geometry.actionsEnd).toBeLessThanOrEqual(geometry.viewport)
    expect(geometry.searchEnd - geometry.searchStart).toBeGreaterThan(160)
  }

  await page.goto('/#/home')
  await expect(page.getByTestId('hero')).toBeVisible()
  await checkLayout()
  await page.screenshot({ path: testInfo.outputPath('topbar-default.png') })

  await page.setViewportSize({ width: 1280, height: 860 })
  await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ theme: 'light', dpi: 150 })))
  await page.reload()
  await expect(page.locator('html')).toHaveCSS('--user-zoom', '1.5')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await expect(page.getByTestId('hero')).toBeVisible()
  await checkLayout()

  const input = topbar.getByRole('combobox', { name: '搜索' })
  await input.fill('替嫁')
  const suggestions = topbar.getByRole('listbox')
  const option = suggestions.getByRole('option').filter({ hasText: '替嫁新娘' }).first()
  await expect(option).toBeVisible()
  await expect(input).toHaveCSS('color', 'rgb(246, 236, 233)')
  await expect(option).toHaveCSS('color', 'rgb(246, 236, 233)')
  const popup = await suggestions.boundingBox()
  const search = await topbar.getByRole('search').boundingBox()
  if (!popup || !search) throw new Error('Search and suggestions must be visible')
  expect(popup.x).toBeGreaterThanOrEqual(search.x)
  expect(popup.x + popup.width).toBeLessThanOrEqual(search.x + search.width + 1)
  expect(popup.y).toBeGreaterThanOrEqual(search.y + search.height)
  await expect(topbar).toHaveCSS('--wails-draggable', 'drag')
  await expect(topbar.locator('.right')).toHaveCSS('--wails-draggable', 'drag')
  await expect(input).toHaveCSS('--wails-draggable', 'no-drag')
  await expect(topbar.getByRole('button', { name: '关闭' }).locator('svg')).toHaveCSS('--wails-draggable', 'no-drag')
  await page.screenshot({ path: testInfo.outputPath('topbar-scaled-light.png') })

  await input.press('Shift+ArrowDown')
  await expect(input).not.toHaveAttribute('aria-activedescendant')
  await input.press('ArrowDown')
  await expect(suggestions.getByRole('option').first()).toHaveAttribute('aria-selected', 'true')
  await input.press('ArrowDown')
  await expect(option).toHaveAttribute('aria-selected', 'true')
  const activeId = await option.getAttribute('id')
  expect(activeId).toBeTruthy()
  await expect(input).toHaveAttribute('aria-activedescendant', activeId!)
  await input.press('Escape')
  await expect(input).toBeFocused()
  await expect(input).toHaveValue('替嫁')
  await expect(input).toHaveAttribute('aria-expanded', 'false')
  await expect(suggestions).toHaveCount(0)
  await input.fill('萧景')
  await expect(suggestions.getByRole('option').filter({ hasText: '暴君的替身医妃' })).toBeVisible()
  await input.fill('替嫁')
  await expect(option).toBeVisible()
  await input.press('ArrowDown')
  await input.press('ArrowDown')
  await input.dispatchEvent('keydown', { key: 'Enter', isComposing: true })
  await expect(page).toHaveURL(/#\/home$/)
  await input.press('Enter')
  await expect(page).toHaveURL(/q=%E6%9B%BF%E5%AB%81%E6%96%B0%E5%A8%98/)
  await expect(input).toBeFocused()
  await expect(suggestions).toHaveCount(0)
  await input.fill('替嫁')
  await suggestions.getByRole('option').filter({ hasText: '替嫁王妃' }).click()
  await expect(page).toHaveURL(/q=%E6%9B%BF%E5%AB%81%E7%8E%8B%E5%A6%83/)
  await topbar.getByRole('button', { name: '通知' }).click()
  const notification = page.getByRole('dialog', { name: '更新通知' })
  await expect(notification).toBeVisible()
  await notification.getByRole('button', { name: '关闭' }).click()
  await topbar.getByRole('button', { name: '账号' }).click()
  await expect(page).toHaveURL(/#\/login$/)
  for (const name of ['最小化', '最大化', '关闭']) await topbar.getByRole('button', { name, exact: true }).click()
  expect(await page.evaluate(() => (window as Window & { __jdTopbarActions?: string[] }).__jdTopbarActions))
    .toEqual(['min', 'max', 'close'])
})

test('desktop search submits a suggestion without querying it twice', async ({ page }) => {
  const title = '替嫁成了千亿总裁的心尖宠'
  await page.addInitScript((title) => {
    const calls: string[] = []
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      SaveSearchHistory: async () => undefined,
      Search: async (query: string) => {
        if (query) calls.push(query)
        return [{ id: 'd1', title }]
      },
    } } } })
    Object.defineProperty(window, '__jdSearchQueries', { value: calls })
  }, title)
  await page.goto('/#/search')
  const input = page.getByRole('combobox', { name: '搜索' })
  const option = page.getByRole('option', { name: title, exact: true })
  await input.fill('替嫁')
  await expect(option).toBeVisible()
  await input.press('Escape')
  await expect(option).toHaveCount(0)
  await input.press('ArrowDown')
  await expect(option).toHaveAttribute('aria-selected', 'true')
  await input.press('Enter')
  await expect(page.getByRole('heading', { name: /的搜索结果/ })).toContainText(title)
  await expect(page.getByTestId('result-card')).toHaveCount(1)
  expect(await page.evaluate(() => (window as Window & { __jdSearchQueries?: string[] }).__jdSearchQueries))
    .toEqual(['替嫁', title])
})
