<template>
  <div class="space-y-4">
    <div v-if="!isCnPlatform || apiProtocol !== 'adaptive'">
      <label class="input-label">{{ t('admin.accounts.baseUrl') }}</label>
      <input
        v-model="apiKeyBaseUrl"
        type="text"
        class="input"
        :placeholder="apiKeyBaseUrlPlaceholder"
      />
      <p v-if="baseUrlHint" class="input-hint">{{ baseUrlHint }}</p>
      <GrokBaseUrlPresets
        v-if="platform === 'grok'"
        class="mt-2"
        @select="apiKeyBaseUrl = $event"
      />
      <CnBaseUrlPresets
        v-if="isCnPlatform"
        class="mt-2"
        :platform="cnPresetPlatform"
        :mode="accountMode"
        :protocol="apiProtocol"
        :current-url="apiKeyBaseUrl"
        @select="emit('cn-preset-select', $event)"
      />
    </div>
    <div v-else>
      <label class="input-label">{{ t('admin.accounts.cnProviders.apiProtocol.endpoints') }}</label>
      <div class="mt-2 space-y-3">
        <div v-for="item in cnAdaptiveProtocolOptions" :key="item.value">
          <label class="mb-1 block text-xs font-medium text-muted">
            {{ t(`admin.accounts.cnProviders.apiProtocol.${item.labelKey}`) }}
          </label>
          <input
            v-model="adaptiveBaseUrls[item.value]"
            type="text"
            class="input"
            :data-testid="`cn-adaptive-base-url-${item.value}`"
          />
        </div>
      </div>
      <p v-if="!cnSupportsNativeResponses(platform)" class="input-hint">
        {{ t('admin.accounts.cnProviders.apiProtocol.responsesFallbackDesc') }}
      </p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.apiKeyRequired') }}</label>
      <input
        v-model="apiKeyValue"
        type="password"
        required
        class="input font-mono"
        :placeholder="apiKeyValuePlaceholder"
      />
      <p v-if="apiKeyHint" class="input-hint">{{ apiKeyHint }}</p>
    </div>

    <!-- 上游倍率自动探测：全部 API-key 平台可用（所在区块已限定 apikey 类型） -->
    <div
      class="flex items-center justify-between gap-4 border-t border-line pt-4"
    >
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.upstreamBilling.autoProbe') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.upstreamBilling.autoProbeHint') }}
        </p>
      </div>
      <Toggle
        v-model="upstreamBillingAutoProbeEnabled"
        data-testid="upstream-billing-auto-probe"
        :aria-label="t('admin.accounts.upstreamBilling.autoProbe')"
      />
    </div>

    <!-- Gemini API Key tier selection -->
    <div v-if="platform === 'gemini'">
      <label class="input-label">{{ t('admin.accounts.gemini.tier.label') }}</label>
      <select v-model="geminiTierAiStudio" class="input">
        <option value="aistudio_free">{{ t('admin.accounts.gemini.tier.aiStudio.free') }}</option>
        <option value="aistudio_paid">{{ t('admin.accounts.gemini.tier.aiStudio.paid') }}</option>
      </select>
      <p class="input-hint">{{ t('admin.accounts.gemini.tier.aiStudioHint') }}</p>
    </div>

    <!-- Model Restriction Section (Antigravity 已在上层条件排除) -->
    <div class="border-t border-line pt-4">
      <ModelRestrictionEditor
        v-model:mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        :platform="platform"
        :sync-credentials="syncPreviewCredentials"
        :disabled-by-passthrough="isOpenAiModelRestrictionDisabled"
        :show-icons="true"
        :presets="presetMappings"
        @upstream-synced="emit('upstream-synced')"
      />
    </div>

    <!-- Pool Mode Section -->
    <PoolModeSection
      v-model:pool-mode-enabled="poolModeEnabled"
      v-model:pool-mode-retry-count="poolModeRetryCount"
      v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
    />

    <!-- Custom Error Codes Section -->
    <CustomErrorCodesSection
      v-model:custom-error-codes-enabled="customErrorCodesEnabled"
      v-model:selected-error-codes="selectedErrorCodes"
      v-model:custom-error-code-input="customErrorCodeInput"
    />

    <!-- Header Override Section (eligible API-key platforms) -->
    <HeaderOverrideSection
      v-if="isHeaderOverrideCapable(platform, 'apikey')"
      v-model:headerOverrideEnabled="headerOverrideEnabled"
      v-model:headerOverrideRows="headerOverrideRows"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import GrokBaseUrlPresets from '@/components/account/GrokBaseUrlPresets.vue'
import CnBaseUrlPresets from '@/components/account/CnBaseUrlPresets.vue'
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
import PoolModeSection from '@/components/account/shared/PoolModeSection.vue'
import CustomErrorCodesSection from '@/components/account/shared/CustomErrorCodesSection.vue'
import HeaderOverrideSection from '@/components/account/shared/HeaderOverrideSection.vue'
import {
  cnSupportsNativeResponses,
  isHeaderOverrideCapable,
  type CnAccountMode,
  type CnApiProtocol,
  type CnNativeApiProtocol,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import type { getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'

interface ModelMapping {
  from: string
  to: string
}

interface SyncCredentials {
  platform: string
  type: string
  base_url?: string
  api_key: string
}

interface Props {
  platform: string
  isCnPlatform: boolean
  apiProtocol: CnApiProtocol
  accountMode: CnAccountMode
  cnPresetPlatform: 'kimi' | 'zhipu' | 'deepseek'
  cnAdaptiveProtocolOptions: Array<{ value: CnNativeApiProtocol; labelKey: string }>
  apiKeyBaseUrlPlaceholder: string
  baseUrlHint?: string
  apiKeyHint?: string
  apiKeyValuePlaceholder: string
  syncPreviewCredentials?: SyncCredentials
  isOpenAiModelRestrictionDisabled: boolean
  presetMappings: ReturnType<typeof getPresetMappingsByPlatform>
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'cn-preset-select', preset: { mode: CnAccountMode; protocol: CnApiProtocol; url: string }): void
  (e: 'upstream-synced'): void
}>()

const apiKeyBaseUrl = defineModel<string>('apiKeyBaseUrl', { required: true })
const apiKeyValue = defineModel<string>('apiKeyValue', { required: true })
const adaptiveBaseUrls = defineModel<Record<CnNativeApiProtocol, string>>('adaptiveBaseUrls', { required: true })
const upstreamBillingAutoProbeEnabled = defineModel<boolean>('upstreamBillingAutoProbeEnabled', { required: true })
const geminiTierAiStudio = defineModel<'aistudio_free' | 'aistudio_paid'>('geminiTierAiStudio', { required: true })
const modelRestrictionMode = defineModel<'whitelist' | 'mapping'>('modelRestrictionMode', { required: true })
const allowedModels = defineModel<string[]>('allowedModels', { required: true })
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })
const poolModeEnabled = defineModel<boolean>('poolModeEnabled', { required: true })
const poolModeRetryCount = defineModel<number>('poolModeRetryCount', { required: true })
const poolModeRetryStatusCodesInput = defineModel<string>('poolModeRetryStatusCodesInput', { required: true })
const customErrorCodesEnabled = defineModel<boolean>('customErrorCodesEnabled', { required: true })
const selectedErrorCodes = defineModel<number[]>('selectedErrorCodes', { required: true })
const customErrorCodeInput = defineModel<number | null>('customErrorCodeInput', { required: true })
const headerOverrideEnabled = defineModel<boolean>('headerOverrideEnabled', { required: true })
const headerOverrideRows = defineModel<HeaderOverrideRow[]>('headerOverrideRows', { required: true })

const { t } = useI18n()
</script>
