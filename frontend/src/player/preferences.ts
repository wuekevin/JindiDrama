export const PLAY_MODES = ['连续播放（自动下一集）', '单集播放', '单集循环'] as const
export type PlayMode = typeof PLAY_MODES[number]

export function normalizePlayMode(value: unknown): PlayMode {
  return PLAY_MODES.find((mode) => mode === value) ?? PLAY_MODES[0]
}

export function isPlaybackSpeed(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0.5 && value <= 2
}

export function playbackEndAction(mode: PlayMode, hasNext: boolean): 'loop' | 'next' | 'stop' {
  if (mode === '单集循环') return 'loop'
  if (mode === '连续播放（自动下一集）' && hasNext) return 'next'
  return 'stop'
}
