import { nextTick } from 'vue'
import { waitForWindowWrites } from './index'

export const BEFORE_WINDOW_CLOSE = 'jindi-before-window-close'

export function registerWindowClose() {
  const runtime = (window as Window & { runtime?: {
    EventsOn(name: string, callback: () => void): void
    EventsEmit(name: string): void
  } }).runtime
  if (!runtime?.EventsOn || !runtime.EventsEmit) return
  runtime.EventsOn(BEFORE_WINDOW_CLOSE, async () => {
    window.dispatchEvent(new Event(BEFORE_WINDOW_CLOSE))
    try {
      await nextTick()
      await waitForWindowWrites()
    } finally {
      runtime.EventsEmit('jindi-window-close-ready')
    }
  })
}
