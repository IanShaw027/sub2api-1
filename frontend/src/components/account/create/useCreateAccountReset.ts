// Extracted verbatim from CreateAccountModal.vue's <script setup>: the
// resetForm() function that zeroes out every piece of create-flow state when
// the modal is closed/reopened. Pure mechanical relocation — the composable
// receives the host's own refs/reactive objects/functions as `deps` and
// returns the exact same `resetForm` name so the host can bind to an
// identical function as before.
import type { Ref } from 'vue'
import { claudeModels, fetchAntigravityDefaultMappings } from '@/composables/useModelWhitelist'
import { OPENAI_WS_MODE_OFF } from '@/utils/openaiWsMode'

export interface CreateAccountResetDeps {
  step: Ref<number>
  form: any
  accountCategory: Ref<any>
  addMethod: Ref<any>
  accountMode: Ref<any>
  apiProtocol: Ref<any>
  adaptiveBaseUrls: Ref<any>
  apiKeyBaseUrl: Ref<string>
  apiKeyValue: Ref<string>
  upstreamBillingAutoProbeEnabled: Ref<boolean>
  editQuotaLimit: Ref<number | null>
  editQuotaDailyLimit: Ref<number | null>
  editQuotaWeeklyLimit: Ref<number | null>
  editDailyResetMode: Ref<'rolling' | 'fixed' | null>
  editDailyResetHour: Ref<number | null>
  editWeeklyResetMode: Ref<'rolling' | 'fixed' | null>
  editWeeklyResetDay: Ref<number | null>
  editWeeklyResetHour: Ref<number | null>
  editResetTimezone: Ref<string | null>
  modelMappings: Ref<any[]>
  openAICompactModelMappings: Ref<any[]>
  modelRestrictionMode: Ref<'whitelist' | 'mapping'>
  allowedModels: Ref<string[]>
  antigravityModelRestrictionMode: Ref<'whitelist' | 'mapping'>
  antigravityWhitelistModels: Ref<string[]>
  antigravityModelMappings: Ref<any[]>
  DEFAULT_POOL_MODE_RETRY_COUNT: number
  poolModeEnabled: Ref<boolean>
  poolModeRetryCount: Ref<number>
  poolModeRetryStatusCodesInput: Ref<string>
  customErrorCodesEnabled: Ref<boolean>
  selectedErrorCodes: Ref<number[]>
  customErrorCodeInput: Ref<number | null>
  headerOverrideEnabled: Ref<boolean>
  headerOverrideRows: Ref<any[]>
  grokOAuthCustomBaseUrlEnabled: Ref<boolean>
  grokOAuthBaseUrl: Ref<string>
  interceptWarmupRequests: Ref<boolean>
  autoPauseOnExpired: Ref<boolean>
  openaiPassthroughEnabled: Ref<boolean>
  openaiFlattenNamespacesEnabled: Ref<boolean>
  openAILongContextBillingEnabled: Ref<boolean>
  openAILongContextBillingTouched: Ref<boolean>
  openAICompactMode: Ref<any>
  openAIResponsesMode: Ref<any>
  openAIEndpointCapabilities: Ref<any[]>
  openaiOAuthResponsesWebSocketV2Mode: Ref<any>
  openaiAPIKeyResponsesWebSocketV2Mode: Ref<any>
  codexCLIOnlyEnabled: Ref<boolean>
  codexCLIOnlyAppServerEnabled: Ref<boolean>
  codexFingerprintMode: Ref<any>
  anthropicPassthroughEnabled: Ref<boolean>
  anthropicAPIKeyAuthScheme: Ref<any>
  webSearchEmulationMode: Ref<string>
  windowCostEnabled: Ref<boolean>
  windowCostLimit: Ref<number | null>
  windowCostStickyReserve: Ref<number | null>
  sessionLimitEnabled: Ref<boolean>
  maxSessions: Ref<number | null>
  sessionIdleTimeout: Ref<number | null>
  rpmLimitEnabled: Ref<boolean>
  baseRpm: Ref<number | null>
  rpmStrategy: Ref<'tiered' | 'sticky_exempt'>
  rpmStickyBuffer: Ref<number | null>
  userMsgQueueMode: Ref<string>
  tlsFingerprintEnabled: Ref<boolean>
  tlsFingerprintProfileId: Ref<number | null>
  tlsFingerprintRouterId: Ref<number | null>
  tlsFingerprintDefaultOS: Ref<string>
  tlsFingerprintBindingRows: Ref<any[]>
  sessionIdMaskingEnabled: Ref<boolean>
  cacheTTLOverrideEnabled: Ref<boolean>
  cacheTTLOverrideTarget: Ref<string>
  customBaseUrlEnabled: Ref<boolean>
  customBaseUrl: Ref<string>
  allowOverages: Ref<boolean>
  antigravityAccountType: Ref<'oauth' | 'upstream'>
  kiroAccountType: Ref<'oauth' | 'apikey'>
  antigravityProjectId: Ref<string>
  upstreamBaseUrl: Ref<string>
  upstreamApiKey: Ref<string>
  kiroAPIKeyValue: Ref<string>
  kiroRegion: Ref<string>
  kiroAuthRegion: Ref<string>
  kiroAPIRegion: Ref<string>
  kiroProfileARN: Ref<string>
  kiroMachineID: Ref<string>
  vertexServiceAccountJson: Ref<string>
  vertexProjectId: Ref<string>
  vertexClientEmail: Ref<string>
  vertexLocation: Ref<string>
  tempUnschedEnabled: Ref<boolean>
  tempUnschedRules: Ref<any[]>
  geminiOAuthType: Ref<'code_assist' | 'google_one' | 'ai_studio'>
  geminiTierGoogleOne: Ref<any>
  geminiTierGcp: Ref<any>
  geminiTierAIStudio: Ref<any>
  oauth: { resetState: () => void }
  openaiOAuth: { resetState: () => void }
  geminiOAuth: { resetState: () => void }
  antigravityOAuth: { resetState: () => void }
  kiroOAuth: { resetState: () => void }
  grokOAuth: { resetState: () => void }
  oauthFlowRef: Ref<{ reset: () => void } | null>
  antigravityMixedChannelConfirmed: Ref<boolean>
  upstreamModelsPreviewed: Ref<boolean>
  clearMixedChannelDialog: () => void
}

export function useCreateAccountReset(deps: CreateAccountResetDeps) {
  const {
    step,
    form,
    accountCategory,
    addMethod,
    accountMode,
    apiProtocol,
    adaptiveBaseUrls,
    apiKeyBaseUrl,
    apiKeyValue,
    upstreamBillingAutoProbeEnabled,
    editQuotaLimit,
    editQuotaDailyLimit,
    editQuotaWeeklyLimit,
    editDailyResetMode,
    editDailyResetHour,
    editWeeklyResetMode,
    editWeeklyResetDay,
    editWeeklyResetHour,
    editResetTimezone,
    modelMappings,
    openAICompactModelMappings,
    modelRestrictionMode,
    allowedModels,
    antigravityModelRestrictionMode,
    antigravityWhitelistModels,
    antigravityModelMappings,
    DEFAULT_POOL_MODE_RETRY_COUNT,
    poolModeEnabled,
    poolModeRetryCount,
    poolModeRetryStatusCodesInput,
    customErrorCodesEnabled,
    selectedErrorCodes,
    customErrorCodeInput,
    headerOverrideEnabled,
    headerOverrideRows,
    grokOAuthCustomBaseUrlEnabled,
    grokOAuthBaseUrl,
    interceptWarmupRequests,
    autoPauseOnExpired,
    openaiPassthroughEnabled,
    openaiFlattenNamespacesEnabled,
    openAILongContextBillingEnabled,
    openAILongContextBillingTouched,
    openAICompactMode,
    openAIResponsesMode,
    openAIEndpointCapabilities,
    openaiOAuthResponsesWebSocketV2Mode,
    openaiAPIKeyResponsesWebSocketV2Mode,
    codexCLIOnlyEnabled,
    codexCLIOnlyAppServerEnabled,
    codexFingerprintMode,
    anthropicPassthroughEnabled,
    anthropicAPIKeyAuthScheme,
    webSearchEmulationMode,
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
    tlsFingerprintEnabled,
    tlsFingerprintProfileId,
    tlsFingerprintRouterId,
    tlsFingerprintDefaultOS,
    tlsFingerprintBindingRows,
    sessionIdMaskingEnabled,
    cacheTTLOverrideEnabled,
    cacheTTLOverrideTarget,
    customBaseUrlEnabled,
    customBaseUrl,
    allowOverages,
    antigravityAccountType,
    kiroAccountType,
    antigravityProjectId,
    upstreamBaseUrl,
    upstreamApiKey,
    kiroAPIKeyValue,
    kiroRegion,
    kiroAuthRegion,
    kiroAPIRegion,
    kiroProfileARN,
    kiroMachineID,
    vertexServiceAccountJson,
    vertexProjectId,
    vertexClientEmail,
    vertexLocation,
    tempUnschedEnabled,
    tempUnschedRules,
    geminiOAuthType,
    geminiTierGoogleOne,
    geminiTierGcp,
    geminiTierAIStudio,
    oauth,
    openaiOAuth,
    geminiOAuth,
    antigravityOAuth,
    kiroOAuth,
    grokOAuth,
    oauthFlowRef,
    antigravityMixedChannelConfirmed,
    upstreamModelsPreviewed,
    clearMixedChannelDialog
  } = deps

  const resetForm = () => {
    step.value = 1
    form.name = ''
    form.notes = ''
    form.platform = 'anthropic'
    form.type = 'oauth'
    form.credentials = {}
    form.proxy_id = null
    form.concurrency = 10
    form.load_factor = null
    form.priority = 1
    form.rate_multiplier = 1
    form.group_ids = []
    form.expires_at = null
    accountCategory.value = 'oauth-based'
    addMethod.value = 'oauth'
    accountMode.value = 'payg'
    apiProtocol.value = 'adaptive'
    adaptiveBaseUrls.value = { chat_completions: '', anthropic: '', responses: '' }
    apiKeyBaseUrl.value = 'https://api.anthropic.com'
    apiKeyValue.value = ''
    upstreamBillingAutoProbeEnabled.value = true
    editQuotaLimit.value = null
    editQuotaDailyLimit.value = null
    editQuotaWeeklyLimit.value = null
    editDailyResetMode.value = null
    editDailyResetHour.value = null
    editWeeklyResetMode.value = null
    editWeeklyResetDay.value = null
    editWeeklyResetHour.value = null
    editResetTimezone.value = null
    modelMappings.value = []
    openAICompactModelMappings.value = []
    modelRestrictionMode.value = 'whitelist'
    allowedModels.value = [...claudeModels] // Default fill related models

    antigravityModelRestrictionMode.value = 'mapping'
    antigravityWhitelistModels.value = []
    fetchAntigravityDefaultMappings().then(mappings => {
      antigravityModelMappings.value = [...mappings]
    })
    poolModeEnabled.value = false
    poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
    poolModeRetryStatusCodesInput.value = ''
    customErrorCodesEnabled.value = false
    selectedErrorCodes.value = []
    customErrorCodeInput.value = null
    headerOverrideEnabled.value = false
    headerOverrideRows.value = []
    grokOAuthCustomBaseUrlEnabled.value = false
    grokOAuthBaseUrl.value = ''
    interceptWarmupRequests.value = false
    autoPauseOnExpired.value = true
    openaiPassthroughEnabled.value = false
    openaiFlattenNamespacesEnabled.value = false
    openAILongContextBillingEnabled.value = false
    openAILongContextBillingTouched.value = false
    openAICompactMode.value = 'auto'
    openAIResponsesMode.value = 'auto'
    openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
    openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
    openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
    codexCLIOnlyEnabled.value = false
    codexCLIOnlyAppServerEnabled.value = false
    codexFingerprintMode.value = 'off'
    anthropicPassthroughEnabled.value = false
    anthropicAPIKeyAuthScheme.value = 'x_api_key'
    webSearchEmulationMode.value = 'default'
    // Reset quota control state
    windowCostEnabled.value = false
    windowCostLimit.value = null
    windowCostStickyReserve.value = null
    sessionLimitEnabled.value = false
    maxSessions.value = null
    sessionIdleTimeout.value = null
    rpmLimitEnabled.value = false
    baseRpm.value = null
    rpmStrategy.value = 'tiered'
    rpmStickyBuffer.value = null
    userMsgQueueMode.value = ''
    tlsFingerprintEnabled.value = false
    tlsFingerprintProfileId.value = null
    tlsFingerprintRouterId.value = null
    tlsFingerprintDefaultOS.value = ''
    tlsFingerprintBindingRows.value = []
    sessionIdMaskingEnabled.value = false
    cacheTTLOverrideEnabled.value = false
    cacheTTLOverrideTarget.value = '5m'
    customBaseUrlEnabled.value = false
    customBaseUrl.value = ''
    allowOverages.value = false
    antigravityAccountType.value = 'oauth'
    kiroAccountType.value = 'oauth'
    antigravityProjectId.value = ''
    upstreamBaseUrl.value = ''
    upstreamApiKey.value = ''
    kiroAPIKeyValue.value = ''
    kiroRegion.value = 'us-east-1'
    kiroAuthRegion.value = ''
    kiroAPIRegion.value = ''
    kiroProfileARN.value = ''
    kiroMachineID.value = ''
    vertexServiceAccountJson.value = ''
    vertexProjectId.value = ''
    vertexClientEmail.value = ''
    vertexLocation.value = 'global'
    tempUnschedEnabled.value = false
    tempUnschedRules.value = []
    geminiOAuthType.value = 'code_assist'
    geminiTierGoogleOne.value = 'google_one_free'
    geminiTierGcp.value = 'gcp_standard'
    geminiTierAIStudio.value = 'aistudio_free'
    oauth.resetState()
    openaiOAuth.resetState()
    geminiOAuth.resetState()
    antigravityOAuth.resetState()
    kiroOAuth.resetState()
    grokOAuth.resetState()
    oauthFlowRef.value?.reset()
    antigravityMixedChannelConfirmed.value = false
    upstreamModelsPreviewed.value = false
    clearMixedChannelDialog()
  }

  return { resetForm }
}
