import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { cancelUpdateDownload, checkUpdate, downloadUpdate, getUpdateChannel, openReleasePage, setUpdateChannel, supportsUpdateDownload, type UpdateChannel, type UpdateResult } from '@/bindings'
import { useToastStore } from '@/stores/toast'

const checkKey = (channel: UpdateChannel) => channel === 'preview' ? 'jindi.last-update-check.preview' : 'jindi.last-update-check'

export const useDesktopUpdates = defineStore('desktopUpdates', () => {
  const toast = useToastStore()
  const channel = ref<UpdateChannel>('stable')
  const channelBusy = ref(false)
  const channelAvailable = ref(false)
  const checking = ref(false)
  const downloading = ref(false)
  const lastSavedUpdate = ref<{ path: string; version: string } | null>(null)
  const lastCheck = ref<number | null>(null)
  const upd = ref<UpdateResult | null>(null)
  const checkMessage = ref('')
  const canDownload = computed(() => Boolean(upd.value?.downloadName) && supportsUpdateDownload())
  const noticeKey = computed(() => upd.value ? `jindi.update-read.${channel.value}.${upd.value.latest}` : '')
  const read = ref(new Set<string>())
  const unread = computed(() => {
    if (channelBusy.value || !noticeKey.value || read.value.has(noticeKey.value)) return false
    try { return !localStorage.getItem(noticeKey.value) } catch { return true }
  })
  let loaded = false

  function acknowledge() {
    const key = noticeKey.value
    if (!key || !unread.value) return
    read.value.add(key)
    try { localStorage.setItem(key, '1') } catch { /* 当前会话仍可标记已读 */ }
  }

  function restoreLastCheck() {
    lastCheck.value = null
    try {
      const saved = Number(localStorage.getItem(checkKey(channel.value)))
      if (saved > 0 && Number.isFinite(new Date(saved).getTime())) lastCheck.value = saved
    } catch { /* 检查更新仍可用 */ }
  }

  function setCurrentChannel(next: UpdateChannel) {
    if (channel.value !== next) lastSavedUpdate.value = null
    channel.value = next
  }

  async function loadChannel() {
    if (loaded || channelBusy.value) return
    channelBusy.value = true
    try {
      const saved = await getUpdateChannel()
      if (saved !== null) {
        setCurrentChannel(saved)
        channelAvailable.value = true
      }
      loaded = true
    } catch { /* 下次进入设置页时可重试 */ }
    finally {
      restoreLastCheck()
      channelBusy.value = false
    }
  }

  async function chooseChannel(next: UpdateChannel) {
    if (channelBusy.value || checking.value || downloading.value) return false
    channelBusy.value = true
    try {
      await setUpdateChannel(next)
      setCurrentChannel(next)
      upd.value = null
      checkMessage.value = ''
      restoreLastCheck()
      return true
    } catch {
      toast.push('无法保存更新渠道，请稍后重试', 'error')
      return false
    } finally {
      channelBusy.value = false
    }
  }

  async function check(showToast = true) {
    if (checking.value || channelBusy.value || downloading.value) return
    checking.value = true
    upd.value = null
    checkMessage.value = ''
    function report(message: string, kind: 'info' | 'success' | 'error') {
      checkMessage.value = message
      if (showToast) toast.push(message, kind)
    }
    try {
      const result = await checkUpdate()
      if (result === null) {
        report('浏览器预览模式不支持检查更新，请在桌面端使用', 'info')
        return
      }
      if (result.channel === 'stable' || result.channel === 'preview') setCurrentChannel(result.channel)
      lastCheck.value = Date.now()
      try { localStorage.setItem(checkKey(channel.value), String(lastCheck.value)) } catch { /* 保留内存状态 */ }
      upd.value = result.hasUpdate ? result : null
      if (result.hasUpdate) report(`发现新版本 ${result.latest}`, 'success')
      else if (result.noRelease) report(channel.value === 'preview' ? '暂无可用发布版本' : '暂无正式发布版本', 'success')
      else report('暂无可用更新', 'success')
    } catch {
      report('检查更新失败，请稍后重试', 'error')
    } finally {
      checking.value = false
    }
  }

  async function openRelease() {
    if (checking.value || channelBusy.value || !upd.value) return false
    try {
      await openReleasePage(upd.value.url)
      return true
    } catch {
      return false
    }
  }

  let unverifiedDownloadToast: number | undefined
  function dismissUnverifiedDownloadToast() {
    if (unverifiedDownloadToast === undefined) return
    toast.dismiss(unverifiedDownloadToast)
    unverifiedDownloadToast = undefined
  }

  async function download() {
    const candidate = upd.value
    if (!candidate || !canDownload.value || checking.value || channelBusy.value || downloading.value) return
    downloading.value = true
    try {
      const saved = await downloadUpdate(candidate.url)
      if (typeof saved === 'object' && saved.unverified) {
        lastSavedUpdate.value = null
        dismissUnverifiedDownloadToast()
        unverifiedDownloadToast = toast.push(saved.unverified, 'error')
        return
      }
      const path = typeof saved === 'string' ? saved : typeof saved === 'object' ? saved.path : ''
      if (path || saved === true) dismissUnverifiedDownloadToast()
      if (path) {
        lastSavedUpdate.value = { path, version: candidate.latest }
        if (typeof saved === 'object' && saved.warning) toast.push(saved.warning, 'info')
      } else if (saved === true) {
        toast.push(`${candidate.latest} 安装包已校验并保存，可手动安装`, 'success')
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : typeof error === 'string' ? error : '无法下载更新'
      toast.push(message, 'error')
    } finally {
      downloading.value = false
    }
  }

  async function cancelDownload() {
    try { await cancelUpdateDownload() } catch { toast.push('无法取消下载，请稍后重试', 'error') }
  }

  return { channel, channelBusy, channelAvailable, checking, downloading, lastSavedUpdate, canDownload, lastCheck, upd, checkMessage, noticeKey, unread, acknowledge, loadChannel, chooseChannel, check, openRelease, download, cancelDownload }
})
