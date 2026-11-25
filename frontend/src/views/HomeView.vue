<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronRight, Flame, Crown, Heart, Swords, Ghost, Zap, Sprout, Infinity as InfinityIcon, Building2, Medal, Clock, Play } from 'lucide-vue-next'
import { useCatalogStore } from '@/stores/catalog'
import { useLibraryStore } from '@/stores/library'
import { useLicensedCatalog } from '@/stores/licensedCatalog'
import { detailRoute, playbackRoute } from '@/router/libraryRoutes'
import HeroCarousel from '@/components/HeroCarousel.vue'
import NewDramaCard from '@/components/NewDramaCard.vue'
import { ProgressBar, EmptyState, ErrorState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'

const route = useRoute(); const router = useRouter()
const catalog = useCatalogStore(); const lib = useLibraryStore()
const icons: Record<string, unknown> = { Flame, Crown, Heart, Swords, Ghost, Zap, Sprout, Infinity: InfinityIcon, Building2 }
const tab = computed(() => route.query.rank === 'rise' || route.query.rank === 'new' ? route.query.rank : 'hot')
const selectRank = (rank: 'hot' | 'rise' | 'new') =>
  router.replace({ query: { ...route.query, rank: rank === 'hot' ? undefined : rank } })
const tabs = [{ k: 'hot', l: '热门榜' }, { k: 'rise', l: '飙升榜' }, { k: 'new', l: '新剧榜' }] as const
const mode = () => (['empty', 'error', 'loading'].includes(String(route.query.state)) ? (route.query.state as 'empty'|'error'|'loading') : 'normal')
const load = () => catalog.loadHome(mode())
onMounted(load); watch(() => route.query.state, load)
const home = computed(() => catalog.home)
const { card } = useLicensedCatalog()
const ranking = computed(() => home.value?.rankings[tab.value].slice(0, 5) ?? [])
const resume = computed(() => lib.history.flatMap((h) => { const d = card(h.dramaId); return d && h.episode <= d.updatedTo ? [{ h, d }] : [] }).slice(0, 3))
const openDrama = (id: string) => router.push(detailRoute(id))
const watchNow = (id: string) => { const h = lib.history.find((x) => x.dramaId === id); router.push(playbackRoute(id, h?.episode ?? 1)) }
function fav(id: string) { const on = lib.toggleFav(id); void lib.notifySaved(on ? '已加入收藏' : '已取消收藏') }
const rankClass = (i: number) => (i < 3 ? `r${i + 1}` : 'rn')
const autoplay = computed(() => route.query.static === undefined)
</script>
<template>
  <div class="home" data-testid="page-home">
    <template v-if="catalog.status === 'loading'">
      <Skeleton style="height: 294px; border-radius: 24px; margin-top: 11px" />
      <div class="sk-row"><Skeleton v-for="i in 6" :key="i" style="height: 230px" /></div>
    </template>
    <ErrorState v-else-if="catalog.status === 'error'" :message="catalog.error" @retry="load" />
    <EmptyState v-else-if="catalog.status === 'empty'" title="暂无推荐内容" hint="稍后再来看看吧" />
    <template v-else-if="home">
      <HeroCarousel :banners="home.banners" :autoplay="autoplay" :is-fav="lib.isFav" @watch="watchNow" @fav="fav" />

      <section id="categories" class="cats" aria-label="题材精选">
        <h3 class="h-label">题材精选</h3>
        <button v-for="c in home.categories" :key="c.key" class="chip" :data-testid="`cat-${c.key}`" @click="router.push({ name: 'search', query: { q: c.label } })">
          <component :is="icons[c.icon]" :size="18" :color="c.color" :fill="['Flame','Heart','Crown','Zap','Ghost'].includes(c.icon) ? c.color : 'none'" />{{ c.label }}
        </button>
        <button class="more" data-testid="more-topics" @click="router.push({ name: 'search', query: { browse: 'topics' } })">更多<ChevronRight :size="14" /></button>
      </section>

      <section class="block" aria-label="今日上新">
        <div class="sec-h"><h2>今日上新</h2><span class="muted">共 {{ home.todayNew.length }} 部</span><span class="new">NEW</span><button class="link" data-testid="all-today-new" @click="router.push({ name: 'search', query: { feed: 'today' } })">查看更多<ChevronRight :size="15" /></button></div>
        <div class="grid6"><NewDramaCard v-for="d in home.todayNew.slice(0, 6)" :key="d.id" :drama="d" badge="今日上新" @open="openDrama" /></div>
      </section>

      <section id="rank" class="block rank" aria-label="排行榜">
        <div class="sec-h"><Medal :size="26" color="#f4b740" /><h2>排行榜</h2>
          <div class="seg" role="tablist"><button v-for="t in tabs" :key="t.k" role="tab" :aria-selected="tab === t.k" :class="{ on: tab === t.k }" :data-testid="`rank-${t.k}`" @click="selectRank(t.k)">{{ t.l }}</button></div>
          <button class="link" data-testid="full-ranking" @click="router.push({ name: 'search', query: { feed: tab } })">完整榜单<ChevronRight :size="15" /></button></div>
        <ol class="rank-list">
          <li v-for="(d, i) in ranking" :key="d.id"><button class="ri" data-testid="rank-item" @click="openDrama(d.id)">
            <span class="cov"><img :src="d.cover" :alt="d.title" /><b class="no" :class="rankClass(i)">{{ i + 1 }}</b></span>
            <span class="rt"><strong class="clamp-2">{{ d.title }}</strong><em><Flame :size="14" color="#ff6a3d" fill="#ff6a3d" />{{ d.heat.toFixed(1) }}万热度</em></span>
          </button></li>
        </ol>
      </section>

      <section class="block" aria-label="继续观看">
        <div class="sec-h"><Clock :size="26" /><h2>继续观看</h2><button class="link" @click="router.push('/history')">查看更多<ChevronRight :size="15" /></button></div>
        <EmptyState v-if="!resume.length" title="还没有观看记录" hint="去推荐页挑一部开始追剧吧" />
        <div v-else class="resume"><button v-for="r in resume" :key="r.d.id" class="rc" data-testid="resume-card" @click="watchNow(r.d.id)">
          <img :src="r.d.cover" :alt="r.d.title" />
          <span class="rm"><strong>{{ r.d.title }}</strong><em>第{{ r.h.episode }}集 / 共{{ r.d.episodeCount }}集</em>
            <span class="pg"><ProgressBar :value="r.h.percent" /><i>{{ r.h.percent }}%</i></span></span>
          <span class="pl" aria-hidden="true"><Play :size="16" fill="currentColor" /></span>
        </button></div>
      </section>
    </template>
  </div>
</template>
<style scoped lang="scss">
.home { padding-top: 11px; min-height: 100%; }
.sk-row { display: grid; grid-template-columns: repeat(6, 1fr); gap: 10px; margin-top: 30px; }
.cats { display: flex; align-items: center; gap: 12px; margin-top: 18px; height: 38px; padding-left: 4px; }
.h-label { font-size: 20px; font-weight: 600; margin-right: 8px; color: var(--c-text); }
.chip { height: 38px; padding: 0 18px; display: inline-flex; align-items: center; gap: 8px; border-radius: 19px; font-size: 15px; background: var(--c-surface); border: 1px solid var(--c-border); flex: 1; justify-content: center; max-width: 124px; &:hover { background: var(--c-surface-hover); border-color: var(--c-border-strong); } }
.more { margin-left: auto; font-size: 13px; color: var(--c-text-2); display: inline-flex; align-items: center; }
.block { margin-top: 26px; padding-left: 8px; }
.sec-h { display: flex; align-items: center; gap: 10px; h2 { font-size: 24px; font-weight: 700; } }
.muted { color: var(--c-text-2); font-size: 14px; margin-left: 6px; }
.new { padding: 1px 8px; border-radius: 10px; background: var(--g-accent); font-size: 12px; font-weight: 700; }
.link { margin-left: auto; display: inline-flex; align-items: center; font-size: 14px; color: var(--c-text); &:hover { color: var(--c-accent-2); } }
.grid6 { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 11px; margin-top: 11px; }
.rank { margin-top: 24px; }
.seg { display: inline-flex; margin-left: 10px; padding: 3px; border-radius: 20px; background: var(--c-surface); border: 1px solid var(--c-border); button { height: 30px; padding: 0 18px; border-radius: 16px; font-size: 14px; color: var(--c-text-2); &.on { background: var(--g-accent); color: #fff; } } }
.rank-list { display: grid; grid-template-columns: repeat(5, 1fr); margin-top: 12px; }
.ri { display: flex; align-items: center; gap: 14px; width: 100%; text-align: left; padding: 0 8px 0 16px; &:hover strong { color: var(--c-accent-2); } }
.cov { position: relative; flex: none; width: 84px; height: 100px; border-radius: 10px; overflow: visible; img { width: 100%; height: 100%; object-fit: cover; border-radius: 10px; } }
.no { position: absolute; left: -9px; top: -7px; width: 28px; height: 30px; display: grid; place-items: center; font-size: 15px; clip-path: polygon(0 0, 100% 0, 100% 78%, 50% 100%, 0 78%); background: #5a4a4a; color: #fff;
  &.r1 { background: linear-gradient(#ffc24a, #e8892a); } &.r2 { background: linear-gradient(#cfd3de, #8d93a6); } &.r3 { background: linear-gradient(#f0b27a, #b9743c); } &.rn { background: none; top: -16px; left: -4px; font-size: 18px; color: var(--c-text); } }
.rt { display: flex; flex-direction: column; gap: 10px; strong { font-size: 16px; line-height: 1.3; } em { font-style: normal; display: inline-flex; align-items: center; gap: 5px; font-size: 13px; color: var(--c-text-2); } }
.resume { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; margin-top: 13px; }
.rc { position: relative; display: flex; gap: 14px; align-items: center; height: 99px; padding: 8px 14px 8px 10px; border-radius: 14px; background: var(--c-surface); border: 1px solid var(--c-border); text-align: left; &:hover { background: var(--c-surface-hover); }
  img { width: 112px; height: 80px; object-fit: cover; border-radius: 10px; } }
.rm { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; strong { font-size: 16px; } em { font-style: normal; font-size: 13px; color: var(--c-text-2); } }
.pg { display: flex; align-items: center; gap: 10px; :deep(.bar) { flex: 1; } i { font-style: normal; font-size: 13px; color: var(--c-text-2); width: 36px; } }
.pl { position: absolute; right: 14px; top: 14px; width: 30px; height: 30px; border-radius: 50%; background: rgba(255,255,255,.88); color: #2a1517; display: grid; place-items: center; }
</style>
