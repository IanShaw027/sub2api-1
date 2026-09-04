<template>
 <!-- Claude Code Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.claudeCode.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.claudeCode.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.claudeCode.minVersion") }}
 </label>
 <input
 v-model="form.min_claude_code_version"
 type="text"
 class="input max-w-xs font-mono text-sm"
 :placeholder="
 t('admin.settings.claudeCode.minVersionPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.claudeCode.minVersionHint") }}
 </p>
 </div>
 <div class="settings-row mt-4">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.claudeCode.maxVersion") }}
 </label>
 <input
 v-model="form.max_claude_code_version"
 type="text"
 class="input max-w-xs font-mono text-sm"
 :placeholder="
 t('admin.settings.claudeCode.maxVersionPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.claudeCode.maxVersionHint") }}
 </p>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";

const settingsForm = inject(SettingsFormKey)!;
const { t, form } = settingsForm;


</script>
