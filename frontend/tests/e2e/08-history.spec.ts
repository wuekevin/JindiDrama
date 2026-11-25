import { test, expect } from './fixtures'
test.describe('08 观看历史', () => {
  test.beforeEach(async ({ page }) => {
    // The demo groups use relative timestamps. Anchor them away from midnight.
    await page.clock.setFixedTime(new Date(2026, 10, 26, 12))
    await page.goto('/#/history')
    await page.evaluate(() => localStorage.removeItem('jindi.library.v1'))
    await page.reload()
    await page.waitForSelector('[data-testid=history-item]')
  })
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

test('desktop history clear survives a failed write and restart', async ({ page }) => {
  await page.addInitScript(() => {
    const deleted = 'jd-test-sqlite-deleted'
    const failed = 'jd-test-sqlite-failed'
    const existing = [{ dramaId: 'd1', episode: 32, positionMs: 1200, durationMs: 5000, percent: 24, watchedAt: Date.now() }]
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => true,
      Favorites: async () => ['d1'],
      History: async () => localStorage.getItem(deleted) ? [] : existing,
      SearchHistory: async () => [],
      GetSetting: async () => '',
      SaveLibraryChanges: async ({ removed }: { removed: string[] }) => {
        if (!localStorage.getItem(failed)) {
          localStorage.setItem(failed, '1')
          throw new Error('simulated interrupted first write')
        }
        if (removed.includes('d1')) localStorage.setItem(deleted, '1')
      },
    } } } })
  })
  await page.goto('/#/history')
  await expect(page.getByTestId('history-item')).toHaveCount(1)
  await page.getByTestId('clear-all').click()
  await page.getByTestId('modal-confirm').click()
  await expect(page.getByTestId('empty')).toBeVisible()
  await expect.poll(() => page.evaluate(() => localStorage.getItem('jd-test-sqlite-failed'))).toBe('1')
  await page.reload()
  await expect(page.getByTestId('empty')).toBeVisible()
  expect(await page.evaluate(() => localStorage.getItem('jd-test-sqlite-deleted'))).toBe('1')
  await page.reload()
  await expect(page.getByTestId('empty')).toBeVisible()
})

test('desktop startup outage restores offline favorites and history without overwriting unseen records', async ({ page }) => {
  const now = Date.now()
  let ready = false
  let savedFavorites = ['d1', 'd9']
  let savedHistory = [
    { dramaId: 'd1', episode: 2, positionMs: 1200, durationMs: 5000, percent: 24, watchedAt: now - 2000 },
    { dramaId: 'd9', episode: 4, positionMs: 5000, durationMs: 16000, percent: 31, watchedAt: now - 1000 },
  ]
  let replayed = 0
  await page.exposeFunction('readDesktopRecoveryFixture', () => ({ ready, savedFavorites, savedHistory }))
  await page.exposeFunction('writeDesktopRecoveryFixture', (changes: {
    addedFavorites?: string[]; removedFavorites?: string[]; history: typeof savedHistory; removed: string[]
  }) => {
    replayed++
    const changedFavorites = new Set([...(changes.addedFavorites ?? []), ...(changes.removedFavorites ?? [])])
    savedFavorites = [...(changes.addedFavorites ?? []), ...savedFavorites.filter((id) => !changedFavorites.has(id))]
    const changedHistory = new Set([...changes.removed, ...changes.history.map(({ dramaId }) => dramaId)])
    savedHistory = [...changes.history, ...savedHistory.filter(({ dramaId }) => !changedHistory.has(dramaId))]
  })
  await page.addInitScript(({ firstTime }) => {
    const desktop = window as unknown as {
      readDesktopRecoveryFixture(): Promise<{ ready: boolean; savedFavorites: string[]; savedHistory: unknown[] }>
      writeDesktopRecoveryFixture(changes: unknown): Promise<void>
    }
    if (!localStorage.getItem('jd-test-outage-initialized')) {
      localStorage.setItem('jd-test-outage-initialized', '1')
      localStorage.setItem('jindi.library.v1', JSON.stringify({
        favorites: ['d1'],
        history: [{ dramaId: 'd1', episode: 2, positionSec: 1.2, durationSec: 5, percent: 24, watchedAt: firstTime }],
      }))
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => (await desktop.readDesktopRecoveryFixture()).ready,
      Favorites: async () => (await desktop.readDesktopRecoveryFixture()).savedFavorites,
      History: async () => (await desktop.readDesktopRecoveryFixture()).savedHistory,
      SearchHistory: async () => [],
      GetSetting: async () => '',
      SaveLibraryChanges: desktop.writeDesktopRecoveryFixture,
      LocalComments: async () => '',
      LocalDanmaku: async () => [],
    } } } })
  }, { firstTime: now - 2000 })
  await page.goto('/#/history')
  await expect(page.getByTestId('history-item')).toHaveCount(1)
  await page.getByTestId('clear-all').click()
  await page.getByTestId('modal-confirm').click()
  await expect(page.getByTestId('empty')).toBeVisible()
  await page.getByTestId('nav-favorites').click()
  await page.getByTestId('fav-unfav').click()
  await page.goto('/#/watch/d2/1?t=2&paused')
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await player.press('ArrowRight')
  await page.getByTestId('fav').click()
  await page.getByTestId('nav-history').click()
  await expect(page.getByTestId('history-item')).toHaveCount(1)
  expect(replayed).toBe(0)
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!))).toMatchObject({
    pending: true, batches: [{ intent: { addedFavorites: ['d2'], removedFavorites: ['d1'], updatedHistory: [{ dramaId: 'd2' }], removedHistory: ['d1'] } }],
  })

  ready = true
  await page.reload()
  await expect(page.getByTestId('history-item')).toHaveCount(2)
  expect(savedHistory.map(({ dramaId }) => dramaId)).toEqual(['d2', 'd9'])
  expect(savedHistory[0].positionMs).toBe(12000)
  expect(savedFavorites).toEqual(['d2', 'd9'])
  expect(replayed).toBe(1)
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!))).not.toHaveProperty('pending')
  await page.reload()
  await expect(page.getByTestId('history-item')).toHaveCount(2)
  expect(replayed).toBe(1)
})

test('a committed stale journal cannot overwrite newer history or a later offline edit', async ({ page }) => {
  type HistoryRow = { dramaId: string; episode: number; positionMs: number; durationMs: number; percent: number; watchedAt: number }
  type Changes = { history: HistoryRow[]; removed: string[]; readBack?: boolean; receipt: { writer: string; revision: number } }
  let ready = true
  let rows: HistoryRow[] = [{ dramaId: 'd1', episode: 1, positionMs: 500, durationMs: 2000, percent: 25, watchedAt: Date.now() }]
  const applied = new Map<string, number>()
  const attempted: Changes[] = []
  await page.exposeFunction('receiptScenario', () => ({ ready, rows }))
  await page.exposeFunction('applyReceiptScenario', (changes: Changes) => {
    attempted.push(changes)
    const { writer, revision } = changes.receipt
    if ((applied.get(writer) ?? 0) < revision) {
      applied.set(writer, revision)
      const replaced = new Set([...changes.removed, ...changes.history.map((row) => row.dramaId)])
      rows = [...changes.history, ...rows.filter((row) => !replaced.has(row.dramaId))]
    }
    return changes.readBack ? { favorites: [], history: rows } : null
  })
  await page.addInitScript(() => {
    const testWindow = window as unknown as {
      blockJournalCleanup?: boolean
      receiptScenario(): Promise<{ ready: boolean; rows: unknown[] }>
      applyReceiptScenario(changes: unknown): Promise<unknown>
    }
    const setItem = Storage.prototype.setItem
    Storage.prototype.setItem = function (key, value) {
      if (testWindow.blockJournalCleanup && key === 'jindi.library.v1' && !JSON.parse(value).pending) {
        throw new DOMException('test cache failure', 'QuotaExceededError')
      }
      setItem.call(this, key, value)
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => (await testWindow.receiptScenario()).ready,
      Favorites: async () => [],
      History: async () => (await testWindow.receiptScenario()).rows,
      SearchHistory: async () => [],
      GetSetting: async () => '',
      SaveLibraryChanges: testWindow.applyReceiptScenario,
      LocalComments: async () => '',
      LocalDanmaku: async () => [],
    } } } })
  })
  await page.goto('/#/history')
  await expect(page.getByTestId('history-item')).toHaveCount(1)
  await page.evaluate(() => { Object.assign(window, { blockJournalCleanup: true }) })
  await page.getByTestId('clear-all').click()
  await page.getByTestId('modal-confirm').click()
  await expect.poll(() => rows.length).toBe(0)
  const journal = () => page.evaluate(() => JSON.parse(localStorage.getItem('jindi.library.v1')!))
  const original = (await journal()).batches[0].receipt
  expect(original).toEqual(attempted[0].receipt)
  const later = { dramaId: 'd1', episode: 6, positionMs: 1700, durationMs: 2000, percent: 85, watchedAt: Date.now() + 100 }
  rows = [later]

  ready = false
  await page.reload()
  await expect(page.getByTestId('empty')).toBeVisible()
  await page.goto('/#/watch/d2/1?t=2&paused')
  const player = page.getByTestId('player')
  await expect(player).toHaveAttribute('data-state', 'paused')
  await player.press('ArrowRight')
  await page.getByTestId('nav-history').click()
  await expect(page.getByTestId('history-item')).toHaveCount(1)
  const offline = (await journal()).batches
  expect(offline).toHaveLength(2)
  expect(offline[0].receipt).toEqual(original)

  ready = true
  await page.reload()
  await expect(page.getByTestId('history-item')).toHaveCount(2)
  expect(rows.find((row) => row.dramaId === 'd1')).toEqual(later)
  expect(rows.find((row) => row.dramaId === 'd2')?.positionMs).toBe(12000)
  expect(attempted[1].receipt).toEqual(original)
  expect(attempted[2].receipt).toEqual(offline[1].receipt)
  expect(await journal()).not.toHaveProperty('pending')
})
