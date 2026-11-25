import { onBeforeUnmount, nextTick, ref, computed, watch, type Ref } from 'vue'
import { transition, episodeDuration, type PEvent, type PState } from './machine'
import { useSettingsStore } from '@/stores/player'
import { useLibraryStore } from '@/stores/library'
import { BEFORE_WINDOW_CLOSE } from '@/bindings/windowClose'
import { hasBridge, logEvent, resolvePlayback } from '@/bindings'
import { demoQualityId, demoSources } from './demoMedia'
export { demoSources } from './demoMedia'

export type MediaMode = 'sim' | 'demo'
export const MAX_AUTO_RETRY = 2
export const backoffMs = (attempt: number) => 500 * 2 ** attempt // 0.5s, 1s

export function resolveMediaMode(setting: string): MediaMode {
  const o = (window as any).__JD_MEDIA; if (o === 'sim' || o === 'demo') return o
  return setting === 'sim' ? 'sim' : 'demo'
}

type StartPoint = { sec?: number; frac?: number }

export function usePlayer(dramaId: Ref<string>, episode: Ref<number>, opts: { startOverride?: () => string | undefined; startPoint?: (dramaId: string, episode: number) => StartPoint; autoplay: boolean | (() => boolean); mode: MediaMode; failEpisode?: (ep: number) => string | null; onFinish?: () => void }) {
  const settings = useSettingsStore(); const lib = useLibraryStore()
  const state = ref<PState>('idle'); const time = ref(0); const duration = ref(opts.mode === 'sim' ? episodeDuration(episode.value) : 0); const error = ref('')
  const muted = ref(false); const buffering = ref(false); const retries = ref(0)
  const video = ref<HTMLVideoElement | null>(null)
  let loadToken = 0; let timer: number | undefined; let resolveTimer: number | undefined; let retryTimer: number | undefined; let sinceSave = 0
  const shouldAutoplay = () => typeof opts.autoplay === 'function' ? opts.autoplay() : opts.autoplay
  let pendingSeek: StartPoint = {}; let wantPlay = shouldAutoplay()
  let mediaLoadStarted = false
  let restoringPosition = false
  let savedPositionMs = 0
  let savedCompletion = false
  let manualSeek = false
  const send = (e: PEvent) => { state.value = transition(state.value, e) }
  const isDemo = opts.mode === 'demo'
  const desktop = hasBridge()
  const desktopSources = ref<ReturnType<typeof demoSources>>([])
  const sources = computed(() => desktop ? desktopSources.value : demoSources(episode.value, settings.quality))
  const snap = { d: dramaId.value, e: episode.value } // 离开页面时路由参数已变化，保存必须用快照

  function load(sec?: number, frac?: number) {
    const my = ++loadToken; clearTimeout(resolveTimer); clearTimeout(retryTimer); error.value = ''; buffering.value = false
    const requiredQuality = demoQualityId(settings.quality)
    pendingSeek = { sec, frac }
    sinceSave = 0
    duration.value = isDemo ? 0 : episodeDuration(episode.value)
    time.value = isDemo ? sec ?? 0 : Math.min(sec ?? (frac ?? 0) * duration.value, duration.value - 1)
    savedPositionMs = Math.round(time.value * 1000)
    savedCompletion = false
    restoringPosition = false
    manualSeek = false
    mediaLoadStarted = false
    if (isDemo) video.value?.pause()
    if (desktop && isDemo) desktopSources.value = []
    send('load')
    const fail = opts.failEpisode?.(episode.value)
    resolveTimer = window.setTimeout(async () => {
      if (my !== loadToken) return // 过期请求被取消
      if (fail) { error.value = fail; send('fail'); logEvent('warn', 'player', `drama=${dramaId.value} ep=${episode.value} ${fail}`); return }
      if (isDemo) {
        if (desktop) {
          try {
            const ticket = await resolvePlayback(dramaId.value, episode.value)
            if (my !== loadToken) return
            const choices = ticket?.sources?.filter((source) => source.quality === requiredQuality
              || (requiredQuality === 'hd' && !source.quality))
            if (!choices?.length) throw new Error('无可用媒体')
            desktopSources.value = choices
            await nextTick()
          } catch {
            if (my !== loadToken) return
            error.value = '播放源暂时不可用'
            send('fail')
            return
          }
        }
        if (my === loadToken && video.value) {
          mediaLoadStarted = true
          video.value.load()
        }
        return
      }
      send('resolved'); if (wantPlay) send('play'); else send('pause')
    }, 260)
  }
  function loadCurrent() {
    const point = opts.startPoint?.(snap.d, snap.e)
    load(point?.sec, point?.frac)
  }
  // ---- <video> 事件（demo 模式）----
  function onMeta() {
    const v = video.value; if (!v || !mediaLoadStarted || state.value !== 'resolving') return
    duration.value = v.duration || 0
    const t = pendingSeek.sec ?? (pendingSeek.frac !== undefined ? (pendingSeek.frac * duration.value) : 0)
    // WebKit can report metadata before the first seekable range is available.
    if (t > 0 && !v.seekable.length && v.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) return
    if (t > 0 && duration.value > 0 && v.seekable.length) {
      v.currentTime = Math.min(t, Math.max(0, duration.value - 0.05))
      restoringPosition = v.seeking
    }
    time.value = v.currentTime
    savedPositionMs = Math.round(time.value * 1000)
    v.playbackRate = settings.speed; v.volume = settings.volume / 100; v.muted = muted.value
    retries.value = 0; send('resolved')
    if (wantPlay) {
      const my = loadToken
      send('play')
      v.play().catch(() => { if (my === loadToken) send('pause') })
    } else send('pause')
  }
  function canSave() { return state.value === 'playing' || state.value === 'paused' || state.value === 'ended' }
  function onTime() {
    const media = video.value
    if (!media || !canSave()) return
    time.value = media.currentTime
    if (media.duration && duration.value !== media.duration) duration.value = media.duration
  }
  function onSeeked() {
    onTime()
    if (restoringPosition) {
      restoringPosition = false
      savedPositionMs = Math.round(time.value * 1000)
    }
  }
  function onEnded() {
    if (state.value !== 'playing' && state.value !== 'paused') return
    time.value = duration.value
    sinceSave = 0
    send('finish')
    save()
    opts.onFinish?.()
  }
  function onError() {
    const my = loadToken; if (!mediaLoadStarted || state.value === 'idle') return
    save()
    if (retries.value < MAX_AUTO_RETRY) { // 指数退避自动重试
      const wait = backoffMs(retries.value); retries.value++
      retryTimer = window.setTimeout(() => {
        if (my !== loadToken || !video.value) return
        if (duration.value > 0) pendingSeek = { sec: time.value }
        restoringPosition = false
        send('retry')
        video.value.load()
      }, wait)
      if (state.value !== 'resolving') send('load')
      return
    }
    error.value = '媒体加载失败（已自动重试）'; send('fail'); logEvent('error', 'player', `drama=${dramaId.value} ep=${episode.value} media load failed after ${retries.value} retries`)
  }
  const onWaiting = () => { buffering.value = true }
  function onMediaControl(event: Event) {
    const v = video.value
    if (!v || event.currentTarget !== v) return
    buffering.value = false
    if (v.ended || (state.value !== 'playing' && state.value !== 'paused')) return
    wantPlay = !v.paused
    send(v.paused ? 'pause' : 'play')
  }

  const retry = () => {
    retries.value = 0
    wantPlay = true
    if (duration.value > 0) load(time.value)
    else load(pendingSeek.sec, pendingSeek.frac)
  }
  function play() {
    wantPlay = true
    if (state.value === 'ended') {
      time.value = 0
      savedPositionMs = 0
      savedCompletion = false
      if (video.value) video.value.currentTime = 0
    }
    const my = loadToken
    send('play')
    video.value?.play().catch(() => { if (my === loadToken) send('pause') })
  }
  function pause() { wantPlay = false; send('pause'); video.value?.pause() }
  const toggle = () => (state.value === 'playing' ? pause() : play())
  function seek(s: number) {
    if (!canSave() || !duration.value) return
    const target = Math.max(0, Math.min(duration.value - 0.01, s))
    if (target === time.value) return
    restoringPosition = false
    manualSeek = true
    time.value = target
    if (video.value && isDemo) video.value.currentTime = time.value
    if (state.value === 'ended') { send('play'); video.value?.play().catch(() => { /* ignore */ }) }
  }
  const seekBy = (d: number) => seek(time.value + d)
  function tick() {
    if (isDemo || state.value !== 'playing') { if (isDemo && state.value === 'playing') { sinceSave += 0.2; if (sinceSave >= 2) { save(); sinceSave = 0 } } return }
    time.value += 0.2 * settings.speed; sinceSave += 0.2
    if (time.value >= duration.value) { onEnded(); return }
    if (sinceSave >= 2) { save(); sinceSave = 0 }
  }
  function save() {
    if (!canSave() || restoringPosition || !Number.isFinite(duration.value) || duration.value <= 0 || !snap.d || snap.d === 'undefined') return
    const completed = state.value === 'ended'
    const media = video.value
    const position = completed ? duration.value : media && !media.seeking ? media.currentTime : time.value
    if (!Number.isFinite(position)) return
    const positionMs = Math.round(position * 1000)
    if (!manualSeek && positionMs === savedPositionMs && completed === savedCompletion) return
    savedPositionMs = positionMs
    savedCompletion = completed
    manualSeek = false
    const percent = completed ? 100 : Math.min(99, Math.round(position * 100 / duration.value))
    lib.record(snap.d, snap.e, percent, position, duration.value)
  }
  timer = window.setInterval(tick, 200)
  watch(() => settings.quality, () => {
    if (!isDemo || state.value === 'idle') return
    if (state.value === 'playing' || state.value === 'paused') {
      wantPlay = state.value === 'playing'
      save()
    }
    if (state.value === 'ended') wantPlay = false
    retries.value = 0
    if (state.value === 'resolving' || !duration.value) load(pendingSeek.sec, pendingSeek.frac)
    else load(video.value?.currentTime ?? time.value)
  })
  watch(() => settings.speed, (v) => { if (video.value) video.value.playbackRate = v })
  watch(() => settings.volume, (v) => { if (video.value) video.value.volume = v / 100 })
  watch(muted, (m) => { if (video.value) video.value.muted = m })
  watch([dramaId, episode, () => opts.startOverride?.()], ([d, e]) => {
    if (!d || d === 'undefined' || !Number.isFinite(e)) return
    const point = opts.startPoint?.(d, e)
    save()
    snap.d = d
    snap.e = e
    wantPlay = shouldAutoplay()
    load(point?.sec, point?.frac)
  })
  function saveBeforeClose() {
    if (state.value !== 'playing' && state.value !== 'paused') return
    pause()
    save()
  }
  window.addEventListener(BEFORE_WINDOW_CLOSE, saveBeforeClose)
  onBeforeUnmount(() => { window.removeEventListener(BEFORE_WINDOW_CLOSE, saveBeforeClose); clearInterval(timer); clearTimeout(resolveTimer); clearTimeout(retryTimer); loadToken++; if (state.value === 'playing' || state.value === 'paused') save() })
  return { state, time, duration, error, muted, buffering, retries, video, sources, mode: opts.mode, load, loadCurrent, retry, play, pause, toggle, seek, seekBy, onMeta, onTime, onSeeked, onEnded, onError, onWaiting, onMediaControl, percent: computed(() => (duration.value ? (time.value / duration.value) * 100 : 0)) }
}
