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
 <Button variant="secondary" @click="openBulkEditFromHeader">
 {{ t('admin.accounts.bulkEditHeader') }}
 </Button>

 <!-- Import / Export -->
 <div class="acct-menu" ref="importExportDropdownRef">
 <Button variant="secondary" :aria-expanded="showImportExportDropdown" @click="toggleImportExportDropdown">
 <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
 <span>{{ t('admin.accounts.importExport') }}</span>
 </Button>
 <div v-if="showImportExportDropdown" class="dropdown acct-dropdown">
 <div class="dropdown-label">{{ t('admin.accounts.dataActions') }}</div>
 <button type="button" class="dropdown-item" @click="openSyncFromCrsFromMenu">
 <Icon name="sync" size="sm" />
 <span>{{ t('admin.accounts.syncFromCrs') }}</span>
 </button>
 <button type="button" class="dropdown-item" @click="openImportDataFromMenu">
 <Icon name="upload" size="sm" />
 <span>{{ t('admin.accounts.dataImport') }}</span>
 </button>
 <button type="button" class="dropdown-item" @click="openExportDataFromMenu">
 <Icon name="download" size="sm" />
 <span>{{ selIds.length ? t('admin.accounts.dataExportSelected') : t('admin.accounts.dataExport') }}</span>
 <span v-if="selIds.length" class="acct-dropdown-count">{{ selIds.length }}</span>
 </button>
 </div>
 </div>

 <!-- More tools -->
 <div class="acct-menu" ref="accountToolsDropdownRef">
 <Button
 ref="accountToolsTriggerRef"
 variant="secondary"
 :aria-expanded="showAccountToolsDropdown"
 @click="toggleAccountToolsDropdown"
 >
 <span>{{ t('common.more') }}</span>
 <Icon name="chevronDown" size="xs" />
 </Button>
 <Teleport to="body">
 <div
 v-if="showAccountToolsDropdown"
 class="dropdown acct-tools-dropdown"
 :style="accountToolsDropdownStyle"
 @click.stop
 >
 <div class="acct-tools-scroll" :style="{ maxHeight: `${accountToolsDropdownPosition.maxHeight}px` }">
 <div class="dropdown-label">{{ t('admin.accounts.toolActions') }}</div>
 <button type="button" class="dropdown-item" @click="openCapacityForecast">
 <Icon name="chartBar" size="sm" />
 <span>{{ t('admin.accounts.capacityForecast.action') }}</span>
 </button>
 <button type="button" class="dropdown-item" @click="openErrorPassthrough">
 <Icon name="shield" size="sm" />
 <span>{{ t('admin.errorPassthrough.title') }}</span>
 </button>
 <button type="button" class="dropdown-item" @click="openTLSFingerprintProfiles">
 <Icon name="lock" size="sm" />
 <span>{{ t('admin.tlsFingerprintProfiles.title') }}</span>
 </button>
 <button type="button" class="dropdown-item" @click="openTLSFingerprintRouters">
 <Icon name="lock" size="sm" />
 <span>{{ t('admin.tlsFingerprintRouters.title') }}</span>
 </button>
 </div>
 </div>
 </Teleport>
 </div>
 </template>
 </AccountTableActions>
 </template>
 </PageHeader>
 <TablePageLayout class="acct-layout">
 <template #filters>
 <div class="acct-summary" role="group" :aria-label="t('admin.accounts.columns.status')">
 <button
 v-for="chip in statusChips"
 :key="chip.value || 'all'"
 type="button"
 class="summary-chip"
 :class="{ 'is-active': params.status === chip.value }"
 :data-testid="`account-summary-${chip.value || 'all'}`"
 @click="applyStatusChip(chip.value)"
 >
 <span class="summary-chip-label">
 <span class="summary-chip-dot" :style="{ background: chip.color }"></span>
 {{ chip.label }}
 </span>
 <span class="summary-chip-value num">{{ chip.count }}</span>
 </button>
 </div>
 <div class="acct-filter-row">
 <AccountBulkActionsBar
 :selected-ids="selIds"
 :total-results="pagination.total"
 :selecting-all="selectingAllResults"
 :all-results-selected="allResultsSelected"
 @delete="handleBulkDelete"
 @reset-status="handleBulkResetStatus"
 @refresh-token="handleBulkRefreshToken"
 @probe-upstream-billing="handleBulkProbeUpstreamBilling"
 @edit-selected="openBulkEditSelected"
 @edit-filtered="openBulkEditFiltered"
 @clear="clearSelection"
 @select-page="selectPage"
 @select-all-results="handleSelectAllResults"
 @toggle-schedulable="handleBulkToggleSchedulable"
 />
 <AccountTableFilters
 v-model:searchQuery="params.search"
 :filters="params"
 :groups="groups"
 @update:filters="(newFilters) => Object.assign(params, newFilters)"
 @change="debouncedReload"
 @update:searchQuery="debouncedReload"
 />

 <!-- Auto refresh -->
 <div class="acct-menu" ref="autoRefreshDropdownRef">
 <button
 type="button"
 class="filter-pill"
 :class="{ 'is-active': autoRefreshEnabled }"
 :title="t('admin.accounts.autoRefresh')"
 @click="toggleAutoRefreshDropdown"
 >
 <Icon name="refresh" size="xs" :class="autoRefreshEnabled ? 'animate-spin' : ''" />
 <span class="filter-pill-value">
 {{ autoRefreshEnabled ? t('admin.accounts.autoRefreshCountdown', { seconds: autoRefreshCountdown }) : t('admin.accounts.autoRefresh') }}
 </span>
 </button>
 <div v-if="showAutoRefreshDropdown" class="dropdown acct-dropdown">
 <button type="button" class="dropdown-item" :class="{ 'is-active': autoRefreshEnabled }" @click="setAutoRefreshEnabled(!autoRefreshEnabled)">
 <span class="acct-dropdown-text">{{ t('admin.accounts.enableAutoRefresh') }}</span>
 <Icon v-if="autoRefreshEnabled" name="check" size="sm" />
 </button>
 <div class="dropdown-divider"></div>
 <button
 v-for="sec in autoRefreshIntervals"
 :key="sec"
 type="button"
 class="dropdown-item"
 :class="{ 'is-active': autoRefreshIntervalSeconds === sec }"
 @click="setAutoRefreshInterval(sec)"
 >
 <span class="acct-dropdown-text">{{ autoRefreshIntervalLabel(sec) }}</span>
 <Icon v-if="autoRefreshIntervalSeconds === sec" name="check" size="sm" />
 </button>
 </div>
 </div>

 <span class="acct-selection-count">
 {{ t('admin.accounts.selectedOfTotal') }}
 <b>{{ selIds.length }}</b> / {{ pagination.total }}
 </span>

 <!-- Column settings -->
 <div class="acct-menu" ref="columnsDropdownRef">
 <button
 type="button"
 class="acct-icon-pill"
 :title="t('admin.accounts.viewColumns')"
 :aria-expanded="showColumnsDropdown"
 @click="toggleColumnsDropdown"
 >
 <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6" /></svg>
 </button>
 <div v-if="showColumnsDropdown" class="dropdown acct-dropdown acct-columns-dropdown">
 <div class="dropdown-label">{{ t('admin.accounts.viewColumns') }}</div>
 <button
 v-for="col in toggleableColumns"
 :key="col.key"
 type="button"
 class="dropdown-item"
 :class="{ 'is-active': isColumnVisible(col.key) }"
 @click="toggleColumn(col.key)"
 >
 <span class="acct-dropdown-text">{{ col.label }}</span>
 <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" />
 </button>
 </div>
 </div>
 </div>
 <div v-if="hasPendingListSync" class="acct-pending-sync">
 <span>{{ t('admin.accounts.listPendingSyncHint') }}</span>
 <Button variant="secondary" size="sm" @click="syncPendingListChanges">
 {{ t('admin.accounts.listPendingSyncAction') }}
 </Button>
 </div>
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
import { ref, reactive, computed, onMounted, onUnmounted, toRaw, watch, defineAsyncComponent } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import { useAccountColumnVisibility } from '@/composables/useAccountColumnVisibility'
import { useTableLoader } from '@/composables/useTableLoader'
import { useSwipeSelect, type SwipeSelectVirtualContext } from '@/composables/useSwipeSelect'
import { useTableSelection } from '@/composables/useTableSelection'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { CreateAccountModal, EditAccountModal, BulkEditAccountModal, SyncFromCrsModal, TempUnschedStatusModal } from '@/components/account'
import AccountTableActions from '@/components/admin/account/AccountTableActions.vue'
import AccountTableFilters from '@/components/admin/account/AccountTableFilters.vue'
import AccountBulkActionsBar from '@/components/admin/account/AccountBulkActionsBar.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import DeviceProfileInspectModal from '@/components/admin/account/DeviceProfileInspectModal.vue'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'
import ReAuthAccountModal from '@/components/admin/account/ReAuthAccountModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import ScheduledTestsPanel from '@/components/admin/account/ScheduledTestsPanel.vue'
import type { SelectOption } from '@/components/common/Select.vue'
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
import Button from '@/components/ui/Button.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import Fab from '@/components/ui/Fab.vue'
import ErrorPassthroughRulesModal from '@/components/admin/ErrorPassthroughRulesModal.vue'
import TLSFingerprintProfilesModal from '@/components/admin/TLSFingerprintProfilesModal.vue'
import TLSFingerprintRoutersModal from '@/components/admin/TLSFingerprintRoutersModal.vue'
import PlatformCapacityDialog from '@/components/admin/account/PlatformCapacityDialog.vue'
const AccountCyberEventsModal = defineAsyncComponent(() => import('@/components/admin/account/AccountCyberEventsModal.vue'))
const OpsErrorDetailModal = defineAsyncComponent(() => import('@/views/admin/ops/components/OpsErrorDetailModal.vue'))
import { fetchAllAccountIds } from '@/utils/accountSelection'
import { buildGrokUsageRefreshKey, buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import { proxyExpiryBadgeClass, proxyExpiryLabelKey } from '@/utils/proxyExpiry'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getFloatingPanelPosition } from '@/utils/floatingPanel'
import { formatMultiplier } from '@/utils/formatters'
import { platformTileBackground, platformLabel } from '@/utils/platformTile'
import type { Account, AccountPlatform, AccountSchedulerGroupScore, AccountType, AccountUsageInfo, Proxy as AccountProxy, AdminGroup, WindowStats, ClaudeModel, UpstreamBillingProbeSnapshot } from '@/types'
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
  getSchedulerScoreRows,
  buildDefaultTodayStats,
  accountSupportsBatchUsage
} from './accountRowHelpers'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const proxies = ref<AccountProxy[]>([])
const groups = ref<AdminGroup[]>([])
const accountTableRef = ref<HTMLElement | null>(null)
const dataTableRef = ref<InstanceType<typeof DataTable> | null>(null)
type AccountBulkEditTarget =
  | {
      mode: 'selected'
      accountIds: number[]
      selectedPlatforms: AccountPlatform[]
      selectedTypes: AccountType[]
    }
  | {
      mode: 'filtered'
      filters: {
        platform?: string
        type?: string
        status?: string
        group?: string
        search?: string
        privacy_mode?: string
        sort_by?: string
        sort_order?: AccountSortOrder
      }
      previewCount: number
      selectedPlatforms: AccountPlatform[]
      selectedTypes: AccountType[]
    }
const selPlatforms = computed<AccountPlatform[]>(() => {
  const platforms = new Set(
    accounts.value
      .filter(a => isSelected(a.id))
      .map(a => a.platform)
  )
  return [...platforms]
})
const selTypes = computed<AccountType[]>(() => {
  const types = new Set(
    accounts.value
      .filter(a => isSelected(a.id))
      .map(a => a.type)
  )
  return [...types]
})
const showCreate = ref(false)
const showEdit = ref(false)
const showSync = ref(false)
const showImportData = ref(false)
const showExportDataDialog = ref(false)
const includeProxyOnExport = ref(true)
const showBulkEdit = ref(false)
const bulkEditTarget = ref<AccountBulkEditTarget | null>(null)
const showTempUnsched = ref(false)
const showDeleteDialog = ref(false)
const showResetDeviceProfileDialog = ref(false)
const showDeviceProfileInspect = ref(false)
const resettingDeviceProfile = ref(false)
const showCreateShadowDialog = ref(false)
const showReAuth = ref(false)
const showTest = ref(false)
const showStats = ref(false)
const showErrorPassthrough = ref(false)
const showTLSFingerprintProfiles = ref(false)
const showTLSFingerprintRouters = ref(false)
const showCapacityForecast = ref(false)
const showCyberEvents = ref(false)
const showCyberErrorDetail = ref(false)
const cyberEventsAcc = ref<Account | null>(null)
const cyberErrorID = ref<number | null>(null)
const edAcc = ref<Account | null>(null)
const tempUnschedAcc = ref<Account | null>(null)
const deletingAcc = ref<Account | null>(null)
const resettingDeviceAcc = ref<Account | null>(null)
const inspectingDeviceAcc = ref<Account | null>(null)
const creatingShadowAcc = ref<Account | null>(null)
const reAuthAcc = ref<Account | null>(null)
const testingAcc = ref<Account | null>(null)
const statsAcc = ref<Account | null>(null)
const showSchedulePanel = ref(false)
const scheduleAcc = ref<Account | null>(null)
const scheduleModelOptions = ref<SelectOption[]>([])
const togglingSchedulable = ref<number | null>(null)
const menu = reactive<{show:boolean, acc:Account|null, pos:{top:number, left:number}|null}>({ show: false, acc: null, pos: null })
const exportingData = ref(false)
const probingUpstreamBilling = reactive(new Set<number>())
const upstreamBillingProbeGloballyEnabled = ref<boolean | undefined>(undefined)
const upstreamBillingNow = ref(Date.now())
const upstreamBillingRateETag = ref<string | null>(null)
const upstreamBillingRateRefreshing = ref(false)
let upstreamBillingRateAbortController: AbortController | null = null
useIntervalFn(() => { upstreamBillingNow.value = Date.now() }, 60_000)

// Import / export dropdown
const showImportExportDropdown = ref(false)
const importExportDropdownRef = ref<HTMLElement | null>(null)

// Column settings dropdown
const showColumnsDropdown = ref(false)
const columnsDropdownRef = ref<HTMLElement | null>(null)

// Account tools dropdown
const showAccountToolsDropdown = ref(false)
const accountToolsDropdownRef = ref<HTMLElement | null>(null)
const accountToolsTriggerRef = ref<HTMLElement | { $el?: HTMLElement } | null>(null)
const accountToolsDropdownPosition = reactive({
  top: null as number | null,
  bottom: null as number | null,
  left: 16,
  width: 320,
  maxHeight: 0
})
const accountToolsDropdownStyle = computed(() => ({
  top: accountToolsDropdownPosition.top == null ? 'auto' : `${accountToolsDropdownPosition.top}px`,
  bottom: accountToolsDropdownPosition.bottom == null ? 'auto' : `${accountToolsDropdownPosition.bottom}px`,
  left: `${accountToolsDropdownPosition.left}px`,
  width: `${accountToolsDropdownPosition.width}px`
}))
const { hiddenColumns, isColumnVisible, toggleColumnVisibility } = useAccountColumnVisibility()

// Sorting settings
const ACCOUNT_SORT_STORAGE_KEY = 'account-table-sort'
type AccountSortOrder = 'asc' | 'desc'
type AccountSortState = {
  sort_by: string
  sort_order: AccountSortOrder
}
const ACCOUNT_SORTABLE_KEYS = new Set([
  'id',
  'name',
  'status',
  'schedulable',
  'priority',
  'rate_multiplier',
  'upstream_billing_rate',
  'last_used_at',
  'created_at',
  'expires_at'
])
const loadInitialAccountSortState = (): AccountSortState => {
  const fallback: AccountSortState = { sort_by: 'name', sort_order: 'asc' }
  try {
    const raw = localStorage.getItem(ACCOUNT_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!ACCOUNT_SORTABLE_KEYS.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'desc' ? 'desc' : 'asc'
    }
  } catch {
    return fallback
  }
}
const sortState = reactive<AccountSortState>(loadInitialAccountSortState())

// Auto refresh settings
const showAutoRefreshDropdown = ref(false)
const autoRefreshDropdownRef = ref<HTMLElement | null>(null)
const AUTO_REFRESH_STORAGE_KEY = 'account-auto-refresh'
const autoRefreshIntervals = [5, 10, 15, 30] as const
const autoRefreshEnabled = ref(false)
const autoRefreshIntervalSeconds = ref<(typeof autoRefreshIntervals)[number]>(30)
const autoRefreshCountdown = ref(0)
const autoRefreshETag = ref<string | null>(null)
const autoRefreshFetching = ref(false)
const AUTO_REFRESH_SILENT_WINDOW_MS = 15000
const autoRefreshSilentUntil = ref(0)
const hasPendingListSync = ref(false)
const todayStatsByAccountId = ref<Record<string, WindowStats>>({})
const todayStatsLoading = ref(false)
const todayStatsError = ref<string | null>(null)
const todayStatsReqSeq = ref(0)
const pendingTodayStatsRefresh = ref(false)
const usageManualRefreshToken = ref(0)

const desktopViewportQuery = '(min-width: 768px)'
const isDesktopViewport = ref(
  typeof window === 'undefined' ? true : window.matchMedia(desktopViewportQuery).matches
)
let desktopViewportMediaQuery: MediaQueryList | null = null
let desktopViewportListener: ((event: MediaQueryListEvent) => void) | null = null

const usageBatchByAccountId = ref<Record<string, AccountUsageInfo | null>>({})
const usageBatchErrorByAccountId = ref<Record<string, string | null>>({})
const usageBatchLoadingByAccountId = ref<Record<string, boolean>>({})
const usageBatchRequestTokenByAccountId = ref<Record<string, number>>({})
const usageBatchCache = new Map<number, { data: AccountUsageInfo; ts: number }>()
const USAGE_BATCH_CACHE_TTL = 5 * 60 * 1000
const pendingUsageBatchIds = new Set<number>()
let usageBatchFlushTimer: ReturnType<typeof setTimeout> | null = null
let queuedUsageBatchForce = false
let usageBatchRequestToken = 0

const setUsageBatchLoading = (accountID: number, loadingState: boolean) => {
  usageBatchLoadingByAccountId.value = {
    ...usageBatchLoadingByAccountId.value,
    [String(accountID)]: loadingState
  }
}

const setUsageBatchState = (accountID: number, usage: AccountUsageInfo | null, error: string | null) => {
  const key = String(accountID)
  usageBatchByAccountId.value = {
    ...usageBatchByAccountId.value,
    [key]: usage
  }
  usageBatchErrorByAccountId.value = {
    ...usageBatchErrorByAccountId.value,
    [key]: error
  }
}

const handleAccountUsageLoaded = (accountID: number, usage: AccountUsageInfo) => {
  if (usageBatchByAccountId.value[String(accountID)] === usage) return
  setUsageBatchState(accountID, usage, null)
}

const flushQueuedUsageBatch = async () => {
  usageBatchFlushTimer = null
  const accountIDs = Array.from(pendingUsageBatchIds)
  const force = queuedUsageBatchForce
  pendingUsageBatchIds.clear()
  queuedUsageBatchForce = false

  if (accountIDs.length === 0) return

  const requestTokensByAccount = accountIDs.reduce<Record<string, number>>((acc, accountID) => {
    acc[String(accountID)] = usageBatchRequestTokenByAccountId.value[String(accountID)] ?? 0
    return acc
  }, {})

  try {
    const result = await adminAPI.accounts.getBatchUsage(accountIDs, force)

    const usageMap = result.usage ?? {}
    const errorMap = result.errors ?? {}
    const now = Date.now()
    const nextUsage = { ...usageBatchByAccountId.value }
    const nextErrors = { ...usageBatchErrorByAccountId.value }
    const nextLoading = { ...usageBatchLoadingByAccountId.value }

    for (const accountID of accountIDs) {
      const key = String(accountID)
      if ((usageBatchRequestTokenByAccountId.value[key] ?? 0) !== requestTokensByAccount[key]) {
        continue
      }
      const usage = usageMap[key] ?? null
      nextUsage[key] = usage
      nextErrors[key] = errorMap[key] ?? null
      nextLoading[key] = false
      if (usage) {
        usageBatchCache.set(accountID, { data: usage, ts: now })
      } else {
        usageBatchCache.delete(accountID)
      }
    }

    usageBatchByAccountId.value = nextUsage
    usageBatchErrorByAccountId.value = nextErrors
    usageBatchLoadingByAccountId.value = nextLoading
  } catch (error) {
    const nextErrors = { ...usageBatchErrorByAccountId.value }
    const nextLoading = { ...usageBatchLoadingByAccountId.value }
    for (const accountID of accountIDs) {
      const key = String(accountID)
      if ((usageBatchRequestTokenByAccountId.value[key] ?? 0) !== requestTokensByAccount[key]) {
        continue
      }
      nextErrors[key] = 'Failed'
      nextLoading[key] = false
    }
    usageBatchErrorByAccountId.value = nextErrors
    usageBatchLoadingByAccountId.value = nextLoading
    console.error('Failed to load account usage batch:', error)
  }
}

const queueBatchedUsage = (account: Account, options?: { force?: boolean }) => {
  if (!isDesktopViewport.value) return
  if (!accountSupportsBatchUsage(account)) return

  const force = options?.force === true
  const cacheKey = account.id
  const key = String(cacheKey)

  if (force) {
    usageBatchCache.delete(cacheKey)
  } else {
    const cached = usageBatchCache.get(cacheKey)
    if (cached && Date.now() - cached.ts < USAGE_BATCH_CACHE_TTL) {
      setUsageBatchState(cacheKey, cached.data, null)
      setUsageBatchLoading(cacheKey, false)
      return
    }
  }

  usageBatchErrorByAccountId.value = {
    ...usageBatchErrorByAccountId.value,
    [key]: null
  }
  usageBatchRequestTokenByAccountId.value = {
    ...usageBatchRequestTokenByAccountId.value,
    [key]: ++usageBatchRequestToken
  }
  setUsageBatchLoading(cacheKey, true)
  pendingUsageBatchIds.add(cacheKey)
  queuedUsageBatchForce = queuedUsageBatchForce || force

  if (usageBatchFlushTimer !== null) return
  usageBatchFlushTimer = setTimeout(() => {
    void flushQueuedUsageBatch()
  }, 0)
}

const refreshTodayStatsBatch = async () => {
  // Why this checks both columns:
  // - today_stats column shows dedicated today's metrics.
  // - usage column also embeds today's stats for Key/Bedrock rows.
  // So we only skip fetching when BOTH columns are hidden.
  if (hiddenColumns.has('today_stats') && hiddenColumns.has('usage')) {
    todayStatsLoading.value = false
    todayStatsError.value = null
    return
  }

  const accountIDs = accounts.value.map(account => account.id)
  const reqSeq = ++todayStatsReqSeq.value
  if (accountIDs.length === 0) {
    todayStatsByAccountId.value = {}
    todayStatsError.value = null
    todayStatsLoading.value = false
    return
  }

  todayStatsLoading.value = true
  todayStatsError.value = null

  try {
    const result = await adminAPI.accounts.getBatchTodayStats(accountIDs)
    if (reqSeq !== todayStatsReqSeq.value) return
    const serverStats = result.stats ?? {}
    const nextStats: Record<string, WindowStats> = {}
    for (const accountID of accountIDs) {
      const key = String(accountID)
      nextStats[key] = serverStats[key] ?? buildDefaultTodayStats()
    }
    todayStatsByAccountId.value = nextStats
  } catch (error) {
    if (reqSeq !== todayStatsReqSeq.value) return
    todayStatsError.value = 'Failed'
    console.error('Failed to load account today stats:', error)
  } finally {
    if (reqSeq === todayStatsReqSeq.value) {
      todayStatsLoading.value = false
    }
  }
}

const autoRefreshIntervalLabel = (sec: number) => {
  if (sec === 5) return t('admin.accounts.refreshInterval5s')
  if (sec === 10) return t('admin.accounts.refreshInterval10s')
  if (sec === 15) return t('admin.accounts.refreshInterval15s')
  if (sec === 30) return t('admin.accounts.refreshInterval30s')
  return `${sec}s`
}

const formatSchedulerScoreGroup = (score: AccountSchedulerGroupScore): string => {
  if ('group_name' in score && score.group_name) return score.group_name
  if ('group_id' in score && score.group_id != null) return `#${score.group_id}`
  return t('admin.accounts.schedulerScore.ungrouped')
}

const loadSavedAutoRefresh = () => {
  try {
    const saved = localStorage.getItem(AUTO_REFRESH_STORAGE_KEY)
    if (!saved) return
    const parsed = JSON.parse(saved) as { enabled?: boolean; interval_seconds?: number }
    autoRefreshEnabled.value = parsed.enabled === true
    const interval = Number(parsed.interval_seconds)
    if (autoRefreshIntervals.includes(interval as any)) {
      autoRefreshIntervalSeconds.value = interval as any
    }
  } catch (e) {
    console.error('Failed to load saved auto refresh settings:', e)
  }
}

const saveAutoRefreshToStorage = () => {
  try {
    localStorage.setItem(
      AUTO_REFRESH_STORAGE_KEY,
      JSON.stringify({
        enabled: autoRefreshEnabled.value,
        interval_seconds: autoRefreshIntervalSeconds.value
      })
    )
  } catch (e) {
    console.error('Failed to save auto refresh settings:', e)
  }
}

if (typeof window !== 'undefined') {
  loadSavedAutoRefresh()
}

const setAutoRefreshEnabled = (enabled: boolean) => {
  autoRefreshEnabled.value = enabled
  saveAutoRefreshToStorage()
  if (enabled) {
    autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
    resumeAutoRefresh()
  } else {
    pauseAutoRefresh()
    autoRefreshCountdown.value = 0
  }
}

const setAutoRefreshInterval = (seconds: (typeof autoRefreshIntervals)[number]) => {
  autoRefreshIntervalSeconds.value = seconds
  saveAutoRefreshToStorage()
  if (autoRefreshEnabled.value) {
    autoRefreshCountdown.value = seconds
  }
}

const toggleColumn = (key: string) => {
  const wasHidden = hiddenColumns.has(key)
  toggleColumnVisibility(key)
  if ((key === 'today_stats' || key === 'usage') && wasHidden) {
    refreshTodayStatsBatch().catch((error) => {
      console.error('Failed to load account today stats after showing column:', error)
    })
  }
  if (key === 'scheduler_score') {
    // The server only returns scheduler scores when this column is visible, so reload the current page immediately.
    syncAccountListDerivedParams()
    load().catch((error) => {
      console.error('Failed to reload accounts after toggling scheduler score column:', error)
    })
  }
}

const shouldIncludeSchedulerScore = () => isColumnVisible('scheduler_score')
const syncAccountListDerivedParams = () => {
  // Keep every load path, including auto-refresh and sorting, aligned with the current column visibility.
  const requestParams = params as any
  requestParams.include_scheduler_score = shouldIncludeSchedulerScore() ? '1' : '0'
  requestParams.include_cyber_summary = '1'
}

const {
  items: accounts,
  loading,
  params,
  pagination,
  load: baseLoad,
  reload: baseReload,
  debouncedReload: baseDebouncedReload,
  handlePageChange: baseHandlePageChange,
  handlePageSizeChange: baseHandlePageSizeChange
} = useTableLoader<Account, any>({
  fetchFn: adminAPI.accounts.list,
  initialParams: {
    platform: '',
    type: '',
    status: '',
    privacy_mode: '',
    group: '',
    search: '',
    include_scheduler_score: shouldIncludeSchedulerScore() ? '1' : '0',
    include_cyber_summary: '1',
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
})

const {
  selectedSet,
  selectedIds: selIds,
  allVisibleSelected,
  isSelected,
  setSelectedIds,
  select,
  deselect,
  toggle: toggleSel,
  clear: clearSelectedIds,
  removeMany: removeSelectedAccounts,
  toggleVisible,
  selectVisible: selectCurrentPage,
  batchUpdate
} = useTableSelection<Account>({
  rows: accounts,
  getId: (account) => account.id
})

const selectingAllResults = ref(false)
const selectedAllResultIDs = ref<Set<number> | null>(null)
const selectionRequestVersion = ref(0)
const allResultsSelected = computed(() => {
  const snapshot = selectedAllResultIDs.value
  if (!snapshot || snapshot.size === 0 || snapshot.size !== selectedSet.value.size) return false
  return Array.from(snapshot).every(id => selectedSet.value.has(id))
})

const clearSelection = () => {
  selectionRequestVersion.value++
  selectingAllResults.value = false
  selectedAllResultIDs.value = null
  clearSelectedIds()
}

const selectPage = () => {
  selectCurrentPage()
}

const swipeVirtualContext: SwipeSelectVirtualContext = {
  getVirtualizer: () => dataTableRef.value?.virtualizer ?? null,
  getSortedData: () => dataTableRef.value?.sortedData ?? accounts.value,
  getRowId: (row: any) => row.id,
}

useSwipeSelect(accountTableRef, {
  isSelected,
  select,
  deselect,
  batchUpdate
}, swipeVirtualContext)

const resetAutoRefreshCache = () => {
  autoRefreshETag.value = null
  upstreamBillingRateETag.value = null
}

const isFirstLoad = ref(true)

type AccountLoadOptions = {
  refreshTodayStats?: boolean
}

const load = async (options: AccountLoadOptions = {}) => {
  const requestParams = params as any
  syncAccountListDerivedParams()
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = false
  if (isFirstLoad.value) {
    requestParams.lite = '1'
  }
  await baseLoad()
  if (isFirstLoad.value) {
    isFirstLoad.value = false
    delete requestParams.lite
  }
  if (options.refreshTodayStats !== false) await refreshTodayStatsBatch()
}

const reload = async () => {
  syncAccountListDerivedParams()
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = false
  await baseReload()
  await refreshTodayStatsBatch()
}

const buildUpstreamBillingRateFilters = () => {
  const rawParams = toRaw(params) as Record<string, unknown>
  return {
    platform: typeof rawParams.platform === 'string' ? rawParams.platform : '',
    type: typeof rawParams.type === 'string' ? rawParams.type : '',
    status: typeof rawParams.status === 'string' ? rawParams.status : '',
    group: typeof rawParams.group === 'string' ? rawParams.group : '',
    search: typeof rawParams.search === 'string' ? rawParams.search : '',
    privacy_mode: typeof rawParams.privacy_mode === 'string' ? rawParams.privacy_mode : '',
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
}

const sameAccountIDOrder = (left: number[], right: number[]) =>
  left.length === right.length && left.every((id, index) => id === right[index])

const upstreamBillingRateContextKey = () => JSON.stringify({
  page: pagination.page,
  pageSize: pagination.page_size,
  filters: buildUpstreamBillingRateFilters()
})

const applyUpstreamBillingRateSnapshots = async (
  result: NonNullable<Awaited<ReturnType<typeof adminAPI.accounts.getUpstreamBillingRatesWithEtag>>['data']>
) => {
  const nextIDs = result.items.map(item => item.account_id)
  const currentIDs = accounts.value.map(account => account.id)

  // The compact response cannot fill a row that crossed a page boundary.
  // Only that case needs the expensive, full account-list request.
  if (result.total !== pagination.total || !sameAccountIDOrder(nextIDs, currentIDs)) {
    try {
      await load({ refreshTodayStats: false })
    } catch (error) {
      console.error('Failed to reconcile upstream billing sort:', error)
    }
    return
  }

  const itemsByID = new Map(result.items.map(item => [item.account_id, item]))
  let changed = false
  const nextAccounts = accounts.value.map(account => {
    const item = itemsByID.get(account.id)
    if (!item) return account
    const nextSnapshot = item.snapshot ?? null
    const previousSnapshot = account.extra?.upstream_billing_probe ?? null
    if (JSON.stringify(previousSnapshot) === JSON.stringify(nextSnapshot)) return account

    const nextExtra = { ...(account.extra ?? {}) }
    if (nextSnapshot) nextExtra.upstream_billing_probe = nextSnapshot
    else delete nextExtra.upstream_billing_probe
    const nextAccount = {
      ...account,
      ...(typeof nextSnapshot?.synced_rate_multiplier === 'number'
        ? { rate_multiplier: nextSnapshot.synced_rate_multiplier }
        : {}),
      extra: nextExtra
    }
    syncAccountRefs(nextAccount)
    changed = true
    return nextAccount
  })

  if (changed) {
    accounts.value = nextAccounts
    upstreamBillingNow.value = Date.now()
  }
}

const refreshUpstreamBillingRates = async (force = false) => {
  if (upstreamBillingRateRefreshing.value || loading.value || accounts.value.length === 0) return
  if (!force && (
    probingUpstreamBilling.size > 0 ||
    isAnyModalOpen.value ||
    menu.show ||
    showAccountToolsDropdown.value ||
    showAutoRefreshDropdown.value ||
    (typeof document !== 'undefined' && document.hidden)
  )) return

  const controller = new AbortController()
  upstreamBillingRateAbortController = controller
  upstreamBillingRateRefreshing.value = true
  try {
    syncAccountListDerivedParams()
    const requestContextKey = upstreamBillingRateContextKey()
    const result = await adminAPI.accounts.getUpstreamBillingRatesWithEtag(
      pagination.page,
      pagination.page_size,
      buildUpstreamBillingRateFilters(),
      { etag: force ? null : upstreamBillingRateETag.value, signal: controller.signal }
    )
    if (loading.value || requestContextKey !== upstreamBillingRateContextKey()) return
    if (result.etag) upstreamBillingRateETag.value = result.etag
    if (!result.notModified && result.data) await applyUpstreamBillingRateSnapshots(result.data)
  } catch (error) {
    const refreshError = error as { name?: string; code?: string }
    if (refreshError.name !== 'AbortError' && refreshError.name !== 'CanceledError' && refreshError.code !== 'ERR_CANCELED') {
      console.error('Failed to refresh upstream billing rates:', error)
    }
  } finally {
    if (upstreamBillingRateAbortController === controller) upstreamBillingRateAbortController = null
    upstreamBillingRateRefreshing.value = false
  }
}

const refreshUpstreamBillingSortedList = async (force = false) => {
  if (!force && sortState.sort_by !== 'upstream_billing_rate') return
  await refreshUpstreamBillingRates(force)
}

useIntervalFn(() => { void refreshUpstreamBillingRates() }, 5 * 60_000, { immediate: false })

const debouncedReload = () => {
  clearSelection()
  syncAccountListDerivedParams()
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  baseDebouncedReload()
}

const handlePageChange = (page: number) => {
  syncAccountListDerivedParams()
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  baseHandlePageChange(page)
}

const handlePageSizeChange = (size: number) => {
  syncAccountListDerivedParams()
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  baseHandlePageSizeChange(size)
}

const handleSort = (key: string, order: AccountSortOrder) => {
  sortState.sort_by = key
  sortState.sort_order = order
  const requestParams = params as any
  requestParams.sort_by = key
  requestParams.sort_order = order
  syncAccountListDerivedParams()
  pagination.page = 1
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  load()
}

watch(loading, (isLoading, wasLoading) => {
  if (wasLoading && !isLoading) {
    upstreamBillingNow.value = Date.now()
  }
  if (wasLoading && !isLoading && pendingTodayStatsRefresh.value) {
    pendingTodayStatsRefresh.value = false
    refreshTodayStatsBatch().catch((error) => {
      console.error('Failed to refresh account today stats after table load:', error)
    })
  }
})

watch(accounts, (rows) => {
  const visibleIDs = new Set(rows.map((row) => String(row.id)))
  usageBatchByAccountId.value = Object.fromEntries(
    Object.entries(usageBatchByAccountId.value).filter(([key]) => visibleIDs.has(key))
  )
  usageBatchErrorByAccountId.value = Object.fromEntries(
    Object.entries(usageBatchErrorByAccountId.value).filter(([key]) => visibleIDs.has(key))
  )
  usageBatchLoadingByAccountId.value = Object.fromEntries(
    Object.entries(usageBatchLoadingByAccountId.value).filter(([key]) => visibleIDs.has(key))
  )
  usageBatchRequestTokenByAccountId.value = Object.fromEntries(
    Object.entries(usageBatchRequestTokenByAccountId.value).filter(([key]) => visibleIDs.has(key))
  )
})

const isAnyModalOpen = computed(() => {
  return (
    showCreate.value ||
    showEdit.value ||
    showSync.value ||
    showImportData.value ||
    showExportDataDialog.value ||
    showBulkEdit.value ||
    showTempUnsched.value ||
    showDeleteDialog.value ||
    showResetDeviceProfileDialog.value ||
    showDeviceProfileInspect.value ||
    showReAuth.value ||
    showTest.value ||
    showStats.value ||
    showSchedulePanel.value ||
    showErrorPassthrough.value ||
    showTLSFingerprintProfiles.value ||
    showTLSFingerprintRouters.value ||
    showCapacityForecast.value
  )
})

const enterAutoRefreshSilentWindow = () => {
  autoRefreshSilentUntil.value = Date.now() + AUTO_REFRESH_SILENT_WINDOW_MS
  autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
}

const inAutoRefreshSilentWindow = () => {
  return Date.now() < autoRefreshSilentUntil.value
}

const shouldReplaceAutoRefreshRow = (current: Account, next: Account) => {
  return (
    current.updated_at !== next.updated_at ||
    current.current_concurrency !== next.current_concurrency ||
    current.current_window_cost !== next.current_window_cost ||
    current.active_sessions !== next.active_sessions ||
    current.schedulable !== next.schedulable ||
    current.status !== next.status ||
    current.rate_limit_reset_at !== next.rate_limit_reset_at ||
    current.overload_until !== next.overload_until ||
    current.temp_unschedulable_until !== next.temp_unschedulable_until ||
    buildOpenAIUsageRefreshKey(current) !== buildOpenAIUsageRefreshKey(next) ||
    buildGrokUsageRefreshKey(current) !== buildGrokUsageRefreshKey(next)
  )
}

const syncAccountRefs = (nextAccount: Account) => {
  if (edAcc.value?.id === nextAccount.id) edAcc.value = nextAccount
  if (reAuthAcc.value?.id === nextAccount.id) reAuthAcc.value = nextAccount
  if (tempUnschedAcc.value?.id === nextAccount.id) tempUnschedAcc.value = nextAccount
  if (deletingAcc.value?.id === nextAccount.id) deletingAcc.value = nextAccount
  if (resettingDeviceAcc.value?.id === nextAccount.id) resettingDeviceAcc.value = nextAccount
  if (inspectingDeviceAcc.value?.id === nextAccount.id) inspectingDeviceAcc.value = nextAccount
  if (menu.acc?.id === nextAccount.id) menu.acc = nextAccount
}

const mergeAccountsIncrementally = (nextRows: Account[]) => {
  const currentRows = accounts.value
  const currentByID = new Map(currentRows.map(row => [row.id, row]))
  let changed = nextRows.length !== currentRows.length
  const mergedRows = nextRows.map((nextRow) => {
    const currentRow = currentByID.get(nextRow.id)
    if (!currentRow) {
      changed = true
      return nextRow
    }
    if (shouldReplaceAutoRefreshRow(currentRow, nextRow)) {
      changed = true
      syncAccountRefs(nextRow)
      return nextRow
    }
    return currentRow
  })
  if (!changed) {
    for (let i = 0; i < mergedRows.length; i += 1) {
      if (mergedRows[i].id !== currentRows[i]?.id) {
        changed = true
        break
      }
    }
  }
  if (changed) {
    accounts.value = mergedRows
  }
}

const refreshAccountsIncrementally = async () => {
  if (autoRefreshFetching.value) return
  syncAccountListDerivedParams()
  autoRefreshFetching.value = true
  try {
    const result = await adminAPI.accounts.listWithEtag(
      pagination.page,
      pagination.page_size,
      toRaw(params) as {
        platform?: string
        type?: string
        status?: string
        privacy_mode?: string
        group?: string
        search?: string
        sort_by?: string
        sort_order?: AccountSortOrder
        include_scheduler_score?: string
        include_cyber_summary?: string
      },
      { etag: autoRefreshETag.value }
    )

    if (result.etag) {
      autoRefreshETag.value = result.etag
    }
    if (!result.notModified && result.data) {
      pagination.total = result.data.total || 0
      pagination.pages = result.data.pages || 0
      mergeAccountsIncrementally(result.data.items || [])
      hasPendingListSync.value = false
    }
    upstreamBillingNow.value = Date.now()

    await refreshTodayStatsBatch()
  } catch (error) {
    console.error('Auto refresh failed:', error)
  } finally {
    autoRefreshFetching.value = false
  }
}

const handleManualRefresh = async () => {
  await Promise.all([load(), loadUpstreamBillingProbeGlobalState()])
  // Force usage cells to refetch /usage on explicit user refresh.
  usageManualRefreshToken.value += 1
}

const loadUpstreamBillingProbeGlobalState = async () => {
  try {
    const settings = await adminAPI.accounts.getUpstreamBillingProbeSettings()
    upstreamBillingProbeGloballyEnabled.value = settings.enabled
  } catch (error) {
    console.error('Failed to load upstream billing probe settings:', error)
  }
}

const closeAccountToolsDropdown = () => {
  showAccountToolsDropdown.value = false
}

const updateAccountToolsDropdownPosition = () => {
  const raw = accountToolsTriggerRef.value as HTMLElement | { $el?: HTMLElement } | null
  const trigger = (raw && '$el' in raw ? raw.$el : raw) as HTMLElement | null | undefined
  if (!trigger || typeof trigger.getBoundingClientRect !== 'function') return

  const position = getFloatingPanelPosition(
    trigger.getBoundingClientRect(),
    document.documentElement.clientWidth || window.innerWidth,
    window.innerHeight
  )
  Object.assign(accountToolsDropdownPosition, position)
}

const toggleAccountToolsDropdown = () => {
  const nextVisible = !showAccountToolsDropdown.value
  showAutoRefreshDropdown.value = false
  showImportExportDropdown.value = false
  showColumnsDropdown.value = false
  if (nextVisible) updateAccountToolsDropdownPosition()
  showAccountToolsDropdown.value = nextVisible
}

const toggleImportExportDropdown = () => {
  const nextVisible = !showImportExportDropdown.value
  showAccountToolsDropdown.value = false
  showAutoRefreshDropdown.value = false
  showColumnsDropdown.value = false
  showImportExportDropdown.value = nextVisible
}

const toggleAutoRefreshDropdown = () => {
  const nextVisible = !showAutoRefreshDropdown.value
  showAccountToolsDropdown.value = false
  showImportExportDropdown.value = false
  showColumnsDropdown.value = false
  showAutoRefreshDropdown.value = nextVisible
}

const toggleColumnsDropdown = () => {
  const nextVisible = !showColumnsDropdown.value
  showAccountToolsDropdown.value = false
  showImportExportDropdown.value = false
  showAutoRefreshDropdown.value = false
  showColumnsDropdown.value = nextVisible
}

const openSyncFromCrsFromMenu = () => {
  showImportExportDropdown.value = false
  showSync.value = true
}

const openImportDataFromMenu = () => {
  showImportExportDropdown.value = false
  showImportData.value = true
}

const openExportDataFromMenu = () => {
  showImportExportDropdown.value = false
  openExportDataDialog()
}

const openCapacityForecast = () => {
  closeAccountToolsDropdown()
  showCapacityForecast.value = true
}

const openBulkEditFromHeader = () => {
  if (selIds.value.length > 0) {
    openBulkEditSelected()
    return
  }
  void openBulkEditFiltered()
}




const openErrorPassthrough = () => {
  closeAccountToolsDropdown()
  showErrorPassthrough.value = true
}

const openTLSFingerprintRouters = () => {
  closeAccountToolsDropdown()
  showTLSFingerprintRouters.value = true
}

const openTLSFingerprintProfiles = () => {
  closeAccountToolsDropdown()
  showTLSFingerprintProfiles.value = true
}

const syncPendingListChanges = async () => {
  hasPendingListSync.value = false
  await load()
  // Keep behavior consistent with manual refresh.
  usageManualRefreshToken.value += 1
}

const { pause: pauseAutoRefresh, resume: resumeAutoRefresh } = useIntervalFn(
  async () => {
    if (!autoRefreshEnabled.value) return
    if (document.hidden) return
    if (loading.value || autoRefreshFetching.value) return
    if (isAnyModalOpen.value) return
    if (menu.show || showAccountToolsDropdown.value || showAutoRefreshDropdown.value) return
    if (inAutoRefreshSilentWindow()) {
      autoRefreshCountdown.value = Math.max(
        0,
        Math.ceil((autoRefreshSilentUntil.value - Date.now()) / 1000)
      )
      return
    }

    if (autoRefreshCountdown.value <= 0) {
      autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
      await refreshAccountsIncrementally()
      return
    }

    autoRefreshCountdown.value -= 1
  },
  1000,
  { immediate: false }
)

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
  cyberEventsAcc.value = account
  showCyberEvents.value = true
}
const closeCyberEvents = () => {
  showCyberEvents.value = false
  cyberEventsAcc.value = null
}
const openCyberErrorDetail = (errorID: number) => {
  closeCyberEvents()
  cyberErrorID.value = errorID
  showCyberErrorDetail.value = true
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

// All available columns
const allColumns = computed(() => {
  const c = [
    { key: 'select', label: '', sortable: false },
    { key: 'name', label: t('admin.accounts.columns.nameId'), sortable: true },
    { key: 'id', label: t('admin.accounts.columns.id'), sortable: true },
    { key: 'platform', label: t('admin.accounts.columns.platform'), sortable: false },
    { key: 'platform_type', label: t('admin.accounts.columns.type'), sortable: false },
    { key: 'capacity', label: t('admin.accounts.columns.capacity'), sortable: false },
    { key: 'status', label: t('admin.accounts.columns.status'), sortable: true },
    { key: 'schedulable', label: t('admin.accounts.columns.schedulable'), sortable: true },
    { key: 'today_stats', label: t('admin.accounts.columns.todayStats'), sortable: false }
  ]
  if (!authStore.isSimpleMode) {
    c.push({ key: 'groups', label: t('admin.accounts.columns.groups'), sortable: false })
  }
  c.push({ key: 'usage', label: t('admin.accounts.columns.usageWindows'), sortable: false })
  c.push(
    { key: 'priority', label: t('admin.accounts.columns.priority'), sortable: true },
    { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: true },
    { key: 'proxy', label: t('admin.accounts.columns.proxy'), sortable: false },
    { key: 'scheduler_score', label: t('admin.accounts.columns.schedulerScore'), sortable: false },
    { key: 'rate_multiplier', label: t('admin.accounts.columns.billingRateMultiplier'), sortable: true },
    { key: 'upstream_billing_rate', label: t('admin.accounts.columns.upstreamBillingRate'), sortable: true },
    { key: 'created_at', label: t('admin.accounts.columns.createdAt'), sortable: true },
    { key: 'expires_at', label: t('admin.accounts.columns.expiresAt'), sortable: true },
    { key: 'notes', label: t('admin.accounts.columns.notes'), sortable: false },
    { key: 'actions', label: t('admin.accounts.columns.actions'), sortable: false }
  )
  return c
})

// Columns that can be toggled (exclude select, name, and actions)
const toggleableColumns = computed(() =>
  allColumns.value.filter(col => col.key !== 'select' && col.key !== 'name' && col.key !== 'actions')
)

// Filtered columns based on visibility
const cols = computed(() =>
  allColumns.value
    .filter(col => col.key === 'select' || col.key === 'name' || col.key === 'actions' || !hiddenColumns.has(col.key))
    .map(col => ({ ...col, class: `acct-col-${col.key}` }))
)

// ---------------------------------------------------------------------------
// Summary chips (全部 / 正常 / 限流 / 异常 / 已暂停)
// ---------------------------------------------------------------------------
type AccountStatusBucket = 'active' | 'rate_limited' | 'error' | 'unschedulable'

const accountStatusBucket = (account: Account, now: number): AccountStatusBucket => {
  if (account.status === 'error') return 'error'
  if (account.rate_limit_reset_at && new Date(account.rate_limit_reset_at).getTime() > now) {
    return 'rate_limited'
  }
  if (!account.schedulable || account.status !== 'active') return 'unschedulable'
  return 'active'
}

// Counts are derived from the rows currently loaded; the API exposes no
// aggregate endpoint, so the snapshot is captured on the last unfiltered load
// and reused while a status filter narrows the result set.
const statusBucketCounts = ref<Record<AccountStatusBucket, number>>({
  active: 0,
  rate_limited: 0,
  error: 0,
  unschedulable: 0
})
const statusBucketTotal = ref(0)

const captureStatusBucketCounts = () => {
  const now = Date.now()
  const next: Record<AccountStatusBucket, number> = {
    active: 0,
    rate_limited: 0,
    error: 0,
    unschedulable: 0
  }
  for (const account of accounts.value) next[accountStatusBucket(account, now)] += 1
  statusBucketCounts.value = next
  statusBucketTotal.value = pagination.total
}

watch(
  () => accounts.value,
  () => {
    if (!params.status) captureStatusBucketCounts()
  },
  { deep: false }
)

const statusChips = computed(() => [
  {
    value: '',
    label: t('admin.accounts.summary.all'),
    color: 'var(--accent)',
    count: params.status ? statusBucketTotal.value : pagination.total
  },
  {
    value: 'active',
    label: t('admin.accounts.summary.normal'),
    color: 'var(--success)',
    count: statusBucketCounts.value.active
  },
  {
    value: 'rate_limited',
    label: t('admin.accounts.summary.limited'),
    color: 'var(--warning)',
    count: statusBucketCounts.value.rate_limited
  },
  {
    value: 'error',
    label: t('admin.accounts.summary.abnormal'),
    color: 'var(--danger)',
    count: statusBucketCounts.value.error
  },
  {
    value: 'unschedulable',
    label: t('admin.accounts.summary.paused'),
    color: 'var(--muted)',
    count: statusBucketCounts.value.unschedulable
  }
])

const applyStatusChip = (value: string) => {
  if (params.status === value) return
  params.status = value
  clearSelection()
  reload()
}

// ---------------------------------------------------------------------------
// Row display helpers
// ---------------------------------------------------------------------------
const accountGroupLabel = (account: Account): string => {
  const groupList = account.groups ?? []
  if (!groupList.length) return t('admin.accounts.ungroupedGroup')
  const [first, ...rest] = groupList
  return rest.length ? `${first.name} +${rest.length}` : first.name
}

const accountIdentityTitle = (account: Account): string => {
  const parts = [`#${account.id}`, accountGroupLabel(account)]
  const email = accountDisplayEmail(account)
  if (email) parts.push(email)
  if (account.parent_chatgpt_account_id) parts.push(String(account.parent_chatgpt_account_id))
  return parts.join(' · ')
}

const handleEdit = (a: Account) => { edAcc.value = a; showEdit.value = true }
const openMenu = (a: Account, e: MouseEvent) => {
  menu.acc = a

  const target = e.currentTarget as HTMLElement
  if (target) {
    const rect = target.getBoundingClientRect()
    const menuWidth = 200
    const menuHeight = 360
    const padding = 8
    const viewportWidth = window.innerWidth
    const viewportHeight = window.innerHeight

    let left: number
    let top: number

    if (viewportWidth < 768) {
      // 居中显示,水平位置
      left = Math.max(padding, Math.min(
        rect.left + rect.width / 2 - menuWidth / 2,
        viewportWidth - menuWidth - padding
      ))

      // 优先显示在按钮下方
      top = rect.bottom + 4

      // 如果下方空间不够,显示在上方
      if (top + menuHeight > viewportHeight - padding) {
        top = rect.top - menuHeight - 4
        // 如果上方也不够,就贴在视口顶部
        if (top < padding) {
          top = padding
        }
      }
    } else {
      left = Math.max(padding, Math.min(
        e.clientX - menuWidth,
        viewportWidth - menuWidth - padding
      ))
      top = e.clientY
      if (top + menuHeight > viewportHeight - padding) {
        top = viewportHeight - menuHeight - padding
      }
    }

    menu.pos = { top, left }
  } else {
    menu.pos = { top: e.clientY, left: e.clientX - 200 }
  }

  menu.show = true
}
const toggleSelectAllVisible = (event: Event) => {
  const target = event.target as HTMLInputElement
  toggleVisible(target.checked)
}
const handleBulkDelete = async () => {
  const accountIds = [...selIds.value]
  if (!confirm(t('admin.accounts.bulkActions.confirmDelete', { count: accountIds.length }))) return
  try {
    const result = await adminAPI.accounts.batchDelete(accountIds)
    if (result.failed > 0) {
      appStore.showError(t('admin.accounts.bulkActions.partialSuccess', {
        success: result.success,
        failed: result.failed
      }))
      setSelectedIds(result.failed_ids?.length ? result.failed_ids : accountIds)
    } else {
      appStore.showSuccess(t('admin.accounts.bulkActions.deleteSuccess', { count: result.success }))
      clearSelection()
    }
    await reload()
  } catch (error) {
    console.error('Failed to bulk delete accounts:', error)
    appStore.showError(String(error))
  }
}
const handleBulkResetStatus = async () => {
  if (!confirm(t('common.confirm'))) return
  try {
    const result = await adminAPI.accounts.batchClearError(selIds.value)
    if (result.failed > 0) {
      appStore.showError(t('admin.accounts.bulkActions.partialSuccess', { success: result.success, failed: result.failed }))
    } else {
      appStore.showSuccess(t('admin.accounts.bulkActions.resetStatusSuccess', { count: result.success }))
      clearSelection()
    }
    reload()
  } catch (error) {
    console.error('Failed to bulk reset status:', error)
    appStore.showError(String(error))
  }
}
const handleBulkRefreshToken = async () => {
  if (!confirm(t('common.confirm'))) return
  try {
    const result = await adminAPI.accounts.batchRefresh(selIds.value)
    if (result.failed > 0) {
      appStore.showError(t('admin.accounts.bulkActions.partialSuccess', { success: result.success, failed: result.failed }))
    } else {
      appStore.showSuccess(t('admin.accounts.bulkActions.refreshTokenSuccess', { count: result.success }))
      clearSelection()
    }
    reload()
  } catch (error) {
    console.error('Failed to bulk refresh token:', error)
    appStore.showError(String(error))
  }
}
const handleBulkProbeUpstreamBilling = async () => {
  const accountIDs = [...selIds.value]
  if (accountIDs.length === 0) {
    appStore.showError(t('admin.accounts.upstreamBilling.noEligibleAccounts'))
    return
  }
  if (accountIDs.length > 20) {
    appStore.showError(t('admin.accounts.upstreamBilling.batchLimit'))
    return
  }
  accountIDs.forEach(id => probingUpstreamBilling.add(id))
  try {
    const results = await adminAPI.accounts.probeUpstreamBillingBatch(accountIDs)
    let patched = false
    results.forEach(result => {
      if (result.snapshot) {
        patchUpstreamBillingSnapshot(result.account_id, result.snapshot)
        patched = true
      }
    })
    if (patched) await refreshAccountsAfterUpstreamBillingProbe()
    const failed = results.filter(result => result.error).length
    if (failed > 0) {
      appStore.showError(t('admin.accounts.upstreamBilling.batchPartial', { success: results.length - failed, failed }))
    } else {
      appStore.showSuccess(t('admin.accounts.upstreamBilling.batchCompleted', { count: results.length }))
    }
  } catch (error) {
    console.error('Failed to probe upstream billing in batch:', error)
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.upstreamBilling.probeFailed')))
  } finally {
    accountIDs.forEach(id => probingUpstreamBilling.delete(id))
  }
}
const updateSchedulableInList = (accountIds: number[], schedulable: boolean) => {
  if (accountIds.length === 0) return
  const idSet = new Set(accountIds)
  accounts.value = accounts.value.map((account) => (idSet.has(account.id) ? { ...account, schedulable } : account))
}
const normalizeBulkSchedulableResult = (
  result: {
    success?: number
    failed?: number
    success_ids?: number[]
    failed_ids?: number[]
    results?: Array<{ account_id: number; success: boolean }>
  },
  accountIds: number[]
) => {
  const responseSuccessIds = Array.isArray(result.success_ids) ? result.success_ids : []
  const responseFailedIds = Array.isArray(result.failed_ids) ? result.failed_ids : []
  if (responseSuccessIds.length > 0 || responseFailedIds.length > 0) {
    return {
      successIds: responseSuccessIds,
      failedIds: responseFailedIds,
      successCount: typeof result.success === 'number' ? result.success : responseSuccessIds.length,
      failedCount: typeof result.failed === 'number' ? result.failed : responseFailedIds.length,
      hasIds: true,
      hasCounts: true
    }
  }

  const results = Array.isArray(result.results) ? result.results : []
  if (results.length > 0) {
    const successIds = results.filter(item => item.success).map(item => item.account_id)
    const failedIds = results.filter(item => !item.success).map(item => item.account_id)
    return {
      successIds,
      failedIds,
      successCount: typeof result.success === 'number' ? result.success : successIds.length,
      failedCount: typeof result.failed === 'number' ? result.failed : failedIds.length,
      hasIds: true,
      hasCounts: true
    }
  }

  const hasExplicitCounts = typeof result.success === 'number' || typeof result.failed === 'number'
  const successCount = typeof result.success === 'number' ? result.success : 0
  const failedCount = typeof result.failed === 'number' ? result.failed : 0
  if (hasExplicitCounts && failedCount === 0 && successCount === accountIds.length && accountIds.length > 0) {
    return {
      successIds: accountIds,
      failedIds: [],
      successCount,
      failedCount,
      hasIds: true,
      hasCounts: true
    }
  }

  return {
    successIds: [],
    failedIds: [],
    successCount,
    failedCount,
    hasIds: false,
    hasCounts: hasExplicitCounts
  }
}
const handleBulkToggleSchedulable = async (schedulable: boolean) => {
  const accountIds = [...selIds.value]
  try {
    const result = await adminAPI.accounts.bulkUpdate(accountIds, { schedulable })
    const { successIds, failedIds, successCount, failedCount, hasIds, hasCounts } = normalizeBulkSchedulableResult(result, accountIds)
    if (!hasIds && !hasCounts) {
      appStore.showError(t('admin.accounts.bulkSchedulableResultUnknown'))
      setSelectedIds(accountIds)
      load().catch((error) => {
        console.error('Failed to refresh accounts:', error)
      })
      return
    }
    if (successIds.length > 0) {
      updateSchedulableInList(successIds, schedulable)
    }
    if (successCount > 0 && failedCount === 0) {
      const message = schedulable
        ? t('admin.accounts.bulkSchedulableEnabled', { count: successCount })
        : t('admin.accounts.bulkSchedulableDisabled', { count: successCount })
      appStore.showSuccess(message)
    }
    if (failedCount > 0) {
      const message = hasCounts || hasIds
        ? t('admin.accounts.bulkSchedulablePartial', { success: successCount, failed: failedCount })
        : t('admin.accounts.bulkSchedulableResultUnknown')
      appStore.showError(message)
      setSelectedIds(failedIds.length > 0 ? failedIds : accountIds)
    } else {
      if (hasIds) clearSelection()
      else setSelectedIds(accountIds)
    }
  } catch (error) {
    console.error('Failed to bulk toggle schedulable:', error)
    appStore.showError(t('common.error'))
  }
}
const buildBulkEditFilterSnapshot = () => {
  const rawParams = toRaw(params) as Record<string, unknown>
  const sortOrder: AccountSortOrder = rawParams.sort_order === 'desc' ? 'desc' : 'asc'
  return {
    platform: typeof rawParams.platform === 'string' ? rawParams.platform : '',
    type: typeof rawParams.type === 'string' ? rawParams.type : '',
    status: typeof rawParams.status === 'string' ? rawParams.status : '',
    group: typeof rawParams.group === 'string' ? rawParams.group : '',
    search: typeof rawParams.search === 'string' ? rawParams.search : '',
    privacy_mode: typeof rawParams.privacy_mode === 'string' ? rawParams.privacy_mode : '',
    sort_by: typeof rawParams.sort_by === 'string' ? rawParams.sort_by : '',
    sort_order: sortOrder
  }
}

const handleSelectAllResults = async () => {
  if (selectingAllResults.value || pagination.total === 0) return

  const requestVersion = ++selectionRequestVersion.value
  const filters = buildBulkEditFilterSnapshot()
  selectingAllResults.value = true
  try {
    const ids = await fetchAllAccountIds(
      (page, pageSize, requestFilters) => adminAPI.accounts.list(page, pageSize, requestFilters),
      filters
    )
    if (requestVersion !== selectionRequestVersion.value) return

    setSelectedIds(ids)
    selectedAllResultIDs.value = new Set(ids)
  } catch (error) {
    if (requestVersion !== selectionRequestVersion.value) return
    console.error('Failed to select all account results:', error)
    appStore.showError(t('admin.accounts.bulkActions.selectAllFailed'))
  } finally {
    if (requestVersion === selectionRequestVersion.value) {
      selectingAllResults.value = false
    }
  }
}

const collectSelectionMetadata = (rows: Account[]) => {
  const selectedPlatforms = Array.from(new Set(rows.map(account => account.platform)))
  const selectedTypes = Array.from(new Set(rows.map(account => account.type)))
  return { selectedPlatforms, selectedTypes }
}

const openBulkEditSelected = () => {
  bulkEditTarget.value = {
    mode: 'selected',
    accountIds: [...selIds.value],
    selectedPlatforms: [...selPlatforms.value],
    selectedTypes: [...selTypes.value]
  }
  showBulkEdit.value = true
}

const openBulkEditFiltered = async () => {
  const filters = buildBulkEditFilterSnapshot()
  const preview = await adminAPI.accounts.list(1, 100, filters)
  const { selectedPlatforms, selectedTypes } = collectSelectionMetadata(preview.items)
  bulkEditTarget.value = {
    mode: 'filtered',
    filters,
    previewCount: preview.total,
    selectedPlatforms,
    selectedTypes
  }
  showBulkEdit.value = true
}

const handleBulkUpdated = () => {
  showBulkEdit.value = false
  bulkEditTarget.value = null
  clearSelection()
  reload()
}
const handleDataImported = () => { showImportData.value = false; reload() }
const ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE = 'ungrouped'
const ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE = '__unset__'
const buildAccountQueryFilters = () => ({
  platform: params.platform || '',
  type: params.type || '',
  status: params.status || '',
  group: params.group || '',
  privacy_mode: params.privacy_mode || '',
  search: params.search || '',
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})
const accountMatchesCurrentFilters = (account: Account) => {
  const filters = buildAccountQueryFilters()
  if (filters.platform && account.platform !== filters.platform) return false
  if (filters.type && account.type !== filters.type) return false
  if (filters.status) {
    const now = Date.now()
    const rateLimitResetAt = account.rate_limit_reset_at ? new Date(account.rate_limit_reset_at).getTime() : Number.NaN
    const isRateLimited = Number.isFinite(rateLimitResetAt) && rateLimitResetAt > now
    const tempUnschedUntil = account.temp_unschedulable_until ? new Date(account.temp_unschedulable_until).getTime() : Number.NaN
    const isTempUnschedulable = Number.isFinite(tempUnschedUntil) && tempUnschedUntil > now

    if (filters.status === 'active') {
      if (account.status !== 'active' || isRateLimited || isTempUnschedulable || !account.schedulable) return false
    } else if (filters.status === 'rate_limited') {
      if (account.status !== 'active' || !isRateLimited || isTempUnschedulable) return false
    } else if (filters.status === 'temp_unschedulable') {
      if (account.status !== 'active' || !isTempUnschedulable) return false
    } else if (filters.status === 'unschedulable') {
      if (account.status !== 'active' || account.schedulable || isRateLimited || isTempUnschedulable) return false
    } else if (account.status !== filters.status) {
      return false
    }
  }
  if (filters.group) {
    const groupIds = account.group_ids ?? account.groups?.map((group) => group.id) ?? []
    if (filters.group === ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE) {
      if (groupIds.length > 0) return false
    } else if (!groupIds.includes(Number(filters.group))) {
      return false
    }
  }
  const privacyMode = typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
  if (filters.privacy_mode) {
    if (filters.privacy_mode === ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE) {
      if (privacyMode.trim() !== '') return false
    } else if (privacyMode !== filters.privacy_mode) {
      return false
    }
  }
  const search = String(filters.search || '').trim().toLowerCase()
  if (search && !account.name.toLowerCase().includes(search)) return false
  return true
}
const mergeRuntimeFields = (oldAccount: Account, updatedAccount: Account): Account => ({
  ...updatedAccount,
  current_concurrency: updatedAccount.current_concurrency ?? oldAccount.current_concurrency,
  current_window_cost: updatedAccount.current_window_cost ?? oldAccount.current_window_cost,
  active_sessions: updatedAccount.active_sessions ?? oldAccount.active_sessions
})

const syncPaginationAfterLocalRemoval = () => {
  const nextTotal = Math.max(0, pagination.total - 1)
  pagination.total = nextTotal
  pagination.pages = nextTotal > 0 ? Math.ceil(nextTotal / pagination.page_size) : 0

  const maxPage = Math.max(1, pagination.pages || 1)

  if (pagination.page > maxPage) {
    pagination.page = maxPage
  }
  // 行被本地移除后不立刻全量补页，改为提示用户手动同步。
  hasPendingListSync.value = nextTotal > 0
}

const patchAccountInList = (updatedAccount: Account) => {
  const index = accounts.value.findIndex(account => account.id === updatedAccount.id)
  if (index === -1) return
  const mergedAccount = mergeRuntimeFields(accounts.value[index], updatedAccount)
  if (!accountMatchesCurrentFilters(mergedAccount)) {
    accounts.value = accounts.value.filter(account => account.id !== mergedAccount.id)
    syncPaginationAfterLocalRemoval()
    removeSelectedAccounts([mergedAccount.id])
    if (menu.acc?.id === mergedAccount.id) {
      menu.show = false
      menu.acc = null
    }
    return
  }
  const nextAccounts = [...accounts.value]
  nextAccounts[index] = mergedAccount
  accounts.value = nextAccounts
  syncAccountRefs(mergedAccount)
}
const patchUpstreamBillingSnapshot = (accountID: number, snapshot: UpstreamBillingProbeSnapshot) => {
  const account = accounts.value.find(item => item.id === accountID)
  if (!account) return
  upstreamBillingNow.value = Date.now()
  patchAccountInList({
    ...account,
    ...(typeof snapshot.synced_rate_multiplier === 'number'
      ? { rate_multiplier: snapshot.synced_rate_multiplier }
      : {}),
    extra: { ...account.extra, upstream_billing_probe: snapshot }
  })
}
const refreshAccountsAfterUpstreamBillingProbe = async () => {
  await refreshUpstreamBillingSortedList(true)
}
const handleProbeUpstreamBilling = async (account: Account) => {
  if (probingUpstreamBilling.has(account.id)) return
  probingUpstreamBilling.add(account.id)
  try {
    const result = await adminAPI.accounts.probeUpstreamBilling(account.id)
    if (result.snapshot) {
      patchUpstreamBillingSnapshot(account.id, result.snapshot)
      await refreshAccountsAfterUpstreamBillingProbe()
    }
  } catch (error) {
    console.error('Failed to probe upstream billing:', error)
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.upstreamBilling.probeFailed')))
  } finally {
    probingUpstreamBilling.delete(account.id)
  }
}
const handleAccountUpdated = (updatedAccount: Account) => {
  patchAccountInList(updatedAccount)
  enterAutoRefreshSilentWindow()
}
const formatExportTimestamp = () => {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}
const openExportDataDialog = () => {
  includeProxyOnExport.value = true
  showExportDataDialog.value = true
}
const handleExportData = async () => {
  if (exportingData.value) return
  exportingData.value = true
  try {
    const dataPayload = await accountExportStepUp.run(() => adminAPI.accounts.exportData(
      selIds.value.length > 0
        ? { ids: selIds.value, includeProxies: includeProxyOnExport.value }
        : {
            includeProxies: includeProxyOnExport.value,
            filters: buildAccountQueryFilters()
          }
    ))
    const timestamp = formatExportTimestamp()
    const filename = `sub2api-account-${timestamp}.json`
    const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
    // spark 影子账号被后端排除出备份(其凭据透传母账号、调度配置不可经凭据型导入重建);
    // 跳过非零时明确提示用户,避免「下载成功但少了账号」的静默丢失。
    if (dataPayload.skipped_shadows && dataPayload.skipped_shadows > 0) {
      appStore.showWarning(t('admin.accounts.dataExportedSkippedShadows', { count: dataPayload.skipped_shadows }))
    } else {
      appStore.showSuccess(t('admin.accounts.dataExported'))
    }
  } catch (error: any) {
    if (isStepUpCancelled(error)) {
      // 用户主动取消 step-up 验证，静默返回，不弹错误提示。
    } else if (isStepUpBlocked(error)) {
      appStore.showError(
        stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
          ? t('stepUp.adminApiKeyForbidden')
          : t('stepUp.notEnabled')
      )
    } else {
      appStore.showError(error?.message || t('admin.accounts.dataExportFailed'))
    }
  } finally {
    exportingData.value = false
    showExportDataDialog.value = false
  }
}
const accountExportStepUp = useStepUp()
const closeTestModal = () => { showTest.value = false; testingAcc.value = null }
const closeStatsModal = () => { showStats.value = false; statsAcc.value = null }
const closeReAuthModal = () => { showReAuth.value = false; reAuthAcc.value = null }
const handleOpenEditorFromReAuth = (a: Account) => {
  closeReAuthModal()
  void handleEdit(a)
}
const handleTest = (a: Account) => { testingAcc.value = a; showTest.value = true }
const handleViewStats = (a: Account) => { statsAcc.value = a; showStats.value = true }
const handleSchedule = async (a: Account) => {
  scheduleAcc.value = a
  scheduleModelOptions.value = []
  showSchedulePanel.value = true
  try {
    const models = await adminAPI.accounts.getAvailableModels(a.id)
    scheduleModelOptions.value = models.map((m: ClaudeModel) => ({ value: m.id, label: m.display_name || m.id }))
  } catch {
    scheduleModelOptions.value = []
  }
}
const closeSchedulePanel = () => { showSchedulePanel.value = false; scheduleAcc.value = null; scheduleModelOptions.value = [] }
const handleReAuth = (a: Account) => { reAuthAcc.value = a; showReAuth.value = true }
const duplicatingAccountIDs = new Set<number>()
const handleDuplicateAccount = async (a: Account) => {
  if (duplicatingAccountIDs.has(a.id)) return
  duplicatingAccountIDs.add(a.id)
  try {
    const duplicate = await adminAPI.accounts.duplicate(a.id)
    appStore.showSuccess(t('admin.accounts.duplicateSuccess', { name: duplicate.name }))
    reload()
  } catch (error: any) {
    console.error('Failed to duplicate account:', error)
    appStore.showError(error?.message || t('admin.accounts.duplicateFailed'))
  } finally {
    duplicatingAccountIDs.delete(a.id)
  }
}
const handleRefresh = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.refreshCredentials(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
  } catch (error) {
    console.error('Failed to refresh credentials:', error)
  }
}
const handleRecoverState = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.recoverState(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
    appStore.showSuccess(t('admin.accounts.recoverStateSuccess'))
  } catch (error: any) {
    console.error('Failed to recover account state:', error)
    appStore.showError(error?.message || t('admin.accounts.recoverStateFailed'))
  }
}
const handleResetQuota = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.resetAccountQuota(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
    appStore.showSuccess(t('common.success'))
  } catch (error) {
    console.error('Failed to reset quota:', error)
  }
}
const handleInspectDeviceProfile = (a: Account) => {
  inspectingDeviceAcc.value = a
  showDeviceProfileInspect.value = true
}
const closeDeviceProfileInspect = () => {
  showDeviceProfileInspect.value = false
  inspectingDeviceAcc.value = null
}
const handleResetDeviceProfile = (a: Account) => {
  resettingDeviceAcc.value = a
  showResetDeviceProfileDialog.value = true
}
const confirmResetDeviceProfile = async () => {
  const a = resettingDeviceAcc.value
  if (!a || resettingDeviceProfile.value) return
  resettingDeviceProfile.value = true
  try {
    const updated = await adminAPI.accounts.resetDeviceProfile(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
    showResetDeviceProfileDialog.value = false
    resettingDeviceAcc.value = null
    appStore.showSuccess(t('admin.accounts.resetDeviceProfileSuccess'))
  } catch (error: unknown) {
    console.error('Failed to reset device profile:', error)
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.resetDeviceProfileFailed')))
  } finally {
    resettingDeviceProfile.value = false
  }
}

const privacyResultMessageKey = (account: Account): { type: 'success' | 'error'; key: string } => {
  const mode = typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
  if (account.platform === 'openai') {
    switch (mode) {
      case 'training_off':
        return { type: 'success', key: 'admin.accounts.privacyTrainingOff' }
      case 'training_set_cf_blocked':
        return { type: 'error', key: 'admin.accounts.privacyCfBlocked' }
      default:
        return { type: 'error', key: 'admin.accounts.privacyFailed' }
    }
  }
  if (account.platform === 'antigravity') {
    if (mode === 'privacy_set') {
      return { type: 'success', key: 'admin.accounts.privacyAntigravitySet' }
    }
    return { type: 'error', key: 'admin.accounts.privacyAntigravityFailed' }
  }
  return { type: 'error', key: 'admin.accounts.privacyFailed' }
}

const handleSetPrivacy = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.setPrivacy(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
    const result = privacyResultMessageKey(updated)
    if (result.type === 'success') {
      appStore.showSuccess(t(result.key))
    } else {
      appStore.showError(t(result.key))
    }
  } catch (error: any) {
    console.error('Failed to set privacy:', error)
    appStore.showError(error?.response?.data?.message || t('admin.accounts.privacyFailed'))
  }
}
const onRevertFallback = async (a: Account) => {
  try {
    await adminAPI.accounts.revertProxyFallback(a.id)
    appStore.showSuccess(t('admin.accounts.revertProxySuccess'))
    reload()
  } catch (error: any) {
    console.error('Failed to revert proxy fallback:', error)
    appStore.showError(error?.response?.data?.message || t('admin.accounts.revertProxyFailed'))
  }
}
const handleCreateSparkShadow = (a: Account) => {
  creatingShadowAcc.value = a
  showCreateShadowDialog.value = true
}
const confirmCreateSparkShadow = async () => {
  const a = creatingShadowAcc.value
  if (!a) return
  try {
    await adminAPI.accounts.createSparkShadow(a.id, { name: `${a.name} (Spark)` })
    showCreateShadowDialog.value = false
    creatingShadowAcc.value = null
    appStore.showSuccess(t('admin.accounts.createSparkShadowSuccess'))
    reload()
  } catch (error: any) {
    console.error('Failed to create spark shadow:', error)
    appStore.showError(error?.response?.data?.message || t('admin.accounts.createSparkShadowFailed'))
  }
}
const handleDelete = (a: Account) => { deletingAcc.value = a; showDeleteDialog.value = true }
const confirmDelete = async () => { if(!deletingAcc.value) return; try { await adminAPI.accounts.delete(deletingAcc.value.id); showDeleteDialog.value = false; deletingAcc.value = null; reload() } catch (error) { console.error('Failed to delete account:', error) } }
const handleToggleSchedulable = async (a: Account) => {
  const nextSchedulable = !a.schedulable
  togglingSchedulable.value = a.id
  try {
    const updated = await adminAPI.accounts.setSchedulable(a.id, nextSchedulable)
    updateSchedulableInList([a.id], updated?.schedulable ?? nextSchedulable)
    enterAutoRefreshSilentWindow()
  } catch (error) {
    console.error('Failed to toggle schedulable:', error)
    appStore.showError(t('admin.accounts.failedToToggleSchedulable'))
  } finally {
    togglingSchedulable.value = null
  }
}
const handleShowTempUnsched = (a: Account) => { tempUnschedAcc.value = a; showTempUnsched.value = true }
const handleTempUnschedReset = async (updated: Account) => {
  showTempUnsched.value = false
  tempUnschedAcc.value = null
  patchAccountInList(updated)
  enterAutoRefreshSilentWindow()
}
const formatExpiresAt = (value: number | null) => {
  if (!value) return '-'
  return formatDateTime(
    new Date(value * 1000),
    {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    },
    'sv-SE'
  )
}
const isExpired = (value: number | null) => {
  if (!value) return false
  return value * 1000 <= Date.now()
}
// 所绑定代理的有效期(逻辑同 /admin/proxies,见 utils/proxyExpiry)
const proxyExpiryBadge = (p: AccountProxy): string => proxyExpiryBadgeClass(p.expires_at, p.status)
const proxyExpiryText = (p: AccountProxy): string => {
  const { key, params } = proxyExpiryLabelKey(p.expires_at, p.status)
  return params ? t(key, params) : t(key)
}

// 表格滚动时关闭行操作菜单，并让顶部工具菜单继续贴紧触发按钮。
const handleScroll = () => {
  menu.show = false
  if (showAccountToolsDropdown.value) updateAccountToolsDropdownPosition()
}

const handleViewportResize = () => {
  if (showAccountToolsDropdown.value) updateAccountToolsDropdownPosition()
}

// 点击外部关闭顶部下拉菜单
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (accountToolsDropdownRef.value && !accountToolsDropdownRef.value.contains(target)) {
    showAccountToolsDropdown.value = false
  }
  if (autoRefreshDropdownRef.value && !autoRefreshDropdownRef.value.contains(target)) {
    showAutoRefreshDropdown.value = false
  }
  if (importExportDropdownRef.value && !importExportDropdownRef.value.contains(target)) {
    showImportExportDropdown.value = false
  }
  if (columnsDropdownRef.value && !columnsDropdownRef.value.contains(target)) {
    showColumnsDropdown.value = false
  }
}

onMounted(async () => {
  if (typeof window !== 'undefined') {
    desktopViewportMediaQuery = window.matchMedia(desktopViewportQuery)
    isDesktopViewport.value = desktopViewportMediaQuery.matches
    desktopViewportListener = (event: MediaQueryListEvent) => {
      isDesktopViewport.value = event.matches
    }
    if (typeof desktopViewportMediaQuery.addEventListener === 'function') {
      desktopViewportMediaQuery.addEventListener('change', desktopViewportListener)
    } else {
      desktopViewportMediaQuery.addListener(desktopViewportListener)
    }
  }

  load()
  loadUpstreamBillingProbeGlobalState()
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

  if (autoRefreshEnabled.value) {
    autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
    resumeAutoRefresh()
  } else {
    pauseAutoRefresh()
  }
})

onUnmounted(() => {
  upstreamBillingRateAbortController?.abort()
  if (usageBatchFlushTimer !== null) {
    clearTimeout(usageBatchFlushTimer)
    usageBatchFlushTimer = null
  }
  pendingUsageBatchIds.clear()
  window.removeEventListener('scroll', handleScroll, true)
  window.removeEventListener('resize', handleViewportResize)
  document.removeEventListener('click', handleClickOutside)
  if (desktopViewportMediaQuery && desktopViewportListener) {
    if (typeof desktopViewportMediaQuery.removeEventListener === 'function') {
      desktopViewportMediaQuery.removeEventListener('change', desktopViewportListener)
    } else {
      desktopViewportMediaQuery.removeListener(desktopViewportListener)
    }
  }
  desktopViewportListener = null
  desktopViewportMediaQuery = null
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

/* ---------- Summary chips ---------- */
.acct-summary {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  margin-bottom: 14px;
}

.acct-summary .summary-chip {
  width: 100%;
  text-align: left;
}

/* ---------- Filter row ---------- */
.acct-filter-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-height: 36px;
}

.acct-filter-row :deep(.search-input) {
  width: 260px;
  flex: none;
}

.acct-selection-count {
  margin-left: auto;
  font-size: var(--fs-12-5);
  color: var(--muted);
  white-space: nowrap;
}

.acct-selection-count b {
  color: var(--foreground);
  font-weight: var(--fw-semibold);
  font-variant-numeric: tabular-nums;
}

.acct-icon-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex: none;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  box-shadow: var(--field-shadow);
  color: var(--muted);
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.acct-icon-pill:hover {
  color: var(--foreground);
  border-color: color-mix(in oklch, var(--foreground) 18%, transparent);
}

/* ---------- Menus ---------- */
.acct-menu {
  position: relative;
  display: inline-flex;
  flex: none;
}

.acct-dropdown {
  top: 100%;
  right: 0;
  margin-top: 6px;
  min-width: 200px;
}

.acct-columns-dropdown {
  max-height: 60vh;
  overflow-y: auto;
}

.acct-dropdown-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.acct-dropdown-count {
  margin-left: auto;
  border-radius: var(--radius-pill);
  padding: 1px 7px;
  font-size: var(--fs-11);
  font-weight: var(--fw-semibold);
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}

.acct-tools-dropdown {
  position: fixed;
  z-index: 9999;
}

.acct-tools-scroll {
  overflow-y: auto;
}

.acct-pending-sync {
  margin-top: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-radius: var(--radius-field);
  border: 1px solid color-mix(in oklch, var(--warning) 35%, transparent);
  background: color-mix(in oklch, var(--warning) 12%, transparent);
  color: var(--warning-text);
  font-size: var(--fs-13);
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

  .acct-summary {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .acct-filter-row {
    gap: 8px;
  }

  .acct-selection-count {
    order: 5;
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
