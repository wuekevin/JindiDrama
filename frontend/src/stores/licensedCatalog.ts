import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { loadLicensedCatalog, type LicensedDrama } from '@/bindings'
import { byId } from '@/mocks/data'
import { posterSvg } from '@/mocks/posters'
import type { Drama } from '@/types'

const unavailableCover = posterSvg({ hue: 220, hue2: 222, seed: 1 })

export const useLicensedCatalog = defineStore('licensedCatalog', () => {
  const entries = ref<LicensedDrama[]>([])
  const error = ref('')
  const status = ref<'loading' | 'ready' | 'error'>('loading')

  async function load() {
    try {
      entries.value = (await loadLicensedCatalog()) ?? []
      status.value = 'ready'
    } catch (cause) {
      error.value = typeof cause === 'string' ? cause : cause instanceof Error ? cause.message : '无法读取本地片库'
      status.value = 'error'
    }
  }

  const localCards = computed(() => new Map<string, Drama>(entries.value.map((entry, index) => [entry.id, {
    ...entry,
    cover: posterSvg({ hue: 213, hue2: 178, seed: index + 1 }),
    kind: '本地片库',
    finished: true,
    heat: 0,
  }])))
  const card = (id: string) => localCards.value.get(id) ?? byId(id)

  function savedCard(id: string): Drama | undefined {
    const available = card(id)
    if (available || !id.startsWith('local-')) return available
    return {
      id, title: `本地短剧 · ${id.slice(6)}`, summary: '', cover: unavailableCover,
      kind: '本地片库', tags: ['当前不可用'], episodeCount: 0, updatedTo: 0, finished: false, heat: 0,
    }
  }

  return { entries, status, error, load, card, savedCard }
})
