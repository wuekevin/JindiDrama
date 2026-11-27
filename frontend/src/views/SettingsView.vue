<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useDesktopUpdates } from '@/stores/desktopUpdates'
import { Settings, CirclePlay, MessageSquareText, Download, Keyboard, Palette, Info, Check, FolderOpen, Trash2, RefreshCw, ChevronRight, FileText, CircleHelp, Sun, Moon, ChevronDown, RotateCcw } from 'lucide-vue-next'
import { useAppSettings, SHORTCUT_LABELS, normalizeKey, DEFAULT_SHORTCUTS, CLOSE_TO_TRAY, CLOSE_TO_TRAY_LABEL } from '@/stores/appSettings'
import { DEMO_QUALITIES } from '@/player/demoMedia'
import { PLAY_MODES } from '@/player/preferences'
import { formatSpeed } from '@/player/machine'
import { useSettingsStore } from '@/stores/player'
import { useLibraryStore } from '@/stores/library'
import { useToastStore } from '@/stores/toast'
import { observeContentWidth } from '@/layoutWidth'
import { cacheStats, cacheClear, cacheSetLimit, hasCacheDirectoryPicker, chooseCacheDirectory, resetCacheDirectory, diagnostics, backupNow, LibraryBackupPendingError, canRestoreLibraryBackup, hasBridge, hasAppleCMSSettings, autostartEnabled, setAutostart, appVersion, type CacheStats } from '@/bindings'
import { BaseButton, Modal } from '@/components/ui'
import BrandLogo from '@/components/BrandLogo.vue'
import PrivacyNoticeDialog from '@/components/PrivacyNoticeDialog.vue'
import AppleCMSSettings from '@/components/AppleCMSSettings.vue'

const a = useAppSettings(); const st = a.s; const pl = useSettingsStore(); const toast = useToastStore()
const sections = [{ k: 'general', l: '通用', i: Settings }, { k: 'playback', l: '播放', i: CirclePlay }, { k: 'danmaku', l: '弹幕', i: MessageSquareText }, { k: 'cache', l: '下载缓存', i: Download }, { k: 'keys', l: '快捷键', i: Keyboard }, { k: 'look', l: '外观', i: Palette }, { k: 'about', l: '关于与更新', i: Info }]
const showPrivacy = ref(false)
const privacyButton = ref<HTMLButtonElement>()
const showReleaseNotes = ref(false)
const releaseNotesButton = ref<HTMLButtonElement>()
const releaseNotesContent = ref<HTMLElement>()
const active = ref('general'); const root = ref<HTMLElement>()
const layout = ref<'wide' | 'stacked' | 'compact'>('wide')
let layoutObserver: ResizeObserver | undefined
function go(k: string) {
  active.value = k
  const panel = root.value?.querySelector(`#sec-${k}`)
  if (!panel) return
  if (layout.value === 'compact') {
    const heading = panel.querySelector('h2')
    if (heading) {
      heading.tabIndex = -1
      heading.focus({ preventScroll: true })
    }
  }
  panel.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
const desktop = hasBridge()
const canShowAppleCMSSettings = hasAppleCMSSettings()
const closeAction = computed(() => st.closeAction === CLOSE_TO_TRAY && !a.canCloseToTray ? '最小化到任务栏' : st.closeAction)
const closeActionPending = ref(false)
async function changeCloseAction(event: Event) {
  const select = event.target as HTMLSelectElement
  const action = select.value
  select.value = closeAction.value
  closeActionPending.value = true
  try {
    await a.setCloseAction(action)
  } catch {
    toast.push('无法保存关闭窗口设置', 'error')
  } finally {
    closeActionPending.value = false
  }
}
const canChooseCacheDirectory = hasCacheDirectoryPicker()
const cache = ref<CacheStats | null>(desktop ? null : { bytes: 12.6 * 1024 ** 3, files: 236, max: st.cacheLimitGB * 1024 ** 3, path: st.cachePath })
let cacheRevision = 0
const cacheLoaded = ref(false)
const cacheBusy = ref(false)
const confirmClear = ref(false)
const cacheText = computed(() => cache.value
  ? `${desktop ? '' : '预览演示：'}已缓存 ${(cache.value.bytes / 1024 ** 3).toFixed(1)} GB（${cache.value.files} 个文件）`
  : '缓存用量暂不可用')
const cacheLimits = [1, 2, 5, 10, 20]
const currentLimit = computed(() => cache.value ? cache.value.max / 1024 ** 3 : st.cacheLimitGB)
const limitOptions = computed(() => [...new Set([...cacheLimits, currentLimit.value])].sort((a, b) => a - b))

async function refreshCache() {
  if (!desktop) return
  const request = ++cacheRevision
  try {
    const stats = await cacheStats()
    if (request === cacheRevision) cache.value = stats
  } catch {
    if (request === cacheRevision) cache.value = null
  } finally {
    if (request === cacheRevision) cacheLoaded.value = true
  }
}

async function clearCache() {
  if (cacheBusy.value) return
  cacheBusy.value = true
  try {
    await cacheClear()
    if (!desktop && cache.value) cache.value = { ...cache.value, bytes: 0, files: 0 }
    toast.push(desktop ? '缓存已清理' : '演示缓存已清理', 'success')
  } catch {
    toast.push('缓存清理失败', 'error')
  } finally {
    await refreshCache()
    cacheBusy.value = false
    confirmClear.value = false
  }
}

async function setLimit(event: Event) {
  const select = event.target as HTMLSelectElement
  const limit = Number(select.value)
  if (cacheBusy.value) return
  cacheBusy.value = true
  try {
    await cacheSetLimit(limit)
    st.cacheLimitGB = limit
    if (!desktop && cache.value) cache.value = { ...cache.value, max: limit * 1024 ** 3 }
    toast.push(`${desktop ? '' : '演示：'}缓存上限已设为 ${limit} GB`, 'success')
  } catch {
    toast.push('设置缓存上限失败', 'error')
  } finally {
    await refreshCache()
    select.value = String(currentLimit.value)
    cacheBusy.value = false
  }
}

async function copyDiag() {
  let lines: string[]
  try {
    lines = desktop ? await diagnostics() : ['（浏览器预览模式无本地日志）']
  } catch {
    toast.push('无法读取诊断日志', 'error')
    return
  }
  try {
    await navigator.clipboard.writeText(lines.join('\n'))
    toast.push('诊断信息已复制（已脱敏）', 'success')
  } catch {
    toast.push('无法写入剪贴板', 'error')
  }
}

const backupBusy = ref(false)
const backupPath = ref('')
const showRestore = ref(false)
const restoreMessage = ref('')
const restoreFailed = ref(false)
const restoreBusy = ref(false)
const restoreButton = ref<HTMLButtonElement>()
const restoreDescription = ref<HTMLElement>()
const canRestore = canRestoreLibraryBackup()
const library = useLibraryStore()
const showBackup = computed(() => !!backupPath.value && !showReleaseNotes.value && !showRestore.value)
let backupErrorToast: number | undefined
const backupButton = ref<HTMLButtonElement>()
const backupMessage = ref<HTMLElement>()
watch(showBackup, (open) => (open ? backupMessage.value : backupButton.value)?.focus(), { flush: 'post' })
watch(showReleaseNotes, (open) => {
  if (open) releaseNotesContent.value?.focus()
  else if (!showBackup.value) releaseNotesButton.value?.focus()
}, { flush: 'post' })
watch([showRestore, restoreBusy], ([open]) => {
  if (open) restoreDescription.value?.focus()
  else if (!open && !showBackup.value) restoreButton.value?.focus()
}, { flush: 'post' })
function openRestore() {
  restoreMessage.value = ''
  restoreFailed.value = false
  showRestore.value = true
}
function closeRestore() {
  if (!restoreBusy.value) showRestore.value = false
}
async function confirmRestore() {
  if (restoreBusy.value) return
  if (restoreMessage.value) {
    closeRestore()
    return
  }
  restoreBusy.value = true
  try {
    const result = await library.restoreMissingFromBackup()
    if (!result) {
      showRestore.value = false
      return
    }
    restoreMessage.value = result.favorites || result.history
      ? `已恢复 ${result.favorites} 项收藏和 ${result.history} 条观看历史。`
      : '备份中的收藏和观看历史已存在。'
  } catch (error) {
    restoreFailed.value = true
    restoreMessage.value = error instanceof Error ? error.message : typeof error === 'string' ? error : '恢复数据失败'
  } finally {
    restoreBusy.value = false
  }
}
async function createBackup() {
  if (!desktop) {
    toast.push('仅桌面端支持数据库备份')
    return
  }
  if (backupBusy.value) return
  if (backupErrorToast !== undefined) {
    toast.dismiss(backupErrorToast)
    backupErrorToast = undefined
  }
  backupBusy.value = true
  try {
    backupPath.value = await backupNow()
  } catch (error) {
    const detail = typeof error === 'string' ? error : error instanceof Error ? error.message : ''
    const knownBackupError = error instanceof LibraryBackupPendingError ||
      detail === '备份目录已移动或替换，请确认应用数据目录后重试' ||
      detail === '备份已保存到原目录，请在原目录中查找'
    const message = knownBackupError ? detail : '数据库备份失败'
    backupErrorToast = toast.push(message, 'error')
  } finally {
    backupBusy.value = false
  }
}

async function changeCacheDirectory(reset = false) {
  if (!desktop) {
    const dirs = ['JindiDrama/Cache/Downloads', 'D:/JindiDrama/Downloads', 'E:/Videos/Jindi']
    st.cachePath = dirs[(dirs.indexOf(st.cachePath) + 1) % dirs.length]
    toast.push('已更改演示缓存路径')
    return
  }
  if (cacheBusy.value) return
  cacheBusy.value = true
  try {
    const stats = await (reset ? resetCacheDirectory() : chooseCacheDirectory())
    if (!stats) return
    cacheRevision++
    cache.value = stats
    cacheLoaded.value = true
    toast.push(reset ? '已恢复默认缓存位置' : '已切换缓存目录，原目录文件保留', 'success')
  } catch (error) {
    toast.push(typeof error === 'string' ? error : error instanceof Error ? error.message : '无法更改缓存目录', 'error')
  } finally {
    cacheBusy.value = false
  }
}
const rec = ref('')
function startRec(action: string, event: MouseEvent) {
  const button = event.currentTarget as HTMLButtonElement
  button.focus({ preventScroll: true })
  rec.value = action
}
function onKey(e: KeyboardEvent) {
  if (!rec.value || e.isComposing) return
  if (e.key === 'Escape' || e.key === 'Tab') {
    if (e.key === 'Escape') e.preventDefault()
    rec.value = ''
    return
  }
  e.preventDefault()
  const combo = normalizeKey(e)
  if (!combo) return
  const conflict = a.rebind(rec.value, combo)
  if (conflict) toast.push(`与“${conflict}”冲突`, 'error')
  else toast.push('快捷键已更新', 'success')
  rec.value = ''
}
const updates = useDesktopUpdates()
const { channel, channelBusy, channelAvailable, checking, downloading, lastSavedUpdate, canDownload, lastCheck, upd } = storeToRefs(updates)
if (desktop) void updates.loadChannel()
const savedUpdateCard = ref<HTMLElement>()
watch(lastSavedUpdate, (saved) => {
  if (saved) savedUpdateCard.value?.scrollIntoView({ block: 'nearest' })
}, { flush: 'post' })

async function copyUpdatePath() {
  const path = lastSavedUpdate.value?.path
  if (!path) return
  const runtime = (window as Window & { runtime?: { ClipboardSetText?(text: string): Promise<boolean> } }).runtime
  try {
    if (runtime?.ClipboardSetText) {
      if (!(await runtime.ClipboardSetText(path))) throw new Error('clipboard')
    } else {
      await navigator.clipboard.writeText(path)
    }
    toast.push('安装包路径已复制', 'success')
  } catch {
    toast.push('无法写入剪贴板', 'error')
  }
}
const autostartPending = ref(false)
const autostartReadFailed = ref(false)
async function loadAutostart() {
  if (autostartPending.value) return
  autostartPending.value = true
  try {
    const enabled = await autostartEnabled()
    if (enabled !== null) st.autostart = enabled
    autostartReadFailed.value = false
  } catch {
    autostartReadFailed.value = true
    toast.push('无法读取系统开机启动状态', 'error')
  } finally {
    autostartPending.value = false
  }
}
async function toggleAutostart() {
  if (autostartPending.value || autostartReadFailed.value) return
  autostartPending.value = true
  const want = !st.autostart
  try {
    await setAutostart(want)
    st.autostart = want
  } catch (error) {
    const message = error instanceof Error ? error.message : typeof error === 'string' ? error : ''
    toast.push(message || '设置开机启动失败', 'error')
  } finally {
    autostartPending.value = false
  }
}
void loadAutostart()
const version = ref('v0.1.0'); appVersion().then((v) => { version.value = v }).catch(() => {})
async function changeUpdateChannel(event: Event) {
  const select = event.target as HTMLSelectElement
  const next = select.value
  if ((next !== 'stable' && next !== 'preview') || !(await updates.chooseChannel(next))) {
    select.value = channel.value
  }
}
async function viewRelease() {
  if (!(await updates.openRelease())) toast.push('无法打开版本下载页面', 'error')
}
const checkLabel = computed(() => lastCheck.value
  ? `上次成功检查：${new Date(lastCheck.value).toLocaleString('zh-CN')}`
  : '尚未检查更新')
onMounted(() => {
  a.applyTheme()
  refreshCache()
  if (root.value) {
    layoutObserver = observeContentWidth(root.value, (width) => {
      layout.value = width <= 650 ? 'compact' : width <= 1040 ? 'stacked' : 'wide'
    })
  }
})
onBeforeUnmount(() => layoutObserver?.disconnect())
const pct = (v: number, lo: number, hi: number) => `${((v - lo) / (hi - lo)) * 100}%`
const sizeIdx = ['小', '中', '大']; const sizeLabel = { 小: '小', 标准: '中', 大: '大' } as Record<string, string>
</script>
<template>
  <div ref="root" class="set" :class="layout" data-testid="page-settings" @keydown.capture="onKey">
    <header><Settings :size="40" :stroke-width="1.6" color="#ff6a4d" /><h1>设置</h1><p>个性化你的观剧体验，让追剧更顺畅、更舒适</p></header>
    <div class="cols">
      <nav class="nav" aria-label="设置分类"><button v-for="s in sections" :key="s.k" :class="{ on: active === s.k }" :data-testid="`snav-${s.k}`" @click="go(s.k)"><component :is="s.i" :size="20" />{{ s.l }}<ChevronRight v-if="active === s.k" :size="16" class="ch" /></button></nav>
      <div class="panels">
        <section id="sec-general" class="pn"><h2><Settings :size="22" />通用</h2>
          <div class="r">
            <div>开机启动<small>随系统启动金帝短剧</small></div>
            <BaseButton v-if="autostartReadFailed" variant="outline" aria-label="重试读取开机启动状态" :disabled="autostartPending" @click="loadAutostart">重试读取</BaseButton>
            <button v-else class="tg" :class="{ on: st.autostart }" role="switch" :aria-checked="st.autostart" aria-label="开机启动" data-testid="autostart" :disabled="autostartPending" @click="toggleAutostart"><i /></button>
          </div>
          <label class="r">启动时进入<span class="sel"><select v-model="st.startPage" data-testid="start-page"><option>上次观看</option><option>推荐首页</option><option>我的收藏</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">关闭窗口时<span class="sel"><select :value="closeAction" :disabled="closeActionPending" data-testid="close-action" @change="changeCloseAction"><option>最小化到任务栏</option><option v-if="a.canCloseToTray" :value="CLOSE_TO_TRAY">{{ CLOSE_TO_TRAY_LABEL }}</option><option>退出程序</option></select><ChevronDown :size="14" /></span></label></section>
        <section id="sec-playback" class="pn"><h2><CirclePlay :size="22" />播放</h2>
          <label class="r">默认播放清晰度<span class="sel"><select v-model="st.defaultQuality" @change="pl.quality = st.defaultQuality" data-testid="default-quality"><option v-for="q in DEMO_QUALITIES" :key="q.id" :value="q.label">{{ q.label }} {{ q.detail }}</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">播放源<span class="sel"><select v-model="st.mediaSource" data-testid="media-source"><option value="demo">演示媒体（自制片段）</option><option value="sim">模拟画面</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">播放方式<span class="sel wide"><select v-model="st.playMode"><option v-for="mode in PLAY_MODES" :key="mode">{{ mode }}</option></select><ChevronDown :size="14" /></span></label>
          <div class="r">解码模式<span class="cs" data-testid="decode-mode">系统自动选择</span></div>
          <label class="r">播放倍速默认值<input v-model.number="st.defaultSpeed" @change="pl.speed = st.defaultSpeed" class="rg" type="range" min="0.5" max="2" step="0.25" aria-label="播放倍速默认值" data-testid="default-speed" :style="{ '--p': pct(st.defaultSpeed, 0.5, 2) }" /><em>{{ formatSpeed(st.defaultSpeed) }}x</em></label>
          <AppleCMSSettings v-if="canShowAppleCMSSettings" />
        </section>
        <section id="sec-danmaku" class="pn"><h2><MessageSquareText :size="22" />弹幕</h2>
          <div class="r">开启弹幕<button class="tg" :class="{ on: pl.danmaku.enabled }" role="switch" :aria-checked="pl.danmaku.enabled" aria-label="开启弹幕" data-testid="s-dm-enabled" @click="pl.danmaku.enabled = !pl.danmaku.enabled"><i /></button></div>
          <label class="r">弹幕透明度<input v-model.number="pl.danmaku.opacity" class="rg" type="range" min="20" max="100" aria-label="弹幕透明度" data-testid="s-dm-opacity" :style="{ '--p': pct(pl.danmaku.opacity, 20, 100) }" /><em>{{ pl.danmaku.opacity }}%</em></label>
          <label class="r">弹幕字体大小<input v-model.number="pl.danmaku.fontSize" class="rg" type="range" min="12" max="28" aria-label="弹幕字体大小" data-testid="s-dm-size" :style="{ '--p': pct(pl.danmaku.fontSize, 12, 28) }" /><em>{{ pl.danmaku.fontSize <= 14 ? '中' : pl.danmaku.fontSize <= 20 ? '大' : '特大' }}</em></label>
          <label class="r">弹幕速度<input v-model.number="pl.danmaku.speed" class="rg" type="range" min="0.5" max="2" step="0.25" aria-label="弹幕速度" :style="{ '--p': pct(pl.danmaku.speed, 0.5, 2) }" /><em>{{ pl.danmaku.speed === 1 ? '适中' : pl.danmaku.speed < 1 ? '较慢' : '较快' }}</em></label></section>
        <section id="sec-cache" class="pn"><h2><Download :size="22" />下载缓存</h2>
          <label class="r">下载清晰度<span class="sel"><select v-model="st.dlQuality" data-testid="download-quality"><option v-for="q in DEMO_QUALITIES" :key="q.id" :value="q.label">{{ q.label }} {{ q.detail }}</option></select><ChevronDown :size="14" /></span></label>
          <label class="r">同时下载任务数<span class="sel"><select v-model.number="st.dlConcurrent" data-testid="dl-concurrent"><option v-for="n in [1, 2, 3, 4, 5]" :key="n" :value="n">{{ n }} 个</option></select><ChevronDown :size="14" /></span></label>
          <div class="r">缓存保存位置<div class="path"><input :value="desktop ? cache?.path ?? '' : st.cachePath" readonly aria-label="缓存路径" data-testid="cache-path" /><button v-if="!desktop || canChooseCacheDirectory" :disabled="cacheBusy" data-testid="change-path" @click="changeCacheDirectory()"><FolderOpen :size="15" />更改</button></div></div>
          <div v-if="cache?.custom || cache?.fallback || (canChooseCacheDirectory && cacheLoaded && !cache)" class="r">
            <span role="status">{{ !cache ? '缓存目录暂不可用' : cache.fallback ? '所设目录不可用，使用默认位置' : '自定义缓存位置' }}</span>
            <BaseButton v-if="canChooseCacheDirectory" variant="outline" size="sm" :disabled="cacheBusy" data-testid="reset-cache-directory" @click="changeCacheDirectory(true)">恢复默认位置</BaseButton>
          </div>
          <p v-if="desktop && cache && cache.bytes > cache.max" role="status">缓存暂时超出上限，可尝试清理缓存。</p>
          <label class="r">缓存上限<span class="sel"><select :value="currentLimit" :disabled="cacheBusy || !cache" data-testid="cache-limit" @change="setLimit"><option v-for="g in limitOptions" :key="g" :value="g">{{ g }} GB</option></select><ChevronDown :size="14" /></span></label>
          <div class="r">缓存管理<span class="cs" data-testid="cache-size">{{ cacheText }}</span><button class="clr" data-testid="clear-cache" :disabled="cacheBusy || !cache || cache.files === 0" @click="confirmClear = true"><Trash2 :size="15" />清理缓存</button></div></section>
        <section id="sec-keys" class="pn"><h2><Keyboard :size="22" />快捷键<button class="rs" data-testid="reset-keys" @click="st.shortcuts = { ...DEFAULT_SHORTCUTS }; toast.push('已恢复默认快捷键')"><RotateCcw :size="13" />恢复默认</button></h2>
          <div v-for="(l, k) in SHORTCUT_LABELS" :key="k" class="r k"><span>{{ l }}</span><button class="kbd" :class="{ rec: rec === k }" :data-testid="`key-${k}`" :aria-label="`修改${l}快捷键`" @click="startRec(k, $event)">{{ rec === k ? '按下新按键…' : st.shortcuts[k] || '未设置' }}</button></div></section>
        <section id="sec-look" class="pn"><h2><Palette :size="22" />外观</h2>
          <div class="r tm">主题模式<div class="themes"><button v-for="t in [['dark','深色（推荐）'],['light','浅色'],['system','跟随系统']] as const" :key="t[0]" :class="[t[0], { on: st.theme === t[0] }]" :data-testid="`theme-${t[0]}`" :aria-pressed="st.theme === t[0]" @click="st.theme = t[0]"><span><Sun v-if="t[0] === 'system'" :size="20" /><Moon v-if="t[0] === 'system'" :size="20" /><Check v-if="st.theme === t[0]" :size="13" class="ck" /></span><em>{{ t[1] }}</em></button></div></div>
          <label class="r">界面缩放（DPI）<input v-model.number="st.dpi" class="rg" type="range" min="80" max="150" step="10" aria-label="界面缩放" data-testid="dpi" :style="{ '--p': pct(st.dpi, 80, 150) }" /><em>{{ st.dpi }}%</em></label>
          <label class="r">字体大小<span class="sel wide"><select v-model="st.fontSize" data-testid="font-size"><option>小</option><option>标准</option><option>大</option></select><ChevronDown :size="14" /></span></label></section>
        <section id="sec-about" class="pn about"><h2><Info :size="22" />关于与更新</h2>
          <label v-if="channelAvailable" class="r">更新渠道<span class="sel"><select :value="channel" :disabled="channelBusy || checking || downloading" data-testid="update-channel" @change="changeUpdateChannel"><option value="stable">稳定版</option><option value="preview">预发布（提前试用）</option></select><ChevronDown :size="14" /></span></label>
          <div class="ab" :class="{ 'has-update': upd }"><div class="lg"><BrandLogo /></div><div class="ver"><small>当前版本</small><b data-testid="version">{{ version }}<em>演示版</em></b><small data-testid="last-check">{{ checkLabel }}</small></div>
            <div class="update-actions">
              <button class="chk" :disabled="checking || channelBusy || downloading" data-testid="check-update" @click="updates.check()"><RefreshCw :size="18" :class="{ spin: checking }" />{{ checking ? '检查中…' : '检查更新' }}</button>
              <button v-if="canDownload" class="chk download-update" :disabled="downloading" @click="updates.download()"><Download :size="18" />{{ downloading ? '正在下载并校验…' : '校验并下载安装包' }}</button>
              <button v-if="upd && !canDownload" class="chk download-update" data-testid="open-release" :disabled="checking || channelBusy" @click="viewRelease"><FileText :size="18" />{{ upd.preview ? '下载预发布版' : '前往下载' }} {{ upd.latest }}</button>
            </div>
            <div class="links">
              <button v-if="downloading" @click="updates.cancelDownload()">取消下载<ChevronRight :size="14" /></button>
              <button v-if="upd && canDownload" data-testid="open-release" @click="viewRelease"><FileText :size="16" />发布页 {{ upd.latest }}<ChevronRight :size="14" /></button>
              <button ref="releaseNotesButton" @click="showReleaseNotes = true"><FileText :size="16" />版本更新日志<ChevronRight :size="14" /></button>
              <button data-testid="copy-diag" @click="copyDiag"><FileText :size="16" />复制诊断信息<ChevronRight :size="14" /></button>
              <button ref="backupButton" data-testid="create-backup" :disabled="backupBusy" @click="createBackup"><Download :size="16" />{{ backupBusy ? '备份中…' : '创建数据备份' }}<ChevronRight :size="14" /></button>
              <button v-if="canRestore" ref="restoreButton" :disabled="backupBusy" @click="openRestore"><FolderOpen :size="16" />从备份恢复收藏和历史<ChevronRight :size="14" /></button>
              <button ref="privacyButton" data-testid="privacy-notice" @click="showPrivacy = true"><CircleHelp :size="16" />隐私与反馈<ChevronRight :size="14" /></button>
            </div>
          </div>
          <div v-if="lastSavedUpdate" ref="savedUpdateCard" class="saved-update" data-testid="saved-update">
            <p role="status">{{ lastSavedUpdate.version }} 安装包已校验并保存，可手动安装</p>
            <code>{{ lastSavedUpdate.path }}</code>
            <BaseButton variant="outline" size="sm" @click="copyUpdatePath">复制保存路径</BaseButton>
          </div>
        </section>
      </div>
    </div>
    <PrivacyNoticeDialog :open="showPrivacy" :return-focus="privacyButton" @close="showPrivacy = false" />
    <Modal :open="showReleaseNotes" title="版本更新日志" confirm-text="关闭" :show-cancel="false" wide @confirm="showReleaseNotes = false" @cancel="showReleaseNotes = false">
      <div ref="releaseNotesContent" class="release-notes-text" tabindex="0">
        <h4 v-if="upd && !checking && !channelBusy">{{ upd.latest }}</h4>
        <p v-if="channelBusy">正在处理更新渠道…</p>
        <p v-else-if="checking">正在检查更新…</p>
        <p v-else-if="!upd">暂无新版本更新日志</p>
        <p v-else-if="upd.notes.trim()" data-testid="release-notes-body">{{ upd.notes }}</p>
        <p v-else-if="!upd.notesTruncated">该版本未提供更新说明</p>
      </div>
      <div v-if="upd?.notesTruncated && !checking && !channelBusy" class="release-notes-more">
        <p>当前仅显示部分更新说明。</p>
        <BaseButton variant="outline" size="sm" @click="viewRelease">在发布页查看完整说明</BaseButton>
      </div>
    </Modal>
    <Modal :open="showBackup" title="数据备份已创建" confirm-text="关闭" :show-cancel="false" @confirm="backupPath = ''" @cancel="backupPath = ''">
      <p ref="backupMessage" class="backup-path" tabindex="0">{{ backupPath }}</p>
    </Modal>
    <Modal :open="showRestore" title="从备份恢复收藏和历史"
      :confirm-text="restoreBusy ? '正在恢复…' : restoreMessage ? '关闭' : '选择备份'"
      :confirm-disabled="restoreBusy" :show-cancel="!restoreBusy && !restoreMessage"
      @confirm="confirmRestore" @cancel="closeRestore">
      <p ref="restoreDescription" tabindex="0" :role="restoreFailed ? 'alert' : 'status'">
        {{ restoreBusy ? '正在读取并恢复备份…' : restoreMessage || '请选择金帝短剧生成的数据库备份。只补回缺失的收藏和观看历史，保留当前已有记录。' }}
      </p>
    </Modal>
    <Modal :open="confirmClear" title="清理缓存" confirm-text="确认清理" @cancel="confirmClear = false" @confirm="clearCache">将删除全部已缓存的下载文件，不影响收藏和观看历史。</Modal>
  </div>
</template>
<style scoped lang="scss">
.set { padding: 10px 16px 24px 14px; header { display: flex; align-items: center; gap: 14px; height: 78px; padding: 0 22px; border-radius: 24px; background: linear-gradient(90deg, rgba(120,32,28,.35), rgba(60,20,22,.2)); border: 1px solid var(--c-border); h1 { font-size: 34px; } p { font-size: 14px; color: var(--c-text-2); margin-left: 8px; } } }
.cols { display: flex; gap: 14px; margin-top: 12px; align-items: flex-start; }
.nav { position: sticky; top: 0; width: 196px; flex: none; padding: 8px; border-radius: 18px; background: rgba(255,255,255,.05); border: 1px solid var(--c-border); display: flex; flex-direction: column; gap: 2px; button { height: 50px; display: flex; align-items: center; gap: 14px; padding: 0 16px; border-radius: 12px; font-size: 16px; text-align: left; position: relative; &:hover { background: var(--c-surface-hover); } &.on { background: var(--g-nav-active); } .ch { margin-left: auto; } } }
.panels { flex: 1; min-width: 0; display: grid; grid-template-columns: 1fr 1fr; gap: 12px; align-items: start; }
.pn { padding: 14px 18px 8px; border-radius: 18px; background: linear-gradient(180deg, rgba(255,255,255,.065), rgba(255,255,255,.03)); border: 1px solid var(--c-border); scroll-margin-top: 4px; h2 { display: flex; align-items: center; gap: 10px; font-size: 20px; margin-bottom: 4px; svg { color: var(--c-accent-2); } } &.about { grid-column: 1 / -1; } }
.r { display: flex; align-items: center; gap: 12px; min-height: 42px; font-size: 14.5px; border-top: 1px solid var(--c-border); &:first-of-type { border-top: 0; } > div:first-child:not(.path) { flex: 1; } small { display: block; font-size: 12px; color: var(--c-text-2); } em { font-style: normal; width: 48px; text-align: right; color: var(--c-text-2); } }
.r > .sel, .r > .rg, .r > .path { margin-left: auto; }
.sel { position: relative; display: inline-flex; align-items: center; select { appearance: none; height: 34px; min-width: 190px; padding: 0 34px 0 14px; border-radius: 10px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border-strong); color: var(--c-text); outline: none; font-size: 14px; option { background: #2a1719; } &:focus-visible { outline: 2px solid var(--c-accent-2); } } svg { position: absolute; right: 12px; pointer-events: none; } &.wide select { min-width: 240px; } }
.tg { width: 46px; height: 26px; border-radius: 13px; background: var(--c-border-strong); box-shadow: inset 0 0 0 1px var(--c-border); position: relative; margin-left: auto; transition: background .2s; i { position: absolute; top: 3px; left: 3px; width: 20px; height: 20px; border-radius: 50%; background: #fff; transition: transform .2s; } &.on { background: #ee4b39; i { transform: translateX(20px); } } }
.rg { appearance: none; width: 190px; height: 5px; border-radius: 5px; outline: none; background: linear-gradient(90deg, #ff7a5c var(--p), rgba(255,255,255,.2) var(--p)); &::-webkit-slider-thumb { appearance: none; width: 17px; height: 17px; border-radius: 50%; background: #fff; box-shadow: 0 0 0 4px rgba(238,75,57,.35); cursor: pointer; } }
.path { display: flex; gap: 8px; input { width: 200px; height: 34px; padding: 0 12px; border-radius: 10px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border); color: var(--c-text); font-size: 13px; } button { display: inline-flex; align-items: center; gap: 5px; height: 34px; padding: 0 14px; border-radius: 10px; border: 1px solid var(--c-border-strong); } }
.cs { margin-left: auto; font-size: 13px; color: var(--c-text-2); } .clr { display: inline-flex; align-items: center; gap: 6px; height: 34px; padding: 0 16px; border-radius: 17px; background: var(--g-accent); font-weight: 600; &:disabled { opacity: .45; } }
.rs { margin-left: auto; display: inline-flex; align-items: center; gap: 4px; font-size: 12.5px; font-weight: 400; color: var(--c-text-2); }
.k { min-height: 32px; } .kbd { margin-left: auto; min-width: 86px; height: 26px; padding: 0 10px; border-radius: 6px; background: rgba(255,255,255,.1); border: 1px solid var(--c-border-strong); font-size: 12.5px; &.rec { border-color: var(--c-accent); color: var(--c-accent-2); } }
.tm { align-items: flex-start; padding-top: 8px; white-space: nowrap; } .themes { display: flex; gap: 12px; margin-left: auto; button { width: 118px; text-align: center; span { position: relative; display: grid; place-items: center; height: 60px; border-radius: 10px; border: 2px solid transparent; background: linear-gradient(135deg, #7a2420, #2a1214); gap: 4px; } &.light span { background: linear-gradient(135deg, #fbe1d8, #f3b7a4); } &.system span { background: linear-gradient(90deg, #6a6f7a 50%, #3a3e48 50%); grid-auto-flow: column; } &.on span { border-color: var(--c-accent); } .ck { position: absolute; right: 4px; bottom: 4px; background: var(--c-accent); border-radius: 50%; padding: 2px; } em { display: block; width: auto; margin-top: 6px; font-size: 12.5px; text-align: center; } } }
.ab {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 36px;
  padding: 12px 8px 14px;

  .ver {
    flex: 1 1 170px;
    min-width: 0;
    overflow-wrap: anywhere;
    small { color: var(--c-text-2); font-size: 13px; }
    b { display: block; font-size: 30px; margin: 4px 0;
      em { display: inline-block; width: auto; margin-left: 10px; padding: 1px 10px; border-radius: 6px; background: rgba(70,194,138,.2); color: var(--c-success); font-size: 12px; font-weight: 400; vertical-align: middle; }
    }
  }
  .update-actions { display: flex; flex-direction: column; gap: 10px; min-width: 0; max-width: 100%; }
  .chk {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    min-height: 52px;
    padding: 8px 56px;
    border-radius: 26px;
    background: var(--g-accent);
    overflow-wrap: anywhere;
    font-size: 18px;
    font-weight: 700;
    svg { flex: none; }
    &:disabled { opacity: .7; }
    &.download-update { padding-inline: 24px; }
  }
  .links {
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
    max-width: 100%;
    button {
      display: flex;
      align-items: center;
      gap: 8px;
      max-width: 100%;
      overflow-wrap: anywhere;
      text-align: left;
      font-size: 14.5px;
      svg { flex: none; }
      svg:last-child { margin-left: 6px; }
    }
  }
}
.backup-path { overflow-wrap: anywhere; }
.saved-update {
  display: grid;
  gap: 8px;
  padding: 12px 8px;
  border-top: 1px solid var(--c-border);
  code { color: var(--c-text-2); overflow-wrap: anywhere; user-select: text; }
  button { justify-self: start; }
}
.release-notes-more { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 14px; margin-top: 12px; }
.release-notes-text { max-height: 52vh; overflow-y: auto; overflow-wrap: anywhere; white-space: pre-wrap; h4 { margin-bottom: 8px; } }
.spin { animation: sp 1s linear infinite; } @keyframes sp { to { transform: rotate(360deg); } }
.ab.has-update .links {
  flex: 1 0 100%;
  flex-direction: row;
  flex-wrap: wrap;
  gap: 14px 28px;
}


.set.stacked, .set.compact {
  .panels { grid-template-columns: minmax(0, 1fr); }
  .r { flex-wrap: wrap; padding-block: 6px; }
  .r > .sel, .r > .path, .themes { max-width: 100%; }
  .sel select { max-width: 100%; min-width: min(190px, 100%); }
  .sel.wide select { min-width: min(240px, 100%); }
  .path input { min-width: 0; }
  .path button { flex: none; }
  .themes { flex-wrap: wrap; }
  .tm { white-space: normal; }
  .ab { gap: 16px; .chk { padding-inline: 24px; } }
}

.set.compact {
  & > header {
    height: auto;
    min-height: 78px;
    padding-block: 12px;
    h1, > svg { flex: none; }
    p { margin-left: 0; }
  }
  .cols { flex-direction: column; align-items: stretch; }
  .nav {
    position: static;
    width: auto;
    flex-direction: row;
    flex-wrap: wrap;
    button { flex: 1 0 128px; padding-inline: 10px; gap: 8px; }
  }
}
</style>
