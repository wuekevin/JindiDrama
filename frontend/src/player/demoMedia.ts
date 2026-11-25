import type { MediaSource } from '../../../contracts/provider-contracts'

export const DEMO_QUALITIES = [
  { label: '流畅', detail: '240×426', id: 'sd' },
  { label: '高清', detail: '360×640', id: 'hd' },
] as const

export type DemoQuality = typeof DEMO_QUALITIES[number]['label']
export type DemoQualityId = typeof DEMO_QUALITIES[number]['id']

export function demoQualityId(quality: DemoQuality): DemoQualityId {
  return quality === '流畅' ? 'sd' : 'hd'
}

export function normalizeDemoQuality(value: unknown): DemoQuality {
  return value === '流畅' || value === '标清 480P' ? '流畅' : '高清'
}

/** Both formats of the repository's three rotating demo clips. */
export function demoSources(episode: number, quality: DemoQuality = '高清'): MediaSource[] {
  return demoClipSources((episode % 3) + 1, quality)
}

export function demoClipSources(clip: number, quality: DemoQuality = '高清'): MediaSource[] {
  const id = demoQualityId(quality)
  const base = `./demo/demo-${clip}${id === 'sd' ? '-sd' : ''}`
  return [
    { src: base + '.webm', type: 'video/webm', quality: id },
    { src: base + '.mp4', type: 'video/mp4', quality: id },
  ]
}
