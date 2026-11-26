import { afterEach, expect, it, vi } from 'vitest'
import { saveAppleCMSURL, type AppleCMSConfiguration } from './index'
import { BEFORE_WINDOW_CLOSE, registerWindowClose, WINDOW_CLOSE_CANCELED } from './windowClose'

afterEach(() => vi.unstubAllGlobals())

it.each([false, true])('reports a pending CMS write only at the deadline (deadline %s)', async (deadline) => {
  const listeners = new Map<string, (attempt: string) => void | Promise<void>>()
  const emit = vi.fn()
  vi.stubGlobal('runtime', {
    EventsOn(name: string, listener: (attempt: string) => void | Promise<void>) {
      listeners.set(name, listener)
    },
    EventsEmit: emit,
  })
  const address = 'https://catalog.example.test/api.php/provide/vod/'
  const config = { savedURL: address, active: false, managed: false, canConfigure: true, restartRequired: true }
  let finish!: (value: AppleCMSConfiguration) => void
  const nativeWrite = new Promise<AppleCMSConfiguration>((resolve) => { finish = resolve })
  const SaveAppleCMSURL = vi.fn().mockReturnValue(nativeWrite)
  vi.stubGlobal('go', { app: { App: { SaveAppleCMSURL } } })
  registerWindowClose()
  const signal = (name: string, attempt: string) => listeners.get(name)!(attempt)

  const save = saveAppleCMSURL(address)
  const close = signal(BEFORE_WINDOW_CLOSE, '1')
  await vi.waitFor(() => expect(SaveAppleCMSURL).toHaveBeenCalledWith(address))
  expect(emit).not.toHaveBeenCalled()
  if (deadline) {
    await signal('jindi-window-close-deadline', '1')
    expect(emit).toHaveBeenCalledExactlyOnceWith('jindi-window-close-ready', '1', false, false, true)
    await signal(WINDOW_CLOSE_CANCELED, '1')
  }

  finish(config)
  await expect(save).resolves.toEqual(config)
  await close
  if (deadline) {
    expect(emit).toHaveBeenCalledTimes(1)
    await signal(BEFORE_WINDOW_CLOSE, '2')
    expect(emit).toHaveBeenLastCalledWith('jindi-window-close-ready', '2', false)
    await signal(WINDOW_CLOSE_CANCELED, '2')
  } else {
    expect(emit).toHaveBeenCalledExactlyOnceWith('jindi-window-close-ready', '1', false)
    await signal(WINDOW_CLOSE_CANCELED, '1')
  }
})
