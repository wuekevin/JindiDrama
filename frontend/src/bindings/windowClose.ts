import { nextTick } from 'vue'
import { hasPendingDanmakuWrite, hasUnconfirmedLibrarySave, waitForWindowWrites } from './index'

export const BEFORE_WINDOW_CLOSE = 'jindi-before-window-close'
export const WINDOW_CLOSE_CANCELED = 'jindi-window-close-canceled'

let repliedAttempt: string | undefined
export const windowCloseReplyPending = () => repliedAttempt !== undefined

export function registerWindowClose() {
  const runtime = (window as Window & { runtime?: {
    EventsOn(name: string, callback: (attempt: string) => void): void
    EventsEmit(name: string, attempt: string, unconfirmedLibrary: boolean, pendingDanmaku?: boolean): void
  } }).runtime
  if (!runtime?.EventsOn || !runtime.EventsEmit) return
  const emit = runtime.EventsEmit.bind(runtime)
  let active: { id: string; reply: () => void } | undefined

  runtime.EventsOn(WINDOW_CLOSE_CANCELED, (id) => {
    if (active || repliedAttempt !== id) return
    repliedAttempt = undefined
    window.dispatchEvent(new Event(WINDOW_CLOSE_CANCELED))
  })
  runtime.EventsOn('jindi-window-close-deadline', async (id) => {
    await nextTick()
    if (active?.id === id) active?.reply()
  })
  runtime.EventsOn(BEFORE_WINDOW_CLOSE, async (id) => {
    const attempt = {
      id,
      reply() {
        if (active !== attempt) return
        active = undefined
        repliedAttempt = id
        const library = hasUnconfirmedLibrarySave()
        if (hasPendingDanmakuWrite()) emit('jindi-window-close-ready', id, library, true)
        else emit('jindi-window-close-ready', id, library)
      },
    }
    active = attempt
    window.dispatchEvent(new Event(BEFORE_WINDOW_CLOSE))
    try {
      await nextTick()
      await waitForWindowWrites()
    } finally {
      attempt.reply()
    }
  })
}
