import { test as base, expect, type Locator, type Page } from '@playwright/test'
// 视觉验收与 UI 逻辑测试使用确定性的“模拟画面”；真实媒体在 11-p1-playback.spec.ts 单独验证。
export const test = base.extend({ page: async ({ page }, use) => { await page.addInitScript(() => { (window as any).__JD_MEDIA = 'sim' }); await use(page) } })
export { expect }

export function controlsTabKey(browserName: string) {
  return browserName === 'webkit' && process.platform === 'darwin' ? 'Alt+Tab' : 'Tab'
}

export function deferPanelResize(page: Page, panelId: string) {
  return page.addInitScript((id) => {
    const BrowserResizeObserver = window.ResizeObserver
    window.ResizeObserver = class extends BrowserResizeObserver {
      observe(target: Element, options?: ResizeObserverOptions) {
        if (target.getAttribute('data-testid') !== id) super.observe(target, options)
      }
    }
  }, panelId)
}

export async function checkpointEarlyPlayback(video: Locator, eventName: string) {
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2)
  return video.evaluate(async (media: HTMLVideoElement, eventName) => {
    const read = () => JSON.parse(localStorage.getItem('jindi.library.v1') ?? '{"history":[]}').history
    media.muted = true
    const started = new Promise<void>((resolve) => {
      const sample = () => {
        if (media.currentTime <= 0.05) return
        media.removeEventListener('timeupdate', sample)
        resolve()
      }
      media.addEventListener('timeupdate', sample)
    })
    await media.play()
    await started
    const before = read()
    const positionSec = media.currentTime
    if (eventName === 'minimize') {
      const button = document.querySelector<HTMLButtonElement>('button[aria-label="最小化"]')
      if (!button) throw new Error('Missing minimize button')
      button.click()
    } else {
      window.dispatchEvent(new Event(eventName))
    }
    await Promise.resolve()
    return { before, positionSec, saved: read(), paused: media.paused }
  }, eventName)
}
