<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.linuxdo.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.linuxdo.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-flex-row settings-control-row flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.linuxdo.enable")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.linuxdo.enableHint") }}
 </p>
 </div>
 <Toggle v-model="form.linuxdo_connect_enabled" />
 </div>

 <div
 v-if="form.linuxdo_connect_enabled"
 class="border-t border-line pt-4 "
 >
 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.linuxdo.clientId") }}
 </label>
 <input
 v-model="form.linuxdo_connect_client_id"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.linuxdo.clientIdPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.linuxdo.clientIdHint") }}
 </p>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.linuxdo.clientSecret") }}
 </label>
 <input
 v-model="form.linuxdo_connect_client_secret"
 type="password"
 class="input font-mono text-sm"
 :placeholder="
 form.linuxdo_connect_client_secret_configured
 ? t(
 'admin.settings.linuxdo.clientSecretConfiguredPlaceholder',
 )
 : t('admin.settings.linuxdo.clientSecretPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{
 form.linuxdo_connect_client_secret_configured
 ? t(
 "admin.settings.linuxdo.clientSecretConfiguredHint",
 )
 : t("admin.settings.linuxdo.clientSecretHint")
 }}
 </p>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.linuxdo.redirectUrl") }}
 </label>
 <input
 v-model="form.linuxdo_connect_redirect_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.linuxdo.redirectUrlPlaceholder')
 "
 />
 <div
 class="settings-flex-row mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
 >
 <button
 type="button"
 class="btn-glass-secondary w-fit"
 @click="setAndCopyLinuxdoRedirectUrl"
 >
 {{ t("admin.settings.linuxdo.quickSetCopy") }}
 </button>
 <code
 v-if="linuxdoRedirectUrlSuggestion"
 class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted "
 >
 {{ linuxdoRedirectUrlSuggestion }}
 </code>
 </div>
 <p class="settings-row-hint">
 {{ t("admin.settings.linuxdo.redirectUrlHint") }}
 </p>
 </div>
 </div>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject, computed } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Toggle from "@/components/common/Toggle.vue";
import { useClipboard } from "@/composables/useClipboard";
import { buildApiCallbackUrl } from "./oauthCallbackUrl";

const settingsForm = inject(SettingsFormKey)!;
const { t, form, currentOrigin } = settingsForm;

const { copyToClipboard } = useClipboard();

const linuxdoRedirectUrlSuggestion = computed(() => {
  return buildApiCallbackUrl(
    form.api_base_url,
    currentOrigin,
    "/auth/oauth/linuxdo/callback",
  );
});

async function setAndCopyLinuxdoRedirectUrl() {
  const url = linuxdoRedirectUrlSuggestion.value;
  if (!url) return;

  form.linuxdo_connect_redirect_url = url;
  await copyToClipboard(
    url,
    t("admin.settings.linuxdo.redirectUrlSetAndCopied"),
  );
}
</script>
