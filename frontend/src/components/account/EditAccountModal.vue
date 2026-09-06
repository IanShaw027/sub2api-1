<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.editAccount')"
    width="wide"
    @close="handleClose"
  >
    <form
      v-if="account"
      id="edit-account-form"
      @submit.prevent="handleSubmit"
      class="space-y-5"
    >
      <div>
        <label class="input-label">{{ t('common.name') }}</label>
        <input v-model="form.name" type="text" required class="input" data-tour="edit-account-form-name" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.notes') }}</label>
        <textarea
          v-model="form.notes"
          rows="3"
          class="input"
          :placeholder="t('admin.accounts.notesPlaceholder')"
        ></textarea>
        <p class="input-hint">{{ t('admin.accounts.notesHint') }}</p>
      </div>

      <!-- Kiro section (extracted) -->
      <EditKiroSection
        v-model:edit-api-key="editApiKey"
        v-model:selected-kiro-profile-arn-choice="selectedKiroProfileArnChoice"
        v-model:model-restriction-mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        :account="account"
        :discovering-kiro-profiles="discoveringKiroProfiles"
        :discover-kiro-profiles-for-edit="discoverKiroProfilesForEdit"
        :kiro-profile-select-options="kiroProfileSelectOptions"
        :kiro-profile-choice-keep="KIRO_PROFILE_CHOICE_KEEP"
        :kiro-profile-choice-auto="KIRO_PROFILE_CHOICE_AUTO"
        :kiro-profile-status-badge-class="kiroProfileStatusBadgeClass"
        :kiro-profile-status-label="kiroProfileStatusLabel"
        :kiro-profile-pending-hint="kiroProfilePendingHint"
        :current-kiro-profile-arn="currentKiroProfileArn"
        :preset-mappings="presetMappings"
      />

      <!-- API Key / OAuth fields section (extracted) -->
      <EditApiKeyOAuthFieldsSection
        v-model:edit-api-protocol="editApiProtocol"
        v-model:edit-base-url="editBaseUrl"
        v-model:edit-account-mode="editAccountMode"
        v-model:edit-adaptive-base-urls="editAdaptiveBaseUrls"
        v-model:edit-zhipu-organization="editZhipuOrganization"
        v-model:edit-zhipu-project="editZhipuProject"
        v-model:edit-api-key="editApiKey"
        v-model:model-restriction-mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        v-model:pool-mode-enabled="poolModeEnabled"
        v-model:pool-mode-retry-count="poolModeRetryCount"
        v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
        v-model:custom-error-codes-enabled="customErrorCodesEnabled"
        v-model:selected-error-codes="selectedErrorCodes"
        v-model:grok-client-tool-cache-enabled="grokClientToolCacheEnabled"
        v-model:grok-o-auth-custom-base-url-enabled="grokOAuthCustomBaseUrlEnabled"
        v-model:grok-o-auth-base-url="grokOAuthBaseUrl"
        v-model:header-override-enabled="headerOverrideEnabled"
        v-model:header-override-rows="headerOverrideRows"
        :account="account"
        :is-c-n-api-key-account="isCNApiKeyAccount"
        :base-url-hint="baseUrlHint"
        :edit-adaptive-protocol-options="editAdaptiveProtocolOptions"
        :cn-preset-platform="cnPresetPlatform"
        :on-cn-preset-select="onCnPresetSelect"
        :cn-supports-native-responses="cnSupportsNativeResponses"
        :cn-account-mode-options="cnAccountModeOptions"
        :cn-protocol-options="cnProtocolOptions"
        :cn-protocol-desc-key="cnProtocolDescKey"
        :is-open-a-i-model-restriction-disabled="isOpenAIModelRestrictionDisabled"
        :preset-mappings="presetMappings"
      />

      <!-- Upstream fields (only for upstream type) -->
      <div v-if="account.type === 'upstream'" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.upstream.baseUrl') }}</label>
          <input
            v-model="editBaseUrl"
            type="text"
            class="input"
            placeholder="https://cloudcode-pa.googleapis.com"
          />
          <p class="input-hint">{{ t('admin.accounts.upstream.baseUrlHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.upstream.apiKey') }}</label>
          <input
            v-model="editApiKey"
            type="password"
            class="input font-mono"
            placeholder="sk-..."
          />
          <p class="input-hint">{{ t('admin.accounts.leaveEmptyToKeep') }}</p>
        </div>
      </div>

      <!-- Vertex Service Account -->
      <div v-if="(account.platform === 'gemini' || account.platform === 'anthropic') && account.type === 'service_account'" class="space-y-4">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">Project ID</label>
            <input
              v-model="editVertexProjectId"
              type="text"
              class="input font-mono"
              readonly
              :placeholder="t('admin.accounts.vertexProjectIdPlaceholder')"
            />
            <p class="input-hint">{{ t('admin.accounts.vertexSaJsonEditHint') }}</p>
          </div>
          <div>
            <label class="input-label">Location</label>
            <select
              v-model="editVertexLocation"
              required
              class="input font-mono"
            >
              <optgroup
                v-for="group in VERTEX_LOCATION_OPTIONS"
                :key="group.label"
                :label="group.label"
              >
                <option
                  v-for="option in group.options"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </optgroup>
            </select>
            <p class="input-hint">{{ t('admin.accounts.vertexLocationHint') }}</p>
          </div>
        </div>

        <!-- Model Restriction Section for Service Account -->
        <div class="border-t border-line pt-4">
          <ModelRestrictionEditor
            v-model:mode="modelRestrictionMode"
            v-model:allowed-models="allowedModels"
            v-model:model-mappings="modelMappings"
            :platform="account?.platform || 'anthropic'"
            :account-id="account?.id"
            :presets="presetMappings"
          />
        </div>
      </div>

      <!-- Bedrock fields (for bedrock type, both SigV4 and API Key modes) -->
      <EditBedrockCredentialsSection
        v-if="account.type === 'bedrock'"
        v-model:edit-bedrock-access-key-id="editBedrockAccessKeyId"
        v-model:edit-bedrock-secret-access-key="editBedrockSecretAccessKey"
        v-model:edit-bedrock-session-token="editBedrockSessionToken"
        v-model:edit-bedrock-api-key-value="editBedrockApiKeyValue"
        v-model:edit-bedrock-region="editBedrockRegion"
        v-model:edit-bedrock-force-global="editBedrockForceGlobal"
        v-model:model-restriction-mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        v-model:pool-mode-enabled="poolModeEnabled"
        v-model:pool-mode-retry-count="poolModeRetryCount"
        v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
        :is-bedrock-api-key-mode="isBedrockAPIKeyMode"
        :bedrock-presets="bedrockPresets"
        :get-model-mapping-key="getModelMappingKey"
      />

      <div
        v-if="account.platform === 'antigravity' && account.type === 'oauth'"
        class="border-t border-line pt-4"
      >
        <label class="input-label">{{ t('admin.accounts.antigravityProjectIdLabel') }}</label>
        <input
          v-model="antigravityProjectId"
          data-testid="antigravity-project-id-input"
          type="text"
          class="input font-mono"
          :placeholder="t('admin.accounts.antigravityProjectIdPlaceholder')"
        />
        <p class="input-hint">{{ t('admin.accounts.antigravityProjectIdHint') }}</p>
      </div>

      <!-- Antigravity model restriction (applies to all antigravity types) -->
      <EditAntigravityModelMappingSection
        v-model:antigravity-model-mappings="antigravityModelMappings"
        :account="account"
      />

      <TempUnschedulableRulesSection
        v-model:tempUnschedEnabled="tempUnschedEnabled"
        v-model:tempUnschedRules="tempUnschedRules"
        :temp-unsched-presets="tempUnschedPresets"
        :get-temp-unsched-rule-key="getTempUnschedRuleKey"
        :add-temp-unsched-rule="addTempUnschedRule"
        :remove-temp-unsched-rule="removeTempUnschedRule"
        :move-temp-unsched-rule="moveTempUnschedRule"
      />


      <EditSchedulingSection
        v-model:account-scheduling-threshold-override-enabled="accountSchedulingThresholdOverrideEnabled"
        v-model:account-scheduling-threshold-override-value="accountSchedulingThresholdOverrideValue"
        v-model:intercept-warmup-requests="interceptWarmupRequests"
        :account="account"
        :proxies="proxies"
        :form="form"
        :is-spark-shadow="isSparkShadow"
        :supports-account-scheduling-threshold-override="supportsAccountSchedulingThresholdOverride"
        :upstream-billing-rate-sync-enabled="upstreamBillingRateSyncEnabled"
        :handle-upstream-billing-rate-sync-change="handleUpstreamBillingRateSyncChange"
        :browser-time-zone="browserTimeZone"
      />

      <EditAdvancedOptionsSection
        v-model:openai-passthrough-enabled="openaiPassthroughEnabled"
        v-model:openai-flatten-namespaces-enabled="openaiFlattenNamespacesEnabled"
        v-model:codex-image-tool-mode="codexImageToolMode"
        v-model:openai-responses-web-socket-v2-mode="openaiResponsesWebSocketV2Mode"
        v-model:open-a-i-responses-mode="openAIResponsesMode"
        v-model:anthropic-passthrough-enabled="anthropicPassthroughEnabled"
        v-model:anthropic-a-p-i-key-auth-scheme="anthropicAPIKeyAuthScheme"
        v-model:web-search-emulation-mode="webSearchEmulationMode"
        v-model:edit-quota-limit="editQuotaLimit"
        v-model:edit-quota-daily-limit="editQuotaDailyLimit"
        v-model:edit-quota-weekly-limit="editQuotaWeeklyLimit"
        v-model:edit-daily-reset-mode="editDailyResetMode"
        v-model:edit-daily-reset-hour="editDailyResetHour"
        v-model:edit-weekly-reset-mode="editWeeklyResetMode"
        v-model:edit-weekly-reset-day="editWeeklyResetDay"
        v-model:edit-weekly-reset-hour="editWeeklyResetHour"
        v-model:edit-reset-timezone="editResetTimezone"
        v-model:open-a-i-long-context-billing-enabled="openAILongContextBillingEnabled"
        v-model:codex-c-l-i-only-enabled="codexCLIOnlyEnabled"
        v-model:codex-c-l-i-only-app-server-enabled="codexCLIOnlyAppServerEnabled"
        v-model:codex-fingerprint-mode="codexFingerprintMode"
        v-model:edit-plan-type="editPlanType"
        v-model:open-a-i-compact-mode="openAICompactMode"
        v-model:open-a-i-compact-model-mappings="openAICompactModelMappings"
        v-model:auto-pause-on-expired="autoPauseOnExpired"
        v-model:auto-pause5h-disabled="autoPause5hDisabled"
        v-model:auto-pause5h-threshold="autoPause5hThreshold"
        v-model:auto-pause7d-disabled="autoPause7dDisabled"
        v-model:auto-pause7d-threshold="autoPause7dThreshold"
        v-model:auto-reset-credit-enabled="autoResetCreditEnabled"
        v-model:auto-reset-credit5h-threshold="autoResetCredit5hThreshold"
        v-model:auto-reset-credit7d-threshold="autoResetCredit7dThreshold"
        v-model:window-cost-enabled="windowCostEnabled"
        v-model:window-cost-limit="windowCostLimit"
        v-model:window-cost-sticky-reserve="windowCostStickyReserve"
        v-model:session-limit-enabled="sessionLimitEnabled"
        v-model:max-sessions="maxSessions"
        v-model:session-idle-timeout="sessionIdleTimeout"
        v-model:rpm-limit-enabled="rpmLimitEnabled"
        v-model:base-rpm="baseRpm"
        v-model:rpm-strategy="rpmStrategy"
        v-model:rpm-sticky-buffer="rpmStickyBuffer"
        v-model:user-msg-queue-mode="userMsgQueueMode"
        v-model:session-id-masking-enabled="sessionIdMaskingEnabled"
        v-model:cache-t-t-l-override-enabled="cacheTTLOverrideEnabled"
        v-model:cache-t-t-l-override-target="cacheTTLOverrideTarget"
        v-model:custom-base-url-enabled="customBaseUrlEnabled"
        v-model:custom-base-url="customBaseUrl"
        v-model:tls-fingerprint-enabled="tlsFingerprintEnabled"
        v-model:tls-fingerprint-profile-id="tlsFingerprintProfileId"
        v-model:tls-fingerprint-router-id="tlsFingerprintRouterId"
        v-model:tls-fingerprint-default-o-s="tlsFingerprintDefaultOS"
        v-model:tls-fingerprint-binding-rows="tlsFingerprintBindingRows"
        v-model:device-learning-enabled="deviceLearningEnabled"
        v-model:device-t-l-s-profile-selection="deviceTLSProfileSelection"
        v-model:mixed-scheduling="mixedScheduling"
        v-model:allow-overages="allowOverages"
        :account="account"
        :is-spark-shadow="isSparkShadow"
        :hide-account-long-context-billing="hideAccountLongContextBilling"
        :web-search-global-enabled="webSearchGlobalEnabled"
        :quota-notify-global-enabled="quotaNotifyGlobalEnabled"
        :quota-notify-state="quotaNotifyState"
        :form="form"
        :codex-image-tool-badge-class="codexImageToolBadgeClass"
        :codex-image-tool-badge-label="codexImageToolBadgeLabel"
        :codex-image-tool-options="codexImageToolOptions"
        :open-a-i-w-s-mode-concurrency-hint-key="openAIWSModeConcurrencyHintKey"
        :open-a-i-w-s-mode-options="openAIWSModeOptions"
        :open-a-i-responses-mode-options="openAIResponsesModeOptions"
        :open-a-i-text-generation-capability-enabled="openAITextGenerationCapabilityEnabled"
        :open-a-i-responses-status-key="openAIResponsesStatusKey"
        :open-a-i-endpoint-capability-options="openAIEndpointCapabilityOptions"
        :open-a-i-endpoint-capabilities="openAIEndpointCapabilities"
        :toggle-open-a-i-endpoint-capability="toggleOpenAIEndpointCapability"
        :upstream-billing-auto-probe-enabled="upstreamBillingAutoProbeEnabled"
        :handle-upstream-billing-auto-probe-change="handleUpstreamBillingAutoProbeChange"
        :handle-ollama-cloud-usage-updated="handleOllamaCloudUsageUpdated"
        :codex-fingerprint-mode-options="codexFingerprintModeOptions"
        :plan-type-options="planTypeOptions"
        :open-a-i-compact-mode-options="openAICompactModeOptions"
        :open-a-i-compact-status-key="openAICompactStatusKey"
        :supports-t-l-s-fingerprint="supportsTLSFingerprint"
        :tls-fingerprint-profiles="tlsFingerprintProfiles"
        :tls-fingerprint-routers="tlsFingerprintRouters"
        :device-t-l-s-catalog-options="deviceTLSCatalogOptions"
        :format-device-t-l-s-profile-label="formatDeviceTLSProfileLabel"
        :status-options="statusOptions"
      />

      <!-- Group Selection - 仅标准模式显示 -->
      <GroupSelector
        v-if="!authStore.isSimpleMode"
        v-model="form.group_ids"
        :groups="groups"
        :platform="account?.platform"
        :mixed-scheduling="mixedScheduling"
        data-tour="account-form-groups"
      />

    </form>

    <template #footer>
      <div v-if="account" class="flex justify-end gap-3">
        <button @click="handleClose" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="edit-account-form"
          :disabled="submitting"
          class="btn btn-primary"
          data-tour="account-form-submit"
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
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ submitting ? t('admin.accounts.updating') : t('common.update') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <!-- Mixed Channel Warning Dialog -->
  <ConfirmDialog
    :show="showMixedChannelWarning"
    :title="t('admin.accounts.mixedChannelWarningTitle')"
    :message="mixedChannelWarningMessageText"
    :confirm-text="t('common.confirm')"
    :cancel-text="t('common.cancel')"
    :danger="true"
    @confirm="handleMixedChannelConfirm"
    @cancel="handleMixedChannelCancel"
  />
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import { useEditAccountMixedChannel } from './edit/useEditAccountMixedChannel'
import { useEditAccountSubmit } from './edit/useEditAccountSubmit'
import { useEditAccountSync } from './edit/useEditAccountSync'
import { useEditAccountOpenAIOptions, type CodexFingerprintMode, type CodexImageToolMode } from './edit/useEditAccountOpenAIOptions'
import { useEditAccountTlsFingerprint } from './edit/useEditAccountTlsFingerprint'
import EditBedrockCredentialsSection from './edit/EditBedrockCredentialsSection.vue'
import EditAdvancedOptionsSection from './edit/EditAdvancedOptionsSection.vue'
import EditApiKeyOAuthFieldsSection from './edit/EditApiKeyOAuthFieldsSection.vue'
import EditAntigravityModelMappingSection from './edit/EditAntigravityModelMappingSection.vue'
import EditKiroSection from './edit/EditKiroSection.vue'
import EditSchedulingSection from './edit/EditSchedulingSection.vue'
import type { KiroDiscoveredProfile } from '@/api/admin/accounts'
import { useQuotaNotifyState } from '@/composables/useQuotaNotifyState'
import type {
  Account,
  Proxy,
  AdminGroup,
  OpenAICompactMode,
  OpenAIResponsesMode,
  OpenAIEndpointCapability,
  OllamaCloudUsageState
} from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useTempUnschedRules } from '@/components/account/shared/useTempUnschedRules'
import TempUnschedulableRulesSection from '@/components/account/shared/TempUnschedulableRulesSection.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
import {
  cnSupportsNativeResponses,
  defaultCNAdaptiveBaseUrls,
  defaultCNBaseUrl,
  type CnAccountMode,
  type CnApiProtocol,
  type CnNativeApiProtocol,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import { getBrowserTimeZone } from '@/utils/format'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'
import { allSelectedGroupsEnableLongContextPricing } from '@/components/account/longContextBilling'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'
import { OPENAI_WS_MODE_OFF, type OpenAIWSMode } from '@/utils/openaiWsMode'
import {
  getPresetMappingsByPlatform,
  buildModelMappingObject,
  splitModelMappingObject
} from '@/composables/useModelWhitelist'

interface Props {
  show: boolean
  account: Account | null
  proxies: Proxy[]
  groups: AdminGroup[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const browserTimeZone = getBrowserTimeZone()

// Spark 影子账号(parent_account_id 非空):代理恒继承母账号,不可独立编辑(外审 B/P1),
// 故隐藏代理选择器。
const isSparkShadow = computed(() => props.account?.parent_account_id != null)

const hideAccountLongContextBilling = computed(() => {
  return allSelectedGroupsEnableLongContextPricing(form.group_ids, props.groups)
})

const handleOllamaCloudUsageUpdated = (state: OllamaCloudUsageState) => {
  if (props.account) emit('updated', { ...props.account, ollama_cloud_usage: state })
}

// Platform-specific hint for Base URL
const baseUrlHint = computed(() => {
  if (!props.account) return t('admin.accounts.baseUrlHint')
  if (props.account.platform === 'openai') return t('admin.accounts.openai.baseUrlHint')
  if (props.account.platform === 'gemini') return t('admin.accounts.gemini.baseUrlHint')
  if (props.account.platform === 'grok') return ''
  return t('admin.accounts.baseUrlHint')
})

const bedrockPresets = computed(() => getPresetMappingsByPlatform('bedrock'))

// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

// State
const submitting = ref(false)
const editBaseUrl = ref('https://api.anthropic.com')
const editApiKey = ref('')

// ── 国产供应商（Kimi / Zhipu / DeepSeek）account_mode / api_protocol 编辑 ──
// account_mode 决定额度/余额监控路径，api_protocol 决定转发端点与格式；
// 二者均可修正（早期创建的账号可能存错默认值），切换时重置 base_url 预置。
const isCNApiKeyAccount = computed(
  () =>
    props.account?.type === 'apikey' &&
    (props.account.platform === 'kimi' ||
      props.account.platform === 'zhipu' ||
      props.account.platform === 'deepseek')
)
// CnBaseUrlPresets 的 platform prop 是平台字面量联合类型，模板里不能写
// `as` 断言（其中的 `|` 会被 eslint 误判为 Vue2 filter 语法），经此 computed 传递。
const cnPresetPlatform = computed<'kimi' | 'zhipu' | 'deepseek'>(() => {
  const platform = props.account?.platform
  if (platform === 'kimi' || platform === 'zhipu' || platform === 'deepseek') {
    return platform
  }
  return 'kimi'
})
const editApiProtocol = ref<CnApiProtocol>('adaptive')
const editAccountMode = ref<CnAccountMode>('payg')
// 智谱团队版 Coding Plan：组织/项目 ID，写入 credentials 供额度探测切换团队端点
const editZhipuOrganization = ref('')
const editZhipuProject = ref('')
const editAdaptiveBaseUrls = ref<Record<CnNativeApiProtocol, string>>({
  chat_completions: '',
  anthropic: '',
  responses: ''
})
// 回填窗口标志：syncFormFromAccount 会同步改写 editAccountMode / editApiProtocol，
// 而 watcher（pre-flush）在同步代码执行完之后才触发——若不抑制，会把刚恢复的
// 存储版 base_url（可能是用户自定义/中转地址）覆盖为官方预设并在下次保存时持久化。
// nextTick 后解除，此后用户主动切换模式/协议仍正常联动重置。
const syncingForm = ref(false)
const cnAccountModeOptions = computed<Array<{ value: CnAccountMode; labelKey: 'payg' | 'coding' }>>(
  () => {
    // DeepSeek 无 coding 套餐（与创建弹窗一致），仅保留按量付费。
    if (props.account?.platform === 'deepseek') {
      return [{ value: 'payg', labelKey: 'payg' }]
    }
    return [
      { value: 'payg', labelKey: 'payg' },
      { value: 'coding', labelKey: 'coding' }
    ]
  }
)
const cnProtocolOptions = computed<Array<{ value: CnApiProtocol; labelKey: string }>>(() => {
  const opts: Array<{ value: CnApiProtocol; labelKey: string }> = [
    { value: 'adaptive', labelKey: 'adaptive' },
    { value: 'chat_completions', labelKey: 'chatCompletions' },
    { value: 'anthropic', labelKey: 'anthropic' }
  ]
  if (cnSupportsNativeResponses(props.account?.platform ?? '')) {
    opts.push({ value: 'responses', labelKey: 'responses' })
  }
  return opts
})
const editAdaptiveProtocolOptions = computed<Array<{ value: CnNativeApiProtocol; labelKey: string }>>(() => {
  const opts: Array<{ value: CnNativeApiProtocol; labelKey: string }> = [
    { value: 'chat_completions', labelKey: 'chatCompletions' },
    { value: 'anthropic', labelKey: 'anthropic' }
  ]
  if (cnSupportsNativeResponses(props.account?.platform ?? '')) opts.push({ value: 'responses', labelKey: 'responses' })
  return opts
})
watch(editApiProtocol, (protocol, previousProtocol) => {
  if (!isCNApiKeyAccount.value || syncingForm.value) return
  if (protocol === 'adaptive') {
    const defaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, editAccountMode.value)
    for (const item of editAdaptiveProtocolOptions.value) {
      if (!editAdaptiveBaseUrls.value[item.value]) editAdaptiveBaseUrls.value[item.value] = defaults[item.value]
    }
    if (previousProtocol !== 'adaptive' && editBaseUrl.value.trim()) {
      editAdaptiveBaseUrls.value[previousProtocol] = editBaseUrl.value.trim()
    }
    editBaseUrl.value = editAdaptiveBaseUrls.value.chat_completions
    return
  }
  if (previousProtocol === 'adaptive') {
    editBaseUrl.value = editAdaptiveBaseUrls.value[protocol] ||
      defaultCNBaseUrl(props.account!.platform, editAccountMode.value, protocol)
    return
  }
  editBaseUrl.value = defaultCNBaseUrl(props.account!.platform, editAccountMode.value, protocol)
})
watch(editAccountMode, (mode, previousMode) => {
  if (!isCNApiKeyAccount.value || syncingForm.value) return
  // deepseek 无 coding 套餐：防御性回退（UI 已隐藏该选项）。
  const effectiveMode = props.account!.platform === 'deepseek' && mode === 'coding' ? 'payg' : mode
  if (effectiveMode !== mode) {
    editAccountMode.value = effectiveMode
    return
  }
  if (editApiProtocol.value === 'adaptive') {
    const previousDefaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, previousMode)
    const nextDefaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, mode)
    for (const item of editAdaptiveProtocolOptions.value) {
      if (!editAdaptiveBaseUrls.value[item.value] || editAdaptiveBaseUrls.value[item.value] === previousDefaults[item.value]) {
        editAdaptiveBaseUrls.value[item.value] = nextDefaults[item.value]
      }
    }
    editBaseUrl.value = editAdaptiveBaseUrls.value.chat_completions
    return
  }
  editBaseUrl.value = defaultCNBaseUrl(props.account!.platform, mode, editApiProtocol.value)
})
const cnProtocolDescKey = computed(
  () => cnProtocolOptions.value.find(o => o.value === editApiProtocol.value)?.labelKey ?? 'chatCompletions'
)
// 点击预设端点：回填 base url 与对应模式/协议。
function onCnPresetSelect(preset: { mode: CnAccountMode; protocol: CnApiProtocol; url: string }) {
  editAccountMode.value = preset.mode
  editApiProtocol.value = preset.protocol
  editBaseUrl.value = preset.url
}
// Bedrock credentials
const editBedrockAccessKeyId = ref('')
const editBedrockSecretAccessKey = ref('')
const editBedrockSessionToken = ref('')
const editBedrockRegion = ref('')
const editBedrockForceGlobal = ref(false)
const editBedrockApiKeyValue = ref('')
const editVertexProjectId = ref('')
const editVertexClientEmail = ref('')
const editVertexLocation = ref('us-central1')
const isBedrockAPIKeyMode = computed(() =>
  props.account?.type === 'bedrock' &&
  (props.account?.credentials as Record<string, unknown>)?.auth_mode === 'apikey'
)
const modelMappings = ref<ModelMapping[]>([])
const openAICompactModelMappings = ref<ModelMapping[]>([])
const modelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const allowedModels = ref<string[]>([])
const KIRO_PROFILE_CHOICE_KEEP = '__keep__'
const KIRO_PROFILE_CHOICE_AUTO = '__auto__'
const discoveringKiroProfiles = ref(false)
const discoveredKiroProfiles = ref<KiroDiscoveredProfile[]>([])
const selectedKiroProfileArnChoice = ref<string>(KIRO_PROFILE_CHOICE_KEEP)
const currentKiroProfileArn = computed(() => {
  if (props.account?.platform !== 'kiro') return ''
  const raw = (props.account.credentials as Record<string, unknown> | undefined)?.profile_arn
  return typeof raw === 'string' ? raw : ''
})
const effectiveKiroProfileChoice = computed(() => {
  if (selectedKiroProfileArnChoice.value === KIRO_PROFILE_CHOICE_KEEP) {
    return currentKiroProfileArn.value
  }
  if (selectedKiroProfileArnChoice.value === KIRO_PROFILE_CHOICE_AUTO) {
    return ''
  }
  return selectedKiroProfileArnChoice.value
})
const kiroProfileStatusLabel = computed(() => {
  return effectiveKiroProfileChoice.value
    ? t('admin.accounts.kiro.profileStateManual')
    : t('admin.accounts.kiro.profileStateAuto')
})
const kiroProfileStatusBadgeClass = computed(() => {
  return effectiveKiroProfileChoice.value
    ? 'bg-[color-mix(in_oklch,var(--accent)_16%,transparent)] text-accent'
    : 'bg-surface-3 text-muted'
})
const kiroProfilePendingHint = computed(() => {
  if (selectedKiroProfileArnChoice.value === KIRO_PROFILE_CHOICE_KEEP) return ''
  return selectedKiroProfileArnChoice.value === KIRO_PROFILE_CHOICE_AUTO
    ? t('admin.accounts.kiro.profileStatePendingAuto')
    : t('admin.accounts.kiro.profileStatePendingManual')
})
const kiroProfileSelectOptions = computed(() => {
  const seen = new Set<string>()
  const options: Array<{ arn: string; name: string }> = []
  for (const profile of discoveredKiroProfiles.value) {
    const arn = typeof profile?.profileArn === 'string' && profile.profileArn
      ? profile.profileArn
      : typeof profile?.arn === 'string'
        ? profile.arn
        : ''
    if (!arn || seen.has(arn)) continue
    seen.add(arn)
    const label = profile.profileName || profile.profile_name || arn.split('/').pop() || arn
    options.push({ arn, name: label })
  }
  if (currentKiroProfileArn.value && !seen.has(currentKiroProfileArn.value)) {
    options.unshift({ arn: currentKiroProfileArn.value, name: currentKiroProfileArn.value })
  }
  return options
})
const DEFAULT_POOL_MODE_RETRY_COUNT = 3
const MAX_POOL_MODE_RETRY_COUNT = 10
const GROK_CLIENT_TOOL_CACHE_EXTRA_KEY = 'grok_client_tool_cache_enabled'
const poolModeEnabled = ref(false)
const poolModeRetryCount = ref(DEFAULT_POOL_MODE_RETRY_COUNT)
const poolModeRetryStatusCodesInput = ref('')

function parsePoolModeRetryStatusCodes(input: string): number[] {
  if (!input || !input.trim()) return []
  const seen = new Set<number>()
  const out: number[] = []
  for (const token of input.split(/[,\s]+/)) {
    const trimmed = token.trim()
    if (!trimmed) continue
    const n = Number(trimmed)
    if (!Number.isFinite(n) || !Number.isInteger(n)) continue
    if (n < 100 || n > 599) continue
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out.sort((a, b) => a - b)
}

function formatPoolModeRetryStatusCodes(value: unknown): string {
  if (!Array.isArray(value)) return ''
  const out: number[] = []
  const seen = new Set<number>()
  for (const v of value) {
    const n = typeof v === 'string' ? Number(v.trim()) : Number(v)
    if (!Number.isFinite(n) || !Number.isInteger(n)) continue
    if (n < 100 || n > 599) continue
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out.sort((a, b) => a - b).join(', ')
}
const customErrorCodesEnabled = ref(false)
const selectedErrorCodes = ref<number[]>([])
const headerOverrideEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

// Grok OAuth 自定义上游地址（仅转发端点；OAuth 授权/令牌刷新不受影响）
const grokOAuthCustomBaseUrlEnabled = ref(false)
const grokOAuthBaseUrl = ref('')
// Grok Free OAuth accounts use client-tool prompt caching by default. Keep an
// explicit false in the account extra as the opt-out signal.
const grokClientToolCacheEnabled = ref(true)

const interceptWarmupRequests = ref(false)
const autoPauseOnExpired = ref(false)
const autoPause5hThreshold = ref<number | null>(null)
const autoPause7dThreshold = ref<number | null>(null)
const autoPause5hDisabled = ref(false)
const autoPause7dDisabled = ref(false)
const autoResetCreditEnabled = ref(false)
const autoResetCredit5hThreshold = ref(100)
const autoResetCredit7dThreshold = ref(100)
const upstreamBillingAutoProbeEnabled = ref(false)
const upstreamBillingRateSyncEnabled = ref(false)
const mixedScheduling = ref(false) // For antigravity accounts: enable mixed scheduling
const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityProjectId = ref('')
const antigravityModelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const antigravityWhitelistModels = ref<string[]>([])
const antigravityModelMappings = ref<ModelMapping[]>([])
const {
  tempUnschedEnabled,
  tempUnschedRules,
  getTempUnschedRuleKey,
  tempUnschedPresets,
  addTempUnschedRule,
  removeTempUnschedRule,
  moveTempUnschedRule,
  applyTempUnschedConfig
} = useTempUnschedRules('edit-temp-unsched-rule')
const accountSchedulingThresholdOverrideEnabled = ref(false)
const accountSchedulingThresholdOverrideValue = ref(100)
const ACCOUNT_SCHEDULING_THRESHOLD_CREDENTIAL_KEY = 'account_scheduling_threshold'
const supportsAccountSchedulingThresholdOverride = computed(() =>
  supportsAccountSchedulingThresholdOverridePlatform(props.account?.platform)
)
const getModelMappingKey = createStableObjectKeyResolver<ModelMapping>('edit-model-mapping')

// Quota control state (Anthropic OAuth/SetupToken only)
const windowCostEnabled = ref(false)
const windowCostLimit = ref<number | null>(null)
const windowCostStickyReserve = ref<number | null>(null)
const sessionLimitEnabled = ref(false)
const maxSessions = ref<number | null>(null)
const sessionIdleTimeout = ref<number | null>(null)
const rpmLimitEnabled = ref(false)
const baseRpm = ref<number | null>(null)
const rpmStrategy = ref<'tiered' | 'sticky_exempt'>('tiered')
const rpmStickyBuffer = ref<number | null>(null)
const userMsgQueueMode = ref('')
const {
  tlsFingerprintEnabled,
  tlsFingerprintProfileId,
  tlsFingerprintProfiles,
  tlsFingerprintRouters,
  tlsFingerprintRouterId,
  tlsFingerprintDefaultOS,
  tlsFingerprintBindingRows,
  deviceLearningEnabled,
  deviceTLSProfileId,
  deviceTLSCatalogOptions,
  deviceTLSProfileSelection,
  formatDeviceTLSProfileLabel,
  readDeviceTLSProfileId,
  supportsTLSFingerprint,
  applyTLSFingerprintToExtra,
  tlsFingerprintRowsFromBindings,
  loadTLSProfiles,
  loadCompleteDeviceTLSProfiles
} = useEditAccountTlsFingerprint({ t })
const sessionIdMaskingEnabled = ref(false)
const cacheTTLOverrideEnabled = ref(false)
const cacheTTLOverrideTarget = ref<string>('5m')
const customBaseUrlEnabled = ref(false)
const customBaseUrl = ref('')

// OpenAI 自动透传开关（OAuth/API Key）
const openaiPassthroughEnabled = ref(false)
// OpenAI Codex namespace 工具摊平兼容开关（仅 OAuth），缺省关闭即原样保留
const openaiFlattenNamespacesEnabled = ref(false)
const openAILongContextBillingEnabled = ref(false)
// OpenAI 订阅档位（Plus/Pro/Free）手动覆盖值,存于 credentials.plan_type;'' 表示清空/自动识别
const editPlanType = ref<string>('')
const openAICompactMode = ref<OpenAICompactMode>('auto')
const openAIResponsesMode = ref<OpenAIResponsesMode>('auto')
const openAIEndpointCapabilities = ref<OpenAIEndpointCapability[]>(['chat_completions', 'embeddings'])
const openaiOAuthResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const openaiAPIKeyResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const codexCLIOnlyEnabled = ref(false)
const codexCLIOnlyAppServerEnabled = ref(false)
const codexFingerprintMode = ref<CodexFingerprintMode>('off')
const codexImageToolMode = ref<CodexImageToolMode>('inherit')
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicPassthroughEnabled = ref(false)
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const webSearchEmulationMode = ref('default')
const webSearchGlobalEnabled = ref(false)
const {
  globalEnabled: quotaNotifyGlobalEnabled,
  state: quotaNotifyState,
  loadGlobalState: loadQuotaNotifyGlobal,
  loadFromExtra: loadQuotaNotifyFromExtra,
  writeToExtra: writeQuotaNotifyToExtra,
  reset: resetQuotaNotify,
} = useQuotaNotifyState()

// Load global feature states once
adminAPI.settings.getWebSearchEmulationConfig().then(cfg => {
  webSearchGlobalEnabled.value = cfg?.enabled === true && (cfg?.providers?.length ?? 0) > 0
}).catch(() => { webSearchGlobalEnabled.value = false })

loadQuotaNotifyGlobal()
const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)
const editDailyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editDailyResetHour = ref<number | null>(null)
const editWeeklyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editWeeklyResetDay = ref<number | null>(null)
const editWeeklyResetHour = ref<number | null>(null)
const editResetTimezone = ref<string | null>(null)
const {
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
  openAIEndpointCapabilityOptions,
  openAITextGenerationCapabilityEnabled,
  readOpenAIEndpointCapabilities,
  toggleOpenAIEndpointCapability,
  applyOpenAIEndpointCapabilities,
  normalizeOpenAIResponsesMode,
  isOpenAIModelRestrictionDisabled,
  openAIResponsesStatusKey,
  openAICompactStatusKey
} = useEditAccountOpenAIOptions({
  t,
  props,
  codexImageToolMode,
  openaiAPIKeyResponsesWebSocketV2Mode,
  openaiOAuthResponsesWebSocketV2Mode,
  openAIEndpointCapabilities,
  openAIResponsesMode,
  editPlanType,
  openaiPassthroughEnabled
})


// Computed: current preset mappings based on platform
const presetMappings = computed(() => getPresetMappingsByPlatform(props.account?.platform || 'anthropic'))

// Computed: default base URL based on platform
const defaultBaseUrl = computed(() => {
  if (props.account?.platform === 'openai') return 'https://api.openai.com'
  if (props.account?.platform === 'gemini') return 'https://generativelanguage.googleapis.com'
  if (props.account?.platform === 'grok') return 'https://api.x.ai/v1'
  // CN 供应商：按当前模式/协议回落到官方预设（清空输入框提交时使用），
  // 不能落到 anthropic 默认值（会被当 CC base 拼出错误端点）。
  if (
    props.account?.platform === 'kimi' ||
    props.account?.platform === 'zhipu' ||
    props.account?.platform === 'deepseek'
  ) {
    return defaultCNBaseUrl(props.account.platform, editAccountMode.value, editApiProtocol.value)
  }
  return 'https://api.anthropic.com'
})

const form = reactive({
  name: '',
  notes: '',
  proxy_id: null as number | null,
  concurrency: 1,
  load_factor: null as number | null,
  priority: 1,
  rate_multiplier: 1,
  status: 'active' as 'active' | 'inactive' | 'error',
  group_ids: [] as number[],
  expires_at: null as number | null
})

const {
  showMixedChannelWarning,
  mixedChannelWarningDetails,
  mixedChannelWarningRawMessage,
  mixedChannelWarningAction,
  antigravityMixedChannelConfirmed,
  handleClose,
  ensureAntigravityMixedChannelConfirmed,
  submitUpdateAccount,
  handleMixedChannelConfirm,
  handleMixedChannelCancel,
  mixedChannelWarningMessageText
} = useEditAccountMixedChannel({ props, form, appStore, t, emit, submitting })

const handleUpstreamBillingRateSyncChange = (enabled: boolean) => {
  upstreamBillingRateSyncEnabled.value = enabled
  if (enabled) {
    upstreamBillingAutoProbeEnabled.value = true
  }
}

const handleUpstreamBillingAutoProbeChange = (enabled: boolean) => {
  upstreamBillingAutoProbeEnabled.value = enabled
  if (!enabled) {
    upstreamBillingRateSyncEnabled.value = false
  }
}

const statusOptions = computed(() => {
  const options = [
    { value: 'active', label: t('common.active') },
    { value: 'inactive', label: t('common.inactive') }
  ]
  if (form.status === 'error') {
    options.push({ value: 'error', label: t('admin.accounts.status.error') })
  }
  return options
})

// Watchers
const normalizePoolModeRetryCount = (value: number) => {
  if (!Number.isFinite(value)) {
    return DEFAULT_POOL_MODE_RETRY_COUNT
  }
  const normalized = Math.trunc(value)
  if (normalized < 0) {
    return 0
  }
  if (normalized > MAX_POOL_MODE_RETRY_COUNT) {
    return MAX_POOL_MODE_RETRY_COUNT
  }
  return normalized
}

const loadModelRestrictionFromMapping = (rawMapping?: Record<string, unknown>) => {
  const parsed = splitModelMappingObject(rawMapping)
  allowedModels.value = parsed.allowedModels
  modelMappings.value = parsed.modelMappings
  modelRestrictionMode.value =
    parsed.modelMappings.length > 0 && parsed.allowedModels.length === 0
      ? 'mapping'
      : 'whitelist'
}

const buildModelRestrictionMapping = () =>
  buildModelMappingObject('combined', allowedModels.value, modelMappings.value)

const applyOpenAIModelMappingCredentials = (credentials: Record<string, unknown>) => {
  const shouldApplyModelMapping = !openaiPassthroughEnabled.value

  if (shouldApplyModelMapping) {
    const modelMapping = buildModelRestrictionMapping()
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    } else {
      delete credentials.model_mapping
    }
  } else if (!credentials.model_mapping) {
    delete credentials.model_mapping
  }

  const compactModelMapping = buildModelMappingObject('mapping', [], openAICompactModelMappings.value)
  if (compactModelMapping) {
    credentials.compact_model_mapping = compactModelMapping
  } else {
    delete credentials.compact_model_mapping
  }
}

const {
  syncFormFromAccount
} = useEditAccountSync({
  form, syncingForm, nextTick,
  showMixedChannelWarning, mixedChannelWarningDetails, mixedChannelWarningRawMessage,
  mixedChannelWarningAction, antigravityMixedChannelConfirmed,
  interceptWarmupRequests, autoPauseOnExpired, editVertexProjectId, editVertexClientEmail,
  editVertexLocation, antigravityProjectId,
  mixedScheduling, allowOverages, deviceLearningEnabled, deviceTLSProfileId, readDeviceTLSProfileId,
  autoPause5hThreshold, autoPause7dThreshold, autoPause5hDisabled, autoPause7dDisabled,
  autoResetCreditEnabled, autoResetCredit5hThreshold, autoResetCredit7dThreshold,
  upstreamBillingAutoProbeEnabled, upstreamBillingRateSyncEnabled,
  openaiPassthroughEnabled, openaiFlattenNamespacesEnabled, openAILongContextBillingEnabled,
  editPlanType, openAICompactMode, openAIResponsesMode, openAIEndpointCapabilities,
  openAICompactModelMappings, openaiOAuthResponsesWebSocketV2Mode, openaiAPIKeyResponsesWebSocketV2Mode,
  codexCLIOnlyEnabled, codexCLIOnlyAppServerEnabled, codexFingerprintMode, codexImageToolMode,
  anthropicPassthroughEnabled, anthropicAPIKeyAuthScheme, webSearchEmulationMode,
  openAITextGenerationCapabilityEnabled, normalizeOpenAIResponsesMode, readOpenAIEndpointCapabilities,
  editQuotaLimit, editQuotaDailyLimit, editQuotaWeeklyLimit, editDailyResetMode, editDailyResetHour,
  editWeeklyResetMode, editWeeklyResetDay, editWeeklyResetHour, editResetTimezone,
  loadQuotaNotifyFromExtra, resetQuotaNotify,
  antigravityModelRestrictionMode, antigravityWhitelistModels, antigravityModelMappings,
  loadQuotaControlSettings, loadTempUnschedRules, loadAccountSchedulingThresholdOverride,
  headerOverrideEnabled, headerOverrideRows,
  grokOAuthCustomBaseUrlEnabled, grokOAuthBaseUrl, grokClientToolCacheEnabled, GROK_CLIENT_TOOL_CACHE_EXTRA_KEY,
  loadModelRestrictionFromMapping, discoveredKiroProfiles, selectedKiroProfileArnChoice, KIRO_PROFILE_CHOICE_KEEP,
  editBaseUrl, poolModeEnabled, poolModeRetryCount, poolModeRetryStatusCodesInput,
  normalizePoolModeRetryCount, formatPoolModeRetryStatusCodes, DEFAULT_POOL_MODE_RETRY_COUNT,
  customErrorCodesEnabled, selectedErrorCodes,
  editAccountMode, editApiProtocol, editAdaptiveBaseUrls, editZhipuOrganization, editZhipuProject, isCNApiKeyAccount,
  editBedrockRegion, editBedrockForceGlobal, editBedrockApiKeyValue, editBedrockAccessKeyId,
  editBedrockSecretAccessKey, editBedrockSessionToken,
  modelRestrictionMode, modelMappings, allowedModels,
  editApiKey
})

watch(
  [() => props.show, () => props.account, () => props.account?.platform],
  ([show, newAccount, platform], [wasShow, previousAccount, previousPlatform]) => {
    if (!show || !newAccount) {
      deviceTLSCatalogOptions.value = []
      return
    }
    if (!wasShow || newAccount !== previousAccount || platform !== previousPlatform) {
      syncFormFromAccount(newAccount)
      loadTLSProfiles()
      loadCompleteDeviceTLSProfiles(platform)
    }
  },
  { immediate: true }
)

const applyKiroModelRestrictionPatch = (
  newCredentials: Record<string, unknown>,
  currentCredentials: Record<string, unknown>
) => {
  const modelMapping = buildModelMappingObject(
    modelRestrictionMode.value,
    allowedModels.value,
    modelMappings.value,
    'kiro'
  )
  if (modelMapping) {
    newCredentials.model_mapping = modelMapping
  } else if (
    currentCredentials.model_mapping &&
    Object.keys(currentCredentials.model_mapping as Record<string, unknown>).length > 0
  ) {
    newCredentials.model_mapping = {}
  }
}

const discoverKiroProfilesForEdit = async () => {
  if (!props.account || props.account.platform !== 'kiro' || props.account.type !== 'oauth') return
  discoveringKiroProfiles.value = true
  try {
    discoveredKiroProfiles.value = await adminAPI.accounts.getKiroProfiles(props.account.id)
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.kiro.discoverProfilesFailed'))
  } finally {
    discoveringKiroProfiles.value = false
  }
}


function supportsAccountSchedulingThresholdOverridePlatform(platform: Account['platform'] | undefined) {
  return platform === 'openai' || platform === 'anthropic' || platform === 'grok'
}

function normalizeAccountSchedulingThresholdOverride(value: unknown): number | null {
  if (value === null || value === undefined || value === '') {
    return null
  }
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) {
    return null
  }
  const integer = Math.trunc(numeric)
  if (integer < 1 || integer > 100) {
    return null
  }
  return integer
}

function clampAccountSchedulingThresholdOverride(value: unknown): number {
  return Math.min(100, Math.max(1, Math.trunc(Number(value) || 100)))
}

function loadAccountSchedulingThresholdOverride(
  platform: Account['platform'] | undefined,
  credentials: Record<string, unknown> | undefined
) {
  if (!supportsAccountSchedulingThresholdOverridePlatform(platform)) {
    accountSchedulingThresholdOverrideEnabled.value = false
    accountSchedulingThresholdOverrideValue.value = 100
    return
  }
  const value = normalizeAccountSchedulingThresholdOverride(
    credentials?.[ACCOUNT_SCHEDULING_THRESHOLD_CREDENTIAL_KEY]
  )
  accountSchedulingThresholdOverrideEnabled.value = value !== null
  accountSchedulingThresholdOverrideValue.value = value ?? 100
}

const applyAccountSchedulingThresholdOverridePatch = (
  credentials: Record<string, unknown>,
  currentCredentials: Record<string, unknown>,
  platform: Account['platform'] | undefined = props.account?.platform
) => {
  if (!supportsAccountSchedulingThresholdOverridePlatform(platform)) {
    return
  }
  const current = normalizeAccountSchedulingThresholdOverride(
    currentCredentials[ACCOUNT_SCHEDULING_THRESHOLD_CREDENTIAL_KEY]
  )
  if (!accountSchedulingThresholdOverrideEnabled.value) {
    if (current !== null) {
      credentials[ACCOUNT_SCHEDULING_THRESHOLD_CREDENTIAL_KEY] = null
    }
    return
  }
  const next = clampAccountSchedulingThresholdOverride(accountSchedulingThresholdOverrideValue.value)
  if (current !== next) {
    credentials[ACCOUNT_SCHEDULING_THRESHOLD_CREDENTIAL_KEY] = next
  }
}

function loadTempUnschedRules(credentials?: Record<string, unknown>) {
  tempUnschedEnabled.value = credentials?.temp_unschedulable_enabled === true
  const rawRules = credentials?.temp_unschedulable_rules
  if (!Array.isArray(rawRules)) {
    tempUnschedRules.value = []
    return
  }

  tempUnschedRules.value = rawRules.map((rule) => {
    const entry = rule as Record<string, unknown>
    return {
      error_code: toPositiveNumber(entry.error_code),
      keywords: formatTempUnschedKeywords(entry.keywords),
      duration_minutes: toPositiveNumber(entry.duration_minutes),
      description: typeof entry.description === 'string' ? entry.description : ''
    }
  })
}

function manualRPMStickyBufferFromExtra(extra: Account['extra'] | undefined): number | null {
  if (!extra) return null
  const raw = extra.rpm_sticky_buffer
  const val = typeof raw === 'number' ? raw : Number(raw)
  if (Number.isInteger(val) && val >= 1 && val <= 10000) {
    return val
  }
  return null
}

// Load quota control settings from account (Anthropic OAuth/SetupToken only)
function loadQuotaControlSettings(account: Account) {
  // Reset all quota control state first
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

  if (supportsTLSFingerprint(account.platform)) {
    if (account.enable_tls_fingerprint === true) {
      tlsFingerprintEnabled.value = true
    }
    tlsFingerprintProfileId.value = account.tls_fingerprint_profile_id ?? null
    tlsFingerprintRouterId.value = account.tls_fingerprint_router_id ?? null
    tlsFingerprintDefaultOS.value = account.tls_fingerprint_default_os ?? ''
    tlsFingerprintRowsFromBindings(account.tls_fingerprint_bindings)
  }

  // Remaining quota control settings only apply to Anthropic accounts
  if (account.platform !== 'anthropic') {
    return
  }

  // Window cost / session limit only apply to Anthropic OAuth/SetupToken accounts
  if (account.type !== 'oauth' && account.type !== 'setup-token') {
    return
  }

  // Load from extra field (via backend DTO fields)
  if (account.window_cost_limit != null && account.window_cost_limit > 0) {
    windowCostEnabled.value = true
    windowCostLimit.value = account.window_cost_limit
    windowCostStickyReserve.value = account.window_cost_sticky_reserve ?? 10
  }

  if (account.max_sessions != null && account.max_sessions > 0) {
    sessionLimitEnabled.value = true
    maxSessions.value = account.max_sessions
    sessionIdleTimeout.value = account.session_idle_timeout_minutes ?? 5
  }

  // RPM limit
  if (account.base_rpm != null && account.base_rpm > 0) {
    rpmLimitEnabled.value = true
    baseRpm.value = account.base_rpm
    rpmStrategy.value = (account.rpm_strategy as 'tiered' | 'sticky_exempt') || 'tiered'
    rpmStickyBuffer.value = manualRPMStickyBufferFromExtra(account.extra)
  }

  // UMQ mode（独立于 RPM 加载，防止编辑无 RPM 账号时丢失已有配置）
  userMsgQueueMode.value = account.user_msg_queue_mode ?? ''

  // Load session ID masking setting
  if (account.session_id_masking_enabled === true) {
    sessionIdMaskingEnabled.value = true
  }

  // Load cache TTL override setting
  if (account.cache_ttl_override_enabled === true) {
    cacheTTLOverrideEnabled.value = true
    cacheTTLOverrideTarget.value = account.cache_ttl_override_target || '5m'
  }

  // Load custom base URL setting
  if (account.custom_base_url_enabled === true) {
    customBaseUrlEnabled.value = true
    customBaseUrl.value = account.custom_base_url || ''
  }
}

function formatTempUnschedKeywords(value: unknown) {
  if (Array.isArray(value)) {
    return value
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter((item) => item.length > 0)
      .join(', ')
  }
  if (typeof value === 'string') {
    return value
  }
  return ''
}

function toPositiveNumber(value: unknown) {
  const num = Number(value)
  if (!Number.isFinite(num) || num <= 0) {
    return null
  }
  return Math.trunc(num)
}

// Methods
const {
  handleSubmit
} = useEditAccountSubmit({
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
})

</script>
