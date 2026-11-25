// 弹幕引擎：纯函数，便于测试。屏蔽规则、区域、密度、热评过滤、滚动位置全部在此计算。
export interface DanmakuItem { id: string; at: number; text: string; likes: number; hot?: boolean; lane: number; emoji?: string; mine?: boolean }
export type Area = 'full' | 'top' | 'bottom'
export type Density = 'low' | 'mid' | 'high'
export interface DanmakuSettings { enabled: boolean; area: Area; opacity: number; fontSize: number; speed: number; density: Density; hotOnly: boolean; blocked: string[] }
export const DEFAULT_DANMAKU: DanmakuSettings = { enabled: true, area: 'full', opacity: 80, fontSize: 14, speed: 1, density: 'mid', hotOnly: false, blocked: ['剧透', '广告', '骂人', '无关'] }
export const LANES = 12
export const BASE_DURATION = 8 // 秒，1.0x 下弹幕飞越屏幕时间

const hash = (s: string) => { let h = 0; for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0; return h }
export const isBlocked = (text: string, blocked: string[]) => blocked.some((k) => k && text.includes(k))
const densityKeep: Record<Density, number> = { low: 0.34, mid: 0.67, high: 1 }
export function laneRange(area: Area): [number, number] { return area === 'full' ? [0, LANES - 1] : area === 'top' ? [0, LANES / 2 - 1] : [LANES / 2, LANES - 1] }

export function filterDanmaku(items: DanmakuItem[], s: DanmakuSettings): DanmakuItem[] {
  if (!s.enabled) return []
  const [lo, hi] = laneRange(s.area)
  return items.filter((d) => !isBlocked(d.text, s.blocked) && (!s.hotOnly || d.hot || d.mine) && (d.mine || hash(d.id) % 100 < densityKeep[s.density] * 100))
    .map((d) => ({ ...d, lane: lo + (d.lane % (hi - lo + 1)) }))
}
export interface Placed { item: DanmakuItem; x: number; y: number } // x: 0=右缘外 …1=左缘外；y: 0..1
export function place(items: DanmakuItem[], t: number, s: DanmakuSettings): Placed[] {
  const dur = BASE_DURATION / s.speed
  return filterDanmaku(items, s).filter((d) => t >= d.at && t < d.at + dur).map((d) => ({ item: d, x: (t - d.at) / dur, y: (d.lane + 0.5) / LANES }))
}
export function genDanmaku(episode: number, durationSec: number): DanmakuItem[] {
  const pool = ['这也太甜了吧！', '男主好帅啊！', '终于在一起了！等了这么多集', '这眼神！我沦陷了…', '这对CP我嗑定了！', '女主好美，演技绝了！', '这段好有张力！', '编剧太会写了！', '替嫁的反转来了吗？！', '啊啊啊终于亲上了', '年度最佳名场面！', '求更新！根本不够看！', '总裁终于不装了！', '没想到替嫁还有这种反转！', '两个人的氛围感绝了', '这段我直接爆哭…', '我已经循环三遍了…', '这种拉扯感太上头了！', '前方高能', '剧透预警别说', '广告好烦', '无关的话题', '好想看下一集']
  const n = 600; const out: DanmakuItem[] = []
  for (let i = 0; i < n; i++) out.push({ id: `e${episode}-${i}`, at: (i / n) * durationSec, text: pool[(i * 7 + episode) % pool.length], likes: ((i * 37) % 50) * 100, hot: i % 5 === 0, lane: (i * 5 + episode) % LANES })
  return out
}
