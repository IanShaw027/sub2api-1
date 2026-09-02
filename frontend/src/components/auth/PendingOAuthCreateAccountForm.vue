<template>
 <form class="space-y-3" @submit.prevent="handleSubmit">
 <input
 v-model="email"
 :data-testid="`${testIdPrefix}-create-account-email`"
 type="email"
 class="input w-full"
 :placeholder="t('auth.emailPlaceholder')"
 :disabled="isSubmitting || isSendingCode || isAliyunRunning"
 />
 <input
 v-model="password"
 :data-testid="`${testIdPrefix}-create-account-password`"
 type="password"
 class="input w-full"
 :placeholder="t('auth.passwordPlaceholder')"
 :disabled="isSubmitting"
 />
 <div v-if="captchaEnabled" class="space-y-2">
 <TurnstileWidget
 ref="turnstileRef"
 :site-key="turnstileSiteKey"
 :turnstile-enabled="turnstileWidgetActive"
 :turnstile-site-key="turnstileSiteKey"
 :tencent-enabled="tencentWidgetActive"
 :tencent-app-id="tencentCaptchaAppId"
 :tencent-region="tencentCaptchaRegion"
 :aliyun-enabled="aliyunWidgetActive"
 :aliyun-scene-id="aliyunCaptchaSceneId"
 :aliyun-prefix="aliyunCaptchaPrefix"
 :aliyun-region="aliyunCaptchaRegion"
 @verify="onTurnstileVerify"
 @expire="onTurnstileExpire"
 @error="onTurnstileError"
 />
 </div>
 <div v-if="emailVerifyEnabled" class="flex gap-3">
 <input
 v-model="verifyCode"
 :data-testid="`${testIdPrefix}-create-account-verify-code`"
 type="text"
 inputmode="numeric"
 maxlength="6"
 class="input min-w-0 flex-1"
 placeholder="123456"
 :disabled="isSubmitting"
 />
 <button
 :data-testid="`${testIdPrefix}-create-account-send-code`"
 type="button"
 class="btn btn-secondary shrink-0"
 :disabled="isSubmitting || isSendingCode || isAliyunRunning || countdown > 0 || !email.trim() || (turnstileWidgetActive && !turnstileToken)"
 @click="handleSendCode"
 >
 {{
 isSendingCode
 ? t('auth.sendingCode')
 : countdown > 0
 ? t('auth.resendCountdown', { countdown })
 : t('auth.sendCode')
 }}
 </button>
 </div>
 <p v-if="emailVerifyEnabled && sendCodeSuccess" class="text-sm text-green-600">
 {{ t('auth.codeSentSuccess') }}
 </p>
 <p v-else-if="emailVerifyEnabled" class="text-xs text-muted">
 {{ t('auth.verificationCodeHint') }}
 </p>
 <input
 v-if="invitationCodeEnabled"
 v-model="invitationCode"
 :data-testid="`${testIdPrefix}-create-account-invitation-code`"
 type="text"
 class="input w-full"
 :placeholder="t('auth.invitationCodePlaceholder')"
 :disabled="isSubmitting"
 />
 <button
 :data-testid="`${testIdPrefix}-create-account-submit`"
 type="button"
 class="btn btn-primary w-full"
 :disabled="isSubmitting || isSendingCode || isAliyunRunning || !email.trim() || password.length < 6 || (invitationCodeEnabled && !invitationCode.trim()) || (turnstileWidgetActive && !turnstileToken)"
 @click="handleSubmit"
 >
 {{ isSubmitting ? t('common.processing') : t('auth.createAccount') }}
 </button>
 <button
 type="button"
 class="btn btn-secondary w-full"
 :disabled="isSubmitting"
 @click="emitSwitchToBind"
 >
 {{ t('auth.alreadyHaveAccount') }}
 </button>
 </form>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import TurnstileWidget from '@/components/CaptchaChallenge.vue'
import type { AliyunCaptchaBizResult } from '@/components/AliyunCaptchaWidget.vue'
import { getPublicSettings, sendPendingOAuthVerifyCode } from '@/api/auth'
import {
 extractApiErrorCode,
 extractApiErrorMetadata,
 isAliyunCaptchaVerificationError
} from '@/utils/apiError'
import {
 canonicalRegistrationEmail,
 isCanonicalRegistrationEmail
} from '@/utils/registrationEmailPolicy'
import { useAppStore } from '@/stores'

export type PendingOAuthCreateAccountPayload = {
 email: string
 password: string
 verifyCode: string
 turnstileToken?: string
 tencentCaptchaTicket?: string
 tencentCaptchaRandstr?: string
 invitationCode?: string
}

const props = defineProps<{
 initialEmail: string
 testIdPrefix: string
 isSubmitting: boolean
 errorMessage?: string
}>()

const emit = defineEmits<{
 submit: [payload: PendingOAuthCreateAccountPayload, settle?: (error?: unknown) => void]
 switchToBind: [email: string]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const email = ref('')
const password = ref('')
const verifyCode = ref('')
const invitationCode = ref('')
const isSendingCode = ref(false)
const isAliyunRunning = ref(false)
const sendCodeError = ref('')
const sendCodeSuccess = ref(false)
const countdown = ref(0)
const invitationCodeEnabled = ref(false)
const emailVerifyEnabled = ref(true)
const turnstileEnabled = ref(false)
const turnstileSiteKey = ref('')
const tencentCaptchaEnabled = ref(false)
const tencentCaptchaAppId = ref('')
const tencentCaptchaRegion = ref('cn')
const aliyunCaptchaEnabled = ref(false)
const aliyunCaptchaSceneId = ref('')
const aliyunCaptchaPrefix = ref('')
const aliyunCaptchaRegion = ref('cn')
const turnstileToken = ref('')
const tencentCaptchaRandstr = ref('')
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const aliyunCaptchaReady = computed(
 () =>
 aliyunCaptchaEnabled.value &&
 Boolean(aliyunCaptchaSceneId.value) &&
 Boolean(aliyunCaptchaPrefix.value)
)
// 与 CaptchaChallenge 展示优先级一致：Turnstile > 腾讯 > 阿里云
const turnstileWidgetActive = computed(
 () => turnstileEnabled.value && Boolean(turnstileSiteKey.value)
)
const tencentWidgetActive = computed(
 () =>
 !turnstileWidgetActive.value &&
 tencentCaptchaEnabled.value &&
 Boolean(tencentCaptchaAppId.value)
)
const aliyunWidgetActive = computed(
 () => !turnstileWidgetActive.value && !tencentWidgetActive.value && aliyunCaptchaReady.value
)
// 动作触发式验证码（腾讯/阿里云）：发送验证码、提交时弹窗验证
const actionCaptchaEnabled = computed(
 () => tencentWidgetActive.value || aliyunWidgetActive.value
)
const captchaEnabled = computed(
 () => turnstileWidgetActive.value || actionCaptchaEnabled.value
)

let countdownTimer: ReturnType<typeof setInterval> | null = null

watch(
 () => props.initialEmail,
 value => {
 email.value = value || ''
 },
 { immediate: true }
)

watch(sendCodeError, value => {
 if (value) {
 appStore.showError(value)
 }
})

watch(
 () => props.errorMessage,
 value => {
 if (value) {
 appStore.showError(value)
 if (captchaEnabled.value) {
 resetTurnstile()
 }
 }
 }
)

function clearCountdown() {
 if (countdownTimer) {
 clearInterval(countdownTimer)
 countdownTimer = null
 }
}

function startCountdown(seconds: number) {
 clearCountdown()
 countdown.value = Math.max(0, seconds)

 if (countdown.value <= 0) {
 return
 }

 countdownTimer = setInterval(() => {
 if (countdown.value <= 1) {
 countdown.value = 0
 clearCountdown()
 return
 }

 countdown.value -= 1
 }, 1000)
}

function getRequestErrorMessage(error: unknown, fallback: string): string {
 if (extractApiErrorCode(error) === 'EMAIL_ALIAS_NOT_ALLOWED') {
 const canonical = String(extractApiErrorMetadata(error)?.canonical_email || '')
 return t('auth.emailAliasNotAllowed', { canonical_email: canonical })
 }
 const err = error as { message?: string; response?: { data?: { detail?: string; message?: string } } }
 return err.response?.data?.detail || err.response?.data?.message || err.message || fallback
}

function resetTurnstile() {
 turnstileToken.value = ''
 tencentCaptchaRandstr.value = ''
 turnstileRef.value?.reset()
}

function onTurnstileVerify(token: string, randstr = '') {
 turnstileToken.value = token
 tencentCaptchaRandstr.value = randstr
 sendCodeError.value = ''
}

function onTurnstileExpire() {
 turnstileToken.value = ''
 tencentCaptchaRandstr.value = ''
 sendCodeError.value = t('auth.turnstileExpired')
}

function onTurnstileError() {
 turnstileToken.value = ''
 tencentCaptchaRandstr.value = ''
 sendCodeError.value = t('auth.turnstileFailed')
}

async function acquireActionProof(): Promise<boolean> {
 if (!tencentWidgetActive.value) return true

 const proof = await turnstileRef.value?.verifyAction()
 if (!proof) return false

 turnstileToken.value = proof.token
 tencentCaptchaRandstr.value = proof.randstr
 return true
}

async function sendCodeWithCaptcha(
 trimmedEmail: string,
 captchaParam?: string
): Promise<AliyunCaptchaBizResult> {
 isSendingCode.value = true
 sendCodeError.value = ''
 sendCodeSuccess.value = false

 try {
 const response = await sendPendingOAuthVerifyCode({
 email: trimmedEmail,
 turnstile_token:
 turnstileWidgetActive.value || aliyunWidgetActive.value ? captchaParam : undefined,
 tencent_captcha_ticket: tencentWidgetActive.value ? captchaParam : undefined,
 tencent_captcha_randstr: tencentWidgetActive.value ? tencentCaptchaRandstr.value : undefined
 })
 sendCodeSuccess.value = true
 startCountdown(response.countdown)
 return { captchaResult: true, bizResult: true }
 } catch (error: unknown) {
 sendCodeError.value = getRequestErrorMessage(error, t('auth.sendCodeFailed'))
 if (isAliyunCaptchaVerificationError(error)) {
 return { captchaResult: false, bizResult: false }
 }
 return { captchaResult: true, bizResult: false }
 } finally {
 if (captchaEnabled.value) {
 resetTurnstile()
 }
 isSendingCode.value = false
 }
}

async function handleSendCode() {
 if (props.isSubmitting || isSendingCode.value || isAliyunRunning.value) {
 return
 }
 const trimmedEmail = email.value.trim()
 if (!trimmedEmail) {
 return
 }
 if (!isCanonicalRegistrationEmail(trimmedEmail)) {
 sendCodeError.value = t('auth.emailAliasNotAllowed', {
 canonical_email: canonicalRegistrationEmail(trimmedEmail)
 })
 return
 }

 if (turnstileWidgetActive.value && !turnstileToken.value) {
 sendCodeError.value = t('auth.completeVerification')
 return
 }

 if (aliyunWidgetActive.value) {
 isAliyunRunning.value = true
 try {
 const result = await turnstileRef.value?.runAliyunVerify((param) =>
 sendCodeWithCaptcha(trimmedEmail, param)
 )
 if (!result?.captchaResult && !result?.bizResult) {
 sendCodeError.value = t('auth.completeVerification')
 }
 } finally {
 isAliyunRunning.value = false
 }
 return
 }

 if (!(await acquireActionProof())) {
 return
 }

 await sendCodeWithCaptcha(
 trimmedEmail,
 turnstileWidgetActive.value || tencentWidgetActive.value ? turnstileToken.value : undefined
 )
}

async function handleSubmit() {
 if (props.isSubmitting || isSendingCode.value || isAliyunRunning.value) {
 return
 }
 const trimmedEmail = email.value.trim()
 if (!trimmedEmail || password.value.length < 6) {
 return
 }
 if (!isCanonicalRegistrationEmail(trimmedEmail)) {
 sendCodeError.value = t('auth.emailAliasNotAllowed', {
 canonical_email: canonicalRegistrationEmail(trimmedEmail)
 })
 return
 }

 // Turnstile 票据一次性：发送验证码已消耗上一枚，reset 后要等新票据回调。
 // 缺票时不能提交——create-account 端点会校验验证码，空 token 直接被判失败。
 // 表单的隐式提交（输入框回车）绕得过按钮的 disabled，所以这里必须再挡一次。
 if (turnstileWidgetActive.value && !turnstileToken.value) {
 sendCodeError.value = t('auth.completeVerification')
 return
 }

 if (aliyunWidgetActive.value) {
 isAliyunRunning.value = true
 try {
 const result = await turnstileRef.value?.runAliyunVerify(async (param) => {
 try {
 await submitAccount({
 email: trimmedEmail,
 password: password.value,
 verifyCode: emailVerifyEnabled.value ? verifyCode.value.trim() : '',
 turnstileToken: param,
 invitationCode: invitationCode.value.trim() || undefined
 })
 return { captchaResult: true, bizResult: true }
 } catch (error: unknown) {
 if (isAliyunCaptchaVerificationError(error)) {
 sendCodeError.value = t('auth.completeVerification')
 return { captchaResult: false, bizResult: false }
 }
 return { captchaResult: true, bizResult: false }
 }
 })
 if (!result?.captchaResult) {
 sendCodeError.value = t('auth.completeVerification')
 }
 resetTurnstile()
 } finally {
 isAliyunRunning.value = false
 }
 return
 }

 if (!(await acquireActionProof())) {
 return
 }

 emit('submit', {
 email: trimmedEmail,
 password: password.value,
 verifyCode: emailVerifyEnabled.value ? verifyCode.value.trim() : '',
 ...(turnstileWidgetActive.value && turnstileToken.value
 ? { turnstileToken: turnstileToken.value }
 : {}),
 ...(tencentWidgetActive.value && turnstileToken.value
 ? {
 tencentCaptchaTicket: turnstileToken.value,
 tencentCaptchaRandstr: tencentCaptchaRandstr.value
 }
 : {}),
 invitationCode: invitationCode.value.trim() || undefined
 })

 if (actionCaptchaEnabled.value) {
 resetTurnstile()
 }
}

const SETTLE_WAIT_MS = 30000

function submitAccount(payload: PendingOAuthCreateAccountPayload): Promise<void> {
 return new Promise((resolve, reject) => {
 let settled = false
 const timer = setTimeout(() => {
 if (settled) return
 settled = true
 reject(new Error('create-account timed out'))
 }, SETTLE_WAIT_MS)

 emit('submit', payload, (error?: unknown) => {
 if (settled) return
 settled = true
 clearTimeout(timer)
 if (error !== undefined && error !== null) {
 reject(error)
 return
 }
 resolve()
 })
 })
}

function emitSwitchToBind() {
 emit('switchToBind', email.value.trim())
}

onMounted(async () => {
 try {
 const settings = await getPublicSettings()
 invitationCodeEnabled.value = settings.invitation_code_enabled === true
 emailVerifyEnabled.value = settings.email_verify_enabled !== false
 turnstileEnabled.value = settings.turnstile_enabled === true
 turnstileSiteKey.value = settings.turnstile_site_key || ''
 tencentCaptchaEnabled.value = settings.tencent_captcha_enabled === true
 tencentCaptchaAppId.value = settings.tencent_captcha_app_id || ''
 tencentCaptchaRegion.value = settings.tencent_captcha_region || 'cn'
 aliyunCaptchaEnabled.value = settings.aliyun_captcha_enabled === true
 aliyunCaptchaSceneId.value = settings.aliyun_captcha_scene_id || ''
 aliyunCaptchaPrefix.value = settings.aliyun_captcha_prefix || ''
 aliyunCaptchaRegion.value = settings.aliyun_captcha_region || 'cn'
 } catch {
 invitationCodeEnabled.value = false
 emailVerifyEnabled.value = true
 turnstileEnabled.value = false
 turnstileSiteKey.value = ''
 tencentCaptchaEnabled.value = false
 tencentCaptchaAppId.value = ''
 tencentCaptchaRegion.value = 'cn'
 aliyunCaptchaEnabled.value = false
 aliyunCaptchaSceneId.value = ''
 aliyunCaptchaPrefix.value = ''
 aliyunCaptchaRegion.value = 'cn'
 }
})

onUnmounted(() => {
 clearCountdown()
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
 transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
 opacity: 0;
 transform: translateY(-8px);
}
</style>
