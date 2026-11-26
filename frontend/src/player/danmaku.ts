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
export interface DanmakuTrajectory {
  item: DanmakuItem
  y: number
  startsAt: number
  exitsAt: number
}

export interface DanmakuSchedule {
  trajectories: DanmakuTrajectory[]
  width: number
  velocity: number
}

export function scheduleDanmaku(
  items: DanmakuItem[],
  settings: DanmakuSettings,
  width: number,
  height: number,
  measureWidth: (item: DanmakuItem) => number,
): DanmakuSchedule {
  const velocity = width * 1.3 * settings.speed / BASE_DURATION
  const trajectories: DanmakuTrajectory[] = []
  const schedule = { trajectories, width, velocity }
  if (!Number.isFinite(velocity) || velocity <= 0 || !Number.isFinite(height) || height <= 0) return schedule

  const areaHeight = settings.area === 'full' ? height : height / 2
  const areaTop = settings.area === 'bottom' ? areaHeight : 0
  const linePitch = Math.ceil(settings.fontSize * 1.2) + 5
  const maxLanes = settings.area === 'full' ? LANES : LANES / 2
  const laneCount = Math.min(maxLanes, Math.floor(areaHeight / linePitch))
  if (!Number.isFinite(laneCount) || laneCount < 1) return schedule

  const lanes = Array.from({ length: laneCount }, (_, i) => ({
    y: (areaTop + (i + .5) * areaHeight / laneCount) / height,
    availableAt: -Infinity,
    reservations: [] as { start: number; clear: number }[],
    nextReservation: 0,
  }))
  const ordered = filterDanmaku(items, settings)
    .filter((item) => Number.isFinite(item.at) && item.at >= 0)
    .sort((a, b) => a.at - b.at)
  const clearance = (textWidth: number) => (textWidth + 8) / velocity
  const append = (item: DanmakuItem, textWidth: number, y: number, startsAt: number) => {
    const exitsAt = startsAt + (width + textWidth) / velocity
    trajectories.push({ item, y, startsAt, exitsAt })
  }

  // Reserve the user's own submissions first so a burst of regular comments cannot hide them.
  for (const item of ordered.filter((entry) => entry.mine)) {
    const lane = lanes.reduce((earliest, candidate) =>
      candidate.availableAt < earliest.availableAt ? candidate : earliest)
    const textWidth = measureWidth(item)
    const startsAt = Math.max(item.at, lane.availableAt)
    const clear = startsAt + clearance(textWidth)
    lane.reservations.push({ start: startsAt, clear })
    lane.availableAt = clear
    append(item, textWidth, lane.y, startsAt)
  }
  for (const lane of lanes) lane.availableAt = -Infinity

  for (const item of ordered.filter((entry) => !entry.mine)) {
    const textWidth = measureWidth(item)
    const clearsAt = item.at + clearance(textWidth)
    for (let i = 0; i < laneCount; i++) {
      const lane = lanes[(item.lane + i) % laneCount]
      while (lane.nextReservation < lane.reservations.length && lane.reservations[lane.nextReservation].clear <= item.at) {
        lane.nextReservation++
      }
      const reserved = lane.reservations[lane.nextReservation]
      if (lane.availableAt > item.at || (reserved && reserved.start < clearsAt)) continue
      append(item, textWidth, lane.y, item.at)
      lane.availableAt = clearsAt
      break
    }
  }
  return schedule
}

export function activeDanmaku(schedule: DanmakuSchedule, time: number) {
  return schedule.trajectories.filter(({ startsAt, exitsAt }) => time >= startsAt && time < exitsAt)
    .map(({ item, y, startsAt }) => ({ item, y, left: schedule.width - (time - startsAt) * schedule.velocity }))
}

export function genDanmaku(episode: number, durationSec: number): DanmakuItem[] {
  const pool = ['这也太甜了吧！', '男主好帅啊！', '终于在一起了！等了这么多集', '这眼神！我沦陷了…', '这对CP我嗑定了！', '女主好美，演技绝了！', '这段好有张力！', '编剧太会写了！', '替嫁的反转来了吗？！', '啊啊啊终于亲上了', '年度最佳名场面！', '求更新！根本不够看！', '总裁终于不装了！', '没想到替嫁还有这种反转！', '两个人的氛围感绝了', '这段我直接爆哭…', '我已经循环三遍了…', '这种拉扯感太上头了！', '前方高能', '剧透预警别说', '广告好烦', '无关的话题', '好想看下一集']
  const n = 600; const out: DanmakuItem[] = []
  for (let i = 0; i < n; i++) out.push({ id: `e${episode}-${i}`, at: (i / n) * durationSec, text: pool[(i * 7 + episode) % pool.length], likes: ((i * 37) % 50) * 100, hot: i % 5 === 0, lane: (i * 5 + episode) % LANES })
  return out
}
