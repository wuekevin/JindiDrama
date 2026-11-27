import { test, expect, controlsTabKey } from './fixtures'
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
  test('sidebar preserves default positions and separates decoration at 150% zoom', async ({ page }, testInfo) => {
    const sidebar = page.getByRole('complementary', { name: '主导航' })
    const nav = sidebar.getByRole('navigation')
    const settings = nav.getByRole('button', { name: '设置' })
    const decoration = sidebar.locator('.deco')
    await page.evaluate(() => document.fonts.ready)
    const defaultNav = (await nav.boundingBox())!
    const defaultDeco = (await decoration.boundingBox())!
    const defaultSidebar = (await sidebar.boundingBox())!
    expect(defaultDeco.y).toBeGreaterThan(defaultNav.y + defaultNav.height)
    expect(defaultDeco.y + defaultDeco.height).toBeCloseTo(defaultSidebar.height, 0)
    await sidebar.screenshot({ path: testInfo.outputPath('sidebar-default.png') })

    await page.setViewportSize({ width: 1280, height: 860 })
    await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi: 150, theme: 'light' })))
    await page.reload()
    await expect(page.locator('html')).toHaveCSS('--user-zoom', '1.5')
    await expect(settings).toBeInViewport({ ratio: 1 })
    await sidebar.screenshot({ path: testInfo.outputPath('sidebar-150.png') })
    const scaledNav = (await nav.boundingBox())!
    const scaledDeco = (await decoration.boundingBox())!
    expect(scaledDeco.y).toBeGreaterThanOrEqual(scaledNav.y + scaledNav.height - 1)
    await settings.click()
    await expect(settings).toHaveAttribute('aria-current', 'page')
  })

  test('sidebar can scroll by keyboard and mouse with an older 220% setting', async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 1280, height: 860 })
    await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi: 220 })))
    await page.reload()
    await expect(page.locator('html')).toHaveCSS('--user-zoom', '2.2')
    const documentOverflows = await page.evaluate(() => {
      const root = document.documentElement
      return root.scrollWidth > root.clientWidth || root.scrollHeight > root.clientHeight
    })
    expect(documentOverflows).toBe(false)
    const sidebar = page.getByRole('complementary', { name: '主导航' })
    const nav = sidebar.getByRole('navigation')
    const settings = nav.getByRole('button', { name: '设置' })
    const scrollTop = () => nav.evaluate((element) => element.scrollTop)
    await expect(nav.getByRole('button', { name: '推荐' })).toBeInViewport({ ratio: 1 })
    await expect(settings).not.toBeInViewport()
    for (const name of ['推荐', '热门', '漫剧', '真人剧', 'AI剧', '排行榜', '收藏', '历史', '设置']) {
      await page.keyboard.press('Tab')
      await expect(nav.getByRole('button', { name, exact: true })).toBeFocused()
    }
    await expect.poll(scrollTop).toBeGreaterThan(0)
    await expect(settings).toBeInViewport({ ratio: 0.99 })
    await expect(settings).toHaveCSS('outline-style', 'solid')
    expect(await settings.evaluate((element) => parseFloat(getComputedStyle(element).outlineOffset))).toBeLessThan(0)
    await sidebar.screenshot({ path: testInfo.outputPath('sidebar-220-keyboard.png') })
    await page.keyboard.press('Enter')
    await expect(settings).toHaveAttribute('aria-current', 'page')

    await page.goto('/#/home?static')
    await page.reload()
    await expect.poll(scrollTop).toBe(0)
    const { x, y, width, height } = (await nav.boundingBox())!
    await page.mouse.move(x + width / 2, y + height / 2)
    await page.mouse.wheel(0, 1000)
    await expect.poll(scrollTop).toBeGreaterThan(0)
    await expect(settings).toBeInViewport({ ratio: 0.99 })
    await settings.click()
    await expect(settings).toHaveAttribute('aria-current', 'page')
  })

  test('home topics reflow without clipping at 150% zoom', async ({ page, browserName }, testInfo) => {
    const topics = page.getByRole('region', { name: '题材精选' })
    async function measure() {
      return topics.evaluate((section) => {
        const content = section.closest('main')!.getBoundingClientRect()
        const bounds = section.getBoundingClientRect()
        const items = [...section.querySelectorAll('h3, button')]
        const following = section.nextElementSibling!.getBoundingClientRect()
        const rowCenters: number[] = []
        const clipped: string[] = []
        const wrappedText: string[] = []
        for (const item of items) {
          const box = item.getBoundingClientRect()
          const center = box.top + box.height / 2
          if (!rowCenters.some((row) => Math.abs(row - center) < 1)) rowCenters.push(center)
          const label = item.textContent?.trim() ?? ''
          if (box.left < content.left || box.right > content.right + 1 ||
              box.top < bounds.top - 1 || box.bottom > bounds.bottom + 1) clipped.push(label)

          const textLines = new Set<number>()
          const range = document.createRange()
          for (const node of item.childNodes) {
            if (node.nodeType !== Node.TEXT_NODE || !node.textContent?.trim()) continue
            range.selectNodeContents(node)
            for (const rect of range.getClientRects()) {
              if (rect.width) textLines.add(Math.round(rect.top))
            }
          }
          if (textLines.size > 1) wrappedText.push(label)
        }
        return {
          rows: rowCenters.length, clipped, wrappedText,
          overlapsNext: bounds.bottom > following.top,
        }
      })
    }
    await page.evaluate(() => document.fonts.ready)
    expect(await measure()).toEqual({ rows: 1, clipped: [], wrappedText: [], overlapsNext: false })
    await page.screenshot({ path: testInfo.outputPath('home-topics-default.png') })

    await page.setViewportSize({ width: 1280, height: 860 })
    await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi: 150, theme: 'light' })))
    await page.reload()
    await expect(page.locator('html')).toHaveCSS('--user-zoom', '1.5')
    await expect(topics).toBeVisible()
    await page.evaluate(() => document.fonts.ready)
    const compact = await measure()
    expect(compact.clipped).toEqual([])
    expect(compact.wrappedText).toEqual([])
    expect(compact.overlapsNext).toBe(false)
    expect(compact.rows).toBeGreaterThan(1)
    await page.screenshot({ path: testInfo.outputPath('home-topics-scaled.png') })

    const luxury = topics.getByRole('button', { name: '豪门', exact: true })
    await luxury.click()
    await expect(page.getByRole('heading', { name: /豪门.*搜索结果/ })).toBeVisible()
    await page.goBack()
    await luxury.focus()
    await page.keyboard.press(controlsTabKey(browserName))
    await expect(topics.getByRole('button', { name: '更多', exact: true })).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('heading', { name: '按题材浏览' })).toBeVisible()
    const filter = page.getByTestId('topic-select')
    await filter.selectOption('先婚后爱')
    await expect(filter).toHaveValue('先婚后爱')
    await expect(page.getByTestId('result-card')).toHaveCount(1)
    await expect(page.getByTestId('result-card')).toContainText('替嫁成了千亿总裁的心尖宠')
  })
})

test('today new shows the full Provider feed while keeping six previews', async ({ page }) => {
  await page.addInitScript(() => {
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      Drama: async () => ({}),
      Episodes: async () => [],
      HomeFeed: async () => {
        const reads = Number(sessionStorage.getItem('home-feed-reads') ?? '0')
        sessionStorage.setItem('home-feed-reads', String(reads + 1))
        return {
          todayNew: ['d7', 'd6', 'd5', 'd4', 'd3', 'd2', 'd1'],
          rankings: { hot: [], rise: ['d4', 'd7', 'd1', 'd2', 'd3', 'd6', 'd5'], new: [] },
        }
      },
    } } } })
  })
  await page.goto('/#/home?static')
  const section = page.getByRole('region', { name: '今日上新' })
  await expect(section).toContainText('共 7 部')
  await expect(section.getByTestId('drama-card')).toHaveCount(6)
  await section.getByTestId('all-today-new').click()
  await expect(page.getByRole('heading', { name: '今日上新' })).toBeVisible()
  await expect(page.getByTestId('result-count')).toContainText('7 部')
  await expect(page.getByTestId('result-card').first()).toContainText('天命之子')
  const feedReads = () => page.evaluate(() => sessionStorage.getItem('home-feed-reads'))
  const readsBeforePaging = await feedReads()
  await page.getByRole('navigation', { name: '分页' }).getByRole('button', { name: '2' }).click()
  await expect(page.getByTestId('result-card')).toHaveCount(2)
  expect(await feedReads()).toBe(readsBeforePaging)
  await expect(page.getByTestId('result-card').last()).toContainText('替嫁成了千亿总裁的心尖宠')
  await page.reload()
  await expect(page.getByTestId('result-card')).toHaveCount(2)
  await page.getByTestId('view-detail').last().click()
  await expect(page).toHaveURL(/#\/drama\/d1/)
  await page.goBack()
  await expect(page.getByTestId('result-card').last()).toContainText('替嫁成了千亿总裁的心尖宠')

  await page.goto('/#/home?static&rank=rise')
  await expect(page.getByTestId('rank-item')).toHaveCount(5)
  await expect(page.getByTestId('rank-item').first()).toContainText('重回高三')
  await page.getByTestId('full-ranking').click()
  await expect(page.getByTestId('result-count')).toContainText('7 部')
  const rankedTitles = page.getByTestId('result-card').getByRole('heading', { level: 3 })
  await expect(rankedTitles).toHaveText([
    '重回高三：我把学霸追到手了',
    '天命之子：从废柴到神尊',
    '替嫁成了千亿总裁的心尖宠',
    '暴君的替身医妃',
    '雨夜逢君',
  ])
  await page.getByRole('navigation', { name: '分页' }).getByRole('button', { name: '2' }).click()
  await expect(page.getByTestId('result-card')).toHaveCount(2)
  await expect(rankedTitles).toHaveText(['离婚后，我成了顶流导演', '仙门小师妹今天也在逆天'])
})

test('full ranking keeps the selected Provider ranking across navigation and reload', async ({ page }) => {
  await page.goto('/#/home?static')
  await page.getByTestId('rank-rise').click()
  await expect(page.getByTestId('rank-item').first()).toContainText('离婚后')
  await page.getByTestId('full-ranking').click()
  await expect(page).toHaveURL(/feed=rise/)
  await expect(page.getByRole('heading', { name: '飙升榜' })).toBeVisible()
  await expect(page.getByTestId('sort-hot')).toHaveCount(0)
  await expect(page.getByTestId('result-card')).toHaveCount(5)
  await expect(page.getByTestId('result-card').first()).toContainText('离婚后')
  await page.reload()
  await expect(page.getByTestId('result-card').first()).toContainText('离婚后')
  await page.goBack()
  await expect(page.getByTestId('rank-rise')).toHaveAttribute('aria-selected', 'true')
  await page.getByTestId('rank-new').click()
  await page.getByTestId('full-ranking').click()
  await expect(page.getByRole('heading', { name: '新剧榜' })).toBeVisible()
  await expect(page.getByTestId('result-card').first()).toContainText('暴君的替身医妃')
})

test('more topics browses exact catalog tags and preserves the selected topic on back', async ({ page }) => {
  await page.goto('/#/home?static')
  await page.getByTestId('more-topics').click()
  await expect(page.getByRole('heading', { name: '按题材浏览' })).toBeVisible()
  const topic = page.getByTestId('topic-select')
  await topic.focus()
  await topic.selectOption('先婚后爱')
  await expect(topic).toHaveValue('先婚后爱')
  await expect(topic).toBeFocused()
  await expect(page.getByTestId('result-card')).toHaveCount(1)
  await expect(page.getByTestId('result-card').first()).toContainText('替嫁成了千亿总裁的心尖宠')
  await page.getByTestId('view-detail').first().click()
  await expect(page).toHaveURL(/#\/drama\//)
  await page.goBack()
  await expect(topic).toHaveValue('先婚后爱')
  await expect(page.getByTestId('result-card')).toHaveCount(1)
  await page.getByTestId('search-input').fill('替嫁')
  await page.getByTestId('search-input').press('Enter')
  await expect(page.getByRole('heading', { name: /“替嫁”\s*的搜索结果/ })).toBeVisible()
  await expect(topic).toHaveCount(0)
  await page.getByTestId('tab-category').click()
  await expect(topic).toHaveValue('')
  await expect(page.getByTestId('tab-category')).toHaveAttribute('aria-current', 'page')
})

test('a missing topic links back to the available tags and home keywords still match the catalog', async ({ page }) => {
  await page.goto('/#/search?browse=topics&topic=战神')
  await expect(page.getByText('目录没有这个题材')).toBeVisible()
  const topic = page.getByTestId('topic-select')
  await expect(topic).toHaveValue('')
  await expect(topic.getByRole('option', { name: '战神' })).toHaveCount(0)
  await page.getByRole('button', { name: '清除筛选' }).click()
  await expect(page.getByTestId('result-card').first()).toBeVisible()
  await page.goto('/#/home?static')
  await page.getByTestId('cat-chongsheng').click()
  await expect(page.getByTestId('result-card').first()).toContainText('重生')
})

test('Tab keeps the home resume card visible at 220% in the minimum window', async ({ page, browserName }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.goto('/#/home?static')
  await page.locator('html').evaluate(element => element.style.setProperty('--user-zoom', '2.2'))
  const resume = page.getByTestId('resume-card').first()
  await expect(resume).toBeEnabled()
  const tabKey = controlsTabKey(browserName)
  for (let step = 0; step < 100; step++) {
    await page.keyboard.press(tabKey)
    if (await resume.evaluate(element => document.activeElement === element)) break
  }
  await expect(resume).toBeFocused()
  await expect(resume).toBeInViewport({ ratio: 0.9 })
  const content = page.locator('main#content')
  expect(await content.evaluate(element => element.scrollLeft)).toBe(0)
  expect(await content.evaluate(element => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1)
  await page.screenshot({ path: testInfo.outputPath('resume-focused.png') })
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/#\/watch\/d1\/32/)
})
