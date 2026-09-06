<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="login-title">
        <h2>{{ t('auth.resetPasswordTitle') }}</h2>
        <p>{{ t('auth.resetPasswordHint') }}</p>
      </div>

      <div v-if="isInvalidLink" class="forgot-result">
        <div class="notice notice-warning">
          <Icon name="exclamationCircle" size="sm" class="notice-icon" />
          <div>
            <p class="notice-title">{{ t('auth.invalidResetLink') }}</p>
            <p>{{ t('auth.invalidResetLinkHint') }}</p>
          </div>
        </div>

        <div class="forgot-result-back">
          <router-link to="/forgot-password" class="login-link">
            {{ t('auth.requestNewResetLink') }}
          </router-link>
        </div>
      </div>

      <div v-else-if="isSuccess" class="forgot-result">
        <div class="notice notice-success">
          <Icon name="checkCircle" size="sm" class="notice-icon" />
          <div>
            <p class="notice-title">{{ t('auth.passwordResetSuccess') }}</p>
            <p>{{ t('auth.passwordResetSuccessHint') }}</p>
          </div>
        </div>

        <div class="forgot-result-back">
          <Button to="/login" size="lg" class="forgot-submit">
            <Icon name="login" size="sm" />
            {{ t('auth.signIn') }}
          </Button>
        </div>
      </div>

      <form v-else @submit.prevent="handleSubmit" class="login-form">
        <TextInput
          id="email"
          :model-value="email"
          :label="t('auth.emailLabel')"
          type="email"
          readonly
          disabled
          class="input-lg"
        />

        <TextInput
          id="password"
          v-model="formData.password"
          :label="t('auth.newPassword')"
          :type="showPassword ? 'text' : 'password'"
          required
          autocomplete="new-password"
          :disabled="isLoading"
          class="input-lg"
          :error="errors.password"
          :placeholder="t('auth.newPasswordPlaceholder')"
        >
          <template #suffix>
            <button
              type="button"
              @click="showPassword = !showPassword"
              class="login-eye"
              :disabled="isLoading"
              :aria-label="t('auth.newPassword')"
              :aria-pressed="showPassword"
              aria-controls="password"
            >
              <Icon v-if="showPassword" name="eyeOff" size="sm" />
              <Icon v-else name="eye" size="sm" />
            </button>
          </template>
        </TextInput>

        <TextInput
          id="confirmPassword"
          v-model="formData.confirmPassword"
          :label="t('auth.confirmPassword')"
          :type="showConfirmPassword ? 'text' : 'password'"
          required
          autocomplete="new-password"
          :disabled="isLoading"
          class="input-lg"
          :error="errors.confirmPassword"
          :placeholder="t('auth.confirmPasswordPlaceholder')"
        >
          <template #suffix>
            <button
              type="button"
              @click="showConfirmPassword = !showConfirmPassword"
              class="login-eye"
              :disabled="isLoading"
              :aria-label="t('auth.confirmPassword')"
              :aria-pressed="showConfirmPassword"
              aria-controls="confirmPassword"
            >
              <Icon v-if="showConfirmPassword" name="eyeOff" size="sm" />
              <Icon v-else name="eye" size="sm" />
            </button>
          </template>
        </TextInput>

        <Button
          native-type="submit"
          size="lg"
          class="forgot-submit"
          :disabled="isLoading"
          :loading="isLoading"
        >
          <Icon v-if="!isLoading" name="checkCircle" size="sm" />
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
import TextInput from '@/components/ui/TextInput.vue'
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

.forgot-result {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.forgot-result-back {
  display: flex;
  justify-content: center;
}

.forgot-submit {
  width: 100%;
}

.login-eye {
  height: 32px;
  width: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
  background: transparent;
  border: 0;
  cursor: pointer;
}

.login-eye:hover {
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
