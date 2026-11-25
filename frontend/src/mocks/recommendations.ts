import { computed, ref, watch, type Ref } from 'vue'
import { DRAMAS } from './data'
import type { Drama } from '@/types'

export function rankRelatedDramas(source: Drama, catalog: readonly Drama[]): Drama[] {
  const seen = new Set([source.id])
  const tags = new Set(source.tags)
  return catalog.filter((drama) => {
    if (drama.updatedTo < 1 || seen.has(drama.id)) return false
    seen.add(drama.id)
    return true
  }).map((drama) => ({
    drama,
    sameKind: Number(drama.kind === source.kind),
    sharedTags: new Set(drama.tags.filter((tag) => tags.has(tag))).size,
  })).sort((a, b) => b.sameKind - a.sameKind || b.sharedTags - a.sharedTags || b.drama.heat - a.drama.heat)
    .map(({ drama }) => drama)
}

export function useRelatedDramas(source: Ref<Drama | undefined>, size: number, catalog: readonly Drama[] = DRAMAS) {
  const start = ref(0)
  watch(() => source.value?.id, () => { start.value = 0 }, { flush: 'sync' })
  const candidates = computed(() => source.value ? rankRelatedDramas(source.value, catalog) : [])
  const canShuffle = computed(() => candidates.value.length > size)
  const cards = computed(() => {
    const list = candidates.value
    const count = Math.min(size, list.length)
    const head = list.slice(start.value, start.value + count)
    return [...head, ...list.slice(0, count - head.length)]
  })
  function shuffle() {
    if (canShuffle.value) start.value = (start.value + size) % candidates.value.length
  }
  return { cards, canShuffle, shuffle }
}
