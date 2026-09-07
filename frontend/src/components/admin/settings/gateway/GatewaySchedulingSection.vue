<template>
  <!-- Gateway Scheduling Settings -->
  <SettingsSection
    :title="t('admin.settings.scheduling.title')"
    :description="t('admin.settings.scheduling.description')"
  >
    <div class="settings-rows">
      <SettingRow
        :label="t('admin.settings.scheduling.allowUngroupedKey')"
        :description="t('admin.settings.scheduling.allowUngroupedKeyHint')"
      >
        <Toggle v-model="form.allow_ungrouped_key_scheduling" />
      </SettingRow>
      <div class="border-t border-line pt-4 settings-block">
        <div class="mb-3">
          <label class="font-medium text-foreground">
            {{
              t("admin.settings.scheduling.accountSchedulingThresholdsTitle")
            }}
          </label>
          <p class="settings-card-desc">
            {{
              t(
                "admin.settings.scheduling.accountSchedulingThresholdsDescription",
              )
            }}
          </p>
          <p class="mt-0.5 text-xs text-muted">
            {{
              t(
                "admin.settings.scheduling.accountSchedulingThresholdsGlobalHint",
              )
            }}
          </p>
          <p class="mt-0.5 text-xs text-warning-600">
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
            class="rounded-lg border border-line p-4"
          >
            <SettingRow
              :label="platform"
              :description="
                t(
                  'admin.settings.scheduling.accountSchedulingThresholdsRangeHint',
                )
              "
            >
              <span
                class="rounded bg-surface-2 px-2 py-0.5 text-[11px] font-medium text-muted"
              >
                %
              </span>
            </SettingRow>
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
      <SettingRow
        v-if="!form.openai_advanced_scheduler_enabled"
        :label="
          t('admin.settings.openaiExperimentalScheduler.lowRatePriorityTitle')
        "
        :description="
          t(
            'admin.settings.openaiExperimentalScheduler.lowRatePriorityDescription',
          )
        "
      >
        <Toggle
          v-model="form.openai_low_upstream_rate_priority_enabled"
          data-testid="openai-low-rate-priority-toggle"
        />
      </SettingRow>
      <div
        v-if="
          !form.openai_advanced_scheduler_enabled &&
          form.openai_low_upstream_rate_priority_enabled
        "
        class="settings-flex-row settings-divider-row flex flex-col items-stretch gap-3 border-t border-line pt-5 sm:flex-row sm:items-start sm:justify-between sm:gap-6 settings-block"
      >
        <div class="min-w-0">
          <label
            class="text-sm font-medium text-foreground"
            for="openai-oauth-scheduling-rate-multiplier"
          >
            {{ t("admin.settings.openaiExperimentalScheduler.oauthRateTitle") }}
          </label>
          <p class="mt-0.5 text-xs text-muted">
            {{
              t(
                "admin.settings.openaiExperimentalScheduler.oauthRatePriorityDescription",
              )
            }}
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
            >x</span
          >
        </div>
      </div>
      <SettingRow
        :label="t('admin.settings.openaiExperimentalScheduler.title')"
        :description="
          t('admin.settings.openaiExperimentalScheduler.description')
        "
      >
        <Toggle
          v-model="form.openai_advanced_scheduler_enabled"
          data-testid="openai-advanced-scheduler-toggle"
        /> </SettingRow
      ><SettingRow
        v-if="form.openai_advanced_scheduler_enabled"
        :label="
          t('admin.settings.openaiExperimentalScheduler.stickyWeightedTitle')
        "
        :description="
          t(
            'admin.settings.openaiExperimentalScheduler.stickyWeightedDescription',
          )
        "
      >
        <Toggle
          v-model="form.openai_advanced_scheduler_sticky_weighted_enabled"
        /> </SettingRow
      ><SettingRow
        v-if="form.openai_advanced_scheduler_enabled"
        :label="
          t(
            'admin.settings.openaiExperimentalScheduler.subscriptionPriorityTitle',
          )
        "
        :description="
          t(
            'admin.settings.openaiExperimentalScheduler.subscriptionPriorityDescription',
          )
        "
      >
        <Toggle
          v-model="form.openai_advanced_scheduler_subscription_priority_enabled"
        />
      </SettingRow>
      <div
        v-if="form.openai_advanced_scheduler_enabled"
        class="settings-flex-row settings-divider-row flex flex-col items-stretch gap-3 border-t border-line pt-5 sm:flex-row sm:items-start sm:justify-between sm:gap-6 settings-block"
      >
        <div class="min-w-0">
          <label
            class="text-sm font-medium text-foreground"
            for="openai-oauth-scheduling-rate-multiplier"
          >
            {{ t("admin.settings.openaiExperimentalScheduler.oauthRateTitle") }}
          </label>
          <p class="mt-0.5 text-xs text-muted">
            {{
              t(
                "admin.settings.openaiExperimentalScheduler.oauthRateWeightedDescription",
              )
            }}
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
            >x</span
          >
        </div>
      </div>
      <div
        v-if="form.openai_advanced_scheduler_enabled"
        class="border-t border-line pt-5 settings-block"
      >
        <div>
          <label class="text-sm font-medium text-foreground">
            {{ t("admin.settings.openaiExperimentalScheduler.weightsTitle") }}
          </label>
          <p class="mt-0.5 text-xs text-muted">
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
            <span class="text-xs font-medium text-muted">
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
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import { inject, computed } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { SCHEDULING_THRESHOLD_PLATFORMS } from "@/api/admin/settings";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, form } = settingsForm;

const schedulingThresholdPlatforms = SCHEDULING_THRESHOLD_PLATFORMS;

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

</script>
