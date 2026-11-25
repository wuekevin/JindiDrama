import { defineStore } from 'pinia'
import { ref } from 'vue'
interface ToastAction {
  label: string
  run: () => void
  busy: () => boolean
}
export interface ToastItem { id: number; text: string; kind: 'info'|'success'|'error'; action?: ToastAction }
export const useToastStore = defineStore('toast', () => {
  const items = ref<ToastItem[]>([])
  let seq = 0
  function push(text: string, kind: ToastItem['kind'] = 'info', ms = 2400, action?: ToastAction) {
    const id = ++seq
    items.value.push({ id, text, kind, action })
    if (ms > 0) setTimeout(() => dismiss(id), ms)
    return id
  }
  const dismiss = (id: number) => { items.value = items.value.filter((t) => t.id !== id) }
  return { items, push, dismiss }
})
