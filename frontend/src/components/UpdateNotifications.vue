<script setup lang="ts">
import { ref, watch } from 'vue'
import { Bell } from 'lucide-vue-next'
import { hasBridge } from '@/bindings'
import { useDesktopUpdates } from '@/stores/desktopUpdates'
import { BaseButton, Modal } from '@/components/ui'

const updates = useDesktopUpdates()
const desktop = hasBridge()
const open = ref(false)
const trigger = ref<HTMLButtonElement>()
const status = ref<HTMLElement>()
const releaseError = ref(false)

function show() {
  releaseError.value = false
  open.value = true
  if (desktop) void updates.loadChannel()
}

function focusClose() {
  status.value?.closest('[role=dialog]')?.querySelector<HTMLButtonElement>('[data-testid=modal-confirm]')?.focus()
}

watch(open, (visible) => {
  if (visible) focusClose()
  else trigger.value?.focus()
}, { flush: 'post' })
watch([open, () => updates.noticeKey, () => updates.channelBusy], ([visible]) => {
  if (visible) updates.acknowledge()
})

function check() {
  releaseError.value = false
  focusClose()
  void updates.check(false)
}

async function viewRelease() {
  releaseError.value = !(await updates.openRelease())
}
</script>

<template>
  <button ref="trigger" class="bell" aria-label="通知" aria-haspopup="dialog" :aria-expanded="open"
    :aria-describedby="updates.unread ? 'notification-unread-label' : undefined" @click="show">
    <Bell :size="22" />
    <span v-if="updates.unread" class="dot" data-testid="notification-unread" aria-hidden="true" />
    <span v-if="updates.unread" class="sr-only" id="notification-unread-label">有未读更新通知</span>
  </button>
  <Modal :open="open" title="更新通知" confirm-text="关闭" :show-cancel="false" @confirm="open = false" @cancel="open = false">
    <p v-if="desktop && updates.channelAvailable && !updates.channelBusy" class="channel">更新渠道 · {{ updates.channel === 'preview' ? '预发布' : '稳定版' }}</p>
    <p ref="status" role="status">{{ !desktop ? '更新通知在桌面应用中可用。' : updates.channelBusy ? '正在处理更新渠道…' : updates.checking ? '正在检查更新…' : updates.checkMessage || '暂无更新通知' }}</p>
    <div v-if="desktop" class="actions">
      <BaseButton variant="outline" :disabled="updates.checking || updates.channelBusy || updates.downloading" @click="check">检查更新</BaseButton>
      <BaseButton v-if="updates.upd && !updates.channelBusy" @click="viewRelease">{{ updates.upd.preview ? '下载预发布版' : '前往下载' }} {{ updates.upd.latest }}</BaseButton>
    </div>
    <p v-if="releaseError" role="alert">无法打开版本下载页面</p>
  </Modal>
</template>

<style scoped lang="scss">
.bell { position: relative; color: var(--c-text); display: grid; place-items: center; }
.dot { position: absolute; top: -2px; right: -2px; width: 8px; height: 8px; border-radius: 50%; background: #ff3b3b; }
.channel { margin-bottom: 4px; color: var(--c-text-2); font-size: 13px; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
</style>
