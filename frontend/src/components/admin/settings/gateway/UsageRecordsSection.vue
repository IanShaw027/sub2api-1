<template>
 <!-- Usage Records Settings -->
 <div class="glass-card settings-card">
 <div class="settings-card-head">
 <h2 class="settings-card-title">
 {{ t('admin.settings.usageRecords.title') }}
 </h2>
 <p class="settings-card-desc">
 {{ t('admin.settings.usageRecords.description') }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- User error requests visibility -->
 <div class="flex items-center justify-between">
 <div>
 <label class="text-sm font-medium text-foreground ">
 {{ t('admin.settings.user_error_view.label') }}
 </label>
 <p class="text-xs text-muted ">
 {{ t('admin.settings.user_error_view.description') }}
 </p>
 </div>
 <label class="toggle">
 <input v-model="form.allow_user_view_error_requests" type="checkbox" />
 <span class="toggle-slider"></span>
 </label>
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
