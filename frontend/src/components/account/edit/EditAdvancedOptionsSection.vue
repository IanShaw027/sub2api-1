<template>
      <!-- OpenAI 自动透传开关（OAuth/API Key） -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token' || account?.type === 'apikey')"
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
        v-if="account?.platform === 'openai' && account?.type === 'oauth'"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.flattenNamespaces') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.flattenNamespacesDesc') }}
            </p>
          </div>
          <InlineToggleSwitch v-model="openaiFlattenNamespacesEnabled" data-testid="edit-openai-flatten-namespaces-toggle" />
        </div>
      </div>

      <!-- OpenAI Codex hosted image_generation bridge policy -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token' || account?.type === 'apikey')"
        class="border-t border-line pt-4"
      >
        <div class="overflow-hidden rounded-lg border border-[color-mix(in_oklch,var(--accent)_20%,transparent)] bg-[color-mix(in_oklch,var(--accent)_8%,transparent)] shadow-sm">
          <div class="flex items-start gap-3 px-4 py-3">
            <div class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface text-accent shadow-sm ring-1 ring-[color-mix(in_oklch,var(--accent)_20%,transparent)]">
              <Icon name="sparkles" size="sm" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <label class="input-label mb-0">{{ t('admin.accounts.openai.codexImageTool') }}</label>
                <span
                  class="rounded-full px-2 py-0.5 text-[11px] font-medium"
                  :class="codexImageToolBadgeClass"
                >
                  {{ codexImageToolBadgeLabel }}
                </span>
              </div>
              <p class="mt-1 text-xs leading-5 text-muted">
                {{ t('admin.accounts.openai.codexImageToolDesc') }}
              </p>
            </div>
          </div>
          <div class="border-t border-[color-mix(in_oklch,var(--accent)_20%,transparent)] bg-surface/70 p-2">
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <button
                v-for="option in codexImageToolOptions"
                :key="option.value"
                type="button"
                :data-testid="`codex-image-tool-${option.value}`"
                @click="codexImageToolMode = option.value"
                :class="[
 'group flex min-h-[62px] items-start gap-2 rounded-md border px-3 py-2 text-left transition-all',
 codexImageToolMode === option.value
 ? option.selectedCardClass
 : 'border-transparent bg-transparent text-muted hover:border-line hover:bg-surface-2'
 ]"
              >
                <span
                  :class="[
 'mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-colors',
 codexImageToolMode === option.value
 ? option.selectedDotClass
 : 'border-line text-transparent group-hover:border-line'
 ]"
                >
                  <Icon name="check" size="xs" :stroke-width="2" />
                </span>
                <span class="min-w-0">
                  <span class="block text-sm font-medium">{{ option.label }}</span>
                  <span class="mt-0.5 block text-xs leading-4 text-muted">{{ option.description }}</span>
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- OpenAI WS Mode 三态（off/ctx_pool/passthrough） -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token' || account?.type === 'apikey')"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.wsMode') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.wsModeDesc') }}
            </p>
            <p class="mt-1 text-xs text-muted">
              {{ t(openAIWSModeConcurrencyHintKey) }}
            </p>
          </div>
          <div class="w-52">
            <Select v-model="openaiResponsesWebSocketV2Mode" data-testid="edit-openai-ws-mode-select" :options="openAIWSModeOptions" />
          </div>
        </div>
      </div>

      <!-- OpenAI APIKey Responses API support mode -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'apikey'"
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
              v-model="openAIResponsesMode"
              :options="openAIResponsesModeOptions"
              :disabled="!openAITextGenerationCapabilityEnabled"
              data-testid="openai-responses-mode-select"
            />
          </div>
        </div>
        <div
          v-if="openAITextGenerationCapabilityEnabled"
          class="rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted"
        >
          <span class="font-medium">{{ t(openAIResponsesStatusKey) }}</span>
        </div>
        <div
          v-else
          class="rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-xs text-warning-text"
          data-testid="openai-responses-mode-not-applicable"
        >
          {{ t('admin.accounts.openai.responsesModeTextDisabledHint') }}
        </div>
        <div>
          <label class="input-label mb-2 block">{{ t('admin.accounts.openai.endpointCapabilities') }}</label>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <label
              v-for="option in openAIEndpointCapabilityOptions"
              :key="option.value"
              class="flex cursor-pointer items-center gap-2 rounded-lg border border-line px-3 py-2 text-sm"
            >
              <input
                type="checkbox"
                class="rounded border-line text-accent focus:ring-accent"
                :data-testid="`openai-endpoint-capability-${option.value}`"
                :checked="openAIEndpointCapabilities.includes(option.value)"
                @change="toggleOpenAIEndpointCapability(option.value, $event)"
              />
              <span class="text-foreground">{{ option.label }}</span>
            </label>
          </div>
          <p class="input-hint">{{ t('admin.accounts.openai.endpointCapabilitiesDesc') }}</p>
        </div>
      </div>

      <div
        v-if="account?.type === 'apikey'"
        class="flex items-center justify-between gap-4 border-t border-line pt-4"
      >
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.upstreamBilling.autoProbe') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.upstreamBilling.autoProbeHint') }}
          </p>
        </div>
        <Toggle
          :model-value="upstreamBillingAutoProbeEnabled"
          data-testid="upstream-billing-auto-probe"
          :aria-label="t('admin.accounts.upstreamBilling.autoProbe')"
          @update:model-value="handleUpstreamBillingAutoProbeChange"
        />
      </div>

      <OllamaCloudUsageSettings
        v-if="account?.ollama_cloud_usage?.eligible"
        :account="account"
        @updated="handleOllamaCloudUsageUpdated"
      />

      <!-- Anthropic API Key 自动透传开关 -->
      <div
        v-if="account?.platform === 'anthropic' && account?.type === 'apikey'"
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
        v-if="account?.platform === 'anthropic' && account?.type === 'apikey'"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyAuthScheme') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.anthropic.apiKeyAuthSchemeDesc') }}
            </p>
          </div>
          <select v-model="anthropicAPIKeyAuthScheme" class="input w-52 text-sm">
            <option value="x_api_key">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeXApiKey') }}</option>
            <option value="authorization_bearer">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeBearer') }}</option>
          </select>
        </div>
      </div>

      <!-- Anthropic API Key: Web Search Emulation (hidden when global disabled) -->
      <div
        v-if="account?.platform === 'anthropic' && account?.type === 'apikey' && webSearchGlobalEnabled"
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

      <!-- eslint-disable vue/no-mutating-props -- `quotaNotifyState` is a reactive object shared
           with the host by design; the bindings below set the object's own nested fields, not
           the prop binding itself (mirrors the pre-split direct `quotaNotifyState.daily.enabled =
           ...` mutation). -->
      <!-- 配额控制 (Anthropic apikey/bedrock: 配额限制 + 亲和) -->
      <QuotaLimitCardSection
        v-if="account?.platform === 'anthropic' && (account?.type === 'apikey' || account?.type === 'bedrock')"
        :hint="t('admin.accounts.quotaControl.hint')"
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
      <!-- 配额控制 (非 Anthropic apikey/bedrock) -->
      <QuotaLimitCardSection
        v-else-if="account?.type === 'apikey' || account?.type === 'bedrock'"
        :hint="t('admin.accounts.quotaLimitHint')"
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
      <!-- eslint-enable vue/no-mutating-props -->

      <!-- OpenAI API 长上下文计费开关 -->
      <div
        v-if="account?.platform === 'openai' && !isSparkShadow && !hideAccountLongContextBilling && (account?.type === 'oauth' || account?.type === 'setup-token' || account?.type === 'apikey')"
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
            v-model="openAILongContextBillingEnabled"
            data-testid="openai-long-context-billing-toggle"
            switch-role
          />
        </div>
      </div>

      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token')"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnly') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
            </p>
          </div>
          <InlineToggleSwitch v-model="codexCLIOnlyEnabled" />
        </div>
        <div
          v-if="codexCLIOnlyEnabled"
          class="mt-4 flex items-center justify-between border-l-2 border-line pl-4"
        >
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnlyAppServer') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.codexCLIOnlyAppServerDesc') }}
            </p>
          </div>
          <InlineToggleSwitch v-model="codexCLIOnlyAppServerEnabled" />
        </div>
      </div>

      <!-- Codex 指纹收敛模式（仅 OpenAI OAuth） -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth'"
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
            <Select v-model="codexFingerprintMode" data-testid="edit-codex-fingerprint-mode-select" :options="codexFingerprintModeOptions" />
          </div>
        </div>
      </div>

      <!-- OpenAI 订阅档位手动覆盖（Plus/Pro/Free），仅 OAuth 非影子账号 -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth' && !isSparkShadow"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.openai.planType') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.openai.planTypeDesc') }}
            </p>
          </div>
          <div class="w-44 flex-shrink-0">
            <Select v-model="editPlanType" :options="planTypeOptions" />
          </div>
        </div>
      </div>

      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token' || account?.type === 'apikey')"
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
            <Select v-model="openAICompactMode" :options="openAICompactModeOptions" />
          </div>
        </div>
        <div class="rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted">
          <span class="font-medium">{{ t(openAICompactStatusKey) }}</span>
          <span
            v-if="account?.extra?.openai_compact_checked_at"
            class="ml-2 text-muted"
          >
            {{ t('admin.accounts.openai.compactLastChecked') }}:
            {{ formatDateTime(new Date(String(account.extra.openai_compact_checked_at))) }}
          </span>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.openai.compactModelMapping') }}</label>
          <p class="input-hint">{{ t('admin.accounts.openai.compactModelMappingDesc') }}</p>
          <div v-if="openAICompactModelMappings.length > 0" class="mb-3 space-y-2">
            <div
              v-for="(mapping, index) in openAICompactModelMappings"
              :key="getOpenAICompactModelMappingKey(mapping)"
              class="flex items-center gap-2"
            >
              <input
                v-model="mapping.from"
                type="text"
                class="input flex-1"
                :placeholder="t('admin.accounts.fromModel')"
              />
              <span class="text-muted">→</span>
              <input
                v-model="mapping.to"
                type="text"
                class="input flex-1"
                :placeholder="t('admin.accounts.toModel')"
              />
              <button type="button" @click="removeOpenAICompactModelMapping(index)" class="text-danger-text hover:text-danger-text">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>
          <button type="button" @click="addOpenAICompactModelMapping" class="btn btn-secondary text-sm">
            + {{ t('admin.accounts.addMapping') }}
          </button>
        </div>
      </div>

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

      <div
        v-if="account?.platform === 'openai'"
        class="border-t border-line pt-4 space-y-4"
      >
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('admin.accounts.autoPause5hDisabled') }}</label>
            <InlineToggleSwitch v-model="autoPause5hDisabled" data-testid="auto-pause-5h-disabled" />
          </div>
          <p class="input-hint">{{ t('admin.accounts.autoPauseDisabledHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.autoPause5hThreshold') }}</label>
          <input
            v-model.number="autoPause5hThreshold"
            type="number"
            min="0"
            max="100"
            step="0.1"
            class="input"
            :disabled="autoPause5hDisabled"
            data-testid="auto-pause-5h-threshold"
          />
          <p class="input-hint">{{ t('admin.accounts.autoPauseThresholdHint') }}</p>
        </div>
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('admin.accounts.autoPause7dDisabled') }}</label>
            <InlineToggleSwitch v-model="autoPause7dDisabled" data-testid="auto-pause-7d-disabled" />
          </div>
          <p class="input-hint">{{ t('admin.accounts.autoPauseDisabledHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.autoPause7dThreshold') }}</label>
          <input
            v-model.number="autoPause7dThreshold"
            type="number"
            min="0"
            max="100"
            step="0.1"
            class="input"
            :disabled="autoPause7dDisabled"
            data-testid="auto-pause-7d-threshold"
          />
          <p class="input-hint">{{ t('admin.accounts.autoPauseThresholdHint') }}</p>
        </div>
      </div>

      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth' && !isSparkShadow"
        class="space-y-4 border-t border-line pt-4"
        data-testid="auto-reset-credit-settings"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.autoResetCredit.title') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.autoResetCredit.hint') }}
            </p>
          </div>
          <InlineToggleSwitch v-model="autoResetCreditEnabled" data-testid="auto-reset-credit-enabled" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.accounts.autoResetCredit.threshold5h') }}</label>
            <input
              v-model.number="autoResetCredit5hThreshold"
              type="number"
              min="0.1"
              max="100"
              step="0.1"
              class="input"
              :disabled="!autoResetCreditEnabled"
              data-testid="auto-reset-credit-5h-threshold"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.autoResetCredit.threshold7d') }}</label>
            <input
              v-model.number="autoResetCredit7dThreshold"
              type="number"
              min="0.1"
              max="100"
              step="0.1"
              class="input"
              :disabled="!autoResetCreditEnabled"
              data-testid="auto-reset-credit-7d-threshold"
            />
          </div>
        </div>
        <p class="input-hint">{{ t('admin.accounts.autoResetCredit.thresholdHint') }}</p>
      </div>

      <!-- 配额控制 (Anthropic OAuth/SetupToken: 亲和 + 窗口费用 + 会话 + RPM 等) -->
      <QuotaControlPanel
        v-if="account?.platform === 'anthropic' && (account?.type === 'oauth' || account?.type === 'setup-token')"
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
        v-if="supportsTLSFingerprint(account?.platform)"
        v-model:tls-fingerprint-enabled="tlsFingerprintEnabled"
        v-model:tls-fingerprint-profile-id="tlsFingerprintProfileId"
        v-model:tls-fingerprint-router-id="tlsFingerprintRouterId"
        v-model:tls-fingerprint-default-o-s="tlsFingerprintDefaultOS"
        v-model:tls-fingerprint-binding-rows="tlsFingerprintBindingRows"
        :tls-fingerprint-profiles="tlsFingerprintProfiles"
        :tls-fingerprint-routers="tlsFingerprintRouters"
      />

      <div
        v-if="!isSparkShadow"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.deviceLearning.label') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.deviceLearning.hint') }}
            </p>
          </div>
          <Toggle
            v-model="deviceLearningEnabled"
            data-testid="device-learning-toggle"
            :aria-label="t('admin.accounts.deviceLearning.label')"
          />
        </div>
        <div v-if="deviceLearningEnabled" class="mt-3 space-y-1">
          <label class="input-label" for="device-tls-profile-select">
            {{ t('admin.accounts.deviceLearning.tlsProfileLabel') }}
          </label>
          <select
            id="device-tls-profile-select"
            v-model="deviceTLSProfileSelection"
            class="input"
            data-testid="device-tls-profile-select"
          >
            <option value="">{{ t('admin.accounts.deviceLearning.tlsProfileAutomatic') }}</option>
            <option v-for="option in deviceTLSCatalogOptions" :key="option.id" :value="String(option.id)">
              {{ formatDeviceTLSProfileLabel(option) }}
            </option>
          </select>
          <p class="input-hint">{{ t('admin.accounts.deviceLearning.tlsProfileHint') }}</p>
        </div>
      </div>

      <div class="border-t border-line pt-4">
        <div>
          <label class="input-label">{{ t('common.status') }}</label>
          <!-- eslint-disable vue/no-mutating-props -- `form` is a reactive object shared with the
               host by design; this sets the object's own field, not the prop binding itself
               (mirrors the pre-split direct `form.status = ...` mutation). -->
          <Select v-model="form.status" :options="statusOptions" />
          <!-- eslint-enable vue/no-mutating-props -->
        </div>

        <!-- Mixed Scheduling (only for antigravity accounts, read-only in edit mode) -->
        <div v-if="account?.platform === 'antigravity'" class="flex items-center gap-2">
          <label class="flex cursor-not-allowed items-center gap-2 opacity-60">
            <input
              type="checkbox"
              v-model="mixedScheduling"
              disabled
              class="h-4 w-4 cursor-not-allowed rounded border-line text-accent focus:ring-accent"
            />
            <span class="text-sm font-medium text-foreground">
              {{ t('admin.accounts.mixedScheduling') }}
            </span>
          </label>
          <div class="group relative">
            <span
              class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-surface-3 text-xs text-muted hover:bg-surface-3"
            >
              ?
            </span>
            <!-- Tooltip（向下显示避免被弹窗裁剪） -->
            <div
              class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-[var(--code-bg)] px-3 py-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100"
            >
              {{ t('admin.accounts.mixedSchedulingTooltip') }}
              <div
                class="absolute bottom-full left-3 border-4 border-transparent border-b-[var(--code-bg)]"
              ></div>
            </div>
          </div>
        </div>
        <div v-if="account?.platform === 'antigravity'" class="mt-3 flex items-center gap-2">
          <label class="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              v-model="allowOverages"
              class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
            />
            <span class="text-sm font-medium text-foreground">
              {{ t('admin.accounts.allowOverages') }}
            </span>
          </label>
          <div class="group relative">
            <span
              class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-surface-3 text-xs text-muted hover:bg-surface-3"
            >
              ?
            </span>
            <div
              class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-[var(--code-bg)] px-3 py-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100"
            >
              {{ t('admin.accounts.allowOveragesTooltip') }}
              <div
                class="absolute bottom-full left-3 border-4 border-transparent border-b-[var(--code-bg)]"
              ></div>
            </div>
          </div>
        </div>
      </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'
import QuotaControlPanel from '@/components/account/shared/QuotaControlPanel.vue'
import TlsFingerprintPanel from '@/components/account/shared/TlsFingerprintPanel.vue'
import QuotaLimitCardSection from '@/components/account/shared/QuotaLimitCardSection.vue'
import OllamaCloudUsageSettings from '@/components/account/OllamaCloudUsageSettings.vue'
import { formatDateTime } from '@/utils/format'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'
import type { DeviceTLSCatalogOption } from '@/api/admin/tlsFingerprintProfile'
import type { OpenAIWSMode } from '@/utils/openaiWsMode'
import type {
  Account,
  OpenAICompactMode,
  OpenAIResponsesMode,
  OpenAIEndpointCapability,
  OllamaCloudUsageState
} from '@/types'

interface ModelMapping {
  from: string
  to: string
}

type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
type CodexImageToolMode = 'inherit' | 'enabled' | 'disabled' | 'block'
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'

interface Props {
  account: Account | null
  isSparkShadow: boolean
  hideAccountLongContextBilling: boolean
  webSearchGlobalEnabled: boolean
  quotaNotifyGlobalEnabled: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  quotaNotifyState: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  form: any
  codexImageToolBadgeClass: string
  codexImageToolBadgeLabel: string
  codexImageToolOptions: Array<{
    value: CodexImageToolMode
    label: string
    description: string
    selectedCardClass: string
    selectedDotClass: string
  }>
  openAIWSModeConcurrencyHintKey: string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  openAIWSModeOptions: any[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  openAIResponsesModeOptions: any[]
  openAITextGenerationCapabilityEnabled: boolean
  openAIResponsesStatusKey: string
  openAIEndpointCapabilityOptions: { value: OpenAIEndpointCapability; label: string }[]
  openAIEndpointCapabilities: OpenAIEndpointCapability[]
  toggleOpenAIEndpointCapability: (capability: OpenAIEndpointCapability, event?: Event) => void
  upstreamBillingAutoProbeEnabled: boolean
  handleUpstreamBillingAutoProbeChange: (enabled: boolean) => void
  handleOllamaCloudUsageUpdated: (state: OllamaCloudUsageState) => void
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  codexFingerprintModeOptions: any[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  planTypeOptions: any[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  openAICompactModeOptions: any[]
  openAICompactStatusKey: string
  supportsTLSFingerprint: (platform?: string | null) => boolean
  tlsFingerprintProfiles: { id: number; name: string }[]
  tlsFingerprintRouters: { id: number; name: string }[]
  deviceTLSCatalogOptions: DeviceTLSCatalogOption[]
  formatDeviceTLSProfileLabel: (option: DeviceTLSCatalogOption) => string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  statusOptions: any[]
}

defineProps<Props>()

const openaiPassthroughEnabled = defineModel<boolean>('openaiPassthroughEnabled', { required: true })
const openaiFlattenNamespacesEnabled = defineModel<boolean>('openaiFlattenNamespacesEnabled', { required: true })
const codexImageToolMode = defineModel<CodexImageToolMode>('codexImageToolMode', { required: true })
const openaiResponsesWebSocketV2Mode = defineModel<OpenAIWSMode>('openaiResponsesWebSocketV2Mode', { required: true })
const openAIResponsesMode = defineModel<OpenAIResponsesMode>('openAIResponsesMode', { required: true })
const anthropicPassthroughEnabled = defineModel<boolean>('anthropicPassthroughEnabled', { required: true })
const anthropicAPIKeyAuthScheme = defineModel<AnthropicAPIKeyAuthScheme>('anthropicAPIKeyAuthScheme', { required: true })
const webSearchEmulationMode = defineModel<string>('webSearchEmulationMode', { required: true })
const editQuotaLimit = defineModel<number | null>('editQuotaLimit', { required: true })
const editQuotaDailyLimit = defineModel<number | null>('editQuotaDailyLimit', { required: true })
const editQuotaWeeklyLimit = defineModel<number | null>('editQuotaWeeklyLimit', { required: true })
const editDailyResetMode = defineModel<'rolling' | 'fixed' | null>('editDailyResetMode', { required: true })
const editDailyResetHour = defineModel<number | null>('editDailyResetHour', { required: true })
const editWeeklyResetMode = defineModel<'rolling' | 'fixed' | null>('editWeeklyResetMode', { required: true })
const editWeeklyResetDay = defineModel<number | null>('editWeeklyResetDay', { required: true })
const editWeeklyResetHour = defineModel<number | null>('editWeeklyResetHour', { required: true })
const editResetTimezone = defineModel<string | null>('editResetTimezone', { required: true })
const openAILongContextBillingEnabled = defineModel<boolean>('openAILongContextBillingEnabled', { required: true })
const codexCLIOnlyEnabled = defineModel<boolean>('codexCLIOnlyEnabled', { required: true })
const codexCLIOnlyAppServerEnabled = defineModel<boolean>('codexCLIOnlyAppServerEnabled', { required: true })
const codexFingerprintMode = defineModel<CodexFingerprintMode>('codexFingerprintMode', { required: true })
const editPlanType = defineModel<string>('editPlanType', { required: true })
const openAICompactMode = defineModel<OpenAICompactMode>('openAICompactMode', { required: true })
const openAICompactModelMappings = defineModel<ModelMapping[]>('openAICompactModelMappings', { required: true })
const autoPauseOnExpired = defineModel<boolean>('autoPauseOnExpired', { required: true })
const autoPause5hDisabled = defineModel<boolean>('autoPause5hDisabled', { required: true })
const autoPause5hThreshold = defineModel<number | null>('autoPause5hThreshold', { required: true })
const autoPause7dDisabled = defineModel<boolean>('autoPause7dDisabled', { required: true })
const autoPause7dThreshold = defineModel<number | null>('autoPause7dThreshold', { required: true })
const autoResetCreditEnabled = defineModel<boolean>('autoResetCreditEnabled', { required: true })
const autoResetCredit5hThreshold = defineModel<number>('autoResetCredit5hThreshold', { required: true })
const autoResetCredit7dThreshold = defineModel<number>('autoResetCredit7dThreshold', { required: true })
const windowCostEnabled = defineModel<boolean>('windowCostEnabled', { required: true })
const windowCostLimit = defineModel<number | null>('windowCostLimit', { required: true })
const windowCostStickyReserve = defineModel<number | null>('windowCostStickyReserve', { required: true })
const sessionLimitEnabled = defineModel<boolean>('sessionLimitEnabled', { required: true })
const maxSessions = defineModel<number | null>('maxSessions', { required: true })
const sessionIdleTimeout = defineModel<number | null>('sessionIdleTimeout', { required: true })
const rpmLimitEnabled = defineModel<boolean>('rpmLimitEnabled', { required: true })
const baseRpm = defineModel<number | null>('baseRpm', { required: true })
const rpmStrategy = defineModel<'tiered' | 'sticky_exempt'>('rpmStrategy', { required: true })
const rpmStickyBuffer = defineModel<number | null>('rpmStickyBuffer', { required: true })
const userMsgQueueMode = defineModel<string>('userMsgQueueMode', { required: true })
const sessionIdMaskingEnabled = defineModel<boolean>('sessionIdMaskingEnabled', { required: true })
const cacheTTLOverrideEnabled = defineModel<boolean>('cacheTTLOverrideEnabled', { required: true })
const cacheTTLOverrideTarget = defineModel<string>('cacheTTLOverrideTarget', { required: true })
const customBaseUrlEnabled = defineModel<boolean>('customBaseUrlEnabled', { required: true })
const customBaseUrl = defineModel<string>('customBaseUrl', { required: true })
const tlsFingerprintEnabled = defineModel<boolean>('tlsFingerprintEnabled', { required: true })
const tlsFingerprintProfileId = defineModel<number | null>('tlsFingerprintProfileId', { required: true })
const tlsFingerprintRouterId = defineModel<number | null>('tlsFingerprintRouterId', { required: true })
const tlsFingerprintDefaultOS = defineModel<string>('tlsFingerprintDefaultOS', { required: true })
const tlsFingerprintBindingRows = defineModel<{ os: string; client: string; protocol: string; profileId: number }[]>('tlsFingerprintBindingRows', { required: true })
const deviceLearningEnabled = defineModel<boolean>('deviceLearningEnabled', { required: true })
const deviceTLSProfileSelection = defineModel<string>('deviceTLSProfileSelection', { required: true })
const mixedScheduling = defineModel<boolean>('mixedScheduling', { required: true })
const allowOverages = defineModel<boolean>('allowOverages', { required: true })

const { t } = useI18n()

const getOpenAICompactModelMappingKey = createStableObjectKeyResolver<ModelMapping>('edit-openai-compact-model-mapping')

const addOpenAICompactModelMapping = () => {
  openAICompactModelMappings.value.push({ from: '', to: '' })
}

const removeOpenAICompactModelMapping = (index: number) => {
  openAICompactModelMappings.value.splice(index, 1)
}
</script>
