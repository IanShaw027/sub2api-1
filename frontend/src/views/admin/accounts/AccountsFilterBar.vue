<template>
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
  <div class="acct-filter-row">
    <!-- eslint-disable vue/no-mutating-props -- `params` is a composable ref shared with the
         parent by design; the bindings below set the ref's own fields, not the prop binding
         itself (mirrors the pre-split direct `params.search = ...` mutation). -->
    <AccountTableFilters
      v-model:searchQuery="params.search"
      :filters="params"
      :groups="groups"
      @update:filters="(newFilters: Record<string, unknown>) => Object.assign(params, newFilters)"
      @change="debouncedReload"
      @update:searchQuery="debouncedReload"
    />
    <!-- eslint-enable vue/no-mutating-props -->

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

<script setup lang="ts">
// The TablePageLayout "#filters" slot content for AccountsView.vue
// (frontend-health-cleanup 6.4 split): status summary chips, the bulk-actions
// bar + column filters, the auto-refresh dropdown, selection count, the
// column-visibility dropdown, and the pending-list-sync banner. Every piece of
// state/behavior here is owned by composables in the parent and passed in by
// reference so this stays a pure "wire the markup up" component.
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import AccountBulkActionsBar from '@/components/admin/account/AccountBulkActionsBar.vue'
import AccountTableFilters from '@/components/admin/account/AccountTableFilters.vue'
import type { AdminGroup } from '@/types'
import type { AccountListState } from './useAccountListState'
import type { AccountSelectionState } from './useAccountSelection'
import type { AccountBulkActionsState } from './useAccountBulkActions'
import type { UpstreamBillingRatesState } from './useUpstreamBillingRates'
import type { AccountAutoRefreshState } from './useAccountAutoRefresh'
import type { AccountToolbarMenusState } from './useAccountToolbarMenus'

const { t } = useI18n()

const props = defineProps<{
  listState: AccountListState
  selection: AccountSelectionState
  bulkActions: AccountBulkActionsState
  upstreamBilling: UpstreamBillingRatesState
  autoRefresh: AccountAutoRefreshState
  toolbarMenus: AccountToolbarMenusState
  groups: AdminGroup[]
}>()

const { statusChips, params, applyStatusChip, debouncedReload, toggleableColumns, isColumnVisible, toggleColumn, hasPendingListSync, syncPendingListChanges, pagination } = props.listState
const { selIds, selectingAllResults, allResultsSelected, clearSelection, selectPage, handleSelectAllResults } = props.selection
const { handleBulkDelete, handleBulkResetStatus, handleBulkRefreshToken, openBulkEditSelected, openBulkEditFiltered, handleBulkToggleSchedulable } = props.bulkActions
const { handleBulkProbeUpstreamBilling } = props.upstreamBilling
const { autoRefreshEnabled, autoRefreshCountdown, autoRefreshIntervals, autoRefreshIntervalSeconds, autoRefreshIntervalLabel, setAutoRefreshEnabled, setAutoRefreshInterval } = props.autoRefresh
const { showAutoRefreshDropdown, toggleAutoRefreshDropdown, showColumnsDropdown, toggleColumnsDropdown, autoRefreshDropdownRef, columnsDropdownRef } = props.toolbarMenus
</script>

<style scoped>
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

@media (max-width: 767px) {
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
}
</style>
