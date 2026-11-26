import type { DanmakuSubmission } from '@/player/danmakuDraft'

export class UndurableDanmakuError extends Error {}

const scope = (profile: string, drama: string, episode: number) =>
  `jindi.danmaku.pending.v1.${profile}.${JSON.stringify([drama, episode])}.`
// Go TrimSpace and JavaScript trim disagree on a few Unicode code points.
export const normalizeDanmakuText = (text: string) => text.replace(/^[\p{White_Space}\uFEFF]+|[\p{White_Space}\uFEFF]+$/gu, '')
const validRequest = /^[0-9a-f]{32}$/

function serialized(request: DanmakuSubmission) {
  return JSON.stringify({ requestId: request.requestId, text: request.text, offsetMs: request.offsetMs })
}

export function retainDanmaku(profile: string | undefined, drama: string, episode: number, request: DanmakuSubmission) {
  if (!profile) return false
  const key = scope(profile, drama, episode) + request.requestId
  const encoded = serialized(request)
  try {
    const existing = localStorage.getItem(key)
    if (existing !== null && existing !== encoded) return false
    localStorage.setItem(key, encoded)
    return true
  } catch {
    return false
  }
}

export function acknowledgeDanmaku(profile: string | undefined, drama: string, episode: number, request: DanmakuSubmission) {
  if (!profile) return
  const key = scope(profile, drama, episode) + request.requestId
  try {
    if (localStorage.getItem(key) === serialized(request)) localStorage.removeItem(key)
  } catch {
    // SQLite already committed. A later replay still uses the same request ID.
  }
}

export function pendingDanmaku(profile: string, drama: string, episode: number): DanmakuSubmission[] | undefined {
  const prefix = scope(profile, drama, episode)
  try {
    const requests: DanmakuSubmission[] = []
    for (const key of Object.keys(localStorage)) {
      if (!key.startsWith(prefix) || !validRequest.test(key.slice(prefix.length))) continue
      const raw = localStorage.getItem(key)
      if (!raw || raw.length > 512) continue
      try {
        const request = JSON.parse(raw) as Partial<DanmakuSubmission> | null
        if (!request || request.requestId !== key.slice(prefix.length) || typeof request.text !== 'string' ||
            !request.text || request.text !== normalizeDanmakuText(request.text) || request.text.length > 40 || typeof request.offsetMs !== 'number' ||
            !Number.isSafeInteger(request.offsetMs) || request.offsetMs < 0) continue
        // Ignore records outside the writer's canonical encoding.
        const decoded = { requestId: request.requestId, text: request.text, offsetMs: request.offsetMs }
        if (raw === serialized(decoded)) requests.push(decoded)
      } catch {
        // Ignore damaged records without sending or deleting them.
      }
    }
    return requests
  } catch {
    return undefined
  }
}
