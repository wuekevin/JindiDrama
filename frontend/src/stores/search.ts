import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { saveSearch } from '@/bindings'
const KEY = 'jindi.search.v1'
export const useSearchStore = defineStore('search', () => {
  let init: string[] = ['替嫁', '古装甜宠', '总裁追妻', '仙侠恋爱', '暴君']
  try { const r = localStorage.getItem(KEY); if (r) init = JSON.parse(r) } catch { /* ignore */ }
  const history = ref<string[]>(init)
  watch(history, () => saveSearch(history.value), { deep: true })
  const add = (q: string) => { const t = q.trim(); if (t) history.value = [t, ...history.value.filter((x) => x !== t)].slice(0, 10) }
  const remove = (q: string) => { history.value = history.value.filter((x) => x !== q) }
  const clear = () => { history.value = [] }
  return { history, add, remove, clear }
})
