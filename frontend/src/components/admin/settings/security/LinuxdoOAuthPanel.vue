<template>
  <SettingsSection
    :title="t('admin.settings.linuxdo.title')"
    :description="t('admin.settings.linuxdo.description')"
  >
    <div class="settings-rows">
      <SettingRow
        :label="t('admin.settings.linuxdo.enable')"
        :description="t('admin.settings.linuxdo.enableHint')"
      >
        <Toggle v-model="form.linuxdo_connect_enabled" />
      </SettingRow>
      <div v-if="form.linuxdo_connect_enabled" class="settings-rows">
        <div class="settings-rows">
          <SettingRow
            :label="t('admin.settings.linuxdo.clientId')"
            :description="t('admin.settings.linuxdo.clientIdHint')"
          >
            <input
              v-model="form.linuxdo_connect_client_id"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.linuxdo.clientIdPlaceholder')"
            />
          </SettingRow>
          <SettingRow
            :label="t('admin.settings.linuxdo.clientSecret')"
            :description="
              form.linuxdo_connect_client_secret_configured
                ? t('admin.settings.linuxdo.clientSecretConfiguredHint')
                : t('admin.settings.linuxdo.clientSecretHint')
            "
          >
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
          </SettingRow>
          <SettingRow
            :label="t('admin.settings.linuxdo.redirectUrl')"
            :description="t('admin.settings.linuxdo.redirectUrlHint')"
          >
            <input
              v-model="form.linuxdo_connect_redirect_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.linuxdo.redirectUrlPlaceholder')"
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
                class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted"
              >
                {{ linuxdoRedirectUrlSuggestion }}
              </code>
            </div>
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
