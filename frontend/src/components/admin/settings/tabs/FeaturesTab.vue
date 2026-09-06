<template>
 <div v-show="activeTab === 'features'" class="settings-stack">

 <SettingsSection
 :title="t('admin.settings.features.ticket.title')"
 :description="t('admin.settings.features.ticket.description')"
 >
 <SettingRow
 :label="t('admin.settings.features.ticket.enabled')"
 :description="t('admin.settings.features.ticket.enabledHint')"
 >
 <Toggle v-model="form.ticket_enabled" />
 </SettingRow>
 </SettingsSection>

 <SettingsSection
 :title="t('admin.settings.features.creationCenter.title')"
 :description="t('admin.settings.features.creationCenter.description')"
 >
 <SettingRow
 :label="t('admin.settings.features.creationCenter.enabled')"
 :description="t('admin.settings.features.creationCenter.enabledHint')"
 >
 <Toggle v-model="form.creation_center_enabled" />
 </SettingRow>
 </SettingsSection>

 <SettingsSection>
 <template #header>
 <h2 class="settings-card-title">
 {{ t('admin.settings.features.channelMonitor.title') }}
 </h2>
 <p class="settings-card-desc">
 {{ t('admin.settings.features.channelMonitor.description') }}
 </p>
 <p class="mt-1.5 text-xs">
 <router-link
 to="/admin/channels/monitor"
 class="inline-flex items-center gap-1 text-accent hover:underline "
 >
 {{ t('admin.settings.features.channelMonitor.configureLink') }}
 <span aria-hidden="true">→</span>
 </router-link>
 </p>
 </template>
 <SettingRow
 :label="t('admin.settings.features.channelMonitor.enabled')"
 :description="t('admin.settings.features.channelMonitor.enabledHint')"
 >
 <Toggle v-model="form.channel_monitor_enabled" />
 </SettingRow>

 <SettingRow v-if="form.channel_monitor_enabled" :label="t('admin.settings.features.channelMonitor.mode')">
 <div class="inline-flex w-full max-w-md rounded-lg border border-line bg-surface-2 p-1 ">
 <button
 type="button"
 class="inline-flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
 :class="
 form.channel_monitor_mode === 'v2'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="form.channel_monitor_mode = 'v2'"
 >
 {{ t('admin.settings.features.channelMonitor.modeV2') }}
 </button>
 <button
 type="button"
 class="inline-flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
 :class="
 form.channel_monitor_mode === 'v1'
 ? 'bg-surface text-accent shadow-sm '
 : 'text-muted hover:text-foreground '
 "
 @click="form.channel_monitor_mode = 'v1'"
 >
 {{ t('admin.settings.features.channelMonitor.modeV1') }}
 </button>
 </div>
 <p class="settings-row-hint mt-1.5">
 {{
 form.channel_monitor_mode === 'v1'
 ? t('admin.settings.features.channelMonitor.modeV1Hint')
 : t('admin.settings.features.channelMonitor.modeV2Hint')
 }}
 </p>
 <p class="mt-1 text-xs text-muted ">
 {{ t('admin.settings.features.channelMonitor.modeHint') }}
 </p>
 </SettingRow>

 <SettingRow
 v-if="form.channel_monitor_enabled && form.channel_monitor_mode === 'v1'"
 :label="t('admin.settings.features.channelMonitor.defaultInterval')"
 :description="t('admin.settings.features.channelMonitor.defaultIntervalHint')"
 >
 <div class="settings-flex-row flex items-center gap-1">
 <input
 v-model.number="form.channel_monitor_default_interval_seconds"
 type="number"
 min="15"
 max="3600"
 class="input"
 />
 <span class="text-danger-text">*</span>
 </div>
 </SettingRow>

 <SettingRow
 v-if="form.channel_monitor_enabled && form.channel_monitor_mode === 'v2'"
 :label="t('admin.settings.features.channelMonitor.hideThroughput')"
 :description="t('admin.settings.features.channelMonitor.hideThroughputHint')"
 >
 <Toggle v-model="form.channel_monitor_hide_throughput" />
 </SettingRow>

 <SettingRow
 v-if="form.channel_monitor_enabled && form.channel_monitor_mode === 'v1'"
 :label="t('admin.settings.features.channelMonitor.showQuota')"
 :description="t('admin.settings.features.channelMonitor.showQuotaHint')"
 >
 <Toggle v-model="form.channel_monitor_show_quota" />
 </SettingRow>
 </SettingsSection>

 <SettingsSection>
 <template #header>
 <h2 class="settings-card-title">
 {{ t('admin.settings.features.availableChannels.title') }}
 </h2>
 <p class="settings-card-desc">
 {{ t('admin.settings.features.availableChannels.description') }}
 </p>
 <p class="mt-1.5 text-xs">
 <router-link
 to="/admin/channels/pricing"
 class="inline-flex items-center gap-1 text-accent hover:underline "
 >
 {{ t('admin.settings.features.availableChannels.configureLink') }}
 <span aria-hidden="true">→</span>
 </router-link>
 </p>
 </template>
 <SettingRow
 :label="t('admin.settings.features.availableChannels.enabled')"
 :description="t('admin.settings.features.availableChannels.enabledHint')"
 >
 <Toggle v-model="form.available_channels_enabled" />
 </SettingRow>
 </SettingsSection>

 <SettingsSection
 :title="t('admin.settings.features.modelPlaza.title')"
 :description="t('admin.settings.features.modelPlaza.description')"
 >
 <SettingRow
 :label="t('admin.settings.features.modelPlaza.enabled')"
 :description="t('admin.settings.features.modelPlaza.enabledHint')"
 >
 <Toggle v-model="form.model_plaza_enabled" />
 </SettingRow>

 <SettingRow
 v-if="form.model_plaza_enabled"
 :label="t('admin.settings.features.modelPlaza.requireAuth')"
 :description="t('admin.settings.features.modelPlaza.requireAuthHint')"
 >
 <Toggle v-model="form.model_plaza_require_auth" />
 </SettingRow>

 <SettingRow
 v-if="form.model_plaza_enabled"
 :label="t('admin.settings.features.modelPlaza.priceDescription')"
 :description="t('admin.settings.features.modelPlaza.priceDescriptionHint')"
 >
 <textarea
 v-model="form.model_plaza_description"
 rows="6"
 class="input font-mono text-sm"
 ></textarea>
 </SettingRow>
 </SettingsSection>

 <SettingsSection
 :title="t('admin.settings.features.pluginManagement.title')"
 :description="t('admin.settings.features.pluginManagement.description')"
 >
 <SettingRow
 :label="t('admin.settings.features.pluginManagement.enabled')"
 :description="t('admin.settings.features.pluginManagement.enabledHint')"
 >
 <Toggle v-model="form.plugin_management_enabled" />
 </SettingRow>
 </SettingsSection>

 <SettingsSection>
 <template #header>
 <h2 class="settings-card-title">
 {{ t('admin.settings.features.riskControl.title') }}
 </h2>
 <p class="settings-card-desc">
 {{ t('admin.settings.features.riskControl.description') }}
 </p>
 <p class="mt-1.5 text-xs">
 <router-link
 to="/admin/risk-control"
 class="inline-flex items-center gap-1 text-accent hover:underline "
 >
 {{ t('admin.settings.features.riskControl.configureLink') }}
 <span aria-hidden="true">→</span>
 </router-link>
 </p>
 </template>
 <SettingRow
 :label="t('admin.settings.features.riskControl.enabled')"
 :description="t('admin.settings.features.riskControl.enabledHint')"
 >
 <Toggle v-model="form.risk_control_enabled" />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.riskControl.cyberSessionBlock')"
 :description="t('admin.settings.features.riskControl.cyberSessionBlockHint')"
 >
 <Toggle v-model="form.cyber_session_block_enabled" />
 </SettingRow>

 <SettingRow v-if="form.cyber_session_block_enabled" :label="t('admin.settings.features.riskControl.cyberSessionBlockTTL')">
 <div class="settings-flex-row flex items-center gap-1">
 <input
 v-model.number="form.cyber_session_block_ttl_seconds"
 type="number"
 min="1"
 class="input"
 />
 <span class="text-danger-text">*</span>
 </div>
 </SettingRow>
 </SettingsSection>

 <!-- Affiliate (邀请返利) feature card -->
 <SettingsSection
 :title="t('admin.settings.features.affiliate.title')"
 :description="t('admin.settings.features.affiliate.description')"
 >
 <SettingRow
 :label="t('admin.settings.features.affiliate.enabled')"
 :description="t('admin.settings.features.affiliate.enabledHint')"
 >
 <Toggle v-model="form.affiliate_enabled" />
 </SettingRow>

 <template v-if="form.affiliate_enabled">
 <SettingRow
 :label="t('admin.settings.features.affiliate.adminRechargeRebate')"
 :description="t('admin.settings.features.affiliate.adminRechargeRebateHint')"
 >
 <Toggle v-model="form.affiliate_admin_recharge_enabled" />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.rebateRate')"
 :description="t('admin.settings.features.affiliate.rebateRateHint')"
 >
 <div class="relative max-w-[420px]">
 <input
 v-model.number="form.affiliate_rebate_rate"
 type="number"
 step="0.01"
 min="0"
 max="100"
 class="input pr-8"
 placeholder="20"
 />
 <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted">%</span>
 </div>
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.freezeHours')"
 :description="t('admin.settings.features.affiliate.freezeHoursDesc')"
 >
 <input
 v-model.number="form.affiliate_rebate_freeze_hours"
 type="number"
 step="1"
 min="0"
 max="720"
 class="input"
 />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.durationDays')"
 :description="t('admin.settings.features.affiliate.durationDaysDesc')"
 >
 <input
 v-model.number="form.affiliate_rebate_duration_days"
 type="number"
 step="1"
 min="0"
 max="3650"
 class="input"
 />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.perInviteeCap')"
 :description="t('admin.settings.features.affiliate.perInviteeCapDesc')"
 >
 <input
 v-model.number="form.affiliate_rebate_per_invitee_cap"
 type="number"
 step="0.01"
 min="0"
 class="input"
 />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.lifetimeCap')"
 :description="t('admin.settings.features.affiliate.lifetimeCapDesc')"
 >
 <input
 v-model.number="form.affiliate_rebate_cap"
 type="number"
 step="0.01"
 min="0"
 class="input"
 />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.inviteeLimit')"
 :description="t('admin.settings.features.affiliate.inviteeLimitDesc')"
 >
 <input
 v-model.number="form.affiliate_rebate_invitee_limit"
 type="number"
 step="1"
 min="0"
 class="input"
 />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.features.affiliate.signupBonus')"
 :description="t('admin.settings.features.affiliate.signupBonusDesc')"
 >
 <input
 v-model.number="form.affiliate_signup_bonus"
 type="number"
 step="0.01"
 min="0"
 class="input"
 />
 </SettingRow>

 <!-- 专属用户管理 -->
 <div class="settings-card-body">
 <div class="settings-flex-row settings-control-row mb-3 flex items-center justify-between">
 <div>
 <h3 class="text-sm font-semibold text-foreground ">
 {{ t('admin.settings.features.affiliate.customUsers.title') }}
 </h3>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t('admin.settings.features.affiliate.customUsers.description') }}
 </p>
 </div>
 <button
 type="button"
 class="btn-glass-primary"
 @click="openAffiliateModal(null)"
 >
 + {{ t('admin.settings.features.affiliate.customUsers.addButton') }}
 </button>
 </div>

 <div class="settings-flex-row mb-3 flex items-center gap-2">
 <input
 v-model="affiliateState.search"
 type="text"
 class="input flex-1"
 :placeholder="t('admin.settings.features.affiliate.customUsers.searchPlaceholder')"
 @input="onAffiliateSearchInput"
 />
 <button
 v-if="affiliateState.selected.length > 0"
 type="button"
 class="btn-glass-secondary"
 @click="openAffiliateBatchModal"
 >
 {{ t('admin.settings.features.affiliate.customUsers.batchButton', { count: affiliateState.selected.length }) }}
 </button>
 </div>

 <div class="overflow-x-auto rounded-lg border border-line ">
 <table class="min-w-full divide-y divide-line ">
 <thead class="bg-surface-2 ">
 <tr>
 <th class="px-3 py-2 text-left">
 <input
 type="checkbox"
 :checked="affiliateState.entries.length > 0 && affiliateState.selected.length === affiliateState.entries.length"
 @change="toggleAffiliateSelectAll"
 />
 </th>
 <th class="px-3 py-2 text-left text-xs font-medium uppercase text-muted">{{ t('admin.settings.features.affiliate.customUsers.col.email') }}</th>
 <th class="px-3 py-2 text-left text-xs font-medium uppercase text-muted">{{ t('admin.settings.features.affiliate.customUsers.col.username') }}</th>
 <th class="px-3 py-2 text-left text-xs font-medium uppercase text-muted">{{ t('admin.settings.features.affiliate.customUsers.col.code') }}</th>
 <th class="px-3 py-2 text-left text-xs font-medium uppercase text-muted">{{ t('admin.settings.features.affiliate.customUsers.col.rate') }}</th>
 <th class="px-3 py-2 text-left text-xs font-medium uppercase text-muted">{{ t('admin.settings.features.affiliate.customUsers.col.actions') }}</th>
 </tr>
 </thead>
 <tbody class="divide-y divide-line bg-surface ">
 <tr v-if="affiliateState.loading">
 <td colspan="6" class="px-3 py-6 text-center text-sm text-muted">
 {{ t('common.loading') }}
 </td>
 </tr>
 <tr v-else-if="affiliateState.entries.length === 0">
 <td colspan="6" class="px-3 py-6 text-center text-sm text-muted">
 {{ t('admin.settings.features.affiliate.customUsers.empty') }}
 </td>
 </tr>
 <tr v-for="entry in affiliateState.entries" :key="entry.user_id">
 <td class="px-3 py-2">
 <input
 type="checkbox"
 :checked="affiliateState.selected.includes(entry.user_id)"
 @change="toggleAffiliateSelect(entry.user_id)"
 />
 </td>
 <td class="px-3 py-2 text-sm text-foreground ">{{ entry.email }}</td>
 <td class="px-3 py-2 text-sm text-muted ">{{ entry.username }}</td>
 <td class="px-3 py-2 text-sm font-mono">
 {{ entry.aff_code }}
 <span
 v-if="entry.aff_code_custom"
 class="ml-1 inline-block rounded bg-accent/10 px-1.5 py-0.5 text-[10px] font-medium text-accent "
 >{{ t('admin.settings.features.affiliate.customUsers.customBadge') }}</span>
 </td>
 <td class="px-3 py-2 text-sm">
 <span v-if="entry.aff_rebate_rate_percent != null">{{ entry.aff_rebate_rate_percent }}%</span>
 <span v-else class="text-muted">{{ t('admin.settings.features.affiliate.customUsers.useGlobal') }}</span>
 </td>
 <td class="px-3 py-2 text-sm">
 <div class="settings-flex-row flex items-center gap-2">
 <button type="button" class="text-accent hover:underline" @click="openAffiliateModal(entry)">
 {{ t('common.edit') }}
 </button>
 <button
 type="button"
 class="text-danger-text hover:underline"
 @click="askResetAffiliateUser(entry)"
 >
 {{ t('common.delete') }}
 </button>
 </div>
 </td>
 </tr>
 </tbody>
 </table>
 </div>

 <div v-if="affiliateState.total > affiliateState.pageSize" class="settings-flex-row settings-control-row mt-3 flex items-center justify-between text-sm">
 <span class="text-muted">
 {{ t('admin.settings.features.affiliate.customUsers.totalLabel', { total: affiliateState.total }) }}
 </span>
 <div class="settings-flex-row flex items-center gap-2">
 <button
 type="button"
 class="btn-glass-secondary"
 :disabled="affiliateState.page <= 1"
 @click="changeAffiliatePage(affiliateState.page - 1)"
 >
 {{ t('pagination.previous') }}
 </button>
 <span class="text-muted">{{ affiliateState.page }} / {{ Math.max(1, Math.ceil(affiliateState.total / affiliateState.pageSize)) }}</span>
 <button
 type="button"
 class="btn-glass-secondary"
 :disabled="affiliateState.page >= Math.ceil(affiliateState.total / affiliateState.pageSize)"
 @click="changeAffiliatePage(affiliateState.page + 1)"
 >
 {{ t('pagination.next') }}
 </button>
 </div>
 </div>
 </div>
 </template>
 </SettingsSection>

 <!-- Affiliate add/edit modal -->
 <div
 v-if="affiliateModal.open"
 class="settings-flex-row fixed inset-0 z-50 flex items-center justify-center glass-modal-scrim p-4"
 @click.self="closeAffiliateModal"
 >
 <div class="w-full max-w-md glass-card-solid rounded-hero p-6">
 <h3 class="mb-4 settings-card-title">
 {{ affiliateModal.mode === 'add' ? t('admin.settings.features.affiliate.modal.addTitle') : t('admin.settings.features.affiliate.modal.editTitle') }}
 </h3>
 <div class="space-y-4">
 <div v-if="affiliateModal.mode === 'add'">
 <label class="input-label">{{ t('admin.settings.features.affiliate.modal.userLabel') }}</label>
 <!-- Chip showing the picked user; clicking it re-opens the search -->
 <div
 v-if="affiliateModal.selectedUser"
 class="settings-flex-row settings-control-row flex items-center justify-between rounded-md border border-accent bg-accent/10 px-3 py-2 "
 >
 <div class="text-sm">
 <span class="font-medium text-foreground ">{{ affiliateModal.selectedUser.email }}</span>
 <span class="ml-1 text-xs text-muted">({{ affiliateModal.selectedUser.username }})</span>
 </div>
 <button
 type="button"
 class="text-lg leading-none text-muted hover:text-danger-text"
 :title="t('admin.settings.features.affiliate.modal.changeUser')"
 @click="clearSelectedAffiliateUser"
 >
 ×
 </button>
 </div>
 <!-- Search input + result dropdown — hidden once a selection is made -->
 <template v-else>
 <input
 v-model="affiliateModal.userQuery"
 type="text"
 class="input"
 :placeholder="t('admin.settings.features.affiliate.modal.userPlaceholder')"
 @input="onAffiliateUserSearchInput"
 />
 <div
 v-if="affiliateModal.userResults.length > 0"
 class="mt-1 max-h-40 overflow-y-auto rounded border border-line "
 >
 <button
 v-for="u in affiliateModal.userResults"
 :key="u.id"
 type="button"
 class="w-full px-3 py-1.5 text-left text-sm hover:bg-surface-2 "
 @click="selectAffiliateUser(u)"
 >
 {{ u.email }} <span class="text-xs text-muted">({{ u.username }})</span>
 </button>
 </div>
 </template>
 </div>
 <div v-else>
 <label class="input-label">{{ t('admin.settings.features.affiliate.modal.userLabel') }}</label>
 <input
 type="text"
 class="input"
 :value="affiliateModal.editingEntry ? affiliateModal.editingEntry.email : ''"
 disabled
 />
 </div>

 <div>
 <label class="input-label">{{ t('admin.settings.features.affiliate.modal.codeLabel') }}</label>
 <input
 v-model="affiliateModal.code"
 type="text"
 class="input font-mono"
 :placeholder="t('admin.settings.features.affiliate.modal.codePlaceholder')"
 maxlength="32"
 />
 <p class="mt-1 text-xs text-muted">
 {{ t('admin.settings.features.affiliate.modal.codeHint') }}
 </p>
 </div>

 <div>
 <label class="input-label">{{ t('admin.settings.features.affiliate.modal.rateLabel') }}</label>
 <div class="relative">
 <input
 v-model="affiliateModal.rate"
 type="number"
 step="0.01"
 min="0"
 max="100"
 class="input pr-8"
 :placeholder="t('admin.settings.features.affiliate.modal.ratePlaceholder')"
 />
 <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted">%</span>
 </div>
 <p class="mt-1 text-xs text-muted">
 {{ t('admin.settings.features.affiliate.modal.rateHint') }}
 </p>
 </div>
 </div>

 <div class="settings-flex-row settings-control-row mt-6 flex items-center justify-between gap-3">
 <p
 v-if="!affiliateModalCanSubmit"
 class="text-xs text-muted "
 >
 {{ t('admin.settings.features.affiliate.modal.errorEmpty') }}
 </p>
 <span v-else></span>
 <div class="settings-flex-row flex gap-2">
 <button type="button" class="btn-glass-secondary" @click="closeAffiliateModal">
 {{ t('common.cancel') }}
 </button>
 <button
 type="button"
 class="btn-glass-primary"
 :disabled="affiliateModal.saving || !affiliateModalCanSubmit"
 @click="submitAffiliateModal"
 >
 {{ affiliateModal.saving ? t('common.saving') : t('common.save') }}
 </button>
 </div>
 </div>
 </div>
 </div>

 <!-- Affiliate batch rate modal -->
 <div
 v-if="affiliateBatchModal.open"
 class="settings-flex-row fixed inset-0 z-50 flex items-center justify-center glass-modal-scrim p-4"
 @click.self="affiliateBatchModal.open = false"
 >
 <div class="w-full max-w-md glass-card-solid rounded-hero p-6">
 <h3 class="mb-4 settings-card-title">
 {{ t('admin.settings.features.affiliate.batchModal.title', { count: affiliateState.selected.length }) }}
 </h3>
 <p class="mb-4 text-sm text-muted">
 {{ t('admin.settings.features.affiliate.batchModal.hint') }}
 </p>
 <div class="relative">
 <input
 v-model="affiliateBatchModal.rate"
 type="number"
 step="0.01"
 min="0"
 max="100"
 class="input pr-8"
 :placeholder="t('admin.settings.features.affiliate.batchModal.placeholder')"
 />
 <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted">%</span>
 </div>
 <p class="mt-2 text-xs text-muted">
 {{ t('admin.settings.features.affiliate.batchModal.clearHint') }}
 </p>
 <div class="settings-flex-row mt-6 flex justify-end gap-2">
 <button type="button" class="btn-glass-secondary" @click="affiliateBatchModal.open = false">
 {{ t('common.cancel') }}
 </button>
 <button
 type="button"
 class="btn-glass-primary"
 :disabled="affiliateBatchModal.saving"
 @click="submitAffiliateBatchModal"
 >
 {{ affiliateBatchModal.saving ? t('common.saving') : t('common.save') }}
 </button>
 </div>
 </div>
 </div>

 </div><!-- /Tab: Features -->
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { reactive, computed } from "vue";
import Toggle from "@/components/common/Toggle.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import SettingRow from "@/components/ui/SettingRow.vue";
import { affiliatesAPI, type AffiliateAdminEntry, type SimpleUser as AffiliateSimpleUser } from "@/api/admin/affiliates";
import { extractApiErrorMessage } from "@/utils/apiError";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  appStore,
  activeTab,
  form,
  affiliateState,
  affiliateConfirmDialog,
  loadAffiliateUsers,
} = settingsForm;

interface AffiliateModalState {
  open: boolean;
  mode: "add" | "edit";
  saving: boolean;
  userQuery: string;
  userResults: AffiliateSimpleUser[];
  selectedUser: AffiliateSimpleUser | null;
  editingEntry: AffiliateAdminEntry | null;
  code: string;
  rate: string | number;
  searchTimer: number | null;
}

const affiliateModal = reactive<AffiliateModalState>({
  open: false,
  mode: "add",
  saving: false,
  userQuery: "",
  userResults: [],
  selectedUser: null,
  editingEntry: null,
  code: "",
  rate: "",
  searchTimer: null,
});

const affiliateBatchModal = reactive<{
  open: boolean;
  saving: boolean;
  rate: string | number;
}>({
  open: false,
  saving: false,
  rate: "",
});

function openAffiliateConfirm(
  title: string,
  message: string,
  confirmText: string,
  fn: () => Promise<unknown>,
) {
  affiliateConfirmDialog.title = title;
  affiliateConfirmDialog.message = message;
  affiliateConfirmDialog.confirmText = confirmText;
  affiliateConfirmDialog.pending = fn;
  affiliateConfirmDialog.show = true;
}

function debounceTimer(slot: { searchTimer: number | null }, delayMs: number, run: () => void) {
  if (slot.searchTimer != null) window.clearTimeout(slot.searchTimer);
  slot.searchTimer = window.setTimeout(run, delayMs);
}

function parseRebateRate(raw: unknown): number | null | undefined {
  const s = String(raw ?? "").trim();
  if (s === "") return null;
  const parsed = Number(s);
  if (Number.isNaN(parsed) || parsed < 0 || parsed > 100) {
    appStore.showError(t("admin.settings.features.affiliate.modal.errorBadRate"));
    return undefined;
  }
  return parsed;
}

function onAffiliateSearchInput() {
  debounceTimer(affiliateState, 300, () => {
    affiliateState.page = 1;
    loadAffiliateUsers();
  });
}

function changeAffiliatePage(page: number) {
  if (page < 1) return;
  affiliateState.page = page;
  loadAffiliateUsers();
}

function toggleAffiliateSelectAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked;
  affiliateState.selected = checked ? affiliateState.entries.map((entry) => entry.user_id) : [];
}

function toggleAffiliateSelect(userId: number) {
  const idx = affiliateState.selected.indexOf(userId);
  if (idx >= 0) affiliateState.selected.splice(idx, 1);
  else affiliateState.selected.push(userId);
}

function openAffiliateModal(entry: AffiliateAdminEntry | null) {
  affiliateModal.open = true;
  affiliateModal.mode = entry ? "edit" : "add";
  affiliateModal.userQuery = "";
  affiliateModal.userResults = [];
  affiliateModal.selectedUser = null;
  affiliateModal.editingEntry = entry;
  affiliateModal.code = entry?.aff_code_custom ? entry.aff_code : "";
  affiliateModal.rate =
    entry?.aff_rebate_rate_percent != null ? String(entry.aff_rebate_rate_percent) : "";
}

function closeAffiliateModal() {
  affiliateModal.open = false;
  if (affiliateModal.searchTimer != null) {
    window.clearTimeout(affiliateModal.searchTimer);
    affiliateModal.searchTimer = null;
  }
}

function onAffiliateUserSearchInput() {
  const q = affiliateModal.userQuery.trim();
  if (!q) {
    affiliateModal.userResults = [];
    return;
  }
  debounceTimer(affiliateModal, 300, async () => {
    try {
      affiliateModal.userResults = await affiliatesAPI.lookupUsers(q);
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    }
  });
}

function selectAffiliateUser(user: AffiliateSimpleUser) {
  affiliateModal.selectedUser = user;
  affiliateModal.userQuery = "";
  affiliateModal.userResults = [];
}

function clearSelectedAffiliateUser() {
  affiliateModal.selectedUser = null;
}

const affiliateModalCanSubmit = computed(() => {
  if (affiliateModal.mode === "add") {
    if (!affiliateModal.selectedUser) return false;
  } else if (!affiliateModal.editingEntry) {
    return false;
  }
  const codeFilled = affiliateModal.code.trim() !== "";
  const rateFilled = String(affiliateModal.rate ?? "").trim() !== "";
  if (codeFilled || rateFilled) return true;
  // Edit mode + empty rate input is a meaningful "clear" only if the user
  // currently has an exclusive rate to clear.
  return (
    affiliateModal.mode === "edit" &&
    affiliateModal.editingEntry?.aff_rebate_rate_percent != null
  );
});

async function submitAffiliateModal() {
  if (!affiliateModalCanSubmit.value) {
    // Should be unreachable because the button is disabled, but keep a guard.
    appStore.showError(t("admin.settings.features.affiliate.modal.errorEmpty"));
    return;
  }

  let userId: number;
  if (affiliateModal.mode === "add") {
    userId = affiliateModal.selectedUser!.id;
  } else {
    userId = affiliateModal.editingEntry!.user_id;
  }

  const payload: Parameters<typeof affiliatesAPI.updateUserSettings>[1] = {};
  const codeRaw = affiliateModal.code.trim();
  if (codeRaw) payload.aff_code = codeRaw.toUpperCase();

  const rateInput = parseRebateRate(affiliateModal.rate);
  if (rateInput === undefined) return; // toast already shown
  if (rateInput === null) {
    if (affiliateModal.mode === "edit" && affiliateModal.editingEntry?.aff_rebate_rate_percent != null) {
      payload.clear_rebate_rate = true;
    }
  } else {
    payload.aff_rebate_rate_percent = rateInput;
  }

  affiliateModal.saving = true;
  try {
    await affiliatesAPI.updateUserSettings(userId, payload);
    appStore.showSuccess(t("common.saved"));
    closeAffiliateModal();
    affiliateState.page = 1;
    await loadAffiliateUsers();
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    affiliateModal.saving = false;
  }
}

function askResetAffiliateUser(entry: AffiliateAdminEntry) {
  openAffiliateConfirm(
    t("admin.settings.features.affiliate.customUsers.resetTitle"),
    t("admin.settings.features.affiliate.customUsers.resetMessage", {
      email: entry.email || `#${entry.user_id}`,
    }),
    t("common.delete"),
    () => affiliatesAPI.clearUserSettings(entry.user_id),
  );
}

function openAffiliateBatchModal() {
  if (affiliateState.selected.length === 0) return;
  affiliateBatchModal.open = true;
  affiliateBatchModal.rate = "";
}

async function submitAffiliateBatchModal() {
  const rateInput = parseRebateRate(affiliateBatchModal.rate);
  if (rateInput === undefined) return;
  const userIDs = [...affiliateState.selected];
  const payload: Parameters<typeof affiliatesAPI.batchSetRate>[0] =
    rateInput === null
      ? { user_ids: userIDs, clear: true }
      : { user_ids: userIDs, aff_rebate_rate_percent: rateInput };

  affiliateBatchModal.saving = true;
  try {
    await affiliatesAPI.batchSetRate(payload);
    appStore.showSuccess(t("common.saved"));
    affiliateBatchModal.open = false;
    affiliateState.selected = [];
    await loadAffiliateUsers();
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t("common.error")));
  } finally {
    affiliateBatchModal.saving = false;
  }
}
</script>
