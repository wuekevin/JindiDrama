import { test, expect, controlsTabKey, deferPanelResize } from './fixtures'
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
    const batchbar = page.getByTestId('batchbar')
    await expect(batchbar.getByTestId('select-all')).toBeDisabled()
    await expect(batchbar.getByRole('button', { name: '完成', exact: true })).toBeFocused()
    await page.reload(); await expect(page.getByTestId('empty')).toBeVisible()
  })
  test('list view, play and detail navigation', async ({ page }) => { await page.getByTestId('view-list').click(); await expect(page.getByTestId('fav-grid')).toHaveClass(/list/); await page.getByTestId('fav-play').first().click(); await expect(page).toHaveURL(/#\/watch\//); await page.goBack(); await page.getByTestId('fav-card').first().locator('.cover').click(); await expect(page).toHaveURL(/#\/drama\//) })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=fav-card]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/07-${w}x${h}.png` }) })
})

test('favorites chooses the correct layout at fractional initial widths', async ({ page }) => {
  await deferPanelResize(page, 'page-favorites')
  let contentWidth = 570.25
  await page.route('**/', async (route) => {
    const response = await route.fetch()
    const html = await response.text()
    const style = `<style>[data-testid="page-favorites"] { width: calc(${contentWidth}px + 30px) }</style>`
    await route.fulfill({ response, body: html.replace('</head>', style + '</head>') })
  })
  const favorites = page.getByTestId('page-favorites')
  for (const [width, layout] of [[570, 'compact'], [570.25, 'stacked'], [1050, 'stacked'], [1050.25, 'wide']] as const) {
    contentWidth = width
    await page.goto('/#/favorites')
    await expect(favorites).toHaveClass(`fav ${layout}`)
    // Make the next iteration a new document and measure the initial layout again.
    await page.goto('about:blank')
  }
})

for (const dpi of [150, 220]) {
  test(`favorites keeps toolbar, cards and batch operations reachable at ${dpi}% zoom`, async ({ page, browserName }, testInfo) => {
    await page.setViewportSize({ width: 1280, height: 860 })
    await deferPanelResize(page, 'page-favorites')
    await page.addInitScript((dpi) => {
      localStorage.setItem('jindi.settings.v1', JSON.stringify({ theme: 'light', dpi }))
    }, dpi)
    await page.goto('/#/favorites')
    const favorites = page.getByTestId('page-favorites')
    const cards = favorites.getByTestId('fav-card')
    const edit = favorites.getByTestId('edit-manage')
    await expect(cards).toHaveCount(11)
    const bounds = (await favorites.boundingBox())!
    const withinFavorites = async (control: ReturnType<typeof page.getByTestId>) => {
      const rect = (await control.boundingBox())!
      expect(rect.width).toBeGreaterThan(0)
      expect(rect.x).toBeGreaterThanOrEqual(bounds.x - 1)
      expect(rect.x + rect.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1)
    }
    for (const id of ['ftab-real', 'edit-manage', 'sort', 'genre', 'view-list', 'batch-toggle']) {
      await withinFavorites(favorites.getByTestId(id))
    }
    await withinFavorites(cards.nth(3).getByTestId('fav-more'))
    expect((await cards.first().locator('.info').boundingBox())!.width).toBeGreaterThan(100)
    await page.screenshot({ path: testInfo.outputPath('favorites-grid.png') })

    const first = cards.first()
    await expect(first.locator('.st')).toHaveCSS('color', 'rgb(246, 236, 233)')
    const more = first.getByTestId('fav-more')
    await more.press('Enter')
    await expect(more).toHaveAttribute('aria-expanded', 'true')
    const goToDetail = first.getByRole('button', { name: '查看详情' })
    await withinFavorites(goToDetail)
    await expect(goToDetail).toHaveCSS('color', 'rgb(246, 236, 233)')
    await page.keyboard.press(controlsTabKey(browserName))
    await expect(goToDetail).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(more).toBeFocused()
    await expect(goToDetail).toHaveCount(0)

    await favorites.getByTestId('view-list').click()
    await withinFavorites(first.getByTestId('fav-play'))
    await withinFavorites(first.getByTestId('fav-unfav'))
    if (dpi === 220) {
      const actions = first.locator('.ops')
      await expect(actions).toHaveCSS('grid-column-start', '1')
      await expect(actions).toHaveCSS('grid-column-end', '-1')
      await expect(actions).toHaveCSS('flex-direction', 'row')
    }
    await first.getByTestId('fav-play').press('Enter')
    await expect(page).toHaveURL(/#\/watch\//)
    await page.goBack()

    if (dpi === 220) {
      await edit.click()
      await more.evaluate((button) => {
        const content = button.closest('#content')!
        const footer = content.querySelector('[data-testid="batchbar"]')!
        const zoom = Number.parseFloat(getComputedStyle(document.getElementById('app')!).zoom)
        content.scrollTop += (button.getBoundingClientRect().top - footer.getBoundingClientRect().top + 36) / zoom
      })
      await more.click()
      const [popup, footer] = await Promise.all([goToDetail.boundingBox(), favorites.getByTestId('batchbar').boundingBox()])
      expect(popup!.y + popup!.height / 2).toBeGreaterThan(footer!.y)
      expect(popup!.y + popup!.height / 2).toBeLessThan(footer!.y + footer!.height)
      expect(await goToDetail.evaluate((button) => {
        const box = button.getBoundingClientRect()
        return button.contains(document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2))
      })).toBe(true)
      await goToDetail.click()
      await expect(page).toHaveURL(/#\/drama\//)
      await page.goBack()
    }
    await edit.click()
    const cover = first.locator('.cover')
    await cover.press('Enter')
    await expect(cover).toHaveAttribute('aria-pressed', 'true')
    const batchbar = favorites.getByTestId('batchbar')
    await expect(batchbar).toContainText('已选 1')
    const trigger = batchbar.getByTestId('batch-unfav')
    await withinFavorites(trigger)
    await withinFavorites(batchbar.getByRole('button', { name: '完成', exact: true }))
    await trigger.press('Enter')
    const modal = page.getByRole('dialog', { name: '取消收藏' })
    await expect(modal).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(modal).toHaveCount(0)
    await expect(trigger).toBeFocused()
    await trigger.press('Enter')
    await page.keyboard.press('Shift+Tab')
    await expect(modal.getByTestId('modal-confirm')).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(cards).toHaveCount(10)
    await expect(batchbar.getByTestId('select-all')).toBeFocused()
    await page.reload()
    await expect(cards).toHaveCount(10)
  })
}

test('favorites contains a long licensed tag in the toolbar and both card views at 220% zoom', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 860 })
  const tag = 'LongLicensedTopic'.repeat(12)
  await page.addInitScript((topic) => {
    const entry = { id: 'local-long-tag', title: '授权短片', tags: [topic], summary: '', license: '作者自有', updatedTo: 1, episodeCount: 1 }
    localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi: 220 }))
    localStorage.setItem('jindi.library.v1', JSON.stringify({ favorites: [entry.id], history: [] }))
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false, LicensedCatalog: async () => [entry],
    } } } })
  }, tag)
  await page.goto('/#/favorites')
  const favorites = page.getByTestId('page-favorites')
  const genre = favorites.getByTestId('genre')
  await expect(genre.locator('option').filter({ hasText: tag })).toHaveCount(1)
  const bounds = (await favorites.boundingBox())!
  const box = (await genre.boundingBox())!
  expect(box.x + box.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1)
  await genre.selectOption(tag)
  await expect(genre).toHaveValue(tag)
  const card = favorites.getByTestId('fav-card')
  await expect(card).toContainText('授权短片')
  for (const view of ['view-grid', 'view-list']) {
    await favorites.getByTestId(view).click()
    const meta = card.locator('.mt')
    expect(await meta.evaluate(el => el.scrollWidth - el.clientWidth)).toBeLessThanOrEqual(1)
  }
})
