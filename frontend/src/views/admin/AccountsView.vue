<template>

 <AppLayout>
 <PageHeader class="acct-header" :title="t('admin.accounts.title')" :description="t('admin.accounts.description')">
 <template #actions>
 <AccountTableActions
 :loading="loading"
 @refresh="handleManualRefresh"
 @create="showCreate = true"
 >
 <template #after>
 <AccountsHeaderMenus
 :toolbar-menus="toolbarMenus"
 :sel-ids="selIds"
 @bulk-edit-header="openBulkEditFromHeader"
 @open-sync="showSync = true"
 @open-import="showImportData = true"
 @open-export="openExportDataDialog"
 @open-capacity-forecast="showCapacityForecast = true"
 @open-error-passthrough="showErrorPassthrough = true"
 @open-tls-routers="showTLSFingerprintRouters = true"
 @open-tls-profiles="showTLSFingerprintProfiles = true"
 />
 </template>
 </AccountTableActions>
 </template>
 </PageHeader>
 <TablePageLayout class="acct-layout">
 <template #filters>
 <AccountsFilterBar
 :list-state="listState"
 :selection="selection"
 :bulk-actions="bulkActions"
 :upstream-billing="upstreamBilling"
 :auto-refresh="autoRefresh"
 :toolbar-menus="toolbarMenus"
 :groups="groups"
 />
 </template>
 <template #table>
 <div ref="accountTableRef" class="flex min-h-0 flex-1 flex-col overflow-hidden">
 <DataTable
 ref="dataTableRef"
 :columns="cols"
 :data="accounts"
 :loading="loading"
 row-key="id"
 :server-side-sort="true"
 @sort="handleSort"
 default-sort-key="name"
 default-sort-order="asc"
 :sort-storage-key="ACCOUNT_SORT_STORAGE_KEY"
 :estimate-row-height="61"
 :overscan="5"
 :virtualize-threshold="50"
 >
 <template #header-select>
 <input
 type="checkbox"
 class="acct-checkbox"
 :checked="allVisibleSelected"
 @click.stop
 @change="toggleSelectAllVisible($event)"
 />
 </template>
 <template #cell-select="{ row }">
 <input type="checkbox" class="acct-checkbox" :checked="isSelected(row.id)" @change="toggleSel(row.id)" />
 </template>
 <template #cell-id="{ value }">
 <span class="acct-mono-muted">#{{ value }}</span>
 </template>
 <template #cell-name="{ row, value }">
 <div class="acct-name-wrap">
 <NameIdCell
 class="acct-name-id"
 :name="value"
 :id="row.id"
 :meta="accountGroupLabel(row)"
 :email="accountDisplayEmail(row)"
 :href="accountHomepageUrl(row)"
 :title="accountHomepageUrl(row) ? `${accountHomepageUrl(row)} · ${accountIdentityTitle(row)}` : accountIdentityTitle(row)"
 >
 <template #name>
 <span :class="{ 'acct-name-alert': hasCyberAlert(row) }">{{ value }}</span>
 </template>
 </NameIdCell>
 <button
 v-if="isOpenAIOAuthAccount(row) && row.cyber_count != null"
 type="button"
 class="acct-cyber-badge"
 :class="{ 'is-alert': hasCyberAlert(row) }"
 :title="row.cyber_latest_at ? t('admin.accounts.cyber.latest', { time: formatDateTime(row.cyber_latest_at) }) : t('admin.accounts.cyber.viewDetail')"
 @click="openCyberEvents(row)"
 >
 <Icon name="shield" size="xs" />
 <span>{{ row.cyber_count }}</span>
 </button>
 </div>
 </template>
 <template #cell-notes="{ value }">
 <span v-if="value" :title="value" class="acct-note-text">{{ value }}</span>
 <span v-else class="acct-muted-text">-</span>
 </template>
 <template #cell-platform="{ row }">
 <div class="acct-platform-cell">
 <span class="acct-platform-tile" :style="{ background: platformTileBackground(row.platform) }">
 <PlatformIcon :platform="row.platform" size="xs" />
 </span>
 <span class="acct-platform-name">{{ platformLabel(row.platform) }}</span>
 </div>
 </template>
 <template #cell-platform_type="{ row }">
 <div class="acct-type-cell">
 <PlatformTypeBadge :platform="row.platform" :type="row.type"
 :auth-mode="getOpenAIAuthMode(row)"
 :plan-type="getAccountPlanType(row)"
 :privacy-mode="row.extra?.privacy_mode || row.parent_privacy_mode"
 :subscription-expires-at="row.credentials?.subscription_expires_at || row.parent_subscription_expires_at" />
 <span
 v-if="getAntigravityTierLabel(row)"
 :class="['tag', getAntigravityTierClass(row)]"
 >
 {{ getAntigravityTierLabel(row) }}
 </span>
 <div
 v-if="getOpenAICompactMeta(row)"
 :class="['acct-compact-meta', getOpenAICompactMeta(row)?.className]"
 :title="getOpenAICompactTitle(row)"
 >
 <span :class="['acct-compact-dot', getOpenAICompactMeta(row)?.dotClass]" />
 <span>{{ getOpenAICompactMeta(row)?.label }}</span>
 </div>
 </div>
 </template>
 <template #cell-capacity="{ row }">
 <AccountCapacityCell :account="row" />
 </template>
 <template #cell-status="{ row }">
 <AccountStatusIndicator :account="row" @show-temp-unsched="handleShowTempUnsched" />
 </template>
 <template #cell-schedulable="{ row }">
 <ToggleSwitch
 size="compact"
 :model-value="!!row.schedulable"
 :disabled="togglingSchedulable === row.id"
 :aria-label="row.schedulable ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled')"
 @update:model-value="handleToggleSchedulable(row)"
 />
 </template>
 <template #cell-today_stats="{ row }">
 <AccountTodayStatsCell
 :stats="todayStatsByAccountId[String(row.id)] ?? null"
 :loading="todayStatsLoading"
 :error="todayStatsError"
 />
 </template>
 <template #cell-groups="{ row }">
 <AccountGroupsCell :groups="row.groups" :max-display="4" />
 </template>
 <template #header-usage="{ column }">
 <div class="flex items-center">
 <span>{{ column.label }}</span>
 <HelpTooltip :content="t('admin.accounts.usageWindowsHint')" width-class="w-72" />
 </div>
 </template>
 <template #cell-usage="{ row }">
 <AccountUsageCell
 compact
 :account="row"
 :today-stats="todayStatsByAccountId[String(row.id)] ?? null"
 :today-stats-loading="todayStatsLoading"
 :manual-refresh-token="usageManualRefreshToken"
 :batched-usage="usageBatchByAccountId[String(row.id)] ?? null"
 :batched-usage-error="usageBatchErrorByAccountId[String(row.id)] ?? null"
 :batched-usage-loading="usageBatchLoadingByAccountId[String(row.id)] === true"
 :request-batched-usage="isDesktopViewport ? queueBatchedUsage : null"
 @account-updated="handleAccountUpdated"
 @usage-loaded="handleAccountUsageLoaded(row.id, $event)"
 />
 </template>
 <template #cell-proxy="{ row }">
 <div class="acct-proxy-cell">
 <div v-if="row.proxy" class="flex items-center gap-2">
 <span class="acct-proxy-name">{{ row.proxy.name }}</span>
 <span v-if="row.proxy.country_code" class="acct-muted-text">({{ row.proxy.country_code }})</span>
 </div>
 <span v-else class="acct-muted-text">-</span>
 <div v-if="row.proxy && row.proxy.expires_at" class="flex items-center gap-2">
 <span class="acct-muted-text">{{ formatDateTime(row.proxy.expires_at) }}</span>
 <span :class="proxyExpiryBadge(row.proxy)">{{ proxyExpiryText(row.proxy) }}</span>
 </div>
 <div v-if="row.proxy_fallback_origin_id" class="flex items-center gap-1">
 <span class="tag tag-warning" :title="t('admin.accounts.fallbackActiveTip', { origin: row.proxy_fallback_origin_name })">
 {{ t('admin.accounts.fallbackActive') }}
 </span>
 <button type="button" class="acct-inline-btn" @click="onRevertFallback(row)">{{ t('admin.accounts.revertProxy') }}</button>
 </div>
 </div>
 </template>
 <template #cell-rate_multiplier="{ row }">
 <span class="acct-rate-cell">
 <span>{{ formatMultiplier(row.rate_multiplier ?? 1) }}x</span>
 <span
 v-if="row.extra?.upstream_billing_rate_sync_enabled === true"
 class="acct-rate-sync"
 :aria-label="t('admin.accounts.upstreamBilling.syncedRateTooltip')"
 :title="t('admin.accounts.upstreamBilling.syncedRateTooltip')"
 data-testid="account-rate-sync-indicator"
 >
 <Icon name="sync" size="xs" />
 </span>
 </span>
 </template>
 <template #header-upstream_billing_rate="{ column }">
 <div class="flex items-center gap-1">
 <span>{{ column.label }}</span>
 <span @click.stop>
 <HelpTooltip :content="t('admin.accounts.upstreamBilling.trustWarning')" width-class="w-80" />
 </span>
 </div>
 </template>
 <template #cell-upstream_billing_rate="{ row }">
 <UpstreamBillingRateCell
 :account="row"
 :global-probe-enabled="upstreamBillingProbeGloballyEnabled"
 :now="upstreamBillingNow"
 :probing="probingUpstreamBilling.has(row.id)"
 @probe="handleProbeUpstreamBilling(row)"
 />
 </template>
 <template #cell-priority="{ value }">
 <span class="acct-priority-cell num">{{ value }}</span>
 </template>
 <template #header-scheduler_score="{ column }">
 <div class="flex items-center">
 <span>{{ column.label }}</span>
 <HelpTooltip :content="t('admin.accounts.schedulerScore.hint')" width-class="w-80" />
 </div>
 </template>
 <template #cell-scheduler_score="{ row }">
 <div v-if="getSchedulerScoreRows(row).length" class="acct-score-cell">
 <div
 v-for="score in getSchedulerScoreRows(row)"
 :key="String(score.group_id)"
 class="acct-score-row"
 :title="`${formatSchedulerScoreGroup(score)} / ${formatSchedulerScore(score.base_score)} / ${formatStickySchedulerScore(score)}`"
 >
 <span class="acct-score-group">{{ formatSchedulerScoreGroup(score) }}</span>
 <span class="acct-muted-text">/</span>
 <span>{{ formatSchedulerScore(score.base_score) }}</span>
 <span class="acct-muted-text">/</span>
 <span class="text-accent">{{ formatStickySchedulerScore(score) }}</span>
 </div>
 </div>
 <span v-else class="acct-muted-text">-</span>
 </template>
 <template #cell-last_used_at="{ value }">
 <span class="acct-last-used">{{ formatRelativeTime(value) }}</span>
 </template>
 <template #cell-created_at="{ value }">
 <span class="acct-last-used">{{ formatDateTime(value) }}</span>
 </template>
 <template #cell-expires_at="{ row, value }">
 <div class="acct-expires-cell">
 <span class="acct-last-used">{{ formatExpiresAt(value) }}</span>
 <div v-if="isExpired(value) || (row.auto_pause_on_expired && value)" class="flex items-center gap-1">
 <span v-if="isExpired(value)" class="tag tag-warning">{{ t('admin.accounts.expired') }}</span>
 <span v-if="row.auto_pause_on_expired && value" class="tag tag-success">{{ t('admin.accounts.autoPauseOnExpired') }}</span>
 </div>
 </div>
 </template>
 <template #cell-actions="{ row }">
 <div class="acct-actions-cell">
 <button type="button" class="icon-btn" :title="t('common.edit')" :aria-label="t('common.edit')" @click="handleEdit(row)">
 <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z" /></svg>
 </button>
 <button type="button" class="icon-btn" :title="t('common.more')" :aria-label="t('common.more')" @click="openMenu(row, $event)">
 <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h.01M12 12h.01M19 12h.01" /></svg>
 </button>
 </div>
 </template>
 </DataTable>
 </div>
 <div v-if="pagination.total > 0" class="acct-table-footer">
 <Pagination
 class="acct-pagination"
 :page="pagination.page"
 :total="pagination.total"
 :page-size="pagination.page_size"
 :show-page-size-selector="false"
 @update:page="handlePageChange"
 @update:pageSize="handlePageSizeChange"
 />
 </div>
 </template>
 </TablePageLayout>
 <Fab class="acct-fab" data-tour="accounts-create-btn" :label="t('admin.accounts.createAccount')" @click="showCreate = true">
 <Icon name="plus" size="md" />
 {{ t('admin.accounts.createAccount') }}
 </Fab>
 <CreateAccountModal :show="showCreate" :proxies="proxies" :groups="groups" @close="showCreate = false" @created="reload" />
 <EditAccountModal :show="showEdit" :account="edAcc" :proxies="proxies" :groups="groups" @close="showEdit = false" @updated="handleAccountUpdated" />
 <ReAuthAccountModal
 :show="showReAuth"
 :account="reAuthAcc"
 @close="closeReAuthModal"
 @reauthorized="handleAccountUpdated"
 @refresh="reload"
 @open-editor="handleOpenEditorFromReAuth"
 />
 <AccountTestModal :show="showTest" :account="testingAcc" @close="closeTestModal" />
 <AccountStatsModal :show="showStats" :account="statsAcc" @close="closeStatsModal" />
 <ScheduledTestsPanel :show="showSchedulePanel" :account-id="scheduleAcc?.id ?? null" :model-options="scheduleModelOptions" @close="closeSchedulePanel" />
 <AccountActionMenu :show="menu.show" :account="menu.acc" :position="menu.pos" @close="menu.show = false" @test="handleTest" @stats="handleViewStats" @schedule="handleSchedule" @duplicate="handleDuplicateAccount" @reauth="handleReAuth" @refresh-token="handleRefresh" @recover-state="handleRecoverState" @reset-quota="handleResetQuota" @inspect-device-profile="handleInspectDeviceProfile" @reset-device-profile="handleResetDeviceProfile" @set-privacy="handleSetPrivacy" @create-spark-shadow="handleCreateSparkShadow" @delete="handleDelete" />
 <SyncFromCrsModal :show="showSync" @close="showSync = false" @synced="reload" />
 <ImportDataModal :show="showImportData" @close="showImportData = false" @imported="handleDataImported" />
 <BulkEditAccountModal
 :show="showBulkEdit"
 :account-ids="selIds"
 :selected-platforms="selPlatforms"
 :selected-types="selTypes"
 :target="bulkEditTarget ?? undefined"
 :proxies="proxies"
 :groups="groups"
 @close="showBulkEdit = false"
 @updated="handleBulkUpdated"
 />
 <TempUnschedStatusModal :show="showTempUnsched" :account="tempUnschedAcc" @close="showTempUnsched = false" @reset="handleTempUnschedReset" />
 <ConfirmDialog :show="showDeleteDialog" :title="t('admin.accounts.deleteAccount')" :message="t('admin.accounts.deleteConfirm', { name: deletingAcc?.name })" :confirm-text="t('common.delete')" :cancel-text="t('common.cancel')" :danger="true" @confirm="confirmDelete" @cancel="showDeleteDialog = false" />
 <ConfirmDialog :show="showResetDeviceProfileDialog" :title="t('admin.accounts.resetDeviceProfile')" :message="t('admin.accounts.resetDeviceProfileConfirm', { name: resettingDeviceAcc?.name })" :confirm-text="t('common.confirm')" :cancel-text="t('common.cancel')" :danger="true" :confirming="resettingDeviceProfile" @confirm="confirmResetDeviceProfile" @cancel="showResetDeviceProfileDialog = false" />
 <DeviceProfileInspectModal :show="showDeviceProfileInspect" :account="inspectingDeviceAcc" @close="closeDeviceProfileInspect" />
 <ConfirmDialog :show="showCreateShadowDialog" :title="t('admin.accounts.createSparkShadow')" :message="t('admin.accounts.createSparkShadowConfirm', { name: creatingShadowAcc?.name })" @confirm="confirmCreateSparkShadow" @cancel="showCreateShadowDialog = false" />
 <ConfirmDialog :show="showExportDataDialog" :title="t('admin.accounts.dataExport')" :message="t('admin.accounts.dataExportConfirmMessage')" :confirm-text="t('admin.accounts.dataExportConfirm')" :cancel-text="t('common.cancel')" @confirm="handleExportData" @cancel="showExportDataDialog = false">
 <label class="acct-export-option">
 <input type="checkbox" class="acct-checkbox" v-model="includeProxyOnExport" />
 <span>{{ t('admin.accounts.dataExportIncludeProxies') }}</span>
 </label>
 </ConfirmDialog>
 <ErrorPassthroughRulesModal :show="showErrorPassthrough" @close="showErrorPassthrough = false" />
 <TLSFingerprintProfilesModal :show="showTLSFingerprintProfiles" @close="showTLSFingerprintProfiles = false" />
 <TLSFingerprintRoutersModal :show="showTLSFingerprintRouters" @close="showTLSFingerprintRouters = false" />
 <PlatformCapacityDialog
 v-if="showCapacityForecast"
 :show="showCapacityForecast"
 @close="showCapacityForecast = false"
 />
 <AccountCyberEventsModal
 v-if="showCyberEvents && cyberEventsAcc"
 :show="showCyberEvents"
 :account="cyberEventsAcc"
 @close="closeCyberEvents"
 @open-detail="openCyberErrorDetail"
 />
 <OpsErrorDetailModal
 v-if="showCyberErrorDetail"
 :show="showCyberErrorDetail"
 :error-id="cyberErrorID"
 error-type="request"
 @update:show="showCyberErrorDetail = $event"
 />
 <TotpStepUpDialog :controller="accountExportStepUp" />
 </AppLayout>
</template>
<script setup lang="ts">
// AccountsView.vue (frontend-health-cleanup 6.4 split): thin shell wiring the
// admin accounts list-page layout (PageHeader / FilterBar / TablePageLayout /
// DataTable / pagination / dialogs) up to a set of composables under
// src/views/admin/accounts/*.ts and two extracted subcomponents
// (AccountsHeaderMenus.vue, AccountsFilterBar.vue). See those files for the
// actual state/behavior — this file only holds template-only glue that reads
// from multiple composables at once, plus DataTable's cell/dialog markup
// (kept here per the split brief).
import { computed, onMounted, onUnmounted, watch, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { CreateAccountModal, EditAccountModal, BulkEditAccountModal, SyncFromCrsModal, TempUnschedStatusModal } from '@/components/account'
import AccountTableActions from '@/components/admin/account/AccountTableActions.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import DeviceProfileInspectModal from '@/components/admin/account/DeviceProfileInspectModal.vue'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'
import ReAuthAccountModal from '@/components/admin/account/ReAuthAccountModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import ScheduledTestsPanel from '@/components/admin/account/ScheduledTestsPanel.vue'
import NameIdCell from '@/components/common/cells/NameIdCell.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountUsageCell from '@/components/account/AccountUsageCell.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import UpstreamBillingRateCell from '@/components/account/UpstreamBillingRateCell.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import Fab from '@/components/ui/Fab.vue'
import ErrorPassthroughRulesModal from '@/components/admin/ErrorPassthroughRulesModal.vue'
import TLSFingerprintProfilesModal from '@/components/admin/TLSFingerprintProfilesModal.vue'
import TLSFingerprintRoutersModal from '@/components/admin/TLSFingerprintRoutersModal.vue'
import PlatformCapacityDialog from '@/components/admin/account/PlatformCapacityDialog.vue'
const AccountCyberEventsModal = defineAsyncComponent(() => import('@/components/admin/account/AccountCyberEventsModal.vue'))
const OpsErrorDetailModal = defineAsyncComponent(() => import('@/views/admin/ops/components/OpsErrorDetailModal.vue'))
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { platformTileBackground, platformLabel } from '@/utils/platformTile'
import type { Account, AccountSchedulerGroupScore, Proxy as AccountProxy, AdminGroup } from '@/types'
import {
  getAccountPlanType,
  getOpenAIAuthMode,
  getAntigravityTierFromRow,
  getAntigravityTierClass,
  accountDisplayEmail,
  isOpenAIOAuthAccount,
  hasCyberAlert,
  accountHomepageUrl,
  getOpenAICompactState,
  formatSchedulerScore,
  formatStickySchedulerScore,
  getSchedulerScoreRows
} from './accountRowHelpers'
import AccountsHeaderMenus from './accounts/AccountsHeaderMenus.vue'
import AccountsFilterBar from './accounts/AccountsFilterBar.vue'
import { useAccountUsageBatch } from './accounts/useAccountUsageBatch'
import { useAccountRowState } from './accounts/useAccountRowState'
import { useAccountToolbarMenus } from './accounts/useAccountToolbarMenus'
import { useAccountListState, ACCOUNT_SORT_STORAGE_KEY } from './accounts/useAccountListState'
import { useAccountSelection } from './accounts/useAccountSelection'
import { useAccountBulkActions } from './accounts/useAccountBulkActions'
import { useUpstreamBillingRates } from './accounts/useUpstreamBillingRates'
import { useAccountAutoRefresh } from './accounts/useAccountAutoRefresh'
import { useAccountRowActions } from './accounts/useAccountRowActions'
import { ref } from 'vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const proxies = ref<AccountProxy[]>([])
const groups = ref<AdminGroup[]>([])

// ---------------------------------------------------------------------------
// Composable wiring — see the "Key Technical Concepts" instantiation order:
// lazy ref-boxes break the circular dependency between listState/selection
// (needs clearSelection/resetCaches before they exist) and autoRefresh/
// upstreamBilling/rowActions (need patchAccountInList before it exists).
// ---------------------------------------------------------------------------
let clearSelectionRef: () => void = () => {}
let resetCachesRef: () => void = () => {}
let patchAccountInListRef: (account: Account) => void = () => {}

const usageBatch = useAccountUsageBatch()
const rowState = useAccountRowState()
const toolbarMenus = useAccountToolbarMenus()

const listState = useAccountListState({
  isSimpleMode: computed(() => authStore.isSimpleMode),
  clearSelection: () => clearSelectionRef(),
  bumpUsageManualRefreshToken: () => { usageBatch.usageManualRefreshToken.value += 1 },
  resetCaches: () => resetCachesRef(),
  t
})

const selection = useAccountSelection({
  accounts: listState.accounts,
  appStore,
  t,
  pagination: listState.pagination,
  buildBulkEditFilterSnapshot: listState.buildBulkEditFilterSnapshot
})
clearSelectionRef = selection.clearSelection

const bulkActions = useAccountBulkActions({
  t,
  appStore,
  accounts: listState.accounts,
  selIds: selection.selIds,
  selPlatforms: selection.selPlatforms,
  selTypes: selection.selTypes,
  setSelectedIds: selection.setSelectedIds,
  clearSelection: selection.clearSelection,
  load: listState.load,
  reload: listState.reload,
  buildBulkEditFilterSnapshot: listState.buildBulkEditFilterSnapshot,
  buildAccountQueryFilters: listState.buildAccountQueryFilters,
  showBulkEdit: rowState.showBulkEdit,
  bulkEditTarget: rowState.bulkEditTarget,
  showImportData: rowState.showImportData,
  includeProxyOnExport: rowState.includeProxyOnExport,
  showExportDataDialog: rowState.showExportDataDialog
})

// Relays a refreshed row into any dialog/menu state currently holding it —
// injected into both useUpstreamBillingRates and useAccountAutoRefresh.
const syncAccountRefs = (nextAccount: Account) => {
  if (rowState.edAcc.value?.id === nextAccount.id) rowState.edAcc.value = nextAccount
  if (rowState.reAuthAcc.value?.id === nextAccount.id) rowState.reAuthAcc.value = nextAccount
  if (rowState.tempUnschedAcc.value?.id === nextAccount.id) rowState.tempUnschedAcc.value = nextAccount
  if (rowState.deletingAcc.value?.id === nextAccount.id) rowState.deletingAcc.value = nextAccount
  if (rowState.resettingDeviceAcc.value?.id === nextAccount.id) rowState.resettingDeviceAcc.value = nextAccount
  if (rowState.inspectingDeviceAcc.value?.id === nextAccount.id) rowState.inspectingDeviceAcc.value = nextAccount
  if (rowState.menu.acc?.id === nextAccount.id) rowState.menu.acc = nextAccount
}

const upstreamBilling = useUpstreamBillingRates({
  t,
  appStore,
  accounts: listState.accounts,
  pagination: listState.pagination,
  params: listState.params,
  sortState: listState.sortState,
  loading: listState.loading,
  syncAccountListDerivedParams: listState.syncAccountListDerivedParams,
  syncAccountRefs,
  load: listState.load,
  isAnyModalOpen: rowState.isAnyModalOpen,
  menu: rowState.menu,
  showAccountToolsDropdown: toolbarMenus.showAccountToolsDropdown,
  showAutoRefreshDropdown: toolbarMenus.showAutoRefreshDropdown,
  selIds: selection.selIds,
  patchAccountInList: (account) => patchAccountInListRef(account)
})

const autoRefresh = useAccountAutoRefresh({
  t,
  accounts: listState.accounts,
  pagination: listState.pagination,
  params: listState.params,
  loading: listState.loading,
  hasPendingListSync: listState.hasPendingListSync,
  syncAccountListDerivedParams: listState.syncAccountListDerivedParams,
  refreshTodayStatsBatch: listState.refreshTodayStatsBatch,
  syncAccountRefs,
  upstreamBillingNow: upstreamBilling.upstreamBillingNow,
  loadUpstreamBillingProbeGlobalState: upstreamBilling.loadUpstreamBillingProbeGlobalState,
  load: listState.load,
  usageManualRefreshToken: usageBatch.usageManualRefreshToken,
  isAnyModalOpen: rowState.isAnyModalOpen,
  menu: rowState.menu,
  showAccountToolsDropdown: toolbarMenus.showAccountToolsDropdown,
  showAutoRefreshDropdown: toolbarMenus.showAutoRefreshDropdown
})

const rowActions = useAccountRowActions({
  t,
  appStore,
  accounts: listState.accounts,
  pagination: listState.pagination,
  hasPendingListSync: listState.hasPendingListSync,
  buildAccountQueryFilters: listState.buildAccountQueryFilters,
  removeSelectedAccounts: selection.removeSelectedAccounts,
  reload: listState.reload,
  menu: rowState.menu,
  syncAccountRefs,
  enterAutoRefreshSilentWindow: autoRefresh.enterAutoRefreshSilentWindow,
  updateSchedulableInList: bulkActions.updateSchedulableInList,
  showEdit: rowState.showEdit,
  edAcc: rowState.edAcc,
  showTest: rowState.showTest,
  testingAcc: rowState.testingAcc,
  showStats: rowState.showStats,
  statsAcc: rowState.statsAcc,
  showReAuth: rowState.showReAuth,
  reAuthAcc: rowState.reAuthAcc,
  showSchedulePanel: rowState.showSchedulePanel,
  scheduleAcc: rowState.scheduleAcc,
  scheduleModelOptions: rowState.scheduleModelOptions,
  showDeviceProfileInspect: rowState.showDeviceProfileInspect,
  inspectingDeviceAcc: rowState.inspectingDeviceAcc,
  showResetDeviceProfileDialog: rowState.showResetDeviceProfileDialog,
  resettingDeviceAcc: rowState.resettingDeviceAcc,
  resettingDeviceProfile: rowState.resettingDeviceProfile,
  showCreateShadowDialog: rowState.showCreateShadowDialog,
  creatingShadowAcc: rowState.creatingShadowAcc,
  showDeleteDialog: rowState.showDeleteDialog,
  deletingAcc: rowState.deletingAcc,
  togglingSchedulable: rowState.togglingSchedulable,
  showTempUnsched: rowState.showTempUnsched,
  tempUnschedAcc: rowState.tempUnschedAcc
})
patchAccountInListRef = rowActions.patchAccountInList
resetCachesRef = () => { autoRefresh.resetETag(); upstreamBilling.resetETag() }

// ---------------------------------------------------------------------------
// Bare local bindings for the template — destructured straight out of the
// composables above so template expressions read/assign the exact identifier
// names the pre-split template used (e.g. `showCreate.value = true` instead
// of `rowState.showCreate.value = true`). These are plain object-reference
// copies (Refs/reactive objects are objects, not primitives), so reactivity
// and the composables' own internal state are entirely unaffected.
// ---------------------------------------------------------------------------
const { handleManualRefresh } = autoRefresh
const { accountExportStepUp, handleBulkUpdated, handleDataImported, handleExportData, openExportDataDialog } = bulkActions
const { accounts, cols, handlePageChange, handlePageSizeChange, handleSort, loading, pagination, reload, todayStatsByAccountId, todayStatsError, todayStatsLoading } = listState
const {
  accountGroupLabel,
  accountIdentityTitle,
  closeDeviceProfileInspect,
  closeReAuthModal,
  closeSchedulePanel,
  closeStatsModal,
  closeTestModal,
  confirmCreateSparkShadow,
  confirmDelete,
  confirmResetDeviceProfile,
  formatExpiresAt,
  handleAccountUpdated,
  handleCreateSparkShadow,
  handleDelete,
  handleDuplicateAccount,
  handleEdit,
  handleInspectDeviceProfile,
  handleOpenEditorFromReAuth,
  handleReAuth,
  handleRecoverState,
  handleRefresh,
  handleResetDeviceProfile,
  handleResetQuota,
  handleSchedule,
  handleSetPrivacy,
  handleShowTempUnsched,
  handleTempUnschedReset,
  handleTest,
  handleToggleSchedulable,
  handleViewStats,
  isExpired,
  onRevertFallback,
  openMenu,
  proxyExpiryBadge,
  proxyExpiryText
} = rowActions
const {
  bulkEditTarget,
  creatingShadowAcc,
  cyberErrorID,
  cyberEventsAcc,
  deletingAcc,
  edAcc,
  includeProxyOnExport,
  inspectingDeviceAcc,
  menu,
  reAuthAcc,
  resettingDeviceAcc,
  resettingDeviceProfile,
  scheduleAcc,
  scheduleModelOptions,
  showBulkEdit,
  showCapacityForecast,
  showCreate,
  showCreateShadowDialog,
  showCyberErrorDetail,
  showCyberEvents,
  showDeleteDialog,
  showDeviceProfileInspect,
  showEdit,
  showErrorPassthrough,
  showExportDataDialog,
  showImportData,
  showReAuth,
  showResetDeviceProfileDialog,
  showSchedulePanel,
  showStats,
  showSync,
  showTLSFingerprintProfiles,
  showTLSFingerprintRouters,
  showTempUnsched,
  showTest,
  statsAcc,
  tempUnschedAcc,
  testingAcc,
  togglingSchedulable
} = rowState
const { accountTableRef, allVisibleSelected, dataTableRef, isSelected, selIds, selPlatforms, selTypes, toggleSel, toggleSelectAllVisible } = selection
const { handleProbeUpstreamBilling, probingUpstreamBilling, upstreamBillingNow, upstreamBillingProbeGloballyEnabled } = upstreamBilling
const { handleAccountUsageLoaded, isDesktopViewport, queueBatchedUsage, usageBatchByAccountId, usageBatchErrorByAccountId, usageBatchLoadingByAccountId, usageManualRefreshToken } = usageBatch

// ---------------------------------------------------------------------------
// Small view-local helpers/handlers that read from more than one composable
// (or are too small to warrant their own file).
// ---------------------------------------------------------------------------
const formatSchedulerScoreGroup = (score: AccountSchedulerGroupScore): string => {
  if ('group_name' in score && score.group_name) return score.group_name
  if ('group_id' in score && score.group_id != null) return `#${score.group_id}`
  return t('admin.accounts.schedulerScore.ungrouped')
}

function getAntigravityTierLabel(row: any): string | null {
  const tier = getAntigravityTierFromRow(row)
  switch (tier) {
    case 'free-tier': return t('admin.accounts.tier.free')
    case 'g1-pro-tier': return t('admin.accounts.tier.pro')
    case 'g1-ultra-tier': return t('admin.accounts.tier.ultra')
    default: return null
  }
}

const openCyberEvents = (account: Account) => {
  if (!isOpenAIOAuthAccount(account)) return
  rowState.cyberEventsAcc.value = account
  rowState.showCyberEvents.value = true
}
const closeCyberEvents = () => {
  rowState.showCyberEvents.value = false
  rowState.cyberEventsAcc.value = null
}
const openCyberErrorDetail = (errorID: number) => {
  closeCyberEvents()
  rowState.cyberErrorID.value = errorID
  rowState.showCyberErrorDetail.value = true
}

function getOpenAICompactMeta(row: any): { label: string; className: string; dotClass: string } | null {
  const state = getOpenAICompactState(row)
  if (!state) return null
  switch (state) {
    case 'active':
      return {
        label: t('admin.accounts.openai.compactSupported'),
        className: 'dash-tone-success',
        dotClass: 'bg-success-500 shadow-[0_0_0_2px_rgba(16,185,129,0.14)]'
      }
    case 'blocked':
      return {
        label: t('admin.accounts.openai.compactUnsupported'),
        className: 'dash-tone-danger',
        dotClass: 'bg-danger-500 shadow-[0_0_0_2px_rgba(244,63,94,0.14)]'
      }
    case 'auto':
      return {
        label: t('admin.accounts.openai.compactAuto'),
        className: 'dash-tone-muted',
        dotClass: 'bg-surface-3'
      }
  }
}

function getOpenAICompactTitle(row: any): string {
  const extra = row.extra as Record<string, unknown> | undefined
  const checkedAt = typeof extra?.openai_compact_checked_at === 'string' ? extra.openai_compact_checked_at : ''
  const label = getOpenAICompactMeta(row)?.label || ''
  if (!checkedAt) return label
  return `${label} | ${t('admin.accounts.openai.compactLastChecked')}: ${formatDateTime(new Date(checkedAt))}`
}

const openBulkEditFromHeader = () => {
  if (selection.selIds.value.length > 0) {
    bulkActions.openBulkEditSelected()
    return
  }
  void bulkActions.openBulkEditFiltered()
}

// Keeps the usage-batch caches pruned to whatever rows are currently loaded.
watch(listState.accounts, (rows) => {
  usageBatch.pruneUsageBatchToVisibleAccounts(rows)
})

// 表格滚动时关闭行操作菜单，并让顶部工具菜单继续贴紧触发按钮。
const handleScroll = () => {
  rowState.menu.show = false
  if (toolbarMenus.showAccountToolsDropdown.value) toolbarMenus.updateAccountToolsDropdownPosition()
}

const handleViewportResize = () => {
  if (toolbarMenus.showAccountToolsDropdown.value) toolbarMenus.updateAccountToolsDropdownPosition()
}

// 点击外部关闭顶部下拉菜单
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (toolbarMenus.accountToolsDropdownRef.value && !toolbarMenus.accountToolsDropdownRef.value.contains(target)) {
    toolbarMenus.showAccountToolsDropdown.value = false
  }
  if (toolbarMenus.autoRefreshDropdownRef.value && !toolbarMenus.autoRefreshDropdownRef.value.contains(target)) {
    toolbarMenus.showAutoRefreshDropdown.value = false
  }
  if (toolbarMenus.importExportDropdownRef.value && !toolbarMenus.importExportDropdownRef.value.contains(target)) {
    toolbarMenus.showImportExportDropdown.value = false
  }
  if (toolbarMenus.columnsDropdownRef.value && !toolbarMenus.columnsDropdownRef.value.contains(target)) {
    toolbarMenus.showColumnsDropdown.value = false
  }
}

onMounted(async () => {
  usageBatch.setupViewportListener()

  listState.load()
  upstreamBilling.loadUpstreamBillingProbeGlobalState()
  const [proxiesResult, groupsResult] = await Promise.allSettled([
    adminAPI.proxies.getAll(),
    adminAPI.groups.getAll()
  ])
  if (proxiesResult.status === 'fulfilled') {
    proxies.value = proxiesResult.value
  } else {
    console.error('Failed to load proxies:', proxiesResult.reason)
  }
  if (groupsResult.status === 'fulfilled') {
    groups.value = groupsResult.value
  } else {
    console.error('Failed to load groups:', groupsResult.reason)
  }
  window.addEventListener('scroll', handleScroll, true)
  window.addEventListener('resize', handleViewportResize)
  document.addEventListener('click', handleClickOutside)

  autoRefresh.initAutoRefreshTicker()
})

onUnmounted(() => {
  upstreamBilling.teardown()
  usageBatch.teardownViewportListener()
  window.removeEventListener('scroll', handleScroll, true)
  window.removeEventListener('resize', handleViewportResize)
  document.removeEventListener('click', handleClickOutside)
})
</script>


<style scoped>
/* ---------------------------------------------------------------------------
 * 04 账号管理 — prototype geometry
 * content column gap 14 · summary chips 5×gap 10 · filter row gap 8
 * table card radius 14 · header 42 · rows 60 · footer 10/16
 * ------------------------------------------------------------------------- */
.acct-header {
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.acct-header :deep(.ui-page-header-main) {
  min-width: 0;
  flex: 1 1 auto;
}

.acct-header :deep(.ui-page-header-actions) {
  flex-wrap: wrap;
  row-gap: 8px;
}

@media (max-width: 640px) {
  .acct-header {
    align-items: flex-start;
  }

  .acct-header :deep(.ui-page-header-main) {
    flex: 1 1 100%;
  }

  .acct-header :deep(.ui-page-header-actions) {
    flex: 1 1 100%;
    justify-content: flex-start;
  }
}

.acct-layout {
  gap: 14px;
}

/* Layout sits below the 72px compact page header (h1 35 + desc 19 + gap 4 + margin 14),
   so subtract it from TablePageLayout's viewport rule: card bottom lands at 100vh − 24. */
@media (min-width: 1024px) {
  .acct-layout {
    height: calc(100vh - 170px);
  }
}


/* ---------- Table card ---------- */
.acct-layout :deep(.table-scroll-container) {
  border-radius: var(--radius-card);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  box-shadow: var(--shadow);
}

/* Fixed layout + explicit per-column widths so the 11 default columns fit the 1440 content width
 * without a horizontal scrollbar (matches prototype 04). */
.acct-layout :deep(.table-scroll-container table) {
  table-layout: fixed;
  width: 100%;
}

/* Column widths = prototype target + 12px (td/th now use 6px symmetric padding in place of the
 * prototype grid's 12px column gap). Name/usage columns intentionally have no explicit width so
 * they absorb the table's remaining space, matching the prototype grid's fr-tracks. */
.acct-layout :deep(.acct-col-select) { width: 44px; }
.acct-layout :deep(.acct-col-id) { width: 110px; }
.acct-layout :deep(.acct-col-platform) { width: 112px; }
.acct-layout :deep(.acct-col-platform_type) { width: 92px; }
.acct-layout :deep(.acct-col-capacity) { width: 76px; }
.acct-layout :deep(.acct-col-status) { width: 96px; }
.acct-layout :deep(.acct-col-schedulable) { width: 72px; }
.acct-layout :deep(.acct-col-today_stats) { width: 132px; }
.acct-layout :deep(.acct-col-groups) { width: 130px; }
.acct-layout :deep(.acct-col-priority) { width: 64px; }
.acct-layout :deep(.acct-col-last_used_at) { width: 100px; }
.acct-layout :deep(.acct-col-proxy) { width: 120px; }
.acct-layout :deep(.acct-col-scheduler_score) { width: 130px; }
.acct-layout :deep(.acct-col-rate_multiplier) { width: 90px; }
.acct-layout :deep(.acct-col-upstream_billing_rate) { width: 100px; }
.acct-layout :deep(.acct-col-created_at) { width: 120px; }
.acct-layout :deep(.acct-col-expires_at) { width: 120px; }
.acct-layout :deep(.acct-col-notes) { width: 140px; }
.acct-layout :deep(.acct-col-actions) { width: 76px; }

/* 43px header (42 content + 1px bottom border) · 11px/600 uppercase · surface-secondary 45% */
.acct-layout :deep(.table-scroll-container th) {
  height: 42px;
  padding: 0 6px;
  font-size: var(--fs-11);
  font-weight: var(--fw-semibold);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
  background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
  border-bottom: 1px solid var(--border);
  box-sizing: border-box;
  overflow: hidden;
  text-overflow: ellipsis;
}

.acct-layout :deep(.table-scroll-container th:first-child) {
  padding-left: 16px;
}

.acct-layout :deep(.table-scroll-container th:last-child) {
  padding-right: 16px;
}

/* 61px rows · 12px inter-column gap via 6px symmetric padding · vertically centered content */
.acct-layout :deep(.table-scroll-container td) {
  height: 61px;
  padding: 0 6px;
  vertical-align: middle;
  font-size: var(--fs-13);
  line-height: 1.2;
  border-bottom: 1px solid var(--border);
  box-sizing: border-box;
  overflow: hidden;
}

.acct-layout :deep(.table-scroll-container td:first-child) {
  padding-left: 16px;
}

.acct-layout :deep(.table-scroll-container td:last-child) {
  padding-right: 16px;
}

.acct-layout :deep(.table-scroll-container tbody tr:last-child td) {
  border-bottom: 0;
}

/* Let the compact usage cell's hover popover escape the row's clipping box. */
.acct-layout :deep(.table-scroll-container td.acct-col-usage) {
  overflow: visible;
}

/* Sort indicator · 10px accent triangle */
.acct-layout :deep(.data-table-th-sortable svg) {
  width: 10px;
  height: 10px;
}

/* ---------- Table footer ---------- */
.acct-table-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex: none;
  padding: 10px 16px;
  font-size: var(--fs-12-5);
  color: var(--muted);
  border-top: 1px solid var(--border);
}

/* Pagination · 28×28 radius 8, current page solid accent */
.acct-pagination {
  flex: 1 1 auto;
  min-width: 0;
  width: 100%;
  padding: 0;
  border-top: 0;
  background: transparent;
}

.acct-pagination :deep(nav) {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: unset; /* = 0: the nav is a bare flex row, not a pill */
  box-shadow: none;
  margin: 0;
}

.acct-pagination :deep(nav > button) {
  width: 28px;
  height: 28px;
  min-width: 28px;
  padding: 0;
  margin: 0;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--muted);
  font-size: var(--fs-12-5);
  font-weight: var(--fw-medium);
  justify-content: center;
}

.acct-pagination :deep(nav > button:hover:not(:disabled)) {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.acct-pagination :deep(nav > button[aria-current='page']) {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--on-tone);
  font-weight: var(--fw-semibold);
}

.acct-pagination :deep(nav > button:disabled) {
  opacity: 0.45;
}

/* the "…" separator keeps no border */
.acct-pagination :deep(nav > button.cursor-default) {
  border-color: transparent;
}

/* ---------- Cells ---------- */
.acct-checkbox {
  width: 16px;
  height: 16px;
  border-radius: var(--radius-5);
  border: 1.5px solid var(--border);
  accent-color: var(--accent);
  cursor: pointer;
}

/* Compact 2-line name/id cell (glass-04): NameIdCell renders name (600/13px) + `#id · group`
 * (11.5px mono) at line-height 1.2 to fit the 61px row; email + homepage link surface via the
 * cell's native title tooltip instead of a permanent 3rd line, and the cyber-alert badge sits
 * beside the name so it doesn't add height. */
.acct-name-wrap {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  min-width: 0;
  width: 100%;
}

.acct-name-id {
  flex: 1;
  min-width: 0;
}

.acct-name-alert {
  color: var(--danger-text);
}

.acct-cyber-badge {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 3px;
  margin-top: 1px;
  border: 0;
  background: transparent;
  padding: 0;
  font-size: var(--fs-10-5);
  font-weight: var(--fw-medium);
  color: var(--muted);
  cursor: pointer;
}

.acct-cyber-badge:hover {
  color: var(--accent);
}

.acct-cyber-badge.is-alert {
  color: var(--danger-text);
}

.acct-platform-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.acct-platform-tile {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  flex: none;
  border-radius: var(--radius-6);
  color: var(--on-tone);
  box-shadow: inset 0 0 0 1px var(--ring-on-tone);
}

.acct-platform-name {
  font-size: var(--fs-13);
  font-weight: var(--fw-medium);
  white-space: nowrap;
}

.acct-type-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
  min-width: 0;
}

/* The 平台 column already carries the brand tile + name. */
.acct-type-cell :deep(.ptb-tile),
.acct-type-cell :deep(.ptb-name) {
  display: none;
}

.acct-compact-meta {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: var(--fs-11);
  font-weight: var(--fw-medium);
  line-height: 1.3;
}

.acct-compact-dot {
  width: 6px;
  height: 6px;
  border-radius: var(--radius-pill);
  flex: none;
}

.acct-mono-muted {
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  color: var(--muted);
}

.acct-muted-text {
  font-size: var(--fs-12-5);
  color: var(--muted);
}

.acct-note-text {
  display: block;
  max-width: 18rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--fs-12-5);
  color: var(--muted);
}

.acct-priority-cell {
  font-size: var(--fs-13);
  color: var(--muted);
}

.acct-last-used {
  font-size: var(--fs-12-5);
  color: var(--muted);
  white-space: nowrap;
}

.acct-actions-cell {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
}

.acct-proxy-cell,
.acct-expires-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
}

.acct-proxy-name {
  font-size: var(--fs-12-5);
  color: var(--foreground);
}

.acct-inline-btn {
  border-radius: var(--radius-6);
  border: 1px solid var(--border);
  background: transparent;
  padding: 1px 6px;
  font-size: var(--fs-11);
  color: var(--muted);
  cursor: pointer;
}

.acct-inline-btn:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.acct-rate-cell {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-family: var(--font-mono);
  font-size: var(--fs-12-5);
  font-variant-numeric: tabular-nums;
  color: var(--foreground);
}

.acct-rate-sync {
  display: inline-flex;
  cursor: help;
  color: var(--success-text);
}

.acct-score-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 7rem;
  font-family: var(--font-mono);
  font-size: var(--fs-11);
  line-height: 1.4;
}

.acct-score-row {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.acct-score-group {
  max-width: 4.75rem;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--muted);
}

.acct-export-option {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--fs-13);
  color: var(--foreground);
}

/* ---------- FAB (mobile only) ---------- */
.acct-fab {
  display: none;
}

/* ---------- Mobile card mode ---------- */
@media (max-width: 767px) {
  .acct-fab {
    display: inline-flex;
  }


  .acct-layout :deep(.data-table-mobile-card) {
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  /* keys-card pattern: 4-up mini stats, 11px labels / 13px 600 values */
  .acct-layout :deep(.data-table-mobile-card [data-field]) {
    align-items: center;
  }

  .acct-layout :deep(.data-table-mobile-card [data-field] > span:first-child) {
    font-size: var(--fs-11);
    letter-spacing: 0.02em;
    text-transform: none;
    color: var(--muted);
  }

  .acct-layout :deep(.data-table-mobile-card [data-field] > div) {
    font-size: var(--fs-13);
    font-weight: var(--fw-semibold);
  }

  .acct-layout :deep(.data-table-mobile-card [data-field='name'] > span:first-child) {
    display: none;
  }

  .acct-layout :deep(.data-table-mobile-card [data-field='name'] > div) {
    width: 100%;
    text-align: left;
    font-weight: var(--fw-regular);
  }

  .acct-name-wrap :deep(.cell-name-id-name) {
    font-size: var(--fs-15);
  }

  .acct-name-wrap :deep(.cell-name-id-meta) {
    font-size: var(--fs-12);
  }

  .acct-actions-cell {
    justify-content: flex-start;
    gap: 8px;
  }

  .acct-actions-cell .icon-btn {
    width: 44px;
    height: 44px;
    border-radius: var(--radius-field);
    border: 1px solid var(--border);
    background: color-mix(in oklch, var(--surface) 80%, transparent);
  }

  .acct-table-footer {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
}
</style>
