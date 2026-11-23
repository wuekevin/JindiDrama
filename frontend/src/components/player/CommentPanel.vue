<script setup lang="ts">
import { computed, ref } from 'vue'
import { X, Smile, Image, Video, AtSign, ThumbsUp, MessageCircle, Ellipsis, Crown, ChevronRight } from 'lucide-vue-next'
import { useCommentStore, MAX_LEN, type Comment } from '@/stores/comments'
import { useToastStore } from '@/stores/toast'
import { EmptyState } from '@/components/ui'
const emit = defineEmits<{ close: [] }>()
const c = useCommentStore(); const toast = useToastStore()
const text = ref(''); const err = ref(''); const replyFor = ref<string | null>(null); const replyText = ref(''); const menuFor = ref<string | null>(null)
const tabs = [{ k: 'new', l: '最新' }, { k: 'hot', l: '最热' }, { k: 'author', l: '只看作者' }] as const
const n = (v: number) => (v >= 10000 ? (v / 10000).toFixed(1) + '万' : String(v))
const over = computed(() => text.value.length > MAX_LEN)
function submit() { const e = c.post(text.value); err.value = e ?? ''; if (!e) { text.value = ''; toast.push('评论已发布（本地演示，不会同步到红果）', 'success') } }
function sendReply(cm: Comment) { const e = c.reply(cm, replyText.value); if (e) { toast.push(e, 'error'); return } replyText.value = ''; replyFor.value = null; if (!c.expanded.has(cm.id)) c.toggleReplies(cm.id) }
function del(id: string) { c.remove(id); menuFor.value = null; toast.push('评论已删除') }
const dummy = () => toast.push('该功能在演示模式下暂不可用')
</script>
<template>
  <aside class="cp" role="complementary" aria-label="本集评论" data-testid="comment-panel" @keydown.esc="emit('close')">
    <header><h2>本集评论 <small data-testid="comment-total">({{ c.total.toLocaleString() }})</small></h2><button aria-label="关闭评论" data-testid="comment-close" @click="emit('close')"><X :size="24" /></button></header>
    <div class="sorts"><button v-for="t in tabs" :key="t.k" :class="{ on: c.sort === t.k }" :data-testid="`csort-${t.k}`" @click="c.sort = t.k">{{ t.l }}</button></div>
    <div class="editor"><i class="av" />
      <div class="box"><textarea v-model="text" rows="2" placeholder="说说你对这一集的看法吧～" aria-label="发表评论" data-testid="comment-input" @keydown.ctrl.enter="submit" @keydown.meta.enter="submit" /></div></div>
    <div class="tools"><button aria-label="表情" @click="dummy"><Smile :size="22" /></button><button aria-label="图片" @click="dummy"><Image :size="22" /></button><button aria-label="视频" @click="dummy"><Video :size="22" /></button><button aria-label="@" @click="dummy"><AtSign :size="22" /></button>
      <span class="cnt" :class="{ over }" data-testid="comment-count">{{ text.length }}/{{ MAX_LEN }}</span><button class="post" :disabled="!text.trim() || over" data-testid="comment-post" @click="submit">发表评论</button></div>
    <p v-if="err" class="err" role="alert">{{ err }}</p>
    <div class="list">
      <EmptyState v-if="!c.visible.length" title="暂无评论" hint="来发表第一条评论吧" />
      <article v-for="cm in c.visible" :key="cm.id" class="cm" data-testid="comment">
        <i class="av a2" /><div class="body">
          <div class="who"><b>{{ cm.user }}</b><span class="lv"><Crown :size="11" fill="currentColor" />Lv{{ cm.level }}</span><em v-if="cm.byAuthor" class="au">作者</em></div>
          <time>{{ cm.ago }}</time><p>{{ cm.text }}</p>
          <div class="acts"><button class="lk" :class="{ on: cm.liked }" data-testid="like" :aria-pressed="cm.liked" @click="c.like(cm)"><ThumbsUp :size="15" :fill="cm.liked ? 'currentColor' : 'none'" />{{ n(cm.likes) }}</button>
            <button data-testid="reply-btn" @click="replyFor = replyFor === cm.id ? null : cm.id"><MessageCircle :size="15" />回复 ({{ cm.replyTotal }})</button>
            <span class="sp" /><div class="more"><button aria-label="更多" data-testid="more" @click="menuFor = menuFor === cm.id ? null : cm.id"><Ellipsis :size="18" /></button>
              <ul v-if="menuFor === cm.id" class="mn"><li v-if="cm.mine"><button data-testid="delete" @click="del(cm.id)">删除</button></li><li><button @click="dummy(); menuFor = null">举报</button></li></ul></div></div>
          <div v-if="replyFor === cm.id" class="rbox"><input v-model="replyText" :placeholder="`回复 ${cm.user}`" data-testid="reply-input" @keydown.enter="sendReply(cm)" /><button data-testid="reply-send" @click="sendReply(cm)">发送</button></div>
          <div v-if="cm.replies.length && c.expanded.has(cm.id)" class="replies" data-testid="replies"><div v-for="r in cm.replies" :key="r.id" class="rp"><i class="av a3" /><div><div class="who"><b>{{ r.user }}</b><span class="lv"><Crown :size="10" fill="currentColor" />Lv{{ r.level }}</span></div><p>{{ r.text }}</p><div class="acts s"><time>{{ r.ago }}</time><button class="lk" :class="{ on: r.liked }" @click="c.like(r)"><ThumbsUp :size="13" :fill="r.liked ? 'currentColor' : 'none'" />{{ n(r.likes) }}</button></div></div></div></div>
          <button v-if="cm.replyTotal > 0" class="all" data-testid="toggle-replies" @click="c.toggleReplies(cm.id)">{{ c.expanded.has(cm.id) ? '收起回复' : `共 ${cm.replyTotal} 条回复` }}<ChevronRight :size="13" /></button>
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
.err { padding: 0 22px 6px 76px; color: var(--c-danger); font-size: 13px; }
.list { flex: 1; min-height: 0; overflow-y: auto; padding: 6px 20px 0; border-top: 1px solid var(--c-border); margin-top: 10px; }
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
