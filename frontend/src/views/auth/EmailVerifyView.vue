<template>
  <AuthLayout>
    <div class="space-y-6">
      <!-- Title -->
      <div class="text-center">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('auth.verifyYourEmail') }}
        </h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          {{ t('auth.sendCodeDesc') }}
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ email }}</span>
        </p>
      </div>

      <!-- No Data Warning -->
      <div
        v-if="!hasRegisterData"
        class="rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-800/50 dark:bg-amber-900/20"
      >
        <div class="flex items-start gap-3">
          <div class="flex-shrink-0">
            <Icon name="exclamationCircle" size="md" class="text-amber-500" />
          </div>
          <div class="text-sm text-amber-700 dark:text-amber-400">
            <p class="font-medium">{{ t('auth.sessionExpired') }}</p>
            <p class="mt-1">{{ t('auth.sessionExpiredDesc') }}</p>
          </div>
        </div>
      </div>

      <!-- Verification Form -->
      <form v-else @submit.prevent="handleVerify" class="space-y-5">
        <div>
          <label for="password" class="input-label">
            {{ t('auth.passwordLabel') }}
          </label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            minlength="8"
            autocomplete="new-password"
            class="input"
            :class="{ 'input-error': errors.password }"
            :placeholder="t('auth.createPasswordPlaceholder')"
          />
          <p class="input-hint">{{ t('auth.passwordHint') }}</p>
        </div>

        <div>
          <label for="confirm-password" class="input-label">
            {{ t('auth.confirmPassword') }}
          </label>
          <input
            id="confirm-password"
            v-model="confirmPassword"
            type="password"
            required
            minlength="8"
            autocomplete="new-password"
            class="input"
            :class="{ 'input-error': errors.confirmPassword }"
            :placeholder="t('auth.confirmPasswordPlaceholder')"
          />
        </div>

        <!-- Verification Code Input -->
        <div>
          <label for="code" class="input-label text-center">
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
            class="input py-3 text-center font-mono text-xl tracking-[0.5em]"
            :class="{ 'input-error': errors.code }"
            placeholder="000000"
          />
          <p class="input-hint text-center">{{ t('auth.verificationCodeHint') }}</p>
        </div>

        <!-- Code Status -->
        <div
          v-if="codeSent"
          class="rounded-xl border border-green-200 bg-green-50 p-4 dark:border-green-800/50 dark:bg-green-900/20"
        >
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
              <Icon name="checkCircle" size="md" class="text-green-500" />
            </div>
            <p class="text-sm text-green-700 dark:text-green-400">
              {{ t('auth.codeSentSuccess') }}
            </p>
          </div>
        </div>

        <!-- Turnstile Widget for Resend -->
        <div v-if="turnstileEnabled && turnstileSiteKey && showResendTurnstile">
          <TurnstileWidget
            ref="turnstileRef"
            :site-key="turnstileSiteKey"
            @verify="onTurnstileVerify"
            @expire="onTurnstileExpire"
            @error="onTurnstileError"
          />
        </div>

        <!-- Submit Button -->
        <button
          type="submit"
          :disabled="isLoading || !verifyCode || password.length < 8 || password !== confirmPassword"
          class="btn btn-primary w-full"
        >
          <svg
            v-if="isLoading"
            class="-ml-1 mr-2 h-4 w-4 animate-spin text-white"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          <Icon v-else name="checkCircle" size="md" class="mr-2" />
          {{ isLoading ? t('auth.verifying') : t('auth.verifyAndCreate') }}
        </button>

        <!-- Resend Code -->
        <div class="text-center">
          <button
            v-if="countdown > 0"
            type="button"
            disabled
            class="cursor-not-allowed text-sm text-gray-400 dark:text-dark-500"
          >
            {{ t('auth.resendCountdown', { countdown }) }}
          </button>
          <button
            v-else
            type="button"
            @click="handleResendCode"
            :disabled="
              isSendingCode || (turnstileEnabled && showResendTurnstile && !resendTurnstileToken)
            "
            class="text-sm text-primary-600 transition-colors hover:text-primary-500 disabled:cursor-not-allowed disabled:opacity-50 dark:text-primary-400 dark:hover:text-primary-300"
          >
            <span v-if="isSendingCode">{{ t('auth.sendingCode') }}</span>
            <span v-else-if="turnstileEnabled && !showResendTurnstile">
              {{ t('auth.clickToResend') }}
            </span>
            <span v-else>{{ t('auth.resendCode') }}</span>
          </button>
        </div>
      </form>
    </div>

    <!-- Footer -->
    <template #footer>
      <button
        @click="handleBack"
        class="flex items-center gap-2 text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-gray-300"
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
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { useAuthStore, useAppStore } from '@/stores'
import { sanitizeRedirectPath } from '@/utils/sanitize'
import {
  persistOAuthTokenContext,
  getPublicSettings,
  isOAuthLoginCompletion,
  type PendingOAuthSendVerifyCodeResponse,
  sendPendingOAuthVerifyCode,
  sendVerifyCode,
} from '@/api/auth'
import { apiClient } from '@/api/client'
import { buildAuthErrorMessage } from '@/utils/authError'
import {
  formatRegistrationEmailSuffixWhitelistForMessage,
  isRegistrationEmailSuffixAllowed,
  normalizeRegistrationEmailSuffixWhitelist
} from '@/utils/registrationEmailPolicy'
import {
  clearAllAffiliateReferralCodes,
  loadAffiliateReferralCode,
  oauthAffiliatePayload
} from '@/utils/oauthAffiliate'

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

// Non-sensitive registration metadata from sessionStorage.
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
const confirmPassword = ref<string>('')
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

// Public settings
const turnstileEnabled = ref<boolean>(false)
const turnstileSiteKey = ref<string>('')
const siteName = ref<string>('Sub2API')
const registrationEmailSuffixWhitelist = ref<string[]>([])

// Turnstile for resend
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const resendTurnstileToken = ref<string>('')
const showResendTurnstile = ref<boolean>(false)

const errors = ref({
  code: '',
  password: '',
  confirmPassword: '',
  turnstile: ''
})

const validationToastMessage = computed(
  () =>
    errors.value.code ||
    errors.value.password ||
    errors.value.confirmPassword ||
    errors.value.turnstile ||
    ''
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
      promoCode.value = registerData.promo_code || ''
      invitationCode.value = registerData.invitation_code || ''
      affCode.value = registerData.aff_code || loadAffiliateReferralCode()
      pendingAuthToken.value = activePendingSession?.token || ''
      pendingAuthTokenField.value = activePendingSession?.token_field || 'pending_auth_token'
      pendingProvider.value = registerData.pending_provider || activePendingSession?.provider || ''
      pendingRedirect.value = sanitizeRedirectPath(registerData.pending_redirect || activePendingSession?.redirect || '')
      pendingAdoptionDecision.value = registerData.pending_adoption_decision
        ? {
            adoptDisplayName: registerData.pending_adoption_decision.adopt_display_name === true,
            adoptAvatar: registerData.pending_adoption_decision.adopt_avatar === true
          }
        : null
      hasRegisterData.value = !!email.value
      const sentAt = Number(registerData.code_sent_at)
      const originalCountdown = Number(registerData.code_countdown)
      if (Number.isFinite(sentAt) && Number.isFinite(originalCountdown) && originalCountdown > 0) {
        const elapsedSeconds = Math.floor((Date.now() - sentAt) / 1000)
        const remaining = Math.max(0, Math.ceil(originalCountdown) - elapsedSeconds)
        if (remaining > 0) {
          codeSent.value = true
          startCountdown(remaining)
        }
      }
    } catch {
      hasRegisterData.value = false
    }
  } else if (activePendingSession) {
    pendingAuthToken.value = activePendingSession.token
    pendingAuthTokenField.value = activePendingSession.token_field
    pendingProvider.value = activePendingSession.provider
    pendingRedirect.value = sanitizeRedirectPath(activePendingSession.redirect || '')
  }

  // Load public settings
  try {
    const settings = await getPublicSettings()
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
    siteName.value = settings.site_name || 'Sub2API'
    registrationEmailSuffixWhitelist.value = normalizeRegistrationEmailSuffixWhitelist(
      settings.registration_email_suffix_whitelist || []
    )
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }

  // RegisterView sends the initial code before navigation so no Turnstile token
  // has to cross a storage boundary. Legacy/recovered flows can send here only
  // when no challenge is required; otherwise the user completes a new widget.
  if (hasRegisterData.value && !codeSent.value && turnstileEnabled.value) {
    showResendTurnstile.value = true
  } else if (hasRegisterData.value && !codeSent.value) {
    await sendCode()
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

function onTurnstileVerify(token: string): void {
  resendTurnstileToken.value = token
  errors.value.turnstile = ''
}

function onTurnstileExpire(): void {
  resendTurnstileToken.value = ''
  errors.value.turnstile = t('auth.turnstileExpired')
}

function onTurnstileError(): void {
  resendTurnstileToken.value = ''
  errors.value.turnstile = t('auth.turnstileFailed')
}

function isPendingOAuthFlow(): boolean {
  return Boolean(pendingProvider.value.trim())
}

function shouldBypassRegistrationEmailPolicy(): boolean {
  return isPendingOAuthFlow() || Boolean(pendingAuthToken.value.trim())
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
    redirect: (() => {
      const raw = redirect || pendingRedirect.value
      if (!raw) return undefined
      const cleaned = sanitizeRedirectPath(raw, '')
      return cleaned || undefined
    })(),
  })
}

// ==================== Send Code ====================

async function sendCode(): Promise<void> {
  isSendingCode.value = true
  errorMessage.value = ''

  try {
    if (!shouldBypassRegistrationEmailPolicy() && !isRegistrationEmailSuffixAllowed(email.value, registrationEmailSuffixWhitelist.value)) {
      errorMessage.value = buildEmailSuffixNotAllowedMessage()
      appStore.showError(errorMessage.value)
      return
    }

    const requestPayload = {
      email: email.value,
      [pendingAuthTokenField.value]: pendingAuthToken.value || undefined,
      turnstile_token: resendTurnstileToken.value || undefined
    } as Parameters<typeof sendVerifyCode>[0]
    const response = isPendingOAuthFlow()
      ? await sendPendingOAuthVerifyCode(requestPayload)
      : await sendVerifyCode(requestPayload)

    const pendingSendCodeSession = isPendingOAuthFlow()
      ? getPendingOAuthSendCodeSessionResponse(response as PendingOAuthSendVerifyCodeResponse)
      : null
    if (pendingSendCodeSession) {
      sessionStorage.removeItem('register_data')
      persistPendingOAuthSession(
        pendingSendCodeSession.provider || pendingProvider.value,
        pendingSendCodeSession.redirect,
      )
      await router.push(
        resolvePendingOAuthCallbackRoute(pendingSendCodeSession.provider || pendingProvider.value),
      )
      return
    }

    codeSent.value = true
    startCountdown(response.countdown)
    persistSafeRegistrationMetadata(response.countdown)

    // Reset turnstile state（token 已使用，清除以避免重复使用）
    showResendTurnstile.value = false
    resendTurnstileToken.value = ''
  } catch (error: unknown) {
    errorMessage.value = buildAuthErrorMessage(error, {
      fallback: t('auth.sendCodeFailed')
    })

    appStore.showError(errorMessage.value)
  } finally {
    isSendingCode.value = false
  }
}

// ==================== Handlers ====================

async function handleResendCode(): Promise<void> {
  // If turnstile is enabled and we haven't shown it yet, show it
  if (turnstileEnabled.value && !showResendTurnstile.value) {
    showResendTurnstile.value = true
    return
  }

  // If turnstile is enabled but no token yet, wait
  if (turnstileEnabled.value && !resendTurnstileToken.value) {
    errors.value.turnstile = t('auth.completeVerification')
    return
  }

  await sendCode()
}

function validateForm(): boolean {
  errors.value.code = ''
  errors.value.password = ''
  errors.value.confirmPassword = ''

  if (!password.value) {
    errors.value.password = t('auth.passwordRequired')
    return false
  }
  if (password.value.length < 8) {
    errors.value.password = t('auth.passwordMinLength')
    return false
  }
  if (!confirmPassword.value) {
    errors.value.confirmPassword = t('auth.confirmPasswordRequired')
    return false
  }
  if (password.value !== confirmPassword.value) {
    errors.value.confirmPassword = t('auth.passwordsDoNotMatch')
    return false
  }

  if (!verifyCode.value.trim()) {
    errors.value.code = t('auth.codeRequired')
    return false
  }

  if (!/^\d{6}$/.test(verifyCode.value.trim())) {
    errors.value.code = t('auth.invalidCode')
    return false
  }

  return true
}

async function handleVerify(): Promise<void> {
  errorMessage.value = ''

  if (!validateForm()) {
    return
  }

  isLoading.value = true

  try {
    if (!shouldBypassRegistrationEmailPolicy() && !isRegistrationEmailSuffixAllowed(email.value, registrationEmailSuffixWhitelist.value)) {
      errorMessage.value = buildEmailSuffixNotAllowedMessage()
      appStore.showError(errorMessage.value)
      return
    }

    if (isPendingOAuthFlow()) {
      const payload: Record<string, unknown> = {
        email: email.value,
        password: password.value,
        verify_code: verifyCode.value.trim(),
        ...oauthAffiliatePayload(affCode.value || loadAffiliateReferralCode()),
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
        return
      }
      if (!isOAuthLoginCompletion(data)) {
        throw new Error(t('auth.verifyFailed'))
      }

      const effectiveTokens = (await persistOAuthTokenContext(data)) || data
      await authStore.setToken(effectiveTokens.access_token || data.access_token)
      authStore.clearPendingAuthSession?.()
    } else {
      // Register with verification code
      const registerResponse = await authStore.register({
        email: email.value,
        password: password.value,
        verify_code: verifyCode.value.trim(),
        promo_code: promoCode.value || undefined,
        invitation_code: invitationCode.value || undefined,
        ...(affCode.value ? { aff_code: affCode.value } : {})
      })
      const successMessages = registerResponse.messages?.length
        ? registerResponse.messages.join('\n')
        : t('auth.accountCreatedSuccess', { siteName: siteName.value })
      appStore.showSuccess(successMessages)
    }

    // Clear session data
    sessionStorage.removeItem('register_data')
    password.value = ''
    confirmPassword.value = ''
    clearAllAffiliateReferralCodes()

    // Redirect to dashboard
    await router.push(sanitizeRedirectPath(pendingRedirect.value, '/dashboard'))
  } catch (error: unknown) {
    errorMessage.value = buildAuthErrorMessage(error, {
      fallback: t('auth.verifyFailed')
    })

    appStore.showError(errorMessage.value)
  } finally {
    isLoading.value = false
  }
}

function handleBack(): void {
  // Clear session data
  sessionStorage.removeItem('register_data')
  password.value = ''
  confirmPassword.value = ''

  // Go back to registration
  router.push('/register')
}

function persistSafeRegistrationMetadata(codeCountdown: number): void {
  sessionStorage.setItem(
    'register_data',
    JSON.stringify({
      email: email.value,
      promo_code: promoCode.value || undefined,
      invitation_code: invitationCode.value || undefined,
      aff_code: affCode.value || undefined,
      pending_provider: pendingProvider.value || undefined,
      pending_redirect: pendingRedirect.value || undefined,
      pending_adoption_decision: pendingAdoptionDecision.value
        ? {
            adopt_display_name: pendingAdoptionDecision.value.adoptDisplayName === true,
            adopt_avatar: pendingAdoptionDecision.value.adoptAvatar === true
          }
        : undefined,
      code_sent_at: Date.now(),
      code_countdown: codeCountdown
    })
  )
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
