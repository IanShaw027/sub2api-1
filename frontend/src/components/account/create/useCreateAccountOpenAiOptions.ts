import { ref, computed, type Ref } from 'vue'
import { adminAPI } from '@/api/admin'
import { buildModelMappingObject } from '@/composables/useModelWhitelist'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'
import {
  OPENAI_WS_MODE_CTX_POOL,
  OPENAI_WS_MODE_OFF,
  OPENAI_WS_MODE_PASSTHROUGH,
  OPENAI_WS_MODE_HTTP_BRIDGE,
  isOpenAIWSModeEnabled,
  resolveOpenAIWSModeConcurrencyHintKey,
  type OpenAIWSMode
} from '@/utils/openaiWsMode'
import type { AccountPlatform, AccountType, OpenAICompactMode, OpenAIResponsesMode, OpenAIEndpointCapability } from '@/types'

export type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
export type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'

interface ModelMapping {
  from: string
  to: string
}

export interface CreateAccountOpenAiOptionsDeps {
  form: { platform: AccountPlatform; type: AccountType }
  accountCategory: Ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>
  t: (key: string, params?: Record<string, unknown>) => string
  openAICompactModelMappings: Ref<ModelMapping[]>
}

export function useCreateAccountOpenAiOptions(deps: CreateAccountOpenAiOptionsDeps) {
  const { form, accountCategory, t, openAICompactModelMappings } = deps

  const openaiPassthroughEnabled = ref(false)
  // OpenAI Codex namespace 工具摊平兼容开关（仅 OAuth），缺省关闭即原样保留
  const openaiFlattenNamespacesEnabled = ref(false)
  const openAILongContextBillingEnabled = ref(false)
  const openAILongContextBillingTouched = ref(false)
  const openAICompactMode = ref<OpenAICompactMode>('auto')
  const openAIResponsesMode = ref<OpenAIResponsesMode>('auto')
  const openAIEndpointCapabilities = ref<OpenAIEndpointCapability[]>(['chat_completions', 'embeddings'])
  const openaiOAuthResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
  const openaiAPIKeyResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
  const codexCLIOnlyEnabled = ref(false)
  const codexCLIOnlyAppServerEnabled = ref(false)
  const codexFingerprintMode = ref<CodexFingerprintMode>('off')
  const codexFingerprintModeOptions = computed(() => [
    { value: 'off' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintOff') },
    { value: 'device' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintDevice') },
    { value: 'session' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintSession') },
    { value: 'full' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintFull') },
  ])
  const anthropicPassthroughEnabled = ref(false)
  const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
  const webSearchEmulationMode = ref('default')
  const webSearchGlobalEnabled = ref(false)

  const toggleOpenAILongContextBilling = () => {
    openAILongContextBillingEnabled.value = !openAILongContextBillingEnabled.value
    openAILongContextBillingTouched.value = true
  }

  // Load global feature states once
  adminAPI.settings.getWebSearchEmulationConfig().then(cfg => {
    webSearchGlobalEnabled.value = cfg?.enabled === true && (cfg?.providers?.length ?? 0) > 0
  }).catch(() => { webSearchGlobalEnabled.value = false })

  const getOpenAICompactModelMappingKey = createStableObjectKeyResolver<ModelMapping>('create-openai-compact-model-mapping')

  const openAICompactModeOptions = computed(() => [
    { value: 'auto', label: t('admin.accounts.openai.compactModeAuto') },
    { value: 'force_on', label: t('admin.accounts.openai.compactModeForceOn') },
    { value: 'force_off', label: t('admin.accounts.openai.compactModeForceOff') }
  ])
  const openAIResponsesModeOptions = computed(() => [
    { value: 'auto', label: t('admin.accounts.openai.responsesModeAuto') },
    { value: 'force_responses', label: t('admin.accounts.openai.responsesModeForceResponses') },
    { value: 'force_chat_completions', label: t('admin.accounts.openai.responsesModeForceChatCompletions') }
  ])
  const openAITextEndpointCapabilityLabel = computed(() => {
    if (openAIResponsesMode.value === 'force_responses') {
      return t('admin.accounts.openai.capabilityResponses')
    }
    if (openAIResponsesMode.value === 'force_chat_completions') {
      return t('admin.accounts.openai.capabilityChatCompletions')
    }
    return t('admin.accounts.openai.capabilityTextAuto')
  })
  const openAIEndpointCapabilityOptions = computed<{ value: OpenAIEndpointCapability; label: string }[]>(() => [
    { value: 'chat_completions', label: openAITextEndpointCapabilityLabel.value },
    { value: 'embeddings', label: t('admin.accounts.openai.capabilityEmbeddings') }
  ])
  const openAITextGenerationCapabilityEnabled = computed(() =>
    openAIEndpointCapabilities.value.includes('chat_completions')
  )

  const normalizeOpenAIEndpointCapabilities = (values: OpenAIEndpointCapability[]) => {
    const allowed: OpenAIEndpointCapability[] = ['chat_completions', 'embeddings']
    const selected = allowed.filter((value) => values.includes(value))
    return selected.length > 0 ? selected : allowed
  }

  const toggleOpenAIEndpointCapability = (capability: OpenAIEndpointCapability, event?: Event) => {
    if (openAIEndpointCapabilities.value.includes(capability)) {
      if (openAIEndpointCapabilities.value.length <= 1) {
        const input = event?.target as HTMLInputElement | null
        if (input) input.checked = true
        return
      }
      openAIEndpointCapabilities.value = openAIEndpointCapabilities.value.filter(
        (value) => value !== capability
      )
      if (!openAITextGenerationCapabilityEnabled.value) {
        openAIResponsesMode.value = 'auto'
      }
      return
    }
    openAIEndpointCapabilities.value = normalizeOpenAIEndpointCapabilities([
      ...openAIEndpointCapabilities.value,
      capability
    ])
  }

  const applyOpenAIEndpointCapabilities = (credentials: Record<string, unknown>) => {
    const capabilities = normalizeOpenAIEndpointCapabilities(openAIEndpointCapabilities.value)
    if (capabilities.length === 2) {
      delete credentials.openai_capabilities
      return
    }
    credentials.openai_capabilities = capabilities
  }

  const buildOpenAICompactModelMapping = () =>
    buildModelMappingObject('mapping', [], openAICompactModelMappings.value)

  // Model mapping helpers
  const addOpenAICompactModelMapping = () => {
    openAICompactModelMappings.value.push({ from: '', to: '' })
  }

  const removeOpenAICompactModelMapping = (index: number) => {
    openAICompactModelMappings.value.splice(index, 1)
  }

  const openAIWSModeOptions = computed(() => [
    { value: OPENAI_WS_MODE_OFF, label: t('admin.accounts.openai.wsModeOff') },
    { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
    { value: OPENAI_WS_MODE_PASSTHROUGH, label: t('admin.accounts.openai.wsModePassthrough') },
    { value: OPENAI_WS_MODE_HTTP_BRIDGE, label: t('admin.accounts.openai.wsModeHttpBridge') }
  ])

  const openaiResponsesWebSocketV2Mode = computed({
    get: () => {
      if (form.platform === 'openai' && accountCategory.value === 'apikey') {
        return openaiAPIKeyResponsesWebSocketV2Mode.value
      }
      return openaiOAuthResponsesWebSocketV2Mode.value
    },
    set: (mode: OpenAIWSMode) => {
      if (form.platform === 'openai' && accountCategory.value === 'apikey') {
        openaiAPIKeyResponsesWebSocketV2Mode.value = mode
        return
      }
      openaiOAuthResponsesWebSocketV2Mode.value = mode
    }
  })

  const openAIWSModeConcurrencyHintKey = computed(() =>
    resolveOpenAIWSModeConcurrencyHintKey(openaiResponsesWebSocketV2Mode.value)
  )

  const isOpenAIModelRestrictionDisabled = computed(() =>
    form.platform === 'openai' && openaiPassthroughEnabled.value
  )

  const buildOpenAIExtra = (base?: Record<string, unknown>): Record<string, unknown> | undefined => {
    if (form.platform !== 'openai') {
      return base
    }

    const extra: Record<string, unknown> = { ...(base || {}) }
    if (accountCategory.value === 'oauth-based') {
      extra.openai_oauth_responses_websockets_v2_mode = openaiOAuthResponsesWebSocketV2Mode.value
      extra.openai_oauth_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiOAuthResponsesWebSocketV2Mode.value)
    } else if (accountCategory.value === 'apikey') {
      extra.openai_apikey_responses_websockets_v2_mode = openaiAPIKeyResponsesWebSocketV2Mode.value
      extra.openai_apikey_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiAPIKeyResponsesWebSocketV2Mode.value)
    }
    // 清理兼容旧键，统一改用分类型开关。
    delete extra.responses_websockets_v2_enabled
    delete extra.openai_ws_enabled
    if (openaiPassthroughEnabled.value) {
      extra.openai_passthrough = true
    } else {
      delete extra.openai_passthrough
      delete extra.openai_oauth_passthrough
    }
    // 缺省即保留 namespace，不写空值，避免 extra 里堆积默认项
    if (form.type === 'oauth' && openaiFlattenNamespacesEnabled.value) {
      extra.openai_responses_flatten_namespaces = true
    } else {
      delete extra.openai_responses_flatten_namespaces
    }
    extra.openai_long_context_billing_enabled = openAILongContextBillingEnabled.value

    if (accountCategory.value === 'oauth-based' && codexCLIOnlyEnabled.value) {
      extra.codex_cli_only = true
    } else {
      delete extra.codex_cli_only
    }
    delete extra.codex_cli_only_allowed_clients
    if (
      accountCategory.value === 'oauth-based' &&
      codexCLIOnlyEnabled.value &&
      codexCLIOnlyAppServerEnabled.value
    ) {
      extra.codex_cli_only_allow_app_server = true
    } else {
      delete extra.codex_cli_only_allow_app_server
    }
    // 收敛是显式 opt-in：off 即默认值，不落键；device/session/full 必须显式写入，
    // 否则管理员的选择会被当成默认而丢失（#5610）。
    if (codexFingerprintMode.value !== 'off') {
      extra.codex_fingerprint_mode = codexFingerprintMode.value
    } else {
      delete extra.codex_fingerprint_mode
    }
    if (openAICompactMode.value !== 'auto') {
      extra.openai_compact_mode = openAICompactMode.value
    } else {
      delete extra.openai_compact_mode
    }

    if (
      accountCategory.value === 'apikey' &&
      openAITextGenerationCapabilityEnabled.value &&
      openAIResponsesMode.value !== 'auto'
    ) {
      extra.openai_responses_mode = openAIResponsesMode.value
    } else {
      delete extra.openai_responses_mode
    }

    return Object.keys(extra).length > 0 ? extra : undefined
  }

  const buildOpenAICodexImportExtra = (): Record<string, unknown> | undefined => {
    const extra = buildOpenAIExtra()
    if (!extra) {
      return undefined
    }
    if (!openAILongContextBillingTouched.value) {
      delete extra.openai_long_context_billing_enabled
    }
    return Object.keys(extra).length > 0 ? extra : undefined
  }

  const buildAnthropicExtra = (base?: Record<string, unknown>): Record<string, unknown> | undefined => {
    if (form.platform !== 'anthropic' || accountCategory.value !== 'apikey') {
      return base
    }

    const extra: Record<string, unknown> = { ...(base || {}) }
    if (anthropicPassthroughEnabled.value) {
      extra.anthropic_passthrough = true
    } else {
      delete extra.anthropic_passthrough
    }
    if (anthropicAPIKeyAuthScheme.value === 'authorization_bearer') {
      extra.anthropic_apikey_auth_scheme = 'authorization_bearer'
    } else {
      delete extra.anthropic_apikey_auth_scheme
    }
    if (webSearchEmulationMode.value === 'default') {
      delete extra.web_search_emulation
    } else {
      extra.web_search_emulation = webSearchEmulationMode.value
    }

    return Object.keys(extra).length > 0 ? extra : undefined
  }

  return {
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
    codexFingerprintModeOptions,
    anthropicPassthroughEnabled,
    anthropicAPIKeyAuthScheme,
    webSearchEmulationMode,
    webSearchGlobalEnabled,
    toggleOpenAILongContextBilling,
    getOpenAICompactModelMappingKey,
    openAICompactModeOptions,
    openAIResponsesModeOptions,
    openAITextEndpointCapabilityLabel,
    openAIEndpointCapabilityOptions,
    openAITextGenerationCapabilityEnabled,
    normalizeOpenAIEndpointCapabilities,
    toggleOpenAIEndpointCapability,
    applyOpenAIEndpointCapabilities,
    buildOpenAICompactModelMapping,
    addOpenAICompactModelMapping,
    removeOpenAICompactModelMapping,
    openAIWSModeOptions,
    openaiResponsesWebSocketV2Mode,
    openAIWSModeConcurrencyHintKey,
    isOpenAIModelRestrictionDisabled,
    buildOpenAIExtra,
    buildOpenAICodexImportExtra,
    buildAnthropicExtra
  }
}
