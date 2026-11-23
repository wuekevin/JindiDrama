<script setup lang="ts">
import { ref } from 'vue'
import { MessageSquareText, Eye, Scan, Flame, ShieldCheck, Plus, X, Smile, Check } from 'lucide-vue-next'
import { useSettingsStore } from '@/stores/player'
import { useToastStore } from '@/stores/toast'
import { isBlocked, type Area, type Density } from '@/player/danmaku'
const props = defineProps<{ time: number }>()
const emit = defineEmits<{ send: [text: string] }>()
const s = useSettingsStore(); const toast = useToastStore()
const d = s.danmaku
const adding = ref(false); const word = ref(''); const text = ref('')
const areas: { k: Area; l: string }[] = [{ k: 'full', l: '全屏显示' }, { k: 'top', l: '上半屏' }, { k: 'bottom', l: '下半屏' }]
const DENS: Density[] = ['low', 'mid', 'high']; const DL = { low: '低', mid: '中', high: '高' }
function addWord() { const w = word.value.trim(); if (!w) { toast.push('关键词不能为空', 'error'); return } if (w.length > 10) { toast.push('关键词最多 10 个字', 'error'); return } if (!s.addBlock(w)) { toast.push('该关键词已存在', 'error'); return } word.value = ''; adding.value = false }
function send() { const t = text.value.trim(); if (!t) { toast.push('请输入弹幕内容', 'error'); return } if (t.length > 40) { toast.push('弹幕最多 40 个字', 'error'); return } if (!d.enabled) { toast.push('请先开启弹幕', 'error'); return } if (isBlocked(t, d.blocked)) { toast.push('包含屏蔽关键词，发送后你也看不到这条弹幕', 'error'); return } emit('send', t); text.value = ''; toast.push('弹幕已发送（本地演示）', 'success') }
const pct = (v: number, lo: number, hi: number) => ((v - lo) / (hi - lo)) * 100
</script>
<template>
  <aside class="ds" data-testid="danmaku-settings" aria-label="弹幕设置">
    <h2><MessageSquareText :size="26" color="#ff6a4d" />弹幕设置</h2>
    <div class="row sw"><Eye :size="22" /><div><b>弹幕开关</b><small>开启后，视频中将显示弹幕</small></div><button class="tg" :class="{ on: d.enabled }" role="switch" :aria-checked="d.enabled" aria-label="弹幕开关" data-testid="dm-enabled" @click="d.enabled = !d.enabled"><i /></button></div>
    <div class="card"><h4><Scan :size="18" />显示区域</h4>
      <div class="areas"><button v-for="a in areas" :key="a.k" :class="{ on: d.area === a.k }" :data-testid="`area-${a.k}`" :aria-pressed="d.area === a.k" @click="d.area = a.k"><span class="ico" :class="a.k"><i /><i /><i /></span><Check v-if="d.area === a.k" :size="14" class="ck" /><em>{{ a.l }}</em></button></div>
      <label class="sl"><span>透明度</span><input v-model.number="d.opacity" type="range" min="20" max="100" data-testid="dm-opacity" :style="{ '--p': pct(d.opacity, 20, 100) + '%' }" /><em>{{ d.opacity }}%</em></label>
      <label class="sl"><span>字号</span><input v-model.number="d.fontSize" type="range" min="12" max="28" data-testid="dm-size" :style="{ '--p': pct(d.fontSize, 12, 28) + '%' }" /><em>{{ d.fontSize }}px</em></label>
      <label class="sl"><span>速度</span><input v-model.number="d.speed" type="range" min="0.5" max="2" step="0.25" data-testid="dm-speed" :style="{ '--p': pct(d.speed, 0.5, 2) + '%' }" /><em>{{ d.speed.toFixed(1) }}x</em></label>
      <label class="sl"><span>密度</span><input type="range" min="0" max="2" :value="DENS.indexOf(d.density)" data-testid="dm-density" :style="{ '--p': DENS.indexOf(d.density) * 50 + '%' }" @input="d.density = DENS[+($event.target as HTMLInputElement).value]" /><em>{{ DL[d.density] }}</em></label>
    </div>
    <div class="row sw"><Flame :size="22" color="#ff6a3d" fill="#ff6a3d" /><div><b>仅看热评弹幕</b><small>仅显示高赞、精选的优质弹幕</small></div><button class="tg" :class="{ on: d.hotOnly }" role="switch" :aria-checked="d.hotOnly" aria-label="仅看热评弹幕" data-testid="dm-hot" @click="d.hotOnly = !d.hotOnly"><i /></button></div>
    <div class="card kw"><div class="kh"><ShieldCheck :size="22" /><div><b>屏蔽关键词</b><small>包含以下关键词的弹幕将不再显示</small></div><button class="add" data-testid="kw-add-btn" @click="adding = !adding"><Plus :size="14" />添加</button></div>
      <div v-if="adding" class="addrow"><input v-model="word" placeholder="输入要屏蔽的关键词" data-testid="kw-input" @keydown.enter="addWord" /><button data-testid="kw-confirm" @click="addWord">确定</button></div>
      <ul><li v-for="w in d.blocked" :key="w" data-testid="kw">{{ w }}<button :aria-label="`移除${w}`" @click="s.removeBlock(w)"><X :size="12" /></button></li></ul></div>
    <div class="send"><h4><MessageSquareText :size="22" />发送弹幕</h4><div class="srow"><div class="ip"><input v-model="text" placeholder="发一条弹幕，和大家一起聊剧情吧~" aria-label="弹幕内容" data-testid="dm-input" @keydown.enter="send" /><Smile :size="22" /></div><button class="go" data-testid="dm-send" @click="send">发送</button></div></div>
  </aside>
</template>
<style scoped lang="scss">
.ds { height: 100%; overflow-y: auto; padding: 18px 16px 14px; border-radius: 20px; background: linear-gradient(180deg, #2a1618, #1f1214); border: 1px solid var(--c-border); display: flex; flex-direction: column; gap: 10px; h2 { display: flex; align-items: center; gap: 10px; font-size: 24px; padding-left: 8px; } }
.row.sw { display: flex; align-items: center; gap: 14px; padding: 10px 14px; border-radius: 14px; background: var(--c-surface); border: 1px solid var(--c-border); div { flex: 1; } b { display: block; font-size: 16px; } small { display: block; font-size: 12.5px; color: var(--c-text-2); margin-top: 2px; } }
.tg { width: 52px; height: 28px; border-radius: 14px; background: rgba(255,255,255,.2); position: relative; transition: background .2s; i { position: absolute; top: 3px; left: 3px; width: 22px; height: 22px; border-radius: 50%; background: #fff; transition: transform .2s var(--ease); } &.on { background: #ee4b39; i { transform: translateX(24px); } } }
.card { padding: 12px 14px; border-radius: 14px; background: var(--c-surface); border: 1px solid var(--c-border); h4 { display: flex; align-items: center; gap: 8px; font-size: 16px; margin-bottom: 10px; } }
.areas { display: flex; gap: 14px; justify-content: space-between; margin-bottom: 12px; padding-bottom: 12px; border-bottom: 1px solid var(--c-border);
  button { position: relative; flex: 1; text-align: center; .ico { display: flex; flex-direction: column; gap: 5px; justify-content: center; height: 66px; border-radius: 8px; background: rgba(255,255,255,.08); border: 1.5px solid transparent; padding: 0 14px; i { height: 2px; background: rgba(255,255,255,.55); border-radius: 2px; } &.top i:nth-child(3), &.bottom i:nth-child(1) { opacity: 0; } } em { display: block; margin-top: 6px; font-style: normal; font-size: 13px; color: var(--c-text-2); } .ck { position: absolute; right: 4px; top: 44px; background: var(--c-accent); border-radius: 50%; padding: 2px; } &.on .ico { border-color: var(--c-accent); background: rgba(238,75,57,.18); } &.on em { color: var(--c-accent-2); } } }
.sl { display: flex; align-items: center; gap: 14px; height: 38px; span { width: 52px; white-space: nowrap; font-size: 15px; } em { width: 48px; text-align: right; font-style: normal; font-size: 14px; color: var(--c-text-2); } input { flex: 1; appearance: none; height: 5px; border-radius: 5px; outline: none; background: linear-gradient(90deg, #ff7a5c var(--p), rgba(255,255,255,.2) var(--p)); &::-webkit-slider-thumb { appearance: none; width: 18px; height: 18px; border-radius: 50%; background: #fff; box-shadow: 0 0 0 4px rgba(238,75,57,.35); cursor: pointer; } } }
.kh { display: flex; align-items: center; gap: 12px; div { flex: 1; } b { display: block; font-size: 16px; } small { font-size: 12.5px; color: var(--c-text-2); } .add { display: inline-flex; align-items: center; gap: 4px; height: 32px; padding: 0 16px; border-radius: 16px; border: 1px solid var(--c-border-strong); background: rgba(255,255,255,.06); font-size: 14px; } }
.addrow { display: flex; gap: 8px; margin-top: 10px; input { flex: 1; height: 34px; padding: 0 12px; border-radius: 17px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border); outline: 0; } button { padding: 0 16px; border-radius: 17px; background: var(--g-accent); } }
.kw ul { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 12px; li { display: inline-flex; align-items: center; gap: 8px; height: 30px; padding: 0 12px; border-radius: 15px; background: rgba(255,255,255,.1); font-size: 14px; button { display: grid; place-items: center; color: var(--c-text-2); } } }
.send { margin-top: auto; padding: 14px; border-radius: 14px; background: var(--c-surface); border: 1px solid var(--c-border); h4 { display: flex; align-items: center; gap: 8px; font-size: 18px; margin-bottom: 12px; } .srow { display: flex; gap: 10px; } .ip { flex: 1; display: flex; align-items: center; gap: 8px; height: 48px; padding: 0 14px; border-radius: 24px; border: 1px solid var(--c-border-strong); input { flex: 1; background: none; border: 0; outline: 0; font-size: 14.5px; min-width: 0; } } .go { width: 76px; height: 48px; border-radius: 24px; background: var(--g-accent); font-weight: 700; font-size: 16px; } }
</style>
