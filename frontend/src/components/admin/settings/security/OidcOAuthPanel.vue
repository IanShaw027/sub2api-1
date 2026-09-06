<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.oidc.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.oidc.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.oidc.enable")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.oidc.enableHint") }}
 </p>
 </div>
 <Toggle v-model="form.oidc_connect_enabled" />
 </div>

 <div
 v-if="form.oidc_connect_enabled"
 class="space-y-6 border-t border-line pt-4 "
 >
 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.providerName") }}
 </label>
 <input
 v-model="form.oidc_connect_provider_name"
 type="text"
 class="input"
 :placeholder="
 t('admin.settings.oidc.providerNamePlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.clientId") }}
 </label>
 <input
 v-model="form.oidc_connect_client_id"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.clientIdPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.clientSecret") }}
 </label>
 <input
 v-model="form.oidc_connect_client_secret"
 type="password"
 class="input font-mono text-sm"
 :placeholder="
 form.oidc_connect_client_secret_configured
 ? t(
 'admin.settings.oidc.clientSecretConfiguredPlaceholder',
 )
 : t('admin.settings.oidc.clientSecretPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{
 form.oidc_connect_client_secret_configured
 ? t("admin.settings.oidc.clientSecretConfiguredHint")
 : t("admin.settings.oidc.clientSecretHint")
 }}
 </p>
 </div>
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.issuerUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_issuer_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.issuerUrlPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.discoveryUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_discovery_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.discoveryUrlPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.authorizeUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_authorize_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.authorizeUrlPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.tokenUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_token_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.tokenUrlPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.userinfoUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_userinfo_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.userinfoUrlPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.jwksUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_jwks_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.oidc.jwksUrlPlaceholder')"
 />
 </div>
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.scopes") }}
 </label>
 <input
 v-model="form.oidc_connect_scopes"
 type="text"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.oidc.scopesPlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.oidc.scopesHint") }}
 </p>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.redirectUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_redirect_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.redirectUrlPlaceholder')
 "
 />
 <div
 class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
 >
 <button
 type="button"
 class="btn-glass-secondary w-fit"
 @click="setAndCopyOIDCRedirectUrl"
 >
 {{ t("admin.settings.oidc.quickSetCopy") }}
 </button>
 <code
 v-if="oidcRedirectUrlSuggestion"
 class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted "
 >
 {{ oidcRedirectUrlSuggestion }}
 </code>
 </div>
 <p class="settings-row-hint">
 {{ t("admin.settings.oidc.redirectUrlHint") }}
 </p>
 </div>

 <div class="settings-row lg:col-span-2">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.frontendRedirectUrl") }}
 </label>
 <input
 v-model="form.oidc_connect_frontend_redirect_url"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.frontendRedirectUrlPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.oidc.frontendRedirectUrlHint") }}
 </p>
 </div>
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.tokenAuthMethod") }}
 </label>
 <select
 v-model="form.oidc_connect_token_auth_method"
 class="input font-mono text-sm"
 >
 <option value="client_secret_post">
 client_secret_post
 </option>
 <option value="client_secret_basic">
 client_secret_basic
 </option>
 <option value="none">none</option>
 </select>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.clockSkewSeconds") }}
 </label>
 <input
 v-model.number="form.oidc_connect_clock_skew_seconds"
 type="number"
 min="0"
 max="600"
 class="input"
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.allowedSigningAlgs") }}
 </label>
 <input
 v-model="form.oidc_connect_allowed_signing_algs"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.allowedSigningAlgsPlaceholder')
 "
 />
 </div>
 </div>

 <div class="settings-row-group">
 <div
 class="flex items-center justify-between rounded border border-line px-4 py-3 "
 >
 <div>
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.oidc.usePkce") }}
 </label>
 </div>
 <Toggle
 v-model="form.oidc_connect_use_pkce"
 data-testid="oidc-connect-use-pkce"
 />
 </div>

 <div
 class="flex items-center justify-between rounded border border-line px-4 py-3 "
 >
 <div>
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.oidc.validateIdToken") }}
 </label>
 </div>
 <Toggle
 v-model="form.oidc_connect_validate_id_token"
 data-testid="oidc-connect-validate-id-token"
 />
 </div>

 <div
 class="flex items-center justify-between rounded border border-line px-4 py-3 "
 >
 <div>
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.oidc.requireEmailVerified") }}
 </label>
 </div>
 <Toggle
 v-model="form.oidc_connect_require_email_verified"
 />
 </div>
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.userinfoEmailPath") }}
 </label>
 <input
 v-model="form.oidc_connect_userinfo_email_path"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.userinfoEmailPathPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.userinfoIdPath") }}
 </label>
 <input
 v-model="form.oidc_connect_userinfo_id_path"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.userinfoIdPathPlaceholder')
 "
 />
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.oidc.userinfoUsernamePath") }}
 </label>
 <input
 v-model="form.oidc_connect_userinfo_username_path"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.oidc.userinfoUsernamePathPlaceholder')
 "
 />
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

const oidcRedirectUrlSuggestion = computed(() => {
  return buildApiCallbackUrl(
    form.api_base_url,
    currentOrigin,
    "/auth/oauth/oidc/callback",
  );
});

async function setAndCopyOIDCRedirectUrl() {
  const url = oidcRedirectUrlSuggestion.value;
  if (!url) return;

  form.oidc_connect_redirect_url = url;
  await copyToClipboard(url, t("admin.settings.oidc.redirectUrlSetAndCopied"));
}
</script>
