<template>
 <div v-show="activeTab === 'users'" class="settings-stack">
 <!-- Default Settings -->
 <SettingsSection
 :title="t('admin.settings.defaults.title')"
 :description="t('admin.settings.defaults.description')"
 >
 <SettingRow
 :label="t('admin.settings.defaults.defaultBalance')"
 :description="t('admin.settings.defaults.defaultBalanceHint')"
 >
 <input
 v-model.number="form.default_balance"
 type="number"
 step="0.01"
 min="0"
 class="input"
 placeholder="0.00"
 />
 </SettingRow>
 <SettingRow
 :label="t('admin.settings.defaults.defaultConcurrency')"
 :description="t('admin.settings.defaults.defaultConcurrencyHint')"
 >
 <input
 v-model.number="form.default_concurrency"
 type="number"
 min="1"
 class="input"
 placeholder="1"
 />
 </SettingRow>
 <SettingRow
 :label="t('admin.settings.defaults.defaultUserRpmLimit')"
 :description="t('admin.settings.defaults.defaultUserRpmLimitHint')"
 >
 <input
 v-model.number="form.default_user_rpm_limit"
 type="number"
 min="0"
 step="1"
 class="input"
 placeholder="0"
 />
 </SettingRow>

 <SettingRow
 :label="t('admin.settings.defaults.defaultSubscriptions')"
 :description="t('admin.settings.defaults.defaultSubscriptionsHint')"
 >
 <div class="settings-flex-row mb-3 flex justify-end">
 <button
 type="button"
 class="btn-glass-secondary"
 @click="addDefaultSubscription"
 :disabled="subscriptionGroups.length === 0"
 >
 {{ t("admin.settings.defaults.addDefaultSubscription") }}
 </button>
 </div>

 <div
 v-if="form.default_subscriptions.length === 0"
 class="rounded border border-dashed border-line px-4 py-3 text-sm text-muted "
 >
 {{ t("admin.settings.defaults.defaultSubscriptionsEmpty") }}
 </div>

 <div v-else class="space-y-3">
 <div
 v-for="(item, index) in form.default_subscriptions"
 :key="`default-sub-${index}`"
 class="grid grid-cols-1 gap-3 rounded border border-line p-3 md:grid-cols-[1fr_160px_auto] "
 >
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.defaults.subscriptionGroup") }}
 </label>
 <Select
 v-model="item.group_id"
 class="default-sub-group-select"
 :options="defaultSubscriptionGroupOptions"
 :placeholder="
 t('admin.settings.defaults.subscriptionGroup')
 "
 >
 <template #selected="{ option }">
 <GroupBadge
 v-if="option"
 :name="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).label
 "
 :platform="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).platform
 "
 :subscription-type="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).subscriptionType
 "
 :rate-multiplier="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).rate
 "
 />
 <span v-else class="text-muted">
 {{ t("admin.settings.defaults.subscriptionGroup") }}
 </span>
 </template>
 <template #option="{ option, selected }">
 <GroupOptionItem
 :name="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).label
 "
 :platform="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).platform
 "
 :subscription-type="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).subscriptionType
 "
 :rate-multiplier="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).rate
 "
 :description="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).description
 "
 :selected="selected"
 />
 </template>
 </Select>
 </div>
 <div>
 <label
 class="settings-sub-label"
 >
 {{
 t("admin.settings.defaults.subscriptionValidityDays")
 }}
 </label>
 <input
 v-model.number="item.validity_days"
 type="number"
 min="1"
 max="36500"
 class="input h-[42px]"
 />
 </div>
 <div class="settings-flex-row flex items-end">
 <button
 type="button"
 class="btn-glass-secondary default-sub-delete-btn w-full text-danger-text "
 @click="removeDefaultSubscription(index)"
 >
 {{ t("common.delete") }}
 </button>
 </div>
 </div>
 </div>
 </SettingRow>

 <!-- ★ 新增：系统全局默认平台限额矩阵 -->
 <SettingRow
 :label="t('admin.settings.defaults.defaultPlatformQuotas')"
 :description="t('admin.settings.defaults.defaultPlatformQuotasHint')"
 >
 <p class="mb-2 text-xs text-warning-text ">
 {{ t("admin.settings.defaults.platformQuotaNotice") }}
 </p>
 <div class="overflow-x-auto">
 <table class="min-w-full text-sm">
 <thead>
 <tr class="text-left text-xs text-muted ">
 <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.platform") }}</th>
 <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.daily") }}</th>
 <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.weekly") }}</th>
 <th class="pb-2 font-medium">{{ t("admin.settings.platformQuota.monthly") }}</th>
 </tr>
 </thead>
 <tbody class="space-y-2">
 <tr v-for="p in (['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro'] as const)" :key="p" class="align-top">
 <td class="pr-4 py-1">
 <span class="font-mono text-xs text-foreground ">{{ p }}</span>
 </td>
 <td class="pr-4 py-1">
 <input
 v-model.number="form.default_platform_quotas[p]!.daily"
 type="number"
 step="0.01"
 min="0"
 class="input h-8 w-28 text-sm"
 :placeholder="t('admin.settings.platformQuota.placeholder')"
 />
 </td>
 <td class="pr-4 py-1">
 <input
 v-model.number="form.default_platform_quotas[p]!.weekly"
 type="number"
 step="0.01"
 min="0"
 class="input h-8 w-28 text-sm"
 :placeholder="t('admin.settings.platformQuota.placeholder')"
 />
 </td>
 <td class="py-1">
 <input
 v-model.number="form.default_platform_quotas[p]!.monthly"
 type="number"
 step="0.01"
 min="0"
 class="input h-8 w-28 text-sm"
 :placeholder="t('admin.settings.platformQuota.placeholder')"
 />
 </td>
 </tr>
 </tbody>
 </table>
 </div>
 </SettingRow>
 <!-- /全局平台限额矩阵 -->
 </SettingsSection>

 <SettingsSection
 :title="t('admin.settings.authSourceDefaults.title')"
 :description="t('admin.settings.authSourceDefaults.description')"
 >
 <SettingRow
 :label="t('admin.settings.authSourceDefaults.requireEmailLabel')"
 :description="t('admin.settings.authSourceDefaults.requireEmailHint')"
 >
 <Toggle v-model="form.force_email_on_third_party_signup" />
 </SettingRow>

 <div class="settings-card-body">
 <div class="space-y-4">
 <div
 v-for="authSource in authSourceDefaultsMeta"
 :key="authSource.source"
 class="rounded-xl border border-line p-4 "
 >
 <div class="settings-flex-row settings-control-row flex items-center justify-between gap-4">
 <div>
 <div class="font-medium text-foreground ">
 {{ authSource.title }}
 </div>
 <p class="settings-card-desc">
 {{ authSource.description }}
 </p>
 </div>
 <Toggle
 v-model="
 authSourceDefaults[authSource.source].grant_on_signup
 "
 :data-testid="`auth-source-${authSource.source}-enabled`"
 />
 </div>

 <div
 v-if="authSourceDefaults[authSource.source].grant_on_signup"
 :data-testid="`auth-source-${authSource.source}-panel`"
 class="mt-4 space-y-4 border-t border-line pt-4 "
 >
 <p class="text-sm text-muted ">
 {{ t("admin.settings.authSourceDefaults.enabledHint") }}
 </p>

 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.defaults.defaultBalance") }}
 </label>
 <input
 v-model.number="
 authSourceDefaults[authSource.source].balance
 "
 type="number"
 step="0.01"
 min="0"
 class="input"
 placeholder="0.00"
 />
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.defaults.defaultConcurrency") }}
 </label>
 <input
 v-model.number="
 authSourceDefaults[authSource.source].concurrency
 "
 type="number"
 min="1"
 class="input"
 placeholder="5"
 />
 </div>
 </div>

 <div
 class="settings-flex-row settings-control-row flex items-center justify-between rounded border border-line px-4 py-3 "
 >
 <div>
 <label
 class="font-medium text-foreground "
 >
 {{ t("admin.settings.authSourceDefaults.grantOnFirstBindLabel") }}
 </label>
 <p
 class="mt-0.5 text-xs text-muted "
 >
 {{ t("admin.settings.authSourceDefaults.grantOnFirstBindHint") }}
 </p>
 </div>
 <Toggle
 v-model="
 authSourceDefaults[authSource.source]
 .grant_on_first_bind
 "
 />
 </div>

 <div class="settings-flex-row settings-control-row mb-3 flex items-center justify-between">
 <div>
 <label
 class="font-medium text-foreground "
 >
 {{ t("admin.settings.authSourceDefaults.defaultSubscriptionsLabel") }}
 </label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.authSourceDefaults.defaultSubscriptionsHint") }}
 </p>
 </div>
 <button
 type="button"
 class="btn-glass-secondary"
 @click="
 addAuthSourceDefaultSubscription(authSource.source)
 "
 :disabled="subscriptionGroups.length === 0"
 >
 {{
 t("admin.settings.defaults.addDefaultSubscription")
 }}
 </button>
 </div>

 <div
 v-if="
 authSourceDefaults[authSource.source].subscriptions
 .length === 0
 "
 class="rounded border border-dashed border-line px-4 py-3 text-sm text-muted "
 >
 {{ t("admin.settings.authSourceDefaults.noSourceSubscriptions") }}
 </div>

 <div v-else class="space-y-3">
 <div
 v-for="(item, index) in authSourceDefaults[
 authSource.source
 ].subscriptions"
 :key="`${authSource.source}-sub-${index}`"
 class="grid grid-cols-1 gap-3 rounded border border-line p-3 md:grid-cols-[1fr_160px_auto] "
 >
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.defaults.subscriptionGroup") }}
 </label>
 <Select
 v-model="item.group_id"
 class="default-sub-group-select"
 :options="defaultSubscriptionGroupOptions"
 :placeholder="
 t('admin.settings.defaults.subscriptionGroup')
 "
 >
 <template #selected="{ option }">
 <GroupBadge
 v-if="option"
 :name="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).label
 "
 :platform="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).platform
 "
 :subscription-type="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).subscriptionType
 "
 :rate-multiplier="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).rate
 "
 />
 <span v-else class="text-muted">
 {{
 t("admin.settings.defaults.subscriptionGroup")
 }}
 </span>
 </template>
 <template #option="{ option, selected }">
 <GroupOptionItem
 :name="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).label
 "
 :platform="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).platform
 "
 :subscription-type="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).subscriptionType
 "
 :rate-multiplier="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).rate
 "
 :description="
 (
 option as unknown as DefaultSubscriptionGroupOption
 ).description
 "
 :selected="selected"
 />
 </template>
 </Select>
 </div>
 <div>
 <label
 class="settings-sub-label"
 >
 {{
 t(
 "admin.settings.defaults.subscriptionValidityDays",
 )
 }}
 </label>
 <input
 v-model.number="item.validity_days"
 type="number"
 min="1"
 max="36500"
 class="input h-[42px]"
 />
 </div>
 <div class="settings-flex-row flex items-end">
 <button
 type="button"
 class="btn-glass-secondary w-full text-danger-text "
 @click="
 removeAuthSourceDefaultSubscription(
 authSource.source,
 index,
 )
 "
 >
 {{ t("common.delete") }}
 </button>
 </div>
 </div>
 </div>

 <!-- ★ 新增：auth source 平台限额覆盖区块 -->
 <div class="border-t border-line pt-4 ">
 <div class="mb-3">
 <label class="font-medium text-foreground ">
 {{ t("admin.settings.authSourceDefaults.platformQuotasOverride") }}
 </label>
 <p class="mt-0.5 text-xs text-muted ">
 {{ t("admin.settings.authSourceDefaults.platformQuotasOverrideHint") }}
 </p>
 </div>
 <div class="overflow-x-auto">
 <table class="min-w-full text-sm">
 <thead>
 <tr class="text-left text-xs text-muted ">
 <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.platform") }}</th>
 <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.daily") }}</th>
 <th class="pb-2 pr-4 font-medium">{{ t("admin.settings.platformQuota.weekly") }}</th>
 <th class="pb-2 font-medium">{{ t("admin.settings.platformQuota.monthly") }}</th>
 </tr>
 </thead>
 <tbody>
 <tr v-for="p in (['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro'] as const)" :key="`${authSource.source}-pq-${p}`" class="align-top">
 <td class="pr-4 py-1">
 <span class="font-mono text-xs text-foreground ">{{ p }}</span>
 </td>
 <td class="pr-4 py-1">
 <input
 v-model.number="authSourceDefaults[authSource.source].platform_quotas[p]!.daily"
 type="number"
 step="0.01"
 min="0"
 class="input h-8 w-28 text-sm"
 :placeholder="t('admin.settings.platformQuota.placeholder')"
 />
 </td>
 <td class="pr-4 py-1">
 <input
 v-model.number="authSourceDefaults[authSource.source].platform_quotas[p]!.weekly"
 type="number"
 step="0.01"
 min="0"
 class="input h-8 w-28 text-sm"
 :placeholder="t('admin.settings.platformQuota.placeholder')"
 />
 </td>
 <td class="py-1">
 <input
 v-model.number="authSourceDefaults[authSource.source].platform_quotas[p]!.monthly"
 type="number"
 step="0.01"
 min="0"
 class="input h-8 w-28 text-sm"
 :placeholder="t('admin.settings.platformQuota.placeholder')"
 />
 </td>
 </tr>
 </tbody>
 </table>
 </div>
 </div>
 <!-- /auth source 平台限额覆盖区块 -->
 </div>
 </div>
 </div>
 </div>
 </SettingsSection>
 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { computed } from "vue";
import type { AuthSourceType } from "@/api/admin/settings";
import type { AdminGroup } from "@/types";
import Select from "@/components/common/Select.vue";
import GroupBadge from "@/components/common/GroupBadge.vue";
import GroupOptionItem from "@/components/common/GroupOptionItem.vue";
import Toggle from "@/components/common/Toggle.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import SettingRow from "@/components/ui/SettingRow.vue";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  activeTab,
  subscriptionGroups,
  form,
  authSourceDefaults,
  authSourceDefaultsMeta,
} = settingsForm;

interface DefaultSubscriptionGroupOption {
  value: number;
  label: string;
  description: string | null;
  platform: AdminGroup["platform"];
  subscriptionType: AdminGroup["subscription_type"];
  rate: number;
  [key: string]: unknown;
}

const defaultSubscriptionGroupOptions = computed<
  DefaultSubscriptionGroupOption[]
>(() =>
  subscriptionGroups.value.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    platform: group.platform,
    subscriptionType: group.subscription_type,
    rate: group.rate_multiplier,
  })),
);

function findNextAvailableSubscriptionGroup(
  existingGroupIDs: number[],
): AdminGroup | undefined {
  const existing = new Set(existingGroupIDs);
  return subscriptionGroups.value.find((group) => !existing.has(group.id));
}

function addDefaultSubscription() {
  if (subscriptionGroups.value.length === 0) return;
  const candidate = findNextAvailableSubscriptionGroup(
    form.default_subscriptions.map((item) => item.group_id),
  );
  if (!candidate) return;
  form.default_subscriptions.push({
    group_id: candidate.id,
    validity_days: 30,
  });
}

function removeDefaultSubscription(index: number) {
  form.default_subscriptions.splice(index, 1);
}

function addAuthSourceDefaultSubscription(source: AuthSourceType) {
  if (subscriptionGroups.value.length === 0) return;
  const candidate = findNextAvailableSubscriptionGroup(
    authSourceDefaults[source].subscriptions.map((item) => item.group_id),
  );
  if (!candidate) return;
  authSourceDefaults[source].subscriptions.push({
    group_id: candidate.id,
    validity_days: 30,
  });
}

function removeAuthSourceDefaultSubscription(
  source: AuthSourceType,
  index: number,
) {
  authSourceDefaults[source].subscriptions.splice(index, 1);
}
</script>
