<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.createAccount')"
    width="wide"
    @close="handleClose"
  >
    <CreateAccountStepIndicator :is-o-auth-flow="isOAuthFlow" :step="step" :oauth-step-title="oauthStepTitle" />

    <!-- Step 1: Basic Info -->
    <form
      v-if="step === 1"
      id="create-account-form"
      @submit.prevent="handleSubmit"
      class="space-y-5"
    >
      <div>
        <label class="input-label">{{ t('admin.accounts.accountName') }}</label>
        <input
          v-model="form.name"
          type="text"
          :required="!isOAuthFlow"
          class="input"
          :placeholder="accountNamePlaceholder"
          data-tour="account-form-name"
        />
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

      <PlatformSelector
        v-model:platform="form.platform"
        @select-cn-platform="selectCNPlatform"
      />

      <!-- Account Type Selection (Anthropic) -->
      <!-- Account Type Selection (Anthropic) -->
      <AnthropicPanel v-if="form.platform === 'anthropic'" v-model:account-category="accountCategory" />

      <!-- Account Type Selection (OpenAI) -->
      <OpenAIPanel v-if="form.platform === 'openai'" v-model:account-category="accountCategory" />

      <!-- Account Type Selection (Grok) -->
      <GrokPanel v-if="form.platform === 'grok'" v-model:account-category="accountCategory" />

      <!-- Account Type Selection (Kiro - OAuth or API Key) -->
      <KiroPanel
        v-if="form.platform === 'kiro'"
        v-model:kiro-account-type="kiroAccountType"
        v-model:api-key-value="kiroAPIKeyValue"
        v-model:region="kiroRegion"
        v-model:auth-region="kiroAuthRegion"
        v-model:api-region="kiroAPIRegion"
        v-model:profile-arn="kiroProfileARN"
        v-model:machine-id="kiroMachineID"
      />

      <div v-if="form.platform === 'kiro'" class="border-t border-line pt-4">
        <ModelRestrictionEditor
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          platform="kiro"
          preset-button-class="rounded-lg px-3 py-2 text-sm font-medium transition-colors"
          :presets="presetMappings"
        />
      </div>

      <CnAccountOptionsSection
        :platform="form.platform"
        :is-cn-platform="isCNPlatform"
        :cn-accent-active-class="cnAccentActiveClass"
        :cn-accent-icon-class="cnAccentIconClass"
        :cn-protocol-options="cnProtocolOptions"
        v-model:account-mode="accountMode"
        v-model:api-protocol="apiProtocol"
        v-model:zhipu-organization="zhipuOrganization"
        v-model:zhipu-project="zhipuProject"
      />

      <!-- Account Type Selection (Gemini) -->
      <GeminiPanel
        v-if="form.platform === 'gemini'"
        v-model:account-category="accountCategory"
        v-model:gemini-oauth-type="geminiOAuthType"
        v-model:show-advanced-oauth="showAdvancedOAuth"
        v-model:show-help-dialog="showGeminiHelpDialog"
        v-model:tier-google-one="geminiTierGoogleOne"
        v-model:tier-gcp="geminiTierGcp"
        v-model:tier-ai-studio="geminiTierAIStudio"
        :gemini-a-i-studio-o-auth-enabled="geminiAIStudioOAuthEnabled"
        :gemini-help-links="geminiHelpLinks"
      />

      <!-- Account Type Selection (Antigravity - OAuth or Upstream) -->
      <!-- Account Type Selection (Antigravity - OAuth or Upstream) -->
      <AntigravityPanel
        v-if="form.platform === 'antigravity'"
        v-model:account-type="antigravityAccountType"
        v-model:project-id="antigravityProjectId"
        v-model:upstream-base-url="upstreamBaseUrl"
        v-model:upstream-api-key="upstreamApiKey"
        v-model:upstream-billing-auto-probe-enabled="upstreamBillingAutoProbeEnabled"
        v-model:model-mappings="antigravityModelMappings"
      />

      <!-- Vertex Service Account -->
      <VertexServiceAccountPanel
        v-if="(form.platform === 'gemini' || form.platform === 'anthropic') && accountCategory === 'service_account'"
        v-model:location="vertexLocation"
        :project-id="vertexProjectId"
        :client-email="vertexClientEmail"
        @file-text-received="applyVertexServiceAccountJson"
      />

      <AnthropicAddMethodSection
        :show="form.platform === 'anthropic' && isOAuthFlow"
        v-model:add-method="addMethod"
      />

      <!-- API Key input (only for apikey type, excluding Antigravity which has its own fields) -->
      <CreateApiKeySection
        v-if="form.type === 'apikey' && form.platform !== 'antigravity' && form.platform !== 'kiro'"
        :platform="form.platform"
        :is-cn-platform="isCNPlatform"
        :api-protocol="apiProtocol"
        :account-mode="accountMode"
        :cn-preset-platform="cnPresetPlatform"
        :cn-adaptive-protocol-options="cnAdaptiveProtocolOptions"
        :api-key-base-url-placeholder="apiKeyBaseUrlPlaceholder"
        :base-url-hint="baseUrlHint"
        :api-key-hint="apiKeyHint"
        :api-key-value-placeholder="apiKeyValuePlaceholder"
        :sync-preview-credentials="syncPreviewCredentials"
        :is-open-ai-model-restriction-disabled="isOpenAIModelRestrictionDisabled"
        :preset-mappings="presetMappings"
        v-model:api-key-base-url="apiKeyBaseUrl"
        v-model:api-key-value="apiKeyValue"
        v-model:adaptive-base-urls="adaptiveBaseUrls"
        v-model:upstream-billing-auto-probe-enabled="upstreamBillingAutoProbeEnabled"
        v-model:gemini-tier-ai-studio="geminiTierAIStudio"
        v-model:model-restriction-mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        v-model:pool-mode-enabled="poolModeEnabled"
        v-model:pool-mode-retry-count="poolModeRetryCount"
        v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
        v-model:custom-error-codes-enabled="customErrorCodesEnabled"
        v-model:selected-error-codes="selectedErrorCodes"
        v-model:custom-error-code-input="customErrorCodeInput"
        v-model:header-override-enabled="headerOverrideEnabled"
        v-model:header-override-rows="headerOverrideRows"
        @cn-preset-select="onCnPresetSelect"
        @upstream-synced="upstreamModelsPreviewed = true"
      />

      <!-- Bedrock credentials (only for Anthropic Bedrock type) -->
      <CreateBedrockCredentialsSection
        v-if="form.platform === 'anthropic' && accountCategory === 'bedrock'"
        v-model:bedrock-auth-mode="bedrockAuthMode"
        v-model:bedrock-access-key-id="bedrockAccessKeyId"
        v-model:bedrock-secret-access-key="bedrockSecretAccessKey"
        v-model:bedrock-session-token="bedrockSessionToken"
        v-model:bedrock-api-key-value="bedrockApiKeyValue"
        v-model:bedrock-region="bedrockRegion"
        v-model:bedrock-force-global="bedrockForceGlobal"
        v-model:model-restriction-mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        v-model:pool-mode-enabled="poolModeEnabled"
        v-model:pool-mode-retry-count="poolModeRetryCount"
        v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
        :sync-preview-credentials="syncPreviewCredentials"
        :bedrock-presets="bedrockPresets"
        @upstream-synced="upstreamModelsPreviewed = true"
      />

      <!-- 配额控制 (apikey/bedrock only；Anthropic 使用专属提示文案) -->
      <QuotaLimitCardSection
        v-if="form.type === 'apikey' || form.type === 'bedrock'"
        :hint="form.platform === 'anthropic' ? t('admin.accounts.quotaControl.hint') : t('admin.accounts.quotaLimitHint')"
        :quotaNotifyGlobalEnabled="quotaNotifyGlobalEnabled"
        v-model:editQuotaLimit="editQuotaLimit"
        v-model:editQuotaDailyLimit="editQuotaDailyLimit"
        v-model:editQuotaWeeklyLimit="editQuotaWeeklyLimit"
        v-model:editDailyResetMode="editDailyResetMode"
        v-model:editDailyResetHour="editDailyResetHour"
        v-model:editWeeklyResetMode="editWeeklyResetMode"
        v-model:editWeeklyResetDay="editWeeklyResetDay"
        v-model:editWeeklyResetHour="editWeeklyResetHour"
        v-model:editResetTimezone="editResetTimezone"
        v-model:quotaNotifyDailyEnabled="quotaNotifyState.daily.enabled"
        v-model:quotaNotifyDailyThreshold="quotaNotifyState.daily.threshold"
        v-model:quotaNotifyDailyThresholdType="quotaNotifyState.daily.thresholdType"
        v-model:quotaNotifyWeeklyEnabled="quotaNotifyState.weekly.enabled"
        v-model:quotaNotifyWeeklyThreshold="quotaNotifyState.weekly.threshold"
        v-model:quotaNotifyWeeklyThresholdType="quotaNotifyState.weekly.thresholdType"
        v-model:quotaNotifyTotalEnabled="quotaNotifyState.total.enabled"
        v-model:quotaNotifyTotalThreshold="quotaNotifyState.total.threshold"
        v-model:quotaNotifyTotalThresholdType="quotaNotifyState.total.thresholdType"
      />

      <!-- Grok OAuth Custom Upstream URL (仅改写转发端点，OAuth 授权/刷新不受影响) -->
      <GrokCustomBaseUrlSection
        v-if="form.platform === 'grok' && isOAuthFlow"
        v-model:grokOAuthCustomBaseUrlEnabled="grokOAuthCustomBaseUrlEnabled"
        v-model:grokOAuthBaseUrl="grokOAuthBaseUrl"
      />

      <!-- Grok OAuth Header Override (OAuth 类型没有 apikey 容器，需要独立区域) -->
      <HeaderOverrideSection
        v-if="form.platform === 'grok' && isOAuthFlow"
        v-model:headerOverrideEnabled="headerOverrideEnabled"
        v-model:headerOverrideRows="headerOverrideRows"
      />

      <!-- OpenAI OAuth Model Mapping (OAuth 类型没有 apikey 容器，需要独立的模型映射区域) -->
      <div
        v-if="(form.platform === 'openai' || form.platform === 'grok') && isOAuthFlow"
        class="border-t border-line pt-4"
      >
        <ModelRestrictionEditor
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          :platform="form.platform"
          :sync-credentials="syncPreviewCredentials"
          :disabled-by-passthrough="isOpenAIModelRestrictionDisabled"
          :presets="presetMappings"
          @upstream-synced="upstreamModelsPreviewed = true"
        />
      </div>

      <TempUnschedulableRulesSection
        v-model:tempUnschedEnabled="tempUnschedEnabled"
        v-model:tempUnschedRules="tempUnschedRules"
        :temp-unsched-presets="tempUnschedPresets"
        :get-temp-unsched-rule-key="getTempUnschedRuleKey"
        :add-temp-unsched-rule="addTempUnschedRule"
        :remove-temp-unsched-rule="removeTempUnschedRule"
        :move-temp-unsched-rule="moveTempUnschedRule"
      />

      <InterceptWarmupRequestsSection
        :show="form.platform === 'anthropic' || form.platform === 'antigravity'"
        v-model:interceptWarmupRequests="interceptWarmupRequests"
      />

      <!-- 配额控制 (Anthropic OAuth/SetupToken: 亲和 + 窗口费用 + 会话 + RPM 等) -->
      <QuotaControlPanel
        v-if="form.platform === 'anthropic' && accountCategory === 'oauth-based'"
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
      />

      <TlsFingerprintPanel
        v-if="supportsTLSFingerprint(form.platform)"
        v-model:tls-fingerprint-enabled="tlsFingerprintEnabled"
        v-model:tls-fingerprint-profile-id="tlsFingerprintProfileId"
        v-model:tls-fingerprint-router-id="tlsFingerprintRouterId"
        v-model:tls-fingerprint-default-o-s="tlsFingerprintDefaultOS"
        v-model:tls-fingerprint-binding-rows="tlsFingerprintBindingRows"
        :tls-fingerprint-profiles="tlsFingerprintProfiles"
        :tls-fingerprint-routers="tlsFingerprintRouters"
      />

      <div>
        <div class="mb-1 flex items-center gap-2">
          <label class="input-label mb-0">{{ t('admin.accounts.proxy') }}</label>
          <ProxyAdBanner />
        </div>
        <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
      </div>

      <CreateAccountLimitsFieldsSection
        v-model:concurrency="form.concurrency"
        v-model:load-factor="form.load_factor"
        v-model:priority="form.priority"
        v-model:rate-multiplier="form.rate_multiplier"
      />
      <div class="border-t border-line pt-4">
        <label class="input-label">{{ t('admin.accounts.expiresAt') }}</label>
        <input v-model="expiresAtInput" type="datetime-local" class="input" />
        <p class="input-hint">
          {{ t('admin.accounts.expiresAtHint') }}
          {{ t('admin.accounts.expiresAtTimezoneHint', { timezone: browserTimeZone }) }}
        </p>
      </div>

      <CreateOpenAIAnthropicOptionsSection
        :platform="form.platform"
        :account-type="form.type"
        :account-category="accountCategory"
        v-model:openai-passthrough-enabled="openaiPassthroughEnabled"
        v-model:openai-flatten-namespaces-enabled="openaiFlattenNamespacesEnabled"
        v-model:openai-responses-web-socket-v2-mode="openaiResponsesWebSocketV2Mode"
        :open-ai-ws-mode-concurrency-hint-key="openAIWSModeConcurrencyHintKey"
        :open-ai-ws-mode-options="openAIWSModeOptions"
        v-model:anthropic-passthrough-enabled="anthropicPassthroughEnabled"
        v-model:anthropic-api-key-auth-scheme="anthropicAPIKeyAuthScheme"
        :web-search-global-enabled="webSearchGlobalEnabled"
        v-model:web-search-emulation-mode="webSearchEmulationMode"
        :hide-account-long-context-billing="hideAccountLongContextBilling"
        :open-ai-long-context-billing-enabled="openAILongContextBillingEnabled"
        :toggleOpenAILongContextBilling="toggleOpenAILongContextBilling"
        v-model:codex-cli-only-enabled="codexCLIOnlyEnabled"
        v-model:codex-cli-only-app-server-enabled="codexCLIOnlyAppServerEnabled"
        v-model:codex-fingerprint-mode="codexFingerprintMode"
        :codex-fingerprint-mode-options="codexFingerprintModeOptions"
        v-model:open-ai-compact-mode="openAICompactMode"
        :open-ai-compact-mode-options="openAICompactModeOptions"
        :open-ai-compact-model-mappings="openAICompactModelMappings"
        :get-open-ai-compact-model-mapping-key="getOpenAICompactModelMappingKey"
        :remove-open-ai-compact-model-mapping="removeOpenAICompactModelMapping"
        :add-open-ai-compact-model-mapping="addOpenAICompactModelMapping"
        v-model:open-ai-responses-mode="openAIResponsesMode"
        :open-ai-responses-mode-options="openAIResponsesModeOptions"
        :open-ai-text-generation-capability-enabled="openAITextGenerationCapabilityEnabled"
        :open-ai-endpoint-capability-options="openAIEndpointCapabilityOptions"
        :open-ai-endpoint-capabilities="openAIEndpointCapabilities"
        :toggle-open-ai-endpoint-capability="toggleOpenAIEndpointCapability"
      />

      <div>
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{
              t('admin.accounts.autoPauseOnExpired')
            }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.autoPauseOnExpiredDesc') }}
            </p>
          </div>
          <InlineToggleSwitch v-model="autoPauseOnExpired" />
        </div>
      </div>

      <div class="border-t border-line pt-4">
        <AntigravityMixedSchedulingSection
          :show="form.platform === 'antigravity'"
          v-model:mixed-scheduling="mixedScheduling"
          v-model:allow-overages="allowOverages"
        />

        <!-- Group Selection - 仅标准模式显示 -->
        <GroupSelector
          v-if="!authStore.isSimpleMode"
          v-model="form.group_ids"
          :groups="groups"
          :platform="form.platform"
          :mixed-scheduling="mixedScheduling"
          data-tour="account-form-groups"
        />
      </div>

    </form>

    <!-- Step 2: OAuth Authorization -->
    <div v-else class="space-y-5">
      <KiroAuthorizationFlow
        v-if="form.platform === 'kiro' && form.type === 'oauth'"
        mode="create"
        :auth-url="kiroOAuth.authUrl.value"
        :callback-base-url="kiroOAuth.callbackBaseUrl?.value || ''"
        :proxy-id="form.proxy_id || null"
        :loading="kiroOAuth.loading.value"
        :error="kiroOAuth.error.value"
        :continuation="kiroOAuth.continuation.value"
        :external-i-d-p-authorization="kiroOAuth.externalIDPAuthorization.value"
        @generate-url="handleGenerateUrl"
        @submit="handleKiroAuthorize"
        @submit-refresh-token="handleKiroValidateRT"
        @cancel-continuation="kiroOAuth.cancelDeviceAuthorization"
      />
      <OAuthAuthorizationFlow
        v-else
        ref="oauthFlowRef"
        :add-method="form.platform === 'anthropic' ? addMethod : 'oauth'"
        :auth-url="currentAuthUrl"
        :session-id="currentSessionId"
        :loading="currentOAuthLoading"
        :error="currentOAuthError"
        :show-help="form.platform === 'anthropic'"
        :show-proxy-warning="form.platform !== 'openai' && form.platform !== 'grok' && !!form.proxy_id"
        :allow-multiple="form.platform === 'anthropic'"
        :show-cookie-option="form.platform === 'anthropic'"
        :show-refresh-token-option="form.platform === 'openai' || form.platform === 'antigravity' || form.platform === 'grok'"
        :show-mobile-refresh-token-option="form.platform === 'openai'"
        :show-session-token-option="false"
        :show-access-token-option="false"
        :show-codex-session-import-option="form.platform === 'openai'"
        :show-agent-identity-option="form.platform === 'openai'"
        :show-codex-pat-option="form.platform === 'openai'"
        :show-sso-option="form.platform === 'grok'"
        :show-email-password-option="false"
        :show-manual-option="true"
        :initial-input-method="'manual'"
        :platform="form.platform"
        :show-project-id="geminiOAuthType === 'code_assist'"
        @generate-url="handleGenerateUrl"
        @cookie-auth="handleCookieAuth"
        @validate-refresh-token="handleValidateRefreshToken"
        @validate-mobile-refresh-token="handleOpenAIValidateMobileRT"
        @validate-session-token="handleValidateSessionToken"
        @import-codex-session="handleOpenAIImportCodexSession"
        @import-codex-pat="handleOpenAIImportCodexPAT"
        @import-sso="handleGrokImportSSO"
        @authorize-password="handleGrokAuthorizePassword"
      />

    </div>

    <template #footer>
      <CreateAccountFooter
        :step="step"
        :submitting="submitting"
        :is-o-auth-flow="isOAuthFlow"
        :platform="form.platform"
        :is-manual-input-method="isManualInputMethod"
        :can-exchange-code="!!canExchangeCode"
        :current-o-auth-loading="currentOAuthLoading"
        @close="handleClose"
        @back="goBackToBasicInfo"
        @exchange-code="handleExchangeCode"
      />
    </template>
  </BaseDialog>

  <GeminiHelpDialog v-model:show="showGeminiHelpDialog" :gemini-help-links="geminiHelpLinks" />

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
import { ref, reactive, computed, watch, onUnmounted, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  getPresetMappingsByPlatform,
  getModelsByPlatform,
  fetchAntigravityDefaultMappings
} from '@/composables/useModelWhitelist'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import { useQuotaNotifyState } from '@/composables/useQuotaNotifyState'
import {
  useAccountOAuth,
  type AddMethod,
  type AuthInputMethod
} from '@/composables/useAccountOAuth'
import { useOpenAIOAuth } from '@/composables/useOpenAIOAuth'
import { useGeminiOAuth } from '@/composables/useGeminiOAuth'
import { useAntigravityOAuth } from '@/composables/useAntigravityOAuth'
import { useGrokOAuth } from '@/composables/useGrokOAuth'
import { useKiroOAuth } from '@/composables/useKiroOAuth'
import type {
  Proxy,
  AdminGroup,
  AccountPlatform,
  AccountType
} from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'
import QuotaControlPanel from '@/components/account/shared/QuotaControlPanel.vue'
import TlsFingerprintPanel from '@/components/account/shared/TlsFingerprintPanel.vue'
import QuotaLimitCardSection from '@/components/account/shared/QuotaLimitCardSection.vue'
import HeaderOverrideSection from '@/components/account/shared/HeaderOverrideSection.vue'
import GrokCustomBaseUrlSection from '@/components/account/shared/GrokCustomBaseUrlSection.vue'
import GeminiHelpDialog from '@/components/account/create/GeminiHelpDialog.vue'
import PlatformSelector from '@/components/account/create/PlatformSelector.vue'
import CnAccountOptionsSection from '@/components/account/create/CnAccountOptionsSection.vue'
import AntigravityMixedSchedulingSection from '@/components/account/create/AntigravityMixedSchedulingSection.vue'
import CreateAccountStepIndicator from '@/components/account/create/CreateAccountStepIndicator.vue'
import CreateAccountFooter from '@/components/account/create/CreateAccountFooter.vue'
import AnthropicAddMethodSection from '@/components/account/create/AnthropicAddMethodSection.vue'
import CreateAccountLimitsFieldsSection from '@/components/account/create/CreateAccountLimitsFieldsSection.vue'
import { useCreateAccountPlatformHints, geminiHelpLinks } from './create/useCreateAccountPlatformHints'
import CreateBedrockCredentialsSection from '@/components/account/create/CreateBedrockCredentialsSection.vue'
import CreateApiKeySection from '@/components/account/create/CreateApiKeySection.vue'
import CreateOpenAIAnthropicOptionsSection from '@/components/account/create/CreateOpenAIAnthropicOptionsSection.vue'
import { useTempUnschedRules } from '@/components/account/shared/useTempUnschedRules'
import TempUnschedulableRulesSection from '@/components/account/shared/TempUnschedulableRulesSection.vue'
import InterceptWarmupRequestsSection from '@/components/account/shared/InterceptWarmupRequestsSection.vue'
const GrokPanel = defineAsyncComponent(() => import('@/components/account/platform/GrokPanel.vue'))
import ProxySelector from '@/components/common/ProxySelector.vue'
import ProxyAdBanner from '@/components/common/ProxyAdBanner.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
const OpenAIPanel = defineAsyncComponent(() => import('@/components/account/platform/OpenAIPanel.vue'))
const KiroPanel = defineAsyncComponent(() => import('@/components/account/platform/KiroPanel.vue'))
const AnthropicPanel = defineAsyncComponent(() => import('@/components/account/platform/AnthropicPanel.vue'))
const GeminiPanel = defineAsyncComponent(() => import('@/components/account/platform/GeminiPanel.vue'))
const AntigravityPanel = defineAsyncComponent(() => import('@/components/account/platform/AntigravityPanel.vue'))
const VertexServiceAccountPanel = defineAsyncComponent(() => import('@/components/account/platform/VertexServiceAccountPanel.vue'))
import { allSelectedGroupsEnableLongContextPricing } from '@/components/account/longContextBilling'
import {
  defaultCNBaseUrl,
  type CnAccountMode,
  type CnApiProtocol,
  type CnNativeApiProtocol,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import { getBrowserTimeZone } from '@/utils/format'
import { OPENAI_WS_MODE_OFF } from '@/utils/openaiWsMode'
import OAuthAuthorizationFlow from './OAuthAuthorizationFlow.vue'
import KiroAuthorizationFlow from './KiroAuthorizationFlow.vue'
import { useCreateAccountOAuthFlows } from './create/useCreateAccountOAuthFlows'
import { useCreateAccountOAuthExchange } from './create/useCreateAccountOAuthExchange'
import { useCreateAccountSubmit } from './create/useCreateAccountSubmit'
import { useCreateAccountReset } from './create/useCreateAccountReset'
import { useCreateAccountMixedChannel } from './create/useCreateAccountMixedChannel'
import { useCreateAccountCnPlatform } from './create/useCreateAccountCnPlatform'
import { useCreateAccountTlsFingerprint } from './create/useCreateAccountTlsFingerprint'
import { useCreateAccountOpenAiOptions } from './create/useCreateAccountOpenAiOptions'
import { useCreateAccountPoolMode } from './create/useCreateAccountPoolMode'
import { useCreateAccountGrokOAuthUpstream } from './create/useCreateAccountGrokOAuthUpstream'
import { useCreateAccountVertexServiceAccount } from './create/useCreateAccountVertexServiceAccount'

// Type for exposed OAuthAuthorizationFlow component
// Note: defineExpose automatically unwraps refs, so we use the unwrapped types
interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  sessionKey: string
  refreshToken: string
  sessionToken: string
  codexSession: string
  codexPAT: string
  ssoCookie: string
  inputMethod: AuthInputMethod
  reset: () => void
}

const { t } = useI18n()
const authStore = useAuthStore()
const browserTimeZone = getBrowserTimeZone()

interface Props {
  show: boolean
  proxies: Proxy[]
  groups: AdminGroup[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  created: []
}>()

const appStore = useAppStore()

const hideAccountLongContextBilling = computed(() => {
  return allSelectedGroupsEnableLongContextPricing(form.group_ids, props.groups)
})

// OAuth composables
const oauth = useAccountOAuth() // For Anthropic OAuth
const openaiOAuth = useOpenAIOAuth() // For OpenAI OAuth
const geminiOAuth = useGeminiOAuth() // For Gemini OAuth
const antigravityOAuth = useAntigravityOAuth() // For Antigravity OAuth
const grokOAuth = useGrokOAuth() // For Grok OAuth
const kiroOAuth = useKiroOAuth() // For Kiro OAuth

// Computed: current OAuth state for template binding
const currentAuthUrl = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.authUrl.value
  if (form.platform === 'gemini') return geminiOAuth.authUrl.value
  if (form.platform === 'antigravity') return antigravityOAuth.authUrl.value
  if (form.platform === 'grok') return grokOAuth.authUrl.value
  return oauth.authUrl.value
})

const currentSessionId = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.sessionId.value
  if (form.platform === 'gemini') return geminiOAuth.sessionId.value
  if (form.platform === 'antigravity') return antigravityOAuth.sessionId.value
  if (form.platform === 'grok') return grokOAuth.sessionId.value
  return oauth.sessionId.value
})

const currentOAuthLoading = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.loading.value
  if (form.platform === 'gemini') return geminiOAuth.loading.value
  if (form.platform === 'antigravity') return antigravityOAuth.loading.value
  if (form.platform === 'grok') return grokOAuth.loading.value
  return oauth.loading.value
})

const currentOAuthError = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.error.value
  if (form.platform === 'gemini') return geminiOAuth.error.value
  if (form.platform === 'antigravity') return antigravityOAuth.error.value
  if (form.platform === 'grok') return grokOAuth.error.value
  return oauth.error.value
})

// Refs
const oauthFlowRef = ref<OAuthFlowExposed | null>(null)

// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

// State
const step = ref(1)
const submitting = ref(false)
const accountCategory = ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>('oauth-based') // UI selection for account category
const addMethod = ref<AddMethod>('oauth') // For oauth-based: 'oauth' or 'setup-token'
const apiKeyBaseUrl = ref('https://api.anthropic.com')
const apiKeyValue = ref('')
const upstreamBillingAutoProbeEnabled = ref(true)

// ── 国产供应商（Kimi / Zhipu / DeepSeek）账号类型、API 协议与端点 ──
const accountMode = ref<CnAccountMode>('payg')
// API 协议决定转发端点与格式：cc=现有转换链，anthropic=原生直通（Claude Code），
// responses=deepseek / kimi 原生 Responses 端点（Codex）。与账号类型正交。
const apiProtocol = ref<CnApiProtocol>('adaptive')
// 智谱团队版 Coding Plan：组织/项目 ID，写入 credentials 供额度探测切换团队端点
const zhipuOrganization = ref('')
const zhipuProject = ref('')
const adaptiveBaseUrls = ref<Record<CnNativeApiProtocol, string>>({
  chat_completions: '',
  anthropic: '',
  responses: ''
})

const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)
const editDailyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editDailyResetHour = ref<number | null>(null)
const editWeeklyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editWeeklyResetDay = ref<number | null>(null)
const editWeeklyResetHour = ref<number | null>(null)
const editResetTimezone = ref<string | null>(null)
const modelMappings = ref<ModelMapping[]>([])
const openAICompactModelMappings = ref<ModelMapping[]>([])
const modelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const allowedModels = ref<string[]>([])
const upstreamModelsPreviewed = ref(false)
const {
  DEFAULT_POOL_MODE_RETRY_COUNT,
  poolModeEnabled,
  poolModeRetryCount,
  poolModeRetryStatusCodesInput,
  parsePoolModeRetryStatusCodes,
  normalizePoolModeRetryCount
} = useCreateAccountPoolMode()
const customErrorCodesEnabled = ref(false)
const selectedErrorCodes = ref<number[]>([])
const customErrorCodeInput = ref<number | null>(null)
const headerOverrideEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

const {
  grokOAuthCustomBaseUrlEnabled,
  grokOAuthBaseUrl,
  validateGrokOAuthUpstreamConfig,
  applyGrokOAuthUpstreamConfig
} = useCreateAccountGrokOAuthUpstream({ appStore, t, headerOverrideEnabled, headerOverrideRows })
const interceptWarmupRequests = ref(false)
const autoPauseOnExpired = ref(true)
const {
  globalEnabled: quotaNotifyGlobalEnabled,
  state: quotaNotifyState,
  loadGlobalState: loadQuotaNotifyGlobal,
  writeToExtra: writeQuotaNotifyToExtra,
} = useQuotaNotifyState()

// Load global feature states once
adminAPI.settings.getWebSearchEmulationConfig().then(cfg => {
  webSearchGlobalEnabled.value = cfg?.enabled === true && (cfg?.providers?.length ?? 0) > 0
}).catch(() => { webSearchGlobalEnabled.value = false })

loadQuotaNotifyGlobal()
const mixedScheduling = ref(false) // For antigravity accounts: enable mixed scheduling
const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityAccountType = ref<'oauth' | 'upstream'>('oauth') // For antigravity: oauth or upstream
const kiroAccountType = ref<'oauth' | 'apikey'>('oauth') // For Kiro: oauth or manual API key
const antigravityProjectId = ref('')
const upstreamBaseUrl = ref('') // For upstream type: base URL
const upstreamApiKey = ref('') // For upstream type: API key
const kiroAPIKeyValue = ref('')
const kiroRegion = ref('us-east-1')
const kiroAuthRegion = ref('')
const kiroAPIRegion = ref('')
const kiroProfileARN = ref('')
const kiroMachineID = ref('')
const antigravityModelRestrictionMode = ref<'whitelist' | 'mapping'>('whitelist')
const antigravityWhitelistModels = ref<string[]>([])
const antigravityModelMappings = ref<ModelMapping[]>([])
const bedrockPresets = computed(() => getPresetMappingsByPlatform('bedrock'))

// Bedrock credentials
const bedrockAuthMode = ref<'sigv4' | 'apikey'>('sigv4')
const bedrockAccessKeyId = ref('')
const bedrockSecretAccessKey = ref('')
const bedrockSessionToken = ref('')
const bedrockRegion = ref('us-east-1')
const bedrockForceGlobal = ref(false)
const bedrockApiKeyValue = ref('')
const {
  vertexServiceAccountJson,
  vertexProjectId,
  vertexClientEmail,
  vertexLocation,
  applyVertexServiceAccountJson,
  parseVertexServiceAccountJson
} = useCreateAccountVertexServiceAccount({ appStore, t })
const {
  tempUnschedEnabled,
  tempUnschedRules,
  getTempUnschedRuleKey,
  tempUnschedPresets,
  addTempUnschedRule,
  removeTempUnschedRule,
  moveTempUnschedRule,
  buildTempUnschedRules,
  applyTempUnschedConfig
} = useTempUnschedRules('create-temp-unsched-rule')
const geminiOAuthType = ref<'code_assist' | 'google_one' | 'ai_studio'>('google_one')
const geminiAIStudioOAuthEnabled = ref(false)

const showAdvancedOAuth = ref(false)
const showGeminiHelpDialog = ref(false)

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
  supportsTLSFingerprint,
  applyTLSFingerprintToExtra,
  withTLSFingerprintExtra
} = useCreateAccountTlsFingerprint()
const sessionIdMaskingEnabled = ref(false)
const cacheTTLOverrideEnabled = ref(false)
const cacheTTLOverrideTarget = ref<string>('5m')
const customBaseUrlEnabled = ref(false)
const customBaseUrl = ref('')

// Gemini tier selection (used as fallback when auto-detection is unavailable/fails)
const geminiTierGoogleOne = ref<'google_one_free' | 'google_ai_pro' | 'google_ai_ultra'>('google_one_free')
const geminiTierGcp = ref<'gcp_standard' | 'gcp_enterprise'>('gcp_standard')
const geminiTierAIStudio = ref<'aistudio_free' | 'aistudio_paid'>('aistudio_free')

// Computed: current preset mappings based on platform
const presetMappings = computed(() => getPresetMappingsByPlatform(form.platform))

const form = reactive({
  name: '',
  notes: '',
  platform: 'anthropic' as AccountPlatform,
  type: 'oauth' as AccountType, // Will be 'oauth', 'setup-token', or 'apikey'
  credentials: {} as Record<string, unknown>,
  proxy_id: null as number | null,
  concurrency: 10,
  load_factor: null as number | null,
  priority: 1,
  rate_multiplier: 1,
  group_ids: [] as number[],
  expires_at: null as number | null
})

const {
  isCNPlatform,
  cnPresetPlatform,
  cnProtocolOptions,
  cnAdaptiveProtocolOptions,
  cnAccentActiveClass,
  cnAccentIconClass,
  selectCNPlatform,
  onCnPresetSelect,
  syncPreviewCredentials
} = useCreateAccountCnPlatform({
  form,
  accountCategory,
  accountMode,
  apiProtocol,
  apiKeyBaseUrl,
  adaptiveBaseUrls,
  apiKeyValue,
  modelRestrictionMode,
  allowedModels,
  modelMappings
})

const {
  oauthStepTitle,
  baseUrlHint,
  apiKeyHint,
  apiKeyBaseUrlPlaceholder,
  apiKeyValuePlaceholder,
  geminiSelectedTier
} = useCreateAccountPlatformHints({
  form,
  accountMode,
  apiProtocol,
  isCNPlatform,
  t,
  accountCategory,
  geminiOAuthType,
  geminiTierGoogleOne,
  geminiTierGcp,
  geminiTierAIStudio
})

const {
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
  openAITextGenerationCapabilityEnabled,
  openAIEndpointCapabilityOptions,
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
} = useCreateAccountOpenAiOptions({
  form,
  accountCategory,
  t,
  openAICompactModelMappings
})

// Helper to check if current type needs OAuth flow
const isOAuthFlow = computed(() => {
  // Antigravity upstream 类型不需要 OAuth 流程
  if (form.platform === 'antigravity' && antigravityAccountType.value === 'upstream') {
    return false
  }
  if (form.platform === 'kiro') {
    return kiroAccountType.value === 'oauth'
  }
  // Bedrock 类型不需要 OAuth 流程
  if (form.platform === 'anthropic' && accountCategory.value === 'bedrock') {
    return false
  }
  return accountCategory.value === 'oauth-based'
})

const accountNamePlaceholder = computed(() =>
  isOAuthFlow.value
    ? t('admin.accounts.accountNameOAuthPlaceholder')
    : t('admin.accounts.accountNameRequiredPlaceholder')
)

const isManualInputMethod = computed(() => {
  return oauthFlowRef.value?.inputMethod === 'manual'
})

const expiresAtInput = computed({
  get: () => formatDateTimeLocal(form.expires_at),
  set: (value: string) => {
    form.expires_at = parseDateTimeLocal(value)
  }
})

const canExchangeCode = computed(() => {
  const authCode = oauthFlowRef.value?.authCode || ''
  if (form.platform === 'openai') {
    return authCode.trim() && openaiOAuth.sessionId.value && !openaiOAuth.loading.value
  }
  if (form.platform === 'gemini') {
    return authCode.trim() && geminiOAuth.sessionId.value && !geminiOAuth.loading.value
  }
  if (form.platform === 'antigravity') {
    return authCode.trim() && antigravityOAuth.sessionId.value && !antigravityOAuth.loading.value
  }
  if (form.platform === 'grok') {
    return authCode.trim() && grokOAuth.sessionId.value && !grokOAuth.loading.value
  }
  return authCode.trim() && oauth.sessionId.value && !oauth.loading.value
})

// Watchers
watch(
  () => props.show,
  (newVal, wasShow) => {
    if (newVal) {
      Promise.all([
        adminAPI.tlsFingerprintProfiles.list(),
        adminAPI.tlsFingerprintRouters.list()
      ])
        .then(([profiles, routers]) => {
          tlsFingerprintProfiles.value = profiles.map(p => ({ id: p.id, name: p.name }))
          tlsFingerprintRouters.value = routers.map(r => ({ id: r.id, name: r.name }))
        })
        .catch(() => {
          tlsFingerprintProfiles.value = []
          tlsFingerprintRouters.value = []
        })
      // Modal opened - fill related models
      allowedModels.value = form.platform === 'kiro' ? [] : [...getModelsByPlatform(form.platform)]
      // Antigravity: 默认使用映射模式并填充默认映射
      if (form.platform === 'antigravity') {
        antigravityModelRestrictionMode.value = 'mapping'
        fetchAntigravityDefaultMappings().then(mappings => {
          antigravityModelMappings.value = [...mappings]
        })
        antigravityWhitelistModels.value = []
      } else {
        antigravityWhitelistModels.value = []
        antigravityModelMappings.value = []
        antigravityModelRestrictionMode.value = 'mapping'
      }
    } else if (wasShow) {
      resetForm()
    }
  },
  { immediate: true }
)

// Sync form.type based on accountCategory, addMethod, and platform-specific type
watch(
  [accountCategory, addMethod, antigravityAccountType, kiroAccountType, () => form.platform],
  ([category, method, agType, currentKiroType]) => {
    if (form.platform === 'kiro') {
      form.type = currentKiroType
      return
    }
    // Antigravity upstream 类型（实际创建为 apikey）
    if (form.platform === 'antigravity' && agType === 'upstream') {
      form.type = 'apikey'
      return
    }
    // Bedrock 类型
    if (form.platform === 'anthropic' && category === 'bedrock') {
      form.type = 'bedrock' as AccountType
      return
    }
    if ((form.platform === 'gemini' || form.platform === 'anthropic') && category === 'service_account') {
      form.type = 'service_account' as AccountType
    } else if (category === 'oauth-based') {
      form.type = form.platform === 'anthropic' ? method as AccountType : 'oauth'
    } else {
      form.type = 'apikey'
    }
  },
  { immediate: true }
)

// Reset platform-specific settings when platform changes
watch(
  () => form.platform,
  (newPlatform) => {
    // Reset base URL based on platform
    if (newPlatform === 'kimi' || newPlatform === 'zhipu' || newPlatform === 'deepseek') {
      apiKeyBaseUrl.value = defaultCNBaseUrl(newPlatform, accountMode.value, apiProtocol.value)
    } else {
      apiKeyBaseUrl.value =
        (newPlatform === 'openai')
          ? 'https://api.openai.com'
          : newPlatform === 'gemini'
            ? 'https://generativelanguage.googleapis.com'
            : newPlatform === 'grok'
              ? 'https://api.x.ai/v1'
              : 'https://api.anthropic.com'
    }
    // Clear model-related settings
    allowedModels.value = []
    upstreamModelsPreviewed.value = false
    modelMappings.value = []
    // Antigravity: 默认使用映射模式并填充默认映射
    if (newPlatform === 'antigravity') {
      antigravityModelRestrictionMode.value = 'mapping'
      fetchAntigravityDefaultMappings().then(mappings => {
        antigravityModelMappings.value = [...mappings]
      })
      antigravityWhitelistModels.value = []
      accountCategory.value = 'oauth-based'
      antigravityAccountType.value = 'oauth'
    } else if (newPlatform === 'kiro') {
      accountCategory.value = 'oauth-based'
      addMethod.value = 'oauth'
      kiroAccountType.value = 'oauth'
      kiroAPIKeyValue.value = ''
      kiroRegion.value = 'us-east-1'
      kiroAuthRegion.value = ''
      kiroAPIRegion.value = ''
      kiroProfileARN.value = ''
      kiroMachineID.value = ''
    } else {
      allowOverages.value = false
      antigravityProjectId.value = ''
      antigravityWhitelistModels.value = []
      antigravityModelMappings.value = []
      antigravityModelRestrictionMode.value = 'mapping'
    }
    if (newPlatform === 'grok') {
      accountCategory.value = 'oauth-based'
      addMethod.value = 'oauth'
      modelRestrictionMode.value = 'mapping'
      form.concurrency = 1
      form.load_factor = null
    }
    if (newPlatform !== 'gemini' && newPlatform !== 'anthropic' && accountCategory.value === 'service_account') {
      accountCategory.value = 'oauth-based'
    }
    if (newPlatform !== 'anthropic' && accountCategory.value === 'bedrock') {
      accountCategory.value = 'oauth-based'
    }
    // Reset Bedrock fields when switching platforms
    bedrockAccessKeyId.value = ''
    bedrockSecretAccessKey.value = ''
    bedrockSessionToken.value = ''
    bedrockRegion.value = 'us-east-1'
    bedrockForceGlobal.value = false
    bedrockAuthMode.value = 'sigv4'
    bedrockApiKeyValue.value = ''
    vertexServiceAccountJson.value = ''
    vertexProjectId.value = ''
    vertexClientEmail.value = ''
    vertexLocation.value = 'global'
    // Reset Anthropic/Antigravity-specific settings when switching to other platforms
    if (newPlatform !== 'anthropic' && newPlatform !== 'antigravity') {
      interceptWarmupRequests.value = false
    }
    if (newPlatform !== 'openai') {
      openaiPassthroughEnabled.value = false
      openaiFlattenNamespacesEnabled.value = false
      openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
      openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
      openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
      codexCLIOnlyEnabled.value = false
      codexCLIOnlyAppServerEnabled.value = false
    }
    if (newPlatform !== 'anthropic') {
      anthropicPassthroughEnabled.value = false
      anthropicAPIKeyAuthScheme.value = 'x_api_key'
      webSearchEmulationMode.value = 'default'
    }
    // 请求头覆写为平台相关配置（常用头集合不同），切换平台时清空，
    // 避免上一平台的配置行被提交到新平台账号
    headerOverrideEnabled.value = false
    headerOverrideRows.value = []
    grokOAuthCustomBaseUrlEnabled.value = false
    grokOAuthBaseUrl.value = ''
    // Reset OAuth states
    oauth.resetState()
    openaiOAuth.resetState()

    geminiOAuth.resetState()
    antigravityOAuth.resetState()
    grokOAuth.resetState()
  }
)

// Gemini AI Studio OAuth availability (requires operator-configured OAuth client)
watch(
  [accountCategory, () => form.platform],
  ([category, platform]) => {
    if (platform === 'openai' && category !== 'oauth-based') {
      codexCLIOnlyEnabled.value = false
      codexCLIOnlyAppServerEnabled.value = false
    }
    if (platform !== 'anthropic' || category !== 'apikey') {
      anthropicPassthroughEnabled.value = false
      anthropicAPIKeyAuthScheme.value = 'x_api_key'
      webSearchEmulationMode.value = 'default'
    }
  }
)

watch(
  [() => props.show, () => form.platform, accountCategory],
  async ([show, platform, category]) => {
    if (!show || platform !== 'gemini' || category !== 'oauth-based') {
      geminiAIStudioOAuthEnabled.value = false
      return
    }
    const caps = await geminiOAuth.getCapabilities()
    geminiAIStudioOAuthEnabled.value = !!caps?.ai_studio_oauth_enabled
    if (!geminiAIStudioOAuthEnabled.value && geminiOAuthType.value === 'ai_studio') {
      geminiOAuthType.value = 'code_assist'
    }
  },
  { immediate: true }
)

// Auto-fill related models when switching to whitelist mode or changing platform
watch(
  [modelRestrictionMode, () => form.platform],
  ([newMode, platform]) => {
    if (newMode === 'whitelist' && platform !== 'kiro') {
      allowedModels.value = [...getModelsByPlatform(form.platform)]
    }
  }
)

watch(
  [antigravityModelRestrictionMode, () => form.platform],
  ([, platform]) => {
    if (platform !== 'antigravity') return
    // Antigravity 默认不做限制：白名单留空表示允许所有（包含未来新增模型）。
    // 如果需要快速填充常用模型，可在组件内点“填充相关模型”。
  }
)



const {
  showMixedChannelWarning,
  antigravityMixedChannelConfirmed,
  clearMixedChannelDialog,
  handleClose,
  withAntigravityConfirmFlag,
  ensureAntigravityMixedChannelConfirmed,
  buildAntigravityExtra,
  doCreateAccount,
  handleMixedChannelConfirm,
  handleMixedChannelCancel,
  mixedChannelWarningMessageText
} = useCreateAccountMixedChannel({
  form,
  appStore,
  t,
  emit,
  submitting,
  upstreamModelsPreviewed,
  kiroOAuth,
  supportsTLSFingerprint,
  applyTLSFingerprintToExtra,
  mixedScheduling,
  allowOverages
})

const { resetForm } = useCreateAccountReset({
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
})

onUnmounted(() => {
  kiroOAuth.cancelDeviceAuthorization()
})

const createAccountOAuthFlowDeps = {
  form,
  emit,
  addMethod,
  allowedModels,
  antigravityModelMappings,
  antigravityProjectId,
  apiKeyBaseUrl,
  autoPauseOnExpired,
  baseRpm,
  cacheTTLOverrideEnabled,
  cacheTTLOverrideTarget,
  customBaseUrl,
  customBaseUrlEnabled,
  editDailyResetHour,
  editDailyResetMode,
  editQuotaDailyLimit,
  editQuotaLimit,
  editQuotaWeeklyLimit,
  editResetTimezone,
  editWeeklyResetDay,
  editWeeklyResetHour,
  editWeeklyResetMode,
  geminiOAuthType,
  interceptWarmupRequests,
  maxSessions,
  modelMappings,
  modelRestrictionMode,
  oauthFlowRef,
  rpmLimitEnabled,
  rpmStickyBuffer,
  rpmStrategy,
  sessionIdMaskingEnabled,
  sessionIdleTimeout,
  sessionLimitEnabled,
  upstreamBillingAutoProbeEnabled,
  userMsgQueueMode,
  windowCostEnabled,
  windowCostLimit,
  windowCostStickyReserve,
  step,
  tempUnschedEnabled,
  tempUnschedRules,
  buildTempUnschedRules,
  applyTempUnschedConfig,
  geminiSelectedTier,
  isOpenAIModelRestrictionDisabled,
  oauth,
  openaiOAuth,
  geminiOAuth,
  antigravityOAuth,
  grokOAuth,
  kiroOAuth,
  applyGrokOAuthUpstreamConfig,
  applyOpenAIEndpointCapabilities,
  applyTLSFingerprintToExtra,
  withTLSFingerprintExtra,
  buildAntigravityExtra,
  buildOpenAICodexImportExtra,
  buildOpenAICompactModelMapping,
  buildOpenAIExtra,
  doCreateAccount,
  handleClose,
  validateGrokOAuthUpstreamConfig,
  withAntigravityConfirmFlag,
  writeQuotaNotifyToExtra
}

const {
  goBackToBasicInfo,
  handleGenerateUrl,
  applyKiroModelRestriction,
  handleKiroAuthorize,
  handleKiroValidateRT,
  handleValidateSessionToken,
  formatDateTimeLocal,
  parseDateTimeLocal,
  createAccountAndFinish,
  handleGrokValidateRT,
  handleGrokImportSSO,
  handleGrokAuthorizePassword
} = useCreateAccountOAuthFlows(createAccountOAuthFlowDeps)

const {
  handleOpenAIImportCodexSession,
  handleOpenAIImportCodexPAT,
  handleOpenAIValidateRT,
  handleOpenAIValidateMobileRT,
  handleAntigravityValidateRT,
  handleExchangeCode,
  handleCookieAuth
} = useCreateAccountOAuthExchange({
  ...createAccountOAuthFlowDeps,
  createAccountAndFinish
})

// Small platform router kept in the host since it needs handlers returned by
// both of the composables above (handleOpenAIValidateRT/handleAntigravityValidateRT
// from useCreateAccountOAuthExchange, handleGrokValidateRT from
// useCreateAccountOAuthFlows).
const handleValidateRefreshToken = (rt: string) => {
  if (form.platform === 'openai') {
    handleOpenAIValidateRT(rt)
  } else if (form.platform === 'antigravity') {
    handleAntigravityValidateRT(rt)
  } else if (form.platform === 'grok') {
    handleGrokValidateRT(rt)
  }
}

// The "step 1 -> create" submit dispatcher; extracted into its own composable
// (task 16A). It needs createAccountAndFinish/applyKiroModelRestriction which
// are only available after the OAuth-flows composable above has run, so this
// call is placed here at the end of the script.
const { handleSubmit } = useCreateAccountSubmit({
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
})
</script>
