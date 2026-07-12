<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="normal"
    @close="handleClose"
  >
    <div v-if="account" class="space-y-4">
      <!-- Account Info -->
      <div
        class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br',
              isOpenAILike
                ? 'from-green-500 to-green-600'
                : isGemini
                  ? 'from-blue-500 to-blue-600'
                  : isAntigravity
                    ? 'from-purple-500 to-purple-600'
                    : isKiro
                      ? 'from-cyan-500 to-sky-600'
                    : isGrok
                      ? 'from-zinc-700 to-zinc-900'
                      : 'from-orange-500 to-orange-600'
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
            <span class="block font-semibold text-gray-900 dark:text-white">{{
              account.name
            }}</span>
            <span class="text-sm text-gray-500 dark:text-gray-400">
              {{
                isOpenAI
                  ? t('admin.accounts.openaiAccount')
                  : isGemini
                    ? t('admin.accounts.geminiAccount')
                    : isAntigravity
                      ? t('admin.accounts.antigravityAccount')
                      : isKiro
                        ? t('admin.accounts.kiroAccount')
                      : isGrok
                        ? t('admin.accounts.grokAccount')
                        : t('admin.accounts.claudeCodeAccount')
              }}
            </span>
          </div>
        </div>
      </div>

      <div
        v-if="isKiroOAuth && kiroDiagnosticItems.length"
        class="rounded-lg border border-cyan-200 bg-cyan-50/60 p-4 dark:border-cyan-900/40 dark:bg-cyan-950/20"
      >
        <div class="mb-2 text-sm font-medium text-cyan-900 dark:text-cyan-100">
          {{ t('admin.accounts.kiro.diagnosticSummaryTitle') }}
        </div>
        <KiroDiagnosticChips
          :credentials="account.credentials || {}"
          :extra="account.extra || {}"
          :usage-info="{}"
          chip-class="inline-flex rounded bg-white/80 px-2 py-1 text-cyan-800 dark:bg-black/10 dark:text-cyan-200"
        />
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
              class="mr-2 text-primary-600 focus:ring-primary-500"
            />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{
              t('admin.accounts.types.oauth')
            }}</span>
          </label>
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="setup-token"
              class="mr-2 text-primary-600 focus:ring-primary-500"
            />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{
              t('admin.accounts.setupTokenLongLived')
            }}</span>
          </label>
        </div>
      </fieldset>

      <!-- Gemini OAuth Type Display (read-only) -->
      <div v-if="isGemini" class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700">
        <div class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.accounts.oauth.gemini.oauthTypeLabel') }}
        </div>
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
              geminiOAuthType === 'google_one'
                ? 'bg-purple-500 text-white'
                : geminiOAuthType === 'code_assist'
                  ? 'bg-blue-500 text-white'
                  : 'bg-gray-400 text-white'
            ]"
          >
            <Icon v-if="geminiOAuthType === 'google_one'" name="user" size="sm" />
            <Icon v-else-if="geminiOAuthType === 'code_assist'" name="cloud" size="sm" />
            <Icon v-else name="shield" size="sm" />
          </div>
          <div>
            <span class="block text-sm font-medium text-gray-900 dark:text-white">
              {{
                geminiOAuthType === 'google_one'
                  ? t('admin.accounts.oauth.gemini.googleOneTitle')
                  : geminiOAuthType === 'code_assist'
                    ? t('admin.accounts.gemini.oauthType.builtInTitle')
                    : t('common.unknown')
              }}
            </span>
            <span class="text-xs text-gray-500 dark:text-gray-400">
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

      <KiroAuthorizationFlow
        v-if="isKiroOAuth"
        mode="reauth"
        :account-id="account?.id || null"
        :proxy-id="account?.proxy_id || null"
        :auth-url="kiroOAuth.authUrl.value"
        :callback-base-url="kiroOAuth.callbackBaseUrl.value"
        :loading="kiroReauthLoading"
        :error="kiroOAuth.error.value"
        :initial-credentials="kiroCredentials"
        :initial-extra="kiroExtra"
        :continuation="kiroOAuth.continuation.value"
        :external-i-d-p-authorization="kiroOAuth.externalIDPAuthorization.value"
        @generate-url="handleGenerateUrl"
        @submit="handleKiroReauthorize"
        @submit-refresh-token="handleKiroValidateRT"
        @cancel-continuation="kiroOAuth.cancelDeviceAuthorization"
      />

      <OAuthAuthorizationFlow
        v-else-if="!isKiro"
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
        :show-refresh-token-option="isGrok"
        :show-sso-token-option="isGrok"
        :show-email-password-option="isGrok"
        :allow-multiple="false"
        :method-label="t('admin.accounts.inputMethod')"
        :platform="isOpenAI ? 'openai' : isGemini ? 'gemini' : isAntigravity ? 'antigravity' : isGrok ? 'grok' : 'anthropic'"
        :show-project-id="isGemini"
        :show-project-id-recovery="isGemini"
        :show-gemini-project-bootstrap-tip="isGemini && geminiOAuthType === 'code_assist'"
        @generate-url="handleGenerateUrl"
        @cookie-auth="handleCookieAuth"
        @validate-refresh-token="handleValidateRefreshToken"
        @validate-sso-token="handleValidateSSOToken"
        @authorize-password="handleAuthorizePassword"
      />

    </div>

    <template #footer>
      <div v-if="account" class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <div v-if="isKiro" class="flex gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            @click="handleOpenEditor"
          >
            {{ t('admin.accounts.kiro.openEditorAction') }}
          </button>
        </div>
        <button
          v-if="!isKiro && isManualInputMethod"
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
import { stripKiroRuntimeExtra, useKiroOAuth } from '@/composables/useKiroOAuth'
import type { KiroTokenInfo } from '@/api/admin/kiro'
import type { Account, KiroAccountExtra, KiroCredentials } from '@/types'
import { stripStaleGeminiExtra } from '@/utils/geminiExtra'
import { inferGeminiOAuthType } from '@/utils/geminiOAuthType'
import { useGrokOAuth } from '@/composables/useGrokOAuth'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import OAuthAuthorizationFlow from '@/components/account/OAuthAuthorizationFlow.vue'
import KiroDiagnosticChips from '@/components/account/KiroDiagnosticChips.vue'
import KiroAuthorizationFlow from '@/components/account/KiroAuthorizationFlow.vue'

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
  reauthorized: [account: Account]
  refresh: []
  openEditor: [account: Account]
}>()

const appStore = useAppStore()
const { t } = useI18n()

// OAuth composables
const claudeOAuth = useAccountOAuth()
const openaiOAuth = useOpenAIOAuth()
const geminiOAuth = useGeminiOAuth()
const antigravityOAuth = useAntigravityOAuth()
const kiroOAuth = useKiroOAuth()
const grokOAuth = useGrokOAuth()

// Refs
const oauthFlowRef = ref<OAuthFlowExposed | null>(null)

// State
const addMethod = ref<AddMethod>('oauth')
const geminiOAuthType = ref<'code_assist' | 'google_one' | ''>('')
const kiroBatchReauthLoading = ref(false)

// Computed - check platform
const isOpenAI = computed(() => props.account?.platform === 'openai')
const isOpenAILike = computed(() => isOpenAI.value)
const isGemini = computed(() => props.account?.platform === 'gemini')
const isAnthropic = computed(() => props.account?.platform === 'anthropic')
const isAntigravity = computed(() => props.account?.platform === 'antigravity')
const isKiro = computed(() => props.account?.platform === 'kiro')
const isKiroOAuth = computed(() => props.account?.platform === 'kiro' && props.account?.type === 'oauth')
const kiroCredentials = computed((): KiroCredentials & Record<string, unknown> => {
  if (!isKiroOAuth.value) {
    return {}
  }
  return props.account?.credentials || {}
})
const kiroExtra = computed((): KiroAccountExtra & Record<string, unknown> => {
  if (!isKiroOAuth.value) {
    return {}
  }
  return props.account?.extra || {}
})
const kiroDiagnosticItems = computed(() => {
  if (!isKiroOAuth.value) return []
  const credentials = (props.account?.credentials || {}) as Record<string, unknown>
  const extra = (props.account?.extra || {}) as Record<string, unknown>
  const hasProfileArn = typeof credentials.profile_arn === 'string' && credentials.profile_arn.trim() !== ''
  const hasProfileID = typeof credentials.profile_id === 'string' && credentials.profile_id.trim() !== ''
    || typeof extra.profile_id === 'string' && extra.profile_id.trim() !== ''
  const hasLoginProvider = typeof credentials.login_provider === 'string' && credentials.login_provider.trim() !== ''
    || typeof extra.login_provider === 'string' && extra.login_provider.trim() !== ''
  const hasStatusReason = typeof credentials.status_reason === 'string' && credentials.status_reason.trim() !== ''
    || typeof credentials.kiro_status_reason === 'string' && credentials.kiro_status_reason.trim() !== ''
    || typeof extra.kiro_status_reason === 'string' && extra.kiro_status_reason.trim() !== ''
  return hasProfileArn || hasProfileID || hasLoginProvider || hasStatusReason ? [1] : []
})
const dialogTitle = computed(() => t('admin.accounts.reAuthorizeAccount'))
const isGrok = computed(() => props.account?.platform === 'grok')

// Computed - current OAuth state based on platform
const currentAuthUrl = computed(() => {
  if (isKiro.value) return ''
  if (isOpenAILike.value) return openaiOAuth.authUrl.value
  if (isGemini.value) return geminiOAuth.authUrl.value
  if (isAntigravity.value) return antigravityOAuth.authUrl.value
  if (isGrok.value) return grokOAuth.authUrl.value
  return claudeOAuth.authUrl.value
})
const currentSessionId = computed(() => {
  if (isKiro.value) return ''
  if (isOpenAILike.value) return openaiOAuth.sessionId.value
  if (isGemini.value) return geminiOAuth.sessionId.value
  if (isAntigravity.value) return antigravityOAuth.sessionId.value
  if (isGrok.value) return grokOAuth.sessionId.value
  return claudeOAuth.sessionId.value
})
const currentLoading = computed(() => {
  if (isKiro.value) return false
  if (isOpenAILike.value) return openaiOAuth.loading.value
  if (isGemini.value) return geminiOAuth.loading.value
  if (isAntigravity.value) return antigravityOAuth.loading.value
  if (isGrok.value) return grokOAuth.loading.value
  return claudeOAuth.loading.value
})
const kiroReauthLoading = computed(() => kiroOAuth.loading.value || kiroBatchReauthLoading.value)
const currentError = computed(() => {
  if (isKiro.value) return ''
  if (isOpenAILike.value) return openaiOAuth.error.value
  if (isGemini.value) return geminiOAuth.error.value
  if (isAntigravity.value) return antigravityOAuth.error.value
  if (isGrok.value) return grokOAuth.error.value
  return claudeOAuth.error.value
})
const currentErrorCode = computed(() => {
  if (isGemini.value) return geminiOAuth.errorCode.value
  return ''
})

// Computed
const isManualInputMethod = computed(() => {
  // OpenAI/Gemini/Antigravity always use manual input (no cookie auth option)
  return isOpenAILike.value || isGemini.value || isAntigravity.value || isGrok.value || oauthFlowRef.value?.inputMethod === 'manual'
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

watch(geminiOAuthType, (newType, oldType) => {
  if (newType === oldType) return
  geminiOAuth.resetState()
  if (oauthFlowRef.value) {
    oauthFlowRef.value.authCode = ''
    oauthFlowRef.value.oauthState = ''
  }
})

// Methods
const resetState = () => {
  addMethod.value = 'oauth'
  geminiOAuthType.value = ''
  claudeOAuth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  kiroOAuth.resetState()
  grokOAuth.resetState()
  oauthFlowRef.value?.reset()
}

const handleClose = () => {
  emit('close')
}

const handleOpenEditor = () => {
  if (!props.account) return
  emit('openEditor', props.account)
  emit('close')
}

const mergeRecord = (
  ...sources: Array<Record<string, unknown> | null | undefined>
): Record<string, unknown> => {
  return sources.reduce<Record<string, unknown>>((merged, source) => {
    return {
      ...merged,
      ...(source || {})
    }
  }, {})
}

const stripEmptyRecordValues = (
  record?: Record<string, unknown> | null
): Record<string, unknown> => {
  return Object.fromEntries(
    Object.entries(record || {}).filter(([, value]) => {
      if (value === null || value === undefined) {
        return false
      }
      return typeof value !== 'string' || value.trim() !== ''
    })
  )
}

const emitKiroBatchRefresh = (refreshTokenCount: number, successCount: number) => {
  if (refreshTokenCount > 1 && successCount > 1) {
    emit('refresh')
  }
}

const sanitizeKiroCredentialsForAuthMethod = (
  credentials: Record<string, unknown>
): Record<string, unknown> => {
  const sanitized = { ...credentials }
  if (sanitized.auth_method === 'external_idp') {
    delete sanitized.client_secret
    delete sanitized.idc_region
    return sanitized
  }
  if (sanitized.auth_method !== 'idc') {
    for (const key of ['client_id', 'client_secret', 'issuer_url', 'idc_region', 'scopes', 'login_hint', 'token_endpoint']) {
      delete sanitized[key]
    }
  }
  return sanitized
}

const finishKiroReauthorization = async (
  name: string,
  credentials: Record<string, unknown>,
  extra: Record<string, unknown>
) => {
  if (!props.account) return

  try {
    await adminAPI.accounts.reauthorizeKiroOAuth(props.account.id, {
      name,
      credentials,
      extra
    })
    const updatedAccount = await adminAPI.accounts.clearError(props.account.id)
    appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
    emit('reauthorized', updatedAccount)
    handleClose()
  } catch (error: any) {
    const message =
      error?.response?.data?.detail ||
      error?.message ||
      t('admin.accounts.oauth.authFailed')
    appStore.showError(message)
  }
}

const finishGrokReauthorization = async (
  credentials: Record<string, unknown>,
  extra: Record<string, unknown>
) => {
  if (!props.account) return

  try {
    const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
      type: 'oauth',
      credentials,
      extra
    })
    appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
    emit('reauthorized', updatedAccount)
    handleClose()
  } catch (error: any) {
    const message =
      error?.response?.data?.detail ||
      error?.message ||
      t('admin.accounts.oauth.authFailed')
    grokOAuth.error.value = message
    appStore.showError(message)
  }
}

const handleGrokManualTokenInfo = async (tokenInfo: Record<string, unknown> | null) => {
  if (!props.account || !tokenInfo) return
  const credentials = mergeRecord(
    (props.account.credentials || {}) as Record<string, unknown>,
    grokOAuth.buildCredentials(tokenInfo)
  )
  const extra = mergeRecord(
    (props.account.extra || {}) as Record<string, unknown>,
    grokOAuth.buildExtraInfo(tokenInfo)
  )
  await finishGrokReauthorization(credentials, extra)
}

const handleKiroReauthorize = async (payload: {
  callbackUrl: string
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
}) => {
  if (!props.account || !isKiroOAuth.value) return

  const tokenInfo = await kiroOAuth.exchangeCallback(payload.callbackUrl, props.account.proxy_id)
  if (!tokenInfo) {
    return
  }
  const credentials = sanitizeKiroCredentialsForAuthMethod(mergeRecord(
    (props.account.credentials || {}) as Record<string, unknown>,
    kiroOAuth.buildCredentials(tokenInfo, payload.credentials)
  ))
  const extra = mergeRecord(
    (props.account.extra || {}) as Record<string, unknown>,
    kiroOAuth.buildExtraInfo(tokenInfo, payload.extra)
  )
  const sanitizedExtra = stripKiroRuntimeExtra(extra)
  const name = kiroOAuth.buildAccountName(tokenInfo, props.account.name)

  await finishKiroReauthorization(name, credentials, sanitizedExtra)
}

const handleKiroValidateRT = async (payload: {
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
}) => {
  if (!props.account || !isKiroOAuth.value) return
  if (kiroBatchReauthLoading.value) return

  const refreshTokens = String(payload.credentials.refresh_token || '')
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    kiroOAuth.error.value = t('admin.accounts.kiro.refreshTokenRequired')
    return
  }

  kiroBatchReauthLoading.value = true
  kiroOAuth.loading.value = true
  kiroOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  let updatedAccount: Account | null = null
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const manualCredentials = {
          ...payload.credentials,
          refresh_token: refreshTokens[i]
        }
        const validatedCredentials = await kiroOAuth.validateRefreshToken(
          manualCredentials,
          payload.extra,
          props.account.proxy_id
        )
        if (!validatedCredentials) {
          failedCount++
          errors.push(`#${i + 1}: ${kiroOAuth.error.value || t('admin.accounts.kiro.failedToValidateRT')}`)
          kiroOAuth.error.value = ''
          continue
        }

        const tokenInfo = validatedCredentials as KiroTokenInfo
        const credentials = sanitizeKiroCredentialsForAuthMethod(stripEmptyRecordValues(mergeRecord(
          (props.account.credentials || {}) as Record<string, unknown>,
          manualCredentials,
          validatedCredentials
        )))
        const extra = stripKiroRuntimeExtra(stripEmptyRecordValues(mergeRecord(
          (props.account.extra || {}) as Record<string, unknown>,
          kiroOAuth.buildExtraInfo(tokenInfo, payload.extra)
        )))
        const baseName = kiroOAuth.buildAccountName(tokenInfo, props.account.name)
        const name = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        if (successCount === 0) {
          await adminAPI.accounts.reauthorizeKiroOAuth(props.account.id, {
            name,
            credentials,
            extra
          })
          updatedAccount = await adminAPI.accounts.clearError(props.account.id)
        } else {
          await adminAPI.accounts.create({
            name,
            notes: props.account.notes,
            platform: 'kiro',
            type: 'oauth',
            credentials,
            extra,
            proxy_id: props.account.proxy_id,
            concurrency: props.account.concurrency,
            load_factor: props.account.load_factor ?? undefined,
            priority: props.account.priority,
            rate_multiplier: props.account.rate_multiplier,
            group_ids: props.account.group_ids,
            expires_at: props.account.expires_at,
            auto_pause_on_expired: props.account.auto_pause_on_expired
          })
        }
        successCount++
      } catch (error: any) {
        failedCount++
        const message =
          error?.response?.data?.detail ||
          error?.message ||
          t('admin.accounts.oauth.authFailed')
        errors.push(`#${i + 1}: ${message}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.reAuthorizedSuccess')
      )
      if (updatedAccount) {
        emit('reauthorized', updatedAccount)
      }
      emitKiroBatchRefresh(refreshTokens.length, successCount)
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      kiroOAuth.error.value = errors.join('\n')
      if (updatedAccount) {
        emit('reauthorized', updatedAccount)
      }
      emitKiroBatchRefresh(refreshTokens.length, successCount)
    } else {
      kiroOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    kiroBatchReauthLoading.value = false
    kiroOAuth.loading.value = false
  }
}

const handleGenerateUrl = async () => {
  if (!props.account) return

  if (isOpenAILike.value) {
    await openaiOAuth.generateAuthUrl(props.account.proxy_id)
  } else if (isKiroOAuth.value) {
    await kiroOAuth.generateAuthUrl(props.account.proxy_id)
  } else if (isKiro.value) {
    return
  } else if (isGemini.value) {
    if (geminiOAuthType.value !== 'code_assist' && geminiOAuthType.value !== 'google_one') {
      appStore.showError(t('admin.accounts.oauth.gemini.cannotInferOAuthType'))
      return
    }
    const projectId = oauthFlowRef.value?.projectId?.trim() || undefined
    await geminiOAuth.generateAuthUrl(props.account.proxy_id, projectId, geminiOAuthType.value)
  } else if (isAntigravity.value) {
    await antigravityOAuth.generateAuthUrl(props.account.proxy_id)
  } else if (isGrok.value) {
    await grokOAuth.generateAuthUrl(props.account.proxy_id)
  } else {
    await claudeOAuth.generateAuthUrl(addMethod.value, props.account.proxy_id)
  }
}

const hasMultipleCredentialLines = (value: string): boolean => {
  return value.split(/\r?\n/).filter((line) => line.trim() !== '').length > 1
}

const handleValidateRefreshToken = async (refreshToken: string) => {
  if (!isGrok.value) return
  if (hasMultipleCredentialLines(refreshToken)) {
    appStore.showError(t('admin.accounts.oauth.grok.singleCredentialOnly'))
    return
  }
  const tokenInfo = await grokOAuth.validateRefreshToken(refreshToken, props.account?.proxy_id)
  await handleGrokManualTokenInfo(tokenInfo as Record<string, unknown> | null)
}

const handleValidateSSOToken = async (ssoToken: string) => {
  if (!isGrok.value) return
  if (hasMultipleCredentialLines(ssoToken)) {
    appStore.showError(t('admin.accounts.oauth.grok.singleCredentialOnly'))
    return
  }
  const tokenInfo = await grokOAuth.validateSSOToken(ssoToken, props.account?.proxy_id)
  await handleGrokManualTokenInfo(tokenInfo as Record<string, unknown> | null)
}

const handleAuthorizePassword = async (emailPasswordInput: string) => {
  if (!isGrok.value) return
  if (hasMultipleCredentialLines(emailPasswordInput)) {
    appStore.showError(t('admin.accounts.oauth.grok.singleCredentialOnly'))
    return
  }
  const tokenInfo = await grokOAuth.authorizePassword(emailPasswordInput, props.account?.proxy_id)
  await handleGrokManualTokenInfo(tokenInfo as Record<string, unknown> | null)
}

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
      const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
        name,
        type: 'oauth',
        credentials,
        extra
      })

      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized', updatedAccount)
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
      const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
        name,
        type: 'oauth',
        credentials,
        extra
      })
      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized', updatedAccount)
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
      const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
        name,
        type: 'oauth',
        credentials,
        extra
      })
      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized', updatedAccount)
      handleClose()
    } catch (error: any) {
      antigravityOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(antigravityOAuth.error.value)
    }
  } else if (isGrok.value) {
    const sessionId = grokOAuth.sessionId.value
    if (!sessionId) return

    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || grokOAuth.state.value
    if (!stateToUse) return

    const tokenInfo = await grokOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId,
      state: stateToUse,
      proxyId: props.account.proxy_id
    })
    if (!tokenInfo) return

    const credentials = grokOAuth.buildCredentials(tokenInfo)
    const extra = grokOAuth.buildExtraInfo(tokenInfo)

    try {
      const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
        type: 'oauth',
        credentials,
        extra
      })

      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized', updatedAccount)
      handleClose()
    } catch (error: any) {
      grokOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(grokOAuth.error.value)
    }
  } else {
    // Claude OAuth flow
    const sessionId = claudeOAuth.sessionId.value
    if (!sessionId) return

    claudeOAuth.loading.value = true
    claudeOAuth.error.value = ''

    try {
      const proxyConfig = props.account.proxy_id ? { proxy_id: props.account.proxy_id } : {}
      const endpoint =
        addMethod.value === 'oauth'
          ? '/admin/accounts/exchange-code'
          : '/admin/accounts/exchange-setup-token-code'

      const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
        session_id: sessionId,
        code: authCode.trim(),
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

      const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
        name,
        type: addMethod.value as 'oauth' | 'setup-token',
        credentials,
        extra
      })

      appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
      emit('reauthorized', updatedAccount)
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
  if (!props.account || isOpenAILike.value || isKiro.value) return

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

    const updatedAccount = await adminAPI.accounts.applyOAuthCredentials(props.account.id, {
      name,
      type: addMethod.value as 'oauth' | 'setup-token',
      credentials,
      extra
    })

    appStore.showSuccess(t('admin.accounts.reAuthorizedSuccess'))
    emit('reauthorized', updatedAccount)
    handleClose()
  } catch (error: any) {
    claudeOAuth.error.value =
      error.response?.data?.detail || t('admin.accounts.oauth.cookieAuthFailed')
  } finally {
    claudeOAuth.loading.value = false
  }
}
</script>
