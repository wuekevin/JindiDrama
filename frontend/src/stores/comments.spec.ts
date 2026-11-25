import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCommentStore, MAX_LEN } from './comments'
describe('comments store', () => {
  beforeEach(() => { setActivePinia(createPinia()); useCommentStore().open('d1', 32) })
  it('validates', () => { const s = useCommentStore(); expect(s.validate('  ')).toContain('不能为空'); expect(s.validate('a'.repeat(MAX_LEN + 1))).toContain('500'); expect(s.validate('好看')).toBeNull() })
  it('posts to top, deletes only own', () => { const s = useCommentStore(); expect(s.post('我的评论')).toBeNull(); expect(s.visible[0].mine).toBe(true); const id = s.visible[1].id; s.remove(id); expect(s.visible.some((c) => c.id === id)).toBe(true); s.remove(s.visible[0].id); expect(s.visible.some((c) => c.mine)).toBe(false) })
  it('sort hot desc; author filter', () => { const s = useCommentStore(); s.sort = 'hot'; s.loadMore(); const l = s.visible.map((c) => c.likes); expect(l).toEqual([...l].sort((a, b) => b - a)); s.sort = 'author'; expect(s.visible.every((c) => c.byAuthor)).toBe(true) })
  it('like toggles; paging', () => { const s = useCommentStore(); const c = s.visible[0]; const n = c.likes; s.like(c); expect(c.likes).toBe(n + 1); s.like(c); expect(c.likes).toBe(n); expect(s.visible).toHaveLength(4); s.loadMore(); expect(s.visible).toHaveLength(8) })
  it('restores desktop comments and saves edits under their episode', async () => {
    const LocalComments = vi.fn().mockResolvedValue('')
    const SaveLocalComments = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('go', { app: { App: { LocalComments, SaveLocalComments } } })
    try {
      const store = useCommentStore()
      const ready = store.open('d1', 31)
      expect(store.loading).toBe(true)
      expect(store.post('尚未加载')).toContain('尚未载入')
      await ready
      expect(store.post('本地评论')).toBeNull()
      const saved = store.persist()
      await store.open('d1', 30)
      await saved
      const [id, episode, snapshot] = SaveLocalComments.mock.lastCall!
      expect([id, episode]).toEqual(['d1', 31])
      expect(JSON.parse(snapshot)[0].text).toBe('本地评论')

      setActivePinia(createPinia())
      LocalComments.mockResolvedValueOnce(snapshot)
      const reopened = useCommentStore()
      await reopened.open('d1', 31)
      expect(reopened.visible[0].text).toBe('本地评论')
    } finally {
      vi.unstubAllGlobals()
    }
  })
  it('rejects demo replies and replies from another episode', () => {
    const store = useCommentStore()
    const parent = store.visible[0]
    const demoReply = parent.replies[0]
    const originalTotal = parent.replyTotal
    expect(store.removeReply(parent.id, demoReply.id)).toBe(false)
    expect(parent.replies[0]).toBe(demoReply)
    expect(parent.replyTotal).toBe(originalTotal)

    expect(store.reply(parent, '自己的回复')).toBeNull()
    const ownReply = parent.replies.at(-1)!
    store.open('d1', 31)
    expect(store.removeReply(parent.id, ownReply.id)).toBe(false)
    store.open('d1', 32)
    expect(store.visible[0].replies).toContain(ownReply)
    expect(store.removeReply(parent.id, ownReply.id)).toBe(true)
    expect(store.removeReply(parent.id, ownReply.id)).toBe(false)
    expect(parent.replyTotal).toBe(originalTotal)
  })
  it('reply adds', () => { const s = useCommentStore(); const c = s.visible[0]; const n = c.replyTotal; expect(s.reply(c, '')).not.toBeNull(); expect(s.reply(c, '同意')).toBeNull(); expect(c.replyTotal).toBe(n + 1) })
})
