// Provider 设计草案；由开发阶段转为实际 types/interfaces
export type Capability = 'catalog'|'search'|'playback'|'account_login'|'comments_read'|'comments_write'|'danmaku_read'|'danmaku_write'|'favorites_sync'|'history_sync';
export type CapabilityState = 'ready'|'mock'|'not_authorized'|'unavailable'|'unsupported';
export interface CapabilityResult { state: CapabilityState; reason?: string; checkedAt?: string }
export interface Drama { id: string; title: string; cover: string; tags: string[]; summary?: string; episodeCount: number }
export interface Episode { id: string; dramaId: string; index: number; title?: string; durationMs?: number }
export interface PlaybackTicket { kind: 'demo'|'authorized'; episodeId: string; playbackUrl?: string; expiresAt?: string; restrictions?: string[] }
export interface Comment { id: string; episodeId: string; authorDisplay: string; text: string; createdAt: string; likes: number; replyCount: number }
export interface Danmaku { id: string; episodeId: string; offsetMs: number; text: string; likes: number }
export interface VideoProvider {
  capabilities(): Promise<Record<Capability, CapabilityResult>>;
  home(): Promise<Drama[]>;
  search(query: string, filters?: Record<string,string>): Promise<Drama[]>;
  drama(id: string): Promise<Drama>;
  episodes(dramaId: string): Promise<Episode[]>;
  resolvePlayback(episodeId: string): Promise<PlaybackTicket>;
  comments(episodeId: string, cursor?: string): Promise<{items: Comment[]; nextCursor?: string}>;
  danmaku(episodeId: string): Promise<Danmaku[]>;
}
