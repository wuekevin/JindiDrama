<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Smartphone, ShieldCheck, Cloud, ScanLine, RefreshCw, LogOut, CircleCheck, TriangleAlert } from 'lucide-vue-next'
import { useAuthStore, DEMO_CODE, isValidCode, isValidPhone } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { BaseButton } from '@/components/ui'
import BrandLogo from '@/components/BrandLogo.vue'
import PrivacyNoticeDialog from '@/components/PrivacyNoticeDialog.vue'

const route = useRoute(); const router = useRouter(); const auth = useAuthStore(); const toast = useToastStore()
const tab = ref<'phone' | 'qr'>(route.query.tab === 'qr' ? 'qr' : 'phone')
const phone = ref(''); const code = ref(''); const agree = ref(false)
const errs = ref<{ phone?: string; code?: string; agree?: string; form?: string }>({})
const showPrivacy = ref(false)
const privacyButton = ref<HTMLButtonElement>()
const status = ref<'idle' | 'submitting' | 'locked'>('idle'); const attempts = ref(5)
let phoneAttempt = 0
const COUNTDOWN = 60; const cd = ref(0); let cdTimer: number | undefined
const expired = computed(() => route.query.state === 'expired')
function startCd() { clearInterval(cdTimer); cd.value = COUNTDOWN; cdTimer = window.setInterval(() => { cd.value--; if (cd.value <= 0) clearInterval(cdTimer) }, 1000) }
function getCode() {
  errs.value = { ...errs.value, phone: undefined, form: undefined }
  if (!isValidPhone(phone.value)) { errs.value.phone = '请输入正确的 11 位手机号'; return }
  if (cd.value > 0) return
  startCd(); toast.push(`演示模式：验证码为 ${DEMO_CODE}（不会发送真实短信）`, 'info', 4000)
}
function requireAgreement() {
  if (agree.value) return true
  errs.value.agree = '请先阅读隐私说明并确认本地演示'
  return false
}
async function submit() {
  if (status.value === 'submitting') return
  errs.value = {}
  if (!isValidPhone(phone.value)) errs.value.phone = '请输入正确的 11 位手机号'
  if (!isValidCode(code.value)) errs.value.code = '请输入 6 位数字验证码'
  requireAgreement()
  if (Object.keys(errs.value).length) return
  if (status.value === 'locked') { errs.value.form = '尝试次数过多，请稍后再试'; return }
  const attempt = ++phoneAttempt
  const submittedPhone = phone.value
  const submittedCode = code.value
  status.value = 'submitting'
  await new Promise((resolve) => setTimeout(resolve, 450))
  if (attempt !== phoneAttempt) return
  status.value = 'idle'
  if (!requireAgreement()) return
  if (submittedCode !== DEMO_CODE) { attempts.value--; status.value = attempts.value <= 0 ? 'locked' : 'idle'; errs.value.form = attempts.value <= 0 ? '尝试次数过多，请稍后再试' : `验证码错误，还可尝试 ${attempts.value} 次`; return }
  auth.loginDemo(submittedPhone); toast.push('已进入演示会话（非官方账号）', 'success')
}
function cancelPhoneAttempt() {
  if (status.value === 'submitting') {
    phoneAttempt++
    status.value = 'idle'
  }
}
function openPrivacy() {
  cancelPhoneAttempt()
  showPrivacy.value = true
}
// 扫码状态机：waiting → scanned → confirmed；超时 expired
const qr = ref<'waiting' | 'scanned' | 'confirmed' | 'expired'>('waiting'); const qrLeft = ref(120); let qrTimer: number | undefined; const token = ref(1)
function startQr() { clearInterval(qrTimer); qr.value = 'waiting'; qrLeft.value = 120; qrTimer = window.setInterval(() => { if (qr.value !== 'waiting' && qr.value !== 'scanned') return; qrLeft.value--; if (qrLeft.value <= 0) { qr.value = 'expired'; clearInterval(qrTimer) } }, 1000) }
function refreshQr() { token.value++; startQr() }
function simScan() { if (qr.value === 'waiting') qr.value = 'scanned' }
function simConfirm() { if (qr.value === 'scanned' && requireAgreement()) { qr.value = 'confirmed'; clearInterval(qrTimer); auth.loginDemo(); toast.push('已进入演示会话（非官方账号）', 'success') } }
function simExpire() { qrLeft.value = 0; qr.value = 'expired' }
watch(tab, (t) => { cancelPhoneAttempt(); if (t === 'qr') startQr(); else clearInterval(qrTimer) }, { immediate: true })
onBeforeUnmount(() => { cancelPhoneAttempt(); clearInterval(cdTimer); clearInterval(qrTimer) })
const cells = computed(() => { let s = token.value * 2654435761 >>> 0; const g: boolean[] = []; for (let i = 0; i < 21 * 21; i++) { s = (s * 1664525 + 1013904223) >>> 0; g.push((s >>> 24) % 2 === 0) } return g })
const inFinder = (i: number) => { const x = i % 21, y = Math.floor(i / 21); return (x < 7 && y < 7) || (x > 13 && y < 7) || (x < 7 && y > 13) }
const finder = (i: number) => { const x = i % 21, y = Math.floor(i / 21); const fx = x > 13 ? x - 14 : x; const fy = y > 13 ? y - 14 : y; return fx === 0 || fx === 6 || fy === 0 || fy === 6 || (fx >= 2 && fx <= 4 && fy >= 2 && fy <= 4) }
function logout() { auth.logout(); phone.value = ''; code.value = ''; agree.value = false; if (tab.value === 'qr') refreshQr(); toast.push('已退出演示会话') }
const mm = (n: number) => `${Math.floor(n / 60)}:${String(n % 60).padStart(2, '0')}`
</script>
<template>
  <div class="login" data-testid="page-login">
    <section class="card">
      <div class="reel" aria-hidden="true" />
      <header><BrandLogo /><div class="demo" data-testid="demo-badge"><TriangleAlert :size="14" />演示模式 · 未接入官方账号</div></header>
      <h1>账号登录</h1><p class="sub">一个账号 · 畅享海量精品短剧</p>
      <div v-if="expired && !auth.demo" class="alert" role="alert" data-testid="expired"><TriangleAlert :size="16" />登录已过期，请重新登录</div>

      <div v-if="auth.demo" class="done" data-testid="logged-in"><CircleCheck :size="44" color="#46c28a" /><h2>{{ auth.name }}</h2><p>当前为本地演示会话，不会访问红果官方账号，也不会同步云端数据。</p>
        <div class="dbtn"><BaseButton data-testid="go-home" @click="router.push('/home')">开始看剧</BaseButton><BaseButton variant="ghost" data-testid="logout" @click="logout"><LogOut :size="16" />退出登录</BaseButton></div></div>
      <template v-else>
        <div class="tabs" role="tablist"><button role="tab" :aria-selected="tab === 'phone'" :class="{ on: tab === 'phone' }" data-testid="tab-phone" @click="tab = 'phone'">手机号登录</button><i /><button role="tab" :aria-selected="tab === 'qr'" :class="{ on: tab === 'qr' }" data-testid="tab-qr" @click="tab = 'qr'">扫码登录</button></div>
        <form v-if="tab === 'phone'" novalidate @submit.prevent="submit">
          <label class="f" :class="{ bad: errs.phone }"><Smartphone :size="20" /><input v-model="phone" inputmode="numeric" maxlength="11" placeholder="请输入手机号" aria-label="手机号" data-testid="phone" /></label><p v-if="errs.phone" class="e" role="alert" data-testid="err-phone">{{ errs.phone }}</p>
          <div class="row"><label class="f" :class="{ bad: errs.code }"><ShieldCheck :size="20" /><input v-model="code" inputmode="numeric" maxlength="6" placeholder="请输入验证码" aria-label="验证码" data-testid="code" /></label>
            <button type="button" class="get" :disabled="cd > 0" data-testid="get-code" @click="getCode">{{ cd > 0 ? `重新获取(${cd}s)` : '获取验证码' }}</button></div><p v-if="errs.code" class="e" role="alert" data-testid="err-code">{{ errs.code }}</p>
          <p v-if="errs.form" class="e form" role="alert" data-testid="err-form">{{ errs.form }}</p>
          <button class="go" type="submit" :disabled="status === 'submitting' || status === 'locked'" data-testid="submit">{{ status === 'submitting' ? '登录中…' : '登录' }}</button>
          <div class="or"><span>或使用扫码登录</span></div>
          <button type="button" class="qrmini" aria-label="切换到扫码登录" @click="tab = 'qr'"><ScanLine :size="48" :stroke-width="1.2" /></button><p class="qt"><b>扫码登录</b></p><p class="qs">演示模式下不连接红果 App，点此体验扫码流程</p>
        </form>
        <div v-else class="qrbox" data-testid="qr-panel">
          <div class="qr" :class="qr"><svg viewBox="0 0 21 21" shape-rendering="crispEdges"><template v-for="(c, i) in cells" :key="i"><rect v-if="inFinder(i) ? finder(i) : c" :x="i % 21" :y="Math.floor(i / 21)" width="1" height="1" fill="#f6ece9" /></template></svg>
            <div v-if="qr !== 'waiting'" class="ov" data-testid="qr-overlay"><template v-if="qr === 'scanned'"><CircleCheck :size="34" color="#46c28a" /><span>已扫码，请在手机端确认</span></template><template v-else-if="qr === 'expired'"><span>二维码已过期</span><BaseButton size="sm" data-testid="qr-refresh" @click="refreshQr"><RefreshCw :size="14" />刷新</BaseButton></template></div></div>
          <p class="qt"><b>扫码登录（演示）</b></p><p class="qs">二维码仅为演示图案，{{ qr === 'expired' ? '已失效' : `有效期 ${mm(qrLeft)}` }}</p>
          <div class="sim"><button data-testid="sim-scan" :disabled="qr !== 'waiting'" @click="simScan">模拟扫码</button><button data-testid="sim-confirm" :disabled="qr !== 'scanned'" @click="simConfirm">模拟确认登录</button><button data-testid="sim-expire" :disabled="qr === 'expired'" @click="simExpire">模拟过期</button></div></div>
        <div class="agree" :class="{ bad: errs.agree }">
          <input id="demo-agreement" v-model="agree" type="checkbox" data-testid="agree" @change="errs.agree = undefined" />
          <label for="demo-agreement">我已阅读说明，了解这是本地演示</label>
          <button ref="privacyButton" type="button" @click="openPrivacy">《隐私说明》</button>
        </div>
        <p v-if="errs.agree" class="e" role="alert" data-testid="err-agree">{{ errs.agree }}</p>
      </template>
      <div class="note"><Cloud :size="22" /><span>演示模式：收藏 / 历史仅保存在本机，未与红果云端同步；官方账号登录需取得授权后开放。</span></div>
    </section>
    <PrivacyNoticeDialog :open="showPrivacy" :return-focus="privacyButton" @close="showPrivacy = false" />
  </div>
</template>
<style scoped lang="scss">
.login { min-height: 100%; display: flex; justify-content: center; padding: 62px 0 24px; background: radial-gradient(500px 300px at 85% 85%, rgba(238,75,57,.18), transparent 70%); }
.card { position: relative; width: 670px; padding: 44px 78px 26px; border-radius: 30px; background: linear-gradient(180deg, rgba(255,255,255,.08), rgba(255,255,255,.035)); border: 1px solid rgba(255,140,110,.28); box-shadow: 0 0 40px rgba(238,75,57,.12); overflow: hidden; align-self: flex-start; header { display: flex; justify-content: space-between; align-items: center; } h1 { margin-top: 22px; font-size: 28px; text-align: center; } .sub { text-align: center; color: var(--c-text-2); margin-top: 6px; font-size: 14px; } }
.reel { position: absolute; right: -20px; top: -6px; width: 150px; height: 70px; transform: rotate(32deg); background: repeating-linear-gradient(90deg, rgba(255,255,255,.12) 0 14px, transparent 14px 20px); opacity: .5; border-radius: 6px; }
.demo { display: inline-flex; align-items: center; gap: 6px; padding: 4px 12px; border-radius: 14px; background: rgba(244,183,64,.16); border: 1px solid rgba(244,183,64,.5); color: var(--c-gold); font-size: 12.5px; position: relative; z-index: 1; }
.alert { display: flex; align-items: center; gap: 8px; margin-top: 16px; padding: 10px 14px; border-radius: 10px; background: rgba(255,91,91,.14); border: 1px solid rgba(255,91,91,.5); font-size: 14px; }
.tabs { display: flex; align-items: center; margin: 24px 0 20px; i { width: 1px; height: 22px; background: var(--c-border-strong); } button { flex: 1; font-size: 20px; font-weight: 600; color: var(--c-text-2); padding-bottom: 10px; position: relative; &.on { color: var(--c-text); &::after { content: ''; position: absolute; left: 50%; bottom: 0; width: 64px; height: 3px; border-radius: 3px; transform: translateX(-50%); background: var(--c-accent); } } } }
.f { flex: 1; display: flex; align-items: center; gap: 14px; height: 58px; padding: 0 20px; border-radius: 14px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border); color: var(--c-text-2); margin-top: 12px; &:focus-within { border-color: var(--c-accent); } &.bad { border-color: var(--c-danger); } input { flex: 1; min-width: 0; background: none; border: 0; outline: 0; font-size: 16px; color: var(--c-text); } }
.row { display: flex; gap: 18px; } .get { flex: none; width: 146px; height: 58px; margin-top: 12px; border-radius: 14px; border: 1px solid var(--c-accent); color: var(--c-accent-2); font-size: 16px; &:disabled { opacity: .55; cursor: not-allowed; } }
.go { width: 100%; height: 58px; margin-top: 20px; border-radius: 29px; background: linear-gradient(90deg, #f46a4e, #e4402f); font-size: 20px; font-weight: 700; box-shadow: 0 8px 22px rgba(220,58,42,.35); &:disabled { opacity: .6; } }
.e { margin-top: 6px; padding-left: 6px; font-size: 13px; color: var(--c-danger); &.form { text-align: center; margin-top: 12px; } }
.or { display: flex; align-items: center; gap: 14px; margin: 18px 0 10px; color: var(--c-text-2); font-size: 14px; &::before, &::after { content: ''; flex: 1; height: 1px; background: var(--c-border); } }
.qrmini { display: block; margin: 0 auto; width: 110px; height: 110px; border-radius: 18px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border-strong); display: grid; place-items: center; }
.qt { text-align: center; margin-top: 10px; b { font-size: 16px; } } .qs { text-align: center; font-size: 13.5px; color: var(--c-text-2); margin-top: 4px; }
.qrbox { padding-top: 6px; } .qr { position: relative; width: 190px; height: 190px; margin: 0 auto; padding: 12px; border-radius: 16px; background: rgba(255,255,255,.07); border: 1px solid var(--c-border-strong); svg { width: 100%; height: 100%; } &.expired svg { opacity: .15; } .ov { position: absolute; inset: 0; border-radius: 16px; background: rgba(15,8,9,.78); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; font-size: 14px; } }
.sim { display: flex; justify-content: center; gap: 10px; margin-top: 14px; button { height: 34px; padding: 0 14px; border-radius: 17px; border: 1px dashed var(--c-border-strong); font-size: 13px; &:disabled { opacity: .35; } &:hover:not(:disabled) { background: var(--c-surface-hover); } } }
.agree { display: flex; align-items: center; justify-content: center; flex-wrap: wrap; gap: 8px; margin-top: 22px; font-size: 14px; color: var(--c-text-2); &.bad { color: var(--c-danger); } input { width: 18px; height: 18px; accent-color: #ee4b39; } button { color: var(--c-accent-2); } }
.agree + .e { text-align: center; }
.note { display: flex; align-items: center; gap: 14px; margin-top: 22px; padding: 14px 20px; border-radius: 14px; background: rgba(244,183,64,.08); border: 1px solid rgba(244,183,64,.2); font-size: 13.5px; line-height: 1.5; color: var(--c-text-2); svg { color: var(--c-gold); flex: none; } }
.done { text-align: center; padding: 30px 0 4px; h2 { margin-top: 12px; font-size: 24px; } p { margin-top: 10px; font-size: 14px; color: var(--c-text-2); line-height: 1.6; } .dbtn { display: flex; justify-content: center; gap: 14px; margin-top: 22px; } }
</style>
