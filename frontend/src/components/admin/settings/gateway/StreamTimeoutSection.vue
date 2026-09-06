<template>
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
 class="settings-flex-row flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <template v-else>
 <!-- Enable Stream Timeout -->
 <div class="settings-flex-row settings-control-row flex items-center justify-between">
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
 class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 "
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
</template>

<script setup lang="ts">
import { inject, ref } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { adminAPI } from "@/api";
import { extractApiErrorMessage } from "@/utils/apiError";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, appStore, streamTimeoutLoading, streamTimeoutForm } = settingsForm;

const streamTimeoutSaving = ref(false);

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

</script>
