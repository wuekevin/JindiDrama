import type { HistoryEntry } from '@/types'

export function resumePosition(checkpoint: HistoryEntry | undefined, episode: number, duration: number) {
  if (!checkpoint || checkpoint.episode !== episode || checkpoint.percent >= 100) return 0
  const previousPosition = checkpoint.positionSec ?? 0
  const previousDuration = checkpoint.durationSec ?? 0
  if (Number.isFinite(previousPosition) && previousPosition > 0) {
    if (previousPosition < duration) return previousPosition
    if (Number.isFinite(previousDuration) && previousPosition < previousDuration) {
      return (previousPosition / previousDuration) * duration
    }
  }
  const percent = Number.isFinite(checkpoint.percent) ? Math.min(99, Math.max(0, checkpoint.percent)) : 0
  return (percent / 100) * duration
}

export function seekablePosition(ranges: TimeRanges, position: number, currentTime: number) {
  let nearest = position
  let bestDistance = Infinity
  for (let index = 0; index < ranges.length; index++) {
    const candidate = Math.min(Math.max(position, ranges.start(index)), ranges.end(index))
    const distance = Math.abs(candidate - position)
    if (distance < bestDistance || (distance === bestDistance &&
      Math.abs(candidate - currentTime) < Math.abs(nearest - currentTime))) {
      nearest = candidate
      bestDistance = distance
    }
  }
  return nearest
}
