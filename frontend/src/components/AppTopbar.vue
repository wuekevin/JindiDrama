<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, Bell, Minus, Square, X, CornerUpLeft } from 'lucide-vue-next'

const route = useRoute(); const router = useRouter()
const q = ref(''); const focused = ref(false)
const SUGGEST = ['替嫁', '替嫁新娘', '替嫁千金', '替嫁后总裁狠狠爱', '替嫁王妃', '替嫁成了他的白月光']
watch(() => route.query.q, (v) => { if (route.name === 'search') q.value = String(v ?? '') }, { immediate: true })
const suggestions = () => (q.value.trim() ? SUGGEST.filter((s) => s.includes(q.value.trim()[0])) : [])
const onBlur = () => { window.setTimeout(() => (focused.value = false), 150) }
function submit(text = q.value) { const t = text.trim(); if (!t) return; q.value = t; focused.value = false; router.push({ name: 'search', query: { q: t } }) }
const tabs = [{ k: 'discover', label: '发现', to: '/home' }, { k: 'library', label: '片库', to: '/search' }, { k: 'category', label: '分类', to: '/home#categories' }]
const tabActive = (k: string) => (k === 'discover' ? route.name !== 'search' : k === 'library' && route.name === 'search')
const win = (op: 'min'|'max'|'close') => { const rt = (window as any).runtime; if (!rt) return; ({ min: rt.WindowMinimise, max: rt.WindowToggleMaximise, close: rt.Quit })[op]?.() }
</script>
<template>
  <header class="topbar" style="--wails-draggable: drag">
    <nav class="tabs" aria-label="一级栏目">
      <button v-for="t in tabs" :key="t.k" class="tab" :class="{ on: tabActive(t.k) }" :data-testid="`tab-${t.k}`" @click="router.push(t.to)">{{ t.label }}</button>
    </nav>
    <div class="search" :class="{ focus: focused }" role="search">
      <Search :size="20" />
      <input v-model="q" data-testid="search-input" placeholder="搜索短剧、主角或题材，如 “契约婚姻”" aria-label="搜索" @focus="focused = true" @blur="onBlur" @keydown.enter="submit()" @keydown.esc="focused = false" />
      <button v-if="q" class="clear" aria-label="清除" @click="q = ''"><X :size="16" /></button>
      <ul v-if="focused && suggestions().length" class="suggest" role="listbox">
        <li v-for="(s, i) in suggestions()" :key="s" role="option" :class="{ hl: i === 0 }" @mousedown.prevent="submit(s)"><Search :size="15" /><span>{{ s }}</span><CornerUpLeft :size="14" class="fill" /></li>
      </ul>
    </div>
    <div class="right">
      <button class="icon bell" aria-label="通知"><Bell :size="22" /><i class="dot" /></button>
      <button class="avatar" aria-label="账号" data-testid="avatar" @click="router.push('/login')"><svg viewBox="0 0 40 40"><circle cx="20" cy="15" r="7" fill="#c9bfd0"/><path d="M5 40c2-11 9-14 15-14s13 3 15 14z" fill="#9a8fa6"/></svg></button>
      <div class="win">
        <button aria-label="最小化" @click="win('min')"><Minus :size="18" /></button>
        <button aria-label="最大化" @click="win('max')"><Square :size="15" /></button>
        <button aria-label="关闭" @click="win('close')"><X :size="20" /></button>
      </div>
    </div>
  </header>
</template>
<style scoped lang="scss">
.topbar { position: relative; height: var(--topbar-h); flex: none; display: flex; align-items: center; padding: 0 12px 0 40px; z-index: 20; }
.tabs { display: flex; gap: 0; }
.tab { position: relative; padding: 0 31px; height: 40px; font-size: 19px; font-weight: 600; color: var(--c-text-2);
  &.on { color: var(--c-text); &::after { content: ''; position: absolute; left: 50%; bottom: 0; width: 56px; height: 3px; border-radius: 3px; transform: translateX(-50%); background: var(--c-accent); } }
  &:hover { color: var(--c-text); } }
.search { position: absolute; left: 499px; width: 440px; height: 40px; display: flex; align-items: center; gap: 10px; padding: 0 14px; border-radius: 20px; background: rgba(255, 255, 255, 0.06); border: 1px solid var(--c-border); color: var(--c-text-2);
  &.focus { border-color: var(--c-accent); box-shadow: 0 0 0 2px rgba(238, 75, 57, 0.25); background: rgba(40, 18, 18, 0.9); }
  input { flex: 1; background: none; border: 0; outline: 0; font-size: 14.5px; color: var(--c-text); min-width: 0; &::placeholder { color: var(--c-text-2); } } }
.clear { color: var(--c-text-2); display: grid; place-items: center; width: 22px; height: 22px; }
.suggest { position: absolute; top: 46px; left: 6px; right: 0; padding: 8px 0; border-radius: 14px; background: #241416; border: 1px solid var(--c-border); box-shadow: 0 14px 40px rgba(0, 0, 0, 0.55);
  li { display: flex; align-items: center; gap: 12px; height: 28px; padding: 0 16px; font-size: 14px; color: var(--c-text); cursor: pointer; &:hover { background: var(--c-surface-hover); } &.hl { color: var(--c-accent-2); } .fill { margin-left: auto; color: var(--c-text-3); } } }
.right { margin-left: auto; display: flex; align-items: center; gap: 22px; }
.icon { position: relative; color: var(--c-text); display: grid; place-items: center; }
.dot { position: absolute; top: -2px; right: -2px; width: 8px; height: 8px; border-radius: 50%; background: #ff3b3b; }
.avatar { width: 36px; height: 36px; border-radius: 50%; background: #6f6578; overflow: hidden; }
.win { display: flex; gap: 6px; margin-left: 4px; button { width: 36px; height: 32px; display: grid; place-items: center; color: var(--c-text); border-radius: 8px; &:hover { background: var(--c-surface-hover); } } }
</style>
