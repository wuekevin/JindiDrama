// MockProvider：离线演示数据，所有数据来自 mocks/data.ts，永不访问网络。
import type { Capability, CapabilityState, Drama, HistoryEntry } from '@/types'
import { activeProviderName, hasBridge, loadHomeFeed, loadProviderDetail, searchProvider } from '@/bindings'
import { BANNERS, byId, CATEGORIES, demoCastFor, demoSeasonsFor, DRAMAS, RANKINGS, type CastMember, type DemoSeason } from './data'

const delay = (ms = 120) => new Promise((r) => setTimeout(r, ms))
export interface HomePayload { banners: typeof BANNERS; categories: typeof CATEGORIES; todayNew: Drama[]; rankings: Record<'hot'|'rise'|'new', Drama[]> }

export const mockProvider = {
  name: 'mock' as const,
  async capabilities(): Promise<Record<Capability, CapabilityState>> {
    return { catalog: 'mock', search: 'mock', playback: 'mock', account_login: 'not_authorized', comments_read: 'mock', comments_write: 'mock', danmaku_read: 'mock', danmaku_write: 'mock', favorites_sync: 'not_authorized', history_sync: 'not_authorized' }
  },
  async home(mode: 'normal'|'empty'|'error' = 'normal'): Promise<HomePayload> {
    await delay()
    if (mode === 'error') throw new Error('网络开小差了，请稍后重试')
    if (mode === 'empty') return { banners: [], categories: CATEGORIES, todayNew: [], rankings: { hot: [], rise: [], new: [] } }
    const feed = await loadHomeFeed()
    const pick = (ids: string[]) => ids.map(byId).filter((drama): drama is Drama => drama !== undefined)
    const rankings = feed?.rankings ?? RANKINGS
    const todayNew = feed ? pick(feed.todayNew) : DRAMAS.filter((d) => d.isNewToday)
    return { banners: BANNERS, categories: CATEGORIES, todayNew, rankings: { hot: pick(rankings.hot), rise: pick(rankings.rise), new: pick(rankings.new) } }
  },
}
const MIN = 60e3, DAY = 86400e3
export const DEFAULT_HISTORY: HistoryEntry[] = [
  { dramaId: 'd1', episode: 32, percent: 40, watchedAt: Date.now() - 5 * MIN },
  { dramaId: 'd3', episode: 18, percent: 25, watchedAt: Date.now() - 6 * MIN },
  { dramaId: 'd10', episode: 56, percent: 62, watchedAt: Date.now() - 7 * MIN },
  { dramaId: 'd6', episode: 12, percent: 18, watchedAt: Date.now() - 1 * DAY },
  { dramaId: 'd5', episode: 27, percent: 30, watchedAt: Date.now() - 1 * DAY - MIN },
  { dramaId: 'd4', episode: 60, percent: 100, watchedAt: Date.now() - 1 * DAY - 2 * MIN },
  { dramaId: 'd7', episode: 3, percent: 3, watchedAt: Date.now() - 1 * DAY - 3 * MIN },
  { dramaId: 'd2', episode: 80, percent: 100, watchedAt: Date.now() - 3 * DAY },
  { dramaId: 'd11', episode: 22, percent: 49, watchedAt: Date.now() - 5 * DAY },
  { dramaId: 'd12', episode: 15, percent: 22, watchedAt: Date.now() - 7 * DAY },
  { dramaId: 'd9', episode: 76, percent: 100, watchedAt: Date.now() - 10 * DAY },
  { dramaId: 'd8', episode: 82, percent: 100, watchedAt: Date.now() - 12 * DAY },
]

export interface SearchFilters { kind?: string; status?: 'finished'|'ongoing'|''; sort?: 'comprehensive'|'hot'|'new'|'rating'; castOnly?: boolean }
const SUGGESTIONS = ['替嫁', '替嫁新娘', '替嫁千金', '替嫁后总裁狠狠爱', '替嫁王妃', '替嫁成了他的白月光']
const matchesDemoCast = (id: string, query: string) => demoCastFor(id).some(({ name }) => name.includes(query))
export function suggestDemoSearch(query: string) {
  return [
    ...SUGGESTIONS.filter((title) => title.includes(query)),
    ...DRAMAS.filter(({ id }) => matchesDemoCast(id, query)).map(({ title }) => title),
  ]
}
async function usesDemoCatalog() {
  return !hasBridge() || (await activeProviderName()?.catch(() => undefined)) === 'mock'
}
export const HOT_WORDS: { word: string; heat: string }[] = [
  { word: '替嫁', heat: '1268.5万' }, { word: '替嫁新娘', heat: '982.3万' }, { word: '替嫁成了心尖宠', heat: '756.8万' }, { word: '豪门替嫁', heat: '618.4万' }, { word: '替嫁王妃', heat: '502.7万' },
  { word: '替嫁仙妻', heat: '468.1万' }, { word: '替嫁后总裁狠狠爱', heat: '389.6万' }, { word: '先婚后爱', heat: '345.2万' }, { word: '替嫁千金', heat: '310.8万' }, { word: '替嫁虐恋', heat: '268.9万' },
]
export async function searchDramas(q: string, f: SearchFilters = {}, mode: 'normal'|'error' = 'normal', card = byId): Promise<Drama[]> {
  await delay(150)
  if (mode === 'error') throw new Error('搜索服务暂时不可用，请稍后重试')
  const kw = q.trim()
  const desktopSearch = searchProvider(kw)
  // 桌面端由 Go 匹配目录，Vue 补上海报及其他展示字段。
  let list = desktopSearch
    ? (await desktopSearch).map(({ id }) => card(id)).filter((drama): drama is Drama => drama !== undefined)
    : DRAMAS.filter((d) => !kw || d.title.includes(kw) || d.tags.some((t) => t.includes(kw)) || d.summary.includes(kw) || matchesDemoCast(d.id, kw))
  if (f.castOnly) {
    if (!kw || !(await usesDemoCatalog())) return []
    list = list.filter(({ id }) => demoCastFor(id).some(({ name }) => name === kw))
  }
  if (f.kind) list = list.filter((d) => d.kind === f.kind)
  const local = !f.status || f.kind === '本地片库' ? list.filter((d) => d.kind === '本地片库') : []
  list = list.filter((d) => d.kind !== '本地片库')
  if (f.status === 'finished') list = list.filter((d) => d.finished)
  if (f.status === 'ongoing') list = list.filter((d) => !d.finished)
  const s = f.sort ?? 'comprehensive'
  if (s === 'hot' || s === 'comprehensive') list = [...list].sort((a, b) => b.heat - a.heat)
  if (s === 'new') list = [...list].sort((a, b) => Number(b.id.slice(1)) - Number(a.id.slice(1)))
  if (s === 'rating') list = [...list].sort((a, b) => (b.heat % 10) - (a.heat % 10))
  return [...list, ...local]
}

export interface DemoHighlight { id: number; title: string; dur: string }
export interface DramaDetail { drama: Drama; episodes: { index: number; available: boolean }[]; rating: number; plays: string; favs: string; cast: readonly CastMember[]; schedule: string; long: string; seasons: DemoSeason[]; highlights: DemoHighlight[] }
const DEMO_HIGHLIGHTS: DemoHighlight[] = [1, 2, 3].map((id) => ({ id, title: `演示片段 ${id}`, dur: '00:24' }))
export async function getDetail(id: string, mode: 'normal'|'error' = 'normal'): Promise<DramaDetail | null> {
  await delay(120)
  if (mode === 'error') throw new Error('详情加载失败，请稍后重试')
  const fixture = byId(id)
  if (!fixture) return null
  const [desktopDetail, isDemo] = await Promise.all([loadProviderDetail(id), usesDemoCatalog()])
  const d = desktopDetail ? { ...fixture, ...desktopDetail[0] } : fixture
  const episodes = desktopDetail?.[1] ?? Array.from({ length: d.episodeCount }, (_, i) => ({
    index: i + 1, available: i < d.updatedTo,
  }))
  const n = Number(id.slice(1))
  return {
    drama: d, episodes, rating: +(9.6 - (n % 5) * 0.2).toFixed(1), plays: `${(682.1 - n * 11.3).toFixed(1)}万播放`, favs: `${(128.6 - n * 3.1).toFixed(1)}万收藏`,
    cast: isDemo ? demoCastFor(d.id) : [], schedule: d.finished ? '已完结' : '连载中',
    long: d.summary + '原以为只是一场各取所需的交易，却在步步试探中，见他隐忍深情的真心。两人在权力与情感的漩涡里彼此救赎，终成命定良缘。',
    seasons: isDemo ? demoSeasonsFor(d) : [],
    highlights: DEMO_HIGHLIGHTS,
  }
}
export const epHot = (ep: number) => ep % 6 === 0
export function groupEpisodes(total: number, size = 20) { const g: { label: string; from: number; to: number }[] = []; for (let s = 1; s <= total; s += size) g.push({ label: `${s}-${Math.min(total, s + size - 1)}集`, from: s, to: Math.min(total, s + size - 1) }); return g }
