<template>
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
</template>

<script setup lang="ts">
import { inject, ref } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { adminAPI } from "@/api";
import { extractApiErrorMessage } from "@/utils/apiError";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, appStore, upstreamBillingProbeLoading, upstreamBillingProbeForm } = settingsForm;

const upstreamBillingProbeSaving = ref(false);

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

</script>
