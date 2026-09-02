<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="login-title">
        <h2>{{ t('auth.forgotPasswordTitle') }}</h2>
        <p>{{ t('auth.forgotPasswordHint') }}</p>
      </div>

      <div v-if="isSubmitted" class="space-y-6">
        <div class="rounded-xl border border-[color-mix(in_oklch,var(--success)_35%,transparent)] bg-[color-mix(in_oklch,var(--success)_12%,transparent)] p-6">
          <div class="flex flex-col items-center gap-4 text-center">
            <div class="flex h-12 w-12 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--success)_18%,transparent)]">
              <Icon name="checkCircle" size="lg" class="text-[var(--success-text)]" />
            </div>
            <div>
              <h3 class="text-lg font-semibold text-foreground">
                {{ t('auth.resetEmailSent') }}
              </h3>
              <p class="mt-2 text-sm text-muted">
                {{ t('auth.resetEmailSentHint') }}
              </p>
            </div>
          </div>
        </div>

        <div class="text-center">
          <router-link to="/login" class="login-link inline-flex items-center gap-2">
            <Icon name="arrowLeft" size="sm" />
            {{ t('auth.backToLogin') }}
          </router-link>
        </div>
      </div>

      <form v-else @submit.prevent="handleSubmit" class="login-form">
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
              :disabled="isLoading"
              class="field pl-11"
              :class="{ 'input-error': errors.email }"
              :placeholder="t('auth.emailPlaceholder')"
            />
          </div>
        </div>

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
          :disabled="isLoading || (turnstileWidgetActive && !turnstileToken)"
          :loading="isLoading"
        >
          <Icon v-if="!isLoading" name="mail" size="md" />
          {{ isLoading ? t('auth.sendingResetLink') : t('auth.sendResetLink') }}
        </Button>
      </form>
    </div>

    <template #footer>
      <p>
        {{ t('auth.rememberedPassword') }}
        <router-link to="/login" class="login-link">
          {{ t('auth.signIn') }}
        </router-link>
      </p>
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import TurnstileWidget from '@/components/CaptchaChallenge.vue'
import type { AliyunCaptchaBizResult } from '@/components/AliyunCaptchaWidget.vue'
import { useAppStore } from '@/stores'
import { getPublicSettings, forgotPassword } from '@/api/auth'
import { isAliyunCaptchaVerificationError } from '@/utils/apiError'

const { t } = useI18n()

// ==================== Stores ====================

const appStore = useAppStore()

// ==================== State ====================

const isLoading = ref<boolean>(false)
const isSubmitted = ref<boolean>(false)
const errorMessage = ref<string>('')

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
const actionCaptchaEnabled = computed(
  () => tencentWidgetActive.value || aliyunWidgetActive.value
)
const captchaEnabled = computed(
  () => turnstileWidgetActive.value || actionCaptchaEnabled.value
)

const formData = reactive({
  email: ''
})

const errors = reactive({
  email: '',
  turnstile: ''
})

const validationToastMessage = computed(() => errors.email || errors.turnstile || '')

watch(validationToastMessage, (value, previousValue) => {
  if (value && value !== previousValue) {
    appStore.showError(value)
  }
})

// ==================== Lifecycle ====================

onMounted(async () => {
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
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
})

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
  if (!tencentWidgetActive.value) return true

  const proof = await turnstileRef.value?.verifyAction()
  if (!proof) return false

  turnstileToken.value = proof.token
  tencentCaptchaRandstr.value = proof.randstr
  return true
}

// ==================== Validation ====================

function validateForm(): boolean {
  errors.email = ''
  errors.turnstile = ''

  let isValid = true

  // Email validation
  if (!formData.email.trim()) {
    errors.email = t('auth.emailRequired')
    isValid = false
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
    errors.email = t('auth.invalidEmail')
    isValid = false
  }

  // 阿里云嵌入式：点提交后再取参，不在这里预检 token
  if (turnstileWidgetActive.value && !turnstileToken.value) {
    errors.turnstile = t('auth.completeVerification')
    isValid = false
  }

  return isValid
}

function forgotPasswordErrorMessage(error: unknown): string {
  const err = error as { message?: string; response?: { data?: { detail?: string } } }
  if (err.response?.data?.detail) return err.response.data.detail
  if (err.message) return err.message
  return t('auth.sendResetLinkFailed')
}

async function submitForgotWithCaptcha(captchaParam?: string): Promise<AliyunCaptchaBizResult> {
  try {
    await forgotPassword({
      email: formData.email,
      // 阿里云 captchaVerifyParam 复用后端 turnstile_token 字段。
      turnstile_token:
        turnstileWidgetActive.value || aliyunWidgetActive.value ? captchaParam : undefined,
      tencent_captcha_ticket: tencentWidgetActive.value ? captchaParam : undefined,
      tencent_captcha_randstr: tencentWidgetActive.value ? tencentCaptchaRandstr.value : undefined
    })
    return { captchaResult: true, bizResult: true }
  } catch (error: unknown) {
    errorMessage.value = forgotPasswordErrorMessage(error)
    appStore.showError(errorMessage.value)
    if (isAliyunCaptchaVerificationError(error)) {
      errors.turnstile = t('auth.completeVerification')
      return { captchaResult: false, bizResult: false }
    }
    return { captchaResult: true, bizResult: false }
  }
}

// ==================== Form Handlers ====================

async function handleSubmit(): Promise<void> {
  errorMessage.value = ''

  if (!validateForm()) {
    return
  }

  isLoading.value = true
  let submitted = false

  try {
    if (aliyunWidgetActive.value) {
      const result = await turnstileRef.value?.runAliyunVerify((param) =>
        submitForgotWithCaptcha(param)
      )
      if (!result?.captchaResult && !result?.bizResult) {
        errors.turnstile = t('auth.completeVerification')
      }
      submitted = result?.bizResult === true
    } else {
      if (!(await acquireActionProof())) {
        return
      }

      const result = await submitForgotWithCaptcha(
        turnstileWidgetActive.value || tencentWidgetActive.value ? turnstileToken.value : undefined
      )
      submitted = result.bizResult === true
    }
  } finally {
    // 先 reset 再切成功态，避免 v-if 卸载验证码组件后拿不到 ref
    if (captchaEnabled.value) {
      resetCaptchaProof()
    }
    isLoading.value = false
  }

  if (submitted) {
    isSubmitted.value = true
    appStore.showSuccess(t('auth.resetEmailSent'))
  }
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

.login-link {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--accent);
  text-decoration: none;
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
