<script setup lang="ts">
import { computed } from 'vue'
import type { DanmakuItem } from '@/player/danmaku'
import { place } from '@/player/danmaku'
import { useSettingsStore } from '@/stores/player'
const props = defineProps<{ items: DanmakuItem[]; time: number }>()
const s = useSettingsStore()
const placed = computed(() => place(props.items, props.time, s.danmaku))
</script>
<template>
  <div class="dm" aria-hidden="true" data-testid="danmaku-layer">
    <span v-for="p in placed" :key="p.item.id" class="d" :class="{ hot: p.item.hot, mine: p.item.mine }" data-testid="danmaku-item"
      :style="{ top: p.y * 100 + '%', left: (100 - p.x * 130) + '%', opacity: s.danmaku.opacity / 100, fontSize: s.danmaku.fontSize + 'px' }">{{ p.item.text }}</span>
  </div>
</template>
<style scoped lang="scss">
.dm { position: absolute; inset: 40px 0 70px; overflow: hidden; pointer-events: none; }
.d { position: absolute; white-space: nowrap; font-weight: 600; text-shadow: 0 1px 4px rgba(0,0,0,.9); transform: translateY(-50%); &.mine { outline: 1px solid var(--c-accent-2); border-radius: 12px; padding: 1px 10px; } &.hot { color: #ffd9c9; } }
</style>
