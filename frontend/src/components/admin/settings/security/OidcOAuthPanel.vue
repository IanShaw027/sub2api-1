<template>
  <SettingsSection
    :title="t('admin.settings.oidc.title')"
    :description="t('admin.settings.oidc.description')"
  >
    <div class="settings-rows">
      <SettingRow
        :label="t('admin.settings.oidc.enable')"
        :description="t('admin.settings.oidc.enableHint')"
      >
        <Toggle v-model="form.oidc_connect_enabled" />
      </SettingRow>
      <div v-if="form.oidc_connect_enabled" class="settings-rows">
        <div class="settings-rows">
          <SettingRow :label="t('admin.settings.oidc.providerName')">
            <input
              v-model="form.oidc_connect_provider_name"
              type="text"
              class="input"
              :placeholder="t('admin.settings.oidc.providerNamePlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.clientId')">
            <input
              v-model="form.oidc_connect_client_id"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.clientIdPlaceholder')"
            />
          </SettingRow>
          <SettingRow
            :label="t('admin.settings.oidc.clientSecret')"
            :description="
              form.oidc_connect_client_secret_configured
                ? t('admin.settings.oidc.clientSecretConfiguredHint')
                : t('admin.settings.oidc.clientSecretHint')
            "
          >
            <input
              v-model="form.oidc_connect_client_secret"
              type="password"
              class="input font-mono text-sm"
              :placeholder="
                form.oidc_connect_client_secret_configured
                  ? t('admin.settings.oidc.clientSecretConfiguredPlaceholder')
                  : t('admin.settings.oidc.clientSecretPlaceholder')
              "
            />
          </SettingRow>
        </div>
        <div class="settings-rows">
          <SettingRow :label="t('admin.settings.oidc.issuerUrl')">
            <input
              v-model="form.oidc_connect_issuer_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.issuerUrlPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.discoveryUrl')">
            <input
              v-model="form.oidc_connect_discovery_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.discoveryUrlPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.authorizeUrl')">
            <input
              v-model="form.oidc_connect_authorize_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.authorizeUrlPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.tokenUrl')">
            <input
              v-model="form.oidc_connect_token_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.tokenUrlPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.userinfoUrl')">
            <input
              v-model="form.oidc_connect_userinfo_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.userinfoUrlPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.jwksUrl')">
            <input
              v-model="form.oidc_connect_jwks_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.jwksUrlPlaceholder')"
            />
          </SettingRow>
        </div>
        <div class="settings-rows">
          <SettingRow
            :label="t('admin.settings.oidc.scopes')"
            :description="t('admin.settings.oidc.scopesHint')"
          >
            <input
              v-model="form.oidc_connect_scopes"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.scopesPlaceholder')"
            />
          </SettingRow>
          <SettingRow
            :label="t('admin.settings.oidc.redirectUrl')"
            :description="t('admin.settings.oidc.redirectUrlHint')"
          >
            <input
              v-model="form.oidc_connect_redirect_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.redirectUrlPlaceholder')"
            />
            <div
              class="settings-flex-row mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
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
                class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted"
              >
                {{ oidcRedirectUrlSuggestion }}
              </code>
            </div>
          </SettingRow>
          <SettingRow
            :label="t('admin.settings.oidc.frontendRedirectUrl')"
            :description="t('admin.settings.oidc.frontendRedirectUrlHint')"
          >
            <input
              v-model="form.oidc_connect_frontend_redirect_url"
              type="text"
              class="input font-mono text-sm"
              :placeholder="
                t('admin.settings.oidc.frontendRedirectUrlPlaceholder')
              "
            />
          </SettingRow>
        </div>
        <div class="settings-rows">
          <SettingRow :label="t('admin.settings.oidc.tokenAuthMethod')">
            <select
              v-model="form.oidc_connect_token_auth_method"
              class="input font-mono text-sm"
            >
              <option value="client_secret_post">client_secret_post</option>
              <option value="client_secret_basic">client_secret_basic</option>
              <option value="none">none</option>
            </select>
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.clockSkewSeconds')">
            <input
              v-model.number="form.oidc_connect_clock_skew_seconds"
              type="number"
              min="0"
              max="600"
              class="input"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.allowedSigningAlgs')">
            <input
              v-model="form.oidc_connect_allowed_signing_algs"
              type="text"
              class="input font-mono text-sm"
              :placeholder="
                t('admin.settings.oidc.allowedSigningAlgsPlaceholder')
              "
            />
          </SettingRow>
        </div>
        <div class="settings-rows">
          <SettingRow :label="t('admin.settings.oidc.usePkce')">
            <Toggle
              v-model="form.oidc_connect_use_pkce"
              data-testid="oidc-connect-use-pkce"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.validateIdToken')">
            <Toggle
              v-model="form.oidc_connect_validate_id_token"
              data-testid="oidc-connect-validate-id-token"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.requireEmailVerified')">
            <Toggle v-model="form.oidc_connect_require_email_verified" />
          </SettingRow>
        </div>
        <div class="settings-rows">
          <SettingRow :label="t('admin.settings.oidc.userinfoEmailPath')">
            <input
              v-model="form.oidc_connect_userinfo_email_path"
              type="text"
              class="input font-mono text-sm"
              :placeholder="
                t('admin.settings.oidc.userinfoEmailPathPlaceholder')
              "
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.userinfoIdPath')">
            <input
              v-model="form.oidc_connect_userinfo_id_path"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.oidc.userinfoIdPathPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('admin.settings.oidc.userinfoUsernamePath')">
            <input
              v-model="form.oidc_connect_userinfo_username_path"
              type="text"
              class="input font-mono text-sm"
              :placeholder="
                t('admin.settings.oidc.userinfoUsernamePathPlaceholder')
              "
            />
          </SettingRow>
        </div>
      </div>
    </div>
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
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
