<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { closeDesktopWindow, hasLicensedLibrary, hideWindowToTray, searchProvider } from '@/bindings'
import { CLOSE_TO_TRAY, TRAY_UNAVAILABLE_MESSAGE, useAppSettings } from '@/stores/appSettings'
import { useToastStore } from '@/stores/toast'
import { useSearchStore } from '@/stores/search'
import { searchDiscovery } from '@/router/searchDiscovery'
import { Search, Minus, Square, X, CornerUpLeft } from 'lucide-vue-next'
import UpdateNotifications from './UpdateNotifications.vue'
import { suggestDemoSearch } from '@/mocks/provider'

const route = useRoute(); const router = useRouter()
const recentSearch = useSearchStore()
const topbar = ref<HTMLElement>()
const compact = ref(false)
let layoutObserver: ResizeObserver | undefined
onMounted(() => {
  if (!topbar.value) return
  // Use layout width including padding, so the compact padding cannot change the breakpoint.
  layoutObserver = new ResizeObserver(([{ target }]) => {
    compact.value = target.clientWidth <= 760
  })
  layoutObserver.observe(topbar.value)
})
onBeforeUnmount(() => layoutObserver?.disconnect())

const q = ref('')
const focused = ref(false)
const suggestionsOpen = ref(false)
const activeSuggestion = ref(-1)
const suggestions = ref<string[]>([])
const expanded = computed(() => suggestionsOpen.value && suggestions.value.length > 0)
const suggestionId = (index: number) => `topbar-suggestion-${index}`
watch(() => route.query.q, (v) => { if (route.name === 'search') q.value = String(v ?? '') }, { immediate: true })
watch([q, focused, suggestionsOpen], async ([text, active, open], [previous], onCleanup) => {
  if (text !== previous || !active) {
    suggestions.value = []
    activeSuggestion.value = -1
  }
  const query = text.trim()
  if (!active || !open || !query || suggestions.value.length) return
  let current = true
  onCleanup(() => { current = false })
  try {
    const desktop = searchProvider(query)
    const titles = desktop
      ? (await desktop).map(({ title }) => title)
      : suggestDemoSearch(query)
    if (current) suggestions.value = [...new Set(titles)].slice(0, 6)
  } catch {
    // 搜索提交仍可用，结果页负责显示错误。
  }
})
function closeSuggestions() {
  suggestionsOpen.value = false
  activeSuggestion.value = -1
}
function focusSearch() {
  focused.value = true
  suggestionsOpen.value = true
}
function blurSearch() {
  focused.value = false
  closeSuggestions()
}
function searchInput(event: Event) {
  if (!(event as InputEvent).isComposing) suggestionsOpen.value = true
}
function submit(text = q.value) {
  const query = text.trim()
  if (!query) return
  recentSearch.add(query)
  q.value = query
  closeSuggestions()
  router.push({ name: 'search', query: { q: query } })
}
function searchKey(event: KeyboardEvent) {
  if (event.isComposing || event.keyCode === 229 || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return
  if (event.key === 'Escape' && suggestionsOpen.value) {
    event.preventDefault()
    closeSuggestions()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    submit(expanded.value ? suggestions.value[activeSuggestion.value] ?? q.value : q.value)
  } else if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && q.value.trim()) {
    suggestionsOpen.value = true
    if (!suggestions.value.length) return
    event.preventDefault()
    const last = suggestions.value.length - 1
    activeSuggestion.value = event.key === 'ArrowDown'
      ? Math.min(activeSuggestion.value + 1, last)
      : activeSuggestion.value < 0 ? last : Math.max(activeSuggestion.value - 1, 0)
  }
}
const tabs = [{ k: 'discover', label: '发现', to: '/home' }, { k: 'library', label: '片库', to: hasLicensedLibrary() ? '/licensed' : '/search' }, { k: 'category', label: '分类', to: { name: 'search', query: { browse: 'topics' } } }]
const tabActive = (k: string) => {
  const discovery = route.name === 'search' ? searchDiscovery(route.query) : undefined
  if (discovery === 'topics') return k === 'category'
  if (discovery) return k === 'discover'
  const inLibrary = route.name === 'search' || route.name === 'licensed'
  return k === 'discover' ? !inLibrary : k === 'library' && inLibrary
}
const appSettings = useAppSettings()
const toast = useToastStore()
let closingWindow = false
async function win(op: 'min' | 'max' | 'close') {
  const rt = (window as any).runtime
  if (!rt) return
  if (op === 'min') return rt.WindowMinimise()
  if (op === 'max') return rt.WindowToggleMaximise()
  if (closingWindow) return
  const action = appSettings.s.closeAction
  closingWindow = true
  try {
    const desktopClose = closeDesktopWindow()
    if (desktopClose) {
      if (!(await desktopClose)) toast.push(TRAY_UNAVAILABLE_MESSAGE, 'error')
      return
    }
    if (action === '最小化到任务栏') return rt.WindowMinimise()
    if (action !== CLOSE_TO_TRAY) return rt.Quit()
    if (!appSettings.canCloseToTray) return rt.WindowMinimise()
    if (await hideWindowToTray()) return
  } catch {
    if (action !== CLOSE_TO_TRAY) {
      toast.push('无法关闭窗口', 'error')
      return
    }
  } finally {
    closingWindow = false
  }
  toast.push(TRAY_UNAVAILABLE_MESSAGE, 'error')
  rt.WindowMinimise()
}
</script>
<template>
  <header ref="topbar" class="topbar" :class="{ compact }" style="--wails-draggable: drag">
    <nav class="tabs" aria-label="一级栏目">
      <button v-for="t in tabs" :key="t.k" class="tab" :class="{ on: tabActive(t.k) }" :aria-current="tabActive(t.k) ? 'page' : undefined" :data-testid="`tab-${t.k}`" @click="router.push(t.to)">{{ t.label }}</button>
    </nav>
    <div class="search" :class="{ focus: focused, 'on-dark': focused }" role="search">
      <Search :size="20" />
      <input v-model="q" data-testid="search-input" placeholder="搜索短剧、主演或题材，如 “契约婚姻”"
        role="combobox" aria-label="搜索" aria-autocomplete="list" :aria-expanded="expanded"
        :aria-controls="expanded ? 'topbar-suggestions' : undefined"
        :aria-activedescendant="expanded && activeSuggestion >= 0 ? suggestionId(activeSuggestion) : undefined"
        @focus="focusSearch" @blur="blurSearch" @input="searchInput" @keydown="searchKey" />
      <button v-if="q" class="clear" aria-label="清除" @mousedown.prevent @click="q = ''"><X :size="16" /></button>
      <ul v-if="expanded" id="topbar-suggestions" class="suggest" role="listbox" aria-label="搜索联想">
        <li v-for="(s, i) in suggestions" :id="suggestionId(i)" :key="s" role="option" :aria-selected="i === activeSuggestion"
          :class="{ hl: i === activeSuggestion }" @mousedown.prevent @click="submit(s)"><Search :size="15" /><span>{{ s }}</span><CornerUpLeft :size="14" class="fill" /></li>
      </ul>
    </div>
    <div class="right">
      <UpdateNotifications />
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
.topbar { position: relative; height: var(--topbar-h); flex: none; display: flex; align-items: center; gap: 16px; padding: 0 12px 0 40px; z-index: 20; }
.tab, .search, .right :deep(button) { --wails-draggable: no-drag; }
.tabs { display: flex; gap: 0; }
.tabs, .right { flex: none; }
.tab { position: relative; padding: 0 31px; height: 40px; font-size: 19px; font-weight: 600; color: var(--c-text-2);
  &.on { color: var(--c-text); &::after { content: ''; position: absolute; left: 50%; bottom: 0; width: 56px; height: 3px; border-radius: 3px; transform: translateX(-50%); background: var(--c-accent); } }
  &:hover { color: var(--c-text); } }
.search { position: relative; flex: 0 1 440px; min-width: 0; margin-inline-start: auto; height: 40px; display: flex; align-items: center; gap: 10px; padding: 0 14px; border-radius: 20px; background: rgba(255, 255, 255, 0.06); border: 1px solid var(--c-border); color: var(--c-text-2);
  &.focus { border-color: var(--c-accent); box-shadow: 0 0 0 2px rgba(238, 75, 57, 0.25); background: rgba(40, 18, 18, 0.9); }
  > svg { flex: none; }
  input { flex: 1; background: none; border: 0; outline: 0; font-size: 14.5px; color: var(--c-text); min-width: 0; &::placeholder { color: var(--c-text-2); } } }
.clear { flex: none; color: var(--c-text-2); display: grid; place-items: center; width: 22px; height: 22px; }
.suggest { position: absolute; top: 46px; left: 6px; right: 0; padding: 8px 0; border-radius: 14px; background: #241416; border: 1px solid var(--c-border); box-shadow: 0 14px 40px rgba(0, 0, 0, 0.55);
  li { display: flex; align-items: center; gap: 12px; height: 28px; padding: 0 16px; font-size: 14px; color: var(--c-text); cursor: pointer; &:hover, &.hl { background: var(--c-surface-hover); } &.hl { color: var(--c-accent-2); } }
  li > svg { flex: none; }
  li > span { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fill { color: var(--c-text-3); } }
.right { display: flex; align-items: center; gap: 22px; }
.avatar { width: 36px; height: 36px; border-radius: 50%; background: #6f6578; overflow: hidden; }
.win { display: flex; gap: 6px; margin-left: 4px; button { width: 36px; height: 32px; display: grid; place-items: center; color: var(--c-text); border-radius: 8px; &:hover { background: var(--c-surface-hover); } } }


.topbar.compact {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  height: auto;
  gap: 4px 12px;
  padding: 6px 12px 8px;

  .tabs { grid-column: 1 / -1; }
  .tab {
    height: 36px;
    padding-inline: 16px;
    &:focus-visible { outline-offset: -3px; }
  }
  .search { margin-inline-start: 0; }
  .right { gap: 12px; }
}
</style>
