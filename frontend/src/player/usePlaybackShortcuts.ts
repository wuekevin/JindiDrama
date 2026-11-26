import { onBeforeUnmount, onMounted } from 'vue'
import { normalizeKey, useAppSettings } from '@/stores/appSettings'

export function usePlaybackShortcuts(
  actions: Record<string, () => void>,
  options: { enabled?: () => boolean; escape?: () => void } = {},
) {
  const preferences = useAppSettings()

  function key(e: KeyboardEvent) {
    const target = e.target
    if (options.enabled?.() === false || e.defaultPrevented || e.isComposing || e.keyCode === 229 ||
        !(target instanceof HTMLElement) || target.isContentEditable ||
        target.closest('input,textarea,select,[data-player-scroll],[data-player-shortcuts-ignore]') ||
        document.querySelector('[aria-modal=true]')) return
    if (e.key === 'Escape') {
      options.escape?.()
      return
    }
    if (e.key === ' ' && target.closest('button,a[href],summary,[role=button]')) return
    const pressed = normalizeKey(e)
    if (!pressed) return
    const action = Object.keys(actions).find((name) => preferences.s.shortcuts[name] === pressed)
    if (!action) return
    e.preventDefault()
    if (!e.repeat || action === 'forward' || action === 'back') actions[action]()
  }

  onMounted(() => window.addEventListener('keydown', key))
  onBeforeUnmount(() => window.removeEventListener('keydown', key))
}
