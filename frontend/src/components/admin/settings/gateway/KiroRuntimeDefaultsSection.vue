<template>
  <!-- Kiro Runtime Defaults -->
  <SettingsSection
    :title="t('admin.settings.kiroRuntime.title')"
    :description="t('admin.settings.kiroRuntime.description')"
  >
    <div class="settings-rows">
      <div class="settings-rows">
        <SettingRow :label="t('admin.settings.kiroRuntime.kiroVersion')">
          <input
            v-model="form.kiro_version"
            type="text"
            class="input font-mono text-sm"
            data-testid="kiro-runtime-version"
            :placeholder="
              t('admin.settings.kiroRuntime.kiroVersionPlaceholder')
            "
          />
        </SettingRow>
        <SettingRow :label="t('admin.settings.kiroRuntime.kiroCommit')">
          <input
            v-model="form.kiro_commit"
            type="text"
            class="input font-mono text-sm"
            data-testid="kiro-runtime-commit"
            :placeholder="t('admin.settings.kiroRuntime.kiroCommitPlaceholder')"
          />
        </SettingRow>
        <SettingRow :label="t('admin.settings.kiroRuntime.systemVersion')">
          <input
            v-model="form.system_version"
            type="text"
            class="input font-mono text-sm"
            data-testid="kiro-runtime-system-version"
            :placeholder="
              t('admin.settings.kiroRuntime.systemVersionPlaceholder')
            "
          />
        </SettingRow>
        <SettingRow :label="t('admin.settings.kiroRuntime.nodeVersion')">
          <input
            v-model="form.node_version"
            type="text"
            class="input font-mono text-sm"
            data-testid="kiro-runtime-node-version"
            :placeholder="
              t('admin.settings.kiroRuntime.nodeVersionPlaceholder')
            "
          />
        </SettingRow>
      </div>
      <SettingRow
        :label="t('admin.settings.kiroRuntime.codeExecutionSandboxCommand')"
        :description="
          t('admin.settings.kiroRuntime.codeExecutionSandboxCommandHint')
        "
      >
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
        <div
          class="mt-2 rounded-lg border border-danger-200 bg-danger-50 p-3 text-xs leading-5 text-danger-700"
        >
          {{
            t("admin.settings.kiroRuntime.codeExecutionSandboxCommandWarning")
          }}
        </div>
      </SettingRow>
      <div class="settings-rows">
        <SettingRow
          :label="t('admin.settings.kiroRuntime.cacheHitRateScale')"
          :description="t('admin.settings.kiroRuntime.cacheHitRateScaleHint')"
        >
          <input
            :value="form.cache_hit_rate_scale ?? ''"
            type="number"
            min="0"
            max="100"
            step="1"
            class="input"
            data-testid="kiro-runtime-cache-hit-rate-scale"
            :placeholder="
              t('admin.settings.kiroRuntime.cacheHitRateScalePlaceholder')
            "
            @input="
              form.cache_hit_rate_scale = parseOptionalIntegerInput(
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </SettingRow>
        <SettingRow
          :label="t('admin.settings.kiroRuntime.cacheMinBlockTokens')"
          :description="t('admin.settings.kiroRuntime.cacheMinBlockTokensHint')"
        >
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
        </SettingRow>
        <SettingRow
          :label="t('admin.settings.kiroRuntime.cacheIndependentTtlSeconds')"
          :description="
            t('admin.settings.kiroRuntime.cacheIndependentTtlSecondsHint')
          "
        >
          <input
            :value="form.cache_independent_ttl_seconds ?? ''"
            type="number"
            min="60"
            max="86400"
            step="1"
            class="input"
            data-testid="kiro-runtime-cache-independent-ttl-seconds"
            :placeholder="
              t(
                'admin.settings.kiroRuntime.cacheIndependentTtlSecondsPlaceholder',
              )
            "
            @input="
              form.cache_independent_ttl_seconds = parseOptionalIntegerInput(
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </SettingRow>
        <SettingRow
          :label="t('admin.settings.kiroRuntime.cachePrefixTtlSeconds')"
          :description="
            t('admin.settings.kiroRuntime.cachePrefixTtlSecondsHint')
          "
        >
          <input
            :value="form.cache_prefix_ttl_seconds ?? ''"
            type="number"
            min="60"
            max="3600"
            step="1"
            class="input"
            data-testid="kiro-runtime-cache-prefix-ttl-seconds"
            :placeholder="
              t('admin.settings.kiroRuntime.cachePrefixTtlSecondsPlaceholder')
            "
            @input="
              form.cache_prefix_ttl_seconds = parseOptionalIntegerInput(
                ($event.target as HTMLInputElement).value,
              )
            "
          />
        </SettingRow>
      </div>
    </div>
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
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
