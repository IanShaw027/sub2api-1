import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { formatOAuthAccountName } from '@/utils/oauthAccountName'
import type {
  GeminiAuthUrlRequest,
  GeminiExchangeCodeRequest
} from '@/api/admin/gemini'

export interface GeminiTokenInfo {
  access_token?: string
  refresh_token?: string
  id_token?: string
  token_type?: string
  scope?: string
  expires_at?: number | string
  project_id?: string
  email?: string
  auth_id?: string
  name?: string
  plan_name?: string
  oauth_type?: string
  tier_id?: string
  extra?: Record<string, unknown>
  [key: string]: unknown
}

export function useGeminiOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const sessionId = ref('')
  const state = ref('')
  const loading = ref(false)
  const error = ref('')
  const errorCode = ref('')

  const resetState = () => {
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    loading.value = false
    error.value = ''
    errorCode.value = ''
  }

  const isCodeAssistUserDefinedProjectError = (message: string): boolean => {
    return message.includes('user_defined_project_required:')
  }

  const isCodeAssistProjectIdError = (message: string): boolean => {
    return (
      message.includes('missing project_id') ||
      message.includes('missing project_id for Code Assist OAuth') ||
      message.includes('project_id required for Gemini Code Assist authorization') ||
      message.includes('Please provide Project ID manually') ||
      message.includes('failed to auto-detect project_id') ||
      message.includes('empty result')
    )
  }

  const isGoogleOneProjectDetectionError = (message: string): boolean => {
    return (
      message.includes('registered_tier_missing_companion_project:') ||
      message.includes('google One accounts require a project_id, failed to auto-detect:') ||
      message.includes('onboardUser completed but no project_id returned')
    )
  }

  const isGoogleOneUserDefinedProjectError = (message: string): boolean => {
    return message.includes('user_defined_project_required:')
  }

  const isIneligibleTierError = (message: string): boolean => {
    return message.toLowerCase().includes('ineligible_tier')
  }

  const extractIneligibleTierReasonCodes = (message: string): string[] => {
    const match = message.match(/ineligible_tier\[([A-Z_,]+)\]:/)
    if (!match || !match[1]) return []
    return match[1]
      .split(',')
      .map((code) => code.trim())
      .filter((code) => code)
  }

  const hasIneligibleTierReasonCode = (message: string, reasonCode: string): boolean => {
    return extractIneligibleTierReasonCodes(message).includes(reasonCode)
  }

  const isAgeVerificationError = (message: string): boolean => {
    const normalized = message.toLowerCase()
    return (
      hasIneligibleTierReasonCode(message, 'RESTRICTED_AGE') ||
      (
        isIneligibleTierError(normalized) &&
        (
          normalized.includes('18 years old or older') ||
          normalized.includes('verified your age')
        )
      )
    )
  }

  const generateAuthUrl = async (
    proxyId: number | null | undefined,
    projectId?: string | null,
    oauthType?: string
  ): Promise<boolean> => {
    loading.value = true
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    error.value = ''
    errorCode.value = ''

    try {
      const payload: GeminiAuthUrlRequest = {}
      if (proxyId) payload.proxy_id = proxyId
      const trimmedProjectID = projectId?.trim()
      if (trimmedProjectID) {
        payload.project_id_hint = trimmedProjectID
      }
      if (oauthType === 'code_assist' || oauthType === 'google_one') {
        payload.oauth_type = oauthType
      }

      const response = await adminAPI.gemini.generateAuthUrl(payload)
      authUrl.value = response.auth_url
      sessionId.value = response.session_id
      state.value = response.state
      return true
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.gemini.failedToGenerateUrl')
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const exchangeAuthCode = async (params: {
    code: string
    sessionId: string
    state: string
    proxyId?: number | null
    oauthType?: string
  }): Promise<GeminiTokenInfo | null> => {
    const code = params.code?.trim()
    if (!code || !params.sessionId || !params.state) {
      error.value = t('admin.accounts.oauth.gemini.missingExchangeParams')
      errorCode.value = 'missing_exchange_params'
      return null
    }

    loading.value = true
    error.value = ''
    errorCode.value = ''

    try {
      const payload: GeminiExchangeCodeRequest = {
        session_id: params.sessionId,
        state: params.state,
        code
      }
      if (params.proxyId) payload.proxy_id = params.proxyId
      if (
        params.oauthType === 'code_assist' ||
        params.oauthType === 'google_one'
      ) {
        payload.oauth_type = params.oauthType
      }

      const tokenInfo = await adminAPI.gemini.exchangeCode(payload)
      return tokenInfo as GeminiTokenInfo
    } catch (err: any) {
      const errorMessage =
        err?.response?.data?.message ||
        err?.response?.data?.detail ||
        err?.message ||
        ''
      if (params.oauthType === 'code_assist' && isCodeAssistUserDefinedProjectError(errorMessage)) {
        error.value = errorMessage || t('admin.accounts.oauth.gemini.failedToExchangeCode')
        errorCode.value = 'code_assist_user_defined_project_required'
      } else if (params.oauthType === 'code_assist' && isCodeAssistProjectIdError(errorMessage)) {
        error.value = t('admin.accounts.oauth.gemini.missingProjectId')
        errorCode.value = 'missing_project_id'
      } else if (params.oauthType === 'google_one' && isGoogleOneUserDefinedProjectError(errorMessage)) {
        error.value = t('admin.accounts.oauth.gemini.googleOneUserDefinedProjectRequired')
        errorCode.value = 'google_one_user_defined_project_required'
      } else if (params.oauthType === 'google_one' && isGoogleOneProjectDetectionError(errorMessage)) {
        error.value = t('admin.accounts.oauth.gemini.googleOneProjectDetectionFailed')
        errorCode.value = 'google_one_project_detection_failed'
      } else if (isAgeVerificationError(errorMessage)) {
        error.value = t('admin.accounts.oauth.gemini.codeAssistAgeVerificationRequired')
        errorCode.value = 'age_verification_required'
      } else if (isIneligibleTierError(errorMessage)) {
        error.value = t('admin.accounts.oauth.gemini.ineligibleTier')
        errorCode.value = 'ineligible_tier'
      } else {
        error.value = errorMessage || t('admin.accounts.oauth.gemini.failedToExchangeCode')
        errorCode.value = 'exchange_failed'
      }
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: GeminiTokenInfo): Record<string, unknown> => {
    let expiresAt: string | undefined
    if (typeof tokenInfo.expires_at === 'number' && Number.isFinite(tokenInfo.expires_at)) {
      expiresAt = Math.floor(tokenInfo.expires_at).toString()
    } else if (typeof tokenInfo.expires_at === 'string' && tokenInfo.expires_at.trim()) {
      expiresAt = tokenInfo.expires_at.trim()
    }

    return stripEmptyCredentialValues({
      access_token: tokenInfo.access_token,
      refresh_token: tokenInfo.refresh_token,
      id_token: tokenInfo.id_token,
      token_type: tokenInfo.token_type,
      expires_at: expiresAt,
      scope: tokenInfo.scope,
      project_id: tokenInfo.project_id,
      email: tokenInfo.email,
      auth_id: tokenInfo.auth_id,
      subject: tokenInfo.auth_id,
      name: tokenInfo.name,
      plan_name: tokenInfo.plan_name,
      oauth_type: tokenInfo.oauth_type,
      tier_id: tokenInfo.tier_id
    })
  }

  const stripEmptyCredentialValues = (credentials: Record<string, unknown>): Record<string, unknown> => {
    return Object.fromEntries(
      Object.entries(credentials).filter(([, value]) => {
        if (value === undefined || value === null) return false
        if (typeof value === 'string' && value.trim() === '') return false
        return true
      })
    )
  }

  const cleanNamePart = (value?: string | null): string => value?.trim() || ''

  const buildExtraInfo = (tokenInfo: GeminiTokenInfo): Record<string, unknown> | undefined => {
    const extra: Record<string, unknown> = {
      ...(tokenInfo.extra && typeof tokenInfo.extra === 'object' ? tokenInfo.extra : {})
    }
    if (tokenInfo.plan_name) {
      extra.plan_name = tokenInfo.plan_name
    }
    if (tokenInfo.email) {
      extra.email = tokenInfo.email
    }
    if (tokenInfo.name) {
      extra.name = tokenInfo.name
    }
    if (tokenInfo.auth_id) {
      extra.auth_id = tokenInfo.auth_id
    }
    if (tokenInfo.oauth_type) {
      extra.oauth_type = tokenInfo.oauth_type
    }
    return Object.keys(extra).length > 0 ? extra : undefined
  }

  const buildAccountName = (tokenInfo: GeminiTokenInfo, fallbackName?: string): string => {
    const manualName = cleanNamePart(fallbackName)
    if (manualName) return manualName

    const email = cleanNamePart(tokenInfo.email)
    const name = cleanNamePart(tokenInfo.name)
    const projectID = cleanNamePart(tokenInfo.project_id)

    if (email && projectID) return `${email}(${projectID})`
    if (email) return email
    if (name && projectID) return `${name}(${projectID})`

    const primary = name || projectID
    return formatOAuthAccountName({
      manualName: '',
      primary,
      details: [tokenInfo.plan_name, tokenInfo.tier_id, tokenInfo.oauth_type],
      platformLabel: 'Gemini',
      fallbackDetail: tokenInfo.plan_name || tokenInfo.tier_id || tokenInfo.oauth_type,
      defaultName: 'Gemini OAuth Account'
    })
  }

  return {
    authUrl,
    sessionId,
    state,
    loading,
    error,
    errorCode,
    resetState,
    generateAuthUrl,
    exchangeAuthCode,
    buildCredentials,
    buildExtraInfo,
    buildAccountName
  }
}
