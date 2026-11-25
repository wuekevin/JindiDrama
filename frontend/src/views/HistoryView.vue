<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Clock, Play, Trash2, LayoutGrid, List, ChevronDown, ChevronUp, ChartColumn, Filter, Settings, ChevronRight, Ellipsis, Check } from 'lucide-vue-next'
import { useLicensedCatalog } from '@/stores/licensedCatalog'
import { detailRoute, playbackRoute, savedLocalMediaIssue } from '@/router/libraryRoutes'
import { useLibraryStore } from '@/stores/library'
import { useToastStore } from '@/stores/toast'
import { watchStatus, historyDay, agoText, watchHours } from '@/composables'
import { BaseButton, EmptyState, ProgressBar, Modal } from '@/components/ui'
import type { Drama, HistoryEntry } from '@/types'

const router = useRouter(); const lib = useLibraryStore(); const toast = useToastStore()
const { savedCard } = useLicensedCatalog()
const now = ref(Date.now())
let midnightTimer: number | undefined
function refreshCalendar() {
  now.value = Date.now()
  window.clearTimeout(midnightTimer)
  const midnight = new Date(now.value)
  midnight.setHours(24, 0, 0, 0)
  midnightTimer = window.setTimeout(refreshCalendar, midnight.getTime() - now.value)
}
onMounted(() => {
  refreshCalendar()
  window.addEventListener('focus', refreshCalendar)
  document.addEventListener('visibilitychange', refreshCalendar)
})
onUnmounted(() => {
  window.clearTimeout(midnightTimer)
  window.removeEventListener('focus', refreshCalendar)
  document.removeEventListener('visibilitychange', refreshCalendar)
})

type Row = ReturnType<typeof historyDay> & {
  h: HistoryEntry
  d: Drama
  status: ReturnType<typeof watchStatus>
  issue: ReturnType<typeof savedLocalMediaIssue>
}
const rows = computed<Row[]>(() => [...lib.history].sort((a, b) => b.watchedAt - a.watchedAt).flatMap((h) => {
  const d = savedCard(h.dramaId)
  if (!d) return []
  return [{
    h, d, ...historyDay(h.watchedAt, now.value),
    status: watchStatus(h, d), issue: savedLocalMediaIssue(d, h.episode),
  }]
}))
const status = ref<'all'|'watching'|'done'>('all'); const genre = ref('全部'); const when = ref('全部'); const view = ref<'grid'|'list'>('grid'); const sort = ref<'time'|'progress'>('time')
const batch = ref(false); const picked = ref<Set<string>>(new Set()); const confirmClear = ref(false); const confirmBatch = ref(false); const menu = ref(''); const fopen = ref(true)
const GENRES = ['全部', '甜宠', '逆袭', '都市', '古装', '玄幻', '悬疑', '其他']; const WHEN = ['全部', '今天', '昨天', '近7天', '近30天', '更早']
const filtered = computed(() => rows.value.filter((r) => (status.value === 'all' || status.value === r.status)
  && (genre.value === '全部' || (genre.value === '其他' ? !GENRES.slice(1, 7).some((g) => r.d.tags.includes(g)) : r.d.tags.includes(genre.value)))
  && (when.value === '全部' || (when.value === '今天' && r.bucket === 'today') || (when.value === '昨天' && r.bucket === 'yesterday') || (when.value === '近7天' && r.age < 7) || (when.value === '近30天' && r.age < 30) || (when.value === '更早' && r.bucket === 'earlier'))))
const ordered = computed(() => (sort.value === 'progress' ? [...filtered.value].sort((a, b) => b.h.percent - a.h.percent) : filtered.value))
const groups = computed(() => [{ k: 'today', l: '今天' }, { k: 'yesterday', l: '昨天' }, { k: 'earlier', l: '更早' }].map((g) => ({ ...g, items: ordered.value.filter((r) => r.bucket === g.k) })).filter((g) => g.items.length))
const stat = computed(() => ({ total: rows.value.length, watching: rows.value.filter((r) => r.status === 'watching').length, done: rows.value.filter((r) => r.status === 'done').length, hours: watchHours(lib.history) }))
const latest = computed(() => {
  const playable = rows.value.filter((r) => !r.issue)
  return playable.find((r) => r.status === 'watching' && r.h.percent < 100)
    ?? playable.find((r) => r.status === 'watching')
    ?? playable[0]
})
const detailTarget = (r: Row) => r.issue === 'library' ? { name: 'licensed' } : detailRoute(r.d.id)
const resume = (r: Row) => router.push(r.issue ? detailTarget(r) : playbackRoute(r.d.id, r.h.episode))
function toggleBatch() { batch.value = !batch.value; picked.value = new Set() }
function pick(id: string) { const s = new Set(picked.value); s.has(id) ? s.delete(id) : s.add(id); picked.value = s }
function delOne(id: string) { lib.removeHistory([id]); menu.value = ''; toast.push('已删除该记录', 'success') }
function delBatch() { const ids = [...picked.value]; lib.removeHistory(ids); picked.value = new Set(); confirmBatch.value = false; batch.value = false; toast.push(`已删除 ${ids.length} 条记录`, 'success') }
function clearAll() { lib.clearHistory(); confirmClear.value = false; batch.value = false; toast.push('观看历史已清空', 'success') }
const rowClick = (r: Row) => (batch.value ? pick(r.d.id) : resume(r))
</script>
<template>
  <div class="his" data-testid="page-history">
    <header class="hero"><Clock :size="44" :stroke-width="1.6" /><div><h1>观看历史</h1><p>记录你的每一次追剧，随时继续精彩</p></div>
      <div class="ha"><BaseButton size="lg" data-testid="continue-latest" :disabled="!latest" @click="latest && resume(latest)"><Play :size="16" fill="currentColor" />继续观看</BaseButton>
        <BaseButton size="lg" variant="ghost" data-testid="batch-toggle" :disabled="!rows.length" @click="toggleBatch"><Trash2 :size="16" />{{ batch ? '取消批量' : '批量删除' }}</BaseButton>
        <BaseButton size="lg" variant="ghost" data-testid="clear-all" :disabled="!rows.length" @click="confirmClear = true"><Trash2 :size="16" />清空历史</BaseButton></div></header>
    <div class="cols"><section class="main">
      <div class="tabs"><div class="seg"><button :class="{ on: status === 'all' }" data-testid="htab-all" @click="status = 'all'">全部历史 ({{ stat.total }})</button><button :class="{ on: status === 'watching' }" data-testid="htab-watching" @click="status = 'watching'">在追 ({{ stat.watching }})</button><button :class="{ on: status === 'done' }" data-testid="htab-done" @click="status = 'done'">已看完 ({{ stat.done }})</button></div>
        <span class="sp" /><button class="sort" data-testid="sort" @click="sort = sort === 'time' ? 'progress' : 'time'">{{ sort === 'time' ? '按观看时间' : '按观看进度' }}<ChevronDown :size="14" /></button>
        <div class="vt"><button :class="{ on: view === 'grid' }" aria-label="网格" data-testid="view-grid" @click="view = 'grid'"><LayoutGrid :size="17" /></button><button :class="{ on: view === 'list' }" aria-label="列表" data-testid="view-list" @click="view = 'list'"><List :size="17" /></button></div></div>
      <EmptyState v-if="!groups.length" :title="rows.length ? '没有符合条件的记录' : '还没有观看记录'" :hint="rows.length ? '调整右侧筛选条件试试' : '去看一部短剧吧，记录会自动出现在这里'"><BaseButton variant="outline" @click="router.push('/home')">去推荐页</BaseButton></EmptyState>
      <section v-for="g in groups" :key="g.k" class="grp" :data-testid="`group-${g.k}`"><h2><i />{{ g.l }}<small>（{{ g.items.length }}部）</small></h2>
        <ul :class="[view === 'list' || g.k === 'earlier' ? 'rows' : 'cards', g.k]">
          <li v-for="r in g.items" :key="r.d.id" class="it" :class="{ sel: picked.has(r.d.id) }" data-testid="history-item">
            <button v-if="batch || g.k === 'earlier'" class="cb" :class="{ on: picked.has(r.d.id) }" role="checkbox" :aria-checked="picked.has(r.d.id)" :aria-label="`选择${r.d.title}`" data-testid="hcheck" @click="pick(r.d.id)"><Check v-if="picked.has(r.d.id)" :size="12" /></button>
            <img :src="r.d.cover" :alt="r.d.title" @click="rowClick(r)" />
            <div class="b"><h3 class="clamp-2">{{ r.d.title }}</h3><p class="tg">{{ r.issue === 'library' ? '本地媒体不可用' : r.issue === 'episode' ? '该分集不可用' : r.status === 'unknown' ? '完成状态未知' : r.d.tags.slice(0, 2).join(' · ') }}<span v-if="r.issue && r.status === 'unknown'"> · 完成状态未知</span><span v-else-if="r.issue && r.status === 'done'"> · 已看完</span></p><p class="ep">第{{ r.h.episode }}集<template v-if="r.h.episodeCountAtWatch && r.h.episodeCountAtWatch !== r.d.episodeCount"> / 当时共{{ r.h.episodeCountAtWatch }}集</template><template v-else-if="!r.issue"> / 共{{ r.d.episodeCount }}集</template></p>
              <div class="pg"><ProgressBar :value="r.h.percent" /><i>{{ r.h.percent }}%</i></div></div>
            <div class="ac"><span v-if="g.k === 'earlier'" class="ago">{{ agoText(r.age) }}</span>
              <button v-if="r.issue" class="done" @click="resume(r)">{{ r.issue === 'library' ? '查看片库' : '选集' }}</button>
              <button v-else-if="r.status === 'done'" class="done" data-testid="done-badge" @click="resume(r)">已看完<Check :size="12" /></button><button v-else class="go" data-testid="resume" @click="resume(r)"><Play :size="11" fill="currentColor" />继续观看</button>
              <div class="mw"><button aria-label="更多" data-testid="h-more" @click="menu = menu === r.d.id ? '' : r.d.id"><Ellipsis :size="18" /></button><ul v-if="menu === r.d.id" class="mn"><li><button data-testid="h-delete" @click="delOne(r.d.id)">删除此记录</button></li><li><button @click="router.push(detailTarget(r))">{{ r.issue === 'library' ? '查看片库' : '查看详情' }}</button></li></ul></div></div>
          </li></ul></section>
      <div v-if="batch" class="batchbar" data-testid="batchbar"><span>已选 {{ picked.size }} 条</span><BaseButton :disabled="!picked.size" data-testid="batch-delete" @click="confirmBatch = true">删除选中</BaseButton><BaseButton variant="ghost" @click="toggleBatch">取消</BaseButton></div>
    </section>
    <aside class="side">
      <section class="pn"><h3><ChartColumn :size="20" color="#ff6a4d" />观看历史概览</h3><div class="nums"><div><b data-testid="stat-total">{{ stat.total }}</b><span>部记录</span></div><div><b data-testid="stat-watching">{{ stat.watching }}</b><span>部在追</span></div><div><b data-testid="stat-done">{{ stat.done }}</b><span>部已看完</span></div></div><div class="hrs"><Clock :size="18" />累计观看 <b>{{ stat.hours }}</b> 小时<ChevronRight :size="15" /></div></section>
      <section class="pn"><h3 class="fh" @click="fopen = !fopen"><Filter :size="20" />筛选历史<component :is="fopen ? ChevronUp : ChevronDown" :size="16" /></h3>
        <template v-if="fopen"><h4>观看状态</h4><div class="ch"><button v-for="s in [['all','全部'],['watching','在追'],['done','已看完']] as const" :key="s[0]" :class="{ on: status === s[0] }" :data-testid="`fs-${s[0]}`" @click="status = s[0]">{{ s[1] }}</button></div>
          <h4>题材类型</h4><div class="ch"><button v-for="g in GENRES" :key="g" :class="{ on: genre === g }" :data-testid="`fg-${g}`" @click="genre = g">{{ g }}</button></div>
          <h4>观看时间</h4><div class="ch"><button v-for="w in WHEN" :key="w" :class="{ on: when === w }" :data-testid="`fw-${w}`" @click="when = w">{{ w }}</button></div></template></section>
      <section class="pn"><h3><Settings :size="20" />历史管理</h3><button class="mrow" @click="toggleBatch"><Trash2 :size="16" />批量删除<ChevronRight :size="15" /></button><button class="mrow" @click="confirmClear = true"><Trash2 :size="16" /><span>清空历史<small>（将删除全部观看记录）</small></span><ChevronRight :size="15" /></button><p class="note">观看历史仅保存在本机；云端同步需官方授权，当前未启用。</p></section>
    </aside></div>
    <Modal :open="confirmClear" title="清空观看历史" confirm-text="确认清空" @cancel="confirmClear = false" @confirm="clearAll">将删除全部 {{ rows.length }} 条观看记录。此操作无法撤销，确定继续吗？</Modal>
    <Modal :open="confirmBatch" title="删除所选记录" confirm-text="确认删除" @cancel="confirmBatch = false" @confirm="delBatch">确定删除已选的 {{ picked.size }} 条观看记录吗？</Modal>
  </div>
</template>
<style scoped lang="scss">
.his { padding: 8px 16px 20px 14px; }
.hero { display: flex; align-items: center; gap: 18px; height: 100px; padding: 0 28px 0 24px; border-radius: 24px; background: linear-gradient(90deg, rgba(120,32,28,.35), rgba(60,20,22,.25)); border: 1px solid var(--c-border); h1 { font-size: 32px; } p { margin-top: 6px; font-size: 14px; color: var(--c-text-2); } .ha { margin-left: auto; display: flex; gap: 14px; } }
.cols { display: flex; gap: 22px; margin-top: 16px; } .main { flex: 1; min-width: 0; position: relative; } .side { width: 294px; flex: none; display: flex; flex-direction: column; gap: 14px; }
.tabs { display: flex; align-items: center; gap: 16px; margin-bottom: 8px; .sp { flex: 1; } .seg { display: flex; padding: 3px; border-radius: 22px; background: var(--c-surface); border: 1px solid var(--c-border); button { height: 38px; padding: 0 24px; border-radius: 19px; font-size: 16px; color: var(--c-text-2); &.on { background: var(--g-accent); color: #fff; } } } .sort { display: inline-flex; align-items: center; gap: 4px; font-size: 15px; } .vt { display: flex; padding: 3px; border-radius: 10px; background: var(--c-surface); border: 1px solid var(--c-border); button { width: 38px; height: 30px; display: grid; place-items: center; border-radius: 8px; color: var(--c-text-2); &.on { background: rgba(255,255,255,.14); color: var(--c-text); } } } }
.grp { margin-top: 8px; h2 { display: flex; align-items: center; gap: 12px; font-size: 20px; margin: 10px 0; i { width: 11px; height: 11px; border-radius: 50%; border: 2px solid var(--c-accent); } small { font-size: 15px; color: var(--c-text-2); font-weight: 400; } } }
.cards { display: grid; gap: 12px; &.today { grid-template-columns: repeat(3, 1fr); } &.yesterday { grid-template-columns: repeat(4, 1fr); } }
.rows { display: flex; flex-direction: column; gap: 4px; padding: 4px 12px; border-radius: 14px; background: rgba(255,255,255,.04); border: 1px solid var(--c-border); }
.it { position: relative; border-radius: 14px; background: linear-gradient(180deg, rgba(255,255,255,.07), rgba(255,255,255,.035)); border: 1px solid var(--c-border); &.sel { border-color: var(--c-accent); } }
.cards .it { display: grid; grid-template-columns: 90px 1fr; grid-template-rows: auto auto; gap: 4px 12px; padding: 10px 12px; min-height: 178px; img { grid-row: 1 / 2; width: 90px; height: 130px; object-fit: cover; border-radius: 8px; cursor: pointer; } .ac { grid-column: 1 / -1; display: flex; align-items: center; justify-content: space-between; } .cb { position: absolute; right: 10px; top: 10px; z-index: 2; } }
.cards.yesterday .it { grid-template-columns: 74px 1fr; img { width: 74px; height: 110px; } min-height: 160px; }
.rows .it { display: grid; grid-template-columns: 24px 56px minmax(0, 1fr) 215px; align-items: center; gap: 12px; padding: 6px 4px; border: 0; background: none; border-bottom: 1px solid var(--c-border); &:last-child { border-bottom: 0; } img { width: 56px; height: 62px; object-fit: cover; border-radius: 6px; cursor: pointer; } .ac { display: flex; align-items: center; justify-content: flex-end; gap: 10px; } }
.rows .b { display: grid; grid-template-columns: minmax(0, 1fr) 230px; column-gap: 16px; align-items: center; h3 { grid-column: 1; grid-row: 1; } .tg { grid-column: 1; grid-row: 2; margin: 2px 0 0; } .ep { grid-column: 2; grid-row: 1; margin: 0; } .pg { grid-column: 2; grid-row: 2; margin: 2px 0 0; } }
.b { min-width: 0; h3 { font-size: 16px; line-height: 1.3; } .tg { font-size: 12.5px; color: var(--c-text-2); margin-top: 4px; } .ep { font-size: 13px; margin-top: 8px; color: var(--c-text-2); } .pg { display: flex; align-items: center; gap: 8px; margin-top: 8px; :deep(.bar) { flex: 1; } i { font-style: normal; font-size: 12.5px; color: var(--c-text-2); width: 34px; text-align: right; } } }
.go, .done { white-space: nowrap; flex: none; display: inline-flex; align-items: center; gap: 5px; height: 28px; padding: 0 14px; border-radius: 14px; font-size: 13px; font-weight: 600; } .go { background: var(--g-accent); } .done { background: rgba(255,255,255,.1); border: 1px solid var(--c-border-strong); } .ago { white-space: nowrap; font-size: 12.5px; color: var(--c-text-2); margin-left: auto; }
.cb { width: 20px; height: 20px; border-radius: 6px; border: 2px solid rgba(255,255,255,.55); display: grid; place-items: center; background: rgba(0,0,0,.2); &.on { background: var(--c-accent); border-color: var(--c-accent); } }
.mw { position: relative; } .mn { position: absolute; right: 0; top: 26px; z-index: 6; min-width: 120px; padding: 6px; border-radius: 10px; background: #2a1719; border: 1px solid var(--c-border-strong); button { width: 100%; height: 32px; border-radius: 6px; &:hover { background: var(--c-surface-hover); } } }
.batchbar { position: sticky; bottom: 6px; display: flex; align-items: center; gap: 14px; margin-top: 14px; padding: 10px 18px; border-radius: 16px; background: rgba(40,22,24,.96); border: 1px solid var(--c-border-strong); span { margin-right: auto; color: var(--c-text-2); } }
.pn { padding: 14px 16px; border-radius: 18px; background: linear-gradient(180deg, rgba(255,255,255,.07), rgba(255,255,255,.035)); border: 1px solid var(--c-border); h3 { display: flex; align-items: center; gap: 10px; font-size: 18px; &.fh { cursor: pointer; svg:last-child { margin-left: auto; } } } h4 { font-size: 14px; margin: 14px 0 8px; &::before { content: '•'; color: var(--c-accent); margin-right: 8px; } } }
.nums { display: grid; grid-template-columns: repeat(3, 1fr); margin: 12px 0; text-align: center; b { display: block; font-size: 28px; } span { font-size: 12.5px; color: var(--c-text-2); } }
.hrs { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-radius: 12px; background: var(--c-surface); font-size: 14px; b { color: var(--c-accent-2); font-size: 20px; } svg:last-child { margin-left: auto; } }
.ch { display: flex; flex-wrap: wrap; gap: 8px; button { height: 32px; min-width: 62px; padding: 0 12px; border-radius: 16px; background: rgba(255,255,255,.08); border: 1px solid var(--c-border); font-size: 13.5px; &.on { background: rgba(238,75,57,.3); border-color: var(--c-accent); } } }
.mrow { display: flex; align-items: center; gap: 12px; width: 100%; height: 46px; border-top: 1px solid var(--c-border); margin-top: 8px; font-size: 15px; text-align: left; span { flex: 1; small { display: block; font-size: 12px; color: var(--c-text-2); } } svg:last-child { margin-left: auto; color: var(--c-text-2); } }
.note { margin-top: 10px; font-size: 12px; color: var(--c-text-3); }
</style>
