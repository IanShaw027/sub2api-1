<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkEdit.title')"
    width="wide"
    @close="handleClose"
  >
    <form id="bulk-edit-account-form" class="space-y-5" @submit.prevent="() => handleSubmit()">
      <!-- Info -->
      <div class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-4">
        <p class="text-sm text-accent">
          <svg class="mr-1.5 inline h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          {{ t('admin.accounts.bulkEdit.selectionInfo', { count: targetMode === 'filtered' ? targetPreviewCount : accountIds.length }) }}
        </p>
      </div>

      <!-- Mixed platform warning -->
      <div v-if="isMixedPlatform" class="rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-4">
        <p class="text-sm text-warning-text">
          <svg class="mr-1.5 inline h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          {{ t('admin.accounts.bulkEdit.mixedPlatformWarning', { platforms: targetSelectedPlatforms.join(', ') }) }}
        </p>
      </div>

      <!-- OpenAI passthrough -->
      <div
        v-if="allOpenAIPassthroughCapable"
        class="border-t border-line pt-4"
      >
        <div class="mb-3 flex items-center justify-between">
          <div class="flex-1 pr-4">
            <label
              id="bulk-edit-openai-passthrough-label"
              class="input-label mb-0"
              for="bulk-edit-openai-passthrough-enabled"
            >
              {{ t('admin.accounts.openai.oauthPassthrough') }}
            </label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.oauthPassthroughDesc') }}
            </p>
          </div>
          <input
            v-model="enableOpenAIPassthrough"
            id="bulk-edit-openai-passthrough-enabled"
            type="checkbox"
            aria-controls="bulk-edit-openai-passthrough-body"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div
          id="bulk-edit-openai-passthrough-body"
          :class="!enableOpenAIPassthrough && 'pointer-events-none opacity-50'"
          role="group"
          aria-labelledby="bulk-edit-openai-passthrough-label"
        >
          <button
            id="bulk-edit-openai-passthrough-toggle"
            type="button"
            :class="[
 'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
 openaiPassthroughEnabled ? 'bg-accent' : 'bg-surface-3'
 ]"
            @click="openaiPassthroughEnabled = !openaiPassthroughEnabled"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
                openaiPassthroughEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- OpenAI Codex namespace 工具摊平（兼容开关，仅 OAuth） -->
      <div
        v-if="allOpenAIOAuthOnly"
        class="border-t border-line pt-4"
      >
        <div class="mb-3 flex items-center justify-between">
          <div class="flex-1 pr-4">
            <label
              id="bulk-edit-openai-flatten-namespaces-label"
              class="input-label mb-0"
              for="bulk-edit-openai-flatten-namespaces-enabled"
            >
              {{ t('admin.accounts.openai.flattenNamespaces') }}
            </label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.flattenNamespacesDesc') }}
            </p>
          </div>
          <input
            v-model="enableOpenAIFlattenNamespaces"
            id="bulk-edit-openai-flatten-namespaces-enabled"
            type="checkbox"
            aria-controls="bulk-edit-openai-flatten-namespaces-body"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div
          id="bulk-edit-openai-flatten-namespaces-body"
          :class="!enableOpenAIFlattenNamespaces && 'pointer-events-none opacity-50'"
          role="group"
          aria-labelledby="bulk-edit-openai-flatten-namespaces-label"
        >
          <button
            id="bulk-edit-openai-flatten-namespaces-toggle"
            type="button"
            :class="[
 'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
 openaiFlattenNamespacesEnabled ? 'bg-accent' : 'bg-surface-3'
 ]"
            @click="openaiFlattenNamespacesEnabled = !openaiFlattenNamespacesEnabled"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
                openaiFlattenNamespacesEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- OpenAI API long-context billing -->
      <div
        v-if="allOpenAIPassthroughCapable"
        class="border-t border-line pt-4"
      >
        <div class="mb-3 flex items-center justify-between gap-4">
          <div class="flex-1">
            <label
              id="bulk-edit-openai-long-context-billing-label"
              class="input-label mb-0"
              for="bulk-edit-openai-long-context-billing-enabled"
            >
              {{ t('admin.accounts.openai.longContextBilling') }}
            </label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.longContextBillingDesc') }}
            </p>
          </div>
          <input
            v-model="enableOpenAILongContextBilling"
            id="bulk-edit-openai-long-context-billing-enabled"
            type="checkbox"
            aria-controls="bulk-edit-openai-long-context-billing-body"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div
          id="bulk-edit-openai-long-context-billing-body"
          :class="!enableOpenAILongContextBilling && 'pointer-events-none opacity-50'"
          role="group"
          aria-labelledby="bulk-edit-openai-long-context-billing-label"
        >
          <button
            type="button"
            data-testid="bulk-edit-openai-long-context-billing-toggle"
            role="switch"
            :disabled="!enableOpenAILongContextBilling"
            :aria-checked="openAILongContextBillingEnabled"
            :class="[
 'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
 openAILongContextBillingEnabled ? 'bg-accent' : 'bg-surface-3'
 ]"
            @click="openAILongContextBillingEnabled = !openAILongContextBillingEnabled"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
                openAILongContextBillingEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
        <p
          class="mt-3 rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-xs text-warning-text"
          data-testid="bulk-edit-openai-long-context-shadow-hint"
        >
          {{ t('admin.accounts.bulkEdit.longContextShadowHint') }}
        </p>
      </div>

      <!-- Base URL (API Key only) -->
      <div class="border-t border-line pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-base-url-label"
            class="input-label mb-0"
            for="bulk-edit-base-url-enabled"
          >
            {{ t('admin.accounts.baseUrl') }}
          </label>
          <input
            v-model="enableBaseUrl"
            id="bulk-edit-base-url-enabled"
            type="checkbox"
            aria-controls="bulk-edit-base-url"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <input
          v-model="baseUrl"
          id="bulk-edit-base-url"
          type="text"
          :disabled="!enableBaseUrl"
          class="input"
          :class="!enableBaseUrl && 'cursor-not-allowed opacity-50'"
          :placeholder="t('admin.accounts.bulkEdit.baseUrlPlaceholder')"
          aria-labelledby="bulk-edit-base-url-label"
        />
        <GrokBaseUrlPresets
          v-if="allTargetsGrok"
          class="mt-2"
          @select="baseUrl = $event; enableBaseUrl = true"
        />
        <p class="input-hint">
          {{ t('admin.accounts.bulkEdit.baseUrlNotice') }}
        </p>
      </div>

      <!-- Model restriction -->
      <ModelRestrictionSection
        v-model:enabled="enableModelRestriction"
        v-model:mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        :platforms="targetSelectedPlatforms"
        :disabled-by-passthrough="isOpenAIModelRestrictionDisabled"
        :presets="filteredPresets"
      />

      <!-- Custom error codes -->
      <CustomErrorCodesSection
        v-model:enabled="enableCustomErrorCodes"
        v-model:selected-codes="selectedErrorCodes"
      />

      <!-- Intercept warmup requests (Anthropic only) -->
      <div class="border-t border-line pt-4">
        <div class="flex items-center justify-between">
          <div class="flex-1 pr-4">
            <label
              id="bulk-edit-intercept-warmup-label"
              class="input-label mb-0"
              for="bulk-edit-intercept-warmup-enabled"
            >
              {{ t('admin.accounts.interceptWarmupRequests') }}
            </label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.interceptWarmupRequestsDesc') }}
            </p>
          </div>
          <input
            v-model="enableInterceptWarmup"
            id="bulk-edit-intercept-warmup-enabled"
            type="checkbox"
            aria-controls="bulk-edit-intercept-warmup-body"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div v-if="enableInterceptWarmup" id="bulk-edit-intercept-warmup-body" class="mt-3">
          <button
            type="button"
            :class="[
 'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
 interceptWarmupRequests ? 'bg-accent' : 'bg-surface-3'
 ]"
            @click="interceptWarmupRequests = !interceptWarmupRequests"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
                interceptWarmupRequests ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- Header Override (eligible API-key platforms + grok OAuth) -->
      <HeaderOverrideSection
        :show="allHeaderOverrideCapable"
        v-model:enable-header-override="enableHeaderOverride"
        v-model:header-override-enabled="headerOverrideEnabled"
        v-model:header-override-rows="headerOverrideRows"
      />

      <!-- Proxy -->
      <div class="border-t border-line pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-proxy-label"
            class="input-label mb-0"
            for="bulk-edit-proxy-enabled"
          >
            {{ t('admin.accounts.proxy') }}
          </label>
          <input
            v-model="enableProxy"
            id="bulk-edit-proxy-enabled"
            type="checkbox"
            aria-controls="bulk-edit-proxy-body"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div id="bulk-edit-proxy-body" :class="!enableProxy && 'pointer-events-none opacity-50'">
          <ProxySelector
            v-model="proxyId"
            :proxies="proxies"
            allow-rotation
            aria-labelledby="bulk-edit-proxy-label"
          />
          <ProxyRotationSelector
            v-if="proxyId === PROXY_ROTATION_VALUE"
            v-model="proxyIds"
            :proxies="proxies"
            class="mt-3"
          />
        </div>
      </div>

      <!-- Concurrency & Priority -->
      <ConcurrencyPrioritySection
        v-model:enable-concurrency="enableConcurrency"
        v-model:concurrency="concurrency"
        v-model:enable-load-factor="enableLoadFactor"
        v-model:load-factor="loadFactor"
        v-model:enable-priority="enablePriority"
        v-model:priority="priority"
        v-model:enable-rate-multiplier="enableRateMultiplier"
        v-model:rate-multiplier="rateMultiplier"
      />

      <!-- Status -->
      <div class="border-t border-line pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-status-label"
            class="input-label mb-0"
            for="bulk-edit-status-enabled"
          >
            {{ t('common.status') }}
          </label>
          <input
            v-model="enableStatus"
            id="bulk-edit-status-enabled"
            type="checkbox"
            aria-controls="bulk-edit-status"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div id="bulk-edit-status" :class="!enableStatus && 'pointer-events-none opacity-50'">
          <Select
            v-model="status"
            :options="statusOptions"
            aria-labelledby="bulk-edit-status-label"
          />
        </div>
      </div>

      <!-- OpenAI OAuth toggles (WS mode, Codex CLI only, Codex app-server, fingerprint mode) -->
      <OpenAIOAuthTogglesSection
        :show="allOpenAIOAuth"
        :openAIWSModeOptions="openAIWSModeOptions"
        :openAIWSModeConcurrencyHintKey="openAIWSModeConcurrencyHintKey"
        :codexFingerprintModeOptions="codexFingerprintModeOptions"
        v-model:enableOpenAIWSMode="enableOpenAIWSMode"
        v-model:openaiOAuthResponsesWebSocketV2Mode="openaiOAuthResponsesWebSocketV2Mode"
        v-model:enableCodexCLIOnly="enableCodexCLIOnly"
        v-model:codexCLIOnlyEnabled="codexCLIOnlyEnabled"
        v-model:enableCodexCLIOnlyAppServer="enableCodexCLIOnlyAppServer"
        v-model:codexCLIOnlyAppServerEnabled="codexCLIOnlyAppServerEnabled"
        v-model:enableCodexFingerprintMode="enableCodexFingerprintMode"
        v-model:codexFingerprintMode="codexFingerprintMode"
      />

      <!-- Upstream billing auto probe (any API-key platform) -->
      <UpstreamBillingAutoProbeSection
        :show="allBillingProbeCapable"
        :upstreamBillingAutoProbeOptions="upstreamBillingAutoProbeOptions"
        v-model:enableUpstreamBillingAutoProbe="enableUpstreamBillingAutoProbe"
        v-model:upstreamBillingAutoProbeMode="upstreamBillingAutoProbeMode"
      />

      <!-- OpenAI API Key toggles (endpoint capabilities, responses route, WS mode) -->
      <OpenAIApiKeyTogglesSection
        :show="allOpenAIAPIKey"
        :openAIEndpointCapabilityOptions="openAIEndpointCapabilityOptions"
        :openAIResponsesModeOptions="openAIResponsesModeOptions"
        :openAIResponsesModeApplicable="openAIResponsesModeApplicable"
        :openAITextGenerationCapabilityEnabled="openAITextGenerationCapabilityEnabled"
        :openAIWSModeOptions="openAIWSModeOptions"
        :openAIAPIKeyWSModeConcurrencyHintKey="openAIAPIKeyWSModeConcurrencyHintKey"
        v-model:enableOpenAIEndpointCapabilities="enableOpenAIEndpointCapabilities"
        v-model:openAIEndpointCapabilities="openAIEndpointCapabilities"
        v-model:enableOpenAIResponsesMode="enableOpenAIResponsesMode"
        v-model:openAIResponsesMode="openAIResponsesMode"
        v-model:enableOpenAIAPIKeyWSMode="enableOpenAIAPIKeyWSMode"
        v-model:openaiAPIKeyResponsesWebSocketV2Mode="openaiAPIKeyResponsesWebSocketV2Mode"
        @toggle-capability="toggleOpenAIEndpointCapability"
      />

      <!-- OpenAI Compact mode -->
      <OpenAICompactModeSection
        :show="allOpenAIPassthroughCapable"
        :openAICompactModeOptions="openAICompactModeOptions"
        v-model:enableOpenAICompactMode="enableOpenAICompactMode"
        v-model:openAICompactMode="openAICompactMode"
      />

      <!-- OpenAI Compact model mapping -->
      <OpenAICompactModelMappingSection
        :show="allOpenAIPassthroughCapable"
        v-model:enabled="enableOpenAICompactModelMapping"
        v-model:mappings="openAICompactModelMappings"
      />

      <!-- RPM Limit (仅全部为 Anthropic OAuth/SetupToken 时显示) -->
      <RpmLimitSection
        :show="allAnthropicOAuthOrSetupToken"
        v-model:enable-rpm-limit="enableRpmLimit"
        v-model:rpm-limit-enabled="rpmLimitEnabled"
        v-model:bulk-base-rpm="bulkBaseRpm"
        v-model:bulk-rpm-strategy="bulkRpmStrategy"
        v-model:bulk-rpm-sticky-buffer="bulkRpmStickyBuffer"
        v-model:user-msg-queue-mode="userMsgQueueMode"
        :umq-mode-options="umqModeOptions"
      />

      <!-- Groups -->
      <div class="border-t border-line pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-groups-label"
            class="input-label mb-0"
            for="bulk-edit-groups-enabled"
          >
            {{ t('nav.groups') }}
          </label>
          <input
            v-model="enableGroups"
            id="bulk-edit-groups-enabled"
            type="checkbox"
            aria-controls="bulk-edit-groups"
            class="rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div id="bulk-edit-groups" :class="!enableGroups && 'pointer-events-none opacity-50'">
          <GroupSelector
            v-model="groupIds"
            :groups="groups"
            aria-labelledby="bulk-edit-groups-label"
          />
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="bulk-edit-account-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          {{
            submitting ? t('admin.accounts.bulkEdit.updating') : t('admin.accounts.bulkEdit.submit')
          }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="showMixedChannelWarning"
    :title="t('admin.accounts.mixedChannelWarningTitle')"
    :message="mixedChannelWarningMessage"
    :confirm-text="t('common.confirm')"
    :cancel-text="t('common.cancel')"
    :danger="true"
    @confirm="handleMixedChannelConfirm"
    @cancel="handleMixedChannelCancel"
  />
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  Proxy as ProxyConfig,
  AdminGroup,
  AccountPlatform,
  AccountType,
  OpenAICompactMode,
  OpenAIEndpointCapability,
  OpenAIResponsesMode
} from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import ProxySelector, { PROXY_ROTATION_VALUE } from '@/components/common/ProxySelector.vue'
import ProxyRotationSelector from '@/components/common/ProxyRotationSelector.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import ModelRestrictionSection from '@/components/account/bulk/ModelRestrictionSection.vue'
import OpenAICompactModelMappingSection from '@/components/account/bulk/OpenAICompactModelMappingSection.vue'
import ConcurrencyPrioritySection from '@/components/account/bulk/ConcurrencyPrioritySection.vue'
import RpmLimitSection from '@/components/account/bulk/RpmLimitSection.vue'
import CustomErrorCodesSection from '@/components/account/bulk/CustomErrorCodesSection.vue'
import HeaderOverrideSection from '@/components/account/bulk/HeaderOverrideSection.vue'
import OpenAIOAuthTogglesSection from '@/components/account/bulk/OpenAIOAuthTogglesSection.vue'
import UpstreamBillingAutoProbeSection from '@/components/account/bulk/UpstreamBillingAutoProbeSection.vue'
import OpenAIApiKeyTogglesSection from '@/components/account/bulk/OpenAIApiKeyTogglesSection.vue'
import OpenAICompactModeSection from '@/components/account/bulk/OpenAICompactModeSection.vue'
import {
  buildModelMappingObject as buildModelMappingPayload,
  getPresetMappingsByPlatform
} from '@/composables/useModelWhitelist'
import {
  buildHeaderOverridesObject,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows,
  HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY,
  HEADER_OVERRIDES_CREDENTIAL_KEY,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import GrokBaseUrlPresets from '@/components/account/GrokBaseUrlPresets.vue'
import {
  OPENAI_WS_MODE_CTX_POOL,
  OPENAI_WS_MODE_OFF,
  OPENAI_WS_MODE_PASSTHROUGH,
  OPENAI_WS_MODE_HTTP_BRIDGE,
  isOpenAIWSModeEnabled,
  resolveOpenAIWSModeConcurrencyHintKey
} from '@/utils/openaiWsMode'
import type { OpenAIWSMode } from '@/utils/openaiWsMode'
interface Props {
  show: boolean
  accountIds: number[]
  selectedPlatforms: AccountPlatform[]
  selectedTypes: AccountType[]
  target?: {
    mode: 'selected' | 'filtered'
    filters?: Record<string, unknown>
    previewCount?: number
    selectedPlatforms?: AccountPlatform[]
    selectedTypes?: AccountType[]
  }
  proxies: ProxyConfig[]
  groups: AdminGroup[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

// Platform awareness
const targetMode = computed(() => props.target?.mode ?? 'selected')
const targetPreviewCount = computed(() => props.target?.previewCount ?? props.accountIds.length)
const targetSelectedPlatforms = computed(() => props.target?.selectedPlatforms ?? props.selectedPlatforms)
const targetSelectedTypes = computed(() => props.target?.selectedTypes ?? props.selectedTypes)
// Grok 快捷端点仅在所选账号全部为 grok 平台时展示（其他平台不显示）
const allTargetsGrok = computed(
  () =>
    targetSelectedPlatforms.value.length > 0 &&
    targetSelectedPlatforms.value.every((p) => p === 'grok')
)
const isMixedPlatform = computed(() => targetSelectedPlatforms.value.length > 1)

const allOpenAIPassthroughCapable = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'openai' &&
    targetSelectedTypes.value.length > 0 &&
    targetSelectedTypes.value.every(t => t === 'oauth' || t === 'setup-token' || t === 'apikey')
  )
})

const allOpenAIOAuth = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'openai' &&
    targetSelectedTypes.value.length > 0 &&
    targetSelectedTypes.value.every(t => t === 'oauth' || t === 'setup-token')
  )
})

// 严格 OAuth（不含 setup-token）：namespace 摊平兼容开关只对 OAuth 账号生效
const allOpenAIOAuthOnly = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'openai' &&
    targetSelectedTypes.value.length > 0 &&
    targetSelectedTypes.value.every(t => t === 'oauth')
  )
})

const allOpenAIAPIKey = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'openai' &&
    targetSelectedTypes.value.length > 0 &&
    targetSelectedTypes.value.every(t => t === 'apikey')
  )
})

// 上游倍率自动探测已放宽到全部 API-key 平台：只要求所选类型全为 apikey，
// 平台不限（sub2api 上游即可应答 /v1/sub2api/billing）。
const allBillingProbeCapable = computed(() => {
  return (
    targetSelectedTypes.value.length > 0 &&
    targetSelectedTypes.value.every(t => t === 'apikey')
  )
})

// 是否全部为支持请求头覆写的平台/账号类型
// 所选平台 × 所选类型的全组合均需具备覆写资格（实际选中账号是该组合的子集，
// 按交叉积判定偏保守但绝不放行不合资格的账号）
const allHeaderOverrideCapable = computed(() => {
  return (
    targetSelectedPlatforms.value.length > 0 &&
    targetSelectedTypes.value.length > 0 &&
    targetSelectedPlatforms.value.every(p =>
      targetSelectedTypes.value.every(ty => isHeaderOverrideCapable(p, ty))
    )
  )
})

// 是否全部为 Anthropic OAuth/SetupToken（RPM 配置仅在此条件下显示）
const allAnthropicOAuthOrSetupToken = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'anthropic' &&
    targetSelectedTypes.value.every(t => t === 'oauth' || t === 'setup-token')
  )
})

const filteredPresets = computed(() => {
  if (targetSelectedPlatforms.value.length === 0) return []

  const dedupedPresets = new Map<string, ReturnType<typeof getPresetMappingsByPlatform>[number]>()
  for (const platform of targetSelectedPlatforms.value) {
    for (const preset of getPresetMappingsByPlatform(platform)) {
      const key = `${preset.from}=>${preset.to}`
      if (!dedupedPresets.has(key)) {
        dedupedPresets.set(key, preset)
      }
    }
  }

  return Array.from(dedupedPresets.values())
})

// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

// State - field enable flags
const enableBaseUrl = ref(false)
const enableModelRestriction = ref(false)
const enableCustomErrorCodes = ref(false)
const enableInterceptWarmup = ref(false)
const enableHeaderOverride = ref(false)
const enableProxy = ref(false)
const enableConcurrency = ref(false)
const enableLoadFactor = ref(false)
const enablePriority = ref(false)
const enableRateMultiplier = ref(false)
const enableStatus = ref(false)
const enableGroups = ref(false)
const enableOpenAIPassthrough = ref(false)
const enableOpenAIFlattenNamespaces = ref(false)
const enableOpenAILongContextBilling = ref(false)
const enableOpenAIEndpointCapabilities = ref(false)
const enableOpenAIResponsesMode = ref(false)
const enableOpenAIWSMode = ref(false)
const enableOpenAIAPIKeyWSMode = ref(false)
const enableUpstreamBillingAutoProbe = ref(false)
const enableCodexCLIOnly = ref(false)
const enableCodexCLIOnlyAppServer = ref(false)
const enableOpenAICompactMode = ref(false)
const enableOpenAICompactModelMapping = ref(false)
const enableRpmLimit = ref(false)

// State - field values
const submitting = ref(false)
const showMixedChannelWarning = ref(false)
const mixedChannelWarningMessage = ref('')
const pendingUpdatesForConfirm = ref<Record<string, unknown> | null>(null)
const baseUrl = ref('')
const modelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const allowedModels = ref<string[]>([])
const modelMappings = ref<ModelMapping[]>([])
const selectedErrorCodes = ref<number[]>([])
const interceptWarmupRequests = ref(false)
const headerOverrideEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])
const proxyId = ref<number | null>(null)
const proxyIds = ref<number[]>([])
const concurrency = ref(1)
const loadFactor = ref<number | null>(null)
const priority = ref(1)
const rateMultiplier = ref(1)
const status = ref<'active' | 'inactive'>('active')
const groupIds = ref<number[]>([])
const openaiPassthroughEnabled = ref(false)
// Codex namespace 工具摊平兼容开关（仅 OAuth），缺省关闭即原样保留
const openaiFlattenNamespacesEnabled = ref(false)
const openAILongContextBillingEnabled = ref(false)
const openAIEndpointCapabilities = ref<OpenAIEndpointCapability[]>([
  'chat_completions',
  'embeddings'
])
const openAIResponsesMode = ref<OpenAIResponsesMode>('auto')
const openaiOAuthResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const openaiAPIKeyResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const upstreamBillingAutoProbeMode = ref<'enabled' | 'disabled'>('enabled')
const codexCLIOnlyEnabled = ref(false)
const codexCLIOnlyAppServerEnabled = ref(false)
type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
const enableCodexFingerprintMode = ref(false)
const codexFingerprintMode = ref<CodexFingerprintMode>('off')
const codexFingerprintModeOptions = computed(() => [
  { value: 'off' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintOff') },
  { value: 'device' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintDevice') },
  { value: 'session' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintSession') },
  { value: 'full' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintFull') },
])
const openAICompactMode = ref<OpenAICompactMode>('auto')
const openAICompactModelMappings = ref<ModelMapping[]>([])
const rpmLimitEnabled = ref(false)
const bulkBaseRpm = ref<number | null>(null)
const bulkRpmStrategy = ref<'tiered' | 'sticky_exempt'>('tiered')
const bulkRpmStickyBuffer = ref<number | null>(null)
const userMsgQueueMode = ref<string | null>(null)
const umqModeOptions = computed(() => [
  { value: '', label: t('admin.accounts.quotaControl.rpmLimit.umqModeOff') },
  { value: 'throttle', label: t('admin.accounts.quotaControl.rpmLimit.umqModeThrottle') },
  { value: 'serialize', label: t('admin.accounts.quotaControl.rpmLimit.umqModeSerialize') },
])

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])
const upstreamBillingAutoProbeOptions = computed(() => [
  { value: 'enabled', label: t('common.enabled') },
  { value: 'disabled', label: t('common.disabled') }
])
const isOpenAIModelRestrictionDisabled = computed(
  () =>
    allOpenAIPassthroughCapable.value &&
    enableOpenAIPassthrough.value &&
    openaiPassthroughEnabled.value
)

const openAIWSModeOptions = computed(() => [
  { value: OPENAI_WS_MODE_OFF, label: t('admin.accounts.openai.wsModeOff') },
  { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
  { value: OPENAI_WS_MODE_PASSTHROUGH, label: t('admin.accounts.openai.wsModePassthrough') },
  { value: OPENAI_WS_MODE_HTTP_BRIDGE, label: t('admin.accounts.openai.wsModeHttpBridge') }
])
const openAICompactModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.compactModeAuto') },
  { value: 'force_on', label: t('admin.accounts.openai.compactModeForceOn') },
  { value: 'force_off', label: t('admin.accounts.openai.compactModeForceOff') }
])
const openAIResponsesModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.responsesModeAuto') },
  { value: 'force_responses', label: t('admin.accounts.openai.responsesModeForceResponses') },
  {
    value: 'force_chat_completions',
    label: t('admin.accounts.openai.responsesModeForceChatCompletions')
  }
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
const openAIEndpointCapabilityOptions = computed<
  Array<{ value: OpenAIEndpointCapability; label: string }>
>(() => [
  { value: 'chat_completions', label: openAITextEndpointCapabilityLabel.value },
  { value: 'embeddings', label: t('admin.accounts.openai.capabilityEmbeddings') }
])
const openAITextGenerationCapabilityEnabled = computed(() =>
  openAIEndpointCapabilities.value.includes('chat_completions')
)
const openAIResponsesModeApplicable = computed(
  () => !enableOpenAIEndpointCapabilities.value || openAITextGenerationCapabilityEnabled.value
)

const normalizeOpenAIEndpointCapabilities = (values: OpenAIEndpointCapability[]) => {
  const allowed: OpenAIEndpointCapability[] = ['chat_completions', 'embeddings']
  const selected = allowed.filter((value) => values.includes(value))
  return selected.length > 0 ? selected : allowed
}

const toggleOpenAIEndpointCapability = (
  capability: OpenAIEndpointCapability,
  event?: Event
) => {
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
const openAIWSModeConcurrencyHintKey = computed(() =>
  resolveOpenAIWSModeConcurrencyHintKey(openaiOAuthResponsesWebSocketV2Mode.value)
)
const openAIAPIKeyWSModeConcurrencyHintKey = computed(() =>
  resolveOpenAIWSModeConcurrencyHintKey(openaiAPIKeyResponsesWebSocketV2Mode.value)
)

// Model mapping helpers (whitelist/mapping editing lives in ModelRestrictionSection /
// OpenAICompactModelMappingSection)

const buildModelMappingObject = (): Record<string, string> | null => {
  return buildModelMappingPayload(
    modelRestrictionMode.value,
    allowedModels.value,
    modelMappings.value
  )
}

const buildOpenAICompactModelMapping = (): Record<string, string> | null => {
  return buildModelMappingPayload('mapping', [], openAICompactModelMappings.value)
}

const buildUpdatePayload = (): Record<string, unknown> | null => {
  const updates: Record<string, unknown> = {}
  const credentials: Record<string, unknown> = {}
  let credentialsChanged = false
  const applyOpenAILongContextBilling =
    enableOpenAILongContextBilling.value && allOpenAIPassthroughCapable.value
  const applyOpenAIEndpointCapabilities =
    enableOpenAIEndpointCapabilities.value && allOpenAIAPIKey.value
  const applyOpenAIResponsesMode = enableOpenAIResponsesMode.value && allOpenAIAPIKey.value
  const ensureExtra = (): Record<string, unknown> => {
    if (!updates.extra) {
      updates.extra = {}
    }
    return updates.extra as Record<string, unknown>
  }

  if (enableProxy.value) {
    if (proxyId.value === PROXY_ROTATION_VALUE) {
      if (proxyIds.value.length === 0) {
        appStore.showError(t('admin.accounts.bulkEdit.proxyRotationEmpty'))
        return null
      }
      updates.proxy_ids = [...proxyIds.value]
    } else {
      // 后端期望 proxy_id: 0 表示清除代理，而不是 null
      updates.proxy_id = proxyId.value === null ? 0 : proxyId.value
    }
  }

  if (enableConcurrency.value) {
    updates.concurrency = concurrency.value
  }

  if (enableLoadFactor.value) {
    // 空值/NaN/0 时发送 0（后端约定 <= 0 表示清除）
    const lf = loadFactor.value
    updates.load_factor = (lf != null && !Number.isNaN(lf) && lf > 0) ? lf : 0
  }

  if (enablePriority.value) {
    updates.priority = priority.value
  }

  if (enableRateMultiplier.value) {
    updates.rate_multiplier = rateMultiplier.value
  }

  if (enableStatus.value) {
    updates.status = status.value
  }

  if (enableGroups.value) {
    updates.group_ids = groupIds.value
  }

  if (enableBaseUrl.value) {
    const baseUrlValue = baseUrl.value.trim()
    if (baseUrlValue) {
      credentials.base_url = baseUrlValue
      credentialsChanged = true
    }
  }

  if (enableOpenAIPassthrough.value) {
    const extra = ensureExtra()
    extra.openai_passthrough = openaiPassthroughEnabled.value
    if (!openaiPassthroughEnabled.value) {
      extra.openai_oauth_passthrough = false
    }
  }

  // 同时校验可见性：勾选后又改了目标筛选条件时，不应把该键写到非 OAuth 账号上
  if (enableOpenAIFlattenNamespaces.value && allOpenAIOAuthOnly.value) {
    const extra = ensureExtra()
    extra.openai_responses_flatten_namespaces = openaiFlattenNamespacesEnabled.value
  }

  if (applyOpenAILongContextBilling) {
    const extra = ensureExtra()
    extra.openai_long_context_billing_enabled = openAILongContextBillingEnabled.value
  }

  if (applyOpenAIEndpointCapabilities) {
    credentials.openai_capabilities =
      openAIEndpointCapabilities.value.length === 2
        ? null
        : [...openAIEndpointCapabilities.value]
    credentialsChanged = true
  }

  if (
    applyOpenAIResponsesMode ||
    (applyOpenAIEndpointCapabilities && !openAITextGenerationCapabilityEnabled.value)
  ) {
    const extra = ensureExtra()
    extra.openai_responses_mode =
      !openAIResponsesModeApplicable.value || openAIResponsesMode.value === 'auto'
        ? null
        : openAIResponsesMode.value
  }

  if (enableModelRestriction.value && !isOpenAIModelRestrictionDisabled.value) {
    // 统一使用 model_mapping 字段
    if (modelRestrictionMode.value === 'whitelist') {
      // 白名单模式：将模型转换为 model_mapping 格式（key=value）
      // 空白名单表示“支持所有模型”，需显式发送空对象以覆盖已有限制。
      const mapping: Record<string, string> = {}
      for (const m of allowedModels.value) {
        mapping[m] = m
      }
      credentials.model_mapping = mapping
      credentialsChanged = true
    } else {
      // 映射模式下空配置同样表示“支持所有模型”。
      const modelMapping = buildModelMappingObject()
      credentials.model_mapping = modelMapping ?? {}
      credentialsChanged = true
    }
  }

  if (enableCustomErrorCodes.value) {
    credentials.custom_error_codes_enabled = true
    credentials.custom_error_codes = [...selectedErrorCodes.value]
    credentialsChanged = true
  }

  if (enableInterceptWarmup.value) {
    credentials.intercept_warmup_requests = interceptWarmupRequests.value
    credentialsChanged = true
  }

  if (enableHeaderOverride.value) {
    // 后端使用 JSONB || merge 语义：关闭时显式写入 false + 空对象以清除旧配置
    credentials[HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY] = headerOverrideEnabled.value
    credentials[HEADER_OVERRIDES_CREDENTIAL_KEY] = headerOverrideEnabled.value
      ? buildHeaderOverridesObject(headerOverrideRows.value)
      : {}
    credentialsChanged = true
  }

  if (enableOpenAIWSMode.value) {
    const extra = ensureExtra()
    extra.openai_oauth_responses_websockets_v2_mode = openaiOAuthResponsesWebSocketV2Mode.value
    extra.openai_oauth_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(
      openaiOAuthResponsesWebSocketV2Mode.value
    )
  }

  if (enableOpenAIAPIKeyWSMode.value) {
    const extra = ensureExtra()
    extra.openai_apikey_responses_websockets_v2_mode = openaiAPIKeyResponsesWebSocketV2Mode.value
    extra.openai_apikey_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(
      openaiAPIKeyResponsesWebSocketV2Mode.value
    )
  }

  if (enableUpstreamBillingAutoProbe.value) {
    updates.upstream_billing_probe_enabled = upstreamBillingAutoProbeMode.value === 'enabled'
  }

  if (enableCodexCLIOnly.value) {
    const extra = ensureExtra()
    extra.codex_cli_only = codexCLIOnlyEnabled.value
  }

  // 子开关从属于 codex_cli_only：仅当同一次批量编辑也把父开关设为开启时才写入，
  // 与 Create/Edit 语义对齐，避免在父开关关闭的账号上写入无意义的孤立字段。
  if (
    enableCodexCLIOnlyAppServer.value &&
    enableCodexCLIOnly.value &&
    codexCLIOnlyEnabled.value
  ) {
    const extra = ensureExtra()
    extra.codex_cli_only_allow_app_server = codexCLIOnlyAppServerEnabled.value
  }

  if (enableCodexFingerprintMode.value) {
    const extra = ensureExtra()
    // off 必须显式落键，不能靠删本地键表达。批量更新走 JSONB 顶层合并
    // （extra = COALESCE(extra,'{}') || payload），删掉 payload 里的键只表示
    // "本次不更新该键"，清不掉账号上已有的 device/session/full；而且只删不写会让
    // 整个 payload 退化成 {extra:{}}，被后端 len(req.Extra) > 0 判为空更新直接 400
    // "No updates provided"（#6327）。
    //
    // Create/Edit 那两个表单可以删键，是因为它们提交完整 extra 对象、后端整体
    // SetExtra 覆盖；批量接口只合并增量键，两种持久化语义不能共用同一套写法。
    //
    // 显式 off 与不设置在读取侧完全等价：codexFingerprintModeFromExtra 对空值/
    // 非法值走 default 回落 off，对 "off" 命中同一分支，所以 #5610 定下的
    // "不显式 opt-in 就保持旧客户端身份" 不受影响；ShouldEnsureCodexFingerprintSeed-
    // ForExtraUpdates 同样只在 device/session/full 时要种子，off 不会触发。
    //
    // 与本函数里其它"关闭/清除"字段的写法一致：codex_cli_only 直接落 false，
    // load_factor 落 0，proxy_id 落 0 —— 批量路径一律用显式哨兵值，不用省略。
    extra.codex_fingerprint_mode = codexFingerprintMode.value
  }

  if (enableOpenAICompactMode.value) {
    const extra = ensureExtra()
    extra.openai_compact_mode = openAICompactMode.value
  }

  if (enableOpenAICompactModelMapping.value) {
    credentials.compact_model_mapping = buildOpenAICompactModelMapping() ?? {}
    credentialsChanged = true
  }

  // RPM limit settings (写入 extra 字段)
  if (enableRpmLimit.value) {
    const extra = ensureExtra()
    if (rpmLimitEnabled.value && bulkBaseRpm.value != null && bulkBaseRpm.value > 0) {
      extra.base_rpm = bulkBaseRpm.value
      extra.rpm_strategy = bulkRpmStrategy.value
      if (bulkRpmStickyBuffer.value != null && bulkRpmStickyBuffer.value > 0) {
        extra.rpm_sticky_buffer = bulkRpmStickyBuffer.value
      }
    } else {
      // 关闭 RPM 限制 - 设置 base_rpm 为 0，并用空值覆盖关联字段
      // 后端使用 JSONB || merge 语义，不会删除已有 key，
      // 所以必须显式发送空值来重置（后端读取时会 fallback 到默认值）
      extra.base_rpm = 0
      extra.rpm_strategy = ''
      extra.rpm_sticky_buffer = 0
    }
    updates.extra = extra
  }

  // UMQ mode（独立于 RPM 保存）
  if (userMsgQueueMode.value !== null) {
    const umqExtra = ensureExtra()
    umqExtra.user_msg_queue_mode = userMsgQueueMode.value  // '' = 清除账号级覆盖
    umqExtra.user_msg_queue_enabled = false  // 清理旧字段（JSONB merge）
  }

  if (credentialsChanged) {
    updates.credentials = credentials
  }

  return Object.keys(updates).length > 0 ? updates : null
}

const mixedChannelConfirmed = ref(false)

// 是否需要预检查：改了分组 + 全是单一的 antigravity 或 anthropic 平台
// 多平台混合的情况由 submitBulkUpdate 的 409 catch 兜底
const canPreCheck = () =>
  enableGroups.value &&
  groupIds.value.length > 0 &&
  targetSelectedPlatforms.value.length === 1 &&
  (targetSelectedPlatforms.value[0] === 'antigravity' || targetSelectedPlatforms.value[0] === 'anthropic')

const handleClose = () => {
  showMixedChannelWarning.value = false
  mixedChannelWarningMessage.value = ''
  pendingUpdatesForConfirm.value = null
  mixedChannelConfirmed.value = false
  emit('close')
}

// 预检查：提交前调接口检测，有风险就弹窗阻止，返回 false 表示需要用户确认
const preCheckMixedChannelRisk = async (built: Record<string, unknown>): Promise<boolean> => {
  if (!canPreCheck()) return true
  if (mixedChannelConfirmed.value) return true

  try {
    const result = await adminAPI.accounts.checkMixedChannelRisk({
      platform: targetSelectedPlatforms.value[0],
      group_ids: groupIds.value
    })
    if (!result.has_risk) return true

    pendingUpdatesForConfirm.value = built
    mixedChannelWarningMessage.value = result.message || t('admin.accounts.bulkEdit.failed')
    showMixedChannelWarning.value = true
    return false
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.bulkEdit.failed'))
    return false
  }
}

const handleSubmit = async () => {
  if (targetMode.value === 'selected' && props.accountIds.length === 0) {
    appStore.showError(t('admin.accounts.bulkEdit.noSelection'))
    return
  }

  const hasAnyFieldEnabled =
    enableBaseUrl.value ||
    enableOpenAIPassthrough.value ||
    enableOpenAIFlattenNamespaces.value ||
    (enableOpenAILongContextBilling.value && allOpenAIPassthroughCapable.value) ||
    (enableOpenAIEndpointCapabilities.value && allOpenAIAPIKey.value) ||
    (enableOpenAIResponsesMode.value && allOpenAIAPIKey.value) ||
    enableModelRestriction.value ||
    enableCustomErrorCodes.value ||
    enableInterceptWarmup.value ||
    enableHeaderOverride.value ||
    enableProxy.value ||
    enableConcurrency.value ||
    enableLoadFactor.value ||
    enablePriority.value ||
    enableRateMultiplier.value ||
    enableStatus.value ||
    enableGroups.value ||
    enableOpenAIWSMode.value ||
    enableOpenAIAPIKeyWSMode.value ||
    enableUpstreamBillingAutoProbe.value ||
    enableCodexCLIOnly.value ||
    enableCodexCLIOnlyAppServer.value ||
    enableCodexFingerprintMode.value ||
    enableOpenAICompactMode.value ||
    enableOpenAICompactModelMapping.value ||
    enableRpmLimit.value ||
    userMsgQueueMode.value !== null

  if (!hasAnyFieldEnabled) {
    appStore.showError(t('admin.accounts.bulkEdit.noFieldsSelected'))
    return
  }

  // base_url 现在也会作用于 Grok OAuth 订阅账号的转发端点；坏值会让请求期
  // 校验失败、账号请求全挂，因此保存前强制格式校验（与单账号编辑一致）。
  if (enableBaseUrl.value) {
    const trimmedBaseUrl = baseUrl.value.trim()
    if (trimmedBaseUrl && !/^https?:\/\//i.test(trimmedBaseUrl)) {
      appStore.showError(t('admin.accounts.grokCustomBaseUrl.invalid'))
      return
    }
  }

  if (enableHeaderOverride.value && headerOverrideEnabled.value) {
    // 批量保存对 header_overrides 是整键替换：开启但没有任何有效行会把所选账号的
    // 既有覆写配置静默清空，必须显式拦截（清空请走关闭开关的路径，有专门提示）
    if (!headerOverrideRows.value.some((row) => row.name.trim())) {
      appStore.showError(t('admin.accounts.headerOverride.bulkEmptyRows'))
      return
    }
    const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
    if (headerError) {
      appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
      return
    }
  }

  const built = buildUpdatePayload()
  if (!built) {
    appStore.showError(t('admin.accounts.bulkEdit.noFieldsSelected'))
    return
  }

  const canContinue = await preCheckMixedChannelRisk(built)
  if (!canContinue) return

  await submitBulkUpdate(built)
}

const submitBulkUpdate = async (baseUpdates: Record<string, unknown>) => {
  // 无论是预检查确认还是 409 兜底确认，只要 mixedChannelConfirmed 为 true 就带上 flag
  const updates = mixedChannelConfirmed.value
    ? { ...baseUpdates, confirm_mixed_channel_risk: true }
    : baseUpdates

  submitting.value = true

  try {
    const res = targetMode.value === 'filtered' && props.target?.filters
      ? await adminAPI.accounts.bulkUpdate({
        filters: props.target.filters,
        ...updates
      })
      : await adminAPI.accounts.bulkUpdate(props.accountIds, updates)
    const success = res.success || 0
    const failed = res.failed || 0
    const inherited = res.long_context_inherited_count || 0

    if (success > 0 && failed === 0) {
      if (inherited > 0) {
        appStore.showSuccess(t('admin.accounts.bulkEdit.successWithInherited', {
          count: success,
          inherited
        }))
      } else {
        appStore.showSuccess(t('admin.accounts.bulkEdit.success', { count: success }))
      }
    } else if (success > 0) {
      const key = inherited > 0
        ? 'admin.accounts.bulkEdit.partialSuccessWithInherited'
        : 'admin.accounts.bulkEdit.partialSuccess'
      appStore.showError(t(key, { success, failed, inherited }))
    } else {
      appStore.showError(t('admin.accounts.bulkEdit.failed'))
    }

    if (success > 0) {
      pendingUpdatesForConfirm.value = null
      emit('updated')
      handleClose()
    }
  } catch (error: any) {
    // 兜底：多平台混合场景下，预检查跳过，由后端 409 触发确认框
    if (error.status === 409 && error.error === 'mixed_channel_warning') {
      pendingUpdatesForConfirm.value = baseUpdates
      mixedChannelWarningMessage.value = error.message
      showMixedChannelWarning.value = true
    } else if (error.reason === 'UPSTREAM_BILLING_RATE_SYNC_BULK_CONFLICT') {
      appStore.showError(t('admin.accounts.bulkEdit.rateSyncConflict', {
        count: error.metadata?.count ?? 1
      }))
    } else if (error.reason === 'OPENAI_LONG_CONTEXT_PARENT_REQUIRED') {
      appStore.showError(t('admin.accounts.bulkEdit.longContextParentRequired'))
    } else {
      appStore.showError(error.message || t('admin.accounts.bulkEdit.failed'))
      console.error('Error bulk updating accounts:', error)
    }
  } finally {
    submitting.value = false
  }
}

const handleMixedChannelConfirm = async () => {
  showMixedChannelWarning.value = false
  mixedChannelConfirmed.value = true
  if (pendingUpdatesForConfirm.value) {
    await submitBulkUpdate(pendingUpdatesForConfirm.value)
  }
}

const handleMixedChannelCancel = () => {
  showMixedChannelWarning.value = false
  pendingUpdatesForConfirm.value = null
}

// Reset form when modal closes
watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      // Reset all enable flags
      enableBaseUrl.value = false
      enableModelRestriction.value = false
      enableCustomErrorCodes.value = false
      enableInterceptWarmup.value = false
      enableHeaderOverride.value = false
      enableProxy.value = false
      enableConcurrency.value = false
      enableLoadFactor.value = false
      enablePriority.value = false
      enableRateMultiplier.value = false
      enableStatus.value = false
      enableGroups.value = false
      enableOpenAIPassthrough.value = false
      enableOpenAIFlattenNamespaces.value = false
      enableOpenAILongContextBilling.value = false
      enableOpenAIEndpointCapabilities.value = false
      enableOpenAIResponsesMode.value = false
      enableOpenAIWSMode.value = false
      enableOpenAIAPIKeyWSMode.value = false
      enableUpstreamBillingAutoProbe.value = false
      enableCodexCLIOnly.value = false
      enableCodexCLIOnlyAppServer.value = false
      enableCodexFingerprintMode.value = false
      codexFingerprintMode.value = 'off'
      enableOpenAICompactMode.value = false
      enableOpenAICompactModelMapping.value = false
      enableRpmLimit.value = false

      // Reset all values
      baseUrl.value = ''
      openaiPassthroughEnabled.value = false
      openaiFlattenNamespacesEnabled.value = false
      openAILongContextBillingEnabled.value = false
      openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
      openAIResponsesMode.value = 'auto'
      modelRestrictionMode.value = 'whitelist'
      allowedModels.value = []
      modelMappings.value = []
      selectedErrorCodes.value = []
      interceptWarmupRequests.value = false
      headerOverrideEnabled.value = false
      headerOverrideRows.value = []
      proxyId.value = null
      proxyIds.value = []
      concurrency.value = 1
      loadFactor.value = null
      priority.value = 1
      rateMultiplier.value = 1
      status.value = 'active'
      groupIds.value = []
      openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
      openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
      upstreamBillingAutoProbeMode.value = 'enabled'
      codexCLIOnlyEnabled.value = false
      codexCLIOnlyAppServerEnabled.value = false
      openAICompactMode.value = 'auto'
      openAICompactModelMappings.value = []
      rpmLimitEnabled.value = false
      bulkBaseRpm.value = null
      bulkRpmStrategy.value = 'tiered'
      bulkRpmStickyBuffer.value = null
      userMsgQueueMode.value = null

      // Reset mixed channel warning state
      showMixedChannelWarning.value = false
      mixedChannelWarningMessage.value = ''
      pendingUpdatesForConfirm.value = null
      mixedChannelConfirmed.value = false
    }
  }
)
</script>
