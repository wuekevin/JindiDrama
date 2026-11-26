import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { loadCachedSearchHistory, saveSearch } from '@/bindings'
import { useToastStore } from './toast'

export const useSearchStore = defineStore('search', () => {
  const toast = useToastStore()
  const history = ref(loadCachedSearchHistory() ?? ['替嫁', '古装甜宠', '总裁追妻', '仙侠恋爱', '暴君'])
  watch(history, async (snapshot) => {
    if (!(await saveSearch(snapshot)) && history.value === snapshot) {
      toast.push('最近搜索的更改未能保存，重启后可能恢复旧记录。', 'error')
    }
  }, { flush: 'sync' })
  const add = (q: string) => { const t = q.trim(); if (t) history.value = [t, ...history.value.filter((x) => x !== t)].slice(0, 10) }
  const remove = (q: string) => { history.value = history.value.filter((x) => x !== q) }
  const clear = () => { history.value = [] }
  return { history, add, remove, clear }
})
