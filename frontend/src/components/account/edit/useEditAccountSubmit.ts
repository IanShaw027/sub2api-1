// Extracted verbatim from EditAccountModal.vue's <script setup>: the
// handleSubmit "build update payload -> PATCH account" dispatcher across all
// platform/type combinations (kiro oauth/apikey, generic apikey, upstream,
// vertex service_account, bedrock, oauth/setup-token fallback) plus the
// unconditional post-branch extra-field patches (model mapping persistence,
// grok custom base url, openai plan type, antigravity mapping/mixed
// scheduling, anthropic quota control, openai advanced options, quota
// limits, kiro extra stripping, TLS fingerprint, device learning). Pure
// mechanical relocation — the composable receives the host's own refs and
// functions as `deps` and returns the exact same `handleSubmit` name so the
// host can bind the template to an identical function as before.
import {
  applyAntigravityProjectID,
  applyHeaderOverride,
  applyInterceptWarmup,
  applyPlanType,
  defaultCNAdaptiveBaseUrls,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows
} from '@/components/account/credentialsBuilder'
import { buildModelMappingObject } from '@/composables/useModelWhitelist'
import { stripKiroRuntimeExtra } from '@/composables/useKiroOAuth'
import { isOpenAIWSModeEnabled } from '@/utils/openaiWsMode'
import type { Account } from '@/types'

export interface EditAccountSubmitDeps {
  props: { account: Account | null }
  form: any
  appStore: { showError: (message: string) => void }
  t: (key: string, params?: Record<string, unknown>) => string
  ensureAntigravityMixedChannelConfirmed: (onConfirm: () => Promise<void>) => Promise<boolean>
  submitUpdateAccount: (accountID: number, updatePayload: Record<string, unknown>) => Promise<void>

  autoResetCreditEnabled: any
  autoResetCredit5hThreshold: any
  autoResetCredit7dThreshold: any
  autoPauseOnExpired: any
  upstreamBillingAutoProbeEnabled: any
  upstreamBillingRateSyncEnabled: any

  tempUnschedEnabled: any
  applyTempUnschedConfig: (credentials: Record<string, unknown>) => boolean
  applyKiroModelRestrictionPatch: (
    newCredentials: Record<string, unknown>,
    currentCredentials: Record<string, unknown>
  ) => void
  selectedKiroProfileArnChoice: any
  KIRO_PROFILE_CHOICE_AUTO: string
  KIRO_PROFILE_CHOICE_KEEP: string

  editApiKey: any
  editBaseUrl: any
  defaultBaseUrl: any
  openaiPassthroughEnabled: any
  isCNApiKeyAccount: any
  editAccountMode: any
  editApiProtocol: any
  cnPresetPlatform: any
  editAdaptiveProtocolOptions: any
  editAdaptiveBaseUrls: any
  editZhipuOrganization: any
  editZhipuProject: any

  buildModelRestrictionMapping: () => Record<string, string> | null
  applyOpenAIEndpointCapabilities: (credentials: Record<string, unknown>) => void
  openAICompactModelMappings: any

  poolModeEnabled: any
  poolModeRetryCount: any
  poolModeRetryStatusCodesInput: any
  normalizePoolModeRetryCount: (value: number) => number
  parsePoolModeRetryStatusCodes: (input: string) => number[]

  customErrorCodesEnabled: any
  selectedErrorCodes: any

  headerOverrideEnabled: any
  headerOverrideRows: any

  interceptWarmupRequests: any
  applyAccountSchedulingThresholdOverridePatch: (
    newCredentials: Record<string, unknown>,
    currentCredentials: Record<string, unknown>
  ) => void

  editVertexProjectId: any
  editVertexClientEmail: any
  editVertexLocation: any

  editBedrockRegion: any
  editBedrockForceGlobal: any
  isBedrockAPIKeyMode: any
  editBedrockApiKeyValue: any
  editBedrockAccessKeyId: any
  editBedrockSecretAccessKey: any
  editBedrockSessionToken: any

  isSparkShadow: any
  applyOpenAIModelMappingCredentials: (credentials: Record<string, unknown>) => void

  grokOAuthCustomBaseUrlEnabled: any
  grokOAuthBaseUrl: any
  GROK_CLIENT_TOOL_CACHE_EXTRA_KEY: string
  grokClientToolCacheEnabled: any

  editPlanType: any

  antigravityProjectId: any
  antigravityModelMappings: any
  mixedScheduling: any
  allowOverages: any

  windowCostEnabled: any
  windowCostLimit: any
  windowCostStickyReserve: any
  sessionLimitEnabled: any
  maxSessions: any
  sessionIdleTimeout: any
  rpmLimitEnabled: any
  baseRpm: any
  rpmStrategy: any
  rpmStickyBuffer: any
  userMsgQueueMode: any
  sessionIdMaskingEnabled: any
  cacheTTLOverrideEnabled: any
  cacheTTLOverrideTarget: any
  customBaseUrlEnabled: any
  customBaseUrl: any

  anthropicPassthroughEnabled: any
  anthropicAPIKeyAuthScheme: any
  webSearchEmulationMode: any

  openaiOAuthResponsesWebSocketV2Mode: any
  openaiAPIKeyResponsesWebSocketV2Mode: any
  openaiFlattenNamespacesEnabled: any
  openAILongContextBillingEnabled: any
  openAICompactMode: any
  openAITextGenerationCapabilityEnabled: any
  openAIResponsesMode: any
  autoPause5hThreshold: any
  autoPause7dThreshold: any
  autoPause5hDisabled: any
  autoPause7dDisabled: any
  codexImageToolMode: any
  codexCLIOnlyEnabled: any
  codexCLIOnlyAppServerEnabled: any
  codexFingerprintMode: any

  editQuotaLimit: any
  editQuotaDailyLimit: any
  editQuotaWeeklyLimit: any
  editDailyResetMode: any
  editDailyResetHour: any
  editWeeklyResetMode: any
  editWeeklyResetDay: any
  editWeeklyResetHour: any
  editResetTimezone: any
  writeQuotaNotifyToExtra: (extra: Record<string, unknown>, mode: 'update') => void

  supportsTLSFingerprint: (platform?: string | null) => boolean
  applyTLSFingerprintToExtra: (extra: Record<string, unknown>) => void

  deviceLearningEnabled: any
  deviceTLSProfileId: any
}

export function useEditAccountSubmit(deps: EditAccountSubmitDeps) {
  const {
    props,
    form,
    appStore,
    t,
    ensureAntigravityMixedChannelConfirmed,
    submitUpdateAccount,
    autoResetCreditEnabled,
    autoResetCredit5hThreshold,
    autoResetCredit7dThreshold,
    autoPauseOnExpired,
    upstreamBillingAutoProbeEnabled,
    upstreamBillingRateSyncEnabled,
    tempUnschedEnabled,
    applyTempUnschedConfig,
    applyKiroModelRestrictionPatch,
    selectedKiroProfileArnChoice,
    KIRO_PROFILE_CHOICE_AUTO,
    KIRO_PROFILE_CHOICE_KEEP,
    editApiKey,
    editBaseUrl,
    defaultBaseUrl,
    openaiPassthroughEnabled,
    isCNApiKeyAccount,
    editAccountMode,
    editApiProtocol,
    cnPresetPlatform,
    editAdaptiveProtocolOptions,
    editAdaptiveBaseUrls,
    editZhipuOrganization,
    editZhipuProject,
    buildModelRestrictionMapping,
    applyOpenAIEndpointCapabilities,
    openAICompactModelMappings,
    poolModeEnabled,
    poolModeRetryCount,
    poolModeRetryStatusCodesInput,
    normalizePoolModeRetryCount,
    parsePoolModeRetryStatusCodes,
    customErrorCodesEnabled,
    selectedErrorCodes,
    headerOverrideEnabled,
    headerOverrideRows,
    interceptWarmupRequests,
    applyAccountSchedulingThresholdOverridePatch,
    editVertexProjectId,
    editVertexClientEmail,
    editVertexLocation,
    editBedrockRegion,
    editBedrockForceGlobal,
    isBedrockAPIKeyMode,
    editBedrockApiKeyValue,
    editBedrockAccessKeyId,
    editBedrockSecretAccessKey,
    editBedrockSessionToken,
    isSparkShadow,
    applyOpenAIModelMappingCredentials,
    grokOAuthCustomBaseUrlEnabled,
    grokOAuthBaseUrl,
    GROK_CLIENT_TOOL_CACHE_EXTRA_KEY,
    grokClientToolCacheEnabled,
    editPlanType,
    antigravityProjectId,
    antigravityModelMappings,
    mixedScheduling,
    allowOverages,
    windowCostEnabled,
    windowCostLimit,
    windowCostStickyReserve,
    sessionLimitEnabled,
    maxSessions,
    sessionIdleTimeout,
    rpmLimitEnabled,
    baseRpm,
    rpmStrategy,
    rpmStickyBuffer,
    userMsgQueueMode,
    sessionIdMaskingEnabled,
    cacheTTLOverrideEnabled,
    cacheTTLOverrideTarget,
    customBaseUrlEnabled,
    customBaseUrl,
    anthropicPassthroughEnabled,
    anthropicAPIKeyAuthScheme,
    webSearchEmulationMode,
    openaiOAuthResponsesWebSocketV2Mode,
    openaiAPIKeyResponsesWebSocketV2Mode,
    openaiFlattenNamespacesEnabled,
    openAILongContextBillingEnabled,
    openAICompactMode,
    openAITextGenerationCapabilityEnabled,
    openAIResponsesMode,
    autoPause5hThreshold,
    autoPause7dThreshold,
    autoPause5hDisabled,
    autoPause7dDisabled,
    codexImageToolMode,
    codexCLIOnlyEnabled,
    codexCLIOnlyAppServerEnabled,
    codexFingerprintMode,
    editQuotaLimit,
    editQuotaDailyLimit,
    editQuotaWeeklyLimit,
    editDailyResetMode,
    editDailyResetHour,
    editWeeklyResetMode,
    editWeeklyResetDay,
    editWeeklyResetHour,
    editResetTimezone,
    writeQuotaNotifyToExtra,
    supportsTLSFingerprint,
    applyTLSFingerprintToExtra,
    deviceLearningEnabled,
    deviceTLSProfileId
  } = deps

  const handleSubmit = async () => {
    if (!props.account) return
    const accountID = props.account.id

    if (form.status !== 'active' && form.status !== 'inactive' && form.status !== 'error') {
      appStore.showError(t('admin.accounts.pleaseSelectStatus'))
      return
    }
    if (autoResetCreditEnabled.value) {
      const thresholds = [autoResetCredit5hThreshold.value, autoResetCredit7dThreshold.value]
      if (thresholds.some((value: number) => !Number.isFinite(value) || value < 0.1 || value > 100)) {
        appStore.showError(t('admin.accounts.autoResetCredit.thresholdInvalid'))
        return
      }
    }

    const updatePayload: Record<string, unknown> = { ...form }
    try {
      // 后端期望 proxy_id: 0 表示清除代理，而不是 null
      if (updatePayload.proxy_id === null) {
        updatePayload.proxy_id = 0
      }
      if (form.expires_at === null) {
        updatePayload.expires_at = 0
      }
      // load_factor: 空值/NaN/0/负数 时发送 0（后端约定 <= 0 = 清除）
      const lf = form.load_factor
      if (lf == null || Number.isNaN(lf) || lf <= 0) {
        updatePayload.load_factor = 0
      }
      updatePayload.auto_pause_on_expired = autoPauseOnExpired.value
      if (props.account.type === 'apikey') {
        updatePayload.upstream_billing_probe_enabled = upstreamBillingAutoProbeEnabled.value
        updatePayload.upstream_billing_rate_sync_enabled = upstreamBillingRateSyncEnabled.value
        if (upstreamBillingRateSyncEnabled.value) {
          delete updatePayload.rate_multiplier
        }
      }
      if (props.account.platform === 'kiro' && props.account.type === 'oauth') {
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newCredentials: Record<string, unknown> = {}

        if (tempUnschedEnabled.value) {
          if (!applyTempUnschedConfig(newCredentials)) {
            return
          }
        } else if (currentCredentials.temp_unschedulable_enabled === true) {
          newCredentials.temp_unschedulable_enabled = false
          newCredentials.temp_unschedulable_rules = []
        }

        applyKiroModelRestrictionPatch(newCredentials, currentCredentials)

        if (selectedKiroProfileArnChoice.value === KIRO_PROFILE_CHOICE_AUTO) {
          newCredentials.profile_arn = ''
        } else if (selectedKiroProfileArnChoice.value !== KIRO_PROFILE_CHOICE_KEEP) {
          newCredentials.profile_arn = selectedKiroProfileArnChoice.value
        }

        if (Object.keys(newCredentials).length > 0) {
          updatePayload.credentials = newCredentials
        }

        const currentExtra = (props.account.extra || {}) as Record<string, unknown>
        updatePayload.extra = stripKiroRuntimeExtra(currentExtra)
      } else if (props.account.platform === 'kiro' && props.account.type === 'apikey') {
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newCredentials: Record<string, unknown> = {}

        if (editApiKey.value.trim()) {
          newCredentials.api_key = editApiKey.value.trim()
        }

        applyKiroModelRestrictionPatch(newCredentials, currentCredentials)

        if (tempUnschedEnabled.value) {
          if (!applyTempUnschedConfig(newCredentials)) {
            return
          }
        } else if (currentCredentials.temp_unschedulable_enabled === true) {
          newCredentials.temp_unschedulable_enabled = false
          newCredentials.temp_unschedulable_rules = []
        }

        if (Object.keys(newCredentials).length > 0) {
          updatePayload.credentials = newCredentials
        }
      } else if (props.account.type === 'apikey') {
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newBaseUrl = editBaseUrl.value.trim() || defaultBaseUrl.value
        const shouldApplyModelMapping = !(props.account.platform === 'openai' && openaiPassthroughEnabled.value)

        // Always update credentials for apikey type to handle model mapping changes
        const newCredentials: Record<string, unknown> = {
          ...currentCredentials,
          base_url: newBaseUrl
        }

        // 国产供应商：模式与协议写入凭据（决定额度/余额探测与转发端点/格式）。
        if (isCNApiKeyAccount.value) {
          newCredentials.account_mode = editAccountMode.value
          newCredentials.api_protocol = editApiProtocol.value
          if (editApiProtocol.value === 'adaptive') {
            const defaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, editAccountMode.value)
            const protocolBaseUrls: Record<string, string> = {}
            for (const item of editAdaptiveProtocolOptions.value) {
              protocolBaseUrls[item.value] = (editAdaptiveBaseUrls.value[item.value] || (defaults as any)[item.value]).trim()
            }
            newCredentials.api_base_urls = protocolBaseUrls
            newCredentials.base_url = protocolBaseUrls.chat_completions
          } else {
            delete newCredentials.api_base_urls
          }
          // 智谱团队版 Coding Plan：组织/项目 ID 写入凭据（非空才写，清空即移除回落个人版路径）
          if (props.account.platform === 'zhipu') {
            const org = editZhipuOrganization.value.trim()
            const project = editZhipuProject.value.trim()
            if (org) {
              newCredentials.zhipu_organization = org
              if (project) newCredentials.zhipu_project = project
              else delete newCredentials.zhipu_project
            } else {
              delete newCredentials.zhipu_organization
              delete newCredentials.zhipu_project
            }
          }
        }

        // Handle API key
        // 后端响应已脱敏：currentCredentials 不会再包含 api_key 原文。
        // 用户填入新值则覆盖；留空时优先看 credentials_status.has_api_key；
        // 若后端尚未升级（无 credentials_status），回退读旧结构 currentCredentials.api_key。
        // 两者都无才报错。
        const hasExistingApiKey =
          props.account.credentials_status?.has_api_key ?? Boolean(currentCredentials.api_key)
        if (editApiKey.value.trim()) {
          newCredentials.api_key = editApiKey.value.trim()
        } else if (!hasExistingApiKey) {
          appStore.showError(t('admin.accounts.apiKeyIsRequired'))
          return
        }

        // Add model mapping if configured（OpenAI 开启自动透传时保留现有映射，不再编辑）
        if (shouldApplyModelMapping) {
          const modelMapping = buildModelRestrictionMapping()
          if (modelMapping) {
            newCredentials.model_mapping = modelMapping
          } else {
            delete newCredentials.model_mapping
          }
        } else if (currentCredentials.model_mapping) {
          newCredentials.model_mapping = currentCredentials.model_mapping
        }
        if (props.account.platform === 'openai') {
          applyOpenAIEndpointCapabilities(newCredentials)
          const compactModelMapping = buildModelMappingObject('mapping', [], openAICompactModelMappings.value)
          if (compactModelMapping) {
            newCredentials.compact_model_mapping = compactModelMapping
          } else {
            delete newCredentials.compact_model_mapping
          }
        }

        // Add pool mode if enabled
        if (poolModeEnabled.value) {
          newCredentials.pool_mode = true
          newCredentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
          const parsedRetryStatusCodes = parsePoolModeRetryStatusCodes(poolModeRetryStatusCodesInput.value)
          if (parsedRetryStatusCodes.length > 0) {
            newCredentials.pool_mode_retry_status_codes = parsedRetryStatusCodes
          } else {
            delete newCredentials.pool_mode_retry_status_codes
          }
        } else {
          delete newCredentials.pool_mode
          delete newCredentials.pool_mode_retry_count
          delete newCredentials.pool_mode_retry_status_codes
        }

        // Add custom error codes if enabled
        if (customErrorCodesEnabled.value) {
          newCredentials.custom_error_codes_enabled = true
          newCredentials.custom_error_codes = [...selectedErrorCodes.value]
        } else {
          delete newCredentials.custom_error_codes_enabled
          delete newCredentials.custom_error_codes
        }

        // Add header override if enabled for this API-key platform
        if (isHeaderOverrideCapable(props.account.platform, 'apikey')) {
          if (headerOverrideEnabled.value) {
            const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
            if (headerError) {
              appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
              return
            }
          }
          applyHeaderOverride(newCredentials, headerOverrideEnabled.value, headerOverrideRows.value, 'edit')
        }

        // Add intercept warmup requests setting
        applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
        applyAccountSchedulingThresholdOverridePatch(newCredentials, currentCredentials)
        if (!applyTempUnschedConfig(newCredentials)) {
          return
        }

        updatePayload.credentials = newCredentials
      } else if (props.account.type === 'upstream') {
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newCredentials: Record<string, unknown> = { ...currentCredentials }

        newCredentials.base_url = editBaseUrl.value.trim()

        if (editApiKey.value.trim()) {
          newCredentials.api_key = editApiKey.value.trim()
        }

        // Add intercept warmup requests setting
        applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

        applyAccountSchedulingThresholdOverridePatch(newCredentials, currentCredentials)
        if (!applyTempUnschedConfig(newCredentials)) {
          return
        }

        updatePayload.credentials = newCredentials
      } else if ((props.account.platform === 'gemini' || props.account.platform === 'anthropic') && props.account.type === 'service_account') {
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newCredentials: Record<string, unknown> = { ...currentCredentials }

        if (!editVertexProjectId.value.trim()) {
          appStore.showError(t('admin.accounts.vertexSaJsonMissingProjectId'))
          return
        }
        if (!editVertexClientEmail.value.trim()) {
          appStore.showError(t('admin.accounts.vertexSaJsonMissingClientEmail'))
          return
        }
        if (!editVertexLocation.value.trim()) {
          appStore.showError(t('admin.accounts.vertexLocationRequired'))
          return
        }

        // SA JSON 已脱敏不再随 credentials 返回，存在性优先读 credentials_status。
        // 若后端尚未升级（无 credentials_status），回退读旧结构 service_account_json / service_account。
        const credentialsStatus = props.account.credentials_status
        const hasExistingServiceAccountJson = credentialsStatus
          ? Boolean(
              credentialsStatus.has_service_account_json || credentialsStatus.has_service_account
            )
          : Boolean(currentCredentials.service_account_json || currentCredentials.service_account)
        if (!hasExistingServiceAccountJson) {
          appStore.showError(t('admin.accounts.vertexSaJsonRequired'))
          return
        }
        newCredentials.project_id = editVertexProjectId.value.trim()
        newCredentials.client_email = editVertexClientEmail.value.trim()
        newCredentials.location = editVertexLocation.value.trim()
        newCredentials.tier_id = 'vertex'

        // Add model mapping if configured
        const modelMapping = buildModelRestrictionMapping()
        if (modelMapping) {
          newCredentials.model_mapping = modelMapping
        } else {
          delete newCredentials.model_mapping
        }

        applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
        applyAccountSchedulingThresholdOverridePatch(newCredentials, currentCredentials)
        if (!applyTempUnschedConfig(newCredentials)) {
          return
        }

        updatePayload.credentials = newCredentials
      } else if (props.account.type === 'bedrock') {
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newCredentials: Record<string, unknown> = { ...currentCredentials }

        newCredentials.aws_region = editBedrockRegion.value.trim()
        if (editBedrockForceGlobal.value) {
          newCredentials.aws_force_global = 'true'
        } else {
          delete newCredentials.aws_force_global
        }

        if (isBedrockAPIKeyMode.value) {
          // API Key mode: only update api_key if user provided new value
          if (editBedrockApiKeyValue.value.trim()) {
            newCredentials.api_key = editBedrockApiKeyValue.value.trim()
          }
        } else {
          // SigV4 mode
          newCredentials.aws_access_key_id = editBedrockAccessKeyId.value.trim()
          if (editBedrockSecretAccessKey.value.trim()) {
            newCredentials.aws_secret_access_key = editBedrockSecretAccessKey.value.trim()
          }
          if (editBedrockSessionToken.value.trim()) {
            newCredentials.aws_session_token = editBedrockSessionToken.value.trim()
          }
        }

        // Pool mode
        if (poolModeEnabled.value) {
          newCredentials.pool_mode = true
          newCredentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
          const parsedRetryStatusCodes = parsePoolModeRetryStatusCodes(poolModeRetryStatusCodesInput.value)
          if (parsedRetryStatusCodes.length > 0) {
            newCredentials.pool_mode_retry_status_codes = parsedRetryStatusCodes
          } else {
            delete newCredentials.pool_mode_retry_status_codes
          }
        } else {
          delete newCredentials.pool_mode
          delete newCredentials.pool_mode_retry_count
          delete newCredentials.pool_mode_retry_status_codes
        }

        // Model mapping
        const modelMapping = buildModelRestrictionMapping()
        if (modelMapping) {
          newCredentials.model_mapping = modelMapping
        } else {
          delete newCredentials.model_mapping
        }

        applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
        applyAccountSchedulingThresholdOverridePatch(newCredentials, currentCredentials)
        if (!applyTempUnschedConfig(newCredentials)) {
          return
        }

        updatePayload.credentials = newCredentials
      } else {
        // For oauth/setup-token types, only update intercept_warmup_requests if changed
        const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
        const newCredentials: Record<string, unknown> = { ...currentCredentials }

        applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')
        applyAccountSchedulingThresholdOverridePatch(newCredentials, currentCredentials)
        if (!applyTempUnschedConfig(newCredentials)) {
          return
        }

        updatePayload.credentials = newCredentials
      }

      // OpenAI/Grok OAuth: persist model mapping to credentials
      if ((props.account.platform === 'openai' || props.account.platform === 'grok') && props.account.type === 'oauth') {
        const currentCredentials = isSparkShadow.value
          ? {}
          : (updatePayload.credentials as Record<string, unknown>) ||
            ((props.account.credentials as Record<string, unknown>) || {})
        const newCredentials: Record<string, unknown> = { ...currentCredentials }
        if (props.account.platform === 'openai') {
          applyOpenAIModelMappingCredentials(newCredentials)
        } else {
          const modelMapping = buildModelRestrictionMapping()
          if (modelMapping) {
            newCredentials.model_mapping = modelMapping
          } else {
            delete newCredentials.model_mapping
          }
        }

        updatePayload.credentials = newCredentials
      }

      // Grok OAuth: 自定义上游地址 + 请求头覆写。base_url 仅改写转发端点，
      // OAuth 授权与令牌刷新链路不读取该值；关闭开关即恢复默认官方网关。
      if (props.account.platform === 'grok' && props.account.type === 'oauth') {
        const currentCredentials =
          (updatePayload.credentials as Record<string, unknown>) ||
          ((props.account.credentials as Record<string, unknown>) || {})
        const newCredentials: Record<string, unknown> = { ...currentCredentials }

        if (grokOAuthCustomBaseUrlEnabled.value) {
          const trimmedBaseUrl = grokOAuthBaseUrl.value.trim()
          if (!trimmedBaseUrl) {
            appStore.showError(t('admin.accounts.grokCustomBaseUrl.required'))
            return
          }
          if (!/^https?:\/\//i.test(trimmedBaseUrl)) {
            appStore.showError(t('admin.accounts.grokCustomBaseUrl.invalid'))
            return
          }
          newCredentials.base_url = trimmedBaseUrl
        } else {
          delete newCredentials.base_url
        }

        if (headerOverrideEnabled.value) {
          const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
          if (headerError) {
            appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
            return
          }
        }
        applyHeaderOverride(newCredentials, headerOverrideEnabled.value, headerOverrideRows.value, 'edit')

        updatePayload.credentials = newCredentials

        const newExtra: Record<string, unknown> = {
          ...((props.account.extra as Record<string, unknown>) || {})
        }
        // Persist both states so a disabled account remains opted out when the
        // backend applies the default-enabled policy to missing values.
        newExtra[GROK_CLIENT_TOOL_CACHE_EXTRA_KEY] = grokClientToolCacheEnabled.value
        updatePayload.extra = newExtra
      }

      // OpenAI: 手动覆盖订阅档位 plan_type（Plus/Pro/Free）。仅 OAuth 非影子账号：
      // 影子账号凭据由母账号管理(且后端会 sanitize),setup-token 无订阅调度语义。
      if (props.account.platform === 'openai' && props.account.type === 'oauth' && !isSparkShadow.value) {
        const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
          ((props.account.credentials as Record<string, unknown>) || {})
        updatePayload.credentials = applyPlanType({ ...currentCredentials }, editPlanType.value)
      }

      // Antigravity: persist model mapping to credentials (applies to all antigravity types)
      // Antigravity 只支持映射模式
      if (props.account.platform === 'antigravity') {
        const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
          ((props.account.credentials as Record<string, unknown>) || {})
        const newCredentials: Record<string, unknown> = { ...currentCredentials }
        if (props.account.type === 'oauth') {
          applyAntigravityProjectID(newCredentials, antigravityProjectId.value, 'edit')
        }

        // 移除旧字段
        delete newCredentials.model_whitelist
        delete newCredentials.model_mapping

        // 只使用映射模式
        const antigravityModelMapping = buildModelMappingObject(
          'mapping',
          [],
          antigravityModelMappings.value
        )
        if (antigravityModelMapping) {
          newCredentials.model_mapping = antigravityModelMapping
        }

        updatePayload.credentials = newCredentials
      }

      // For antigravity accounts, handle mixed_scheduling and allow_overages in extra
      if (props.account.platform === 'antigravity') {
        const currentExtra = (props.account.extra as Record<string, unknown>) || {}
        const newExtra: Record<string, unknown> = { ...currentExtra }
        if (mixedScheduling.value) {
          newExtra.mixed_scheduling = true
        } else {
          delete newExtra.mixed_scheduling
        }
        if (allowOverages.value) {
          newExtra.allow_overages = true
        } else {
          delete newExtra.allow_overages
        }
        updatePayload.extra = newExtra
      }

      // For Anthropic OAuth/SetupToken accounts, handle quota control settings in extra
      if (props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token')) {
        const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
        const newExtra: Record<string, unknown> = { ...currentExtra }

        // Window cost limit settings
        if (windowCostEnabled.value && windowCostLimit.value != null && windowCostLimit.value > 0) {
          newExtra.window_cost_limit = windowCostLimit.value
          newExtra.window_cost_sticky_reserve = windowCostStickyReserve.value ?? 10
        } else {
          delete newExtra.window_cost_limit
          delete newExtra.window_cost_sticky_reserve
        }

        // Session limit settings
        if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
          newExtra.max_sessions = maxSessions.value
          newExtra.session_idle_timeout_minutes = sessionIdleTimeout.value ?? 5
        } else {
          delete newExtra.max_sessions
          delete newExtra.session_idle_timeout_minutes
        }

        // RPM limit settings
        if (rpmLimitEnabled.value) {
          const DEFAULT_BASE_RPM = 15
          newExtra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
            ? baseRpm.value
            : DEFAULT_BASE_RPM
          newExtra.rpm_strategy = rpmStrategy.value
          if (rpmStickyBuffer.value != null && rpmStickyBuffer.value >= 1 && rpmStickyBuffer.value <= 10000) {
            newExtra.rpm_sticky_buffer = rpmStickyBuffer.value
          } else {
            delete newExtra.rpm_sticky_buffer
          }
        } else {
          delete newExtra.base_rpm
          delete newExtra.rpm_strategy
          delete newExtra.rpm_sticky_buffer
        }

        // UMQ mode（独立于 RPM 保存）
        if (userMsgQueueMode.value) {
          newExtra.user_msg_queue_mode = userMsgQueueMode.value
        } else {
          delete newExtra.user_msg_queue_mode
        }
        delete newExtra.user_msg_queue_enabled  // 清理旧字段

        // Session ID masking setting
        if (sessionIdMaskingEnabled.value) {
          newExtra.session_id_masking_enabled = true
        } else {
          delete newExtra.session_id_masking_enabled
        }

        // Cache TTL override setting
        if (cacheTTLOverrideEnabled.value) {
          newExtra.cache_ttl_override_enabled = true
          newExtra.cache_ttl_override_target = cacheTTLOverrideTarget.value
        } else {
          delete newExtra.cache_ttl_override_enabled
          delete newExtra.cache_ttl_override_target
        }

        // Custom base URL relay setting
        if (customBaseUrlEnabled.value && customBaseUrl.value.trim()) {
          newExtra.custom_base_url_enabled = true
          newExtra.custom_base_url = customBaseUrl.value.trim()
        } else {
          delete newExtra.custom_base_url_enabled
          delete newExtra.custom_base_url
        }

        updatePayload.extra = newExtra
      }

      // For Anthropic API Key accounts, handle passthrough mode + web search emulation in extra
      if (props.account.platform === 'anthropic' && props.account.type === 'apikey') {
        const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
        const newExtra: Record<string, unknown> = { ...currentExtra }
        if (anthropicPassthroughEnabled.value) {
          newExtra.anthropic_passthrough = true
        } else {
          delete newExtra.anthropic_passthrough
        }
        if (anthropicAPIKeyAuthScheme.value === 'authorization_bearer') {
          newExtra.anthropic_apikey_auth_scheme = 'authorization_bearer'
        } else {
          delete newExtra.anthropic_apikey_auth_scheme
        }
        if (webSearchEmulationMode.value === 'default') {
          delete newExtra.web_search_emulation
        } else {
          newExtra.web_search_emulation = webSearchEmulationMode.value
        }
        updatePayload.extra = newExtra
      }

      // For OpenAI OAuth/SetupToken/API Key accounts, handle passthrough mode in extra
      if (props.account.platform === 'openai' && (props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'apikey')) {
        const currentExtra = (props.account.extra as Record<string, unknown>) || {}
        const newExtra: Record<string, unknown> = { ...currentExtra }
        const hadCodexCLIOnlyEnabled = currentExtra.codex_cli_only === true
        if (props.account.type === 'oauth' || props.account.type === 'setup-token') {
          newExtra.openai_oauth_responses_websockets_v2_mode = openaiOAuthResponsesWebSocketV2Mode.value
          newExtra.openai_oauth_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiOAuthResponsesWebSocketV2Mode.value)
        } else if (props.account.type === 'apikey') {
          newExtra.openai_apikey_responses_websockets_v2_mode = openaiAPIKeyResponsesWebSocketV2Mode.value
          newExtra.openai_apikey_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiAPIKeyResponsesWebSocketV2Mode.value)
        }
        delete newExtra.responses_websockets_v2_enabled
        delete newExtra.openai_ws_enabled
        if (openaiPassthroughEnabled.value) {
          newExtra.openai_passthrough = true
        } else {
          delete newExtra.openai_passthrough
          delete newExtra.openai_oauth_passthrough
        }
        // 缺省即保留 namespace，不写空值，避免 extra 里堆积默认项
        if (props.account.type === 'oauth' && openaiFlattenNamespacesEnabled.value) {
          newExtra.openai_responses_flatten_namespaces = true
        } else {
          delete newExtra.openai_responses_flatten_namespaces
        }
        if (isSparkShadow.value) {
          delete newExtra.openai_long_context_billing_enabled
        } else {
          newExtra.openai_long_context_billing_enabled = openAILongContextBillingEnabled.value
        }
        if (openAICompactMode.value === 'auto') {
          delete newExtra.openai_compact_mode
        } else {
          newExtra.openai_compact_mode = openAICompactMode.value
        }
        if (props.account.type === 'apikey') {
          if (!openAITextGenerationCapabilityEnabled.value || openAIResponsesMode.value === 'auto') {
            delete newExtra.openai_responses_mode
          } else {
            newExtra.openai_responses_mode = openAIResponsesMode.value
          }
        }
        if (autoPause5hThreshold.value != null && autoPause5hThreshold.value > 0) {
          newExtra.auto_pause_5h_threshold = autoPause5hThreshold.value / 100
        } else {
          delete newExtra.auto_pause_5h_threshold
        }
        if (autoPause7dThreshold.value != null && autoPause7dThreshold.value > 0) {
          newExtra.auto_pause_7d_threshold = autoPause7dThreshold.value / 100
        } else {
          delete newExtra.auto_pause_7d_threshold
        }
        if (autoPause5hDisabled.value) {
          newExtra.auto_pause_5h_disabled = true
        } else {
          delete newExtra.auto_pause_5h_disabled
        }
        if (autoPause7dDisabled.value) {
          newExtra.auto_pause_7d_disabled = true
        } else {
          delete newExtra.auto_pause_7d_disabled
        }
        if (props.account.type === 'oauth' && !isSparkShadow.value) {
          newExtra.auto_reset_credit_enabled = autoResetCreditEnabled.value
          newExtra.auto_reset_credit_5h_threshold = autoResetCredit5hThreshold.value / 100
          newExtra.auto_reset_credit_7d_threshold = autoResetCredit7dThreshold.value / 100
        }
        // 运行态只允许后端服务更新，账号编辑不得回写旧状态。
        delete newExtra.codex_auto_reset_credit_state

        delete newExtra.codex_image_generation_bridge_enabled
        switch (codexImageToolMode.value) {
          case 'enabled':
          case 'disabled':
            newExtra.codex_image_generation_bridge = codexImageToolMode.value === 'enabled'
            delete newExtra.codex_image_generation_explicit_tool_policy
            break
          case 'block':
            newExtra.codex_image_generation_explicit_tool_policy = 'strip'
            delete newExtra.codex_image_generation_bridge
            break
          default:
            delete newExtra.codex_image_generation_bridge
            delete newExtra.codex_image_generation_explicit_tool_policy
        }

        if (props.account.type === 'oauth' || props.account.type === 'setup-token') {
          if (codexCLIOnlyEnabled.value) {
            newExtra.codex_cli_only = true
          } else if (hadCodexCLIOnlyEnabled) {
            // 关闭时显式写 false，避免 extra 为空被后端忽略导致旧值无法清除
            newExtra.codex_cli_only = false
          } else {
            delete newExtra.codex_cli_only
          }
          // Claude Code 插件放行已迁移到全局 codex_cli_only_whitelist，编辑时清理废弃账号级快捷字段。
          delete newExtra.codex_cli_only_allowed_clients
          if (codexCLIOnlyEnabled.value && codexCLIOnlyAppServerEnabled.value) {
            newExtra.codex_cli_only_allow_app_server = true
          } else {
            delete newExtra.codex_cli_only_allow_app_server
          }
        }

        // 指纹收敛模式：默认 off（不写入）；device/session/full 是显式 opt-in，
        // 必须落键，否则管理员的选择会被后端当作"未设置"而回落到 off（#5610）。
        if (props.account.type === 'oauth') {
          if (codexFingerprintMode.value !== 'off') {
            newExtra.codex_fingerprint_mode = codexFingerprintMode.value
          } else {
            delete newExtra.codex_fingerprint_mode
          }
        }

        updatePayload.extra = newExtra
      }

      // For apikey/bedrock accounts, handle quota_limit in extra
      if (props.account.type === 'apikey' || props.account.type === 'bedrock') {
        const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
          (props.account.extra as Record<string, unknown>) || {}
        const newExtra: Record<string, unknown> = { ...currentExtra }
        // 上游倍率自动探测对全部 API-key 平台开放（sub2api 上游即可应答），
        // Bedrock 凭证无静态 Key 不参与。
        if (props.account.type === 'apikey') {
          delete newExtra.upstream_billing_probe_enabled
          delete newExtra.upstream_billing_rate_sync_enabled
        }
        // Total quota
        if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
          newExtra.quota_limit = editQuotaLimit.value
        } else {
          delete newExtra.quota_limit
        }
        // Daily quota
        if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
          newExtra.quota_daily_limit = editQuotaDailyLimit.value
        } else {
          delete newExtra.quota_daily_limit
          delete newExtra.quota_daily_used
          delete newExtra.quota_daily_start
        }
        // Weekly quota
        if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
          newExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
        } else {
          delete newExtra.quota_weekly_limit
          delete newExtra.quota_weekly_used
          delete newExtra.quota_weekly_start
        }
        // Quota reset mode config
        if (editDailyResetMode.value === 'fixed') {
          newExtra.quota_daily_reset_mode = 'fixed'
          newExtra.quota_daily_reset_hour = editDailyResetHour.value ?? 0
        } else {
          delete newExtra.quota_daily_reset_mode
          delete newExtra.quota_daily_reset_hour
        }
        if (editWeeklyResetMode.value === 'fixed') {
          newExtra.quota_weekly_reset_mode = 'fixed'
          newExtra.quota_weekly_reset_day = editWeeklyResetDay.value ?? 1
          newExtra.quota_weekly_reset_hour = editWeeklyResetHour.value ?? 0
        } else {
          delete newExtra.quota_weekly_reset_mode
          delete newExtra.quota_weekly_reset_day
          delete newExtra.quota_weekly_reset_hour
        }
        if (editDailyResetMode.value === 'fixed' || editWeeklyResetMode.value === 'fixed') {
          newExtra.quota_reset_timezone = editResetTimezone.value || 'UTC'
        } else {
          delete newExtra.quota_reset_timezone
        }
        // Quota notify config
        writeQuotaNotifyToExtra(newExtra, 'update')
        updatePayload.extra = newExtra
      }

      if (props.account.platform === 'kiro') {
        const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
          (props.account.extra as Record<string, unknown>) ||
          {}
        updatePayload.extra = stripKiroRuntimeExtra(currentExtra)
      }

      if (supportsTLSFingerprint(props.account.platform)) {
        const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
          (props.account.extra as Record<string, unknown>) ||
          {}
        const newExtra: Record<string, unknown> = { ...currentExtra }
        applyTLSFingerprintToExtra(newExtra)
        updatePayload.extra = newExtra
      }

      if (!isSparkShadow.value) {
        const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
          (props.account.extra as Record<string, unknown>) ||
          {}
        const newExtra: Record<string, unknown> = { ...currentExtra }
        newExtra.device_learning_enabled = deviceLearningEnabled.value
        if (deviceTLSProfileId.value) {
          newExtra.device_tls_profile_id = deviceTLSProfileId.value
        } else {
          delete newExtra.device_tls_profile_id
        }
        updatePayload.extra = newExtra
      }

      const canContinue = await ensureAntigravityMixedChannelConfirmed(async () => {
        await submitUpdateAccount(accountID, updatePayload)
      })
      if (!canContinue) {
        return
      }

      await submitUpdateAccount(accountID, updatePayload)
    } catch (error: any) {
      appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
    }
  }

  return { handleSubmit }
}
