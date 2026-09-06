<template>
 <div v-show="activeTab === 'backup'">
 <BackupSettings />
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import BackupSettings from "@/views/admin/BackupView.vue";

const settingsForm = inject(SettingsFormKey)!;
const {
  activeTab,
} = settingsForm;


</script>
