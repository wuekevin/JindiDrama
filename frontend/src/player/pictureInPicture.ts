import { ref, watch, type Ref } from 'vue'
import { useToastStore } from '@/stores/toast'

type VideoWithWebKitPresentation = HTMLVideoElement & {
  webkitPresentationMode?: 'inline' | 'fullscreen' | 'picture-in-picture'
  webkitSupportsPresentationMode?(mode: 'picture-in-picture'): boolean
  webkitSetPresentationMode?(mode: 'inline' | 'picture-in-picture'): void
}

const events = ['enterpictureinpicture', 'leavepictureinpicture', 'webkitpresentationmodechanged']

function isActive(video: VideoWithWebKitPresentation) {
  return document.pictureInPictureElement === video || video.webkitPresentationMode === 'picture-in-picture'
}

function webKitTransition(video: VideoWithWebKitPresentation, mode: 'inline' | 'picture-in-picture') {
  return new Promise<void>((resolve, reject) => {
    function cleanup() {
      window.clearTimeout(timeout)
      video.removeEventListener('webkitpresentationmodechanged', sync)
    }
    function sync() {
      if (video.webkitPresentationMode !== mode) return
      cleanup()
      resolve()
    }
    const timeout = window.setTimeout(() => {
      cleanup()
      reject(new Error('Picture-in-picture transition timed out'))
    }, 3000)
    video.addEventListener('webkitpresentationmodechanged', sync)
    try {
      video.webkitSetPresentationMode?.(mode)
      sync()
    } catch (error) {
      cleanup()
      reject(error)
    }
  })
}

async function close(video: VideoWithWebKitPresentation) {
  if (document.pictureInPictureElement === video) {
    await document.exitPictureInPicture()
  } else if (video.webkitPresentationMode === 'picture-in-picture' && video.webkitSetPresentationMode) {
    await webKitTransition(video, 'inline')
  }
}

export function usePictureInPicture(videoRef: Ref<HTMLVideoElement | null>, simulated = false) {
  const toast = useToastStore()
  const active = ref(false)
  const busy = ref(false)
  const pending = new WeakMap<HTMLVideoElement, number>()
  let generation = 0
  function reset() {
    generation++
    active.value = false
    if (videoRef.value) void close(videoRef.value).catch(() => {})
  }

  watch(videoRef, (video, _, onCleanup) => {
    active.value = video ? isActive(video) : false
    if (!video) return
    const sync = () => {
      const request = pending.get(video)
      if (request === undefined || request === generation) active.value = isActive(video)
    }
    for (const event of events) video.addEventListener(event, sync)
    video.addEventListener('emptied', reset)
    onCleanup(() => {
      generation++
      busy.value = false
      for (const event of events) video.removeEventListener(event, sync)
      video.removeEventListener('emptied', reset)
      void close(video).catch(() => {})
    })
  }, { immediate: true, flush: 'sync' })

  async function toggle() {
    const video: VideoWithWebKitPresentation | null = videoRef.value
    if (video && pending.has(video)) return
    if (simulated) {
      toast.push('模拟画面无法开启画中画')
      return
    }
    if (!video || (!isActive(video) && (video.readyState < HTMLMediaElement.HAVE_METADATA || !video.videoWidth))) {
      toast.push('视频就绪后即可开启画中画')
      return
    }
    const current = generation
    pending.set(video, current)
    busy.value = true
    try {
      const legacy = video.webkitSetPresentationMode && video.webkitSupportsPresentationMode?.('picture-in-picture')
      if (isActive(video)) {
        await close(video)
      } else if (document.pictureInPictureEnabled && typeof video.requestPictureInPicture === 'function') {
        try {
          await video.requestPictureInPicture()
        } catch (error) {
          if (generation !== current || !legacy || !(error instanceof DOMException) || error.name !== 'NotSupportedError') throw error
          await webKitTransition(video, 'picture-in-picture')
        }
      } else if (legacy) {
        await webKitTransition(video, 'picture-in-picture')
      } else {
        toast.push('当前环境不支持画中画')
      }
      if (generation !== current) await close(video)
    } catch (error) {
      if (generation === current) {
        toast.push(error instanceof DOMException && error.name === 'NotSupportedError'
          ? '当前环境不支持此视频的画中画' : '无法切换画中画，请稍后重试', 'error')
      }
    } finally {
      pending.delete(video)
      if (videoRef.value === video) {
        if (generation === current) active.value = isActive(video)
        busy.value = false
      }
    }
  }

  return { active, busy, toggle, reset }
}
