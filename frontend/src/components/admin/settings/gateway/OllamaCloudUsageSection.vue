<template>
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
 <div v-if="ollamaCloudUsageLoading" class="settings-flex-row flex items-center gap-2 text-muted">
 <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"></div>
 {{ t("common.loading") }}
 </div>
 <template v-else>
 <div class="settings-flex-row settings-control-row flex items-center justify-between gap-4">
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
 <div class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 ">
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
</template>

<script setup lang="ts">
import { inject, ref } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { adminAPI } from "@/api";
import { extractApiErrorMessage } from "@/utils/apiError";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, appStore, ollamaCloudUsageLoading, ollamaCloudUsageForm } = settingsForm;

const ollamaCloudUsageSaving = ref(false);

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

</script>
