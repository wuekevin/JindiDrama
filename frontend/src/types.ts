export type Capability = 'catalog'|'search'|'playback'|'account_login'|'comments_read'|'comments_write'|'danmaku_read'|'danmaku_write'|'favorites_sync'|'history_sync'
export type CapabilityState = 'ready'|'mock'|'not_authorized'|'unavailable'|'unsupported'
export type DramaKind = '真人剧' | '漫剧' | 'AI剧' | '本地片库'
export interface Drama {
  id: string; title: string; cover: string; tags: string[]; kind: DramaKind
  summary: string; episodeCount: number; updatedTo: number; finished: boolean
  heat: number // 万
  isNewToday?: boolean
}
export interface HistoryEntry { dramaId: string; episode: number; percent: number; watchedAt: number; positionSec?: number; durationSec?: number; episodeCountAtWatch?: number }
export interface Banner { id: string; dramaId: string; title: string; subtitle: string; tags: string[]; hue: number; hue2: number; seed: number }
export interface LoadState<T> { status: 'idle'|'loading'|'ready'|'empty'|'error'; data: T; error?: string }
