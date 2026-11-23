<script setup lang="ts">
import BaseButton from './BaseButton.vue'
defineProps<{ open: boolean; title: string; confirmText?: string; danger?: boolean }>()
defineEmits<{ confirm: []; cancel: [] }>()
</script>
<template>
  <Teleport to="body">
    <div v-if="open" class="mask" @click.self="$emit('cancel')" @keydown.esc="$emit('cancel')">
      <div class="modal" role="dialog" aria-modal="true" :aria-label="title" data-testid="modal">
        <h3>{{ title }}</h3><div class="body"><slot /></div>
        <div class="foot"><BaseButton variant="ghost" @click="$emit('cancel')">取消</BaseButton><BaseButton data-testid="modal-confirm" @click="$emit('confirm')">{{ confirmText ?? '确定' }}</BaseButton></div>
      </div>
    </div>
  </Teleport>
</template>
<style scoped lang="scss">
.mask { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: grid; place-items: center; z-index: 300; }
.modal { width: 400px; padding: 24px; border-radius: 18px; background: #261719; border: 1px solid var(--c-border-strong); h3 { font-size: 18px; margin-bottom: 12px; } .body { color: var(--c-text-2); font-size: 14px; line-height: 1.6; } .foot { display: flex; justify-content: flex-end; gap: 10px; margin-top: 22px; } }
</style>
