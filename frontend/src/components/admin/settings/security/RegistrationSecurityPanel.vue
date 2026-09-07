<template>
  <SettingsSection
    :title="t('admin.settings.registration.title')"
    :description="t('admin.settings.registration.description')"
  >
    <div class="settings-rows">
      <!-- Enable Registration --><SettingRow
        :label="t('admin.settings.registration.enableRegistration')"
        :description="t('admin.settings.registration.enableRegistrationHint')"
      >
        <Toggle v-model="form.registration_enabled" /> </SettingRow
      ><!-- Email Verification --><SettingRow
        :label="t('admin.settings.registration.emailVerification')"
        :description="t('admin.settings.registration.emailVerificationHint')"
      >
        <Toggle v-model="form.email_verify_enabled" /> </SettingRow
      ><!-- Email Suffix Whitelist -->
      <SettingRow
        :label="t('admin.settings.registration.emailSuffixWhitelist')"
      >
        <p class="settings-card-desc">
          {{ t("admin.settings.registration.emailSuffixWhitelistHint") }}
        </p>
        <div class="mt-3 rounded-lg border border-line bg-surface p-2">
          <div class="settings-flex-row flex flex-wrap items-center gap-2">
            <span
              v-for="suffix in registrationEmailSuffixWhitelistTags"
              :key="suffix"
              class="inline-flex items-center gap-1 rounded bg-surface-2 px-2 py-1 text-xs font-mono text-foreground"
            >
              <span>{{ suffix }}</span>
              <button
                type="button"
                class="rounded-full text-muted hover:bg-surface-2 hover:text-foreground"
                @click="removeRegistrationEmailSuffixWhitelistTag(suffix)"
              >
                <Icon
                  name="x"
                  size="xs"
                  class="h-3.5 w-3.5"
                  :stroke-width="2"
                />
              </button>
            </span>

            <div
              class="settings-flex-row flex min-w-[220px] flex-1 items-center gap-1 rounded border border-transparent px-2 py-1 focus-within:border-accent"
            >
              <input
                v-model="registrationEmailSuffixWhitelistDraft"
                type="text"
                class="w-full bg-transparent text-sm font-mono text-foreground outline-none placeholder:text-muted"
                :placeholder="
                  t(
                    'admin.settings.registration.emailSuffixWhitelistPlaceholder',
                  )
                "
                @input="handleRegistrationEmailSuffixWhitelistDraftInput"
                @keydown="handleRegistrationEmailSuffixWhitelistDraftKeydown"
                @blur="commitRegistrationEmailSuffixWhitelistDraft"
                @paste="handleRegistrationEmailSuffixWhitelistPaste"
              />
            </div>
          </div>
        </div>
        <p class="mt-2 text-xs text-muted">
          {{ t("admin.settings.registration.emailSuffixWhitelistInputHint") }}
        </p>
      </SettingRow>
      <!-- Email Domain Quota --><SettingRow
        :label="t('admin.settings.registration.emailDomainQuota')"
        :description="t('admin.settings.registration.emailDomainQuotaHint')"
      >
        <Toggle
          v-model="form.registration_email_domain_quota_enabled"
        /> </SettingRow
      ><!-- Promo Code --><SettingRow
        :label="t('admin.settings.registration.promoCode')"
        :description="t('admin.settings.registration.promoCodeHint')"
      >
        <Toggle v-model="form.promo_code_enabled" /> </SettingRow
      ><!-- Invitation Code --><SettingRow
        :label="t('admin.settings.registration.invitationCode')"
        :description="t('admin.settings.registration.invitationCodeHint')"
      >
        <Toggle v-model="form.invitation_code_enabled" /> </SettingRow
      ><!-- Password Reset - Only show when email verification is enabled --><SettingRow
        v-if="form.email_verify_enabled"
        :label="t('admin.settings.registration.passwordReset')"
        :description="t('admin.settings.registration.passwordResetHint')"
      >
        <Toggle v-model="form.password_reset_enabled" /> </SettingRow
      ><!-- Frontend URL - Only show when password reset is enabled --><SettingRow
        v-if="form.email_verify_enabled && form.password_reset_enabled"
        :label="t('admin.settings.registration.frontendUrl')"
        :description="t('admin.settings.registration.frontendUrlHint')"
      >
        <input
          v-model="form.frontend_url"
          type="url"
          class="input"
          :placeholder="t('admin.settings.registration.frontendUrlPlaceholder')"
        /> </SettingRow
      ><!-- TOTP 2FA --><SettingRow
        :label="t('admin.settings.registration.totp')"
      >
        <template #description
          ><p class="text-sm text-muted">
            {{ t("admin.settings.registration.totpHint") }}
          </p>
          <p
            v-if="!form.totp_encryption_key_configured"
            class="mt-2 text-sm text-warning-text"
          >
            {{ t("admin.settings.registration.totpKeyNotConfigured") }}
          </p></template
        >
        <Toggle
          v-model="form.totp_enabled"
          :disabled="!form.totp_encryption_key_configured"
        /> </SettingRow
      ><!-- Passkey sign-in -->
      <div
        data-testid="passkey-settings"
        class="border-t border-line pt-4 settings-block"
      >
        <SettingRow
          :label="t('admin.settings.security.passkey')"
          :description="t('admin.settings.security.passkeyHint')"
        >
          <Toggle
            v-model="form.passkey_enabled"
            data-testid="passkey-toggle"
            :disabled="!form.passkey_configured"
          />
        </SettingRow>
        <div
          class="mt-3 rounded-lg border px-3 py-2 text-sm"
          :class="
            form.passkey_configured
              ? 'border-success bg-[color-mix(in_oklch,var(--success)_10%,transparent)] text-success-text '
              : 'border-warning bg-[color-mix(in_oklch,var(--warning)_10%,transparent)] text-warning-text '
          "
          data-testid="passkey-config-status"
        >
          <p class="font-medium">
            {{
              form.passkey_configured
                ? t("admin.settings.security.passkeyConfigured")
                : t("admin.settings.security.passkeyNotConfigured")
            }}
          </p>
          <p class="mt-1 break-all">
            {{ t("admin.settings.security.passkeyRPID") }}:
            {{
              form.passkey_rp_id ||
              t("admin.settings.security.passkeyValueNotConfigured")
            }}
          </p>
          <p class="mt-1 break-all">
            {{ t("admin.settings.security.passkeyOrigins") }}:
            {{
              form.passkey_rp_origins.length > 0
                ? form.passkey_rp_origins.join(", ")
                : t("admin.settings.security.passkeyValueNotConfigured")
            }}
          </p>
          <p v-if="!form.passkey_configured" class="mt-2">
            {{ t("admin.settings.security.passkeyDeploymentHint") }}
          </p>
        </div>
      </div>
      <!-- 敏感操作 step-up 2FA --><SettingRow
        :label="t('admin.settings.security.stepUp')"
        :description="t('admin.settings.security.stepUpHint')"
      >
        <Toggle v-model="form.step_up_enabled" /> </SettingRow
      ><!-- 会话 IP/UA 绑定 --><SettingRow
        :label="t('admin.settings.security.sessionBinding')"
        :description="t('admin.settings.security.sessionBindingHint')"
      >
        <Toggle v-model="form.session_binding_enabled" /> </SettingRow
      ><!-- 审计日志保留天数 --><SettingRow
        :label="t('admin.settings.security.auditRetention')"
        :description="t('admin.settings.security.auditRetentionHint')"
      >
        <input
          v-model.number="form.audit_log_retention_days"
          type="number"
          min="0"
          class="input w-28 text-right"
        />
      </SettingRow>
    </div>
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Icon from "@/components/icons/Icon.vue";
import Toggle from "@/components/common/Toggle.vue";
import { isRegistrationEmailSuffixDomainValid, normalizeRegistrationEmailSuffixDomain, parseRegistrationEmailSuffixWhitelistInput } from "@/utils/registrationEmailPolicy";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  form,
  registrationEmailSuffixWhitelistTags,
  registrationEmailSuffixWhitelistDraft,
} = settingsForm;

const registrationEmailSuffixWhitelistSeparatorKeys = new Set([
  " ",
  ",",
  "，",
  "Enter",
  "Tab",
]);

function removeRegistrationEmailSuffixWhitelistTag(suffix: string) {
  registrationEmailSuffixWhitelistTags.value =
    registrationEmailSuffixWhitelistTags.value.filter(
      (item) => item !== suffix,
    );
}

function addRegistrationEmailSuffixWhitelistTag(raw: string) {
  const suffix = normalizeRegistrationEmailSuffixDomain(raw);
  if (
    !isRegistrationEmailSuffixDomainValid(suffix) ||
    registrationEmailSuffixWhitelistTags.value.includes(suffix)
  ) {
    return;
  }
  registrationEmailSuffixWhitelistTags.value = [
    ...registrationEmailSuffixWhitelistTags.value,
    suffix,
  ];
}

function commitRegistrationEmailSuffixWhitelistDraft() {
  if (!registrationEmailSuffixWhitelistDraft.value) {
    return;
  }
  addRegistrationEmailSuffixWhitelistTag(
    registrationEmailSuffixWhitelistDraft.value,
  );
  registrationEmailSuffixWhitelistDraft.value = "";
}

function handleRegistrationEmailSuffixWhitelistDraftInput() {
  registrationEmailSuffixWhitelistDraft.value =
    normalizeRegistrationEmailSuffixDomain(
      registrationEmailSuffixWhitelistDraft.value,
    );
}

function handleRegistrationEmailSuffixWhitelistDraftKeydown(
  event: KeyboardEvent,
) {
  if (event.isComposing) {
    return;
  }

  if (registrationEmailSuffixWhitelistSeparatorKeys.has(event.key)) {
    event.preventDefault();
    commitRegistrationEmailSuffixWhitelistDraft();
    return;
  }

  if (
    event.key === "Backspace" &&
    !registrationEmailSuffixWhitelistDraft.value &&
    registrationEmailSuffixWhitelistTags.value.length > 0
  ) {
    registrationEmailSuffixWhitelistTags.value.pop();
  }
}

function handleRegistrationEmailSuffixWhitelistPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData("text") || "";
  if (!text.trim()) {
    return;
  }
  event.preventDefault();
  const tokens = parseRegistrationEmailSuffixWhitelistInput(text);
  for (const token of tokens) {
    addRegistrationEmailSuffixWhitelistTag(token);
  }
}
</script>
