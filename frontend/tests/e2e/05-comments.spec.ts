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

test('desktop close waits for queued comment snapshots, including edits queued during close', async ({ page }) => {
  await page.addInitScript(() => {
    let notifyClose: () => void
    let started = 0
    let release: () => void
    let releaseLike: () => void
    Object.assign(window, {
      runtime: {
        EventsOn: (name: string, listener: () => void) => {
          if (name === 'jindi-before-window-close') notifyClose = listener
        },
        EventsEmit: (name: string) => {
          if (name === 'jindi-window-close-ready') sessionStorage.setItem('comment-close-ready', 'yes')
        },
      },
      go: { app: { App: {
        StorageReady: async () => false,
        MarkFrontendReady: async () => {},
        Drama: async () => undefined,
        Episodes: async () => [],
        LocalDanmaku: async () => [],
        LocalComments: async () => '',
        LocalEpisodeLiked: async () => false,
        SetLocalEpisodeLiked: async () => {
          sessionStorage.setItem('comment-close-like-started', 'yes')
          await new Promise<void>((resolve) => { releaseLike = resolve })
        },
        SaveLocalComments: async (dramaId: string, episode: number, snapshot: string) => {
          const write = ++started
          sessionStorage.setItem('comment-writes-started', String(write))
          sessionStorage.setItem('comment-latest-snapshot', JSON.stringify({ dramaId, episode, snapshot }))
          await new Promise<void>((resolve, reject) => {
            release = () => write === 1 ? reject(new Error('test write failure')) : resolve()
          })
        },
      } } },
    })
    window.addEventListener('test-comment-close', () => notifyClose())
    window.addEventListener('test-comment-release', () => release())
    window.addEventListener('test-comment-release-like', () => releaseLike())
  })
  const read = (key: string) => page.evaluate((item) => sessionStorage.getItem(item), key)
  const releaseWrite = () => page.evaluate(() => window.dispatchEvent(new Event('test-comment-release')))
  await page.goto('/#/watch/d1/32?panel=comments&paused')
  const input = page.getByTestId('comment-input')
  const post = page.getByTestId('comment-post')
  await input.fill('退出前的评论')
  await post.click()
  await expect.poll(() => read('comment-writes-started')).toBe('1')
  const ownComment = page.getByTestId('comment').first()
  await ownComment.getByTestId('reply-btn').click()
  await ownComment.getByTestId('reply-input').fill('退出前的回复')
  await ownComment.getByTestId('reply-send').click()
  await page.getByRole('button', { name: '本地点赞' }).click()
  await expect.poll(() => read('comment-close-like-started')).toBe('yes')

  await page.evaluate(() => {
    window.dispatchEvent(new Event('test-comment-close'))
    return new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
  })
  expect(await read('comment-close-ready')).toBeNull()

  await releaseWrite()
  await expect.poll(() => read('comment-writes-started')).toBe('2')
  const error = page.getByRole('alert')
  await expect(error).toContainText('本地评论保存失败')
  await error.getByRole('button', { name: '重试' }).click()
  const previousSnapshot = await read('comment-latest-snapshot')
  await releaseWrite()
  await expect.poll(() => read('comment-writes-started')).toBe('3')
  expect(await read('comment-latest-snapshot')).toBe(previousSnapshot)
  await releaseWrite()
  await expect(error).toHaveCount(0)
  expect(await read('comment-close-ready')).toBeNull()

  await input.fill('等待期间的评论')
  await post.click()
  await expect.poll(() => read('comment-writes-started')).toBe('4')
  await page.evaluate(() => {
    window.dispatchEvent(new Event('test-comment-release-like'))
    return new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
  })
  expect(await read('comment-close-ready')).toBeNull()
  const latest = JSON.parse((await read('comment-latest-snapshot'))!)
  expect([latest.dramaId, latest.episode]).toEqual(['d1', 32])
  expect(JSON.parse(latest.snapshot)).toEqual(expect.arrayContaining([
    expect.objectContaining({ text: '等待期间的评论' }),
    expect.objectContaining({ text: '退出前的评论', replies: [expect.objectContaining({ text: '退出前的回复' })] }),
  ]))
  await releaseWrite()
  await expect.poll(() => read('comment-close-ready')).toBe('yes')
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

  test('inserts emoji at the selection and keeps focus and length limits', async ({ page }, testInfo) => {
    const editor = page.getByTestId('comment-input')
    const trigger = page.getByRole('button', { name: '表情', exact: true })
    const choices = page.getByRole('group', { name: '选择表情' })
    const smile = choices.getByRole('button', { name: '微笑' })
    await editor.fill('开头替换末尾')
    await editor.evaluate((input: HTMLTextAreaElement) => input.setSelectionRange(2, 4))
    await trigger.press('Enter')
    await expect(smile).toBeFocused()
    await expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByTestId('player')).toHaveAttribute('data-state', 'paused')
    await page.screenshot({ path: testInfo.outputPath('comment-emoji-picker.png') })
    await smile.press('Escape')
    await expect(choices).toBeHidden()
    await expect(trigger).toBeFocused()
    await expect(editor).toHaveValue('开头替换末尾')
    await trigger.click()
    await trigger.click()
    await expect(choices).toBeHidden()
    await trigger.click()
    const outside = page.getByTestId('nav-home')
    await outside.focus()
    await expect(choices).toBeHidden()
    await expect(outside).toBeFocused()

    await trigger.press('Space')
    await smile.press('Enter')
    await expect(editor).toBeFocused()
    await expect(editor).toHaveValue('开头😊末尾')
    expect(await editor.evaluate((input: HTMLTextAreaElement) => input.selectionStart)).toBe(4)
    await trigger.click()
    await choices.getByRole('button', { name: '喜欢' }).click()
    await expect(editor).toHaveValue('开头😊💖末尾')
    await trigger.click()
    await editor.focus()
    await editor.dispatchEvent('compositionstart')
    await editor.evaluate((input: HTMLTextAreaElement) => { input.value += '输入' })
    await smile.dispatchEvent('click')
    await expect(choices).toBeVisible()
    await expect(editor).toHaveValue('开头😊💖末尾输入')
    await editor.dispatchEvent('compositionend')
    await smile.click()
    await expect(editor).toHaveValue('开头😊💖末尾输入😊')
    await page.getByTestId('comment-post').click()
    await expect(page.getByTestId('comment').first()).toContainText('开头😊💖末尾输入😊')

    await editor.fill('甲'.repeat(499))
    await trigger.click()
    await smile.click()
    await expect(editor).toHaveValue('甲'.repeat(499))
    await expect(page.getByRole('alert')).toContainText('500')
    await expect(editor).toBeFocused()
    await expect(page.getByTestId('comment-count')).toHaveText('499/500')
    await editor.press('Backspace')
    await expect(page.getByRole('alert')).toHaveCount(0)
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

test('only the selected own reply is deleted, with persistence and keyboard focus', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    Object.assign(window, { __JD_MEDIA: 'sim', go: { app: { App: {
      StorageReady: async () => false,
      LocalDanmaku: async () => [],
      LocalEpisodeLiked: async () => false,
      LocalComments: async (drama: string, episode: number) =>
        sessionStorage.getItem(`comment-snapshot-${drama}-${episode}`) ?? '',
      SaveLocalComments: async (drama: string, episode: number, snapshot: string) => {
        if (sessionStorage.getItem('comment-fail-next-save')) {
          sessionStorage.removeItem('comment-fail-next-save')
          throw new Error('write failed')
        }
        sessionStorage.setItem(`comment-snapshot-${drama}-${episode}`, snapshot)
      },
    } } } })
  })
  await page.goto('/#/watch/d1/32?panel=comments&t=192&paused=1')
  const comment = page.getByTestId('comment').first()
  const replyButton = comment.getByTestId('reply-btn')
  const replies = comment.getByTestId('comment-reply')
  const count = comment.getByTestId('toggle-replies')
  const saved = () => page.evaluate(() => sessionStorage.getItem('comment-snapshot-d1-32') ?? '')

  await expect(replyButton).toHaveText('回复 (326)')
  await count.click()
  await expect(replies).toHaveCount(2)
  await expect(comment.getByRole('button', { name: '删除回复' })).toHaveCount(0)

  for (const text of ['只删除这一条', '保留这条回复']) {
    await replyButton.click()
    await comment.getByTestId('reply-input').fill(text)
    await comment.getByTestId('reply-send').click()
  }
  await expect.poll(saved).toContain('保留这条回复')
  await expect(replyButton).toHaveText('回复 (328)')
  await expect(replies).toHaveCount(4)
  const remove = replies.filter({ hasText: '只删除这一条' }).getByRole('button', { name: '删除回复' })
  await remove.focus()
  await expect(page.getByTestId('player')).toHaveAttribute('data-state', /^(ready|paused)$/)
  await page.screenshot({ path: testInfo.outputPath('comment-own-reply.png') })
  await page.evaluate(() => sessionStorage.setItem('comment-fail-next-save', '1'))
  await remove.press('Enter')

  await expect(replies).toHaveCount(3)
  await expect(replies.filter({ hasText: '只删除这一条' })).toHaveCount(0)
  await expect(replyButton).toHaveText('回复 (327)')
  await expect(replyButton).toBeFocused()
  const failedSave = page.getByRole('alert').filter({ hasText: '本地评论保存失败' })
  await expect(failedSave).toBeVisible()
  expect(await saved()).toContain('只删除这一条')
  await failedSave.getByRole('button', { name: '重试' }).click()
  await expect(failedSave).toHaveCount(0)
  expect(await saved()).not.toContain('只删除这一条')

  await page.reload()
  await expect(replyButton).toHaveText('回复 (327)')
  await count.click()
  await expect(replies).toHaveCount(3)
  await expect(replies.filter({ hasText: '保留这条回复' })).toBeVisible()
  await expect(comment.getByRole('button', { name: '删除回复' })).toHaveCount(1)
})

test('comment read failures can retry and edit older snapshots without losing replies', async ({ page }) => {
  await page.addInitScript(() => {
    Object.assign(window, { __JD_MEDIA: 'sim', go: { app: { App: {
      StorageReady: async () => false,
      LocalDanmaku: async () => [],
      LocalEpisodeLiked: async () => false,
      LocalComments: async () => {
        if (!sessionStorage.getItem('legacy-comments-ready')) throw new Error('本集评论数据无效')
        return sessionStorage.getItem('legacy-comments-saved') ?? JSON.stringify([
          { id: 'legacy-comment', text: '旧版主评论', replies: [{ id: 'legacy-reply', text: '旧版回复' }] },
          { id: 'legacy-text-only', text: '旧版纯文本评论' },
        ])
      },
      SaveLocalComments: async (_drama: string, _episode: number, snapshot: string) =>
        sessionStorage.setItem('legacy-comments-saved', snapshot),
    } } } })
  })
  await page.goto('/#/watch/d1/32?panel=comments&paused=1')
  const panel = page.getByTestId('comment-panel')
  const readFailure = panel.getByRole('alert').filter({ hasText: '本地评论读取失败' })
  await expect(readFailure).toBeVisible()
  await panel.getByTestId('comment-input').fill('重试前的草稿')
  await expect(panel.getByTestId('comment-post')).toBeDisabled()
  const saved = () => page.evaluate(() => sessionStorage.getItem('legacy-comments-saved') ?? '')
  expect(await saved()).toBe('')

  await page.evaluate(() => sessionStorage.setItem('legacy-comments-ready', '1'))
  await readFailure.getByRole('button', { name: '重试' }).click()
  await expect(readFailure).toHaveCount(0)
  await expect(panel.getByText('旧版纯文本评论')).toBeVisible()
  const legacy = panel.getByTestId('comment').filter({ hasText: '旧版主评论' })
  await expect(legacy.getByTestId('reply-btn')).toHaveText('回复 (1)')
  await legacy.getByTestId('toggle-replies').click()
  await expect(legacy.getByText('旧版回复')).toBeVisible()
  await expect(legacy.getByRole('button', { name: '删除回复' })).toHaveCount(0)
  await panel.getByTestId('comment-post').click()
  await expect.poll(saved).toContain('重试前的草稿')
  expect(await saved()).toContain('旧版回复')
  expect(await saved()).toContain('旧版纯文本评论')

  await page.reload()
  await expect(panel.getByText('重试前的草稿')).toBeVisible()
  await legacy.getByTestId('toggle-replies').click()
  await expect(legacy.getByText('旧版回复')).toBeVisible()
})
