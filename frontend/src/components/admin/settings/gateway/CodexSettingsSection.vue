<template>
 <!-- Codex Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.gatewayForwarding.codexHardeningTitle") }}
 </h2>
 </div>
 <div class="settings-card-body">
 <div>
 <h3 class="text-base font-semibold text-foreground ">
 {{ t("admin.settings.gatewayForwarding.codexClientRestrictionTitle") }}
 </h3>
 <p class="settings-card-desc">
 {{ t("admin.settings.gatewayForwarding.codexHardeningDesc") }}
 </p>
 </div>
 <div class="grid gap-4 sm:grid-cols-2">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.gatewayForwarding.minCodexVersion") }}
 </label>
 <input
 v-model="form.min_codex_version"
 type="text"
 class="input w-full font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.minCodexVersionPlaceholder',
 )
 "
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.gatewayForwarding.maxCodexVersion") }}
 </label>
 <input
 v-model="form.max_codex_version"
 type="text"
 class="input w-full font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.maxCodexVersionPlaceholder',
 )
 "
 />
 </div>
 </div>
 <p class="text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexVersionHint") }}
 </p>

 <div>
 <label class="block text-sm font-medium text-foreground ">
 {{ t("admin.settings.gatewayForwarding.codexFingerprintSignals") }}
 </label>
 <p class="mb-2 mt-1 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexFingerprintSignalsDesc") }}
 </p>
 <div
 v-for="(row, i) in codexFingerprintRows"
 :key="`codex-fp-${i}`"
 class="mb-2 flex items-center gap-2"
 >
 <select v-model="row.type" class="input w-32 text-sm">
 <option value="header_exact">{{ t("admin.settings.gatewayForwarding.codexFpTypeHeaderExact") }}</option>
 <option value="header_prefix">{{ t("admin.settings.gatewayForwarding.codexFpTypeHeaderPrefix") }}</option>
 <option value="body_path">{{ t("admin.settings.gatewayForwarding.codexFpTypeBodyPath") }}</option>
 </select>
 <input
 v-model="row.match"
 type="text"
 class="input flex-1 font-mono text-sm"
 :placeholder="t('admin.settings.gatewayForwarding.codexFpMatchPlaceholder')"
 />
 <label class="flex shrink-0 items-center gap-1 text-xs text-muted ">
 <input v-model="row.required" type="checkbox" />
 {{ t("admin.settings.gatewayForwarding.codexFpRequired") }}
 </label>
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-danger-600 hover:text-danger-700 "
 @click="removeCodexFingerprintRow(i)"
 >
 {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
 </button>
 </div>
 <button type="button" class="btn-glass-secondary" @click="addCodexFingerprintRow">
 {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
 </button>
 <p
 v-if="codexFingerprintNoRequired"
 class="mt-2 text-xs text-warning-600 "
 >
 {{ t("admin.settings.gatewayForwarding.codexFingerprintNoRequiredWarn") }}
 </p>
 </div>

 <div class="flex items-center justify-between">
 <div class="pr-4">
 <label
 class="block text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.gatewayForwarding.codexAllowAppServer")
 }}
 </label>
 <p class="mt-1 text-xs text-muted ">
 {{
 t(
 "admin.settings.gatewayForwarding.codexAllowAppServerDesc",
 )
 }}
 </p>
 </div>
 <Toggle
 v-model="form.codex_cli_only_allow_app_server_clients"
 />
 </div>

 <div>
 <label
 class="block text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.codexBlacklist") }}
 </label>
 <p class="mb-2 mt-1 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexBlacklistDesc") }}
 </p>
 <div
 v-for="(row, i) in codexBlacklistRows"
 :key="`codex-bl-${i}`"
 class="mb-2 flex gap-2"
 >
 <input
 v-model="row.originator"
 type="text"
 class="input w-1/3 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexOriginatorPlaceholder',
 )
 "
 />
 <input
 v-model="row.uaContains"
 type="text"
 class="input flex-1 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexUaContainsPlaceholder',
 )
 "
 />
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-danger-600 hover:text-danger-700 "
 @click="removeCodexBlacklistRow(i)"
 >
 {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
 </button>
 </div>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addCodexBlacklistRow"
 >
 {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
 </button>
 </div>

 <div>
 <label
 class="block text-sm font-medium text-foreground "
 >
 {{ t("admin.settings.gatewayForwarding.codexWhitelist") }}
 </label>
 <p class="mb-2 mt-1 text-xs text-muted ">
 {{ t("admin.settings.gatewayForwarding.codexWhitelistDesc") }}
 </p>
 <div
 v-for="(row, i) in codexWhitelistRows"
 :key="`codex-wl-${i}`"
 class="mb-2 flex gap-2"
 >
 <input
 v-model="row.originator"
 type="text"
 class="input w-1/3 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexOriginatorPlaceholder',
 )
 "
 />
 <input
 v-model="row.uaContains"
 type="text"
 class="input flex-1 font-mono text-sm"
 :placeholder="
 t(
 'admin.settings.gatewayForwarding.codexUaContainsPlaceholder',
 )
 "
 />
 <label
 class="flex shrink-0 items-center gap-1 text-xs text-muted "
 :title="
 t(
 'admin.settings.gatewayForwarding.codexWhitelistSkipFingerprintTooltip',
 )
 "
 >
 <input
 v-model="row.skipEngineFingerprint"
 type="checkbox"
 />
 {{
 t(
 'admin.settings.gatewayForwarding.codexWhitelistSkipFingerprint',
 )
 }}
 </label>
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-danger-600 hover:text-danger-700 "
 @click="removeCodexWhitelistRow(i)"
 >
 {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
 </button>
 </div>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addCodexWhitelistRow"
 >
 {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
 </button>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject, computed } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, form, codexBlacklistRows, codexWhitelistRows, codexFingerprintRows } = settingsForm;

const codexFingerprintNoRequired = computed(
  () => !codexFingerprintRows.value.some((r) => r.required),
);

function addCodexFingerprintRow(): void {
  codexFingerprintRows.value.push({ type: "header_exact", match: "", required: false });
}

function removeCodexFingerprintRow(i: number): void {
  codexFingerprintRows.value.splice(i, 1);
}

function addCodexBlacklistRow(): void {
  codexBlacklistRows.value.push({ originator: "", uaContains: "" });
}

function removeCodexBlacklistRow(i: number): void {
  codexBlacklistRows.value.splice(i, 1);
}

function addCodexWhitelistRow(): void {
  codexWhitelistRows.value.push({
    originator: "",
    uaContains: "",
    skipEngineFingerprint: false,
  });
}

function removeCodexWhitelistRow(i: number): void {
  codexWhitelistRows.value.splice(i, 1);
}

</script>
