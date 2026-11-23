import { defineStore } from 'pinia'
import { ref } from 'vue'
export interface ToastItem { id: number; text: string; kind: 'info'|'success'|'error' }
export const useToastStore = defineStore('toast', () => {
  const items = ref<ToastItem[]>([])
  let seq = 0
  function push(text: string, kind: ToastItem['kind'] = 'info', ms = 2400) {
    const id = ++seq
    items.value.push({ id, text, kind })
    if (ms > 0) setTimeout(() => dismiss(id), ms)
    return id
  }
  const dismiss = (id: number) => { items.value = items.value.filter((t) => t.id !== id) }
  return { items, push, dismiss }
})
