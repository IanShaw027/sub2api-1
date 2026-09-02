<template>
  <div class="login-page">
    <GlassCard class="login-brand" padding="lg">
      <div class="login-brand-inner">
        <div class="login-brand-mark">
          <img :src="siteLogo || '/logo.svg'" alt="" />
          <span>{{ siteName }}</span>
        </div>
        <div class="login-brand-copy">
          <h1>{{ t('home.heroSubtitle') }}</h1>
          <p>{{ siteSubtitle || t('home.heroDescription') }}</p>
          <ul>
            <li>{{ t('home.features.unifiedGateway') }} · {{ t('home.features.unifiedGatewayDesc') }}</li>
            <li>{{ t('home.features.multiAccount') }} · {{ t('home.features.multiAccountDesc') }}</li>
            <li>{{ t('home.features.balanceQuota') }} · {{ t('home.features.balanceQuotaDesc') }}</li>
          </ul>
        </div>
        <StatusBadge tone="success" dot :label="t('home.providers.supported')" />
      </div>
    </GlassCard>

    <div class="login-form-wrap">
      <GlassCard class="login-card" padding="lg">
        <div class="login-title">
          <h2>{{ t('auth.welcomeBack') }}</h2>
          <p>{{ t('auth.signInToAccount') }}</p>
        </div>
      <!-- Login Form -->
      <form @submit.prevent="handleLogin" class="login-form">
        <!-- Email Input -->
        <div>
          <label for="email" class="login-label">
            {{ t('auth.emailLabel') }}
          </label>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
              <Icon name="mail" size="md" class="text-muted" />
            </div>
            <input
              id="email"
              v-model="formData.email"
              type="email"
              required
              autofocus
              autocomplete="email"
              :disabled="authActionDisabled"
              class="field pl-11"
              :class="{ 'ui-text-input-error': errors.email }"
              :placeholder="t('auth.emailPlaceholder')"
            />
          </div>
        </div>

        <!-- Password Input -->
        <div>
          <label for="password" class="login-label">
            {{ t('auth.passwordLabel') }}
          </label>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
              <Icon name="lock" size="md" class="text-muted" />
            </div>
            <input
              id="password"
              v-model="formData.password"
              :type="showPassword ? 'text' : 'password'"
              required
              autocomplete="current-password"
              :disabled="authActionDisabled"
              class="field pl-11 pr-11"
              :class="{ 'ui-text-input-error': errors.password }"
              :placeholder="t('auth.passwordPlaceholder')"
            />
            <button
              type="button"
              @click="showPassword = !showPassword"
              :disabled="authActionDisabled"
              class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-muted"
            >
              <Icon v-if="showPassword" name="eyeOff" size="md" />
              <Icon v-else name="eye" size="md" />
            </button>
          </div>
          <div class="mt-1 flex items-center justify-between">
            <span></span>
            <router-link
              v-if="passwordResetEnabled && !backendModeEnabled"
              to="/forgot-password"
              class="login-link"
            >
              {{ t('auth.forgotPassword') }}
            </router-link>
          </div>
        </div>

        <!-- Turnstile Widget -->
        <div v-if="captchaEnabled">
          <TurnstileWidget
            ref="turnstileRef"
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

        <Button
          native-type="submit"
          size="md"
          class="w-full"
          :disabled="authActionDisabled || (turnstileWidgetActive && !turnstileToken)"
          :loading="isLoading"
        >
          <Icon v-if="!isLoading" name="login" size="md" />
          {{ isLoading ? t('auth.signingIn') : t('auth.signIn') }}
        </Button>

        <LoginAgreementPrompt
          v-if="loginAgreementEnabled"
          :accepted="agreementAccepted"
          :documents="loginAgreementDocuments"
          :mode="loginAgreementMode"
          :updated-at="loginAgreementUpdatedAt"
          :visible="showAgreementModal"
          @accept="acceptLoginAgreement"
          @reject="rejectLoginAgreement"
          @open="showAgreementModal = true"
        />

        <div v-if="showPasskeyLogin || showOAuthLogin" class="space-y-3 pt-1">
          <div class="login-divider">
            <span>{{ t('auth.oauthOrContinue') }}</span>
          </div>

          <Button
            v-if="showPasskeyLogin"
            variant="secondary"
            size="md"
            class="btn-secondary w-full"
            :disabled="authActionDisabled"
            :loading="passkeyLoading"
            @click="handlePasskeyLogin"
          >
            <Icon v-if="!passkeyLoading" name="key" size="md" />
            {{ passkeyLoading ? t('auth.passkeySigningIn') : t('auth.passkeySignIn') }}
          </Button>

          <EmailOAuthButtons
            :disabled="authActionDisabled"
            :github-enabled="githubOAuthEnabled"
            :google-enabled="googleOAuthEnabled"
            :show-divider="false"
            @start="handleOAuthStart"
          />

          <LinuxDoOAuthSection
            v-if="linuxdoOAuthEnabled"
            :disabled="authActionDisabled"
            :show-divider="false"
            @start="handleOAuthStart"
          />
          <DingTalkOAuthSection
            v-if="dingtalkOAuthEnabled"
            :disabled="authActionDisabled"
            :show-divider="false"
            @start="handleOAuthStart"
          />
          <WechatOAuthSection
            v-if="wechatOAuthEnabled"
            :disabled="authActionDisabled"
            :show-divider="false"
            @start="handleOAuthStart"
          />
          <OidcOAuthSection
            v-if="oidcOAuthEnabled"
            :disabled="authActionDisabled"
            :provider-name="oidcOAuthProviderName"
            :show-divider="false"
            @start="handleOAuthStart"
          />
        </div>
      </form>
        <p v-if="!backendModeEnabled" class="login-footer">
          {{ t('auth.dontHaveAccount') }}
          <router-link to="/register" class="login-link">
            {{ t('auth.signUp') }}
          </router-link>
        </p>
      </GlassCard>
    </div>
  </div>

  <!-- 2FA Modal -->
  <TotpLoginModal
    v-if="show2FAModal"
    ref="totpModalRef"
    :temp-token="totpTempToken"
    :user-email-masked="totpUserEmailMasked"
    @verify="handle2FAVerify"
    @cancel="handle2FACancel"
  />
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LinuxDoOAuthSection from '@/components/auth/LinuxDoOAuthSection.vue'
import DingTalkOAuthSection from '@/components/auth/DingTalkOAuthSection.vue'
import OidcOAuthSection from '@/components/auth/OidcOAuthSection.vue'
import WechatOAuthSection from '@/components/auth/WechatOAuthSection.vue'
import EmailOAuthButtons from '@/components/auth/EmailOAuthButtons.vue'
import LoginAgreementPrompt from '@/components/auth/LoginAgreementPrompt.vue'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import TurnstileWidget from '@/components/CaptchaChallenge.vue'
import { useAuthStore, useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import {
  buildOAuthLoginStartURL,
  getPublicSettings,
  isTotp2FARequired,
  isWeChatWebOAuthEnabled,
  startOAuthLogin,
  type OAuthLoginStart
} from '@/api/auth'
import type {
  ActionCaptchaRequestProof,
  LoginAgreementDocument,
  TotpLoginResponse
} from '@/types'
import { extractI18nErrorMessage, isAliyunCaptchaVerificationError } from '@/utils/apiError'
import type { AliyunCaptchaBizResult } from '@/components/AliyunCaptchaWidget.vue'
import { clearAllAffiliateReferralCodes } from '@/utils/oauthAffiliate'
import { sanitizeAuthRedirect } from '@/utils/authRedirect'

const { t } = useI18n()
const LOGIN_AGREEMENT_STORAGE_KEY = 'sub2api_login_agreement_consent'

// ==================== Router & Stores ====================

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')

// ==================== State ====================

const isLoading = ref<boolean>(false)
const passkeyLoading = ref<boolean>(false)
const errorMessage = ref<string>('')
const showPassword = ref<boolean>(false)
const publicSettingsLoaded = ref<boolean>(false)

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
const linuxdoOAuthEnabled = ref<boolean>(false)
const dingtalkOAuthEnabled = ref<boolean>(false)
const wechatOAuthEnabled = ref<boolean>(false)
const backendModeEnabled = ref<boolean>(false)
const oidcOAuthEnabled = ref<boolean>(false)
const oidcOAuthProviderName = ref<string>('OIDC')
const githubOAuthEnabled = ref<boolean>(false)
const googleOAuthEnabled = ref<boolean>(false)
const passwordResetEnabled = ref<boolean>(false)
const passkeyEnabled = ref<boolean>(false)
const loginAgreementEnabled = ref<boolean>(false)
const loginAgreementMode = ref<'modal' | 'checkbox' | string>('modal')
const loginAgreementUpdatedAt = ref<string>('')
const loginAgreementRevision = ref<string>('')
const loginAgreementDocuments = ref<LoginAgreementDocument[]>([])
const agreementAccepted = ref<boolean>(false)
const showAgreementModal = ref<boolean>(false)

// Turnstile
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const turnstileToken = ref<string>('')
const tencentCaptchaRandstr = ref<string>('')
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
// 动作触发式验证码（腾讯/阿里云）：提交、OAuth 启动、passkey 时弹窗验证
const actionCaptchaEnabled = computed(
  () => tencentWidgetActive.value || aliyunWidgetActive.value
)
const captchaEnabled = computed(
  () => turnstileWidgetActive.value || actionCaptchaEnabled.value
)

// 2FA state
const show2FAModal = ref<boolean>(false)
const totpTempToken = ref<string>('')
const totpUserEmailMasked = ref<string>('')
const totpModalRef = ref<InstanceType<typeof TotpLoginModal> | null>(null)

const formData = reactive({
  email: '',
  password: ''
})

const errors = reactive({
  email: '',
  password: '',
  turnstile: ''
})

const validationToastMessage = computed(
  () => errors.email || errors.password || errors.turnstile || ''
)

const agreementGateActive = computed(
  () => loginAgreementEnabled.value && !agreementAccepted.value
)

const authActionDisabled = computed(
  () => isLoading.value || passkeyLoading.value || !publicSettingsLoaded.value || agreementGateActive.value
)

const showPasskeyLogin = computed(
  () => passkeyEnabled.value && typeof window.PublicKeyCredential !== 'undefined'
)

const showOAuthLogin = computed(
  () =>
    !backendModeEnabled.value &&
    (linuxdoOAuthEnabled.value ||
      dingtalkOAuthEnabled.value ||
      wechatOAuthEnabled.value ||
      oidcOAuthEnabled.value ||
      githubOAuthEnabled.value ||
      googleOAuthEnabled.value)
)

watch(validationToastMessage, (value, previousValue) => {
  if (value && value !== previousValue) {
    appStore.showError(value)
  }
})

// ==================== Lifecycle ====================

onMounted(async () => {
  const expiredFlag = sessionStorage.getItem('auth_expired')
  if (expiredFlag) {
    sessionStorage.removeItem('auth_expired')
    const message = t('auth.reloginRequired')
    errorMessage.value = message
    appStore.showWarning(message)
  }

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
    linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled
    dingtalkOAuthEnabled.value = settings.dingtalk_oauth_enabled ?? false
    wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
    backendModeEnabled.value = settings.backend_mode_enabled
    oidcOAuthEnabled.value = settings.oidc_oauth_enabled
    oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
    githubOAuthEnabled.value = settings.github_oauth_enabled
    googleOAuthEnabled.value = settings.google_oauth_enabled
    backendModeEnabled.value = settings.backend_mode_enabled
    passwordResetEnabled.value = settings.password_reset_enabled
    passkeyEnabled.value = settings.passkey_enabled === true
    applyLoginAgreementSettings(settings)
  } catch (error) {
    console.error('Failed to load public settings:', error)
    loginAgreementEnabled.value = false
    agreementAccepted.value = true
  } finally {
    publicSettingsLoaded.value = true
  }
})

// ==================== Login Agreement ====================

function applyLoginAgreementSettings(settings: {
  login_agreement_enabled?: boolean
  login_agreement_mode?: string
  login_agreement_updated_at?: string
  login_agreement_revision?: string
  login_agreement_documents?: LoginAgreementDocument[]
}): void {
  const documents = Array.isArray(settings.login_agreement_documents)
    ? settings.login_agreement_documents.filter((doc) => doc.title?.trim())
    : []
  loginAgreementDocuments.value = documents
  loginAgreementEnabled.value = settings.login_agreement_enabled === true && documents.length > 0
  loginAgreementMode.value = settings.login_agreement_mode === 'checkbox' ? 'checkbox' : 'modal'
  loginAgreementUpdatedAt.value = settings.login_agreement_updated_at || ''
  loginAgreementRevision.value =
    settings.login_agreement_revision ||
    `${loginAgreementUpdatedAt.value}:${documents.map((doc) => `${doc.id}:${doc.title}`).join('|')}`

  agreementAccepted.value = !loginAgreementEnabled.value || hasAcceptedLoginAgreement(loginAgreementRevision.value)
  showAgreementModal.value =
    loginAgreementEnabled.value && !agreementAccepted.value && loginAgreementMode.value !== 'checkbox'
}

function hasAcceptedLoginAgreement(revision: string): boolean {
  if (!revision) {
    return false
  }
  try {
    const raw = localStorage.getItem(LOGIN_AGREEMENT_STORAGE_KEY)
    if (!raw) {
      return false
    }
    const parsed = JSON.parse(raw) as { revision?: string }
    return parsed.revision === revision
  } catch {
    return false
  }
}

function acceptLoginAgreement(): void {
  if (loginAgreementRevision.value) {
    localStorage.setItem(
      LOGIN_AGREEMENT_STORAGE_KEY,
      JSON.stringify({
        revision: loginAgreementRevision.value,
        accepted_at: new Date().toISOString()
      })
    )
  }
  agreementAccepted.value = true
  showAgreementModal.value = false
}

function rejectLoginAgreement(): void {
  localStorage.removeItem(LOGIN_AGREEMENT_STORAGE_KEY)
  agreementAccepted.value = false
  showAgreementModal.value = false
  appStore.showWarning(t('legal.loginAgreementPrompt.loginRejectedWarning'))
}

// ==================== Turnstile Handlers ====================

function onTurnstileVerify(token: string, randstr = ''): void {
  turnstileToken.value = token
  tencentCaptchaRandstr.value = randstr
  errors.turnstile = ''
}

function onTurnstileExpire(): void {
  turnstileToken.value = ''
  tencentCaptchaRandstr.value = ''
  errors.turnstile = t('auth.turnstileExpired')
}

function onTurnstileError(): void {
  turnstileToken.value = ''
  tencentCaptchaRandstr.value = ''
  errors.turnstile = t('auth.turnstileFailed')
}

function resetCaptchaProof(): void {
  turnstileRef.value?.reset()
  turnstileToken.value = ''
  tencentCaptchaRandstr.value = ''
  errors.turnstile = ''
}

async function acquireActionProof(): Promise<boolean> {
  if (!actionCaptchaEnabled.value) return true

  const proof = await turnstileRef.value?.verifyAction()
  if (!proof) return false

  turnstileToken.value = proof.token
  tencentCaptchaRandstr.value = proof.randstr
  return true
}

// ==================== Validation ====================

function validateForm(): boolean {
  // Reset errors
  errors.email = ''
  errors.password = ''
  errors.turnstile = ''

  let isValid = true

  if (agreementGateActive.value) {
    appStore.showWarning(t('legal.loginAgreementPrompt.loginRequiredWarning'))
    if (loginAgreementMode.value !== 'checkbox') {
      showAgreementModal.value = true
    }
    return false
  }

  // Email validation
  if (!formData.email.trim()) {
    errors.email = t('auth.emailRequired')
    isValid = false
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
    errors.email = t('auth.invalidEmail')
    isValid = false
  }

  // Password validation
  if (!formData.password) {
    errors.password = t('auth.passwordRequired')
    isValid = false
  } else if (formData.password.length < 6) {
    errors.password = t('auth.passwordMinLength')
    isValid = false
  }

  // 阿里云嵌入式：完成滑块后点登录按钮才取参，不在这里预检 token
  if (turnstileWidgetActive.value && !turnstileToken.value) {
    errors.turnstile = t('auth.completeVerification')
    isValid = false
  }

  return isValid
}

async function submitLoginWithCaptcha(captchaParam?: string): Promise<AliyunCaptchaBizResult> {
  try {
    const response = await authStore.login({
      email: formData.email,
      password: formData.password,
      turnstile_token:
        turnstileWidgetActive.value || aliyunWidgetActive.value ? captchaParam : undefined,
      tencent_captcha_ticket: tencentWidgetActive.value ? captchaParam : undefined,
      tencent_captcha_randstr: tencentWidgetActive.value
        ? tencentCaptchaRandstr.value
        : undefined
    })

    if (isTotp2FARequired(response)) {
      const totpResponse = response as TotpLoginResponse
      totpTempToken.value = totpResponse.temp_token || ''
      totpUserEmailMasked.value = totpResponse.user_email_masked || ''
      show2FAModal.value = true
      return { captchaResult: true, bizResult: true }
    }

    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.loginSuccess'))
    const redirectTo = sanitizeAuthRedirect(router.currentRoute.value.query.redirect)
    await router.push(redirectTo)
    return { captchaResult: true, bizResult: true }
  } catch (error: unknown) {
    errorMessage.value = extractI18nErrorMessage(error, t, 'auth.errors', t('auth.loginFailed'))
    appStore.showError(errorMessage.value)
    if (isAliyunCaptchaVerificationError(error)) {
      errors.turnstile = t('auth.completeVerification')
      return { captchaResult: false, bizResult: false }
    }
    return { captchaResult: true, bizResult: false }
  }
}

// ==================== Form Handlers ====================

async function handleLogin(): Promise<void> {
  errorMessage.value = ''

  if (!validateForm()) {
    return
  }

  isLoading.value = true
  try {
    if (aliyunWidgetActive.value) {
      // 官方 V2：业务请求放在 captchaVerifyCallback 内，与 VerifyIntelligentCaptcha 同一趟
      const result = await turnstileRef.value?.runAliyunVerify((param) =>
        submitLoginWithCaptcha(param)
      )
      if (!result?.captchaResult && !result?.bizResult) {
        errors.turnstile = t('auth.completeVerification')
      }
      return
    }

    if (!(await acquireActionProof())) {
      return
    }

    await submitLoginWithCaptcha(
      turnstileWidgetActive.value || tencentWidgetActive.value ? turnstileToken.value : undefined
    )
  } finally {
    if (captchaEnabled.value) {
      resetCaptchaProof()
    }
    isLoading.value = false
  }
}

async function handlePasskeyLogin(): Promise<void> {
  if (agreementGateActive.value) {
    appStore.showWarning(t('legal.loginAgreementPrompt.loginRequiredWarning'))
    if (loginAgreementMode.value !== 'checkbox') {
      showAgreementModal.value = true
    }
    return
  }

  passkeyLoading.value = true
  try {
    if (aliyunWidgetActive.value) {
      const result = await turnstileRef.value?.runAliyunVerify((param) =>
        submitPasskeyWithCaptcha({ turnstile_token: param })
      )
      if (!result?.captchaResult && !result?.bizResult) {
        errors.turnstile = t('auth.completeVerification')
      }
      return
    }

    let proof: ActionCaptchaRequestProof | undefined
    if (tencentWidgetActive.value) {
      const result = await turnstileRef.value?.verifyAction()
      if (!result) return
      proof = {
        tencent_captcha_ticket: result.token,
        tencent_captcha_randstr: result.randstr
      }
    }

    await submitPasskeyWithCaptcha(proof)
  } finally {
    if (actionCaptchaEnabled.value) {
      resetCaptchaProof()
    }
    passkeyLoading.value = false
  }
}

async function submitPasskeyWithCaptcha(
  proof?: ActionCaptchaRequestProof
): Promise<AliyunCaptchaBizResult> {
  try {
    await authStore.loginWithPasskey(proof)
    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.loginSuccess'))
    const redirectTo = sanitizeAuthRedirect(router.currentRoute.value.query.redirect)
    await router.push(redirectTo)
    return { captchaResult: true, bizResult: true }
  } catch (error: unknown) {
    const fallback = error instanceof DOMException && error.name === 'NotAllowedError'
      ? t('auth.passkeyCancelled')
      : t('auth.passkeyFailed')
    errorMessage.value = extractI18nErrorMessage(error, t, 'auth.errors', fallback)
    appStore.showError(errorMessage.value)
    if (isAliyunCaptchaVerificationError(error)) {
      errors.turnstile = t('auth.completeVerification')
      return { captchaResult: false, bizResult: false }
    }
    return { captchaResult: true, bizResult: false }
  }
}

async function handleOAuthStart(request: OAuthLoginStart): Promise<void> {
  if (authActionDisabled.value) return

  if (!actionCaptchaEnabled.value) {
    window.location.href = buildOAuthLoginStartURL(request)
    return
  }

  isLoading.value = true
  try {
    if (aliyunWidgetActive.value) {
      const result = await turnstileRef.value?.runAliyunVerify((param) =>
        startOAuthWithCaptcha(request, { turnstile_token: param })
      )
      if (!result?.captchaResult && !result?.bizResult) {
        errors.turnstile = t('auth.completeVerification')
      }
      return
    }

    const proof = await turnstileRef.value?.verifyAction()
    if (!proof) return

    await startOAuthWithCaptcha(
      request,
      tencentWidgetActive.value
        ? {
            tencent_captcha_ticket: proof.token,
            tencent_captcha_randstr: proof.randstr
          }
        : { turnstile_token: proof.token }
    )
  } finally {
    resetCaptchaProof()
    isLoading.value = false
  }
}

async function startOAuthWithCaptcha(
  request: OAuthLoginStart,
  proof: ActionCaptchaRequestProof
): Promise<AliyunCaptchaBizResult> {
  try {
    const result = await startOAuthLogin(request, proof)
    window.location.href = result.authorize_url
    return { captchaResult: true, bizResult: true }
  } catch (error: unknown) {
    errorMessage.value = extractI18nErrorMessage(
      error,
      t,
      'auth.errors',
      t('auth.turnstileFailed')
    )
    appStore.showError(errorMessage.value)
    if (isAliyunCaptchaVerificationError(error)) {
      errors.turnstile = t('auth.completeVerification')
      return { captchaResult: false, bizResult: false }
    }
    return { captchaResult: true, bizResult: false }
  }
}

// ==================== 2FA Handlers ====================

async function handle2FAVerify(code: string): Promise<void> {
  if (totpModalRef.value) {
    totpModalRef.value.setVerifying(true)
  }

  try {
    await authStore.login2FA(totpTempToken.value, code)

    // Close modal and show success
    show2FAModal.value = false
    clearAllAffiliateReferralCodes()
    appStore.showSuccess(t('auth.loginSuccess'))

    // Redirect to dashboard or intended route
    const redirectTo = sanitizeAuthRedirect(router.currentRoute.value.query.redirect)
    await router.push(redirectTo)
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { message?: string } } }
    const message = err.response?.data?.message || err.message || t('profile.totp.loginFailed')

    if (totpModalRef.value) {
      totpModalRef.value.setError(message)
      totpModalRef.value.setVerifying(false)
    }
  }
}

function handle2FACancel(): void {
  show2FAModal.value = false
  totpTempToken.value = ''
  totpUserEmailMasked.value = ''
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  background:
    linear-gradient(color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
    linear-gradient(90deg, color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
    radial-gradient(circle at 0% 0%, color-mix(in oklch, var(--accent) 22%, transparent) 0%, transparent 30rem),
    radial-gradient(circle at 100% 0%, color-mix(in oklch, var(--success) 14%, transparent) 0%, transparent 24rem),
    var(--background);
}

.login-brand {
  margin: 20px 0 20px 20px;
  border-radius: 20px;
  position: relative;
  overflow: hidden;
}

.login-brand-inner {
  min-height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 24px;
  padding: 28px 36px;
}

.login-brand-mark {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 700;
}

.login-brand-mark img {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  object-fit: contain;
}

.login-brand-copy h1 {
  margin: 0;
  font-family: var(--display);
  font-size: 46px;
  line-height: 1.1;
  font-weight: 800;
  letter-spacing: -0.035em;
}

.login-brand-copy p,
.login-brand-copy li {
  color: var(--muted);
  font-size: 15px;
  line-height: 1.65;
}

.login-brand-copy ul {
  margin: 16px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.login-form-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.login-card {
  width: 440px;
  max-width: 100%;
  border-radius: 20px;
}

.login-title h2 {
  margin: 0;
  font-family: var(--display);
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.login-title p {
  margin: 6px 0 0;
  font-size: 13.5px;
  color: var(--muted);
}

.login-form {
  margin-top: 22px;
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

.login-link {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--accent);
  text-decoration: none;
}

.login-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}

.login-divider::before,
.login-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border);
}

.login-footer {
  margin: 16px 0 0;
  text-align: center;
  font-size: 12.5px;
  color: var(--muted);
}

@media (max-width: 900px) {
  .login-page {
    grid-template-columns: 1fr;
  }

  .login-brand {
    display: none;
  }

  .login-form-wrap {
    padding: 24px 16px;
  }
}
</style>
