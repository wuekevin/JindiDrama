<script setup lang="ts">
import { useToastStore } from '@/stores/toast'
const t = useToastStore()
</script>
<template>
  <div class="toasts" role="status" aria-live="polite">
    <div v-for="i in t.items" :key="i.id" class="toast" :class="i.kind" data-testid="toast">{{ i.text }}</div>
  </div>
</template>
<style scoped lang="scss">
.toasts { position: fixed; left: 50%; bottom: 36px; transform: translateX(-50%); display: flex; flex-direction: column; gap: 8px; z-index: 200; }
.toast { padding: 10px 18px; border-radius: 12px; background: rgba(30, 18, 20, 0.96); border: 1px solid var(--c-border-strong); font-size: 14px; box-shadow: var(--shadow-card);
  &.success { border-color: var(--c-success); } &.error { border-color: var(--c-danger); } }
</style>
