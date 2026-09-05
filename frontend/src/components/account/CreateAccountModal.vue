<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.createAccount')"
    width="wide"
    @close="handleClose"
  >
    <!-- Step Indicator for OAuth accounts -->
    <div v-if="isOAuthFlow" class="mb-6 flex items-center justify-center">
      <div class="flex items-center space-x-4">
        <div class="flex items-center">
          <div
            :class="[
 'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
 step >= 1 ? 'bg-accent text-white' : 'bg-surface-3 text-muted'
 ]"
          >
            1
          </div>
          <span class="ml-2 text-sm font-medium text-foreground">{{
            t('admin.accounts.oauth.authMethod')
          }}</span>
        </div>
        <div class="h-0.5 w-8 bg-surface-3" />
        <div class="flex items-center">
          <div
            :class="[
 'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
 step >= 2 ? 'bg-accent text-white' : 'bg-surface-3 text-muted'
 ]"
          >
            2
          </div>
          <span class="ml-2 text-sm font-medium text-foreground">{{
            oauthStepTitle
          }}</span>
        </div>
      </div>
    </div>

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

      <!-- Account Mode Selection (Kimi / Zhipu / DeepSeek) -->
      <div v-if="isCNPlatform">
        <label class="input-label">{{ t('admin.accounts.cnProviders.accountMode.title') }}</label>
        <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2" data-tour="account-form-mode">
          <!-- Pay-as-you-go (token balance) -->
          <button
            type="button"
            @click="accountMode = 'payg'"
            :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountMode === 'payg'
 ? cnAccentActiveClass
 : 'border-line hover:border-line'
 ]"
          >
            <div
              :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountMode === 'payg'
 ? cnAccentIconClass
 : 'bg-surface-2 text-muted'
 ]"
            >
              <Icon name="creditCard" size="sm" />
            </div>
            <div>
              <span class="block text-sm font-medium text-foreground">{{ t('admin.accounts.cnProviders.accountMode.payg') }}</span>
              <span class="text-xs text-muted">{{ t('admin.accounts.cnProviders.accountMode.paygDesc') }}</span>
            </div>
          </button>
          <!-- Coding Plan (kimi / zhipu only — DeepSeek has no coding plan) -->
          <button
            v-if="form.platform !== 'deepseek'"
            type="button"
            @click="accountMode = 'coding'"
            :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountMode === 'coding'
 ? cnAccentActiveClass
 : 'border-line hover:border-line'
 ]"
          >
            <div
              :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountMode === 'coding'
 ? cnAccentIconClass
 : 'bg-surface-2 text-muted'
 ]"
            >
              <Icon name="bolt" size="sm" />
            </div>
            <div>
              <span class="block text-sm font-medium text-foreground">{{ t('admin.accounts.cnProviders.accountMode.coding') }}</span>
              <span class="text-xs text-muted">{{ t('admin.accounts.cnProviders.accountMode.codingDesc') }}</span>
            </div>
          </button>
        </div>
      </div>

      <!-- API Protocol Selection (Kimi / Zhipu / DeepSeek) -->
      <div v-if="isCNPlatform" class="mt-4">
        <label class="input-label">{{ t('admin.accounts.cnProviders.apiProtocol.title') }}</label>
        <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-3">
          <button
            v-for="opt in cnProtocolOptions"
            :key="opt.value"
            type="button"
            @click="apiProtocol = opt.value"
            :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 apiProtocol === opt.value
 ? cnAccentActiveClass
 : 'border-line hover:border-line'
 ]"
          >
            <div
              :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 apiProtocol === opt.value
 ? cnAccentIconClass
 : 'bg-surface-2 text-muted'
 ]"
            >
              <Icon :name="opt.value === 'adaptive' ? 'swap' : opt.value === 'anthropic' ? 'sparkles' : opt.value === 'responses' ? 'terminal' : 'chat'" size="sm" />
            </div>
            <div>
              <span class="block text-sm font-medium text-foreground">{{ t(`admin.accounts.cnProviders.apiProtocol.${opt.labelKey}`) }}</span>
              <span class="text-xs text-muted">{{ t(`admin.accounts.cnProviders.apiProtocol.${opt.labelKey}Desc`) }}</span>
            </div>
          </button>
        </div>
      </div>

      <!-- Zhipu 团队版 Coding Plan：组织/项目 ID（可选，填写后额度探测走团队版端点） -->
      <div v-if="form.platform === 'zhipu' && accountMode === 'coding'" class="mt-4">
        <div class="flex items-center">
          <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.title') }}</label>
          <HelpTooltip trigger="click" width-class="w-80">
            <p class="mb-1 font-medium">{{ t('admin.accounts.cnProviders.zhipuTeam.help.title') }}</p>
            <ol class="list-decimal space-y-1 pl-4">
              <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step1') }}</li>
              <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step2') }}</li>
              <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step3') }}</li>
              <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step4') }}</li>
            </ol>
            <p class="mt-2 break-all rounded bg-black/20 p-1.5 font-mono text-[11px] leading-relaxed">
              {{ t('admin.accounts.cnProviders.zhipuTeam.help.example') }}
            </p>
          </HelpTooltip>
        </div>
        <div class="mt-2 grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.organization') }}</label>
            <input v-model="zhipuOrganization" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.organizationPlaceholder')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.project') }}</label>
            <input v-model="zhipuProject" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.projectPlaceholder')" />
          </div>
        </div>
        <p class="input-hint mt-2">{{ t('admin.accounts.cnProviders.zhipuTeam.hint') }}</p>
      </div>

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

      <!-- Add Method (only for Anthropic OAuth-based type) -->
      <div v-if="form.platform === 'anthropic' && isOAuthFlow">
        <label class="input-label">{{ t('admin.accounts.addMethod') }}</label>
        <div class="mt-2 flex gap-4">
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="oauth"
              class="mr-2 text-accent focus:ring-accent"
            />
            <span class="text-sm text-foreground">{{ t('admin.accounts.types.oauth') }}</span>
          </label>
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="setup-token"
              class="mr-2 text-accent focus:ring-accent"
            />
            <span class="text-sm text-foreground">{{
              t('admin.accounts.setupTokenLongLived')
            }}</span>
          </label>
        </div>
      </div>

      <!-- API Key input (only for apikey type, excluding Antigravity which has its own fields) -->
      <div v-if="form.type === 'apikey' && form.platform !== 'antigravity' && form.platform !== 'kiro'" class="space-y-4">
        <div v-if="!isCNPlatform || apiProtocol !== 'adaptive'">
          <label class="input-label">{{ t('admin.accounts.baseUrl') }}</label>
          <input
            v-model="apiKeyBaseUrl"
            type="text"
            class="input"
            :placeholder="apiKeyBaseUrlPlaceholder"
          />
          <p v-if="baseUrlHint" class="input-hint">{{ baseUrlHint }}</p>
          <GrokBaseUrlPresets
            v-if="form.platform === 'grok'"
            class="mt-2"
            @select="apiKeyBaseUrl = $event"
          />
          <CnBaseUrlPresets
            v-if="isCNPlatform"
            class="mt-2"
            :platform="cnPresetPlatform"
            :mode="accountMode"
            :protocol="apiProtocol"
            :current-url="apiKeyBaseUrl"
            @select="onCnPresetSelect"
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
          <p v-if="!cnSupportsNativeResponses(form.platform)" class="input-hint">
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
        <div v-if="form.platform === 'gemini'">
          <label class="input-label">{{ t('admin.accounts.gemini.tier.label') }}</label>
          <select v-model="geminiTierAIStudio" class="input">
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
            :platform="form.platform"
            :sync-credentials="syncPreviewCredentials"
            :disabled-by-passthrough="isOpenAIModelRestrictionDisabled"
            :show-icons="true"
            :presets="presetMappings"
            @upstream-synced="upstreamModelsPreviewed = true"
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
          v-if="isHeaderOverrideCapable(form.platform, 'apikey')"
          v-model:headerOverrideEnabled="headerOverrideEnabled"
          v-model:headerOverrideRows="headerOverrideRows"
        />

      </div>

      <!-- Bedrock credentials (only for Anthropic Bedrock type) -->
      <div v-if="form.platform === 'anthropic' && accountCategory === 'bedrock'" class="space-y-4">
        <!-- Auth Mode Radio -->
        <div>
          <label class="input-label">{{ t('admin.accounts.bedrockAuthMode') }}</label>
          <div class="mt-2 flex gap-4">
            <label class="flex cursor-pointer items-center">
              <input
                v-model="bedrockAuthMode"
                type="radio"
                value="sigv4"
                class="mr-2 text-accent focus:ring-accent"
              />
              <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockAuthModeSigv4') }}</span>
            </label>
            <label class="flex cursor-pointer items-center">
              <input
                v-model="bedrockAuthMode"
                type="radio"
                value="apikey"
                class="mr-2 text-accent focus:ring-accent"
              />
              <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockAuthModeApikey') }}</span>
            </label>
          </div>
        </div>

        <!-- SigV4 fields -->
        <template v-if="bedrockAuthMode === 'sigv4'">
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
            <input
              v-model="bedrockAccessKeyId"
              type="text"
              required
              class="input font-mono"
              placeholder="AKIA..."
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
            <input
              v-model="bedrockSecretAccessKey"
              type="password"
              required
              class="input font-mono"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSessionToken') }}</label>
            <input
              v-model="bedrockSessionToken"
              type="password"
              class="input font-mono"
            />
            <p class="input-hint">{{ t('admin.accounts.bedrockSessionTokenHint') }}</p>
          </div>
        </template>

        <!-- API Key field -->
        <div v-if="bedrockAuthMode === 'apikey'">
          <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
          <input
            v-model="bedrockApiKeyValue"
            type="password"
            required
            class="input font-mono"
          />
        </div>

        <!-- Shared: Region -->
        <div>
          <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
          <select v-model="bedrockRegion" class="input">
            <optgroup label="US">
              <option value="us-east-1">us-east-1 (N. Virginia)</option>
              <option value="us-east-2">us-east-2 (Ohio)</option>
              <option value="us-west-1">us-west-1 (N. California)</option>
              <option value="us-west-2">us-west-2 (Oregon)</option>
              <option value="us-gov-east-1">us-gov-east-1 (GovCloud US-East)</option>
              <option value="us-gov-west-1">us-gov-west-1 (GovCloud US-West)</option>
            </optgroup>
            <optgroup label="Europe">
              <option value="eu-west-1">eu-west-1 (Ireland)</option>
              <option value="eu-west-2">eu-west-2 (London)</option>
              <option value="eu-west-3">eu-west-3 (Paris)</option>
              <option value="eu-central-1">eu-central-1 (Frankfurt)</option>
              <option value="eu-central-2">eu-central-2 (Zurich)</option>
              <option value="eu-south-1">eu-south-1 (Milan)</option>
              <option value="eu-south-2">eu-south-2 (Spain)</option>
              <option value="eu-north-1">eu-north-1 (Stockholm)</option>
            </optgroup>
            <optgroup label="Asia Pacific">
              <option value="ap-northeast-1">ap-northeast-1 (Tokyo)</option>
              <option value="ap-northeast-2">ap-northeast-2 (Seoul)</option>
              <option value="ap-northeast-3">ap-northeast-3 (Osaka)</option>
              <option value="ap-south-1">ap-south-1 (Mumbai)</option>
              <option value="ap-south-2">ap-south-2 (Hyderabad)</option>
              <option value="ap-southeast-1">ap-southeast-1 (Singapore)</option>
              <option value="ap-southeast-2">ap-southeast-2 (Sydney)</option>
            </optgroup>
            <optgroup label="Canada">
              <option value="ca-central-1">ca-central-1 (Canada)</option>
            </optgroup>
            <optgroup label="South America">
              <option value="sa-east-1">sa-east-1 (São Paulo)</option>
            </optgroup>
          </select>
          <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
        </div>

        <!-- Shared: Force Global -->
        <div>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="bedrockForceGlobal"
              type="checkbox"
              class="rounded border-line text-accent focus:ring-accent"
            />
            <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
          </label>
          <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
        </div>

        <!-- Model Restriction Section for Bedrock -->
        <div class="border-t border-line pt-4">
          <ModelRestrictionEditor
            v-model:mode="modelRestrictionMode"
            v-model:allowed-models="allowedModels"
            v-model:model-mappings="modelMappings"
            platform="anthropic"
            :sync-credentials="syncPreviewCredentials"
            :presets="bedrockPresets"
            @upstream-synced="upstreamModelsPreviewed = true"
          />
        </div>

        <!-- Pool Mode Section for Bedrock -->
        <PoolModeSection
          v-model:pool-mode-enabled="poolModeEnabled"
          v-model:pool-mode-retry-count="poolModeRetryCount"
          v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
        />
      </div>

      <!-- 配额控制 (Anthropic apikey/bedrock: 配额限制 + 亲和) -->
      <QuotaLimitCardSection
        v-if="form.platform === 'anthropic' && (form.type === 'apikey' || form.type === 'bedrock')"
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
        v-else-if="form.type === 'apikey' || form.type === 'bedrock'"
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

      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
          <input v-model.number="form.concurrency" type="number" min="1" class="input"
            @input="form.concurrency = Math.max(1, form.concurrency || 1)" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.loadFactor') }}</label>
          <input v-model.number="form.load_factor" type="number" min="1"
            class="input" :placeholder="String(form.concurrency || 1)"
            @input="form.load_factor = (form.load_factor &amp;&amp; form.load_factor >= 1) ? form.load_factor : null" />
          <p class="input-hint">{{ t('admin.accounts.loadFactorHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.priority') }}</label>
          <input
            v-model.number="form.priority"
            type="number"
            min="1"
            class="input"
            data-tour="account-form-priority"
          />
          <p class="input-hint">{{ t('admin.accounts.priorityHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.billingRateMultiplier') }}</label>
          <input v-model.number="form.rate_multiplier" type="number" min="0" step="0.001" class="input" />
          <p class="input-hint">{{ t('admin.accounts.billingRateMultiplierHint') }}</p>
        </div>
      </div>
      <div class="border-t border-line pt-4">
        <label class="input-label">{{ t('admin.accounts.expiresAt') }}</label>
        <input v-model="expiresAtInput" type="datetime-local" class="input" />
        <p class="input-hint">
          {{ t('admin.accounts.expiresAtHint') }}
          {{ t('admin.accounts.expiresAtTimezoneHint', { timezone: browserTimeZone }) }}
        </p>
      </div>

      <!-- OpenAI 自动透传开关（OAuth/API Key） -->
      <div
        v-if="form.platform === 'openai'"
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
        v-if="form.platform === 'openai' && form.type === 'oauth'"
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
        v-if="form.platform === 'openai' && (accountCategory === 'oauth-based' || accountCategory === 'apikey')"
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
              {{ t(openAIWSModeConcurrencyHintKey) }}
            </p>
          </div>
          <div class="w-52">
            <Select v-model="openaiResponsesWebSocketV2Mode" :options="openAIWSModeOptions" />
          </div>
        </div>
      </div>

      <!-- Anthropic API Key 自动透传开关 -->
      <div
        v-if="form.platform === 'anthropic' && accountCategory === 'apikey'"
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
        v-if="form.platform === 'anthropic' && accountCategory === 'apikey'"
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
        v-if="form.platform === 'anthropic' && accountCategory === 'apikey' && webSearchGlobalEnabled"
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
        v-if="form.platform === 'openai' && !hideAccountLongContextBilling && (accountCategory === 'oauth-based' || accountCategory === 'apikey')"
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
            :model-value="openAILongContextBillingEnabled"
            data-testid="openai-long-context-billing-toggle"
            switch-role
            :auto-toggle="false"
            @click="toggleOpenAILongContextBilling"
          />
        </div>
      </div>

      <div
        v-if="form.platform === 'openai' && accountCategory === 'oauth-based'"
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
        v-if="form.platform === 'openai' && accountCategory === 'oauth-based'"
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
        v-if="form.platform === 'openai' && (accountCategory === 'oauth-based' || accountCategory === 'apikey')"
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
        <div>
          <label class="input-label">{{ t('admin.accounts.openai.compactModelMapping') }}</label>
          <p class="input-hint">{{ t('admin.accounts.openai.compactModelMappingDesc') }}</p>
          <div v-if="openAICompactModelMappings.length > 0" class="mb-3 space-y-2">
            <div
              v-for="(mapping, index) in openAICompactModelMappings"
              :key="getOpenAICompactModelMappingKey(mapping)"
              class="flex items-center gap-2"
            >
              <input v-model="mapping.from" type="text" class="input flex-1" :placeholder="t('admin.accounts.fromModel')" />
              <span class="text-muted">→</span>
              <input v-model="mapping.to" type="text" class="input flex-1" :placeholder="t('admin.accounts.toModel')" />
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

      <!-- OpenAI APIKey Responses API support mode -->
      <div
        v-if="form.platform === 'openai' && accountCategory === 'apikey'"
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
        <p
          v-if="!openAITextGenerationCapabilityEnabled"
          class="rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-xs text-warning-text"
          data-testid="openai-responses-mode-not-applicable"
        >
          {{ t('admin.accounts.openai.responsesModeTextDisabledHint') }}
        </p>
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
        <!-- Mixed Scheduling (only for antigravity accounts) -->
        <div v-if="form.platform === 'antigravity'" class="flex items-center gap-2">
          <label class="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              v-model="mixedScheduling"
              class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
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
        <div v-if="form.platform === 'antigravity'" class="mt-3 flex items-center gap-2">
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
      <div v-if="step === 1" class="flex justify-end gap-3">
        <button @click="handleClose" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="create-account-form"
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
          {{
            isOAuthFlow
              ? t('common.next')
              : submitting
                ? t('admin.accounts.creating')
                : t('common.create')
          }}
        </button>
      </div>
      <div v-else class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="goBackToBasicInfo">
          {{ t('common.back') }}
        </button>
        <button
          v-if="form.platform !== 'kiro' && isManualInputMethod"
          type="button"
          :disabled="!canExchangeCode"
          class="btn btn-primary"
          @click="handleExchangeCode"
        >
          <svg
            v-if="currentOAuthLoading"
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
          {{
            currentOAuthLoading
              ? t('admin.accounts.oauth.verifying')
              : t('admin.accounts.oauth.completeAuth')
          }}
        </button>
      </div>
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
  claudeModels,
  getPresetMappingsByPlatform,
  getModelsByPlatform,
  buildModelMappingObject,
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
import type { KiroTokenInfo } from '@/api/admin/kiro'
import type {
  Proxy,
  AdminGroup,
  AccountPlatform,
  AccountType,
  CheckMixedChannelResponse,
  CreateAccountRequest,
  CodexSessionImportMessage,
  OpenAICompactMode,
  OpenAIResponsesMode,
  OpenAIEndpointCapability
} from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'
import QuotaControlPanel from '@/components/account/shared/QuotaControlPanel.vue'
import TlsFingerprintPanel from '@/components/account/shared/TlsFingerprintPanel.vue'
import PoolModeSection from '@/components/account/shared/PoolModeSection.vue'
import CustomErrorCodesSection from '@/components/account/shared/CustomErrorCodesSection.vue'
import QuotaLimitCardSection from '@/components/account/shared/QuotaLimitCardSection.vue'
import HeaderOverrideSection from '@/components/account/shared/HeaderOverrideSection.vue'
import GrokCustomBaseUrlSection from '@/components/account/shared/GrokCustomBaseUrlSection.vue'
import GeminiHelpDialog from '@/components/account/create/GeminiHelpDialog.vue'
import PlatformSelector from '@/components/account/create/PlatformSelector.vue'
import { useTempUnschedRules } from '@/components/account/shared/useTempUnschedRules'
import TempUnschedulableRulesSection from '@/components/account/shared/TempUnschedulableRulesSection.vue'
import InterceptWarmupRequestsSection from '@/components/account/shared/InterceptWarmupRequestsSection.vue'
const GrokPanel = defineAsyncComponent(() => import('@/components/account/platform/GrokPanel.vue'))
import ProxySelector from '@/components/common/ProxySelector.vue'
import ProxyAdBanner from '@/components/common/ProxyAdBanner.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
import OpenAIPanel from '@/components/account/platform/OpenAIPanel.vue'
import KiroPanel from '@/components/account/platform/KiroPanel.vue'
import AnthropicPanel from '@/components/account/platform/AnthropicPanel.vue'
import GeminiPanel from '@/components/account/platform/GeminiPanel.vue'
import AntigravityPanel from '@/components/account/platform/AntigravityPanel.vue'
import VertexServiceAccountPanel from '@/components/account/platform/VertexServiceAccountPanel.vue'
import Toggle from '@/components/common/Toggle.vue'
import GrokBaseUrlPresets from '@/components/account/GrokBaseUrlPresets.vue'
import CnBaseUrlPresets from '@/components/account/CnBaseUrlPresets.vue'
import { allSelectedGroupsEnableLongContextPricing } from '@/components/account/longContextBilling'
import {
  applyAntigravityProjectID,
  applyHeaderOverride,
  applyInterceptWarmup,
  cnSupportsNativeResponses,
  defaultCNAdaptiveBaseUrls,
  defaultCNBaseUrl,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows,
  type CnAccountMode,
  type CnApiProtocol,
  type CnNativeApiProtocol,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import {
  formatDateTimeLocalInput,
  getBrowserTimeZone,
  parseDateTimeLocalInput
} from '@/utils/format'
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
import OAuthAuthorizationFlow from './OAuthAuthorizationFlow.vue'
import KiroAuthorizationFlow from './KiroAuthorizationFlow.vue'

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

const oauthStepTitle = computed(() => {
  if (form.platform === 'openai') return t('admin.accounts.oauth.openai.title')
  if (form.platform === 'gemini') return t('admin.accounts.oauth.gemini.title')
  if (form.platform === 'antigravity') return t('admin.accounts.oauth.antigravity.title')
  if (form.platform === 'grok') return t('admin.accounts.oauth.grok.title')
  if (form.platform === 'kiro') return t('admin.accounts.kiro.authorizationTitle')
  return t('admin.accounts.oauth.title')
})

// Platform-specific hints for API Key type
const baseUrlHint = computed(() => {
  if (form.platform === 'openai') return t('admin.accounts.openai.baseUrlHint')
  if (form.platform === 'gemini') return t('admin.accounts.gemini.baseUrlHint')
  if (form.platform === 'grok') return ''
  return t('admin.accounts.baseUrlHint')
})

const apiKeyHint = computed(() => {
  if (form.platform === 'openai') return t('admin.accounts.openai.apiKeyHint')
  if (form.platform === 'gemini') return t('admin.accounts.gemini.apiKeyHint')
  if (form.platform === 'grok') return ''
  return t('admin.accounts.apiKeyHint')
})

// Base URL / API Key 占位符：国产供应商随账号类型变化。
const apiKeyBaseUrlPlaceholder = computed(() => {
  if (isCNPlatform.value) {
    return defaultCNBaseUrl(form.platform, accountMode.value, apiProtocol.value) || 'https://api.example.com'
  }
  switch (form.platform) {
    case 'openai':
      return 'https://api.openai.com'
    case 'gemini':
      return 'https://generativelanguage.googleapis.com'
    case 'grok':
      return 'https://api.x.ai/v1'
    default:
      return 'https://api.anthropic.com'
  }
})

const apiKeyValuePlaceholder = computed(() => {
  switch (form.platform) {
    case 'openai':
      return 'sk-proj-...'
    case 'gemini':
      return 'AIza...'
    case 'grok':
      return 'xai-...'
    case 'kimi':
      return 'sk-...'
    case 'zhipu':
      return '<api-key>.<secret>'
    case 'deepseek':
      return 'sk-...'
    default:
      return 'sk-ant-...'
  }
})

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
const isCNPlatform = computed(
  () => form.platform === 'kimi' || form.platform === 'zhipu' || form.platform === 'deepseek'
)
// CnBaseUrlPresets 的 platform prop 是平台字面量联合类型，模板里不能写
// `as` 断言（其中的 `|` 会被 eslint 误判为 Vue2 filter 语法），经此 computed 传递。
const cnPresetPlatform = computed<'kimi' | 'zhipu' | 'deepseek'>(() => {
  if (form.platform === 'kimi' || form.platform === 'zhipu' || form.platform === 'deepseek') {
    return form.platform
  }
  return 'kimi'
})
// 当前平台可选的协议档（responses 仅 deepseek / kimi）。
const cnProtocolOptions = computed<Array<{ value: CnApiProtocol; labelKey: string }>>(() => {
  const opts: Array<{ value: CnApiProtocol; labelKey: string }> = [
    { value: 'adaptive', labelKey: 'adaptive' },
    { value: 'chat_completions', labelKey: 'chatCompletions' },
    { value: 'anthropic', labelKey: 'anthropic' }
  ]
  if (cnSupportsNativeResponses(form.platform)) {
    opts.push({ value: 'responses', labelKey: 'responses' })
  }
  return opts
})
const cnAdaptiveProtocolOptions = computed<Array<{ value: CnNativeApiProtocol; labelKey: string }>>(() => {
  const opts: Array<{ value: CnNativeApiProtocol; labelKey: string }> = [
    { value: 'chat_completions', labelKey: 'chatCompletions' },
    { value: 'anthropic', labelKey: 'anthropic' }
  ]
  if (cnSupportsNativeResponses(form.platform)) opts.push({ value: 'responses', labelKey: 'responses' })
  return opts
})

function resetAdaptiveBaseUrls(platform: 'kimi' | 'zhipu' | 'deepseek', mode: CnAccountMode) {
  adaptiveBaseUrls.value = defaultCNAdaptiveBaseUrls(platform, mode)
}
// 当前选中平台的品牌色（选中卡片描边 / 图标底色），与 platformColors 取色一致。
const cnAccentActiveClass = computed(() => {
  switch (form.platform) {
    case 'kimi':
      return 'border-accent-500 bg-accent-50'
    case 'zhipu':
      return 'border-accent-500 bg-accent-50'
    case 'deepseek':
      return 'border-success-500 bg-success-50'
    default:
      return 'border-accent bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]'
  }
})
const cnAccentIconClass = computed(() => {
  switch (form.platform) {
    case 'kimi':
      return 'bg-accent-500 text-white'
    case 'zhipu':
      return 'bg-accent-500 text-white'
    case 'deepseek':
      return 'bg-success-500 text-white'
    default:
      return 'bg-accent text-white'
  }
})
// 切换国产供应商平台：强制 apikey 类型，deepseek 无 coding 套餐故锁定 payg，
// 协议回落 adaptive，并把 base url 重置为该平台默认端点。
function selectCNPlatform(platform: 'kimi' | 'zhipu' | 'deepseek') {
  form.platform = platform
  form.type = 'apikey'
  accountCategory.value = 'apikey'
  apiProtocol.value = 'adaptive'
  if (platform === 'deepseek') {
    accountMode.value = 'payg'
  }
  apiKeyBaseUrl.value = defaultCNBaseUrl(platform, accountMode.value, apiProtocol.value)
  resetAdaptiveBaseUrls(platform, accountMode.value)
}
// 账号类型 / 协议变更时同步默认 base url。
watch(accountMode, (mode, previousMode) => {
  if (!isCNPlatform.value) return
  if (apiProtocol.value === 'adaptive') {
    const previousDefaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, previousMode)
    const nextDefaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, mode)
    for (const item of cnAdaptiveProtocolOptions.value) {
      if (!adaptiveBaseUrls.value[item.value] || adaptiveBaseUrls.value[item.value] === previousDefaults[item.value]) {
        adaptiveBaseUrls.value[item.value] = nextDefaults[item.value]
      }
    }
    apiKeyBaseUrl.value = adaptiveBaseUrls.value.chat_completions
    return
  }
  apiKeyBaseUrl.value = defaultCNBaseUrl(form.platform, mode, apiProtocol.value)
})
watch(apiProtocol, (protocol) => {
  if (!isCNPlatform.value) return
  if (protocol === 'adaptive') {
    const defaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, accountMode.value)
    for (const item of cnAdaptiveProtocolOptions.value) {
      if (!adaptiveBaseUrls.value[item.value]) adaptiveBaseUrls.value[item.value] = defaults[item.value]
    }
    apiKeyBaseUrl.value = adaptiveBaseUrls.value.chat_completions
    return
  }
  apiKeyBaseUrl.value = defaultCNBaseUrl(form.platform, accountMode.value, protocol)
})
// 点击预设端点：同时回填 base url、账号类型与协议。
function onCnPresetSelect(preset: { mode: CnAccountMode; protocol: CnApiProtocol; url: string }) {
  accountMode.value = preset.mode
  apiProtocol.value = preset.protocol
  apiKeyBaseUrl.value = preset.url
}

const syncPreviewCredentials = computed(() => {
  if (!apiKeyValue.value) return undefined
  const baseUrl = isCNPlatform.value && apiProtocol.value === 'adaptive'
    ? adaptiveBaseUrls.value.chat_completions.trim() || apiKeyBaseUrl.value.trim()
    : apiKeyBaseUrl.value.trim()
  const modelMapping = buildModelMappingObject(
    modelRestrictionMode.value,
    allowedModels.value,
    modelMappings.value
  )
  return {
    platform: form.platform,
    type: form.type,
    base_url: baseUrl || undefined,
    api_key: apiKeyValue.value,
    ...(modelMapping ? { model_mapping: modelMapping } : {})
  }
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
const DEFAULT_POOL_MODE_RETRY_COUNT = 3
const MAX_POOL_MODE_RETRY_COUNT = 10
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
const customErrorCodesEnabled = ref(false)
const selectedErrorCodes = ref<number[]>([])
const customErrorCodeInput = ref<number | null>(null)
const headerOverrideEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

// Grok OAuth：自定义上游地址（base_url 仅改写转发端点，OAuth 授权/刷新不受影响）
const grokOAuthCustomBaseUrlEnabled = ref(false)
const grokOAuthBaseUrl = ref('')

// Grok OAuth 三条创建路径（授权码/RT 批量/SSO 批量）共用的前置校验。
// 授权码路径必须在兑换 code 之前调用，避免校验失败时白白消耗一次性授权码。
const validateGrokOAuthUpstreamConfig = (): boolean => {
  if (grokOAuthCustomBaseUrlEnabled.value) {
    const trimmed = grokOAuthBaseUrl.value.trim()
    if (!trimmed) {
      appStore.showError(t('admin.accounts.grokCustomBaseUrl.required'))
      return false
    }
    if (!/^https?:\/\//i.test(trimmed)) {
      appStore.showError(t('admin.accounts.grokCustomBaseUrl.invalid'))
      return false
    }
  }
  if (headerOverrideEnabled.value) {
    const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
    if (headerError) {
      appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
      return false
    }
  }
  return true
}

// 把已通过校验的自定义上游地址与请求头覆写写入 credentials
const applyGrokOAuthUpstreamConfig = (credentials: Record<string, unknown>) => {
  if (grokOAuthCustomBaseUrlEnabled.value) {
    credentials.base_url = grokOAuthBaseUrl.value.trim()
  }
  applyHeaderOverride(credentials, headerOverrideEnabled.value, headerOverrideRows.value, 'create')
}
const interceptWarmupRequests = ref(false)
const autoPauseOnExpired = ref(true)
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
type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
const codexFingerprintMode = ref<CodexFingerprintMode>('off')
const codexFingerprintModeOptions = computed(() => [
  { value: 'off' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintOff') },
  { value: 'device' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintDevice') },
  { value: 'session' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintSession') },
  { value: 'full' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintFull') },
])
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicPassthroughEnabled = ref(false)
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const webSearchEmulationMode = ref('default')
const webSearchGlobalEnabled = ref(false)

const toggleOpenAILongContextBilling = () => {
  openAILongContextBillingEnabled.value = !openAILongContextBillingEnabled.value
  openAILongContextBillingTouched.value = true
}
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
const vertexServiceAccountJson = ref('')
const vertexProjectId = ref('')
const vertexClientEmail = ref('')
const vertexLocation = ref('global')
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
const getOpenAICompactModelMappingKey = createStableObjectKeyResolver<ModelMapping>('create-openai-compact-model-mapping')
const geminiOAuthType = ref<'code_assist' | 'google_one' | 'ai_studio'>('google_one')
const geminiAIStudioOAuthEnabled = ref(false)
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

function buildAntigravityExtra(): Record<string, unknown> | undefined {
  const extra: Record<string, unknown> = {}
  if (mixedScheduling.value) extra.mixed_scheduling = true
  if (allowOverages.value) extra.allow_overages = true
  return Object.keys(extra).length > 0 ? extra : undefined
}

const buildOpenAICompactModelMapping = () =>
  buildModelMappingObject('mapping', [], openAICompactModelMappings.value)

const showMixedChannelWarning = ref(false)
const mixedChannelWarningDetails = ref<{ groupName: string; currentPlatform: string; otherPlatform: string } | null>(
  null
)
const mixedChannelWarningRawMessage = ref('')
const mixedChannelWarningAction = ref<(() => Promise<void>) | null>(null)
const antigravityMixedChannelConfirmed = ref(false)
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
const tlsFingerprintEnabled = ref(false)
const tlsFingerprintProfileId = ref<number | null>(null)
const tlsFingerprintProfiles = ref<{ id: number; name: string }[]>([])
const tlsFingerprintRouters = ref<{ id: number; name: string }[]>([])
const tlsFingerprintRouterId = ref<number | null>(null)
const tlsFingerprintDefaultOS = ref('')
const tlsFingerprintBindingRows = ref<{ os: string; client: string; protocol: string; profileId: number }[]>([])

function supportsTLSFingerprint(platform?: string | null) {
  return ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro'].includes(platform || '')
}

function tlsFingerprintBindingsFromRows(): Record<string, number> | undefined {
  const out: Record<string, number> = {}
  for (const row of tlsFingerprintBindingRows.value) {
    const parts = [row.os, row.client].filter((part) => part.trim())
    let key = parts.join('/').toLowerCase()
    if (row.protocol.trim()) {
      key = key ? `${key}@${row.protocol.trim().toLowerCase()}` : row.protocol.trim().toLowerCase()
    }
    if (!key || !row.profileId) continue
    out[key] = row.profileId
  }
  return Object.keys(out).length ? out : undefined
}

function applyTLSFingerprintToExtra(extra: Record<string, unknown>) {
  if (!tlsFingerprintEnabled.value) return
  extra.enable_tls_fingerprint = true
  if (tlsFingerprintProfileId.value) {
    extra.tls_fingerprint_profile_id = tlsFingerprintProfileId.value
  }
  if (tlsFingerprintRouterId.value) {
    extra.tls_fingerprint_router_id = tlsFingerprintRouterId.value
  }
  if (tlsFingerprintDefaultOS.value) {
    extra.tls_fingerprint_default_os = tlsFingerprintDefaultOS.value
  }
  const bindings = tlsFingerprintBindingsFromRows()
  if (bindings) {
    extra.tls_fingerprint_bindings = bindings
  }
}

function withTLSFingerprintExtra(base?: Record<string, unknown>) {
  const extra: Record<string, unknown> = { ...(base || {}) }
  applyTLSFingerprintToExtra(extra)
  return extra
}
const sessionIdMaskingEnabled = ref(false)
const cacheTTLOverrideEnabled = ref(false)
const cacheTTLOverrideTarget = ref<string>('5m')
const customBaseUrlEnabled = ref(false)
const customBaseUrl = ref('')

// Gemini tier selection (used as fallback when auto-detection is unavailable/fails)
const geminiTierGoogleOne = ref<'google_one_free' | 'google_ai_pro' | 'google_ai_ultra'>('google_one_free')
const geminiTierGcp = ref<'gcp_standard' | 'gcp_enterprise'>('gcp_standard')
const geminiTierAIStudio = ref<'aistudio_free' | 'aistudio_paid'>('aistudio_free')

const geminiSelectedTier = computed(() => {
  if (form.platform !== 'gemini') return ''
  if (accountCategory.value === 'apikey') return geminiTierAIStudio.value
  switch (geminiOAuthType.value) {
    case 'google_one':
      return geminiTierGoogleOne.value
    case 'code_assist':
      return geminiTierGcp.value
    default:
      return geminiTierAIStudio.value
  }
})

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

const mixedChannelWarningMessageText = computed(() => {
  if (mixedChannelWarningDetails.value) {
    return t('admin.accounts.mixedChannelWarning', mixedChannelWarningDetails.value)
  }
  return mixedChannelWarningRawMessage.value
})

const geminiHelpLinks = {
  apiKey: 'https://aistudio.google.com/app/apikey',
  aiStudioPricing: 'https://ai.google.dev/pricing',
  gcpProject: 'https://console.cloud.google.com/welcome/new',
  geminiWebActivation: 'https://gemini.google.com/gems/create?hl=en-US&pli=1',
  countryCheck: 'https://policies.google.com/terms',
  countryChange: 'https://policies.google.com/country-association-form'
}

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
  (newVal) => {
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
    } else {
      resetForm()
    }
  }
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

// Model mapping helpers
const addOpenAICompactModelMapping = () => {
  openAICompactModelMappings.value.push({ from: '', to: '' })
}

const removeOpenAICompactModelMapping = (index: number) => {
  openAICompactModelMappings.value.splice(index, 1)
}


const needsMixedChannelCheck = (platform: AccountPlatform) => platform === 'antigravity' || platform === 'anthropic'

const buildMixedChannelDetails = (resp?: CheckMixedChannelResponse) => {
  const details = resp?.details
  if (!details) {
    return null
  }
  return {
    groupName: details.group_name || 'Unknown',
    currentPlatform: details.current_platform || 'Unknown',
    otherPlatform: details.other_platform || 'Unknown'
  }
}

const clearMixedChannelDialog = () => {
  showMixedChannelWarning.value = false
  mixedChannelWarningDetails.value = null
  mixedChannelWarningRawMessage.value = ''
  mixedChannelWarningAction.value = null
}

const openMixedChannelDialog = (opts: {
  response?: CheckMixedChannelResponse
  message?: string
  onConfirm: () => Promise<void>
}) => {
  mixedChannelWarningDetails.value = buildMixedChannelDetails(opts.response)
  mixedChannelWarningRawMessage.value =
    opts.message || opts.response?.message || t('admin.accounts.failedToCreate')
  mixedChannelWarningAction.value = opts.onConfirm
  showMixedChannelWarning.value = true
}

const withAntigravityConfirmFlag = (payload: CreateAccountRequest): CreateAccountRequest => {
  if (needsMixedChannelCheck(payload.platform) && antigravityMixedChannelConfirmed.value) {
    return {
      ...payload,
      confirm_mixed_channel_risk: true
    }
  }
  const cloned = { ...payload }
  delete cloned.confirm_mixed_channel_risk
  return cloned
}

const ensureAntigravityMixedChannelConfirmed = async (onConfirm: () => Promise<void>): Promise<boolean> => {
  if (!needsMixedChannelCheck(form.platform)) {
    return true
  }
  if (antigravityMixedChannelConfirmed.value) {
    return true
  }

  try {
    const result = await adminAPI.accounts.checkMixedChannelRisk({
      platform: form.platform,
      group_ids: form.group_ids
    })
    if (!result.has_risk) {
      return true
    }
    openMixedChannelDialog({
      response: result,
      onConfirm: async () => {
        antigravityMixedChannelConfirmed.value = true
        await onConfirm()
      }
    })
    return false
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.accounts.failedToCreate'))
    return false
  }
}

const submitCreateAccount = async (payload: CreateAccountRequest) => {
  submitting.value = true
  try {
    const account = await adminAPI.accounts.create(withAntigravityConfirmFlag(payload))
    const modelMapping = payload.credentials.model_mapping
    const hasConcreteMappedTarget = payload.type === 'apikey' &&
      typeof modelMapping === 'object' &&
      modelMapping !== null &&
      Object.values(modelMapping).some((target) =>
        typeof target === 'string' && target.trim() !== '' && !target.includes('*')
      )
    if (upstreamModelsPreviewed.value || hasConcreteMappedTarget) {
      try {
        const result = await adminAPI.accounts.syncUpstreamModels(account.id)
        if (result.warnings?.some(warning => warning.code === 'upstream_model_metadata_incomplete')) {
          appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataIncomplete'))
        }
      } catch {
        appStore.showWarning(t('admin.accounts.syncUpstreamModelsFailed'))
      }
    }
    if (
      payload.type === 'apikey' &&
      payload.upstream_billing_probe_enabled === true
    ) {
      try {
        await adminAPI.accounts.probeUpstreamBilling(account.id)
      } catch {
        appStore.showWarning(t('admin.accounts.upstreamBilling.probeFailed'))
      }
    }
    appStore.showSuccess(t('admin.accounts.accountCreated'))
    emit('created')
    handleClose()
  } catch (error: any) {
    if (error.response?.status === 409 && error.response?.data?.error === 'mixed_channel_warning' && needsMixedChannelCheck(form.platform)) {
      openMixedChannelDialog({
        message: error.response?.data?.message,
        onConfirm: async () => {
          antigravityMixedChannelConfirmed.value = true
          await submitCreateAccount(payload)
        }
      })
      return
    }
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.accounts.failedToCreate'))
  } finally {
    submitting.value = false
  }
}

// Methods
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

const handleClose = () => {
  kiroOAuth.cancelDeviceAuthorization()
  antigravityMixedChannelConfirmed.value = false
  clearMixedChannelDialog()
  emit('close')
}

onUnmounted(() => {
  kiroOAuth.cancelDeviceAuthorization()
})

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

// Helper function to create account with mixed channel warning handling
const doCreateAccount = async (payload: CreateAccountRequest) => {
  if (supportsTLSFingerprint(payload.platform)) {
    const extra: Record<string, unknown> = { ...(payload.extra || {}) }
    applyTLSFingerprintToExtra(extra)
    payload.extra = extra
  }
  const canContinue = await ensureAntigravityMixedChannelConfirmed(async () => {
    await submitCreateAccount(payload)
  })
  if (!canContinue) {
    return
  }
  await submitCreateAccount(payload)
}

// Handle mixed channel warning confirmation
const handleMixedChannelConfirm = async () => {
  const action = mixedChannelWarningAction.value
  if (!action) {
    clearMixedChannelDialog()
    return
  }
  clearMixedChannelDialog()
  submitting.value = true
  try {
    await action()
  } finally {
    submitting.value = false
  }
}

const handleMixedChannelCancel = () => {
  clearMixedChannelDialog()
}

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

const applyVertexServiceAccountJson = (value: string) => {
  const raw = value.trim()
  if (!raw) {
    vertexProjectId.value = ''
    vertexClientEmail.value = ''
    return false
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const projectId = typeof parsed.project_id === 'string' ? parsed.project_id.trim() : ''
    const clientEmail = typeof parsed.client_email === 'string' ? parsed.client_email.trim() : ''
    const privateKey = typeof parsed.private_key === 'string' ? parsed.private_key.trim() : ''
    if (!projectId || !clientEmail || !privateKey) {
      appStore.showError(t('admin.accounts.vertexSaJsonMissingFields'))
      return false
    }
    vertexProjectId.value = projectId
    vertexClientEmail.value = clientEmail
    vertexServiceAccountJson.value = JSON.stringify(parsed)
    return true
  } catch {
    appStore.showError(t('admin.accounts.vertexSaJsonInvalid'))
    return false
  }
}

const parseVertexServiceAccountJson = () => applyVertexServiceAccountJson(vertexServiceAccountJson.value)

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

const goBackToBasicInfo = () => {
  step.value = 1
  oauth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  kiroOAuth.resetState()
  grokOAuth.resetState()
  oauthFlowRef.value?.reset()
}

const handleGenerateUrl = async () => {
  if (form.platform === 'kiro') {
    await kiroOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'openai') {
    await openaiOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'gemini') {
    await geminiOAuth.generateAuthUrl(
      form.proxy_id,
      oauthFlowRef.value?.projectId,
      geminiOAuthType.value,
      geminiSelectedTier.value
    )
  } else if (form.platform === 'antigravity') {
    await antigravityOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'grok') {
    await grokOAuth.generateAuthUrl(form.proxy_id)
  } else {
    await oauth.generateAuthUrl(addMethod.value, form.proxy_id)
  }
}

const handleValidateRefreshToken = (rt: string) => {
  if (form.platform === 'openai') {
    handleOpenAIValidateRT(rt)
  } else if (form.platform === 'antigravity') {
    handleAntigravityValidateRT(rt)
  } else if (form.platform === 'grok') {
    handleGrokValidateRT(rt)
  }
}

const applyKiroModelRestriction = (credentials: Record<string, unknown>) => {
  const modelMapping = buildModelMappingObject(
    modelRestrictionMode.value,
    allowedModels.value,
    modelMappings.value,
    'kiro'
  )
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
}

const handleKiroAuthorize = async (payload: {
  callbackUrl: string
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
}) => {
  const tokenInfo = await kiroOAuth.exchangeCallback(payload.callbackUrl, form.proxy_id)
  if (!tokenInfo) {
    return
  }
  const credentials = kiroOAuth.buildCredentials(tokenInfo, payload.credentials)
  const extra = kiroOAuth.buildExtraInfo(tokenInfo, payload.extra)
  applyKiroModelRestriction(credentials)
  await createAccountAndFinish('kiro', 'oauth', credentials, extra, kiroOAuth.buildAccountName(tokenInfo, form.name))
}

const handleKiroValidateRT = async (payload: {
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
}) => {
  const refreshTokens = String(payload.credentials.refresh_token || '')
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    kiroOAuth.error.value = t('admin.accounts.kiro.refreshTokenRequired')
    return
  }

  kiroOAuth.loading.value = true
  kiroOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const manualCredentials = {
          ...payload.credentials,
          refresh_token: refreshTokens[i]
        }
        const validatedCredentials = await kiroOAuth.validateRefreshToken(
          manualCredentials,
          payload.extra,
          form.proxy_id,
          false
        )
        if (!validatedCredentials) {
          failedCount++
          errors.push(`#${i + 1}: ${kiroOAuth.error.value || t('admin.accounts.kiro.failedToValidateRT')}`)
          kiroOAuth.error.value = ''
          continue
        }

        const tokenInfo = validatedCredentials as KiroTokenInfo
        const credentials = { ...validatedCredentials }
        const extra = withTLSFingerprintExtra(kiroOAuth.buildExtraInfo(tokenInfo, payload.extra))
        applyKiroModelRestriction(credentials)
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        const baseName = kiroOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: 'kiro',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error?.response?.data?.detail || error?.message || t('admin.accounts.failedToCreate')
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      kiroOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      kiroOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    kiroOAuth.loading.value = false
  }
}

const handleValidateSessionToken = (_sessionToken: string) => {
  // Session token validation removed
}

const formatDateTimeLocal = formatDateTimeLocalInput
const parseDateTimeLocal = parseDateTimeLocalInput

// Create account and handle success/failure
const createAccountAndFinish = async (
  platform: AccountPlatform,
  type: AccountType,
  credentials: Record<string, unknown>,
  extra?: Record<string, unknown>,
  nameOverride?: string
) => {
  if (!applyTempUnschedConfig(credentials)) {
    return
  }
  // Inject quota limits for apikey/bedrock accounts
  let finalExtra = extra
  if (type === 'apikey' || type === 'bedrock') {
    const quotaExtra: Record<string, unknown> = { ...(extra || {}) }
    if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
      quotaExtra.quota_limit = editQuotaLimit.value
    }
    if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
      quotaExtra.quota_daily_limit = editQuotaDailyLimit.value
    }
    if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
      quotaExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
    }
    // Quota reset mode config
    if (editDailyResetMode.value === 'fixed') {
      quotaExtra.quota_daily_reset_mode = 'fixed'
      quotaExtra.quota_daily_reset_hour = editDailyResetHour.value ?? 0
    }
    if (editWeeklyResetMode.value === 'fixed') {
      quotaExtra.quota_weekly_reset_mode = 'fixed'
      quotaExtra.quota_weekly_reset_day = editWeeklyResetDay.value ?? 1
      quotaExtra.quota_weekly_reset_hour = editWeeklyResetHour.value ?? 0
    }
    if (editDailyResetMode.value === 'fixed' || editWeeklyResetMode.value === 'fixed') {
      quotaExtra.quota_reset_timezone = editResetTimezone.value || 'UTC'
    }
    // Quota notify config
    writeQuotaNotifyToExtra(quotaExtra, 'create')
    if (Object.keys(quotaExtra).length > 0) {
      finalExtra = quotaExtra
    }
  }
  if (platform === 'openai') {
    if (type === 'apikey') {
      applyOpenAIEndpointCapabilities(credentials)
    }
    const compactModelMapping = buildOpenAICompactModelMapping()
    if (compactModelMapping) {
      credentials.compact_model_mapping = compactModelMapping
    } else {
      delete credentials.compact_model_mapping
    }
  }
  if (platform === 'grok') {
    if (!credentials.base_url) {
      credentials.base_url = apiKeyBaseUrl.value.trim() || 'https://api.x.ai/v1'
    }
    const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    } else {
      delete credentials.model_mapping
    }
  }
  await doCreateAccount({
    name: nameOverride || form.name,
    notes: form.notes,
    platform,
    type,
    credentials,
    extra: finalExtra,
    proxy_id: form.proxy_id,
    concurrency: form.concurrency,
    load_factor: form.load_factor ?? undefined,
    priority: form.priority,
    rate_multiplier: form.rate_multiplier,
    group_ids: form.group_ids,
    expires_at: form.expires_at,
    // 上游倍率探测对全部 API-key 平台开放（antigravity upstream 走本 helper）；
    // 非 apikey 类型（bedrock/oauth）不传，后端不动作。
    upstream_billing_probe_enabled: type === 'apikey' ? upstreamBillingAutoProbeEnabled.value : undefined,
    auto_pause_on_expired: autoPauseOnExpired.value
  })
}

// Grok 手动 RT 批量验证和创建
const handleGrokValidateRT = async (refreshTokenInput: string) => {
  if (!refreshTokenInput.trim()) return

  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    grokOAuth.error.value = t('admin.accounts.oauth.grok.pleaseEnterRefreshToken')
    return
  }
  if (!validateGrokOAuthUpstreamConfig()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await grokOAuth.validateRefreshToken(refreshTokens[i], form.proxy_id)
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${grokOAuth.error.value || 'Validation failed'}`)
          grokOAuth.error.value = ''
          continue
        }

        const credentials = grokOAuth.buildCredentials(tokenInfo)
        applyGrokOAuthUpstreamConfig(credentials)
        const extra = withTLSFingerprintExtra(grokOAuth.buildExtraInfo(tokenInfo))
        const baseName = grokOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
        if (modelMapping) {
          credentials.model_mapping = modelMapping
        }
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0) {
      appStore.showWarning(t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount }))
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    grokOAuth.loading.value = false
  }
}

const handleGrokImportSSO = async (ssoInput: string) => {
  // Align with OpenAI/Grok RT batch import: one token per line, no client-side dedupe.
  const ssoTokens = ssoInput
    .split('\n')
    .map((token) => token.trim())
    .filter((token) => token)
  if (ssoTokens.length === 0) return
  if (!validateGrokOAuthUpstreamConfig()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  const credentials: Record<string, unknown> = {}
  applyGrokOAuthUpstreamConfig(credentials)
  const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
  if (!applyTempUnschedConfig(credentials)) {
    grokOAuth.loading.value = false
    return
  }

  try {
    const result = await adminAPI.grok.createFromSSO({
      sso_tokens: ssoTokens,
      name: form.name || undefined,
      notes: form.notes || undefined,
      proxy_id: form.proxy_id,
      group_ids: form.group_ids,
      credentials,
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value
    })

    const successCount = result.created?.length || 0
    const failedCount = result.failed?.length || 0
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        ssoTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      // Same as OpenAI/Grok RT: keep input, show failures, refresh list.
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n') || t('admin.accounts.oauth.grok.failedToConvertSSO')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } catch (error: any) {
    grokOAuth.error.value = error.response?.data?.detail || error.message || t('admin.accounts.oauth.grok.failedToConvertSSO')
    appStore.showError(grokOAuth.error.value)
  } finally {
    grokOAuth.loading.value = false
  }
}

/**
 * Grok password login: each line is email----password.
 * Password is only used for the authorize API call; buildCredentials never stores it.
 */
const handleGrokAuthorizePassword = async (emailPasswordInput: string) => {
  if (!emailPasswordInput.trim()) return
  if (!validateGrokOAuthUpstreamConfig()) return

  const lines = emailPasswordInput
    .split('\n')
    // Keep the password portion byte-for-byte; trim is only for determining
    // whether this textarea line is blank.
    .filter((line) => line.trim() && line.includes('----'))

  if (lines.length === 0) {
    grokOAuth.error.value = t(
      'admin.accounts.oauth.grok.pleaseEnterPassword',
      'Please enter email----password (one per line)'
    )
    return
  }

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < lines.length; i++) {
      try {
        const tokenInfo = await grokOAuth.authorizePassword(lines[i], form.proxy_id)
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${grokOAuth.error.value || 'Authorization failed'}`)
          grokOAuth.error.value = ''
          continue
        }

        const credentials = grokOAuth.buildCredentials(tokenInfo)
        applyGrokOAuthUpstreamConfig(credentials)
        const extra = withTLSFingerprintExtra(grokOAuth.buildExtraInfo(tokenInfo))
        const baseName = grokOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = lines.length > 1 ? `${baseName} #${i + 1}` : baseName

        const modelMapping = buildModelMappingObject(
          modelRestrictionMode.value,
          allowedModels.value,
          modelMappings.value
        )
        if (modelMapping) {
          credentials.model_mapping = modelMapping
        }
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        lines.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', {
          success: successCount,
          failed: failedCount
        })
      )
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    grokOAuth.loading.value = false
  }
}

// OpenAI OAuth 授权码兑换
const handleOpenAIExchange = async (authCode: string) => {
  const oauthClient = openaiOAuth
  if (!authCode.trim() || !oauthClient.sessionId.value) return

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const stateToUse = (oauthFlowRef.value?.oauthState || oauthClient.oauthState.value || '').trim()
    if (!stateToUse) {
      oauthClient.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(oauthClient.error.value)
      return
    }

    const tokenInfo = await oauthClient.exchangeAuthCode(
      authCode.trim(),
      oauthClient.sessionId.value,
      stateToUse,
      form.proxy_id
    )
    if (!tokenInfo) return

    const credentials = oauthClient.buildCredentials(tokenInfo)
    const oauthExtra = oauthClient.buildExtraInfo(tokenInfo) as Record<string, unknown> | undefined
    const extra = withTLSFingerprintExtra(buildOpenAIExtra(oauthExtra))
    const shouldCreateOpenAI = form.platform === 'openai'

    // Add model mapping for OpenAI OAuth accounts（透传模式下不应用）
    if (shouldCreateOpenAI && !isOpenAIModelRestrictionDisabled.value) {
      const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
      if (modelMapping) {
        credentials.model_mapping = modelMapping
      }
    }
    if (shouldCreateOpenAI) {
      const compactModelMapping = buildOpenAICompactModelMapping()
      if (compactModelMapping) {
        credentials.compact_model_mapping = compactModelMapping
      }
    }

    // 应用临时不可调度配置
    if (!applyTempUnschedConfig(credentials)) {
      return
    }

    if (shouldCreateOpenAI) {
      await adminAPI.accounts.create({
        name: oauthClient.buildAccountName(tokenInfo, form.name),
        notes: form.notes,
        platform: 'openai',
        type: 'oauth',
        credentials,
        extra,
        proxy_id: form.proxy_id,
        concurrency: form.concurrency,
        load_factor: form.load_factor ?? undefined,
        priority: form.priority,
        rate_multiplier: form.rate_multiplier,
        group_ids: form.group_ids,
        expires_at: form.expires_at,
        auto_pause_on_expired: autoPauseOnExpired.value
      })
      appStore.showSuccess(t('admin.accounts.accountCreated'))
    }

    emit('created')
    handleClose()
  } catch (error: any) {
    oauthClient.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(oauthClient.error.value)
  } finally {
    oauthClient.loading.value = false
  }
}

// OpenAI 手动 RT 批量验证和创建
// OpenAI Mobile RT client_id
const OPENAI_MOBILE_RT_CLIENT_ID = 'app_LlGpXReQgckcGGUo2JrYvtJK'

const buildOpenAICodexImportCredentialExtras = (): Record<string, unknown> | null => {
  const credentials: Record<string, unknown> = {}
  if (!isOpenAIModelRestrictionDisabled.value) {
    const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    }
  }

  const compactModelMapping = buildOpenAICompactModelMapping()
  if (compactModelMapping) {
    credentials.compact_model_mapping = compactModelMapping
  }

  if (!applyTempUnschedConfig(credentials)) {
    return null
  }
  return credentials
}

const formatCodexImportMessages = (messages?: CodexSessionImportMessage[]) => {
  return (messages || [])
    .map((item) => {
      const name = item.name ? ` ${item.name}` : ''
      return `#${item.index}${name}: ${item.message}`
    })
    .join('\n')
}

const isAgentIdentityImportContent = (content: string) => {
  const isAgentIdentityValue = (value: unknown): boolean => {
    if (Array.isArray(value)) return value.length > 0 && value.every(isAgentIdentityValue)
    if (!value || typeof value !== 'object') return false
    const record = value as Record<string, unknown>
    const authMode = record.auth_mode ?? record.authMode
    const agentIdentity = record.agent_identity ?? record.agentIdentity
    return (typeof authMode === 'string' && authMode.toLowerCase() === 'agentidentity')
      || (!!agentIdentity && typeof agentIdentity === 'object')
  }

  try {
    return isAgentIdentityValue(JSON.parse(content))
  } catch {
    const lines = content.split('\n').map((line) => line.trim()).filter(Boolean)
    if (lines.length === 0) return false
    try {
      return lines.every((line) => isAgentIdentityValue(JSON.parse(line)))
    } catch {
      return false
    }
  }
}

const handleOpenAIImportCodexSession = async (content: string) => {
  const oauthClient = openaiOAuth
  const trimmed = content.trim()
  if (!trimmed) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.codexSessionEmpty')
    return
  }
  if (oauthFlowRef.value?.inputMethod === 'agent_identity' && !isAgentIdentityImportContent(trimmed)) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.agentIdentityInvalid')
    return
  }

  const credentialExtras = buildOpenAICodexImportCredentialExtras()
  if (credentialExtras === null) {
    return
  }

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const extra = buildOpenAICodexImportExtra()
    const result = await adminAPI.accounts.importCodexSession({
      content: trimmed,
      name: form.name.trim() || 'OpenAI OAuth Account',
      notes: form.notes || null,
      proxy_id: form.proxy_id,
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      group_ids: form.group_ids,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value,
      credential_extras: Object.keys(credentialExtras).length > 0 ? credentialExtras : undefined,
      extra,
      update_existing: true
    })

    const successCount = result.created + result.updated
    const params = {
      created: result.created,
      updated: result.updated,
      skipped: result.skipped,
      failed: result.failed
    }

    if (successCount > 0 && result.failed === 0) {
      appStore.showSuccess(t('admin.accounts.oauth.openai.codexSessionImportSuccess', params))
      emit('created')
      handleClose()
      return
    }

    const errorText = formatCodexImportMessages(result.errors)
    const warningText = formatCodexImportMessages(result.warnings)
    oauthClient.error.value = [errorText, warningText].filter(Boolean).join('\n')

    if (result.failed === 0) {
      appStore.showWarning(t('admin.accounts.oauth.openai.codexSessionImportSuccess', params))
      return
    }

    if (successCount > 0) {
      appStore.showWarning(t('admin.accounts.oauth.openai.codexSessionImportPartial', params))
      emit('created')
      return
    }

    appStore.showError(t('admin.accounts.oauth.openai.codexSessionImportFailed'))
  } catch (error: any) {
    oauthClient.error.value =
      error.response?.data?.detail ||
      error.response?.data?.message ||
      error.message ||
      t('admin.accounts.oauth.openai.codexSessionImportFailed')
    appStore.showError(oauthClient.error.value)
  } finally {
    oauthClient.loading.value = false
  }
}

const handleOpenAIImportCodexPAT = async (accessToken: string) => {
  const oauthClient = openaiOAuth
  const trimmed = accessToken.trim()
  if (!trimmed) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.codexPatEmpty')
    return
  }

  const credentialExtras = buildOpenAICodexImportCredentialExtras()
  if (credentialExtras === null) {
    return
  }

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const extra = buildOpenAICodexImportExtra()
    await adminAPI.accounts.createOpenAICodexPAT({
      access_token: trimmed,
      name: form.name.trim() || 'OpenAI OAuth Account',
      notes: form.notes || null,
      proxy_id: form.proxy_id,
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      group_ids: form.group_ids,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value,
      credential_extras: Object.keys(credentialExtras).length > 0 ? credentialExtras : undefined,
      extra
    })

    appStore.showSuccess(t('admin.accounts.messages.accountCreated'))
    emit('created')
    handleClose()
  } catch (error: any) {
    oauthClient.error.value =
      error.response?.data?.detail ||
      error.response?.data?.message ||
      error.message ||
      t('admin.accounts.oauth.openai.codexPatImportFailed')
    appStore.showError(oauthClient.error.value)
  } finally {
    oauthClient.loading.value = false
  }
}

// OpenAI RT 批量验证和创建（共享逻辑）
const handleOpenAIBatchRT = async (refreshTokenInput: string, clientId?: string) => {
  const oauthClient = openaiOAuth
  if (!refreshTokenInput.trim()) return

  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.pleaseEnterRefreshToken')
    return
  }

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []
  const shouldCreateOpenAI = form.platform === 'openai'

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await oauthClient.validateRefreshToken(
          refreshTokens[i],
          form.proxy_id,
          clientId
        )
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${oauthClient.error.value || 'Validation failed'}`)
          oauthClient.error.value = ''
          continue
        }

        const credentials = oauthClient.buildCredentials(tokenInfo)
        if (clientId) {
          credentials.client_id = clientId
        }
        const oauthExtra = oauthClient.buildExtraInfo(tokenInfo) as Record<string, unknown> | undefined
        const extra = withTLSFingerprintExtra(buildOpenAIExtra(oauthExtra))

        // Add model mapping for OpenAI OAuth accounts（透传模式下不应用）
        if (shouldCreateOpenAI && !isOpenAIModelRestrictionDisabled.value) {
          const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
          if (modelMapping) {
            credentials.model_mapping = modelMapping
          }
        }
        if (shouldCreateOpenAI) {
          const compactModelMapping = buildOpenAICompactModelMapping()
          if (compactModelMapping) {
            credentials.compact_model_mapping = compactModelMapping
          }
        }

        // Generate account name; fallback to email if name is empty (ent schema requires NotEmpty)
        const baseName = oauthClient.buildAccountName(tokenInfo, form.name)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        if (shouldCreateOpenAI) {
          await adminAPI.accounts.create({
            name: accountName,
            notes: form.notes,
            platform: 'openai',
            type: 'oauth',
            credentials,
            extra,
            proxy_id: form.proxy_id,
            concurrency: form.concurrency,
            load_factor: form.load_factor ?? undefined,
            priority: form.priority,
            rate_multiplier: form.rate_multiplier,
            group_ids: form.group_ids,
            expires_at: form.expires_at,
            auto_pause_on_expired: autoPauseOnExpired.value
          })
        }

        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    // Show results
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      oauthClient.error.value = errors.join('\n')
      emit('created')
    } else {
      oauthClient.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    oauthClient.loading.value = false
  }
}

// 手动输入 RT（Codex CLI client_id，默认）
const handleOpenAIValidateRT = (rt: string) => handleOpenAIBatchRT(rt)

// 手动输入 Mobile RT
const handleOpenAIValidateMobileRT = (rt: string) => handleOpenAIBatchRT(rt, OPENAI_MOBILE_RT_CLIENT_ID)

// Antigravity 手动 RT 批量验证和创建
const handleAntigravityValidateRT = async (refreshTokenInput: string) => {
  if (!refreshTokenInput.trim()) return

  // Parse multiple refresh tokens (one per line)
  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    antigravityOAuth.error.value = t('admin.accounts.oauth.antigravity.pleaseEnterRefreshToken')
    return
  }

  antigravityOAuth.loading.value = true
  antigravityOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await antigravityOAuth.validateRefreshToken(
          refreshTokens[i],
          form.proxy_id
        )
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${antigravityOAuth.error.value || 'Validation failed'}`)
          antigravityOAuth.error.value = ''
          continue
        }

        const credentials = antigravityOAuth.buildCredentials(tokenInfo, refreshTokens[i])
        applyAntigravityProjectID(credentials, antigravityProjectId.value, 'create')
        
        // Generate account name with index for batch
        const baseName = antigravityOAuth.buildAccountName(tokenInfo, form.name)
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        // Note: Antigravity doesn't have buildExtraInfo, so we pass empty extra or rely on credentials
        const createPayload = withAntigravityConfirmFlag({
          name: accountName,
          notes: form.notes,
          platform: 'antigravity',
          type: 'oauth',
          credentials,
          extra: withTLSFingerprintExtra(),
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        await adminAPI.accounts.create(createPayload)
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    // Show results
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      antigravityOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      antigravityOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    antigravityOAuth.loading.value = false
  }
}

// Gemini OAuth 授权码兑换
const handleGeminiExchange = async (authCode: string) => {
  if (!authCode.trim() || !geminiOAuth.sessionId.value) return

  geminiOAuth.loading.value = true
  geminiOAuth.error.value = ''

  try {
    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || geminiOAuth.state.value
    if (!stateToUse) {
      geminiOAuth.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(geminiOAuth.error.value)
      return
    }

    const tokenInfo = await geminiOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: geminiOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id,
      oauthType: geminiOAuthType.value,
      tierId: geminiSelectedTier.value
    })
    if (!tokenInfo) return

    const credentials = geminiOAuth.buildCredentials(tokenInfo)
    const extra = geminiOAuth.buildExtraInfo(tokenInfo)
    await createAccountAndFinish('gemini', 'oauth', credentials, extra, geminiOAuth.buildAccountName(tokenInfo, form.name))
  } catch (error: any) {
    geminiOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(geminiOAuth.error.value)
  } finally {
    geminiOAuth.loading.value = false
  }
}

// Antigravity OAuth 授权码兑换
const handleAntigravityExchange = async (authCode: string) => {
  if (!authCode.trim() || !antigravityOAuth.sessionId.value) return

  antigravityOAuth.loading.value = true
  antigravityOAuth.error.value = ''

  try {
    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || antigravityOAuth.state.value
    if (!stateToUse) {
      antigravityOAuth.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(antigravityOAuth.error.value)
      return
    }

    const tokenInfo = await antigravityOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: antigravityOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id
    })
		if (!tokenInfo) return

		const credentials = antigravityOAuth.buildCredentials(tokenInfo)
		applyAntigravityProjectID(credentials, antigravityProjectId.value, 'create')
		applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
		// Antigravity 只使用映射模式
		const antigravityModelMapping = buildModelMappingObject(
			'mapping',
			[],
			antigravityModelMappings.value
		)
		if (antigravityModelMapping) {
			credentials.model_mapping = antigravityModelMapping
		}
		const extra = buildAntigravityExtra()
		await createAccountAndFinish('antigravity', 'oauth', credentials, extra, antigravityOAuth.buildAccountName(tokenInfo, form.name))
  } catch (error: any) {
    antigravityOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(antigravityOAuth.error.value)
  } finally {
    antigravityOAuth.loading.value = false
  }
}

// Grok OAuth 授权码兑换
const handleGrokExchange = async (authCode: string) => {
  if (!authCode.trim() || !grokOAuth.sessionId.value) return
  if (!validateGrokOAuthUpstreamConfig()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  try {
    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || grokOAuth.state.value
    if (!stateToUse) {
      grokOAuth.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(grokOAuth.error.value)
      return
    }

    const tokenInfo = await grokOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: grokOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id
    })
    if (!tokenInfo) return

    const credentials = grokOAuth.buildCredentials(tokenInfo)
    applyGrokOAuthUpstreamConfig(credentials)
    const extra = grokOAuth.buildExtraInfo(tokenInfo)
    await createAccountAndFinish('grok', 'oauth', credentials, extra, grokOAuth.buildAccountName(tokenInfo, form.name))
  } catch (error: any) {
    grokOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(grokOAuth.error.value)
  } finally {
    grokOAuth.loading.value = false
  }
}

// Anthropic OAuth 授权码兑换
const handleAnthropicExchange = async (authCode: string) => {
  if (!authCode.trim() || !oauth.sessionId.value) return

  oauth.loading.value = true
  oauth.error.value = ''

  try {
    const proxyConfig = form.proxy_id ? { proxy_id: form.proxy_id } : {}
    const endpoint =
      addMethod.value === 'oauth'
        ? '/admin/accounts/exchange-code'
        : '/admin/accounts/exchange-setup-token-code'

    const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
      session_id: oauth.sessionId.value,
      code: authCode.trim(),
      ...proxyConfig
    })

    // Build extra with quota control settings
    const baseExtra = oauth.buildExtraInfo(tokenInfo) || {}
    const extra: Record<string, unknown> = { ...baseExtra }

    // Add window cost limit settings
    if (windowCostEnabled.value && windowCostLimit.value != null && windowCostLimit.value > 0) {
      extra.window_cost_limit = windowCostLimit.value
      extra.window_cost_sticky_reserve = windowCostStickyReserve.value ?? 10
    }

    // Add session limit settings
    if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
      extra.max_sessions = maxSessions.value
      extra.session_idle_timeout_minutes = sessionIdleTimeout.value ?? 5
    }

    // Add RPM limit settings
    if (rpmLimitEnabled.value) {
      const DEFAULT_BASE_RPM = 15
      extra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
        ? baseRpm.value
        : DEFAULT_BASE_RPM
      extra.rpm_strategy = rpmStrategy.value
      if (rpmStickyBuffer.value != null && rpmStickyBuffer.value > 0) {
        extra.rpm_sticky_buffer = rpmStickyBuffer.value
      }
    }

    // UMQ mode（独立于 RPM）
    if (userMsgQueueMode.value) {
      extra.user_msg_queue_mode = userMsgQueueMode.value
    }

    applyTLSFingerprintToExtra(extra)

    // Add session ID masking settings
    if (sessionIdMaskingEnabled.value) {
      extra.session_id_masking_enabled = true
    }

    // Add cache TTL override settings
    if (cacheTTLOverrideEnabled.value) {
      extra.cache_ttl_override_enabled = true
      extra.cache_ttl_override_target = cacheTTLOverrideTarget.value
    }

    // Add custom base URL settings
    if (customBaseUrlEnabled.value && customBaseUrl.value.trim()) {
      extra.custom_base_url_enabled = true
      extra.custom_base_url = customBaseUrl.value.trim()
    }

    const credentials: Record<string, unknown> = { ...tokenInfo }
    applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
    await createAccountAndFinish(form.platform, addMethod.value as AccountType, credentials, extra, oauth.buildAccountName(tokenInfo, form.name))
  } catch (error: any) {
    oauth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(oauth.error.value)
  } finally {
    oauth.loading.value = false
  }
}

// 主入口：根据平台路由到对应处理函数
const handleExchangeCode = async () => {
  const authCode = oauthFlowRef.value?.authCode || ''

  switch (form.platform) {
    case 'openai':
      return handleOpenAIExchange(authCode)
    case 'gemini':
      return handleGeminiExchange(authCode)
    case 'antigravity':
      return handleAntigravityExchange(authCode)
    case 'grok':
      return handleGrokExchange(authCode)
    default:
      return handleAnthropicExchange(authCode)
  }
}

const handleCookieAuth = async (sessionKey: string) => {
  oauth.loading.value = true
  oauth.error.value = ''

  try {
    const proxyConfig = form.proxy_id ? { proxy_id: form.proxy_id } : {}
    const keys = oauth.parseSessionKeys(sessionKey)

    if (keys.length === 0) {
      oauth.error.value = t('admin.accounts.oauth.pleaseEnterSessionKey')
      return
    }

    const tempUnschedPayload = tempUnschedEnabled.value
      ? buildTempUnschedRules(tempUnschedRules.value)
      : []
    if (tempUnschedEnabled.value && tempUnschedPayload.length === 0) {
      appStore.showError(t('admin.accounts.tempUnschedulable.rulesInvalid'))
      return
    }

    const endpoint =
      addMethod.value === 'oauth'
        ? '/admin/accounts/cookie-auth'
        : '/admin/accounts/setup-token-cookie-auth'

    let successCount = 0
    let failedCount = 0
    const errors: string[] = []

    for (let i = 0; i < keys.length; i++) {
      try {
        const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
          session_id: '',
          code: keys[i],
          ...proxyConfig
        })

        // Build extra with quota control settings
        const baseExtra = oauth.buildExtraInfo(tokenInfo) || {}
        const extra: Record<string, unknown> = { ...baseExtra }

        // Add window cost limit settings
        if (windowCostEnabled.value && windowCostLimit.value != null && windowCostLimit.value > 0) {
          extra.window_cost_limit = windowCostLimit.value
          extra.window_cost_sticky_reserve = windowCostStickyReserve.value ?? 10
        }

        // Add session limit settings
        if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
          extra.max_sessions = maxSessions.value
          extra.session_idle_timeout_minutes = sessionIdleTimeout.value ?? 5
        }

        // Add RPM limit settings
        if (rpmLimitEnabled.value) {
          const DEFAULT_BASE_RPM = 15
          extra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
            ? baseRpm.value
            : DEFAULT_BASE_RPM
          extra.rpm_strategy = rpmStrategy.value
          if (rpmStickyBuffer.value != null && rpmStickyBuffer.value > 0) {
            extra.rpm_sticky_buffer = rpmStickyBuffer.value
          }
        }

        // UMQ mode（独立于 RPM）
        if (userMsgQueueMode.value) {
          extra.user_msg_queue_mode = userMsgQueueMode.value
        }

        applyTLSFingerprintToExtra(extra)

        // Add session ID masking settings
        if (sessionIdMaskingEnabled.value) {
          extra.session_id_masking_enabled = true
        }

        // Add cache TTL override settings
        if (cacheTTLOverrideEnabled.value) {
          extra.cache_ttl_override_enabled = true
          extra.cache_ttl_override_target = cacheTTLOverrideTarget.value
        }

        // Add custom base URL settings
        if (customBaseUrlEnabled.value && customBaseUrl.value.trim()) {
          extra.custom_base_url_enabled = true
          extra.custom_base_url = customBaseUrl.value.trim()
        }

        const baseName = oauth.buildAccountName(tokenInfo, form.name)
        const accountName = keys.length > 1 ? `${baseName} #${i + 1}` : baseName

        const credentials: Record<string, unknown> = { ...tokenInfo }
        applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
        if (tempUnschedEnabled.value) {
          credentials.temp_unschedulable_enabled = true
          credentials.temp_unschedulable_rules = tempUnschedPayload
        }

        await adminAPI.accounts.create({
          name: accountName,
          notes: form.notes,
          platform: form.platform,
          type: addMethod.value, // Use addMethod as type: 'oauth' or 'setup-token'
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          group_ids: form.group_ids,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })

        successCount++
      } catch (error: any) {
        failedCount++
        errors.push(
          t('admin.accounts.oauth.keyAuthFailed', {
            index: i + 1,
            error: error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
          })
        )
      }
    }

    if (successCount > 0) {
      appStore.showSuccess(t('admin.accounts.oauth.successCreated', { count: successCount }))
      if (failedCount === 0) {
        emit('created')
        handleClose()
      } else {
        emit('created')
      }
    }

    if (failedCount > 0) {
      oauth.error.value = errors.join('\n')
    }
  } catch (error: any) {
    oauth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.cookieAuthFailed')
  } finally {
    oauth.loading.value = false
  }
}
</script>
