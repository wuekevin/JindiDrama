import type { Banner, Drama } from '@/types'
import { posterSvg } from './posters'
import rankings from '../../../internal/provider/mock/rankings.json'
import seeds from '../../../internal/provider/mock/catalog.json'
import cast from '../../../internal/provider/mock/cast.json'

export interface CastMember { name: string; role: string; hue: number }

const demoCast: Readonly<Record<string, readonly CastMember[] | undefined>> = cast
export const demoCastFor = (id: string) => demoCast[id] ?? []

type Seed = Omit<Drama, 'cover'> & { hue: number; hue2: number; seed: number; style?: 'warm'|'cool'|'night'|'bright'; series?: string; season?: number }
const S = seeds as Seed[]

export const DRAMAS: Drama[] = S.map(({ hue, hue2, seed, style, series, season, ...d }) => ({ ...d, cover: posterSvg({ hue, hue2, seed, style }) }))
export const byId = (id: string) => DRAMAS.find((d) => d.id === id)

export interface DemoSeason { number: number; drama: Drama }

export function demoSeasonsFor(current: Drama): DemoSeason[] {
  const series = S.find(({ id }) => id === current.id)?.series
  if (!series) return []
  return S.flatMap((entry) => {
    if (entry.series !== series || !entry.season) return []
    const drama = entry.id === current.id ? current : byId(entry.id)
    return drama ? [{ number: entry.season, drama }] : []
  }).sort((a, b) => a.number - b.number)
}

export const CATEGORIES = [
  { key: 'tianchong', label: '甜宠', icon: 'Flame', color: '#ff7a3d' },
  { key: 'nixi', label: '逆袭', icon: 'Crown', color: '#f4b740' },
  { key: 'dushi', label: '都市', icon: 'Heart', color: '#ff4d73' },
  { key: 'guzhuang', label: '古装', icon: 'Swords', color: '#b27cf5' },
  { key: 'xuanyi', label: '悬疑', icon: 'Ghost', color: '#7a8cf5' },
  { key: 'zhanshen', label: '战神', icon: 'Zap', color: '#ffc43d' },
  { key: 'chongsheng', label: '重生', icon: 'Sprout', color: '#6fcf7a' },
  { key: 'chuanyue', label: '穿越', icon: 'Infinity', color: '#4aa8ff' },
  { key: 'haomen', label: '豪门', icon: 'Building2', color: '#4a8cf5' },
]

export const BANNERS: Banner[] = [
  { id: 'b1', dramaId: 'd1', title: '替嫁成了\n千亿总裁的心尖宠', subtitle: '替嫁入豪门，本以为是交易，\n却被他宠成了全世界的例外…', tags: ['爆款短剧', '甜宠', '逆袭'], hue: 350, hue2: 20, seed: 21 },
  { id: 'b2', dramaId: 'd3', title: '雨夜逢君', subtitle: '一场雨夜的偶遇，\n让两个孤独的灵魂彼此靠近。', tags: ['新剧首发', '都市', '救赎'], hue: 215, hue2: 200, seed: 22 },
  { id: 'b3', dramaId: 'd10', title: '将门凰女', subtitle: '将门之女执掌天下，\n红妆亦可封侯。', tags: ['古装', '权谋'], hue: 8, hue2: 30, seed: 23 },
  { id: 'b4', dramaId: 'd5', title: '仙门小师妹\n今天也在逆天', subtitle: '废柴师妹一夜觉醒，\n逆天改命。', tags: ['漫剧', '玄幻'], hue: 205, hue2: 270, seed: 24 },
  { id: 'b5', dramaId: 'd6', title: '离婚后，\n我成了顶流导演', subtitle: '离婚之后，她用作品征服了整个行业。', tags: ['都市', '成长'], hue: 30, hue2: 210, seed: 25 },
]

export const RANKINGS = rankings

export const HEAT_TEXT = (w: number) => `${w.toFixed(1)}万热度`
