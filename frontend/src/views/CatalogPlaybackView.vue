<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft } from 'lucide-vue-next'
import type Hls from 'hls.js'
import { isAppleCMSDramaId } from '@/catalogIdentity'
import { releaseCatalogPlayback, resolveCatalogPlayback } from '@/bindings'
import { BEFORE_WINDOW_CLOSE, BEFORE_WINDOW_HIDE, WINDOW_CLOSE_CANCELED } from '@/bindings/windowClose'
import { ErrorState } from '@/components/ui'
import CommentPanel from '@/components/player/CommentPanel.vue'
import DanmakuLayer from '@/components/player/DanmakuLayer.vue'
import DanmakuSettings from '@/components/player/DanmakuSettings.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import { useLocalEpisodeDanmaku } from '@/composables/useLocalEpisodeDanmaku'
import { getDetail, type DramaDetail } from '@/mocks/provider'
import { demoQualityId } from '@/player/demoMedia'
import { resumePosition, seekablePosition } from '@/player/playbackBookmark'
import { isPlaybackSpeed, normalizePlaybackVolume, playbackEndAction } from '@/player/preferences'
import { availableCatalogEpisode, catalogPlaybackRoute, detailRoute } from '@/router/libraryRoutes'
import { useAppSettings } from '@/stores/appSettings'
import { useCommentStore } from '@/stores/comments'
import { useSettingsStore } from '@/stores/player'
import { useLibraryStore } from '@/stores/library'
import type { HistoryEntry } from '@/types'
import type { MediaSource, PlaybackTicket } from '../../../contracts/provider-contracts'

const hlsType = 'application/vnd.apple.mpegurl'
const cmsRateLimitError = '苹果 CMS 请求过于频繁，请稍后重试'
const playbackStallTimeout = 20_000

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
const comments = useCommentStore()
const state = ref<'loading' | 'ready' | 'error'>('loading')
const error = ref('')
const playbackNotice = ref('')
const detail = ref<DramaDetail>()
const canInteract = computed(() => !!detail.value?.appleCMS && detail.value.catalogPlayback === true &&
  detail.value.drama.id === id.value && !!availableCatalogEpisode(detail.value, episode.value))
type LocalPanel = 'comments' | 'danmaku'
const localPanel = ref<LocalPanel>()
const commentTrigger = ref<HTMLButtonElement>()
const danmakuTrigger = ref<HTMLButtonElement>()
const panelRegion = ref<HTMLElement>()
const video = ref<HTMLVideoElement>()
const { mine, danmakuDraft, sendDm, recoveringDanmaku, pendingDanmakuRecovery, retryDanmakuRecovery } =
  useLocalEpisodeDanmaku(id, episode, () => canInteract.value)
const mediaTime = ref(0)
const mediaDuration = ref(0)
let danmakuFrame = 0
const currentPosition = () => video.value?.currentTime ?? 0

function syncDanmakuClock() {
  cancelAnimationFrame(danmakuFrame)
  const media = video.value
  if (!canInteract.value || !media) {
    mediaTime.value = 0
    mediaDuration.value = 0
    return
  }
  mediaTime.value = media.currentTime
  mediaDuration.value = Number.isFinite(media.duration) ? media.duration : 0
  if (!media.paused && !media.ended && mine.value.length && settings.danmaku.enabled) {
    danmakuFrame = requestAnimationFrame(syncDanmakuClock)
  }
}
watch([video, () => mine.value.length, () => settings.danmaku.enabled], syncDanmakuClock)
const sources = ref<MediaSource[]>([])
const selected = ref(0)
const source = computed(() => sources.value[selected.value])
let generation = 0
let playbackPath = ''
let expiresAt = 0
let expiryTimer: number | undefined
let stallTimer: number | undefined
let activeLease: string | undefined
let hls: Hls | undefined

type WatchTarget = { id: string; episode: number; episodeCount: number; protectCompletion: boolean }
type Progress = { position: number; duration: number; percent: number }
type MediaHistory = {
  generation: number
  path: string
  target: WatchTarget
  media: HTMLVideoElement
  source: MediaSource
  checkpoint?: HistoryEntry
  pending: boolean
  restoring: boolean
  endHandled: boolean
  autoAttempted: boolean
  terminalSeek?: boolean
  startedAt?: number
  lastPosition: number
  observedPosition?: number
  progress?: Progress
  saved?: Progress
}
type Advance = { fromGeneration: number; path: string; muted: boolean }
type AutoStart = { reason: 'next' | 'fallback'; muted: boolean }
let watchTarget: WatchTarget | undefined
let mediaHistory: MediaHistory | undefined
let preferenceMedia: HTMLVideoElement | undefined
let nextEpisode: Advance | undefined
const autoStart = shallowRef<AutoStart>()
let manualNavigation: ReturnType<typeof router.push> | undefined
let closing = false

function isPlaybackRoute(path: string, current: number) {
  return route.name === 'catalog-play' && route.path === path && generation === current
}

function isCurrentSource(media: HTMLVideoElement, choice: MediaSource, path: string, current: number) {
  return isPlaybackRoute(path, current) && state.value === 'ready' &&
    video.value === media && source.value === choice
}

function isCurrentHistory(current: MediaHistory) {
  return current === mediaHistory && Date.now() < expiresAt &&
    isCurrentSource(current.media, current.source, current.path, current.generation)
}

function currentHistory(event: Event) {
  const current = mediaHistory
  if (current && event.currentTarget === current.media && isCurrentHistory(current)) return current
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
  if (!current) return
  restoreHistory(current)
  syncDanmakuClock()
  startWhenReady(current)
}

function startWhenReady(current: MediaHistory) {
  const { media } = current
  const start = autoStart.value
  if (!start || current.autoAttempted || current.pending || current.restoring || media.seeking ||
      media.readyState < HTMLMediaElement.HAVE_METADATA || closing || !isCurrentHistory(current)) return
  current.autoAttempted = true
  media.muted = start.muted
  startMedia(current, start.reason === 'next' ? '已切换到下一集，请点击播放。' : '已切换到备用源，请点击播放。')
}

function startMedia(current: MediaHistory, fallback: string) {
  void current.media.play().catch(() => {
    if (closing || !isCurrentHistory(current) || !current.media.paused || current.media.error) return
    autoStart.value = undefined
    playbackNotice.value = fallback
  })
}

function onPlaying(event: Event) {
  const current = currentHistory(event)
  if (!current || current.media.paused) return
  if (closing) {
    current.media.pause()
    return
  }
  autoStart.value = undefined
  playbackNotice.value = ''
  watchPlaybackProgress(current)
  current.endHandled = false
  if (current.startedAt === undefined) {
    current.startedAt = current.media.currentTime
    current.progress = undefined
  }
}

function onSeeking(event: Event) {
  const current = currentHistory(event)
  if (!current) return
  const { currentTime, duration } = current.media
  current.terminalSeek = Number.isFinite(duration) && duration > 0 && currentTime >= duration
  watchPlaybackProgress(current)
}

function onSeeked(event: Event) {
  const current = currentHistory(event)
  if (!current || current.media.seeking || closing) return
  syncDanmakuClock()
  if (current.restoring) {
    current.restoring = false
    current.lastPosition = current.media.currentTime
    startWhenReady(current)
    return
  }
  saveProgress(true)
}

function captureProgress(current: MediaHistory, failed: boolean) {
  const media = current.media
  const { currentTime, duration } = media
  if (current.startedAt === undefined || current.pending || current.restoring || media.seeking || current.terminalSeek ||
      (!failed && media.error) || !media.played.length || !Number.isFinite(duration) || duration <= 0 ||
      !Number.isFinite(currentTime)) return
  // A failed decoder can reset its position. Keep the last valid sample in that case.
  if (failed && (currentTime <= 0 || currentTime > duration)) return
  const position = Math.max(0, Math.min(currentTime, duration))
  const ended = !failed && media.ended
  // A seek straight to the end can emit "ended" without playing the tail.
  if (ended && media.played.end(media.played.length - 1) < position - 0.05) return
  if (!current.progress && (position === current.lastPosition || position === current.startedAt) && !ended) return
  const percent = ended ? 100 : Math.min(99, Math.floor(position * 100 / duration))
  current.progress = { position, duration, percent }
  if (ended) current.startedAt = undefined
}

function saveProgress(force: boolean, failed = false) {
  const current = mediaHistory
  if (!current || (current.target.protectCompletion && (failed || current.media.error))) return
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
  current.target.protectCompletion = false
}

function onProgress(event: Event) {
  const current = currentHistory(event)
  if (!closing && current) {
    syncDanmakuClock()
    watchPlaybackProgress(current)
    saveProgress(event.type !== 'timeupdate')
  }
}

function onPlay(event: Event) {
  const current = currentHistory(event)
  if (current) {
    syncDanmakuClock()
    watchPlaybackProgress(current)
  }
}

function watchPlaybackProgress(current: MediaHistory) {
  const { media } = current
  if (closing || media.paused || media.ended) {
    clearTimeout(stallTimer)
    stallTimer = undefined
    return
  }
  const position = media.currentTime
  if (stallTimer !== undefined && current.observedPosition === position) return
  clearTimeout(stallTimer)
  current.observedPosition = position
  // Some native decoders keep requesting a failed range without reporting a media error.
  stallTimer = window.setTimeout(() => {
    stallTimer = undefined
    if (!isCurrentHistory(current) || closing || media.paused || media.ended) return
    if (media.currentTime !== position) watchPlaybackProgress(current)
    else failMedia(media, current.source)
  }, playbackStallTimeout)
}

function onEnded(event: Event) {
  const current = currentHistory(event)
  if (!current || !current.media.ended || current.endHandled || current.terminalSeek) return
  syncDanmakuClock()
  saveProgress(true)
  if (current.progress?.percent !== 100) return
  current.endHandled = true
  if (closing || manualNavigation) return

  const next = current.target.episode + 1
  const action = playbackEndAction(preferences.s.playMode, !!availableCatalogEpisode(detail.value, next))
  if (action === 'loop') {
    current.media.currentTime = 0
    startMedia(current, '请点击播放重新开始本集。')
  } else if (action === 'next') {
    const destination = catalogPlaybackRoute(current.target.id, next)
    const advance = { fromGeneration: current.generation, path: router.resolve(destination).path, muted: current.media.muted }
    nextEpisode = advance
    const clearPending = () => {
      if (nextEpisode === advance) nextEpisode = undefined
    }
    void router.replace(destination).then(clearPending, clearPending)
  }
}

function saveOnExit() {
  syncMediaPreferences()
  saveProgress(true)
}

function onWindowClose() {
  if (closing) return
  closing = true
  nextEpisode = undefined
  autoStart.value = undefined
  if (watchTarget) video.value?.pause()
  saveOnExit()
}

function onWindowCloseCanceled() { closing = false }

function selectEpisode(index: number) {
  nextEpisode = undefined
  autoStart.value = undefined
  const navigation = router.push(catalogPlaybackRoute(id.value, index))
  manualNavigation = navigation
  const settled = () => {
    if (manualNavigation === navigation) manualNavigation = undefined
  }
  void navigation.then(settled, settled)
}

function stopMedia(failed = false) {
  cancelAnimationFrame(danmakuFrame)
  clearTimeout(stallTimer)
  stallTimer = undefined
  syncMediaPreferences()
  preferenceMedia = undefined
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
  autoStart.value = undefined
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
  const path = route.path
  if (route.name !== 'catalog-play') return
  const automatic = !closing && nextEpisode?.fromGeneration === generation && nextEpisode.path === path
    ? nextEpisode : undefined
  nextEpisode = undefined
  autoStart.value = automatic && { reason: 'next', muted: automatic.muted }
  const current = ++generation
  playbackPath = path
  const targetId = id.value
  const targetEpisode = episode.value
  clearTimeout(expiryTimer)
  stopMedia()
  releaseLease()
  watchTarget = undefined
  sources.value = []
  selected.value = 0
  error.value = ''
  playbackNotice.value = ''
  state.value = 'loading'
  detail.value = undefined
  const fail = (message: string) => {
    autoStart.value = undefined
    error.value = automatic ? '无法播放下一集。' + message : message
    state.value = 'error'
  }
  try {
    const result = await getDetail(targetId, 'normal', true)
    if (!isPlaybackRoute(path, current)) return
    detail.value = result ?? undefined
    if (!result?.catalogPlayback || (automatic && (!result.appleCMS || closing))) {
      fail('当前剧目播放暂不可用。')
      return
    }
    const requested = availableCatalogEpisode(result, targetEpisode)
    if (!requested?.id) {
      fail('所选分集暂不可播放。')
      return
    }
    const ticket = await resolveCatalogPlayback(targetId, targetEpisode)
    if (!isPlaybackRoute(path, current)) {
      if (ticket?.leaseId) releaseLease(ticket.leaseId)
      return
    }
    if (!ticket) throw new Error()
    const choices = validateSources(ticket, requested.id)
    if (!choices.length || (automatic && closing)) {
      if (ticket.leaseId) releaseLease(ticket.leaseId)
      throw new Error()
    }
    activeLease = ticket.leaseId
    if (result.appleCMS) {
      watchTarget = { id: targetId, episode: targetEpisode, episodeCount: result.drama.episodeCount, protectCompletion: !!automatic }
    }
    expiresAt = Date.parse(ticket.expiresAt ?? '')
    sources.value = choices
    state.value = 'ready'
    watchExpiry(current)
  } catch (cause) {
    if (!isPlaybackRoute(path, current)) return
    const message = cause instanceof Error ? cause.message : cause
    fail(isAppleCMSDramaId(targetId) && message === cmsRateLimitError
      ? cmsRateLimitError + '。' : '无法取得可用的播放源，请重试。')
  }
}

function failMedia(media: HTMLVideoElement, choice: MediaSource) {
  if (!isCurrentSource(media, choice, playbackPath, generation)) return
  if (expiresAt <= Date.now()) {
    expireTicket()
    return
  }
  const resumeBackup = watchTarget && !closing && !manualNavigation && !media.paused && !media.ended
  const muted = media.muted
  stopMedia(true)
  if (selected.value + 1 < sources.value.length) {
    if (resumeBackup && !autoStart.value) autoStart.value = { reason: 'fallback', muted }
    selected.value++
    return
  }
  state.value = 'error'
  error.value = autoStart.value?.reason === 'next' ? '下一集媒体加载失败，请重试。' : '媒体加载失败，请重试。'
  autoStart.value = undefined
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
  media.volume = settings.volume / 100
  const current = generation
  const path = route.path
  if (watchTarget) {
    const target = watchTarget
    preferenceMedia = media
    media.defaultPlaybackRate = settings.speed
    media.playbackRate = settings.speed
    const checkpoint = library.history.find((item) => item.dramaId === target.id && item.episode === target.episode)
    mediaHistory = {
      generation: current, path, target, media, source: choice, checkpoint,
      pending: true, restoring: false, endHandled: false, autoAttempted: false, lastPosition: 0,
    }
    restoreHistory(mediaHistory)
    startWhenReady(mediaHistory)
  }
  if (choice.type !== hlsType) return
  const isCurrent = () => isCurrentSource(media, choice, path, current)
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
    class CatalogHlsPlayer extends HlsPlayer {
      override recoverMediaError() {
        // Recovery reloads the element before queued preference events can run.
        if (hls === this && isCurrent()) syncMediaPreferences()
        super.recoverMediaError()
      }
    }
    const player = new CatalogHlsPlayer({ lowLatencyMode: false, emeEnabled: false })
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

function onLoadedMetadata(event: Event) {
  if (event.currentTarget !== video.value || !video.value) return
  if (video.value !== preferenceMedia) video.value.playbackRate = settings.speed
  onPlayable(event)
}

function syncMediaPreferences() {
  if (!preferenceMedia || Date.now() >= expiresAt) return
  const volume = normalizePlaybackVolume(preferenceMedia.volume * 100)
  if (volume !== settings.volume) settings.volume = volume
  const rate = preferenceMedia.playbackRate
  if (!isPlaybackSpeed(rate)) return
  settings.speed = rate
  if (preferenceMedia.defaultPlaybackRate !== rate) preferenceMedia.defaultPlaybackRate = rate
}

function onMediaPreferenceChange(event: Event) {
  if (event.currentTarget === preferenceMedia) syncMediaPreferences()
}

async function toggleLocalPanel(panel: LocalPanel) {
  if (localPanel.value === panel) {
    localPanel.value = undefined
    const trigger = panel === 'comments' ? commentTrigger : danmakuTrigger
    trigger.value?.focus()
    return
  }
  localPanel.value = panel
  if (panel === 'comments') void comments.open(id.value, episode.value)
  await nextTick()
  if (localPanel.value !== panel) return
  if (panel === 'comments') panelRegion.value?.querySelector<HTMLElement>('[data-testid="comment-panel"]')?.focus()
  else panelRegion.value?.focus()
}

watch(() => route.path, () => {
  localPanel.value = undefined
  void load()
}, { immediate: true })
window.addEventListener('pagehide', onWindowClose)
window.addEventListener(BEFORE_WINDOW_HIDE, saveOnExit)
window.addEventListener(BEFORE_WINDOW_CLOSE, onWindowClose)
window.addEventListener(WINDOW_CLOSE_CANCELED, onWindowCloseCanceled)
onBeforeUnmount(() => {
  generation++
  window.removeEventListener('pagehide', onWindowClose)
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
      <video v-else-if="source" :key="source.src" ref="video" controls playsinline :preload="autoStart ? 'auto' : 'metadata'"
        controlslist="nodownload" :src="source.type === hlsType ? undefined : source.src"
        :aria-label="`第${episode}集视频`" @error="mediaError" @loadedmetadata="onLoadedMetadata"
        @durationchange="onPlayable" @progress="onPlayable" @canplay="onPlayable" @seeking="onSeeking" @seeked="onSeeked"
        @play="onPlay" @playing="onPlaying" @volumechange="onMediaPreferenceChange" @ratechange="onMediaPreferenceChange" @timeupdate="onProgress" @pause="onProgress" @ended="onEnded" />
      <DanmakuLayer v-if="canInteract && video && mine.length && state === 'ready'" :items="mine" :time="mediaTime" :duration="mediaDuration" />
    </div>
    <p v-if="playbackNotice" role="status">{{ playbackNotice }}</p>
    <nav v-if="detail?.catalogPlayback && detail.episodes.length" class="episode-nav" aria-label="选集">
      <button v-for="item in detail.episodes" :key="item.index" type="button" :disabled="!item.available"
        :aria-current="item.index === episode ? 'page' : undefined"
        @click="selectEpisode(item.index)">第{{ item.index }}集</button>
    </nav>
    <div v-if="canInteract" class="local-interactions">
      <div class="local-actions">
        <button ref="commentTrigger" type="button" :aria-expanded="localPanel === 'comments'" @click="toggleLocalPanel('comments')">本机评论</button>
        <button ref="danmakuTrigger" type="button" :aria-expanded="localPanel === 'danmaku'" @click="toggleLocalPanel('danmaku')">本机弹幕</button>
      </div>
      <div v-if="localPanel === 'comments'" ref="panelRegion" class="local-comment-panel">
        <CommentPanel local-only @close="toggleLocalPanel('comments')" />
      </div>
      <div v-else-if="localPanel === 'danmaku'" ref="panelRegion" class="local-danmaku-panel" tabindex="-1" @keydown.esc="toggleLocalPanel('danmaku')">
        <DanmakuSettings local-only :time="mediaTime" :current-time="currentPosition" :draft="danmakuDraft"
          :send-danmaku="sendDm" :local-items="mine" :pending-recovery="pendingDanmakuRecovery"
          :recovering="recoveringDanmaku" :retry-recovery="retryDanmakuRecovery" />
      </div>
    </div>
  </article>
</template>

<style scoped lang="scss">
.catalog-playback { display: grid; gap: 16px; padding: 18px 12px 32px; }
.back { display: inline-flex; align-items: center; gap: 4px; justify-self: start; color: var(--c-text-2); }
h1 { font-size: 28px; overflow-wrap: anywhere; }
.episode-label { color: var(--c-text-2); }
.stage { --danmaku-top: 8px; --danmaku-bottom: 60px; position: relative; min-height: 260px; display: grid; place-items: center; border-radius: 18px; overflow: hidden; background: var(--c-surface-solid); }
video { display: block; width: 100%; max-width: 960px; max-height: 65vh; background: #000; }
.loading { width: 100%; min-height: 360px; }
.episode-nav, .local-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.episode-nav button, .local-actions button {
  border-radius: var(--r-pill); min-height: 38px; padding: 0 16px; background: var(--c-surface-hover);
  &[aria-current='page'] { background: var(--g-accent); color: #fff; }
  &:disabled { opacity: .45; cursor: not-allowed; }
}
.local-comment-panel { width: min(100%, 640px); height: min(560px, 75vh); min-height: 290px; margin-top: 12px; }
.local-danmaku-panel { width: min(100%, 640px); height: min(660px, 75vh); min-height: 290px; margin-top: 12px; }
</style>
