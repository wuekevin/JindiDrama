import { computed, ref, watch, type Ref } from 'vue'
import { loadLocalDanmaku, recoverLocalDanmaku, sendLocalDanmaku, type LocalDanmaku } from '@/bindings'
import { normalizeDanmakuText } from '@/bindings/danmakuRecovery'
import { draftForDanmaku, type DanmakuSubmission } from '@/player/danmakuDraft'
import type { DanmakuItem } from '@/player/danmaku'
import { useToastStore } from '@/stores/toast'

export function useLocalEpisodeDanmaku(dramaId: Readonly<Ref<string>>, episode: Readonly<Ref<number>>, enabled: () => boolean) {
  const toast = useToastStore()
  const mine = ref<DanmakuItem[]>([])
  const toItem = (item: LocalDanmaku): DanmakuItem => ({
    id: `local-${item.id}`, at: item.offsetMs / 1000, text: item.text, likes: 0, lane: item.id % 12, mine: true,
  })
  function mergeMine(items: DanmakuItem[]) {
    mine.value = [...new Map([...mine.value, ...items].map((item) => [item.id, item])).values()]
  }
  const recoveringDanmaku = ref(false)
  const pendingDanmakuRecovery = ref(false)
  let danmakuEpisodeGeneration = 0
  async function retryDanmakuRecovery() {
    if (recoveringDanmaku.value) return
    const generation = danmakuEpisodeGeneration
    const current = () => generation === danmakuEpisodeGeneration
    recoveringDanmaku.value = true
    try {
      const pending = await recoverLocalDanmaku(dramaId.value, episode.value, current, (item, entry) => {
        if (!current()) return
        mergeMine([toItem(item)])
        const draft = danmakuDraft.value
        if (!draft.sending && draft.attempt?.requestId === entry.requestId) {
          if (normalizeDanmakuText(draft.text) === entry.text) draft.text = ''
          draft.attempt = undefined
        }
      })
      if (current()) pendingDanmakuRecovery.value = pending
    } finally {
      if (current()) recoveringDanmaku.value = false
    }
  }
  watch([dramaId, episode, enabled], async ([id, ep, active], _, onCleanup) => {
    danmakuEpisodeGeneration++
    onCleanup(() => { danmakuEpisodeGeneration++ })
    const generation = danmakuEpisodeGeneration
    mine.value = []
    recoveringDanmaku.value = false
    pendingDanmakuRecovery.value = false
    if (!active) return
    void retryDanmakuRecovery()
    try {
      const saved = await loadLocalDanmaku(id, ep)
      if (generation === danmakuEpisodeGeneration && saved) mergeMine(saved.map(toItem))
    } catch {
      if (generation === danmakuEpisodeGeneration) toast.push('本地弹幕读取失败', 'error')
    }
  }, { immediate: true })
  const danmakuDraft = computed(() => draftForDanmaku(dramaId.value, episode.value))
  async function sendDm(submission: DanmakuSubmission) {
    const id = dramaId.value
    const ep = episode.value
    const generation = danmakuEpisodeGeneration
    let saved: LocalDanmaku | undefined
    try {
      saved = await sendLocalDanmaku(id, ep, submission)
    } catch (error) {
      if (generation === danmakuEpisodeGeneration) pendingDanmakuRecovery.value = true
      throw error
    }
    if (generation !== danmakuEpisodeGeneration) return
    mergeMine([saved ? toItem(saved) : {
      id: `demo-${submission.requestId}`, at: submission.offsetMs / 1000, text: submission.text,
      likes: 0, lane: Math.floor(Math.random() * 12), mine: true,
    }])
    if (pendingDanmakuRecovery.value) void retryDanmakuRecovery()
  }
  return { mine, danmakuDraft, sendDm, recoveringDanmaku, pendingDanmakuRecovery, retryDanmakuRecovery }
}
