import { reactive } from 'vue'

export interface DanmakuSubmission {
  text: string
  offsetMs: number
  requestId: string
}

export interface DanmakuDraft {
  text: string
  sending: boolean
  attempt?: DanmakuSubmission
}

const drafts = new Map<string, DanmakuDraft>()

export function draftForDanmaku(dramaId: string, episode: number): DanmakuDraft {
  const key = JSON.stringify([dramaId, episode])
  const existing = drafts.get(key)
  if (existing) return existing
  const draft = reactive<DanmakuDraft>({ text: '', sending: false })
  drafts.set(key, draft)
  return draft
}
