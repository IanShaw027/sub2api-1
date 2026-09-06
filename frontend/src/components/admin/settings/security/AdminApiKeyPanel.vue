<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.adminApiKey.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.adminApiKey.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Security Warning -->
 <div
 class="rounded-lg border border-warning bg-[color-mix(in_oklch,var(--warning)_10%,transparent)] p-4 "
 >
 <div class="settings-flex-row flex items-start">
 <Icon
 name="exclamationTriangle"
 size="md"
 class="mt-0.5 flex-shrink-0 text-warning-text"
 />
 <p class="ml-3 text-sm text-warning-text ">
 {{ t("admin.settings.adminApiKey.securityWarning") }}
 </p>
 </div>
 </div>

 <!-- Loading State -->
 <div
 v-if="adminApiKeyLoading"
 class="settings-flex-row flex items-center gap-2 text-muted"
 >
 <div
 class="h-4 w-4 animate-spin rounded-full border-b-2 border-accent"
 ></div>
 {{ t("common.loading") }}
 </div>

 <!-- No Key Configured -->
 <div
 v-else-if="!adminApiKeyExists"
 class="settings-flex-row settings-control-row flex items-center justify-between"
 >
 <span class="text-muted ">
 {{ t("admin.settings.adminApiKey.notConfigured") }}
 </span>
 <button
 type="button"
 @click="createAdminApiKey"
 :disabled="adminApiKeyOperating"
 class="btn-glass-primary"
 >
 <svg
 v-if="adminApiKeyOperating"
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
 adminApiKeyOperating
 ? t("admin.settings.adminApiKey.creating")
 : t("admin.settings.adminApiKey.create")
 }}
 </button>
 </div>

 <!-- Key Exists -->
 <div v-else class="space-y-4">
 <div class="settings-flex-row settings-control-row flex items-center justify-between">
 <div>
 <label
 class="mb-1 block text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.adminApiKey.currentKey") }}
 </label>
 <code
 class="rounded bg-surface-2 px-2 py-1 font-mono text-sm text-foreground "
 >
 {{ adminApiKeyMasked }}
 </code>
 </div>
 <div class="settings-flex-row flex gap-2">
 <button
 type="button"
 @click="regenerateAdminApiKey"
 :disabled="adminApiKeyOperating"
 class="btn-glass-secondary"
 >
 {{
 adminApiKeyOperating
 ? t("admin.settings.adminApiKey.regenerating")
 : t("admin.settings.adminApiKey.regenerate")
 }}
 </button>
 <button
 type="button"
 @click="deleteAdminApiKey"
 :disabled="adminApiKeyOperating"
 class="btn-glass-secondary text-danger-text "
 >
 {{ t("admin.settings.adminApiKey.delete") }}
 </button>
 </div>
 </div>

 <!-- Newly Generated Key Display -->
 <div
 v-if="newAdminApiKey"
 class="space-y-3 rounded-lg border border-success bg-[color-mix(in_oklch,var(--success)_10%,transparent)] p-4 "
 >
 <p
 class="text-sm font-medium text-success-text "
 >
 {{ t("admin.settings.adminApiKey.keyWarning") }}
 </p>
 <div class="settings-flex-row flex items-center gap-2">
 <code
 class="flex-1 select-all break-all rounded border border-success bg-surface px-3 py-2 font-mono text-sm "
 >
 {{ newAdminApiKey }}
 </code>
 <button
 type="button"
 @click="copyNewKey"
 class="btn-glass-primary flex-shrink-0"
 >
 {{ t("admin.settings.adminApiKey.copyKey") }}
 </button>
 </div>
 <p class="text-xs text-success-text ">
 {{ t("admin.settings.adminApiKey.usage") }}
 </p>
 </div>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject, ref } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { adminAPI } from "@/api";
import Icon from "@/components/icons/Icon.vue";
import { isStepUpCancelled, isStepUpBlocked, stepUpBlockReason } from "@/composables/useStepUp";
import { extractApiErrorMessage } from "@/utils/apiError";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  appStore,
  settingsStepUp,
  adminApiKeyLoading,
  adminApiKeyExists,
  adminApiKeyMasked,
} = settingsForm;

const adminApiKeyOperating = ref(false);

const newAdminApiKey = ref("");

async function createAdminApiKey() {
  adminApiKeyOperating.value = true;
  try {
    const result = await settingsStepUp.run(() =>
      adminAPI.settings.regenerateAdminApiKey(),
    );
    newAdminApiKey.value = result.key;
    adminApiKeyExists.value = true;
    adminApiKeyMasked.value =
      result.key.substring(0, 10) + "..." + result.key.slice(-4);
    appStore.showSuccess(t("admin.settings.adminApiKey.keyGenerated"));
  } catch (error: unknown) {
    if (isStepUpCancelled(error)) return;
    if (isStepUpBlocked(error)) {
      appStore.showError(
        stepUpBlockReason(error) === "STEP_UP_ADMIN_API_KEY_FORBIDDEN"
          ? t("stepUp.adminApiKeyForbidden")
          : t("stepUp.notEnabled"),
      );
      return;
    }
    appStore.showError(extractApiErrorMessage(error, t("common.error")));
  } finally {
    adminApiKeyOperating.value = false;
  }
}

async function regenerateAdminApiKey() {
  if (!confirm(t("admin.settings.adminApiKey.regenerateConfirm"))) return;
  await createAdminApiKey();
}

async function deleteAdminApiKey() {
  if (!confirm(t("admin.settings.adminApiKey.deleteConfirm"))) return;
  adminApiKeyOperating.value = true;
  try {
    await settingsStepUp.run(() => adminAPI.settings.deleteAdminApiKey());
    adminApiKeyExists.value = false;
    adminApiKeyMasked.value = "";
    newAdminApiKey.value = "";
    appStore.showSuccess(t("admin.settings.adminApiKey.keyDeleted"));
  } catch (error: unknown) {
    if (isStepUpCancelled(error)) return;
    if (isStepUpBlocked(error)) {
      appStore.showError(
        stepUpBlockReason(error) === "STEP_UP_ADMIN_API_KEY_FORBIDDEN"
          ? t("stepUp.adminApiKeyForbidden")
          : t("stepUp.notEnabled"),
      );
      return;
    }
    appStore.showError(extractApiErrorMessage(error, t("common.error")));
  } finally {
    adminApiKeyOperating.value = false;
  }
}

function copyNewKey() {
  navigator.clipboard
    .writeText(newAdminApiKey.value)
    .then(() => {
      appStore.showSuccess(t("admin.settings.adminApiKey.keyCopied"));
    })
    .catch(() => {
      appStore.showError(t("common.copyFailed"));
    });
}
</script>
