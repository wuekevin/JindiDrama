<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ChevronLeft, ChevronDown, ChevronUp, ChevronRight, Star, CirclePlay, Heart, Play, Plus, Check, Share2, Flame, Crown, RefreshCw, Landmark, Swords, Sprout, Sparkles } from 'lucide-vue-next'
import { getDetail, groupEpisodes, epHot, type DemoHighlight, type DramaDetail } from '@/mocks/provider'
import { useRelatedDramas } from '@/mocks/recommendations'
import { demoClipSources } from '@/player/demoMedia'
import { useAppSettings } from '@/stores/appSettings'
import { useLibraryStore } from '@/stores/library'
import { useToastStore } from '@/stores/toast'
import { copyShareLink } from '@/router/shareLinks'
import { continuationEpisode, detailRoute, savedMediaIssue } from '@/router/libraryRoutes'
import { BaseButton, ErrorState, EmptyState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'

const route = useRoute(); const router = useRouter(); const lib = useLibraryStore(); const toast = useToastStore()
const root = ref<HTMLElement>()
const layout = ref<'wide' | 'stacked' | 'compact'>('wide')
let layoutObserver: ResizeObserver | undefined
onMounted(() => {
  if (!root.value) return
  // Use local CSS width so page zoom behaves consistently in Chromium and WKWebView.
  layoutObserver = new ResizeObserver(([entry]) => {
    const width = entry.contentRect.width
    layout.value = width <= 660 ? 'compact' : width <= 1080 ? 'stacked' : 'wide'
  })
  layoutObserver.observe(root.value)
})
onBeforeUnmount(() => layoutObserver?.disconnect())
const id = computed(() => String(route.params.id))
const state = ref<'loading'|'ready'|'notfound'|'error'>('loading'); const err = ref('')
const detail = ref<DramaDetail | null>(null)
const detailHeading = ref<HTMLHeadingElement | null>(null)
const tab = ref<'eps'|'clips'>('eps'); const group = ref(0); const expanded = ref(false)
const settings = useAppSettings()
const clipDialog = ref<HTMLDialogElement | null>(null)
const clipVideo = ref<HTMLVideoElement | null>(null)
const selectedClip = ref<DemoHighlight | null>(null)
const clipError = ref(false)
const clipSources = computed(() => selectedClip.value ? demoClipSources(selectedClip.value.id, settings.s.defaultQuality) : [])

async function openClip(clip: DemoHighlight, event: MouseEvent) {
  // WKWebView can activate a button without focusing it first.
  if (event.currentTarget instanceof HTMLButtonElement) event.currentTarget.focus()
  selectedClip.value = clip
  clipError.value = false
  await nextTick()
  if (selectedClip.value?.id === clip.id) clipDialog.value?.showModal()
}

function closeClip() {
  clipVideo.value?.pause()
  clipDialog.value?.close()
  selectedClip.value = null
}

watch(id, () => { closeClip(); tab.value = 'eps' })
let token = 0
async function load(focusHeading = false) {
  const my = ++token; state.value = 'loading'
  if (route.query.state === 'loading') return
  try {
    const result = await getDetail(id.value, route.query.state === 'error' ? 'error' : 'normal')
    if (my !== token) return
    detail.value = result
    state.value = result ? 'ready' : 'notfound'
    const history = lib.history.find((entry) => entry.dramaId === id.value)
    group.value = history && result && !savedMediaIssue(result.drama, history.episode)
      ? Math.floor((history.episode - 1) / 20) : 0
    expanded.value = false
    if (result && focusHeading) {
      await nextTick()
      if (my === token) detailHeading.value?.focus()
    }
  } catch (error) {
    if (my !== token) return
    err.value = (error as Error).message
    state.value = 'error'
  }
}
watch(id, (_, previous) => { void load(previous !== undefined) }, { immediate: true })
const groups = computed(() => (detail.value ? groupEpisodes(detail.value.drama.episodeCount) : []))
const eps = computed(() => {
  const range = groups.value[group.value]
  return range ? (detail.value?.episodes ?? []).filter(({ index }) => index >= range.from && index <= range.to) : []
})
const hist = computed(() => lib.history.find((h) => h.dramaId === id.value))
const hasEpisode = (episode: number) => detail.value?.episodes.some((item) => item.index === episode && item.available)
const continuation = computed(() => {
  const history = hist.value
  const current = detail.value
  if (!history || !current || !hasEpisode(history.episode)) return
  const next = continuationEpisode(current.drama, history)
  if (next !== undefined && next !== history.episode && hasEpisode(next)) {
    return { episode: next, percent: 0, label: '看下一集' }
  }
  return { episode: history.episode, percent: history.percent, label: history.percent === 100 ? '重播本集' : '继续观看' }
})
const watchEp = (ep: number) => { if (!hasEpisode(ep)) { toast.push(`第${ep}集尚未更新`, 'error'); return } router.push(`/watch/${id.value}/${ep}`) }
const progressOf = (ep: number) => (hist.value && hist.value.episode === ep ? hist.value.percent : ep < (hist.value?.episode ?? 0) ? 100 : 0)
function fav() { void lib.notifySaved(lib.toggleFav(id.value) ? '已加入收藏' : '已取消收藏') }
const share = () => copyShareLink(`/drama/${id.value}`)
const tagIcon = (t: string) => ({ 古装: Landmark, 甜宠: Heart, 重生: Sprout, 权谋: Swords, 逆袭: Crown }[t] ?? Sparkles)
const recommendationSource = computed(() => {
  const current = detail.value?.drama
  return current?.id === id.value ? current : undefined
})
const { cards: guess, canShuffle, shuffle } = useRelatedDramas(recommendationSource, 6)
</script>
<template>
  <div ref="root" class="detail" :class="layout" data-testid="page-detail">
    <button class="back" data-testid="back" @click="router.back()"><ChevronLeft :size="18" />返回</button>
    <div v-if="state === 'loading'" class="sk"><Skeleton style="width: 286px; height: 415px" /><Skeleton style="flex: 1; height: 415px" /></div>
    <ErrorState v-else-if="state === 'error'" :message="err" @retry="load()" />
    <EmptyState v-else-if="state === 'notfound'" title="没有找到这部短剧"><BaseButton variant="outline" @click="router.push('/home')">回到首页</BaseButton></EmptyState>
    <template v-else-if="detail">
      <section class="top">
        <div class="poster"><img :src="detail.drama.cover" :alt="detail.drama.title" /><span class="badge"><Crown :size="13" fill="currentColor" />独播</span><b class="pt">{{ detail.drama.title }}</b></div>
        <div class="meta">
          <h1 ref="detailHeading" tabindex="-1">{{ detail.drama.title }}<span class="heat"><Flame :size="20" color="#ff6a3d" fill="#ff6a3d" />{{ detail.drama.heat.toFixed(1) }}万热度</span></h1>
          <div class="stats"><span class="score"><Star :size="19" color="#f4b740" fill="#f4b740" /><b>{{ detail.rating }}</b> 分</span><i /><span><CirclePlay :size="17" />{{ detail.plays }}</span><i /><span><Heart :size="17" />{{ detail.favs }}</span></div>
          <div class="tags"><span v-for="t in detail.drama.tags" :key="t" class="tg"><component :is="tagIcon(t)" :size="15" />{{ t }}</span></div>
          <div class="sum"><h4>剧情简介</h4><p :class="{ clamp: !expanded }" data-testid="summary">{{ expanded ? detail.long : detail.drama.summary }}</p><button class="exp" data-testid="expand" @click="expanded = !expanded">{{ expanded ? '收起' : '展开' }}<component :is="expanded ? ChevronUp : ChevronDown" :size="14" /></button></div>
          <div v-if="detail.cast.length" class="cast">
            <h4>演示主演</h4>
            <RouterLink v-for="c in detail.cast" :key="c.name" class="cm" :to="{ name: 'search', query: { q: c.name, cast: '1' } }" :aria-label="`搜索 ${c.name} 的作品`">
              <i :style="{ background: `hsl(${c.hue} 40% 45%)` }">{{ c.name[0] }}</i>
              <span><b>{{ c.name }}</b><em>饰 {{ c.role }}</em></span>
              <ChevronRight :size="14" />
            </RouterLink>
          </div>
          <p class="status"><b>更新状态</b> {{ detail.schedule }} <u>|</u> 已更新至 第{{ detail.drama.updatedTo }}集（共{{ detail.drama.episodeCount }}集）</p>
          <div class="cta">
            <BaseButton size="lg" data-testid="watch-now" @click="watchEp(1)"><Play :size="17" fill="currentColor" />立即观看</BaseButton>
            <button v-if="continuation" class="cont" data-testid="continue" @click="watchEp(continuation.episode)"><CirclePlay :size="30" /><span>{{ continuation.label }}<em>第{{ continuation.episode }}集 · {{ continuation.percent }}%</em></span></button>
            <BaseButton size="lg" variant="ghost" data-testid="fav" @click="fav"><component :is="lib.isFav(id) ? Check : Plus" :size="18" />{{ lib.isFav(id) ? '已收藏' : '收藏' }}</BaseButton>
            <BaseButton size="lg" variant="ghost" data-testid="share" @click="share"><Share2 :size="17" />分享</BaseButton>
          </div>
        </div>
      </section>

      <section class="mid">
        <div class="panel eps">
          <header><div class="tabs"><button :class="{ on: tab === 'eps' }" data-testid="tab-eps" @click="tab = 'eps'">选集</button><button :class="{ on: tab === 'clips' }" data-testid="tab-clips" @click="tab = 'clips'">精彩片段</button></div><span class="range">{{ groups[group]?.label }}<ChevronDown :size="14" /></span></header>
          <template v-if="tab === 'eps'">
            <div class="groups"><button v-for="(g, i) in groups" :key="g.label" :class="{ on: i === group }" data-testid="ep-group" @click="group = i">{{ g.label }}</button></div>
            <div class="grid"><button v-for="{ index: ep, available } in eps" :key="ep" class="ep" :class="{ cur: hist?.episode === ep, lock: !available }" data-testid="ep" :aria-label="`第${ep}集`" @click="watchEp(ep)">
              <Play v-if="ep === 1 && !hist" :size="14" fill="currentColor" /><template v-else><Flame v-if="epHot(ep)" :size="13" color="#ff6a3d" fill="#ff6a3d" />{{ ep }}</template>
              <i v-if="progressOf(ep) > 0 && progressOf(ep) < 100" class="pg" :style="{ width: progressOf(ep) + '%' }" /></button></div>
          </template>
          <ul v-else class="clips"><li v-for="c in detail.highlights" :key="c.id"><button data-testid="clip" @click="openClip(c, $event)"><Play :size="16" />{{ c.title }}<em>{{ c.dur }}</em></button></li></ul>
        </div>
        <div v-if="detail.seasons.length > 1" class="panel rel">
          <header><h3>演示系列</h3></header>
          <component v-for="{ number, drama } in detail.seasons" :key="drama.id"
            :is="drama.id === id ? 'div' : RouterLink"
            :to="drama.id === id ? undefined : detailRoute(drama.id)"
            :aria-current="drama.id === id ? 'page' : undefined"
            :class="{ on: drama.id === id }" class="sea" data-testid="season">
            <img :src="drama.cover" alt="" />
            <span><b class="clamp-2">第{{ number }}季 · {{ drama.title }}</b><em>共{{ drama.episodeCount }}集 · {{ drama.finished ? '已完结' : '连载中' }} <Flame :size="12" color="#ff6a3d" fill="#ff6a3d" /> {{ drama.heat.toFixed(1) }}万热度</em></span>
            <i v-if="drama.id === id" class="ck"><Check :size="14" /></i><i v-else class="pl"><ChevronRight :size="16" /></i>
          </component>
        </div>
      </section>

      <section v-if="guess.length" class="guess"><header><h2><Flame :size="24" color="#ff6a3d" fill="#ff6a3d" />猜你喜欢</h2><button data-testid="shuffle" :disabled="!canShuffle" @click="shuffle">换一换<RefreshCw :size="14" /></button></header>
        <div class="row"><button v-for="d in guess" :key="d.id" class="gc on-dark" data-testid="guess-card" @click="router.push(`/drama/${d.id}`)"><img :src="d.cover" :alt="d.title" /><span v-if="d.kind === detail.drama.kind" class="bd">同类型</span><b class="clamp-1">{{ d.title }}</b><em><Flame :size="12" color="#ff6a3d" fill="#ff6a3d" />{{ d.heat.toFixed(1) }}万热度</em></button></div></section>
    </template>
    <dialog ref="clipDialog" class="clip-dialog on-dark" :aria-label="selectedClip?.title" @cancel.prevent="closeClip">
      <h2>{{ selectedClip?.title }}</h2>
      <video v-if="selectedClip && !clipError" ref="clipVideo" controls playsinline preload="metadata" :aria-label="selectedClip.title" @error="clipError = true">
        <source v-for="(source, i) in clipSources" :key="source.type" :src="source.src" :type="source.type" @error="i === clipSources.length - 1 && (clipError = true)" />
      </video>
      <p v-if="clipError" role="alert">演示片段无法播放，请关闭后重试。</p>
      <BaseButton autofocus variant="ghost" @click="closeClip">关闭</BaseButton>
    </dialog>
  </div>
</template>
<style scoped lang="scss">
.detail { padding: 6px 12px 16px 14px; }
.back { display: inline-flex; align-items: center; gap: 2px; margin: 8px 0 8px 6px; font-size: 15px; color: var(--c-text-2); &:hover { color: var(--c-text); } }
.sk { display: flex; gap: 32px; }
.top { display: flex; gap: 34px; padding-left: 16px; }
.poster { position: relative; flex: none; width: 286px; height: 415px; border-radius: 14px; overflow: hidden; border: 1px solid var(--c-border-strong); img { width: 100%; height: 100%; object-fit: cover; } .badge { position: absolute; left: 0; top: 10px; display: inline-flex; gap: 4px; align-items: center; padding: 3px 10px; border-radius: 0 10px 10px 0; background: var(--g-accent-h); font-size: 12px; font-weight: 700; } .pt { position: absolute; left: 18px; right: 18px; bottom: 28px; font-family: var(--font-display); font-weight: 400; font-size: 34px; line-height: 1.15; text-shadow: 0 2px 10px #000; } }
.meta { flex: 1; min-width: 0; h1 { display: flex; align-items: baseline; gap: 16px; font-size: 36px; font-weight: 700; } .heat { display: inline-flex; align-items: center; gap: 4px; font-size: 18px; font-weight: 400; color: var(--c-text-2); } }
.stats { display: flex; align-items: center; margin-top: 6px; font-size: 15px; color: var(--c-text-2); span { display: inline-flex; align-items: center; gap: 6px; } .score { color: var(--c-gold); b { font-size: 22px; } } i { width: 1px; height: 18px; margin: 0 22px; background: var(--c-border-strong); } }
.tags { display: flex; gap: 12px; margin-top: 12px; } .tg { display: inline-flex; align-items: center; gap: 6px; height: 32px; padding: 0 16px; border-radius: 16px; background: var(--c-surface); border: 1px solid var(--c-border); font-size: 14px; }
.sum { position: relative; margin-top: 8px; h4 { font-size: 17px; margin-bottom: 6px; } p { font-size: 14.5px; line-height: 1.6; color: var(--c-text-2); max-width: 760px; padding-right: 60px; &.clamp { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; } } .exp { position: absolute; right: 8px; top: 34px; display: inline-flex; align-items: center; font-size: 13px; } }
.cast { display: flex; white-space: nowrap; align-items: center; gap: 12px; margin-top: 12px; h4 { font-size: 17px; margin-right: 6px; } }
.cm { display: flex; align-items: center; gap: 10px; height: 52px; padding: 0 12px 0 6px; border-radius: 26px; background: var(--c-surface); border: 1px solid var(--c-border); width: 200px; i { width: 40px; height: 40px; border-radius: 50%; display: grid; place-items: center; font-style: normal; font-weight: 700; flex: none; } span { flex: 1; text-align: left; b { display: block; font-size: 14px; } em { font-style: normal; font-size: 12px; color: var(--c-text-2); } } }
.status { margin-top: 14px; font-size: 14.5px; color: var(--c-text-2); b { color: var(--c-text); margin-right: 10px; } u { text-decoration: none; margin: 0 10px; color: var(--c-text-3); } }
.cta { display: flex; gap: 16px; margin-top: 14px; align-items: center; :deep(.btn.lg) { min-width: 186px; } :deep(.btn.ghost) { min-width: 144px; } }
.cont { display: flex; align-items: center; gap: 12px; height: 52px; padding: 0 24px 0 14px; border-radius: 26px; background: var(--c-surface); border: 1px solid var(--c-border-strong); min-width: 200px; text-align: left; span { font-size: 15px; font-weight: 600; em { display: block; font-style: normal; font-size: 12px; font-weight: 400; color: var(--c-text-2); } } &:hover { background: var(--c-surface-hover); } }
.mid { display: flex; gap: 22px; margin-top: 22px; }
.panel { border-radius: 20px; background: linear-gradient(180deg, rgba(255,255,255,.06), rgba(255,255,255,.03)); border: 1px solid var(--c-border); padding: 14px 20px; header { display: flex; justify-content: space-between; align-items: center; } }
.eps { flex: 1; min-width: 0; header { border-bottom: 1px solid var(--c-border); } .tabs button { position: relative; font-size: 18px; font-weight: 600; padding: 4px 22px 12px 4px; margin-right: 6px; color: var(--c-text-2); &.on { color: var(--c-text); &::after { content: ''; position: absolute; left: 0; bottom: -1px; width: 44px; height: 3px; background: var(--c-accent); border-radius: 3px; } } } .range { display: inline-flex; align-items: center; gap: 4px; font-size: 14px; } }
.groups { display: flex; gap: 12px; margin: 12px 0; button { height: 30px; padding: 0 22px; border-radius: 15px; background: var(--c-surface); border: 1px solid var(--c-border); font-size: 14px; &.on { background: var(--g-accent); border-color: transparent; } } }
.grid { display: grid; grid-template-columns: repeat(10, 1fr); gap: 10px 9px; }
.ep { position: relative; height: 46px; display: flex; align-items: center; justify-content: center; gap: 3px; border-radius: 10px; background: rgba(255,255,255,.08); font-size: 17px; overflow: hidden; &:hover { background: rgba(255,255,255,.16); } &.cur { box-shadow: inset 0 0 0 2px var(--c-accent); } &.lock { opacity: .5; } .pg { position: absolute; left: 10px; bottom: 5px; height: 3px; border-radius: 3px; background: var(--c-accent); max-width: calc(100% - 20px); } }
.clips li button { display: flex; align-items: center; gap: 10px; width: 100%; height: 44px; padding: 0 8px; border-bottom: 1px solid var(--c-border); font-size: 15px; em { margin-left: auto; font-style: normal; color: var(--c-text-2); } }
.clip-dialog { margin: auto; padding: 22px;
  width: min(420px, calc(100vw - 32px), calc(100vw / var(--z) - 24px));
  max-height: min(calc(100vh - 32px), calc(100vh / var(--z) - 24px));
  overflow: auto;
  text-align: center; color: var(--c-text); border: 1px solid var(--c-border-strong); border-radius: 18px; background: #261719;
  &::backdrop { background: rgba(0,0,0,.65); }
  h2 { margin-bottom: 14px; font-size: 20px; }
  video { display: block; margin: 0 auto 14px; width: 100%; height: min(58vh, 500px, calc(58vh / var(--z))); background: #0f0b0c; }
  p { margin-bottom: 14px; }
}
.rel { width: 390px; flex: none; header h3 { font-size: 20px; margin-bottom: 8px; } }
.sea { position: relative; display: flex; gap: 12px; align-items: center; width: 100%; padding: 8px 40px 8px 8px; margin-bottom: 8px; border-radius: 12px; border: 1px solid transparent; text-align: left; &.on { background: rgba(238,75,57,.18); border-color: var(--c-accent); } img { width: 86px; height: 62px; object-fit: cover; border-radius: 8px; } b { display: block; font-size: 14.5px; line-height: 1.3; } em { display: block; margin-top: 6px; font-style: normal; font-size: 12px; color: var(--c-text-2); } .ck, .pl { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); width: 28px; height: 28px; border-radius: 50%; display: grid; place-items: center; } .ck { background: rgba(238,75,57,.35); color: #ff9a86; } .pl { background: rgba(255,255,255,.16); } }
.guess { margin-top: 18px; padding-left: 8px; header { display: flex; justify-content: space-between; align-items: center; h2 { display: flex; align-items: center; gap: 6px; font-size: 22px; } button { display: inline-flex; align-items: center; gap: 4px; font-size: 14px; } } .row { display: grid; grid-template-columns: repeat(6, 1fr); gap: 14px; margin-top: 12px; } }
.gc { position: relative; height: 198px; border-radius: 14px; overflow: hidden; text-align: left; padding: 0; img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; } &::after { content: ''; position: absolute; inset: 50% 0 0; background: linear-gradient(transparent, rgba(0,0,0,.85)); } .bd { position: absolute; left: 0; top: 0; z-index: 1; padding: 2px 8px; border-radius: 0 0 8px 0; background: var(--g-accent-h); font-size: 11px; } b, em { position: absolute; left: 12px; right: 8px; z-index: 1; } b { bottom: 30px; font-size: 16px; } em { bottom: 10px; font-style: normal; font-size: 12px; color: var(--c-text-2); display: flex; align-items: center; gap: 3px; } }

.detail:is(.stacked, .compact) {
  .top { gap: 22px; padding-left: 0; }
  .poster {
    width: min(210px, 100%);
    height: auto;
    aspect-ratio: 286 / 415;
    align-self: start;
    .pt { font-size: 26px; overflow-wrap: anywhere; }
  }
  .meta h1 { flex-wrap: wrap; gap: 6px 16px; overflow-wrap: anywhere; }
  .heat { white-space: nowrap; }
  .tags, .cast, .cta, .groups { flex-wrap: wrap; }
  .mid { flex-direction: column; }
  .rel { width: auto; }
  .guess .row { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}
.detail.compact {
  .top { flex-direction: column; gap: 14px; }
  .poster { width: min(145px, 100%); .pt { font-size: 21px; bottom: 14px; } }
  .stats { flex-wrap: wrap; gap: 8px 16px; i { display: none; } }
  .cta { gap: 10px; :deep(.btn.lg) { min-width: 142px; } }
  .cast h4 { flex-basis: 100%; }
  .grid { grid-template-columns: repeat(5, minmax(0, 1fr)); }
  .guess .row { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
</style>
