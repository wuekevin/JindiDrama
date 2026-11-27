import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { catalogProviderName, getDetail, type DramaDetail } from '@/mocks/provider'

const batchSize = 12
const concurrentLookups = 3
const waitingLookups: Array<() => void> = []
let activeLookups = 0

async function lookupDetail(id: string, wanted: () => boolean) {
  if (activeLookups < concurrentLookups) activeLookups++
  else await new Promise<void>((resolve) => waitingLookups.push(resolve))
  try {
    return wanted() ? await getDetail(id, 'normal', true) : null
  } finally {
    // Transfer the slot to the next waiter before allowing a new lookup to start.
    const next = waitingLookups.shift()
    if (next) next()
    else activeLookups--
  }
}

export function useCMSLibraryDetails(ids: () => string[]) {
  const details = reactive(new Map<string, DramaDetail | null>())
  const failed = reactive(new Set<string>())
  const retrying = reactive(new Set<string>())
  const remaining = computed(() => ids().filter((id) => !details.has(id)))
  const loading = ref(false)
  let active = true
  let started = false
  let loadingRun: Promise<void> | undefined
  onBeforeUnmount(() => { active = false })

  function loadMore() {
    if (!active || !remaining.value.length) return loadingRun
    loadingRun ??= loadBatch().finally(() => { loadingRun = undefined })
    return loadingRun
  }

  async function retryFailed() {
    await loadingRun
    if (!active) return
    await Promise.all(failedIds.value.map(retryDetail))
  }

  async function refreshDetail(id: string) {
    const wanted = () => active && ids().includes(id)
    if (!wanted()) return
    try {
      const detail = await lookupDetail(id, wanted)
      if (!wanted()) return
      failed.delete(id)
      details.set(id, detail?.appleCMS ? detail : null)
    } catch {
      if (!wanted()) return
      failed.add(id)
      details.set(id, null)
    }
  }

  const canRetry = (id: string) => details.get(id) === null && failed.has(id)
  const failedIds = computed(() => ids().filter(canRetry))

  async function retryDetail(id: string) {
    if (!canRetry(id) || retrying.has(id)) return
    retrying.add(id)
    try {
      await refreshDetail(id)
    } finally {
      retrying.delete(id)
    }
  }

  async function loadBatch() {
    const batch = remaining.value.slice(0, batchSize)
    loading.value = true
    const appleCMS = await catalogProviderName().then((name) => name === 'apple-cms', () => false)
    if (!active) return
    if (appleCMS) {
      let index = 0
      async function lookup() {
        while (active && index < batch.length) {
          await refreshDetail(batch[index++])
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

  return { details, loadMore, retryFailed, failedIds, canRetry, retryDetail, retrying, loading,
    hasMore: computed(() => remaining.value.length > 0) }
}
