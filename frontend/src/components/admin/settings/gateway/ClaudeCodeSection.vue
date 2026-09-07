<template>
  <!-- Claude Code Settings -->
  <SettingsSection
    :title="t('admin.settings.claudeCode.title')"
    :description="t('admin.settings.claudeCode.description')"
  >
    <div class="settings-rows">
      <SettingRow
        :label="t('admin.settings.claudeCode.minVersion')"
        :description="t('admin.settings.claudeCode.minVersionHint')"
      >
        <input
          v-model="form.min_claude_code_version"
          type="text"
          class="input max-w-xs font-mono text-sm"
          :placeholder="t('admin.settings.claudeCode.minVersionPlaceholder')"
        />
      </SettingRow>
      <SettingRow
        :label="t('admin.settings.claudeCode.maxVersion')"
        :description="t('admin.settings.claudeCode.maxVersionHint')"
      >
        <input
          v-model="form.max_claude_code_version"
          type="text"
          class="input max-w-xs font-mono text-sm"
          :placeholder="t('admin.settings.claudeCode.maxVersionPlaceholder')"
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

const settingsForm = inject(SettingsFormKey)!;
const { t, form } = settingsForm;


</script>
