<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.captcha.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.captcha.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Enable Captcha -->
 <div class="settings-flex-row settings-control-row flex items-center justify-between">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.captcha.enable")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.captcha.enableHint") }}
 </p>
 </div>
 <Toggle
 v-model="captchaMasterEnabled"
 data-testid="captcha-enabled-toggle"
 />
 </div>

 <!-- Provider fields - Only show when enabled -->
 <div
 v-if="captchaMasterEnabled"
 class="border-t border-line pt-4 "
 >
 <!-- Provider Selector -->
 <div class="settings-row mb-6">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.captcha.provider") }}
 </label>
 <div
 class="grid grid-cols-3 gap-2 rounded-lg bg-surface-2 p-1 "
 >
 <button
 type="button"
 data-testid="captcha-provider-turnstile"
 class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
 :class="
 captchaProviderSelection === 'turnstile'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="selectCaptchaProvider('turnstile')"
 >
 {{ t("admin.settings.captcha.providerTurnstile") }}
 </button>
 <button
 type="button"
 data-testid="captcha-provider-tencent"
 class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
 :class="
 captchaProviderSelection === 'tencent'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="selectCaptchaProvider('tencent')"
 >
 {{ t("admin.settings.captcha.providerTencent") }}
 </button>
 <button
 type="button"
 data-testid="captcha-provider-aliyun"
 class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
 :class="
 captchaProviderSelection === 'aliyun'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="selectCaptchaProvider('aliyun')"
 >
 {{ t("admin.settings.captcha.providerAliyun") }}
 </button>
 </div>
 </div>

 <!-- Cloudflare Turnstile fields -->
 <div
 v-if="captchaProviderSelection === 'turnstile'"
 class="settings-row-group"
 >
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.turnstile.siteKey") }}
 </label>
 <input
 v-model="form.turnstile_site_key"
 type="text"
 class="input font-mono text-sm"
 placeholder="0x4AAAAAAA..."
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.turnstile.siteKeyHint") }}
 <a
 href="https://dash.cloudflare.com/"
 target="_blank"
 class="text-accent hover:text-accent"
 >{{
 t("admin.settings.turnstile.cloudflareDashboard")
 }}</a
 >
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.turnstile.secretKey") }}
 </label>
 <input
 v-model="form.turnstile_secret_key"
 type="password"
 class="input font-mono text-sm"
 placeholder="0x4AAAAAAA..."
 />
 <p class="settings-row-hint">
 {{
 form.turnstile_secret_key_configured
 ? t(
 "admin.settings.turnstile.secretKeyConfiguredHint",
 )
 : t("admin.settings.turnstile.secretKeyHint")
 }}
 </p>
 </div>
 </div>

 <!-- Tencent Captcha fields -->
 <div v-else-if="captchaProviderSelection === 'tencent'">
 <div class="settings-row mb-6 max-w-sm">
 <label class="settings-row-label">
 {{ t("admin.settings.tencentCaptcha.region") }}
 </label>
 <div class="grid grid-cols-2 gap-2 rounded-lg bg-surface-2 p-1 ">
 <button
 type="button"
 data-testid="tencent-captcha-region-cn"
 class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
 :class="
 form.tencent_captcha_region !== 'intl'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="form.tencent_captcha_region = 'cn'"
 >
 {{ t("admin.settings.tencentCaptcha.regionCn") }}
 </button>
 <button
 type="button"
 data-testid="tencent-captcha-region-intl"
 class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
 :class="
 form.tencent_captcha_region === 'intl'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="form.tencent_captcha_region = 'intl'"
 >
 {{ t("admin.settings.tencentCaptcha.regionIntl") }}
 </button>
 </div>
 <p class="settings-row-hint">
 {{ t("admin.settings.tencentCaptcha.regionHint") }}
 </p>
 </div>
 <div class="settings-row-group">
 <div class="md:col-span-2">
 <h3 class="text-sm font-semibold text-foreground ">
 {{ t("admin.settings.tencentCaptcha.appCredentialsTitle") }}
 </h3>
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.tencentCaptcha.appCredentialsHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label class="settings-row-label">
 {{ t("admin.settings.tencentCaptcha.appId") }}
 </label>
 <input
 v-model="form.tencent_captcha_app_id"
 type="text"
 inputmode="numeric"
 class="input font-mono text-sm"
 placeholder="123456789"
 />
 </div>
 <div class="settings-row">
 <label class="settings-row-label">
 {{ t("admin.settings.tencentCaptcha.appSecretKey") }}
 </label>
 <input
 v-model="form.tencent_captcha_app_secret_key"
 type="password"
 autocomplete="new-password"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.tencentCaptcha.keepExisting')"
 />
 <p class="settings-row-hint">
 {{ form.tencent_captcha_app_secret_key_configured ? t("admin.settings.tencentCaptcha.configured") : t("admin.settings.tencentCaptcha.required") }}
 </p>
 </div>
 <div class="border-t border-line pt-5 md:col-span-2 ">
 <h3 class="text-sm font-semibold text-foreground ">
 {{ t("admin.settings.tencentCaptcha.cloudCredentialsTitle") }}
 </h3>
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.tencentCaptcha.cloudCredentialsHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label class="settings-row-label">
 {{ t("admin.settings.tencentCaptcha.cloudSecretId") }}
 </label>
 <input
 v-model="form.tencent_captcha_cloud_secret_id"
 type="password"
 autocomplete="new-password"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.tencentCaptcha.keepExisting')"
 />
 <p class="settings-row-hint">
 {{ form.tencent_captcha_cloud_secret_id_configured ? t("admin.settings.tencentCaptcha.configured") : t("admin.settings.tencentCaptcha.required") }}
 </p>
 </div>
 <div class="settings-row">
 <label class="settings-row-label">
 {{ t("admin.settings.tencentCaptcha.cloudSecretKey") }}
 </label>
 <input
 v-model="form.tencent_captcha_cloud_secret_key"
 type="password"
 autocomplete="new-password"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.tencentCaptcha.keepExisting')"
 />
 <p class="settings-row-hint">
 {{ form.tencent_captcha_cloud_secret_key_configured ? t("admin.settings.tencentCaptcha.configured") : t("admin.settings.tencentCaptcha.required") }}
 </p>
 </div>
 </div>
 <p class="mt-5 text-xs text-muted ">
 {{ t("admin.settings.tencentCaptcha.camPermissionHint") }}
 </p>
 <p class="mt-2 text-xs text-muted ">
 {{ t("admin.settings.tencentCaptcha.aidEncryptedHint") }}
 </p>
 <div class="settings-flex-row mt-3 flex flex-wrap gap-x-4 gap-y-2 text-sm">
 <a
 :href="tencentCaptchaLinks.console"
 target="_blank"
 rel="noopener noreferrer"
 class="text-accent hover:text-accent"
 >
 {{ t("admin.settings.tencentCaptcha.openCaptchaConsole") }}
 </a>
 <a
 :href="tencentCaptchaLinks.cloudKeys"
 target="_blank"
 rel="noopener noreferrer"
 class="text-accent hover:text-accent"
 >
 {{ t("admin.settings.tencentCaptcha.createCloudKeys") }}
 </a>
 <a
 :href="tencentCaptchaLinks.webDocs"
 target="_blank"
 rel="noopener noreferrer"
 class="text-accent hover:text-accent"
 >
 {{ t("admin.settings.tencentCaptcha.openWebDocs") }}
 </a>
 </div>
 </div>

 <!-- Aliyun Captcha 2.0 fields -->
 <div v-else class="settings-row-group">
 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.aliyunCaptcha.region") }}
 </label>
 <div
 class="grid grid-cols-2 gap-2 rounded-lg bg-surface-2 p-1 "
 >
 <button
 type="button"
 class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
 :class="
 form.aliyun_captcha_region !== 'sgp'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="form.aliyun_captcha_region = 'cn'"
 >
 {{ t("admin.settings.aliyunCaptcha.regionCn") }}
 </button>
 <button
 type="button"
 class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
 :class="
 form.aliyun_captcha_region === 'sgp'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="form.aliyun_captcha_region = 'sgp'"
 >
 {{ t("admin.settings.aliyunCaptcha.regionSgp") }}
 </button>
 </div>
 <p class="settings-row-hint">
 {{ t("admin.settings.aliyunCaptcha.regionHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.aliyunCaptcha.prefix") }}
 </label>
 <input
 v-model="form.aliyun_captcha_prefix"
 type="text"
 class="input font-mono text-sm"
 placeholder="14xxxxx"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.aliyunCaptcha.prefixHint") }}
 </p>
 </div>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.aliyunCaptcha.sceneId") }}
 </label>
 <input
 v-model="form.aliyun_captcha_scene_id"
 type="text"
 class="input font-mono text-sm"
 placeholder="1cxxxxxx"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.aliyunCaptcha.sceneIdHint") }}
 </p>
 <a
 href="https://yundun.console.aliyun.com/?p=captcha"
 target="_blank"
 rel="noopener noreferrer"
 class="mt-2 inline-block text-sm text-accent hover:text-accent"
 >
 {{ t("admin.settings.aliyunCaptcha.openCaptchaConsole") }}
 </a>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.aliyunCaptcha.accessKeyId") }}
 </label>
 <input
 v-model="form.aliyun_captcha_access_key_id"
 type="text"
 class="input font-mono text-sm"
 placeholder="LTAI..."
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.aliyunCaptcha.accessKeyIdHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.aliyunCaptcha.accessKeySecret") }}
 </label>
 <input
 v-model="form.aliyun_captcha_access_key_secret"
 type="password"
 autocomplete="new-password"
 class="input font-mono text-sm"
 placeholder="••••••••"
 />
 <p class="settings-row-hint">
 {{
 form.aliyun_captcha_access_key_secret_configured
 ? t(
 "admin.settings.aliyunCaptcha.accessKeySecretConfiguredHint",
 )
 : t("admin.settings.aliyunCaptcha.accessKeySecretHint")
 }}
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
import type { CaptchaProviderSelection } from "../useSettingsForm";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, form, captchaProviderSelection } = settingsForm;

function applyCaptchaSelection(provider: CaptchaProviderSelection | null): void {
  form.turnstile_enabled = provider === "turnstile";
  form.tencent_captcha_enabled = provider === "tencent";
  form.aliyun_captcha_enabled = provider === "aliyun";
}

const captchaMasterEnabled = computed({
  get: () =>
    form.turnstile_enabled ||
    form.tencent_captcha_enabled ||
    form.aliyun_captcha_enabled,
  set: (enabled: boolean) =>
    applyCaptchaSelection(enabled ? captchaProviderSelection.value : null),
});

function selectCaptchaProvider(provider: CaptchaProviderSelection): void {
  captchaProviderSelection.value = provider;
  applyCaptchaSelection(provider);
}

const tencentCaptchaLinks = computed(() =>
  form.tencent_captcha_region === "intl"
    ? {
        console: "https://console.tencentcloud.com/captcha/graphical",
        cloudKeys: "https://console.tencentcloud.com/cam/capi",
        webDocs: "https://www.tencentcloud.com/document/product/1159/49680",
      }
    : {
        console: "https://console.cloud.tencent.com/captcha",
        cloudKeys: "https://console.cloud.tencent.com/cam/capi",
        webDocs: "https://cloud.tencent.com/document/product/1110/36841",
      },
);
</script>
