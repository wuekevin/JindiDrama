import { test as base, expect } from '@playwright/test'
// 视觉验收与 UI 逻辑测试使用确定性的“模拟画面”；真实媒体在 11-p1-playback.spec.ts 单独验证。
export const test = base.extend({ page: async ({ page }, use) => { await page.addInitScript(() => { (window as any).__JD_MEDIA = 'sim' }); await use(page) } })
export { expect }
