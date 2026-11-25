import { describe, expect, it } from 'vitest'
import { transition, canTransition, fmt, episodeDuration } from './machine'
describe('player machine', () => {
  it('happy path', () => { let s = transition('idle', 'load'); expect(s).toBe('resolving'); s = transition(s, 'resolved'); expect(s).toBe('ready'); s = transition(s, 'play'); expect(s).toBe('playing'); s = transition(s, 'pause'); expect(s).toBe('paused'); s = transition(s, 'play'); s = transition(s, 'finish'); expect(s).toBe('ended') })
  it('ignores invalid events', () => { expect(transition('idle', 'play')).toBe('idle'); expect(canTransition('ended', 'pause')).toBe(false) })
  it('error and retry', () => { expect(transition('resolving', 'fail')).toBe('error'); expect(transition('error', 'retry')).toBe('resolving') })
  it('switching episode reloads from any live state', () => { for (const s of ['ready', 'playing', 'paused', 'ended'] as const) expect(transition(s, 'load')).toBe('resolving') })
  it('fmt/duration', () => { expect(fmt(192)).toBe('03:12'); expect(fmt(-3)).toBe('00:00'); expect(episodeDuration(32)).toBeGreaterThan(390) })
})
