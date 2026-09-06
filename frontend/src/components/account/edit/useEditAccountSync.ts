// Extracted verbatim from EditAccountModal.vue's <script setup>: the
// syncFormFromAccount hydration function that populates every local
// ref/form field from an incoming `Account` whenever the edit modal opens
// or the account prop changes. Pure mechanical relocation — the composable
// receives the host's own refs and functions as `deps` and returns the
// exact same `syncFormFromAccount` name so the host watcher can call an
// identical function as before.
import {
  cnSupportsNativeResponses,
  defaultCNAdaptiveBaseUrls,
  defaultCNBaseUrl,
  isCustomGrokBaseUrl,
  isHeaderOverrideCapable,
  readPlanType,
  splitHeaderOverridesObject,
  HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY,
  HEADER_OVERRIDES_CREDENTIAL_KEY,
  type CnNativeApiProtocol
} from '@/components/account/credentialsBuilder'
import { OPENAI_WS_MODE_OFF, resolveOpenAIWSModeFromExtra } from '@/utils/openaiWsMode'
import type { Account, OpenAICompactMode } from '@/types'

type AccountPlatform = Account['platform']

type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'

export interface EditAccountSyncDeps {
  form: any
  syncingForm: any
  nextTick: (cb: () => void) => void

  showMixedChannelWarning: any
  mixedChannelWarningDetails: any
  mixedChannelWarningRawMessage: any
  mixedChannelWarningAction: any
  antigravityMixedChannelConfirmed: any

  interceptWarmupRequests: any
  autoPauseOnExpired: any
  editVertexProjectId: any
  editVertexClientEmail: any
  editVertexLocation: any
  antigravityProjectId: any

  mixedScheduling: any
  allowOverages: any
  deviceLearningEnabled: any
  deviceTLSProfileId: any
  readDeviceTLSProfileId: (value: unknown) => number | null
  autoPause5hThreshold: any
  autoPause7dThreshold: any
  autoPause5hDisabled: any
  autoPause7dDisabled: any
  autoResetCreditEnabled: any
  autoResetCredit5hThreshold: any
  autoResetCredit7dThreshold: any
  upstreamBillingAutoProbeEnabled: any
  upstreamBillingRateSyncEnabled: any

  openaiPassthroughEnabled: any
  openaiFlattenNamespacesEnabled: any
  openAILongContextBillingEnabled: any
  editPlanType: any
  openAICompactMode: any
  openAIResponsesMode: any
  openAIEndpointCapabilities: any
  openAICompactModelMappings: any
  openaiOAuthResponsesWebSocketV2Mode: any
  openaiAPIKeyResponsesWebSocketV2Mode: any
  codexCLIOnlyEnabled: any
  codexCLIOnlyAppServerEnabled: any
  codexFingerprintMode: any
  codexImageToolMode: any
  anthropicPassthroughEnabled: any
  anthropicAPIKeyAuthScheme: any
  webSearchEmulationMode: any
  openAITextGenerationCapabilityEnabled: any
  normalizeOpenAIResponsesMode: (mode: unknown) => any
  readOpenAIEndpointCapabilities: (credentials?: Record<string, unknown>) => any

  editQuotaLimit: any
  editQuotaDailyLimit: any
  editQuotaWeeklyLimit: any
  editDailyResetMode: any
  editDailyResetHour: any
  editWeeklyResetMode: any
  editWeeklyResetDay: any
  editWeeklyResetHour: any
  editResetTimezone: any
  loadQuotaNotifyFromExtra: (extra?: Record<string, unknown>) => void
  resetQuotaNotify: () => void

  antigravityModelRestrictionMode: any
  antigravityWhitelistModels: any
  antigravityModelMappings: any

  loadQuotaControlSettings: (account: Account) => void
  loadTempUnschedRules: (credentials?: Record<string, unknown>) => void
  loadAccountSchedulingThresholdOverride: (
    platform: AccountPlatform | undefined,
    credentials: Record<string, unknown> | undefined
  ) => void

  headerOverrideEnabled: any
  headerOverrideRows: any

  grokOAuthCustomBaseUrlEnabled: any
  grokOAuthBaseUrl: any
  grokClientToolCacheEnabled: any
  GROK_CLIENT_TOOL_CACHE_EXTRA_KEY: string

  loadModelRestrictionFromMapping: (rawMapping?: Record<string, unknown>) => void
  discoveredKiroProfiles: any
  selectedKiroProfileArnChoice: any
  KIRO_PROFILE_CHOICE_KEEP: string

  editBaseUrl: any
  poolModeEnabled: any
  poolModeRetryCount: any
  poolModeRetryStatusCodesInput: any
  normalizePoolModeRetryCount: (value: number) => number
  formatPoolModeRetryStatusCodes: (value: unknown) => string
  DEFAULT_POOL_MODE_RETRY_COUNT: number
  customErrorCodesEnabled: any
  selectedErrorCodes: any

  editAccountMode: any
  editApiProtocol: any
  editAdaptiveBaseUrls: any
  editZhipuOrganization: any
  editZhipuProject: any
  isCNApiKeyAccount: any

  editBedrockRegion: any
  editBedrockForceGlobal: any
  editBedrockApiKeyValue: any
  editBedrockAccessKeyId: any
  editBedrockSecretAccessKey: any
  editBedrockSessionToken: any

  modelRestrictionMode: any
  modelMappings: any
  allowedModels: any

  editApiKey: any
}

export function useEditAccountSync(deps: EditAccountSyncDeps) {
  const {
    form,
    syncingForm,
    nextTick,
    showMixedChannelWarning,
    mixedChannelWarningDetails,
    mixedChannelWarningRawMessage,
    mixedChannelWarningAction,
    antigravityMixedChannelConfirmed,
    interceptWarmupRequests,
    autoPauseOnExpired,
    editVertexProjectId,
    editVertexClientEmail,
    editVertexLocation,
    antigravityProjectId,
    mixedScheduling,
    allowOverages,
    deviceLearningEnabled,
    deviceTLSProfileId,
    readDeviceTLSProfileId,
    autoPause5hThreshold,
    autoPause7dThreshold,
    autoPause5hDisabled,
    autoPause7dDisabled,
    autoResetCreditEnabled,
    autoResetCredit5hThreshold,
    autoResetCredit7dThreshold,
    upstreamBillingAutoProbeEnabled,
    upstreamBillingRateSyncEnabled,
    openaiPassthroughEnabled,
    openaiFlattenNamespacesEnabled,
    openAILongContextBillingEnabled,
    editPlanType,
    openAICompactMode,
    openAIResponsesMode,
    openAIEndpointCapabilities,
    openAICompactModelMappings,
    openaiOAuthResponsesWebSocketV2Mode,
    openaiAPIKeyResponsesWebSocketV2Mode,
    codexCLIOnlyEnabled,
    codexCLIOnlyAppServerEnabled,
    codexFingerprintMode,
    codexImageToolMode,
    anthropicPassthroughEnabled,
    anthropicAPIKeyAuthScheme,
    webSearchEmulationMode,
    openAITextGenerationCapabilityEnabled,
    normalizeOpenAIResponsesMode,
    readOpenAIEndpointCapabilities,
    editQuotaLimit,
    editQuotaDailyLimit,
    editQuotaWeeklyLimit,
    editDailyResetMode,
    editDailyResetHour,
    editWeeklyResetMode,
    editWeeklyResetDay,
    editWeeklyResetHour,
    editResetTimezone,
    loadQuotaNotifyFromExtra,
    resetQuotaNotify,
    antigravityModelRestrictionMode,
    antigravityWhitelistModels,
    antigravityModelMappings,
    loadQuotaControlSettings,
    loadTempUnschedRules,
    loadAccountSchedulingThresholdOverride,
    headerOverrideEnabled,
    headerOverrideRows,
    grokOAuthCustomBaseUrlEnabled,
    grokOAuthBaseUrl,
    grokClientToolCacheEnabled,
    GROK_CLIENT_TOOL_CACHE_EXTRA_KEY,
    loadModelRestrictionFromMapping,
    discoveredKiroProfiles,
    selectedKiroProfileArnChoice,
    KIRO_PROFILE_CHOICE_KEEP,
    editBaseUrl,
    poolModeEnabled,
    poolModeRetryCount,
    poolModeRetryStatusCodesInput,
    normalizePoolModeRetryCount,
    formatPoolModeRetryStatusCodes,
    DEFAULT_POOL_MODE_RETRY_COUNT,
    customErrorCodesEnabled,
    selectedErrorCodes,
    editAccountMode,
    editApiProtocol,
    editAdaptiveBaseUrls,
    editZhipuOrganization,
    editZhipuProject,
    isCNApiKeyAccount,
    editBedrockRegion,
    editBedrockForceGlobal,
    editBedrockApiKeyValue,
    editBedrockAccessKeyId,
    editBedrockSecretAccessKey,
    editBedrockSessionToken,
    modelRestrictionMode,
    modelMappings,
    allowedModels,
    editApiKey
  } = deps

  const syncFormFromAccount = (newAccount: Account | null) => {
    if (!newAccount) {
      return
    }
    // 进入回填窗口：抑制 CN 模式/协议 watcher 联动重置 base_url（见 syncingForm 注释）。
    syncingForm.value = true
    void nextTick(() => {
      syncingForm.value = false
    })
    antigravityMixedChannelConfirmed.value = false
    showMixedChannelWarning.value = false
    mixedChannelWarningDetails.value = null
    mixedChannelWarningRawMessage.value = ''
    mixedChannelWarningAction.value = null
    form.name = newAccount.name
    form.notes = newAccount.notes || ''
    form.proxy_id = newAccount.proxy_id
    form.concurrency = newAccount.concurrency
    form.load_factor = newAccount.load_factor ?? null
    form.priority = newAccount.priority
    form.rate_multiplier = newAccount.rate_multiplier ?? 1
    form.status = (newAccount.status === 'active' || newAccount.status === 'inactive' || newAccount.status === 'error')
      ? newAccount.status
      : 'active'
    form.group_ids = newAccount.group_ids || []
    form.expires_at = newAccount.expires_at ?? null

    // Load intercept warmup requests setting (applies to all account types)
    const credentials = newAccount.credentials as Record<string, unknown> | undefined
    interceptWarmupRequests.value = credentials?.intercept_warmup_requests === true
    autoPauseOnExpired.value = newAccount.auto_pause_on_expired === true
    editVertexProjectId.value = ''
    editVertexClientEmail.value = ''
    editVertexLocation.value = 'us-central1'
    antigravityProjectId.value =
      newAccount.platform === 'antigravity' &&
      newAccount.type === 'oauth' &&
      typeof credentials?.antigravity_project_id === 'string'
        ? credentials.antigravity_project_id.trim()
        : ''

    // Load mixed scheduling setting (only for antigravity accounts)
    mixedScheduling.value = false
    allowOverages.value = false
    const extra = newAccount.extra as Record<string, unknown> | undefined
    deviceLearningEnabled.value = extra?.device_learning_enabled === true
    deviceTLSProfileId.value = readDeviceTLSProfileId(extra?.device_tls_profile_id)
    mixedScheduling.value = extra?.mixed_scheduling === true
    allowOverages.value = extra?.allow_overages === true
    autoPause5hThreshold.value = typeof extra?.auto_pause_5h_threshold === 'number' ? extra.auto_pause_5h_threshold * 100 : null
    autoPause7dThreshold.value = typeof extra?.auto_pause_7d_threshold === 'number' ? extra.auto_pause_7d_threshold * 100 : null
    autoPause5hDisabled.value = extra?.auto_pause_5h_disabled === true
    autoPause7dDisabled.value = extra?.auto_pause_7d_disabled === true
    autoResetCreditEnabled.value = extra?.auto_reset_credit_enabled === true
    autoResetCredit5hThreshold.value =
      typeof extra?.auto_reset_credit_5h_threshold === 'number' ? extra.auto_reset_credit_5h_threshold * 100 : 100
    autoResetCredit7dThreshold.value =
      typeof extra?.auto_reset_credit_7d_threshold === 'number' ? extra.auto_reset_credit_7d_threshold * 100 : 100
    upstreamBillingAutoProbeEnabled.value = extra?.upstream_billing_probe_enabled === true
    upstreamBillingRateSyncEnabled.value =
      upstreamBillingAutoProbeEnabled.value && extra?.upstream_billing_rate_sync_enabled === true

    // Load OpenAI passthrough toggle (OpenAI OAuth/SetupToken/API Key)
    openaiPassthroughEnabled.value = false
    openaiFlattenNamespacesEnabled.value = false
    openAILongContextBillingEnabled.value = false
    editPlanType.value = ''
    openAICompactMode.value = 'auto'
    openAIResponsesMode.value = 'auto'
    openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
    openAICompactModelMappings.value = []
    openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
    openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
    codexCLIOnlyEnabled.value = false
    codexCLIOnlyAppServerEnabled.value = false
    codexFingerprintMode.value = 'off'
    codexImageToolMode.value = 'inherit'
    anthropicPassthroughEnabled.value = false
    anthropicAPIKeyAuthScheme.value = 'x_api_key'
    webSearchEmulationMode.value = 'default'
    if (newAccount.platform === 'openai' && (newAccount.type === 'oauth' || newAccount.type === 'setup-token' || newAccount.type === 'apikey')) {
      openaiPassthroughEnabled.value = extra?.openai_passthrough === true || extra?.openai_oauth_passthrough === true
      openaiFlattenNamespacesEnabled.value =
        newAccount.type === 'oauth' && extra?.openai_responses_flatten_namespaces === true
      const longContextBillingValue = extra?.openai_long_context_billing_enabled
      openAILongContextBillingEnabled.value = longContextBillingValue === true
      // plan_type 手动覆盖仅 OAuth 有实际调度语义(IsOpenAIChatGPTSubscription 要求 oauth),故只对 oauth 回填
      editPlanType.value = newAccount.type === 'oauth'
        ? readPlanType(newAccount.credentials as Record<string, unknown> | undefined)
        : ''
      openAICompactMode.value = (extra?.openai_compact_mode as OpenAICompactMode) || 'auto'
      if (newAccount.type === 'apikey') {
        openAIResponsesMode.value = normalizeOpenAIResponsesMode(extra?.openai_responses_mode)
        openAIEndpointCapabilities.value = readOpenAIEndpointCapabilities(
          newAccount.credentials as Record<string, unknown> | undefined
        )
        if (!openAITextGenerationCapabilityEnabled.value) {
          openAIResponsesMode.value = 'auto'
        }
      }
      const codexImageGenerationBridgeValue = typeof extra?.codex_image_generation_bridge === 'boolean'
        ? extra.codex_image_generation_bridge
        : extra?.codex_image_generation_bridge_enabled
      if (extra?.codex_image_generation_explicit_tool_policy === 'strip') {
        codexImageToolMode.value = 'block'
      } else if (codexImageGenerationBridgeValue === true) {
        codexImageToolMode.value = 'enabled'
      } else if (codexImageGenerationBridgeValue === false) {
        codexImageToolMode.value = 'disabled'
      }
      openaiOAuthResponsesWebSocketV2Mode.value = resolveOpenAIWSModeFromExtra(extra, {
        modeKey: 'openai_oauth_responses_websockets_v2_mode',
        enabledKey: 'openai_oauth_responses_websockets_v2_enabled',
        fallbackEnabledKeys: ['responses_websockets_v2_enabled', 'openai_ws_enabled'],
        defaultMode: OPENAI_WS_MODE_OFF
      })
      openaiAPIKeyResponsesWebSocketV2Mode.value = resolveOpenAIWSModeFromExtra(extra, {
        modeKey: 'openai_apikey_responses_websockets_v2_mode',
        enabledKey: 'openai_apikey_responses_websockets_v2_enabled',
        fallbackEnabledKeys: ['responses_websockets_v2_enabled', 'openai_ws_enabled'],
        defaultMode: OPENAI_WS_MODE_OFF
      })
      if (newAccount.type === 'oauth' || newAccount.type === 'setup-token') {
        codexCLIOnlyEnabled.value = extra?.codex_cli_only === true
        codexCLIOnlyAppServerEnabled.value =
          extra?.codex_cli_only_allow_app_server === true
      }
      if (newAccount.type === 'oauth') {
        const fpMode = extra?.codex_fingerprint_mode as string | undefined
        // 缺省/非法值按 off 呈现，与后端 GetCodexFingerprintMode 的 opt-in 语义一致（#5610）
        codexFingerprintMode.value = (['off', 'device', 'session', 'full'].includes(fpMode || '')
          ? fpMode as CodexFingerprintMode
          : 'off')
      }
      const credentials = newAccount.credentials as Record<string, unknown> | undefined
      const compactMappings = credentials?.compact_model_mapping as Record<string, string> | undefined
      if (compactMappings && typeof compactMappings === 'object') {
        openAICompactModelMappings.value = Object.entries(compactMappings).map(([from, to]) => ({ from, to }))
      }
    }
    if (newAccount.platform === 'anthropic' && newAccount.type === 'apikey') {
      anthropicPassthroughEnabled.value = extra?.anthropic_passthrough === true
      anthropicAPIKeyAuthScheme.value = extra?.anthropic_apikey_auth_scheme === 'authorization_bearer'
        ? 'authorization_bearer'
        : 'x_api_key'
      // 三态：string "default"/"enabled"/"disabled"，向后兼容旧 bool
      const wsVal = extra?.web_search_emulation
      if (wsVal === 'enabled' || wsVal === 'disabled') {
        webSearchEmulationMode.value = wsVal
      } else if (wsVal === true) {
        webSearchEmulationMode.value = 'enabled'
      } else {
        webSearchEmulationMode.value = 'default'
      }
    }

    // Load quota limit for apikey/bedrock accounts (bedrock quota is also loaded in its own branch above)
    if (newAccount.type === 'apikey' || newAccount.type === 'bedrock') {
      const quotaVal = extra?.quota_limit as number | undefined
      editQuotaLimit.value = (quotaVal && quotaVal > 0) ? quotaVal : null
      const dailyVal = extra?.quota_daily_limit as number | undefined
      editQuotaDailyLimit.value = (dailyVal && dailyVal > 0) ? dailyVal : null
      const weeklyVal = extra?.quota_weekly_limit as number | undefined
      editQuotaWeeklyLimit.value = (weeklyVal && weeklyVal > 0) ? weeklyVal : null
      // Load quota reset mode config
      editDailyResetMode.value = (extra?.quota_daily_reset_mode as 'rolling' | 'fixed') || null
      editDailyResetHour.value = (extra?.quota_daily_reset_hour as number) ?? null
      editWeeklyResetMode.value = (extra?.quota_weekly_reset_mode as 'rolling' | 'fixed') || null
      editWeeklyResetDay.value = (extra?.quota_weekly_reset_day as number) ?? null
      editWeeklyResetHour.value = (extra?.quota_weekly_reset_hour as number) ?? null
      editResetTimezone.value = (extra?.quota_reset_timezone as string) || null
      // Load quota notify config
      loadQuotaNotifyFromExtra(extra)
    } else {
      editQuotaLimit.value = null
      editQuotaDailyLimit.value = null
      editQuotaWeeklyLimit.value = null
      editDailyResetMode.value = null
      editDailyResetHour.value = null
      editWeeklyResetMode.value = null
      editWeeklyResetDay.value = null
      editWeeklyResetHour.value = null
      editResetTimezone.value = null
      resetQuotaNotify()
    }

    // Load antigravity model mapping (Antigravity 只支持映射模式)
    if (newAccount.platform === 'antigravity') {
      const credentials = newAccount.credentials as Record<string, unknown> | undefined

      // Antigravity 始终使用映射模式
      antigravityModelRestrictionMode.value = 'mapping'
      antigravityWhitelistModels.value = []

      // 从 model_mapping 读取映射配置
      const rawAgMapping = credentials?.model_mapping as Record<string, string> | undefined
      if (rawAgMapping && typeof rawAgMapping === 'object') {
        const entries = Object.entries(rawAgMapping)
        // 无论是白名单样式(key===value)还是真正的映射，都统一转换为映射列表
        antigravityModelMappings.value = entries.map(([from, to]) => ({ from, to }))
      } else {
        // 兼容旧数据：从 model_whitelist 读取，转换为映射格式
        const rawWhitelist = credentials?.model_whitelist
        if (Array.isArray(rawWhitelist) && rawWhitelist.length > 0) {
          antigravityModelMappings.value = rawWhitelist
            .map((v) => String(v).trim())
            .filter((v) => v.length > 0)
            .map((m) => ({ from: m, to: m }))
        } else {
          antigravityModelMappings.value = []
        }
      }
    } else {
      antigravityModelRestrictionMode.value = 'mapping'
      antigravityWhitelistModels.value = []
      antigravityModelMappings.value = []
    }

    // Load quota control settings (Anthropic OAuth/SetupToken only)
    loadQuotaControlSettings(newAccount)

    loadTempUnschedRules(credentials)
    loadAccountSchedulingThresholdOverride(newAccount.platform, credentials)

    // Load header override state for eligible account platforms/types
    headerOverrideEnabled.value = false
    headerOverrideRows.value = []
    if (newAccount.credentials && isHeaderOverrideCapable(newAccount.platform, newAccount.type)) {
      const overrideCreds = newAccount.credentials as Record<string, unknown>
      headerOverrideEnabled.value = overrideCreds[HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY] === true
      headerOverrideRows.value = splitHeaderOverridesObject(
        overrideCreds[HEADER_OVERRIDES_CREDENTIAL_KEY]
      )
    }

    // Load Grok OAuth custom upstream URL state（存储的官方地址视同未定制）
    grokOAuthCustomBaseUrlEnabled.value = false
    grokOAuthBaseUrl.value = ''
    const grokClientToolCacheSetting =
      newAccount.platform === 'grok' && newAccount.type === 'oauth'
        ? newAccount.extra?.[GROK_CLIENT_TOOL_CACHE_EXTRA_KEY]
        : undefined
    grokClientToolCacheEnabled.value =
      newAccount.platform === 'grok' &&
      newAccount.type === 'oauth' &&
      (grokClientToolCacheSetting === undefined || grokClientToolCacheSetting === true)
    if (newAccount.platform === 'grok' && newAccount.type === 'oauth' && newAccount.credentials) {
      const grokCreds = newAccount.credentials as Record<string, unknown>
      if (isCustomGrokBaseUrl(grokCreds.base_url)) {
        grokOAuthCustomBaseUrlEnabled.value = true
        grokOAuthBaseUrl.value = (grokCreds.base_url as string).trim()
      }
    }

    // Initialize API Key fields for apikey type
    if (newAccount.platform === 'kiro') {
      loadModelRestrictionFromMapping(
        ((newAccount.credentials || {}) as Record<string, unknown>).model_mapping as Record<string, unknown> | undefined
      )
      discoveredKiroProfiles.value = []
      selectedKiroProfileArnChoice.value = KIRO_PROFILE_CHOICE_KEEP
    }

    if (newAccount.platform === 'kiro' && newAccount.type === 'apikey' && newAccount.credentials) {
      editBaseUrl.value = ''
      poolModeEnabled.value = false
      poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
      poolModeRetryStatusCodesInput.value = ''
      customErrorCodesEnabled.value = false
      selectedErrorCodes.value = []
    } else if (newAccount.type === 'apikey' && newAccount.credentials) {
      const credentials = newAccount.credentials as Record<string, unknown>
      // 国产供应商：读取 account_mode 与 api_protocol 作为可编辑初始值
      // （编辑弹窗允许修正两者，用于修复早期存错默认值的账号）。
      if (newAccount.platform === 'kimi' || newAccount.platform === 'zhipu' || newAccount.platform === 'deepseek') {
        editAccountMode.value = credentials.account_mode === 'coding' ? 'coding' : 'payg'
        const storedProtocol = credentials.api_protocol
        editApiProtocol.value =
          storedProtocol === 'adaptive' ||
          storedProtocol === 'chat_completions' ||
          storedProtocol === 'anthropic' ||
          storedProtocol === 'responses'
            ? storedProtocol
            : 'chat_completions'
        if (!cnSupportsNativeResponses(newAccount.platform) && editApiProtocol.value === 'responses') {
          editApiProtocol.value = 'chat_completions'
        }
        const adaptiveDefaults = defaultCNAdaptiveBaseUrls(newAccount.platform, editAccountMode.value)
        const storedBaseUrls = (credentials.api_base_urls as Record<string, unknown> | undefined) || {}
        const legacyBaseUrl = typeof credentials.base_url === 'string' ? credentials.base_url.trim() : ''
        const storedChatBaseUrl = typeof storedBaseUrls.chat_completions === 'string'
          ? storedBaseUrls.chat_completions.trim()
          : ''
        const storedAnthropicBaseUrl = typeof storedBaseUrls.anthropic === 'string'
          ? storedBaseUrls.anthropic.trim()
          : ''
        const storedResponsesBaseUrl = typeof storedBaseUrls.responses === 'string'
          ? storedBaseUrls.responses.trim()
          : ''
        const nextAdaptiveBaseUrls: Record<CnNativeApiProtocol, string> = {
          chat_completions: storedChatBaseUrl || adaptiveDefaults.chat_completions,
          anthropic: storedAnthropicBaseUrl || adaptiveDefaults.anthropic,
          responses: storedResponsesBaseUrl || adaptiveDefaults.responses
        }
        const legacyProtocol: CnNativeApiProtocol = editApiProtocol.value === 'anthropic'
          ? 'anthropic'
          : editApiProtocol.value === 'responses'
            ? 'responses'
            : 'chat_completions'
        const storedLegacyBaseUrl = legacyProtocol === 'anthropic'
          ? storedAnthropicBaseUrl
          : legacyProtocol === 'responses'
            ? storedResponsesBaseUrl
            : storedChatBaseUrl
        if (legacyBaseUrl && !storedLegacyBaseUrl) {
          nextAdaptiveBaseUrls[legacyProtocol] = legacyBaseUrl
        }
        editAdaptiveBaseUrls.value = nextAdaptiveBaseUrls
        // 智谱团队版 Coding Plan：回填组织/项目 ID
        if (newAccount.platform === 'zhipu') {
          editZhipuOrganization.value = typeof credentials.zhipu_organization === 'string' ? credentials.zhipu_organization : ''
          editZhipuProject.value = typeof credentials.zhipu_project === 'string' ? credentials.zhipu_project : ''
        }
      }
      const platformDefaultUrl =
        newAccount.platform === 'openai'
          ? 'https://api.openai.com'
          : newAccount.platform === 'gemini'
            ? 'https://generativelanguage.googleapis.com'
            : newAccount.platform === 'grok'
              ? 'https://api.x.ai/v1'
              : newAccount.platform === 'kimi' ||
                  newAccount.platform === 'zhipu' ||
                  newAccount.platform === 'deepseek'
                ? defaultCNBaseUrl(newAccount.platform, editAccountMode.value, editApiProtocol.value)
                : 'https://api.anthropic.com'
      editBaseUrl.value = isCNApiKeyAccount.value && editApiProtocol.value === 'adaptive'
        ? editAdaptiveBaseUrls.value.chat_completions
        : (credentials.base_url as string) || platformDefaultUrl

      // Load model mappings and detect mode
      loadModelRestrictionFromMapping(credentials.model_mapping as Record<string, unknown> | undefined)

      // Load pool mode
      poolModeEnabled.value = credentials.pool_mode === true
      poolModeRetryCount.value = normalizePoolModeRetryCount(
        Number(credentials.pool_mode_retry_count ?? DEFAULT_POOL_MODE_RETRY_COUNT)
      )
      poolModeRetryStatusCodesInput.value = formatPoolModeRetryStatusCodes(credentials.pool_mode_retry_status_codes)

      // Load custom error codes
      customErrorCodesEnabled.value = credentials.custom_error_codes_enabled === true
      const existingErrorCodes = credentials.custom_error_codes as number[] | undefined
      if (existingErrorCodes && Array.isArray(existingErrorCodes)) {
        selectedErrorCodes.value = [...existingErrorCodes]
      } else {
        selectedErrorCodes.value = []
      }

    } else if (newAccount.type === 'bedrock' && newAccount.credentials) {
      const bedrockCreds = newAccount.credentials as Record<string, unknown>
      const authMode = (bedrockCreds.auth_mode as string) || 'sigv4'
      editBedrockRegion.value = (bedrockCreds.aws_region as string) || ''
      editBedrockForceGlobal.value = (bedrockCreds.aws_force_global as string) === 'true'

      if (authMode === 'apikey') {
        editBedrockApiKeyValue.value = ''
      } else {
        editBedrockAccessKeyId.value = (bedrockCreds.aws_access_key_id as string) || ''
        editBedrockSecretAccessKey.value = ''
        editBedrockSessionToken.value = ''
      }

      // Load pool mode for bedrock
      poolModeEnabled.value = bedrockCreds.pool_mode === true
      const retryCount = bedrockCreds.pool_mode_retry_count
      poolModeRetryCount.value = (typeof retryCount === 'number' && retryCount >= 0) ? retryCount : DEFAULT_POOL_MODE_RETRY_COUNT
      poolModeRetryStatusCodesInput.value = formatPoolModeRetryStatusCodes(bedrockCreds.pool_mode_retry_status_codes)

      // Load quota limits for bedrock
      const bedrockExtra = (newAccount.extra as Record<string, unknown>) || {}
      editQuotaLimit.value = typeof bedrockExtra.quota_limit === 'number' ? bedrockExtra.quota_limit : null
      editQuotaDailyLimit.value = typeof bedrockExtra.quota_daily_limit === 'number' ? bedrockExtra.quota_daily_limit : null
      editQuotaWeeklyLimit.value = typeof bedrockExtra.quota_weekly_limit === 'number' ? bedrockExtra.quota_weekly_limit : null
      // Load quota notify for bedrock
      loadQuotaNotifyFromExtra(bedrockExtra)

      // Load model mappings for bedrock
      loadModelRestrictionFromMapping(bedrockCreds.model_mapping as Record<string, unknown> | undefined)
    } else if (newAccount.type === 'upstream' && newAccount.credentials) {
      const credentials = newAccount.credentials as Record<string, unknown>
      editBaseUrl.value = (credentials.base_url as string) || ''
    } else if (newAccount.platform === 'kiro') {
      editBaseUrl.value = ''
      poolModeEnabled.value = false
      poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
      poolModeRetryStatusCodesInput.value = ''
      customErrorCodesEnabled.value = false
      selectedErrorCodes.value = []
    } else if ((newAccount.platform === 'gemini' || newAccount.platform === 'anthropic') && newAccount.type === 'service_account' && newAccount.credentials) {
      const credentials = newAccount.credentials as Record<string, unknown>
      editVertexProjectId.value = (credentials.project_id as string) || ''
      editVertexClientEmail.value = (credentials.client_email as string) || ''
      editVertexLocation.value = (credentials.location as string) || (credentials.vertex_location as string) || 'us-central1'

      // Load model mappings for service_account
      loadModelRestrictionFromMapping(credentials.model_mapping as Record<string, unknown> | undefined)
    } else {
      const platformDefaultUrl =
        newAccount.platform === 'openai'
          ? 'https://api.openai.com'
          : newAccount.platform === 'gemini'
            ? 'https://generativelanguage.googleapis.com'
            : newAccount.platform === 'grok'
              ? 'https://api.x.ai/v1'
              : 'https://api.anthropic.com'
      editBaseUrl.value = platformDefaultUrl

      // Load model mappings for OpenAI/Grok OAuth accounts
      if ((newAccount.platform === 'openai' || newAccount.platform === 'grok') && newAccount.credentials) {
        const oauthCredentials = newAccount.credentials as Record<string, unknown>
        loadModelRestrictionFromMapping(oauthCredentials.model_mapping as Record<string, unknown> | undefined)
      } else {
        modelRestrictionMode.value = 'whitelist'
        modelMappings.value = []
        allowedModels.value = []
      }
      poolModeEnabled.value = false
      poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
      poolModeRetryStatusCodesInput.value = ''
      customErrorCodesEnabled.value = false
      selectedErrorCodes.value = []
    }
    editApiKey.value = ''
  }

  return { syncFormFromAccount }
}
