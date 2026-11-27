<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, Video, Cat, Bot, BookOpen, CircleCheck, RefreshCw, ChevronDown, Flame, Play, X, Clock } from 'lucide-vue-next'
import type { CatalogGenre } from '@/bindings'
import { catalogProviderName, cmsClassificationError, fallbackCatalogCover, vCancelCatalogCover, loadCatalogHome, loadCMSCatalogGenrePage, loadCMSCatalogGenres, loadCMSCatalogPage, searchDramas, HOT_WORDS, MAX_CMS_PAGE, type SearchFilters } from '@/mocks/provider'
import { useSearchStore } from '@/stores/search'
import { useLicensedCatalog } from '@/stores/licensedCatalog'
import { detailRoute, playbackRoute } from '@/router/libraryRoutes'
import { feedLabels, searchDiscovery } from '@/router/searchDiscovery'
import type { Drama } from '@/types'
import { BaseButton, EmptyState, ErrorState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'

const route = useRoute(); const router = useRouter(); const hist = useSearchStore()
const catalog = useLicensedCatalog()
const PAGE = 5
const root = ref<HTMLElement>()
const heading = ref<HTMLElement>()
const layout = ref<'wide' | 'stacked' | 'compact'>('wide')
let layoutObserver: ResizeObserver | undefined
onMounted(() => {
  if (!root.value) return
  // Measure unscaled layout space consistently in Chromium and WKWebView.
  layoutObserver = new ResizeObserver(([entry]) => {
    const width = entry.contentRect.width
    layout.value = width <= 620 ? 'compact' : width <= 950 ? 'stacked' : 'wide'
  })
  layoutObserver.observe(root.value)
})
onBeforeUnmount(() => {
  token++
  layoutObserver?.disconnect()
})
const q = computed(() => String(route.query.q ?? ''))
const castOnly = computed(() => route.query.cast === '1')
const discovery = computed(() => searchDiscovery(route.query))
const browseTopics = computed(() => discovery.value === 'topics')
const feed = computed(() => discovery.value === 'topics' ? undefined : discovery.value)
const topic = computed(() => typeof route.query.topic === 'string' ? route.query.topic.trim() : '')
const genre = computed(() => route.query.genre)
const requestedCMSGenre = computed(() => typeof genre.value === 'string' && /^[1-9][0-9]{0,29}$/.test(genre.value)
  ? genre.value : undefined)
const catalogLabel = ref<string>()
const discoveryTitle = computed(() => {
  if (feed.value === 'today' && catalogLabel.value) return catalogLabel.value
  if (feed.value) return feedLabels[feed.value]
  return browseTopics.value ? (cmsTopics.value ? '按分类浏览' : '按题材浏览') : ''
})
const topics = ref<string[]>([])
const knownTopic = computed(() => topics.value.includes(topic.value))
const cmsGenres = ref<CatalogGenre[]>()
const genreOptions = computed(() => {
  const genres = cmsGenres.value ?? []
  const counts = new Map<string, number>()
  for (const { name } of genres) counts.set(name, (counts.get(name) ?? 0) + 1)
  const labels = new Set(['全部分类', ...counts.keys()])
  return genres.map(({ id, name }) => {
    if (counts.get(name) === 1 && name !== '全部分类') return { id, name }
    let label = `${name}（ID ${id}）`
    for (let suffix = 2; labels.has(label); suffix++) label = `${name}（ID ${id} · ${suffix}）`
    labels.add(label)
    return { id, name: label }
  })
})
const selectedGenre = computed(() => cmsGenres.value?.find(({ id }) => id === genre.value))
const invalidCMSGenre = ref(false)
const kind = computed(() => String(route.query.kind ?? ''))
const status = computed(() => String(route.query.status ?? '') as SearchFilters['status'])
const sort = computed(() => String(route.query.sort ?? 'comprehensive') as NonNullable<SearchFilters['sort']>)
const page = ref(1)
const cmsPage = computed(() => {
  const raw = route.query.page
  if (typeof raw !== 'string' || !/^[0-9]{1,6}$/.test(raw)) return 1
  const value = Number(raw)
  return value >= 1 && value <= MAX_CMS_PAGE ? value : 1
})
const cmsPagination = ref(false)
const cmsTopics = computed(() => browseTopics.value && cmsPagination.value)
const cmsHasMore = ref(false)
const cmsFallback = ref<{ page: number; reason: string }>()
let lastCMSResult: { scope: string; page: number; items: Drama[]; hasMore: boolean } | undefined
let cmsReturnPath: string | undefined
const state = ref<'loading'|'ready'|'empty'|'error'>('loading')
const pendingCMSGenre = computed(() => !selectedGenre.value && (state.value === 'loading' || state.value === 'error')
  ? requestedCMSGenre.value : undefined)
const isDemo = ref(false)
const localResultsOnly = ref(false)
const err = ref(''); const items = ref<Drama[]>([])
let token = 0
let providerKnown = false
let providerName: string | undefined
let lastGenreScope: string | undefined
let lastCMSRequest: { scope: string; page: number } | undefined
const patch = (o: Record<string, string | undefined>) => router.replace({ query: { ...route.query, ...o } })
const errorMessage = (cause: unknown) => typeof cause === 'string' ? cause
  : cause instanceof Error ? cause.message : '搜索服务暂时不可用'
function focusResults() {
  heading.value?.focus({ preventScroll: true })
  root.value?.scrollIntoView({ block: 'start' })
}
function retry() {
  if (cmsPagination.value) focusResults()
  void run(true)
}
function showInvalidCMSGenre() {
  invalidCMSGenre.value = true
  lastCMSResult = undefined
  items.value = []
  state.value = 'empty'
}
async function run(isRetry = false) {
  const my = ++token
  const returningToCMSResult = route.fullPath === cmsReturnPath
  cmsReturnPath = undefined
  state.value = 'loading'
  page.value = 1
  cmsHasMore.value = false
  cmsFallback.value = undefined
  invalidCMSGenre.value = false
  localResultsOnly.value = false
  const topicBrowse = browseTopics.value
  if (!topicBrowse) {
    cmsGenres.value = undefined
    lastGenreScope = undefined
  }
  if (route.query.state === 'loading') return
  let cmsScope: string | undefined
  try {
    const name = await catalogProviderName()
    if (my !== token) return
    const sourceChanged = providerKnown && name !== providerName
    providerKnown = true
    providerName = name
    if (sourceChanged) {
      cmsGenres.value = undefined
      lastGenreScope = undefined
      lastCMSRequest = undefined
      lastCMSResult = undefined
    }
    if (sourceChanged && route.query.page !== undefined && (cmsPagination.value || name === 'apple-cms')) {
      await patch({ page: undefined })
      return
    }

    const demo = name === 'mock'
    isDemo.value = demo
    const mode = route.query.state === 'error' ? 'error' : 'normal'
    const section = feed.value
    const selectedTopic = topic.value
    cmsPagination.value = name === 'apple-cms' && (section === 'today' || topicBrowse ||
      (!section && !castOnly.value && kind.value !== '本地片库'))
    if (!cmsPagination.value) lastCMSResult = undefined
    let results: Drama[]
    let localOnly = false
    let tags: string[] = []
    let label: string | undefined
    if (cmsPagination.value) {
      const requestedGenre = genre.value
      const genreScope = requestedCMSGenre.value
      const keyword = section || topicBrowse ? '' : q.value
      const requestedPage = cmsPage.value
      const scope = topicBrowse ? `genre:${genreScope ?? (requestedGenre === undefined ? '' : 'invalid')}`
        : section ? 'today' : `search:${keyword.trim()}`
      cmsScope = scope
      if (lastCMSResult?.scope !== scope) lastCMSResult = undefined
      if (returningToCMSResult && lastCMSResult?.page === requestedPage) {
        lastCMSRequest = { scope, page: requestedPage }
        cmsHasMore.value = lastCMSResult.hasMore
        items.value = lastCMSResult.items
        state.value = 'ready'
        await nextTick()
        if (my === token) focusResults()
        return
      }
      if (topicBrowse) {
        if (isRetry || cmsGenres.value === undefined || (genreScope && lastGenreScope !== genreScope)) {
          const genres = await loadCMSCatalogGenres(mode)
          if (my !== token) return
          cmsGenres.value = genres
        }
        lastGenreScope = genreScope
        if (requestedGenre !== undefined && !selectedGenre.value) {
          showInvalidCMSGenre()
          return
        }
      }
      const genreID = topicBrowse ? selectedGenre.value?.id : undefined
      const pageChanged = lastCMSRequest?.scope === scope
        ? lastCMSRequest.page !== requestedPage
        : requestedPage > 1
      lastCMSRequest = { scope, page: requestedPage }
      label = section ? '片源目录' : undefined
      catalogLabel.value = label
      if (pageChanged) {
        await nextTick()
        if (my !== token) return
        focusResults()
      }
      const local = !section && !topicBrowse && requestedPage === 1 ? catalog.searchLocal(keyword) : []
      try {
        const remote = genreID
          ? await loadCMSCatalogGenrePage(genreID, requestedPage, mode)
          : await loadCMSCatalogPage(keyword, requestedPage, mode)
        if (my !== token) return
        if (genreID && !remote.entries.length) {
          const genres = await loadCMSCatalogGenres(mode)
          if (my !== token) return
          cmsGenres.value = genres
          if (!selectedGenre.value) {
            showInvalidCMSGenre()
            return
          }
        }
        cmsHasMore.value = remote.hasMore
        results = [...local, ...remote.entries]
        lastCMSResult = results.length ? { scope, page: requestedPage, items: results, hasMore: remote.hasMore } : undefined
      } catch (error) {
        if (my !== token) return
        if (topicBrowse) throw cmsClassificationError(error)
        if (lastCMSResult || !local.length) throw error
        localOnly = true
        results = local
      }
    } else if (section) {
      const home = await loadCatalogHome(mode)
      label = home.catalogLabel
      results = section === 'today' ? home.todayNew : home.rankings[section]
    } else if (topicBrowse) {
      const dramas = await searchDramas('', {}, mode, catalog.card)
      tags = [...new Set(dramas.flatMap((drama) => drama.tags.map((tag) => tag.trim()).filter(Boolean)))]
        .sort((a, b) => a.localeCompare(b, 'zh-CN'))
      results = selectedTopic ? dramas.filter((drama) => drama.tags.some((tag) => tag.trim() === selectedTopic)) : dramas
    } else if (!demo && kind.value === '本地片库') {
      results = catalog.searchLocal(q.value)
    } else {
      const local = demo || castOnly.value ? [] : catalog.searchLocal(q.value)
      try {
        const matched = await searchDramas(q.value, { kind: kind.value, status: status.value, sort: sort.value, castOnly: castOnly.value }, mode, catalog.card)
        results = [...local, ...matched]
      } catch (error) {
        if (!local.length) throw error
        localOnly = true
        results = local
      }
    }
    if (my !== token) return
    catalogLabel.value = label
    localResultsOnly.value = localOnly
    topics.value = tags
    items.value = results
    state.value = results.length ? 'ready' : 'empty'
  } catch (cause) {
    if (my !== token) return
    const reason = errorMessage(cause)
    if (lastCMSResult && lastCMSResult.scope === cmsScope) {
      cmsFallback.value = { page: lastCMSResult.page, reason }
      items.value = lastCMSResult.items
      state.value = 'ready'
      return
    }
    err.value = reason
    state.value = 'error'
  }
}
watch([
  () => discovery.value ?? route.fullPath,
  () => browseTopics.value ? topic.value : '',
  () => browseTopics.value ? genre.value : undefined,
  () => route.query.state,
  () => feed.value === 'today' || browseTopics.value ? route.query.page : undefined,
], ([key, selectedTopic, , mode], [previousKey, previousTopic, , previousMode]) => {
  // Other providers already have the full discovery list in memory.
  if (key === previousKey && selectedTopic === previousTopic && mode === previousMode &&
      providerKnown && providerName !== 'apple-cms' && !cmsPagination.value) return
  void run()
}, { immediate: true })
const pages = computed(() => Math.max(1, Math.ceil(items.value.length / PAGE)))
const currentPage = computed(() => {
  if (!discovery.value) return page.value
  const raw = route.query.page
  const value = typeof raw === 'string' ? Number(raw) : 1
  return Number.isSafeInteger(value) && value > 0 ? Math.min(value, pages.value) : 1
})
const shown = computed(() => cmsPagination.value ? items.value
  : items.value.slice((currentPage.value - 1) * PAGE, currentPage.value * PAGE))
async function returnToCMSResult() {
  const retained = cmsFallback.value
  if (!retained || retained.page === cmsPage.value) return
  const target = { query: { ...route.query, page: retained.page === 1 ? undefined : String(retained.page) } }
  const path = router.resolve(target).fullPath
  cmsReturnPath = path
  try {
    await router.push(target)
  } finally {
    if (cmsReturnPath === path) cmsReturnPath = undefined
  }
}
function selectPage(value: number) {
  const routePage = value === 1 ? undefined : String(value)
  if (cmsPagination.value) void router.push({ query: { ...route.query, page: routePage } })
  else if (discovery.value) void patch({ page: routePage })
  else page.value = value
}
const selectTopic = (event: Event) => {
  const value = (event.target as HTMLSelectElement).value
  return patch({ topic: value || undefined, page: undefined })
}
function selectGenre(value: string) {
  return router.push({ query: { ...route.query, genre: value || undefined, topic: undefined, page: undefined } })
}
const selectKind = (value: string) => patch(value === '本地片库'
  ? { kind: value, status: undefined, page: undefined }
  : { kind: value || undefined, page: undefined })
const kinds = computed(() => isDemo.value ? [
  { k: '', l: '全部', i: null }, { k: '真人剧', l: '真人剧', i: Video }, { k: '漫剧', l: '漫剧', i: Cat }, { k: 'AI剧', l: 'AI剧', i: Bot },
  ...(!castOnly.value && catalog.entries.length ? [{ k: '本地片库', l: '本地片库', i: BookOpen }] : []),
] : catalog.entries.length ? [
  { k: '', l: '全部', i: null },
  { k: '本地片库', l: '本地片库', i: BookOpen },
] : [])
const sorts = [{ k: 'hot', l: '最热' }, { k: 'new', l: '最新' }, { k: 'rating', l: '评分最高' }]
const kindTone = (k: string) => (k === '漫剧' ? 'purple' : k === 'AI剧' ? 'blue' : 'red')
const go = (t: string) => {
  hist.add(t)
  return router.push({ name: 'search', query: { q: t } })
}
const watchIt = (id: string) => router.push(playbackRoute(id, 1, true))
</script>
<template>
  <div ref="root" class="search" :class="[layout, { discovery: discoveryTitle }]" data-testid="page-search">
    <div class="main">
      <header class="head">
        <h1 ref="heading" :tabindex="cmsPagination ? -1 : undefined"><Search :size="38" :stroke-width="2.2" /><span class="heading-text"><template v-if="discoveryTitle">{{ discoveryTitle }}</template><template v-else-if="q"><em>“{{ q }}”</em>的搜索结果</template><template v-else>全部短剧</template></span></h1>
        <p v-if="state === 'ready' || state === 'empty' && !invalidCMSGenre" class="count" data-testid="result-count">{{ cmsPagination ? '本页共' : '共找到' }} {{ items.length }} 部相关内容</p>
        <p v-if="cmsFallback" class="count" role="status">第 {{ cmsPage }} 页加载失败（{{ cmsFallback.reason }}）。当前显示已加载的第 {{ cmsFallback.page }} 页。</p>
        <p v-if="localResultsOnly" class="count" role="status">主目录暂不可用，已显示本地片库的匹配结果。</p>
        <BaseButton v-if="cmsPagination && (localResultsOnly || cmsFallback)" class="catalog-retry" variant="outline" @click="retry">重试</BaseButton>
      </header>
      <label v-if="cmsTopics" class="topic-filter">
        分类
        <select :value="selectedGenre?.id ?? pendingCMSGenre ?? ''" :disabled="!genreOptions.length" data-testid="genre-select" @change="selectGenre(($event.target as HTMLSelectElement).value)">
          <option value="">全部分类</option>
          <option v-if="pendingCMSGenre" :value="pendingCMSGenre" disabled>所选分类暂未加载</option>
          <option v-for="{ id, name } in genreOptions" :key="id" :value="id">{{ name }}</option>
        </select>
      </label>
      <p v-if="cmsTopics && cmsGenres?.length === 0" class="genre-status" role="status">片源未提供分类，可浏览全部目录。</p>
      <label v-if="browseTopics && !cmsTopics" class="topic-filter">
        题材
        <select :value="knownTopic ? topic : ''" data-testid="topic-select" @change="selectTopic">
          <option value="">全部题材</option>
          <option v-for="tag in topics" :key="tag" :value="tag">{{ tag }}</option>
        </select>
      </label>
      <div v-if="kinds.length && !discoveryTitle" class="filters" :class="{ 'filters-last': kind === '本地片库' }" role="group" aria-label="筛选">
        <button v-for="k in kinds" :key="k.l" class="pill" :class="{ on: kind === k.k }" :data-testid="`kind-${k.l}`" @click="selectKind(k.k)"><component :is="k.i" v-if="k.i" :size="17" />{{ k.l }}</button>
        <button v-if="isDemo && kind !== '本地片库'" class="pill" :class="{ on: status === 'finished' }" data-testid="status-finished" @click="patch({ status: status === 'finished' ? undefined : 'finished' })"><CircleCheck :size="17" />已完结</button>
        <button v-if="isDemo && kind !== '本地片库'" class="pill" :class="{ on: status === 'ongoing' }" data-testid="status-ongoing" @click="patch({ status: status === 'ongoing' ? undefined : 'ongoing' })"><RefreshCw :size="17" />连载中</button>
      </div>
      <div v-if="isDemo && !discoveryTitle && kind !== '本地片库'" class="sorts"><span class="comp">综合排序<ChevronDown :size="14" /></span>
        <button v-for="s in sorts" :key="s.k" :class="{ on: sort === s.k }" :data-testid="`sort-${s.k}`" @click="patch({ sort: s.k })">{{ s.l }}</button></div>

      <div v-if="state === 'loading'" class="list"><Skeleton v-for="i in 4" :key="i" style="height: 138px" /></div>
      <ErrorState v-else-if="state === 'error'" :message="err" @retry="retry" />
      <EmptyState v-else-if="invalidCMSGenre" title="片源目录没有这个分类">
        <BaseButton variant="outline" @click="selectGenre('')">查看全部分类</BaseButton>
      </EmptyState>
      <EmptyState v-else-if="state === 'empty'" :title="cmsPagination && cmsPage > 1 ? '本页暂无短剧' : feed ? '暂无短剧' : browseTopics && !cmsTopics && topic && !knownTopic ? '目录没有这个题材' : '没有找到相关短剧'" :hint="discoveryTitle ? undefined : '换个关键词，或查看热搜关键词'">
        <BaseButton v-if="cmsTopics && selectedGenre" variant="outline" @click="selectGenre('')">查看全部分类</BaseButton>
        <BaseButton v-else-if="!discoveryTitle || browseTopics && !cmsTopics && topic" variant="outline" @click="browseTopics ? patch({ topic: undefined, page: undefined }) : router.push({ name: 'search' })">清除筛选</BaseButton>
      </EmptyState>
      <ul v-else class="list">
        <li v-for="d in shown" :key="`${d.providerOnly ? 'catalog' : 'local'}:${d.id}`" class="item" data-testid="result-card">
          <button class="cover" :aria-label="`${d.providerOnly ? '查看' : '播放'}${d.title}`" @click="d.providerOnly ? router.push(detailRoute(d.id, true)) : watchIt(d.id)"><img v-cancel-catalog-cover :crossorigin="d.providerOnly ? 'anonymous' : undefined" :src="d.cover" :alt="d.title" :loading="d.providerOnly ? 'lazy' : undefined" @error="fallbackCatalogCover" /><i v-if="!d.providerOnly" class="pb"><Play :size="12" fill="currentColor" /></i></button>
          <div class="info">
            <div class="t"><h3 class="clamp-1">{{ d.title }}</h3><span v-if="!d.providerOnly && d.kind !== '本地片库'" class="heat"><Flame :size="15" color="#ff6a3d" fill="#ff6a3d" />{{ d.heat.toFixed(1) }}万热度</span></div>
            <div class="tags"><span v-if="d.kind" class="tag" :class="kindTone(d.kind)">{{ d.kind }}</span><span v-for="t in d.tags" :key="t" class="tag">{{ t }}</span></div>
            <p class="sum clamp-2">{{ d.summary }}</p>
          </div>
          <div class="act"><span class="eps">共{{ d.episodeCount }}集<template v-if="!d.providerOnly && d.kind !== '本地片库'"> · {{ d.finished ? '已完结' : '连载中' }}</template></span>
            <BaseButton v-if="!d.providerOnly" data-testid="play-now" @click="watchIt(d.id)"><Play :size="15" fill="currentColor" />立即播放</BaseButton>
            <BaseButton variant="ghost" data-testid="view-detail" @click="router.push(detailRoute(d.id, d.providerOnly))">查看详情</BaseButton></div>
        </li>
      </ul>
      <nav v-if="cmsPagination && !invalidCMSGenre && state !== 'loading'" class="pager cms-pager" aria-label="分页">
        <button v-if="cmsFallback && cmsFallback.page !== cmsPage" @click="returnToCMSResult">返回第 {{ cmsFallback.page }} 页</button>
        <button v-else-if="!cmsFallback && cmsPage > 1" @click="selectPage(cmsPage - 1)">上一页</button>
        <span aria-current="page">第 {{ cmsFallback?.page ?? cmsPage }} 页</span>
        <button v-if="cmsHasMore && cmsPage < MAX_CMS_PAGE" @click="selectPage(cmsPage + 1)">下一页</button>
      </nav>
      <nav v-else-if="!cmsPagination && state === 'ready' && pages > 1" class="pager" aria-label="分页"><button v-for="p in pages" :key="p" :class="{ on: p === currentPage }" :aria-current="p === currentPage ? 'page' : undefined" @click="selectPage(p)">{{ p }}</button></nav>
    </div>

    <aside class="side">
      <section class="panel"><header><Clock :size="26" /><h3>最近搜索</h3><button class="clr" data-testid="clear-history" @click="hist.clear()">清空</button></header>
        <p v-if="!hist.history.length" class="none">暂无搜索记录</p>
        <ul><li v-for="h in hist.history.slice(0, 5)" :key="h" data-testid="history-item"><button class="w" @click="go(h)"><Search :size="16" /><span class="clamp-1">{{ h }}</span></button><button class="x" :aria-label="`删除${h}`" @click="hist.remove(h)"><X :size="14" /></button></li></ul></section>
      <section v-if="isDemo" class="panel hot"><header><Flame :size="26" color="#ff6a3d" fill="#ff6a3d" /><h3>热搜关键词</h3></header>
        <ol><li v-for="(w, i) in HOT_WORDS" :key="w.word"><button data-testid="hot-word" @click="go(w.word)"><b :class="`n${i < 3 ? i + 1 : 0}`">{{ i + 1 }}</b><span class="clamp-1" :class="{ top: i === 0 }">{{ w.word }}</span><em><Flame :size="13" color="#ff6a3d" fill="#ff6a3d" />{{ w.heat }}</em></button></li></ol></section>
    </aside>
  </div>
</template>
<style scoped lang="scss">
.search { display: flex; gap: 24px; padding: 6px 8px 0 17px; min-height: 100%; }
.main { flex: 1; min-width: 0; }
.head { padding-top: 38px; padding-left: 10px; h1 { display: flex; align-items: center; gap: 12px; font-size: 34px; font-weight: 700; svg { flex: none; } em { font-style: normal; color: var(--c-accent-2); } } .count { margin-top: 18px; font-size: 15px; color: var(--c-text-2); } }
.heading-text { min-width: 0; overflow-wrap: anywhere; }
.search.wide .heading-text em { margin-inline-end: 12px; }
.filters { display: flex; flex-wrap: wrap; gap: 14px; margin-top: 30px; padding-left: 0; }
.discovery .list { margin-top: 18px; }
.topic-filter { display: flex; align-items: center; gap: 12px; margin: 22px 0 16px 10px; }
.topic-filter select { min-width: 0; max-width: 85%; background: var(--c-surface); color: var(--c-text); border: 1px solid var(--c-border-strong); border-radius: 8px; padding: 6px 10px; }
.genre-status { margin: 0 0 16px 10px; color: var(--c-text-2); font-size: 15px; }
.filters-last { margin-bottom: 16px; }
.pill { height: 36px; padding: 0 22px; display: inline-flex; align-items: center; gap: 8px; border-radius: 18px; background: var(--c-surface); border: 1px solid var(--c-border-strong); font-size: 15px; &:hover { background: var(--c-surface-hover); } &.on { background: var(--g-accent); border-color: transparent; box-shadow: 0 4px 14px rgba(220,58,42,.35); } }
.sorts { display: flex; align-items: center; gap: 0; margin: 18px 0 16px 0; font-size: 16px; .comp { display: inline-flex; align-items: center; gap: 6px; padding-right: 20px; } button { padding: 0 24px; border-left: 1px solid var(--c-border-strong); color: var(--c-text); line-height: 1; &.on { color: var(--c-accent-2); } &:first-of-type { border-left: 0; padding-left: 12px; } } }
.list { display: flex; flex-direction: column; gap: 12px; }
.item { display: flex; align-items: center; gap: 16px; height: 138px; padding: 8px 14px 8px 10px; border-radius: 14px; background: linear-gradient(90deg, rgba(255,255,255,.07), rgba(255,255,255,.04)); border: 1px solid var(--c-border); }
.cover { position: relative; flex: none; width: 214px; height: 122px; border-radius: 10px; overflow: hidden; img { width: 100%; height: 100%; object-fit: cover; } .pb { position: absolute; right: 8px; bottom: 8px; width: 28px; height: 28px; border-radius: 50%; background: rgba(0,0,0,.6); display: grid; place-items: center; } }
.info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 9px; }
.t { display: flex; align-items: center; gap: 12px; h3 { font-size: 21px; max-width: 360px; } }
.heat { display: inline-flex; align-items: center; gap: 4px; font-size: 14px; color: var(--c-text); margin-left: auto; }
.tags { display: flex; gap: 8px; overflow: hidden; } .tag { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 2px 10px; border-radius: 6px; font-size: 12.5px; background: rgba(255,255,255,.1); color: var(--c-text-2); &.red { background: #b04336; color: #fff; } &.purple { background: #6a55c8; color: #fff; } &.blue { background: #4a6fd6; color: #fff; } }
.sum { font-size: 13.5px; line-height: 1.55; color: var(--c-text-2); }
.act { flex: none; width: 142px; display: flex; flex-direction: column; gap: 8px; align-items: stretch; .eps { font-size: 13px; text-align: center; color: var(--c-text-2); } :deep(.btn) { border-radius: 20px; } }
.pager { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; margin-top: 18px; button { width: 34px; height: 34px; border-radius: 10px; background: var(--c-surface); &.on { background: var(--g-accent); } } }
.cms-pager { align-items: center; }
.cms-pager button { width: auto; padding-inline: 14px; }
.cms-pager span { padding-inline: 6px; }
.catalog-retry { margin-top: 10px; }
.side { flex: none; width: 268px; display: flex; flex-direction: column; gap: 14px; padding-top: 178px; }
.panel { padding: 16px 16px 12px; border-radius: 20px; background: linear-gradient(180deg, rgba(255,255,255,.07), rgba(255,255,255,.035)); border: 1px solid var(--c-border);
  header { display: flex; align-items: center; gap: 10px; padding-bottom: 10px; border-bottom: 1px solid var(--c-border); margin-bottom: 6px; h3 { font-size: 20px; } .clr { margin-left: auto; font-size: 13px; color: var(--c-text-2); } }
  .none { padding: 14px 0; font-size: 13px; color: var(--c-text-3); }
  li { display: flex; align-items: center; height: 34.5px; }
  .w { min-width: 0; flex: 1; display: flex; align-items: center; gap: 10px; text-align: left; font-size: 15px; }
  .w svg, .x { flex: none; }
  .x { color: var(--c-text-2); }
}
.hot li { height: 41px; button { width: 100%; display: flex; align-items: center; gap: 12px; font-size: 15px; } span { flex: 1; text-align: left; &.top { color: var(--c-accent-2); } } em { font-style: normal; display: inline-flex; align-items: center; gap: 4px; font-size: 13px; color: var(--c-text-2); }
  b { width: 26px; height: 26px; border-radius: 8px; display: grid; place-items: center; font-size: 14px; flex: none; background: rgba(255,255,255,.08); &.n1 { background: linear-gradient(#ffc24a, #e8892a); } &.n2 { background: linear-gradient(#cfd3de, #8d93a6); } &.n3 { background: linear-gradient(#f0b27a, #b9743c); } } }

.search:is(.stacked, .compact) {
  flex-direction: column;

  .side { width: 100%; padding-top: 0; padding-bottom: 16px; }
  .sorts { flex-wrap: wrap; row-gap: 10px; }
}
.search.compact {
  .item {
    display: grid;
    grid-template-columns: minmax(0, 120px) minmax(0, 1fr);
    height: auto;
    align-items: start;
    gap: 12px;
    padding: 10px;
  }
  .cover { width: 100%; height: auto; aspect-ratio: 16 / 9; }
  .t { align-items: flex-start; flex-direction: column; gap: 4px; }
  .t h3 { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; white-space: normal; overflow-wrap: anywhere; }
  .heat { margin-left: 0; }
  .act { grid-column: 1 / -1; width: auto; flex-direction: row; flex-wrap: wrap; align-items: center; }
  .act .eps { margin-right: auto; text-align: left; }
}
</style>
