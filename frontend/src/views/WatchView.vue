<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { Star, Play, Plus, Check, Download, ThumbsUp, Share2, Flame, ChevronDown, RefreshCw } from 'lucide-vue-next'
import { exportDemoEpisode, loadEpisodeLike, loadLocalDanmaku, recoverLocalDanmaku, saveEpisodeLike, sendLocalDanmaku, type LocalDanmaku } from '@/bindings'
import { byId, DRAMAS } from '@/mocks/data'
import { epHot, groupEpisodes } from '@/mocks/provider'
import { useRelatedDramas } from '@/mocks/recommendations'
import { usePlayer, resolveMediaMode } from '@/player/usePlayer'
import { demoQualityId, demoSources } from '@/player/demoMedia'
import { useAppSettings } from '@/stores/appSettings'
import { observeContentWidth } from '@/layoutWidth'
import { playbackEndAction } from '@/player/preferences'
import { episodeDuration, fmt } from '@/player/machine'
import { normalizeDanmakuText } from '@/bindings/danmakuRecovery'
import { draftForDanmaku, type DanmakuSubmission } from '@/player/danmakuDraft'
import { genDanmaku, type DanmakuItem } from '@/player/danmaku'
import { useLibraryStore } from '@/stores/library'
import { useToastStore } from '@/stores/toast'
import { copyShareLink, SHARE_LINK_PAUSE } from '@/router/shareLinks'
import { useCommentStore } from '@/stores/comments'
import { useSettingsStore } from '@/stores/player'
import PlayerSurface from '@/components/player/PlayerSurface.vue'
import CommentPanel from '@/components/player/CommentPanel.vue'
import DanmakuSettings from '@/components/player/DanmakuSettings.vue'
import { posterSvg } from '@/mocks/posters'
import { BaseButton, EmptyState } from '@/components/ui'

const route = useRoute(); const router = useRouter(); const lib = useLibraryStore(); const toast = useToastStore(); const cs = useCommentStore(); const settings = useSettingsStore()
const dramaId = computed(() => String(route.params.dramaId)); const episode = computed(() => Math.max(1, Number(route.params.episodeId) || 1))
const drama = computed(() => byId(dramaId.value))
const panel = computed(() => String(route.query.panel ?? ''))
const layout = computed(() => panel.value === 'comments' ? 'cmt' : panel.value === 'danmaku' ? 'dmk' : 'play')
const pageTestId = computed(() => layout.value === 'cmt' ? 'page-comments' : layout.value === 'dmk' ? 'page-danmaku' : 'page-watch')
const surfaceVariant = computed(() => layout.value === 'cmt' ? 'wide' : layout.value === 'dmk' ? 'danmaku' : 'tall')
const watchRoot = ref<HTMLElement>()
const stage = ref<HTMLElement>()
const companion = ref<HTMLElement>()
const stacked = ref(false)
watch(watchRoot, (element, _, onCleanup) => {
  if (!element) return
  const observer = observeContentWidth(element, (width) => { stacked.value = width <= 980 })
  onCleanup(() => observer.disconnect())
}, { flush: 'post' })
const mine = ref<DanmakuItem[]>([])
const appPreferences = useAppSettings().s
const mode = resolveMediaMode(appPreferences.mediaSource)
const startOverride = () => typeof route.query.t === 'string' ? route.query.t : undefined
function startPoint(id: string, index: number) {
  const parameter = startOverride()
  const explicit = Number(parameter)
  if (parameter?.trim() && Number.isFinite(explicit) && explicit >= 0) return { sec: explicit }
  const bookmark = lib.history.find((item) => item.dramaId === id && item.episode === index && item.percent < 100)
  if (!bookmark) return {}
  return { sec: bookmark.positionSec && bookmark.positionSec > 0 ? bookmark.positionSec : undefined, frac: bookmark.percent / 100 }
}
const player = usePlayer(dramaId, episode, { mode, startPoint, startOverride, autoplay: () => route.query.paused === undefined, onFinish: finishEpisode, failEpisode: (ep) => (route.query.state === 'error' ? '播放源暂时不可用（模拟故障）' : drama.value && ep > drama.value.updatedTo ? `第${ep}集尚未更新` : null) })
watch(() => route.query.paused, (paused) => { if (paused !== undefined) player.pause() })
onMounted(() => {
  window.addEventListener(SHARE_LINK_PAUSE, player.pause)
  player.loadCurrent()
})
onBeforeUnmount(() => window.removeEventListener(SHARE_LINK_PAUSE, player.pause))
const items = computed<DanmakuItem[]>(() => [...genDanmaku(episode.value, player.duration.value || episodeDuration(episode.value)), ...mine.value])
const toItem = (item: LocalDanmaku): DanmakuItem => ({
  id: `local-${item.id}`, at: item.offsetMs / 1000, text: item.text, likes: 0, lane: item.id % 12, mine: true,
})
function mergeMine(items: DanmakuItem[]) {
  mine.value = [...new Map([...mine.value, ...items].map((item) => [item.id, item])).values()]
}
const recoveringDanmaku = ref(false)
const pendingDanmakuRecovery = ref(false)
let danmakuEpisodeGeneration = 0
async function retryDanmakuRecovery() {
  if (recoveringDanmaku.value) return
  const generation = danmakuEpisodeGeneration
  const current = () => generation === danmakuEpisodeGeneration
  recoveringDanmaku.value = true
  try {
    const pending = await recoverLocalDanmaku(dramaId.value, episode.value, current, (item, entry) => {
      if (!current()) return
      mergeMine([toItem(item)])
      const draft = danmakuDraft.value
      if (!draft.sending && draft.attempt?.requestId === entry.requestId) {
        if (normalizeDanmakuText(draft.text) === entry.text) draft.text = ''
        draft.attempt = undefined
      }
    })
    if (current()) pendingDanmakuRecovery.value = pending
  } finally {
    if (current()) recoveringDanmaku.value = false
  }
}
watch([dramaId, episode], async ([id, ep], _, onCleanup) => {
  danmakuEpisodeGeneration++
  onCleanup(() => { danmakuEpisodeGeneration++ })
  const generation = danmakuEpisodeGeneration
  mine.value = []
  recoveringDanmaku.value = false
  pendingDanmakuRecovery.value = false
  if (!byId(id)) return
  cs.open(id, ep)
  void retryDanmakuRecovery()
  try {
    const saved = await loadLocalDanmaku(id, ep)
    if (generation === danmakuEpisodeGeneration && saved) mergeMine(saved.map(toItem))
  } catch {
    if (generation === danmakuEpisodeGeneration) toast.push('本地弹幕读取失败', 'error')
  }
}, { immediate: true })
const total = computed(() => drama.value?.episodeCount ?? 0)
const hasNext = computed(() => episode.value < (drama.value?.updatedTo ?? 0))
function goEp(e: number) {
  const query = { ...route.query, t: undefined, paused: undefined }
  return router.replace({ path: `/watch/${dramaId.value}/${e}`, query })
}
let departing = false
onBeforeRouteLeave(() => { departing = true })
function finishEpisode() {
  if (departing) return
  const action = playbackEndAction(appPreferences.playMode, hasNext.value)
  if (action === 'loop') player.play()
  else if (action === 'next') void goEp(episode.value + 1)
}
const next = () => { if (hasNext.value) goEp(episode.value + 1); else toast.push('已经是最新一集了') }
const prev = () => { if (episode.value > 1) goEp(episode.value - 1) }
async function focusStackedPanel(targetPanel: string, showPlayer = targetPanel === '') {
  if (!stacked.value) return
  await nextTick()
  if (panel.value !== targetPanel) return
  const surface = stage.value?.querySelector<HTMLElement>('[data-testid="player"]')
  const panelControl = targetPanel === 'comments'
    ? companion.value?.querySelector<HTMLElement>('[role="complementary"]')
    : companion.value
  const target = showPlayer ? surface : panelControl
  target?.scrollIntoView({ block: 'start' })
  target?.focus({ preventScroll: true })
}
async function chooseEpisode(index: number) {
  await goEp(index)
  if (episode.value === index) await focusStackedPanel(panel.value, true)
}
async function setPanel(targetPanel: '' | 'comments' | 'danmaku') {
  await router.replace({ query: { ...route.query, panel: targetPanel || undefined } })
  await focusStackedPanel(targetPanel)
}
const back = () => (panel.value ? setPanel('') : router.push(`/drama/${dramaId.value}`))
const canContinue = computed(() => ['playing', 'paused', 'ended'].includes(player.state.value))
async function continuePlayback() {
  if (!canContinue.value) return
  if (player.state.value !== 'playing') player.play()
  await router.replace({ query: { ...route.query, panel: undefined, paused: undefined } })
  await focusStackedPanel('')
}
const danmakuDraft = computed(() => draftForDanmaku(dramaId.value, episode.value))
async function sendDm(submission: DanmakuSubmission) {
  const id = dramaId.value
  const ep = episode.value
  let saved: LocalDanmaku | undefined
  try {
    saved = await sendLocalDanmaku(id, ep, submission)
  } catch (error) {
    if (dramaId.value === id && episode.value === ep) pendingDanmakuRecovery.value = true
    throw error
  }
  if (dramaId.value !== id || episode.value !== ep) return
  mergeMine([saved ? toItem(saved) : {
    id: `demo-${submission.requestId}`, at: submission.offsetMs / 1000, text: submission.text,
    likes: 0, lane: Math.floor(Math.random() * 12), mine: true,
  }])
  if (pendingDanmakuRecovery.value) void retryDanmakuRecovery()
}
const rtab = ref<'eps' | 'cm'>('eps'); const desc = ref(false); const grp = ref(Math.floor((episode.value - 1) / 20)); const exp = ref(false)
watch(episode, (index) => { grp.value = Math.floor((index - 1) / 20) })
const groups = computed(() => groupEpisodes(total.value))
const rows = computed(() => { const g = groups.value[grp.value]; if (!g) return []; const l = Array.from({ length: g.to - g.from + 1 }, (_, i) => g.from + i); return desc.value ? l.reverse() : l })
const thumb = (ep: number) => posterSvg({ hue: 350, hue2: 20, seed: 1 + (ep % 3) })
const { cards: related, canShuffle: canShuffleRelated, shuffle: shuffleRelated } = useRelatedDramas(drama, 2)
const guess = computed(() => DRAMAS.filter((d) => d.id !== dramaId.value && d.isNewToday).slice(0, 4))
function fav() { void lib.notifySaved(lib.toggleFav(dramaId.value) ? '已加入收藏' : '已取消收藏') }
const exporting = ref(false)
const shareEpisode = () => copyShareLink(`/watch/${dramaId.value}/${episode.value}`)
async function downloadEpisode() {
  if (exporting.value || !drama.value) return
  const id = dramaId.value
  const ep = episode.value
  if (ep > drama.value.updatedTo) {
    toast.push(`第${ep}集尚未更新`, 'error')
    return
  }
  exporting.value = true
  try {
    const choice = appPreferences.dlQuality
    const quality = demoQualityId(choice)
    const request = exportDemoEpisode(id, ep, quality)
    if (request) {
      if (await request) toast.push('演示视频已保存', 'success')
      return
    }
    const mp4 = demoSources(ep, choice).find(({ type }) => type === 'video/mp4')
    if (!mp4) return
    const link = document.createElement('a')
    link.href = mp4.src
    link.download = `JindiDrama-demo-${id}-${ep}${quality === 'sd' ? '-sd' : ''}.mp4`
    link.click()
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    toast.push(message || '演示视频保存失败', 'error')
  } finally {
    exporting.value = false
  }
}
const liked = ref(false)
const likeStatus = ref<'loading' | 'ready' | 'saving' | 'error'>('loading')
const likeBusy = computed(() => likeStatus.value === 'loading' || likeStatus.value === 'saving')
const likePressed = computed(() => likeStatus.value === 'ready' || likeStatus.value === 'saving' ? liked.value : undefined)
const likeLabel = computed(() => likeStatus.value === 'error' ? '重试读取本地点赞' : '本地点赞')
const episodeCanBeLiked = computed(() =>
  Number.isSafeInteger(episode.value) && String(episode.value) === route.params.episodeId &&
  !!drama.value && episode.value <= drama.value.updatedTo,
)
let likeGeneration = 0

async function refreshEpisodeLike(generation: number) {
  likeStatus.value = 'loading'
  try {
    const saved = await loadEpisodeLike(dramaId.value, episode.value)
    if (generation !== likeGeneration) return
    liked.value = saved
    likeStatus.value = 'ready'
  } catch {
    if (generation !== likeGeneration) return
    likeStatus.value = 'error'
    toast.push('无法读取本地点赞', 'error')
  }
}

watch([layout, dramaId, episode, episodeCanBeLiked], () => {
  const generation = ++likeGeneration
  liked.value = false
  likeStatus.value = 'loading'
  if (layout.value === 'cmt' && episodeCanBeLiked.value) void refreshEpisodeLike(generation)
}, { immediate: true })
onBeforeUnmount(() => { likeGeneration++ })

async function toggleEpisodeLike() {
  const generation = likeGeneration
  if (!episodeCanBeLiked.value) return
  if (likeStatus.value === 'error') return refreshEpisodeLike(generation)
  if (likeStatus.value !== 'ready') return
  const nextLiked = !liked.value
  likeStatus.value = 'saving'
  try {
    await saveEpisodeLike(dramaId.value, episode.value, nextLiked)
    if (generation === likeGeneration) liked.value = nextLiked
  } catch {
    if (generation === likeGeneration) toast.push('无法保存本地点赞', 'error')
  } finally {
    if (generation === likeGeneration) likeStatus.value = 'ready'
  }
}
</script>
<template>
  <div v-if="!drama" class="nf"><EmptyState title="没有找到这部短剧"><BaseButton variant="outline" @click="router.push('/home')">回到首页</BaseButton></EmptyState></div>

  <div v-else ref="watchRoot" class="w" :class="[layout, { stacked }]" :data-testid="pageTestId">
    <header v-if="layout === 'dmk'" class="ph">
      <button class="rb" aria-label="返回播放" data-testid="danmaku-back" @click="setPanel('')">‹</button>
      <h1>弹幕设置</h1><p>自定义你的弹幕体验，让追剧更有趣</p>
    </header>
    <div class="watch-body">
      <section class="watch-main">
        <div ref="stage" class="stage">
          <PlayerSurface :player="player" :drama="drama" :items="items" :variant="surfaceVariant" :episode="episode"
            :exporting="exporting" :has-next="hasNext" @download="downloadEpisode" @next="next" @prev="prev"
            @back="back" @comments="setPanel('comments')" @danmaku-panel="setPanel('danmaku')" />
        </div>
        <template v-if="layout === 'cmt'">
          <div class="inf"><h2>{{ drama.title }}<span class="st">{{ drama.finished ? '已完结' : '连载中' }}</span></h2>
            <p class="sub">第{{ episode }}集 / 共{{ total }}集 · {{ drama.tags.slice(0, 3).join(' · ') }} · {{ drama.heat.toFixed(1) }}万播放</p>
            <div class="acts"><BaseButton data-testid="continue" :disabled="!canContinue" @click="continuePlayback"><Play :size="15" fill="currentColor" />继续播放</BaseButton><BaseButton variant="ghost" @click="fav"><component :is="lib.isFav(dramaId) ? Check : Plus" :size="16" />{{ lib.isFav(dramaId) ? '已收藏' : '收藏' }}</BaseButton>
              <BaseButton variant="ghost" data-testid="video-like" :aria-label="likeLabel" :aria-pressed="likePressed" :aria-busy="episodeCanBeLiked && likeBusy"
                :title="liked ? '取消本地点赞' : '仅保存在本机'" :disabled="!episodeCanBeLiked || likeBusy" @click="toggleEpisodeLike">
                <ThumbsUp :size="16" :fill="liked ? 'currentColor' : 'none'" />{{ likeStatus === 'error' ? '重试' : liked ? '已点赞' : '本地点赞' }}
              </BaseButton><BaseButton variant="ghost" @click="shareEpisode"><Share2 :size="16" />分享</BaseButton><BaseButton variant="ghost" :disabled="exporting" @click="downloadEpisode"><Download :size="16" />下载</BaseButton></div>
            <p class="intro"><b>简介</b><span :class="{ c1: !exp }">{{ drama.summary }}</span><button @click="exp = !exp">{{ exp ? '收起' : '展开' }}<ChevronDown :size="13" /></button></p></div>
          <div class="sel"><h3>选集</h3><div class="gs"><button v-for="(g, i) in groups" :key="g.label" :class="{ on: i === grp }" @click="grp = i">{{ g.label.replace('集', '') }}</button></div>
            <div class="eg"><button class="ep0" aria-label="首集" @click="chooseEpisode(1)"><Play :size="14" fill="currentColor" /></button><button v-for="e in rows.slice(0, 11)" :key="e" class="ep" :class="{ cur: e === episode, vip: e > 25 && e < 32 }" data-testid="ep" @click="chooseEpisode(e)">{{ e }}</button></div></div>
          <div class="rel"><header><h3>相关推荐</h3><button @click="router.push('/home')">查看更多 ›</button></header><div class="rg"><button v-for="d in guess" :key="d.id" @click="router.push(`/drama/${d.id}`)"><img :src="d.cover" :alt="d.title" /><span class="bd">今日上新</span><b class="clamp-1">{{ d.title }}</b><em>{{ d.tags.slice(0, 2).join(' · ') }} · {{ d.episodeCount }}集</em></button></div></div>
        </template>
      </section>
      <section v-if="layout === 'dmk'" ref="companion" class="rail" tabindex="-1">
        <DanmakuSettings :key="`${dramaId}:${episode}`" :time="player.time.value" :draft="danmakuDraft" :local-items="mine" :send-danmaku="sendDm" :pending-recovery="pendingDanmakuRecovery" :recovering="recoveringDanmaku" :retry-recovery="retryDanmakuRecovery" />
      </section>
      <section v-else-if="layout === 'cmt'" ref="companion" class="drawer" tabindex="-1">
        <CommentPanel :key="`${dramaId}-${episode}`" @close="setPanel('')" />
      </section>
      <aside v-else class="rail">
        <div class="card info"><div class="irow"><img :src="drama.cover" :alt="drama.title" /><div><h2 class="clamp-1">{{ drama.title }}</h2><div class="tg"><span v-for="t in drama.tags.slice(0, 3)" :key="t">{{ t }}</span></div><p>共{{ total }}集 · 已更新至{{ drama.updatedTo }}集</p><p class="ht"><Flame :size="14" color="#ff6a3d" fill="#ff6a3d" />{{ drama.heat.toFixed(1) }}万热度</p><p class="sm" :class="{ c2: !exp }">{{ drama.summary }}<button @click="exp = !exp">{{ exp ? '收起' : '展开' }}</button></p></div></div>
          <div class="ibt"><button class="gh" data-testid="fav" @click="fav"><Star :size="17" :fill="lib.isFav(dramaId) ? 'currentColor' : 'none'" />{{ lib.isFav(dramaId) ? '已收藏' : '收藏' }}</button><button class="pr" :disabled="exporting" @click="downloadEpisode"><Download :size="17" />下载</button><button class="gh share" aria-label="分享" title="分享" @click="shareEpisode"><Share2 :size="17" /></button></div></div>
        <div class="card eps"><header><div class="tb"><button :class="{ on: rtab === 'eps' }" data-testid="rtab-eps" @click="rtab = 'eps'">选集</button><button data-testid="rtab-comments" @click="setPanel('comments')">评论 <small>2.8万</small></button></div><button class="ord" data-testid="order" @click="desc = !desc">{{ desc ? '倒序' : '正序' }}<ChevronDown :size="13" /></button></header>
          <div class="gp"><button v-for="(g, i) in groups" :key="g.label" :class="{ on: i === grp }" data-testid="ep-group" @click="grp = i">{{ g.label.replace('集', '') }}</button></div>
          <div class="tg4"><button v-for="e in rows" :key="e" class="th on-dark" :class="{ cur: e === episode }" data-testid="ep" :aria-label="`第${e}集`" @click="chooseEpisode(e)"><img :src="thumb(e)" alt="" /><span><b>第{{ e }}集</b><em>{{ fmt(episodeDuration(e)) }}</em></span><Play v-if="e === episode" :size="11" fill="currentColor" class="np" /><Flame v-else-if="epHot(e)" :size="11" color="#ff6a3d" fill="#ff6a3d" class="np" /></button></div></div>
        <div v-if="related.length" class="card rec"><header><h3>相关推荐</h3><button :disabled="!canShuffleRelated" @click="shuffleRelated">换一换<RefreshCw :size="12" /></button></header><button v-for="d in related" :key="d.id" class="ri" @click="router.push(`/drama/${d.id}`)"><img :src="d.cover" :alt="d.title" /><span><b>{{ d.title }}</b><em>{{ d.tags.slice(0, 2).join(' ') }} {{ d.episodeCount }}集</em><em class="h"><Flame :size="12" color="#ff6a3d" fill="#ff6a3d" />{{ d.heat.toFixed(1) }}万热度</em></span><i><Play :size="13" fill="currentColor" /></i></button></div>
      </aside>
    </div>
  </div>
</template>
<style scoped lang="scss">
.w { height: 100%; } .nf { padding: 40px; }
.watch-body { display: flex; min-height: 0; height: 100%; }
.watch-main { flex: 1; min-width: 0; }
.play { padding-top: 4px; .watch-body { gap: 14px; } .stage { height: calc(100% - 4px); } .rail { width: 404px; flex: none; display: flex; flex-direction: column; gap: 10px; overflow-y: auto; height: calc(100% - 4px); } }
.card { border-radius: 18px; background: linear-gradient(180deg, rgba(255,255,255,.06), rgba(255,255,255,.03)); border: 1px solid var(--c-border); padding: 14px 16px; }
.info .irow { display: flex; gap: 14px; img { width: 100px; height: 150px; object-fit: cover; border-radius: 10px; } h2 { font-size: 20px; } .tg { display: flex; gap: 6px; margin: 6px 0; span { padding: 1px 9px; border-radius: 6px; background: rgba(255,255,255,.1); font-size: 12px; color: var(--c-text-2); } } p { font-size: 13px; color: var(--c-text-2); margin-top: 3px; } .ht { display: flex; gap: 4px; align-items: center; color: var(--c-text); } .sm.c2 { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; } .sm button { margin-left: 6px; color: var(--c-text); } }
.ibt { display: flex; gap: 12px; margin-top: 12px; button { flex: 1; height: 44px; border-radius: 22px; display: inline-flex; align-items: center; justify-content: center; gap: 8px; font-size: 16px; font-weight: 600; } .gh { background: rgba(255,255,255,.1); border: 1px solid var(--c-border-strong); } .pr { background: linear-gradient(180deg, #ff7a5c, #ee4b39); } }
.ibt .share { flex: 0 0 44px; }
.eps { flex: 1; min-height: 300px; display: flex; flex-direction: column; header { display: flex; justify-content: space-between; .tb button { font-size: 18px; font-weight: 600; margin-right: 22px; padding-bottom: 8px; color: var(--c-text-2); position: relative; small { font-size: 13px; font-weight: 400; } &.on { color: var(--c-text); &::after { content: ''; position: absolute; left: 0; bottom: 0; width: 42px; height: 3px; background: var(--c-accent); border-radius: 3px; } } } .ord { display: inline-flex; align-items: center; font-size: 13px; color: var(--c-text-2); } } }
.gp { display: flex; gap: 12px; margin: 8px 0; button { flex: 1; height: 34px; border-radius: 17px; background: rgba(255,255,255,.08); font-size: 14px; &.on { background: var(--g-accent); } } }
.tg4 { flex: 1; min-height: 0; overflow-y: auto; display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; align-content: start; }
.th { position: relative; height: 106px; border-radius: 10px; overflow: hidden; border: 1.5px solid transparent; img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; } &::after { content: ''; position: absolute; inset: 40% 0 0; background: linear-gradient(transparent, rgba(0,0,0,.8)); } span { position: absolute; left: 8px; bottom: 6px; z-index: 1; text-align: left; b { display: block; font-size: 12px; } em { font-style: normal; font-size: 12px; color: var(--c-text-2); } } .np { position: absolute; z-index: 1; right: 8px; bottom: 8px; } &.cur { border-color: var(--c-accent); } }
.rec { header { display: flex; justify-content: space-between; align-items: center; h3 { font-size: 20px; } button { display: inline-flex; gap: 4px; align-items: center; font-size: 13px; color: var(--c-text-2); } } .ri { display: flex; gap: 12px; align-items: center; width: 100%; padding: 6px 0; text-align: left; img { width: 96px; height: 66px; object-fit: cover; border-radius: 8px; } span { flex: 1; b { display: block; font-size: 16px; } em { display: block; font-style: normal; font-size: 12.5px; color: var(--c-text-2); margin-top: 3px; &.h { display: flex; gap: 3px; align-items: center; } } } i { width: 30px; height: 30px; border-radius: 50%; background: rgba(255,255,255,.9); color: #2a1517; display: grid; place-items: center; } } }

.cmt { padding-top: 2px; .watch-body { gap: 12px; } .watch-main { overflow-y: auto; padding-right: 2px; } .drawer { width: 490px; flex: none; height: calc(100% - 2px); } .stage { height: 354px; } }
.inf { padding: 10px 6px 0; h2 { font-size: 24px; display: flex; align-items: center; gap: 12px; } .st { font-size: 12px; font-weight: 400; padding: 2px 10px; border-radius: 8px; background: rgba(255,255,255,.1); color: var(--c-text-2); } .sub { font-size: 13px; color: var(--c-text-2); margin-top: 8px; } .acts { display: flex; gap: 12px; margin-top: 10px; } .intro { display: flex; gap: 14px; margin-top: 14px; font-size: 14px; b { font-size: 16px; } span { color: var(--c-text-2); flex: 1; &.c1 { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; max-width: 560px; } } button { display: inline-flex; align-items: center; font-size: 13px; } } }
.sel { padding: 6px 6px 0; margin-top: 14px; border-top: 1px solid var(--c-border); h3 { font-size: 18px; margin: 12px 0 8px; } .gs { display: flex; gap: 14px; margin-bottom: 10px; button { height: 30px; padding: 0 16px; border-radius: 15px; color: var(--c-text-2); &.on { background: var(--g-accent); color: #fff; } } } .eg { display: flex; gap: 8px; button { width: 50px; height: 44px; border-radius: 10px; background: rgba(255,255,255,.1); font-size: 16px; position: relative; &.cur { box-shadow: inset 0 0 0 2px var(--c-accent); } &.vip::before { content: 'VIP'; position: absolute; right: -2px; top: -8px; font-size: 9px; padding: 0 4px; border-radius: 4px; background: linear-gradient(#f9d27a, #c88a2a); color: #3a2205; } } .ep0 { display: grid; place-items: center; width: 56px; } } }
.rel { padding: 10px 6px 12px; margin-top: 14px; border-top: 1px solid var(--c-border); header { display: flex; justify-content: space-between; align-items: center; h3 { font-size: 18px; } button { font-size: 13px; color: var(--c-text-2); } } .rg { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-top: 10px; button { position: relative; height: 215px; border-radius: 12px; overflow: hidden; text-align: left; img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; } &::after { content: ''; position: absolute; inset: 50% 0 0; background: linear-gradient(transparent, rgba(0,0,0,.85)); } .bd { position: absolute; left: 0; top: 0; z-index: 1; padding: 2px 8px; border-radius: 0 0 8px 0; background: var(--g-accent-h); font-size: 11px; } b, em { position: absolute; z-index: 1; left: 10px; right: 8px; } b { bottom: 26px; font-size: 15px; } em { bottom: 8px; font-style: normal; font-size: 12px; color: var(--c-text-2); } } } }

.dmk { display: flex; flex-direction: column; .ph { display: flex; align-items: center; gap: 16px; padding: 24px 0 18px 30px; height: 90px; .rb { width: 42px; height: 42px; border-radius: 50%; border: 1px solid var(--c-border-strong); background: var(--c-surface); font-size: 26px; line-height: 1; } h1 { font-size: 30px; } p { font-size: 14px; color: var(--c-text-2); margin-top: 6px; } } .watch-body { flex: 1; gap: 14px; padding-bottom: 6px; } .stage { height: 100%; } .rail { width: 478px; flex: none; height: 100%; } }

.w.stacked {
  height: auto;
  min-height: 100%;
  .watch-body { height: auto; flex-direction: column; }
  .watch-main { flex: none; overflow: visible; padding-right: 0; }
  .stage { height: clamp(360px, calc(66vh / var(--z)), 540px); }
  .rail, .drawer { width: 100%; }
  .acts, .gs, .eg, .gp, .tb, .tg { flex-wrap: wrap; }
  .tg4 { grid-template-columns: repeat(auto-fill, minmax(min(96px, 100%), 1fr)); }
}
.w.play.stacked .rail { height: auto; overflow: visible; }
.w.cmt.stacked .drawer { height: clamp(460px, calc(75vh / var(--z)), 650px); }
.w.dmk.stacked {
  .ph { height: auto; flex-wrap: wrap; padding: 14px 12px; }
  .rail { height: auto; }
}
</style>
