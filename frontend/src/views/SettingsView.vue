<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Settings, CirclePlay, MessageSquareText, Download, Keyboard, Palette, Info, Check, FolderOpen, Trash2, RefreshCw, ChevronRight, FileText, CircleHelp, Sun, Moon, ChevronDown, RotateCcw } from 'lucide-vue-next'
import { useAppSettings, SHORTCUT_LABELS, normalizeKey, DEFAULT_SHORTCUTS } from '@/stores/appSettings'
import { useSettingsStore } from '@/stores/player'
import { useToastStore } from '@/stores/toast'
import { cacheStats, cacheClear, cacheSetLimit, diagnostics, hasBridge, checkUpdate, openReleasePage, appVersion, type UpdateResult } from '@/bindings'
import { Modal } from '@/components/ui'
import BrandLogo from '@/components/BrandLogo.vue'

const a = useAppSettings(); const st = a.s; const pl = useSettingsStore(); const toast = useToastStore()
const sections = [{ k: 'general', l: '通用', i: Settings }, { k: 'playback', l: '播放', i: CirclePlay }, { k: 'danmaku', l: '弹幕', i: MessageSquareText }, { k: 'cache', l: '下载缓存', i: Download }, { k: 'keys', l: '快捷键', i: Keyboard }, { k: 'look', l: '外观', i: Palette }, { k: 'about', l: '关于与更新', i: Info }]
const active = ref('general'); const root = ref<HTMLElement>()
function go(k: string) { active.value = k; root.value?.querySelector(`#sec-${k}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }
const cacheMB = ref(12.6 * 1024); const cacheFiles = ref(236); const confirmClear = ref(false)
const cacheText = computed(() => `已缓存 ${(cacheMB.value / 1024).toFixed(1)} GB（${cacheFiles.value} 个文件）`)
async function refreshCache() { const s = await cacheStats().catch(() => null); if (s) { cacheMB.value = s.bytes / 1048576; cacheFiles.value = s.files } }
async function clearCache() { await cacheClear().catch(() => {}); cacheMB.value = 0; cacheFiles.value = 0; confirmClear.value = false; toast.push('缓存已清理', 'success') }
async function setLimit() { try { await cacheSetLimit(st.cacheLimitGB); await refreshCache(); toast.push(`缓存上限已设为 ${st.cacheLimitGB} GB`, 'success') } catch (e) { toast.push(String((e as Error).message ?? e), 'error') } }
async function copyDiag() { const lines = hasBridge() ? await diagnostics() : ['（浏览器预览模式无本地日志）']; try { await navigator.clipboard.writeText(lines.join('\n')); toast.push('诊断信息已复制（已脱敏）', 'success') } catch { toast.push('无法写入剪贴板', 'error') } }
const pickDir = () => { const dirs = ['JindiDrama/Cache/Downloads', 'D:/JindiDrama/Downloads', 'E:/Videos/Jindi']; const i = dirs.indexOf(st.cachePath); st.cachePath = dirs[(i + 1) % dirs.length]; toast.push('已更改缓存位置（演示：Wails 目录选择器将在桌面端接入）') }
const rec = ref(''); function startRec(action: string) { rec.value = action }
function onKey(e: KeyboardEvent) { if (!rec.value) return; e.preventDefault(); if (e.key === 'Escape') { rec.value = ''; return } const c = normalizeKey(e); if (!c) return; const conflict = a.rebind(rec.value, c); if (conflict) toast.push(`与“${conflict}”冲突`, 'error'); else toast.push('快捷键已更新', 'success'); rec.value = '' }
const checking = ref(false); const lastCheck = ref(new Date())
const version = ref('v0.1.0'); appVersion().then((v) => { version.value = v }).catch(() => {})
const upd = ref<UpdateResult | null>(null)
async function check() {
  checking.value = true
  try {
    const r = await checkUpdate()
    lastCheck.value = new Date()
    if (r === null) toast.push('浏览器预览模式不支持检查更新，请在桌面端使用')
    else if (r.hasUpdate) { upd.value = r; toast.push(`发现新版本 ${r.latest}`, 'success') }
    else if (r.noRelease) toast.push('暂无已发布版本，当前已是最新', 'success')
    else toast.push('当前已是最新版本', 'success')
  } catch (e) { toast.push(e instanceof Error ? e.message : '检查更新失败', 'error') } finally { checking.value = false }
}
const hhmm = (d: Date) => `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
const QUALS = ['标清 480P', '高清 720P', '高清 1080P', '蓝光 4K']
onMounted(() => { a.applyTheme(); refreshCache() })
const pct = (v: number, lo: number, hi: number) => `${((v - lo) / (hi - lo)) * 100}%`
const sizeIdx = ['小', '中', '大']; const sizeLabel = { 小: '小', 标准: '中', 大: '大' } as Record<string, string>
</script>
<template>
  <div ref="root" class="set" data-testid="page-settings" @keydown.capture="onKey">
    <header><Settings :size="40" :stroke-width="1.6" color="#ff6a4d" /><h1>设置</h1><p>个性化你的观剧体验，让追剧更顺畅、更舒适</p></header>
    <div class="cols">
      <nav class="nav" aria-label="设置分类"><button v-for="s in sections" :key="s.k" :class="{ on: active === s.k }" :data-testid="`snav-${s.k}`" @click="go(s.k)"><component :is="s.i" :size="20" />{{ s.l }}<ChevronRight v-if="active === s.k" :size="16" class="ch" /></button></nav>
      <div class="panels">
        <section id="sec-general" class="pn"><h2><Settings :size="22" />通用</h2>
          <div class="r"><div>开机启动<small>随系统启动金帝短剧</small></div><button class="tg" :class="{ on: st.autostart }" role="switch" :aria-checked="st.autostart" aria-label="开机启动" data-testid="autostart" @click="st.autostart = !st.autostart"><i /></button></div>
          <label class="r">启动时进入<span class="sel"><select v-model="st.startPage" data-testid="start-page"><option>上次观看</option><option>推荐首页</option><option>我的收藏</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">最小化时<span class="sel"><select v-model="st.minimize"><option>显示在任务栏</option><option>最小化到托盘</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">关闭窗口时<span class="sel"><select v-model="st.closeAction" data-testid="close-action"><option>最小化到托盘</option><option>退出程序</option></select><ChevronDown :size="14" /></span></label></section>
        <section id="sec-playback" class="pn"><h2><CirclePlay :size="22" />播放</h2>
          <label class="r">默认播放清晰度<span class="sel"><select v-model="st.defaultQuality" data-testid="default-quality"><option v-for="q in QUALS" :key="q">{{ q }}</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">播放源<span class="sel"><select v-model="st.mediaSource" data-testid="media-source"><option value="demo">演示媒体（自制片段）</option><option value="sim">模拟画面</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">播放方式<span class="sel wide"><select v-model="st.playMode"><option>连续播放（自动下一集）</option><option>单集播放</option><option>单集循环</option></select><ChevronDown :size="14" /></span></label>
          <div class="r">解码模式<div class="radios"><label><input v-model="st.decode" type="radio" value="hw" data-testid="decode-hw" />硬件解码（推荐）</label><label><input v-model="st.decode" type="radio" value="sw" data-testid="decode-sw" />软件解码</label></div></div>
          <label class="r">播放倍速默认值<input v-model.number="st.defaultSpeed" class="rg" type="range" min="0.5" max="2" step="0.25" aria-label="播放倍速默认值" data-testid="default-speed" :style="{ '--p': pct(st.defaultSpeed, 0.5, 2) }" /><em>{{ st.defaultSpeed.toFixed(1) }}x</em></label></section>
        <section id="sec-danmaku" class="pn"><h2><MessageSquareText :size="22" />弹幕</h2>
          <div class="r">开启弹幕<button class="tg" :class="{ on: pl.danmaku.enabled }" role="switch" :aria-checked="pl.danmaku.enabled" aria-label="开启弹幕" data-testid="s-dm-enabled" @click="pl.danmaku.enabled = !pl.danmaku.enabled"><i /></button></div>
          <label class="r">弹幕透明度<input v-model.number="pl.danmaku.opacity" class="rg" type="range" min="20" max="100" aria-label="弹幕透明度" data-testid="s-dm-opacity" :style="{ '--p': pct(pl.danmaku.opacity, 20, 100) }" /><em>{{ pl.danmaku.opacity }}%</em></label>
          <label class="r">弹幕字体大小<input v-model.number="pl.danmaku.fontSize" class="rg" type="range" min="12" max="28" aria-label="弹幕字体大小" data-testid="s-dm-size" :style="{ '--p': pct(pl.danmaku.fontSize, 12, 28) }" /><em>{{ pl.danmaku.fontSize <= 14 ? '中' : pl.danmaku.fontSize <= 20 ? '大' : '特大' }}</em></label>
          <label class="r">弹幕速度<input v-model.number="pl.danmaku.speed" class="rg" type="range" min="0.5" max="2" step="0.25" aria-label="弹幕速度" :style="{ '--p': pct(pl.danmaku.speed, 0.5, 2) }" /><em>{{ pl.danmaku.speed === 1 ? '适中' : pl.danmaku.speed < 1 ? '较慢' : '较快' }}</em></label></section>
        <section id="sec-cache" class="pn"><h2><Download :size="22" />下载缓存</h2>
          <label class="r">下载清晰度<span class="sel"><select v-model="st.dlQuality"><option v-for="q in QUALS" :key="q">{{ q }}</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">同时下载任务数<span class="sel"><select v-model.number="st.dlConcurrent" data-testid="dl-concurrent"><option v-for="n in [1, 2, 3, 4, 5]" :key="n" :value="n">{{ n }} 个</option></select><ChevronDown :size="14" /></span></label>
          <div class="r">缓存保存位置<div class="path"><input :value="st.cachePath" readonly aria-label="缓存路径" data-testid="cache-path" /><button data-testid="change-path" @click="pickDir"><FolderOpen :size="15" />更改</button></div></div>
          <label class="r">缓存上限<span class="sel"><select v-model.number="st.cacheLimitGB" data-testid="cache-limit" @change="setLimit"><option v-for="g in [1, 2, 5, 10, 20]" :key="g" :value="g">{{ g }} GB</option></select><ChevronDown :size="14" /></span></label>
          <div class="r">缓存管理<span class="cs" data-testid="cache-size">{{ cacheText }}</span><button class="clr" data-testid="clear-cache" :disabled="cacheMB === 0" @click="confirmClear = true"><Trash2 :size="15" />清理缓存</button></div></section>
        <section id="sec-keys" class="pn"><h2><Keyboard :size="22" />快捷键<button class="rs" data-testid="reset-keys" @click="st.shortcuts = { ...DEFAULT_SHORTCUTS }; toast.push('已恢复默认快捷键')"><RotateCcw :size="13" />恢复默认</button></h2>
          <div v-for="(l, k) in SHORTCUT_LABELS" :key="k" class="r k"><span>{{ l }}</span><button class="kbd" :class="{ rec: rec === k }" :data-testid="`key-${k}`" :aria-label="`修改${l}快捷键`" @click="startRec(k)">{{ rec === k ? '按下新按键…' : st.shortcuts[k] }}</button></div></section>
        <section id="sec-look" class="pn"><h2><Palette :size="22" />外观</h2>
          <div class="r tm">主题模式<div class="themes"><button v-for="t in [['dark','深色（推荐）'],['light','浅色'],['system','跟随系统']] as const" :key="t[0]" :class="[t[0], { on: st.theme === t[0] }]" :data-testid="`theme-${t[0]}`" :aria-pressed="st.theme === t[0]" @click="st.theme = t[0]"><span><Sun v-if="t[0] === 'system'" :size="20" /><Moon v-if="t[0] === 'system'" :size="20" /><Check v-if="st.theme === t[0]" :size="13" class="ck" /></span><em>{{ t[1] }}</em></button></div></div>
          <label class="r">界面缩放（DPI）<input v-model.number="st.dpi" class="rg" type="range" min="80" max="150" step="10" aria-label="界面缩放" data-testid="dpi" :style="{ '--p': pct(st.dpi, 80, 150) }" /><em>{{ st.dpi }}%</em></label>
          <label class="r">字体大小<span class="sel wide"><select v-model="st.fontSize" data-testid="font-size"><option>小</option><option>标准</option><option>大</option></select><ChevronDown :size="14" /></span></label></section>
        <section id="sec-about" class="pn about"><h2><Info :size="22" />关于与更新</h2>
          <div class="ab"><div class="lg"><BrandLogo /></div><div class="ver"><small>当前版本</small><b data-testid="version">{{ version }}<em>演示版</em></b><small data-testid="last-check">上次检查：今天 {{ hhmm(lastCheck) }}</small></div>
            <button v-if="upd" class="chk" data-testid="open-release" @click="openReleasePage(upd.url)"><FileText :size="18" />前往下载 {{ upd.latest }}</button>
            <button class="chk" :disabled="checking" data-testid="check-update" @click="check"><RefreshCw :size="18" :class="{ spin: checking }" />{{ checking ? '检查中…' : '检查更新' }}</button>
            <div class="links"><button @click="upd ? toast.push(upd.notes.slice(0, 120) || '该版本未提供更新说明') : toast.push('暂无新版本更新日志')"><FileText :size="16" />版本更新日志<ChevronRight :size="14" /></button><button data-testid="copy-diag" @click="copyDiag"><FileText :size="16" />复制诊断信息<ChevronRight :size="14" /></button><button @click="toast.push('帮助与反馈入口待配置')"><CircleHelp :size="16" />帮助与反馈<ChevronRight :size="14" /></button></div></div></section>
      </div>
    </div>
    <Modal :open="confirmClear" title="清理缓存" confirm-text="确认清理" @cancel="confirmClear = false" @confirm="clearCache">将删除全部已缓存的下载文件，不影响收藏和观看历史。</Modal>
  </div>
</template>
<style scoped lang="scss">
.set { padding: 10px 16px 24px 14px; header { display: flex; align-items: center; gap: 14px; height: 78px; padding: 0 22px; border-radius: 24px; background: linear-gradient(90deg, rgba(120,32,28,.35), rgba(60,20,22,.2)); border: 1px solid var(--c-border); h1 { font-size: 34px; } p { font-size: 14px; color: var(--c-text-2); margin-left: 8px; } } }
.cols { display: flex; gap: 14px; margin-top: 12px; align-items: flex-start; }
.nav { position: sticky; top: 0; width: 196px; flex: none; padding: 8px; border-radius: 18px; background: rgba(255,255,255,.05); border: 1px solid var(--c-border); display: flex; flex-direction: column; gap: 2px; button { height: 50px; display: flex; align-items: center; gap: 14px; padding: 0 16px; border-radius: 12px; font-size: 16px; text-align: left; position: relative; &:hover { background: var(--c-surface-hover); } &.on { background: var(--g-nav-active); } .ch { margin-left: auto; } } }
.panels { flex: 1; min-width: 0; display: grid; grid-template-columns: 1fr 1fr; gap: 12px; align-items: start; }
.pn { padding: 14px 18px 8px; border-radius: 18px; background: linear-gradient(180deg, rgba(255,255,255,.065), rgba(255,255,255,.03)); border: 1px solid var(--c-border); scroll-margin-top: 4px; h2 { display: flex; align-items: center; gap: 10px; font-size: 20px; margin-bottom: 4px; svg { color: var(--c-accent-2); } } &.about { grid-column: 1 / -1; } }
.r { display: flex; align-items: center; gap: 12px; min-height: 42px; font-size: 14.5px; border-top: 1px solid var(--c-border); &:first-of-type { border-top: 0; } > div:first-child:not(.radios):not(.path) { flex: 1; } small { display: block; font-size: 12px; color: var(--c-text-2); } em { font-style: normal; width: 48px; text-align: right; color: var(--c-text-2); } }
.r > .sel, .r > .rg, .r > .radios, .r > .path { margin-left: auto; }
.sel { position: relative; display: inline-flex; align-items: center; select { appearance: none; height: 34px; min-width: 190px; padding: 0 34px 0 14px; border-radius: 10px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border-strong); color: var(--c-text); outline: none; font-size: 14px; option { background: #2a1719; } &:focus-visible { outline: 2px solid var(--c-accent-2); } } svg { position: absolute; right: 12px; pointer-events: none; } &.wide select { min-width: 240px; } }
.tg { width: 46px; height: 26px; border-radius: 13px; background: rgba(255,255,255,.2); position: relative; margin-left: auto; transition: background .2s; i { position: absolute; top: 3px; left: 3px; width: 20px; height: 20px; border-radius: 50%; background: #fff; transition: transform .2s; } &.on { background: #ee4b39; i { transform: translateX(20px); } } }
.rg { appearance: none; width: 190px; height: 5px; border-radius: 5px; outline: none; background: linear-gradient(90deg, #ff7a5c var(--p), rgba(255,255,255,.2) var(--p)); &::-webkit-slider-thumb { appearance: none; width: 17px; height: 17px; border-radius: 50%; background: #fff; box-shadow: 0 0 0 4px rgba(238,75,57,.35); cursor: pointer; } }
.radios { display: flex; gap: 18px; label { display: inline-flex; align-items: center; gap: 6px; } input { accent-color: #ee4b39; } }
.path { display: flex; gap: 8px; input { width: 200px; height: 34px; padding: 0 12px; border-radius: 10px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border); color: var(--c-text); font-size: 13px; } button { display: inline-flex; align-items: center; gap: 5px; height: 34px; padding: 0 14px; border-radius: 10px; border: 1px solid var(--c-border-strong); } }
.cs { margin-left: auto; font-size: 13px; color: var(--c-text-2); } .clr { display: inline-flex; align-items: center; gap: 6px; height: 34px; padding: 0 16px; border-radius: 17px; background: var(--g-accent); font-weight: 600; &:disabled { opacity: .45; } }
.rs { margin-left: auto; display: inline-flex; align-items: center; gap: 4px; font-size: 12.5px; font-weight: 400; color: var(--c-text-2); }
.k { min-height: 32px; } .kbd { margin-left: auto; min-width: 86px; height: 26px; padding: 0 10px; border-radius: 6px; background: rgba(255,255,255,.1); border: 1px solid var(--c-border-strong); font-size: 12.5px; &.rec { border-color: var(--c-accent); color: var(--c-accent-2); } }
.tm { align-items: flex-start; padding-top: 8px; white-space: nowrap; } .themes { display: flex; gap: 12px; margin-left: auto; button { width: 118px; text-align: center; span { position: relative; display: grid; place-items: center; height: 60px; border-radius: 10px; border: 2px solid transparent; background: linear-gradient(135deg, #7a2420, #2a1214); gap: 4px; } &.light span { background: linear-gradient(135deg, #fbe1d8, #f3b7a4); } &.system span { background: linear-gradient(90deg, #6a6f7a 50%, #3a3e48 50%); grid-auto-flow: column; } &.on span { border-color: var(--c-accent); } .ck { position: absolute; right: 4px; bottom: 4px; background: var(--c-accent); border-radius: 50%; padding: 2px; } em { display: block; width: auto; margin-top: 6px; font-size: 12.5px; text-align: center; } } }
.ab { display: flex; align-items: center; gap: 36px; padding: 12px 8px 14px; .ver { flex: 1; small { color: var(--c-text-2); font-size: 13px; } b { display: block; font-size: 30px; margin: 4px 0; em { display: inline-block; width: auto; margin-left: 10px; padding: 1px 10px; border-radius: 6px; background: rgba(70,194,138,.2); color: var(--c-success); font-size: 12px; font-weight: 400; vertical-align: middle; } } } .chk { display: inline-flex; align-items: center; gap: 10px; height: 52px; padding: 0 56px; border-radius: 26px; background: var(--g-accent); font-size: 18px; font-weight: 700; &:disabled { opacity: .7; } } .links { display: flex; flex-direction: column; gap: 14px; button { display: flex; align-items: center; gap: 8px; font-size: 14.5px; svg:last-child { margin-left: 6px; } } } }
.spin { animation: sp 1s linear infinite; } @keyframes sp { to { transform: rotate(360deg); } }
</style>
