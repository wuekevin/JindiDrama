import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

export interface Reply { id: string; user: string; level: number; text: string; likes: number; liked: boolean; ago: string; mine?: boolean }
export interface Comment { id: string; user: string; level: number; text: string; likes: number; liked: boolean; ago: string; ts: number; replies: Reply[]; replyTotal: number; byAuthor?: boolean; mine?: boolean }
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
  const key = ref('d1-1')
  const all = computed(() => (byEp.value[key.value] ??= gen(key.value)))
  function open(dramaId: string, ep: number) { key.value = `${dramaId}-${ep}`; shown.value = 4; expanded.value = new Set(); byEp.value[key.value] ??= gen(key.value) }
  const sorted = computed(() => { let l = [...all.value]; if (sort.value === 'author') l = l.filter((c) => c.byAuthor || c.mine); l.sort((a, b) => (sort.value === 'hot' ? b.likes - a.likes : b.ts - a.ts)); return l })
  const visible = computed(() => sorted.value.slice(0, shown.value))
  const total = computed(() => 3217 + all.value.filter((c) => c.mine).length)
  const hasMore = computed(() => shown.value < sorted.value.length)
  const loadMore = () => { shown.value += 4 }
  function validate(text: string): string | null { const t = text.trim(); if (!t) return '评论不能为空'; if (t.length > MAX_LEN) return `评论不能超过 ${MAX_LEN} 字`; return null }
  function post(text: string): string | null { const e = validate(text); if (e) return e; all.value.unshift({ id: `my-${Date.now()}`, user: '我', level: 1, text: text.trim(), likes: 0, liked: false, ago: '刚刚', ts: Date.now(), replies: [], replyTotal: 0, mine: true }); sort.value = 'new'; return null }
  function like(c: Comment | Reply) { c.liked = !c.liked; c.likes += c.liked ? 1 : -1 }
  function remove(id: string) { const l = all.value; const i = l.findIndex((c) => c.id === id && c.mine); if (i >= 0) l.splice(i, 1) }
  function reply(c: Comment, text: string): string | null { const e = validate(text); if (e) return e; c.replies.push({ id: `${c.id}-m${Date.now()}`, user: '我', level: 1, text: text.trim(), likes: 0, liked: false, ago: '刚刚', mine: true }); c.replyTotal++; return null }
  const toggleReplies = (id: string) => { const s = new Set(expanded.value); s.has(id) ? s.delete(id) : s.add(id); expanded.value = s }
  return { sort, visible, total, hasMore, loadMore, post, like, remove, reply, validate, open, expanded, toggleReplies }
})
