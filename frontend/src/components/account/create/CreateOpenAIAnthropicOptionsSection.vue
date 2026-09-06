<template>
  <div>
    <!-- OpenAI 自动透传开关（OAuth/API Key） -->
    <div
      v-if="platform === 'openai'"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.oauthPassthrough') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.oauthPassthroughDesc') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="openaiPassthroughEnabled" />
      </div>
    </div>

    <!-- OpenAI Codex namespace 工具摊平（兼容开关，仅 OAuth） -->
    <div
      v-if="platform === 'openai' && accountType === 'oauth'"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.flattenNamespaces') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.flattenNamespacesDesc') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="openaiFlattenNamespacesEnabled" data-testid="create-openai-flatten-namespaces-toggle" />
      </div>
    </div>

    <!-- OpenAI WS Mode 三态（off/ctx_pool/passthrough） -->
    <div
      v-if="platform === 'openai' && (accountCategory === 'oauth-based' || accountCategory === 'apikey')"
      data-testid="create-openai-ws-mode"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.wsMode') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.wsModeDesc') }}
          </p>
          <p class="mt-1 text-xs text-muted">
            {{ t(openAiWsModeConcurrencyHintKey) }}
          </p>
        </div>
        <div class="w-52">
          <Select v-model="openaiResponsesWebSocketV2Mode" :options="openAiWsModeOptions" />
        </div>
      </div>
    </div>

    <!-- Anthropic API Key 自动透传开关 -->
    <div
      v-if="platform === 'anthropic' && accountCategory === 'apikey'"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyPassthrough') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.anthropic.apiKeyPassthroughDesc') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="anthropicPassthroughEnabled" />
      </div>
    </div>

    <div
      v-if="platform === 'anthropic' && accountCategory === 'apikey'"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between gap-4">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyAuthScheme') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.anthropic.apiKeyAuthSchemeDesc') }}
          </p>
        </div>
        <select v-model="anthropicApiKeyAuthScheme" class="input w-52 text-sm">
          <option value="x_api_key">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeXApiKey') }}</option>
          <option value="authorization_bearer">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeBearer') }}</option>
        </select>
      </div>
    </div>

    <!-- Anthropic API Key: Web Search Emulation (hidden when global disabled) -->
    <div
      v-if="platform === 'anthropic' && accountCategory === 'apikey' && webSearchGlobalEnabled"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.anthropic.webSearchEmulation') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.anthropic.webSearchEmulationDesc') }}
          </p>
        </div>
        <select v-model="webSearchEmulationMode" class="input w-24 text-sm">
          <option value="default">{{ t('admin.accounts.anthropic.webSearchDefault') }}</option>
          <option value="enabled">{{ t('admin.accounts.anthropic.webSearchEnabled') }}</option>
          <option value="disabled">{{ t('admin.accounts.anthropic.webSearchDisabled') }}</option>
        </select>
      </div>
    </div>

    <!-- OpenAI API 长上下文计费开关 -->
    <div
      v-if="platform === 'openai' && !hideAccountLongContextBilling && (accountCategory === 'oauth-based' || accountCategory === 'apikey')"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between gap-4">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.longContextBilling') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.longContextBillingDesc') }}
          </p>
        </div>
        <InlineToggleSwitch
          :model-value="openAiLongContextBillingEnabled"
          data-testid="openai-long-context-billing-toggle"
          switch-role
          :auto-toggle="false"
          @click="toggleOpenAILongContextBilling"
        />
      </div>
    </div>

    <div
      v-if="platform === 'openai' && accountCategory === 'oauth-based'"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnly') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="codexCliOnlyEnabled" />
      </div>
      <div
        v-if="codexCliOnlyEnabled"
        class="mt-4 flex items-center justify-between border-l-2 border-line pl-4"
      >
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnlyAppServer') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.codexCLIOnlyAppServerDesc') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="codexCliOnlyAppServerEnabled" />
      </div>
    </div>

    <!-- Codex 指纹收敛模式（仅 OpenAI OAuth） -->
    <div
      v-if="platform === 'openai' && accountCategory === 'oauth-based'"
      class="border-t border-line pt-4"
    >
      <div class="flex items-center justify-between gap-4">
        <div class="min-w-0">
          <label class="input-label mb-0">{{ t('admin.accounts.openai.codexFingerprintMode') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.codexFingerprintModeDesc') }}
          </p>
        </div>
        <div class="w-52 flex-shrink-0">
          <Select v-model="codexFingerprintMode" data-testid="create-codex-fingerprint-mode-select" :options="codexFingerprintModeOptions" />
        </div>
      </div>
    </div>

    <!-- OpenAI Compact 能力配置 -->
    <div
      v-if="platform === 'openai' && (accountCategory === 'oauth-based' || accountCategory === 'apikey')"
      class="border-t border-line pt-4 space-y-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.compactMode') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.compactModeDesc') }}
          </p>
        </div>
        <div class="w-44">
          <Select v-model="openAiCompactMode" :options="openAiCompactModeOptions" />
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.openai.compactModelMapping') }}</label>
        <p class="input-hint">{{ t('admin.accounts.openai.compactModelMappingDesc') }}</p>
        <div v-if="openAiCompactModelMappings.length > 0" class="mb-3 space-y-2">
          <div
            v-for="(mapping, index) in openAiCompactModelMappings"
            :key="getOpenAiCompactModelMappingKey(mapping)"
            class="flex items-center gap-2"
          >
            <input v-model="mapping.from" type="text" class="input flex-1" :placeholder="t('admin.accounts.fromModel')" />
            <span class="text-muted">→</span>
            <input v-model="mapping.to" type="text" class="input flex-1" :placeholder="t('admin.accounts.toModel')" />
            <button type="button" @click="removeOpenAiCompactModelMapping(index)" class="text-danger-text hover:text-danger-text">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
        <button type="button" @click="addOpenAiCompactModelMapping" class="btn btn-secondary text-sm">
          + {{ t('admin.accounts.addMapping') }}
        </button>
      </div>
    </div>

    <!-- OpenAI APIKey Responses API support mode -->
    <div
      v-if="platform === 'openai' && accountCategory === 'apikey'"
      class="space-y-4 border-t border-line pt-4"
    >
      <div class="flex items-center justify-between gap-4">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.responsesMode') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.responsesModeDesc') }}
          </p>
        </div>
        <div class="w-56">
          <Select
            v-model="openAiResponsesMode"
            :options="openAiResponsesModeOptions"
            :disabled="!openAiTextGenerationCapabilityEnabled"
            data-testid="openai-responses-mode-select"
          />
        </div>
      </div>
      <p
        v-if="!openAiTextGenerationCapabilityEnabled"
        class="rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-xs text-warning-text"
        data-testid="openai-responses-mode-not-applicable"
      >
        {{ t('admin.accounts.openai.responsesModeTextDisabledHint') }}
      </p>
      <div>
        <label class="input-label mb-2 block">{{ t('admin.accounts.openai.endpointCapabilities') }}</label>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <label
            v-for="option in openAiEndpointCapabilityOptions"
            :key="option.value"
            class="flex cursor-pointer items-center gap-2 rounded-lg border border-line px-3 py-2 text-sm"
          >
            <input
              type="checkbox"
              class="rounded border-line text-accent focus:ring-accent"
              :data-testid="`openai-endpoint-capability-${option.value}`"
              :checked="openAiEndpointCapabilities.includes(option.value)"
              @change="toggleOpenAiEndpointCapability(option.value, $event)"
            />
            <span class="text-foreground">{{ option.label }}</span>
          </label>
        </div>
        <p class="input-hint">{{ t('admin.accounts.openai.endpointCapabilitiesDesc') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'
import type { OpenAICompactMode, OpenAIResponsesMode, OpenAIEndpointCapability } from '@/types'
import type { OpenAIWSMode } from '@/utils/openaiWsMode'

type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'

interface ModelMapping {
  from: string
  to: string
}

interface Props {
  platform: string
  accountType: string
  accountCategory: string
  webSearchGlobalEnabled: boolean
  hideAccountLongContextBilling: boolean
  openAiLongContextBillingEnabled: boolean
  toggleOpenAILongContextBilling: () => void
  openAiWsModeConcurrencyHintKey: string
  openAiWsModeOptions: Array<Record<string, unknown>>
  codexFingerprintModeOptions: Array<Record<string, unknown>>
  openAiCompactModeOptions: Array<Record<string, unknown>>
  openAiCompactModelMappings: ModelMapping[]
  getOpenAiCompactModelMappingKey: (mapping: ModelMapping) => string
  removeOpenAiCompactModelMapping: (index: number) => void
  addOpenAiCompactModelMapping: () => void
  openAiResponsesModeOptions: Array<Record<string, unknown>>
  openAiTextGenerationCapabilityEnabled: boolean
  openAiEndpointCapabilityOptions: Array<{ value: OpenAIEndpointCapability; label: string }>
  openAiEndpointCapabilities: OpenAIEndpointCapability[]
  toggleOpenAiEndpointCapability: (capability: OpenAIEndpointCapability, event?: Event) => void
}

defineProps<Props>()

const openaiPassthroughEnabled = defineModel<boolean>('openaiPassthroughEnabled', { required: true })
const openaiFlattenNamespacesEnabled = defineModel<boolean>('openaiFlattenNamespacesEnabled', { required: true })
const openaiResponsesWebSocketV2Mode = defineModel<OpenAIWSMode>('openaiResponsesWebSocketV2Mode', { required: true })
const anthropicPassthroughEnabled = defineModel<boolean>('anthropicPassthroughEnabled', { required: true })
const anthropicApiKeyAuthScheme = defineModel<AnthropicAPIKeyAuthScheme>('anthropicApiKeyAuthScheme', { required: true })
const webSearchEmulationMode = defineModel<string>('webSearchEmulationMode', { required: true })
const codexCliOnlyEnabled = defineModel<boolean>('codexCliOnlyEnabled', { required: true })
const codexCliOnlyAppServerEnabled = defineModel<boolean>('codexCliOnlyAppServerEnabled', { required: true })
const codexFingerprintMode = defineModel<CodexFingerprintMode>('codexFingerprintMode', { required: true })
const openAiCompactMode = defineModel<OpenAICompactMode>('openAiCompactMode', { required: true })
const openAiResponsesMode = defineModel<OpenAIResponsesMode>('openAiResponsesMode', { required: true })

const { t } = useI18n()
</script>
