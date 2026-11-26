import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { catalogProviderName, getDetail, type DramaDetail } from '@/mocks/provider'

const batchSize = 12
const concurrentLookups = 3

export function useCMSHistoryDetails(ids: () => string[]) {
  const details = reactive(new Map<string, DramaDetail | null>())
  const remaining = computed(() => ids().filter((id) => !details.has(id)))
  const loading = ref(false)
  let active = true
  let started = false
  onBeforeUnmount(() => { active = false })

  async function loadMore() {
    if (loading.value || !remaining.value.length) return
    const batch = remaining.value.slice(0, batchSize)
    loading.value = true
    const appleCMS = await catalogProviderName().then((name) => name === 'apple-cms', () => false)
    if (!active) return
    if (appleCMS) {
      let index = 0
      async function lookup() {
        while (active && index < batch.length) {
          const id = batch[index++]
          if (!ids().includes(id)) continue
          const detail = await getDetail(id, 'normal', true).catch(() => null)
          if (active && ids().includes(id)) details.set(id, detail?.appleCMS ? detail : null)
        }
      }
      await Promise.all(Array.from({ length: Math.min(concurrentLookups, batch.length) }, lookup))
    } else {
      for (const id of remaining.value) details.set(id, null)
    }
    if (active) loading.value = false
  }

  watch(() => remaining.value.length, (length) => {
    if (!length || started) return
    started = true
    void loadMore()
  }, { immediate: true })

  return { details, loadMore, loading, hasMore: computed(() => remaining.value.length > 0) }
}
