import type { Router } from 'vue-router'
import { nextTick } from 'vue'
import { hasBridge } from '@/bindings'
import { byId } from '@/mocks/data'
import { useToastStore } from '@/stores/toast'

const routePattern = /^\/(drama|watch)\/(d[1-9][0-9]{0,5})(?:\/([1-9][0-9]{0,3}))?$/
export const SHARE_LINK_PAUSE = 'jindi-share-link-pause'
function validPath(path: string) {
  const match = routePattern.exec(path)
  if (!match || match[0] !== path) return false
  const [, kind, id, episode] = match
  const drama = byId(id)
  return !!drama && (kind === 'drama' ? !episode : Number(episode) <= drama.updatedTo)
}
const landingPath = (path: string) => path.startsWith('/watch/') ? path + '?paused=1' : path

function previewLink(path: string) {
  const url = new URL(window.location.href)
  url.search = ''
  url.hash = landingPath(path)
  return url.href
}

function desktopRuntime() {
  return (window as Window & { runtime?: {
    ClipboardSetText?(text: string): Promise<boolean>
    EventsOn?(name: string, callback: (path: unknown) => void): void
  } }).runtime
}

export async function copyShareLink(path: string) {
  const toast = useToastStore()
  if (!validPath(path)) {
    toast.push('当前内容没有可分享的链接', 'error')
    return
  }
  const desktop = hasBridge()
  const link = desktop ? 'jindi://' + path.slice(1) : previewLink(path)
  const runtime = desktopRuntime()
  try {
    if (desktop && runtime?.ClipboardSetText) {
      if (!(await runtime.ClipboardSetText(link))) throw new Error('clipboard')
    } else {
      await navigator.clipboard.writeText(link)
    }
    toast.push('分享链接已复制', 'success')
  } catch {
    toast.push('无法复制分享链接，请检查剪贴板权限', 'error')
  }
}

export function registerShareLinks(router: Router) {
  desktopRuntime()?.EventsOn?.('jindi-open-share-link', async (path) => {
    if (typeof path !== 'string' || !validPath(path)) return
    const target = landingPath(path)
    await router.push(target)
    await nextTick()
    if (path.startsWith('/watch/') && router.currentRoute.value.fullPath === target) {
      window.dispatchEvent(new Event(SHARE_LINK_PAUSE))
    }
  })
}
