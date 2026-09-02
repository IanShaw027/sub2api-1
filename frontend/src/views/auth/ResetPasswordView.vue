<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="login-title">
        <h2>{{ t('auth.resetPasswordTitle') }}</h2>
        <p>{{ t('auth.resetPasswordHint') }}</p>
      </div>

      <div v-if="isInvalidLink" class="space-y-6">
        <div class="rounded-xl border border-[color-mix(in_oklch,var(--warning)_35%,transparent)] bg-[color-mix(in_oklch,var(--warning)_12%,transparent)] p-6">
          <div class="flex flex-col items-center gap-4 text-center">
            <div class="flex h-12 w-12 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]">
              <Icon name="exclamationCircle" size="lg" class="text-[var(--warning-text)]" />
            </div>
            <div>
              <h3 class="text-lg font-semibold text-foreground">
                {{ t('auth.invalidResetLink') }}
              </h3>
              <p class="mt-2 text-sm text-muted">
                {{ t('auth.invalidResetLinkHint') }}
              </p>
            </div>
          </div>
        </div>

        <div class="text-center">
          <router-link to="/forgot-password" class="login-link inline-flex items-center gap-2">
            {{ t('auth.requestNewResetLink') }}
          </router-link>
        </div>
      </div>

      <div v-else-if="isSuccess" class="space-y-6">
        <div class="rounded-xl border border-[color-mix(in_oklch,var(--success)_35%,transparent)] bg-[color-mix(in_oklch,var(--success)_12%,transparent)] p-6">
          <div class="flex flex-col items-center gap-4 text-center">
            <div class="flex h-12 w-12 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--success)_18%,transparent)]">
              <Icon name="checkCircle" size="lg" class="text-[var(--success-text)]" />
            </div>
            <div>
              <h3 class="text-lg font-semibold text-foreground">
                {{ t('auth.passwordResetSuccess') }}
              </h3>
              <p class="mt-2 text-sm text-muted">
                {{ t('auth.passwordResetSuccessHint') }}
              </p>
            </div>
          </div>
        </div>

        <div class="text-center">
          <Button to="/login" size="md" class="inline-flex items-center gap-2">
            <Icon name="login" size="md" />
            {{ t('auth.signIn') }}
          </Button>
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
              :value="email"
              type="email"
              readonly
              disabled
              class="field pl-11"
            />
          </div>
        </div>

        <div>
          <label for="password" class="login-label">
            {{ t('auth.newPassword') }}
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
              autocomplete="new-password"
              :disabled="isLoading"
              class="field pl-11 pr-11"
              :class="{ 'input-error': errors.password }"
              :placeholder="t('auth.newPasswordPlaceholder')"
            />
            <button
              type="button"
              @click="showPassword = !showPassword"
              class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-muted"
            >
              <Icon v-if="showPassword" name="eyeOff" size="md" />
              <Icon v-else name="eye" size="md" />
            </button>
          </div>
        </div>

        <div>
          <label for="confirmPassword" class="login-label">
            {{ t('auth.confirmPassword') }}
          </label>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
              <Icon name="lock" size="md" class="text-muted" />
            </div>
            <input
              id="confirmPassword"
              v-model="formData.confirmPassword"
              :type="showConfirmPassword ? 'text' : 'password'"
              required
              autocomplete="new-password"
              :disabled="isLoading"
              class="field pl-11 pr-11"
              :class="{ 'input-error': errors.confirmPassword }"
              :placeholder="t('auth.confirmPasswordPlaceholder')"
            />
            <button
              type="button"
              @click="showConfirmPassword = !showConfirmPassword"
              class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-muted"
            >
              <Icon v-if="showConfirmPassword" name="eyeOff" size="md" />
              <Icon v-else name="eye" size="md" />
            </button>
          </div>
        </div>

        <Button
          native-type="submit"
          size="md"
          class="w-full"
          :disabled="isLoading"
          :loading="isLoading"
        >
          <Icon v-if="!isLoading" name="checkCircle" size="md" />
          {{ isLoading ? t('auth.resettingPassword') : t('auth.resetPassword') }}
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
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import { useAppStore } from '@/stores'
import { resetPassword } from '@/api/auth'

const { t } = useI18n()

// ==================== Router & Stores ====================

const route = useRoute()
const appStore = useAppStore()

// ==================== State ====================

const isLoading = ref<boolean>(false)
const isSuccess = ref<boolean>(false)
const errorMessage = ref<string>('')
const showPassword = ref<boolean>(false)
const showConfirmPassword = ref<boolean>(false)

// URL parameters
const email = ref<string>('')
const token = ref<string>('')

const formData = reactive({
  password: '',
  confirmPassword: ''
})

const errors = reactive({
  password: '',
  confirmPassword: ''
})

const validationToastMessage = computed(
  () => errors.password || errors.confirmPassword || ''
)

watch(validationToastMessage, (value, previousValue) => {
  if (value && value !== previousValue) {
    appStore.showError(value)
  }
})

// Check if the reset link is valid (has email and token)
const isInvalidLink = computed(() => !email.value || !token.value)

// ==================== Lifecycle ====================

onMounted(() => {
  // Get email and token from URL query parameters
  email.value = (route.query.email as string) || ''
  token.value = (route.query.token as string) || ''

  // Keep the one-time secret only in memory. Same-origin requests otherwise
  // send the full query in Referer under strict-origin-when-cross-origin.
  if (email.value || token.value) {
    window.history.replaceState(window.history.state, '', route.path)
  }

  if (!email.value || !token.value) {
    appStore.showError(t('auth.invalidResetLink'))
  }
})

// ==================== Validation ====================

function validateForm(): boolean {
  errors.password = ''
  errors.confirmPassword = ''

  let isValid = true

  // Password validation
  if (!formData.password) {
    errors.password = t('auth.passwordRequired')
    isValid = false
  } else if (formData.password.length < 6) {
    errors.password = t('auth.passwordMinLength')
    isValid = false
  }

  // Confirm password validation
  if (!formData.confirmPassword) {
    errors.confirmPassword = t('auth.confirmPasswordRequired')
    isValid = false
  } else if (formData.password !== formData.confirmPassword) {
    errors.confirmPassword = t('auth.passwordsDoNotMatch')
    isValid = false
  }

  return isValid
}

// ==================== Form Handlers ====================

async function handleSubmit(): Promise<void> {
  errorMessage.value = ''

  if (!validateForm()) {
    return
  }

  isLoading.value = true

  try {
    await resetPassword({
      email: email.value,
      token: token.value,
      new_password: formData.password
    })

    isSuccess.value = true
    appStore.showSuccess(t('auth.passwordResetSuccess'))
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { detail?: string; code?: string } } }

    // Check for invalid/expired token error
    if (err.response?.data?.code === 'INVALID_RESET_TOKEN') {
      errorMessage.value = t('auth.invalidOrExpiredToken')
    } else if (err.response?.data?.detail) {
      errorMessage.value = err.response.data.detail
    } else if (err.message) {
      errorMessage.value = err.message
    } else {
      errorMessage.value = t('auth.resetPasswordFailed')
    }

    appStore.showError(errorMessage.value)
  } finally {
    isLoading.value = false
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
