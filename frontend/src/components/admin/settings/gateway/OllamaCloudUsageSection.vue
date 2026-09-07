<template>
  <!-- Ollama Cloud Usage Settings -->
  <SettingsSection
    data-testid="ollama-cloud-usage-global-settings"
    :title="t('admin.settings.ollamaCloudUsage.title')"
    :description="t('admin.settings.ollamaCloudUsage.description')"
  >
    <div class="settings-rows">
      <div
        v-if="ollamaCloudUsageLoading"
        class="settings-flex-row flex items-center gap-2 text-muted settings-block"
      >
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
        ></div>
        {{ t("common.loading") }}
      </div>
      <template v-else>
        <SettingRow
          :label="t('admin.settings.ollamaCloudUsage.enabled')"
          :description="t('admin.settings.ollamaCloudUsage.enabledHint')"
        >
          <Toggle
            v-model="ollamaCloudUsageForm.enabled"
            :aria-label="t('admin.settings.ollamaCloudUsage.enabled')"
            data-testid="ollama-cloud-usage-global-enabled"
          />
        </SettingRow>
        <div v-if="ollamaCloudUsageForm.enabled" class="settings-rows">
          <SettingRow
            :label="t('admin.settings.ollamaCloudUsage.debounceMinutes')"
            label-for="ollama-cloud-usage-debounce"
            :description="t('admin.settings.ollamaCloudUsage.debounceHint')"
          >
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
          </SettingRow>
          <SettingRow
            :label="t('admin.settings.ollamaCloudUsage.intervalMinutes')"
            label-for="ollama-cloud-usage-interval"
            :description="t('admin.settings.ollamaCloudUsage.intervalHint')"
          >
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
          </SettingRow>
        </div>
        <div
          class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 settings-block"
        >
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
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
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
