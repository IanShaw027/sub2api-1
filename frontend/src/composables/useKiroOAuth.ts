import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { KiroAccountExtra, KiroCredentials } from '@/types'
import type { KiroTokenInfo } from '@/api/admin/kiro'

const KIRO_RUNTIME_EXTRA_KEYS = ['kiro_version', 'kiro_commit', 'system_version', 'node_version'] as const

export const stripKiroRuntimeExtra = (
  extra?: Record<string, unknown> | null
): Record<string, unknown> => {
  const cleaned = { ...(extra || {}) }
  for (const key of KIRO_RUNTIME_EXTRA_KEYS) {
    delete cleaned[key]
  }
  return cleaned
}

export function useKiroOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const sessionId = ref('')
  const callbackBaseUrl = ref('')
  const loading = ref(false)
  const error = ref('')

  const resetState = () => {
    authUrl.value = ''
    sessionId.value = ''
    callbackBaseUrl.value = ''
    loading.value = false
    error.value = ''
  }

  const generateAuthUrl = async (proxyId?: number | null): Promise<boolean> => {
    loading.value = true
    authUrl.value = ''
    sessionId.value = ''
    callbackBaseUrl.value = ''
    error.value = ''

    try {
      const payload: Record<string, unknown> = {}
      if (proxyId) payload.proxy_id = proxyId

      const response = await adminAPI.kiro.generateAuthUrl(payload as any)
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

  const exchangeCallback = async (
    callbackUrl: string,
    proxyId?: number | null
  ): Promise<KiroTokenInfo | null> => {
    const trimmedCallback = callbackUrl.trim()
    if (!trimmedCallback || !sessionId.value) {
      error.value = t('admin.accounts.kiro.callbackUrlRequired')
      return null
    }

    loading.value = true
    error.value = ''

    try {
      const payload: Record<string, unknown> = {
        session_id: sessionId.value,
        callback_url: trimmedCallback
      }
      if (proxyId) payload.proxy_id = proxyId
      return await adminAPI.kiro.exchangeCallback(payload as any)
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
    if (tokenInfo.auth_region) credentials.auth_region = tokenInfo.auth_region
    if (tokenInfo.api_region) credentials.api_region = tokenInfo.api_region
    if (tokenInfo.profile_arn) credentials.profile_arn = tokenInfo.profile_arn
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
    return (
      fallbackName?.trim() ||
      tokenInfo.name?.trim() ||
      tokenInfo.email?.trim() ||
      'Kiro OAuth Account'
    )
  }

  return {
    authUrl,
    sessionId,
    callbackBaseUrl,
    loading,
    error,
    resetState,
    generateAuthUrl,
    exchangeCallback,
    validateRefreshToken,
    buildCredentials,
    buildExtraInfo,
    buildAccountName
  }
}
