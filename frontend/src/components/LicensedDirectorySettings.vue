<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { FolderOpen } from 'lucide-vue-next'
import {
  chooseLicensedDirectory, clearLicensedDirectory, readLicensedConfiguration, type LicensedConfiguration,
} from '@/bindings'
import { BaseButton } from '@/components/ui'

const emit = defineEmits<{ pendingActivation: [pending: boolean] }>()
const config = ref<LicensedConfiguration>()
watch(config, (current) => emit('pendingActivation', Boolean(current?.restartRequired && current.savedDirectory)))
const error = ref('')
const busy = ref(false)

function report(cause: unknown) {
  error.value = typeof cause === 'string' ? cause : cause instanceof Error ? cause.message : '无法读取或保存片库目录。'
}

onMounted(async () => {
  try {
    config.value = await readLicensedConfiguration()
  } catch (cause) {
    report(cause)
  }
})

async function change(operation: () => Promise<LicensedConfiguration> | undefined) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const updated = await operation()
    if (updated) config.value = updated
  } catch (cause) {
    report(cause)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section v-if="config || error" class="configuration" aria-labelledby="licensed-directory-heading">
    <div class="top">
      <h2 id="licensed-directory-heading"><FolderOpen :size="20" />片库目录</h2>
      <div v-if="config?.canConfigure" class="actions">
        <BaseButton variant="outline" :disabled="busy" @click="change(chooseLicensedDirectory)">
          {{ config.savedDirectory || config.startupDirectory ? '更换目录' : '选择目录' }}
        </BaseButton>
        <BaseButton
          v-if="config.savedDirectory || config.readError" variant="ghost" :disabled="busy"
          @click="change(clearLicensedDirectory)"
        >移除配置</BaseButton>
      </div>
    </div>
    <p v-if="config?.activeDirectory">当前启用目录 <code>{{ config.activeDirectory }}</code></p>
    <p v-else-if="config?.startupDirectory">本次启动指定的目录 <code>{{ config.startupDirectory }}</code></p>
    <p v-if="config?.managed">此目录由运营方配置。</p>
    <p v-else-if="config && !config.canConfigure">本地存储不可用，暂时无法保存目录。</p>
    <p v-else-if="config?.restartRequired" role="status">
      <template v-if="config.savedDirectory">已保存 <code>{{ config.savedDirectory }}</code>，重启后启用。</template>
      <template v-else>已移除目录配置，重启后停用本地片库。媒体文件仍保存在原处。</template>
    </p>
    <p v-else-if="config && !config.activeDirectory">选择包含 catalog.json 和获授权视频的目录。</p>
    <p v-if="error || config?.readError" role="alert">{{ error || config?.readError }}</p>
  </section>
</template>

<style scoped lang="scss">
.configuration { padding: 16px; margin-bottom: 16px; border: 1px solid var(--c-border); border-radius: 14px; background: var(--c-surface); }
.top, h2, .actions { display: flex; align-items: center; gap: 10px; }
.top { justify-content: space-between; flex-wrap: wrap; }
h2 { font-size: 18px; margin: 0; }
h2 svg { color: var(--c-accent); }
.actions { flex-wrap: wrap; }
p { margin-top: 8px; color: var(--c-text-2); overflow-wrap: anywhere; }
code { margin-inline: 4px; color: var(--c-text); }
[role='alert'] { color: var(--c-accent-2); }
</style>
