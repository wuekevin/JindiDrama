// Provider 设计草案；由开发阶段转为实际 types/interfaces
export type Capability = 'catalog'|'search'|'playback'|'account_login'|'comments_read'|'comments_write'|'danmaku_read'|'danmaku_write'|'favorites_sync'|'history_sync';
export type CapabilityState = 'ready'|'mock'|'not_authorized'|'unavailable'|'unsupported';
export interface CapabilityResult { state: CapabilityState; reason?: string; checkedAt?: string }
export interface Drama { id: string; title: string; cover: string; tags: string[]; summary?: string; episodeCount: number }
export interface Episode { id: string; dramaId: string; index: number; title?: string; durationMs?: number; available: boolean }
export interface MediaSource { src: string; type: string; quality?: 'sd' | 'hd'; routeName?: string }
export interface PlaybackTicket { kind: 'demo'|'licensed'|'authorized'; episodeId: string; sources?: MediaSource[]; playbackUrl?: string; expiresAt?: string; leaseId?: string; restrictions?: string[] }
export interface Comment { id: string; episodeId: string; authorDisplay: string; text: string; createdAt: string; likes: number; replyCount: number }
export interface Danmaku { id: string; episodeId: string; offsetMs: number; text: string; likes: number }
export interface VideoProvider {
  capabilities(): Promise<Record<Capability, CapabilityResult>>;
  home(): Promise<Drama[]>;
  search(query: string, filters?: Record<string,string>): Promise<Drama[]>;
  drama(id: string): Promise<Drama>;
  episodes(dramaId: string): Promise<Episode[]>;
  resolvePlayback(dramaId: string, index: number): Promise<PlaybackTicket>;
  comments(episodeId: string, cursor?: string): Promise<{items: Comment[]; nextCursor?: string}>;
  danmaku(episodeId: string): Promise<Danmaku[]>;
}
