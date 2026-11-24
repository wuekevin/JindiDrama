<script setup lang="ts">
import type { Drama } from '@/types'
defineProps<{ drama: Drama; badge?: string }>()
defineEmits<{ open: [id: string] }>()
</script>
<template>
  <button class="card on-dark" :aria-label="`${drama.title}，${drama.episodeCount}集`" data-testid="drama-card" @click="$emit('open', drama.id)">
    <img :src="drama.cover" :alt="drama.title" draggable="false" />
    <span v-if="badge" class="badge">{{ badge }}</span>
    <span class="shade" />
    <span class="info"><b class="clamp-2">{{ drama.title }}</b><em>{{ drama.tags.slice(0, 2).join(' · ') }} · {{ drama.episodeCount }}集</em></span>
  </button>
</template>
<style scoped lang="scss">
.card { position: relative; display: block; width: 100%; height: 230px; border-radius: 16px; overflow: hidden; text-align: left; background: var(--c-surface-solid); border: 1px solid var(--c-border); transition: transform .2s var(--ease), box-shadow .2s;
  &:hover { transform: translateY(-3px); box-shadow: 0 10px 24px rgba(0,0,0,.5); } img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; } }
.badge { position: absolute; left: 0; top: 0; padding: 3px 10px 3px 9px; border-radius: 0 0 10px 0; background: linear-gradient(90deg, #f0583f, #e0402e); font-size: 12px; font-weight: 600; }
.shade { position: absolute; inset: 45% 0 0 0; background: linear-gradient(180deg, transparent, rgba(0,0,0,.85)); }
.info { position: absolute; left: 12px; right: 10px; bottom: 10px; display: flex; flex-direction: column; gap: 4px; b { font-size: 17px; line-height: 1.25; } em { font-style: normal; font-size: 12.5px; color: var(--c-text-2); } }
</style>
