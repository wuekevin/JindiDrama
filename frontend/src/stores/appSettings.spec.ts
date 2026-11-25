import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppSettings, normalizeKey } from './appSettings'
describe('app settings', () => {
  beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
  it('normalizes key combos', () => { expect(normalizeKey({ key: 's', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false })).toBe('Ctrl + S'); expect(normalizeKey({ key: ' ', ctrlKey: false, shiftKey: false, altKey: false, metaKey: false })).toBe('空格'); expect(normalizeKey({ key: 'Control', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false })).toBeNull() })
  it('rebind detects conflicts', () => { const a = useAppSettings(); expect(a.rebind('danmaku', 'F')).toBe('全屏 / 退出全屏'); expect(a.s.shortcuts.danmaku).toBe('D'); expect(a.rebind('danmaku', 'B')).toBeNull(); expect(a.s.shortcuts.danmaku).toBe('B') })
  it('retains modified Enter and repairs legacy Tab when its default is free', () => {
    localStorage.setItem('jindi.settings.v1', JSON.stringify({
      shortcuts: { playPause: 'Ctrl + Tab', next: '空格', screenshot: 'Ctrl + Enter', danmaku: 'Tab' },
    }))
    const { shortcuts } = useAppSettings().s
    expect(shortcuts).toMatchObject({ playPause: '', next: '空格', screenshot: 'Ctrl + Enter', danmaku: 'D' })
    expect(normalizeKey({ key: 'Enter', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false })).toBe('Ctrl + Enter')
    expect(normalizeKey({ key: '\u0013', code: 'KeyS', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false })).toBe('Ctrl + S')
  })
  it('restores legacy WebKit control characters as letter shortcuts', () => {
    localStorage.setItem('jindi.settings.v1', JSON.stringify({ shortcuts: { screenshot: 'Ctrl + \u0013' } }))
    expect(useAppSettings().s.shortcuts.screenshot).toBe('Ctrl + S')
  })
  it('reset restores defaults', () => { const a = useAppSettings(); a.s.theme = 'light'; a.reset(); expect(a.s.theme).toBe('dark') })
})
