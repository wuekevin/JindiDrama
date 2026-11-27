<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { X, Smile, ThumbsUp, MessageCircle, Ellipsis, Crown, ChevronRight } from 'lucide-vue-next'
import { useCommentStore, MAX_LEN, replyCount, type Comment } from '@/stores/comments'
import { useToastStore } from '@/stores/toast'
import { EmptyState } from '@/components/ui'
const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ localOnly?: boolean }>()
const c = useCommentStore(); const toast = useToastStore()
const err = ref('')
const replyFor = ref<string | null>(null)
const menuFor = ref<string | null>(null)
let menuTrigger: HTMLButtonElement | undefined
watch(() => c.saveConflict, () => { menuFor.value = null })
const draft = c.draftForEpisode()
const editor = ref<HTMLTextAreaElement>()
const emojiTrigger = ref<HTMLButtonElement>()
const emojiChoices = ref<HTMLElement>()
const emojiOpen = ref(false)
const editorComposing = ref(false)
const emojis = [
  { label: '微笑', value: '😊' },
  { label: '喜欢', value: '💖' },
  { label: '笑哭', value: '😂' },
  { label: '鼓掌', value: '👏' },
  { label: '惊讶', value: '😮' },
  { label: '流泪', value: '😢' },
  { label: '期待', value: '✨' },
  { label: '赞', value: '👍' },
]
const tabs = [{ k: 'new', l: '最新' }, { k: 'hot', l: '最热' }, { k: 'author', l: '只看作者' }] as const
const sorts = computed(() => props.localOnly ? tabs.filter(({ k }) => k !== 'author') : tabs)
const n = (v: number) => (v >= 10000 ? (v / 10000).toFixed(1) + '万' : String(v))
const over = computed(() => draft.text.length > MAX_LEN)
const composing = (event?: KeyboardEvent) => event?.isComposing || event?.keyCode === 229
function closeEmojiPicker() {
  emojiOpen.value = false
  emojiTrigger.value?.focus()
}
async function toggleEmojiPicker() {
  if (emojiOpen.value) {
    closeEmojiPicker()
    return
  }
  menuFor.value = null
  emojiOpen.value = true
  await nextTick()
  if (emojiOpen.value) emojiChoices.value?.querySelector('button')?.focus()
}
async function insertEmoji(value: string) {
  const input = editor.value
  if (!input || editorComposing.value || c.draftForEpisode() !== draft) return
  const { selectionStart: start, selectionEnd: end } = input
  const updated = draft.text.slice(0, start) + value + draft.text.slice(end)
  const fits = updated.length <= MAX_LEN
  if (fits) draft.text = updated
  err.value = fits ? '' : `评论不能超过 ${MAX_LEN} 字`
  emojiOpen.value = false
  await nextTick()
  editor.value?.focus()
  if (fits) editor.value?.setSelectionRange(start + value.length, start + value.length)
}
function closePickerOnFocusExit(event: FocusEvent) {
  const panel = event.currentTarget
  const next = event.relatedTarget
  if (panel instanceof HTMLElement && (!(next instanceof Node) || !panel.contains(next))) {
    emojiOpen.value = false
  }
}
function closeOnEscape(event: KeyboardEvent) {
  if (composing(event)) return
  if (emojiOpen.value) {
    event.stopPropagation()
    closeEmojiPicker()
    return
  }
  if (menuFor.value !== null) {
    event.stopPropagation()
    menuFor.value = null
    menuTrigger?.focus()
    return
  }
  emit('close')
}
function toggleCommentMenu(id: string, event: MouseEvent) {
  menuFor.value = menuFor.value === id ? null : id
  if (menuFor.value !== null) emojiOpen.value = false
  menuTrigger = event.currentTarget as HTMLButtonElement
}
function closeCommentMenuOnFocusExit(event: FocusEvent) {
  const menu = event.currentTarget as HTMLElement
  const next = event.relatedTarget
  if (!(next instanceof Node) || !menu.contains(next)) menuFor.value = null
}
function submit(event?: KeyboardEvent) {
  if (composing(event) || c.draftForEpisode() !== draft) return
  const error = c.post(draft.text)
  err.value = error ?? ''
  if (!error) {
    draft.text = ''
    toast.push(props.localOnly ? '已添加本机评论' : '评论已发布（本地演示，不会同步到红果）', 'success')
  }
}
function sendReply(cm: Comment, event?: KeyboardEvent) {
  if (composing(event) || replyFor.value !== cm.id || c.draftForEpisode() !== draft) return
  const error = c.reply(cm, draft.replies[cm.id] ?? '')
  if (error) {
    toast.push(error, 'error')
    return
  }
  delete draft.replies[cm.id]
  replyFor.value = null
  if (!c.expanded.has(cm.id)) c.toggleReplies(cm.id)
}
async function deleteComment(id: string) {
  if (!c.remove(id)) return
  menuFor.value = null
  toast.push('评论已删除')
  await nextTick()
  editor.value?.focus()
}
async function deleteReply(commentId: string, replyId: string, event: MouseEvent) {
  const trigger = (event.currentTarget as HTMLButtonElement).closest('article')?.querySelector<HTMLButtonElement>('.reply-button')
  if (!c.removeReply(commentId, replyId)) return
  await nextTick()
  trigger?.focus()
}
</script>
<template>
  <aside class="cp on-dark" role="complementary" :aria-label="localOnly ? '本机评论' : '本集评论'" tabindex="-1" data-testid="comment-panel"
    @keydown.esc="closeOnEscape" @focusout="closePickerOnFocusExit">
    <header><h2>{{ localOnly ? '本机评论' : '本集评论' }} <small data-testid="comment-total">({{ c.total.toLocaleString() }})</small></h2><button aria-label="关闭评论" data-testid="comment-close" @click="emit('close')"><X :size="24" /></button></header>
    <div class="sorts"><button v-for="t in sorts" :key="t.k" :class="{ on: c.sort === t.k }" :data-testid="`csort-${t.k}`" @click="c.sort = t.k">{{ t.l }}</button></div>
    <div class="editor"><i class="av" />
      <div class="box"><textarea ref="editor" v-model="draft.text" rows="2" placeholder="说说你对这一集的看法吧～" aria-label="发表评论" data-testid="comment-input"
        @input="err = ''" @compositionstart="editorComposing = true" @compositionend="editorComposing = false"
        @change="editorComposing = false" @keydown.ctrl.enter="submit" @keydown.meta.enter="submit" /></div></div>
    <div class="tools"><button ref="emojiTrigger" aria-label="表情" :aria-expanded="emojiOpen" aria-controls="comment-emoji-choices" @mousedown.prevent @click="toggleEmojiPicker"><Smile :size="22" /></button>
      <span class="cnt" :class="{ over }" data-testid="comment-count">{{ draft.text.length }}/{{ MAX_LEN }}</span><button class="post" :disabled="!draft.text.trim() || over || c.loading || c.readError || c.saveConflict" data-testid="comment-post" @click="submit()">发表评论</button></div>
    <div v-show="emojiOpen" id="comment-emoji-choices" ref="emojiChoices" class="emoji-choices" role="group" aria-label="选择表情">
      <button v-for="emoji in emojis" :key="emoji.value" type="button" :aria-label="emoji.label" @mousedown.prevent @click="insertEmoji(emoji.value)">{{ emoji.value }}</button>
    </div>
    <p v-if="err" class="err" role="alert">{{ err }}</p>
    <p v-if="c.saveConflict" class="err" role="alert">本集评论已在其他位置更新，暂缓补写本机未确认的编辑。 <button @click="c.discardPending()">放弃待补写编辑</button></p>
    <p v-if="c.saveError" class="err" role="alert">本地评论保存失败{{ c.saveNotDurable ? '，更改只保留在当前窗口，退出会丢失。' : '' }} <button @click="c.persist()">重试</button></p>
    <div class="list">
      <details v-if="c.legacyBackups.length" class="comment-backups">
        <summary>查看旧版待保存评论</summary>
        <p>这些暂存的数据库归属无法确认。可复制内容，或在确认归属后恢复到尚未保存评论的数据库。</p>
        <div v-for="backup in c.legacyBackups" :key="backup.key">
          <textarea :value="backup.snapshot" rows="3" readonly aria-label="旧版暂存评论内容" />
          <button v-if="c.canRestoreBackup" type="button" :disabled="c.loading" @click="c.restoreBackup(backup.key)">确认恢复到当前数据库</button>
        </div>
        <p v-if="c.backupError" role="alert">恢复尚未确认，或原暂存已有更新。剩余内容仍保留在本机。</p>
      </details>
      <p v-if="c.loading" role="status">正在载入本集评论</p>
      <p v-else-if="c.readError" role="alert">本地评论读取失败 <button @click="c.load()">重试</button></p>
      <EmptyState v-else-if="!c.visible.length" title="暂无评论" hint="来发表第一条评论吧" />
      <article v-for="cm in c.visible" :key="cm.id" class="cm" data-testid="comment">
        <i class="av a2" /><div class="body">
          <div class="who"><b>{{ cm.user ?? '未知用户' }}</b><span v-if="!localOnly" class="lv"><Crown :size="11" fill="currentColor" />Lv{{ cm.level ?? 1 }}</span><em v-if="!localOnly && cm.byAuthor" class="au">作者</em></div>
          <time v-if="cm.ago">{{ cm.ago }}</time><p>{{ cm.text }}</p>
          <div class="acts"><button class="lk" :class="{ on: cm.liked }" data-testid="like" :aria-pressed="cm.liked ?? false" :disabled="c.saveConflict" @click="c.like(cm)"><ThumbsUp :size="15" :fill="cm.liked ? 'currentColor' : 'none'" />{{ n(cm.likes ?? 0) }}</button>
            <button class="reply-button" data-testid="reply-btn" @click="replyFor = replyFor === cm.id ? null : cm.id"><MessageCircle :size="15" />回复 ({{ replyCount(cm) }})</button>
            <span class="sp" />
            <div v-if="cm.mine === true" class="more" @focusout="closeCommentMenuOnFocusExit">
              <button type="button" aria-label="更多" :aria-expanded="menuFor === cm.id" :disabled="c.saveConflict" data-testid="more" @click="toggleCommentMenu(cm.id, $event)"><Ellipsis :size="18" /></button>
              <ul v-if="menuFor === cm.id" class="mn"><li><button type="button" data-testid="delete" :disabled="c.saveConflict" @mousedown.prevent @click="deleteComment(cm.id)">删除</button></li></ul>
            </div>
          </div>
          <div v-if="replyFor === cm.id" class="rbox"><input v-model="draft.replies[cm.id]" :placeholder="`回复 ${cm.user ?? '未知用户'}`" data-testid="reply-input" @keydown.enter="sendReply(cm, $event)" /><button data-testid="reply-send" :disabled="c.saveConflict" @click="sendReply(cm)">发送</button></div>
          <div v-if="cm.replies?.length && c.expanded.has(cm.id)" class="replies" data-testid="replies">
            <div v-for="r in cm.replies" :key="r.id" class="rp" data-testid="comment-reply">
              <i class="av a3" />
              <div>
                <div class="who"><b>{{ r.user ?? '未知用户' }}</b><span v-if="!localOnly" class="lv"><Crown :size="10" fill="currentColor" />Lv{{ r.level ?? 1 }}</span></div>
                <p>{{ r.text }}</p>
                <div class="acts s">
                  <time v-if="r.ago">{{ r.ago }}</time>
                  <button class="lk" :class="{ on: r.liked }" :disabled="c.saveConflict" @click="c.like(r)"><ThumbsUp :size="13" :fill="r.liked ? 'currentColor' : 'none'" />{{ n(r.likes ?? 0) }}</button>
                  <button v-if="r.mine === true" type="button" :disabled="c.saveConflict" @click="deleteReply(cm.id, r.id, $event)">删除回复</button>
                </div>
              </div>
            </div>
          </div>
          <button v-if="replyCount(cm) > 0" class="all" data-testid="toggle-replies" @click="c.toggleReplies(cm.id)">{{ c.expanded.has(cm.id) ? '收起回复' : `共 ${replyCount(cm)} 条回复` }}<ChevronRight :size="13" /></button>
        </div></article>
      <button v-if="c.hasMore" class="loadmore" data-testid="load-more" @click="c.loadMore()">加载更多评论</button>
    </div>
  </aside>
</template>
<style scoped lang="scss">
.cp { display: flex; flex-direction: column; height: 100%; border-radius: 22px; background: linear-gradient(180deg, #2c1618, #201214); border: 1px solid var(--c-border); padding: 0 0 8px; overflow: hidden; }
header { display: flex; justify-content: space-between; align-items: center; padding: 18px 24px 10px; h2 { font-size: 24px; small { font-size: 18px; font-weight: 400; color: var(--c-text-2); } } }
.sorts { display: flex; gap: 12px; padding: 4px 22px 14px; button { height: 32px; padding: 0 20px; border-radius: 16px; background: var(--c-surface-hover); font-size: 14px; &.on { background: var(--g-accent); } } }
.editor { display: flex; gap: 14px; padding: 0 20px; .box { flex: 1; } textarea { width: 100%; resize: none; padding: 12px 16px; border-radius: 12px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border); outline: 0; font-size: 15px; &:focus { border-color: var(--c-accent); } } }
.av { flex: none; width: 40px; height: 40px; border-radius: 50%; background: radial-gradient(circle at 50% 38%, #cfc4d6 22%, transparent 24%), linear-gradient(#8c8097, #6c6178); &.a2 { background: radial-gradient(circle at 50% 38%, #f1d2c2 24%, transparent 26%), linear-gradient(#b97c86, #6e3d4c); } &.a3 { width: 30px; height: 30px; background: radial-gradient(circle at 50% 38%, #f1d2c2 24%, transparent 26%), linear-gradient(#7d8fb9, #3d4d6e); } }
.tools { display: flex; align-items: center; gap: 22px; padding: 8px 22px 6px 76px; button:not(.post) { color: var(--c-text); } .cnt { margin-left: auto; font-size: 13px; color: var(--c-text-2); &.over { color: var(--c-danger); } } .post { height: 36px; padding: 0 20px; border-radius: 18px; background: var(--g-accent); font-weight: 600; font-size: 14px; &:disabled { opacity: .5; cursor: not-allowed; } } }
.emoji-choices {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 2px 22px 8px 76px;

  button {
    width: 36px;
    height: 36px;
    border: 1px solid var(--c-border);
    border-radius: 8px;
    background: var(--c-surface);
    font-size: 19px;

    &:hover, &:focus-visible { background: var(--c-surface-hover); }
  }
}
.err { padding: 0 22px 6px 76px; color: var(--c-danger); font-size: 13px; }
.list { flex: 1; min-height: 0; overflow-y: auto; padding: 6px 20px 0; border-top: 1px solid var(--c-border); margin-top: 10px; }
.comment-backups {
  padding: 8px 0;
  font-size: 13px;
  color: var(--c-text-2);
  summary { cursor: pointer; color: var(--c-text); }
  p { margin: 8px 0; }
  textarea { display: block; width: 100%; color: var(--c-text); background: var(--c-surface); }
  button { padding: 6px 0; text-align: left; color: var(--c-text); text-decoration: underline; }
}
.cm { display: flex; gap: 14px; padding: 16px 0 10px; border-bottom: 1px solid var(--c-border); }
.body { flex: 1; min-width: 0; position: relative; time { display: block; font-size: 12.5px; color: var(--c-text-2); margin: 3px 0 6px; } p { font-size: 15.5px; line-height: 1.6; } }
.who { display: flex; align-items: center; gap: 8px; b { font-size: 15px; color: var(--c-text-2); font-weight: 600; } .au { font-style: normal; font-size: 11px; padding: 0 6px; border-radius: 4px; background: var(--c-accent-soft); color: var(--c-accent-2); } }
.lv { display: inline-flex; align-items: center; gap: 2px; padding: 1px 7px; border-radius: 10px; font-size: 11px; background: rgba(244,183,64,.18); color: var(--c-gold); }
.acts { display: flex; align-items: center; gap: 24px; margin-top: 10px; font-size: 14px; color: var(--c-text-2); button { display: inline-flex; align-items: center; gap: 6px; } .lk { color: var(--c-accent-2); &.on { color: #ff5d4a; font-weight: 700; } } .sp { flex: 1; } &.s { margin-top: 4px; gap: 18px; font-size: 12.5px; time { margin: 0; } } }
.more { position: relative; } .mn { position: absolute; right: 0; top: 26px; z-index: 5; min-width: 90px; padding: 6px; border-radius: 10px; background: #2a1719; border: 1px solid var(--c-border-strong); button { width: 100%; height: 30px; justify-content: center; } }
.rbox { display: flex; gap: 8px; margin-top: 10px; input { flex: 1; height: 36px; padding: 0 12px; border-radius: 18px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border); outline: 0; &:focus { border-color: var(--c-accent); } } button { padding: 0 16px; border-radius: 18px; background: var(--g-accent); } }
.replies { margin-top: 10px; padding: 8px 14px; border-radius: 12px; background: rgba(255,255,255,.05); } .rp { display: flex; gap: 10px; padding: 8px 0; p { font-size: 14px; } }
.all { margin-top: 8px; display: inline-flex; align-items: center; font-size: 13px; color: var(--c-text-2); }
.loadmore { width: 100%; height: 40px; margin: 10px 0; color: var(--c-text-2); border-radius: 10px; &:hover { background: var(--c-surface); } }
</style>
