import { expect, test } from '@playwright/test'

test('licensed directory setup preserves current media until restart and supports removal', async ({ page }, testInfo) => {
  let activeDirectory = ''
  let savedDirectory = ''
  let response: 'cancel' | 'invalid' | 'choose' = 'cancel'
  const chosenDirectory = '/Media/授权短剧'
  const configuration = () => ({
    startupDirectory: activeDirectory, activeDirectory, savedDirectory, managed: false, canConfigure: true, restartRequired: activeDirectory !== savedDirectory,
  })
  await page.exposeFunction('testLicensedConfiguration', configuration)
  await page.exposeFunction('testChooseLicensedDirectory', () => {
    if (response === 'invalid') throw new Error('授权片库缺少 catalog.json')
    if (response === 'choose') savedDirectory = chosenDirectory
    return configuration()
  })
  await page.exposeFunction('testClearLicensedDirectory', () => {
    savedDirectory = ''
    return configuration()
  })
  await page.addInitScript(() => {
    const fixture = window as unknown as {
      testLicensedConfiguration(): Promise<{ activeDirectory: string }>
      testChooseLicensedDirectory(): Promise<unknown>
      testClearLicensedDirectory(): Promise<unknown>
    }
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      LicensedConfiguration: fixture.testLicensedConfiguration,
      ChooseLicensedDirectory: fixture.testChooseLicensedDirectory,
      ClearLicensedDirectory: fixture.testClearLicensedDirectory,
      LicensedCatalog: async () => (await fixture.testLicensedConfiguration()).activeDirectory
        ? [{ id: 'local-landscape', title: '南风纪事', summary: '', tags: [], license: '作者自有', updatedTo: 1, episodeCount: 1 }]
        : [],
    } } } })
  })
  await page.goto('/#/licensed')
  const directoryPanel = page.getByRole('region', { name: '片库目录' })
  const choose = directoryPanel.getByRole('button', { name: '选择目录' })
  await choose.click()
  await expect(choose).toBeEnabled()
  await expect(directoryPanel.getByRole('status')).toHaveCount(0)
  expect(savedDirectory).toBe('')

  response = 'invalid'
  await choose.click()
  await expect(directoryPanel.getByRole('alert')).toContainText('授权片库缺少 catalog.json')
  await expect(directoryPanel.getByRole('status')).toHaveCount(0)

  response = 'choose'
  await choose.click()
  await expect(directoryPanel.getByRole('status')).toContainText(chosenDirectory)
  await expect(directoryPanel.getByRole('status')).toContainText('重启后启用')
  await expect(page.getByText('尚未配置本地片库')).toHaveCount(0)
  await expect(directoryPanel.getByRole('alert')).toHaveCount(0)
  await page.screenshot({ path: testInfo.outputPath('licensed-directory-pending.png') })

  activeDirectory = savedDirectory
  await page.reload()
  await expect(page.getByRole('button', { name: /南风纪事/ })).toBeVisible()
  await expect(directoryPanel.getByText(/当前启用目录/)).toContainText(chosenDirectory)
  await expect(directoryPanel.getByRole('status')).toHaveCount(0)
  response = 'cancel'
  await directoryPanel.getByRole('button', { name: '更换目录' }).click()
  expect(savedDirectory).toBe(chosenDirectory)

  await directoryPanel.getByRole('button', { name: '移除配置' }).click()
  await expect(directoryPanel.getByRole('status')).toContainText('重启后停用本地片库')
  await expect(page.getByRole('button', { name: /南风纪事/ })).toBeVisible()
  activeDirectory = savedDirectory
  await page.reload()
  await expect(page.getByText('尚未配置本地片库')).toBeVisible()
  await expect(choose).toBeVisible()
})

test('managed licensed directory is displayed as read-only', async ({ page }) => {
  await page.addInitScript(() => {
    Object.assign(window, { go: { app: { App: {
      StorageReady: async () => false,
      LicensedCatalog: async () => { throw '授权片库缺少 catalog.json' },
      LicensedConfiguration: async () => ({
        startupDirectory: '/Managed/Media', activeDirectory: '', savedDirectory: '', canConfigure: false, managed: true, restartRequired: false,
      }),
    } } } })
  })
  await page.goto('/#/licensed')
  const directoryPanel = page.getByRole('region', { name: '片库目录' })
  await expect(directoryPanel.getByText('此目录由运营方配置。')).toBeVisible()
  await expect(directoryPanel.getByText('/Managed/Media')).toBeVisible()
  await expect(directoryPanel.getByText('当前启用目录')).toHaveCount(0)
  await expect(directoryPanel.getByRole('button')).toHaveCount(0)
  await expect(page.getByRole('alert')).toContainText('授权片库缺少 catalog.json')
})
