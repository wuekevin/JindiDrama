import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { DEFAULT_DANMAKU, type DanmakuSettings } from '@/player/danmaku'
import { saveSetting, LS } from '@/bindings'
const KEY = LS.player
export const useSettingsStore = defineStore('playerSettings', () => {
  let init: { danmaku: DanmakuSettings; speed: number; quality: string; volume: number } = { danmaku: { ...DEFAULT_DANMAKU }, speed: 1, quality: '蓝光', volume: 70 }
  try { const r = localStorage.getItem(KEY); if (r) init = { ...init, ...JSON.parse(r) } } catch { /* ignore */ }
  const danmaku = ref<DanmakuSettings>({ ...DEFAULT_DANMAKU, ...init.danmaku })
  const speed = ref(init.speed); const quality = ref(init.quality); const volume = ref(init.volume)
  watch([danmaku, speed, quality, volume], () => saveSetting(LS.player, { danmaku: danmaku.value, speed: speed.value, quality: quality.value, volume: volume.value }), { deep: true })
  const addBlock = (w: string) => { const t = w.trim(); if (!t) return false; if (danmaku.value.blocked.includes(t)) return false; danmaku.value.blocked = [...danmaku.value.blocked, t]; return true }
  const removeBlock = (w: string) => { danmaku.value.blocked = danmaku.value.blocked.filter((x) => x !== w) }
  return { danmaku, speed, quality, volume, addBlock, removeBlock }
})
