<script setup lang="ts">
import BaseButton from './BaseButton.vue'
withDefaults(defineProps<{ open: boolean; title: string; confirmText?: string; confirmDisabled?: boolean; danger?: boolean; showCancel?: boolean; wide?: boolean }>(), { showCancel: true })
defineEmits<{ confirm: []; cancel: [] }>()
function keepFocus(event: KeyboardEvent) {
  const dialog = event.currentTarget as HTMLElement
  const controls = [...dialog.querySelectorAll<HTMLElement>(
    'button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
  )]
  const first = controls[0]
  const last = controls.at(-1)
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog)) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
</script>
<template>
  <Teleport to="body">
    <div v-if="open" class="mask" @click.self="$emit('cancel')" @keydown.esc="$emit('cancel')">
      <div class="modal on-dark" :class="{ wide }" @keydown.tab="keepFocus" role="dialog" aria-modal="true" :aria-label="title" tabindex="-1" data-testid="modal">
        <h3>{{ title }}</h3><div class="body"><slot /></div>
        <div class="foot"><BaseButton v-if="showCancel" variant="ghost" @click="$emit('cancel')">取消</BaseButton><BaseButton data-testid="modal-confirm" :disabled="confirmDisabled" @click="$emit('confirm')">{{ confirmText ?? '确定' }}</BaseButton></div>
      </div>
    </div>
  </Teleport>
</template>
<style scoped lang="scss">
.mask { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: grid; place-items: center; z-index: 300; }
.modal { width: 400px; padding: 24px; border-radius: 18px; background: #261719; border: 1px solid var(--c-border-strong); h3 { font-size: 18px; margin-bottom: 12px; } .body { color: var(--c-text-2); font-size: 14px; line-height: 1.6; } .foot { display: flex; justify-content: flex-end; gap: 10px; margin-top: 22px; } }
.modal.wide { width: min(560px, calc(100vw - 32px)); }
</style>
