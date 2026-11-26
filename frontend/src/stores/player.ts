import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { DEFAULT_DANMAKU, type DanmakuSettings } from '@/player/danmaku'
import { saveSetting, LS } from '@/bindings'
import { DEFAULT_PLAYBACK_VOLUME, isPlaybackSpeed, normalizePlaybackVolume } from '@/player/preferences'
import { normalizeDemoQuality } from '@/player/demoMedia'
import { useAppSettings } from './appSettings'
const KEY = LS.player
export const useSettingsStore = defineStore('playerSettings', () => {
  const preferences = useAppSettings().s
  const defaultSpeed = preferences.defaultSpeed
  let init: { danmaku: DanmakuSettings; speed: number; quality: string; volume: number } = { danmaku: { ...DEFAULT_DANMAKU }, speed: defaultSpeed, quality: preferences.defaultQuality, volume: DEFAULT_PLAYBACK_VOLUME }
  try { const r = localStorage.getItem(KEY); if (r) init = { ...init, ...JSON.parse(r) } } catch { /* ignore */ }
  const danmaku = ref<DanmakuSettings>({ ...DEFAULT_DANMAKU, ...init.danmaku })
  const speed = ref(isPlaybackSpeed(init.speed) ? init.speed : defaultSpeed); const quality = ref(normalizeDemoQuality(init.quality)); const volume = ref(normalizePlaybackVolume(init.volume))
  watch([danmaku, speed, quality, volume], () => saveSetting(LS.player, { danmaku: danmaku.value, speed: speed.value, quality: quality.value, volume: volume.value }), { deep: true })
  const addBlock = (w: string) => { const t = w.trim(); if (!t) return false; if (danmaku.value.blocked.includes(t)) return false; danmaku.value.blocked = [...danmaku.value.blocked, t]; return true }
  const removeBlock = (w: string) => { danmaku.value.blocked = danmaku.value.blocked.filter((x) => x !== w) }
  return { danmaku, speed, quality, volume, addBlock, removeBlock }
})
