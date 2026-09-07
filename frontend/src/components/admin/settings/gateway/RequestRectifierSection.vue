<template>
  <!-- Request Rectifier Settings -->
  <SettingsSection
    :title="t('admin.settings.rectifier.title')"
    :description="t('admin.settings.rectifier.description')"
  >
    <div class="settings-rows">
      <!-- Loading State -->
      <div
        v-if="rectifierLoading"
        class="settings-flex-row flex items-center gap-2 text-muted settings-block"
      >
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
        ></div>
        {{ t("common.loading") }}
      </div>
      <template v-else>
        <!-- Master Toggle -->
        <SettingRow
          :label="t('admin.settings.rectifier.enabled')"
          :description="t('admin.settings.rectifier.enabledHint')"
        >
          <Toggle v-model="rectifierForm.enabled" />
        </SettingRow>

        <!-- Sub-toggles (only show when master is enabled) -->
        <div
          v-if="rectifierForm.enabled"
          class="space-y-4 border-t border-line pt-4 settings-block"
        >
          <!-- Thinking Signature Rectifier --><SettingRow
            :label="t('admin.settings.rectifier.thinkingSignature')"
            :description="t('admin.settings.rectifier.thinkingSignatureHint')"
          >
            <Toggle
              v-model="rectifierForm.thinking_signature_enabled"
            /> </SettingRow
          ><!-- Thinking Budget Rectifier --><SettingRow
            :label="t('admin.settings.rectifier.thinkingBudget')"
            :description="t('admin.settings.rectifier.thinkingBudgetHint')"
          >
            <Toggle
              v-model="rectifierForm.thinking_budget_enabled"
            /> </SettingRow
          ><!-- API Key Signature Rectifier --><SettingRow
            :label="t('admin.settings.rectifier.apikeySignature')"
            :description="t('admin.settings.rectifier.apikeySignatureHint')"
          >
            <Toggle
              v-model="rectifierForm.apikey_signature_enabled"
            /> </SettingRow
          ><!-- Custom Patterns (only when apikey_signature_enabled) -->
          <div
            v-if="rectifierForm.apikey_signature_enabled"
            class="ml-4 space-y-3 border-l-2 border-line pl-4"
          >
            <div>
              <label class="text-sm font-medium text-foreground">{{
                t("admin.settings.rectifier.apikeyPatterns")
              }}</label>
              <p class="text-xs text-muted">
                {{ t("admin.settings.rectifier.apikeyPatternsHint") }}
              </p>
            </div>
            <div
              v-for="(_, index) in rectifierForm.apikey_signature_patterns"
              :key="index"
              class="settings-flex-row flex items-center gap-2"
            >
              <input
                v-model="rectifierForm.apikey_signature_patterns[index]"
                type="text"
                class="input input-sm flex-1"
                :placeholder="
                  t('admin.settings.rectifier.apikeyPatternPlaceholder')
                "
              />
              <button
                type="button"
                @click="
                  rectifierForm.apikey_signature_patterns.splice(index, 1)
                "
                class="btn btn-ghost btn-xs text-danger-500 hover:text-danger-700"
              >
                <svg
                  class="h-4 w-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
            <button
              type="button"
              @click="rectifierForm.apikey_signature_patterns.push('')"
              class="btn btn-ghost btn-xs text-accent"
            >
              + {{ t("admin.settings.rectifier.addPattern") }}
            </button>
          </div>
        </div>

        <!-- Save Button -->
        <div
          class="settings-flex-row settings-divider-row flex justify-end border-t border-line pt-4 settings-block"
        >
          <button
            type="button"
            @click="saveRectifierSettings"
            :disabled="rectifierSaving"
            class="btn-glass-primary"
          >
            <svg
              v-if="rectifierSaving"
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
            {{ rectifierSaving ? t("common.saving") : t("common.save") }}
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
const { t, appStore, rectifierLoading, rectifierForm } = settingsForm;

const rectifierSaving = ref(false);

async function saveRectifierSettings() {
  rectifierSaving.value = true;
  try {
    const updated = await adminAPI.settings.updateRectifierSettings({
      enabled: rectifierForm.enabled,
      thinking_signature_enabled: rectifierForm.thinking_signature_enabled,
      thinking_budget_enabled: rectifierForm.thinking_budget_enabled,
      apikey_signature_enabled: rectifierForm.apikey_signature_enabled,
      apikey_signature_patterns: rectifierForm.apikey_signature_patterns.filter(
        (p) => p.trim() !== "",
      ),
    });
    Object.assign(rectifierForm, updated);
    if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
      rectifierForm.apikey_signature_patterns = [];
    }
    appStore.showSuccess(t("admin.settings.rectifier.saved"));
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.settings.rectifier.saveFailed")),
    );
  } finally {
    rectifierSaving.value = false;
  }
}

</script>
