<template>
  <!-- Rate Limit Cooldown (429) Settings -->
  <SettingsSection
    :title="t('admin.settings.rateLimit429Cooldown.title')"
    :description="t('admin.settings.rateLimit429Cooldown.description')"
  >
    <div class="settings-rows">
      <div
        v-if="rateLimit429CooldownLoading"
        class="settings-flex-row flex items-center gap-2 text-muted settings-block"
      >
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
        ></div>
        {{ t("common.loading") }}
      </div>
      <template v-else>
        <SettingRow
          :label="t('admin.settings.rateLimit429Cooldown.enabled')"
          :description="t('admin.settings.rateLimit429Cooldown.enabledHint')"
        >
          <Toggle v-model="rateLimit429CooldownForm.enabled" />
        </SettingRow>

        <div v-if="rateLimit429CooldownForm.enabled" class="settings-rows">
          <SettingRow
            :label="t('admin.settings.rateLimit429Cooldown.cooldownSeconds')"
            :description="
              t('admin.settings.rateLimit429Cooldown.cooldownSecondsHint')
            "
          >
            <input
              v-model.number="rateLimit429CooldownForm.cooldown_seconds"
              type="number"
              min="1"
              max="7200"
              class="input w-32"
            />
          </SettingRow>
        </div>

        <SettingRow v-else>
          <template #label
            ><label class="text-sm"
              >冷却/重试间隔(ms)<input
                v-model.number="rateLimit429CooldownForm.retry_interval_ms"
                type="number"
                min="100"
                max="60000"
                class="input mt-2 w-full" /></label
          ></template>
          <label class="text-sm"
            >最大重试时间(秒)<input
              v-model.number="
                rateLimit429CooldownForm.retry_max_duration_seconds
              "
              type="number"
              min="1"
              max="600"
              class="input mt-2 w-full" /></label
          ><label class="text-sm"
            >换号次数<input
              v-model.number="rateLimit429CooldownForm.max_account_switches"
              type="number"
              min="0"
              max="10"
              class="input mt-2 w-full"
          /></label>
        </SettingRow>

        <div
          class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 settings-block"
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
              rateLimit429CooldownSaving ? t("common.saving") : t("common.save")
            }}
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
const { t, appStore, rateLimit429CooldownLoading, rateLimit429CooldownForm } = settingsForm;

const rateLimit429CooldownSaving = ref(false);

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

</script>
