<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft } from 'lucide-vue-next'
import { releaseCatalogPlayback, resolveCatalogPlayback } from '@/bindings'
import { ErrorState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'
import { getDetail, type DramaDetail } from '@/mocks/provider'
import { demoQualityId } from '@/player/demoMedia'
import { catalogPlaybackRoute, detailRoute } from '@/router/libraryRoutes'
import { useAppSettings } from '@/stores/appSettings'
import { useSettingsStore } from '@/stores/player'
import type { MediaSource, PlaybackTicket } from '../../../contracts/provider-contracts'

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

function stopMedia() {
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
    if (choice.type !== 'video/mp4' && choice.type !== 'video/webm') continue
    try {
      const url = new URL(choice.src)
      if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port ||
          url.username || url.password || url.hash || url.search) continue
      if (!new RegExp(`^/catalog-media/${token}/[0-9]+$`).test(url.pathname)) continue
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
    const requested = result.episodes.find(({ index }) => index === targetEpisode)
    if (!requested?.available || !requested.id) {
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

function mediaError(event: Event) {
  if (event.currentTarget !== video.value || state.value !== 'ready') return
  if (expiresAt <= Date.now()) {
    expireTicket()
    return
  }
  stopMedia()
  if (selected.value + 1 < sources.value.length) {
    selected.value++
    return
  }
  state.value = 'error'
  error.value = '媒体加载失败，请重试。'
  sources.value = []
  releaseLease()
}

function applyPreferences(event: Event) {
  if (event.currentTarget !== video.value || !video.value) return
  video.value.volume = settings.volume / 100
  video.value.playbackRate = settings.speed
}

watch([id, episode], load, { immediate: true })
onBeforeUnmount(() => {
  generation++
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
        controlslist="nodownload" :src="source.src"
        :aria-label="`第${episode}集视频`" @error="mediaError" @loadedmetadata="applyPreferences" />
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
