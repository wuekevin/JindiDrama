import { onBeforeUnmount, ref, computed, watch, type Ref } from 'vue'
import { transition, episodeDuration, type PEvent, type PState } from './machine'
import { useSettingsStore } from '@/stores/player'
import { useLibraryStore } from '@/stores/library'
import { logEvent } from '@/bindings'

export type MediaMode = 'sim' | 'demo'
/** 合法演示媒体：本项目自制（见 public/demo/README.md），按集号轮换 3 个片段。 */
export const demoSources = (ep: number) => { const n = (ep % 3) + 1; return [{ src: `./demo/demo-${n}.webm`, type: 'video/webm' }, { src: `./demo/demo-${n}.mp4`, type: 'video/mp4' }] }
export const MAX_AUTO_RETRY = 2
export const backoffMs = (attempt: number) => 500 * 2 ** attempt // 0.5s, 1s

export function resolveMediaMode(setting: string): MediaMode {
  const o = (window as any).__JD_MEDIA; if (o === 'sim' || o === 'demo') return o
  return setting === 'sim' ? 'sim' : 'demo'
}

export function usePlayer(dramaId: Ref<string>, episode: Ref<number>, opts: { startAt?: number; autoplay: boolean; mode: MediaMode; failEpisode?: (ep: number) => string | null }) {
  const settings = useSettingsStore(); const lib = useLibraryStore()
  const state = ref<PState>('idle'); const time = ref(0); const duration = ref(opts.mode === 'sim' ? episodeDuration(episode.value) : 0); const error = ref('')
  const muted = ref(false); const buffering = ref(false); const retries = ref(0)
  const video = ref<HTMLVideoElement | null>(null)
  let loadToken = 0; let timer: number | undefined; let resolveTimer: number | undefined; let retryTimer: number | undefined; let sinceSave = 0
  let pendingSeek: { sec?: number; frac?: number } = {}; let wantPlay = opts.autoplay
  const send = (e: PEvent) => { state.value = transition(state.value, e) }
  const isDemo = opts.mode === 'demo'
  const snap = { d: dramaId.value, e: episode.value } // 离开页面时路由参数已变化，保存必须用快照

  function load(startAt = 0, frac?: number) {
    const my = ++loadToken; clearTimeout(resolveTimer); clearTimeout(retryTimer); error.value = ''; buffering.value = false
    pendingSeek = { sec: startAt || undefined, frac }
    if (!isDemo) { duration.value = episodeDuration(episode.value); time.value = Math.min(startAt, duration.value - 1) } else { time.value = startAt || 0 }
    send('load')
    const fail = opts.failEpisode?.(episode.value)
    resolveTimer = window.setTimeout(() => {
      if (my !== loadToken) return // 过期请求被取消
      if (fail) { error.value = fail; send('fail'); logEvent('warn', 'player', `drama=${dramaId.value} ep=${episode.value} ${fail}`); return }
      if (isDemo) { const v = video.value; if (v && state.value === 'resolving') v.load(); return } // 元数据若已先到（状态已前进）则不重复加载 // 之后由 <video> 事件推进状态
      send('resolved'); if (wantPlay) send('play'); else send('pause')
    }, 260)
  }
  // ---- <video> 事件（demo 模式）----
  function onMeta() {
    const v = video.value; if (!v || state.value !== 'resolving') return
    duration.value = v.duration || 0
    const t = pendingSeek.sec ?? (pendingSeek.frac !== undefined ? (pendingSeek.frac * duration.value) : 0)
    if (t > 0 && t < duration.value) v.currentTime = t
    time.value = v.currentTime
    v.playbackRate = settings.speed; v.volume = settings.volume / 100; v.muted = muted.value
    retries.value = 0; send('resolved')
    if (wantPlay) { send('play'); v.play().catch(() => { send('pause') }) } else send('pause')
  }
  function onTime() { const v = video.value; if (!v) return; time.value = v.currentTime; if (v.duration && duration.value !== v.duration) duration.value = v.duration }
  function onEnded() { if (state.value === 'playing' || state.value === 'paused') { send('finish'); save() } }
  function onError() {
    const my = loadToken; if (state.value === 'idle') return
    if (retries.value < MAX_AUTO_RETRY) { // 指数退避自动重试
      const wait = backoffMs(retries.value); retries.value++
      retryTimer = window.setTimeout(() => { if (my === loadToken && video.value) { pendingSeek = { sec: time.value || undefined }; send('retry'); video.value.load() } }, wait)
      if (state.value !== 'resolving') send('load')
      return
    }
    error.value = '媒体加载失败（已自动重试）'; send('fail'); logEvent('error', 'player', `drama=${dramaId.value} ep=${episode.value} media load failed after ${retries.value} retries src=${video.value?.currentSrc ?? ''}`)
  }
  const onWaiting = () => { buffering.value = true }; const onPlaying = () => { buffering.value = false }

  const retry = () => { retries.value = 0; send('retry'); wantPlay = true; load(time.value) }
  function play() { wantPlay = true; if (state.value === 'ended') { time.value = 0; if (video.value) video.value.currentTime = 0 } send('play'); video.value?.play().catch(() => { send('pause') }) }
  function pause() { wantPlay = false; send('pause'); video.value?.pause() }
  const toggle = () => (state.value === 'playing' ? pause() : play())
  function seek(s: number) { const d = duration.value || 0; time.value = Math.max(0, Math.min(d - 0.01, s)); if (video.value && isDemo) video.value.currentTime = time.value; if (state.value === 'ended') { send('play'); video.value?.play().catch(() => { /* ignore */ }) } }
  const seekBy = (d: number) => seek(time.value + d)
  function tick() {
    if (isDemo || state.value !== 'playing') { if (isDemo && state.value === 'playing') { sinceSave += 0.2; if (sinceSave >= 2) { save(); sinceSave = 0 } } return }
    time.value += 0.2 * settings.speed; sinceSave += 0.2
    if (time.value >= duration.value) { time.value = duration.value; send('finish'); save(); return }
    if (sinceSave >= 2) { save(); sinceSave = 0 }
  }
  function save() { if (!duration.value || !snap.d || snap.d === 'undefined') return; lib.record(snap.d, snap.e, Math.round((time.value / duration.value) * 100), time.value, duration.value) }
  timer = window.setInterval(tick, 200)
  watch(() => settings.speed, (v) => { if (video.value) video.value.playbackRate = v })
  watch(() => settings.volume, (v) => { if (video.value) video.value.volume = v / 100 })
  watch(muted, (m) => { if (video.value) video.value.muted = m })
  watch(episode, (e) => { save(); if (Number.isFinite(e) && dramaId.value !== 'undefined') { snap.e = e; wantPlay = true; load(0) } })
  watch(dramaId, (d) => { if (d && d !== 'undefined') snap.d = d }, { flush: 'sync' })
  onBeforeUnmount(() => { clearInterval(timer); clearTimeout(resolveTimer); clearTimeout(retryTimer); loadToken++; if (state.value === 'playing' || state.value === 'paused') save() })
  return { state, time, duration, error, muted, buffering, retries, video, mode: opts.mode, load, retry, play, pause, toggle, seek, seekBy, onMeta, onTime, onEnded, onError, onWaiting, onPlaying, percent: computed(() => (duration.value ? (time.value / duration.value) * 100 : 0)) }
}
