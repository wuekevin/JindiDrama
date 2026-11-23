import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCommentStore, MAX_LEN } from './comments'
describe('comments store', () => {
  beforeEach(() => { setActivePinia(createPinia()); useCommentStore().open('d1', 32) })
  it('validates', () => { const s = useCommentStore(); expect(s.validate('  ')).toContain('不能为空'); expect(s.validate('a'.repeat(MAX_LEN + 1))).toContain('500'); expect(s.validate('好看')).toBeNull() })
  it('posts to top, deletes only own', () => { const s = useCommentStore(); expect(s.post('我的评论')).toBeNull(); expect(s.visible[0].mine).toBe(true); const id = s.visible[1].id; s.remove(id); expect(s.visible.some((c) => c.id === id)).toBe(true); s.remove(s.visible[0].id); expect(s.visible.some((c) => c.mine)).toBe(false) })
  it('sort hot desc; author filter', () => { const s = useCommentStore(); s.sort = 'hot'; s.loadMore(); const l = s.visible.map((c) => c.likes); expect(l).toEqual([...l].sort((a, b) => b - a)); s.sort = 'author'; expect(s.visible.every((c) => c.byAuthor)).toBe(true) })
  it('like toggles; paging', () => { const s = useCommentStore(); const c = s.visible[0]; const n = c.likes; s.like(c); expect(c.likes).toBe(n + 1); s.like(c); expect(c.likes).toBe(n); expect(s.visible).toHaveLength(4); s.loadMore(); expect(s.visible).toHaveLength(8) })
  it('reply adds', () => { const s = useCommentStore(); const c = s.visible[0]; const n = c.replyTotal; expect(s.reply(c, '')).not.toBeNull(); expect(s.reply(c, '同意')).toBeNull(); expect(c.replyTotal).toBe(n + 1) })
})
