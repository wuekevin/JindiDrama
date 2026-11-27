import { test, expect, deferPanelResize } from './fixtures'

test('desktop Apple CMS settings test, preserve failed edits, and defer activation until restart', async ({ page }, testInfo) => {
  const source = 'https://catalog.example.test/api.php/provide/vod/'
  await page.addInitScript(({ source }) => {
    const key = 'test.apple-cms'
    const startup = sessionStorage.getItem(key) ?? ''
    const read = () => {
      const savedURL = sessionStorage.getItem(key) ?? ''
      return { active: Boolean(startup), savedURL, managed: false, canConfigure: true, restartRequired: savedURL !== startup }
    }
    const check = async (url: string) => {
      if (url.startsWith('http:')) throw new Error('请输入无需凭据或查询参数的公网 HTTPS 地址')
      if (url !== source) throw new Error('fixture connection response-private')
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AppVersion: async () => 'v0.1.0',
      AutostartEnabled: async () => false,
      CacheStats: async () => null,
      AppleCMSConfiguration: async () => read(),
      TestAppleCMSConnection: check,
      SaveAppleCMSURL: async (url: string) => {
        await check(url)
        sessionStorage.setItem(key, url)
        return read()
      },
      ClearAppleCMSURL: async () => {
        sessionStorage.removeItem(key)
        return read()
      },
    } } } })
  }, { source })
  await page.goto('/#/settings')
  const section = page.getByRole('region', { name: '苹果 CMS 片源' })
  const address = section.getByRole('textbox', { name: '片源地址' })
  const testConnection = section.getByRole('button', { name: '测试连接' })
  const save = section.getByRole('button', { name: '保存片源' })
  const clear = section.getByRole('button', { name: '清除片源' })
  const demo = section.getByText('当前使用演示目录。')
  const active = section.getByText('当前已启用苹果 CMS 片源。')
  const saved = section.getByText('片源已保存，完全退出并重新打开应用后启用。')
  const cleared = section.getByText('片源已清除，完全退出并重新打开应用后恢复演示目录。')

  await expect(demo).toBeVisible()
  await expect(clear).toBeDisabled()
  await address.fill(source)
  await page.keyboard.press('Tab')
  await expect(testConnection).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(section.getByRole('status')).toHaveText('连接成功，已读取片源目录。')
  expect(await page.evaluate(() => sessionStorage.getItem('test.apple-cms'))).toBeNull()
  await page.keyboard.press('Tab')
  await expect(save).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(saved).toBeVisible()
  await expect(demo).toBeVisible()
  await expect(active).toHaveCount(0)
  await section.screenshot({ path: testInfo.outputPath('apple-cms-restart-pending.png') })

  await address.fill('http://catalog.example.test')
  await save.click()
  await expect(section.getByRole('alert')).toHaveText('请输入无需凭据或查询参数的公网 HTTPS 地址。')
  await expect(address).toHaveAttribute('aria-invalid', 'true')
  await address.fill('https://unavailable.example.test/private')
  await address.press('Enter')
  await expect(section.getByRole('alert')).toHaveText('无法测试片源连接。')
  await expect(section).not.toContainText('response-private')
  await save.click()
  await expect(section.getByRole('alert')).toHaveText('无法保存苹果 CMS 片源设置。')
  await expect(section.getByText(source, { exact: true })).toBeVisible()

  await page.reload()
  await expect(address).toHaveValue(source)
  await expect(active).toBeVisible()
  await expect(saved).toHaveCount(0)
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi: 220 })))
  await page.reload()
  await expect(page.getByTestId('page-settings')).toHaveClass(/\bcompact\b/)
  await expect(active).toBeVisible()
  expect(await section.evaluate((panel) => {
    const panelBox = panel.getBoundingClientRect()
    return Array.from(panel.querySelectorAll('input, button')).every((control) => {
      const controlBox = control.getBoundingClientRect()
      return controlBox.left >= panelBox.left - 1 && controlBox.right <= panelBox.right + 1
    })
  })).toBe(true)
  await section.screenshot({ path: testInfo.outputPath('apple-cms-compact.png') })
  await clear.click()
  await expect(active).toBeVisible()
  await expect(cleared).toBeVisible()
  await expect(address).toHaveValue('')
  await page.reload()
  await expect(demo).toBeVisible()
  await expect(cleared).toHaveCount(0)
})

for (const mode of ['preview', 'legacy bridge', 'managed'] as const) {
  test(`Apple CMS settings show only supported controls in ${mode}`, async ({ page }, testInfo) => {
    const network: string[] = []
    page.on('request', (request) => {
      if (new URL(request.url()).hostname === 'catalog.example.test') network.push(request.url())
    })
    if (mode !== 'preview') {
      await page.addInitScript((managed) => {
        const config = async () => {
          document.body.dataset.cmsSettingsRead = 'yes'
          return { active: true, savedURL: '', managed: true, canConfigure: false, restartRequired: false }
        }
        const call = async () => {
          document.body.dataset.cmsSettingsMutated = 'yes'
          throw new Error('managed settings are read-only')
        }
        Object.assign(window, { go: { app: { App: {
          StorageReady: async () => false,
          AppVersion: async () => 'v0.1.0',
          AutostartEnabled: async () => false,
          CacheStats: async () => null,
          AppleCMSConfiguration: config,
          SaveAppleCMSURL: call,
          ClearAppleCMSURL: call,
          ...(managed ? { TestAppleCMSConnection: call } : {}),
        } } } })
      }, mode === 'managed')
    }
    await page.goto('/#/settings')
    await page.getByTestId('snav-playback').click()
    await expect(page.getByTestId('media-source')).toBeVisible()
    const section = page.getByRole('region', { name: '苹果 CMS 片源' })
    if (mode === 'managed') {
      await expect(section).toContainText('当前已启用苹果 CMS 片源。')
      await expect(section).toContainText('片源由运营方配置。')
      await expect(section.getByRole('textbox')).toHaveCount(0)
      await expect(section.getByRole('button')).toHaveCount(0)
      await section.screenshot({ path: testInfo.outputPath('apple-cms-managed.png') })
    } else {
      await expect(section).toHaveCount(0)
      expect(await page.locator('body').getAttribute('data-cms-settings-read')).toBeNull()
    }
    expect(await page.locator('body').getAttribute('data-cms-settings-mutated')).toBeNull()
    expect(network).toEqual([])
  })
}

test('desktop cache directory changes only after a confirmed choice and can be reset', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    let picks = 0
    const defaultStats = { bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }
    const customStats = { ...defaultStats, path: '/external/JindiDrama-cache-qa', custom: true }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AutostartEnabled: async () => false,
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => {
        if (sessionStorage.getItem('cache-disconnected')) throw new Error('缓存目录暂不可用')
        return sessionStorage.getItem('custom-cache') ? customStats : defaultStats
      },
      ChooseCacheDirectory: async () => {
        if (++picks === 1) return null
        if (picks === 2) throw new Error('无法在所选位置创建缓存目录')
        sessionStorage.setItem('custom-cache', '1')
        return customStats
      },
      ResetCacheDirectory: async () => {
        sessionStorage.removeItem('custom-cache')
        sessionStorage.removeItem('cache-disconnected')
        return defaultStats
      },
    } } } })
  })
  await page.goto('/#/settings')
  const path = page.getByTestId('cache-path')
  const choose = page.getByTestId('change-path')
  await expect(path).toHaveValue('/native/cache')
  await choose.click()
  await expect(path).toHaveValue('/native/cache')
  await choose.click()
  await expect(page.getByTestId('toast').last()).toHaveText('无法在所选位置创建缓存目录')
  await expect(path).toHaveValue('/native/cache')
  await choose.click()
  await expect(path).toHaveValue('/external/JindiDrama-cache-qa')
  await expect(page.getByTestId('cache-limit')).toHaveValue('2')
  await page.reload()
  await expect(path).toHaveValue('/external/JindiDrama-cache-qa')
  await page.getByTestId('snav-cache').click()
  await expect(path).toBeInViewport()
  await page.screenshot({ path: testInfo.outputPath('cache-directory.png') })
  await page.evaluate(() => sessionStorage.setItem('cache-disconnected', '1'))
  await page.reload()
  await expect(page.getByText('缓存目录暂不可用', { exact: true })).toBeVisible()
  await page.getByTestId('reset-cache-directory').click()
  await expect(path).toHaveValue('/native/cache')
  await expect(page.getByTestId('reset-cache-directory')).toHaveCount(0)
})

test('browser notifications explain desktop availability and restore focus', async ({ page }) => {
  await page.goto('/#/home')
  const bell = page.getByRole('button', { name: '通知' })
  await expect(page.getByTestId('notification-unread')).toHaveCount(0)
  await bell.press('Enter')
  const dialog = page.getByRole('dialog', { name: '更新通知' })
  await expect(dialog.getByRole('status')).toHaveText('更新通知在桌面应用中可用。')
  await expect(dialog.getByRole('button', { name: '关闭' })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: '关闭' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(bell).toBeFocused()
})

test('desktop notifications share available releases, persist read state and clear stale links', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    let channel = 'stable'
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AutostartEnabled: async () => false,
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
      UpdateChannel: async () => channel,
      SetUpdateChannel: async (value: string) => {
        while (document.body.dataset.holdChannel) await new Promise((resolve) => setTimeout(resolve, 20))
        channel = value
      },
      CheckUpdate: async () => {
        while (document.body.dataset.holdCheck) await new Promise((resolve) => setTimeout(resolve, 20))
        if (document.body.dataset.failUpdate) throw new Error('fixture')
        return { current: 'v0.1.0', latest: 'v0.2.0', hasUpdate: true, noRelease: false, preview: channel === 'preview', channel,
          url: 'https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0', notes: '' }
      },
      OpenReleasePage: async (url: string) => { document.body.dataset.openedRelease = url },
    } } } })
  })
  await page.goto('/#/settings')
  const bell = page.getByRole('button', { name: '通知' })
  const unread = page.getByTestId('notification-unread')
  const dialog = page.getByRole('dialog', { name: '更新通知' })
  const check = page.getByTestId('check-update')
  await check.click()
  await expect(unread).toBeVisible()
  await expect(bell).toHaveAccessibleDescription('有未读更新通知')
  await bell.click()
  await expect(unread).toHaveCount(0)
  await expect(dialog.getByRole('status')).toHaveText('发现新版本 v0.2.0')
  await page.screenshot({ path: testInfo.outputPath('update-notifications.png') })
  await dialog.getByRole('button', { name: '前往下载 v0.2.0' }).click()
  await expect(page.locator('body')).toHaveAttribute('data-opened-release', 'https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0')
  await dialog.getByRole('button', { name: '关闭' }).click()
  await page.reload()
  await check.click()
  await expect(page.getByTestId('open-release')).toBeVisible()
  await expect(unread).toHaveCount(0)
  await page.evaluate(() => { document.body.dataset.holdChannel = '1' })
  await page.getByTestId('update-channel').selectOption('preview')
  await bell.click()
  const download = dialog.getByRole('button', { name: /v0\.2\.0/ })
  await expect(dialog.getByRole('status')).toHaveText('正在处理更新渠道…')
  await expect(download).toHaveCount(0)
  await page.evaluate(() => { delete document.body.dataset.holdChannel })
  await expect(dialog.getByRole('status')).toHaveText('暂无更新通知')
  await dialog.getByRole('button', { name: '关闭' }).click()
  await expect(page.getByTestId('open-release')).toHaveCount(0)
  await check.click()
  await expect(unread).toBeVisible()
  await bell.click()
  await expect(dialog.getByText('更新渠道 · 预发布')).toBeVisible()
  await expect(download).toHaveText('下载预发布版 v0.2.0')
  await page.evaluate(() => {
    document.body.dataset.failUpdate = '1'
    document.body.dataset.holdCheck = '1'
  })
  await dialog.getByRole('button', { name: '检查更新' }).click()
  await expect(dialog.getByRole('status')).toHaveText('正在检查更新…')
  await expect(dialog.getByRole('button', { name: '关闭' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(bell).toBeFocused()
  await page.evaluate(() => { delete document.body.dataset.holdCheck })
  await bell.click()
  await expect(dialog.getByRole('status')).toHaveText('检查更新失败，请稍后重试')
  await expect(download).toHaveCount(0)
  await expect(unread).toHaveCount(0)
})

test('reads the release notes as scrollable plain text and clears stale candidates', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    let channel = 'stable'
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AutostartEnabled: async () => false,
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
      BackupNow: async () => {
        while (document.body.dataset.holdBackup) await new Promise((resolve) => setTimeout(resolve, 20))
        document.body.dataset.backupComplete = '1'
        return '/native/backups/demo.sqlite'
      },
      UpdateChannel: async () => channel,
      SetUpdateChannel: async (next: string) => { channel = next },
      OpenReleasePage: async (url: string) => { document.body.dataset.openedRelease = url },
      CheckUpdate: async () => ({
        current: 'v0.1.0', latest: channel === 'preview' ? `v0.3.0-rc.${'a'.repeat(160)}` : 'v0.2.0',
        hasUpdate: true, noRelease: false, preview: channel === 'preview', channel,
        url: 'https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0',
        notesTruncated: channel === 'stable',
        notes: channel === 'preview' ? '  ' : `${Array.from({ length: 42 }, (_, i) => `第 ${i + 1} 项更新说明`).join('\n')}\n<b>说明到此结束</b>`,
      }),
    } } } })
  })
  await page.goto('/#/settings')
  const trigger = page.getByRole('button', { name: '版本更新日志' })
  const dialog = page.getByRole('dialog', { name: '版本更新日志' })
  const fullNotes = dialog.getByRole('button', { name: '在发布页查看完整说明' })
  await trigger.click({ timeout: 5000 })
  await expect(dialog).toContainText('暂无新版本更新日志', { timeout: 3000 })
  await dialog.getByRole('button', { name: '关闭' }).click()
  await page.getByTestId('check-update').click()
  await expect(page.getByTestId('open-release')).toBeVisible()
  await trigger.press('Enter')
  await expect(dialog).toContainText('v0.2.0')
  const notes = dialog.getByTestId('release-notes-body')
  await expect(notes).toContainText('<b>说明到此结束</b>')
  await expect(notes.locator('b')).toHaveCount(0)
  const scrollArea = dialog.locator('[tabindex="0"]')
  await expect(scrollArea).toBeFocused()
  await expect(dialog.getByText('当前仅显示部分更新说明。')).toBeVisible()
  await expect(fullNotes).toBeVisible()
  await page.screenshot({ path: '../design/rendered/10-版本更新说明.png' })
  await page.keyboard.press('End')
  await expect.poll(() => scrollArea.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((html) => { html.style.setProperty('--user-zoom', '2.2') })
  await expect(fullNotes).toBeInViewport({ ratio: 0.95 })
  await page.screenshot({ path: testInfo.outputPath('release-notes-truncated-220.png') })
  await page.keyboard.press('Tab')
  await expect(fullNotes).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('body')).toHaveAttribute('data-opened-release', 'https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0')
  await page.locator('html').evaluate((html) => { html.style.setProperty('--user-zoom', '1') })
  await page.setViewportSize({ width: 1448, height: 1086 })
  await dialog.getByRole('heading', { name: '版本更新日志' }).click()
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  await page.getByTestId('update-channel').selectOption('preview')
  await trigger.click()
  await expect(dialog).toContainText('暂无新版本更新日志')
  await expect(dialog.getByRole('heading', { name: 'v0.2.0' })).toHaveCount(0)
  await expect(dialog.getByTestId('release-notes-body')).toHaveCount(0)
  await expect(fullNotes).toHaveCount(0)
  await dialog.getByRole('button', { name: '关闭' }).click()
  await page.getByTestId('check-update').click()
  await expect(page.getByTestId('open-release')).toBeVisible()
  await trigger.click()
  await expect(dialog).toContainText('该版本未提供更新说明')
  await expect(fullNotes).toHaveCount(0)
  await expect(dialog.getByRole('heading', { name: /^v0\.3\.0-rc\./ })).toBeVisible()
  expect(await scrollArea.evaluate((el) => el.scrollWidth - el.clientWidth)).toBeLessThanOrEqual(1)
  await page.keyboard.press('Escape')

  await page.evaluate(() => { document.body.dataset.holdBackup = '1' })
  const backupButton = page.getByTestId('create-backup')
  await backupButton.click()
  await trigger.click()
  await page.evaluate(() => { delete document.body.dataset.holdBackup })
  await expect(page.locator('body')).toHaveAttribute('data-backup-complete', '1')
  const backup = page.getByRole('dialog', { name: '数据备份已创建' })
  await expect(backup).toHaveCount(0)
  await expect(scrollArea).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(backup.getByText('/native/backups/demo.sqlite')).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(backupButton).toBeFocused()
})

test('downloads a verified update through the desktop bridge and shows its path after cancellation and a write failure', async ({ page, browserName }, testInfo) => {
  const url = 'https://github.com/wuekevin/JindiDrama/releases/tag/v1.2.3'
  const nextURL = 'https://github.com/wuekevin/JindiDrama/releases/tag/v1.2.4'
  const nextPath = '/native/downloads/JindiDrama-v1.2.4.zip'
  let checks = 0
  let attempts = 0
  let cancelled = 0
  const writeFailure = '无法写入更新文件，请检查保存位置和可用空间'
  const savedPath = '/native/downloads/' + '带空格 的更新目录/'.repeat(12) + '金帝短剧.zip'
  let firstComplete!: (saved: string) => void
  let copied = ''
  await page.exposeFunction('copyUpdatePathFixture', (value: string) => { copied = value; return true })
  await page.exposeFunction('checkUpdateFixture', () => {
    checks++
    if (checks === 2) throw new Error('offline')
    const next = checks >= 4
    return { current: 'v0.1.0', latest: next ? 'v1.2.4' : 'v1.2.3', channel: checks === 5 ? 'preview' : 'stable', hasUpdate: true, url: next ? nextURL : url, downloadName: 'JindiDrama-macos-universal.zip' }
  })
  await page.exposeFunction('downloadUpdateFixture', (release: string) => {
    if (release === nextURL) return nextPath
    if (release !== url) throw new Error('wrong candidate')
    attempts++
    if (attempts === 1) return new Promise<string>((resolve) => { firstComplete = resolve })
    if (attempts === 2) throw new Error(writeFailure)
    return savedPath
  })
  await page.exposeFunction('cancelUpdateFixture', () => { cancelled++; firstComplete('') })
  await page.addInitScript(() => {
    const desktop = window as unknown as {
      checkUpdateFixture(): Promise<unknown>
      downloadUpdateFixture(url: string): Promise<string>
      cancelUpdateFixture(): Promise<void>
      copyUpdatePathFixture(value: string): Promise<boolean>
    }
    Object.assign(window, {
      runtime: { ClipboardSetText: desktop.copyUpdatePathFixture },
      go: { app: { App: {
        StorageReady: async () => false,
        AppVersion: async () => 'v0.1.0',
        CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
        AutostartEnabled: async () => false,
        UpdateChannel: async () => 'stable',
        SetUpdateChannel: async () => {},
        CheckUpdate: desktop.checkUpdateFixture,
        DownloadUpdateFile: desktop.downloadUpdateFixture,
        CancelUpdateDownload: desktop.cancelUpdateFixture,
      } } },
    })
  })
  await page.goto('/#/settings')
  const check = page.getByTestId('check-update')
  const download = page.getByRole('button', { name: '校验并下载安装包' })
  const cancel = page.getByRole('button', { name: '取消下载' })
  const saved = page.getByTestId('saved-update')
  const copy = page.getByRole('button', { name: '复制保存路径' })
  await expect(download).toHaveCount(0)
  await check.click()
  await expect(page.getByTestId('open-release')).toBeVisible()
  await download.click()
  await expect(cancel).toBeVisible()
  await expect(check).toBeDisabled()
  await expect(page.getByTestId('update-channel')).toBeDisabled()
  await cancel.click()
  await expect(download).toBeEnabled()
  expect(cancelled).toBe(1)
  await expect(saved).toHaveCount(0)
  await download.click()
  await expect(page.getByTestId('toast').last()).toContainText(writeFailure)
  await expect(download).toBeEnabled()
  await expect(saved).toHaveCount(0)
  await download.click()
  await expect(saved.getByRole('status')).toContainText('安装包已校验并保存')
  expect(attempts).toBe(3)
  await expect(saved.locator('code')).toHaveText(savedPath)
  await expect(copy).toBeInViewport()
  await expect(page.getByTestId('toast').filter({ hasText: '发现新版本' })).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('update-saved-default.png') })
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.locator('html').evaluate((el) => el.style.setProperty('--user-zoom', '2.2'))
  await expect(page.getByTestId('page-settings')).toHaveClass(/\bcompact\b/)
  await page.getByTestId('privacy-notice').focus()
  const tabKey = browserName === 'webkit' && process.platform === 'darwin' ? 'Alt+Tab' : 'Tab'
  await page.keyboard.press(tabKey)
  await expect(copy).toBeFocused()
  await expect(copy).toBeInViewport()
  const savedBounds = (await saved.boundingBox())!
  expect(savedBounds.x + savedBounds.width).toBeLessThanOrEqual(1280)
  await page.screenshot({ path: testInfo.outputPath('update-saved-220.png') })
  await copy.press('Enter')
  await expect(page.getByTestId('toast').last()).toContainText('安装包路径已复制')
  expect(copied).toBe(savedPath)

  await check.click()
  await expect(page.getByTestId('toast').last()).toContainText('检查更新失败')
  await expect(download).toHaveCount(0)
  await expect(page.getByTestId('open-release')).toHaveCount(0)
  await expect(saved.locator('code')).toHaveText(savedPath)
  await expect(saved.getByRole('status')).toContainText('v1.2.3')
  copied = ''
  await copy.click()
  await expect.poll(() => copied).toBe(savedPath)

  await check.click()
  await expect(download).toBeEnabled()
  await expect(saved.locator('code')).toHaveText(savedPath)
  await check.click()
  await expect(page.getByTestId('open-release')).toContainText('v1.2.4')
  await expect(saved.getByRole('status')).toContainText('v1.2.3')
  await expect(saved.locator('code')).toHaveText(savedPath)
  await download.click()
  await expect(saved.getByRole('status')).toContainText('v1.2.4')
  await expect(saved.locator('code')).toHaveText(nextPath)
  await expect(copy).toBeInViewport()
  const updateChannel = page.getByTestId('update-channel')
  await check.click()
  await expect(updateChannel).toHaveValue('preview')
  await expect(saved).toHaveCount(0)
  await download.click()
  await expect(saved.locator('code')).toHaveText(nextPath)
  await updateChannel.selectOption('stable')
  await expect(saved).toHaveCount(0)
})

test('restores missing library rows from a selected backup and keeps existing records', async ({ page }) => {
  const existing = { dramaId: 'd1', episode: 4, percent: 40, positionMs: 9600, durationMs: 24000, watchedAt: 1000 }
  const incoming = { dramaId: 'd2', episode: 6, percent: 25, positionMs: 6000, durationMs: 24000, watchedAt: 500 }
  let state = { favorites: ['d1'], history: [existing] }
  let reads = 0
  let writes = 0
  let action: 'cancel' | 'invalid' | 'restore' = 'cancel'
  await page.exposeFunction('readRestoreFixture', () => state)
  await page.exposeFunction('applyRestoreFixture', () => {
    reads++
    if (action === 'cancel') return null
    if (action === 'invalid') throw new Error('无法读取所选备份')
    if (state.favorites.includes('d2')) return { state, favoritesAdded: 0, historyAdded: 0 }
    state = { favorites: ['d1', 'd2'], history: [existing, incoming] }
    return { state, favoritesAdded: 1, historyAdded: 1 }
  })
  await page.exposeFunction('countRestoreWrites', () => { writes++ })
  await page.addInitScript(() => {
    const desktop = window as unknown as {
      readRestoreFixture(): Promise<{ favorites: string[]; history: unknown[] }>
      applyRestoreFixture(): Promise<unknown>
      countRestoreWrites(): Promise<void>
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => true,
      Favorites: async () => (await desktop.readRestoreFixture()).favorites,
      History: async () => (await desktop.readRestoreFixture()).history,
      SearchHistory: async () => [],
      GetSetting: async () => '',
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
      AutostartEnabled: async () => false,
      RestoreLibraryBackup: desktop.applyRestoreFixture,
      SaveLibraryChanges: desktop.countRestoreWrites,
    } } } })
  })
  await page.goto('/#/settings')
  const label = '从备份恢复收藏和历史'
  const trigger = page.getByRole('button', { name: label })
  const dialog = page.getByRole('dialog', { name: label })
  await trigger.click()
  await expect(dialog).toContainText('保留当前已有记录')
  await dialog.getByRole('button', { name: '取消' }).click()
  await expect(trigger).toBeFocused()
  expect(reads).toBe(0)

  await trigger.click()
  await dialog.getByRole('button', { name: '选择备份' }).click()
  await expect(dialog).toHaveCount(0)
  expect(reads).toBe(1)
  action = 'invalid'
  await trigger.click()
  await dialog.getByRole('button', { name: '选择备份' }).click()
  await expect(dialog.getByRole('alert')).toContainText('无法读取所选备份')
  await dialog.getByRole('button', { name: '关闭' }).click()

  action = 'restore'
  await trigger.click()
  await dialog.getByRole('button', { name: '选择备份' }).click()
  await expect(dialog.getByRole('status')).toContainText('已恢复 1 项收藏和 1 条观看历史')
  await page.screenshot({ path: '../design/rendered/10-恢复收藏和历史.png' })
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  await page.goto('/#/favorites')
  await expect(page.getByTestId('fav-card')).toHaveCount(2)
  await page.getByTestId('nav-history').click()
  await expect(page.getByTestId('history-item')).toHaveCount(2)
  await expect(page.getByTestId('history-item').first()).toContainText('第4集')
  await page.reload()
  await expect(page.getByTestId('history-item')).toHaveCount(2)
  await page.goto('/#/settings')
  await trigger.click()
  await dialog.getByRole('button', { name: '选择备份' }).click()
  await expect(dialog.getByRole('status')).toContainText('备份中的收藏和观看历史已存在')
  expect(writes).toBe(0)
})

test.describe('10 应用设置', () => {
  test.beforeEach(async ({ page }) => { await page.goto('/#/settings'); await page.evaluate(() => { localStorage.removeItem('jindi.settings.v1'); localStorage.removeItem('jindi.player.v1') }); await page.reload(); await page.waitForSelector('[data-testid=page-settings]') })
  test('screenshot', async ({ page }) => { await expect(page.locator('#sec-keys')).toBeVisible(); await page.screenshot({ path: '../design/rendered/10-应用设置.png' }) })
  test('toggles & selects persist across reload', async ({ page }) => {
    await page.getByTestId('autostart').click(); await page.getByTestId('close-action').selectOption('退出程序'); await page.getByTestId('default-quality').selectOption('流畅'); await page.getByTestId('dl-concurrent').selectOption('5')
    await page.reload(); await expect(page.getByTestId('autostart')).toHaveAttribute('aria-checked', 'true'); await expect(page.getByTestId('close-action')).toHaveValue('退出程序'); await expect(page.getByTestId('default-quality')).toHaveValue('流畅'); await expect(page.getByTestId('dl-concurrent')).toHaveValue('5')
  })
  test('shows automatic decoding even with an older saved software preference', async ({ page }, testInfo) => {
    await expect(page.getByTestId('decode-mode')).toHaveText('系统自动选择')
    await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ decode: 'sw', dpi: 220 })))
    await page.setViewportSize({ width: 1280, height: 860 })
    await page.reload()
    await expect(page.getByTestId('page-settings')).toHaveClass(/\bcompact\b/)
    const playback = page.locator('#sec-playback')
    await expect(playback.getByTestId('decode-mode')).toHaveText('系统自动选择')
    await expect(playback.getByRole('radio')).toHaveCount(0)
    await playback.screenshot({ path: testInfo.outputPath('settings-decoder-220.png') })
    await playback.getByTestId('default-quality').selectOption('流畅')
    await expect.poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('jindi.settings.v1') ?? '{}')))
      .toMatchObject({ decode: 'sw', defaultQuality: '流畅' })
  })
  test('theme switch applies data-theme and persists', async ({ page }) => { await page.getByTestId('theme-light').click(); await expect(page.locator('html')).toHaveAttribute('data-theme', 'light'); await page.reload(); await expect(page.locator('html')).toHaveAttribute('data-theme', 'light'); await page.getByTestId('theme-dark').click(); await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark') })
  test('DPI slider scales UI', async ({ page }) => { const w0 = (await page.locator('.nav').boundingBox())!.width; await page.getByTestId('dpi').fill('120'); await expect.poll(async () => (await page.locator('.nav').boundingBox())!.width).toBeGreaterThan(w0 * 1.1) })
  test('danmaku settings are shared with player settings', async ({ page }) => { await page.getByTestId('s-dm-size').fill('22'); await page.goto('/#/watch/d1/32?panel=danmaku&paused&t=100'); await expect(page.getByTestId('dm-size')).toHaveValue('22') })
  test('cache: change path, clear with confirm', async ({ page }) => {
    await page.getByTestId('change-path').click(); await expect(page.getByTestId('cache-path')).not.toHaveValue('JindiDrama/Cache/Downloads')
    await page.getByTestId('clear-cache').click(); await page.getByTestId('modal-confirm').click(); await expect(page.getByTestId('cache-size')).toContainText('0.0 GB'); await expect(page.getByTestId('clear-cache')).toBeDisabled()
  })
  test('shortcut rebind: accept, conflict rejected, reset', async ({ page }) => {
    await page.getByTestId('key-danmaku').click(); await page.keyboard.press('B'); await expect(page.getByTestId('key-danmaku')).toHaveText('B')
    await page.getByTestId('key-fullscreen').click(); await page.keyboard.press('B'); await expect(page.getByTestId('toast').last()).toContainText('冲突'); await expect(page.getByTestId('key-fullscreen')).toHaveText('F')
    await page.reload(); await expect(page.getByTestId('key-danmaku')).toHaveText('B'); await page.getByTestId('reset-keys').click(); await expect(page.getByTestId('key-danmaku')).toHaveText('D')
  })
  test('Esc cancels key recording', async ({ page }) => { await page.getByTestId('key-danmaku').click(); await expect(page.getByTestId('key-danmaku')).toContainText('按下'); await page.keyboard.press('Escape'); await expect(page.getByTestId('key-danmaku')).toHaveText('D') })
  test('check update in browser preview reports unsupported (no fake result)', async ({ page }) => { await page.getByTestId('check-update').click(); await expect(page.getByTestId('toast')).toContainText('检查更新'); await expect(page.getByTestId('version')).toContainText('v0.1.0') })
  test('cache limit selectable, persists; diagnostics copy handled', async ({ page }) => {
    await page.getByTestId('cache-limit').selectOption('10'); await expect(page.getByTestId('toast').last()).toContainText('10 GB'); await page.reload(); await expect(page.getByTestId('cache-limit')).toHaveValue('10')
    await page.getByTestId('copy-diag').click(); await expect(page.getByTestId('toast').last()).toContainText(/诊断信息已复制|无法写入/)
  })
  test('privacy disclosure opens offline and restores keyboard focus', async ({ page }, testInfo) => {
    await page.context().setOffline(true)
    const trigger = page.getByTestId('privacy-notice')
    await trigger.focus()
    await trigger.press('Enter')
    const dialog = page.getByRole('dialog', { name: '金帝短剧隐私说明' })
    await expect(dialog).toContainText('SQLite')
    await expect(dialog).toContainText('GitHub')
    const text = dialog.getByText('适用范围', { exact: false })
    await expect(text).toBeFocused()
    await page.keyboard.press('Shift+Tab')
    await expect(dialog.getByRole('button', { name: '关闭' })).toBeFocused()
    await page.keyboard.press('Tab')
    await expect(text).toBeFocused()
    await page.screenshot({ path: testInfo.outputPath('privacy-dialog.png') })
    await page.keyboard.press('Escape')
    await expect(dialog).toHaveCount(0)
    await expect(trigger).toBeFocused()
  })
  test('left nav scrolls to section', async ({ page }) => { await page.getByTestId('snav-about').click(); await expect(page.getByTestId('snav-about')).toHaveClass(/on/) })
  for (const [w, h] of [[1366, 768], [1920, 1080], [2560, 1440]]) test(`responsive ${w}`, async ({ page }) => { await page.setViewportSize({ width: w, height: h }); await page.reload(); await page.waitForSelector('[data-testid=page-settings]'); expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1)).toBe(false); await page.screenshot({ path: `../design/rendered/responsive/10-${w}x${h}.png` }) })
  test('settings controls remain reachable in a scaled minimum window', async ({ page }, testInfo) => {
    const settings = page.getByTestId('page-settings')
    const firstPanel = settings.locator('#sec-general')
    const secondPanel = settings.locator('#sec-playback')
    expect((await firstPanel.boundingBox())!.y).toBeCloseTo((await secondPanel.boundingBox())!.y, 0)
    await page.evaluate(() => document.fonts.ready)
    await page.screenshot({ path: testInfo.outputPath('settings-default.png') })

    await page.setViewportSize({ width: 1280, height: 860 })
    for (const dpi of [150, 220]) {
      if (dpi === 150) {
        await page.getByTestId('dpi').press('End')
        await expect(settings).toHaveClass(/\bstacked\b/)
        await deferPanelResize(page, 'page-settings')
        await page.reload()
      } else {
        await page.evaluate((dpi) => localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi })), dpi)
        await page.reload()
      }
      await expect(page.locator('html')).toHaveCSS('--user-zoom', String(dpi / 100))
      await expect(settings).toHaveClass(dpi === 150 ? /\bstacked\b/ : /\bcompact\b/)
      const clipped = await settings.evaluate((root) => {
        const main = root.closest('main')!.getBoundingClientRect()
        return [...root.querySelectorAll('section button, section select, section input')].flatMap((control) => {
          const box = control.getBoundingClientRect()
          const panel = control.closest('section')!.getBoundingClientRect()
          return box.width && (box.left < Math.max(main.left, panel.left) - 1 ||
            box.right > Math.min(main.right, panel.right) + 1)
            ? [control.getAttribute('data-testid') || control.textContent?.trim()]
            : []
        })
      })
      expect(clipped).toEqual([])
      const heading = settings.getByRole('heading', { name: '设置', exact: true })
      expect(await heading.evaluate((element) => {
        const title = document.createRange()
        title.selectNodeContents(element)
        return title.getClientRects().length
      })).toBe(1)
      if (dpi === 220) await page.screenshot({ path: testInfo.outputPath('settings-compact-categories.png') })
      const looks = page.getByTestId('snav-look')
      const theme = page.getByTestId('theme-light')
      if (dpi === 220) {
        await looks.press('Enter')
        await expect(settings.getByRole('heading', { name: '外观' })).toBeFocused()
        await page.keyboard.press('Tab')
        await expect(page.getByTestId('theme-dark')).toBeFocused()
        await page.keyboard.press('Tab')
        await expect(theme).toBeFocused()
        await page.keyboard.press('Enter')
      } else {
        await looks.click()
        await expect(theme).toBeInViewport()
        await theme.click()
      }
      await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
      await page.screenshot({ path: testInfo.outputPath(`settings-${dpi}.png`) })

      const check = page.getByTestId('check-update')
      await page.getByTestId('snav-about').click()
      await expect(check).toBeInViewport({ ratio: 0.98 })
      await check.click()
      await expect(page.getByTestId('toast')).toContainText('预览模式不支持检查更新')
    }
  })
  for (const { layout, width, dpi, packageAvailable } of [
    { layout: 'compact', width: 1280, dpi: 220, packageAvailable: true },
    { layout: 'wide', width: 1800, dpi: 150, packageAvailable: true },
    { layout: 'wide', width: 1800, dpi: 150, packageAvailable: false },
  ]) {
    test(`long desktop release link fits the ${layout} settings panel (package ${packageAvailable})`, async ({ page }) => {
      await page.setViewportSize({ width, height: 860 })
      await page.addInitScript(({ dpi, packageAvailable }) => {
        localStorage.setItem('jindi.settings.v1', JSON.stringify({ dpi }))
        const latest = 'v1.2.3+' + 'x'.repeat(118)
        const url = 'https://github.com/wuekevin/JindiDrama/releases/tag/' + latest
        Object.assign(window, { go: { app: { App: {
          StorageReady: async () => false,
          AppVersion: async () => 'v0.1.0',
          CheckUpdate: async () => ({
            current: 'v0.1.0', latest, channel: 'stable', hasUpdate: true, url,
            downloadName: packageAvailable ? 'JindiDrama-macos-universal.zip' : undefined,
          }),
          DownloadUpdate: async () => false,
          CancelUpdateDownload: async () => {},
          OpenReleasePage: async () => { sessionStorage.setItem('opened-release', url) },
        } } } })
      }, { dpi, packageAvailable })
      await page.reload()
      await expect(page.getByTestId('page-settings')).toHaveClass(new RegExp(`\\b${layout}\\b`))
      await page.getByTestId('snav-about').click()
      await page.getByTestId('check-update').click()
      const link = page.getByTestId('open-release')
      const panel = page.locator('#sec-about')
      await expect(link).toContainText('v1.2.3+')
      const [control, bounds] = await Promise.all([link.boundingBox(), panel.boundingBox()])
      expect(control!.x + control!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width + 1)
      await link.click()
      expect(await page.evaluate(() => sessionStorage.getItem('opened-release'))).toContain('/releases/tag/v1.2.3+')
    })
  }
  test('saved light theme is applied on startup without opening settings', async ({ page }) => {
    await page.getByTestId('theme-light').click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await page.goto('/#/home'); await page.reload()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  })
  test('close-action offers only implemented options; legacy tray value migrates to 退出程序', async ({ page }) => {
    await expect(page.getByTestId('close-action').locator('option')).toHaveText(['最小化到任务栏', '退出程序'])
    await expect(page.getByText('最小化到托盘')).toHaveCount(0)
    await page.evaluate(() => localStorage.setItem('jindi.settings.v1', JSON.stringify({ closeAction: '最小化到托盘' })))
    await page.reload()
    await expect(page.getByTestId('close-action')).toHaveValue('退出程序')
  })
  test('autostart toggle works in browser preview (no desktop bridge)', async ({ page }) => {
    await page.getByTestId('autostart').click()
    await expect(page.getByTestId('autostart')).toHaveAttribute('aria-checked', 'true')
  })
})


test('desktop autostart reports failures and preserves the last confirmed value', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      SaveSetting: async () => {},
      AutostartEnabled: async () => {
        if (sessionStorage.getItem('fail-autostart-read')) throw new Error('fixture: read denied')
        return true
      },
      SetAutostart: async () => { throw '无法关闭 Windows 开机启动' },
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
    } } } })
  })
  await page.goto('/#/settings')
  const toggle = page.getByRole('switch', { name: '开机启动' })
  await expect(toggle).toHaveAttribute('aria-checked', 'true')
  await toggle.click()
  await expect(page.getByTestId('toast').last()).toContainText('无法关闭 Windows 开机启动')
  await expect(toggle).toHaveAttribute('aria-checked', 'true')
  await page.screenshot({ path: testInfo.outputPath('autostart-error.png') })

  await page.evaluate(() => sessionStorage.setItem('fail-autostart-read', '1'))
  await page.reload()
  await expect(page.getByTestId('toast').last()).toContainText('无法读取系统开机启动状态')
  await expect(toggle).toHaveCount(0)
  const retry = page.getByRole('button', { name: '重试读取开机启动状态' })
  await expect(retry).toBeEnabled()
  await page.screenshot({ path: testInfo.outputPath('autostart-read-error.png') })
  await page.evaluate(() => sessionStorage.removeItem('fail-autostart-read'))
  await retry.click()
  await expect(toggle).toHaveAttribute('aria-checked', 'true')
})

test('desktop reports failed cache and diagnostics operations without changing the displayed usage', async ({ page }) => {
  await page.addInitScript(() => {
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AutostartEnabled: async () => false,
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => ({ bytes: 2 * 1024 ** 3, files: 3, max: 5 * 1024 ** 3, path: '/native/cache' }),
      CacheClear: async () => { throw new Error('fixture: clear rejected') },
      CacheSetLimitMB: async () => { throw new Error('fixture: limit rejected') },
      DiagnosticsTail: async () => { throw new Error('fixture: read rejected') },
    } } } })
  })
  await page.goto('/#/settings')
  await expect(page.getByTestId('cache-path')).toHaveValue('/native/cache')
  await expect(page.getByTestId('change-path')).toHaveCount(0)
  await expect(page.getByTestId('cache-limit')).toHaveValue('5')
  await page.getByTestId('cache-limit').selectOption('10')
  await expect(page.getByTestId('toast').last()).toContainText('设置缓存上限失败')
  await expect(page.getByTestId('cache-limit')).toHaveValue('5')
  await page.getByTestId('clear-cache').click()
  await page.getByTestId('modal-confirm').click()
  await expect(page.getByTestId('toast').last()).toContainText('缓存清理失败')
  await expect(page.getByTestId('cache-size')).toContainText('2.0 GB（3 个文件）')
  await page.getByTestId('copy-diag').click()
  await expect(page.getByTestId('toast').last()).toContainText('无法读取诊断日志')
})

test('desktop update status only records success, and database backup shows the returned path', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    let check = 0
    let backup = 0
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AutostartEnabled: async () => false,
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
      CheckUpdate: async () => {
        if (++check === 1 || check === 3) throw new Error('fixture: update failed')
        return { current: 'v0.1.0', latest: 'v0.2.0', hasUpdate: check === 2 || check === 4, noRelease: false, url: 'https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0', notes: '' }
      },
      OpenReleasePage: async (url: string) => { document.body.dataset.openedRelease = url },
      BackupNow: async () => {
        if (++backup === 1) throw new Error('fixture: backup failed')
        if (backup === 2) return Promise.reject('备份目录已移动或替换，请确认应用数据目录后重试')
        if (backup === 3) throw new Error('备份已保存到原目录，请在原目录中查找')
        return '/native/backups/jindi-fixture.db'
      },
    } } } })
  })
  await page.goto('/#/settings')
  const check = page.getByTestId('check-update')
  const checkedAt = page.getByTestId('last-check')
  await expect(checkedAt).toHaveText('尚未检查更新')
  await check.click()
  await expect(page.getByTestId('toast').last()).toContainText('检查更新失败')
  await expect(checkedAt).toHaveText('尚未检查更新')
  await check.click()
  await expect(page.getByTestId('open-release')).toBeVisible()
  await expect(checkedAt).toContainText('上次成功检查')
  await page.getByTestId('open-release').click()
  await expect(page.locator('body')).toHaveAttribute('data-opened-release', 'https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0')
  const lastSuccess = await checkedAt.innerText()
  await check.click()
  await expect(page.getByTestId('toast').last()).toContainText('检查更新失败')
  await expect(page.getByTestId('open-release')).toHaveCount(0)
  await expect(checkedAt).toHaveText(lastSuccess)
  await check.click()
  await expect(page.getByTestId('open-release')).toBeVisible()
  await check.click()
  await expect(page.getByTestId('toast').last()).toContainText('暂无可用更新')
  await expect(page.getByTestId('open-release')).toHaveCount(0)
  await page.reload()
  await expect(checkedAt).toContainText('上次成功检查')

  const backup = page.getByTestId('create-backup')
  await backup.click()
  await expect(page.getByTestId('toast').last()).toContainText('数据库备份失败')
  await backup.click()
  await expect(page.getByTestId('toast').last()).toHaveText('备份目录已移动或替换，请确认应用数据目录后重试')
  await backup.click()
  await expect(page.getByTestId('toast').last()).toHaveText('备份已保存到原目录，请在原目录中查找')
  await backup.click()
  const dialog = page.getByRole('dialog', { name: '数据备份已创建' })
  await expect(dialog).toContainText('/native/backups/jindi-fixture.db')
  await expect(page.getByTestId('toast').filter({ hasText: '数据库备份失败' })).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('backup-dialog.png') })
  await page.keyboard.press('Escape')
  await expect(backup).toBeFocused()
})

for (const { preference, savedHistory, startPath } of [
  { preference: '我的收藏', savedHistory: true, startPath: '/favorites' },
  { preference: '上次观看', savedHistory: true, startPath: '/watch/d1/31' },
  { preference: '上次观看', savedHistory: false, startPath: '/home' },
  { preference: '推荐首页', savedHistory: true, startPath: '/home' },
]) {
  test(`desktop startup preference ${preference} with saved history ${savedHistory}`, async ({ page }) => {
    await page.addInitScript(({ preference, savedHistory }) => {
      localStorage.setItem('jindi.settings.v1', JSON.stringify({ startPage: preference, mediaSource: 'sim' }))
      if (savedHistory) localStorage.setItem('jindi.library.v1', JSON.stringify({
        favorites: ['d1'],
        history: [
          { dramaId: 'd3', episode: 31, percent: 100, watchedAt: 4 },
          { dramaId: 'd1', episode: 32, percent: 100, watchedAt: 3 },
          { dramaId: 'd1', episode: 33, percent: 10, watchedAt: 2 },
          { dramaId: 'd1', episode: 31, percent: 99, watchedAt: 1 },
        ],
      }))
      Object.assign(window, { go: { app: { App: {
        StorageReady: async () => false,
        HomeFeed: async () => ({ todayNew: [], rankings: { hot: [], rise: [], new: [] } }),
        LocalComments: async () => '',
        LocalDanmaku: async () => [],
      } } } })
    }, { preference, savedHistory })
    await page.goto('/')
    await expect.poll(() => new URL(page.url()).hash).toBe('#' + startPath)
    await page.goto('/#/history')
    await expect(page.getByTestId('page-history')).toBeVisible()
    await expect.poll(() => new URL(page.url()).hash).toBe('#/history')
  })
}

test('desktop users opt in to previews and can return to stable without keeping a preview link', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    const flag = 'test.update-channel'
    const now = Date.now()
    if (!localStorage.getItem('jindi.last-update-check')) localStorage.setItem('jindi.last-update-check', String(now - 86400000))
    const readChannel = () => localStorage.getItem(flag) ?? 'stable'
    let attempts = 0
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      AutostartEnabled: async () => false,
      AppVersion: async () => 'v1.0.0',
      CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: '/native/cache' }),
      UpdateChannel: async () => readChannel(),
      SetUpdateChannel: async (channel: string) => {
        if (++attempts === 1 && readChannel() === 'stable') throw new Error('fixture: failed store write')
        if (channel === 'preview') {
          await new Promise<void>((commit) => Object.assign(window, { commitPreviewChannel: commit }))
        }
        localStorage.setItem(flag, channel)
      },
      CheckUpdate: async () => readChannel() === 'preview'
        ? { current: 'v1.0.0', latest: 'v2.0.0-rc.10', hasUpdate: true, preview: true, channel: 'preview', url: 'https://github.com/wuekevin/JindiDrama/releases/tag/v2.0.0-rc.10', notes: '' }
        : { current: 'v1.0.0', latest: '', hasUpdate: false, preview: false, channel: 'stable', noRelease: true, url: '', notes: '' },
    } } } })
  })

  await page.goto('/#/settings')
  await page.getByTestId('snav-about').click()
  const channel = page.getByRole('combobox', { name: '更新渠道' })
  const checkedAt = page.getByTestId('last-check')
  const check = page.getByTestId('check-update')
  const release = page.getByTestId('open-release')
  await expect(channel).toHaveValue('stable')
  await expect(checkedAt).toContainText('上次成功检查')
  const stableStamp = await checkedAt.innerText()
  await channel.selectOption('preview')
  await expect(page.getByTestId('toast').last()).toContainText('无法保存更新渠道')
  await expect(channel).toHaveValue('stable')

  await channel.selectOption('preview')
  await expect(channel).toBeDisabled()
  await page.getByTestId('nav-history').click()
  await expect(page.getByTestId('page-history')).toBeVisible()
  await page.getByTestId('nav-settings').click()
  await expect(channel).toBeDisabled()
  await page.evaluate(() => {
    ;(window as Window & { commitPreviewChannel?: () => void }).commitPreviewChannel?.()
  })
  await expect(channel).toHaveValue('preview')
  await expect(checkedAt).toHaveText('尚未检查更新')
  await check.click()
  await expect(release).toContainText('下载预发布版 v2.0.0-rc.10')
  await expect(checkedAt).toContainText('上次成功检查')
  await expect(page.getByTestId('toast')).toHaveCount(0)
  await page.locator('#sec-about').screenshot({ path: testInfo.outputPath('update-channel-preview.png') })
  await page.setViewportSize({ width: 1280, height: 860 })
  await expect.poll(() => page.locator('#sec-about .ab').evaluate((panel) => panel.scrollWidth - panel.clientWidth)).toBe(0)

  await page.reload()
  await expect(channel).toHaveValue('preview')
  await expect(checkedAt).toContainText('上次成功检查')
  await check.click()
  await expect(release).toBeVisible()
  await channel.selectOption('stable')
  await expect(release).toHaveCount(0)
  await expect(checkedAt).toHaveText(stableStamp)
  await check.click()
  await expect(page.getByTestId('toast').last()).toContainText('暂无正式发布版本')
})

test('supported desktop close action hides to the system menu or falls back to minimizing', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    const record = (key: string) => sessionStorage.setItem(key, String(Number(sessionStorage.getItem(key) || '0') + 1))
    Object.assign(window, {
      go: { app: { App: {
        StorageReady: async () => true,
        Favorites: async () => [],
        History: async () => [],
        SearchHistory: async () => [],
        GetSetting: async (key: string) => sessionStorage.getItem(`native-setting-${key}`) ?? '',
        SaveSetting: async (key: string, value: string) => { sessionStorage.setItem(`native-setting-${key}`, value) },
        AutostartEnabled: async () => false,
        AppVersion: async () => 'v0.1.0',
        WindowTrayAvailable: async () => !sessionStorage.getItem('tray-unsupported'),
        HideWindowToTray: async () => {
          record('tray')
          return sessionStorage.getItem('tray') === '1'
        },
      } } },
      runtime: {
        WindowMinimise: () => record('minimized'),
        Quit: () => record('quit'),
      },
    })
  })
  await page.goto('/#/settings')
  const closeAction = page.getByTestId('close-action')
  await closeAction.selectOption('隐藏到菜单栏')
  await expect.poll(() => page.evaluate(() => JSON.parse(sessionStorage.getItem('native-setting-app') ?? '{}').closeAction)).toBe('隐藏到菜单栏')
  await page.evaluate(() => localStorage.removeItem('jindi.settings.v1'))
  await page.reload()
  await expect(closeAction).toHaveValue('隐藏到菜单栏')
  await page.screenshot({ path: testInfo.outputPath('window-status-menu.png') })

  const close = page.getByRole('button', { name: '关闭', exact: true })
  await close.click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('tray'))).toBe('1')
  expect(await page.evaluate(() => sessionStorage.getItem('minimized'))).toBeNull()
  await close.click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('minimized'))).toBe('1')
  await expect(page.getByTestId('toast').last()).toHaveText(/^(系统托盘|菜单栏)暂不可用，已最小化窗口$/)
  await page.evaluate(() => sessionStorage.setItem('tray-unsupported', '1'))
  await page.reload()
  await expect(closeAction).toHaveValue('最小化到任务栏')
  await close.click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('minimized'))).toBe('2')
  expect(await page.evaluate(() => sessionStorage.getItem('tray'))).toBe('2')
  await closeAction.selectOption('退出程序')
  await close.click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('quit'))).toBe('1')
})


test('Windows settings use the system tray label while keeping the saved close preference', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'userAgent', { value: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)' })
    Object.assign(window, {
      go: { app: { App: {
        StorageReady: async () => false,
        AutostartEnabled: async () => false,
        AppVersion: async () => 'v0.1.0',
        WindowTrayAvailable: async () => true,
        CacheStats: async () => ({ bytes: 0, files: 0, max: 2 * 1024 ** 3, path: 'C:/JindiDrama/cache' }),
        SaveSetting: async () => {},
      } } },
    })
  })
  await page.goto('/#/settings')
  const closeAction = page.getByTestId('close-action')
  await closeAction.selectOption({ label: '隐藏到系统托盘' })
  await expect(closeAction).toHaveValue('隐藏到菜单栏')
  await page.screenshot({ path: testInfo.outputPath('windows-tray.png') })
})

test('the shared native close preference is displayed only after a successful save', async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('close-test-initialized')) {
      localStorage.setItem('jindi.settings.v1', JSON.stringify({ closeAction: '最小化到任务栏' }))
      sessionStorage.setItem('close-test-initialized', '1')
    }
    Object.assign(window, {
      go: { app: { App: {
        StorageReady: async () => false,
        AppVersion: async () => 'v0.1.0',
        AutostartEnabled: async () => false,
        CacheStats: async () => null,
        WindowTrayAvailable: async () => true,
        SaveSetting: async () => {},
        WindowCloseAction: async () => sessionStorage.getItem('saved-close-action') ?? '',
        SetWindowCloseAction: async (action: string) => {
          if (sessionStorage.getItem('reject-close-action')) throw new Error('settings unavailable')
          sessionStorage.setItem('saved-close-action', action)
        },
        CloseWindow: async () => {
          const action = sessionStorage.getItem('saved-close-action')
          sessionStorage.setItem('close-action', action ?? '')
          return action !== '隐藏到菜单栏'
        },
      } } },
      runtime: {
        WindowMinimise: () => sessionStorage.setItem('legacy-close', 'minimize'),
        Quit: () => sessionStorage.setItem('legacy-close', 'quit'),
      },
    })
  })
  await page.goto('/#/settings')
  const close = page.getByRole('button', { name: '关闭', exact: true })
  const setting = page.getByTestId('close-action')
  await expect(setting).toHaveValue('最小化到任务栏')
  expect(await page.evaluate(() => sessionStorage.getItem('saved-close-action'))).toBe('最小化到任务栏')
  await setting.selectOption('隐藏到菜单栏')
  await expect(setting).toHaveValue('隐藏到菜单栏')
  await page.evaluate(() => localStorage.removeItem('jindi.settings.v1'))
  await page.reload()
  await expect(setting).toHaveValue('隐藏到菜单栏')
  await page.evaluate(() => sessionStorage.setItem('reject-close-action', '1'))
  await setting.selectOption('退出程序')
  await expect(page.getByTestId('toast').last()).toHaveText('无法保存关闭窗口设置')
  await expect(setting).toHaveValue('隐藏到菜单栏')
  await close.click()
  await expect(page.getByTestId('toast').last()).toHaveText(/^(系统托盘|菜单栏)暂不可用，已最小化窗口$/)
  expect(await page.evaluate(() => sessionStorage.getItem('close-action'))).toBe('隐藏到菜单栏')
  await page.evaluate(() => sessionStorage.removeItem('reject-close-action'))
  await setting.selectOption('退出程序')
  await expect(setting).toHaveValue('退出程序')
  await close.click()
  await expect.poll(() => page.evaluate(() => sessionStorage.getItem('close-action'))).toBe('退出程序')
  expect(await page.evaluate(() => sessionStorage.getItem('legacy-close'))).toBeNull()
})

test('backup prompts for a failed library write and succeeds after retrying the save', async ({ page }) => {
  await page.addInitScript(() => {
    let history = [{ dramaId: 'd1', episode: 32, positionMs: 12000, durationMs: 24000, percent: 50, watchedAt: Date.now() }]
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => true,
      Favorites: async () => [],
      History: async () => history,
      SearchHistory: async () => [],
      GetSetting: async () => '',
      SaveLibraryChanges: async () => {
        if (!sessionStorage.getItem('jd-backup-unlocked')) throw new Error('database is locked')
        history = []
        return { favorites: [], history }
      },
      BackupNow: async () => {
        sessionStorage.setItem('jd-backup-calls', '1')
        return '/native/backups/library.db'
      },
      AppVersion: async () => 'v0.1.0',
      CacheStats: async () => null,
      AutostartEnabled: async () => false,
    } } } })
  })

  await page.goto('/#/history')
  await expect(page.getByTestId('history-item')).toHaveCount(1)
  await page.getByTestId('clear-all').click()
  await page.getByTestId('modal-confirm').click()
  const retry = page.getByRole('button', { name: '重试保存' })
  await expect(retry).toBeVisible()

  await page.getByTestId('nav-settings').click()
  const backup = page.getByTestId('create-backup')
  await backup.click()
  await expect(page.getByText('收藏或观看历史尚未写入数据库。请先重试保存，再创建备份。')).toBeVisible()
  await expect(page.getByRole('dialog', { name: '数据备份已创建' })).toHaveCount(0)
  expect(await page.evaluate(() => sessionStorage.getItem('jd-backup-calls'))).toBeNull()

  await page.evaluate(() => sessionStorage.setItem('jd-backup-unlocked', '1'))
  await retry.click()
  await expect(retry).toHaveCount(0)
  await backup.click()
  const result = page.getByRole('dialog', { name: '数据备份已创建' })
  await expect(result.getByText('/native/backups/library.db')).toBeVisible()
  expect(await page.evaluate(() => sessionStorage.getItem('jd-backup-calls'))).toBe('1')
})
