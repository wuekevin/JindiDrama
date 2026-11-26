import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent, h, ref } from 'vue'
import { useSettingsStore } from '@/stores/player'
import { normalizeDemoQuality } from './demoMedia'
import { backoffMs, demoSources, resolveMediaMode, usePlayer, MAX_AUTO_RETRY } from './usePlayer'
afterEach(() => { delete (window as any).__JD_MEDIA })
describe('player helpers', () => {
  it('exponential backoff', () => { expect([0, 1, 2].map(backoffMs)).toEqual([500, 1000, 2000]); expect(MAX_AUTO_RETRY).toBe(2) })
  it('demo sources rotate over 3 clips and offer webm + mp4', () => { const s = demoSources(4); expect(s.map((x) => x.type)).toEqual(['video/webm', 'video/mp4']); expect(demoSources(1).at(0)!.src).not.toBe(demoSources(2).at(0)!.src); expect(demoSources(3).at(0)!.src).toBe(demoSources(6).at(0)!.src) })
  it('migrates stored quality labels to supported media variants', () => {
    expect(normalizeDemoQuality('标清 480P')).toBe('流畅')
    expect(normalizeDemoQuality('蓝光 4K')).toBe('高清')
    expect(normalizeDemoQuality(undefined)).toBe('高清')
  })
  it('media mode: setting and test override', () => { expect(resolveMediaMode('demo')).toBe('demo'); expect(resolveMediaMode('sim')).toBe('sim'); (window as any).__JD_MEDIA = 'sim'; expect(resolveMediaMode('demo')).toBe('sim') })
})

it('uses the desktop playback ticket and ignores a late ticket after switching episodes', async () => {
  vi.useFakeTimers()
  type Ticket = { sources: ReturnType<typeof demoSources> }
  let finishFirst!: (ticket: Ticket) => void
  const first = new Promise<Ticket>((resolve) => { finishFirst = resolve })
  const second = { sources: demoSources(2) }
  const resolvePlayback = vi.fn()
    .mockReturnValueOnce(first)
    .mockResolvedValueOnce(second)
  vi.stubGlobal('go', { app: { App: { ResolvePlayback: resolvePlayback } } })
  const episode = ref(1)
  let player!: ReturnType<typeof usePlayer>
  const wrapper = mount(defineComponent({
    setup() {
      player = usePlayer(ref('d16'), episode, { mode: 'demo', autoplay: false })
      return () => h('div')
    },
  }), { global: { plugins: [createPinia()] } })
  try {
    player.load()
    await vi.advanceTimersByTimeAsync(260)
    episode.value = 2
    await vi.advanceTimersByTimeAsync(260)
    expect(resolvePlayback.mock.calls).toEqual([['d16', 1], ['d16', 2]])
    expect(player.sources.value).toEqual(second.sources)
    finishFirst({ sources: demoSources(1) })
    await vi.advanceTimersByTimeAsync(0)
    expect(player.sources.value).toEqual(second.sources)
  } finally {
    wrapper.unmount()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  }
})

it('selects the quality from the desktop ticket and ignores a superseded quality request', async () => {
  vi.useFakeTimers()
  const hd = demoSources(1)
  const sd = demoSources(1, '流畅')
  let finishFirst!: (ticket: { sources: typeof hd }) => void
  const first = new Promise<{ sources: typeof hd }>((resolve) => { finishFirst = resolve })
  const resolvePlayback = vi.fn().mockReturnValueOnce(first).mockResolvedValue({ sources: [...hd, ...sd] })
  vi.stubGlobal('go', { app: { App: { ResolvePlayback: resolvePlayback, SaveSetting: vi.fn().mockResolvedValue(undefined) } } })
  const pinia = createPinia()
  const settings = useSettingsStore(pinia)
  const noManualQuality = localStorage.getItem('jindi.player.v1')
  settings.quality = '高清'
  let player!: ReturnType<typeof usePlayer>
  const wrapper = mount(defineComponent({
    setup() {
      player = usePlayer(ref('d1'), ref(1), { mode: 'demo', autoplay: false })
      return () => h('div')
    },
  }), { global: { plugins: [pinia] } })
  try {
    player.load()
    await vi.advanceTimersByTimeAsync(260)
    settings.quality = '流畅'
    await vi.advanceTimersByTimeAsync(260)
    expect(player.sources.value).toEqual(sd)
    finishFirst({ sources: hd })
    await vi.advanceTimersByTimeAsync(0)
    expect(player.sources.value).toEqual(sd)
    expect(resolvePlayback.mock.calls).toEqual([['d1', 1], ['d1', 1]])
  } finally {
    wrapper.unmount()
    if (noManualQuality === null) localStorage.removeItem('jindi.player.v1')
    else localStorage.setItem('jindi.player.v1', noManualQuality)
    vi.unstubAllGlobals()
    vi.useRealTimers()
  }
})

it.each([
  [null, 70],
  ['broken', 70],
  [-20, 0],
  [140, 100],
  [0, 0],
  [42, 42],
])('normalizes the stored player volume %j to %d', (stored, expected) => {
  const key = 'jindi.player.v1'
  const previous = localStorage.getItem(key)
  try {
    localStorage.setItem(key, JSON.stringify({ volume: stored }))
    expect(useSettingsStore(createPinia()).volume).toBe(expected)
  } finally {
    if (previous === null) localStorage.removeItem(key)
    else localStorage.setItem(key, previous)
  }
})
