<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronLeft, Cast, PictureInPicture2, Pause, Play, SkipForward, MessageSquareText, MessageCircle, Download, Volume2, VolumeX, Maximize, RotateCw, Loader } from 'lucide-vue-next'
import type { usePlayer } from '@/player/usePlayer'
import { fmt } from '@/player/machine'
import { posterSvg } from '@/mocks/posters'
import type { Drama } from '@/types'
import type { DanmakuItem } from '@/player/danmaku'
import { useSettingsStore } from '@/stores/player'
import { useToastStore } from '@/stores/toast'
import DanmakuLayer from './DanmakuLayer.vue'
import { demoSources } from '@/player/usePlayer'

const props = defineProps<{ player: ReturnType<typeof usePlayer>; drama: Drama; items: DanmakuItem[]; variant: 'tall' | 'wide' | 'danmaku'; episode: number; hasNext: boolean; title?: string }>()
const emit = defineEmits<{ next: []; prev: []; back: []; comments: []; danmakuPanel: [] }>()
const s = useSettingsStore(); const toast = useToastStore(); const root = ref<HTMLElement>()
const bg = computed(() => posterSvg({ hue: 350, hue2: 20, seed: 1 + (props.episode % 3) }))
const SPEEDS = [0.75, 1, 1.25, 1.5, 2]; const QUALITIES = ['流畅', '高清', '超清', '蓝光']
const menu = ref<'' | 'speed' | 'quality'>('')
const p = props.player
const toggleDm = () => { s.danmaku.enabled = !s.danmaku.enabled }
function seekClick(e: MouseEvent) { const el = e.currentTarget as HTMLElement; const r = el.getBoundingClientRect(); p.seek(((e.clientX - r.left) / r.width) * p.duration.value) }
async function fullscreen() { try { if (document.fullscreenElement) await document.exitFullscreen(); else await root.value?.requestFullscreen() } catch { toast.push('当前环境不支持全屏', 'error') } }
function pip() { if (!(document as any).pictureInPictureEnabled) { toast.push('当前环境不支持画中画', 'error'); return } toast.push('演示模式使用模拟画面，暂无可用画中画视频源') }
function key(e: KeyboardEvent) {
  const t = (e.target as HTMLElement).tagName; if (t === 'INPUT' || t === 'TEXTAREA') return
  if (e.code === 'Space') { e.preventDefault(); p.toggle() }
  else if (e.key === 'ArrowRight' && e.ctrlKey) emit('next')
  else if (e.key === 'ArrowLeft' && e.ctrlKey) emit('prev')
  else if (e.key === 'ArrowRight') p.seekBy(10)
  else if (e.key === 'ArrowLeft') p.seekBy(-10)
  else if (e.key.toLowerCase() === 'f') fullscreen()
  else if (e.key.toLowerCase() === 'd') toggleDm()
  else if (e.key === 'Escape') menu.value = ''
}
const volIcon = computed(() => (p.muted.value || s.volume === 0 ? VolumeX : Volume2))
</script>
<template>
  <div ref="root" class="player" :class="variant" tabindex="0" data-testid="player" :data-state="player.state.value" @keydown="key">
    <video v-if="p.mode === 'demo'" :ref="(el) => (p.video.value = el as HTMLVideoElement | null)" class="vid" playsinline preload="auto" data-testid="video" @loadedmetadata="p.onMeta" @timeupdate="p.onTime" @ended="p.onEnded" @waiting="p.onWaiting" @playing="p.onPlaying" @error="p.onError" @click="p.toggle()"><source v-for="(s, i) in demoSources(episode)" :key="s.src" :src="s.src" :type="s.type" @error="i === demoSources(episode).length - 1 && p.onError()" /></video>
    <div v-else class="bg" :style="{ backgroundImage: `url('${bg}')` }" @click="p.toggle()" />
    <DanmakuLayer :items="items" :time="player.time.value" />
    <header class="top">
      <button class="rnd" aria-label="返回" data-testid="player-back" @click="emit('back')"><ChevronLeft :size="22" /></button>
      <span v-if="variant === 'danmaku'" class="ttl">{{ drama.title }} 第{{ episode }}集</span>
      <span class="sp" />
      <button v-if="variant === 'tall'" class="chip" data-testid="cast" @click="toast.push('投屏需要局域网设备，演示模式未启用')"><Cast :size="16" />投屏</button>
      <button v-if="variant !== 'wide'" class="chip" data-testid="pip" @click="pip"><PictureInPicture2 :size="16" />画中画</button>
    </header>
    <div v-if="player.buffering.value && player.state.value === 'playing'" class="buf" data-testid="buffering"><Loader :size="22" class="spin" /></div>
    <div v-if="player.state.value === 'resolving'" class="over" data-testid="loading"><Loader :size="34" class="spin" /><span>正在获取播放源…</span></div>
    <div v-else-if="player.state.value === 'error'" class="over err" role="alert" data-testid="player-error"><span>播放失败：{{ player.error.value }}</span><span class="hint">仅演示数据，可重试或切换其他集</span><button class="retry" data-testid="retry" @click="player.retry()"><RotateCw :size="15" />重试</button></div>
    <div v-else-if="player.state.value === 'ended'" class="over end" data-testid="ended"><button class="retry" @click="player.play()"><RotateCw :size="15" />重新播放</button><button v-if="hasNext" class="retry" @click="emit('next')">下一集</button></div>
    <footer class="ctl">
      <div v-if="variant !== 'wide'" class="time">{{ fmt(player.time.value) }} / {{ fmt(player.duration.value) }}</div>
      <div class="seek" role="slider" tabindex="0" aria-label="播放进度" :aria-valuenow="Math.round(player.percent.value)" aria-valuemin="0" aria-valuemax="100" data-testid="seek" @click="seekClick" @keydown.left.stop="p.seekBy(-10)" @keydown.right.stop="p.seekBy(10)"><i :style="{ width: player.percent.value + '%' }" /><b :style="{ left: player.percent.value + '%' }" /></div>
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
        <div v-if="variant !== 'danmaku'" class="menu"><button class="pill plain" data-testid="speed" @click="menu = menu === 'speed' ? '' : 'speed'">{{ variant === 'tall' ? '倍速 ' : '' }}{{ s.speed.toFixed(1) }}x ▾</button>
          <ul v-if="menu === 'speed'" class="pop"><li v-for="v in SPEEDS" :key="v"><button :class="{ on: s.speed === v }" :data-testid="`speed-${v}`" @click="s.speed = v; menu = ''">{{ v }}x</button></li></ul></div>
        <div class="menu"><button class="pill plain" data-testid="quality" @click="menu = menu === 'quality' ? '' : 'quality'">{{ variant === 'tall' ? '画质 ' : '' }}{{ s.quality }} ▾</button>
          <ul v-if="menu === 'quality'" class="pop"><li v-for="q in QUALITIES" :key="q"><button :class="{ on: s.quality === q }" :data-testid="`q-${q}`" @click="s.quality = q; menu = ''">{{ q }}</button></li></ul></div>
        <button v-if="variant === 'tall'" class="pill plain" @click="toast.push('下载功能将在 P1 本地存储完成后开放')"><Download :size="14" />下载</button>
        <button class="ic" :aria-label="p.muted.value ? '取消静音' : '静音'" data-testid="mute" @click="p.muted.value = !p.muted.value"><component :is="volIcon" :size="20" /></button>
        <input v-if="variant !== 'wide' || true" v-model.number="s.volume" class="vol" type="range" min="0" max="100" aria-label="音量" data-testid="volume" />
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
.top { position: absolute; left: 0; right: 0; top: 0; z-index: 3; display: flex; align-items: center; gap: 10px; padding: 14px 16px; .sp { flex: 1; } .ttl { font-size: 18px; font-weight: 600; margin-left: 4px; } }
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
.menu { position: relative; } .pop { position: absolute; bottom: 38px; right: 0; min-width: 90px; padding: 6px; border-radius: 12px; background: #2a1719; border: 1px solid var(--c-border-strong); button { width: 100%; height: 30px; border-radius: 8px; &.on { color: var(--c-accent-2); } &:hover { background: var(--c-surface-hover); } } }
.vol { width: 70px; accent-color: #ee4b39; }
.wide .vol { display: none; } .danmaku .ctl { padding-bottom: 14px; }
</style>
