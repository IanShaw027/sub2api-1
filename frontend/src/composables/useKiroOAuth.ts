import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { KiroAccountExtra, KiroCredentials } from '@/types'
import type {
  KiroAuthUrlRequest,
  KiroExchangeCallbackRequest,
  KiroExternalIDPAuthorizationInfo,
  KiroIDCContinuationInfo,
  KiroOAuthProgressResult,
  KiroTokenInfo
} from '@/api/admin/kiro'
import { isKiroContinuationResponse } from '@/api/admin/kiro'
import { formatOAuthAccountName } from '@/utils/oauthAccountName'

const KIRO_RUNTIME_EXTRA_KEYS = ['kiro_version', 'kiro_commit', 'system_version', 'node_version'] as const

const KIRO_DEVICE_POLL_FALLBACK_INTERVAL_MS = 5000
const KIRO_DEVICE_POLL_MIN_INTERVAL_MS = 2000
const KIRO_DEVICE_POLL_MAX_INTERVAL_MS = 30000

export const stripKiroRuntimeExtra = (
  extra?: Record<string, unknown> | null
): Record<string, unknown> => {
  const cleaned = { ...(extra || {}) }
  for (const key of KIRO_RUNTIME_EXTRA_KEYS) {
    delete cleaned[key]
  }
  return cleaned
}

const sleep = (ms: number) =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, ms)
  })

const clampPollInterval = (seconds?: number | null): number => {
  if (!seconds || seconds <= 0) return KIRO_DEVICE_POLL_FALLBACK_INTERVAL_MS
  const ms = seconds * 1000
  if (ms < KIRO_DEVICE_POLL_MIN_INTERVAL_MS) return KIRO_DEVICE_POLL_MIN_INTERVAL_MS
  if (ms > KIRO_DEVICE_POLL_MAX_INTERVAL_MS) return KIRO_DEVICE_POLL_MAX_INTERVAL_MS
  return ms
}

const extractKiroProfileLabel = (tokenInfo: KiroTokenInfo): string => {
  const profileID = tokenInfo.profile_id?.trim() || ''
  if (profileID) return profileID

  const userID = tokenInfo.user_id?.trim() || ''
  if (!userID) return ''
  const match = userID.match(/profile\/([^/]+)$/i)
  return match?.[1]?.trim() || ''
}

export function useKiroOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const sessionId = ref('')
  const callbackBaseUrl = ref('')
  const loading = ref(false)
  const error = ref('')
  const continuation = ref<KiroIDCContinuationInfo | null>(null)
  const externalIDPAuthorization = ref<KiroExternalIDPAuthorizationInfo | null>(null)
  const cancelToken = ref(0)

  const resetState = () => {
    authUrl.value = ''
    sessionId.value = ''
    callbackBaseUrl.value = ''
    loading.value = false
    error.value = ''
    continuation.value = null
    externalIDPAuthorization.value = null
    cancelToken.value += 1
  }

  const cancelDeviceAuthorization = () => {
    cancelToken.value += 1
    continuation.value = null
    externalIDPAuthorization.value = null
    loading.value = false
  }

  const generateAuthUrl = async (proxyId?: number | null): Promise<boolean> => {
    loading.value = true
    authUrl.value = ''
    sessionId.value = ''
    callbackBaseUrl.value = ''
    error.value = ''
    continuation.value = null
    externalIDPAuthorization.value = null

    try {
      const payload: KiroAuthUrlRequest = {}
      if (proxyId) payload.proxy_id = proxyId

      const response = await adminAPI.kiro.generateAuthUrl(payload)
      authUrl.value = response.auth_url
      sessionId.value = response.session_id
      callbackBaseUrl.value = response.callback_url
      return true
    } catch (err: any) {
      error.value = err?.response?.data?.detail || err?.message || t('admin.accounts.oauth.failedToGenerateUrl')
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const pollDeviceAuthorization = async (
    info: KiroIDCContinuationInfo,
    proxyId?: number | null
  ): Promise<KiroTokenInfo | null> => {
    const pollSessionId = info.session_id || sessionId.value
    if (!pollSessionId) {
      error.value = t('admin.accounts.kiro.idcSessionMissing')
      return null
    }
    const myToken = cancelToken.value
    let intervalMs = clampPollInterval(info.interval_seconds)
    const expiresAt = info.expires_at ? Date.parse(info.expires_at) : NaN
    while (cancelToken.value === myToken) {
      await sleep(intervalMs)
      if (cancelToken.value !== myToken) return null
      if (!Number.isNaN(expiresAt) && Date.now() > expiresAt) {
        error.value = t('admin.accounts.kiro.idcDeviceExpired')
        continuation.value = null
        return null
      }
      try {
        const result = await adminAPI.kiro.deviceComplete({
          session_id: pollSessionId,
          ...(proxyId ? { proxy_id: proxyId } : {})
        })
        if (cancelToken.value !== myToken) return null
        if (result.token_info && result.token_info.access_token) {
          continuation.value = null
          return result.token_info
        }
        if (result.continuation) {
          continuation.value = result.continuation
          intervalMs = clampPollInterval(result.continuation.interval_seconds)
          continue
        }
        error.value = t('admin.accounts.kiro.idcDeviceUnexpected')
        return null
      } catch (err: any) {
        error.value =
          err?.response?.data?.detail || err?.message || t('admin.accounts.kiro.idcDeviceFailed')
        appStore.showError(error.value)
        continuation.value = null
        return null
      }
    }
    return null
  }

  const exchangeCallback = async (
    callbackUrl: string,
    proxyId?: number | null
  ): Promise<KiroTokenInfo | null> => {
    const trimmedCallback = callbackUrl.trim()
    if (!trimmedCallback || !sessionId.value) {
      error.value = t('admin.accounts.kiro.callbackUrlRequired')
      return null
    }

    cancelToken.value += 1
    loading.value = true
    error.value = ''
    continuation.value = null
    externalIDPAuthorization.value = null

    try {
      const payload: KiroExchangeCallbackRequest = {
        session_id: sessionId.value,
        callback_url: trimmedCallback
      }
      if (proxyId) payload.proxy_id = proxyId
      const response = await adminAPI.kiro.exchangeCallback(payload)
      if (isKiroContinuationResponse(response)) {
        const progress = response as KiroOAuthProgressResult
        if (progress.token_info && progress.token_info.access_token) {
          return progress.token_info
        }
        if (progress.continuation) {
          continuation.value = progress.continuation
          return await pollDeviceAuthorization(progress.continuation, proxyId)
        }
        if (progress.external_idp) {
          externalIDPAuthorization.value = progress.external_idp
          if (typeof window !== 'undefined' && progress.external_idp.auth_url) {
            window.open(progress.external_idp.auth_url, '_blank', 'noopener')
          }
          return null
        }
        error.value = t('admin.accounts.kiro.idcDeviceUnexpected')
        return null
      }
      return response as KiroTokenInfo
    } catch (err: any) {
      error.value = err?.response?.data?.detail || err?.message || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const validateRefreshToken = async (
    credentials: Partial<KiroCredentials> & Record<string, unknown>,
    extra?: KiroAccountExtra & Record<string, unknown>,
    proxyId?: number | null
  ): Promise<Record<string, unknown> | null> => {
    const refreshToken = String(credentials.refresh_token || '').trim()
    if (!refreshToken) {
      error.value = t('admin.accounts.kiro.refreshTokenRequired')
      return null
    }

    loading.value = true
    error.value = ''

    try {
      const payloadCredentials: Record<string, unknown> = {
        ...credentials,
        refresh_token: refreshToken
      }
      return await adminAPI.kiro.refreshToken({
        credentials: payloadCredentials,
        extra,
        proxy_id: proxyId || undefined
      })
    } catch (err: any) {
      error.value = err?.response?.data?.detail || err?.message || t('admin.accounts.kiro.failedToValidateRT')
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (
    tokenInfo: KiroTokenInfo,
    overrides?: Partial<KiroCredentials> & Record<string, unknown>
  ): Record<string, unknown> => {
    const credentials: Record<string, unknown> = {
      access_token: tokenInfo.access_token,
      refresh_token: tokenInfo.refresh_token,
      expires_at: tokenInfo.expires_at,
      auth_method: tokenInfo.auth_method || 'social',
      region: tokenInfo.region || 'us-east-1'
    }

    if (tokenInfo.client_id) credentials.client_id = tokenInfo.client_id
    if (tokenInfo.client_secret) credentials.client_secret = tokenInfo.client_secret
    if (tokenInfo.token_endpoint) credentials.token_endpoint = tokenInfo.token_endpoint
    if (tokenInfo.auth_region) credentials.auth_region = tokenInfo.auth_region
    if (tokenInfo.api_region) credentials.api_region = tokenInfo.api_region
    if (tokenInfo.profile_arn) credentials.profile_arn = tokenInfo.profile_arn
    if (tokenInfo.profile_id) credentials.profile_id = tokenInfo.profile_id
    if (tokenInfo.user_id) credentials.user_id = tokenInfo.user_id
    if (tokenInfo.email) credentials.email = tokenInfo.email
    if (tokenInfo.login_provider) credentials.login_provider = tokenInfo.login_provider
    if (tokenInfo.plan_name) credentials.plan_name = tokenInfo.plan_name
    if (tokenInfo.plan_tier) credentials.plan_tier = tokenInfo.plan_tier
    if (tokenInfo.subscription_type) credentials.subscription_type = tokenInfo.subscription_type
    if (tokenInfo.usage_reset_at) credentials.usage_reset_at = tokenInfo.usage_reset_at
    if (tokenInfo.status) credentials.status = tokenInfo.status
    if (tokenInfo.status_reason) credentials.status_reason = tokenInfo.status_reason
    if (tokenInfo.issuer_url) credentials.issuer_url = tokenInfo.issuer_url
    if (tokenInfo.idc_region) credentials.idc_region = tokenInfo.idc_region
    if (tokenInfo.scopes) credentials.scopes = tokenInfo.scopes
    if (tokenInfo.login_hint) credentials.login_hint = tokenInfo.login_hint

    if (overrides?.machine_id) credentials.machine_id = overrides.machine_id
    if (overrides?.region) credentials.region = overrides.region
    if (overrides?.auth_region) credentials.auth_region = overrides.auth_region
    if (overrides?.api_region) credentials.api_region = overrides.api_region
    if (overrides?.profile_arn) credentials.profile_arn = overrides.profile_arn

    return credentials
  }

  const buildExtraInfo = (
    tokenInfo: KiroTokenInfo,
    overrides?: KiroAccountExtra & Record<string, unknown>
  ): Record<string, unknown> => {
    const extra: Record<string, unknown> = stripKiroRuntimeExtra(overrides)

    if (tokenInfo.email) extra.email = tokenInfo.email
    if (tokenInfo.profile_id) extra.profile_id = tokenInfo.profile_id
    if (tokenInfo.name) extra.name = tokenInfo.name
    if (tokenInfo.login_provider) extra.login_provider = tokenInfo.login_provider
    if (tokenInfo.subscription_type || tokenInfo.plan_name) {
      extra.subscription_type = tokenInfo.subscription_type || tokenInfo.plan_name
    }
    if (tokenInfo.plan_tier) extra.subscription_tier = tokenInfo.plan_tier
    if (tokenInfo.usage_reset_at) extra.usage_reset_at = tokenInfo.usage_reset_at
    if (tokenInfo.status) extra.kiro_status = tokenInfo.status
    if (tokenInfo.status_reason) extra.kiro_status_reason = tokenInfo.status_reason

    return extra
  }

  const buildAccountName = (tokenInfo: KiroTokenInfo, fallbackName?: string): string => {
    const subscriptionLabel = (
      tokenInfo.subscription_type?.trim() ||
      tokenInfo.plan_name?.trim() ||
      tokenInfo.plan_tier?.trim()
    )
    const profileLabel = extractKiroProfileLabel(tokenInfo)

    return formatOAuthAccountName({
      manualName: fallbackName,
      primary: tokenInfo.email || tokenInfo.name || profileLabel,
      details: [],
      platformLabel: 'Kiro',
      fallbackDetail: subscriptionLabel,
      defaultName: 'Kiro OAuth Account'
    })
  }

  return {
    authUrl,
    sessionId,
    callbackBaseUrl,
    loading,
    error,
    continuation,
    externalIDPAuthorization,
    resetState,
    cancelDeviceAuthorization,
    generateAuthUrl,
    exchangeCallback,
    validateRefreshToken,
    buildCredentials,
    buildExtraInfo,
    buildAccountName
  }
}
