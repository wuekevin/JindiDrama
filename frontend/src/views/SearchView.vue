<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, Video, Cat, Bot, CircleCheck, RefreshCw, ChevronDown, Flame, Play, X, Clock } from 'lucide-vue-next'
import { searchDramas, HOT_WORDS, type SearchFilters } from '@/mocks/provider'
import { useSearchStore } from '@/stores/search'
import type { Drama } from '@/types'
import { BaseButton, EmptyState, ErrorState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'

const route = useRoute(); const router = useRouter(); const hist = useSearchStore()
const PAGE = 5
const q = computed(() => String(route.query.q ?? ''))
const kind = computed(() => String(route.query.kind ?? ''))
const status = computed(() => String(route.query.status ?? '') as SearchFilters['status'])
const sort = computed(() => String(route.query.sort ?? 'comprehensive') as NonNullable<SearchFilters['sort']>)
const page = ref(1)
const state = ref<'loading'|'ready'|'empty'|'error'>('loading')
const err = ref(''); const items = ref<Drama[]>([])
let token = 0
async function run() {
  const my = ++token; state.value = 'loading'; page.value = 1
  if (q.value) hist.add(q.value)
  if (route.query.state === 'loading') return
  try {
    const r = await searchDramas(q.value, { kind: kind.value, status: status.value, sort: sort.value }, route.query.state === 'error' ? 'error' : 'normal')
    if (my !== token) return
    items.value = r; state.value = r.length ? 'ready' : 'empty'
  } catch (e) { if (my !== token) return; err.value = (e as Error).message; state.value = 'error' }
}
watch(() => route.fullPath, run, { immediate: true })
const pages = computed(() => Math.max(1, Math.ceil(items.value.length / PAGE)))
const shown = computed(() => items.value.slice((page.value - 1) * PAGE, page.value * PAGE))
const patch = (o: Record<string, string | undefined>) => router.replace({ query: { ...route.query, ...o } })
const kinds = [{ k: '', l: '全部', i: null }, { k: '真人剧', l: '真人剧', i: Video }, { k: '漫剧', l: '漫剧', i: Cat }, { k: 'AI剧', l: 'AI剧', i: Bot }]
const sorts = [{ k: 'hot', l: '最热' }, { k: 'new', l: '最新' }, { k: 'rating', l: '评分最高' }]
const kindTone = (k: string) => (k === '漫剧' ? 'purple' : k === 'AI剧' ? 'blue' : 'red')
const go = (t: string) => router.push({ name: 'search', query: { q: t } })
const watchIt = (id: string) => router.push(`/watch/${id}/1`)
</script>
<template>
  <div class="search" data-testid="page-search">
    <div class="main">
      <header class="head">
        <h1><Search :size="38" :stroke-width="2.2" /><template v-if="q"><em>“{{ q }}”</em>的搜索结果</template><template v-else>全部短剧</template></h1>
        <p v-if="state === 'ready' || state === 'empty'" class="count" data-testid="result-count">共找到 {{ items.length }} 部相关内容</p>
      </header>
      <div class="filters" role="group" aria-label="筛选">
        <button v-for="k in kinds" :key="k.l" class="pill" :class="{ on: kind === k.k }" :data-testid="`kind-${k.l}`" @click="patch({ kind: k.k || undefined })"><component :is="k.i" v-if="k.i" :size="17" />{{ k.l }}</button>
        <button class="pill" :class="{ on: status === 'finished' }" data-testid="status-finished" @click="patch({ status: status === 'finished' ? undefined : 'finished' })"><CircleCheck :size="17" />已完结</button>
        <button class="pill" :class="{ on: status === 'ongoing' }" data-testid="status-ongoing" @click="patch({ status: status === 'ongoing' ? undefined : 'ongoing' })"><RefreshCw :size="17" />连载中</button>
      </div>
      <div class="sorts"><span class="comp">综合排序<ChevronDown :size="14" /></span>
        <button v-for="s in sorts" :key="s.k" :class="{ on: sort === s.k }" :data-testid="`sort-${s.k}`" @click="patch({ sort: s.k })">{{ s.l }}</button></div>

      <div v-if="state === 'loading'" class="list"><Skeleton v-for="i in 4" :key="i" style="height: 138px" /></div>
      <ErrorState v-else-if="state === 'error'" :message="err" @retry="run" />
      <EmptyState v-else-if="state === 'empty'" title="没有找到相关短剧" hint="换个关键词，或查看右侧热搜"><BaseButton variant="outline" @click="router.push({ name: 'search' })">清除筛选</BaseButton></EmptyState>
      <ul v-else class="list">
        <li v-for="d in shown" :key="d.id" class="item" data-testid="result-card">
          <button class="cover" :aria-label="`播放${d.title}`" @click="watchIt(d.id)"><img :src="d.cover" :alt="d.title" /><i class="pb"><Play :size="12" fill="currentColor" /></i></button>
          <div class="info">
            <div class="t"><h3 class="clamp-1">{{ d.title }}</h3><span class="heat"><Flame :size="15" color="#ff6a3d" fill="#ff6a3d" />{{ d.heat.toFixed(1) }}万热度</span></div>
            <div class="tags"><span class="tag" :class="kindTone(d.kind)">{{ d.kind }}</span><span v-for="t in d.tags" :key="t" class="tag">{{ t }}</span></div>
            <p class="sum clamp-2">{{ d.summary }}</p>
          </div>
          <div class="act"><span class="eps">共{{ d.episodeCount }}集 · {{ d.finished ? '已完结' : '连载中' }}</span>
            <BaseButton data-testid="play-now" @click="watchIt(d.id)"><Play :size="15" fill="currentColor" />立即播放</BaseButton>
            <BaseButton variant="ghost" data-testid="view-detail" @click="router.push(`/drama/${d.id}`)">查看详情</BaseButton></div>
        </li>
      </ul>
      <nav v-if="pages > 1" class="pager" aria-label="分页"><button v-for="p in pages" :key="p" :class="{ on: p === page }" @click="page = p">{{ p }}</button></nav>
    </div>

    <aside class="side">
      <section class="panel"><header><Clock :size="26" /><h3>最近搜索</h3><button class="clr" data-testid="clear-history" @click="hist.clear()">清空</button></header>
        <p v-if="!hist.history.length" class="none">暂无搜索记录</p>
        <ul><li v-for="h in hist.history.slice(0, 5)" :key="h" data-testid="history-item"><button class="w" @click="go(h)"><Search :size="16" />{{ h }}</button><button class="x" :aria-label="`删除${h}`" @click="hist.remove(h)"><X :size="14" /></button></li></ul></section>
      <section class="panel hot"><header><Flame :size="26" color="#ff6a3d" fill="#ff6a3d" /><h3>热搜关键词</h3></header>
        <ol><li v-for="(w, i) in HOT_WORDS" :key="w.word"><button data-testid="hot-word" @click="go(w.word)"><b :class="`n${i < 3 ? i + 1 : 0}`">{{ i + 1 }}</b><span class="clamp-1" :class="{ top: i === 0 }">{{ w.word }}</span><em><Flame :size="13" color="#ff6a3d" fill="#ff6a3d" />{{ w.heat }}</em></button></li></ol></section>
    </aside>
  </div>
</template>
<style scoped lang="scss">
.search { display: flex; gap: 24px; padding: 6px 8px 0 17px; min-height: 100%; }
.main { flex: 1; min-width: 0; }
.head { padding-top: 38px; padding-left: 10px; h1 { display: flex; align-items: center; gap: 12px; font-size: 34px; font-weight: 700; em { font-style: normal; color: var(--c-accent-2); } } .count { margin-top: 18px; font-size: 15px; color: var(--c-text-2); } }
.filters { display: flex; gap: 14px; margin-top: 30px; padding-left: 0; }
.pill { height: 36px; padding: 0 22px; display: inline-flex; align-items: center; gap: 8px; border-radius: 18px; background: var(--c-surface); border: 1px solid var(--c-border-strong); font-size: 15px; &:hover { background: var(--c-surface-hover); } &.on { background: var(--g-accent); border-color: transparent; box-shadow: 0 4px 14px rgba(220,58,42,.35); } }
.sorts { display: flex; align-items: center; gap: 0; margin: 18px 0 16px 0; font-size: 16px; .comp { display: inline-flex; align-items: center; gap: 6px; padding-right: 20px; } button { padding: 0 24px; border-left: 1px solid var(--c-border-strong); color: var(--c-text); line-height: 1; &.on { color: var(--c-accent-2); } &:first-of-type { border-left: 0; padding-left: 12px; } } }
.list { display: flex; flex-direction: column; gap: 12px; }
.item { display: flex; align-items: center; gap: 16px; height: 138px; padding: 8px 14px 8px 10px; border-radius: 14px; background: linear-gradient(90deg, rgba(255,255,255,.07), rgba(255,255,255,.04)); border: 1px solid var(--c-border); }
.cover { position: relative; flex: none; width: 214px; height: 122px; border-radius: 10px; overflow: hidden; img { width: 100%; height: 100%; object-fit: cover; } .pb { position: absolute; right: 8px; bottom: 8px; width: 28px; height: 28px; border-radius: 50%; background: rgba(0,0,0,.6); display: grid; place-items: center; } }
.info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 9px; }
.t { display: flex; align-items: center; gap: 12px; h3 { font-size: 21px; max-width: 360px; } }
.heat { display: inline-flex; align-items: center; gap: 4px; font-size: 14px; color: var(--c-text); margin-left: auto; }
.tags { display: flex; gap: 8px; } .tag { padding: 2px 10px; border-radius: 6px; font-size: 12.5px; background: rgba(255,255,255,.1); color: var(--c-text-2); &.red { background: #b04336; color: #fff; } &.purple { background: #6a55c8; color: #fff; } &.blue { background: #4a6fd6; color: #fff; } }
.sum { font-size: 13.5px; line-height: 1.55; color: var(--c-text-2); }
.act { flex: none; width: 142px; display: flex; flex-direction: column; gap: 8px; align-items: stretch; .eps { font-size: 13px; text-align: center; color: var(--c-text-2); } :deep(.btn) { border-radius: 20px; } }
.pager { display: flex; justify-content: center; gap: 8px; margin-top: 18px; button { width: 34px; height: 34px; border-radius: 10px; background: var(--c-surface); &.on { background: var(--g-accent); } } }
.side { flex: none; width: 268px; display: flex; flex-direction: column; gap: 14px; padding-top: 178px; }
.panel { padding: 16px 16px 12px; border-radius: 20px; background: linear-gradient(180deg, rgba(255,255,255,.07), rgba(255,255,255,.035)); border: 1px solid var(--c-border);
  header { display: flex; align-items: center; gap: 10px; padding-bottom: 10px; border-bottom: 1px solid var(--c-border); margin-bottom: 6px; h3 { font-size: 20px; } .clr { margin-left: auto; font-size: 13px; color: var(--c-text-2); } }
  .none { padding: 14px 0; font-size: 13px; color: var(--c-text-3); }
  li { display: flex; align-items: center; height: 34.5px; } .w { flex: 1; display: flex; align-items: center; gap: 10px; text-align: left; font-size: 15px; } .x { color: var(--c-text-2); } }
.hot li { height: 41px; button { width: 100%; display: flex; align-items: center; gap: 12px; font-size: 15px; } span { flex: 1; text-align: left; &.top { color: var(--c-accent-2); } } em { font-style: normal; display: inline-flex; align-items: center; gap: 4px; font-size: 13px; color: var(--c-text-2); }
  b { width: 26px; height: 26px; border-radius: 8px; display: grid; place-items: center; font-size: 14px; flex: none; background: rgba(255,255,255,.08); &.n1 { background: linear-gradient(#ffc24a, #e8892a); } &.n2 { background: linear-gradient(#cfd3de, #8d93a6); } &.n3 { background: linear-gradient(#f0b27a, #b9743c); } } }
</style>
