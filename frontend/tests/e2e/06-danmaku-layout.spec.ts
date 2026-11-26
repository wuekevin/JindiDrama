import { expect, test } from '@playwright/test'

for (const [dpi, width, height] of [[100, 1448, 1086], [150, 1280, 860], [220, 1280, 860]]) {
  test(`${dpi}% keeps visible danmaku text apart`, async ({ page }) => {
    await page.setViewportSize({ width, height })
    await page.addInitScript((value) => localStorage.setItem('jindi.settings.v1', JSON.stringify({ theme: 'light', dpi: value })), dpi)
    await page.goto('/#/watch/d1/32?t=4&paused')
    const player = page.getByTestId('player')
    await expect(player).toHaveAttribute('data-state', 'paused')
    const layer = player.getByTestId('danmaku-layer')
    await expect.poll(() => layer.evaluate((container) => {
      const surface = container.parentElement!
      const visible = container.getBoundingClientRect()
      const top = surface.querySelector('header')!.getBoundingClientRect()
      const footer = surface.querySelector('footer')!.getBoundingClientRect()
      return visible.top >= top.bottom - 1 && visible.bottom <= footer.top + 1
    })).toBe(true)
    const result = await layer.evaluate((container) => {
      const r = container.getBoundingClientRect()
      const visible = Array.from(container.querySelectorAll('[data-testid=danmaku-item]')).map((el) => {
        const b = el.getBoundingClientRect()
        return { left: Math.max(r.left, b.left), right: Math.min(r.right, b.right), top: Math.max(r.top, b.top), bottom: Math.min(r.bottom, b.bottom) }
      }).filter((b) => b.right > b.left && b.bottom > b.top)
      const overlaps = visible.flatMap((a, i) => visible.slice(i + 1).filter((b) => (
        Math.min(a.right, b.right) - Math.max(a.left, b.left) > 3 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 3
      ))).length
      return { visible: visible.length, overlaps }
    })
    expect(result.visible).toBeGreaterThan(5)
    expect(result.overlaps).toBe(0)
  })
}

test('maximum font and density stay in the selected half when resized', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.addInitScript(() => {
    localStorage.setItem('jindi.settings.v1', JSON.stringify({ theme: 'light', dpi: 220 }))
    localStorage.setItem('jindi.player.v1', JSON.stringify({ danmaku: { density: 'high', fontSize: 28, speed: 2 } }))
  })
  await page.goto('/#/watch/d1/32?panel=danmaku&t=4&paused')
  const player = page.getByTestId('player')
  const layer = player.getByTestId('danmaku-layer')
  await expect(player).toHaveAttribute('data-state', 'paused')
  for (const width of [1280, 1680, 1280]) {
    await page.setViewportSize({ width, height: 860 })
    for (const area of ['top', 'bottom']) {
      await page.getByTestId(`area-${area}`).click()
      await expect.poll(() => layer.evaluate((el, area) => {
        const surface = el.getBoundingClientRect()
        const middle = (surface.top + surface.bottom) / 2
        const visible = Array.from(el.querySelectorAll('[data-testid=danmaku-item]')).map((e) => e.getBoundingClientRect())
          .filter((e) => e.right > surface.left && e.left < surface.right)
        const inside = visible.every((r) => area === 'top' ? r.bottom <= middle + 1 : r.top >= middle - 1)
        const collisions = visible.flatMap((a, i) => visible.slice(i + 1).filter((b) =>
          Math.min(a.right, b.right) - Math.max(a.left, b.left) > 3 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 3
        )).length
        return visible.length > 0 && inside && collisions === 0
      }, area)).toBe(true)
    }
  }
})
