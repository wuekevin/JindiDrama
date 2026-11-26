<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft } from 'lucide-vue-next'
import type Hls from 'hls.js'
import { releaseCatalogPlayback, resolveCatalogPlayback } from '@/bindings'
import { BEFORE_WINDOW_CLOSE, BEFORE_WINDOW_HIDE, WINDOW_CLOSE_CANCELED } from '@/bindings/windowClose'
import { ErrorState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'
import { getDetail, type DramaDetail } from '@/mocks/provider'
import { demoQualityId } from '@/player/demoMedia'
import { resumePosition, seekablePosition } from '@/player/playbackBookmark'
import { availableCatalogEpisode, catalogPlaybackRoute, detailRoute } from '@/router/libraryRoutes'
import { useAppSettings } from '@/stores/appSettings'
import { useSettingsStore } from '@/stores/player'
import { useLibraryStore } from '@/stores/library'
import type { HistoryEntry } from '@/types'
import type { MediaSource, PlaybackTicket } from '../../../contracts/provider-contracts'

const hlsType = 'application/vnd.apple.mpegurl'

const route = useRoute()
const router = useRouter()
const id = computed(() => String(route.params.id))
const episode = computed(() => {
  const value = String(route.params.episode)
  const index = /^\d+$/.test(value) ? Number(value) : 0
  return Number.isSafeInteger(index) ? index : 0
})
const preferences = useAppSettings()
const settings = useSettingsStore()
const library = useLibraryStore()
const state = ref<'loading' | 'ready' | 'error'>('loading')
const error = ref('')
const detail = ref<DramaDetail>()
const video = ref<HTMLVideoElement>()
const sources = ref<MediaSource[]>([])
const selected = ref(0)
const source = computed(() => sources.value[selected.value])
let generation = 0
let expiresAt = 0
let expiryTimer: number | undefined
let activeLease: string | undefined
let hls: Hls | undefined

type WatchTarget = { id: string; episode: number; episodeCount: number }
type Progress = { position: number; duration: number; percent: number }
type MediaHistory = {
  target: WatchTarget
  media: HTMLVideoElement
  source: MediaSource
  checkpoint?: HistoryEntry
  pending: boolean
  restoring: boolean
  startedAt?: number
  lastPosition: number
  progress?: Progress
  saved?: Progress
}
let watchTarget: WatchTarget | undefined
let mediaHistory: MediaHistory | undefined
let closing = false

function currentHistory(event: Event) {
  const current = mediaHistory
  if (current && state.value === 'ready' && event.currentTarget === current.media &&
      video.value === current.media && source.value === current.source && Date.now() < expiresAt) return current
}

function restoreHistory(current: MediaHistory) {
  const { media, checkpoint, target } = current
  if (!current.pending || !Number.isFinite(media.duration) || media.duration <= 0) return
  const position = resumePosition(checkpoint, target.episode, media.duration)
  const ranges = media.seekable
  if (position > 0 && !ranges.length && media.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) return
  current.pending = false
  if (position <= 0 || !ranges.length) return
  const seek = seekablePosition(ranges, position, media.currentTime)
  current.lastPosition = seek
  if (seek !== media.currentTime) {
    current.restoring = true
    media.currentTime = seek
  }
}

function onPlayable(event: Event) {
  const current = currentHistory(event)
  if (current) restoreHistory(current)
}

function onPlaying(event: Event) {
  const current = currentHistory(event)
  if (!current || current.media.paused) return
  if (closing) {
    current.media.pause()
  } else if (current.startedAt === undefined) {
    current.startedAt = current.media.currentTime
    current.progress = undefined
  }
}

function onSeeked(event: Event) {
  const current = currentHistory(event)
  if (!current || current.media.seeking || closing) return
  if (current.restoring) {
    current.restoring = false
    current.lastPosition = current.media.currentTime
    return
  }
  saveProgress(true)
}

function captureProgress(current: MediaHistory, failed: boolean) {
  const media = current.media
  const { currentTime, duration } = media
  if (current.startedAt === undefined || current.pending || current.restoring || media.seeking ||
      (!failed && media.error) || !media.played.length || !Number.isFinite(duration) || duration <= 0 ||
      !Number.isFinite(currentTime)) return
  // A failed decoder can reset its position. Keep the last valid sample in that case.
  if (failed && (currentTime <= 0 || currentTime > duration)) return
  const position = Math.max(0, Math.min(currentTime, duration))
  const ended = !failed && media.ended
  if (!current.progress && (position === current.lastPosition || position === current.startedAt) && !ended) return
  const percent = ended ? 100 : Math.min(99, Math.floor(position * 100 / duration))
  current.progress = { position, duration, percent }
  if (ended) current.startedAt = undefined
}

function saveProgress(force: boolean, failed = false) {
  const current = mediaHistory
  if (!current) return
  // A ticket can expire between media events. Keep only the last position sampled while it was valid.
  if (Date.now() < expiresAt) captureProgress(current, failed)
  const progress = current.progress
  if (!progress || (!force && Math.abs(progress.position - current.lastPosition) < 5 && progress.percent !== 100)) return
  const saved = current.saved
  if (saved?.position === progress.position && saved.duration === progress.duration && saved.percent === progress.percent) return
  current.saved = progress
  current.lastPosition = progress.position
  const { id, episode, episodeCount } = current.target
  library.record(id, episode, progress.percent, progress.position, progress.duration, episodeCount)
}

function onProgress(event: Event) {
  if (!closing && currentHistory(event)) saveProgress(event.type !== 'timeupdate')
}

function saveOnExit() { saveProgress(true) }

function onWindowClose() {
  if (closing || !watchTarget) return
  closing = true
  video.value?.pause()
  saveOnExit()
}

function onWindowCloseCanceled() { closing = false }

function stopMedia(failed = false) {
  saveProgress(true, failed)
  mediaHistory = undefined
  const player = hls
  hls = undefined
  player?.destroy()
  const media = video.value
  if (!media) return
  media.pause()
  media.removeAttribute('src')
  media.load()
}

function validateSources(ticket: PlaybackTicket, expectedEpisodeId: string) {
  if (ticket.kind !== 'authorized' || ticket.episodeId !== expectedEpisodeId || ticket.restrictions?.length) return []
  if (!(Date.parse(ticket.expiresAt ?? '') > Date.now())) return []
  const token = ticket.leaseId
  if (!token || !/^[A-Z2-7]{26,}$/.test(token)) return []
  const quality = demoQualityId(preferences.s.defaultQuality)
  const distinct = new Map<string, MediaSource>()
  for (const choice of ticket.sources ?? []) {
    if (choice.type !== 'video/mp4' && choice.type !== 'video/webm' && choice.type !== hlsType) continue
    try {
      const url = new URL(choice.src)
      if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port ||
          url.username || url.password || url.hash || url.search) continue
      const localPath = new RegExp(`^/catalog-media/${token}/[0-9]+${choice.type === hlsType ? '[.]m3u8' : ''}$`)
      if (!localPath.test(url.pathname)) continue
      if (!distinct.has(url.href)) distinct.set(url.href, { ...choice, src: url.href })
    } catch { /* Invalid URLs never reach the media element. */ }
  }
  return [...distinct.values()].sort((left, right) => Number(right.quality === quality) - Number(left.quality === quality))
}

function releaseLease(token = activeLease) {
  if (!token) return
  if (token === activeLease) activeLease = undefined
  void releaseCatalogPlayback(token)?.catch(() => {})
}

function expireTicket() {
  stopMedia()
  releaseLease()
  sources.value = []
  state.value = 'error'
  error.value = '播放票据已过期，请重试。'
}

function watchExpiry(current: number) {
  const remaining = expiresAt - Date.now()
  if (remaining <= 0) {
    expireTicket()
    return
  }
  expiryTimer = window.setTimeout(() => {
    if (current === generation && state.value === 'ready') watchExpiry(current)
  }, Math.min(remaining, 2_147_483_647))
}

async function load() {
  const current = ++generation
  const targetId = id.value
  const targetEpisode = episode.value
  clearTimeout(expiryTimer)
  stopMedia()
  releaseLease()
  watchTarget = undefined
  sources.value = []
  selected.value = 0
  error.value = ''
  state.value = 'loading'
  detail.value = undefined
  try {
    const result = await getDetail(targetId, 'normal', true)
    if (current !== generation) return
    detail.value = result ?? undefined
    if (!result?.catalogPlayback) {
      error.value = '当前剧目播放暂不可用。'
      state.value = 'error'
      return
    }
    const requested = availableCatalogEpisode(result, targetEpisode)
    if (!requested?.id) {
      error.value = '所选分集暂不可播放。'
      state.value = 'error'
      return
    }
    const ticket = await resolveCatalogPlayback(targetId, targetEpisode)
    if (current !== generation) {
      if (ticket?.leaseId) releaseLease(ticket.leaseId)
      return
    }
    if (!ticket) throw new Error()
    const choices = validateSources(ticket, requested.id)
    if (!choices.length) {
      if (ticket.leaseId) releaseLease(ticket.leaseId)
      throw new Error()
    }
    activeLease = ticket.leaseId
    if (result.appleCMS) watchTarget = { id: targetId, episode: targetEpisode, episodeCount: result.drama.episodeCount }
    expiresAt = Date.parse(ticket.expiresAt ?? '')
    sources.value = choices
    state.value = 'ready'
    watchExpiry(current)
  } catch {
    if (current !== generation) return
    error.value = '无法取得可用的播放源，请重试。'
    state.value = 'error'
  }
}

function failMedia(media: HTMLVideoElement, choice: MediaSource) {
  if (media !== video.value || choice !== source.value || state.value !== 'ready') return
  if (expiresAt <= Date.now()) {
    expireTicket()
    return
  }
  stopMedia(true)
  if (selected.value + 1 < sources.value.length) {
    selected.value++
    return
  }
  state.value = 'error'
  error.value = '媒体加载失败，请重试。'
  sources.value = []
  releaseLease()
}

function mediaError(event: Event) {
  const media = video.value
  const choice = source.value
  if (media && choice && event.currentTarget === media) failMedia(media, choice)
}

watch([video, source], async ([media, choice]) => {
  if (!media || !choice || state.value !== 'ready') return
  if (watchTarget) {
    const target = watchTarget
    const checkpoint = library.history.find((item) => item.dramaId === target.id && item.episode === target.episode)
    mediaHistory = {
      target, media, source: choice, checkpoint, pending: true, restoring: false, lastPosition: 0,
    }
    restoreHistory(mediaHistory)
  }
  if (choice.type !== hlsType) return
  const current = generation
  const isCurrent = () => current === generation && video.value === media && source.value === choice && state.value === 'ready'
  if (expiresAt <= Date.now()) {
    expireTicket()
    return
  }
  if (media.canPlayType(hlsType)) {
    media.src = choice.src
    return
  }
  try {
    const { default: HlsPlayer } = await import('hls.js')
    if (!isCurrent()) return
    if (expiresAt <= Date.now() || !HlsPlayer.isSupported()) {
      failMedia(media, choice)
      return
    }
    const player = new HlsPlayer({ lowLatencyMode: false, emeEnabled: false })
    hls = player
    player.on(HlsPlayer.Events.ERROR, (_, data) => {
      if (data.fatal && hls === player && isCurrent()) failMedia(media, choice)
    })
    player.loadSource(choice.src)
    if (hls === player) player.attachMedia(media)
  } catch {
    if (isCurrent()) failMedia(media, choice)
  }
}, { flush: 'post' })

function applyPreferences(event: Event) {
  if (event.currentTarget !== video.value || !video.value) return
  video.value.volume = settings.volume / 100
  video.value.playbackRate = settings.speed
  onPlayable(event)
}

watch([id, episode], load, { immediate: true })
window.addEventListener('pagehide', saveOnExit)
window.addEventListener(BEFORE_WINDOW_HIDE, saveOnExit)
window.addEventListener(BEFORE_WINDOW_CLOSE, onWindowClose)
window.addEventListener(WINDOW_CLOSE_CANCELED, onWindowCloseCanceled)
onBeforeUnmount(() => {
  generation++
  window.removeEventListener('pagehide', saveOnExit)
  window.removeEventListener(BEFORE_WINDOW_HIDE, saveOnExit)
  window.removeEventListener(BEFORE_WINDOW_CLOSE, onWindowClose)
  window.removeEventListener(WINDOW_CLOSE_CANCELED, onWindowCloseCanceled)
  clearTimeout(expiryTimer)
  stopMedia()
  releaseLease()
})
</script>

<template>
  <article class="catalog-playback" data-testid="catalog-playback">
    <RouterLink class="back" :to="detailRoute(id, true)"><ChevronLeft :size="18" />返回详情</RouterLink>
    <h1 v-if="detail">{{ detail.drama.title }}</h1>
    <p v-if="episode" class="episode-label">第{{ episode }}集</p>
    <div class="stage">
      <Skeleton v-if="state === 'loading'" class="loading" />
      <ErrorState v-else-if="state === 'error'" :message="error" @retry="load" />
      <video v-else-if="source" :key="source.src" ref="video" controls playsinline preload="metadata"
        controlslist="nodownload" :src="source.type === hlsType ? undefined : source.src"
        :aria-label="`第${episode}集视频`" @error="mediaError" @loadedmetadata="applyPreferences"
        @durationchange="onPlayable" @progress="onPlayable" @canplay="onPlayable" @seeked="onSeeked"
        @playing="onPlaying" @timeupdate="onProgress" @pause="onProgress" @ended="onProgress" />
    </div>
    <nav v-if="detail?.catalogPlayback && detail.episodes.length" class="episode-nav" aria-label="选集">
      <button v-for="item in detail.episodes" :key="item.index" type="button" :disabled="!item.available"
        :aria-current="item.index === episode ? 'page' : undefined"
        @click="router.push(catalogPlaybackRoute(id, item.index))">第{{ item.index }}集</button>
    </nav>
  </article>
</template>

<style scoped lang="scss">
.catalog-playback { display: grid; gap: 16px; padding: 18px 12px 32px; }
.back { display: inline-flex; align-items: center; gap: 4px; justify-self: start; color: var(--c-text-2); }
h1 { font-size: 28px; overflow-wrap: anywhere; }
.episode-label { color: var(--c-text-2); }
.stage { min-height: 260px; display: grid; place-items: center; border-radius: 18px; overflow: hidden; background: var(--c-surface-solid); }
video { display: block; width: 100%; max-width: 960px; max-height: 65vh; background: #000; }
.loading { width: 100%; min-height: 360px; }
.episode-nav { display: flex; gap: 10px; flex-wrap: wrap; }
.episode-nav button {
  border-radius: var(--r-pill); min-height: 38px; padding: 0 16px; background: var(--c-surface-hover);
  &[aria-current='page'] { background: var(--g-accent); color: #fff; }
  &:disabled { opacity: .45; cursor: not-allowed; }
}
</style>
