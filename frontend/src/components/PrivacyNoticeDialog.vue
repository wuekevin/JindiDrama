<script setup lang="ts">
import { ref, watch } from 'vue'
import { Modal } from '@/components/ui'
import privacyNotice from '../../../docs/PRIVACY.md?raw'

const props = defineProps<{ open: boolean; returnFocus?: HTMLElement }>()
defineEmits<{ close: [] }>()

const content = ref<HTMLElement>()
const text = privacyNotice.replace(/^#.*\r?\n+/, '').trim()
watch(() => props.open, (open) => (open ? content.value : props.returnFocus)?.focus(), { flush: 'post' })
</script>

<template>
  <Modal :open="open" title="金帝短剧隐私说明" confirm-text="关闭" :show-cancel="false" wide @confirm="$emit('close')" @cancel="$emit('close')">
    <div ref="content" class="privacy-text" tabindex="0">{{ text }}</div>
  </Modal>
</template>

<style scoped>
.privacy-text { max-height: 52vh; overflow-y: auto; white-space: pre-wrap; }
</style>
