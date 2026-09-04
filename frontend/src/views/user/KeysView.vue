<template>
 <AppLayout>
 <div class="keys-page">
 <PageHeader class="keys-header" :title="t('keys.title')" :description="t('keys.description')">
 <template #actions>
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
 {{ t('keys.sortedByPrefix') }}<b>{{ activeSortLabel }}</b>{{ t('keys.sortedBySuffix') }}
 </span>
 <button
 type="button"
 class="filter-pill keys-refresh-btn"
 :disabled="loading"
 :title="t('common.refresh')"
 :aria-label="t('common.refresh')"
 @click="loadApiKeys"
 >
 <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
 </button>
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
 :title="groupCellTooltip(row)"
 @click="openGroupSelector(row)"
 >
 <span v-if="row.group" class="tag tag-accent keys-group-pill">
 <span class="truncate">{{ row.group.name }}</span>
 <span v-if="groupCellSuffix(row)" class="keys-group-suffix">{{ groupCellSuffix(row) }}</span>
 </span>
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
 <span class="keys-expiry" :class="expiryToneClass(value, now)">
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
 <KeyMobileCard
 v-for="row in apiKeys"
 v-else
 :key="row.id"
 :row="row"
 :revealed="isKeyRevealed(row.id)"
 :copied="copiedKeyId === row.id"
 :today-cost="usageStats[row.id]?.today_actual_cost"
 :now="now"
 @more="toggleRowMenu(row, $event)"
 @toggle-reveal="toggleKeyReveal(row.id)"
 @copy="copyToClipboard(row.key, row.id)"
 />
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
 <KeyFormModal
 :open="showCreateModal || showEditModal"
 :editing-key="showEditModal ? selectedKey : null"
 :groups="groups"
 :user-group-rates="userGroupRates"
 @close="closeModals"
 @saved="loadApiKeys"
 />

 <!-- Delete confirmation -->
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

 <!-- CC Switch import + client select -->
 <CcSwitchModals
 :show-import-picker="showCcsImportModal"
 :show-client-select="showCcsClientSelect"
 :api-keys="apiKeys"
 @close-import="showCcsImportModal = false"
 @pick="importFromPicker"
 @close-client-select="closeCcsClientSelect"
 @select-client="handleCcsClientSelect"
 />

 <!-- Row actions menu -->
 <KeyRowActionsMenu
 :api-key="rowMenuKey"
 :position="rowMenuPosition"
 :hide-ccs-import="!!publicSettings?.hide_ccs_import_button"
 :show-reset-rate-limit="hasRateLimitUsage(rowMenuKey)"
 @use="runRowAction(() => openUseKeyModal(rowMenuKey!))"
 @import-ccs="runRowAction(() => importToCcswitch(rowMenuKey!))"
 @copy="runRowAction(() => copyToClipboard(rowMenuKey!.key, rowMenuKey!.id))"
 @edit="runRowAction(() => editKey(rowMenuKey!))"
 @toggle-status="runRowAction(() => toggleKeyStatus(rowMenuKey!))"
 @reset-rate-limit="runRowAction(() => confirmResetRateLimitFromTable(rowMenuKey!))"
 @delete="runRowAction(() => confirmDelete(rowMenuKey!))"
 />

 <!-- Group Selector Dropdown -->
 <KeyGroupPicker
 :options="groupOptions"
 :selected-value="selectedKeyForGroup?.group_id ?? null"
 :position="dropdownPosition"
 @select="(value) => changeGroup(selectedKeyForGroup!, value)"
 />
 </div>
 </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
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
import KeyFormModal from '@/components/keys/KeyFormModal.vue'
import KeyRowActionsMenu from '@/components/keys/KeyRowActionsMenu.vue'
import KeyGroupPicker from '@/components/keys/KeyGroupPicker.vue'
import KeyMobileCard from '@/components/keys/KeyMobileCard.vue'
import CcSwitchModals from '@/components/keys/CcSwitchModals.vue'
import { useKeyColumns } from '@/components/keys/useKeyColumns'
import { useEndpointCards } from '@/components/keys/useEndpointCards'
import { useGroupCellDisplay } from '@/components/keys/useGroupCellDisplay'
import type { ApiKey, Group, PublicSettings, SubscriptionType, GroupPlatform } from '@/types'
import type { Column } from '@/components/common/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import { formatDate } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import {
  formatCost,
  quotaPercent,
  quotaBarClass,
  hasRateLimit,
  statusTone,
  expiryToneClass,
  hasIpRestriction,
  rateLimitSummary,
  rateLimitToneClass,
  rateLimitDetail as rateLimitDetailUtil
} from '@/components/keys/keyUtils'

const { t } = useI18n()
const { isTabletUp } = useIsMobile()

import type { CcSwitchClientType } from '@/utils/ccswitchImport'
import { openCcSwitchDeeplink, formatResetCountdown } from '@/components/keys/ccSwitchHelpers'

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
const { copyToClipboard: clipboardCopy } = useClipboard()

const allColumns = computed<Column[]>(() => [
  { key: 'name', label: t('keys.nameIdColumn'), sortable: true },
  { key: 'id', label: t('keys.id'), sortable: true },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'group', label: t('keys.group'), sortable: false },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: true },
  { key: 'usage', label: t('keys.usageColumnHeader'), sortable: false },
  { key: 'rate_limit', label: t('keys.rateLimitColumn'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'last_used_at', label: t('keys.lastUsedAt'), sortable: true },
  { key: 'last_used_ip', label: t('keys.lastUsedIP'), sortable: false },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const {
  toggleableColumns,
  columns,
  loadSavedColumns,
  toggleColumn,
  isColumnVisible
} = useKeyColumns(allColumns)

const apiKeys = ref<ApiKey[]>([])
const groups = ref<Group[]>([])
const loading = ref(false)
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
const columnDropdownRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<{ top?: number; bottom?: number; left: number } | null>(null)
const revealedKeyIds = reactive<Set<number>>(new Set())
const rowMenuKeyId = ref<number | null>(null)
const rowMenuPosition = ref<{ top: number; left: number } | null>(null)
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

const { groupButtonRefs, setGroupButtonRef, groupCellSuffix, groupCellTooltip } = useGroupCellDisplay(
  userGroupRates,
  () => appStore.cachedPublicSettings?.server_utc_offset,
  t
)

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
  { label: t('keys.miniStats.total'), value: pagination.value.total },
  { label: t('keys.miniStats.active'), value: activeKeyCount.value },
  { label: t('keys.miniStats.todaySpend'), value: `$${todayKeySpend.value.toFixed(2)}` }
])

const copyEndpoint = async (url: string) => {
  await clipboardCopy(url, t('keys.endpoints.copied'))
}

const { endpointCards } = useEndpointCards(publicSettings, t)

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

// Convert groups to options for the row group-change picker (rate multiplier + subscription type)
const groupOptions = computed<GroupOption[]>(() =>
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

const copyToClipboard = async (text: string, keyId: number) => {
  const success = await clipboardCopy(text, t('keys.copied'))
  if (success) {
    copiedKeyId.value = keyId
    setTimeout(() => {
      copiedKeyId.value = null
    }, 800)
  }
}

const isKeyRevealed = (id: number) => revealedKeyIds.has(id)

const toggleKeyReveal = (id: number) => {
  if (revealedKeyIds.has(id)) {
    revealedKeyIds.delete(id)
  } else {
    revealedKeyIds.add(id)
  }
}

const hasRateLimitUsage = (key: ApiKey | null) => !!key && hasRateLimit(key)

const rateLimitDetail = (key: ApiKey) => rateLimitDetailUtil(key, formatResetTime)

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
  if (!target.closest('.group\\/dropdown') && !target.closest('.keys-group-dropdown')) {
    groupSelectorKeyId.value = null
    dropdownPosition.value = null
  }
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false
  }
  if (rowMenuKeyId.value !== null && !target.closest('.keys-row-menu') && !target.closest('.keys-more-btn') && !target.closest('.keys-card-more')) {
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

// 处理删除 API Key：优先显示后端返回的具体错误消息（如权限不足等）
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
}

// Show reset rate limit confirmation dialog (from table row)
const confirmResetRateLimitFromTable = (row: ApiKey) => {
  confirmDialog.value = {
    title: t('keys.resetRateLimitTitle'),
    message: t('keys.resetRateLimitConfirmMessage', { name: row.name }),
    confirmText: t('keys.reset'),
    onConfirm: async () => {
      try {
        await keysAPI.update(row.id, { reset_rate_limit_usage: true })
        appStore.showSuccess(t('keys.rateLimitResetSuccess'))
        loadApiKeys()
      } catch (error: any) {
        const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
        appStore.showError(errorMsg)
      }
    }
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
  openCcSwitchDeeplink(row, clientType, publicSettings.value, () => {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  })
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

const formatResetTime = (resetAt: string | null) =>
  formatResetCountdown(resetAt, now.value, t('keys.resetNow'))

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
    height: calc(100vh - 98px);
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
  padding: 12px 16px;
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

.keys-page .keys-layout :deep(table) {
  table-layout: fixed;
}

.keys-page .keys-layout :deep(th) {
  height: 43px;
  padding: 0 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
}

.keys-page .keys-layout :deep(td) {
  height: 59px;
  padding: 0 8px;
  font-size: 13px;
  color: var(--foreground);
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}

.keys-page .keys-layout :deep(th:first-child),
.keys-page .keys-layout :deep(td:first-child) {
  padding-left: 16px;
}

.keys-page .keys-layout :deep(th:last-child),
.keys-page .keys-layout :deep(td:last-child) {
  padding-right: 16px;
}

/* Fixed column widths (ratios match the 05 reference: name/key flex a bit
   wider, the rest are content-driven fixed widths). table-layout:fixed with
   table {width:100%} scales these proportionally to the card's inner width. */
.keys-page .keys-layout :deep(th:nth-child(1)),
.keys-page .keys-layout :deep(td:nth-child(1)) {
  width: 150px;
}

.keys-page .keys-layout :deep(th:nth-child(2)),
.keys-page .keys-layout :deep(td:nth-child(2)) {
  width: 250px;
}

.keys-page .keys-layout :deep(th:nth-child(3)),
.keys-page .keys-layout :deep(td:nth-child(3)) {
  width: 96px;
}

.keys-page .keys-layout :deep(th:nth-child(4)),
.keys-page .keys-layout :deep(td:nth-child(4)) {
  width: 56px;
}

.keys-page .keys-layout :deep(th:nth-child(5)),
.keys-page .keys-layout :deep(td:nth-child(5)) {
  width: 100px;
}

.keys-page .keys-layout :deep(th:nth-child(6)),
.keys-page .keys-layout :deep(td:nth-child(6)) {
  width: 72px;
}

.keys-page .keys-layout :deep(th:nth-child(7)),
.keys-page .keys-layout :deep(td:nth-child(7)) {
  width: 88px;
}

.keys-page .keys-layout :deep(th:nth-child(8)),
.keys-page .keys-layout :deep(td:nth-child(8)) {
  width: 80px;
}

.keys-page .keys-layout :deep(th:nth-child(9)),
.keys-page .keys-layout :deep(td:nth-child(9)) {
  width: 76px;
}

.keys-page .keys-layout :deep(th:nth-child(10)),
.keys-page .keys-layout :deep(td:nth-child(10)) {
  width: 80px;
}

/* ---------- Cells ---------- */
.keys-cell-name {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 2px;
  min-width: 0;
  line-height: 1.2;
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

.keys-group-pill {
  max-width: 160px;
}

.keys-group-suffix {
  flex: none;
  opacity: 0.65;
  font-weight: 500;
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
  justify-content: center;
  gap: 2px;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
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

.keys-confirm-text {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
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
