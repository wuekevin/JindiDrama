<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'primary'|'ghost'|'outline'; size?: 'sm'|'md'|'lg'; disabled?: boolean }>(), { variant: 'primary', size: 'md' })
</script>
<template><button class="btn" :class="[variant, size]" :disabled="disabled"><slot /></button></template>
<style scoped lang="scss">
.btn { display: inline-flex; align-items: center; justify-content: center; gap: 8px; border-radius: var(--r-pill); font-weight: 600; white-space: nowrap; transition: filter .15s, background .15s, transform .1s;
  &.sm { height: 30px; padding: 0 14px; font-size: 13px; } &.md { height: 38px; padding: 0 20px; font-size: 15px; } &.lg { height: 46px; padding: 0 28px; font-size: 17px; }
  &.primary { background: var(--g-accent); color: #fff; box-shadow: 0 6px 16px rgba(220, 58, 42, .35); &:hover:not(:disabled) { filter: brightness(1.08); } &:active:not(:disabled) { transform: translateY(1px); } }
  &.ghost { background: var(--c-surface-hover); border: 1px solid var(--c-border-strong); &:hover:not(:disabled) { background: rgba(255,255,255,.14); } }
  &.outline { border: 1px solid var(--c-accent); color: var(--c-accent-2); &:hover:not(:disabled) { background: var(--c-accent-soft); } }
  &:disabled { opacity: .45; cursor: not-allowed; } }
</style>
