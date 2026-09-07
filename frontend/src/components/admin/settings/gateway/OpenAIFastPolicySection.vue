<template>
  <!-- OpenAI Fast/Flex Policy Settings -->
  <SettingsSection
    :title="t('admin.settings.openaiFastPolicy.title')"
    :description="t('admin.settings.openaiFastPolicy.description')"
  >
    <div class="settings-rows">
      <!-- Empty state -->
      <div
        v-if="openaiFastPolicyForm.rules.length === 0"
        class="rounded-lg border border-dashed border-line p-6 text-center text-sm text-muted settings-block"
      >
        {{ t("admin.settings.openaiFastPolicy.empty") }}
      </div>
      <!-- Rule Cards -->
      <div
        v-for="(rule, ruleIndex) in openaiFastPolicyForm.rules"
        :key="ruleIndex"
        class="rounded-lg border border-line p-4 settings-block"
      >
        <div
          class="settings-flex-row settings-control-row mb-3 flex items-center justify-between"
        >
          <span class="text-sm font-medium text-foreground">
            {{
              t("admin.settings.openaiFastPolicy.ruleHeader", {
                index: ruleIndex + 1,
              })
            }}
          </span>
          <button
            type="button"
            @click="removeOpenAIFastPolicyRule(ruleIndex)"
            class="rounded p-1 text-danger-400 transition-colors hover:bg-danger-50 hover:text-danger-600"
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
          class="settings-flex-row mb-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted"
          :data-testid="`openai-fast-policy-summary-${ruleIndex}`"
        >
          <span class="font-medium text-foreground">
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
            class="inline-flex items-center rounded bg-accent/10 px-2 py-0.5 font-medium text-accent"
          >
            {{ openaiFastPolicyActionSummary(rule.action) }}
          </span>
          <template v-if="hasOpenAIFastPolicyTargetModels(rule)">
            <span aria-hidden="true">·</span>
            <span class="font-medium text-foreground">
              {{ t("admin.settings.openaiFastPolicy.summaryOtherModels") }}
            </span>
            <span aria-hidden="true">→</span>
            <span
              class="inline-flex items-center rounded bg-surface-2 px-2 py-0.5 font-medium text-foreground"
            >
              {{
                openaiFastPolicyActionSummary(rule.fallback_action || "pass")
              }}
            </span>
          </template>
        </div>
        <div class="settings-rows">
          <!-- Service Tier -->
          <SettingRow :label="t('admin.settings.openaiFastPolicy.serviceTier')">
            <Select
              :modelValue="rule.service_tier"
              @update:modelValue="
                rule.service_tier = $event as 'all' | 'priority' | 'flex'
              "
              :options="openaiFastPolicyTierOptions"
            />
          </SettingRow>
          <!-- Action -->
          <SettingRow :label="t('admin.settings.openaiFastPolicy.action')">
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
          </SettingRow>
          <!-- Scope -->
          <SettingRow :label="t('admin.settings.openaiFastPolicy.scope')">
            <Select
              :modelValue="rule.scope"
              @update:modelValue="
                rule.scope = $event as 'all' | 'oauth' | 'apikey' | 'bedrock'
              "
              :options="openaiFastPolicyScopeOptions"
            />
          </SettingRow>
        </div>
        <!-- User Scope -->
        <div class="mt-3">
          <label class="settings-sub-label">
            {{ t("admin.settings.openaiFastPolicy.userIds") }}
          </label>
          <p class="mb-2 text-xs text-muted">
            {{ t("admin.settings.openaiFastPolicy.userIdsHint") }}
          </p>
          <OpenAIFastPolicyUserSelector
            :model-value="rule.user_ids || []"
            @update:model-value="rule.user_ids = $event"
          />
        </div>
        <!-- Error Message (only when action=block) -->
        <div v-if="rule.action === 'block'" class="mt-3">
          <label class="settings-sub-label">
            {{ t("admin.settings.openaiFastPolicy.errorMessage") }}
          </label>
          <input
            v-model="rule.error_message"
            type="text"
            class="input"
            :placeholder="
              t('admin.settings.openaiFastPolicy.errorMessagePlaceholder')
            "
          />
          <p class="mt-1 text-xs text-muted">
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
            class="mb-2 text-xs text-muted"
          >
            {{ t("admin.settings.openaiFastPolicy.modelWhitelistHint") }}
          </p>
          <div
            v-for="(_, patternIdx) in rule.model_whitelist || []"
            :key="patternIdx"
            class="settings-flex-row mb-1.5 flex items-center gap-2"
          >
            <input
              v-model="rule.model_whitelist![patternIdx]"
              type="text"
              class="input input-sm flex-1"
              :placeholder="
                t('admin.settings.openaiFastPolicy.modelPatternPlaceholder')
              "
            />
            <button
              type="button"
              @click="removeOpenAIFastPolicyModelPattern(rule, patternIdx)"
              class="shrink-0 rounded p-1 text-danger-400 transition-colors hover:bg-danger-50 hover:text-danger-600"
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
            class="mb-2 inline-flex items-center gap-1 text-xs text-accent transition-colors hover:text-accent"
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
        <div v-if="hasOpenAIFastPolicyTargetModels(rule)" class="mt-3">
          <label class="settings-sub-label">
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
          <p class="mt-1 text-xs text-muted">
            {{ t("admin.settings.openaiFastPolicy.fallbackActionHint") }}
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
      <div class="settings-block">
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
        <p class="mt-2 text-xs text-muted">
          {{ t("admin.settings.openaiFastPolicy.saveHint") }}
        </p>
      </div>
    </div>
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import { inject, computed } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import type { OpenAIFastPolicyRule } from "@/api/admin/settings";
import Select from "@/components/common/Select.vue";
import OpenAIFastPolicyUserSelector from "@/views/admin/settings/OpenAIFastPolicyUserSelector.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, openaiFastPolicyForm } = settingsForm;

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

</script>
