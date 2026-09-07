<template>
  <SettingsSection>
    <template #header>
      <div class="settings-flex-row flex items-center gap-2">
        <Icon name="shield" size="md" class="text-accent" />
        <h2 class="settings-card-title">
          {{ t("admin.settings.panelRateLimit.title") }}
        </h2>
      </div>
      <p class="settings-card-desc">
        {{ t("admin.settings.panelRateLimit.description") }}
      </p>
    </template>
    <div class="settings-rows">
      <div
        v-if="panelRateLimitLoading"
        class="settings-flex-row flex items-center gap-2 text-muted settings-block"
      >
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
        ></div>
        {{ t("common.loading") }}
      </div>
      <template v-else>
        <!-- 计数维度说明：按账号计数，反代部署无误伤 -->
        <div
          class="rounded-lg border border-accent-200 bg-accent-50 p-4 settings-block"
        >
          <div class="settings-flex-row flex items-start">
            <Icon
              name="infoCircle"
              size="md"
              class="mt-0.5 flex-shrink-0 text-accent-500"
            />
            <p class="ml-3 text-sm text-accent-700">
              {{ t("admin.settings.panelRateLimit.proxySafeNote") }}
            </p>
          </div>
        </div>

        <SettingRow
          :label="t('admin.settings.panelRateLimit.enabled')"
          :description="t('admin.settings.panelRateLimit.enabledHint')"
        >
          <Toggle v-model="panelRateLimitForm.enabled" />
        </SettingRow>

        <div v-if="panelRateLimitForm.enabled" class="settings-rows">
          <div class="settings-rows">
            <SettingRow
              :label="t('admin.settings.panelRateLimit.userRpm')"
              :description="t('admin.settings.panelRateLimit.userRpmHint')"
            >
              <div class="settings-flex-row flex items-center gap-2">
                <input
                  v-model.number="panelRateLimitForm.user_rpm"
                  data-testid="panel-rate-limit-user-rpm"
                  type="number"
                  min="0"
                  max="100000"
                  class="input w-32"
                />
                <span class="text-sm text-muted">
                  {{ t("admin.settings.panelRateLimit.perMinute") }}
                </span>
              </div>
            </SettingRow>
            <SettingRow
              :label="t('admin.settings.panelRateLimit.heavyRpm')"
              :description="t('admin.settings.panelRateLimit.heavyRpmHint')"
            >
              <div class="settings-flex-row flex items-center gap-2">
                <input
                  v-model.number="panelRateLimitForm.heavy_rpm"
                  type="number"
                  min="0"
                  max="100000"
                  class="input w-32"
                />
                <span class="text-sm text-muted">
                  {{ t("admin.settings.panelRateLimit.perMinute") }}
                </span>
              </div>
            </SettingRow>
            <SettingRow
              :label="t('admin.settings.panelRateLimit.publicIpRpm')"
              :description="t('admin.settings.panelRateLimit.publicIpRpmHint')"
            >
              <div class="settings-flex-row flex items-center gap-2">
                <input
                  v-model.number="panelRateLimitForm.public_ip_rpm"
                  type="number"
                  min="0"
                  max="100000"
                  class="input w-32"
                />
                <span class="text-sm text-muted">
                  {{ t("admin.settings.panelRateLimit.perMinute") }}
                </span>
              </div>
            </SettingRow>
          </div>
          <SettingRow
            :label="t('admin.settings.panelRateLimit.exemptAdmin')"
            :description="t('admin.settings.panelRateLimit.exemptAdminHint')"
          >
            <Toggle v-model="panelRateLimitForm.exempt_admin" />
          </SettingRow>
        </div>

        <div
          class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 settings-block"
        >
          <button
            type="button"
            data-testid="panel-rate-limit-save"
            @click="savePanelRateLimitSettings"
            :disabled="panelRateLimitSaving"
            class="btn-glass-primary"
          >
            <svg
              v-if="panelRateLimitSaving"
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
            {{ panelRateLimitSaving ? t("common.saving") : t("common.save") }}
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
import Icon from "@/components/icons/Icon.vue";
import Toggle from "@/components/common/Toggle.vue";
import { extractApiErrorMessage } from "@/utils/apiError";

const settingsForm = inject(SettingsFormKey)!;
const { t, appStore, panelRateLimitLoading, panelRateLimitForm } = settingsForm;

const panelRateLimitSaving = ref(false);

async function savePanelRateLimitSettings() {
  panelRateLimitSaving.value = true;
  try {
    const updated = await adminAPI.settings.updatePanelRateLimitSettings({
      enabled: panelRateLimitForm.enabled,
      user_rpm: panelRateLimitForm.user_rpm,
      heavy_rpm: panelRateLimitForm.heavy_rpm,
      exempt_admin: panelRateLimitForm.exempt_admin,
      public_ip_rpm: panelRateLimitForm.public_ip_rpm,
    });
    Object.assign(panelRateLimitForm, updated);
    appStore.showSuccess(t("admin.settings.panelRateLimit.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.panelRateLimit.saveFailed"),
      ),
    );
  } finally {
    panelRateLimitSaving.value = false;
  }
}
</script>
