import { localId } from '@/localId'

export type CommentSaveIssue = 'failed' | 'not-durable' | 'conflict'
export type CommentLoad = { snapshot: string; issue?: CommentSaveIssue }

export interface CommentState {
  snapshot: string
  revision: string
  writeId: string
  profile: string
}

interface CommentBridge {
  LocalCommentsState(id: string, episode: number): Promise<CommentState>
  CompareAndSaveLocalComments(id: string, episode: number, expected: string, snapshot: string, writeId: string): Promise<string>
}

interface Journal {
  base: string
  attempt?: { writeId: string; snapshot: string }
  snapshot: string
}

interface Entry {
  key: string
  stored: string | null
  journal: Journal
}

interface EpisodeState {
  prefix: string
  profile: string
  revision: string
  pending?: Entry
  conflicts?: Entry[]
}

const namespace = 'jindi.comments.pending.v1.'
const maxSnapshotBytes = 256 * 1024
const digest = /^[a-f0-9]{64}$/
const writeToken = /^[a-f0-9]{32}$/
const encoder = new TextEncoder()
const states = new Map<string, EpisodeState>()

function validSnapshot(snapshot: unknown): snapshot is string {
  if (typeof snapshot !== 'string' || encoder.encode(snapshot).length > maxSnapshotBytes) return false
  try {
    const data = JSON.parse(snapshot)
    return Array.isArray(data) && data.every((item) => item && typeof item.id === 'string' &&
      typeof item.text === 'string' && (item.replies === undefined ||
        (Array.isArray(item.replies) && item.replies.every((reply: unknown) => reply !== null && typeof reply === 'object'))))
  } catch {
    return false
  }
}

function loadEntry(key: string): Entry | undefined {
  try {
    const stored = localStorage.getItem(key)
    if (!stored || stored.length > 12 * maxSnapshotBytes) return
    const record: Partial<Journal> | null = JSON.parse(stored)
    if (!record || !validSnapshot(record.snapshot) || typeof record.base !== 'string' ||
        (record.base !== '' && !digest.test(record.base))) return
    if (record.attempt !== undefined && (!record.attempt || typeof record.attempt.writeId !== 'string' ||
        !writeToken.test(record.attempt.writeId) || !validSnapshot(record.attempt.snapshot))) return
    return { key, stored, journal: { base: record.base, attempt: record.attempt, snapshot: record.snapshot } }
  } catch {
    return
  }
}

function storedEntries(prefix: string): Entry[] {
  try {
    return Object.keys(localStorage)
      .filter((key) => key.startsWith(prefix) && writeToken.test(key.slice(prefix.length)))
      .map(loadEntry)
      .filter((entry): entry is Entry => entry !== undefined)
  } catch {
    return []
  }
}

function publish(entry: Entry, remove = false): boolean {
  try {
    // Only touch the version we read or last wrote. Each window creates its
    // own key so acknowledgements cannot remove another window's new edits.
    if (localStorage.getItem(entry.key) !== entry.stored) return false
    if (remove) {
      localStorage.removeItem(entry.key)
      entry.stored = null
    } else {
      const serialized = JSON.stringify(entry.journal)
      localStorage.setItem(entry.key, serialized)
      entry.stored = serialized
    }
    return true
  } catch {
    return false
  }
}

function confirmsAttempt(journal: Journal, current: CommentState) {
  return current.writeId === journal.attempt?.writeId && current.snapshot === journal.attempt.snapshot
}

function episodeKey(id: string, episode: number) {
  return JSON.stringify([id, episode])
}

export async function loadRecoverableComments(
  bridge: CommentBridge,
  id: string,
  episode: number,
  queue: (task: () => Promise<CommentSaveIssue | undefined>) => Promise<CommentSaveIssue | undefined>,
): Promise<CommentLoad> {
  const current = await bridge.LocalCommentsState(id, episode)
  if (!digest.test(current.profile)) throw new Error('Invalid comment profile')
  const scope = episodeKey(id, episode)
  const prefix = namespace + current.profile + '.' + scope + '.'
  const state: EpisodeState = { prefix, profile: current.profile, revision: current.revision }
  states.set(scope, state)

  const pending = storedEntries(prefix).filter((entry) => {
    if (!confirmsAttempt(entry.journal, current) || entry.journal.snapshot !== current.snapshot) return true
    publish(entry, true)
    return false
  })
  if (pending.length > 1) {
    state.conflicts = pending
    return { snapshot: current.snapshot, issue: 'conflict' }
  }
  const entry = pending[0]
  if (!entry) return { snapshot: current.snapshot }
  state.pending = entry
  const snapshot = entry.journal.snapshot
  const issue = await queue(() => flush(bridge, id, episode, state, snapshot))
  return { snapshot: issue === 'conflict' ? (await bridge.LocalCommentsState(id, episode)).snapshot : snapshot, issue }
}

export function stageRecoverableComments(
  bridge: CommentBridge,
  id: string,
  episode: number,
  snapshot: string,
): () => Promise<CommentSaveIssue | undefined> {
  const state = states.get(episodeKey(id, episode))
  if (!state || !validSnapshot(snapshot)) return async () => 'not-durable'
  if (state.conflicts) return async () => 'conflict'
  const journal = state.pending?.journal
  const entry = state.pending ?? { key: state.prefix + localId(), stored: null, journal: { base: state.revision, snapshot } }
  entry.journal = { base: journal?.base ?? state.revision, attempt: journal?.attempt, snapshot }
  state.pending = entry
  publish(entry)
  return () => flush(bridge, id, episode, state, snapshot)
}

async function flush(
  bridge: CommentBridge, id: string, episode: number, state: EpisodeState, snapshot: string,
): Promise<CommentSaveIssue | undefined> {
  const entry = state.pending
  if (!entry) return
  if (state.conflicts) return 'conflict'

  try {
    const current = await bridge.LocalCommentsState(id, episode)
    const latest = entry.journal
    const committed = confirmsAttempt(latest, current)
    if (current.profile !== state.profile || (current.revision !== latest.base && !committed)) {
      state.conflicts = [entry]
      return 'conflict'
    }

    let revision = current.revision
    if (!committed || current.snapshot !== snapshot) {
      const attempt = { writeId: localId(), snapshot }
      entry.journal = { base: current.revision, attempt, snapshot: latest.snapshot }
      publish(entry)
      revision = await bridge.CompareAndSaveLocalComments(id, episode, current.revision, snapshot, attempt.writeId)
      if (!revision) {
        state.conflicts = [entry]
        return 'conflict'
      }
    }

    state.revision = revision
    if (entry.journal.snapshot !== snapshot) {
      entry.journal = { base: revision, snapshot: entry.journal.snapshot }
      publish(entry)
    } else {
      state.pending = undefined
      publish(entry, true)
    }
  } catch {
    return publish(entry) ? 'failed' : 'not-durable'
  }
}

export function discardConflictedComments(id: string, episode: number): boolean {
  const scope = episodeKey(id, episode)
  const state = states.get(scope)
  if (!state?.conflicts) return false
  for (const entry of state.conflicts) {
    if (!publish(entry, true)) return false
  }
  states.delete(scope)
  return true
}
