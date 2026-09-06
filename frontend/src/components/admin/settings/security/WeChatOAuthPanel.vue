<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.wechatConnect.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.wechatConnect.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-flex-row settings-control-row flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.wechatConnect.enabledLabel")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.wechatConnect.enabledHint") }}
 </p>
 </div>
 <Toggle
 v-model="form.wechat_connect_enabled"
 data-testid="wechat-connect-enabled"
 />
 </div>

 <div
 v-if="form.wechat_connect_enabled"
 class="space-y-6 border-t border-line pt-4 "
 >
 <div class="space-y-4">
 <div
 class="rounded-lg border border-line p-4 "
 >
 <div class="settings-flex-row settings-control-row-start flex items-start justify-between gap-4">
 <div>
 <h3 class="font-medium text-foreground ">
 {{ localText("PC 应用", "PC App") }}
 </h3>
 <p class="settings-card-desc">
 {{
 localText(
 "桌面浏览器通过微信开放平台扫码登录。可与公众号或移动应用同时存在。",
 "Desktop browsers sign in through WeChat Open Platform QR login. This can coexist with Official Account or Mobile App.",
 )
 }}
 </p>
 </div>
 <Toggle
 :model-value="form.wechat_connect_open_enabled"
 data-testid="wechat-connect-open-enabled"
 @update:model-value="handleWeChatOpenEnabledChange"
 />
 </div>
 <div
 v-if="form.wechat_connect_open_enabled"
 class="settings-row-group"
 >
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ localText("PC AppID", "PC App ID") }}
 </label>
 <input
 v-model="form.wechat_connect_open_app_id"
 data-testid="wechat-connect-open-app-id"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 localText(
 '微信开放平台 PC 应用 AppID',
 'WeChat Open Platform PC App ID',
 )
 "
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ localText("PC AppSecret", "PC App Secret") }}
 </label>
 <input
 v-model="form.wechat_connect_open_app_secret"
 data-testid="wechat-connect-open-app-secret"
 type="password"
 class="input font-mono text-sm"
 :placeholder="
 form.wechat_connect_open_app_secret_configured
 ? localText(
 '密钥已配置，留空以保留当前值。',
 'Secret configured. Leave empty to keep the current value.',
 )
 : localText(
 '微信开放平台 PC 应用 AppSecret',
 'WeChat Open Platform PC App Secret',
 )
 "
 />
 </div>
 </div>
 </div>

 <div
 class="rounded-lg border border-line p-4 "
 >
 <div class="settings-flex-row settings-control-row-start flex items-start justify-between gap-4">
 <div>
 <h3 class="font-medium text-foreground ">
 {{ localText("公众号", "Official Account") }}
 </h3>
 <p class="settings-card-desc">
 {{
 localText(
 "仅在微信内浏览器可用；非微信环境下会显示不可用。",
 "Only available inside the WeChat browser. It is shown as unavailable outside WeChat.",
 )
 }}
 </p>
 </div>
 <Toggle
 :model-value="form.wechat_connect_mp_enabled"
 data-testid="wechat-connect-mp-enabled"
 @update:model-value="handleWeChatMPEnabledChange"
 />
 </div>
 <div
 v-if="form.wechat_connect_mp_enabled"
 class="settings-row-group"
 >
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ localText("公众号 AppID", "Official Account App ID") }}
 </label>
 <input
 v-model="form.wechat_connect_mp_app_id"
 data-testid="wechat-connect-mp-app-id"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 localText(
 '公众号 AppID',
 'Official Account App ID',
 )
 "
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 localText(
 "公众号 AppSecret",
 "Official Account App Secret",
 )
 }}
 </label>
 <input
 v-model="form.wechat_connect_mp_app_secret"
 data-testid="wechat-connect-mp-app-secret"
 type="password"
 class="input font-mono text-sm"
 :placeholder="
 form.wechat_connect_mp_app_secret_configured
 ? localText(
 '密钥已配置，留空以保留当前值。',
 'Secret configured. Leave empty to keep the current value.',
 )
 : localText(
 '公众号 AppSecret',
 'Official Account App Secret',
 )
 "
 />
 </div>
 </div>
 </div>

 <div
 class="rounded-lg border border-line p-4 "
 >
 <div class="settings-flex-row settings-control-row-start flex items-start justify-between gap-4">
 <div>
 <h3 class="font-medium text-foreground ">
 {{ localText("移动应用", "Mobile App") }}
 </h3>
 <p class="settings-card-desc">
 {{
 localText(
 "原生移动端通过微信 SDK 唤起授权，网页端不会直接发起该流程。",
 "Native mobile clients start authorization through the WeChat SDK. The web UI does not launch this flow directly.",
 )
 }}
 </p>
 </div>
 <Toggle
 :model-value="form.wechat_connect_mobile_enabled"
 data-testid="wechat-connect-mobile-enabled"
 @update:model-value="handleWeChatMobileEnabledChange"
 />
 </div>
 <div
 v-if="form.wechat_connect_mobile_enabled"
 class="settings-row-group"
 >
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ localText("移动应用 AppID", "Mobile App ID") }}
 </label>
 <input
 v-model="form.wechat_connect_mobile_app_id"
 data-testid="wechat-connect-mobile-app-id"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 localText(
 '移动应用 AppID',
 'Mobile App ID',
 )
 "
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ localText("移动应用 AppSecret", "Mobile App Secret") }}
 </label>
 <input
 v-model="form.wechat_connect_mobile_app_secret"
 data-testid="wechat-connect-mobile-app-secret"
 type="password"
 class="input font-mono text-sm"
 :placeholder="
 form.wechat_connect_mobile_app_secret_configured
 ? localText(
 '密钥已配置，留空以保留当前值。',
 'Secret configured. Leave empty to keep the current value.',
 )
 : localText(
 '移动应用 AppSecret',
 'Mobile App Secret',
 )
 "
 />
 </div>
 </div>
 </div>
 </div>

 <div
 v-if="
 form.wechat_connect_open_enabled &&
 (form.wechat_connect_mp_enabled ||
 form.wechat_connect_mobile_enabled)
 "
 class="rounded-lg border border-warning bg-[color-mix(in_oklch,var(--warning)_10%,transparent)] px-4 py-3 text-sm text-warning-text "
 >
 {{
 localText(
 "如果同时启用 PC 应用和公众号/移动应用，这些应用需要挂在同一个微信开放平台主体下，否则 UnionID 无法稳定归并账号。",
 "When PC App is enabled together with Official Account or Mobile App, they should belong to the same WeChat Open Platform account so UnionID can merge identities reliably.",
 )
 }}
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 localText(
 "浏览器回调地址",
 "Browser Redirect URL",
 )
 }}
 </label>
 <input
 data-testid="wechat-connect-redirect-url"
 v-model="form.wechat_connect_redirect_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.wechatConnect.redirectUrlPlaceholder')"
 />
 <p class="settings-row-hint">
 {{
 localText(
 "用于 PC 应用和公众号的网页回调。移动应用走原生 SDK 时不直接使用这个浏览器回调。",
 "Used by PC App and Official Account browser callbacks. Native mobile SDK flows do not start from this browser callback directly.",
 )
 }}
 </p>
 <div
 class="settings-flex-row mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
 >
 <button
 type="button"
 class="btn-glass-secondary w-fit"
 @click="setAndCopyWeChatRedirectUrl"
 >
 {{ t("admin.settings.wechatConnect.generateAndCopy") }}
 </button>
 <code
 v-if="wechatRedirectUrlSuggestion"
 class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted "
 >
 {{ wechatRedirectUrlSuggestion }}
 </code>
 </div>
 </div>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.wechatConnect.frontendRedirectUrlLabel") }}
 </label>
 <input
 data-testid="wechat-connect-frontend-redirect-url"
 v-model="form.wechat_connect_frontend_redirect_url"
 type="text"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.wechatConnect.frontendRedirectUrlPlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.wechatConnect.frontendRedirectUrlHint") }}
 </p>
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
const { t, form, localText, currentOrigin, syncWeChatConnectMode } = settingsForm;

const { copyToClipboard } = useClipboard();

const wechatRedirectUrlSuggestion = computed(() => {
  return buildApiCallbackUrl(
    form.api_base_url,
    currentOrigin,
    "/auth/oauth/wechat/callback",
  );
});

function handleWeChatOpenEnabledChange(value: boolean) {
  form.wechat_connect_open_enabled = value;
  syncWeChatConnectMode(value ? "open" : undefined);
}

function handleWeChatMPEnabledChange(value: boolean) {
  form.wechat_connect_mp_enabled = value;
  if (value) {
    form.wechat_connect_mobile_enabled = false;
  }
  syncWeChatConnectMode(value ? "mp" : undefined);
}

function handleWeChatMobileEnabledChange(value: boolean) {
  form.wechat_connect_mobile_enabled = value;
  if (value) {
    form.wechat_connect_mp_enabled = false;
  }
  syncWeChatConnectMode(value ? "mobile" : undefined);
}

async function setAndCopyWeChatRedirectUrl() {
  const url = wechatRedirectUrlSuggestion.value;
  if (!url) return;

  form.wechat_connect_redirect_url = url;
  await copyToClipboard(
    url,
    t("admin.settings.wechatConnect.redirectUrlSetAndCopied"),
  );
}
</script>
