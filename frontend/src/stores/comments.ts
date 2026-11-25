import { localId } from '@/localId'
import { defineStore } from 'pinia'
import { computed, reactive, ref, toRaw } from 'vue'
import { loadLocalComments, saveLocalComments } from '@/bindings'

export interface Reply {
  id: string
  text: string
  user?: string
  level?: number
  likes?: number
  liked?: boolean
  ago?: string
  mine?: boolean
}
export interface Comment extends Reply {
  ts?: number
  replies?: Reply[]
  replyTotal?: number
  byAuthor?: boolean
}

// Keep extension fields outside reactive state and preserve them on the next
// save. Missing display values stay absent in the snapshot.
const extensions = new WeakMap<Reply, Record<string, unknown>>()

function restoreReply({ id, text, user, level, likes, liked, ago, mine, ...extra }: Reply): Reply {
  const reply = { id, text, user, level, likes, liked, ago, mine }
  if (Object.keys(extra).length) extensions.set(reply, extra)
  return reply
}

function restoreComments(snapshot: string): Comment[] {
  const records = JSON.parse(snapshot) as Comment[]
  return records.map(({ ts, byAuthor, replies, replyTotal, ...entry }) => {
    const comment: Comment = restoreReply(entry)
    return Object.assign(comment, { ts, byAuthor, replies: replies?.map(restoreReply), replyTotal })
  })
}

function snapshotReply(reply: Reply): Reply {
  return { ...extensions.get(toRaw(reply)), ...reply }
}

function snapshotComments(comments: Comment[]): string {
  return JSON.stringify(comments.map((comment) => ({
    ...snapshotReply(comment),
    replies: comment.replies?.map(snapshotReply),
  })))
}

export const replyCount = (comment: Comment) => comment.replyTotal ?? comment.replies?.length ?? 0

export const MAX_LEN = 500
const USERS = ['晚风也温柔', '追剧小队长', '奶茶三分糖', '小橘子不吃糖', '星河漫漫', '今天也要追剧', '糖糖不吃糖', '是阿朗呀']
const TEXTS = ['这一集真的甜哭了！男主终于表白了，眼神里的爱意藏都藏不住，替嫁线也越来越精彩了！', '剧情节奏好紧凑！女主也不是傻白甜，智商在线，两人互相试探的戏码太带感了！', '我就知道替嫁不是简单的交易，肯定有更大的反转，坐等下一集揭晓！', '这对CP我嗑死了，求别分开！', '配乐太绝了，每次一响起就起鸡皮疙瘩。', '男二到底什么时候出场啊，等好久了。', '第几集开始变好看的？我要二刷。', '服化道好用心，古风细节满分。', '今天又熬夜追剧了，明天不上班就好了。', '这段对白可以直接背下来了。', '导演太会拍了，雨夜那场戏绝了。', '求更新！根本不够看！']
function gen(key: string): Comment[] {
  return TEXTS.map((text, i) => ({ id: `${key}-c${i}`, user: USERS[i % USERS.length], level: 3 + ((i * 3) % 4), text, likes: [18000, 9847, 6320, 4210, 3105, 2260, 1820, 1203, 960, 540, 302, 88][i], liked: false, ago: ['2小时前', '4小时前', '5小时前', '6小时前', '8小时前', '10小时前', '昨天', '昨天', '2天前', '3天前', '4天前', '5天前'][i], ts: Date.now() - (i + 1) * 3600e3, byAuthor: i % 4 === 1,
    replyTotal: [326, 118, 95, 40, 22, 9, 6, 3, 2, 1, 0, 0][i], replies: i < 3 ? [{ id: `${key}-c${i}-r0`, user: '小橘子不吃糖', level: 3, text: '+1！这段对我来说反复看了好几遍，太上头了！', likes: 2863, liked: false, ago: '1小时前' }, { id: `${key}-c${i}-r1`, user: '星河漫漫', level: 4, text: '同感！感觉男主早就动心了，终于不再克制～', likes: 1201, liked: false, ago: '57分钟前' }] : [] }))
}
export const useCommentStore = defineStore('comments', () => {
  const byEp = ref<Record<string, Comment[]>>({})
  const sort = ref<'new'|'hot'|'author'>('new'); const shown = ref(4); const expanded = ref<Set<string>>(new Set())
  const episode = ref({ dramaId: 'd1', index: 1 })
  const key = computed(() => `${episode.value.dramaId}-${episode.value.index}`)
  const drafts = new Map<string, { text: string; replies: Partial<Record<string, string>> }>()
  function draftForEpisode() {
    const current = key.value
    const saved = drafts.get(current)
    if (saved) return saved
    const draft = reactive({ text: '', replies: Object.create(null) as Partial<Record<string, string>> })
    drafts.set(current, draft)
    return draft
  }
  const pending = ref<Record<string, boolean>>({})
  const readErrors = ref<Record<string, boolean>>({})
  const saveErrors = ref<Record<string, boolean>>({})
  const all = computed(() => byEp.value[key.value] ?? [])
  const loading = computed(() => !!pending.value[key.value])
  const readError = computed(() => !!readErrors.value[key.value])
  const saveError = computed(() => !!saveErrors.value[key.value])
  const canEdit = () => !loading.value && !readError.value && !!byEp.value[key.value]
  function load() {
    const current = key.value
    if (byEp.value[current] || pending.value[current]) return
    readErrors.value[current] = false
    const request = loadLocalComments(episode.value.dramaId, episode.value.index)
    if (!request) {
      byEp.value[current] = gen(current)
      return
    }
    pending.value[current] = true
    return request.then((snapshot) => {
      byEp.value[current] = snapshot ? restoreComments(snapshot) : gen(current)
    }).catch(() => {
      readErrors.value[current] = true
    }).finally(() => {
      pending.value[current] = false
    })
  }
  function open(dramaId: string, ep: number) {
    episode.value = { dramaId, index: ep }
    shown.value = 4
    expanded.value = new Set()
    return load()
  }
  async function persist() {
    if (!canEdit()) return
    const current = key.value
    const { dramaId, index } = episode.value
    const snapshot = snapshotComments(all.value)
    try {
      await saveLocalComments(dramaId, index, snapshot)
      saveErrors.value[current] = false
    } catch {
      saveErrors.value[current] = true
    }
  }
  const sorted = computed(() => { let l = [...all.value]; if (sort.value === 'author') l = l.filter((c) => c.byAuthor || c.mine); l.sort((a, b) => (sort.value === 'hot' ? (b.likes ?? 0) - (a.likes ?? 0) : (b.ts ?? 0) - (a.ts ?? 0))); return l })
  const visible = computed(() => sorted.value.slice(0, shown.value))
  const total = computed(() => 3217 + all.value.filter((c) => c.mine).length)
  const hasMore = computed(() => shown.value < sorted.value.length)
  const loadMore = () => { shown.value += 4 }
  function validate(text: string): string | null { const t = text.trim(); if (!t) return '评论不能为空'; if (t.length > MAX_LEN) return `评论不能超过 ${MAX_LEN} 字`; return null }
  function post(text: string): string | null { if (!canEdit()) return '本集评论尚未载入'; const e = validate(text); if (e) return e; all.value.unshift({ id: `my-${localId()}`, user: '我', level: 1, text: text.trim(), likes: 0, liked: false, ago: '刚刚', ts: Date.now(), replies: [], replyTotal: 0, mine: true }); sort.value = 'new'; void persist(); return null }
  function like(c: Comment | Reply) { if (!canEdit()) return; c.liked = !c.liked; c.likes = Math.max(0, (c.likes ?? 0) + (c.liked ? 1 : -1)); void persist() }
  function remove(id: string) { if (!canEdit()) return; const l = all.value; const i = l.findIndex((c) => c.id === id && c.mine); if (i >= 0) { l.splice(i, 1); void persist() } }
  function reply(c: Comment, text: string): string | null {
    if (!canEdit()) return '本集评论尚未载入'
    const error = validate(text)
    if (error) return error
    const count = replyCount(c)
    if (count >= Number.MAX_SAFE_INTEGER) return '回复数量已达上限'
    c.replyTotal = count + 1
    if (!c.replies) c.replies = []
    c.replies.push({ id: `${c.id}-m${localId()}`, user: '我', level: 1, text: text.trim(), likes: 0, liked: false, ago: '刚刚', mine: true })
    void persist()
    return null
  }

  function removeReply(commentId: string, replyId: string): boolean {
    if (!canEdit()) return false
    const comment = all.value.find(({ id }) => id === commentId)
    if (!comment?.replies) return false
    const index = comment.replies.findIndex(({ id, mine }) => id === replyId && mine === true)
    if (index < 0) return false
    comment.replyTotal = replyCount(comment) - 1
    comment.replies.splice(index, 1)
    void persist()
    return true
  }

  const toggleReplies = (id: string) => { const s = new Set(expanded.value); s.has(id) ? s.delete(id) : s.add(id); expanded.value = s }
  return { draftForEpisode, sort, visible, total, hasMore, loadMore, post, like, remove, reply, removeReply, validate, open, loading, readError, saveError, load, persist, expanded, toggleReplies }
})
