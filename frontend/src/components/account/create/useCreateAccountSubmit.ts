// Extracted verbatim from CreateAccountModal.vue's <script setup>: the
// handleSubmit "step 1 -> create" dispatcher (OAuth continue / Bedrock direct
// create / Antigravity upstream direct create / Kiro API-key direct create /
// Vertex service-account direct create / generic API-key create). Pure
// mechanical relocation — the composable receives the host's own refs and
// functions as `deps` and returns the exact same `handleSubmit` name so the
// host can bind the template to an identical function as before.
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { buildModelMappingObject } from '@/composables/useModelWhitelist'
import {
  applyHeaderOverride,
  applyInterceptWarmup,
  defaultCNAdaptiveBaseUrls,
  defaultCNBaseUrl,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows,
  type CnAccountMode,
  type CnApiProtocol,
  type CnNativeApiProtocol,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import type { ComputedRef, Ref } from 'vue'
import type { AccountType, CreateAccountRequest } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

export interface CreateAccountSubmitDeps {
  form: any
  isOAuthFlow: ComputedRef<boolean>
  ensureAntigravityMixedChannelConfirmed: (onConfirm: () => Promise<void>) => Promise<boolean>
  step: Ref<number>
  accountCategory: Ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>
  bedrockAuthMode: Ref<'sigv4' | 'apikey'>
  bedrockAccessKeyId: Ref<string>
  bedrockSecretAccessKey: Ref<string>
  bedrockSessionToken: Ref<string>
  bedrockApiKeyValue: Ref<string>
  bedrockRegion: Ref<string>
  bedrockForceGlobal: Ref<boolean>
  modelRestrictionMode: Ref<'whitelist' | 'mapping'>
  allowedModels: Ref<string[]>
  modelMappings: Ref<ModelMapping[]>
  poolModeEnabled: Ref<boolean>
  poolModeRetryCount: Ref<number>
  poolModeRetryStatusCodesInput: Ref<string>
  normalizePoolModeRetryCount: (value: number) => number
  parsePoolModeRetryStatusCodes: (input: string) => number[]
  interceptWarmupRequests: Ref<boolean>
  createAccountAndFinish: (
    platform: any,
    type: AccountType,
    credentials: Record<string, unknown>,
    extra?: Record<string, unknown>,
    nameOverride?: string
  ) => Promise<void>
  antigravityAccountType: Ref<'oauth' | 'upstream'>
  upstreamBaseUrl: Ref<string>
  upstreamApiKey: Ref<string>
  antigravityModelMappings: Ref<ModelMapping[]>
  buildAntigravityExtra: () => Record<string, unknown> | undefined
  kiroAccountType: Ref<'oauth' | 'apikey'>
  kiroAPIKeyValue: Ref<string>
  kiroRegion: Ref<string>
  kiroAuthRegion: Ref<string>
  kiroAPIRegion: Ref<string>
  kiroProfileARN: Ref<string>
  kiroMachineID: Ref<string>
  applyKiroModelRestriction: (credentials: Record<string, unknown>) => void
  parseVertexServiceAccountJson: () => boolean
  vertexServiceAccountJson: Ref<string>
  vertexLocation: Ref<string>
  vertexProjectId: Ref<string>
  vertexClientEmail: Ref<string>
  apiKeyValue: Ref<string>
  apiKeyBaseUrl: Ref<string>
  geminiTierAIStudio: Ref<'aistudio_free' | 'aistudio_paid'>
  accountMode: Ref<CnAccountMode>
  apiProtocol: Ref<CnApiProtocol>
  cnAdaptiveProtocolOptions: ComputedRef<Array<{ value: CnNativeApiProtocol; labelKey: string }>>
  adaptiveBaseUrls: Ref<Record<CnNativeApiProtocol, string>>
  zhipuOrganization: Ref<string>
  zhipuProject: Ref<string>
  isOpenAIModelRestrictionDisabled: ComputedRef<boolean>
  applyOpenAIEndpointCapabilities: (credentials: Record<string, unknown>) => void
  buildOpenAICompactModelMapping: () => any
  customErrorCodesEnabled: Ref<boolean>
  selectedErrorCodes: Ref<number[]>
  headerOverrideEnabled: Ref<boolean>
  headerOverrideRows: Ref<HeaderOverrideRow[]>
  applyTempUnschedConfig: (credentials: Record<string, unknown>) => boolean
  buildAnthropicExtra: (base?: Record<string, unknown>) => Record<string, unknown> | undefined
  buildOpenAIExtra: (base?: Record<string, unknown>) => Record<string, unknown> | undefined
  doCreateAccount: (payload: CreateAccountRequest) => Promise<void>
  upstreamBillingAutoProbeEnabled: Ref<boolean>
  autoPauseOnExpired: Ref<boolean>
}

export function useCreateAccountSubmit(deps: CreateAccountSubmitDeps) {
  const { t } = useI18n()
  const appStore = useAppStore()
  const {
    form,
    isOAuthFlow,
    ensureAntigravityMixedChannelConfirmed,
    step,
    accountCategory,
    bedrockAuthMode,
    bedrockAccessKeyId,
    bedrockSecretAccessKey,
    bedrockSessionToken,
    bedrockApiKeyValue,
    bedrockRegion,
    bedrockForceGlobal,
    modelRestrictionMode,
    allowedModels,
    modelMappings,
    poolModeEnabled,
    poolModeRetryCount,
    poolModeRetryStatusCodesInput,
    normalizePoolModeRetryCount,
    parsePoolModeRetryStatusCodes,
    interceptWarmupRequests,
    createAccountAndFinish,
    antigravityAccountType,
    upstreamBaseUrl,
    upstreamApiKey,
    antigravityModelMappings,
    buildAntigravityExtra,
    kiroAccountType,
    kiroAPIKeyValue,
    kiroRegion,
    kiroAuthRegion,
    kiroAPIRegion,
    kiroProfileARN,
    kiroMachineID,
    applyKiroModelRestriction,
    parseVertexServiceAccountJson,
    vertexServiceAccountJson,
    vertexLocation,
    vertexProjectId,
    vertexClientEmail,
    apiKeyValue,
    apiKeyBaseUrl,
    geminiTierAIStudio,
    accountMode,
    apiProtocol,
    cnAdaptiveProtocolOptions,
    adaptiveBaseUrls,
    zhipuOrganization,
    zhipuProject,
    isOpenAIModelRestrictionDisabled,
    applyOpenAIEndpointCapabilities,
    buildOpenAICompactModelMapping,
    customErrorCodesEnabled,
    selectedErrorCodes,
    headerOverrideEnabled,
    headerOverrideRows,
    applyTempUnschedConfig,
    buildAnthropicExtra,
    buildOpenAIExtra,
    doCreateAccount,
    upstreamBillingAutoProbeEnabled,
    autoPauseOnExpired
  } = deps

  const handleSubmit = async () => {
    // For OAuth-based type, handle OAuth flow (goes to step 2)
    if (isOAuthFlow.value) {
      const canContinue = await ensureAntigravityMixedChannelConfirmed(async () => {
        step.value = 2
      })
      if (!canContinue) {
        return
      }
      step.value = 2
      return
    }

    // For Bedrock type, create directly
    if (form.platform === 'anthropic' && accountCategory.value === 'bedrock') {
      if (!form.name.trim()) {
        appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
        return
      }

      const credentials: Record<string, unknown> = {
        auth_mode: bedrockAuthMode.value,
        aws_region: bedrockRegion.value.trim() || 'us-east-1',
      }

      if (bedrockAuthMode.value === 'sigv4') {
        if (!bedrockAccessKeyId.value.trim()) {
          appStore.showError(t('admin.accounts.bedrockAccessKeyIdRequired'))
          return
        }
        if (!bedrockSecretAccessKey.value.trim()) {
          appStore.showError(t('admin.accounts.bedrockSecretAccessKeyRequired'))
          return
        }
        credentials.aws_access_key_id = bedrockAccessKeyId.value.trim()
        credentials.aws_secret_access_key = bedrockSecretAccessKey.value.trim()
        if (bedrockSessionToken.value.trim()) {
          credentials.aws_session_token = bedrockSessionToken.value.trim()
        }
      } else {
        if (!bedrockApiKeyValue.value.trim()) {
          appStore.showError(t('admin.accounts.bedrockApiKeyRequired'))
          return
        }
        credentials.api_key = bedrockApiKeyValue.value.trim()
      }

      if (bedrockForceGlobal.value) {
        credentials.aws_force_global = 'true'
      }

      // Model mapping
      const modelMapping = buildModelMappingObject(
        modelRestrictionMode.value, allowedModels.value, modelMappings.value
      )
      if (modelMapping) {
        credentials.model_mapping = modelMapping
      }

      // Pool mode
      if (poolModeEnabled.value) {
        credentials.pool_mode = true
        credentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
        const parsedRetryStatusCodes = parsePoolModeRetryStatusCodes(poolModeRetryStatusCodesInput.value)
        if (parsedRetryStatusCodes.length > 0) {
          credentials.pool_mode_retry_status_codes = parsedRetryStatusCodes
        }
      }

      applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')

      await createAccountAndFinish('anthropic', 'bedrock' as AccountType, credentials)
      return
    }

    // For Antigravity upstream type, create directly
    if (form.platform === 'antigravity' && antigravityAccountType.value === 'upstream') {
      if (!form.name.trim()) {
        appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
        return
      }
      if (!upstreamBaseUrl.value.trim()) {
        appStore.showError(t('admin.accounts.upstream.pleaseEnterBaseUrl'))
        return
      }
      if (!upstreamApiKey.value.trim()) {
        appStore.showError(t('admin.accounts.upstream.pleaseEnterApiKey'))
        return
      }

      // Build upstream credentials (and optional model restriction)
      const credentials: Record<string, unknown> = {
        base_url: upstreamBaseUrl.value.trim(),
        api_key: upstreamApiKey.value.trim()
      }

      // Antigravity 只使用映射模式
      const antigravityModelMapping = buildModelMappingObject(
        'mapping',
        [],
        antigravityModelMappings.value
      )
      if (antigravityModelMapping) {
        credentials.model_mapping = antigravityModelMapping
      }

      applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')

      const extra = buildAntigravityExtra()
      await createAccountAndFinish(form.platform, 'apikey', credentials, extra)
      return
    }

    if (form.platform === 'kiro' && kiroAccountType.value === 'apikey') {
      if (!kiroAPIKeyValue.value.trim()) {
        appStore.showError(t('admin.accounts.pleaseEnterApiKey'))
        return
      }

      const credentials: Record<string, unknown> = {
        api_key: kiroAPIKeyValue.value.trim(),
        region: kiroRegion.value.trim() || 'us-east-1'
      }
      if (kiroAuthRegion.value.trim()) {
        credentials.auth_region = kiroAuthRegion.value.trim()
      }
      if (kiroAPIRegion.value.trim()) {
        credentials.api_region = kiroAPIRegion.value.trim()
      }
      if (kiroProfileARN.value.trim()) {
        credentials.profile_arn = kiroProfileARN.value.trim()
      }
      if (kiroMachineID.value.trim()) {
        credentials.machine_id = kiroMachineID.value.trim()
      }
      applyKiroModelRestriction(credentials)

      await createAccountAndFinish('kiro', 'apikey', credentials)
      return
    }

    if ((form.platform === 'gemini' || form.platform === 'anthropic') && accountCategory.value === 'service_account') {
      if (!form.name.trim()) {
        appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
        return
      }
      if (!parseVertexServiceAccountJson()) {
        return
      }
      if (!vertexLocation.value.trim()) {
        appStore.showError(t('admin.accounts.vertexLocationRequired'))
        return
      }
      const credentials: Record<string, unknown> = {
        service_account_json: vertexServiceAccountJson.value.trim(),
        project_id: vertexProjectId.value.trim(),
        client_email: vertexClientEmail.value.trim(),
        location: vertexLocation.value.trim(),
        tier_id: 'vertex'
      }
      await createAccountAndFinish(form.platform, 'service_account' as AccountType, credentials)
      return
    }

    // For apikey type, create directly
    if (!apiKeyValue.value.trim()) {
      appStore.showError(t('admin.accounts.pleaseEnterApiKey'))
      return
    }

    // Determine default base URL based on platform
    const defaultBaseUrl =
      form.platform === 'openai'
        ? 'https://api.openai.com'
        : form.platform === 'gemini'
          ? 'https://generativelanguage.googleapis.com'
          : form.platform === 'grok'
            ? 'https://api.x.ai/v1'
            : 'https://api.anthropic.com'

    // Build credentials with optional model mapping
    const credentials: Record<string, unknown> = {
      base_url: apiKeyBaseUrl.value.trim() || defaultBaseUrl,
      api_key: apiKeyValue.value.trim()
    }
    if (form.platform === 'gemini') {
      credentials.tier_id = geminiTierAIStudio.value
    }

    // 国产供应商：账号模式 + 协议 + 对应端点写入凭据；后端按 account_mode 路由
    // 额度/余额探测，按 api_protocol 路由转发端点与格式。注意 CN apikey 走本函数
    // 的通用路径（直接 doCreateAccount），不经过 createAccountAndFinish。
    if (form.platform === 'kimi' || form.platform === 'zhipu' || form.platform === 'deepseek') {
      credentials.account_mode = accountMode.value
      credentials.api_protocol = apiProtocol.value
      if (apiProtocol.value === 'adaptive') {
        const defaults = defaultCNAdaptiveBaseUrls(form.platform, accountMode.value)
        const protocolBaseUrls: Record<string, string> = {}
        for (const item of cnAdaptiveProtocolOptions.value) {
          protocolBaseUrls[item.value] = (adaptiveBaseUrls.value[item.value] || defaults[item.value]).trim()
        }
        credentials.api_base_urls = protocolBaseUrls
        credentials.base_url = protocolBaseUrls.chat_completions
      }
      const resolvedCNBase = (
        apiKeyBaseUrl.value.trim() || defaultCNBaseUrl(form.platform, accountMode.value, apiProtocol.value)
      ).trim()
      if (apiProtocol.value !== 'adaptive' && resolvedCNBase) {
        credentials.base_url = resolvedCNBase
      }
      // 智谱团队版 Coding Plan：组织/项目 ID 写入凭据（非空才写）
      if (form.platform === 'zhipu' && accountMode.value === 'coding') {
        if (zhipuOrganization.value.trim()) credentials.zhipu_organization = zhipuOrganization.value.trim()
        if (zhipuProject.value.trim()) credentials.zhipu_project = zhipuProject.value.trim()
      }
    }

    // Add model mapping if configured（OpenAI 开启自动透传时不应用）
    if (!isOpenAIModelRestrictionDisabled.value) {
      const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
      if (modelMapping) {
        credentials.model_mapping = modelMapping
      }
    }
    if (form.platform === 'openai') {
      applyOpenAIEndpointCapabilities(credentials)
      const compactModelMapping = buildOpenAICompactModelMapping()
      if (compactModelMapping) {
        credentials.compact_model_mapping = compactModelMapping
      }
    }

    // Add pool mode if enabled
    if (poolModeEnabled.value) {
      credentials.pool_mode = true
      credentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
      const parsedRetryStatusCodes = parsePoolModeRetryStatusCodes(poolModeRetryStatusCodesInput.value)
      if (parsedRetryStatusCodes.length > 0) {
        credentials.pool_mode_retry_status_codes = parsedRetryStatusCodes
      }
    }

    // Add custom error codes if enabled
    if (customErrorCodesEnabled.value) {
      credentials.custom_error_codes_enabled = true
      credentials.custom_error_codes = [...selectedErrorCodes.value]
    }

    // Add header override if enabled for this API-key platform
    if (isHeaderOverrideCapable(form.platform, 'apikey')) {
      if (headerOverrideEnabled.value) {
        const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
        if (headerError) {
          appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
          return
        }
      }
      applyHeaderOverride(credentials, headerOverrideEnabled.value, headerOverrideRows.value, 'create')
    }

    applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
    if (!applyTempUnschedConfig(credentials)) {
      return
    }

    form.credentials = credentials
    const extra = buildAnthropicExtra(buildOpenAIExtra())

    await doCreateAccount({
      ...form,
      group_ids: form.group_ids,
      extra,
      upstream_billing_probe_enabled: upstreamBillingAutoProbeEnabled.value,
      auto_pause_on_expired: autoPauseOnExpired.value
    })
  }

  return { handleSubmit }
}
