<template>
 <!-- Kiro Runtime Defaults -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.kiroRuntime.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.kiroRuntime.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.kiroVersion") }}
 </label>
 <input
 v-model="form.kiro_version"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-version"
 :placeholder="t('admin.settings.kiroRuntime.kiroVersionPlaceholder')"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.kiroCommit") }}
 </label>
 <input
 v-model="form.kiro_commit"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-commit"
 :placeholder="t('admin.settings.kiroRuntime.kiroCommitPlaceholder')"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.systemVersion") }}
 </label>
 <input
 v-model="form.system_version"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-system-version"
 :placeholder="t('admin.settings.kiroRuntime.systemVersionPlaceholder')"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.nodeVersion") }}
 </label>
 <input
 v-model="form.node_version"
 type="text"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-node-version"
 :placeholder="t('admin.settings.kiroRuntime.nodeVersionPlaceholder')"
 />
 </div>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.kiroRuntime.codeExecutionSandboxCommand",
 )
 }}
 </label>
 <textarea
 v-model="form.kiro_code_execution_sandbox_command"
 rows="3"
 class="input font-mono text-sm"
 data-testid="kiro-runtime-code-execution-sandbox-command"
 :placeholder="
 t(
 'admin.settings.kiroRuntime.codeExecutionSandboxCommandPlaceholder',
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.kiroRuntime.codeExecutionSandboxCommandHint",
 )
 }}
 </p>
 <div
 class="mt-2 rounded-lg border border-danger-200 bg-danger-50 p-3 text-xs leading-5 text-danger-700 "
 >
 {{
 t(
 "admin.settings.kiroRuntime.codeExecutionSandboxCommandWarning",
 )
 }}
 </div>
 </div>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.cacheHitRateScale") }}
 </label>
 <input
 :value="form.cache_hit_rate_scale ?? ''"
 type="number"
 min="0"
 max="100"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-hit-rate-scale"
 :placeholder="t('admin.settings.kiroRuntime.cacheHitRateScalePlaceholder')"
 @input="
 form.cache_hit_rate_scale = parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.kiroRuntime.cacheHitRateScaleHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.cacheMinBlockTokens") }}
 </label>
 <input
 :value="form.cache_min_block_tokens ?? ''"
 type="number"
 min="0"
 :max="KIRO_CACHE_MIN_BLOCK_TOKENS_MAX"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-min-block-tokens"
 :placeholder="`0 - ${KIRO_CACHE_MIN_BLOCK_TOKENS_MAX}`"
 @input="
 form.cache_min_block_tokens = parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t("admin.settings.kiroRuntime.cacheMinBlockTokensHint")
 }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{
 t(
 "admin.settings.kiroRuntime.cacheIndependentTtlSeconds",
 )
 }}
 </label>
 <input
 :value="form.cache_independent_ttl_seconds ?? ''"
 type="number"
 min="60"
 max="86400"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-independent-ttl-seconds"
 :placeholder="t('admin.settings.kiroRuntime.cacheIndependentTtlSecondsPlaceholder')"
 @input="
 form.cache_independent_ttl_seconds =
 parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.kiroRuntime.cacheIndependentTtlSecondsHint",
 )
 }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.kiroRuntime.cachePrefixTtlSeconds") }}
 </label>
 <input
 :value="form.cache_prefix_ttl_seconds ?? ''"
 type="number"
 min="60"
 max="3600"
 step="1"
 class="input"
 data-testid="kiro-runtime-cache-prefix-ttl-seconds"
 :placeholder="t('admin.settings.kiroRuntime.cachePrefixTtlSecondsPlaceholder')"
 @input="
 form.cache_prefix_ttl_seconds = parseOptionalIntegerInput(
 ($event.target as HTMLInputElement).value,
 )
 "
 />
 <p class="settings-row-hint">
 {{
 t(
 "admin.settings.kiroRuntime.cachePrefixTtlSecondsHint",
 )
 }}
 </p>
 </div>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { KIRO_CACHE_MIN_BLOCK_TOKENS_MAX } from "@/api/admin/settings";

const settingsForm = inject(SettingsFormKey)!;
const { t, form } = settingsForm;

function parseOptionalIntegerInput(value: string): number | null {
  if (!value.trim()) return null;

  const normalized = Math.floor(Number(value));
  return Number.isFinite(normalized) ? normalized : null;
}

</script>
