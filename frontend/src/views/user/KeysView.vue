<template>
 <AppLayout>
 <div class="keys-page">
 <PageHeader class="keys-header" :title="t('keys.title')" :description="t('keys.description')">
 <template #actions>
 <Button
 variant="icon"
 :disabled="loading"
 :title="t('common.refresh')"
 :aria-label="t('common.refresh')"
 @click="loadApiKeys"
 >
 <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
 </Button>
 <Button variant="secondary" to="/key-usage" class="keys-header-link">
 {{ t('keys.usageQuery') }}
 </Button>
 <Button
 v-if="!publicSettings?.hide_ccs_import_button"
 variant="secondary"
 class="keys-header-link"
 @click="openCcsImportPicker"
 >
 <Icon name="externalLink" size="sm" />
 {{ t('keys.importToCcSwitch') }}
 </Button>
 <Button class="keys-create-desktop" data-tour="keys-create-btn" @click="showCreateModal = true">
 <Icon name="plus" size="md" />
 {{ t('keys.createKey') }}
 </Button>
 </template>
 </PageHeader>

 <TablePageLayout class="keys-layout">
 <template #filters>
 <div class="keys-toolbar">
 <div class="keys-top-grid" :class="{ 'is-wide': endpointCards.length > 2 }">
 <EndpointCard
 v-for="endpoint in endpointCards"
 :key="endpoint.url"
 class="keys-endpoint"
 :label="endpoint.label"
 :url="endpoint.url"
 :description="endpoint.description"
 :badge="endpoint.badge"
 :badge-tone="endpoint.badgeTone"
 :copy-label="t('keys.endpoints.copy')"
 @copy="copyEndpoint"
 />
 <MiniStatCard class="keys-stats" :items="keyMiniStats" />
 </div>

 <FilterBar class="keys-filter-bar" :filter-label="t('keys.filters.toggle')" @open-filters="showMobileFilters = !showMobileFilters">
 <template #search>
 <SearchInput
 v-model="filterSearch"
 :placeholder="t('keys.searchPlaceholder')"
 class="w-full"
 @search="onFilterChange"
 />
 </template>
 <template #filters>
 <Select
 variant="pill"
 :pill-label="t('keys.group')"
 :aria-label="t('keys.group')"
 :model-value="filterGroupId"
 :options="groupFilterOptions"
 @update:model-value="onGroupFilterChange"
 />
 <SegmentedControl
 class="keys-status-segmented"
 :model-value="String(filterStatus || '')"
 :options="statusSegmentOptions"
 @update:model-value="onStatusFilterChange"
 />
 </template>
 <template #trailing>
 <span class="keys-sort-note">
 {{ t('keys.sortedByPrefix') }} <b>{{ activeSortLabel }}</b> {{ t('keys.sortedBySuffix') }}
 </span>
 <div ref="columnDropdownRef" class="keys-column-settings">
 <button
 type="button"
 class="filter-pill keys-column-btn"
 :title="t('keys.columnSettings')"
 :aria-label="t('keys.columnSettings')"
 @click="showColumnDropdown = !showColumnDropdown"
 >
 <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
 <path d="M9 4.5v15m6-15v15M4.125 19.5h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
 </svg>
 </button>
 <div v-if="showColumnDropdown" class="dropdown keys-column-dropdown">
 <p class="dropdown-label">{{ t('keys.columnSettings') }}</p>
 <button
 v-for="col in toggleableColumns"
 :key="col.key"
 type="button"
 class="dropdown-item"
 :class="{ 'is-active': isColumnVisible(col.key) }"
 @click="toggleColumn(col.key)"
 >
 <span class="keys-column-item-label">{{ col.label }}</span>
 <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" :stroke-width="2" />
 </button>
 </div>
 </div>
 </template>
 </FilterBar>

 <div v-if="showMobileFilters" class="keys-mobile-filters">
 <Select
 variant="pill"
 :pill-label="t('keys.group')"
 :aria-label="t('keys.group')"
 :model-value="filterGroupId"
 :options="groupFilterOptions"
 @update:model-value="onGroupFilterChange"
 />
 </div>

 <ChipScroller
 class="keys-chips"
 :model-value="String(filterStatus || '')"
 :chips="statusChipOptions"
 @update:model-value="onStatusFilterChange"
 />
 </div>
 </template>

 <template #table>
 <div class="keys-table-wrap">
 <DataTable
 v-if="isTabletUp"
 class="keys-table"
 :columns="columns"
 :data="apiKeys"
 :loading="loading"
 :server-side-sort="true"
 default-sort-key="created_at"
 default-sort-order="desc"
 @sort="handleSort"
 >
 <template #cell-name="{ value, row }">
 <div class="keys-cell-name">
 <span class="keys-name-line">
 <span class="keys-name-text">{{ value }}</span>
 <Icon
 v-if="hasIpRestriction(row)"
 name="shield"
 size="xs"
 class="keys-name-shield"
 :title="t('keys.ipRestrictionEnabled')"
 />
 </span>
 <span class="keys-name-id">#{{ row.id }}</span>
 </div>
 </template>

 <template #cell-id="{ value }">
 <span class="keys-name-id">#{{ value }}</span>
 </template>

 <template #cell-key="{ value, row }">
 <div class="keys-cell-key">
 <code class="code keys-key-code">{{ isKeyRevealed(row.id) ? value : maskApiKey(value) }}</code>
 <button
 type="button"
 class="icon-btn keys-icon-btn-xs"
 :title="isKeyRevealed(row.id) ? t('keys.hideKey') : t('keys.showKey')"
 :aria-label="isKeyRevealed(row.id) ? t('keys.hideKey') : t('keys.showKey')"
 @click.stop="toggleKeyReveal(row.id)"
 >
 <Icon :name="isKeyRevealed(row.id) ? 'eyeOff' : 'eye'" size="xs" />
 </button>
 <button
 type="button"
 class="icon-btn keys-icon-btn-xs"
 :class="{ 'is-copied': copiedKeyId === row.id }"
 :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
 :aria-label="t('keys.copyToClipboard')"
 @click.stop="copyToClipboard(value, row.id)"
 >
 <Icon :name="copiedKeyId === row.id ? 'check' : 'clipboard'" size="xs" />
 </button>
 </div>
 </template>

 <template #cell-group="{ row }">
 <div class="group/dropdown relative">
 <button
 type="button"
 :ref="(el) => setGroupButtonRef(row.id, el)"
 class="keys-group-btn"
 :title="t('keys.clickToChangeGroup')"
 @click="openGroupSelector(row)"
 >
 <GroupBadge
 v-if="row.group"
 class="keys-group-badge"
 :name="row.group.name"
 :platform="row.group.platform"
 :subscription-type="row.group.subscription_type"
 :rate-multiplier="row.group.rate_multiplier"
 :user-rate-multiplier="userGroupRates[row.group.id]"
 :peak-rate-enabled="row.group.peak_rate_enabled"
 :peak-start="row.group.peak_start"
 :peak-end="row.group.peak_end"
 :peak-rate-multiplier="row.group.peak_rate_multiplier"
 />
 <span v-else class="tag">{{ t('keys.noGroup') }}</span>
 <Icon name="chevronDown" size="xs" class="keys-group-caret" />
 </button>
 </div>
 </template>

 <template #cell-current_concurrency="{ value }">
 <span class="keys-concurrency">{{ value ?? 0 }}</span>
 </template>

 <template #cell-usage="{ row }">
 <div class="keys-usage">
 <span class="keys-usage-today" :title="`$${(usageStats[row.id]?.today_actual_cost ?? 0).toFixed(4)}`">
 {{ formatCost(usageStats[row.id]?.today_actual_cost) }}
 </span>
 <span class="keys-usage-total" :title="`$${(usageStats[row.id]?.total_actual_cost ?? 0).toFixed(4)}`">
 {{ formatCost(usageStats[row.id]?.total_actual_cost) }}
 </span>
 <span
 v-if="row.quota > 0"
 class="progress progress-thin keys-usage-quota"
 :title="`${t('keys.quota')} ${formatCost(row.quota_used)} / ${formatCost(row.quota)}`"
 >
 <span
 class="progress-bar"
 :class="quotaBarClass(row)"
 :style="{ width: quotaPercent(row) + '%' }"
 />
 </span>
 </div>
 </template>

 <template #cell-rate_limit="{ row }">
 <span
 v-if="rateLimitSummary(row)"
 class="keys-rate-limit"
 :class="rateLimitToneClass(row)"
 :title="rateLimitDetail(row)"
 >
 {{ rateLimitSummary(row) }}
 </span>
 <span v-else class="keys-cell-empty">—</span>
 </template>

 <template #cell-expires_at="{ value }">
 <span class="keys-expiry" :class="expiryToneClass(value)">
 {{ value ? formatDate(value) : t('keys.noExpiration') }}
 </span>
 </template>

 <template #cell-last_used_at="{ value }">
 <span class="keys-muted-cell">{{ value ? formatDate(value) : '—' }}</span>
 </template>

 <template #cell-last_used_ip="{ value }">
 <span class="keys-muted-cell keys-mono-cell">{{ value || '—' }}</span>
 </template>

 <template #cell-created_at="{ value }">
 <span class="keys-muted-cell">{{ formatDate(value) }}</span>
 </template>

 <template #cell-status="{ value }">
 <StatusBadge :tone="statusTone(value)" :label="t('keys.status.' + value)" dot />
 </template>

 <template #cell-actions="{ row }">
 <div class="keys-actions">
 <button type="button" class="keys-use-btn" @click.stop="openUseKeyModal(row)">
 {{ t('keys.use') }}
 </button>
 <button
 type="button"
 class="icon-btn keys-more-btn"
 :title="t('keys.moreActions')"
 :aria-label="t('keys.moreActions')"
 @click.stop="toggleRowMenu(row, $event)"
 >
 <Icon name="more" size="sm" :stroke-width="2.4" />
 </button>
 </div>
 </template>

 <template #empty>
 <EmptyState
 :title="t('keys.noKeysYet')"
 :description="t('keys.createFirstKey')"
 :action-text="t('keys.createKey')"
 @action="showCreateModal = true"
 />
 </template>
 </DataTable>

 <div v-else class="keys-mobile-wrap">
 <div class="keys-mobile-list">
 <template v-if="loading">
 <div v-for="i in 4" :key="i" class="glass-card keys-card">
 <div class="skeleton h-4 w-32"></div>
 <div class="skeleton h-10 w-full"></div>
 <div class="skeleton h-6 w-full"></div>
 </div>
 </template>
 <EmptyState
 v-else-if="apiKeys.length === 0"
 :title="t('keys.noKeysYet')"
 :description="t('keys.createFirstKey')"
 :action-text="t('keys.createKey')"
 @action="showCreateModal = true"
 />
 <article v-for="row in apiKeys" v-else :key="row.id" class="glass-card keys-card">
 <div class="keys-card-top">
 <div class="keys-card-ident">
 <span class="keys-card-name">{{ row.name }}</span>
 <span class="keys-card-meta">#{{ row.id }} · {{ row.group?.name || t('keys.noGroup') }}</span>
 </div>
 <div class="keys-card-top-right">
 <StatusBadge :tone="statusTone(row.status)" :label="t('keys.status.' + row.status)" />
 <button
 type="button"
 class="icon-btn keys-card-more"
 :title="t('keys.moreActions')"
 :aria-label="t('keys.moreActions')"
 @click.stop="toggleRowMenu(row, $event)"
 >
 <Icon name="more" size="sm" :stroke-width="2.4" />
 </button>
 </div>
 </div>
 <div class="keys-card-key">
 <span class="keys-card-key-text">{{ isKeyRevealed(row.id) ? row.key : maskApiKey(row.key) }}</span>
 <button
 type="button"
 class="keys-card-key-btn"
 :title="isKeyRevealed(row.id) ? t('keys.hideKey') : t('keys.showKey')"
 :aria-label="isKeyRevealed(row.id) ? t('keys.hideKey') : t('keys.showKey')"
 @click.stop="toggleKeyReveal(row.id)"
 >
 <Icon :name="isKeyRevealed(row.id) ? 'eyeOff' : 'eye'" size="sm" />
 </button>
 <button
 type="button"
 class="keys-card-key-btn"
 :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
 :aria-label="t('keys.copyToClipboard')"
 @click.stop="copyToClipboard(row.key, row.id)"
 >
 <Icon :name="copiedKeyId === row.id ? 'check' : 'clipboard'" size="sm" />
 </button>
 </div>
 <div class="keys-card-stats">
 <div class="keys-card-stat">
 <span>{{ t('keys.today') }}</span>
 <b>{{ formatCost(usageStats[row.id]?.today_actual_cost) }}</b>
 </div>
 <div class="keys-card-stat">
 <span>{{ t('keys.currentConcurrency') }}</span>
 <b>{{ row.current_concurrency ?? 0 }}</b>
 </div>
 <div class="keys-card-stat">
 <span>{{ t('keys.expiresAt') }}</span>
 <b :class="expiryToneClass(row.expires_at)">
 {{ row.expires_at ? formatDate(row.expires_at) : t('keys.noExpiration') }}
 </b>
 </div>
 <div class="keys-card-stat is-end">
 <span>{{ t('keys.lastUsedAt') }}</span>
 <b>{{ row.last_used_at ? formatDate(row.last_used_at) : '—' }}</b>
 </div>
 </div>
 </article>
 </div>
 <ListFade class="keys-list-fade" />
 </div>

 <Pagination
 v-if="pagination.total > 0"
 class="keys-pagination"
 :page="pagination.page"
 :total="pagination.total"
 :page-size="pagination.page_size"
 @update:page="handlePageChange"
 @update:pageSize="handlePageSizeChange"
 />
 </div>
 </template>
 </TablePageLayout>

 <Fab class="keys-fab" data-tour="keys-create-btn" :label="t('keys.createKey')" @click="showCreateModal = true">
 <Icon name="plus" size="md" :stroke-width="2.4" />
 {{ t('keys.createKey') }}
 </Fab>

 <!-- Create / Edit key -->
 <UiModal
 :open="showCreateModal || showEditModal"
 :title="showEditModal ? t('keys.editKey') : t('keys.createKey')"
 width="lg"
 :close-label="t('common.close')"
 @close="closeModals"
 >
 <form id="key-form" class="keys-form" @submit.prevent="handleSubmit">
 <TextInput
 v-model="formData.name"
 :label="t('keys.nameLabel')"
 :placeholder="t('keys.namePlaceholder')"
 required
 data-tour="key-form-name"
 />

 <div class="keys-field">
 <FieldLabel>{{ t('keys.groupLabel') }}</FieldLabel>
 <UiSelect
 v-model="formData.group_id"
 :options="groupOptions"
 :placeholder="t('keys.selectGroup')"
 :searchable="true"
 :search-placeholder="t('keys.searchGroup')"
 data-tour="key-form-group"
 >
 <template #selected="{ option }">
 <GroupBadge
 v-if="option"
 :name="(option as unknown as GroupOption).label"
 :platform="(option as unknown as GroupOption).platform"
 :subscription-type="(option as unknown as GroupOption).subscriptionType"
 :rate-multiplier="(option as unknown as GroupOption).rate"
 :user-rate-multiplier="(option as unknown as GroupOption).userRate"
 :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
 :peak-start="(option as unknown as GroupOption).peakStart"
 :peak-end="(option as unknown as GroupOption).peakEnd"
 :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
 />
 <span v-else class="text-muted">{{ t('keys.selectGroup') }}</span>
 </template>
 <template #option="{ option, selected }">
 <GroupOptionItem
 :name="(option as unknown as GroupOption).label"
 :platform="(option as unknown as GroupOption).platform"
 :subscription-type="(option as unknown as GroupOption).subscriptionType"
 :rate-multiplier="(option as unknown as GroupOption).rate"
 :user-rate-multiplier="(option as unknown as GroupOption).userRate"
 :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
 :peak-start="(option as unknown as GroupOption).peakStart"
 :peak-end="(option as unknown as GroupOption).peakEnd"
 :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
 :description="(option as unknown as GroupOption).description"
 :selected="selected"
 />
 </template>
 </UiSelect>
 </div>

 <div v-if="!showEditModal" class="keys-field">
 <div class="keys-toggle-row">
 <span class="keys-toggle-label">{{ t('keys.customKeyLabel') }}</span>
 <ToggleSwitch v-model="formData.use_custom_key" />
 </div>
 <TextInput
 v-if="formData.use_custom_key"
 v-model="formData.custom_key"
 class="keys-mono-input"
 :placeholder="t('keys.customKeyPlaceholder')"
 :error="customKeyError"
 :hint="customKeyError ? undefined : t('keys.customKeyHint')"
 />
 </div>

 <div v-if="showEditModal" class="keys-field">
 <FieldLabel>{{ t('keys.statusLabel') }}</FieldLabel>
 <UiSelect
 v-model="formData.status"
 :options="statusOptions"
 :placeholder="t('keys.selectStatus')"
 />
 </div>

 <div class="keys-field">
 <div class="keys-toggle-row">
 <span class="keys-toggle-label">{{ t('keys.ipRestriction') }}</span>
 <ToggleSwitch v-model="formData.enable_ip_restriction" />
 </div>
 <div v-if="formData.enable_ip_restriction" class="keys-field-stack">
 <div>
 <FieldLabel :hint="t('keys.ipWhitelistHint')">{{ t('keys.ipWhitelist') }}</FieldLabel>
 <textarea
 v-model="formData.ip_whitelist"
 rows="3"
 class="field keys-textarea"
 :placeholder="t('keys.ipWhitelistPlaceholder')"
 />
 </div>
 <div>
 <FieldLabel :hint="t('keys.ipBlacklistHint')">{{ t('keys.ipBlacklist') }}</FieldLabel>
 <textarea
 v-model="formData.ip_blacklist"
 rows="3"
 class="field keys-textarea"
 :placeholder="t('keys.ipBlacklistPlaceholder')"
 />
 </div>
 </div>
 </div>

 <div class="keys-field">
 <TextInput
 :model-value="formData.quota ?? ''"
 type="number"
 step="0.01"
 min="0"
 :label="t('keys.quotaLimit')"
 :hint="t('keys.quotaAmountHint')"
 :placeholder="t('keys.quotaAmountPlaceholder')"
 @update:model-value="(v) => (formData.quota = v === '' ? null : Number(v))"
 />
 <div v-if="showEditModal && selectedKey && selectedKey.quota > 0" class="keys-usage-row">
 <div class="glass-inset keys-usage-readout">
 <b>{{ formatCost(selectedKey.quota_used, 4) }}</b>
 <span class="keys-usage-sep">/</span>
 <span class="text-muted">{{ formatCost(selectedKey.quota) }}</span>
 </div>
 <Button variant="secondary" :title="t('keys.resetQuotaUsed')" @click="confirmResetQuota">
 {{ t('keys.reset') }}
 </Button>
 </div>
 </div>

 <div class="keys-field">
 <div class="keys-toggle-row">
 <span class="keys-toggle-label">{{ t('keys.rateLimitSection') }}</span>
 <ToggleSwitch v-model="formData.enable_rate_limit" />
 </div>
 <div v-if="formData.enable_rate_limit" class="keys-field-stack">
 <p class="input-hint">{{ t('keys.rateLimitHint') }}</p>
 <div v-for="window in rateLimitWindows" :key="window.key">
 <TextInput
 :model-value="formData[window.key] ?? ''"
 type="number"
 step="0.01"
 min="0"
 :label="window.label"
 placeholder="0"
 @update:model-value="(v) => (formData[window.key] = v === '' ? null : Number(v))"
 />
 <div v-if="showEditModal && selectedKey && (selectedKey[window.limitField] ?? 0) > 0" class="keys-window-usage">
 <div class="keys-window-readout">
 <b :class="windowToneClass(selectedKey, window)">
 {{ formatCost(selectedKey[window.usageField], 4) }}
 </b>
 <span class="keys-usage-sep">/</span>
 <span class="text-muted">{{ formatCost(selectedKey[window.limitField]) }}</span>
 </div>
 <ProgressBar :value="windowPercent(selectedKey, window)" />
 </div>
 </div>
 <div v-if="showEditModal && selectedKey && hasRateLimit(selectedKey)">
 <Button variant="secondary" @click="confirmResetRateLimit">
 {{ t('keys.resetRateLimitUsage') }}
 </Button>
 </div>
 </div>
 </div>

 <div class="keys-field">
 <div class="keys-toggle-row">
 <span class="keys-toggle-label">{{ t('keys.expiration') }}</span>
 <ToggleSwitch v-model="formData.enable_expiration" />
 </div>
 <div v-if="formData.enable_expiration" class="keys-field-stack">
 <div class="keys-expiry-presets">
 <button
 v-for="days in ['7', '30', '90']"
 :key="days"
 type="button"
 class="chip chip-filter"
 :class="{ 'is-active': formData.expiration_preset === days }"
 @click="setExpirationDays(parseInt(days))"
 >
 {{ showEditModal ? t('keys.extendDays', { days }) : t('keys.expiresInDays', { days }) }}
 </button>
 <button
 type="button"
 class="chip chip-filter"
 :class="{ 'is-active': formData.expiration_preset === 'custom' }"
 @click="formData.expiration_preset = 'custom'"
 >
 {{ t('keys.customDate') }}
 </button>
 </div>
 <TextInput
 v-model="formData.expiration_date"
 type="datetime-local"
 :label="t('keys.expirationDate')"
 :hint="t('keys.expirationDateHint')"
 />
 <p v-if="showEditModal && selectedKey?.expires_at" class="keys-current-expiry">
 <span class="text-muted">{{ t('keys.currentExpiration') }}: </span>
 <b>{{ formatDateTime(selectedKey.expires_at) }}</b>
 </p>
 </div>
 </div>
 </form>
 <template #footer>
 <Button variant="secondary" @click="closeModals">{{ t('common.cancel') }}</Button>
 <Button
 native-type="submit"
 form="key-form"
 :loading="submitting"
 data-tour="key-form-submit"
 >
 {{ submitting ? t('keys.saving') : showEditModal ? t('common.update') : t('common.create') }}
 </Button>
 </template>
 </UiModal>

 <!-- Confirmations -->
 <UiModal
 :open="confirmDialog !== null"
 :title="confirmDialog?.title || ''"
 width="sm"
 :close-label="t('common.close')"
 @close="confirmDialog = null"
 >
 <p class="keys-confirm-text">{{ confirmDialog?.message }}</p>
 <template #footer>
 <Button variant="secondary" @click="confirmDialog = null">{{ t('common.cancel') }}</Button>
 <Button variant="danger" @click="runConfirm">{{ confirmDialog?.confirmText }}</Button>
 </template>
 </UiModal>

 <!-- Use Key Modal -->
 <UseKeyModal
 :show="showUseKeyModal"
 :api-key="selectedKey?.key || ''"
 :base-url="publicSettings?.api_base_url || ''"
 :platform="selectedKey?.group?.platform || null"
 :allow-messages-dispatch="selectedKey?.group?.allow_messages_dispatch || false"
 @close="closeUseKeyModal"
 />

 <!-- CC Switch: pick a key to import -->
 <UiModal
 :open="showCcsImportModal"
 :title="t('keys.ccsImport.title')"
 width="md"
 :close-label="t('common.close')"
 @close="showCcsImportModal = false"
 >
 <p class="keys-confirm-text">{{ t('keys.ccsImport.description') }}</p>
 <ul class="keys-ccs-list">
 <li v-for="row in apiKeys" :key="row.id" class="keys-ccs-item">
 <span class="keys-ccs-name">{{ row.name }}</span>
 <code class="code keys-ccs-key">{{ maskApiKey(row.key) }}</code>
 <Button variant="secondary" @click="importFromPicker(row)">
 {{ t('keys.importToCcSwitch') }}
 </Button>
 </li>
 </ul>
 </UiModal>

 <!-- CC Switch client selection (Antigravity) -->
 <UiModal
 :open="showCcsClientSelect"
 :title="t('keys.ccsClientSelect.title')"
 width="sm"
 :close-label="t('common.close')"
 @close="closeCcsClientSelect"
 >
 <p class="keys-confirm-text">{{ t('keys.ccsClientSelect.description') }}</p>
 <div class="keys-client-grid">
 <button type="button" class="keys-client-card" @click="handleCcsClientSelect('claude')">
 <Icon name="terminal" size="lg" />
 <span class="keys-client-title">{{ t('keys.ccsClientSelect.claudeCode') }}</span>
 <span class="keys-client-desc">{{ t('keys.ccsClientSelect.claudeCodeDesc') }}</span>
 </button>
 <button type="button" class="keys-client-card" @click="handleCcsClientSelect('gemini')">
 <Icon name="sparkles" size="lg" />
 <span class="keys-client-title">{{ t('keys.ccsClientSelect.geminiCli') }}</span>
 <span class="keys-client-desc">{{ t('keys.ccsClientSelect.geminiCliDesc') }}</span>
 </button>
 </div>
 <template #footer>
 <Button variant="secondary" @click="closeCcsClientSelect">{{ t('common.cancel') }}</Button>
 </template>
 </UiModal>

 <!-- Row actions menu -->
 <Teleport to="body">
 <div
 v-if="rowMenuKeyId !== null && rowMenuPosition && rowMenuKey"
 ref="rowMenuRef"
 class="dropdown keys-row-menu"
 :style="{ top: rowMenuPosition.top + 'px', left: rowMenuPosition.left + 'px' }"
 >
 <button type="button" class="dropdown-item" @click="runRowAction(() => openUseKeyModal(rowMenuKey!))">
 <Icon name="terminal" size="sm" />
 {{ t('keys.useKey') }}
 </button>
 <button
 v-if="!publicSettings?.hide_ccs_import_button"
 type="button"
 class="dropdown-item"
 @click="runRowAction(() => importToCcswitch(rowMenuKey!))"
 >
 <Icon name="upload" size="sm" />
 {{ t('keys.importToCcSwitch') }}
 </button>
 <button type="button" class="dropdown-item" @click="runRowAction(() => copyToClipboard(rowMenuKey!.key, rowMenuKey!.id))">
 <Icon name="clipboard" size="sm" />
 {{ t('keys.copyToClipboard') }}
 </button>
 <div class="dropdown-divider" />
 <button type="button" class="dropdown-item" @click="runRowAction(() => editKey(rowMenuKey!))">
 <Icon name="edit" size="sm" />
 {{ t('common.edit') }}
 </button>
 <button type="button" class="dropdown-item" @click="runRowAction(() => toggleKeyStatus(rowMenuKey!))">
 <Icon :name="rowMenuKey.status === 'active' ? 'ban' : 'checkCircle'" size="sm" />
 {{ rowMenuKey.status === 'active' ? t('keys.disable') : t('keys.enable') }}
 </button>
 <button
 v-if="hasRateLimitUsage(rowMenuKey)"
 type="button"
 class="dropdown-item"
 @click="runRowAction(() => confirmResetRateLimitFromTable(rowMenuKey!))"
 >
 <Icon name="refresh" size="sm" />
 {{ t('keys.resetRateLimitUsage') }}
 </button>
 <div class="dropdown-divider" />
 <button type="button" class="dropdown-item dropdown-item-danger" @click="runRowAction(() => confirmDelete(rowMenuKey!))">
 <Icon name="trash" size="sm" />
 {{ t('common.delete') }}
 </button>
 </div>
 </Teleport>

 <!-- Group Selector Dropdown (Teleported to body to avoid overflow clipping) -->
 <Teleport to="body">
 <div
 v-if="groupSelectorKeyId !== null && dropdownPosition"
 ref="dropdownRef"
 class="dropdown keys-group-dropdown"
 :style="{
 top: dropdownPosition.top !== undefined ? dropdownPosition.top + 'px' : undefined,
 bottom: dropdownPosition.bottom !== undefined ? dropdownPosition.bottom + 'px' : undefined,
 left: dropdownPosition.left + 'px'
 }"
 >
 <div class="keys-group-search">
 <Icon name="search" size="sm" class="text-muted" />
 <input
 v-model="groupSearchQuery"
 type="text"
 class="keys-group-search-input"
 :placeholder="t('keys.searchGroup')"
 @click.stop
 />
 </div>
 <div class="keys-group-options">
 <button
 v-for="option in filteredGroupOptions"
 :key="option.value ?? 'null'"
 type="button"
 class="dropdown-item keys-group-option"
 :class="{
 'is-active':
 selectedKeyForGroup?.group_id === option.value ||
 (!selectedKeyForGroup?.group_id && option.value === null)
 }"
 :title="option.description || undefined"
 @click="changeGroup(selectedKeyForGroup!, option.value)"
 >
 <GroupOptionItem
 :name="option.label"
 :platform="option.platform"
 :subscription-type="option.subscriptionType"
 :rate-multiplier="option.rate"
 :user-rate-multiplier="option.userRate"
 :peak-rate-enabled="option.peakRateEnabled"
 :peak-start="option.peakStart"
 :peak-end="option.peakEnd"
 :peak-rate-multiplier="option.peakRateMultiplier"
 :description="option.description"
 :selected="
 selectedKeyForGroup?.group_id === option.value ||
 (!selectedKeyForGroup?.group_id && option.value === null)
 "
 />
 </button>
 <p v-if="filteredGroupOptions.length === 0" class="keys-group-empty">
 {{ t('keys.noGroupFound') }}
 </p>
 </div>
 </div>
 </Teleport>
 </div>
 </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'
import { useClipboard } from '@/composables/useClipboard'
import { useIsMobile } from '@/composables/useIsMobile'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { keysAPI, authAPI, usageAPI, userGroupsAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FieldLabel from '@/components/ui/FieldLabel.vue'
import TextInput from '@/components/ui/TextInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import ProgressBar from '@/components/ui/ProgressBar.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import UiModal from '@/components/ui/UiModal.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import ChipScroller from '@/components/ui/ChipScroller.vue'
import EndpointCard from '@/components/ui/EndpointCard.vue'
import MiniStatCard from '@/components/ui/MiniStatCard.vue'
import ListFade from '@/components/ui/ListFade.vue'
import Fab from '@/components/ui/Fab.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type { ApiKey, Group, PublicSettings, SubscriptionType, GroupPlatform, UpdateApiKeyRequest } from '@/types'
import type { Column } from '@/components/common/types'
import type { StatusBadgeTone } from '@/components/ui/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import { formatDateTime, formatDate } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'

const { t } = useI18n()
const { isTabletUp } = useIsMobile()

import {
  buildCcSwitchImportDeeplink,
  type CcSwitchClientType
} from '@/utils/ccswitchImport'

// Helper to format date for datetime-local input
const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

interface GroupOption {
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
}

const appStore = useAppStore()
const onboardingStore = useOnboardingStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

const allColumns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name'), sortable: true },
  { key: 'id', label: t('keys.id'), sortable: true },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'group', label: t('keys.group'), sortable: false },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: true },
  { key: 'usage', label: t('keys.usage'), sortable: false },
  { key: 'rate_limit', label: t('keys.rateLimitColumn'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'last_used_at', label: t('keys.lastUsedAt'), sortable: true },
  { key: 'last_used_ip', label: t('keys.lastUsedIP'), sortable: false },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const ALWAYS_VISIBLE_COLUMNS = new Set(['name', 'actions'])
const DEFAULT_HIDDEN_COLUMNS = ['id', 'rate_limit', 'last_used_at', 'last_used_ip']
const HIDDEN_COLUMNS_KEY = 'api-key-hidden-columns'
const COLUMN_SETTINGS_VERSION_KEY = 'api-key-column-settings-version'
const COLUMN_SETTINGS_VERSION = 3
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['last_used_ip'],
  3: ['id']
}

const toggleableColumns = computed(() =>
  allColumns.value.filter((col) => !ALWAYS_VISIBLE_COLUMNS.has(col.key))
)

const hiddenColumns = reactive<Set<string>>(new Set())

const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
  } catch (error) {
    console.error('Failed to save API key table columns:', error)
  }
}

const loadSavedColumns = () => {
  hiddenColumns.clear()
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      const validColumnKeys = new Set(allColumns.value.map((col) => col.key))
      parsed
        .filter((key) =>
          typeof key === 'string' &&
          validColumnKeys.has(key) &&
          !ALWAYS_VISIBLE_COLUMNS.has(key)
        )
        .forEach((key) => hiddenColumns.add(key))
      const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (validColumnKeys.has(key) && !ALWAYS_VISIBLE_COLUMNS.has(key)) {
              hiddenColumns.add(key)
            }
          }
        }
        saveColumnsToStorage()
      } else {
        localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    }
  } catch (error) {
    console.error('Failed to load API key table columns:', error)
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
  }
}

const toggleColumn = (key: string) => {
  if (ALWAYS_VISIBLE_COLUMNS.has(key)) return
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const columns = computed<Column[]>(() =>
  allColumns.value.filter((col) => ALWAYS_VISIBLE_COLUMNS.has(col.key) || !hiddenColumns.has(col.key))
)

const apiKeys = ref<ApiKey[]>([])
const groups = ref<Group[]>([])
const loading = ref(false)
const submitting = ref(false)
const now = ref(new Date())
let resetTimer: ReturnType<typeof setInterval> | null = null
const usageStats = ref<Record<string, BatchApiKeyUsageStats>>({})
const userGroupRates = ref<Record<number, number>>({})

const pagination = ref({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = ref({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// Filter state
const filterSearch = ref('')
const filterStatus = ref('')
const filterGroupId = ref<string | number>('')

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showUseKeyModal = ref(false)
const showCcsClientSelect = ref(false)
const showCcsImportModal = ref(false)
const showColumnDropdown = ref(false)
const showMobileFilters = ref(false)
const pendingCcsRow = ref<ApiKey | null>(null)
const selectedKey = ref<ApiKey | null>(null)
const copiedKeyId = ref<number | null>(null)
const groupSelectorKeyId = ref<number | null>(null)
const publicSettings = ref<PublicSettings | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const columnDropdownRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<{ top?: number; bottom?: number; left: number } | null>(null)
const groupButtonRefs = ref<Map<number, HTMLElement>>(new Map())
const revealedKeyIds = reactive<Set<number>>(new Set())
const rowMenuKeyId = ref<number | null>(null)
const rowMenuPosition = ref<{ top: number; left: number } | null>(null)
const rowMenuRef = ref<HTMLElement | null>(null)
const confirmDialog = ref<{
  title: string
  message: string
  confirmText: string
  onConfirm: () => void | Promise<void>
} | null>(null)

const runConfirm = async () => {
  const dialog = confirmDialog.value
  if (!dialog) return
  confirmDialog.value = null
  await dialog.onConfirm()
}
let abortController: AbortController | null = null

// Get the currently selected key for group change
const selectedKeyForGroup = computed(() => {
  if (groupSelectorKeyId.value === null) return null
  return apiKeys.value.find((k) => k.id === groupSelectorKeyId.value) || null
})

// Key currently targeted by the row "more actions" menu
const rowMenuKey = computed<ApiKey | null>(() => {
  if (rowMenuKeyId.value === null) return null
  return apiKeys.value.find((k) => k.id === rowMenuKeyId.value) || null
})

const setGroupButtonRef = (keyId: number, el: Element | ComponentPublicInstance | null) => {
  if (el instanceof HTMLElement) {
    groupButtonRefs.value.set(keyId, el)
  } else {
    groupButtonRefs.value.delete(keyId)
  }
}

const formData = ref({
  name: '',
  group_id: null as number | null,
  status: 'active' as 'active' | 'inactive',
  use_custom_key: false,
  custom_key: '',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  // Quota settings (empty = unlimited)
  enable_quota: false,
  quota: null as number | null,
  // Rate limit settings
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_preset: '30' as '7' | '30' | '90' | 'custom',
  expiration_date: ''
})

// 自定义Key验证
const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  // 检查字符：只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

const shouldSubmitEditStatus = (key: ApiKey, status: 'active' | 'inactive') => {
  if (key.status === 'quota_exhausted' || key.status === 'expired') {
    return status === 'active'
  }
  return true
}

// Filter dropdown options
const groupFilterOptions = computed(() => [
  { value: '', label: t('keys.allGroups') },
  { value: 0, label: t('keys.noGroup') },
  ...groups.value.map((g) => ({ value: g.id, label: g.name }))
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('keys.allStatus') },
  { value: 'active', label: t('keys.status.active') },
  { value: 'inactive', label: t('keys.status.inactive') },
  { value: 'disabled', label: t('keys.status.disabled') },
  { value: 'quota_exhausted', label: t('keys.status.quota_exhausted') },
  { value: 'expired', label: t('keys.status.expired') }
])

const statusChipOptions = computed(() => statusFilterOptions.value.map((opt) => ({
  value: String(opt.value),
  label: opt.label
})))

// Compact segmented control only surfaces the highest-signal statuses;
// the chip scroller (statusChipOptions) exposes the full list on mobile.
const statusSegmentOptions = computed(() => [
  { value: '', label: t('keys.allStatus') },
  { value: 'active', label: t('keys.status.active') },
  { value: 'expired', label: t('keys.status.expired') },
  { value: 'disabled', label: t('keys.status.disabled') }
])

const activeSortLabel = computed(() => {
  const column = allColumns.value.find((col) => col.key === sortState.value.sort_by)
  return column ? column.label : sortState.value.sort_by
})

const activeKeyCount = computed(() => apiKeys.value.filter((key) => key.status === 'active').length)
const todayKeySpend = computed(() =>
  apiKeys.value.reduce((sum, key) => sum + (usageStats.value[key.id]?.today_actual_cost ?? 0), 0)
)
const keyMiniStats = computed(() => [
  { label: t('common.total'), value: pagination.value.total },
  { label: t('common.active'), value: activeKeyCount.value },
  { label: t('keys.today'), value: `$${todayKeySpend.value.toFixed(2)}` }
])

const copyEndpoint = async (url: string) => {
  await clipboardCopy(url, t('keys.endpoints.copied'))
}

interface EndpointCardEntry {
  url: string
  label: string
  description?: string
  badge?: string
  badgeTone?: StatusBadgeTone
}

const endpointCards = computed<EndpointCardEntry[]>(() => {
  const cards: EndpointCardEntry[] = []
  if (publicSettings.value?.api_base_url) {
    cards.push({
      url: publicSettings.value.api_base_url,
      label: t('keys.endpoints.title'),
      badge: t('keys.endpoints.default'),
      badgeTone: 'accent'
    })
  }
  for (const endpoint of publicSettings.value?.custom_endpoints || []) {
    cards.push({
      url: endpoint.endpoint,
      label: endpoint.name,
      description: endpoint.description || undefined,
      badge: t('keys.endpoints.custom'),
      badgeTone: 'muted'
    })
  }
  return cards
})

const openCcsImportPicker = () => {
  showCcsImportModal.value = true
}

const importFromPicker = (row: ApiKey) => {
  showCcsImportModal.value = false
  importToCcswitch(row)
}

const onFilterChange = () => {
  pagination.value.page = 1
  loadApiKeys()
}

const onGroupFilterChange = (value: string | number | boolean | null) => {
  filterGroupId.value = value as string | number
  onFilterChange()
}

const onStatusFilterChange = (value: string | number | boolean | null) => {
  filterStatus.value = value as string
  onFilterChange()
}

// Convert groups to Select options format with rate multiplier and subscription type
const groupOptions = computed(() =>
  groups.value.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    rate: group.rate_multiplier,
    userRate: userGroupRates.value[group.id] ?? null,
    peakRateEnabled: group.peak_rate_enabled,
    peakStart: group.peak_start,
    peakEnd: group.peak_end,
    peakRateMultiplier: group.peak_rate_multiplier,
    subscriptionType: group.subscription_type,
    platform: group.platform
  }))
)

// Group dropdown search
const groupSearchQuery = ref('')
const filteredGroupOptions = computed(() => {
  const query = groupSearchQuery.value.trim().toLowerCase()
  if (!query) return groupOptions.value
  return groupOptions.value.filter((opt) => {
    return opt.label.toLowerCase().includes(query) ||
      (opt.description && opt.description.toLowerCase().includes(query))
  })
})

const copyToClipboard = async (text: string, keyId: number) => {
  const success = await clipboardCopy(text, t('keys.copied'))
  if (success) {
    copiedKeyId.value = keyId
    setTimeout(() => {
      copiedKeyId.value = null
    }, 800)
  }
}

const hasIpRestriction = (key: ApiKey) => (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)

const isKeyRevealed = (id: number) => revealedKeyIds.has(id)

const toggleKeyReveal = (id: number) => {
  if (revealedKeyIds.has(id)) {
    revealedKeyIds.delete(id)
  } else {
    revealedKeyIds.add(id)
  }
}

const formatCost = (value: number | null | undefined, decimals = 2) => `$${(value ?? 0).toFixed(decimals)}`

const quotaPercent = (key: ApiKey) => {
  if (!key.quota || key.quota <= 0) return 0
  return Math.min(100, (key.quota_used / key.quota) * 100)
}

const quotaBarClass = (key: ApiKey) => {
  const pct = quotaPercent(key)
  if (pct >= 90) return 'progress-bar-danger'
  if (pct >= 70) return 'progress-bar-warning'
  return ''
}

const statusTone = (status: ApiKey['status']): StatusBadgeTone => {
  if (status === 'active') return 'success'
  if (status === 'quota_exhausted') return 'warning'
  if (status === 'expired' || status === 'disabled' || status === 'inactive') return 'danger'
  return 'muted'
}

const expiryToneClass = (value: string | null) => (value && new Date(value) < now.value ? 'keys-tone-danger' : '')

interface RateLimitWindow {
  key: 'rate_limit_5h' | 'rate_limit_1d' | 'rate_limit_7d'
  label: string
  shortLabel: string
  limitField: 'rate_limit_5h' | 'rate_limit_1d' | 'rate_limit_7d'
  usageField: 'usage_5h' | 'usage_1d' | 'usage_7d'
  resetField: 'reset_5h_at' | 'reset_1d_at' | 'reset_7d_at'
}

const rateLimitWindows = computed<RateLimitWindow[]>(() => [
  {
    key: 'rate_limit_5h',
    label: t('keys.rateLimit5h'),
    shortLabel: '5h',
    limitField: 'rate_limit_5h',
    usageField: 'usage_5h',
    resetField: 'reset_5h_at'
  },
  {
    key: 'rate_limit_1d',
    label: t('keys.rateLimit1d'),
    shortLabel: '1d',
    limitField: 'rate_limit_1d',
    usageField: 'usage_1d',
    resetField: 'reset_1d_at'
  },
  {
    key: 'rate_limit_7d',
    label: t('keys.rateLimit7d'),
    shortLabel: '7d',
    limitField: 'rate_limit_7d',
    usageField: 'usage_7d',
    resetField: 'reset_7d_at'
  }
])

const windowPercent = (key: ApiKey, window: RateLimitWindow) => {
  const limit = key[window.limitField]
  if (!limit || limit <= 0) return 0
  return Math.min(100, (key[window.usageField] / limit) * 100)
}

const windowToneClass = (key: ApiKey, window: RateLimitWindow) => {
  const pct = windowPercent(key, window)
  if (pct >= 90) return 'keys-tone-danger'
  if (pct >= 70) return 'keys-tone-warning'
  return ''
}

const hasRateLimit = (key: ApiKey) => key.usage_5h > 0 || key.usage_1d > 0 || key.usage_7d > 0

const hasRateLimitUsage = (key: ApiKey | null) => !!key && hasRateLimit(key)

const activeRateLimitWindows = (key: ApiKey) =>
  rateLimitWindows.value.filter((window) => (key[window.limitField] ?? 0) > 0)

const mostConstrainedRateLimitWindow = (key: ApiKey): RateLimitWindow | null => {
  const windows = activeRateLimitWindows(key)
  if (windows.length === 0) return null
  return windows.reduce((worst, window) =>
    windowPercent(key, window) > windowPercent(key, worst) ? window : worst
  , windows[0])
}

const rateLimitSummary = (key: ApiKey) => {
  const window = mostConstrainedRateLimitWindow(key)
  if (!window) return ''
  return `${window.shortLabel} ${Math.round(windowPercent(key, window))}%`
}

const rateLimitToneClass = (key: ApiKey) => {
  const window = mostConstrainedRateLimitWindow(key)
  return window ? windowToneClass(key, window) : ''
}

const rateLimitDetail = (key: ApiKey) => {
  const windows = activeRateLimitWindows(key)
  if (windows.length === 0) return ''
  return windows
    .map((window) => {
      const base = `${window.shortLabel}: ${formatCost(key[window.usageField])} / ${formatCost(key[window.limitField])}`
      const resetAt = key[window.resetField]
      const resetText = resetAt ? formatResetTime(resetAt) : ''
      return resetText ? `${base} · ${resetText}` : base
    })
    .join('\n')
}

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const { name, code } = error as { name?: string; code?: string }
  return name === 'AbortError' || code === 'ERR_CANCELED'
}

const loadApiKeys = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  const { signal } = controller
  loading.value = true
  try {
    // Build filters
    const filters: {
      search?: string
      status?: string
      group_id?: number | string
      sort_by?: string
      sort_order?: 'asc' | 'desc'
    } = {}
    if (filterSearch.value) filters.search = filterSearch.value
    if (filterStatus.value) filters.status = filterStatus.value
    if (filterGroupId.value !== '') filters.group_id = filterGroupId.value
    filters.sort_by = sortState.value.sort_by
    filters.sort_order = sortState.value.sort_order

    const response = await keysAPI.list(pagination.value.page, pagination.value.page_size, filters, {
      signal
    })
    if (signal.aborted) return
    apiKeys.value = response.items
    pagination.value.total = response.total
    pagination.value.pages = response.pages

    // Load usage stats for all API keys in the list
    if (response.items.length > 0) {
      const keyIds = response.items.map((k) => k.id)
      try {
        const usageResponse = await usageAPI.getDashboardApiKeysUsage(keyIds, { signal })
        if (signal.aborted) return
        usageStats.value = usageResponse.stats
      } catch (e) {
        if (!isAbortError(e)) {
          console.error('Failed to load usage stats:', e)
        }
      }
    }
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('keys.failedToLoad'))
  } finally {
    if (abortController === controller) {
      loading.value = false
    }
  }
}

const loadGroups = async () => {
  try {
    groups.value = await userGroupsAPI.getAvailable()
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

const loadUserGroupRates = async () => {
  try {
    userGroupRates.value = await userGroupsAPI.getUserGroupRates()
  } catch (error) {
    console.error('Failed to load user group rates:', error)
  }
}

const loadPublicSettings = async () => {
  try {
    publicSettings.value = await authAPI.getPublicSettings()
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
}

const openUseKeyModal = (key: ApiKey) => {
  selectedKey.value = key
  showUseKeyModal.value = true
}

const closeUseKeyModal = () => {
  showUseKeyModal.value = false
  selectedKey.value = null
}

const handlePageChange = (page: number) => {
  pagination.value.page = page
  loadApiKeys()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadApiKeys()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.value.sort_by = key
  sortState.value.sort_order = order
  pagination.value.page = 1
  loadApiKeys()
}

const editKey = (key: ApiKey) => {
  selectedKey.value = key
  const hasIPRestriction = (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)
  const hasExpiration = !!key.expires_at
  formData.value = {
    name: key.name,
    group_id: key.group_id,
    status: key.status === 'active' ? 'active' : 'inactive',
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: hasIPRestriction,
    ip_whitelist: (key.ip_whitelist || []).join('\n'),
    ip_blacklist: (key.ip_blacklist || []).join('\n'),
    enable_quota: key.quota > 0,
    quota: key.quota > 0 ? key.quota : null,
    enable_rate_limit: (key.rate_limit_5h > 0) || (key.rate_limit_1d > 0) || (key.rate_limit_7d > 0),
    rate_limit_5h: key.rate_limit_5h || null,
    rate_limit_1d: key.rate_limit_1d || null,
    rate_limit_7d: key.rate_limit_7d || null,
    enable_expiration: hasExpiration,
    expiration_preset: 'custom',
    expiration_date: key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  }
  showEditModal.value = true
}

const toggleKeyStatus = async (key: ApiKey) => {
  const newStatus = key.status === 'active' ? 'inactive' : 'active'
  try {
    await keysAPI.toggleStatus(key.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('keys.keyEnabledSuccess') : t('keys.keyDisabledSuccess')
    )
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToUpdateStatus'))
  }
}

const openGroupSelector = (key: ApiKey) => {
  if (groupSelectorKeyId.value === key.id) {
    groupSelectorKeyId.value = null
    dropdownPosition.value = null
  } else {
    const buttonEl = groupButtonRefs.value.get(key.id)
    if (buttonEl) {
      const rect = buttonEl.getBoundingClientRect()
      const dropdownEstHeight = 400 // estimated max dropdown height
      const dropdownEstWidth = Math.min(380, window.innerWidth - 16)
      const spaceBelow = window.innerHeight - rect.bottom
      const spaceAbove = rect.top
      // 夹取 left，避免窄屏下浮层超出视口右缘
      const left = Math.max(8, Math.min(rect.left, window.innerWidth - dropdownEstWidth - 8))

      if (spaceBelow < dropdownEstHeight && spaceAbove > spaceBelow) {
        // Not enough space below, pop upward
        dropdownPosition.value = {
          bottom: window.innerHeight - rect.top + 4,
          left
        }
      } else {
        // Default: pop downward
        dropdownPosition.value = {
          top: rect.bottom + 4,
          left
        }
      }
    }
    groupSelectorKeyId.value = key.id
    groupSearchQuery.value = ''
  }
}

const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  groupSelectorKeyId.value = null
  dropdownPosition.value = null
  if (key.group_id === newGroupId) return

  try {
    await keysAPI.update(key.id, { group_id: newGroupId })
    appStore.showSuccess(t('keys.groupChangedSuccess'))
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToChangeGroup'))
  }
}

const closeGroupSelector = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  // Check if click is inside the dropdown or the trigger button
  if (!target.closest('.group\\/dropdown') && !dropdownRef.value?.contains(target)) {
    groupSelectorKeyId.value = null
    dropdownPosition.value = null
  }
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false
  }
  if (rowMenuKeyId.value !== null && !rowMenuRef.value?.contains(target)) {
    closeRowMenu()
  }
}

const closeRowMenu = () => {
  rowMenuKeyId.value = null
  rowMenuPosition.value = null
}

const toggleRowMenu = (row: ApiKey, event: MouseEvent) => {
  if (rowMenuKeyId.value === row.id) {
    closeRowMenu()
    return
  }
  const buttonEl = event.currentTarget as HTMLElement
  const rect = buttonEl.getBoundingClientRect()
  const menuEstWidth = 220
  const menuEstHeight = 280
  const left = Math.max(8, Math.min(rect.right - menuEstWidth, window.innerWidth - menuEstWidth - 8))
  const spaceBelow = window.innerHeight - rect.bottom
  const top = spaceBelow < menuEstHeight && rect.top > spaceBelow
    ? Math.max(8, rect.top - menuEstHeight)
    : rect.bottom + 4
  rowMenuPosition.value = { top, left }
  rowMenuKeyId.value = row.id
}

// Runs a row-menu action while its target key is still resolvable, then closes the menu.
const runRowAction = (action: () => void) => {
  action()
  closeRowMenu()
}

const confirmDelete = (key: ApiKey) => {
  selectedKey.value = key
  confirmDialog.value = {
    title: t('keys.deleteKey'),
    message: t('keys.deleteConfirmMessage', { name: key.name }),
    confirmText: t('common.delete'),
    onConfirm: handleDelete
  }
}

const handleSubmit = async () => {
  // Validate group_id is required
  if (formData.value.group_id === null) {
    appStore.showError(t('keys.groupRequired'))
    return
  }

  // Validate custom key if enabled
  if (!showEditModal.value && formData.value.use_custom_key) {
    if (!formData.value.custom_key) {
      appStore.showError(t('keys.customKeyRequired'))
      return
    }
    if (customKeyError.value) {
      appStore.showError(customKeyError.value)
      return
    }
  }

  // Parse IP lists only if IP restriction is enabled
  const parseIPList = (text: string): string[] =>
    text.split('\n').map(ip => ip.trim()).filter(ip => ip.length > 0)
  const ipWhitelist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_whitelist) : []
  const ipBlacklist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_blacklist) : []

  // Calculate quota value (null/empty/0 = unlimited, stored as 0)
  const quota = formData.value.quota && formData.value.quota > 0 ? formData.value.quota : 0

  // Calculate expiration
  let expiresInDays: number | undefined
  let expiresAt: string | null | undefined
  if (formData.value.enable_expiration && formData.value.expiration_date) {
    if (!showEditModal.value) {
      // Create mode: calculate days from date
      const expDate = new Date(formData.value.expiration_date)
      const now = new Date()
      const diffDays = Math.ceil((expDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
      expiresInDays = diffDays > 0 ? diffDays : 1
    } else {
      // Edit mode: use custom date directly
      expiresAt = new Date(formData.value.expiration_date).toISOString()
    }
  } else if (showEditModal.value) {
    // Edit mode: if expiration disabled or date cleared, send empty string to clear
    expiresAt = ''
  }

  // Calculate rate limit values (send 0 when toggle is off)
  const rateLimitData = formData.value.enable_rate_limit ? {
    rate_limit_5h: formData.value.rate_limit_5h && formData.value.rate_limit_5h > 0 ? formData.value.rate_limit_5h : 0,
    rate_limit_1d: formData.value.rate_limit_1d && formData.value.rate_limit_1d > 0 ? formData.value.rate_limit_1d : 0,
    rate_limit_7d: formData.value.rate_limit_7d && formData.value.rate_limit_7d > 0 ? formData.value.rate_limit_7d : 0,
  } : { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }

  submitting.value = true
  try {
    if (showEditModal.value && selectedKey.value) {
      const updates: UpdateApiKeyRequest = {
        name: formData.value.name,
        group_id: formData.value.group_id,
        ip_whitelist: ipWhitelist,
        ip_blacklist: ipBlacklist,
        quota: quota,
        expires_at: expiresAt,
        rate_limit_5h: rateLimitData.rate_limit_5h,
        rate_limit_1d: rateLimitData.rate_limit_1d,
        rate_limit_7d: rateLimitData.rate_limit_7d,
      }
      if (shouldSubmitEditStatus(selectedKey.value, formData.value.status)) {
        updates.status = formData.value.status
      }
      await keysAPI.update(selectedKey.value.id, updates)
      appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    } else {
      const customKey = formData.value.use_custom_key ? formData.value.custom_key : undefined
      await keysAPI.create(
        formData.value.name,
        formData.value.group_id,
        customKey,
        ipWhitelist,
        ipBlacklist,
        quota,
        expiresInDays,
        rateLimitData
      )
      appStore.showSuccess(t('keys.keyCreatedSuccess'))
      // Only advance tour if active, on submit step, and creation succeeded
      if (onboardingStore.isCurrentStep('[data-tour="key-form-submit"]')) {
        onboardingStore.nextStep(500)
      }
    }
    closeModals()
    loadApiKeys()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToSave')
    appStore.showError(errorMsg)
    // Don't advance tour on error
  } finally {
    submitting.value = false
  }
}

/**
 * 处理删除 API Key 的操作
 * 优化：错误处理改进，优先显示后端返回的具体错误消息（如权限不足等），
 * 若后端未返回消息则显示默认的国际化文本
 */
const handleDelete = async () => {
  if (!selectedKey.value) return

  try {
    await keysAPI.delete(selectedKey.value.id)
    appStore.showSuccess(t('keys.keyDeletedSuccess'))
    loadApiKeys()
  } catch (error: any) {
    // 优先使用后端返回的错误消息，提供更具体的错误信息给用户
    const errorMsg = error?.message || t('keys.failedToDelete')
    appStore.showError(errorMsg)
  }
}

const closeModals = () => {
  showCreateModal.value = false
  showEditModal.value = false
  selectedKey.value = null
  formData.value = {
    name: '',
    group_id: null,
    status: 'active',
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: false,
    ip_whitelist: '',
    ip_blacklist: '',
    enable_quota: false,
    quota: null,
    enable_rate_limit: false,
    rate_limit_5h: null,
    rate_limit_1d: null,
    rate_limit_7d: null,
    enable_expiration: false,
    expiration_preset: '30',
    expiration_date: ''
  }
}

// Show reset quota confirmation dialog
const confirmResetQuota = () => {
  if (!selectedKey.value) return
  confirmDialog.value = {
    title: t('keys.resetQuotaTitle'),
    message: t('keys.resetQuotaConfirmMessage', {
      name: selectedKey.value.name,
      used: selectedKey.value.quota_used?.toFixed(4)
    }),
    confirmText: t('keys.reset'),
    onConfirm: resetQuotaUsed
  }
}

// Set expiration date based on quick select days
const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const expDate = new Date()
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

// Reset quota used for an API key
const resetQuotaUsed = async () => {
  if (!selectedKey.value) return
  try {
    await keysAPI.update(selectedKey.value.id, { reset_quota: true })
    appStore.showSuccess(t('keys.quotaResetSuccess'))
    // Update local state
    if (selectedKey.value) {
      selectedKey.value.quota_used = 0
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetQuota')
    appStore.showError(errorMsg)
  }
}

// Show reset rate limit confirmation dialog (from edit modal)
const confirmResetRateLimit = () => {
  if (!selectedKey.value) return
  confirmDialog.value = {
    title: t('keys.resetRateLimitTitle'),
    message: t('keys.resetRateLimitConfirmMessage', { name: selectedKey.value.name }),
    confirmText: t('keys.reset'),
    onConfirm: resetRateLimitUsage
  }
}

// Show reset rate limit confirmation dialog (from table row)
const confirmResetRateLimitFromTable = (row: ApiKey) => {
  selectedKey.value = row
  confirmResetRateLimit()
}

// Reset rate limit usage for an API key
const resetRateLimitUsage = async () => {
  if (!selectedKey.value) return
  try {
    await keysAPI.update(selectedKey.value.id, { reset_rate_limit_usage: true })
    appStore.showSuccess(t('keys.rateLimitResetSuccess'))
    // Refresh key data
    await loadApiKeys()
    // Update the editing key with fresh data
    const refreshedKey = apiKeys.value.find(k => k.id === selectedKey.value!.id)
    if (refreshedKey) {
      selectedKey.value = refreshedKey
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
    appStore.showError(errorMsg)
  }
}

const importToCcswitch = (row: ApiKey) => {
  const platform = row.group?.platform || 'anthropic'

  // For antigravity platform, show client selection dialog
  if (platform === 'antigravity') {
    pendingCcsRow.value = row
    showCcsClientSelect.value = true
    return
  }

  // For other platforms, execute directly
  executeCcsImport(row, platform === 'gemini' ? 'gemini' : 'claude')
}

const executeCcsImport = (row: ApiKey, clientType: CcSwitchClientType) => {
  const baseUrl = publicSettings.value?.api_base_url || window.location.origin
  const platform = row.group?.platform || 'anthropic'

  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`
  const providerName = (publicSettings.value?.site_name || 'sub2api').trim() || 'sub2api'
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl,
    platform,
    clientType,
    providerName,
    apiKey: row.key,
    usageScript
  })

  try {
    window.open(deeplink, '_self')

    // Check if the protocol handler worked by detecting if we're still focused
    setTimeout(() => {
      if (document.hasFocus()) {
        // Still focused means the protocol handler likely failed
        appStore.showError(t('keys.ccSwitchNotInstalled'))
      }
    }, 100)
  } catch (error) {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  }
}

const handleCcsClientSelect = (clientType: CcSwitchClientType) => {
  if (pendingCcsRow.value) {
    executeCcsImport(pendingCcsRow.value, clientType)
  }
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

const closeCcsClientSelect = () => {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

function formatResetTime(resetAt: string | null): string {
  if (!resetAt) return ''
  const diff = new Date(resetAt).getTime() - now.value.getTime()
  if (diff <= 0) return t('keys.resetNow')
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.floor((diff % 3600000) / 60000)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

onMounted(() => {
  loadSavedColumns()
  loadApiKeys()
  loadGroups()
  loadUserGroupRates()
  loadPublicSettings()
  document.addEventListener('click', closeGroupSelector)
  resetTimer = setInterval(() => { now.value = new Date() }, 60000)
})

onUnmounted(() => {
  document.removeEventListener('click', closeGroupSelector)
  if (resetTimer) clearInterval(resetTimer)
})
</script>
<style scoped>
/* ---------- Page shell · prototype 05 (content padding 8/24/24/20, gap 14) ---------- */
.keys-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.keys-page .keys-header {
  margin-bottom: 0;
}

.keys-page .keys-layout {
  gap: 14px;
}

@media (min-width: 1024px) {
  .keys-page {
    height: calc(100vh - 93px);
  }

  .keys-page .keys-layout {
    height: auto;
    flex: 1;
    min-height: 0;
  }
}

/* ---------- Top row: endpoints (1.2fr 1.2fr) + mini stats (1fr) ---------- */
.keys-toolbar {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.keys-top-grid {
  display: grid;
  grid-template-columns: 1.2fr 1.2fr 1fr;
  gap: 12px;
}

.keys-top-grid.is-wide {
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}

.keys-page .keys-endpoint {
  padding: 14px 16px;
}

.keys-page .keys-toolbar .keys-stats :deep(.ui-mini-stat:nth-child(2) .ui-mini-stat-value) {
  color: var(--success-text);
}

/* ---------- Filter row ---------- */
.keys-page .keys-filter-bar {
  margin-bottom: 0;
}

.keys-sort-note {
  font-size: 12.5px;
  color: var(--muted);
  white-space: nowrap;
}

.keys-sort-note b {
  color: var(--foreground);
  font-weight: 600;
}

.keys-column-settings {
  position: relative;
  flex: none;
}

.keys-column-btn {
  width: 36px;
  height: 36px;
  padding: 0;
  justify-content: center;
  color: var(--muted);
}

.keys-column-btn:hover {
  color: var(--foreground);
}

.keys-column-dropdown {
  right: 0;
  top: calc(100% + 6px);
  max-height: 320px;
  overflow-y: auto;
  min-width: 200px;
}

.keys-column-item-label {
  flex: 1;
  min-width: 0;
  text-align: left;
}

.keys-mobile-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.keys-page .keys-chips {
  display: none;
}

.keys-page .keys-chips :deep(.ui-chip) {
  font-size: 12.5px;
}

/* ---------- Table card ---------- */
.keys-table-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.keys-page .keys-layout :deep(.table-scroll-container) {
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  border-radius: 14px;
  box-shadow: var(--shadow);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
}

.keys-page .keys-layout :deep(thead) {
  background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
  backdrop-filter: none;
}

.keys-page .keys-layout :deep(tbody) {
  background: transparent;
}

.keys-page .keys-layout :deep(th) {
  height: 42px;
  padding: 0 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
}

.keys-page .keys-layout :deep(td) {
  height: 58px;
  padding: 8px;
  font-size: 13px;
  color: var(--foreground);
  border-bottom: 1px solid var(--border);
}

.keys-page .keys-layout :deep(th:first-child),
.keys-page .keys-layout :deep(td:first-child) {
  padding-left: 16px;
}

.keys-page .keys-layout :deep(th:last-child),
.keys-page .keys-layout :deep(td:last-child) {
  padding-right: 16px;
}

/* ---------- Cells ---------- */
.keys-cell-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.keys-name-line {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.keys-name-text {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-name-shield {
  flex: none;
  color: var(--accent);
}

.keys-name-id {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--muted);
}

.keys-cell-key {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.keys-key-code {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-icon-btn-xs {
  width: 26px;
  height: 26px;
  border-radius: 7px;
}

.keys-icon-btn-xs.is-copied {
  color: var(--success-text);
}

.keys-group-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  margin: -3px -6px;
  padding: 3px 6px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s ease;
}

.keys-group-btn:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
}

.keys-group-badge {
  height: 22px;
}

.keys-group-caret {
  flex: none;
  color: var(--muted);
  opacity: 0.6;
}

.keys-group-btn:hover .keys-group-caret {
  opacity: 1;
}

.keys-concurrency {
  font-family: var(--font-mono);
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
}

.keys-usage {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-variant-numeric: tabular-nums;
}

.keys-usage-today {
  font-weight: 600;
}

.keys-usage-total {
  font-size: 11.5px;
  color: var(--muted);
}

.keys-usage-quota {
  display: block;
  width: 72px;
  margin-top: 2px;
}

.keys-rate-limit,
.keys-expiry,
.keys-muted-cell {
  font-size: 12.5px;
  color: var(--muted);
}

.keys-mono-cell {
  font-family: var(--font-mono);
  font-size: 12px;
}

.keys-cell-empty {
  color: var(--muted);
}

.keys-tone-danger {
  color: var(--danger-text);
}

.keys-tone-warning {
  color: var(--warning-text);
}

.keys-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}

.keys-use-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 9px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease;
}

.keys-use-btn:hover {
  background: color-mix(in oklch, var(--accent) 10%, transparent);
}

.keys-more-btn {
  width: 28px;
  height: 28px;
}

/* ---------- Dropdowns ---------- */
.keys-row-menu {
  position: fixed;
  z-index: 100000030;
  min-width: 190px;
}

.keys-group-dropdown {
  position: fixed;
  z-index: 100000020;
  width: max-content;
  min-width: 320px;
  max-width: calc(100vw - 16px);
  padding: 0;
  overflow: hidden;
}

.keys-group-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
}

.keys-group-search-input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--foreground);
  font-size: 13px;
}

.keys-group-options {
  max-height: 320px;
  overflow-y: auto;
  padding: 6px;
}

.keys-group-option {
  height: auto;
  padding: 7px 10px;
}

.keys-group-empty {
  padding: 16px 10px;
  text-align: center;
  font-size: 12.5px;
  color: var(--muted);
}

/* ---------- Modals / form ---------- */
.keys-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.keys-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.keys-field-stack {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.keys-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.keys-toggle-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.keys-mono-input :deep(.field) {
  font-family: var(--font-mono);
}

.keys-textarea {
  height: auto;
  min-height: 76px;
  padding: 8px 12px;
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.6;
  resize: vertical;
}

.keys-usage-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.keys-usage-readout,
.keys-window-readout {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
  padding: 0 12px;
  height: 36px;
  border-radius: var(--radius-field);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.keys-window-usage {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 8px;
}

.keys-usage-sep {
  color: var(--muted);
}

.keys-expiry-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.keys-current-expiry {
  font-size: 12.5px;
}

.keys-confirm-text {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
}

.keys-ccs-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
}

.keys-ccs-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-field);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
}

.keys-ccs-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-ccs-key {
  flex: none;
}

.keys-client-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 12px;
}

.keys-client-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 16px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-card);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  color: var(--muted);
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.keys-client-card:hover {
  border-color: var(--accent);
  background: color-mix(in oklch, var(--accent) 8%, transparent);
}

.keys-client-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.keys-client-desc {
  font-size: 11.5px;
  color: var(--muted);
}

/* ---------- Mobile card mode (prototype 08, 390px) ---------- */
.keys-mobile-wrap {
  position: relative;
  flex: 1;
  min-height: 0;
}

.keys-pagination {
  flex: none;
}

.keys-mobile-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.keys-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border-radius: 14px;
}

.keys-card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.keys-card-ident {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.keys-card-name {
  font-size: 15px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-card-meta {
  font-size: 12px;
  color: var(--muted);
}

.keys-card-top-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: none;
}

.keys-card-more {
  width: 32px;
  height: 32px;
}

.keys-card-key {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 40px;
  padding: 0 6px 0 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in oklch, var(--foreground) 4%, transparent);
}

.keys-card-key-text {
  flex: 1;
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  letter-spacing: 0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-card-key-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex: none;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.keys-card-key-btn:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.keys-card-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 6px;
  font-size: 11px;
  color: var(--muted);
}

.keys-card-stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.keys-card-stat.is-end {
  align-items: flex-end;
  text-align: right;
}

.keys-card-stat b {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.keys-list-fade {
  display: none;
}

.keys-fab {
  display: none;
}

@media (max-width: 767px) {
  .keys-page .keys-header {
    display: none;
  }

  .keys-top-grid,
  .keys-top-grid.is-wide {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .keys-page .keys-endpoint {
    display: none;
  }

  .keys-toolbar {
    gap: 12px;
  }

  .keys-page .keys-chips {
    display: flex;
  }

  .keys-sort-note {
    display: none;
  }

  .keys-column-btn {
    width: 44px;
    height: 44px;
  }

  .keys-list-fade {
    display: block;
  }

  .keys-fab {
    display: inline-flex;
  }

  .keys-mobile-list {
    padding-bottom: 72px;
  }
}
</style>
