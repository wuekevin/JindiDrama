<script setup lang="ts">import { Inbox } from 'lucide-vue-next'; defineProps<{ title: string; hint?: string }>()</script>
<template><div class="empty" data-testid="empty"><Inbox :size="48" :stroke-width="1.3" /><h3>{{ title }}</h3><p v-if="hint">{{ hint }}</p><slot /></div></template>
<style scoped lang="scss">.empty { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--c-text-2); h3 { font-size: 18px; color: var(--c-text); } p { font-size: 14px; } }</style>
