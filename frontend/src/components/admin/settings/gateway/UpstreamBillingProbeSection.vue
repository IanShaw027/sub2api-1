<template>
  <!-- Upstream Billing Probe Settings -->
  <SettingsSection
    data-testid="upstream-billing-probe-settings"
    :title="t('admin.settings.upstreamBillingProbe.title')"
    :description="t('admin.settings.upstreamBillingProbe.description')"
  >
    <div class="settings-rows">
      <div
        v-if="upstreamBillingProbeLoading"
        class="settings-flex-row flex items-center gap-2 text-muted settings-block"
      >
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
        ></div>
        {{ t("common.loading") }}
      </div>
      <template v-else>
        <SettingRow
          :label="t('admin.settings.upstreamBillingProbe.enabled')"
          :description="t('admin.settings.upstreamBillingProbe.enabledHint')"
        >
          <Toggle
            v-model="upstreamBillingProbeForm.enabled"
            :aria-label="t('admin.settings.upstreamBillingProbe.enabled')"
            data-testid="upstream-billing-probe-enabled"
          />
        </SettingRow>

        <SettingRow
          v-if="upstreamBillingProbeForm.enabled"
          :label="t('admin.settings.upstreamBillingProbe.intervalMinutes')"
          label-for="upstream-billing-probe-interval"
          :description="t('admin.settings.upstreamBillingProbe.intervalHint')"
        >
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
        </SettingRow>

        <div
          class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 settings-block"
        >
          <button
            type="button"
            class="btn-glass-primary"
            :disabled="upstreamBillingProbeSaving"
            data-testid="upstream-billing-probe-save"
            @click="saveUpstreamBillingProbeSettings"
          >
            {{
              upstreamBillingProbeSaving ? t("common.saving") : t("common.save")
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
