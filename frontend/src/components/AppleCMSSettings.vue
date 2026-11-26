<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { BaseButton } from '@/components/ui'
import {
  clearAppleCMSURL, readAppleCMSConfiguration, saveAppleCMSURL, testAppleCMSConnection,
  type AppleCMSConfiguration,
} from '@/bindings'

const config = ref<AppleCMSConfiguration>()
const address = ref('')
const pending = ref<'load' | 'test' | 'save' | 'clear'>()
const notice = ref('')
const error = ref('')
const addressError = ref(false)

const invalidAddress = '请输入无需凭据或查询参数的公网 HTTPS 地址'
const connectionError = '无法连接片源或读取苹果 CMS 目录'

function accept(updated: AppleCMSConfiguration) {
  config.value = updated
  address.value = updated.savedURL
}

async function load() {
  pending.value = 'load'
  error.value = ''
  try {
    const current = await readAppleCMSConfiguration()
    if (!current) throw new Error()
    accept(current)
  } catch {
    error.value = '无法读取苹果 CMS 片源设置。'
  } finally {
    pending.value = undefined
  }
}

onMounted(load)

function resetFeedback() {
  notice.value = ''
  error.value = ''
  addressError.value = false
}

async function change(operation: 'test' | 'save' | 'clear') {
  if (pending.value || !config.value?.canConfigure) return
  if (operation === 'clear' && !config.value.savedURL && !config.value.readError) return
  pending.value = operation
  resetFeedback()
  try {
    if (operation === 'test') {
      await testAppleCMSConnection(address.value)
      notice.value = '连接成功，已读取片源目录。'
      return
    }
    const updated = await (operation === 'save' ? saveAppleCMSURL(address.value) : clearAppleCMSURL())
    if (!updated) throw new Error()
    accept(updated)
    if (!updated.restartRequired) notice.value = operation === 'save' ? '片源已保存。' : '片源已清除。'
  } catch (cause) {
    const message = typeof cause === 'string' ? cause : cause instanceof Error ? cause.message : ''
    addressError.value = message === invalidAddress
    if (addressError.value || message === connectionError) error.value = message + '。'
    else if (operation === 'test') error.value = '无法测试片源连接。'
    else if (operation === 'save') error.value = '无法保存苹果 CMS 片源设置。'
    else error.value = '无法清除苹果 CMS 片源设置。'
  } finally {
    pending.value = undefined
  }
}
</script>

<template>
  <section class="configuration" aria-labelledby="apple-cms-heading">
    <h3 id="apple-cms-heading">苹果 CMS 片源</h3>
    <p v-if="config">
      {{ config.active ? '当前已启用苹果 CMS 片源。' : '当前使用演示目录。' }}
    </p>
    <p v-if="config?.managed">片源由运营方配置。</p>
    <form v-if="config?.canConfigure" novalidate @submit.prevent="change('test')">
      <label for="apple-cms-address">片源地址</label>
      <input
        id="apple-cms-address" v-model="address" type="url" inputmode="url" autocomplete="off"
        spellcheck="false" maxlength="4096" :readonly="Boolean(pending)" :aria-invalid="addressError"
        :aria-describedby="addressError ? 'apple-cms-help apple-cms-error' : 'apple-cms-help'"
        @input="resetFeedback"
      />
      <p id="apple-cms-help">请输入自有或获授权的公网 HTTPS 地址，无需凭据或查询参数。</p>
      <div class="actions">
        <BaseButton type="submit" variant="outline" :aria-disabled="Boolean(pending)">测试连接</BaseButton>
        <BaseButton type="button" :aria-disabled="Boolean(pending)" @click="change('save')">保存片源</BaseButton>
        <BaseButton
          type="button" variant="ghost" :aria-disabled="Boolean(pending) || (!config.savedURL && !config.readError)"
          @click="change('clear')"
        >清除片源</BaseButton>
      </div>
      <p v-if="config.savedURL && address !== config.savedURL" class="saved-address">已保存 <code>{{ config.savedURL }}</code></p>
    </form>
    <p v-if="config?.restartRequired" role="status">
      {{ config.savedURL ? '片源已保存，完全退出并重新打开应用后启用。' : '片源已清除，完全退出并重新打开应用后恢复演示目录。' }}
    </p>
    <p v-if="notice || pending" role="status">
      {{ pending === 'load' ? '正在读取片源设置…' : pending === 'test' ? '正在测试连接…' : pending === 'save' ? '正在验证并保存片源…' : pending === 'clear' ? '正在清除片源…' : notice }}
    </p>
    <p v-if="config?.readError" role="alert">{{ config.readError }}。</p>
    <p v-if="error" id="apple-cms-error" role="alert">{{ error }}</p>
    <BaseButton v-if="!config && error" type="button" variant="outline" @click="load">重试读取</BaseButton>
  </section>
</template>

<style scoped lang="scss">
.configuration {
  border-top: 1px solid var(--c-border);
  margin-top: 10px;
  padding-top: 16px;
  min-width: 0;
}
h3 { margin: 0; font-size: 17px; }
p { margin: 8px 0 0; color: var(--c-text-2); overflow-wrap: anywhere; }
form { display: grid; gap: 8px; margin-top: 12px; }
form p { margin: 0; }
input {
  min-width: 0;
  width: 100%;
  min-height: 38px;
  padding: 6px 12px;
  border: 1px solid var(--c-border-strong);
  border-radius: 8px;
  background: var(--c-surface);
  color: var(--c-text);
}
input:focus-visible, button:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 2px; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 4px; }
button[aria-disabled='true'] { opacity: .45; cursor: not-allowed; }
[role='alert'] { color: var(--c-accent-2); }
</style>
