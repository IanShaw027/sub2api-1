<template>
 <div v-show="activeTab === 'gateway'" class="settings-stack">
 <!-- Overload Cooldown (529) Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.overloadCooldown.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.overloadCooldown.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div
 v-if="overloadCooldownLoading"
 class="flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <div class="flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.overloadCooldown.enabled")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.overloadCooldown.enabledHint") }}
 </p>
 </div>
 <Toggle v-model="overloadCooldownForm.enabled" />
 </div>

 <div
 v-if="overloadCooldownForm.enabled"
 class="space-y-4 border-t border-line pt-4 "
 >
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.overloadCooldown.cooldownMinutes") }}
 </label>
 <input
 v-model.number="overloadCooldownForm.cooldown_minutes"
 type="number"
 min="1"
 max="120"
 class="input w-32"
 />
 <p class="settings-row-hint">
 {{
 t("admin.settings.overloadCooldown.cooldownMinutesHint")
 }}
 </p>
 </div>
 </div>

 <div
 class="flex justify-end border-t border-line pt-4 "
 >
 <button
 type="button"
 @click="saveOverloadCooldownSettings"
 :disabled="overloadCooldownSaving"
 class="btn-glass-primary"
 >
 <svg
 v-if="overloadCooldownSaving"
 class="mr-1 h-4 w-4 animate-spin"
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
 overloadCooldownSaving
 ? t("common.saving")
 : t("common.save")
 }}
 </button>
 </div>
 </template>
 </div>
 </div>

 <!-- Rate Limit Cooldown (429) Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.rateLimit429Cooldown.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.rateLimit429Cooldown.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div
 v-if="rateLimit429CooldownLoading"
 class="flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <div class="flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.rateLimit429Cooldown.enabled")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.rateLimit429Cooldown.enabledHint") }}
 </p>
 </div>
 <Toggle v-model="rateLimit429CooldownForm.enabled" />
 </div>

 <div
 v-if="rateLimit429CooldownForm.enabled"
 class="space-y-4 border-t border-line pt-4 "
 >
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.rateLimit429Cooldown.cooldownSeconds",
 )
 }}
 </label>
 <input
 v-model.number="rateLimit429CooldownForm.cooldown_seconds"
 type="number"
 min="1"
 max="7200"
 class="input w-32"
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.rateLimit429Cooldown.cooldownSecondsHint",
 )
 }}
 </p>
 </div>
 </div>

 <div v-else class="grid grid-cols-1 gap-4 border-t border-line pt-4 md:grid-cols-3">
 <label class="text-sm">冷却/重试间隔(ms)<input v-model.number="rateLimit429CooldownForm.retry_interval_ms" type="number" min="100" max="60000" class="input mt-2 w-full" /></label>
 <label class="text-sm">最大重试时间(秒)<input v-model.number="rateLimit429CooldownForm.retry_max_duration_seconds" type="number" min="1" max="600" class="input mt-2 w-full" /></label>
 <label class="text-sm">换号次数<input v-model.number="rateLimit429CooldownForm.max_account_switches" type="number" min="0" max="10" class="input mt-2 w-full" /></label>
 </div>

 <div
 class="flex justify-end border-t border-line pt-4 "
 >
 <button
 type="button"
 @click="saveRateLimit429CooldownSettings"
 :disabled="rateLimit429CooldownSaving"
 class="btn-glass-primary"
 >
 <svg
 v-if="rateLimit429CooldownSaving"
 class="mr-1 h-4 w-4 animate-spin"
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
 rateLimit429CooldownSaving
 ? t("common.saving")
 : t("common.save")
 }}
 </button>
 </div>
 </template>
 </div>
 </div>

 <!-- Stream Timeout Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.streamTimeout.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.streamTimeout.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Loading State -->
 <div
 v-if="streamTimeoutLoading"
 class="flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <!-- Enable Stream Timeout -->
 <div class="flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.streamTimeout.enabled")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.streamTimeout.enabledHint") }}
 </p>
 </div>
 <Toggle v-model="streamTimeoutForm.enabled" />
 </div>

 <!-- Settings - Only show when enabled -->
 <div
 v-if="streamTimeoutForm.enabled"
 class="space-y-4 border-t border-line pt-4 "
 >
 <!-- Action -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.streamTimeout.action") }}
 </label>
 <select
 v-model="streamTimeoutForm.action"
 class="input w-64"
 >
 <option value="temp_unsched">
 {{
 t("admin.settings.streamTimeout.actionTempUnsched")
 }}
 </option>
 <option value="error">
 {{ t("admin.settings.streamTimeout.actionError") }}
 </option>
 <option value="none">
 {{ t("admin.settings.streamTimeout.actionNone") }}
 </option>
 </select>
 <p class="settings-row-hint">
 {{ t("admin.settings.streamTimeout.actionHint") }}
 </p>
 </div>

 <!-- Temp Unsched Minutes (only show when action is temp_unsched) -->
 <div class="settings-row" v-if="streamTimeoutForm.action === 'temp_unsched'">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.streamTimeout.tempUnschedMinutes") }}
 </label>
 <input
 v-model.number="streamTimeoutForm.temp_unsched_minutes"
 type="number"
 min="1"
 max="60"
 class="input w-32"
 />
 <p class="settings-row-hint">
 {{
 t("admin.settings.streamTimeout.tempUnschedMinutesHint")
 }}
 </p>
 </div>

 <!-- Threshold Count -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.streamTimeout.thresholdCount") }}
 </label>
 <input
 v-model.number="streamTimeoutForm.threshold_count"
 type="number"
 min="1"
 max="10"
 class="input w-32"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.streamTimeout.thresholdCountHint") }}
 </p>
 </div>

 <!-- Threshold Window Minutes -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t("admin.settings.streamTimeout.thresholdWindowMinutes")
 }}
 </label>
 <input
 v-model.number="
 streamTimeoutForm.threshold_window_minutes
 "
 type="number"
 min="1"
 max="60"
 class="input w-32"
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.streamTimeout.thresholdWindowMinutesHint",
 )
 }}
 </p>
 </div>
 </div>

 <!-- Save Button -->
 <div
 class="flex justify-end border-t border-line pt-4 "
 >
 <button
 type="button"
 @click="saveStreamTimeoutSettings"
 :disabled="streamTimeoutSaving"
 class="btn-glass-primary"
 >
 <svg
 v-if="streamTimeoutSaving"
 class="mr-1 h-4 w-4 animate-spin"
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
 streamTimeoutSaving
 ? t("common.saving")
 : t("common.save")
 }}
 </button>
 </div>
 </template>
 </div>
 </div>

 <!-- Request Rectifier Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.rectifier.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.rectifier.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Loading State -->
 <div
 v-if="rectifierLoading"
 class="flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <!-- Master Toggle -->
 <div class="flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.rectifier.enabled")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.rectifier.enabledHint") }}
 </p>
 </div>
 <Toggle v-model="rectifierForm.enabled" />
 </div>

 <!-- Sub-toggles (only show when master is enabled) -->
 <div
 v-if="rectifierForm.enabled"
 class="space-y-4 border-t border-line pt-4 "
 >
 <!-- Thinking Signature Rectifier -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >{{
 t("admin.settings.rectifier.thinkingSignature")
 }}</label
 >
 <p class="text-xs text-muted ">
 {{
 t("admin.settings.rectifier.thinkingSignatureHint")
 }}
 </p>
 </div>
 <Toggle
 v-model="rectifierForm.thinking_signature_enabled"
 />
 </div>

 <!-- Thinking Budget Rectifier -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >{{
 t("admin.settings.rectifier.thinkingBudget")
 }}</label
 >
 <p class="text-xs text-muted ">
 {{ t("admin.settings.rectifier.thinkingBudgetHint") }}
 </p>
 </div>
 <Toggle v-model="rectifierForm.thinking_budget_enabled" />
 </div>

 <!-- API Key Signature Rectifier -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >{{
 t("admin.settings.rectifier.apikeySignature")
 }}</label
 >
 <p class="text-xs text-muted ">
 {{ t("admin.settings.rectifier.apikeySignatureHint") }}
 </p>
 </div>
 <Toggle v-model="rectifierForm.apikey_signature_enabled" />
 </div>

 <!-- Custom Patterns (only when apikey_signature_enabled) -->
 <div
 v-if="rectifierForm.apikey_signature_enabled"
 class="ml-4 space-y-3 border-l-2 border-line pl-4 "
 >
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >{{
 t("admin.settings.rectifier.apikeyPatterns")
 }}</label
 >
 <p class="text-xs text-muted ">
 {{ t("admin.settings.rectifier.apikeyPatternsHint") }}
 </p>
 </div>
 <div
 v-for="(
 _, index
 ) in rectifierForm.apikey_signature_patterns"
 :key="index"
 class="flex items-center gap-2"
 >
 <input
 v-model="rectifierForm.apikey_signature_patterns[index]"
 type="text"
 class="input input-sm flex-1"
 :placeholder="
 t('admin.settings.rectifier.apikeyPatternPlaceholder')
 "
 />
 <button
 type="button"
 @click="
 rectifierForm.apikey_signature_patterns.splice(
 index,
 1,
 )
 "
 class="btn btn-ghost btn-xs text-red-500 hover:text-red-700"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 stroke="currentColor"
 viewBox="0 0 24 24"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M6 18L18 6M6 6l12 12"
 />
 </svg>
 </button>
 </div>
 <button
 type="button"
 @click="rectifierForm.apikey_signature_patterns.push('')"
 class="btn btn-ghost btn-xs text-accent "
 >
 + {{ t("admin.settings.rectifier.addPattern") }}
 </button>
 </div>
 </div>

 <!-- Save Button -->
 <div
 class="flex justify-end border-t border-line pt-4 "
 >
 <button
 type="button"
 @click="saveRectifierSettings"
 :disabled="rectifierSaving"
 class="btn-glass-primary"
 >
 <svg
 v-if="rectifierSaving"
 class="mr-1 h-4 w-4 animate-spin"
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
 rectifierSaving ? t("common.saving") : t("common.save")
 }}
 </button>
 </div>
 </template>
 </div>
 </div>
 <!-- Beta Policy Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.betaPolicy.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.betaPolicy.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Loading State -->
 <div
 v-if="betaPolicyLoading"
 class="flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <!-- Rule Cards -->
 <div
 v-for="rule in betaPolicyForm.rules"
 :key="rule.beta_token"
 class="rounded-lg border border-line p-4 "
 >
 <div class="mb-3 flex items-center gap-2">
 <span
 class="text-sm font-medium text-foreground "
 >
 {{ getBetaDisplayName(rule.beta_token) }}
 </span>
 <span
 class="rounded bg-surface-2 px-2 py-0.5 text-xs text-muted "
 >
 {{ rule.beta_token }}
 </span>
 </div>

 <div class="settings-row-group">
 <!-- Action -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.betaPolicy.action") }}
 </label>
 <Select
 :modelValue="rule.action"
 @update:modelValue="rule.action = $event as any"
 :options="betaPolicyActionOptions"
 />
 </div>

 <!-- Scope -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.betaPolicy.scope") }}
 </label>
 <Select
 :modelValue="rule.scope"
 @update:modelValue="rule.scope = $event as any"
 :options="betaPolicyScopeOptions"
 />
 </div>
 </div>

 <!-- Error Message (only when action=block) -->
 <div v-if="rule.action === 'block'" class="mt-3">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.betaPolicy.errorMessage") }}
 </label>
 <input
 v-model="rule.error_message"
 type="text"
 class="input"
 :placeholder="
 t('admin.settings.betaPolicy.errorMessagePlaceholder')
 "
 />
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.betaPolicy.errorMessageHint") }}
 </p>
 </div>

 <!-- Quick Presets (only for tokens with presets) -->
 <div v-if="betaPresets[rule.beta_token]?.length" class="mt-3">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.betaPolicy.quickPresets") }}
 </label>
 <div class="flex flex-wrap gap-2">
 <button
 v-for="preset in betaPresets[rule.beta_token]"
 :key="preset.label"
 type="button"
 class="inline-flex items-center gap-1 rounded-md border border-accent bg-accent px-2.5 py-1 text-xs font-medium text-accent transition-colors hover:bg-accent "
 @click="applyBetaPreset(rule, preset)"
 :title="preset.description"
 >
 {{ preset.label }}
 </button>
 </div>
 </div>

 <!-- Model Whitelist -->
 <div class="mt-3">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.betaPolicy.modelWhitelist") }}
 </label>
 <p class="mb-2 text-xs text-muted ">
 {{ t("admin.settings.betaPolicy.modelWhitelistHint") }}
 </p>
 <!-- Existing patterns -->
 <div
 v-for="(_, index) in rule.model_whitelist || []"
 :key="index"
 class="mb-1.5 flex items-center gap-2"
 >
 <input
 v-model="rule.model_whitelist![index]"
 type="text"
 class="input input-sm flex-1"
 :placeholder="
 t('admin.settings.betaPolicy.modelPatternPlaceholder')
 "
 />
 <button
 type="button"
 @click="rule.model_whitelist!.splice(index, 1)"
 class="shrink-0 rounded p-1 text-red-400 transition-colors hover:bg-red-50 hover:text-red-600 "
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M6 18L18 6M6 6l12 12"
 />
 </svg>
 </button>
 </div>
 <!-- Add pattern button -->
 <button
 type="button"
 @click="
 if (!rule.model_whitelist) rule.model_whitelist = [];
 rule.model_whitelist.push('');
 "
 class="mb-2 inline-flex items-center gap-1 text-xs text-accent transition-colors hover:text-accent "
 >
 <svg
 class="h-3.5 w-3.5"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M12 4v16m8-8H4"
 />
 </svg>
 {{ t("admin.settings.betaPolicy.addModelPattern") }}
 </button>
 <!-- Common pattern chips -->
 <div class="flex flex-wrap items-center gap-1.5">
 <span class="text-xs text-muted "
 >{{
 t("admin.settings.betaPolicy.commonPatterns")
 }}:</span
 >
 <button
 v-for="pattern in commonModelPatterns"
 :key="pattern"
 type="button"
 class="rounded border border-line px-2 py-0.5 text-xs text-muted transition-colors hover:border-accent hover:bg-accent hover:text-accent "
 @click="addQuickPattern(rule, pattern)"
 >
 {{ pattern }}
 </button>
 </div>
 </div>

 <!-- Fallback Action (only when model_whitelist is non-empty) -->
 <div
 v-if="
 rule.model_whitelist && rule.model_whitelist.length > 0
 "
 class="mt-3"
 >
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.betaPolicy.fallbackAction") }}
 </label>
 <Select
 :modelValue="rule.fallback_action || 'pass'"
 @update:modelValue="rule.fallback_action = $event as any"
 :options="betaPolicyActionOptions"
 />
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.betaPolicy.fallbackActionHint") }}
 </p>
 <!-- Fallback Error Message (only when fallback_action=block) -->
 <div v-if="rule.fallback_action === 'block'" class="mt-2">
 <input
 v-model="rule.fallback_error_message"
 type="text"
 class="input"
 :placeholder="
 t(
 'admin.settings.betaPolicy.fallbackErrorMessagePlaceholder',
 )
 "
 />
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.betaPolicy.errorMessageHint") }}
 </p>
 </div>
 </div>
 </div>

 <!-- Save Button -->
 <div
 class="flex justify-end border-t border-line pt-4 "
 >
 <button
 type="button"
 @click="saveBetaPolicySettings"
 :disabled="betaPolicySaving"
 class="btn-glass-primary"
 >
 <svg
 v-if="betaPolicySaving"
 class="mr-1 h-4 w-4 animate-spin"
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
 betaPolicySaving ? t("common.saving") : t("common.save")
 }}
 </button>
 </div>
 </template>
 </div>
 </div>
 <!-- OpenAI Fast/Flex Policy Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.openaiFastPolicy.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.openaiFastPolicy.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Empty state -->
 <div
 v-if="openaiFastPolicyForm.rules.length === 0"
 class="rounded-lg border border-dashed border-line p-6 text-center text-sm text-muted "
 >
 {{ t("admin.settings.openaiFastPolicy.empty") }}
 </div>

 <!-- Rule Cards -->
 <div
 v-for="(rule, ruleIndex) in openaiFastPolicyForm.rules"
 :key="ruleIndex"
 class="rounded-lg border border-line p-4 "
 >
 <div class="mb-3 flex items-center justify-between">
 <span
 class="text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.openaiFastPolicy.ruleHeader", {
 index: ruleIndex + 1,
 })
 }}
 </span>
 <button
 type="button"
 @click="removeOpenAIFastPolicyRule(ruleIndex)"
 class="rounded p-1 text-red-400 transition-colors hover:bg-red-50 hover:text-red-600 "
 :title="t('admin.settings.openaiFastPolicy.removeRule')"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M6 18L18 6M6 6l12 12"
 />
 </svg>
 </button>
 </div>

 <div
 class="mb-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted "
 :data-testid="`openai-fast-policy-summary-${ruleIndex}`"
 >
 <span class="font-medium text-foreground ">
 {{
 t(
 hasOpenAIFastPolicyTargetModels(rule)
 ? "admin.settings.openaiFastPolicy.summaryTargetModels"
 : "admin.settings.openaiFastPolicy.summaryAllModels",
 )
 }}
 </span>
 <span aria-hidden="true">→</span>
 <span
 class="inline-flex items-center rounded bg-accent px-2 py-0.5 font-medium text-accent "
 >
 {{ openaiFastPolicyActionSummary(rule.action) }}
 </span>
 <template v-if="hasOpenAIFastPolicyTargetModels(rule)">
 <span aria-hidden="true">·</span>
 <span class="font-medium text-foreground ">
 {{
 t(
 "admin.settings.openaiFastPolicy.summaryOtherModels",
 )
 }}
 </span>
 <span aria-hidden="true">→</span>
 <span
 class="inline-flex items-center rounded bg-surface-2 px-2 py-0.5 font-medium text-foreground "
 >
 {{
 openaiFastPolicyActionSummary(
 rule.fallback_action || "pass",
 )
 }}
 </span>
 </template>
 </div>

 <div class="settings-row-group">
 <!-- Service Tier -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.serviceTier") }}
 </label>
 <Select
 :modelValue="rule.service_tier"
 @update:modelValue="
 rule.service_tier = $event as
 | 'all'
 | 'priority'
 | 'flex'
 "
 :options="openaiFastPolicyTierOptions"
 />
 </div>

 <!-- Action -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.action") }}
 </label>
 <Select
 :modelValue="rule.action"
 @update:modelValue="
 rule.action = $event as
 | 'pass'
 | 'filter'
 | 'block'
 | 'force_priority'
 "
 :options="openaiFastPolicyActionOptions"
 />
 </div>

 <!-- Scope -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.scope") }}
 </label>
 <Select
 :modelValue="rule.scope"
 @update:modelValue="
 rule.scope = $event as
 | 'all'
 | 'oauth'
 | 'apikey'
 | 'bedrock'
 "
 :options="openaiFastPolicyScopeOptions"
 />
 </div>
 </div>

 <!-- User Scope -->
 <div class="mt-3">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.userIds") }}
 </label>
 <p class="mb-2 text-xs text-muted ">
 {{ t("admin.settings.openaiFastPolicy.userIdsHint") }}
 </p>
 <OpenAIFastPolicyUserSelector
 :model-value="rule.user_ids || []"
 @update:model-value="rule.user_ids = $event"
 />
 </div>

 <!-- Error Message (only when action=block) -->
 <div v-if="rule.action === 'block'" class="mt-3">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.errorMessage") }}
 </label>
 <input
 v-model="rule.error_message"
 type="text"
 class="input"
 :placeholder="
 t(
 'admin.settings.openaiFastPolicy.errorMessagePlaceholder',
 )
 "
 />
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.openaiFastPolicy.errorMessageHint") }}
 </p>
 </div>

 <!-- Target Models -->
 <div
 class="mt-3"
 role="group"
 :aria-labelledby="`openai-fast-policy-models-label-${ruleIndex}`"
 :aria-describedby="`openai-fast-policy-models-hint-${ruleIndex}`"
 >
 <label
 :id="`openai-fast-policy-models-label-${ruleIndex}`"
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.modelWhitelist") }}
 </label>
 <p
 :id="`openai-fast-policy-models-hint-${ruleIndex}`"
 class="mb-2 text-xs text-muted "
 >
 {{
 t("admin.settings.openaiFastPolicy.modelWhitelistHint")
 }}
 </p>
 <div
 v-for="(_, patternIdx) in rule.model_whitelist || []"
 :key="patternIdx"
 class="mb-1.5 flex items-center gap-2"
 >
 <input
 v-model="rule.model_whitelist![patternIdx]"
 type="text"
 class="input input-sm flex-1"
 :placeholder="
 t(
 'admin.settings.openaiFastPolicy.modelPatternPlaceholder',
 )
 "
 />
 <button
 type="button"
 @click="
 removeOpenAIFastPolicyModelPattern(rule, patternIdx)
 "
 class="shrink-0 rounded p-1 text-red-400 transition-colors hover:bg-red-50 hover:text-red-600 "
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M6 18L18 6M6 6l12 12"
 />
 </svg>
 </button>
 </div>
 <button
 type="button"
 @click="addOpenAIFastPolicyModelPattern(rule)"
 class="mb-2 inline-flex items-center gap-1 text-xs text-accent transition-colors hover:text-accent "
 >
 <svg
 class="h-3.5 w-3.5"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M12 4v16m8-8H4"
 />
 </svg>
 {{ t("admin.settings.openaiFastPolicy.addModelPattern") }}
 </button>
 </div>

 <!-- Other Models Action (only when target models are non-empty) -->
 <div
 v-if="hasOpenAIFastPolicyTargetModels(rule)"
 class="mt-3"
 >
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.openaiFastPolicy.fallbackAction") }}
 </label>
 <Select
 :modelValue="rule.fallback_action || 'pass'"
 @update:modelValue="
 rule.fallback_action = $event as
 | 'pass'
 | 'filter'
 | 'block'
 | 'force_priority'
 "
 :options="openaiFastPolicyActionOptions"
 />
 <p class="mt-1 text-xs text-muted ">
 {{
 t("admin.settings.openaiFastPolicy.fallbackActionHint")
 }}
 </p>
 <div v-if="rule.fallback_action === 'block'" class="mt-2">
 <input
 v-model="rule.fallback_error_message"
 type="text"
 class="input"
 :placeholder="
 t(
 'admin.settings.openaiFastPolicy.fallbackErrorMessagePlaceholder',
 )
 "
 />
 </div>
 </div>
 </div>

 <!-- Add Rule Button -->
 <div>
 <button
 type="button"
 @click="addOpenAIFastPolicyRule"
 class="btn-glass-secondary inline-flex items-center gap-1"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M12 4v16m8-8H4"
 />
 </svg>
 {{ t("admin.settings.openaiFastPolicy.addRule") }}
 </button>
 <p class="mt-2 text-xs text-muted ">
 {{ t("admin.settings.openaiFastPolicy.saveHint") }}
 </p>
 </div>
 </div>
 </div>
 </div>
 <div v-show="activeTab === 'gateway'" class="settings-stack">
 <!-- Claude Code Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.claudeCode.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.claudeCode.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.claudeCode.minVersion") }}
 </label>
 <input
 v-model="form.min_claude_code_version"
 type="text"
 class="input max-w-xs font-mono text-sm"
 :placeholder="
 t('admin.settings.claudeCode.minVersionPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.claudeCode.minVersionHint") }}
 </p>
 </div>
 <div class="settings-row mt-4">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.claudeCode.maxVersion") }}
 </label>
 <input
 v-model="form.max_claude_code_version"
 type="text"
 class="input max-w-xs font-mono text-sm"
 :placeholder="
 t('admin.settings.claudeCode.maxVersionPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.claudeCode.maxVersionHint") }}
 </p>
 </div>
 </div>
 </div>

 <!-- Kiro Runtime Defaults -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.kiroRuntime.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.kiroRuntime.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.kiroVersion") }}
 </label>
 <input
 v-model="form.kiro_version"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-version"
 :placeholder="t('admin.settings.kiroRuntime.kiroVersionPlaceholder')"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.kiroCommit") }}
 </label>
 <input
 v-model="form.kiro_commit"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-commit"
 :placeholder="t('admin.settings.kiroRuntime.kiroCommitPlaceholder')"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.systemVersion") }}
 </label>
 <input
 v-model="form.system_version"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-system-version"
 :placeholder="t('admin.settings.kiroRuntime.systemVersionPlaceholder')"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.nodeVersion") }}
 </label>
 <input
 v-model="form.node_version"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-node-version"
 :placeholder="t('admin.settings.kiroRuntime.nodeVersionPlaceholder')"
 />
 </div>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.kiroRuntime.codeExecutionSandboxCommand",
 )
 }}
 </label>
 <textarea
 v-model="form.kiro_code_execution_sandbox_command"
 rows="3"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-code-execution-sandbox-command"
 :placeholder="
 t(
 'admin.settings.kiroRuntime.codeExecutionSandboxCommandPlaceholder',
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.kiroRuntime.codeExecutionSandboxCommandHint",
 )
 }}
 </p>
 <div
 class="mt-2 rounded-lg border border-red-200 bg-red-50 p-3 text-xs leading-5 text-red-700 "
 >
 {{
 t(
 "admin.settings.kiroRuntime.codeExecutionSandboxCommandWarning",
 )
 }}
 </div>
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.cacheHitRateScale") }}
 </label>
 <input
 :value="form.cache_hit_rate_scale ?? ''"
 type="number"
 min="0"
 max="100"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-hit-rate-scale"
 :placeholder="t('admin.settings.kiroRuntime.cacheHitRateScalePlaceholder')"
 @input="
 form.cache_hit_rate_scale = parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.kiroRuntime.cacheHitRateScaleHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.cacheMinBlockTokens") }}
 </label>
 <input
 :value="form.cache_min_block_tokens ?? ''"
 type="number"
 min="0"
 :max="KIRO_CACHE_MIN_BLOCK_TOKENS_MAX"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-min-block-tokens"
 :placeholder="`0 - ${KIRO_CACHE_MIN_BLOCK_TOKENS_MAX}`"
 @input="
 form.cache_min_block_tokens = parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t("admin.settings.kiroRuntime.cacheMinBlockTokensHint")
 }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.kiroRuntime.cacheIndependentTtlSeconds",
 )
 }}
 </label>
 <input
 :value="form.cache_independent_ttl_seconds ?? ''"
 type="number"
 min="60"
 max="86400"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-independent-ttl-seconds"
 :placeholder="t('admin.settings.kiroRuntime.cacheIndependentTtlSecondsPlaceholder')"
 @input="
 form.cache_independent_ttl_seconds =
 parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.kiroRuntime.cacheIndependentTtlSecondsHint",
 )
 }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.cachePrefixTtlSeconds") }}
 </label>
 <input
 :value="form.cache_prefix_ttl_seconds ?? ''"
 type="number"
 min="60"
 max="3600"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-prefix-ttl-seconds"
 :placeholder="t('admin.settings.kiroRuntime.cachePrefixTtlSecondsPlaceholder')"
 @input="
 form.cache_prefix_ttl_seconds = parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.kiroRuntime.cachePrefixTtlSecondsHint",
 )
 }}
 </p>
 </div>
 </div>
 </div>
 </div>

 <!-- Codex Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.gatewayForwarding.codexHardeningTitle") }}
 </h2>
 </div>
 <div class="settings-card-body">
 <div>
 <h3 class="text-base font-semibold text-foreground ">
 {{ t("admin.settings.gatewayForwarding.codexClientRestrictionTitle") }}
 </h3>
 <p class="settings-card-desc">
 {{ t("admin.settings.gatewayForwarding.codexHardeningDesc") }}
 </p>
 </div>
 <div class="grid gap-4 sm:grid-cols-2">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.gatewayForwarding.minCodexVersion") }}
 </label>
 <input
 v-model="form.min_codex_version"
 type="text"
 class="input w-full font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.minCodexVersionPlaceholder',
 )
 "
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.gatewayForwarding.maxCodexVersion") }}
 </label>
 <input
 v-model="form.max_codex_version"
 type="text"
 class="input w-full font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.maxCodexVersionPlaceholder',
 )
 "
 />
 </div>
 </div>
 <p class="text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexVersionHint") }}
 </p>

 <div>
 <label class="block text-sm font-medium text-foreground ">
 {{ t("admin.settings.gatewayForwarding.codexFingerprintSignals") }}
 </label>
 <p class="mb-2 mt-1 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexFingerprintSignalsDesc") }}
 </p>
 <div
 v-for="(row, i) in codexFingerprintRows"
 :key="`codex-fp-${i}`"
 class="mb-2 flex items-center gap-2"
 >
 <select v-model="row.type" class="input w-32 text-sm">
 <option value="header_exact">{{ t("admin.settings.gatewayForwarding.codexFpTypeHeaderExact") }}</option>
 <option value="header_prefix">{{ t("admin.settings.gatewayForwarding.codexFpTypeHeaderPrefix") }}</option>
 <option value="body_path">{{ t("admin.settings.gatewayForwarding.codexFpTypeBodyPath") }}</option>
 </select>
 <input
 v-model="row.match"
 type="text"
 class="input flex-1 font-mono text-sm"
 :placeholder="t('admin.settings.gatewayForwarding.codexFpMatchPlaceholder')"
 />
 <label class="flex shrink-0 items-center gap-1 text-xs text-muted ">
 <input v-model="row.required" type="checkbox" />
 {{ t("admin.settings.gatewayForwarding.codexFpRequired") }}
 </label>
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-red-600 hover:text-red-700 "
 @click="removeCodexFingerprintRow(i)"
 >
 {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
 </button>
 </div>
 <button type="button" class="btn-glass-secondary" @click="addCodexFingerprintRow">
 {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
 </button>
 <p
 v-if="codexFingerprintNoRequired"
 class="mt-2 text-xs text-amber-600 "
 >
 {{ t("admin.settings.gatewayForwarding.codexFingerprintNoRequiredWarn") }}
 </p>
 </div>

 <div class="flex items-center justify-between">
 <div class="pr-4">
 <label
 class="block text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.gatewayForwarding.codexAllowAppServer")
 }}
 </label>
 <p class="mt-1 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.codexAllowAppServerDesc",
 )
 }}
 </p>
 </div>
 <Toggle
 v-model="form.codex_cli_only_allow_app_server_clients"
 />
 </div>

 <div>
 <label
 class="block text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.codexBlacklist") }}
 </label>
 <p class="mb-2 mt-1 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexBlacklistDesc") }}
 </p>
 <div
 v-for="(row, i) in codexBlacklistRows"
 :key="`codex-bl-${i}`"
 class="mb-2 flex gap-2"
 >
 <input
 v-model="row.originator"
 type="text"
 class="input w-1/3 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexOriginatorPlaceholder',
 )
 "
 />
 <input
 v-model="row.uaContains"
 type="text"
 class="input flex-1 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexUaContainsPlaceholder',
 )
 "
 />
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-red-600 hover:text-red-700 "
 @click="removeCodexBlacklistRow(i)"
 >
 {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
 </button>
 </div>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addCodexBlacklistRow"
 >
 {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
 </button>
 </div>

 <div>
 <label
 class="block text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.codexWhitelist") }}
 </label>
 <p class="mb-2 mt-1 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexWhitelistDesc") }}
 </p>
 <div
 v-for="(row, i) in codexWhitelistRows"
 :key="`codex-wl-${i}`"
 class="mb-2 flex gap-2"
 >
 <input
 v-model="row.originator"
 type="text"
 class="input w-1/3 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexOriginatorPlaceholder',
 )
 "
 />
 <input
 v-model="row.uaContains"
 type="text"
 class="input flex-1 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexUaContainsPlaceholder',
 )
 "
 />
 <label
 class="flex shrink-0 items-center gap-1 text-xs text-muted "
 :title="
 t(
 'admin.settings.gatewayForwarding.codexWhitelistSkipFingerprintTooltip',
 )
 "
 >
 <input
 v-model="row.skipEngineFingerprint"
 type="checkbox"
 />
 {{
 t(
 'admin.settings.gatewayForwarding.codexWhitelistSkipFingerprint',
 )
 }}
 </label>
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-red-600 hover:text-red-700 "
 @click="removeCodexWhitelistRow(i)"
 >
 {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
 </button>
 </div>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addCodexWhitelistRow"
 >
 {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
 </button>
 </div>
 </div>
 </div>

 <!-- Upstream Billing Probe Settings -->
 <div class="glass-card settings-card" data-testid="upstream-billing-probe-settings">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.upstreamBillingProbe.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.upstreamBillingProbe.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div
 v-if="upstreamBillingProbeLoading"
 class="flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <div class="flex items-center justify-between gap-4">
 <div>
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.upstreamBillingProbe.enabled") }}
 </label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.upstreamBillingProbe.enabledHint") }}
 </p>
 </div>
 <Toggle
 v-model="upstreamBillingProbeForm.enabled"
 :aria-label="t('admin.settings.upstreamBillingProbe.enabled')"
 data-testid="upstream-billing-probe-enabled"
 />
 </div>

 <div
 v-if="upstreamBillingProbeForm.enabled"
 class="settings-row border-t border-line pt-4 "
 >
 <label
 class="settings-row-label"
 for="upstream-billing-probe-interval"
 >
 {{ t("admin.settings.upstreamBillingProbe.intervalMinutes") }}
 </label>
 <input
 id="upstream-billing-probe-interval"
 v-model.number="upstreamBillingProbeForm.interval_minutes"
 type="number"
 min="5"
 max="1440"
 class="input w-32"
 data-testid="upstream-billing-probe-interval"
 @keydown.enter.prevent="saveUpstreamBillingProbeSettings"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.upstreamBillingProbe.intervalHint") }}
 </p>
 </div>

 <div
 class="flex justify-end border-t border-line pt-4 "
 >
 <button
 type="button"
 class="btn-glass-primary"
 :disabled="upstreamBillingProbeSaving"
 data-testid="upstream-billing-probe-save"
 @click="saveUpstreamBillingProbeSettings"
 >
 {{
 upstreamBillingProbeSaving
 ? t("common.saving")
 : t("common.save")
 }}
 </button>
 </div>
 </template>
 </div>
 </div>

 <!-- Ollama Cloud Usage Settings -->
 <div class="glass-card settings-card" data-testid="ollama-cloud-usage-global-settings">
 <div class="settings-card-head">
 <h2 class="settings-card-title">
 {{ t("admin.settings.ollamaCloudUsage.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.ollamaCloudUsage.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div v-if="ollamaCloudUsageLoading" class="flex items-center gap-2 text-muted">
 <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"></div>
 {{ t("common.loading") }}
 </div>
 <template v-else>
 <div class="flex items-center justify-between gap-4">
 <div>
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.ollamaCloudUsage.enabled") }}
 </label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.ollamaCloudUsage.enabledHint") }}
 </p>
 </div>
 <Toggle
 v-model="ollamaCloudUsageForm.enabled"
 :aria-label="t('admin.settings.ollamaCloudUsage.enabled')"
 data-testid="ollama-cloud-usage-global-enabled"
 />
 </div>
 <div v-if="ollamaCloudUsageForm.enabled" class="space-y-4 border-t border-line pt-4 ">
 <div class="settings-row">
 <label class="settings-row-label" for="ollama-cloud-usage-debounce">
 {{ t("admin.settings.ollamaCloudUsage.debounceMinutes") }}
 </label>
 <input
 id="ollama-cloud-usage-debounce"
 v-model.number="ollamaCloudUsageForm.debounce_minutes"
 type="number"
 min="1"
 max="60"
 class="input w-32"
 data-testid="ollama-cloud-usage-global-debounce"
 @keydown.enter.prevent="saveOllamaCloudUsageSettings"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.ollamaCloudUsage.debounceHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label class="settings-row-label" for="ollama-cloud-usage-interval">
 {{ t("admin.settings.ollamaCloudUsage.intervalMinutes") }}
 </label>
 <input
 id="ollama-cloud-usage-interval"
 v-model.number="ollamaCloudUsageForm.interval_minutes"
 type="number"
 min="15"
 max="1440"
 class="input w-32"
 data-testid="ollama-cloud-usage-global-interval"
 @keydown.enter.prevent="saveOllamaCloudUsageSettings"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.ollamaCloudUsage.intervalHint") }}
 </p>
 </div>
 </div>
 <div class="flex justify-end border-t border-line pt-4 ">
 <button
 type="button"
 class="btn-glass-primary"
 :disabled="ollamaCloudUsageSaving"
 data-testid="ollama-cloud-usage-global-save"
 @click="saveOllamaCloudUsageSettings"
 >
 {{ ollamaCloudUsageSaving ? t("common.saving") : t("common.save") }}
 </button>
 </div>
 </template>
 </div>
 </div>

 <!-- Gateway Scheduling Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.scheduling.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.scheduling.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.scheduling.allowUngroupedKey") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.scheduling.allowUngroupedKeyHint") }}
 </p>
 </div>
 <Toggle v-model="form.allow_ungrouped_key_scheduling" />
 </div>

 <div class="border-t border-line pt-4 ">
 <div class="mb-3">
 <label class="font-medium text-foreground ">
 {{
 t(
 "admin.settings.scheduling.accountSchedulingThresholdsTitle",
 )
 }}
 </label>
 <p class="settings-card-desc">
 {{
 t(
 "admin.settings.scheduling.accountSchedulingThresholdsDescription",
 )
 }}
 </p>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.scheduling.accountSchedulingThresholdsGlobalHint",
 )
 }}
 </p>
 <p class="mt-0.5 text-xs text-amber-600 ">
 {{
 t(
 "admin.settings.scheduling.accountSchedulingThresholdsDisabledHint",
 )
 }}
 </p>
 </div>
 <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
 <div
 v-for="platform in schedulingThresholdPlatforms"
 :key="platform"
 class="rounded-lg border border-line p-4 "
 >
 <div class="flex items-start justify-between gap-3">
 <div>
 <label
 class="font-mono text-sm font-medium text-foreground "
 >
 {{ platform }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.scheduling.accountSchedulingThresholdsRangeHint",
 )
 }}
 </p>
 </div>
 <span
 class="rounded bg-surface-2 px-2 py-0.5 text-[11px] font-medium text-muted "
 >
 %
 </span>
 </div>
 <input
 v-model.number="form.account_scheduling_thresholds[platform]"
 type="number"
 min="1"
 max="100"
 step="1"
 class="input mt-3"
 :data-testid="`account-scheduling-threshold-${platform}`"
 placeholder="100"
 />
 </div>
 </div>
 </div>

 <div
 v-if="!form.openai_advanced_scheduler_enabled"
 class="flex items-center justify-between border-t border-line pt-5 "
 >
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.openaiExperimentalScheduler.lowRatePriorityTitle") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t("admin.settings.openaiExperimentalScheduler.lowRatePriorityDescription")
 }}
 </p>
 </div>
 <Toggle
 v-model="form.openai_low_upstream_rate_priority_enabled"
 data-testid="openai-low-rate-priority-toggle"
 />
 </div>

 <div
 v-if="!form.openai_advanced_scheduler_enabled && form.openai_low_upstream_rate_priority_enabled"
 class="flex flex-col items-stretch gap-3 border-t border-line pt-5 sm:flex-row sm:items-start sm:justify-between sm:gap-6 "
 >
 <div class="min-w-0">
 <label
 class="text-sm font-medium text-foreground "
 for="openai-oauth-scheduling-rate-multiplier"
 >
 {{ t("admin.settings.openaiExperimentalScheduler.oauthRateTitle") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.openaiExperimentalScheduler.oauthRatePriorityDescription") }}
 </p>
 </div>
 <div class="relative w-full shrink-0 sm:w-32">
 <input
 id="openai-oauth-scheduling-rate-multiplier"
 v-model.number="form.openai_oauth_scheduling_rate_multiplier"
 class="input pr-8"
 data-testid="openai-oauth-scheduling-rate-multiplier"
 min="0"
 required
 step="0.01"
 type="number"
 />
 <span
 class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted"
 >x</span>
 </div>
 </div>

 <div class="flex items-center justify-between border-t border-line pt-5 ">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.openaiExperimentalScheduler.title") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t("admin.settings.openaiExperimentalScheduler.description")
 }}
 </p>
 </div>
 <Toggle
 v-model="form.openai_advanced_scheduler_enabled"
 data-testid="openai-advanced-scheduler-toggle"
 />
 </div>

 <div
 v-if="form.openai_advanced_scheduler_enabled"
 class="flex items-center justify-between border-t border-line pt-5 "
 >
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.openaiExperimentalScheduler.stickyWeightedTitle") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t("admin.settings.openaiExperimentalScheduler.stickyWeightedDescription")
 }}
 </p>
 </div>
 <Toggle v-model="form.openai_advanced_scheduler_sticky_weighted_enabled" />
 </div>

 <div
 v-if="form.openai_advanced_scheduler_enabled"
 class="flex items-center justify-between border-t border-line pt-5 "
 >
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.openaiExperimentalScheduler.subscriptionPriorityTitle") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t("admin.settings.openaiExperimentalScheduler.subscriptionPriorityDescription")
 }}
 </p>
 </div>
 <Toggle v-model="form.openai_advanced_scheduler_subscription_priority_enabled" />
 </div>

 <div
 v-if="form.openai_advanced_scheduler_enabled"
 class="flex flex-col items-stretch gap-3 border-t border-line pt-5 sm:flex-row sm:items-start sm:justify-between sm:gap-6 "
 >
 <div class="min-w-0">
 <label
 class="text-sm font-medium text-foreground "
 for="openai-oauth-scheduling-rate-multiplier"
 >
 {{ t("admin.settings.openaiExperimentalScheduler.oauthRateTitle") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.openaiExperimentalScheduler.oauthRateWeightedDescription") }}
 </p>
 </div>
 <div class="relative w-full shrink-0 sm:w-32">
 <input
 id="openai-oauth-scheduling-rate-multiplier"
 v-model.number="form.openai_oauth_scheduling_rate_multiplier"
 class="input pr-8"
 data-testid="openai-oauth-scheduling-rate-multiplier"
 min="0"
 required
 step="0.01"
 type="number"
 />
 <span
 class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted"
 >x</span>
 </div>
 </div>

 <div
 v-if="form.openai_advanced_scheduler_enabled"
 class="border-t border-line pt-5 "
 >
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.openaiExperimentalScheduler.weightsTitle") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t("admin.settings.openaiExperimentalScheduler.weightsDescription")
 }}
 </p>
 </div>

 <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-5">
 <label
 v-for="field in openAIAdvancedSchedulerWeightFields"
 :key="field.key"
 class="block"
 >
 <span class="text-xs font-medium text-muted ">
 {{ field.label }}
 </span>
 <input
 v-model="form[field.key]"
 class="input mt-1"
 inputmode="decimal"
 :placeholder="field.placeholder"
 type="text"
 />
 </label>
 </div>
 </div>
 </div>
 </div>

 <!-- Gateway Forwarding Behavior -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.gatewayForwarding.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.gatewayForwarding.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="grid gap-5 border-b border-line pb-5 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
 <div>
 <label
 for="grok-default-text-model"
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.grokDefaultTextModel") }}
 </label>
 <input
 id="grok-default-text-model"
 v-model.trim="form.grok_default_text_model"
 type="text"
 class="input mt-2 w-full"
 list="grok-default-text-model-options"
 data-testid="grok-default-text-model"
 placeholder="grok-4.5"
 />
 <datalist id="grok-default-text-model-options">
 <option value="grok-4.5" />
 <option value="grok-4.1-fast" />
 <option value="grok-4" />
 </datalist>
 <p class="settings-row-hint">
 {{ t("admin.settings.gatewayForwarding.grokDefaultTextModelHint") }}
 </p>
 </div>
 <div class="flex items-center justify-between gap-5 md:min-w-72">
 <div>
 <label class="text-sm font-medium text-foreground ">
 {{ t("admin.settings.gatewayForwarding.grokCrossClientMap") }}
 </label>
 <p class="mt-0.5 max-w-sm text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.grokCrossClientMapHint") }}
 </p>
 </div>
 <Toggle
 v-model="form.grok_cross_client_model_map_enabled"
 data-testid="grok-cross-client-model-map-toggle"
 />
 </div>
 </div>
 <div class="md:col-span-2">
 <label
 for="grok-default-base-url-mode"
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.grokDefaultBaseURLMode") }}
 </label>
 <select
 id="grok-default-base-url-mode"
 v-model="form.grok_default_base_url_mode"
 class="input mt-2 w-full"
 data-testid="grok-default-base-url-mode"
 >
 <option value="cli">{{ t("admin.settings.gatewayForwarding.grokBaseURLModeCLI") }}</option>
 <option value="api">{{ t("admin.settings.gatewayForwarding.grokBaseURLModeAPI") }}</option>
 <option value="us-east-1">{{ t("admin.settings.gatewayForwarding.grokBaseURLModeUSEast1") }}</option>
 <option value="us-west-2">{{ t("admin.settings.gatewayForwarding.grokBaseURLModeUSWest2") }}</option>
 <option value="eu-west-1">{{ t("admin.settings.gatewayForwarding.grokBaseURLModeEUWest1") }}</option>
 </select>
 <p class="settings-row-hint">
 {{ t("admin.settings.gatewayForwarding.grokDefaultBaseURLModeHint") }}
 </p>
 </div>

 <!-- OpenAI Responses 首 token 统计 -->
 <div class="border-b border-line pb-5 md:col-span-2">
 <label
 for="openai-ttft-mode"
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.openaiTTFTMode") }}
 </label>
 <select
 id="openai-ttft-mode"
 v-model="form.openai_ttft_mode"
 class="input mt-2 w-full"
 data-testid="openai-ttft-mode"
 >
 <option value="semantic">
 {{ t("admin.settings.gatewayForwarding.openaiTTFTModeSemantic") }}
 </option>
 <option value="visible">
 {{ t("admin.settings.gatewayForwarding.openaiTTFTModeVisible") }}
 </option>
 </select>
 <p class="settings-row-hint">
 {{ t("admin.settings.gatewayForwarding.openaiTTFTModeHint") }}
 </p>
 </div>

 <!-- Fingerprint Unification -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.fingerprintUnification",
 )
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.fingerprintUnificationHint",
 )
 }}
 </p>
 </div>
 <Toggle v-model="form.enable_fingerprint_unification" />
 </div>

 <!-- Metadata Passthrough -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.gatewayForwarding.metadataPassthrough")
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.metadataPassthroughHint",
 )
 }}
 </p>
 </div>
 <Toggle v-model="form.enable_metadata_passthrough" />
 </div>

 <!-- CCH Signing -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.cchSigning") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.cchSigningHint") }}
 </p>
 </div>
 <Toggle v-model="form.enable_cch_signing" />
 </div>

 <!-- Claude OAuth System Prompt Injection -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.claudeOAuthSystemPromptInjection",
 )
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.claudeOAuthSystemPromptInjectionHint",
 )
 }}
 </p>
 </div>
 <Toggle
 v-model="form.enable_claude_oauth_system_prompt_injection"
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.gatewayForwarding.claudeOAuthSystemPromptBlocks",
 )
 }}
 </label>
 <div class="space-y-3">
 <div
 v-for="(block, index) in claudeOAuthSystemPromptBlocks"
 :key="block.id"
 class="rounded-lg border border-line bg-surface-2 p-4 "
 >
 <div
 :class="[
 'flex flex-wrap items-center justify-between gap-3',
 block.expanded && 'mb-3',
 ]"
 >
 <div class="min-w-0">
 <div
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.systemBlockTitle",
 { index: index + 1 },
 )
 }}
 </div>
 <div
 class="mt-0.5 text-xs text-muted "
 >
 {{ getClaudeOAuthPresetLabel(block.preset) }}
 </div>
 </div>
 <div class="flex items-center gap-2">
 <button
 type="button"
 class="btn-glass-secondary px-2"
 :title="
 block.expanded
 ? t(
 'admin.settings.gatewayForwarding.systemBlockHide',
 )
 : t(
 'admin.settings.gatewayForwarding.systemBlockShow',
 )
 "
 :aria-label="
 block.expanded
 ? t(
 'admin.settings.gatewayForwarding.systemBlockHide',
 )
 : t(
 'admin.settings.gatewayForwarding.systemBlockShow',
 )
 "
 @click="toggleClaudeOAuthSystemPromptBlock(index)"
 >
 <Icon
 :name="block.expanded ? 'eyeOff' : 'eye'"
 size="xs"
 />
 </button>
 <button
 type="button"
 class="btn-glass-secondary px-2"
 :disabled="index === 0"
 @click="moveClaudeOAuthSystemPromptBlock(index, -1)"
 >
 <Icon name="arrowUp" size="xs" />
 </button>
 <button
 type="button"
 class="btn-glass-secondary px-2"
 :disabled="
 index === claudeOAuthSystemPromptBlocks.length - 1
 "
 @click="moveClaudeOAuthSystemPromptBlock(index, 1)"
 >
 <Icon name="arrowDown" size="xs" />
 </button>
 <Toggle v-model="block.enabled" />
 <button
 type="button"
 class="btn-glass-secondary px-2 text-red-600 hover:text-red-700 "
 @click="removeClaudeOAuthSystemPromptBlock(index)"
 >
 <Icon name="trash" size="xs" />
 </button>
 </div>
 </div>

 <div v-show="block.expanded">
 <div class="grid gap-3 md:grid-cols-2">
 <div>
 <label
 class="settings-sub-label"
 >
 {{
 t(
 "admin.settings.gatewayForwarding.systemBlockPreset",
 )
 }}
 </label>
 <Select
 v-model="block.preset"
 :options="claudeOAuthSystemPromptPresetOptions"
 @change="
 (value) =>
 applyClaudeOAuthSystemPromptPreset(index, value)
 "
 />
 </div>
 <div>
 <label
 class="settings-sub-label"
 >
 {{
 t(
 "admin.settings.gatewayForwarding.systemBlockType",
 )
 }}
 </label>
 <Select
 v-model="block.type"
 :options="claudeOAuthSystemPromptBlockTypeOptions"
 />
 </div>
 </div>

 <div class="mt-3">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.gatewayForwarding.systemBlockText") }}
 </label>
 <textarea
 v-model="block.text"
 rows="6"
 class="input w-full resize-y font-mono text-xs leading-5"
 @input="markClaudeOAuthSystemPromptBlockCustom(block)"
 />
 </div>

 <div
 class="mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_160px]"
 >
 <div class="flex items-center justify-between gap-4">
 <div>
 <label
 class="text-xs font-medium text-muted "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.systemBlockCacheControl",
 )
 }}
 </label>
 </div>
 <Toggle v-model="block.cacheControlEnabled" />
 </div>
 <div v-if="block.cacheControlEnabled">
 <Select
 v-model="block.cacheControlTTL"
 :options="claudeOAuthSystemPromptCacheTTLOptions"
 />
 </div>
 </div>
 </div>
 </div>
 </div>

 <div class="mt-3 flex flex-wrap gap-2">
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addClaudeOAuthSystemPromptBlock"
 >
 <Icon name="plus" size="xs" />
 {{ t("admin.settings.gatewayForwarding.addSystemBlock") }}
 </button>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="resetClaudeOAuthSystemPromptBlocks"
 >
 <Icon name="refresh" size="xs" />
 {{
 t("admin.settings.gatewayForwarding.resetSystemBlocks")
 }}
 </button>
 </div>
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.gatewayForwarding.claudeOAuthSystemPromptBlocksHint",
 )
 }}
 </p>
 </div>

 <!-- Anthropic Cache TTL 1h Injection -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.anthropicCacheTTL1hInjection",
 )
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.anthropicCacheTTL1hInjectionHint",
 )
 }}
 </p>
 </div>
 <Toggle
 v-model="form.enable_anthropic_cache_ttl_1h_injection"
 />
 </div>

 <!-- messages cache_control 改写 -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.rewriteMessageCacheControl",
 )
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.rewriteMessageCacheControlHint",
 )
 }}
 </p>
 </div>
 <Toggle v-model="form.rewrite_message_cache_control" />
 </div>

 <!-- 客户端 dateline 归一化（仅 Anthropic OAuth/SetupToken） -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.clientDatelineNormalization",
 )
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.clientDatelineNormalizationHint",
 )
 }}
 </p>
 </div>
 <Toggle
 v-model="form.enable_client_dateline_normalization"
 />
 </div>

 <!-- Antigravity UA 版本 -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.gatewayForwarding.antigravityUserAgentVersion",
 )
 }}
 </label>
 <input
 v-model="form.antigravity_user_agent_version"
 type="text"
 class="input max-w-xs font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.antigravityUserAgentVersionPlaceholder',
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.gatewayForwarding.antigravityUserAgentVersionHint",
 )
 }}
 </p>
 </div>

 <!-- OpenAI Codex UA -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.gatewayForwarding.openaiCodexUserAgent",
 )
 }}
 </label>
 <input
 v-model="form.openai_codex_user_agent"
 type="text"
 class="input w-full font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.openaiCodexUserAgentPlaceholder',
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.gatewayForwarding.openaiCodexUserAgentHint",
 )
 }}
 </p>
 </div>

 <!-- Codex 客户端版本号 -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.gatewayForwarding.openaiCodexClientVersion",
 )
 }}
 </label>
 <input
 v-model="form.openai_codex_client_version"
 type="text"
 class="input w-full font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.openaiCodexClientVersionPlaceholder',
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.gatewayForwarding.openaiCodexClientVersionHint",
 )
 }}
 </p>
 </div>

 <!-- Codex 版本号自动同步 -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{
 t(
 "admin.settings.gatewayForwarding.openaiCodexVersionAutoSync",
 )
 }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.openaiCodexVersionAutoSyncHint",
 )
 }}
 </p>
 <p
 v-if="codexSyncedVersionLabel"
 class="mt-0.5 text-xs text-muted "
 >
 {{ codexSyncedVersionLabel }}
 </p>
 </div>
 <Toggle v-model="form.openai_codex_version_auto_sync_enabled" />
 </div>

 </div>
 </div>

 <!-- Web Search Emulation -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.webSearchEmulation.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.webSearchEmulation.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Global Toggle -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.webSearchEmulation.enabled") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.webSearchEmulation.enabledHint") }}
 </p>
 </div>
 <Toggle v-model="webSearchConfig.enabled" />
 </div>

 <!-- Providers -->
 <div v-if="webSearchConfig.enabled" class="space-y-4">
 <div class="flex items-center justify-between">
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.webSearchEmulation.providers") }}
 </label>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addWebSearchProvider"
 >
 {{ t("admin.settings.webSearchEmulation.addProvider") }}
 </button>
 </div>

 <div
 v-if="webSearchConfig.providers.length === 0"
 class="rounded-lg border border-dashed border-line p-4 text-center text-sm text-muted "
 >
 {{ t("admin.settings.webSearchEmulation.noProviders") }}
 </div>

 <div
 v-for="(provider, pIdx) in webSearchConfig.providers"
 :key="pIdx"
 class="rounded-lg border border-line "
 >
 <!-- Collapsible header -->
 <div
 class="flex cursor-pointer items-center justify-between px-4 py-3"
 @click="toggleProviderExpand(pIdx)"
 >
 <div class="flex items-center gap-3">
 <svg
 class="h-4 w-4 text-muted transition-transform"
 :class="{ 'rotate-90': expandedProviders[pIdx] }"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M9 5l7 7-7 7"
 />
 </svg>
 <Select
 v-model="provider.type"
 :options="[
 { value: 'brave', label: 'Brave Search' },
 { value: 'tavily', label: 'Tavily' },
 ]"
 class="w-36"
 @click.stop
 />
 <!-- Quota summary (always visible) -->
 <span class="text-xs text-muted">
 {{ provider.quota_used ?? 0 }} /
 {{
 provider.quota_limit != null &&
 provider.quota_limit > 0
 ? provider.quota_limit
 : "∞"
 }}
 </span>
 <span
 v-if="
 !expandedProviders[pIdx] &&
 provider.api_key_configured
 "
 class="text-xs text-green-500"
 >
 {{
 t(
 "admin.settings.webSearchEmulation.apiKeyConfigured",
 )
 }}
 </span>
 </div>
 <button
 type="button"
 class="text-red-500 hover:text-red-700 text-xs"
 @click.stop="removeWebSearchProvider(pIdx)"
 >
 {{
 t("admin.settings.webSearchEmulation.removeProvider")
 }}
 </button>
 </div>

 <!-- Expanded content -->
 <div
 v-if="expandedProviders[pIdx]"
 class="space-y-3 border-t border-line px-4 pb-4 pt-3 "
 >
 <!-- API Key with inline show/copy -->
 <div>
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.apiKey")
 }}</label>
 <div class="relative">
 <input
 v-model="provider.api_key"
 :type="apiKeyVisible[pIdx] ? 'text' : 'password'"
 class="input w-full text-sm"
 :class="
 provider.api_key || provider.api_key_configured
 ? 'pr-16'
 : ''
 "
 :placeholder="
 provider.api_key_configured
 ? '••••••••'
 : t(
 'admin.settings.webSearchEmulation.apiKeyPlaceholder',
 )
 "
 />
 <div
 v-if="provider.api_key || provider.api_key_configured"
 class="absolute inset-y-0 right-0 flex items-center pr-1.5"
 >
 <button
 type="button"
 class="rounded p-1 text-muted hover:text-foreground "
 :title="
 apiKeyVisible[pIdx]
 ? t(
 'admin.settings.webSearchEmulation.hideApiKey',
 )
 : t(
 'admin.settings.webSearchEmulation.showApiKey',
 )
 "
 @click="apiKeyVisible[pIdx] = !apiKeyVisible[pIdx]"
 >
 <svg
 v-if="!apiKeyVisible[pIdx]"
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
 />
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
 />
 </svg>
 <svg
 v-else
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"
 />
 </svg>
 </button>
 <button
 type="button"
 class="rounded p-1 text-muted hover:text-foreground "
 :class="{
 'opacity-30 cursor-not-allowed':
 !provider.api_key,
 }"
 :title="
 t('admin.settings.webSearchEmulation.copyApiKey')
 "
 :disabled="!provider.api_key"
 @click="copyApiKey(pIdx)"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
 />
 </svg>
 </button>
 </div>
 </div>
 </div>

 <!-- Quota + Subscription in compact row -->
 <div class="grid grid-cols-2 gap-3">
 <div>
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.quotaLimit")
 }}</label>
 <input
 v-model="provider.quota_limit"
 type="number"
 min="1"
 class="input text-sm"
 :placeholder="'∞'"
 />
 <p class="mt-0.5 text-xs text-muted">
 {{
 t(
 "admin.settings.webSearchEmulation.quotaLimitHint",
 )
 }}
 </p>
 </div>
 <div>
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.subscribedAt")
 }}</label>
 <input
 :value="formatSubscribedAt(provider.subscribed_at)"
 type="date"
 class="input text-sm"
 @input="
 provider.subscribed_at = parseSubscribedAt(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="mt-0.5 text-xs text-muted">
 {{
 t(
 "admin.settings.webSearchEmulation.subscribedAtHint",
 )
 }}
 </p>
 </div>
 </div>

 <!-- Usage display -->
 <div class="flex items-center gap-2">
 <span class="text-xs text-muted"
 >{{
 t("admin.settings.webSearchEmulation.quotaUsage")
 }}:</span
 >
 <div
 v-if="
 provider.quota_limit != null &&
 provider.quota_limit > 0
 "
 class="flex-1 rounded-full bg-surface-3 "
 style="height: 6px"
 >
 <div
 class="h-full rounded-full transition-all"
 :class="
 quotaPercentage(provider) > 90
 ? 'bg-red-500'
 : quotaPercentage(provider) > 70
 ? 'bg-yellow-500'
 : 'bg-green-500'
 "
 :style="{
 width:
 Math.min(quotaPercentage(provider), 100) + '%',
 }"
 />
 </div>
 <div v-else class="flex-1" />
 <span class="text-xs text-muted"
 >{{ provider.quota_used ?? 0 }} /
 {{
 provider.quota_limit != null &&
 provider.quota_limit > 0
 ? provider.quota_limit
 : "∞"
 }}</span
 >
 <button
 v-if="(provider.quota_used ?? 0) > 0"
 type="button"
 class="text-xs text-accent hover:text-accent"
 @click="resetWebSearchUsage(pIdx)"
 >
 {{ t("admin.settings.webSearchEmulation.resetUsage") }}
 </button>
 </div>

 <!-- Proxy + Test on same row -->
 <div class="flex items-end gap-3">
 <div class="flex-1">
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.proxy")
 }}</label>
 <ProxySelector
 v-model="provider.proxy_id"
 :proxies="webSearchProxies"
 />
 </div>
 <button
 type="button"
 class="btn-glass-secondary whitespace-nowrap"
 @click="openTestDialog()"
 >
 {{ t("admin.settings.webSearchEmulation.test") }}
 </button>
 </div>
 </div>
 </div>
 </div>
 </div>
 </div>

 <!-- Web Search Test Dialog -->
 <div
 v-if="wsTestDialogOpen"
 class="fixed inset-0 z-50 flex items-center justify-center glass-modal-scrim"
 @click.self="wsTestDialogOpen = false"
 >
 <div
 class="mx-4 w-full max-w-lg glass-card-solid rounded-hero p-6"
 >
 <h3
 class="mb-4 text-lg font-semibold text-foreground "
 >
 {{ t("admin.settings.webSearchEmulation.testResultTitle") }}
 </h3>
 <div class="flex items-center gap-2">
 <input
 v-model="wsTestQuery"
 type="text"
 class="input flex-1 text-sm"
 :placeholder="
 t('admin.settings.webSearchEmulation.testDefaultQuery')
 "
 @keyup.enter="testWebSearchProvider()"
 />
 <button
 type="button"
 class="btn-glass-primary"
 :disabled="wsTestLoading"
 @click="testWebSearchProvider()"
 >
 {{
 wsTestLoading
 ? t("admin.settings.webSearchEmulation.testing")
 : t("admin.settings.webSearchEmulation.test")
 }}
 </button>
 </div>
 <!-- Test results -->
 <div
 v-if="wsTestResult"
 class="mt-4 max-h-80 overflow-y-auto rounded-lg bg-surface-2 p-4 "
 >
 <p
 class="mb-2 text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.webSearchEmulation.testResultProvider")
 }}: {{ wsTestResult.provider }}
 </p>
 <div
 v-if="wsTestResult.results.length === 0"
 class="text-sm text-muted"
 >
 {{ t("admin.settings.webSearchEmulation.testNoResults") }}
 </div>
 <div
 v-for="(r, rIdx) in wsTestResult.results"
 :key="rIdx"
 class="mt-2 border-t border-line pt-2 first:mt-0 first:border-0 first:pt-0 "
 >
 <a
 :href="r.url"
 target="_blank"
 class="text-sm font-medium text-blue-600 hover:underline "
 >{{ r.title }}</a
 >
 <p class="mt-0.5 text-xs text-muted ">
 {{ r.snippet }}
 </p>
 </div>
 </div>
 <div class="mt-4 flex justify-end">
 <button
 type="button"
 class="btn-glass-secondary"
 @click="wsTestDialogOpen = false"
 >
 {{ t("common.close") }}
 </button>
 </div>
 </div>
 </div>

 <!-- Usage Records Settings -->
 <div class="glass-card settings-card">
 <div class="settings-card-head">
 <h2 class="settings-card-title">
 {{ t('admin.settings.usageRecords.title') }}
 </h2>
 <p class="settings-card-desc">
 {{ t('admin.settings.usageRecords.description') }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- User error requests visibility -->
 <div class="flex items-center justify-between">
 <div>
 <label class="text-sm font-medium text-foreground ">
 {{ t('admin.settings.user_error_view.label') }}
 </label>
 <p class="text-xs text-muted ">
 {{ t('admin.settings.user_error_view.description') }}
 </p>
 </div>
 <label class="toggle">
 <input v-model="form.allow_user_view_error_requests" type="checkbox" />
 <span class="toggle-slider"></span>
 </label>
 </div>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import type { ClaudeOAuthSystemPromptPreset, ClaudeOAuthSystemPromptBlock } from "../useSettingsForm";
import { ref, reactive, computed } from "vue";
import { adminAPI } from "@/api";
import { SCHEDULING_THRESHOLD_PLATFORMS, KIRO_CACHE_MIN_BLOCK_TOKENS_MAX } from "@/api/admin/settings";
import type { OpenAIFastPolicyRule, WebSearchProviderConfig, WebSearchTestResult } from "@/api/admin/settings";
import Icon from "@/components/icons/Icon.vue";
import Select from "@/components/common/Select.vue";
import Toggle from "@/components/common/Toggle.vue";
import ProxySelector from "@/components/common/ProxySelector.vue";
import OpenAIFastPolicyUserSelector from "@/views/admin/settings/OpenAIFastPolicyUserSelector.vue";
import { extractApiErrorMessage } from "@/utils/apiError";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  appStore,
  activeTab,
  upstreamBillingProbeLoading,
  upstreamBillingProbeForm,
  ollamaCloudUsageLoading,
  ollamaCloudUsageForm,
  overloadCooldownLoading,
  overloadCooldownForm,
  rateLimit429CooldownLoading,
  rateLimit429CooldownForm,
  streamTimeoutLoading,
  streamTimeoutForm,
  rectifierLoading,
  rectifierForm,
  betaPolicyLoading,
  betaPolicyForm,
  openaiFastPolicyForm,
  defaultClaudeCodeSystemPrompt,
  defaultClaudeCodeExpansionPrompt,
  detectClaudeOAuthSystemPromptPreset,
  createClaudeOAuthSystemPromptBlock,
  createDefaultClaudeOAuthSystemPromptBlocks,
  claudeOAuthSystemPromptBlocks,
  syncClaudeOAuthSystemPromptBlocksFormField,
  form,
  webSearchProxies,
  webSearchConfig,
  codexBlacklistRows,
  codexWhitelistRows,
  codexFingerprintRows,
} = settingsForm;

type OpenAIAdvancedSchedulerOverrideKey =
  | "openai_advanced_scheduler_lb_top_k"
  | "openai_advanced_scheduler_weight_priority"
  | "openai_advanced_scheduler_weight_load"
  | "openai_advanced_scheduler_weight_queue"
  | "openai_advanced_scheduler_weight_error_rate"
  | "openai_advanced_scheduler_weight_ttft"
  | "openai_advanced_scheduler_weight_reset"
  | "openai_advanced_scheduler_weight_quota_headroom"
  | "openai_advanced_scheduler_weight_upstream_cost"
  | "openai_advanced_scheduler_weight_previous_response"
  | "openai_advanced_scheduler_weight_session_sticky";

type OpenAIAdvancedSchedulerEffectiveKey =
  | "openai_advanced_scheduler_effective_lb_top_k"
  | "openai_advanced_scheduler_effective_weight_priority"
  | "openai_advanced_scheduler_effective_weight_load"
  | "openai_advanced_scheduler_effective_weight_queue"
  | "openai_advanced_scheduler_effective_weight_error_rate"
  | "openai_advanced_scheduler_effective_weight_ttft"
  | "openai_advanced_scheduler_effective_weight_reset"
  | "openai_advanced_scheduler_effective_weight_quota_headroom"
  | "openai_advanced_scheduler_effective_weight_upstream_cost"
  | "openai_advanced_scheduler_effective_weight_previous_response"
  | "openai_advanced_scheduler_effective_weight_session_sticky";

function parseOptionalIntegerInput(value: string): number | null {
  if (!value.trim()) return null;

  const normalized = Math.floor(Number(value));
  return Number.isFinite(normalized) ? normalized : null;
}

const upstreamBillingProbeSaving = ref(false);

const ollamaCloudUsageSaving = ref(false);

const overloadCooldownSaving = ref(false);

const rateLimit429CooldownSaving = ref(false);

const streamTimeoutSaving = ref(false);

const rectifierSaving = ref(false);

const betaPolicySaving = ref(false);

const claudeOAuthSystemPromptPresetOptions = computed(() => [
  {
    value: "billing",
    label: t("admin.settings.gatewayForwarding.systemBlockPresetBilling"),
  },
  {
    value: "system",
    label: t("admin.settings.gatewayForwarding.systemBlockPresetIdentity"),
  },
  {
    value: "expansion",
    label: t("admin.settings.gatewayForwarding.systemBlockPresetExpansion"),
  },
  {
    value: "custom",
    label: t("admin.settings.gatewayForwarding.systemBlockPresetCustom"),
  },
]);

const claudeOAuthSystemPromptBlockTypeOptions = computed(() => [
  {
    value: "text",
    label: t("admin.settings.gatewayForwarding.systemBlockTypeText"),
  },
]);

const claudeOAuthSystemPromptCacheTTLOptions = computed(() => [
  { value: "5m", label: t("admin.settings.gatewayForwarding.cacheTTL5m") },
  { value: "1h", label: t("admin.settings.gatewayForwarding.cacheTTL1h") },
]);

function getClaudeOAuthPresetLabel(
  preset: ClaudeOAuthSystemPromptPreset,
): string {
  return (
    claudeOAuthSystemPromptPresetOptions.value.find(
      (option) => option.value === preset,
    )?.label || t("admin.settings.gatewayForwarding.systemBlockPresetCustom")
  );
}

function addClaudeOAuthSystemPromptBlock(): void {
  claudeOAuthSystemPromptBlocks.value.push(
    createClaudeOAuthSystemPromptBlock({
      expanded: true,
      preset: "custom",
      text: "",
    }),
  );
  syncClaudeOAuthSystemPromptBlocksFormField();
}

function toggleClaudeOAuthSystemPromptBlock(index: number): void {
  const block = claudeOAuthSystemPromptBlocks.value[index];
  if (!block) {
    return;
  }
  block.expanded = !block.expanded;
}

function removeClaudeOAuthSystemPromptBlock(index: number): void {
  claudeOAuthSystemPromptBlocks.value.splice(index, 1);
  syncClaudeOAuthSystemPromptBlocksFormField();
}

function moveClaudeOAuthSystemPromptBlock(
  index: number,
  direction: -1 | 1,
): void {
  const targetIndex = index + direction;
  if (
    targetIndex < 0 ||
    targetIndex >= claudeOAuthSystemPromptBlocks.value.length
  ) {
    return;
  }
  const blocks = claudeOAuthSystemPromptBlocks.value;
  const current = blocks[index];
  blocks[index] = blocks[targetIndex];
  blocks[targetIndex] = current;
  syncClaudeOAuthSystemPromptBlocksFormField();
}

function applyClaudeOAuthSystemPromptPreset(
  index: number,
  value: string | number | boolean | null,
): void {
  const block = claudeOAuthSystemPromptBlocks.value[index];
  if (!block) {
    return;
  }
  const preset = String(value || "custom") as ClaudeOAuthSystemPromptPreset;
  block.preset = preset;
  block.type = "text";
  if (preset === "billing") {
    block.text = "{billing_header}";
    block.cacheControlEnabled = false;
    block.cacheControlTTL = "5m";
  } else if (preset === "system") {
    block.text = defaultClaudeCodeSystemPrompt;
    block.cacheControlEnabled = false;
    block.cacheControlTTL = "5m";
  } else if (preset === "expansion") {
    block.text =
      form.claude_oauth_system_prompt.trim() ||
      defaultClaudeCodeExpansionPrompt;
    block.cacheControlEnabled = true;
    block.cacheControlTTL = "5m";
  }
  syncClaudeOAuthSystemPromptBlocksFormField();
}

function markClaudeOAuthSystemPromptBlockCustom(
  block: ClaudeOAuthSystemPromptBlock,
): void {
  block.preset = detectClaudeOAuthSystemPromptPreset(block.text);
  syncClaudeOAuthSystemPromptBlocksFormField();
}

function resetClaudeOAuthSystemPromptBlocks(): void {
  claudeOAuthSystemPromptBlocks.value = createDefaultClaudeOAuthSystemPromptBlocks(
    form.claude_oauth_system_prompt,
  );
  syncClaudeOAuthSystemPromptBlocksFormField();
}

const schedulingThresholdPlatforms = SCHEDULING_THRESHOLD_PLATFORMS;

const openAIAdvancedSchedulerWeightFields = computed<
  Array<{
    key: OpenAIAdvancedSchedulerOverrideKey;
    label: string;
    placeholder: string;
  }>
>(() => {
  const placeholder = (
    effectiveKey: OpenAIAdvancedSchedulerEffectiveKey,
    fallbackValue: string,
  ) => {
    const effectiveValue = String(
      (form as Record<string, unknown>)[effectiveKey] ?? "",
    ).trim();
    return t("admin.settings.openaiExperimentalScheduler.defaultPlaceholder", {
      value: effectiveValue || fallbackValue,
    });
  };

  return [
    {
      key: "openai_advanced_scheduler_lb_top_k",
      label: t("admin.settings.openaiExperimentalScheduler.topKLabel"),
      placeholder: placeholder("openai_advanced_scheduler_effective_lb_top_k", "7"),
    },
    {
      key: "openai_advanced_scheduler_weight_priority",
      label: t("admin.settings.openaiExperimentalScheduler.priorityWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_priority", "1"),
    },
    {
      key: "openai_advanced_scheduler_weight_load",
      label: t("admin.settings.openaiExperimentalScheduler.loadWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_load", "1"),
    },
    {
      key: "openai_advanced_scheduler_weight_queue",
      label: t("admin.settings.openaiExperimentalScheduler.queueWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_queue", "0.7"),
    },
    {
      key: "openai_advanced_scheduler_weight_error_rate",
      label: t("admin.settings.openaiExperimentalScheduler.errorRateWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_error_rate", "0.8"),
    },
    {
      key: "openai_advanced_scheduler_weight_ttft",
      label: t("admin.settings.openaiExperimentalScheduler.ttftWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_ttft", "0.5"),
    },
    {
      key: "openai_advanced_scheduler_weight_reset",
      label: t("admin.settings.openaiExperimentalScheduler.resetWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_reset", "0"),
    },
    {
      key: "openai_advanced_scheduler_weight_quota_headroom",
      label: t("admin.settings.openaiExperimentalScheduler.quotaHeadroomWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_quota_headroom", "0"),
    },
    {
      key: "openai_advanced_scheduler_weight_upstream_cost",
      label: t("admin.settings.openaiExperimentalScheduler.upstreamCostWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_upstream_cost", "0"),
    },
    {
      key: "openai_advanced_scheduler_weight_previous_response",
      label: t("admin.settings.openaiExperimentalScheduler.previousResponseWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_previous_response", "5"),
    },
    {
      key: "openai_advanced_scheduler_weight_session_sticky",
      label: t("admin.settings.openaiExperimentalScheduler.sessionStickyWeight"),
      placeholder: placeholder("openai_advanced_scheduler_effective_weight_session_sticky", "3"),
    },
  ];
});

const DEFAULT_WEB_SEARCH_QUOTA_LIMIT = 1000;

const expandedProviders = reactive<Record<number, boolean>>({});

const apiKeyVisible = reactive<Record<number, boolean>>({});

const wsTestQuery = ref("");

const wsTestLoading = ref(false);

const wsTestResult = ref<WebSearchTestResult | null>(null);

const wsTestDialogOpen = ref(false);

function openTestDialog() {
  wsTestResult.value = null;
  wsTestDialogOpen.value = true;
}

function toggleProviderExpand(idx: number) {
  expandedProviders[idx] = !expandedProviders[idx];
}

function removeWebSearchProvider(idx: number) {
  webSearchConfig.providers.splice(idx, 1);
  // Re-index expandedProviders and apiKeyVisible after removal
  const newExpanded: Record<number, boolean> = {};
  const newVisible: Record<number, boolean> = {};
  for (let i = 0; i < webSearchConfig.providers.length; i++) {
    const oldIdx = i >= idx ? i + 1 : i;
    newExpanded[i] = expandedProviders[oldIdx] ?? false;
    newVisible[i] = apiKeyVisible[oldIdx] ?? false;
  }
  Object.keys(expandedProviders).forEach(
    (k) => delete expandedProviders[Number(k)],
  );
  Object.keys(apiKeyVisible).forEach((k) => delete apiKeyVisible[Number(k)]);
  Object.assign(expandedProviders, newExpanded);
  Object.assign(apiKeyVisible, newVisible);
}

function addWebSearchProvider() {
  const idx = webSearchConfig.providers.length;
  webSearchConfig.providers.push({
    type: "brave",
    api_key: "",
    api_key_configured: false,
    quota_limit: DEFAULT_WEB_SEARCH_QUOTA_LIMIT,
    subscribed_at: null,
    proxy_id: null,
    expires_at: null,
  } as WebSearchProviderConfig);
  expandedProviders[idx] = true;
}

function formatSubscribedAt(ts: number | null): string {
  if (!ts) return "";
  // Use UTC to avoid timezone drift on repeated edits
  const d = new Date(ts * 1000);
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function parseSubscribedAt(dateStr: string): number | null {
  if (!dateStr) return null;
  // Parse as UTC to match formatSubscribedAt
  return Math.floor(new Date(dateStr + "T00:00:00Z").getTime() / 1000);
}

function quotaPercentage(provider: WebSearchProviderConfig): number {
  if (!provider.quota_limit || provider.quota_limit <= 0) return 0;
  return ((provider.quota_used ?? 0) / provider.quota_limit) * 100;
}

async function resetWebSearchUsage(idx: number) {
  const provider = webSearchConfig.providers[idx];
  if (!provider) return;
  if (!confirm(t("admin.settings.webSearchEmulation.resetUsageConfirm")))
    return;
  try {
    await adminAPI.settings.resetWebSearchUsage({
      provider_type: provider.type,
    });
    provider.quota_used = 0;
    appStore.showSuccess(
      t("admin.settings.webSearchEmulation.resetUsageSuccess"),
    );
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  }
}

async function copyApiKey(idx: number) {
  const key = webSearchConfig.providers[idx]?.api_key;
  if (!key) {
    appStore.showError(
      t("admin.settings.webSearchEmulation.apiKeyPlaceholder"),
    );
    return;
  }
  try {
    await navigator.clipboard.writeText(key);
    appStore.showSuccess(t("admin.settings.webSearchEmulation.copied"));
  } catch {
    appStore.showError(t("common.error"));
  }
}

async function testWebSearchProvider() {
  wsTestLoading.value = true;
  wsTestResult.value = null;
  try {
    const query =
      wsTestQuery.value.trim() ||
      t("admin.settings.webSearchEmulation.testDefaultQuery");
    wsTestResult.value = await adminAPI.settings.testWebSearchEmulation(query);
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    wsTestLoading.value = false;
  }
}

const codexFingerprintNoRequired = computed(
  () => !codexFingerprintRows.value.some((r) => r.required),
);

function addCodexFingerprintRow(): void {
  codexFingerprintRows.value.push({ type: "header_exact", match: "", required: false });
}

function removeCodexFingerprintRow(i: number): void {
  codexFingerprintRows.value.splice(i, 1);
}

function addCodexBlacklistRow(): void {
  codexBlacklistRows.value.push({ originator: "", uaContains: "" });
}

function removeCodexBlacklistRow(i: number): void {
  codexBlacklistRows.value.splice(i, 1);
}

function addCodexWhitelistRow(): void {
  codexWhitelistRows.value.push({
    originator: "",
    uaContains: "",
    skipEngineFingerprint: false,
  });
}

function removeCodexWhitelistRow(i: number): void {
  codexWhitelistRows.value.splice(i, 1);
}

const codexSyncedVersionLabel = computed(() => {
  const synced = form.openai_codex_client_version_synced?.trim();
  if (!synced) return "";
  return t("admin.settings.gatewayForwarding.openaiCodexVersionSyncedValue", {
    version: synced,
  });
});

async function saveUpstreamBillingProbeSettings() {
  upstreamBillingProbeSaving.value = true;
  try {
    const updated = await adminAPI.accounts.updateUpstreamBillingProbeSettings({
      ...upstreamBillingProbeForm,
    });
    Object.assign(upstreamBillingProbeForm, updated);
    appStore.showSuccess(t("admin.settings.upstreamBillingProbe.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.upstreamBillingProbe.saveFailed"),
      ),
    );
  } finally {
    upstreamBillingProbeSaving.value = false;
  }
}

async function saveOllamaCloudUsageSettings() {
  ollamaCloudUsageSaving.value = true;
  try {
    const updated = await adminAPI.accounts.updateOllamaCloudUsageSettings({
      ...ollamaCloudUsageForm,
    });
    Object.assign(ollamaCloudUsageForm, updated);
    appStore.showSuccess(t("admin.settings.ollamaCloudUsage.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.ollamaCloudUsage.saveFailed")),
    );
  } finally {
    ollamaCloudUsageSaving.value = false;
  }
}

async function saveOverloadCooldownSettings() {
  overloadCooldownSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateOverloadCooldownSettings({
      enabled: overloadCooldownForm.enabled,
      cooldown_minutes: overloadCooldownForm.cooldown_minutes,
    });
    Object.assign(overloadCooldownForm, updated);
    appStore.showSuccess(t("admin.settings.overloadCooldown.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.overloadCooldown.saveFailed"),
      ),
    );
  } finally {
    overloadCooldownSaving.value = false;
  }
}

async function saveRateLimit429CooldownSettings() {
  rateLimit429CooldownSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateRateLimit429CooldownSettings({
      enabled: rateLimit429CooldownForm.enabled,
      cooldown_seconds: rateLimit429CooldownForm.cooldown_seconds,
      strategy: rateLimit429CooldownForm.enabled ? "cooldown" : "same_account_retry",
      retry_interval_ms: rateLimit429CooldownForm.retry_interval_ms,
      retry_max_duration_seconds: rateLimit429CooldownForm.retry_max_duration_seconds,
      max_account_switches: rateLimit429CooldownForm.max_account_switches,
    });
    Object.assign(rateLimit429CooldownForm, updated);
    appStore.showSuccess(t("admin.settings.rateLimit429Cooldown.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.rateLimit429Cooldown.saveFailed"),
      ),
    );
  } finally {
    rateLimit429CooldownSaving.value = false;
  }
}

async function saveStreamTimeoutSettings() {
  streamTimeoutSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateStreamTimeoutSettings({
      enabled: streamTimeoutForm.enabled,
      action: streamTimeoutForm.action,
      temp_unsched_minutes: streamTimeoutForm.temp_unsched_minutes,
      threshold_count: streamTimeoutForm.threshold_count,
      threshold_window_minutes: streamTimeoutForm.threshold_window_minutes,
    });
    Object.assign(streamTimeoutForm, updated);
    appStore.showSuccess(t("admin.settings.streamTimeout.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.streamTimeout.saveFailed"),
      ),
    );
  } finally {
    streamTimeoutSaving.value = false;
  }
}

async function saveRectifierSettings() {
  rectifierSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateRectifierSettings({
      enabled: rectifierForm.enabled,
      thinking_signature_enabled: rectifierForm.thinking_signature_enabled,
      thinking_budget_enabled: rectifierForm.thinking_budget_enabled,
      apikey_signature_enabled: rectifierForm.apikey_signature_enabled,
      apikey_signature_patterns: rectifierForm.apikey_signature_patterns.filter(
        (p) => p.trim() !== "",
      ),
    });
    Object.assign(rectifierForm, updated);
    if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
      rectifierForm.apikey_signature_patterns = [];
    }
    appStore.showSuccess(t("admin.settings.rectifier.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.rectifier.saveFailed")),
    );
  } finally {
    rectifierSaving.value = false;
  }
}

const betaPolicyActionOptions = computed(() => [
  { value: "pass", label: t("admin.settings.betaPolicy.actionPass") },
  { value: "filter", label: t("admin.settings.betaPolicy.actionFilter") },
  { value: "block", label: t("admin.settings.betaPolicy.actionBlock") },
]);

const betaPolicyScopeOptions = computed(() => [
  { value: "all", label: t("admin.settings.betaPolicy.scopeAll") },
  { value: "oauth", label: t("admin.settings.betaPolicy.scopeOAuth") },
  { value: "apikey", label: t("admin.settings.betaPolicy.scopeAPIKey") },
  { value: "bedrock", label: t("admin.settings.betaPolicy.scopeBedrock") },
]);

const betaDisplayNames: Record<string, string> = {
  "fast-mode-2026-02-01": "Fast Mode",
  "context-1m-2025-08-07": "Context 1M",
};

const betaPresets: Record<
  string,
  Array<{
    label: string;
    description: string;
    action: "pass" | "filter" | "block";
    model_whitelist: string[];
    fallback_action: "pass" | "filter" | "block";
  }>
> = {
  "context-1m-2025-08-07": [
    {
      label: t("admin.settings.betaPolicy.presetOpusOnly"),
      description: t("admin.settings.betaPolicy.presetOpusOnlyDesc"),
      action: "pass",
      model_whitelist: ["claude-opus-4-6"],
      fallback_action: "filter",
    },
  ],
};

const commonModelPatterns = [
  "claude-opus-4-6",
  "claude-sonnet-4-6",
  "claude-opus-*",
  "claude-sonnet-*",
];

function getBetaDisplayName(token: string): string {
  return betaDisplayNames[token] || token;
}

function applyBetaPreset(
  rule: (typeof betaPolicyForm.rules)[number],
  preset: {
    action: "pass" | "filter" | "block";
    model_whitelist: string[];
    fallback_action: "pass" | "filter" | "block";
  },
) {
  rule.action = preset.action;
  rule.model_whitelist = [...preset.model_whitelist];
  rule.fallback_action = preset.fallback_action;
}

function addQuickPattern(
  rule: (typeof betaPolicyForm.rules)[number],
  pattern: string,
) {
  if (!rule.model_whitelist) rule.model_whitelist = [];
  if (!rule.model_whitelist.includes(pattern)) {
    rule.model_whitelist.push(pattern);
  }
}

const openaiFastPolicyTierOptions = computed(() => [
  { value: "all", label: t("admin.settings.openaiFastPolicy.tierAll") },
  {
    value: "priority",
    label: t("admin.settings.openaiFastPolicy.tierPriority"),
  },
  { value: "flex", label: t("admin.settings.openaiFastPolicy.tierFlex") },
]);

const openaiFastPolicyActionOptions = computed(() => [
  { value: "pass", label: t("admin.settings.openaiFastPolicy.actionPass") },
  { value: "filter", label: t("admin.settings.openaiFastPolicy.actionFilter") },
  {
    value: "force_priority",
    label: t("admin.settings.openaiFastPolicy.actionForcePriority"),
  },
  { value: "block", label: t("admin.settings.openaiFastPolicy.actionBlock") },
]);

function openaiFastPolicyActionSummary(
  action: OpenAIFastPolicyRule["action"],
) {
  return t(`admin.settings.openaiFastPolicy.summaryAction.${action}`);
}

function hasOpenAIFastPolicyTargetModels(rule: OpenAIFastPolicyRule) {
  return Boolean(rule.model_whitelist?.some((pattern) => pattern.trim() !== ""));
}

const openaiFastPolicyScopeOptions = computed(() => [
  { value: "all", label: t("admin.settings.openaiFastPolicy.scopeAll") },
  { value: "oauth", label: t("admin.settings.openaiFastPolicy.scopeOAuth") },
  { value: "apikey", label: t("admin.settings.openaiFastPolicy.scopeAPIKey") },
  {
    value: "bedrock",
    label: t("admin.settings.openaiFastPolicy.scopeBedrock"),
  },
]);

function addOpenAIFastPolicyRule() {
  openaiFastPolicyForm.rules.push({
    service_tier: "priority",
    action: "filter",
    scope: "all",
    user_ids: [],
    error_message: "",
    model_whitelist: [],
    fallback_action: "pass",
    fallback_error_message: "",
  });
}

function removeOpenAIFastPolicyRule(index: number) {
  openaiFastPolicyForm.rules.splice(index, 1);
}

function addOpenAIFastPolicyModelPattern(rule: OpenAIFastPolicyRule) {
  if (!rule.model_whitelist) rule.model_whitelist = [];
  rule.model_whitelist.push("");
}

function removeOpenAIFastPolicyModelPattern(
  rule: OpenAIFastPolicyRule,
  idx: number,
) {
  rule.model_whitelist?.splice(idx, 1);
}

async function saveBetaPolicySettings() {
  betaPolicySaving.value = true;
  try {
    // Clean up empty patterns before saving
    const cleanedRules = betaPolicyForm.rules.map((rule) => {
      const whitelist = rule.model_whitelist?.filter((p) => p.trim() !== "");
      const hasWhitelist = whitelist && whitelist.length > 0;
      return {
        beta_token: rule.beta_token,
        action: rule.action,
        scope: rule.scope,
        error_message: rule.error_message,
        model_whitelist: hasWhitelist ? whitelist : undefined,
        fallback_action: hasWhitelist
          ? rule.fallback_action || "pass"
          : undefined,
        fallback_error_message:
          hasWhitelist && rule.fallback_action === "block"
            ? rule.fallback_error_message
            : undefined,
      };
    });
    const updated = await adminAPI.settings.updateBetaPolicySettings({
      rules: cleanedRules,
    });
    betaPolicyForm.rules = updated.rules;
    appStore.showSuccess(t("admin.settings.betaPolicy.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.betaPolicy.saveFailed")),
    );
  } finally {
    betaPolicySaving.value = false;
  }
}
</script>
