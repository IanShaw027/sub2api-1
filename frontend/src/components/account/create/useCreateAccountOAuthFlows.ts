// Extracted verbatim from CreateAccountModal.vue's <script setup> (formerly
// lines 3042-4421): the entire "Step 2" OAuth generate/validate/exchange/
// import handler chain used only by the create flow (editing an account
// never goes through this UI). Pure mechanical relocation — the composable
// receives the host's own refs/reactive objects/functions as `deps` and
// returns the exact same names so the host can destructure them back and
// the template keeps binding to identical objects/functions as before.
import type { ComputedRef, Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { buildModelMappingObject } from '@/composables/useModelWhitelist'
import { formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import { useAccountOAuth, type AddMethod, type AuthInputMethod } from '@/composables/useAccountOAuth'
import { useOpenAIOAuth } from '@/composables/useOpenAIOAuth'
import { useGeminiOAuth } from '@/composables/useGeminiOAuth'
import { useAntigravityOAuth } from '@/composables/useAntigravityOAuth'
import { useGrokOAuth } from '@/composables/useGrokOAuth'
import { useKiroOAuth } from '@/composables/useKiroOAuth'
import type { KiroTokenInfo } from '@/api/admin/kiro'
import type { AccountPlatform, AccountType, CreateAccountRequest } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  sessionKey: string
  refreshToken: string
  sessionToken: string
  codexSession: string
  codexPAT: string
  ssoCookie: string
  inputMethod: AuthInputMethod
  reset: () => void
}

export interface CreateAccountOAuthFlowDeps {
  form: any
  emit: (event: 'created') => void
  addMethod: Ref<AddMethod>
  allowedModels: Ref<string[]>
  antigravityModelMappings: Ref<ModelMapping[]>
  antigravityProjectId: Ref<string>
  apiKeyBaseUrl: Ref<string>
  autoPauseOnExpired: Ref<boolean>
  baseRpm: Ref<number | null>
  cacheTTLOverrideEnabled: Ref<boolean>
  cacheTTLOverrideTarget: Ref<string>
  customBaseUrl: Ref<string>
  customBaseUrlEnabled: Ref<boolean>
  editDailyResetHour: Ref<number | null>
  editDailyResetMode: Ref<'rolling' | 'fixed' | null>
  editQuotaDailyLimit: Ref<number | null>
  editQuotaLimit: Ref<number | null>
  editQuotaWeeklyLimit: Ref<number | null>
  editResetTimezone: Ref<string | null>
  editWeeklyResetDay: Ref<number | null>
  editWeeklyResetHour: Ref<number | null>
  editWeeklyResetMode: Ref<'rolling' | 'fixed' | null>
  geminiOAuthType: Ref<'code_assist' | 'google_one' | 'ai_studio'>
  interceptWarmupRequests: Ref<boolean>
  maxSessions: Ref<number | null>
  modelMappings: Ref<ModelMapping[]>
  modelRestrictionMode: Ref<'whitelist' | 'mapping'>
  oauthFlowRef: Ref<OAuthFlowExposed | null>
  rpmLimitEnabled: Ref<boolean>
  rpmStickyBuffer: Ref<number | null>
  rpmStrategy: Ref<'tiered' | 'sticky_exempt'>
  sessionIdMaskingEnabled: Ref<boolean>
  sessionIdleTimeout: Ref<number | null>
  sessionLimitEnabled: Ref<boolean>
  upstreamBillingAutoProbeEnabled: Ref<boolean>
  userMsgQueueMode: Ref<string>
  windowCostEnabled: Ref<boolean>
  windowCostLimit: Ref<number | null>
  windowCostStickyReserve: Ref<number | null>
  step: Ref<number>
  tempUnschedEnabled: Ref<boolean>
  tempUnschedRules: Ref<any[]>
  buildTempUnschedRules: (rules: any[]) => any[]
  applyTempUnschedConfig: (credentials: Record<string, unknown>) => boolean
  geminiSelectedTier: ComputedRef<string>
  isOpenAIModelRestrictionDisabled: ComputedRef<boolean>
  oauth: ReturnType<typeof useAccountOAuth>
  openaiOAuth: ReturnType<typeof useOpenAIOAuth>
  geminiOAuth: ReturnType<typeof useGeminiOAuth>
  antigravityOAuth: ReturnType<typeof useAntigravityOAuth>
  grokOAuth: ReturnType<typeof useGrokOAuth>
  kiroOAuth: ReturnType<typeof useKiroOAuth>
  applyGrokOAuthUpstreamConfig: (credentials: Record<string, unknown>) => void
  applyOpenAIEndpointCapabilities: (credentials: Record<string, unknown>) => void
  applyTLSFingerprintToExtra: (extra: Record<string, unknown>) => void
  withTLSFingerprintExtra: (base?: Record<string, unknown>) => Record<string, unknown>
  buildAntigravityExtra: () => Record<string, unknown> | undefined
  buildOpenAICodexImportExtra: () => Record<string, unknown> | undefined
  buildOpenAICompactModelMapping: () => any
  buildOpenAIExtra: (base?: Record<string, unknown>) => Record<string, unknown> | undefined
  doCreateAccount: (payload: CreateAccountRequest) => Promise<void>
  handleClose: () => void
  validateGrokOAuthUpstreamConfig: () => boolean
  withAntigravityConfirmFlag: (payload: CreateAccountRequest) => CreateAccountRequest
  writeQuotaNotifyToExtra: (extra: Record<string, unknown>, mode: 'create' | 'update') => void
}

export function useCreateAccountOAuthFlows(deps: CreateAccountOAuthFlowDeps) {
  const { t } = useI18n()
  const appStore = useAppStore()
  const {
    form,
    emit,
    addMethod,
    allowedModels,
    apiKeyBaseUrl,
    autoPauseOnExpired,
    editDailyResetHour,
    editDailyResetMode,
    editQuotaDailyLimit,
    editQuotaLimit,
    editQuotaWeeklyLimit,
    editResetTimezone,
    editWeeklyResetDay,
    editWeeklyResetHour,
    editWeeklyResetMode,
    geminiOAuthType,
    modelMappings,
    modelRestrictionMode,
    oauthFlowRef,
    upstreamBillingAutoProbeEnabled,
    step,
    applyTempUnschedConfig,
    geminiSelectedTier,
    oauth,
    openaiOAuth,
    geminiOAuth,
    antigravityOAuth,
    grokOAuth,
    kiroOAuth,
    applyGrokOAuthUpstreamConfig,
    applyOpenAIEndpointCapabilities,
    withTLSFingerprintExtra,
    buildOpenAICompactModelMapping,
    doCreateAccount,
    handleClose,
    validateGrokOAuthUpstreamConfig,
    writeQuotaNotifyToExtra
  } = deps

const goBackToBasicInfo = () => {
  step.value = 1
  oauth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  kiroOAuth.resetState()
  grokOAuth.resetState()
  oauthFlowRef.value?.reset()
}

const handleGenerateUrl = async () => {
  if (form.platform === 'kiro') {
    await kiroOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'openai') {
    await openaiOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'gemini') {
    await geminiOAuth.generateAuthUrl(
      form.proxy_id,
      oauthFlowRef.value?.projectId,
      geminiOAuthType.value,
      geminiSelectedTier.value
    )
  } else if (form.platform === 'antigravity') {
    await antigravityOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'grok') {
    await grokOAuth.generateAuthUrl(form.proxy_id)
  } else {
    await oauth.generateAuthUrl(addMethod.value, form.proxy_id)
  }
}

const applyKiroModelRestriction = (credentials: Record<string, unknown>) => {
  const modelMapping = buildModelMappingObject(
    modelRestrictionMode.value,
    allowedModels.value,
    modelMappings.value,
    'kiro'
  )
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
}

const handleKiroAuthorize = async (payload: {
  callbackUrl: string
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
}) => {
  const tokenInfo = await kiroOAuth.exchangeCallback(payload.callbackUrl, form.proxy_id)
  if (!tokenInfo) {
    return
  }
  const credentials = kiroOAuth.buildCredentials(tokenInfo, payload.credentials)
  const extra = kiroOAuth.buildExtraInfo(tokenInfo, payload.extra)
  applyKiroModelRestriction(credentials)
  await createAccountAndFinish('kiro', 'oauth', credentials, extra, kiroOAuth.buildAccountName(tokenInfo, form.name))
}

const handleKiroValidateRT = async (payload: {
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
}) => {
  const refreshTokens = String(payload.credentials.refresh_token || '')
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    kiroOAuth.error.value = t('admin.accounts.kiro.refreshTokenRequired')
    return
  }

  kiroOAuth.loading.value = true
  kiroOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
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
          form.proxy_id,
          false
        )
        if (!validatedCredentials) {
          failedCount++
          errors.push(`#${i + 1}: ${kiroOAuth.error.value || t('admin.accounts.kiro.failedToValidateRT')}`)
          kiroOAuth.error.value = ''
          continue
        }

        const tokenInfo = validatedCredentials as KiroTokenInfo
        const credentials = { ...validatedCredentials }
        const extra = withTLSFingerprintExtra(kiroOAuth.buildExtraInfo(tokenInfo, payload.extra))
        applyKiroModelRestriction(credentials)
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        const baseName = kiroOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: 'kiro',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error?.response?.data?.detail || error?.message || t('admin.accounts.failedToCreate')
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      kiroOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      kiroOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    kiroOAuth.loading.value = false
  }
}

const handleValidateSessionToken = (_sessionToken: string) => {
  // Session token validation removed
}

const formatDateTimeLocal = formatDateTimeLocalInput
const parseDateTimeLocal = parseDateTimeLocalInput

// Create account and handle success/failure
const createAccountAndFinish = async (
  platform: AccountPlatform,
  type: AccountType,
  credentials: Record<string, unknown>,
  extra?: Record<string, unknown>,
  nameOverride?: string
) => {
  if (!applyTempUnschedConfig(credentials)) {
    return
  }
  // Inject quota limits for apikey/bedrock accounts
  let finalExtra = extra
  if (type === 'apikey' || type === 'bedrock') {
    const quotaExtra: Record<string, unknown> = { ...(extra || {}) }
    if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
      quotaExtra.quota_limit = editQuotaLimit.value
    }
    if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
      quotaExtra.quota_daily_limit = editQuotaDailyLimit.value
    }
    if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
      quotaExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
    }
    // Quota reset mode config
    if (editDailyResetMode.value === 'fixed') {
      quotaExtra.quota_daily_reset_mode = 'fixed'
      quotaExtra.quota_daily_reset_hour = editDailyResetHour.value ?? 0
    }
    if (editWeeklyResetMode.value === 'fixed') {
      quotaExtra.quota_weekly_reset_mode = 'fixed'
      quotaExtra.quota_weekly_reset_day = editWeeklyResetDay.value ?? 1
      quotaExtra.quota_weekly_reset_hour = editWeeklyResetHour.value ?? 0
    }
    if (editDailyResetMode.value === 'fixed' || editWeeklyResetMode.value === 'fixed') {
      quotaExtra.quota_reset_timezone = editResetTimezone.value || 'UTC'
    }
    // Quota notify config
    writeQuotaNotifyToExtra(quotaExtra, 'create')
    if (Object.keys(quotaExtra).length > 0) {
      finalExtra = quotaExtra
    }
  }
  if (platform === 'openai') {
    if (type === 'apikey') {
      applyOpenAIEndpointCapabilities(credentials)
    }
    const compactModelMapping = buildOpenAICompactModelMapping()
    if (compactModelMapping) {
      credentials.compact_model_mapping = compactModelMapping
    } else {
      delete credentials.compact_model_mapping
    }
  }
  if (platform === 'grok') {
    if (!credentials.base_url) {
      credentials.base_url = apiKeyBaseUrl.value.trim() || 'https://api.x.ai/v1'
    }
    const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    } else {
      delete credentials.model_mapping
    }
  }
  await doCreateAccount({
    name: nameOverride || form.name,
    notes: form.notes,
    platform,
    type,
    credentials,
    extra: finalExtra,
    proxy_id: form.proxy_id,
    concurrency: form.concurrency,
    load_factor: form.load_factor ?? undefined,
    priority: form.priority,
    rate_multiplier: form.rate_multiplier,
    group_ids: form.group_ids,
    expires_at: form.expires_at,
    // 上游倍率探测对全部 API-key 平台开放（antigravity upstream 走本 helper）；
    // 非 apikey 类型（bedrock/oauth）不传，后端不动作。
    upstream_billing_probe_enabled: type === 'apikey' ? upstreamBillingAutoProbeEnabled.value : undefined,
    auto_pause_on_expired: autoPauseOnExpired.value
  })
}

// Grok 手动 RT 批量验证和创建
const handleGrokValidateRT = async (refreshTokenInput: string) => {
  if (!refreshTokenInput.trim()) return

  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    grokOAuth.error.value = t('admin.accounts.oauth.grok.pleaseEnterRefreshToken')
    return
  }
  if (!validateGrokOAuthUpstreamConfig()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await grokOAuth.validateRefreshToken(refreshTokens[i], form.proxy_id)
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${grokOAuth.error.value || 'Validation failed'}`)
          grokOAuth.error.value = ''
          continue
        }

        const credentials = grokOAuth.buildCredentials(tokenInfo)
        applyGrokOAuthUpstreamConfig(credentials)
        const extra = withTLSFingerprintExtra(grokOAuth.buildExtraInfo(tokenInfo))
        const baseName = grokOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
        if (modelMapping) {
          credentials.model_mapping = modelMapping
        }
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0) {
      appStore.showWarning(t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount }))
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    grokOAuth.loading.value = false
  }
}

const handleGrokImportSSO = async (ssoInput: string) => {
  // Align with OpenAI/Grok RT batch import: one token per line, no client-side dedupe.
  const ssoTokens = ssoInput
    .split('\n')
    .map((token) => token.trim())
    .filter((token) => token)
  if (ssoTokens.length === 0) return
  if (!validateGrokOAuthUpstreamConfig()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  const credentials: Record<string, unknown> = {}
  applyGrokOAuthUpstreamConfig(credentials)
  const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
  if (!applyTempUnschedConfig(credentials)) {
    grokOAuth.loading.value = false
    return
  }

  try {
    const result = await adminAPI.grok.createFromSSO({
      sso_tokens: ssoTokens,
      name: form.name || undefined,
      notes: form.notes || undefined,
      proxy_id: form.proxy_id,
      group_ids: form.group_ids,
      credentials,
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value
    })

    const successCount = result.created?.length || 0
    const failedCount = result.failed?.length || 0
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        ssoTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      // Same as OpenAI/Grok RT: keep input, show failures, refresh list.
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n') || t('admin.accounts.oauth.grok.failedToConvertSSO')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } catch (error: any) {
    grokOAuth.error.value = error.response?.data?.detail || error.message || t('admin.accounts.oauth.grok.failedToConvertSSO')
    appStore.showError(grokOAuth.error.value)
  } finally {
    grokOAuth.loading.value = false
  }
}

/**
 * Grok password login: each line is email----password.
 * Password is only used for the authorize API call; buildCredentials never stores it.
 */
const handleGrokAuthorizePassword = async (emailPasswordInput: string) => {
  if (!emailPasswordInput.trim()) return
  if (!validateGrokOAuthUpstreamConfig()) return

  const lines = emailPasswordInput
    .split('\n')
    // Keep the password portion byte-for-byte; trim is only for determining
    // whether this textarea line is blank.
    .filter((line) => line.trim() && line.includes('----'))

  if (lines.length === 0) {
    grokOAuth.error.value = t(
      'admin.accounts.oauth.grok.pleaseEnterPassword',
      'Please enter email----password (one per line)'
    )
    return
  }

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < lines.length; i++) {
      try {
        const tokenInfo = await grokOAuth.authorizePassword(lines[i], form.proxy_id)
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${grokOAuth.error.value || 'Authorization failed'}`)
          grokOAuth.error.value = ''
          continue
        }

        const credentials = grokOAuth.buildCredentials(tokenInfo)
        applyGrokOAuthUpstreamConfig(credentials)
        const extra = withTLSFingerprintExtra(grokOAuth.buildExtraInfo(tokenInfo))
        const baseName = grokOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = lines.length > 1 ? `${baseName} #${i + 1}` : baseName

        const modelMapping = buildModelMappingObject(
          modelRestrictionMode.value,
          allowedModels.value,
          modelMappings.value
        )
        if (modelMapping) {
          credentials.model_mapping = modelMapping
        }
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        lines.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', {
          success: successCount,
          failed: failedCount
        })
      )
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    grokOAuth.loading.value = false
  }
}

// OpenAI OAuth 授权码兑换

  return {
    oauth,
    openaiOAuth,
    geminiOAuth,
    antigravityOAuth,
    grokOAuth,
    kiroOAuth,
    goBackToBasicInfo,
    handleGenerateUrl,
    applyKiroModelRestriction,
    handleKiroAuthorize,
    handleKiroValidateRT,
    handleValidateSessionToken,
    formatDateTimeLocal,
    parseDateTimeLocal,
    createAccountAndFinish,
    handleGrokValidateRT,
    handleGrokImportSSO,
    handleGrokAuthorizePassword
  }
}
