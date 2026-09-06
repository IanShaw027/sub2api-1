<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.registration.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.registration.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Enable Registration -->
 <div class="flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.enableRegistration")
 }}</label>
 <p class="text-sm text-muted ">
 {{
 t("admin.settings.registration.enableRegistrationHint")
 }}
 </p>
 </div>
 <Toggle v-model="form.registration_enabled" />
 </div>

 <!-- Email Verification -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.emailVerification")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.registration.emailVerificationHint") }}
 </p>
 </div>
 <Toggle v-model="form.email_verify_enabled" />
 </div>

 <!-- Email Suffix Whitelist -->
 <div class="border-t border-line pt-4 ">
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.emailSuffixWhitelist")
 }}</label>
 <p class="settings-card-desc">
 {{
 t("admin.settings.registration.emailSuffixWhitelistHint")
 }}
 </p>
 <div
 class="mt-3 rounded-lg border border-line bg-surface p-2 "
 >
 <div class="flex flex-wrap items-center gap-2">
 <span
 v-for="suffix in registrationEmailSuffixWhitelistTags"
 :key="suffix"
 class="inline-flex items-center gap-1 rounded bg-surface-2 px-2 py-1 text-xs font-mono text-foreground "
 >
 <span>{{ suffix }}</span>
 <button
 type="button"
 class="rounded-full text-muted hover:bg-surface-2 hover:text-foreground "
 @click="
 removeRegistrationEmailSuffixWhitelistTag(suffix)
 "
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
 class="flex min-w-[220px] flex-1 items-center gap-1 rounded border border-transparent px-2 py-1 focus-within:border-accent "
 >
 <input
 v-model="registrationEmailSuffixWhitelistDraft"
 type="text"
 class="w-full bg-transparent text-sm font-mono text-foreground outline-none placeholder:text-muted "
 :placeholder="
 t(
 'admin.settings.registration.emailSuffixWhitelistPlaceholder',
 )
 "
 @input="
 handleRegistrationEmailSuffixWhitelistDraftInput
 "
 @keydown="
 handleRegistrationEmailSuffixWhitelistDraftKeydown
 "
 @blur="commitRegistrationEmailSuffixWhitelistDraft"
 @paste="handleRegistrationEmailSuffixWhitelistPaste"
 />
 </div>
 </div>
 </div>
 <p class="mt-2 text-xs text-muted ">
 {{
 t(
 "admin.settings.registration.emailSuffixWhitelistInputHint",
 )
 }}
 </p>
 </div>

 <!-- Email Domain Quota -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.emailDomainQuota")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.registration.emailDomainQuotaHint") }}
 </p>
 </div>
 <Toggle
 v-model="form.registration_email_domain_quota_enabled"
 />
 </div>

 <!-- Promo Code -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.promoCode")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.registration.promoCodeHint") }}
 </p>
 </div>
 <Toggle v-model="form.promo_code_enabled" />
 </div>

 <!-- Invitation Code -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.invitationCode")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.registration.invitationCodeHint") }}
 </p>
 </div>
 <Toggle v-model="form.invitation_code_enabled" />
 </div>
 <!-- Password Reset - Only show when email verification is enabled -->
 <div
 v-if="form.email_verify_enabled"
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.passwordReset")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.registration.passwordResetHint") }}
 </p>
 </div>
 <Toggle v-model="form.password_reset_enabled" />
 </div>
 <!-- Frontend URL - Only show when password reset is enabled -->
 <div
 v-if="form.email_verify_enabled && form.password_reset_enabled"
 class="settings-row border-t border-line pt-4 "
 >
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.registration.frontendUrl") }}
 </label>
 <input
 v-model="form.frontend_url"
 type="url"
 class="input"
 :placeholder="
 t('admin.settings.registration.frontendUrlPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.registration.frontendUrlHint") }}
 </p>
 </div>

 <!-- TOTP 2FA -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.registration.totp")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.registration.totpHint") }}
 </p>
 <!-- Warning when encryption key not configured -->
 <p
 v-if="!form.totp_encryption_key_configured"
 class="mt-2 text-sm text-warning-text "
 >
 {{ t("admin.settings.registration.totpKeyNotConfigured") }}
 </p>
 </div>
 <Toggle
 v-model="form.totp_enabled"
 :disabled="!form.totp_encryption_key_configured"
 />
 </div>

 <!-- Passkey sign-in -->
 <div
 class="border-t border-line pt-4 "
 data-testid="passkey-settings"
 >
 <div class="flex items-start justify-between gap-4">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.security.passkey")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.security.passkeyHint") }}
 </p>
 </div>
 <Toggle
 v-model="form.passkey_enabled"
 data-testid="passkey-toggle"
 :disabled="!form.passkey_configured"
 />
 </div>
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
 : t(
 "admin.settings.security.passkeyValueNotConfigured",
 )
 }}
 </p>
 <p v-if="!form.passkey_configured" class="mt-2">
 {{ t("admin.settings.security.passkeyDeploymentHint") }}
 </p>
 </div>
 </div>

 <!-- 敏感操作 step-up 2FA -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.security.stepUp")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.security.stepUpHint") }}
 </p>
 </div>
 <Toggle v-model="form.step_up_enabled" />
 </div>

 <!-- 会话 IP/UA 绑定 -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.security.sessionBinding")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.security.sessionBindingHint") }}
 </p>
 </div>
 <Toggle v-model="form.session_binding_enabled" />
 </div>

 <!-- 审计日志保留天数 -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.security.auditRetention")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.security.auditRetentionHint") }}
 </p>
 </div>
 <input
 v-model.number="form.audit_log_retention_days"
 type="number"
 min="0"
 class="input w-28 text-right"
 />
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
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
