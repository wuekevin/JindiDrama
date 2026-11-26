<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { DanmakuItem } from '@/player/danmaku'
import { activeDanmaku, scheduleDanmaku } from '@/player/danmaku'
import { useSettingsStore } from '@/stores/player'
const props = defineProps<{ items: DanmakuItem[]; time: number; duration: number }>()
const s = useSettingsStore()
const layer = ref<HTMLElement>()
const width = ref(0)
const height = ref(0)
const fontFamily = ref('sans-serif')
const textContext = document.createElement('canvas').getContext('2d')
let observer: ResizeObserver | undefined

onMounted(() => {
  const element = layer.value
  if (!element) return
  const measure = () => {
    width.value = element.clientWidth
    height.value = element.clientHeight
  }
  fontFamily.value = getComputedStyle(element).fontFamily
  measure()
  observer = new ResizeObserver(measure)
  observer.observe(element)
})
onBeforeUnmount(() => observer?.disconnect())

const schedule = computed(() => {
  if (textContext) textContext.font = `600 ${s.danmaku.fontSize}px ${fontFamily.value}`
  const widths = new Map<string, number>()
  return scheduleDanmaku(props.items, s.danmaku, width.value, height.value, (item) => {
    let measured = widths.get(item.text)
    if (measured === undefined) {
      measured = Math.ceil(textContext?.measureText(item.text).width ?? item.text.length * s.danmaku.fontSize)
      widths.set(item.text, measured)
    }
    return measured + (item.mine ? 20 : 0)
  }, props.duration > 0 ? props.duration : undefined)
})
const placed = computed(() => activeDanmaku(schedule.value, props.time))
</script>
<template>
  <div ref="layer" class="dm" aria-hidden="true" data-testid="danmaku-layer">
    <span v-for="p in placed" :key="p.item.id" class="d" :class="{ hot: p.item.hot, mine: p.item.mine }" data-testid="danmaku-item"
      :style="{ top: p.y * 100 + '%', left: p.left + 'px', opacity: s.danmaku.opacity / 100, fontSize: s.danmaku.fontSize + 'px' }">{{ p.item.text }}</span>
  </div>
</template>
<style scoped lang="scss">
.dm { position: absolute; inset: var(--danmaku-top) 0 var(--danmaku-bottom); overflow: hidden; pointer-events: none; }
.d { position: absolute; white-space: nowrap; font-weight: 600; line-height: 1.2; text-shadow: 0 1px 4px rgba(0,0,0,.9); transform: translateY(-50%); &.mine { outline: 1px solid var(--c-accent-2); border-radius: 12px; padding: 1px 10px; } &.hot { color: #ffd9c9; } }
</style>
