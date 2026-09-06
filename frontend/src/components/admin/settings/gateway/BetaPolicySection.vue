<template>
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
 class="settings-flex-row flex items-center gap-2 text-muted"
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
 <div class="settings-flex-row mb-3 flex items-center gap-2">
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
 <div class="settings-flex-row flex flex-wrap gap-2">
 <button
 v-for="preset in betaPresets[rule.beta_token]"
 :key="preset.label"
 type="button"
 class="inline-flex items-center gap-1 rounded-md border border-accent bg-accent/10 px-2.5 py-1 text-xs font-medium text-accent transition-colors hover:bg-accent/20 "
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
 class="settings-flex-row mb-1.5 flex items-center gap-2"
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
 class="shrink-0 rounded p-1 text-danger-400 transition-colors hover:bg-danger-50 hover:text-danger-600 "
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
 <div class="settings-flex-row flex flex-wrap items-center gap-1.5">
 <span class="text-xs text-muted "
 >{{
 t("admin.settings.betaPolicy.commonPatterns")
 }}:</span
 >
 <button
 v-for="pattern in commonModelPatterns"
 :key="pattern"
 type="button"
 class="rounded border border-line px-2 py-0.5 text-xs text-muted transition-colors hover:border-accent hover:bg-accent/10 hover:text-accent "
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
 class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 "
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
</template>

<script setup lang="ts">
import { inject, ref, computed } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { adminAPI } from "@/api";
import { extractApiErrorMessage } from "@/utils/apiError";
import Select from "@/components/common/Select.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, appStore, betaPolicyLoading, betaPolicyForm } = settingsForm;

const betaPolicySaving = ref(false);

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
