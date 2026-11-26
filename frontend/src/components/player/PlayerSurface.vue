<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { Camera, ChevronLeft, Cast, PictureInPicture2, Pause, Play, SkipForward, Settings2, MessageSquareText, MessageCircle, Download, Volume2, VolumeX, Maximize, RotateCw, Loader } from 'lucide-vue-next'
import type { usePlayer } from '@/player/usePlayer'
import { DEMO_QUALITIES, demoQualityId } from '@/player/demoMedia'
import { fmt, formatSpeed } from '@/player/machine'
import { posterSvg } from '@/mocks/posters'
import type { Drama } from '@/types'
import type { DanmakuItem } from '@/player/danmaku'
import { useSettingsStore } from '@/stores/player'
import { useToastStore } from '@/stores/toast'
import { exportDemoScreenshot, hasBridge } from '@/bindings'
import { usePictureInPicture } from '@/player/pictureInPicture'
import { usePlaybackShortcuts } from '@/player/usePlaybackShortcuts'
import DanmakuLayer from './DanmakuLayer.vue'

const props = defineProps<{ player: ReturnType<typeof usePlayer>; drama: Drama; items: DanmakuItem[]; variant: 'tall' | 'wide' | 'danmaku'; episode: number; hasNext: boolean; exporting?: boolean; title?: string }>()
const emit = defineEmits<{ next: []; prev: []; back: []; comments: []; danmakuPanel: []; download: [] }>()
const s = useSettingsStore(); const toast = useToastStore(); const root = ref<HTMLElement>()
const screenshotBusy = ref(false)
const header = ref<HTMLElement>()
const footer = ref<HTMLElement>()
const danmakuTop = ref(40)
const danmakuBottom = ref(70)
const surfaceWidth = ref(Infinity)
const popupHeight = ref(240)
const compactControls = computed(() => surfaceWidth.value <= (props.variant === 'tall' ? 720 : 450))
let controlObserver: ResizeObserver | undefined
onMounted(() => {
  const surface = root.value
  const top = header.value
  const controls = footer.value
  if (!surface || !top || !controls) return
  const updateInsets = () => {
    const controlsHeight = controls.offsetHeight
    popupHeight.value = Math.max(36, surface.clientHeight - controlsHeight - 10)
    danmakuTop.value = top.offsetHeight + 4
    danmakuBottom.value = controlsHeight + 4
  }
  const updateControls = () => {
    surfaceWidth.value = surface.clientWidth
    updateInsets()
  }
  controlObserver = new ResizeObserver(updateControls)
  controlObserver.observe(surface)
  controlObserver.observe(top, { box: 'border-box' })
  controlObserver.observe(controls, { box: 'border-box' })
  // The parent can switch to its stacked grid during the first render.
  void nextTick(async () => {
    if (!surface.isConnected) return
    updateControls()
    await nextTick()
    if (surface.isConnected) updateInsets()
  })
})
onBeforeUnmount(() => controlObserver?.disconnect())
const bg = computed(() => posterSvg({ hue: 350, hue2: 20, seed: 1 + (props.episode % 3) }))
const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2]
const menu = ref<'' | 'speed' | 'quality'>('')
function dismissMenu(event: Event) {
  menu.value = ''
  const trigger = (event.currentTarget as HTMLElement).closest('.menu')?.querySelector('button')
  trigger?.focus({ preventScroll: true })
}
watch(() => props.variant, () => {
  menu.value = ''
  if (root.value?.contains(document.fullscreenElement)) {
    void document.exitFullscreen().catch(() => toast.push('无法退出全屏', 'error'))
  }
})
const p = props.player
const macDesktop = hasBridge() && navigator.userAgent.includes('Macintosh')
// WKWebView rendering and AVFoundation screenshots use the same MP4.
const playbackSources = computed(() => macDesktop
  ? [...p.sources.value.filter((source) => source.type === 'video/mp4'), ...p.sources.value.filter((source) => source.type !== 'video/mp4')]
  : p.sources.value)
let presentedFrame: { src: string; mediaTime: number } | undefined
watch(p.video, (video, _, onCleanup) => {
  if (!video || typeof video.requestVideoFrameCallback !== 'function') return
  let callbackId: number
  const remember: VideoFrameRequestCallback = (_, frame) => {
    presentedFrame = { src: video.currentSrc, mediaTime: frame.mediaTime }
    callbackId = video.requestVideoFrameCallback(remember)
  }
  callbackId = video.requestVideoFrameCallback(remember)
  onCleanup(() => {
    video.cancelVideoFrameCallback(callbackId)
    presentedFrame = undefined
  })
})
const toggleDm = () => { s.danmaku.enabled = !s.danmaku.enabled }
function seekClick(e: MouseEvent) { const el = e.currentTarget as HTMLElement; const r = el.getBoundingClientRect(); p.seek(((e.clientX - r.left) / r.width) * p.duration.value) }
async function fullscreen() { try { if (document.fullscreenElement) await document.exitFullscreen(); else await root.value?.requestFullscreen() } catch { toast.push('当前环境不支持全屏', 'error') } }
const { active: pipActive, busy: pipBusy, toggle: togglePip, reset: resetPip } = usePictureInPicture(p.video, p.mode === 'sim')
watch(p.state, (state) => { if (state === 'resolving') resetPip() }, { flush: 'sync' })

async function captureScreenshot() {
  if (screenshotBusy.value) return
  const video = p.video.value
  if (!video || p.state.value === 'resolving' || p.state.value === 'error' ||
      video.seeking || video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA || !video.videoWidth || !video.videoHeight) {
    toast.push('当前没有可截图的视频画面', 'info')
    return
  }
  screenshotBusy.value = true
  try {
    const desktop = hasBridge()
    const prefix = 'data:image/png;base64,'
    let uri = ''
    try {
      const canvas = document.createElement('canvas')
      canvas.width = video.videoWidth
      canvas.height = video.videoHeight
      const ctx = canvas.getContext('2d', { alpha: false })
      if (!ctx) throw new Error('当前环境无法生成视频截图')
      ctx.drawImage(video, 0, 0)
      uri = canvas.toDataURL('image/png')
      if (!uri.startsWith(prefix)) throw new Error('无法生成 PNG 视频截图')
    } catch (error) {
      // WKWebView cannot read a canvas containing a wails:// video.
      if (!desktop || !(error instanceof DOMException) || error.name !== 'SecurityError') throw error
      if (!macDesktop || !video.currentSrc.endsWith('.mp4')) throw new Error('当前媒体格式无法保存视频截图')
    }
    if (desktop) {
      const currentTime = video.ended ? Math.max(0, video.duration - 1 / 30) : video.currentTime
      const seconds = presentedFrame?.src === video.currentSrc && Math.abs(presentedFrame.mediaTime - currentTime) < 0.2
        ? presentedFrame.mediaTime : currentTime
      const job = exportDemoScreenshot(props.drama.id, props.episode, demoQualityId(s.quality), seconds, uri.slice(prefix.length))
      if (!job) throw new Error('当前桌面版本不支持保存截图')
      if (await job) toast.push('截图已保存', 'success')
      else toast.push('已取消保存截图')
    } else {
      const link = document.createElement('a')
      link.href = uri
      link.download = `JindiDrama-${props.drama.id}-${props.episode}.png`
      link.click()
    }
  } catch (error) {
    toast.push(typeof error === 'string' ? error : error instanceof Error ? error.message : '截图保存失败', 'error')
  } finally {
    screenshotBusy.value = false
  }
}

const actions: Record<string, () => void> = {
  playPause: p.toggle,
  forward: () => p.seekBy(10),
  back: () => p.seekBy(-10),
  next: () => emit('next'),
  prev: () => emit('prev'),
  fullscreen,
  danmaku: toggleDm,
  screenshot: captureScreenshot,
}

usePlaybackShortcuts(actions, { escape: () => { menu.value = '' } })

function seekKey(e: KeyboardEvent) {
  if (e.ctrlKey || e.altKey || e.metaKey || e.shiftKey || e.isComposing) return
  if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
  e.preventDefault()
  e.stopPropagation()
  p.seekBy(e.key === 'ArrowLeft' ? -10 : 10)
}
const volIcon = computed(() => (p.muted.value || s.volume === 0 ? VolumeX : Volume2))
</script>
<template>
  <div ref="root" class="player on-dark" :class="[variant, { compactControls, snug: surfaceWidth <= 800 }]" :style="{
      '--popup-height': popupHeight + 'px',
      '--danmaku-top': danmakuTop + 'px',
      '--danmaku-bottom': danmakuBottom + 'px',
    }" tabindex="0" data-testid="player" :data-state="player.state.value">
    <video v-if="p.mode === 'demo'" :ref="(el) => (p.video.value = el as HTMLVideoElement | null)" class="vid" playsinline preload="auto" data-testid="video" @loadedmetadata="p.onMeta" @canplay="p.onMeta" @timeupdate="p.onTime" @seeked="p.onSeeked" @ended="p.onEnded" @waiting="p.onWaiting" @playing="p.onMediaControl" @pause="p.onMediaControl" @error="p.onError" @click="p.toggle()"><source v-for="(s, i) in playbackSources" :key="s.src" :src="s.src" :type="s.type" @error="i === playbackSources.length - 1 && p.onError()" /></video>
    <div v-else class="bg" :style="{ backgroundImage: `url('${bg}')` }" @click="p.toggle()" />
    <DanmakuLayer :items="items" :time="player.time.value" :duration="player.duration.value" />
    <header ref="header" class="top">
      <button class="rnd" aria-label="返回" data-testid="player-back" @click="emit('back')"><ChevronLeft :size="22" /></button>
      <span v-if="variant === 'danmaku'" class="ttl">{{ drama.title }} 第{{ episode }}集</span>
      <span class="sp" />
      <button v-if="variant === 'tall'" class="chip" data-testid="cast" @click="toast.push('投屏需要局域网设备，演示模式未启用')"><Cast :size="16" />投屏</button>
      <button v-if="variant !== 'wide' || pipActive" class="chip" data-testid="pip" :aria-pressed="pipActive" :disabled="pipBusy" @click="togglePip"><PictureInPicture2 :size="16" />{{ pipActive ? '退出画中画' : '画中画' }}</button>
      <button v-if="variant !== 'danmaku'" class="chip" aria-label="弹幕设置" title="弹幕设置" data-testid="open-danmaku-settings" @click="emit('danmakuPanel')"><Settings2 :size="16" /></button>
      <button v-if="p.mode === 'demo'" class="chip" aria-label="截图" title="保存视频截图" :disabled="screenshotBusy" @click="captureScreenshot"><Camera :size="16" /></button>
    </header>
    <div v-if="player.buffering.value && player.state.value === 'playing'" class="buf" data-testid="buffering"><Loader :size="22" class="spin" /></div>
    <div v-if="player.state.value === 'resolving'" class="over" data-testid="loading"><Loader :size="34" class="spin" /><span>正在获取播放源…</span></div>
    <div v-else-if="player.state.value === 'error'" class="over err" role="alert" data-testid="player-error"><span>播放失败：{{ player.error.value }}</span><span class="hint">仅演示数据，可重试或切换其他集</span><button class="retry" data-testid="retry" @click="player.retry()"><RotateCw :size="15" />重试</button></div>
    <div v-else-if="player.state.value === 'ended'" class="over end" data-testid="ended"><button class="retry" @click="player.play()"><RotateCw :size="15" />重新播放</button><button v-if="hasNext" class="retry" @click="emit('next')">下一集</button></div>
    <footer ref="footer" class="ctl">
      <div v-if="variant !== 'wide'" class="time">{{ fmt(player.time.value) }} / {{ fmt(player.duration.value) }}</div>
      <div class="seek" role="slider" tabindex="0" aria-label="播放进度" :aria-valuenow="Math.round(player.percent.value)" aria-valuemin="0" aria-valuemax="100" data-testid="seek" @click="seekClick" @keydown="seekKey"><i :style="{ width: player.percent.value + '%' }" /><b :style="{ left: player.percent.value + '%' }" /></div>
      <div class="row">
        <button class="ic" :aria-label="player.state.value === 'playing' ? '暂停' : '播放'" data-testid="toggle" @click="p.toggle()"><component :is="player.state.value === 'playing' ? Pause : Play" :size="24" fill="currentColor" /></button>
        <button class="ic" aria-label="下一集" :disabled="!hasNext" @click="emit('next')"><SkipForward :size="20" fill="currentColor" /></button>
        <span v-if="variant === 'wide'" class="t2">{{ fmt(player.time.value) }} / {{ fmt(player.duration.value) }}</span>
        <template v-if="variant === 'tall'">
          <button class="pill" data-testid="next-ep" :disabled="!hasNext" @click="emit('next')">下一集</button>
          <button class="pill on" :class="{ off: !s.danmaku.enabled }" data-testid="dm-toggle" @click="toggleDm"><MessageSquareText :size="15" />弹幕</button>
          <button class="pill plain" data-testid="to-comments" @click="emit('comments')"><MessageCircle :size="15" />评论</button>
        </template>
        <span class="sp" />
        <div v-if="variant !== 'danmaku'" class="menu" @keydown.esc.stop="dismissMenu"><button class="pill plain" :aria-expanded="menu === 'speed'" data-testid="speed" @click="menu = menu === 'speed' ? '' : 'speed'">{{ variant === 'tall' ? '倍速 ' : '' }}{{ formatSpeed(s.speed) }}x ▾</button>
          <ul v-if="menu === 'speed'" class="pop"><li v-for="v in SPEEDS" :key="v"><button :class="{ on: s.speed === v }" :data-testid="`speed-${v}`" @click="s.speed = v; dismissMenu($event)">{{ v }}x</button></li></ul></div>
        <div class="menu" @keydown.esc.stop="dismissMenu"><button class="pill plain" :aria-expanded="menu === 'quality'" data-testid="quality" @click="menu = menu === 'quality' ? '' : 'quality'">{{ p.mode === 'sim' ? '模拟 ' : variant === 'tall' ? '画质 ' : '' }}{{ s.quality }} ▾</button>
          <ul v-if="menu === 'quality'" class="pop quality-pop"><li v-for="q in DEMO_QUALITIES" :key="q.id"><button :class="{ on: s.quality === q.label }" :data-testid="`q-${q.label}`" @click="s.quality = q.label; dismissMenu($event)">{{ q.label }} {{ q.detail }}</button></li></ul></div>
        <button v-if="variant === 'tall'" class="pill plain" :disabled="exporting" @click="emit('download')"><Download :size="14" />下载</button>
        <button class="ic" :aria-label="p.muted.value ? '取消静音' : '静音'" data-testid="mute" @click="p.muted.value = !p.muted.value"><component :is="volIcon" :size="20" /></button>
        <input v-model.number="s.volume" class="vol" type="range" min="0" max="100" aria-label="音量" data-testid="volume" />
        <button class="ic" aria-label="全屏" data-testid="fullscreen" @click="fullscreen"><Maximize :size="20" /></button>
      </div>
    </footer>
  </div>
</template>
<style scoped lang="scss">
.player { position: relative; width: 100%; height: 100%; border-radius: 22px; overflow: hidden; background: #1b0d0e; border: 1px solid var(--c-border); outline: none; &:focus-visible { box-shadow: 0 0 0 2px var(--c-accent-2); } }
.vid { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; background: #12090a; cursor: pointer; }
.buf { position: absolute; left: 50%; top: 50%; z-index: 4; transform: translate(-50%, -50%); }
.bg { position: absolute; inset: 0; background-size: cover; background-position: center; cursor: pointer; }
.player::after { content: ''; position: absolute; inset: 0; pointer-events: none; background: linear-gradient(180deg, rgba(0,0,0,.45), transparent 22%, transparent 60%, rgba(0,0,0,.72)); }
.top { position: absolute; left: 0; right: 0; top: 0; z-index: 3; display: flex; align-items: center; gap: 10px; padding: 14px 16px; .sp { flex: 1; } .ttl { min-width: 0; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; font-size: 18px; font-weight: 600; margin-left: 4px; } }
.rnd { width: 38px; height: 38px; border-radius: 50%; background: rgba(0,0,0,.45); border: 1px solid var(--c-border-strong); display: grid; place-items: center; }
.chip { display: inline-flex; align-items: center; gap: 6px; height: 36px; padding: 0 14px; border-radius: 10px; background: rgba(0,0,0,.5); border: 1px solid var(--c-border-strong); font-size: 14px; }
.over { position: absolute; inset: 0; z-index: 4; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; background: rgba(10,4,5,.6); font-size: 16px; .hint { font-size: 13px; color: var(--c-text-2); } &.end { flex-direction: row; } }
.retry { display: inline-flex; align-items: center; gap: 6px; height: 36px; padding: 0 20px; border-radius: 18px; background: var(--g-accent); font-weight: 600; }
.spin { animation: sp 1s linear infinite; } @keyframes sp { to { transform: rotate(360deg); } }
.ctl { position: absolute; left: 0; right: 0; bottom: 0; z-index: 3; padding: 0 22px 16px; font-size: 13px; }
.time { margin-bottom: 8px; color: var(--c-text); }
.seek { position: relative; height: 4px; border-radius: 4px; background: rgba(255,255,255,.28); cursor: pointer; margin: 6px 0 16px; &::before { content: ''; position: absolute; inset: -8px 0; } i { position: absolute; left: 0; top: 0; bottom: 0; border-radius: 4px; background: linear-gradient(90deg, #ff6a4d, #ee4b39); } b { position: absolute; top: 50%; width: 14px; height: 14px; border-radius: 50%; background: #fff; transform: translate(-50%, -50%); box-shadow: 0 0 6px rgba(0,0,0,.6); } }
.row { display: flex; align-items: center; gap: 10px; .sp { flex: 1; } .t2 { margin-left: 6px; } }
.ic { width: 34px; height: 34px; display: grid; place-items: center; color: #fff; border-radius: 8px; &:hover:not(:disabled) { background: rgba(255,255,255,.14); } &:disabled { opacity: .4; } }
.pill { white-space: nowrap; flex: none; display: inline-flex; align-items: center; gap: 5px; height: 30px; padding: 0 12px; border-radius: 15px; background: rgba(255,255,255,.14); font-size: 13px; &.on { background: rgba(238,75,57,.35); border: 1px solid var(--c-accent); &.off { background: rgba(255,255,255,.14); border-color: transparent; color: var(--c-text-2); } } &.plain { background: transparent; border: 1px solid rgba(255,255,255,.3); } &:disabled { opacity: .4; } }
.menu { position: relative; } .pop { &.quality-pop { min-width: 122px; white-space: nowrap; } position: absolute; bottom: 38px; right: 0; min-width: 90px; padding: 6px; border-radius: 12px; background: #2a1719; border: 1px solid var(--c-border-strong); button { width: 100%; height: 30px; border-radius: 8px; &.on { color: var(--c-accent-2); } &:hover { background: var(--c-surface-hover); } } }
.vol { width: 70px; accent-color: #ee4b39; }
.wide .vol { display: none; } .danmaku .ctl { padding-bottom: 14px; }

.player.tall.snug:not(.compactControls) {
  .ctl { padding-inline: 10px; }
  .row { flex-wrap: wrap; gap: 4px; }
  .row .sp { display: none; }
  .ic, .vol { flex: none; }
  .vol { width: 64px; margin: 0; }
}
.player.compactControls {
  .top { flex-wrap: wrap; padding: 10px 12px; gap: 6px; }
  .top .ttl { flex: 1; }
  .chip, .rnd, .ic, .vol { flex: none; }
  .chip { padding-inline: 8px; white-space: nowrap; }
  .ctl { padding: 8px 12px 10px; background: rgba(18, 9, 10, .78); }
  .row { flex-wrap: wrap; gap: 6px; }
  .row .sp { display: none; }
}
.player.compactControls, .player.tall.snug {
  .menu { position: static; }
  .pop { left: 12px; right: auto; bottom: 100%; z-index: 1; max-height: var(--popup-height); overflow-y: auto; }
}
</style>
