<script setup lang="ts">
import { X } from 'lucide-vue-next'
defineProps<{ open: boolean; title: string }>(); defineEmits<{ close: [] }>()
</script>
<template>
  <aside v-if="open" class="drawer" role="complementary" :aria-label="title" data-testid="drawer" @keydown.esc="$emit('close')">
    <header><h3>{{ title }}</h3><button aria-label="关闭" @click="$emit('close')"><X :size="22" /></button></header><slot />
  </aside>
</template>
<style scoped lang="scss">.drawer { display: flex; flex-direction: column; border-radius: 20px; background: linear-gradient(180deg, #2a1618, #1f1214); border: 1px solid var(--c-border); header { display: flex; justify-content: space-between; align-items: center; padding: 18px 22px 8px; h3 { font-size: 22px; } } }</style>
