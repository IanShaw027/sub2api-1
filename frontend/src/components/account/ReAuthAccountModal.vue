<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.reAuthorizeAccount')"
    width="normal"
    @close="handleClose"
  >
    <div v-if="account" class="space-y-4">
      <!-- Account Info -->
      <div
        class="rounded-control border border-line bg-page p-4 dark:border-dark-600 dark:bg-dark-700"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex h-10 w-10 items-center justify-center rounded-control bg-gradient-to-br',
              isOpenAILike
                ? 'from-success to-brand-600'
                : isGemini
                  ? 'from-accent-500 to-accent-600'
                  : isAntigravity
                    ? 'from-brand-500 to-brand-600'
                    : 'from-warning to-brand-600'
            ]"
          >
            <PlatformIcon
              :platform="account.platform"
              size="lg"
              class="text-white"
              data-testid="reauth-platform-icon"
            />
          </div>
          <div>
            <span class="block font-semibold text-ink dark:text-white">{{
              account.name
            }}</span>
            <span class="text-sm text-ink-soft dark:text-ink-soft">
              {{
                isOpenAI
                  ? t('admin.accounts.openaiAccount')
                  : isGemini
                    ? t('admin.accounts.geminiAccount')
                    : isAntigravity
                      ? t('admin.accounts.antigravityAccount')
                      : t('admin.accounts.claudeCodeAccount')
              }}
            </span>
          </div>
        </div>
      </div>

      <!-- Add Method Selection (Claude only) -->
      <fieldset v-if="isAnthropic" class="border-0 p-0">
        <legend class="input-label">{{ t('admin.accounts.oauth.authMethod') }}</legend>
        <div class="mt-2 flex gap-4">
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="oauth"
              class="mr-2 text-brand-600 focus:ring-accent/25"
            />
            <span class="text-sm text-ink-body dark:text-ink-body">{{
              t('admin.accounts.types.oauth')
            }}</span>
          </label>
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="setup-token"
              class="mr-2 text-brand-600 focus:ring-accent/25"
            />
            <span class="text-sm text-ink-body dark:text-ink-body">{{
              t('admin.accounts.setupTokenLongLived')
            }}</span>
          </label>
        </div>
      </fieldset>

      <!-- Gemini OAuth Type Display (read-only) -->
      <div v-if="isGemini" class="rounded-control border border-line bg-page p-4 dark:border-dark-600 dark:bg-dark-700">
        <div class="mb-2 text-sm font-medium text-ink-body dark:text-ink-body">
          {{ t('admin.accounts.oauth.gemini.oauthTypeLabel') }}
        </div>
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex h-8 w-8 shrink-0 items-center justify-center rounded-control',
              geminiOAuthType === 'google_one'
                ? 'bg-purple-500 text-white'
                : geminiOAuthType === 'code_assist'
                  ? 'bg-accent-500 text-white'
                  : 'bg-ink dark:bg-dark-700 text-white'
            ]"
          >
            <Icon v-if="geminiOAuthType === 'google_one'" name="user" size="sm" />
            <Icon v-else-if="geminiOAuthType === 'code_assist'" name="cloud" size="sm" />
            <Icon v-else name="shield" size="sm" />
          </div>
          <div>
            <span class="block text-sm font-medium text-ink dark:text-white">
              {{
                geminiOAuthType === 'google_one'
                  ? t('admin.accounts.oauth.gemini.googleOneTitle')
                  : geminiOAuthType === 'code_assist'
                    ? t('admin.accounts.gemini.oauthType.builtInTitle')
                    : t('common.unknown')
              }}
            </span>
            <span class="text-xs text-ink-soft dark:text-ink-soft">
              {{
                geminiOAuthType === 'google_one'
                  ? t('admin.accounts.oauth.gemini.googleOneDesc')
                  : geminiOAuthType === 'code_assist'
                    ? t('admin.accounts.oauth.gemini.codeAssistDesc')
                    : t('admin.accounts.oauth.gemini.cannotInferOAuthType')
              }}
            </span>
          </div>
        </div>
      </div>

      <OAuthAuthorizationFlow
        ref="oauthFlowRef"
        :add-method="addMethod"
        :auth-url="currentAuthUrl"
        :session-id="currentSessionId"
        :loading="currentLoading"
        :error="currentError"
        :error-code="currentErrorCode"
        :show-help="isAnthropic"
        :show-proxy-warning="isAnthropic"
        :show-cookie-option="isAnthropic"
        :allow-multiple="false"
        :method-label="t('admin.accounts.inputMethod')"
        :platform="isOpenAI ? 'openai' : isGemini ? 'gemini' : isAntigravity ? 'antigravity' : 'anthropic'"
        :show-project-id="isGemini"
        :show-project-id-recovery="isGemini"
        :show-gemini-project-bootstrap-tip="isGemini && geminiOAuthType === 'code_assist'"
        @generate-url="handleGenerateUrl"
        @cookie-auth="handleCookieAuth"
      />

    </div>

    <template #footer>
      <div v-if="account" class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="isManualInputMethod"
          type="button"
          :disabled="!canExchangeCode"
          class="btn btn-primary"
          @click="handleExchangeCode"
        >
          <svg
            v-if="currentLoading"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
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
          {{
            currentLoading
              ? t('admin.accounts.oauth.verifying')
              : t('admin.accounts.oauth.completeAuth')
          }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import {
  useAccountOAuth,
  type AddMethod,
  type AuthInputMethod
} from '@/composables/useAccountOAuth'
import { useOpenAIOAuth } from '@/composables/useOpenAIOAuth'
import { useGeminiOAuth } from '@/composables/useGeminiOAuth'
import { useAntigravityOAuth } from '@/composables/useAntigravityOAuth'
import type { Account } from '@/types'
import { stripStaleGeminiExtra } from '@/utils/geminiExtra'
import { inferGeminiOAuthType } from '@/utils/geminiOAuthType'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import OAuthAuthorizationFlow from './OAuthAuthorizationFlow.vue'

// Type for exposed OAuthAuthorizationFlow component
// Note: defineExpose automatically unwraps refs, so we use the unwrapped types
interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  requiresProjectIdRecovery: boolean
  sessionKey: string
  inputMethod: AuthInputMethod
  reset: () => void
}

interface Props {
  show: boolean
  account: Account | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  reauthorized: []
}>()

const appStore = useAppStore()
const { t } = useI18n()

// OAuth composables
const claudeOAuth = useAccountOAuth()
const openaiOAuth = useOpenAIOAuth()
const geminiOAuth = useGeminiOAuth()
const antigravityOAuth = useAntigravityOAuth()

// Refs
const oauthFlowRef = ref<OAuthFlowExposed | null>(null)

// State
const addMethod = ref<AddMethod>('oauth')
const geminiOAuthType = ref<'code_assist' | 'google_one' | ''>('')

// Computed - check platform
const isOpenAI = computed(() => props.account?.platform === 'openai')
const isOpenAILike = computed(() => isOpenAI.value)
const isGemini = computed(() => props.account?.platform === 'gemini')
const isAnthropic = computed(() => props.account?.platform === 'anthropic')
const isAntigravity = computed(() => props.account?.platform === 'antigravity')

// Computed - current OAuth state based on platform
const currentAuthUrl = computed(() => {
  if (isOpenAILike.value) return openaiOAuth.authUrl.value
  if (isGemini.value) return geminiOAuth.authUrl.value
  if (isAntigravity.value) return antigravityOAuth.authUrl.value
  return claudeOAuth.authUrl.value
})
const currentSessionId = computed(() => {
  if (isOpenAILike.value) return openaiOAuth.sessionId.value
  if (isGemini.value) return geminiOAuth.sessionId.value
  if (isAntigravity.value) return antigravityOAuth.sessionId.value
  return claudeOAuth.sessionId.value
})
const currentLoading = computed(() => {
  if (isOpenAILike.value) return openaiOAuth.loading.value
  if (isGemini.value) return geminiOAuth.loading.value
  if (isAntigravity.value) return antigravityOAuth.loading.value
  return claudeOAuth.loading.value
})
const currentError = computed(() => {
  if (isOpenAILike.value) return openaiOAuth.error.value
  if (isGemini.value) return geminiOAuth.error.value
  if (isAntigravity.value) return antigravityOAuth.error.value
  return claudeOAuth.error.value
})
const currentErrorCode = computed(() => {
  if (isGemini.value) return geminiOAuth.errorCode.value
  return ''
})

// Computed
const isManualInputMethod = computed(() => {
  // OpenAI/Gemini/Antigravity always use manual input (no cookie auth option)
  return isOpenAILike.value || isGemini.value || isAntigravity.value || oauthFlowRef.value?.inputMethod === 'manual'
})

const canExchangeCode = computed(() => {
  const authCode = oauthFlowRef.value?.authCode || ''
  const sessionId = currentSessionId.value
  const loading = currentLoading.value
  return authCode.trim() && sessionId && !loading && !oauthFlowRef.value?.requiresProjectIdRecovery
})

// Watchers
watch(
  () => props.show,
  (newVal) => {
    if (newVal && props.account) {
      // Initialize addMethod based on current account type (Claude only)
      if (
        isAnthropic.value &&
        (props.account.type === 'oauth' || props.account.type === 'setup-token')
      ) {
        addMethod.value = props.account.type as AddMethod
      }
      if (isGemini.value) {
        geminiOAuthType.value = inferGeminiOAuthType(
          (props.account.credentials || {}) as Record<string, unknown>,
          (props.account.extra || {}) as Record<string, unknown>,
          ''
        )
      }
    } else {
      resetState()
    }
  },
  { immediate: true }
)

// Methods
const resetState = () => {
  addMethod.value = 'oauth'
  geminiOAuthType.value = ''
  claudeOAuth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  oauthFlowRef.value?.reset()
}

const handleClose = () => {
  emit('close')
}

const mergeRecord = (
  base?: Record<string, unknown> | null,
  patch?: Record<string, unknown> | null
): Record<string, unknown> => {
  return {
    ...(base || {}),
    ...(patch || {})
  }
}

const handleGenerateUrl = async () => {
  if (!props.account) return

  if (isOpenAILike.value) {
    await openaiOAuth.generateAuthUrl(props.account.proxy_id)
  } else if (isGemini.value) {
    if (geminiOAuthType.value !== 'code_assist' && geminiOAuthType.value !== 'google_one') {
      appStore.showError(t('admin.accounts.oauth.gemini.cannotInferOAuthType'))
      return
    }
    const projectId = oauthFlowRef.value?.projectId?.trim() || undefined
    await geminiOAuth.generateAuthUrl(props.account.proxy_id, projectId, geminiOAuthType.value)
  } else if (isAntigravity.value) {
    await antigravityOAuth.generateAuthUrl(props.account.proxy_id)
  } else {
    await claudeOAuth.generateAuthUrl(addMethod.value, props.account.proxy_id)
  }
}

watch(geminiOAuthType, (newType, oldType) => {
  if (newType === oldType) return
  geminiOAuth.resetState()
  if (oauthFlowRef.value) {
    oauthFlowRef.value.authCode = ''
    oauthFlowRef.value.oauthState = ''
  }
})

const handleExchangeCode = async () => {
  if (!props.account) return
  if (isGemini.value && geminiOAuthType.value !== 'code_assist' && geminiOAuthType.value !== 'google_one') {
    appStore.showError(t('admin.accounts.oauth.gemini.cannotInferOAuthType'))
    return
  }

  const authCode = oauthFlowRef.value?.authCode || ''
  if (!authCode.trim()) return

  if (isOpenAILike.value) {
    // OpenAI OAuth flow
    const oauthClient = openaiOAuth
    const sessionId = oauthClient.sessionId.value
    if (!sessionId) return
    const stateToUse = (oauthFlowRef.value?.oauthState || oauthClient.oauthState.value || '').trim()
    if (!stateToUse) {
      oauthClient.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(oauthClient.error.value)
      return
    }

    const tokenInfo = await oauthClient.exchangeAuthCode(
      authCode.trim(),
      sessionId,
      stateToUse,
      props.account.proxy_id
    )
    if (!tokenInfo) return

    // Build credentials and extra info
    const credentials = mergeRecord(
      (props.account.credentials || {}) as Record<string, unknown>,
      oauthClient.buildCredentials(tokenInfo)
    )
    const extra = mergeRecord(
      (props.account.extra || {}) as Record<string, unknown>,
      oauthClient.buildExtraInfo(tokenInfo)
    )
    const name = oauthClient.buildAccountName(tokenInfo, props.account.name)

    try {
      // Update account with new credentials
      await adminAPI.accounts.update(props.account.id, {
        name,
        type: 'oauth', // OpenAI OAuth is always 'oauth' type
        credentials,
        extra
      })

      // Clear error status after successful re-authorization
      await adminAPI.accounts.clearError(props.account.id)

      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized')
      handleClose()
    } catch (error: any) {
      oauthClient.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(oauthClient.error.value)
    }
  } else if (isGemini.value) {
    const sessionId = geminiOAuth.sessionId.value
    if (!sessionId) return

    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || geminiOAuth.state.value
    if (!stateToUse) return

    const tokenInfo = await geminiOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId,
      state: stateToUse,
      proxyId: props.account.proxy_id,
      oauthType: geminiOAuthType.value
    })
    if (!tokenInfo) return

    const credentials = mergeRecord(
      (props.account.credentials || {}) as Record<string, unknown>,
      geminiOAuth.buildCredentials(tokenInfo)
    )
    const extra = mergeRecord(
      stripStaleGeminiExtra((props.account.extra || {}) as Record<string, unknown>),
      geminiOAuth.buildExtraInfo(tokenInfo)
    )
    const name = geminiOAuth.buildAccountName(tokenInfo, props.account.name)

    try {
      await adminAPI.accounts.update(props.account.id, {
        name,
        type: 'oauth',
        credentials,
        extra
      })
      await adminAPI.accounts.clearError(props.account.id)
      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized')
      handleClose()
    } catch (error: any) {
      geminiOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(geminiOAuth.error.value)
    }
  } else if (isAntigravity.value) {
    // Antigravity OAuth flow
    const sessionId = antigravityOAuth.sessionId.value
    if (!sessionId) return

    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || antigravityOAuth.state.value
    if (!stateToUse) return

    const tokenInfo = await antigravityOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId,
      state: stateToUse,
      proxyId: props.account.proxy_id
    })
    if (!tokenInfo) return

    const credentials = mergeRecord(
      (props.account.credentials || {}) as Record<string, unknown>,
      antigravityOAuth.buildCredentials(tokenInfo)
    )
    const extra = mergeRecord(
      (props.account.extra || {}) as Record<string, unknown>,
      antigravityOAuth.buildExtraInfo(tokenInfo)
    )
    const name = antigravityOAuth.buildAccountName(tokenInfo, props.account.name)

    try {
      await adminAPI.accounts.update(props.account.id, {
        name,
        type: 'oauth',
        credentials,
        extra
      })
      await adminAPI.accounts.clearError(props.account.id)
      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized')
      handleClose()
    } catch (error: any) {
      antigravityOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(antigravityOAuth.error.value)
    }
  } else {
    // Claude OAuth flow
    const sessionId = claudeOAuth.sessionId.value
    if (!sessionId) return

    claudeOAuth.loading.value = true
    claudeOAuth.error.value = ''

    try {
      const stateToUse = (oauthFlowRef.value?.oauthState || claudeOAuth.oauthState.value || '').trim()
      if (!stateToUse) {
        claudeOAuth.error.value = t('admin.accounts.oauth.authFailed')
        appStore.showError(claudeOAuth.error.value)
        return
      }
      const proxyConfig = props.account.proxy_id ? { proxy_id: props.account.proxy_id } : {}
      const endpoint =
        addMethod.value === 'oauth'
          ? '/admin/accounts/exchange-code'
          : '/admin/accounts/exchange-setup-token-code'

      const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
        session_id: sessionId,
        code: authCode.trim(),
        state: stateToUse,
        ...proxyConfig
      })

      const credentials = mergeRecord(
        (props.account.credentials || {}) as Record<string, unknown>,
        tokenInfo as Record<string, unknown>
      )
      const extra = mergeRecord(
        (props.account.extra || {}) as Record<string, unknown>,
        claudeOAuth.buildExtraInfo(tokenInfo)
      )
      const name = claudeOAuth.buildAccountName(tokenInfo, props.account.name)

      // Update account with new credentials and type
      await adminAPI.accounts.update(props.account.id, {
        name,
        type: addMethod.value, // Update type based on selected method
        credentials,
        extra
      })

      // Clear error status after successful re-authorization
      await adminAPI.accounts.clearError(props.account.id)

      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized')
      handleClose()
    } catch (error: any) {
      claudeOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(claudeOAuth.error.value)
    } finally {
      claudeOAuth.loading.value = false
    }
  }
}

const handleCookieAuth = async (sessionKey: string) => {
  if (!props.account || isOpenAILike.value) return

  claudeOAuth.loading.value = true
  claudeOAuth.error.value = ''

  try {
    const proxyConfig = props.account.proxy_id ? { proxy_id: props.account.proxy_id } : {}
    const endpoint =
      addMethod.value === 'oauth'
        ? '/admin/accounts/cookie-auth'
        : '/admin/accounts/setup-token-cookie-auth'

    const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
      session_id: '',
      code: sessionKey.trim(),
      ...proxyConfig
    })

    const credentials = mergeRecord(
      (props.account.credentials || {}) as Record<string, unknown>,
      tokenInfo as Record<string, unknown>
    )
    const extra = mergeRecord(
      (props.account.extra || {}) as Record<string, unknown>,
      claudeOAuth.buildExtraInfo(tokenInfo)
    )
    const name = claudeOAuth.buildAccountName(tokenInfo, props.account.name)

    // Update account with new credentials and type
    await adminAPI.accounts.update(props.account.id, {
      name,
      type: addMethod.value, // Update type based on selected method
      credentials,
      extra
    })

    // Clear error status after successful re-authorization
    await adminAPI.accounts.clearError(props.account.id)

    appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
    emit('reauthorized')
    handleClose()
  } catch (error: any) {
    claudeOAuth.error.value =
      error.response?.data?.detail || t('admin.accounts.oauth.cookieAuthFailed')
  } finally {
    claudeOAuth.loading.value = false
  }
}
</script>
