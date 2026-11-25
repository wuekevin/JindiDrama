<script setup lang="ts">
import { onBeforeUnmount, onMounted, shallowRef } from 'vue'
import { useToastStore } from '@/stores/toast'
const t = useToastStore()
const fullscreenTarget = shallowRef<Element | null>(null)
const updateTarget = () => { fullscreenTarget.value = document.fullscreenElement }
onMounted(() => {
  updateTarget()
  document.addEventListener('fullscreenchange', updateTarget)
})
onBeforeUnmount(() => document.removeEventListener('fullscreenchange', updateTarget))
</script>
<template>
  <Teleport :to="fullscreenTarget ?? 'body'" :disabled="!fullscreenTarget">
    <div class="toasts" role="status" aria-live="polite">
      <div v-for="i in t.items" :key="i.id" class="toast on-dark" :class="i.kind" data-testid="toast">
        <span>{{ i.text }}</span>
        <button v-if="i.action" type="button" :disabled="i.action.busy()" @click="i.action.run()">
          {{ i.action.busy() ? '正在重试…' : i.action.label }}
        </button>
      </div>
    </div>
  </Teleport>
</template>
<style scoped lang="scss">
.toasts { position: fixed; left: 50%; bottom: 36px; transform: translateX(-50%); display: flex; flex-direction: column; gap: 8px; z-index: 200; pointer-events: none; }
.toast { padding: 10px 18px; border-radius: 12px; background: rgba(30, 18, 20, 0.96); border: 1px solid var(--c-border-strong); font-size: 14px; box-shadow: var(--shadow-card);
  button {
    margin-inline-start: 12px;
    padding: 4px 8px;
    border-radius: 6px;
    background: var(--c-surface-hover);
    color: inherit;
    pointer-events: auto;
    &:disabled { opacity: .6; }
  }
  &.success { border-color: var(--c-success); } &.error { border-color: var(--c-danger); } }
</style>
