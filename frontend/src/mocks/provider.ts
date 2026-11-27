// 目录展示数据来自 Go Provider。浏览器预览使用内置演示目录。
import type { ObjectDirective } from 'vue'
import type { Drama, HistoryEntry } from '@/types'
import { isAppleCMSDramaId } from '@/catalogIdentity'
import { activeProviderName, hasCatalogPlayback, hasProviderCapabilities, loadCatalogDrama, loadCatalogGenrePage, loadCatalogGenres, loadCatalogPage, loadHomeFeed, loadProviderCapabilities, loadProviderCatalog, loadProviderDetail, searchActiveCatalog, searchProvider, type CatalogDrama, type CatalogPage } from '@/bindings'
import { BANNERS, byId, CATEGORIES, demoCastFor, demoSeasonsFor, DRAMAS, RANKINGS, type CastMember, type DemoSeason } from './data'
import { posterSvg } from './posters'

const delay = (ms = 120) => new Promise((r) => setTimeout(r, ms))
export interface HomePayload { catalogLabel?: string; banners: typeof BANNERS; categories: typeof CATEGORIES; todayNew: Drama[]; rankings: Record<'hot'|'rise'|'new', Drama[]> }

export const catalogPlaceholderCover = posterSvg({ hue: 218, hue2: 229, seed: 4 })
const catalogCoverURL = /^http:\/\/127\.0\.0\.1:([1-9][0-9]{0,4})\/catalog-cover\/[0-9a-f]{32}$/

function isCatalogCover(value: unknown): value is string {
  const match = typeof value === 'string' ? catalogCoverURL.exec(value) : null
  if (!match) return false
  const port = Number(match[1])
  return port !== 80 && port <= 65535
}

export const vCancelCatalogCover: ObjectDirective<HTMLImageElement> = {
  beforeUnmount(image) {
    if (isCatalogCover(image.getAttribute('src'))) image.removeAttribute('src')
  },
}

export function fallbackCatalogCover(event: Event) {
  const image = event.currentTarget
  if (image instanceof HTMLImageElement && isCatalogCover(image.getAttribute('src'))) {
    image.src = catalogPlaceholderCover
  }
}

export async function catalogProviderName() {
  const name = activeProviderName()
  return name ?? (hasProviderCapabilities() ? undefined : 'mock')
}

export async function usesDemoCatalog() {
  return (await catalogProviderName()) === 'mock'
}

async function requireCapability(capability: 'catalog' | 'search') {
  const capabilities = await loadProviderCapabilities()
  if (capabilities?.[capability] !== 'ready') {
    throw new Error(capability === 'catalog' ? '目录暂不可用' : '搜索暂不可用')
  }
  return capabilities
}

function providerCard(drama: CatalogDrama): Drama {
  const count = Math.max(0, drama.episodeCount)
  return {
    id: drama.id, title: drama.title, summary: drama.summary, tags: drama.tags ?? [],
    episodeCount: count, updatedTo: Math.max(0, Math.min(count, drama.updatedTo)),
    cover: isCatalogCover(drama.cover) ? drama.cover : catalogPlaceholderCover, providerOnly: true, heat: 0, finished: false,
  }
}

function providerNotFound(error: unknown) {
  return (error instanceof Error ? error.message : error) === 'provider: 未找到'
}

export const MAX_CMS_PAGE = 100_000

export async function loadCMSCatalogPage(query: string, page: number, mode: 'normal' | 'error') {
  if (mode === 'error') throw new Error('苹果 CMS 目录暂不可用')
  const result = await loadCatalogPage(query.trim(), page)
  return cmsPageResult(result, page)
}

export async function loadCMSCatalogGenres(mode: 'normal' | 'error') {
  try {
    if (mode === 'error') throw new Error()
    await requireCapability('catalog')
    const genres = await loadCatalogGenres()
    if (!Array.isArray(genres) || genres.some((genre) => !genre || typeof genre.id !== 'string' ||
      !/^[1-9][0-9]{0,29}$/.test(genre.id) || typeof genre.name !== 'string' || !genre.name)) throw new Error()
    return genres
  } catch {
    throw new Error('苹果 CMS 分类暂不可用')
  }
}

export async function loadCMSCatalogGenrePage(genre: string, page: number, mode: 'normal' | 'error') {
  try {
    if (mode === 'error') throw new Error()
    return cmsPageResult(await loadCatalogGenrePage(genre, page), page)
  } catch {
    throw new Error('苹果 CMS 分类暂不可用')
  }
}

export async function loadCMSRelatedDramas(id: string, genre: string) {
  const { entries } = await loadCMSCatalogGenrePage(genre, 1, 'normal')
  const source = id.slice(0, id.lastIndexOf(':') + 1)
  const seen = new Set([id])
  return entries.filter((entry) => {
    if (!isAppleCMSDramaId(entry.id) || !entry.id.startsWith(source) || !entry.updatedTo || seen.has(entry.id)) return false
    seen.add(entry.id)
    return true
  }).slice(0, 6)
}

function cmsPageResult(result: CatalogPage | undefined, page: number) {
  if (!result || result.page !== page || !Array.isArray(result.entries) || typeof result.hasMore !== 'boolean') {
    throw new Error('苹果 CMS 目录响应无效')
  }
  // The provider can filter every title on an otherwise valid page.
  return { entries: result.entries.map(providerCard), hasMore: result.hasMore }
}

export async function loadCatalogHome(mode: 'normal'|'empty'|'error' = 'normal'): Promise<HomePayload> {
  await delay()
  if (mode === 'error') throw new Error('网络开小差了，请稍后重试')
  if (mode === 'empty') return { banners: [], categories: [], todayNew: [], rankings: { hot: [], rise: [], new: [] } }
  const demo = await usesDemoCatalog()
  let card = byId
  let feed
  if (!demo) {
    await requireCapability('catalog')
    const catalog = loadProviderCatalog()
    if (!catalog) throw new Error('目录暂不可用')
    const [records, selection] = await Promise.all([catalog, loadHomeFeed()])
    if (!selection) throw new Error('目录暂不可用')
    feed = selection
    const cards = new Map((records ?? []).filter(({ id }) => id).map((record) => [record.id, providerCard(record)]))
    if (feed.catalogLabel) {
      return { catalogLabel: feed.catalogLabel, banners: [], categories: [], todayNew: [...cards.values()], rankings: { hot: [], rise: [], new: [] } }
    }
    const referenced = new Set([feed.todayNew, feed.rankings.hot, feed.rankings.rise, feed.rankings.new].flatMap((ids) => ids ?? []))
    await Promise.all([...referenced].filter((id) => !cards.has(id)).map(async (id) => {
      try {
        const record = await loadCatalogDrama(id)
        if (record?.id === id) cards.set(id, providerCard(record))
      } catch (e) {
        if (!providerNotFound(e)) throw e
      }
    }))
    card = (id) => cards.get(id)
  } else {
    feed = await loadHomeFeed()
  }
  const pick = (ids: string[] | null) => (ids ?? []).map(card).filter((drama): drama is Drama => drama !== undefined)
  const rankings = feed?.rankings ?? RANKINGS
  const todayNew = feed ? pick(feed.todayNew) : DRAMAS.filter((d) => d.isNewToday)
  return {
    banners: demo ? BANNERS : [], categories: demo ? CATEGORIES : [], todayNew,
    rankings: { hot: pick(rankings.hot), rise: pick(rankings.rise), new: pick(rankings.new) },
  }
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
async function queryDesktopCatalog(query: string, demo: boolean) {
  if (!demo) await requireCapability('search')
  const response = demo ? searchProvider(query) : searchActiveCatalog(query)
  if (!response && !demo) throw new Error('搜索暂不可用')
  return response ? (await response) ?? [] : undefined
}

export async function catalogSuggestions(query: string) {
  const results = await queryDesktopCatalog(query, await usesDemoCatalog())
  return results ? results.map(({ title }) => title) : suggestDemoSearch(query)
}
export const HOT_WORDS: { word: string; heat: string }[] = [
  { word: '替嫁', heat: '1268.5万' }, { word: '替嫁新娘', heat: '982.3万' }, { word: '替嫁成了心尖宠', heat: '756.8万' }, { word: '豪门替嫁', heat: '618.4万' }, { word: '替嫁王妃', heat: '502.7万' },
  { word: '替嫁仙妻', heat: '468.1万' }, { word: '替嫁后总裁狠狠爱', heat: '389.6万' }, { word: '先婚后爱', heat: '345.2万' }, { word: '替嫁千金', heat: '310.8万' }, { word: '替嫁虐恋', heat: '268.9万' },
]
export async function searchDramas(q: string, f: SearchFilters = {}, mode: 'normal'|'error' = 'normal', card = byId): Promise<Drama[]> {
  await delay(150)
  if (mode === 'error') throw new Error('搜索服务暂时不可用，请稍后重试')
  const kw = q.trim()
  const demo = await usesDemoCatalog()
  const desktopSearch = await queryDesktopCatalog(kw, demo)
  let list = desktopSearch
    ? desktopSearch.map((record) => demo ? card(record.id) : providerCard(record))
      .filter((drama): drama is Drama => drama !== undefined)
    : DRAMAS.filter((d) => !kw || d.title.includes(kw) || d.tags.some((t) => t.includes(kw)) || d.summary.includes(kw) || matchesDemoCast(d.id, kw))
  if (f.castOnly) {
    if (!kw || !demo) return []
    list = list.filter(({ id }) => demoCastFor(id).some(({ name }) => name === kw))
  }
  if (!demo) return list
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
export interface DramaDetail { drama: Drama; episodes: { id?: string; index: number; available: boolean }[]; catalogPlayback?: boolean; appleCMS?: boolean; catalogGenreID?: string; catalogCredits?: { cast?: string[]; directors?: string[] }; rating?: number; plays?: string; favs?: string; cast: readonly CastMember[]; schedule?: string; long: string; seasons: DemoSeason[]; highlights: DemoHighlight[] }
const DEMO_HIGHLIGHTS: DemoHighlight[] = [1, 2, 3].map((id) => ({ id, title: `演示片段 ${id}`, dur: '00:24' }))
export async function getDetail(id: string, mode: 'normal'|'error' = 'normal', catalogRoute = false): Promise<DramaDetail | null> {
  await delay(120)
  if (mode === 'error') throw new Error('详情加载失败，请稍后重试')
  const provider = await catalogProviderName()
  const demo = provider === 'mock'
  if (catalogRoute) {
    const appleCMS = isAppleCMSDramaId(id)
    if (demo || (appleCMS && provider !== 'apple-cms')) return null
    const capabilities = await requireCapability('catalog')
    try {
      const detail = await loadProviderDetail(id, true)
      if (!detail) throw new Error('目录暂不可用')
      const [record, episodes, metadata] = detail
      if (record?.id !== id) return null
      const drama = providerCard(record)
      const genre = metadata?.genreId
      const credits = metadata?.cast?.length || metadata?.directors?.length
        ? { cast: metadata.cast, directors: metadata.directors } : undefined
      return {
        drama, episodes: (episodes ?? []).filter(({ dramaId, index }) =>
          dramaId === id && Number.isInteger(index) && index > 0 && index <= drama.episodeCount,
        ).map((episode) => ({ ...episode, available: episode.available && episode.index <= drama.updatedTo })),
        catalogPlayback: capabilities.playback === 'ready' && hasCatalogPlayback(),
        appleCMS,
        catalogGenreID: typeof genre === 'string' && /^[1-9][0-9]{0,29}$/.test(genre) ? genre : undefined,
        catalogCredits: credits,
        cast: [], long: record.summary, seasons: [], highlights: [],
      }
    } catch (error) {
      if (providerNotFound(error)) return null
      throw error
    }
  }
  if (!demo) return null
  const fixture = byId(id)
  if (!fixture) return null
  const desktopDetail = await loadProviderDetail(id)
  const d = desktopDetail ? { ...fixture, ...desktopDetail[0], cover: fixture.cover } : fixture
  const episodes = desktopDetail?.[1] ?? Array.from({ length: d.episodeCount }, (_, i) => ({
    index: i + 1, available: i < d.updatedTo,
  }))
  const n = Number(id.slice(1))
  return {
    drama: d, episodes, rating: +(9.6 - (n % 5) * 0.2).toFixed(1), plays: `${(682.1 - n * 11.3).toFixed(1)}万播放`, favs: `${(128.6 - n * 3.1).toFixed(1)}万收藏`,
    cast: demoCastFor(d.id), schedule: d.finished ? '已完结' : '连载中',
    long: d.summary + '原以为只是一场各取所需的交易，却在步步试探中，见他隐忍深情的真心。两人在权力与情感的漩涡里彼此救赎，终成命定良缘。',
    seasons: demoSeasonsFor(d),
    highlights: DEMO_HIGHLIGHTS,
  }
}
export const epHot = (ep: number) => ep % 6 === 0
export function groupEpisodes(total: number, size = 20) { const g: { label: string; from: number; to: number }[] = []; for (let s = 1; s <= total; s += size) g.push({ label: `${s}-${Math.min(total, s + size - 1)}集`, from: s, to: Math.min(total, s + size - 1) }); return g }
