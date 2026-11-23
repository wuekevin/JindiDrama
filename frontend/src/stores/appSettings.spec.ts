import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppSettings, normalizeKey } from './appSettings'
describe('app settings', () => {
  beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
  it('normalizes key combos', () => { expect(normalizeKey({ key: 's', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false })).toBe('Ctrl + S'); expect(normalizeKey({ key: ' ', ctrlKey: false, shiftKey: false, altKey: false, metaKey: false })).toBe('空格'); expect(normalizeKey({ key: 'Control', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false })).toBeNull() })
  it('rebind detects conflicts', () => { const a = useAppSettings(); expect(a.rebind('danmaku', 'F')).toBe('全屏 / 退出全屏'); expect(a.s.shortcuts.danmaku).toBe('D'); expect(a.rebind('danmaku', 'B')).toBeNull(); expect(a.s.shortcuts.danmaku).toBe('B') })
  it('reset restores defaults', () => { const a = useAppSettings(); a.s.theme = 'light'; a.reset(); expect(a.s.theme).toBe('dark') })
})
