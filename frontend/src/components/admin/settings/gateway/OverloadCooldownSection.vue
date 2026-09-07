<template>
  <!-- Overload Cooldown (529) Settings -->
  <SettingsSection
    :title="t('admin.settings.overloadCooldown.title')"
    :description="t('admin.settings.overloadCooldown.description')"
  >
    <div class="settings-rows">
      <div
        v-if="overloadCooldownLoading"
        class="settings-flex-row flex items-center gap-2 text-muted settings-block"
      >
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
        ></div>
        {{ t("common.loading") }}
      </div>
      <template v-else>
        <SettingRow
          :label="t('admin.settings.overloadCooldown.enabled')"
          :description="t('admin.settings.overloadCooldown.enabledHint')"
        >
          <Toggle v-model="overloadCooldownForm.enabled" />
        </SettingRow>

        <div v-if="overloadCooldownForm.enabled" class="settings-rows">
          <SettingRow
            :label="t('admin.settings.overloadCooldown.cooldownMinutes')"
            :description="
              t('admin.settings.overloadCooldown.cooldownMinutesHint')
            "
          >
            <input
              v-model.number="overloadCooldownForm.cooldown_minutes"
              type="number"
              min="1"
              max="120"
              class="input w-32"
            />
          </SettingRow>
        </div>

        <div
          class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 settings-block"
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
            {{ overloadCooldownSaving ? t("common.saving") : t("common.save") }}
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
const { t, appStore, overloadCooldownLoading, overloadCooldownForm } = settingsForm;

const overloadCooldownSaving = ref(false);

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

</script>
