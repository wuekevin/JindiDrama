import { test, expect } from './fixtures'

test('browser shares an actual detail and paused episode link', async ({ context, page }, testInfo) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await page.goto('/#/drama/d1')
  await page.getByTestId('share').click()
  const readClipboard = () => page.evaluate(() => navigator.clipboard.readText())
  await expect.poll(readClipboard).toMatch(/#\/drama\/d1$/)
  await page.goto('/#/watch/d1/32?panel=comments&t=192&paused')
  await page.getByRole('button', { name: '分享' }).click()
  await expect.poll(readClipboard).toMatch(/#\/watch\/d1\/32\?paused=1$/)
  await page.goto(await readClipboard())
  await expect(page.getByTestId('page-watch')).toBeVisible()
  await expect(page.getByTestId('toggle')).toHaveAttribute('aria-label', '播放')
  await page.screenshot({ path: testInfo.outputPath('watch-share.png') })
  await page.getByRole('button', { name: '分享', exact: true }).click()
  await expect.poll(readClipboard).toMatch(/#\/watch\/d1\/32\?paused=1$/)
  await page.getByTestId('toggle').click()
  await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'playing')
  await page.evaluate(() => { window.location.hash = '/watch/d1/31?paused=1' })
  await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
})

test('failed clipboard copy does not report success', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: async () => { throw new Error('denied') } } })
  })
  await page.goto('/#/drama/d1')
  await page.getByTestId('share').click()
  await expect(page.getByTestId('toast')).toContainText('无法复制分享链接')
})

test('desktop processes cold and warm share links and pauses external playback', async ({ page }) => {
  await page.addInitScript(() => {
    let openLink: (path: string) => void
    Object.assign(window, {
      runtime: {
        EventsOn: (event: string, cb: (path: string) => void) => {
          if (event === 'jindi-open-share-link') openLink = cb
        },
        ClipboardSetText: async (value: string) => { sessionStorage.setItem('copied-link', value); return true },
      },
      go: { app: { App: {
        StorageReady: async () => false,
        MarkFrontendReady: async () => openLink('/drama/d1'),
        HomeFeed: async () => ({ todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
        Drama: async () => undefined,
        Episodes: async () => [],
        LocalDanmaku: async () => [],
      } } },
    })
    window.addEventListener('test-share', (e) => openLink((e as CustomEvent<string>).detail))
  })
  await page.goto('/')
  await page.getByTestId('share').click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('copied-link'))).toBe('jindi://drama/d1')
  const open = (detail: string) => page.evaluate((path) => window.dispatchEvent(new CustomEvent('test-share', { detail: path })), detail)
  await open('/watch/d1/1')
  const toggle = page.getByTestId('toggle')
  await expect(page).toHaveURL(/#\/watch\/d1\/1\?paused=1$/)
  await expect(toggle).toHaveAttribute('aria-label', '播放')
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-label', '暂停')
  await open('/watch/d1/1')
  await expect(toggle).toHaveAttribute('aria-label', '播放')
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-label', '暂停')
  await open('/watch/d1/2')
  await expect(page).toHaveURL(/#\/watch\/d1\/2\?paused=1$/)
  await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
  await open('/settings')
  await open('/watch/d1/9999')
  await open('/drama/d999')
  await expect(page).toHaveURL(/#\/watch\/d1\/2\?paused=1$/)
})

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
  test('keeps unsent drafts with their episode and reply target', async ({ page }) => {
    const input = page.getByTestId('comment-input')
    const reply = page.getByTestId('reply-input')
    const targets = page.getByTestId('reply-btn')
    await input.fill('第32集评论草稿')
    await targets.nth(0).click()
    await reply.fill('写给第一条的文字')
    await targets.nth(1).click()
    await expect(reply).toHaveValue('')
    await reply.fill('写给第二条的文字')
    await targets.nth(0).click()
    await expect(reply).toHaveValue('写给第一条的文字')
    await page.getByTestId('reply-send').click()
    await expect(page.getByTestId('comment').first()).toContainText('写给第一条的文字')

    await page.getByTestId('comment-close').click()
    await page.getByTestId('rtab-comments').click()
    await expect(input).toHaveValue('第32集评论草稿')
    await targets.nth(0).click()
    await expect(reply).toHaveValue('')
    await targets.nth(1).click()
    await expect(reply).toHaveValue('写给第二条的文字')

    const switchTo = (route: string) => page.evaluate((target) => { location.hash = target }, route)
    await switchTo('/watch/d1/31?panel=comments&paused')
    await expect(input).toHaveValue('')
    await expect(reply).toHaveCount(0)
    await input.fill('第31集评论草稿')
    await switchTo('/watch/d2/32?panel=comments&paused')
    await expect(input).toHaveValue('')
    await switchTo('/watch/d1/32?panel=comments&paused')
    await expect(input).toHaveValue('第32集评论草稿')
    await targets.nth(1).click()
    await expect(reply).toHaveValue('写给第二条的文字')
    await page.getByTestId('comment-post').click()
    await expect(input).toHaveValue('')
    await expect(page.getByTestId('comment').first()).toContainText('第32集评论草稿')
    await switchTo('/watch/d1/31?panel=comments&paused')
    await expect(input).toHaveValue('第31集评论草稿')
  })

  test('does not submit while an input method is confirming text', async ({ page }) => {
    const input = page.getByTestId('comment-input')
    await input.fill('输入法确认')
    await input.dispatchEvent('keydown', { key: 'Enter', code: 'Enter', ctrlKey: true, isComposing: true })
    await input.dispatchEvent('keydown', { key: 'Escape', code: 'Escape', isComposing: true })
    await expect(input).toHaveValue('输入法确认')
    await expect(page.getByTestId('comment-total')).toContainText('3,217')

    const first = page.getByTestId('comment').first()
    await first.getByTestId('reply-btn').click()
    const reply = first.getByTestId('reply-input')
    await reply.fill('输入法回复')
    await reply.dispatchEvent('keydown', { key: 'Enter', code: 'Enter', keyCode: 229 })
    await expect(reply).toHaveValue('输入法回复')
    await expect(first).not.toContainText('输入法回复')
    await reply.press('Enter')
    await expect(first).toContainText('输入法回复')
    await input.press('Control+Enter')
    await expect(page.getByTestId('comment').first()).toContainText('输入法确认')
  })

  test('sort hot reorders; author filter', async ({ page }) => { await page.getByTestId('csort-hot').click(); await expect(page.getByTestId('comment').first()).toContainText('晚风也温柔'); await page.getByTestId('csort-author').click(); await expect(page.getByTestId('comment').first()).toContainText('作者') })
  test('load more adds comments', async ({ page }) => { await expect(page.getByTestId('comment')).toHaveCount(4); await page.getByTestId('load-more').click(); await expect(page.getByTestId('comment')).toHaveCount(8) })
  test('close returns to player', async ({ page }) => { await page.getByTestId('comment-close').click(); await expect(page.getByTestId('page-watch')).toBeVisible() })
  test('Esc closes drawer', async ({ page }) => { await page.getByTestId('comment-input').focus(); await page.keyboard.press('Escape'); await expect(page.getByTestId('page-watch')).toBeVisible() })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=comment]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/05-${w}x${h}.png` }) })
})

test('episode likes persist per episode in the browser and can be removed', async ({ page }, testInfo) => {
  await page.goto('/#/watch/d1/32?panel=comments&paused=1')
  const like = page.getByRole('button', { name: '本地点赞' })
  await expect(like).toHaveAttribute('aria-pressed', 'false')
  await like.click()
  await expect(like).toHaveAttribute('aria-pressed', 'true')
  await expect(like).toHaveText('已点赞')
  await page.reload()
  await expect(like).toHaveAttribute('aria-pressed', 'true')
  await page.screenshot({ path: testInfo.outputPath('episode-like.png') })

  await page.evaluate(() => { location.hash = '/watch/d1/31?panel=comments&paused=1' })
  await expect(like).toBeEnabled()
  await expect(like).toHaveAttribute('aria-pressed', 'false')
  await page.evaluate(() => { location.hash = '/watch/d1/32?panel=comments&paused=1' })
  await expect(like).toHaveAttribute('aria-pressed', 'true')
  await like.click()
  await expect(like).toHaveAttribute('aria-pressed', 'false')
  await page.reload()
  await expect(like).toBeEnabled()
  await expect(like).toHaveAttribute('aria-pressed', 'false')

  for (const invalid of ['0', '1.5', 'abc', '33']) {
    await page.goto(`/#/watch/d1/${invalid}?panel=comments&paused=1`)
    await expect(like).toBeDisabled()
  }
  await page.goto('/#/watch/d1/1?panel=comments&paused=1')
  await expect(like).toBeEnabled()
})

test('desktop episode likes retry reads, keep committed state on write failure, and isolate pending saves', async ({ page }) => {
  await page.addInitScript(() => {
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      LocalComments: async () => '',
      LocalDanmaku: async () => [],
      LocalEpisodeLiked: async (_id: string, episode: number) => {
        if (episode === 32 && !sessionStorage.getItem('retried-like-read')) {
          sessionStorage.setItem('retried-like-read', '1')
          throw new Error('read failed')
        }
        return sessionStorage.getItem(`liked-${episode}`) === '1'
      },
      SetLocalEpisodeLiked: async (_id: string, episode: number, liked: boolean) => {
        if (!sessionStorage.getItem('retried-like-write')) {
          sessionStorage.setItem('retried-like-write', '1')
          throw new Error('write failed')
        }
        sessionStorage.setItem('like-write-started', String(episode))
        await new Promise<void>((resolve) => window.addEventListener('finish-like-write', () => resolve(), { once: true }))
        sessionStorage.setItem(`liked-${episode}`, liked ? '1' : '0')
      },
    } } } })
  })
  await page.goto('/#/watch/d1/32?panel=comments&paused=1')
  await page.getByRole('button', { name: '重试读取本地点赞' }).click()
  const like = page.getByRole('button', { name: '本地点赞' })
  await expect(like).toBeEnabled()
  await like.click()
  await expect(page.getByTestId('toast').last()).toHaveText('无法保存本地点赞')
  await expect(like).toHaveAttribute('aria-pressed', 'false')

  await like.click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('like-write-started'))).toBe('32')
  await page.evaluate(() => { location.hash = '/watch/d1/31?panel=comments&paused=1' })
  await expect(like).toBeEnabled()
  await expect(like).toHaveAttribute('aria-pressed', 'false')
  await page.evaluate(() => window.dispatchEvent(new Event('finish-like-write')))
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('liked-32'))).toBe('1')
  await expect(like).toHaveAttribute('aria-pressed', 'false')
  await page.evaluate(() => { location.hash = '/watch/d1/32?panel=comments&paused=1' })
  await expect(like).toHaveAttribute('aria-pressed', 'true')
  await page.reload()
  await expect(like).toHaveAttribute('aria-pressed', 'true')
})
