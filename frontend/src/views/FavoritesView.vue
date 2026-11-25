<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Heart, Play, CircleCheck, Clapperboard, Video, SquarePen, LayoutGrid, List, ListChecks, Ellipsis, ChevronDown } from 'lucide-vue-next'
import { useLicensedCatalog } from '@/stores/licensedCatalog'
import { continuationEpisode, detailRoute, playbackRoute, savedMediaIssue } from '@/router/libraryRoutes'
import { useLibraryStore } from '@/stores/library'
import { watchStatus } from '@/composables'
import { BaseButton, EmptyState, ProgressBar, Modal } from '@/components/ui'
import type { Drama } from '@/types'

const router = useRouter(); const lib = useLibraryStore()
const tab = ref<'all'|'watching'|'done'|'manju'|'real'>('all'); const sort = ref<'recent'|'update'|'heat'>('recent'); const genre = ref(''); const view = ref<'grid'|'list'>('grid')
const batch = ref(false); const picked = ref<Set<string>>(new Set()); const confirm = ref(false); const menu = ref('')
const { savedCard } = useLicensedCatalog()
const watchLabels = { done: '已看过', watching: '在追', unwatched: '未看', unknown: '完成状态未知' }
const rows = computed(() => lib.favorites.map((id) => savedCard(id)).filter((d): d is Drama => !!d).map((d) => {
  const h = lib.history.find((x) => x.dramaId === d.id)
  const status = watchStatus(h, d)
  const issue = savedMediaIssue(d, h?.episode ?? 1)
  const episode = h ? continuationEpisode(d, h) ?? h.episode : 1
  const action = issue === 'library' ? '查看片库' : issue === 'episode' ? '选集'
    : h && episode !== h.episode ? '看下一集' : h?.percent === 100 ? '重播本集' : '播放'
  return { d, issue, status, pct: h?.percent ?? 0, last: h?.episode, episode, action }
}))
const counts = computed(() => ({ all: rows.value.length, watching: rows.value.filter((r) => r.status === 'watching').length, done: rows.value.filter((r) => r.status === 'done').length, manju: rows.value.filter((r) => r.d.kind === '漫剧').length, real: rows.value.filter((r) => r.d.kind === '真人剧').length }))
const genres = computed(() => [...new Set(rows.value.flatMap((r) => r.d.tags))])
const list = computed(() => {
  let l = rows.value.filter((r) => tab.value === 'all' || (tab.value === 'watching' && r.status === 'watching') || (tab.value === 'done' && r.status === 'done') || (tab.value === 'manju' && r.d.kind === '漫剧') || (tab.value === 'real' && r.d.kind === '真人剧'))
  if (genre.value) l = l.filter((r) => r.d.tags.includes(genre.value))
  if (sort.value === 'update') l = [...l].sort((a, b) => b.d.updatedTo - a.d.updatedTo); if (sort.value === 'heat') l = [...l].sort((a, b) => b.d.heat - a.d.heat)
  return l
})
const tabs = computed(() => [{ k: 'all', l: '全部收藏', n: counts.value.all, i: Heart }, { k: 'watching', l: '在追', n: counts.value.watching, i: Play }, { k: 'done', l: '已看过', n: counts.value.done, i: CircleCheck }, { k: 'manju', l: '漫剧', n: counts.value.manju, i: Clapperboard }, { k: 'real', l: '真人剧', n: counts.value.real, i: Video }] as const)
function unfav(id: string) { const idx = lib.favorites.indexOf(id); lib.removeFavs([id]); picked.value.delete(id); menu.value = ''; void lib.notifySaved('已取消收藏', 3500); lastRemoved = { id, idx } }
let lastRemoved: { id: string; idx: number } | null = null
function undo() {
  if (lastRemoved && lib.addFav(lastRemoved.id, lastRemoved.idx)) lastRemoved = null
}
function toggleBatch() { batch.value = !batch.value; picked.value = new Set() }
function pick(id: string) { const s = new Set(picked.value); s.has(id) ? s.delete(id) : s.add(id); picked.value = s }
const allPicked = computed(() => list.value.length > 0 && list.value.every((r) => picked.value.has(r.d.id)))
const selectAll = () => { picked.value = allPicked.value ? new Set() : new Set(list.value.map((r) => r.d.id)) }
function doBatch() { const ids = [...picked.value]; lib.removeFavs(ids); picked.value = new Set(); confirm.value = false; void lib.notifySaved(`已取消收藏 ${ids.length} 部`) }
type Row = (typeof rows.value)[number]
const detailTarget = (r: Row) => r.issue === 'library' ? { name: 'licensed' } : detailRoute(r.d.id)
function play(r: Row) {
  return router.push(r.issue ? detailTarget(r) : playbackRoute(r.d.id, r.episode))
}
const open = (r: Row) => (batch.value ? pick(r.d.id) : router.push(detailTarget(r)))
</script>
<template>
  <div class="fav" data-testid="page-favorites">
    <header class="hero"><div><Heart :size="46" :stroke-width="1.6" class="hi" /></div><div><h1>我的收藏</h1><p data-testid="fav-count">共收藏 {{ counts.all }} 部短剧</p></div>
      <svg class="deco" viewBox="0 0 180 120" aria-hidden="true"><g transform="rotate(-14 90 60)"><rect x="30" y="20" width="120" height="80" rx="10" fill="none" stroke="#ff7a5c" stroke-width="3" opacity=".7"/><path d="M90 78 C 60 58 70 40 90 52 C 110 40 120 58 90 78Z" fill="#ff7a5c" opacity=".8"/></g></svg></header>
    <div class="tabs"><button v-for="t in tabs" :key="t.k" :class="{ on: tab === t.k }" :data-testid="`ftab-${t.k}`" @click="tab = t.k"><component :is="t.i" :size="18" />{{ t.l }} <span>({{ t.n }})</span></button>
      <BaseButton class="edit" variant="outline" data-testid="edit-manage" @click="toggleBatch"><SquarePen :size="16" />{{ batch ? '完成管理' : '编辑管理' }}</BaseButton></div>
    <div class="bar"><label>排序：<span class="sel"><select v-model="sort" data-testid="sort"><option value="recent">最近收藏</option><option value="update">最近更新</option><option value="heat">热度最高</option></select><ChevronDown :size="14" /></span></label>
      <label>筛选：<span class="sel"><select v-model="genre" data-testid="genre"><option value="">全部题材</option><option v-for="g in genres" :key="g" :value="g">{{ g }}</option></select><ChevronDown :size="14" /></span></label>
      <span class="sp" /><div class="vt"><button :class="{ on: view === 'grid' }" aria-label="网格视图" data-testid="view-grid" @click="view = 'grid'"><LayoutGrid :size="18" /></button><button :class="{ on: view === 'list' }" aria-label="列表视图" data-testid="view-list" @click="view = 'list'"><List :size="18" /></button></div>
      <BaseButton variant="ghost" data-testid="batch-toggle" @click="toggleBatch"><ListChecks :size="16" />批量管理</BaseButton></div>
    <EmptyState v-if="!list.length" :title="counts.all ? '该分类下暂无收藏' : '还没有收藏任何短剧'" :hint="counts.all ? '换个分类或筛选条件试试' : '去推荐页发现喜欢的剧吧'"><BaseButton variant="outline" @click="router.push('/home')">去逛逛</BaseButton></EmptyState>
    <ul v-else class="grid" :class="view" data-testid="fav-grid">
      <li v-for="r in list" :key="r.d.id" class="card" :class="{ sel: picked.has(r.d.id) }" data-testid="fav-card">
        <button class="cover" :aria-label="`${r.d.title}`" @click="open(r)"><img :src="r.d.cover" :alt="r.d.title" /><span class="st"><Play v-if="r.status === 'watching'" :size="11" /><CircleCheck v-else-if="r.status === 'done'" :size="11" />{{ r.d.kind === '真人剧' && r.status === 'unwatched' ? '真人剧' : watchLabels[r.status] }}</span>
          <i v-if="batch" class="ck" :class="{ on: picked.has(r.d.id) }" role="checkbox" :aria-checked="picked.has(r.d.id)" /></button>
        <div class="info"><div class="tt"><h3 class="clamp-2">{{ r.d.title }}</h3><div class="mw"><button aria-label="更多" data-testid="fav-more" @click="menu = menu === r.d.id ? '' : r.d.id"><Ellipsis :size="18" /></button><ul v-if="menu === r.d.id" class="mn"><li><button @click="router.push(detailTarget(r))">{{ r.issue === 'library' ? '查看片库' : '查看详情' }}</button></li><li><button data-testid="menu-unfav" @click="unfav(r.d.id)">取消收藏</button></li></ul></div></div>
          <p v-if="r.issue !== 'library'" class="mt">{{ r.d.tags.slice(0, 2).join(' · ') }} · {{ r.d.episodeCount }}集</p>
          <p v-if="!r.issue" class="up">{{ r.d.finished ? `已完结（共${r.d.episodeCount}集）` : `更新至 第${r.d.updatedTo}集` }}<em v-if="!r.d.finished && r.d.updatedTo > (r.last ?? 0) + 3" class="new">NEW</em></p>
          <p v-else class="up">{{ r.issue === 'library' ? '本地媒体不可用' : '上次观看的分集不可用' }}</p>
          <p class="lw">{{ r.last ? `上次看到：第${r.last}集` : '尚未观看' }}</p>
          <div class="pg"><ProgressBar :value="r.pct" /><i>{{ r.pct }}%</i></div></div>
        <div class="ops"><button class="p" data-testid="fav-play" @click="play(r)"><Play v-if="!r.issue" :size="13" fill="currentColor" />{{ r.action }}</button><button class="u" data-testid="fav-unfav" @click="unfav(r.d.id)"><Heart :size="14" fill="currentColor" />取消收藏</button></div>
      </li></ul>
    <div v-if="batch" class="batchbar" data-testid="batchbar"><button class="all" data-testid="select-all" @click="selectAll"><i class="ck" :class="{ on: allPicked }" />全选</button><span>已选 {{ picked.size }} 部</span><BaseButton :disabled="!picked.size" data-testid="batch-unfav" @click="confirm = true">取消收藏</BaseButton><BaseButton variant="ghost" @click="toggleBatch">完成</BaseButton></div>
    <Modal :open="confirm" title="取消收藏" confirm-text="确认取消" @cancel="confirm = false" @confirm="doBatch">确定取消收藏已选的 {{ picked.size }} 部短剧吗？</Modal>
    <button v-if="false" @click="undo" />
  </div>
</template>
<style scoped lang="scss">
.fav { padding: 12px 16px 80px 14px; position: relative; }
.hero { position: relative; display: flex; align-items: center; gap: 20px; height: 112px; padding: 0 40px; border-radius: 24px; background: linear-gradient(90deg, rgba(120,32,28,.45), rgba(60,20,22,.3)); border: 1px solid var(--c-border); overflow: hidden; .hi { color: #ff8a70; } h1 { font-size: 36px; } p { margin-top: 6px; font-size: 15px; color: var(--c-text-2); } .deco { position: absolute; right: 60px; top: -4px; width: 190px; } }
.tabs { display: flex; align-items: center; gap: 22px; margin-top: 18px; button:not(.edit) { height: 48px; padding: 0 30px; border-radius: 24px; display: inline-flex; align-items: center; gap: 8px; font-size: 18px; color: var(--c-text-2); span { font-size: 16px; } &.on { background: var(--g-accent); color: #fff; box-shadow: 0 6px 18px rgba(220,58,42,.35); } } .edit { margin-left: auto; } }
.bar { display: flex; align-items: center; gap: 18px; margin: 18px 0 14px; font-size: 15px; color: var(--c-text-2); .sp { flex: 1; } .sel { position: relative; display: inline-flex; align-items: center; select { appearance: none; height: 40px; padding: 0 40px 0 16px; min-width: 150px; border-radius: 12px; background: var(--c-surface); border: 1px solid var(--c-border-strong); color: var(--c-text); font-size: 15px; outline: none; option { background: #2a1719; } } svg { position: absolute; right: 14px; pointer-events: none; } } .vt { display: flex; padding: 3px; border-radius: 10px; background: var(--c-surface); border: 1px solid var(--c-border); button { width: 40px; height: 34px; border-radius: 8px; display: grid; place-items: center; color: var(--c-text-2); &.on { background: rgba(255,255,255,.14); color: var(--c-text); } } } }
.grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; &.list { grid-template-columns: 1fr; } }
.card { position: relative; display: grid; grid-template-columns: 126px 1fr; grid-template-rows: auto auto; column-gap: 12px; padding: 8px 10px 10px 6px; border-radius: 16px; background: linear-gradient(180deg, rgba(255,255,255,.07), rgba(255,255,255,.035)); border: 1px solid var(--c-border); &.sel { border-color: var(--c-accent); box-shadow: 0 0 0 1px var(--c-accent); } }
.list .card { grid-template-columns: 126px 1fr auto; align-items: center; .ops { grid-column: 3; flex-direction: column; width: 140px; } }
.cover { position: relative; width: 126px; height: 162px; border-radius: 10px; overflow: hidden; img { width: 100%; height: 100%; object-fit: cover; } .st { position: absolute; left: 0; top: 6px; display: inline-flex; align-items: center; gap: 3px; padding: 2px 8px 2px 6px; border-radius: 0 8px 8px 0; background: rgba(160,40,30,.85); font-size: 12px; } .ck { position: absolute; right: 6px; top: 6px; } }
.ck { width: 20px; height: 20px; border-radius: 50%; border: 2px solid rgba(255,255,255,.7); background: rgba(0,0,0,.35); display: inline-block; &.on { background: var(--c-accent); border-color: var(--c-accent); box-shadow: inset 0 0 0 3px rgba(0,0,0,.2); } }
.info { min-width: 0; display: flex; flex-direction: column; gap: 4px; .tt { display: flex; gap: 6px; h3 { font-size: 17px; line-height: 1.25; flex: 1; } } p { font-size: 12.5px; color: var(--c-text-2); } .up { display: flex; align-items: center; gap: 6px; } .new { font-style: normal; padding: 0 6px; border-radius: 6px; background: var(--g-accent); color: #fff; font-size: 11px; font-weight: 700; } .pg { display: flex; align-items: center; gap: 8px; margin-top: auto; :deep(.bar) { flex: 1; } i { font-style: normal; font-size: 12px; color: var(--c-text-2); } } }
.mw { position: relative; } .mn { position: absolute; right: 0; top: 24px; z-index: 6; min-width: 110px; padding: 6px; border-radius: 10px; background: #2a1719; border: 1px solid var(--c-border-strong); button { width: 100%; height: 32px; border-radius: 6px; &:hover { background: var(--c-surface-hover); } } }
.ops { grid-column: 1 / -1; display: flex; gap: 10px; margin-top: 8px; padding-left: 6px; button { flex: 1; height: 34px; border-radius: 17px; display: inline-flex; align-items: center; justify-content: center; gap: 6px; font-size: 14px; } .p { background: var(--g-accent); font-weight: 600; } .u { background: rgba(255,255,255,.08); border: 1px solid var(--c-border-strong); svg { color: var(--c-accent-2); } } }
.batchbar { position: sticky; bottom: 8px; display: flex; align-items: center; gap: 16px; margin-top: 16px; padding: 10px 20px; border-radius: 16px; background: rgba(40,22,24,.96); border: 1px solid var(--c-border-strong); box-shadow: 0 10px 30px rgba(0,0,0,.5); .all { display: inline-flex; align-items: center; gap: 10px; } span { margin-right: auto; color: var(--c-text-2); } }
</style>
