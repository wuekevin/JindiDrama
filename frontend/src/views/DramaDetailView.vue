<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft, ChevronDown, ChevronUp, ChevronRight, Star, CirclePlay, Heart, Play, Plus, Check, Share2, Flame, Crown, RefreshCw, Landmark, Swords, Sprout, Sparkles } from 'lucide-vue-next'
import { getDetail, groupEpisodes, epHot, type DramaDetail } from '@/mocks/provider'
import { DRAMAS } from '@/mocks/data'
import { useLibraryStore } from '@/stores/library'
import { useToastStore } from '@/stores/toast'
import { BaseButton, ErrorState, EmptyState } from '@/components/ui'
import Skeleton from '@/components/ui/Skeleton.vue'

const route = useRoute(); const router = useRouter(); const lib = useLibraryStore(); const toast = useToastStore()
const id = computed(() => String(route.params.id))
const state = ref<'loading'|'ready'|'notfound'|'error'>('loading'); const err = ref('')
const detail = ref<DramaDetail | null>(null)
const tab = ref<'eps'|'clips'>('eps'); const group = ref(0); const expanded = ref(false); const season = ref(0); const rel = ref(0)
let token = 0
async function load() {
  const my = ++token; state.value = 'loading'
  if (route.query.state === 'loading') return
  try { const r = await getDetail(id.value, route.query.state === 'error' ? 'error' : 'normal'); if (my !== token) return; detail.value = r; state.value = r ? 'ready' : 'notfound'; group.value = 0; expanded.value = false
    const h = lib.history.find((x) => x.dramaId === id.value); if (h) group.value = Math.floor((h.episode - 1) / 20) } catch (e) { if (my !== token) return; err.value = (e as Error).message; state.value = 'error' }
}
watch(id, load, { immediate: true })
const groups = computed(() => (detail.value ? groupEpisodes(detail.value.drama.episodeCount) : []))
const eps = computed(() => { const g = groups.value[group.value]; return g ? Array.from({ length: g.to - g.from + 1 }, (_, i) => g.from + i) : [] })
const hist = computed(() => lib.history.find((h) => h.dramaId === id.value))
const watchEp = (ep: number) => { if (detail.value && ep > detail.value.drama.updatedTo) { toast.push(`第${ep}集尚未更新`, 'error'); return } router.push(`/watch/${id.value}/${ep}`) }
const progressOf = (ep: number) => (hist.value && hist.value.episode === ep ? hist.value.percent : ep < (hist.value?.episode ?? 0) ? 100 : 0)
function fav() { toast.push(lib.toggleFav(id.value) ? '已加入收藏' : '已取消收藏', 'success') }
async function share() { const link = `jindi://drama/${id.value}`; try { await navigator.clipboard.writeText(link); toast.push('分享链接已复制', 'success') } catch { toast.push(`分享链接：${link}`) } }
const tagIcon = (t: string) => ({ 古装: Landmark, 甜宠: Heart, 重生: Sprout, 权谋: Swords, 逆袭: Crown }[t] ?? Sparkles)
const shuffle = () => { rel.value = (rel.value + 1) % 3 }
const guess = computed(() => { const all = DRAMAS.filter((d) => d.id !== id.value); const o = rel.value * 2; return all.slice(o, o + 6) })
</script>
<template>
  <div class="detail" data-testid="page-detail">
    <button class="back" data-testid="back" @click="router.back()"><ChevronLeft :size="18" />返回</button>
    <div v-if="state === 'loading'" class="sk"><Skeleton style="width: 286px; height: 415px" /><Skeleton style="flex: 1; height: 415px" /></div>
    <ErrorState v-else-if="state === 'error'" :message="err" @retry="load" />
    <EmptyState v-else-if="state === 'notfound'" title="没有找到这部短剧"><BaseButton variant="outline" @click="router.push('/home')">回到首页</BaseButton></EmptyState>
    <template v-else-if="detail">
      <section class="top">
        <div class="poster"><img :src="detail.drama.cover" :alt="detail.drama.title" /><span class="badge"><Crown :size="13" fill="currentColor" />独播</span><b class="pt">{{ detail.drama.title }}</b></div>
        <div class="meta">
          <h1>{{ detail.drama.title }}<span class="heat"><Flame :size="20" color="#ff6a3d" fill="#ff6a3d" />{{ detail.drama.heat.toFixed(1) }}万热度</span></h1>
          <div class="stats"><span class="score"><Star :size="19" color="#f4b740" fill="#f4b740" /><b>{{ detail.rating }}</b> 分</span><i /><span><CirclePlay :size="17" />{{ detail.plays }}</span><i /><span><Heart :size="17" />{{ detail.favs }}</span></div>
          <div class="tags"><span v-for="t in detail.drama.tags" :key="t" class="tg"><component :is="tagIcon(t)" :size="15" />{{ t }}</span></div>
          <div class="sum"><h4>剧情简介</h4><p :class="{ clamp: !expanded }" data-testid="summary">{{ expanded ? detail.long : detail.drama.summary }}</p><button class="exp" data-testid="expand" @click="expanded = !expanded">{{ expanded ? '收起' : '展开' }}<component :is="expanded ? ChevronUp : ChevronDown" :size="14" /></button></div>
          <div class="cast"><h4>主演</h4><button v-for="c in detail.cast" :key="c.name" class="cm" @click="toast.push(`${c.name} 主页暂未开放`)"><i :style="{ background: `hsl(${c.hue} 40% 45%)` }">{{ c.name[0] }}</i><span><b>{{ c.name }}</b><em>饰 {{ c.role }}</em></span><ChevronRight :size="14" /></button></div>
          <p class="status"><b>更新状态</b> {{ detail.schedule }} <u>|</u> 已更新至 第{{ detail.drama.updatedTo }}集（共{{ detail.drama.episodeCount }}集） <u>|</u> 每日20:00更新2集</p>
          <div class="cta">
            <BaseButton size="lg" data-testid="watch-now" @click="watchEp(1)"><Play :size="17" fill="currentColor" />立即观看</BaseButton>
            <button v-if="hist" class="cont" data-testid="continue" @click="watchEp(hist.episode)"><CirclePlay :size="30" /><span>继续观看<em>第{{ hist.episode }}集 · {{ hist.percent }}%</em></span></button>
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
            <div class="grid"><button v-for="ep in eps" :key="ep" class="ep" :class="{ cur: hist?.episode === ep, lock: ep > detail.drama.updatedTo }" data-testid="ep" :aria-label="`第${ep}集`" @click="watchEp(ep)">
              <Play v-if="ep === 1 && !hist" :size="14" fill="currentColor" /><template v-else><Flame v-if="epHot(ep)" :size="13" color="#ff6a3d" fill="#ff6a3d" />{{ ep }}</template>
              <i v-if="progressOf(ep) > 0 && progressOf(ep) < 100" class="pg" :style="{ width: progressOf(ep) + '%' }" /></button></div>
          </template>
          <ul v-else class="clips"><li v-for="c in detail.highlights" :key="c.title"><button data-testid="clip" @click="toast.push('精彩片段为演示数据')"><Play :size="16" />{{ c.title }}<em>{{ c.dur }}</em></button></li></ul>
        </div>
        <div class="panel rel"><header><h3>相关剧集</h3></header>
          <button v-for="(s, i) in detail.seasons" :key="i" class="sea" :class="{ on: season === i }" data-testid="season" @click="season = i">
            <img :src="detail.drama.cover" :alt="s.title" /><span><b class="clamp-2">{{ s.title }}</b><em>共{{ s.episodes }}集 · {{ s.finished ? '已完结' : '连载中' }} <Flame :size="12" color="#ff6a3d" fill="#ff6a3d" /> {{ s.heat.toFixed(1) }}万热度</em></span>
            <i v-if="season === i" class="ck"><Check :size="14" /></i><i v-else class="pl"><Play :size="13" fill="currentColor" /></i></button></div>
      </section>

      <section class="guess"><header><h2><Flame :size="24" color="#ff6a3d" fill="#ff6a3d" />猜你喜欢</h2><button data-testid="shuffle" @click="shuffle">换一换<RefreshCw :size="14" /></button></header>
        <div class="row"><button v-for="d in guess" :key="d.id" class="gc" data-testid="guess-card" @click="router.push(`/drama/${d.id}`)"><img :src="d.cover" :alt="d.title" /><span class="bd">同类型</span><b class="clamp-1">{{ d.title }}</b><em><Flame :size="12" color="#ff6a3d" fill="#ff6a3d" />{{ d.heat.toFixed(1) }}万热度</em></button></div></section>
    </template>
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
.rel { width: 390px; flex: none; header h3 { font-size: 20px; margin-bottom: 8px; } }
.sea { position: relative; display: flex; gap: 12px; align-items: center; width: 100%; padding: 8px 40px 8px 8px; margin-bottom: 8px; border-radius: 12px; border: 1px solid transparent; text-align: left; &.on { background: rgba(238,75,57,.18); border-color: var(--c-accent); } img { width: 86px; height: 62px; object-fit: cover; border-radius: 8px; } b { display: block; font-size: 14.5px; line-height: 1.3; } em { display: block; margin-top: 6px; font-style: normal; font-size: 12px; color: var(--c-text-2); } .ck, .pl { position: absolute; right: 12px; top: 50%; transform: translateY(-50%); width: 28px; height: 28px; border-radius: 50%; display: grid; place-items: center; } .ck { background: rgba(238,75,57,.35); color: #ff9a86; } .pl { background: rgba(255,255,255,.16); } }
.guess { margin-top: 18px; padding-left: 8px; header { display: flex; justify-content: space-between; align-items: center; h2 { display: flex; align-items: center; gap: 6px; font-size: 22px; } button { display: inline-flex; align-items: center; gap: 4px; font-size: 14px; } } .row { display: grid; grid-template-columns: repeat(6, 1fr); gap: 14px; margin-top: 12px; } }
.gc { position: relative; height: 198px; border-radius: 14px; overflow: hidden; text-align: left; padding: 0; img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; } &::after { content: ''; position: absolute; inset: 50% 0 0; background: linear-gradient(transparent, rgba(0,0,0,.85)); } .bd { position: absolute; left: 0; top: 0; z-index: 1; padding: 2px 8px; border-radius: 0 0 8px 0; background: var(--g-accent-h); font-size: 11px; } b, em { position: absolute; left: 12px; right: 8px; z-index: 1; } b { bottom: 30px; font-size: 16px; } em { bottom: 10px; font-style: normal; font-size: 12px; color: var(--c-text-2); display: flex; align-items: center; gap: 3px; } }
</style>
