<template>
  <!-- Usage Records Settings -->
  <SettingsSection
    :title="t('admin.settings.usageRecords.title')"
    :description="t('admin.settings.usageRecords.description')"
  >
    <div class="settings-rows">
      <!-- User error requests visibility -->
      <SettingRow
        :label="t('admin.settings.user_error_view.label')"
        :description="t('admin.settings.user_error_view.description')"
      >
        <label class="toggle">
          <input
            v-model="form.allow_user_view_error_requests"
            type="checkbox"
          />
          <span class="toggle-slider"></span>
        </label>
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
