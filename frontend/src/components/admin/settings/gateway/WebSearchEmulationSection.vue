<template>
 <!-- Web Search Emulation -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.webSearchEmulation.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.webSearchEmulation.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Global Toggle -->
 <div class="flex items-center justify-between">
 <div>
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.webSearchEmulation.enabled") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.webSearchEmulation.enabledHint") }}
 </p>
 </div>
 <Toggle v-model="webSearchConfig.enabled" />
 </div>

 <!-- Providers -->
 <div v-if="webSearchConfig.enabled" class="space-y-4">
 <div class="flex items-center justify-between">
 <label
 class="text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.webSearchEmulation.providers") }}
 </label>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addWebSearchProvider"
 >
 {{ t("admin.settings.webSearchEmulation.addProvider") }}
 </button>
 </div>

 <div
 v-if="webSearchConfig.providers.length === 0"
 class="rounded-lg border border-dashed border-line p-4 text-center text-sm text-muted "
 >
 {{ t("admin.settings.webSearchEmulation.noProviders") }}
 </div>

 <div
 v-for="(provider, pIdx) in webSearchConfig.providers"
 :key="pIdx"
 class="rounded-lg border border-line "
 >
 <!-- Collapsible header -->
 <div
 class="flex cursor-pointer items-center justify-between px-4 py-3"
 @click="toggleProviderExpand(pIdx)"
 >
 <div class="flex items-center gap-3">
 <svg
 class="h-4 w-4 text-muted transition-transform"
 :class="{ 'rotate-90': expandedProviders[pIdx] }"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M9 5l7 7-7 7"
 />
 </svg>
 <Select
 v-model="provider.type"
 :options="[
 { value: 'brave', label: 'Brave Search' },
 { value: 'tavily', label: 'Tavily' },
 ]"
 class="w-36"
 @click.stop
 />
 <!-- Quota summary (always visible) -->
 <span class="text-xs text-muted">
 {{ provider.quota_used ?? 0 }} /
 {{
 provider.quota_limit != null &&
 provider.quota_limit > 0
 ? provider.quota_limit
 : "∞"
 }}
 </span>
 <span
 v-if="
 !expandedProviders[pIdx] &&
 provider.api_key_configured
 "
 class="text-xs text-green-500"
 >
 {{
 t(
 "admin.settings.webSearchEmulation.apiKeyConfigured",
 )
 }}
 </span>
 </div>
 <button
 type="button"
 class="text-red-500 hover:text-red-700 text-xs"
 @click.stop="removeWebSearchProvider(pIdx)"
 >
 {{
 t("admin.settings.webSearchEmulation.removeProvider")
 }}
 </button>
 </div>

 <!-- Expanded content -->
 <div
 v-if="expandedProviders[pIdx]"
 class="space-y-3 border-t border-line px-4 pb-4 pt-3 "
 >
 <!-- API Key with inline show/copy -->
 <div>
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.apiKey")
 }}</label>
 <div class="relative">
 <input
 v-model="provider.api_key"
 :type="apiKeyVisible[pIdx] ? 'text' : 'password'"
 class="input w-full text-sm"
 :class="
 provider.api_key || provider.api_key_configured
 ? 'pr-16'
 : ''
 "
 :placeholder="
 provider.api_key_configured
 ? '••••••••'
 : t(
 'admin.settings.webSearchEmulation.apiKeyPlaceholder',
 )
 "
 />
 <div
 v-if="provider.api_key || provider.api_key_configured"
 class="absolute inset-y-0 right-0 flex items-center pr-1.5"
 >
 <button
 type="button"
 class="rounded p-1 text-muted hover:text-foreground "
 :title="
 apiKeyVisible[pIdx]
 ? t(
 'admin.settings.webSearchEmulation.hideApiKey',
 )
 : t(
 'admin.settings.webSearchEmulation.showApiKey',
 )
 "
 @click="apiKeyVisible[pIdx] = !apiKeyVisible[pIdx]"
 >
 <svg
 v-if="!apiKeyVisible[pIdx]"
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
 />
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
 />
 </svg>
 <svg
 v-else
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"
 />
 </svg>
 </button>
 <button
 type="button"
 class="rounded p-1 text-muted hover:text-foreground "
 :class="{
 'opacity-30 cursor-not-allowed':
 !provider.api_key,
 }"
 :title="
 t('admin.settings.webSearchEmulation.copyApiKey')
 "
 :disabled="!provider.api_key"
 @click="copyApiKey(pIdx)"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 stroke-width="2"
 d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
 />
 </svg>
 </button>
 </div>
 </div>
 </div>

 <!-- Quota + Subscription in compact row -->
 <div class="grid grid-cols-2 gap-3">
 <div>
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.quotaLimit")
 }}</label>
 <input
 v-model="provider.quota_limit"
 type="number"
 min="1"
 class="input text-sm"
 :placeholder="'∞'"
 />
 <p class="mt-0.5 text-xs text-muted">
 {{
 t(
 "admin.settings.webSearchEmulation.quotaLimitHint",
 )
 }}
 </p>
 </div>
 <div>
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.subscribedAt")
 }}</label>
 <input
 :value="formatSubscribedAt(provider.subscribed_at)"
 type="date"
 class="input text-sm"
 @input="
 provider.subscribed_at = parseSubscribedAt(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="mt-0.5 text-xs text-muted">
 {{
 t(
 "admin.settings.webSearchEmulation.subscribedAtHint",
 )
 }}
 </p>
 </div>
 </div>

 <!-- Usage display -->
 <div class="flex items-center gap-2">
 <span class="text-xs text-muted"
 >{{
 t("admin.settings.webSearchEmulation.quotaUsage")
 }}:</span
 >
 <div
 v-if="
 provider.quota_limit != null &&
 provider.quota_limit > 0
 "
 class="flex-1 rounded-full bg-surface-3 "
 style="height: 6px"
 >
 <div
 class="h-full rounded-full transition-all"
 :class="
 quotaPercentage(provider) > 90
 ? 'bg-red-500'
 : quotaPercentage(provider) > 70
 ? 'bg-yellow-500'
 : 'bg-green-500'
 "
 :style="{
 width:
 Math.min(quotaPercentage(provider), 100) + '%',
 }"
 />
 </div>
 <div v-else class="flex-1" />
 <span class="text-xs text-muted"
 >{{ provider.quota_used ?? 0 }} /
 {{
 provider.quota_limit != null &&
 provider.quota_limit > 0
 ? provider.quota_limit
 : "∞"
 }}</span
 >
 <button
 v-if="(provider.quota_used ?? 0) > 0"
 type="button"
 class="text-xs text-accent hover:text-accent"
 @click="resetWebSearchUsage(pIdx)"
 >
 {{ t("admin.settings.webSearchEmulation.resetUsage") }}
 </button>
 </div>

 <!-- Proxy + Test on same row -->
 <div class="flex items-end gap-3">
 <div class="flex-1">
 <label class="text-xs text-muted">{{
 t("admin.settings.webSearchEmulation.proxy")
 }}</label>
 <ProxySelector
 v-model="provider.proxy_id"
 :proxies="webSearchProxies"
 />
 </div>
 <button
 type="button"
 class="btn-glass-secondary whitespace-nowrap"
 @click="openTestDialog()"
 >
 {{ t("admin.settings.webSearchEmulation.test") }}
 </button>
 </div>
 </div>
 </div>
 </div>
 </div>
 </div>

 <!-- Web Search Test Dialog -->
 <div
 v-if="wsTestDialogOpen"
 class="fixed inset-0 z-50 flex items-center justify-center glass-modal-scrim"
 @click.self="wsTestDialogOpen = false"
 >
 <div
 class="mx-4 w-full max-w-lg glass-card-solid rounded-hero p-6"
 >
 <h3
 class="mb-4 text-base font-semibold text-foreground "
 >
 {{ t("admin.settings.webSearchEmulation.testResultTitle") }}
 </h3>
 <div class="flex items-center gap-2">
 <input
 v-model="wsTestQuery"
 type="text"
 class="input flex-1 text-sm"
 :placeholder="
 t('admin.settings.webSearchEmulation.testDefaultQuery')
 "
 @keyup.enter="testWebSearchProvider()"
 />
 <button
 type="button"
 class="btn-glass-primary"
 :disabled="wsTestLoading"
 @click="testWebSearchProvider()"
 >
 {{
 wsTestLoading
 ? t("admin.settings.webSearchEmulation.testing")
 : t("admin.settings.webSearchEmulation.test")
 }}
 </button>
 </div>
 <!-- Test results -->
 <div
 v-if="wsTestResult"
 class="mt-4 max-h-80 overflow-y-auto rounded-lg bg-surface-2 p-4 "
 >
 <p
 class="mb-2 text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.webSearchEmulation.testResultProvider")
 }}: {{ wsTestResult.provider }}
 </p>
 <div
 v-if="wsTestResult.results.length === 0"
 class="text-sm text-muted"
 >
 {{ t("admin.settings.webSearchEmulation.testNoResults") }}
 </div>
 <div
 v-for="(r, rIdx) in wsTestResult.results"
 :key="rIdx"
 class="mt-2 border-t border-line pt-2 first:mt-0 first:border-0 first:pt-0 "
 >
 <a
 :href="r.url"
 target="_blank"
 class="text-sm font-medium text-blue-600 hover:underline "
 >{{ r.title }}</a
 >
 <p class="mt-0.5 text-xs text-muted ">
 {{ r.snippet }}
 </p>
 </div>
 </div>
 <div class="mt-4 flex justify-end">
 <button
 type="button"
 class="btn-glass-secondary"
 @click="wsTestDialogOpen = false"
 >
 {{ t("common.close") }}
 </button>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject, ref, reactive } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import type { WebSearchProviderConfig, WebSearchTestResult } from "@/api/admin/settings";
import { adminAPI } from "@/api";
import { extractApiErrorMessage } from "@/utils/apiError";
import Select from "@/components/common/Select.vue";
import Toggle from "@/components/common/Toggle.vue";
import ProxySelector from "@/components/common/ProxySelector.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, appStore, webSearchProxies, webSearchConfig } = settingsForm;

const DEFAULT_WEB_SEARCH_QUOTA_LIMIT = 1000;

const expandedProviders = reactive<Record<number, boolean>>({});

const apiKeyVisible = reactive<Record<number, boolean>>({});

const wsTestQuery = ref("");

const wsTestLoading = ref(false);

const wsTestResult = ref<WebSearchTestResult | null>(null);

const wsTestDialogOpen = ref(false);

function openTestDialog() {
  wsTestResult.value = null;
  wsTestDialogOpen.value = true;
}

function toggleProviderExpand(idx: number) {
  expandedProviders[idx] = !expandedProviders[idx];
}

function removeWebSearchProvider(idx: number) {
  webSearchConfig.providers.splice(idx, 1);
  // Re-index expandedProviders and apiKeyVisible after removal
  const newExpanded: Record<number, boolean> = {};
  const newVisible: Record<number, boolean> = {};
  for (let i = 0; i < webSearchConfig.providers.length; i++) {
    const oldIdx = i >= idx ? i + 1 : i;
    newExpanded[i] = expandedProviders[oldIdx] ?? false;
    newVisible[i] = apiKeyVisible[oldIdx] ?? false;
  }
  Object.keys(expandedProviders).forEach(
    (k) => delete expandedProviders[Number(k)],
  );
  Object.keys(apiKeyVisible).forEach((k) => delete apiKeyVisible[Number(k)]);
  Object.assign(expandedProviders, newExpanded);
  Object.assign(apiKeyVisible, newVisible);
}

function addWebSearchProvider() {
  const idx = webSearchConfig.providers.length;
  webSearchConfig.providers.push({
    type: "brave",
    api_key: "",
    api_key_configured: false,
    quota_limit: DEFAULT_WEB_SEARCH_QUOTA_LIMIT,
    subscribed_at: null,
    proxy_id: null,
    expires_at: null,
  } as WebSearchProviderConfig);
  expandedProviders[idx] = true;
}

function formatSubscribedAt(ts: number | null): string {
  if (!ts) return "";
  // Use UTC to avoid timezone drift on repeated edits
  const d = new Date(ts * 1000);
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function parseSubscribedAt(dateStr: string): number | null {
  if (!dateStr) return null;
  // Parse as UTC to match formatSubscribedAt
  return Math.floor(new Date(dateStr + "T00:00:00Z").getTime() / 1000);
}

function quotaPercentage(provider: WebSearchProviderConfig): number {
  if (!provider.quota_limit || provider.quota_limit <= 0) return 0;
  return ((provider.quota_used ?? 0) / provider.quota_limit) * 100;
}

async function resetWebSearchUsage(idx: number) {
  const provider = webSearchConfig.providers[idx];
  if (!provider) return;
  if (!confirm(t("admin.settings.webSearchEmulation.resetUsageConfirm")))
    return;
  try {
    await adminAPI.settings.resetWebSearchUsage({
      provider_type: provider.type,
    });
    provider.quota_used = 0;
    appStore.showSuccess(
      t("admin.settings.webSearchEmulation.resetUsageSuccess"),
    );
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  }
}

async function copyApiKey(idx: number) {
  const key = webSearchConfig.providers[idx]?.api_key;
  if (!key) {
    appStore.showError(
      t("admin.settings.webSearchEmulation.apiKeyPlaceholder"),
    );
    return;
  }
  try {
    await navigator.clipboard.writeText(key);
    appStore.showSuccess(t("admin.settings.webSearchEmulation.copied"));
  } catch {
    appStore.showError(t("common.error"));
  }
}

async function testWebSearchProvider() {
  wsTestLoading.value = true;
  wsTestResult.value = null;
  try {
    const query =
      wsTestQuery.value.trim() ||
      t("admin.settings.webSearchEmulation.testDefaultQuery");
    wsTestResult.value = await adminAPI.settings.testWebSearchEmulation(query);
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    wsTestLoading.value = false;
  }
}

</script>
