import { localId } from '@/localId'

export type CommentSaveIssue = 'failed' | 'not-durable' | 'conflict'
export type LegacyCommentBackup = { key: string; snapshot: string }
export type CommentLoad = {
  snapshot: string
  issue?: CommentSaveIssue
  backups?: LegacyCommentBackup[]
  canRestoreBackup?: boolean
}

export interface CommentState {
  snapshot: string
  revision: string
  writeId: string
  profile: string
  legacyProfile?: string
}

interface CommentBridge {
  LocalCommentsState(id: string, episode: number): Promise<CommentState>
  CompareAndSaveLocalComments(id: string, episode: number, expected: string, snapshot: string, writeId: string): Promise<string>
}

interface Journal {
  base: string
  attempt?: { writeId: string; snapshot: string }
  snapshot: string
  legacy?: { id: string; stored: string }
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
  legacy: Entry[]
  legacyPrefix?: string
}

const namespace = 'jindi.comments.pending.v1.'
const maxSnapshotBytes = 256 * 1024
const maxLegacyJournalLength = 12 * maxSnapshotBytes
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
    if (!stored || stored.length > 3 * maxLegacyJournalLength) return
    const record: Partial<Journal> | null = JSON.parse(stored)
    if (!record || !validSnapshot(record.snapshot) || typeof record.base !== 'string' ||
        (record.base !== '' && !digest.test(record.base))) return
    if (record.attempt !== undefined && (!record.attempt || typeof record.attempt.writeId !== 'string' ||
        !writeToken.test(record.attempt.writeId) || !validSnapshot(record.attempt.snapshot))) return
    if (record.legacy !== undefined && (!record.legacy || typeof record.legacy.id !== 'string' ||
        !writeToken.test(record.legacy.id) || typeof record.legacy.stored !== 'string' ||
        !record.legacy.stored || record.legacy.stored.length > maxLegacyJournalLength)) return
    return {
      key,
      stored,
      journal: { base: record.base, attempt: record.attempt, snapshot: record.snapshot, legacy: record.legacy },
    }
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

function promoteLegacy(state: EpisodeState, source: Entry, journal: Pick<Journal, 'base' | 'attempt' | 'snapshot'>) {
  if (!state.legacyPrefix || !source.stored || source.stored.length > maxLegacyJournalLength ||
      loadEntry(source.key)?.stored !== source.stored) return
  const id = source.key.slice(state.legacyPrefix.length)
  // Leave the original bytes available to older clients until SQLite confirms the copy.
  const copy = { key: state.prefix + id, stored: null, journal: { ...journal, legacy: { id, stored: source.stored } } }
  return publish(copy) ? copy : undefined
}

function clearCommitted(state: EpisodeState, entry: Entry) {
  const link = entry.journal.legacy
  if (state.legacyPrefix && link) {
    const source = loadEntry(state.legacyPrefix + link.id)
    if (source?.stored === link.stored && !publish(source, true)) return false
  }
  return publish(entry, true)
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
  const state: EpisodeState = { prefix, profile: current.profile, revision: current.revision, legacy: [] }
  states.set(scope, state)
  if (current.legacyProfile && current.legacyProfile !== current.profile && digest.test(current.legacyProfile)) {
    state.legacyPrefix = namespace + current.legacyProfile + '.' + scope + '.'
  }

  const unconfirmed = (entry: Entry) => {
    if (!confirmsAttempt(entry.journal, current) || entry.journal.snapshot !== current.snapshot) return true
    clearCommitted(state, entry)
    return false
  }
  const pending = storedEntries(prefix).filter(unconfirmed)
  if (state.legacyPrefix) {
    const proven: Entry[] = []
    for (const entry of storedEntries(state.legacyPrefix)) {
      const id = entry.key.slice(state.legacyPrefix.length)
      if (confirmsAttempt(entry.journal, current) && entry.journal.snapshot === current.snapshot) {
        publish(entry, true)
        continue
      }
      if (!pending.some(({ journal }) => journal.legacy?.id === id) &&
          (confirmsAttempt(entry.journal, current) || (current.revision && entry.journal.base === current.revision))) {
        proven.push(entry)
      }
      state.legacy.push(entry)
    }
    if (!pending.length && proven.length === 1) {
      const entry = proven[0]!
      const { base, attempt, snapshot } = entry.journal
      const copy = promoteLegacy(state, entry, { base, attempt, snapshot })
      if (copy) pending.push(copy)
    }
  }
  const loadBackups = () => {
    state.legacy = state.legacy.flatMap(({ key }) => {
      const entry = loadEntry(key)
      return entry ? [entry] : []
    })
    return state.legacy.map((entry) => ({ key: entry.key, snapshot: entry.journal.snapshot }))
  }
  if (pending.length > 1) {
    state.conflicts = pending
    return { snapshot: current.snapshot, issue: 'conflict', backups: loadBackups() }
  }
  const entry = pending[0]
  if (!entry) return { snapshot: current.snapshot, backups: loadBackups(), canRestoreBackup: current.revision === '' }
  state.pending = entry
  const snapshot = entry.journal.snapshot
  const issue = await queue(() => flush(bridge, id, episode, state, snapshot))
  return {
    snapshot: issue === 'conflict' ? (await bridge.LocalCommentsState(id, episode)).snapshot : snapshot,
    issue,
    backups: loadBackups(),
  }
}

export async function restoreLegacyCommentBackup(
  bridge: CommentBridge, id: string, episode: number, key: string,
): Promise<CommentSaveIssue | undefined> {
  const state = states.get(episodeKey(id, episode))
  const entry = state?.legacy.find((backup) => backup.key === key)
  if (!state || !entry || state.pending || state.conflicts) return 'conflict'
  try {
    const current = await bridge.LocalCommentsState(id, episode)
    if (current.profile !== state.profile || current.revision !== '' || state.pending ||
        loadEntry(key)?.stored !== entry.stored) return 'conflict'
    const snapshot = entry.journal.snapshot
    const copy = promoteLegacy(state, entry, { base: '', snapshot })
    if (!copy) return 'not-durable'
    state.pending = copy
    return await flush(bridge, id, episode, state, snapshot)
  } catch {
    return 'failed'
  }
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
  const entry = state.pending ?? { key: state.prefix + localId(), stored: null, journal: { base: state.revision, snapshot } }
  entry.journal = { ...entry.journal, snapshot }
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
      entry.journal = { ...latest, base: current.revision, attempt }
      publish(entry)
      revision = await bridge.CompareAndSaveLocalComments(id, episode, current.revision, snapshot, attempt.writeId)
      if (!revision) {
        state.conflicts = [entry]
        return 'conflict'
      }
    }

    state.revision = revision
    if (entry.journal.snapshot !== snapshot) {
      entry.journal = { ...entry.journal, base: revision, attempt: undefined }
      publish(entry)
    } else {
      state.pending = undefined
      clearCommitted(state, entry)
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
