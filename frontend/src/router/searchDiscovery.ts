import type { LocationQuery } from 'vue-router'

export const feedLabels = { today: '今日上新', hot: '热门榜', rise: '飙升榜', new: '新剧榜' } as const

export function searchDiscovery(query: LocationQuery): keyof typeof feedLabels | 'topics' | undefined {
  if (String(query.q ?? '') || query.cast === '1') return undefined
  const feed = query.feed
  if (typeof feed === 'string' && Object.hasOwn(feedLabels, feed)) return feed as keyof typeof feedLabels
  if (query.browse === 'topics') return 'topics'
}
