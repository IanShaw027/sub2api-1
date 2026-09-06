// Extracted verbatim from EditAccountModal.vue's <script setup>: the pure
// OpenAI-related option lists / status computeds / endpoint-capability
// helpers that feed EditAdvancedOptionsSection and EditApiKeyOAuthFieldsSection
// props. Mechanical relocation — the composable receives the host's own refs
// as `deps` and returns the exact same names so the host can destructure
// them back unchanged.
import { computed } from 'vue'
import {
  OPENAI_WS_MODE_CTX_POOL,
  OPENAI_WS_MODE_OFF,
  OPENAI_WS_MODE_PASSTHROUGH,
  OPENAI_WS_MODE_HTTP_BRIDGE,
  resolveOpenAIWSModeConcurrencyHintKey,
  type OpenAIWSMode
} from '@/utils/openaiWsMode'
import { buildPlanTypeOptions } from '@/components/account/credentialsBuilder'
import type { Account, OpenAIEndpointCapability, OpenAIResponsesMode } from '@/types'

export type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
export type CodexImageToolMode = 'inherit' | 'enabled' | 'disabled' | 'block'

export interface EditAccountOpenAIOptionsDeps {
  t: (key: string, params?: Record<string, unknown>) => string
  props: { account: Account | null }
  codexImageToolMode: { value: CodexImageToolMode }
  openaiAPIKeyResponsesWebSocketV2Mode: { value: OpenAIWSMode }
  openaiOAuthResponsesWebSocketV2Mode: { value: OpenAIWSMode }
  openAIEndpointCapabilities: { value: OpenAIEndpointCapability[] }
  openAIResponsesMode: { value: OpenAIResponsesMode }
  editPlanType: { value: string }
  openaiPassthroughEnabled: { value: boolean }
}

export function useEditAccountOpenAIOptions(deps: EditAccountOpenAIOptionsDeps) {
  const {
    t,
    props,
    codexImageToolMode,
    openaiAPIKeyResponsesWebSocketV2Mode,
    openaiOAuthResponsesWebSocketV2Mode,
    openAIEndpointCapabilities,
    openAIResponsesMode,
    editPlanType,
    openaiPassthroughEnabled
  } = deps

  const codexFingerprintModeOptions = computed(() => [
    { value: 'off' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintOff') },
    { value: 'device' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintDevice') },
    { value: 'session' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintSession') },
    { value: 'full' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintFull') },
  ])

  const openAIWSModeOptions = computed(() => [
    { value: OPENAI_WS_MODE_OFF, label: t('admin.accounts.openai.wsModeOff') },
    { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
    { value: OPENAI_WS_MODE_PASSTHROUGH, label: t('admin.accounts.openai.wsModePassthrough') },
    { value: OPENAI_WS_MODE_HTTP_BRIDGE, label: t('admin.accounts.openai.wsModeHttpBridge') }
  ])
  const openaiResponsesWebSocketV2Mode = computed({
    get: () => {
      if (props.account?.type === 'apikey') {
        return openaiAPIKeyResponsesWebSocketV2Mode.value
      }
      return openaiOAuthResponsesWebSocketV2Mode.value
    },
    set: (mode: OpenAIWSMode) => {
      if (props.account?.type === 'apikey') {
        openaiAPIKeyResponsesWebSocketV2Mode.value = mode
        return
      }
      openaiOAuthResponsesWebSocketV2Mode.value = mode
    }
  })
  const openAIWSModeConcurrencyHintKey = computed(() =>
    resolveOpenAIWSModeConcurrencyHintKey(openaiResponsesWebSocketV2Mode.value)
  )
  const codexImageToolOptions = computed<Array<{
    value: CodexImageToolMode
    label: string
    description: string
    selectedCardClass: string
    selectedDotClass: string
  }>>(() => [
    {
      value: 'inherit',
      label: t('admin.accounts.openai.codexImageToolInherit'),
      description: t('admin.accounts.openai.codexImageToolInheritDesc'),
      selectedCardClass: 'border-[color-mix(in_oklch,var(--accent)_45%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent shadow-sm ring-1 ring-[color-mix(in_oklch,var(--accent)_25%,transparent)]',
      selectedDotClass: 'border-accent bg-accent text-white'
    },
    {
      value: 'enabled',
      label: t('admin.accounts.openai.codexImageToolEnabled'),
      description: t('admin.accounts.openai.codexImageToolEnabledDesc'),
      selectedCardClass: 'border-success bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text shadow-sm ring-1 ring-[color-mix(in_oklch,var(--success)_30%,transparent)]',
      selectedDotClass: 'border-success bg-[var(--success)] text-white'
    },
    {
      value: 'disabled',
      label: t('admin.accounts.openai.codexImageToolDisabled'),
      description: t('admin.accounts.openai.codexImageToolDisabledDesc'),
      selectedCardClass: 'border-warning bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text shadow-sm ring-1 ring-[color-mix(in_oklch,var(--warning)_30%,transparent)]',
      selectedDotClass: 'border-warning bg-[var(--warning)] text-white'
    },
    {
      value: 'block',
      label: t('admin.accounts.openai.codexImageToolBlock'),
      description: t('admin.accounts.openai.codexImageToolBlockDesc'),
      selectedCardClass: 'border-danger-text bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text shadow-sm ring-1 ring-[color-mix(in_oklch,var(--danger)_30%,transparent)]',
      selectedDotClass: 'border-danger-text bg-[var(--danger)] text-white'
    }
  ])
  const codexImageToolBadgeLabel = computed(() => {
    switch (codexImageToolMode.value) {
      case 'enabled':
        return t('admin.accounts.openai.codexImageToolBadgeEnabled')
      case 'disabled':
        return t('admin.accounts.openai.codexImageToolBadgeDisabled')
      case 'block':
        return t('admin.accounts.openai.codexImageToolBadgeBlock')
      default:
        return t('admin.accounts.openai.codexImageToolBadgeInherit')
    }
  })
  const codexImageToolBadgeClass = computed(() => {
    switch (codexImageToolMode.value) {
      case 'enabled':
        return 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text'
      case 'disabled':
        return 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text'
      case 'block':
        return 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text'
      default:
        return 'bg-surface-3 text-muted'
    }
  })
  const openAICompactModeOptions = computed(() => [
    { value: 'auto', label: t('admin.accounts.openai.compactModeAuto') },
    { value: 'force_on', label: t('admin.accounts.openai.compactModeForceOn') },
    { value: 'force_off', label: t('admin.accounts.openai.compactModeForceOff') }
  ])
  // OpenAI 订阅档位手动覆盖选项(清空 + Plus/Pro/Free;别名/自定义值友好显示且保留 canonical)
  const planTypeOptions = computed(() =>
    buildPlanTypeOptions(editPlanType.value, t('admin.accounts.openai.planTypeClear'))
  )
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
    const extra = props.account?.extra as Record<string, unknown> | undefined
    if (extra?.openai_responses_supported === true) {
      return t('admin.accounts.openai.capabilityResponsesAuto')
    }
    if (extra?.openai_responses_supported === false) {
      return t('admin.accounts.openai.capabilityChatCompletionsAuto')
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

  const readOpenAIEndpointCapabilities = (credentials?: Record<string, unknown>): OpenAIEndpointCapability[] => {
    const raw = credentials?.openai_capabilities
    if (Array.isArray(raw)) {
      return normalizeOpenAIEndpointCapabilities(
        raw.filter((value): value is OpenAIEndpointCapability =>
          value === 'chat_completions' || value === 'embeddings'
        )
      )
    }
    if (raw !== null && typeof raw === 'object') {
      const capabilityMap = raw as Record<string, unknown>
      return normalizeOpenAIEndpointCapabilities(
        openAIEndpointCapabilityOptions.value
          .map((option) => option.value)
          .filter((value) => capabilityMap[value] === true)
      )
    }
    return ['chat_completions', 'embeddings']
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
  const normalizeOpenAIResponsesMode = (mode: unknown): OpenAIResponsesMode => {
    if (mode === 'force_responses' || mode === 'force_chat_completions') {
      return mode
    }
    return 'auto'
  }
  const isOpenAIModelRestrictionDisabled = computed(() =>
    props.account?.platform === 'openai' && openaiPassthroughEnabled.value
  )
  const openAIResponsesStatusKey = computed(() => {
    if (openAIResponsesMode.value === 'force_responses') {
      return 'admin.accounts.openai.responsesStatusForcedResponses'
    }
    if (openAIResponsesMode.value === 'force_chat_completions') {
      return 'admin.accounts.openai.responsesStatusForcedChatCompletions'
    }
    const extra = props.account?.extra as Record<string, unknown> | undefined
    if (extra?.openai_responses_supported === true) {
      return 'admin.accounts.openai.responsesStatusAutoSupported'
    }
    if (extra?.openai_responses_supported === false) {
      return 'admin.accounts.openai.responsesStatusAutoUnsupported'
    }
    return 'admin.accounts.openai.responsesStatusAutoUnknown'
  })
  const openAICompactStatusKey = computed(() => {
    const extra = props.account?.extra as Record<string, unknown> | undefined
    if (!props.account || props.account.platform !== 'openai') return ''
    const mode = typeof extra?.openai_compact_mode === 'string' ? extra.openai_compact_mode : 'auto'
    if (mode === 'force_on') return 'admin.accounts.openai.compactSupported'
    if (mode === 'force_off') return 'admin.accounts.openai.compactUnsupported'
    if (typeof extra?.openai_compact_supported === 'boolean') {
      return extra.openai_compact_supported
        ? 'admin.accounts.openai.compactSupported'
        : 'admin.accounts.openai.compactUnsupported'
    }
    return 'admin.accounts.openai.compactAuto'
  })

  return {
    codexFingerprintModeOptions,
    openAIWSModeOptions,
    openaiResponsesWebSocketV2Mode,
    openAIWSModeConcurrencyHintKey,
    codexImageToolOptions,
    codexImageToolBadgeLabel,
    codexImageToolBadgeClass,
    openAICompactModeOptions,
    planTypeOptions,
    openAIResponsesModeOptions,
    openAITextEndpointCapabilityLabel,
    openAIEndpointCapabilityOptions,
    openAITextGenerationCapabilityEnabled,
    normalizeOpenAIEndpointCapabilities,
    readOpenAIEndpointCapabilities,
    toggleOpenAIEndpointCapability,
    applyOpenAIEndpointCapabilities,
    normalizeOpenAIResponsesMode,
    isOpenAIModelRestrictionDisabled,
    openAIResponsesStatusKey,
    openAICompactStatusKey
  }
}
