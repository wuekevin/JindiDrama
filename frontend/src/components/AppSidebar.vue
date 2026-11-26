<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { House, Flame, Cat, Video, Bot, ChartNoAxesColumn, Heart, Clock, Settings } from 'lucide-vue-next'
import BrandLogo from './BrandLogo.vue'
import { activeProviderName } from '@/bindings'
import { useLicensedCatalog } from '@/stores/licensedCatalog'

const route = useRoute(); const router = useRouter()
const catalog = useLicensedCatalog()
const main = [
  { key: 'home', label: '推荐', icon: House, to: '/home' },
  { key: 'hot', label: '热门', icon: Flame, to: '/search?sort=hot' },
  { key: 'manju', label: '漫剧', icon: Cat, to: '/search?kind=漫剧' },
  { key: 'real', label: '真人剧', icon: Video, to: '/search?kind=真人剧' },
  { key: 'ai', label: 'AI剧', icon: Bot, to: '/search?kind=AI剧' },
  { key: 'rank', label: '排行榜', icon: ChartNoAxesColumn, to: '/home#rank' },
]
const lib = [
  { key: 'favorites', label: '收藏', icon: Heart, to: '/favorites' },
  { key: 'history', label: '历史', icon: Clock, to: '/history' },
  { key: 'settings', label: '设置', icon: Settings, to: '/settings' },
]
const active = computed(() => String(route.meta.nav ?? ''))
const cmsCatalog = ref(false)
onMounted(() => {
  void activeProviderName()?.then((name) => { cmsCatalog.value = name === 'apple-cms' }, () => {})
})
const visibleMain = computed(() => {
  if (cmsCatalog.value) return main.filter(({ key }) => key === 'home')
  return catalog.demoCatalog ? main : main.filter(({ key }) => key === 'home' || key === 'rank')
})
function go(to: string) { router.push(to) }
</script>
<template>
  <aside class="sidebar on-dark" aria-label="主导航">
    <div class="logo"><BrandLogo /></div>
    <nav>
      <ul class="group">
        <li v-for="n in visibleMain" :key="n.key">
          <button class="item" :class="{ active: active === 'home' && n.key === 'home' }" :aria-current="active === 'home' && n.key === 'home' ? 'page' : undefined" :data-testid="`nav-${n.key}`" @click="go(n.to)">
            <component :is="n.icon" :size="24" :stroke-width="1.7" /><span>{{ n.label }}</span>
          </button>
        </li>
      </ul>
      <hr />
      <ul class="group">
        <li v-for="n in lib" :key="n.key">
          <button class="item" :class="{ active: active === n.key }" :aria-current="active === n.key ? 'page' : undefined" :data-testid="`nav-${n.key}`" @click="go(n.to)">
            <component :is="n.icon" :size="24" :stroke-width="1.7" /><span>{{ n.label }}</span>
          </button>
        </li>
      </ul>
    </nav>
    <svg class="deco" viewBox="0 0 200 220" aria-hidden="true">
      <defs>
        <linearGradient id="dc" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ff6a4d" stop-opacity=".75"/><stop offset="1" stop-color="#7c1c15" stop-opacity=".35"/></linearGradient>
        <radialGradient id="dg" cx=".3" cy=".9" r=".9"><stop offset="0" stop-color="#e5402f" stop-opacity=".45"/><stop offset="1" stop-color="#e5402f" stop-opacity="0"/></radialGradient>
      </defs>
      <rect width="200" height="220" fill="url(#dg)"/>
      <g transform="rotate(-14 90 150)"><rect x="40" y="132" width="110" height="70" rx="10" fill="url(#dc)"/><rect x="34" y="108" width="116" height="24" rx="6" fill="url(#dc)"/>
        <path d="M44 108 l16 24 M70 108 l16 24 M96 108 l16 24 M122 108 l16 24" stroke="#1a0c0d" stroke-width="7" opacity=".55"/>
        <path d="M82 150 L112 168 L82 186 Z" fill="#ffd0c4" opacity=".7"/></g>
    </svg>
  </aside>
</template>
<style scoped lang="scss">
.sidebar { position: relative; width: var(--sidebar-w); flex: none; background: linear-gradient(180deg, #1c1011 0%, #170c0d 55%, #2a100e 100%); display: flex; flex-direction: column; overflow: hidden; }
.logo { flex: none; padding: 52px 0 0 26px; height: 142px; }
nav { min-height: 0; overflow-y: auto; padding: 0 8px; }
.group { display: flex; flex-direction: column; gap: 5px; }
hr { border: 0; border-top: 1px solid var(--c-border); margin: 20px 14px; }
.item { width: 100%; height: 52px; display: flex; align-items: center; gap: 14px; padding-left: 22px; border-radius: 14px; font-size: 17px; font-weight: 500; color: var(--c-text); transition: background 0.18s var(--ease);
  &:focus-visible { outline-offset: -4px; }
  &:hover { background: var(--c-surface-hover); }
  &.active { background: var(--g-nav-active); box-shadow: inset 0 0 0 1px rgba(255, 140, 110, 0.25); font-weight: 700; }
  svg { opacity: 0.92; flex: none; }
}
.deco { flex: 1 0 0px; min-height: 0; max-height: 220px; margin-top: auto; width: 200px; pointer-events: none; }
</style>
