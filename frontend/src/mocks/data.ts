import type { Banner, Drama } from '@/types'
import { posterSvg } from './posters'

type Seed = Omit<Drama, 'cover'> & { hue: number; hue2: number; seed: number; style?: 'warm'|'cool'|'night'|'bright' }
const S: Seed[] = [
  { id: 'd1', title: '替嫁成了千亿总裁的心尖宠', tags: ['都市','甜宠','替嫁','先婚后爱'], kind: '真人剧', summary: '被迫替嫁的她，本是家族联姻的棋子，却意外与冷漠总裁假戏真做。婚后她步步沦陷，从不屑到深爱，这场替嫁，竟成了命中注定的宠爱。', episodeCount: 80, updatedTo: 32, finished: false, heat: 982.3, isNewToday: false, hue: 350, hue2: 20, seed: 1 },
  { id: 'd2', title: '暴君的替身医妃', tags: ['古装','甜宠'], kind: '真人剧', summary: '她是侯府庶女，被送入宫中为医。原以为是深宫囚笼，却在暴君的步步试探下，从一枚棋子变成了他唯一的挚爱。', episodeCount: 80, updatedTo: 80, finished: true, heat: 618.4, isNewToday: true, hue: 20, hue2: 40, seed: 2, style: 'warm' },
  { id: 'd3', title: '雨夜逢君', tags: ['都市','救赎'], kind: '真人剧', summary: '一场雨夜的偶遇，让两个孤独的灵魂彼此靠近。', episodeCount: 72, updatedTo: 72, finished: true, heat: 756.8, isNewToday: true, hue: 215, hue2: 200, seed: 3, style: 'night' },
  { id: 'd4', title: '重回高三：我把学霸追到手了', tags: ['青春','校园'], kind: '真人剧', summary: '重回十七岁，这一次她决定追到那个学霸。', episodeCount: 60, updatedTo: 60, finished: true, heat: 589.1, isNewToday: true, hue: 95, hue2: 160, seed: 4, style: 'bright' },
  { id: 'd5', title: '仙门小师妹今天也在逆天', tags: ['玄幻','逆袭'], kind: '漫剧', summary: '废柴师妹一夜觉醒，逆天改命。', episodeCount: 90, updatedTo: 90, finished: true, heat: 502.7, isNewToday: true, hue: 205, hue2: 270, seed: 5, style: 'cool' },
  { id: 'd6', title: '离婚后，我成了顶流导演', tags: ['都市','成长'], kind: '真人剧', summary: '离婚之后，她用作品征服了整个行业。', episodeCount: 68, updatedTo: 58, finished: false, heat: 533.4, isNewToday: true, hue: 30, hue2: 210, seed: 6, style: 'bright' },
  { id: 'd7', title: '天命之子：从废柴到神尊', tags: ['漫剧','玄幻','热血'], kind: '漫剧', summary: '被天道抛弃的少年，踏上一条逆天之路。', episodeCount: 100, updatedTo: 76, finished: false, heat: 468.1, isNewToday: true, hue: 245, hue2: 280, seed: 7, style: 'night' },
  { id: 'd8', title: '禁欲总裁追妻成瘾', tags: ['都市','甜宠'], kind: '真人剧', summary: '总裁追妻火葬场，一路甜到发齁。', episodeCount: 82, updatedTo: 82, finished: true, heat: 982.3, hue: 340, hue2: 15, seed: 8 },
  { id: 'd9', title: '重生后让渣男悔不当初', tags: ['都市','复仇'], kind: '真人剧', summary: '重生归来，她要让渣男悔不当初。', episodeCount: 76, updatedTo: 76, finished: true, heat: 756.8, hue: 330, hue2: 10, seed: 9 },
  { id: 'd10', title: '将门凰女', tags: ['古装','权谋'], kind: '真人剧', summary: '将门之女执掌天下，红妆亦可封侯。', episodeCount: 90, updatedTo: 90, finished: true, heat: 618.4, hue: 8, hue2: 30, seed: 10, style: 'warm' },
  { id: 'd11', title: '我的AI男友有点甜', tags: ['都市','科幻'], kind: 'AI剧', summary: '一场与AI男友的甜蜜实验。', episodeCount: 58, updatedTo: 40, finished: false, heat: 502.7, hue: 200, hue2: 320, seed: 11, style: 'cool' },
  { id: 'd12', title: '山河不负你', tags: ['古装','爱情'], kind: '真人剧', summary: '山河为聘，岁月为证。', episodeCount: 68, updatedTo: 68, finished: true, heat: 468.1, hue: 190, hue2: 30, seed: 12 },
  { id: 'd13', title: '替嫁新娘：被冷极大佬宠上天', tags: ['豪门','虐恋','替嫁','追妻火葬场'], kind: '真人剧', summary: '她替姐姐嫁给传闻中冷血无情的傅家大佬，原以为只是一场交易，却在朝夕相处中，被他步步温柔攻陷……', episodeCount: 72, updatedTo: 60, finished: false, heat: 756.8, hue: 215, hue2: 230, seed: 13, style: 'night' },
  { id: 'd14', title: '替嫁王妃：暴君的白月光', tags: ['古装','权谋','替嫁','宫廷'], kind: '真人剧', summary: '她是侯府庶女，因一纸替嫁被送入宫中。原以为是深宫囚笼，却在暴君的步步试探下，从替嫁棋子变成了他唯一的挚爱。', episodeCount: 90, updatedTo: 90, finished: true, heat: 618.4, hue: 18, hue2: 45, seed: 14, style: 'warm' },
  { id: 'd15', title: '替嫁仙妻：师尊他蓄谋已久', tags: ['仙侠','甜宠','替嫁','师徒'], kind: '漫剧', summary: '为救家族，她替嫡姐嫁给传闻清冷禁欲的上界师尊。却不知，这场替嫁从一开始，就是他酝酿千年的局……', episodeCount: 60, updatedTo: 48, finished: false, heat: 502.7, hue: 210, hue2: 285, seed: 15, style: 'cool' },
  { id: 'd16', title: 'AI替嫁：总裁的虚拟新娘', tags: ['都市','科技','替嫁','虚拟恋爱'], kind: 'AI剧', summary: '一场AI婚约实验，她以虚拟身份嫁入豪门，却意外获得总裁的真实偏爱。当虚拟与现实的界限逐渐模糊，谁才是这场替嫁的真正赢家？', episodeCount: 48, updatedTo: 40, finished: false, heat: 468.1, hue: 250, hue2: 300, seed: 16, style: 'night' },
]

export const DRAMAS: Drama[] = S.map(({ hue, hue2, seed, style, ...d }) => ({ ...d, cover: posterSvg({ hue, hue2, seed, style }) }))
export const byId = (id: string) => DRAMAS.find((d) => d.id === id)

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

export const RANKINGS: Record<'hot'|'rise'|'new', string[]> = {
  hot: ['d8', 'd9', 'd10', 'd11', 'd12'],
  rise: ['d6', 'd7', 'd5', 'd3', 'd4'],
  new: ['d2', 'd3', 'd4', 'd5', 'd6'],
}

export const HEAT_TEXT = (w: number) => `${w.toFixed(1)}万热度`
