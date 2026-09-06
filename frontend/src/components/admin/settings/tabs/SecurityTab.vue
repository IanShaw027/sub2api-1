<template>
 <div v-show="activeTab === 'security'" class="settings-stack">
 <AdminApiKeyPanel />
 <RegistrationSecurityPanel />
 <ApiKeyAclPanel />
 <IPSecurityPanel />
 <PanelRateLimitPanel />
 <CaptchaSettingsPanel />
 <LinuxdoOAuthPanel />
 <EmailOAuthPanel />
 <WeChatOAuthPanel />
 <DingtalkOAuthPanel />
 <OidcOAuthPanel />
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import IPSecurityPanel from "@/components/admin/settings/IPSecurityPanel.vue";
import AdminApiKeyPanel from "../security/AdminApiKeyPanel.vue";
import RegistrationSecurityPanel from "../security/RegistrationSecurityPanel.vue";
import ApiKeyAclPanel from "../security/ApiKeyAclPanel.vue";
import PanelRateLimitPanel from "../security/PanelRateLimitPanel.vue";
import CaptchaSettingsPanel from "../security/CaptchaSettingsPanel.vue";
import LinuxdoOAuthPanel from "../security/LinuxdoOAuthPanel.vue";
import EmailOAuthPanel from "../security/EmailOAuthPanel.vue";
import WeChatOAuthPanel from "../security/WeChatOAuthPanel.vue";
import DingtalkOAuthPanel from "../security/DingtalkOAuthPanel.vue";
import OidcOAuthPanel from "../security/OidcOAuthPanel.vue";

const settingsForm = inject(SettingsFormKey)!;
const { activeTab } = settingsForm;
</script>
