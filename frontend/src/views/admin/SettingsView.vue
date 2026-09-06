<template>

 <AppLayout>
 <div class="settings-page">
 <PageHeader :title="t('admin.settings.title')" :description="t('admin.settings.description')">
 <template #actions>
 <span
 v-if="settingsDirtyCount > 0"
 class="settings-dirty"
 data-testid="settings-dirty-indicator"
 >
 <span class="settings-dirty-dot" aria-hidden="true"></span>
 {{ t("admin.settings.unsavedChanges", { count: settingsDirtyCount }) }}
 </span>
 <Button
 v-show="activeTab !== 'backup'"
 variant="secondary"
 native-type="button"
 :disabled="saving || loading || loadFailed || settingsDirtyCount === 0"
 data-testid="settings-reset"
 @click="resetSettingsForm"
 >
 {{ t("common.reset") }}
 </Button>
 <Button
 v-show="activeTab !== 'backup'"
 native-type="submit"
 form="settings-form"
 :disabled="saving || loadFailed"
 :loading="saving"
 >
 {{ saving ? t("admin.settings.saving") : t("admin.settings.saveSettings") }}
 </Button>
 </template>
 </PageHeader>
 <div v-if="loading" class="flex items-center justify-center py-12">
 <div class="settings-spinner"></div>
 </div>

 <form
 v-else
 id="settings-form"
 @submit.prevent="saveSettings"
 class="settings-layout"
 novalidate
 >
 <aside class="settings-nav">
 <label class="settings-nav-mobile">
 <span class="sr-only">{{ t('admin.settings.title') }}</span>
 <select
 class="field"
 :value="activeTab"
 @change="onSettingsMobileTabChange($event)"
 >
 <option v-for="tab in settingsTabs" :key="tab.key" :value="tab.key">
 {{ t(`admin.settings.tabs.${tab.key}`) }}
 </option>
 </select>
 </label>
 <nav
 class="settings-tabs-scroll"
 role="tablist"
 :aria-label="t('admin.settings.title')"
 >
 <div class="settings-tabs">
 <button
 v-for="tab in settingsTabs"
 :key="tab.key"
 :id="`settings-tab-${tab.key}`"
 type="button"
 role="tab"
 :aria-selected="activeTab === tab.key"
 :tabindex="activeTab === tab.key ? 0 : -1"
 :class="[
 'settings-tab',
 activeTab === tab.key && 'settings-tab-active',
 ]"
 @click="selectSettingsTab(tab.key)"
 @keydown="handleSettingsTabKeydown($event, tab.key)"
 >
 <span class="settings-tab-label">{{
 t(`admin.settings.tabs.${tab.key}`)
 }}</span>
 <span
 v-if="isSettingsTabDirty(tab.key)"
 class="settings-tab-dot"
 aria-hidden="true"
 ></span>
 </button>
 </div>
 </nav>
 <div v-if="hasDeploymentInfo" class="settings-deploy">
 <span class="settings-deploy-title">{{
 t("admin.settings.deployment.title")
 }}</span>
 <div v-if="deploymentVersion" class="settings-deploy-row">
 <span>{{ t("admin.settings.deployment.version") }}</span>
 <span class="settings-deploy-value">{{ deploymentVersion }}</span>
 </div>
 <div v-if="deploymentCodexVersion" class="settings-deploy-row">
 <span>{{ t("admin.settings.deployment.codexSync") }}</span>
 <span class="settings-deploy-value">{{ deploymentCodexVersion }}</span>
 </div>
 </div>
 </aside>
 <div class="settings-content">
 <GeneralTab />
 <AgreementTab />
 <FeaturesTab />
 <SecurityTab />
 <UserDefaultsTab />
 <GatewayTab />
 <PaymentTab />
 <EmailTab />
 <BackupTab />
 </div>
 <!-- Save Button -->
 <div v-show="activeTab !== 'backup'" class="settings-save">
 <button
 type="submit"
 :disabled="saving || loadFailed"
 class="btn-glass-primary"
 >
 <svg
 v-if="saving"
 class="h-4 w-4 animate-spin"
 fill="none"
 viewBox="0 0 24 24"
 >
 <circle
 class="opacity-25"
 cx="12"
 cy="12"
 r="10"
 stroke="currentColor"
 stroke-width="4"
 ></circle>
 <path
 class="opacity-75"
 fill="currentColor"
 d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
 ></path>
 </svg>
 {{
 saving
 ? t("admin.settings.saving")
 : t("admin.settings.saveSettings")
 }}
 </button>
 </div>
 </form>

 <!-- Provider dialogs placed outside the settings form to prevent form submission bubbling -->
 <PaymentProviderDialog
 ref="providerDialogRef"
 :show="showProviderDialog"
 :saving="providerSaving"
 :editing="editingProvider"
 :all-key-options="providerKeyOptions"
 :enabled-key-options="enabledProviderKeyOptions"
 :all-payment-types="allPaymentTypes"
 :redirect-label="t('admin.settings.payment.easypayRedirect')"
 @close="showProviderDialog = false"
 @save="handleSaveProvider"
 />
 <ConfirmDialog
 :show="showDeleteProviderDialog"
 :title="t('admin.settings.payment.deleteProvider')"
 :message="t('admin.settings.payment.deleteProviderConfirm')"
 :confirm-text="t('common.delete')"
 danger
 @confirm="handleDeleteProvider"
 @cancel="showDeleteProviderDialog = false"
 />
 <ConfirmDialog
 :show="affiliateConfirmDialog.show"
 :title="affiliateConfirmDialog.title"
 :message="affiliateConfirmDialog.message"
 :confirm-text="affiliateConfirmDialog.confirmText"
 danger
 @confirm="handleAffiliateConfirm"
 @cancel="cancelAffiliateConfirm"
 />
 <!-- 关闭 step-up 开关等敏感保存操作触发的 TOTP 二次验证 -->
 <TotpStepUpDialog :controller="settingsStepUp" />
 </div>
 </AppLayout>
</template>

<script setup lang="ts">
import { provide } from "vue";
import { useSettingsForm, SettingsFormKey } from "@/components/admin/settings/useSettingsForm";
import "@/components/admin/settings/settings.css";
import AppLayout from "@/components/layout/AppLayout.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import Button from "@/components/ui/Button.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import PaymentProviderDialog from "@/components/payment/PaymentProviderDialog.vue";
import TotpStepUpDialog from "@/components/auth/TotpStepUpDialog.vue";
import GeneralTab from "@/components/admin/settings/tabs/GeneralTab.vue";
import AgreementTab from "@/components/admin/settings/tabs/AgreementTab.vue";
import FeaturesTab from "@/components/admin/settings/tabs/FeaturesTab.vue";
import SecurityTab from "@/components/admin/settings/tabs/SecurityTab.vue";
import UserDefaultsTab from "@/components/admin/settings/tabs/UserDefaultsTab.vue";
import GatewayTab from "@/components/admin/settings/tabs/GatewayTab.vue";
import PaymentTab from "@/components/admin/settings/tabs/PaymentTab.vue";
import EmailTab from "@/components/admin/settings/tabs/EmailTab.vue";
import BackupTab from "@/components/admin/settings/tabs/BackupTab.vue";

const settingsForm = useSettingsForm();
provide(SettingsFormKey, settingsForm);
const {
  t,
  settingsStepUp,
  activeTab,
  settingsTabs,
  selectSettingsTab,
  onSettingsMobileTabChange,
  handleSettingsTabKeydown,
  loading,
  loadFailed,
  saving,
  settingsDirtyCount,
  isSettingsTabDirty,
  resetSettingsForm,
  deploymentVersion,
  deploymentCodexVersion,
  hasDeploymentInfo,
  saveSettings,
  allPaymentTypes,
  providerSaving,
  showProviderDialog,
  showDeleteProviderDialog,
  editingProvider,
  providerDialogRef,
  providerKeyOptions,
  enabledProviderKeyOptions,
  handleSaveProvider,
  handleDeleteProvider,
  affiliateConfirmDialog,
  handleAffiliateConfirm,
  cancelAffiliateConfirm,
} = settingsForm;
</script>

<style scoped></style>
