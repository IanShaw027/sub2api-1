<template>
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.apiKeyAcl.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.apiKeyAcl.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-flex-row settings-control-row flex items-center justify-between gap-4">
 <div>
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.apiKeyAcl.trustForwardedIp") }}
 </label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.apiKeyAcl.trustForwardedIpHint") }}
 </p>
 </div>
 <Toggle v-model="form.api_key_acl_trust_forwarded_ip" />
 </div>

 <div
 v-if="form.api_key_acl_trust_forwarded_ip"
 class="border-t border-line pt-4 "
 >
 <label
 for="forwarded-client-ip-headers"
 class="font-medium text-foreground "
 >
 {{ t("admin.settings.apiKeyAcl.forwardedClientIpHeaders") }}
 </label>
 <p class="settings-card-desc">
 {{ t("admin.settings.apiKeyAcl.forwardedClientIpHeadersHint") }}
 </p>
 <div
 class="mt-3 rounded-lg border border-line bg-surface p-2 "
 >
 <div class="settings-flex-row flex flex-wrap items-center gap-2">
 <span
 v-for="header in form.forwarded_client_ip_headers"
 :key="header"
 data-testid="forwarded-client-ip-header-tag"
 class="inline-flex items-center gap-1 rounded bg-surface-2 px-2 py-1 text-xs font-mono text-foreground "
 >
 <span>{{ header }}</span>
 <button
 type="button"
 class="rounded-full text-muted hover:bg-surface-2 hover:text-foreground "
 :aria-label="t('admin.settings.apiKeyAcl.removeForwardedClientIpHeader', { header })"
 @click="removeForwardedClientIpHeader(header)"
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
 class="settings-flex-row flex min-w-[220px] flex-1 items-center gap-1 rounded border border-transparent px-2 py-1 focus-within:border-accent "
 >
 <input
 id="forwarded-client-ip-headers"
 v-model="forwardedClientIpHeaderDraft"
 data-testid="forwarded-client-ip-headers-input"
 type="text"
 class="w-full bg-transparent text-sm font-mono text-foreground outline-none placeholder:text-muted "
 :placeholder="t('admin.settings.apiKeyAcl.forwardedClientIpHeadersPlaceholder')"
 @keydown="handleForwardedClientIpHeaderKeydown"
 @blur="commitForwardedClientIpHeaderDraft"
 @paste="handleForwardedClientIpHeaderPaste"
 />
 </div>
 </div>
 </div>
 <p class="mt-2 text-xs text-muted ">
 {{ t("admin.settings.apiKeyAcl.forwardedClientIpHeadersRiskHint") }}
 </p>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Icon from "@/components/icons/Icon.vue";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  appStore,
  form,
  forwardedClientIpHeaderDraft,
  maxForwardedClientIpHeaders,
  normalizeForwardedClientIpHeader,
} = settingsForm;

type ForwardedClientIpHeaderResult = "added" | "duplicate" | "invalid" | "full";

const forwardedClientIpHeaderSeparatorKeys = new Set([
  " ",
  ",",
  "，",
  "Enter",
  "Tab",
]);

function removeForwardedClientIpHeader(header: string) {
  form.forwarded_client_ip_headers = form.forwarded_client_ip_headers.filter(
    (item) => item !== header,
  );
}

function addForwardedClientIpHeader(raw: string): ForwardedClientIpHeaderResult {
  const header = normalizeForwardedClientIpHeader(raw);
  if (!header) {
    return "invalid";
  }
  if (
    form.forwarded_client_ip_headers.some(
      (item) => item.toLowerCase() === header.toLowerCase(),
    )
  ) {
    return "duplicate";
  }
  if (form.forwarded_client_ip_headers.length >= maxForwardedClientIpHeaders) {
    return "full";
  }
  form.forwarded_client_ip_headers = [
    ...form.forwarded_client_ip_headers,
    header,
  ];
  return "added";
}

function showForwardedClientIpHeaderError(result: ForwardedClientIpHeaderResult) {
  if (result === "invalid") {
    appStore.showError(t("admin.settings.apiKeyAcl.forwardedClientIpHeaderInvalid"));
  } else if (result === "full") {
    appStore.showError(
      t("admin.settings.apiKeyAcl.forwardedClientIpHeadersLimit", {
        max: maxForwardedClientIpHeaders,
      }),
    );
  }
}

function commitForwardedClientIpHeaderDraft() {
  const draft = forwardedClientIpHeaderDraft.value;
  if (!draft) {
    return;
  }
  const result = addForwardedClientIpHeader(draft);
  showForwardedClientIpHeaderError(result);
  forwardedClientIpHeaderDraft.value = "";
}

function handleForwardedClientIpHeaderKeydown(event: KeyboardEvent) {
  if (event.isComposing) {
    return;
  }
  if (forwardedClientIpHeaderSeparatorKeys.has(event.key)) {
    event.preventDefault();
    commitForwardedClientIpHeaderDraft();
    return;
  }
  if (
    event.key === "Backspace" &&
    !forwardedClientIpHeaderDraft.value &&
    form.forwarded_client_ip_headers.length > 0
  ) {
    form.forwarded_client_ip_headers.pop();
  }
}

function handleForwardedClientIpHeaderPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData("text") || "";
  if (!text.trim()) {
    return;
  }
  event.preventDefault();

  let error: ForwardedClientIpHeaderResult | undefined;
  for (const token of text.split(/[,，;\r\n]+/)) {
    if (!token.trim()) {
      continue;
    }
    const result = addForwardedClientIpHeader(token);
    if (result === "invalid" || result === "full") {
      error = result;
    }
  }
  if (error) {
    showForwardedClientIpHeaderError(error);
  }
}
</script>
