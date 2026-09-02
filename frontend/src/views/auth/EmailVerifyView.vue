<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="login-title">
        <h2>{{ t('auth.verifyYourEmail') }}</h2>
        <p>
          {{ t('auth.sendCodeDesc') }}
          <span class="font-medium text-foreground">{{ email }}</span>
        </p>
      </div>

      <div
        v-if="!hasRegisterData"
        class="rounded-xl border border-[color-mix(in_oklch,var(--warning)_35%,transparent)] bg-[color-mix(in_oklch,var(--warning)_12%,transparent)] p-4"
      >
        <div class="flex items-start gap-3">
          <div class="flex-shrink-0">
            <Icon name="exclamationCircle" size="md" class="text-[var(--warning-text)]" />
          </div>
          <div class="text-sm text-[var(--warning-text)]">
            <p class="font-medium">{{ t('auth.sessionExpired') }}</p>
            <p class="mt-1">{{ t('auth.sessionExpiredDesc') }}</p>
          </div>
        </div>
      </div>

      <form v-else @submit.prevent="handleVerify" class="login-form">
        <div v-if="requiresPasswordReentry">
          <label for="registration-password" class="login-label">
            {{ t('auth.passwordLabel') }}
          </label>
          <input
            id="registration-password"
            v-model="password"
            type="password"
            required
            autocomplete="new-password"
            class="field"
            :placeholder="t('auth.createPasswordPlaceholder')"
          />
          <p class="mt-1 text-xs text-muted">{{ t('auth.passwordHint') }}</p>
        </div>

        <div>
          <label for="code" class="login-label text-center">
            {{ t('auth.verificationCode') }}
          </label>
          <input
            id="code"
            v-model="verifyCode"
            type="text"
            required
            autocomplete="one-time-code"
            inputmode="numeric"
            maxlength="6"
            :disabled="isLoading"
            class="field py-3 text-center font-mono text-xl tracking-[0.5em]"
            :class="{ 'input-error': errors.code }"
            placeholder="000000"
          />
          <p class="mt-1 text-center text-xs text-muted">{{ t('auth.verificationCodeHint') }}</p>
        </div>

        <div
          v-if="codeSent"
          class="rounded-xl border border-[color-mix(in_oklch,var(--success)_35%,transparent)] bg-[color-mix(in_oklch,var(--success)_12%,transparent)] p-4"
        >
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
              <Icon name="checkCircle" size="md" class="text-[var(--success-text)]" />
            </div>
            <p class="text-sm text-[var(--success-text)]">
              {{ t('auth.codeSentSuccess') }}
            </p>
          </div>
        </div>

        <div v-if="actionCaptchaEnabled || (turnstileWidgetActive && showResendTurnstile)">
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

        <div v-if="pendingOAuthCreateCaptchaEnabled" class="space-y-2">
          <TurnstileWidget
            ref="createAccountTurnstileRef"
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
            @verify="onCreateAccountTurnstileVerify"
            @expire="onCreateAccountTurnstileExpire"
            @error="onCreateAccountTurnstileError"
          />
        </div>

        <Button
          native-type="submit"
          size="md"
          class="w-full"
          :disabled="isLoading || !verifyCode || (pendingOAuthCreateTurnstileRequired && !createAccountTurnstileToken)"
          :loading="isLoading"
        >
          <Icon v-if="!isLoading" name="checkCircle" size="md" />
          {{ isLoading ? t('auth.verifying') : t('auth.verifyAndCreate') }}
        </Button>

        <div class="text-center">
          <button
            v-if="countdown > 0"
            type="button"
            disabled
            class="cursor-not-allowed text-sm text-muted"
          >
            {{ t('auth.resendCountdown', { countdown }) }}
          </button>
          <button
            v-else
            type="button"
            @click="handleResendCode"
            :disabled="
              isSendingCode ||
              (turnstileWidgetActive && showResendTurnstile && !resendTurnstileToken)
            "
            class="text-sm text-accent disabled:cursor-not-allowed disabled:opacity-50"
          >
            <span v-if="isSendingCode">{{ t('auth.sendingCode') }}</span>
            <span v-else-if="captchaEnabled && !showResendTurnstile">
              {{ t('auth.clickToResend') }}
            </span>
            <span v-else>{{ t('auth.resendCode') }}</span>
          </button>
        </div>
      </form>
    </div>

    <template #footer>
      <button
        @click="handleBack"
        class="flex items-center gap-2 text-muted transition-colors hover:text-foreground"
      >
        <Icon name="arrowLeft" size="sm" />
        {{ t('auth.backToRegistration') }}
      </button>
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import TurnstileWidget from '@/components/CaptchaChallenge.vue'
import type { AliyunCaptchaBizResult } from '@/components/AliyunCaptchaWidget.vue'
import { useAuthStore, useAppStore } from '@/stores'
import {
  clearPendingRegistrationPassword,
  getPendingRegistrationPassword,
  persistOAuthTokenContext,
  getPublicSettings,
  isOAuthLoginCompletion,
  type PendingOAuthSendVerifyCodeResponse,
  sendPendingOAuthVerifyCode,
  sendVerifyCode,
  setPendingRegistrationPassword,
} from '@/api/auth'
import { apiClient } from '@/api/client'
import { buildAuthErrorMessage } from '@/utils/authError'
import {
  extractApiErrorCode,
  extractApiErrorMetadata,
  isAliyunCaptchaVerificationError
} from '@/utils/apiError'
import {
  canonicalRegistrationEmail,
  formatRegistrationEmailSuffixWhitelistForMessage,
  isCanonicalRegistrationEmail,
  isRegistrationEmailSuffixAllowed,
  normalizeRegistrationEmailSuffixWhitelist
} from '@/utils/registrationEmailPolicy'
import {
  clearAllAffiliateReferralCodes,
  loadAffiliateReferralCode,
  oauthAffiliatePayload
} from '@/utils/oauthAffiliate'
import { sanitizeAuthRedirect } from '@/utils/authRedirect'

const { t, locale } = useI18n()

// ==================== Router & Stores ====================

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// ==================== State ====================

const isLoading = ref<boolean>(false)
const isSendingCode = ref<boolean>(false)
const errorMessage = ref<string>('')
const codeSent = ref<boolean>(false)
const verifyCode = ref<string>('')
const countdown = ref<number>(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

// Registration data from sessionStorage
type PendingAuthTokenField = 'pending_auth_token' | 'pending_oauth_token'
type PendingAuthSessionSummary = {
  token: string
  token_field: PendingAuthTokenField
  provider: string
  redirect?: string
}
type PendingOAuthCreateAccountResponse = {
  auth_result?: string
  access_token: string
  refresh_token?: string
  expires_in?: number
  token_type?: string
  provider?: string
  redirect?: string
}

const email = ref<string>('')
const password = ref<string>('')
const initialTurnstileToken = ref<string>('')
const initialTencentCaptchaRandstr = ref<string>('')
const promoCode = ref<string>('')
const invitationCode = ref<string>('')
const affCode = ref<string>('')
const pendingAuthToken = ref<string>('')
const pendingAuthTokenField = ref<PendingAuthTokenField>('pending_auth_token')
const pendingProvider = ref<string>('')
const pendingRedirect = ref<string>('')
const pendingAdoptionDecision = ref<{
  adoptDisplayName?: boolean
  adoptAvatar?: boolean
} | null>(null)
const hasRegisterData = ref<boolean>(false)
const requiresPasswordReentry = ref<boolean>(false)
const registerDataCodeSent = ref<boolean>(false)
const registerDataCountdown = ref<number>(0)

// Public settings
const turnstileEnabled = ref<boolean>(false)
const turnstileSiteKey = ref<string>('')
const tencentCaptchaEnabled = ref<boolean>(false)
const tencentCaptchaAppId = ref<string>('')
const tencentCaptchaRegion = ref<string>('cn')
const aliyunCaptchaEnabled = ref<boolean>(false)
const aliyunCaptchaSceneId = ref<string>('')
const aliyunCaptchaPrefix = ref<string>('')
const aliyunCaptchaRegion = ref<string>('cn')
const siteName = ref<string>('Sub2API')
const registrationEmailSuffixWhitelist = ref<string[]>([])
// 域名限量注册开关：开启时非白名单域名可注册 1 个账户（由后端判定），前端不做白名单预检。
const emailDomainQuotaEnabled = ref<boolean>(false)

// Turnstile for resend
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const createAccountTurnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const resendTurnstileToken = ref<string>('')
const resendTencentCaptchaRandstr = ref<string>('')
const createAccountTurnstileToken = ref<string>('')
const createAccountTencentCaptchaRandstr = ref<string>('')
const showResendTurnstile = ref<boolean>(false)
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
// 动作触发式验证码（腾讯/阿里云）：重发验证码、创建账号时弹窗验证
const actionCaptchaEnabled = computed(
  () => tencentWidgetActive.value || aliyunWidgetActive.value
)
const captchaEnabled = computed(
  () => turnstileWidgetActive.value || actionCaptchaEnabled.value
)

const errors = ref({
  code: '',
  turnstile: ''
})

const validationToastMessage = computed(
  () => errors.value.code || errors.value.turnstile || ''
)
const pendingOAuthCreateTurnstileRequired = computed(
  () => isPendingOAuthFlow() && turnstileWidgetActive.value
)
const pendingOAuthCreateCaptchaEnabled = computed(
  () => isPendingOAuthFlow() && captchaEnabled.value
)

watch(validationToastMessage, (value, previousValue) => {
  if (value && value !== previousValue) {
    appStore.showError(value)
  }
})

// ==================== Lifecycle ====================

onMounted(async () => {
  const activePendingSession = authStore.pendingAuthSession as PendingAuthSessionSummary | null

  // Load registration data from sessionStorage
  const registerDataStr = sessionStorage.getItem('register_data')
  if (registerDataStr) {
    try {
      const registerData = JSON.parse(registerDataStr)
      email.value = registerData.email || ''
      const legacyPassword = typeof registerData.password === 'string' ? registerData.password : ''
      if (legacyPassword) {
        setPendingRegistrationPassword(legacyPassword)
        delete registerData.password
        sessionStorage.setItem('register_data', JSON.stringify(registerData))
      }
      password.value = getPendingRegistrationPassword()
      initialTurnstileToken.value =
        registerData.tencent_captcha_ticket || registerData.turnstile_token || ''
      initialTencentCaptchaRandstr.value = registerData.tencent_captcha_randstr || ''
      promoCode.value = registerData.promo_code || ''
      invitationCode.value = registerData.invitation_code || ''
      affCode.value = registerData.aff_code || loadAffiliateReferralCode()
      pendingAuthToken.value = registerData.pending_auth_token || activePendingSession?.token || ''
      pendingAuthTokenField.value = registerData.pending_auth_token_field || activePendingSession?.token_field || 'pending_auth_token'
      pendingProvider.value = registerData.pending_provider || activePendingSession?.provider || ''
      pendingRedirect.value = sanitizeAuthRedirect(
        registerData.pending_redirect || activePendingSession?.redirect,
        ''
      )
      pendingAdoptionDecision.value = registerData.pending_adoption_decision
        ? {
            adoptDisplayName: registerData.pending_adoption_decision.adopt_display_name === true,
            adoptAvatar: registerData.pending_adoption_decision.adopt_avatar === true
          }
        : null
      hasRegisterData.value = !!email.value
      requiresPasswordReentry.value = hasRegisterData.value && password.value.length === 0
      registerDataCodeSent.value = registerData.code_sent === true
      registerDataCountdown.value = Number(registerData.countdown) || 0
    } catch {
      hasRegisterData.value = false
      requiresPasswordReentry.value = false
    }
  } else if (activePendingSession) {
    pendingAuthToken.value = activePendingSession.token
    pendingAuthTokenField.value = activePendingSession.token_field
    pendingProvider.value = activePendingSession.provider
    pendingRedirect.value = sanitizeAuthRedirect(activePendingSession.redirect, '')
  }

  // Load public settings
  try {
    const settings = await getPublicSettings()
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
    tencentCaptchaEnabled.value = settings.tencent_captcha_enabled === true
    tencentCaptchaAppId.value = settings.tencent_captcha_app_id || ''
    tencentCaptchaRegion.value = settings.tencent_captcha_region || 'cn'
    aliyunCaptchaEnabled.value = settings.aliyun_captcha_enabled === true
    aliyunCaptchaSceneId.value = settings.aliyun_captcha_scene_id || ''
    aliyunCaptchaPrefix.value = settings.aliyun_captcha_prefix || ''
    aliyunCaptchaRegion.value = settings.aliyun_captcha_region || 'cn'
    siteName.value = settings.site_name || 'Sub2API'
    registrationEmailSuffixWhitelist.value = normalizeRegistrationEmailSuffixWhitelist(
      settings.registration_email_suffix_whitelist || []
    )
    emailDomainQuotaEnabled.value = settings.registration_email_domain_quota_enabled === true
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }

  // Auto-send verification code if we have valid data
  if (hasRegisterData.value) {
    if (registerDataCodeSent.value) {
      codeSent.value = true
      if (registerDataCountdown.value > 0) {
        startCountdown(registerDataCountdown.value)
      }
      clearStoredCaptchaProof()
    } else {
      await sendCode()
    }
  }
})

onUnmounted(() => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
})

// ==================== Countdown ====================

function startCountdown(seconds: number): void {
  countdown.value = seconds

  if (countdownTimer) {
    clearInterval(countdownTimer)
  }

  countdownTimer = setInterval(() => {
    if (countdown.value > 0) {
      countdown.value--
    } else {
      if (countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = null
      }
    }
  }, 1000)
}

// ==================== Turnstile Handlers ====================

function onTurnstileVerify(token: string, randstr = ''): void {
  resendTurnstileToken.value = token
  resendTencentCaptchaRandstr.value = randstr
  errors.value.turnstile = ''
}

function onTurnstileExpire(): void {
  resendTurnstileToken.value = ''
  resendTencentCaptchaRandstr.value = ''
  errors.value.turnstile = t('auth.turnstileExpired')
}

function onTurnstileError(): void {
  resendTurnstileToken.value = ''
  resendTencentCaptchaRandstr.value = ''
  errors.value.turnstile = t('auth.turnstileFailed')
}

function onCreateAccountTurnstileVerify(token: string, randstr = ''): void {
  createAccountTurnstileToken.value = token
  createAccountTencentCaptchaRandstr.value = randstr
  errors.value.turnstile = ''
}

function onCreateAccountTurnstileExpire(): void {
  createAccountTurnstileToken.value = ''
  createAccountTencentCaptchaRandstr.value = ''
  errors.value.turnstile = t('auth.turnstileExpired')
}

function onCreateAccountTurnstileError(): void {
  createAccountTurnstileToken.value = ''
  createAccountTencentCaptchaRandstr.value = ''
  errors.value.turnstile = t('auth.turnstileFailed')
}

function resetCreateAccountTurnstile(): void {
  createAccountTurnstileToken.value = ''
  createAccountTencentCaptchaRandstr.value = ''
  createAccountTurnstileRef.value?.reset()
}

async function acquireResendActionProof(): Promise<boolean> {
  if (!tencentWidgetActive.value) return true

  const proof = await turnstileRef.value?.verifyAction()
  if (!proof) return false

  resendTurnstileToken.value = proof.token
  resendTencentCaptchaRandstr.value = proof.randstr
  return true
}

async function acquireCreateAccountActionProof(): Promise<boolean> {
  if (!isPendingOAuthFlow() || !tencentWidgetActive.value) return true

  const proof = await createAccountTurnstileRef.value?.verifyAction()
  if (!proof) return false

  createAccountTurnstileToken.value = proof.token
  createAccountTencentCaptchaRandstr.value = proof.randstr
  return true
}

function isPendingOAuthFlow(): boolean {
  return Boolean(pendingProvider.value.trim())
}

// 域名限量注册开启时交由后端按额度判定；pending OAuth / 换绑流程沿用后端策略，前端不预检。
function shouldBypassRegistrationEmailPolicy(): boolean {
  return (
    emailDomainQuotaEnabled.value || isPendingOAuthFlow() || Boolean(pendingAuthToken.value.trim())
  )
}

function resolvePendingOAuthCallbackRoute(provider: string): string {
  switch (provider.trim().toLowerCase()) {
    case 'linuxdo':
      return '/auth/linuxdo/callback'
    case 'oidc':
      return '/auth/oidc/callback'
    case 'wechat':
      return '/auth/wechat/callback'
    default:
      return '/auth/callback'
  }
}

function isPendingOAuthSessionResponse(data: PendingOAuthCreateAccountResponse): boolean {
  return data.auth_result === 'pending_session'
}

function getPendingOAuthSendCodeSessionResponse(
  data: PendingOAuthSendVerifyCodeResponse,
): PendingOAuthSendVerifyCodeResponse | null {
  return data.auth_result === 'pending_session' ? data : null
}

function persistPendingOAuthSession(provider: string, redirect?: string): void {
  authStore.setPendingAuthSession({
    token: pendingAuthToken.value,
    token_field: pendingAuthTokenField.value,
    provider: provider.trim() || pendingProvider.value.trim(),
    redirect: sanitizeAuthRedirect(redirect || pendingRedirect.value, '/dashboard'),
  })
}

// ==================== Send Code ====================

async function sendCodeWithCaptcha(captchaParam?: string): Promise<AliyunCaptchaBizResult> {
  isSendingCode.value = true
  errorMessage.value = ''
  let requestSucceeded = false
  let captchaProofUsed = false

  try {
    if (!isCanonicalRegistrationEmail(email.value)) {
      errorMessage.value = t('auth.emailAliasNotAllowed', {
        canonical_email: canonicalRegistrationEmail(email.value)
      })
      appStore.showError(errorMessage.value)
      return { captchaResult: true, bizResult: false }
    }
    if (
      !shouldBypassRegistrationEmailPolicy() &&
      !isRegistrationEmailSuffixAllowed(email.value, registrationEmailSuffixWhitelist.value)
    ) {
      errorMessage.value = buildEmailSuffixNotAllowedMessage()
      appStore.showError(errorMessage.value)
      return { captchaResult: true, bizResult: false }
    }

    const resolvedCaptchaParam =
      captchaParam || resendTurnstileToken.value || initialTurnstileToken.value || undefined
    const requestPayload = {
      email: email.value,
      [pendingAuthTokenField.value]: pendingAuthToken.value || undefined,
      // 优先使用重发时新获取的 token（因为初始 token 可能已被使用）
      turnstile_token:
        turnstileWidgetActive.value || aliyunWidgetActive.value
          ? resolvedCaptchaParam
          : undefined,
      tencent_captcha_ticket: tencentWidgetActive.value ? resolvedCaptchaParam : undefined,
      tencent_captcha_randstr: tencentWidgetActive.value
        ? resendTencentCaptchaRandstr.value || initialTencentCaptchaRandstr.value || undefined
        : undefined
    } as Parameters<typeof sendVerifyCode>[0]
    captchaProofUsed = Boolean(
      requestPayload.turnstile_token || requestPayload.tencent_captcha_ticket
    )
    const response = isPendingOAuthFlow()
      ? await sendPendingOAuthVerifyCode(requestPayload)
      : await sendVerifyCode(requestPayload)
    requestSucceeded = true

    const pendingSendCodeSession = isPendingOAuthFlow()
      ? getPendingOAuthSendCodeSessionResponse(response as PendingOAuthSendVerifyCodeResponse)
      : null
    if (pendingSendCodeSession) {
      sessionStorage.removeItem('register_data')
      persistPendingOAuthSession(
        pendingSendCodeSession.provider || pendingProvider.value,
        pendingSendCodeSession.redirect
      )
      await router.push(
        resolvePendingOAuthCallbackRoute(pendingSendCodeSession.provider || pendingProvider.value)
      )
      return { captchaResult: true, bizResult: true }
    }

    codeSent.value = true
    startCountdown(response.countdown)

    showResendTurnstile.value = false
    return { captchaResult: true, bizResult: true }
  } catch (error: unknown) {
    errorMessage.value = buildRegistrationErrorMessage(error, t('auth.sendCodeFailed'))
    appStore.showError(errorMessage.value)
    if (isAliyunCaptchaVerificationError(error)) {
      errors.value.turnstile = t('auth.completeVerification')
      return { captchaResult: false, bizResult: false }
    }
    return { captchaResult: true, bizResult: false }
  } finally {
    if (captchaProofUsed) {
      clearStoredCaptchaProof()
      initialTurnstileToken.value = ''
      initialTencentCaptchaRandstr.value = ''
      resendTurnstileToken.value = ''
      resendTencentCaptchaRandstr.value = ''
      turnstileRef.value?.reset()
      if (!requestSucceeded && turnstileWidgetActive.value) {
        showResendTurnstile.value = true
      }
    }
    isSendingCode.value = false
  }
}

async function sendCode(): Promise<void> {
  await sendCodeWithCaptcha()
}

function clearStoredCaptchaProof(): void {
  const registerDataStr = sessionStorage.getItem('register_data')
  if (!registerDataStr) return

  try {
    const registerData = JSON.parse(registerDataStr) as Record<string, unknown>
    delete registerData.turnstile_token
    delete registerData.tencent_captcha_ticket
    delete registerData.tencent_captcha_randstr
    sessionStorage.setItem('register_data', JSON.stringify(registerData))
  } catch {
    // Invalid registration state is handled by the existing onMounted parser.
  }
}

// ==================== Handlers ====================

async function handleResendCode(): Promise<void> {
  // Turnstile stays staged; 腾讯/阿里云在动作时取证。
  if (turnstileWidgetActive.value && !showResendTurnstile.value) {
    showResendTurnstile.value = true
    return
  }

  if (turnstileWidgetActive.value && !resendTurnstileToken.value) {
    errors.value.turnstile = t('auth.completeVerification')
    return
  }

  if (aliyunWidgetActive.value) {
    const result = await turnstileRef.value?.runAliyunVerify((param) =>
      sendCodeWithCaptcha(param)
    )
    if (!result?.captchaResult && !result?.bizResult) {
      errors.value.turnstile = t('auth.completeVerification')
    }
    return
  }

  if (!(await acquireResendActionProof())) {
    return
  }

  await sendCode()
}

function validateForm(): boolean {
  errors.value.code = ''

  if (!verifyCode.value.trim()) {
    errors.value.code = t('auth.codeRequired')
    return false
  }

  if (!/^\d{6}$/.test(verifyCode.value.trim())) {
    errors.value.code = t('auth.invalidCode')
    return false
  }

  if (password.value.length < 6) {
    appStore.showError(t('auth.passwordMinLength'))
    return false
  }

  return true
}

async function handleVerify(): Promise<void> {
  errorMessage.value = ''

  if (!validateForm()) {
    return
  }

  if (!isCanonicalRegistrationEmail(email.value)) {
    errorMessage.value = t('auth.emailAliasNotAllowed', {
      canonical_email: canonicalRegistrationEmail(email.value)
    })
    appStore.showError(errorMessage.value)
    return
  }

  if (!shouldBypassRegistrationEmailPolicy() && !isRegistrationEmailSuffixAllowed(email.value, registrationEmailSuffixWhitelist.value)) {
    errorMessage.value = buildEmailSuffixNotAllowedMessage()
    appStore.showError(errorMessage.value)
    return
  }

  isLoading.value = true

  try {
    if (isPendingOAuthFlow() && aliyunWidgetActive.value) {
      const result = await createAccountTurnstileRef.value?.runAliyunVerify((param) =>
        submitPendingOAuthCreateWithCaptcha(param)
      )
      if (!result?.captchaResult && !result?.bizResult) {
        errors.value.turnstile = t('auth.completeVerification')
      }
      return
    }

    if (!(await acquireCreateAccountActionProof())) {
      return
    }

    if (isPendingOAuthFlow()) {
      await submitPendingOAuthCreateWithCaptcha(createAccountTurnstileToken.value || undefined)
      return
    }

    // Register with verification code
    await authStore.register({
      email: email.value,
      password: password.value,
      verify_code: verifyCode.value.trim(),
      turnstile_token:
        turnstileWidgetActive.value || aliyunWidgetActive.value
          ? initialTurnstileToken.value || undefined
          : undefined,
      tencent_captcha_ticket: tencentWidgetActive.value
        ? initialTurnstileToken.value || undefined
        : undefined,
      tencent_captcha_randstr: tencentWidgetActive.value
        ? initialTencentCaptchaRandstr.value || undefined
        : undefined,
      promo_code: promoCode.value || undefined,
      invitation_code: invitationCode.value || undefined,
      ...(affCode.value ? { aff_code: affCode.value } : {})
    })

    sessionStorage.removeItem('register_data')
    clearPendingRegistrationPassword()
    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.accountCreatedSuccess', { siteName: siteName.value }))
    await router.push(sanitizeAuthRedirect(pendingRedirect.value))
  } catch (error: unknown) {
    errorMessage.value = buildRegistrationErrorMessage(error, t('auth.verifyFailed'))
    appStore.showError(errorMessage.value)
  } finally {
    initialTurnstileToken.value = ''
    initialTencentCaptchaRandstr.value = ''
    if (pendingOAuthCreateCaptchaEnabled.value) {
      resetCreateAccountTurnstile()
    }
    isLoading.value = false
  }
}

async function submitPendingOAuthCreateWithCaptcha(
  captchaParam?: string
): Promise<AliyunCaptchaBizResult> {
  try {
    const payload: Record<string, unknown> = {
      email: email.value,
      password: password.value,
      verify_code: verifyCode.value.trim(),
      ...(turnstileWidgetActive.value || aliyunWidgetActive.value
        ? { turnstile_token: captchaParam }
        : {}),
      ...(tencentWidgetActive.value && captchaParam
        ? {
            tencent_captcha_ticket: captchaParam,
            tencent_captcha_randstr: createAccountTencentCaptchaRandstr.value
          }
        : {}),
      ...oauthAffiliatePayload(affCode.value || loadAffiliateReferralCode())
    }
    if (invitationCode.value) {
      payload.invitation_code = invitationCode.value
    }
    if (pendingAdoptionDecision.value?.adoptDisplayName !== undefined) {
      payload.adopt_display_name = pendingAdoptionDecision.value.adoptDisplayName
    }
    if (pendingAdoptionDecision.value?.adoptAvatar !== undefined) {
      payload.adopt_avatar = pendingAdoptionDecision.value.adoptAvatar
    }

    const { data } = await apiClient.post<PendingOAuthCreateAccountResponse>(
      '/auth/oauth/pending/create-account',
      payload
    )
    if (isPendingOAuthSessionResponse(data)) {
      sessionStorage.removeItem('register_data')
      persistPendingOAuthSession(data.provider || pendingProvider.value, data.redirect)
      await router.push(resolvePendingOAuthCallbackRoute(data.provider || pendingProvider.value))
      return { captchaResult: true, bizResult: true }
    }
    if (!isOAuthLoginCompletion(data)) {
      throw new Error(t('auth.verifyFailed'))
    }

    persistOAuthTokenContext(data)
    await authStore.setToken(data.access_token)
    authStore.clearPendingAuthSession?.()

    sessionStorage.removeItem('register_data')
    clearPendingRegistrationPassword()
    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.accountCreatedSuccess', { siteName: siteName.value }))
    await router.push(sanitizeAuthRedirect(pendingRedirect.value))
    return { captchaResult: true, bizResult: true }
  } catch (error: unknown) {
    errorMessage.value = buildRegistrationErrorMessage(error, t('auth.verifyFailed'))
    appStore.showError(errorMessage.value)
    if (isAliyunCaptchaVerificationError(error)) {
      errors.value.turnstile = t('auth.completeVerification')
      return { captchaResult: false, bizResult: false }
    }
    return { captchaResult: true, bizResult: false }
  }
}

function handleBack(): void {
  // Keep non-sensitive context and the in-memory password for SPA back navigation.
  router.push('/register')
}

function buildEmailSuffixNotAllowedMessage(): string {
  const normalizedWhitelist = normalizeRegistrationEmailSuffixWhitelist(
    registrationEmailSuffixWhitelist.value
  )
  if (normalizedWhitelist.length === 0) {
    return t('auth.emailSuffixNotAllowed')
  }
  const separator = String(locale.value || '').toLowerCase().startsWith('zh') ? '、' : ', '
  return t('auth.emailSuffixNotAllowedWithAllowed', {
    suffixes: formatRegistrationEmailSuffixWhitelistForMessage(normalizedWhitelist, {
      separator,
      more: (count) => t('auth.emailSuffixAllowedMore', { count })
    })
  })
}

function buildRegistrationErrorMessage(error: unknown, fallback: string): string {
  if (extractApiErrorCode(error) === 'EMAIL_DOMAIN_REGISTRATION_LIMIT') {
    return t('auth.emailDomainRegistrationLimit')
  }
  if (extractApiErrorCode(error) === 'EMAIL_ALIAS_NOT_ALLOWED') {
    const canonical = String(extractApiErrorMetadata(error)?.canonical_email || '')
    return t('auth.emailAliasNotAllowed', { canonical_email: canonical })
  }
  return buildAuthErrorMessage(error, { fallback })
}
</script>

<style scoped>
.login-title h2 {
  margin: 0;
  font-family: var(--display);
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  text-align: center;
  color: var(--foreground);
}

.login-title p {
  margin: 6px 0 0;
  font-size: 13.5px;
  color: var(--muted);
  text-align: center;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.login-label {
  display: block;
  margin-bottom: 6px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--foreground);
}

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
