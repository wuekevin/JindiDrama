import { test, expect } from './fixtures'
test.describe('06 弹幕设置', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/watch/d1/32?panel=danmaku&t=192&paused'); await page.waitForSelector('[data-testid=danmaku-settings]'); await page.evaluate(() => localStorage.removeItem('jindi.player.v1')); await page.reload(); await page.waitForSelector('[data-testid=danmaku-settings]') })
  test('screenshot', async ({ page }) => { await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.screenshot({ path: '../design/rendered/06-弹幕设置.png' }) })
  test('master switch hides all danmaku', async ({ page }) => { await expect(page.getByTestId('danmaku-item').first()).toBeVisible(); await page.getByTestId('dm-enabled').click(); await expect(page.getByTestId('danmaku-item')).toHaveCount(0); await page.getByTestId('dm-enabled').click(); await expect(page.getByTestId('danmaku-item').first()).toBeVisible() })
  test('opacity / font size sliders change rendering', async ({ page }) => {
    await page.getByTestId('dm-size').fill('24'); await expect(page.getByTestId('danmaku-item').first()).toHaveCSS('font-size', '24px')
    await page.getByTestId('dm-opacity').fill('40'); await expect(page.getByTestId('danmaku-item').first()).toHaveCSS('opacity', '0.4')
  })
  test('area top confines danmaku to upper half', async ({ page }) => {
    await page.getByTestId('area-top').click(); const box = (await page.getByTestId('danmaku-layer').boundingBox())!
    for (const el of await page.getByTestId('danmaku-item').all()) { const b = (await el.boundingBox())!; expect(b.y + b.height / 2).toBeLessThanOrEqual(box.y + box.height / 2 + 2) }
  })
  test('hot only reduces count', async ({ page }) => { const n = await page.getByTestId('danmaku-item').count(); await page.getByTestId('dm-hot').click(); expect(await page.getByTestId('danmaku-item').count()).toBeLessThanOrEqual(n) })
  test('blocked keyword add/remove & validation', async ({ page }) => {
    await page.getByTestId('kw-add-btn').click(); await page.getByTestId('kw-confirm').click(); await expect(page.getByTestId('toast').last()).toContainText('不能为空')
    await page.getByTestId('kw-input').fill('剧透'); await page.getByTestId('kw-confirm').click(); await expect(page.getByTestId('toast').last()).toContainText('已存在')
    await page.getByTestId('kw-input').fill('太甜'); await page.getByTestId('kw-confirm').click(); await expect(page.getByTestId('kw').filter({ hasText: '太甜' })).toHaveCount(1)
    await expect(page.getByTestId('danmaku-item').filter({ hasText: '太甜' })).toHaveCount(0)
    await page.getByRole('button', { name: '移除太甜' }).click(); await expect(page.getByTestId('kw').filter({ hasText: '太甜' })).toHaveCount(0)
  })
  test('send danmaku appears as mine; blocked/empty rejected', async ({ page }) => {
    await page.getByTestId('dm-send').click(); await expect(page.getByTestId('toast').last()).toContainText('请输入')
    await page.getByTestId('dm-input').fill('这是广告'); await page.getByTestId('dm-send').click(); await expect(page.getByTestId('toast').last()).toContainText('屏蔽')
    await page.getByTestId('dm-input').fill('我的弹幕测试'); await page.getByTestId('dm-input').press('Enter'); await expect(page.getByTestId('danmaku-item').filter({ hasText: '我的弹幕测试' })).toBeVisible()
  })
  test('desktop bridge restores saved danmaku and preserves text on write failure', async ({ page }) => {
    await page.addInitScript(() => {
      const key = (id: string, ep: number) => `jd-test-local-danmaku-${id}-${ep}`
      let rejectFirst = true
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDanmaku: async (id: string, ep: number) => JSON.parse(localStorage.getItem(key(id, ep)) ?? '[]'),
        SendLocalDanmakuOnce: async (id: string, ep: number, offsetMs: number, text: string) => {
          if (rejectFirst) {
            rejectFirst = false
            throw new Error('test write failure')
          }
          const item = { id: 42, offsetMs, text }
          localStorage.setItem(key(id, ep), JSON.stringify([item]))
          return item
        },
      } } } })
    })
    await page.reload()
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
    const input = page.getByTestId('dm-input')
    await input.fill('桌面测试弹幕')
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('保存失败')
    await expect(input).toHaveValue('桌面测试弹幕')
    await input.press('Enter')
    const item = page.getByTestId('danmaku-item').filter({ hasText: '桌面测试弹幕' })
    await expect(item).toBeVisible()
    await page.reload()
    await expect(item).toBeVisible()
  })
  test('desktop retries a committed danmaku without inserting it again', async ({ page }) => {
    await page.addInitScript(() => {
      const key = 'jd-test-danmaku-receipts'
      const read = () => JSON.parse(localStorage.getItem(key) ?? '[]') as Array<{
        id: number; drama: string; episode: number; offsetMs: number; text: string; request?: string
      }>
      let rejectFirst = true
      async function save(drama: string, episode: number, offsetMs: number, text: string, request?: string) {
        const rows = read()
        const matching = request && rows.find((row) => row.request === request)
        if (matching) {
          if (matching.drama !== drama || matching.episode !== episode || matching.text !== text || matching.offsetMs !== offsetMs) {
            throw new Error('Existing request belongs to a different submission')
          }
          return matching
        }
        const row = { id: rows.length + 1, drama, episode, offsetMs, text, request }
        localStorage.setItem(key, JSON.stringify([...rows, row]))
        if (rejectFirst) {
          rejectFirst = false
          throw new Error('SQLite committed but bridge acknowledgement was lost')
        }
        return row
      }
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDanmaku: async (drama: string, episode: number) => read().filter((row) => row.drama === drama && row.episode === episode),
        SendLocalDanmaku: save,
        SendLocalDanmakuOnce: save,
      } } } })
    })
    await page.reload()
    const input = page.getByTestId('dm-input')
    const sent = '确认丢失只保存一次'
    await input.fill(sent)
    await input.evaluate((el) => el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, isComposing: true })))
    expect(await page.evaluate(() => localStorage.getItem('jd-test-danmaku-receipts'))).toBeNull()
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('保存失败')
    await expect(input).toHaveValue(sent)
    const rows = () => page.evaluate(() => JSON.parse(localStorage.getItem('jd-test-danmaku-receipts') ?? '[]'))
    const first = await rows()
    expect(first).toHaveLength(1)
    await page.getByTestId('danmaku-back').click()
    await page.getByRole('button', { name: '弹幕设置', exact: true }).click()
    await expect(input).toHaveValue(sent)
    const seek = page.getByTestId('seek')
    const originalPosition = await seek.getAttribute('aria-valuenow')
    await seek.press('ArrowRight')
    await expect(seek).not.toHaveAttribute('aria-valuenow', originalPosition ?? '')
    await input.press('Enter')
    await expect(input).toHaveValue('')
    expect(await rows()).toEqual(first)

    await input.fill(sent)
    await input.press('Enter')
    await expect(input).toHaveValue('')
    const both = await rows()
    expect(both).toHaveLength(2)
    expect(both[1].request).not.toBe(both[0].request)
    expect(both[1].offsetMs).not.toBe(both[0].offsetMs)

    await input.fill('只属于第32集的草稿')
    await page.evaluate(() => { location.hash = '/watch/d1/31?panel=danmaku&paused' })
    await expect(input).toHaveValue('')
    await input.fill(sent)
    await input.press('Enter')
    await expect(input).toHaveValue('')
    const otherEpisode = (await rows()).at(-1)
    expect(otherEpisode).toMatchObject({ episode: 31, text: sent })
    expect(otherEpisode.request).not.toBe(both[0].request)
    await page.evaluate(() => { location.hash = '/watch/d1/32?panel=danmaku&paused' })
    await expect(input).toHaveValue('只属于第32集的草稿')
  })

  test('desktop close waits for a pending danmaku acknowledgement', async ({ page }) => {
    await page.addInitScript(() => {
      let close: (attempt: string) => void
      let deadline: (attempt: string) => void
      let release: () => void
      Object.assign(window, {
        runtime: {
          EventsOn: (name: string, listener: (attempt: string) => void) => {
            if (name === 'jindi-before-window-close') close = listener
            if (name === 'jindi-window-close-deadline') deadline = listener
          },
          EventsEmit: (name: string, attempt: string, _library: boolean, danmaku: boolean) => {
            if (name === 'jindi-window-close-ready') sessionStorage.setItem('danmaku-close-ready', `${attempt}:${Boolean(danmaku)}`)
          },
        },
        go: { app: { App: {
          StorageReady: async () => false,
          LocalComments: async () => '',
          LocalDanmaku: async () => [],
          SendLocalDanmakuOnce: async (_drama: string, _episode: number, offsetMs: number, text: string) => {
            sessionStorage.setItem('danmaku-send-started', 'yes')
            await new Promise<void>((resolve) => { release = resolve })
            return { id: 5, offsetMs, text }
          },
        } } },
      })
      window.addEventListener('danmaku-test-close', (event) => close((event as CustomEvent<string>).detail))
      window.addEventListener('danmaku-test-deadline', () => deadline('second'))
      window.addEventListener('danmaku-test-release', () => release())
    })
    await page.reload()
    const input = page.getByTestId('dm-input')
    await input.fill('等待写入时关闭窗口')
    await input.press('Enter')
    const read = (key: string) => page.evaluate((name) => sessionStorage.getItem(name), key)
    await expect.poll(() => read('danmaku-send-started')).toBe('yes')
    await page.evaluate(() => {
      window.dispatchEvent(new CustomEvent('danmaku-test-close', { detail: 'first' }))
      return new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    })
    expect(await read('danmaku-close-ready')).toBeNull()
    await page.evaluate(() => window.dispatchEvent(new Event('danmaku-test-release')))
    await expect.poll(() => read('danmaku-close-ready')).toBe('first:false')
    await expect(input).toHaveValue('')
    await page.evaluate(() => sessionStorage.removeItem('danmaku-send-started'))
    await input.fill('达到关窗期限仍在保存')
    await input.press('Enter')
    await expect.poll(() => read('danmaku-send-started')).toBe('yes')
    await page.evaluate(() => {
      window.dispatchEvent(new CustomEvent('danmaku-test-close', { detail: 'second' }))
      window.dispatchEvent(new Event('danmaku-test-deadline'))
    })
    await expect.poll(() => read('danmaku-close-ready')).toBe('second:true')
    await page.evaluate(() => window.dispatchEvent(new Event('danmaku-test-release')))
  })

  test('a missing bridge callback leaves the same danmaku available to retry', async ({ page }) => {
    await page.addInitScript(() => {
      const timeout = window.setTimeout.bind(window)
      Object.defineProperty(window, 'setTimeout', {
        value: (handler: TimerHandler, ms?: number, ...args: unknown[]) => timeout(handler, ms === 8000 ? 100 : ms, ...args),
      })
      let request: string | undefined
      let closeWindow: (attempt: string) => void
      window.addEventListener('danmaku-timeout-test-close', () => closeWindow('after-timeout'))
      Object.assign(window, {
        runtime: {
          EventsOn: (name: string, listener: (attempt: string) => void) => {
            if (name === 'jindi-before-window-close') closeWindow = listener
          },
          EventsEmit: (name: string, attempt: string, _library: boolean, pending?: boolean) => {
            if (name === 'jindi-window-close-ready') sessionStorage.setItem('danmaku-timeout-close', `${attempt}:${pending}`)
          },
        },
        go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDanmaku: async () => [],
        SendLocalDanmakuOnce: async (_drama: string, _episode: number, offsetMs: number, text: string, id: string) => {
          if (!request) {
            request = id
            await new Promise(() => {})
          }
          if (request !== id) throw new Error('Unexpected new request')
          return { id: 6, offsetMs, text }
        },
      } } } })
    })
    await page.reload()
    const input = page.getByTestId('dm-input')
    await input.fill('没有返回回执的弹幕')
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('保存失败')
    await expect(input).toHaveValue('没有返回回执的弹幕')
    await page.evaluate(() => window.dispatchEvent(new Event('danmaku-timeout-test-close')))
    await expect.poll(() => page.evaluate(() => sessionStorage.getItem('danmaku-timeout-close'))).toBe('after-timeout:true')
    await input.press('Enter')
    await expect(input).toHaveValue('')
  })
  test('desktop reopens the episode and recovers a failed danmaku only for the original database', async ({ page }, testInfo) => {
    await page.addInitScript(() => {
      const profile = () => sessionStorage.getItem('outbox-profile') ?? 'a'.repeat(64)
      const key = (id: string, ep: number) => `test-danmaku-${profile()}-${id}-${ep}`
      const read = (id: string, ep: number) => JSON.parse(localStorage.getItem(key(id, ep)) ?? '[]') as Array<{
        id: number; offsetMs: number; text: string; request: string
      }>
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDataProfile: async () => profile(),
        LocalDanmaku: async (id: string, ep: number) =>
          id === 'd1' && ep === 32 && profile() === 'a'.repeat(64)
            ? [{ id: 900, offsetMs: 192000, text: '保留已成功保存的弹幕' }, ...read(id, ep)]
            : read(id, ep),
        SendLocalDanmakuOnce: async (id: string, ep: number, offsetMs: number, text: string, request: string) => {
          if (sessionStorage.getItem('outbox-locked') === 'yes') throw new Error('SQLite is locked')
          const rows = read(id, ep)
          const existing = rows.find((row) => row.request === request)
          if (existing) return existing
          const row = { id: rows.length + 1, offsetMs, text, request }
          localStorage.setItem(key(id, ep), JSON.stringify([...rows, row]))
          return row
        },
      } } } })
    })
    await page.reload()
    await page.evaluate(() => sessionStorage.setItem('outbox-locked', 'yes'))
    const input = page.getByTestId('dm-input')
    const text = '重启时补写原弹幕'
    await input.fill(text)
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('保存失败')
    await page.reload()
    await expect(page.getByRole('button', { name: '重试补写' })).toBeVisible()
    await expect(page.getByTestId('danmaku-item').filter({ hasText: '保留已成功保存的弹幕' })).toBeVisible()
    await page.screenshot({ path: testInfo.outputPath('danmaku-pending-recovery.png') })
    await page.evaluate(() => { location.hash = '/watch/d1/31?panel=danmaku&paused' })
    await expect(page.getByRole('button', { name: '重试补写' })).toHaveCount(0)
    await page.evaluate(() => { location.hash = '/watch/d1/32?panel=danmaku&t=192&paused' })
    await expect(page.getByRole('button', { name: '重试补写' })).toBeVisible()

    await page.evaluate(() => {
      sessionStorage.removeItem('outbox-locked')
      sessionStorage.setItem('outbox-profile', 'b'.repeat(64))
    })
    await page.reload()
    await expect(page.getByTestId('danmaku-settings')).toBeVisible()
    await expect(page.getByTestId('danmaku-item').filter({ hasText: text })).toHaveCount(0)

    await page.evaluate(() => sessionStorage.removeItem('outbox-profile'))
    await page.reload()
    const saved = () => page.evaluate(() => JSON.parse(localStorage.getItem(`test-danmaku-${'a'.repeat(64)}-d1-32`) ?? '[]'))
    await expect.poll(saved).toHaveLength(1)
    expect((await saved())[0]).toMatchObject({ offsetMs: 192000, text })
    await expect(page.getByTestId('danmaku-item').filter({ hasText: text })).toHaveCount(1)
    await page.reload()
    await expect(page.getByTestId('danmaku-item').filter({ hasText: text })).toHaveCount(1)
    expect(await saved()).toHaveLength(1)
    expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('jindi.danmaku.pending.v1.')))).toHaveLength(0)
  })
  test('desktop clears the pending journal after replaying a committed danmaku with a lost receipt', async ({ page }) => {
    await page.addInitScript(() => {
      const key = 'test-committed-danmaku'
      const read = () => JSON.parse(localStorage.getItem(key) ?? '[]') as Array<{
        id: number; offsetMs: number; text: string; request: string
      }>
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDataProfile: async () => 'a'.repeat(64),
        LocalDanmaku: async () => read(),
        SendLocalDanmakuOnce: async (_id: string, _ep: number, offsetMs: number, text: string, request: string) => {
          const rows = read()
          const existing = rows.find((row) => row.request === request)
          if (existing) {
            if (existing.offsetMs !== offsetMs || existing.text !== text) throw new Error('Request mismatch')
            if (sessionStorage.getItem('wrong-danmaku-receipt') === text) return { ...existing, offsetMs: offsetMs + 1 }
            return existing
          }
          localStorage.setItem(key, JSON.stringify([...rows, { id: rows.length + 1, offsetMs, text, request }]))
          throw new Error('Callback disconnected after SQLite committed')
        },
      } } } })
    })
    await page.reload()
    const input = page.getByTestId('dm-input')
    const text = '正常化后回执丢失的本地弹幕'
    await input.fill(`\u0085${text}\u0085`)
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('保存失败')
    await input.fill('另一条仍能独立补写')
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('保存失败')
    const pending = () => page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('jindi.danmaku.pending.v1.')))
    expect(await pending()).toHaveLength(2)
    await page.evaluate((original) => sessionStorage.setItem('wrong-danmaku-receipt', original), text)
    await page.reload()
    const retry = page.getByRole('button', { name: '重试补写' })
    await expect(retry).toBeVisible()
    await expect.poll(pending).toHaveLength(1)
    await page.evaluate(() => sessionStorage.removeItem('wrong-danmaku-receipt'))
    await retry.click()
    await expect.poll(pending).toHaveLength(0)
    await expect(page.getByTestId('danmaku-item').filter({ hasText: text })).toHaveCount(1)
    const rows = await page.evaluate(() => JSON.parse(localStorage.getItem('test-committed-danmaku') ?? '[]'))
    expect(rows).toHaveLength(2)
    expect(rows[0].text).toBe(text)
  })

  test('desktop warns about a danmaku held only in memory when both stores fail', async ({ page }) => {
    await page.addInitScript(() => {
      const setItem = Storage.prototype.setItem
      Storage.prototype.setItem = function (key, value) {
        if (key.startsWith('jindi.danmaku.pending.v1.')) throw new Error('WebView quota denied')
        setItem.call(this, key, value)
      }
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDataProfile: async () => 'a'.repeat(64),
        LocalDanmaku: async () => [],
        SendLocalDanmakuOnce: async (_id: string, _ep: number, offsetMs: number, text: string) => {
          if (sessionStorage.getItem('allow-memory-danmaku') !== 'yes') throw new Error('SQLite write failed')
          const writes = JSON.parse(sessionStorage.getItem('memory-danmaku-confirmed') ?? '[]') as string[]
          sessionStorage.setItem('memory-danmaku-confirmed', JSON.stringify([...writes, text]))
          return { id: writes.length + 1, offsetMs, text }
        },
      } } } })
    })
    await page.reload()
    const input = page.getByTestId('dm-input')
    await input.fill('只保存在当前窗口')
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('退出后可能丢失这一条')
    await expect(input).toHaveValue('只保存在当前窗口')
    await input.fill('第二条仍可补写')
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('退出后可能丢失这一条')
    await page.evaluate(() => sessionStorage.setItem('allow-memory-danmaku', 'yes'))
    await page.getByRole('button', { name: '重试补写' }).click()
    await expect.poll(() => page.evaluate(() => sessionStorage.getItem('memory-danmaku-confirmed')))
      .toBe(JSON.stringify(['只保存在当前窗口', '第二条仍可补写']))
    await expect(input).toHaveValue('')
  })
  test('desktop retries a memory-only danmaku when the profile lookup stays unavailable', async ({ page }) => {
    await page.addInitScript(() => {
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        LocalComments: async () => '',
        LocalDataProfile: async () => { throw new Error('Database profile is unavailable') },
        LocalDanmaku: async () => [],
        SendLocalDanmakuOnce: async (_id: string, _ep: number, offsetMs: number, text: string) => {
          if (sessionStorage.getItem('permit-volatile-retry') !== 'yes') throw new Error('SQLite write failed')
          return { id: 1, offsetMs, text }
        },
      } } } })
    })
    await page.reload()
    const input = page.getByTestId('dm-input')
    await input.fill('配置暂不可用也可补写')
    await input.press('Enter')
    await expect(page.getByTestId('toast').last()).toContainText('退出后可能丢失这一条')
    await page.evaluate(() => sessionStorage.setItem('permit-volatile-retry', 'yes'))
    await page.getByRole('button', { name: '重试补写' }).click()
    await expect(input).toHaveValue('')
  })
  test('settings persist after reload', async ({ page }) => { await page.getByTestId('dm-size').fill('20'); await page.reload(); await expect(page.getByTestId('dm-size')).toHaveValue('20') })
  test('back returns to player', async ({ page }) => { await page.getByTestId('danmaku-back').click(); await expect(page.getByTestId('page-watch')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=danmaku-settings]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/06-${w}x${h}.png` }) })
})
