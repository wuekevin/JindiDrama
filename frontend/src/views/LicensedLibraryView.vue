<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import { BookOpen, Download, Film, Heart, PictureInPicture2, Search } from 'lucide-vue-next'
import { usePictureInPicture } from '@/player/pictureInPicture'
import { exportLicensedEpisode, hasLicensedExport, loadProviderEpisodes, resolvePlayback } from '@/bindings'
import { BEFORE_WINDOW_CLOSE, WINDOW_CLOSE_CANCELED } from '@/bindings/windowClose'
import { isPlaybackSpeed, playbackEndAction } from '@/player/preferences'
import { useAppSettings } from '@/stores/appSettings'
import { useSettingsStore } from '@/stores/player'
import { useLicensedCatalog } from '@/stores/licensedCatalog'
import { useLibraryStore } from '@/stores/library'
import { useToastStore } from '@/stores/toast'
import { clearStartupPlayback, detailRoute, playbackRoute, retryStartupPlayback } from '@/router/libraryRoutes'
import LicensedDirectorySettings from '@/components/LicensedDirectorySettings.vue'
import { BaseButton, EmptyState } from '@/components/ui'
import type { HistoryEntry } from '@/types'
import type { Episode, MediaSource } from '../../../contracts/provider-contracts'

const route = useRoute()
const router = useRouter()
const catalog = useLicensedCatalog()
const library = useLibraryStore()
const toast = useToastStore()
const settings = useSettingsStore()
const preferences = useAppSettings()
const query = ref('')
const pendingActivation = ref(false)
const requestedId = computed(() => typeof route.query.id === 'string' ? route.query.id : '')
const selected = computed(() => catalog.entries.find(({ id }) => id === requestedId.value))
const episodes = ref<Episode[]>([])
const episode = computed(() => {
  if (typeof route.query.ep !== 'string' || !/^[1-9]\d*$/.test(route.query.ep)) return 0
  const index = Number(route.query.ep)
  return Number.isSafeInteger(index) ? index : 0
})
const sources = ref<MediaSource[]>([])
const playbackError = ref('')
const playbackNotice = ref('')
const loadingEpisodes = ref(false)
const episodeListError = ref(false)
const episodeListAttempt = ref(0)
const episodeList = ref<HTMLDivElement | null>(null)
const episodeRetryButton = ref<HTMLButtonElement | null>(null)
const loadingPlayback = ref(false)
const playbackAttempt = ref(0)
const exporting = ref(false)

const results = computed(() => {
  const text = query.value.trim().toLowerCase()
  return catalog.entries.filter(({ title, summary, tags }) =>
    [title, summary, ...tags].join('\n').toLowerCase().includes(text))
})

const video = ref<HTMLVideoElement | null>(null)
const { active: pipActive, busy: pipBusy, toggle: togglePip } = usePictureInPicture(video)
type EpisodeRef = { id: string; index: number }
type AutoNext = EpisodeRef & { previous: number; muted: boolean; volume: number }
type ActivePlayback = EpisodeRef & {
  episodeCountAtWatch: number
  checkpoint?: HistoryEntry
  pending: boolean
  autoStart?: AutoNext
  startOnOpen: boolean
  atEnd: boolean
  restoring?: number
  dirty: boolean
  hasPlayed: boolean
  last: number
  lastPercent: number
}
let playback: ActivePlayback | undefined
let nextToStart: AutoNext | undefined
let returnToEnded: EpisodeRef | undefined
let waitingForEpisodeList: ActivePlayback | undefined
const closing = ref(false)
let leaving = false
onBeforeRouteLeave(() => { leaving = true })
onBeforeRouteUpdate(() => { leaving = false })

const canExport = computed(() => !!selected.value?.allowExport && hasLicensedExport() &&
  sources.value.length > 0 && playback?.id === selected.value.id && playback.index === episode.value)

async function saveEpisode() {
  if (!selected.value || !canExport.value || exporting.value) return
  exporting.value = true
  try {
    if (await exportLicensedEpisode(selected.value.id, episode.value)) toast.push('本地片库视频已保存', 'success')
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : String(cause ?? '')
    toast.push(message || '本地片库视频保存失败', 'error')
  } finally {
    exporting.value = false
  }
}

function openDrama(id: string) {
  nextToStart = undefined
  returnToEnded = undefined
  clearStartupPlayback()
  void router.replace(detailRoute(id))
}

function openEpisode(index: number) {
  nextToStart = undefined
  returnToEnded = undefined
  clearStartupPlayback()
  if (!selected.value) return
  if (index !== episode.value) void router.replace(playbackRoute(selected.value.id, index))
  else if (playbackError.value) playbackAttempt.value++
}

function retryEpisodeList() {
  if (loadingEpisodes.value || !episodeListError.value || closing.value || leaving) return
  episodeListAttempt.value++
}

function saveProgress(force = false) {
  const media = video.value
  if (!playback?.dirty || playback.pending || playback.restoring !== undefined || !media || !Number.isFinite(media.duration) || media.duration <= 0) return
  if (!playback.hasPlayed && playback.checkpoint && playback.checkpoint.percent >= 100) return
  const position = media.currentTime
  if (!Number.isFinite(position) || (!force && Math.abs(position - playback.last) < 5)) return
  const progress = media.ended && playback.hasPlayed ? 100 : Math.min(99, Math.floor(position * 100 / media.duration))
  if (position === playback.last && progress === playback.lastPercent) return
  playback.last = position
  playback.lastPercent = progress
  const previousEpisodeCount = playback.checkpoint?.episode === playback.index
    ? playback.checkpoint.episodeCountAtWatch : undefined
  const episodeCountAtWatch = playback.hasPlayed ? playback.episodeCountAtWatch : previousEpisodeCount
  library.record(playback.id, playback.index, progress, position, media.duration, episodeCountAtWatch)
}

function currentMedia(event: Event) {
  return event.currentTarget === video.value
}

function resumePosition(checkpoint: HistoryEntry | undefined, episode: number, duration: number) {
  if (!checkpoint || checkpoint.episode !== episode || checkpoint.percent >= 100) return 0
  const previousPosition = checkpoint.positionSec ?? 0
  const previousDuration = checkpoint.durationSec ?? 0
  if (Number.isFinite(previousPosition) && previousPosition > 0) {
    if (previousPosition < duration) return previousPosition
    if (Number.isFinite(previousDuration) && previousPosition < previousDuration) {
      return (previousPosition / previousDuration) * duration
    }
  }
  const percent = Number.isFinite(checkpoint.percent) ? Math.min(99, Math.max(0, checkpoint.percent)) : 0
  return (percent / 100) * duration
}

function seekablePosition(ranges: TimeRanges, position: number, currentTime: number) {
  let nearest = position
  let bestDistance = Infinity
  for (let index = 0; index < ranges.length; index++) {
    const candidate = Math.min(Math.max(position, ranges.start(index)), ranges.end(index))
    const distance = Math.abs(candidate - position)
    if (distance < bestDistance || (distance === bestDistance &&
      Math.abs(candidate - currentTime) < Math.abs(nearest - currentTime))) {
      nearest = candidate
      bestDistance = distance
    }
  }
  return nearest
}

function restoreProgress(event: Event) {
  const media = video.value
  if (!media || !currentMedia(event) || !playback?.pending || !Number.isFinite(media.duration) || media.duration <= 0) return
  const position = playback.atEnd ? media.duration : resumePosition(playback.checkpoint, playback.index, media.duration)
  const ranges = media.seekable
  if (position > 0 && !ranges.length && media.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) return
  playback.pending = false
  if (!ranges.length || position <= 0) return
  const target = seekablePosition(ranges, position, media.currentTime)
  if (target === media.currentTime) return
  playback.restoring = target
  media.currentTime = target
}

function onPlayable(event: Event) {
  if (!playback || !currentMedia(event)) return
  restoreProgress(event)
  clearStartupPlayback()
  if (playback.pending || playback.restoring !== undefined || !video.value) return
  const automatic = playback.autoStart
  if (automatic) {
    playback.autoStart = undefined
    video.value.muted = automatic.muted
    video.value.volume = automatic.volume
    startMedia('已切换到下一集，请点击播放。')
  } else if (playback.startOnOpen) {
    playback.startOnOpen = false
    startMedia('请点击播放开始本集。')
  }
}

function startMedia(fallback: string) {
  const media = video.value
  if (!media || closing.value || leaving) return
  const target = playback
  void media.play().catch(() => {
    if (!closing.value && !leaving && target === playback && media === video.value && media.paused) playbackNotice.value = fallback
  })
}

watch([video, () => settings.speed], ([media, speed]) => {
  if (!media) return
  media.defaultPlaybackRate = speed
  media.playbackRate = speed
}, { flush: 'post' })

function onRateChange(event: Event) {
  const media = video.value
  if (media && currentMedia(event) && isPlaybackSpeed(media.playbackRate)) {
    settings.speed = media.playbackRate
  }
}

function reportPlaybackFailure(message: string, automatic = playback?.autoStart) {
  playbackError.value = message
  if (!selected.value || closing.value || leaving) return
  if (automatic?.id === selected.value.id && automatic.index === episode.value) {
    if (playback) playback.autoStart = undefined
    returnToEnded = { id: automatic.id, index: automatic.previous }
    void router.replace(playbackRoute(automatic.id, automatic.previous))
    return
  }
  const next = retryStartupPlayback(selected.value.id, episode.value)
  if (next) void router.replace(next)
}

function onMediaFailure(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (playback && (target === video.value || target?.parentElement === video.value)) {
    reportPlaybackFailure('本集媒体无法播放')
  }
}

function onPlaying(event: Event) {
  if (currentMedia(event) && playback) {
    playback.dirty = true
    playback.hasPlayed = true
    playback.atEnd = false
    playbackNotice.value = ''
  }
}
function onSeeked(event: Event) {
  const media = video.value
  if (!media || !currentMedia(event) || !playback) return
  const target = playback.restoring
  playback.restoring = undefined
  if (target !== undefined && Math.abs(seekablePosition(media.seekable, target, media.currentTime) - media.currentTime) < 0.15) {
    playback.last = media.currentTime
    playback.lastPercent = Math.min(99, Math.floor(media.currentTime * 100 / media.duration))
  } else {
    playback.dirty = true
    saveProgress(true)
  }
  if ((playback.autoStart || playback.startOnOpen) && media.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) onPlayable(event)
}
function onPause(event: Event) {
  if (currentMedia(event)) saveProgress(true)
}
function onEnded(event: Event) {
  const media = video.value
  const current = playback
  const drama = selected.value
  if (!media || !currentMedia(event) || !current || drama?.id !== current.id || episode.value !== current.index) return
  saveProgress(true)
  if (closing.value || leaving || current.atEnd || !media.ended) return

  const next = current.index + 1
  if (next <= drama.updatedTo && loadingEpisodes.value && preferences.s.playMode === '连续播放（自动下一集）') {
    waitingForEpisodeList = current
    return
  }
  const available = next <= drama.updatedTo && episodes.value.some((item) => item.index === next && item.available)
  const action = playbackEndAction(preferences.s.playMode, available)
  if (action === 'loop') {
    current.dirty = false
    current.restoring = 0
    media.currentTime = 0
    startMedia('请点击播放重新开始本集。')
  } else if (action === 'next') {
    advanceEpisode(current, media)
  }
}

function advanceEpisode(current: ActivePlayback, media: HTMLVideoElement) {
  const next = current.index + 1
  const automatic = { id: current.id, index: next, previous: current.index, muted: media.muted, volume: media.volume }
  nextToStart = automatic
  void router.replace(playbackRoute(current.id, next)).then(() => {
    if (nextToStart === automatic) nextToStart = undefined
  })
}

function continueAfterEpisodeList() {
  if (closing.value || leaving || loadingEpisodes.value || episodeListError.value) return
  const waiting = waitingForEpisodeList
  waitingForEpisodeList = undefined
  const media = video.value
  if (waiting && waiting === playback && selected.value?.id === waiting.id && media?.ended &&
      preferences.s.playMode === '连续播放（自动下一集）' &&
      episodes.value.some(item => item.index === waiting.index + 1 && item.available)) {
    advanceEpisode(waiting, media)
  }
}

function onTimeUpdate(event: Event) {
  if (currentMedia(event) && video.value && !video.value.seeking) saveProgress()
}
function saveOnExit() { saveProgress(true) }
function onWindowClose() {
  closing.value = true
  video.value?.pause()
  saveOnExit()
}
function onWindowCloseCanceled() {
  closing.value = false
  continueAfterEpisodeList()
}
onMounted(() => {
  window.addEventListener(WINDOW_CLOSE_CANCELED, onWindowCloseCanceled)
  window.addEventListener('pagehide', saveOnExit)
  window.addEventListener(BEFORE_WINDOW_CLOSE, onWindowClose)
})
onBeforeUnmount(() => {
  closing.value = true
  saveOnExit()
  clearStartupPlayback()
  window.removeEventListener(WINDOW_CLOSE_CANCELED, onWindowCloseCanceled)
  window.removeEventListener('pagehide', saveOnExit)
  window.removeEventListener(BEFORE_WINDOW_CLOSE, onWindowClose)
})

watch([selected, episodeListAttempt], async ([drama], previous, onCleanup) => {
  if (drama !== previous?.[0]) episodeListError.value = false
  episodes.value = []
  loadingEpisodes.value = false
  if (!drama) return
  let active = true
  onCleanup(() => { active = false })
  loadingEpisodes.value = true
  try {
    const list = await loadProviderEpisodes(drama.id)
    if (!active) return
    episodes.value = list ?? []
    if (!closing.value && !leaving && episodeRetryButton.value === document.activeElement) episodeList.value?.focus()
    episodeListError.value = false
  } catch {
    if (active) episodeListError.value = true
  } finally {
    if (active) {
      loadingEpisodes.value = false
      continueAfterEpisodeList()
    }
  }
}, { immediate: true })

watch([selected, () => route.query.ep, () => route.query.autoplay, playbackAttempt], async ([drama, requestedEpisode, autoplay], _, onCleanup) => {
  saveProgress(true)
  playback = undefined
  waitingForEpisodeList = undefined
  sources.value = []
  playbackError.value = ''
  playbackNotice.value = ''
  loadingPlayback.value = false
  const automatic = nextToStart
  const previous = returnToEnded
  nextToStart = undefined
  returnToEnded = undefined
  if (!drama || requestedEpisode === undefined) return
  const index = episode.value
  if (index < 1 || index > drama.updatedTo) {
    playbackError.value = '该分集已不可用，请选择其他分集。'
    return
  }
  const autoStart = automatic?.id === drama.id && automatic.index === index ? automatic : undefined
  const atEnd = previous?.id === drama.id && previous.index === index
  if (atEnd) playbackNotice.value = '下一集媒体无法播放，已停在本集结束处。'
  let active = true
  onCleanup(() => { active = false })
  loadingPlayback.value = true
  try {
    const ticket = await resolvePlayback(drama.id, index)
    if (!active) return
    if (ticket?.kind !== 'licensed' || !ticket.sources?.length) {
      reportPlaybackFailure('本集暂无可播放媒体', autoStart)
      return
    }
    const saved = library.history.find((item) => item.dramaId === drama.id)
    playback = { id: drama.id, index, episodeCountAtWatch: drama.episodeCount, checkpoint: saved, pending: true, autoStart, startOnOpen: autoplay === '1', atEnd, dirty: false, hasPlayed: false, last: -1, lastPercent: -1 }
    sources.value = ticket.sources
  } catch {
    if (active) reportPlaybackFailure('无法读取本集媒体', autoStart)
  } finally {
    if (active) loadingPlayback.value = false
  }
}, { immediate: true })
</script>

<template>
  <section class="licensed" data-testid="licensed-library" aria-labelledby="licensed-heading">
    <header class="heading"><BookOpen :size="26" /><h1 id="licensed-heading">本地片库</h1></header>
    <LicensedDirectorySettings @pending-activation="pendingActivation = $event" />
    <p v-if="catalog.status === 'loading'" role="status">正在读取片库…</p>
    <section v-else-if="catalog.status === 'error'" class="detail" role="alert">
      <h2>片库配置未加载</h2>
      <p>{{ catalog.error }}。修改配置后请重启应用。</p>
    </section>
    <EmptyState v-else-if="!catalog.entries.length && !pendingActivation" title="尚未配置本地片库" hint="在桌面应用中选择包含 catalog.json 的授权媒体目录。" />
    <div v-else-if="catalog.entries.length" class="layout">
      <section class="catalog" aria-label="本地短剧">
        <label class="filter">
          <Search :size="17" />
          <input v-model="query" type="search" placeholder="搜索本地片库" aria-label="搜索本地片库" />
        </label>
        <p v-if="!results.length" role="status">未找到匹配的短剧</p>
        <button
          v-for="drama in results" :key="drama.id" :aria-pressed="selected?.id === drama.id"
          class="title" @click="openDrama(drama.id)"
        >
          <Film :size="20" /><span>{{ drama.title }}</span><small>{{ drama.episodeCount }} 集</small>
        </button>
      </section>
      <section v-if="selected" class="detail" aria-label="短剧与分集">
        <div class="detail-heading">
          <h2>{{ selected.title }}</h2>
          <BaseButton variant="outline" :aria-pressed="library.isFav(selected.id)" @click="library.toggleFav(selected.id)">
            <Heart :size="16" :fill="library.isFav(selected.id) ? 'currentColor' : 'none'" />
            {{ library.isFav(selected.id) ? '已收藏' : '收藏' }}
          </BaseButton>
        </div>
        <p v-if="selected.summary">{{ selected.summary }}</p>
        <p class="license">许可说明 · {{ selected.license }}</p>
        <p v-if="loadingEpisodes" role="status">正在读取分集…</p>
        <div v-if="episodeListError" class="episode-recovery">
          <p role="alert">无法读取分集</p>
          <button ref="episodeRetryButton" type="button" class="episode-retry"
            :aria-disabled="loadingEpisodes || closing" @click="retryEpisodeList">重试读取分集</button>
        </div>
        <div ref="episodeList" class="episodes" role="group" aria-label="分集" tabindex="-1">
          <BaseButton
            v-for="item in episodes" :key="item.id" :disabled="!item.available" :aria-pressed="episode === item.index"
            :variant="episode === item.index ? 'primary' : 'outline'" @click="openEpisode(item.index)"
          >第 {{ item.index }} 集</BaseButton>
        </div>
        <p v-if="loadingPlayback" role="status">正在读取媒体…</p>
        <p v-if="playbackError" role="alert">{{ playbackError }}</p>
        <p v-if="playbackNotice" role="status">{{ playbackNotice }}</p>
        <div v-if="sources.length" class="episodes">
          <BaseButton variant="outline" :aria-pressed="pipActive" :disabled="pipBusy" @click="togglePip">
            <PictureInPicture2 :size="16" />{{ pipActive ? '退出画中画' : '画中画' }}
          </BaseButton>
          <BaseButton v-if="canExport" variant="outline" :disabled="exporting" @click="saveEpisode">
            <Download :size="16" />{{ exporting ? '正在保存…' : '保存本集' }}
          </BaseButton>
        </div>
        <video
          v-if="sources.length" ref="video" :key="sources[0].src + ':' + playbackAttempt" controls controlslist="nodownload" playsinline preload="auto"
          @loadedmetadata="restoreProgress" @canplay="onPlayable" @playing="onPlaying"
          @timeupdate="onTimeUpdate" @seeked="onSeeked" @pause="onPause" @ended="onEnded" @ratechange="onRateChange"
          :aria-label="`第 ${episode} 集播放器`" @error="onMediaFailure"
        >
          <source v-for="(source, i) in sources" :key="source.src" :src="source.src" :type="source.type"
            @error="i === sources.length - 1 && onMediaFailure($event)" />
        </video>
      </section>
      <p v-else-if="requestedId" class="detail" role="alert">该短剧当前不在已配置的片库中。</p>
    </div>
  </section>
</template>

<style scoped lang="scss">
.licensed { max-width: 1200px; margin: 0 auto; padding: 24px; }
.heading { display: flex; align-items: center; gap: 12px; margin-bottom: 24px; }
.heading svg { color: var(--c-accent); }
.heading h1 { font-size: 27px; }
.layout { display: grid; grid-template-columns: minmax(210px, 30%) minmax(0, 1fr); gap: 16px; align-items: start; }
.catalog, .detail { background: var(--c-surface); border: 1px solid var(--c-border); border-radius: 14px; padding: 16px; min-width: 0; }
.filter, .title { display: flex; align-items: center; gap: 10px; }
.filter { padding: 10px 12px; border: 1px solid var(--c-border); border-radius: 8px; margin-bottom: 12px; }
.filter:focus-within { outline: 2px solid var(--c-accent-2); outline-offset: 2px; }
.filter input { appearance: none; background: transparent; border: 0; outline: 0; flex: 1; min-width: 0; }
.title { padding: 10px; border-radius: 8px; width: 100%; text-align: left; }
.title:hover, .title[aria-pressed='true'] { background: var(--c-surface-hover); }
.title span { flex: 1; overflow-wrap: anywhere; }
.title small, .license { color: var(--c-text-2); }
.detail-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.detail h2, .detail p { margin-bottom: 12px; overflow-wrap: anywhere; }
.episode-recovery { display: flex; align-items: baseline; flex-wrap: wrap; column-gap: 12px; }
.episode-retry { color: var(--c-accent-2); text-decoration: underline; }
.episode-retry[aria-disabled='true'] { color: var(--c-text-2); cursor: default; }
.episodes { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
video { display: block; width: 100%; max-height: 530px; background: #000; border-radius: 8px; }
@media (max-width: 680px) { .layout { grid-template-columns: 1fr; } }
</style>
