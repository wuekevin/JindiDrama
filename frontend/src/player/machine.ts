// 播放器状态机：Idle → Resolving → Ready → Playing/Paused → Ended/Error
export type PState = 'idle' | 'resolving' | 'ready' | 'playing' | 'paused' | 'ended' | 'error'
export type PEvent = 'load' | 'resolved' | 'fail' | 'play' | 'pause' | 'finish' | 'retry' | 'reset'
const T: Record<PState, Partial<Record<PEvent, PState>>> = {
  idle: { load: 'resolving' },
  resolving: { resolved: 'ready', fail: 'error', reset: 'idle', load: 'resolving' },
  ready: { play: 'playing', pause: 'paused', load: 'resolving', fail: 'error' },
  playing: { pause: 'paused', finish: 'ended', fail: 'error', load: 'resolving' },
  paused: { play: 'playing', finish: 'ended', load: 'resolving', fail: 'error' },
  ended: { play: 'playing', load: 'resolving' },
  error: { retry: 'resolving', load: 'resolving' },
}
export function transition(s: PState, e: PEvent): PState { return T[s][e] ?? s }
export const canTransition = (s: PState, e: PEvent) => T[s][e] !== undefined
export const fmt = (sec: number) => { const s = Math.max(0, Math.floor(sec)); return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}` }
export const episodeDuration = (ep: number) => 6 * 60 + 30 + ((ep * 7) % 30)
