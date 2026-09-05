<template>
  <AppLayout>
    <PageHeader :title="t('admin.subscriptions.title')" :description="t('admin.subscriptions.description')">
      <template #actions>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadSubscriptions">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <div class="relative" ref="columnDropdownRef">
          <Button variant="secondary" :title="t('admin.users.columnSettings')" @click="showColumnDropdown = !showColumnDropdown">
            <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
            </svg>
            <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
          </Button>
          <div v-if="showColumnDropdown" class="dropdown subs-dropdown">
            <div class="dropdown-label">{{ t('admin.subscriptions.columns.user') }}</div>
            <button type="button" class="dropdown-item" :class="{ 'is-active': userColumnMode === 'email' }" @click="setUserColumnMode('email')">
              <span class="subs-dropdown-text">{{ t('admin.users.columns.email') }}</span>
              <Icon v-if="userColumnMode === 'email'" name="check" size="sm" />
            </button>
            <button type="button" class="dropdown-item" :class="{ 'is-active': userColumnMode === 'username' }" @click="setUserColumnMode('username')">
              <span class="subs-dropdown-text">{{ t('admin.users.columns.username') }}</span>
              <Icon v-if="userColumnMode === 'username'" name="check" size="sm" />
            </button>
            <div class="dropdown-divider"></div>
            <button
              v-for="col in toggleableColumns"
              :key="col.key"
              type="button"
              class="dropdown-item"
              :class="{ 'is-active': isColumnVisible(col.key) }"
              @click="toggleColumn(col.key)"
            >
              <span class="subs-dropdown-text">{{ col.label }}</span>
              <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" />
            </button>
          </div>
        </div>
        <Button variant="secondary" :title="t('admin.subscriptions.guide.showGuide')" @click="showGuideModal = true">
          <Icon name="questionCircle" size="md" />
        </Button>
        <Button class="subs-create-desktop" @click="showAssignModal = true">
          <Icon name="plus" size="md" />
          {{ t('admin.subscriptions.assignSubscription') }}
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <FilterBar
            :search-placeholder="t('admin.users.searchUsers')"
            :filter-label="t('common.filter')"
            @open-filters="showMobileFilters = !showMobileFilters"
          >
            <template #search>
              <div class="relative w-full" data-filter-user-search>
                <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
                <input
                  v-model="filterUserKeyword"
                  type="text"
                  :placeholder="t('admin.users.searchUsers')"
                  class="input pl-10 pr-8"
                  @input="debounceSearchFilterUsers"
                  @focus="showFilterUserDropdown = true"
                />
                <button
                  v-if="selectedFilterUser"
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-muted hover:text-foreground"
                  :title="t('common.clear')"
                  @click="clearFilterUser"
                >
                  <Icon name="x" size="sm" :stroke-width="2" />
                </button>
                <div
                  v-if="showFilterUserDropdown && (filterUserResults.length > 0 || filterUserKeyword)"
                  class="dropdown subs-filter-user-dropdown"
                >
                  <div v-if="filterUserLoading" class="px-4 py-3 text-sm text-muted">
                    {{ t('common.loading') }}
                  </div>
                  <div v-else-if="filterUserResults.length === 0 && filterUserKeyword" class="px-4 py-3 text-sm text-muted">
                    {{ t('common.noOptionsFound') }}
                  </div>
                  <button
                    v-for="user in filterUserResults"
                    :key="user.id"
                    type="button"
                    class="dropdown-item"
                    @click="selectFilterUser(user)"
                  >
                    <span class="font-medium text-foreground">{{ user.email }}</span>
                    <span class="ml-2 text-muted">#{{ user.id }}</span>
                  </button>
                </div>
              </div>
            </template>
            <template #filters>
              <Select
                v-model="filters.status"
                :options="statusOptions"
                :placeholder="t('admin.subscriptions.allStatus')"
                class="w-40"
                @change="applyFilters"
              />
              <Select
                v-model="filters.group_id"
                :options="groupOptions"
                :placeholder="t('admin.subscriptions.allGroups')"
                class="w-48"
                @change="applyFilters"
              />
              <Select
                v-model="filters.platform"
                :options="platformFilterOptions"
                :placeholder="t('admin.subscriptions.allPlatforms')"
                class="w-40"
                @change="applyFilters"
              />
            </template>
          </FilterBar>
          <div v-if="showMobileFilters" class="subs-mobile-filters">
            <Select
              v-model="filters.status"
              :options="statusOptions"
              :placeholder="t('admin.subscriptions.allStatus')"
              class="w-full"
              @change="applyFilters"
            />
            <Select
              v-model="filters.group_id"
              :options="groupOptions"
              :placeholder="t('admin.subscriptions.allGroups')"
              class="w-full"
              @change="applyFilters"
            />
            <Select
              v-model="filters.platform"
              :options="platformFilterOptions"
              :placeholder="t('admin.subscriptions.allPlatforms')"
              class="w-full"
              @change="applyFilters"
            />
          </div>
        </div>
      </template>

      <!-- Subscriptions Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="subscriptions"
          :loading="loading"
          :server-side-sort="true"
          sort-storage-key="subscription-sort"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-user="{ row }">
            <div class="flex items-center gap-2">
              <div
                class="flex h-8 w-8 items-center justify-center rounded-full bg-accent/15"
              >
                <span class="text-sm font-medium text-accent">
                  {{ userColumnMode === 'email'
                    ? (row.user?.email?.charAt(0).toUpperCase() || '?')
                    : (row.user?.username?.charAt(0).toUpperCase() || '?')
                  }}
                </span>
              </div>
              <span class="font-medium text-foreground">
                {{ userColumnMode === 'email'
                  ? (row.user?.email || t('admin.redeem.userPrefix', { id: row.user_id }))
                  : (row.user?.username || '-')
                }}
              </span>
            </div>
          </template>

          <template #cell-group="{ row }">
            <GroupBadge
              v-if="row.group"
              :name="row.group.name"
              :platform="row.group.platform"
              :subscription-type="row.group.subscription_type"
              :rate-multiplier="row.group.rate_multiplier"
              :show-rate="false"
            />
            <span v-else class="text-sm text-muted">-</span>
          </template>

          <template #cell-usage="{ row }">
            <div class="min-w-[280px] space-y-2">
              <!-- Daily Usage -->
              <div v-if="row.group?.daily_limit_usd" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.daily') }}</span>
                  <div class="progress progress-thin flex-1">
                    <div
                      class="progress-bar"
                      :class="getProgressClass(row.daily_usage_usd, row.group?.daily_limit_usd)"
                      :style="{
                        width: getProgressWidth(row.daily_usage_usd, row.group?.daily_limit_usd)
                      }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    ${{ row.daily_usage_usd?.toFixed(2) || '0.00' }}
                    <span class="text-muted">/</span>
                    ${{ row.group?.daily_limit_usd?.toFixed(2) }}
                  </span>
                </div>
                <div class="reset-info" v-if="row.daily_window_start">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>{{ formatDailyUsageWindow(row) }}</span>
                </div>
              </div>

              <!-- Weekly Usage -->
              <div v-if="row.group?.weekly_limit_usd" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.weekly') }}</span>
                  <div class="progress progress-thin flex-1">
                    <div
                      class="progress-bar"
                      :class="getProgressClass(row.weekly_usage_usd, row.group?.weekly_limit_usd)"
                      :style="{
                        width: getProgressWidth(row.weekly_usage_usd, row.group?.weekly_limit_usd)
                      }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    ${{ row.weekly_usage_usd?.toFixed(2) || '0.00' }}
                    <span class="text-muted">/</span>
                    ${{ row.group?.weekly_limit_usd?.toFixed(2) }}
                  </span>
                </div>
                <div class="reset-info" v-if="row.weekly_window_start">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>{{ formatResetTime(row.weekly_window_start, 'weekly') }}</span>
                </div>
              </div>

              <!-- Monthly Usage -->
              <div v-if="row.group?.monthly_limit_usd" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.monthly') }}</span>
                  <div class="progress progress-thin flex-1">
                    <div
                      class="progress-bar"
                      :class="getProgressClass(row.monthly_usage_usd, row.group?.monthly_limit_usd)"
                      :style="{
                        width: getProgressWidth(row.monthly_usage_usd, row.group?.monthly_limit_usd)
                      }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    ${{ row.monthly_usage_usd?.toFixed(2) || '0.00' }}
                    <span class="text-muted">/</span>
                    ${{ row.group?.monthly_limit_usd?.toFixed(2) }}
                  </span>
                </div>
                <div class="reset-info" v-if="row.monthly_window_start">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>{{ formatResetTime(row.monthly_window_start, 'monthly') }}</span>
                </div>
              </div>

              <!-- No Limits - Unlimited badge -->
              <div
                v-if="
                  !row.group?.daily_limit_usd &&
                  !row.group?.weekly_limit_usd &&
                  !row.group?.monthly_limit_usd
                "
                class="flex items-center gap-2 rounded-lg px-3 py-2"
                style="background: color-mix(in oklch, var(--success) 14%, transparent)"
              >
                <span class="text-lg text-success-text">∞</span>
                <span class="text-xs font-medium text-success-text">
                  {{ t('admin.subscriptions.unlimited') }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-expires_at="{ value }">
            <div v-if="value">
              <span
                class="text-sm"
                :class="isExpiringSoon(value)
 ? 'text-warning-text '
 : 'text-foreground'"
              >
                {{ formatDateTimeToMinute(value) }}
              </span>
              <template
                v-for="remainingExpiry in [formatRemainingExpiry(value)]"
                :key="remainingExpiry ?? 'expired'"
              >
                <div v-if="remainingExpiry" class="text-xs text-muted">
                  {{ remainingExpiry }}
                </div>
              </template>
            </div>
            <span v-else class="text-sm text-muted">{{
              t('admin.subscriptions.noExpiration')
            }}</span>
          </template>

          <template #cell-status="{ value }">
            <span
              :class="[
 'badge',
 value === 'active'
 ? 'badge-success'
 : value === 'expired'
 ? 'badge-warning'
 : 'badge-danger'
 ]"
            >
              {{ t(`admin.subscriptions.status.${value}`) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1">
              <button
                v-if="row.status === 'active' || row.status === 'expired'"
                type="button"
                class="subs-use-btn text-xs font-semibold"
                @click="handleExtend(row)"
              >
                {{ t('admin.subscriptions.adjust') }}
              </button>
              <button
                v-if="row.status === 'revoked'"
                type="button"
                class="subs-use-btn text-xs font-semibold"
                @click="handleRestore(row)"
              >
                {{ t('admin.subscriptions.restore') }}
              </button>
              <button
                v-if="row.status === 'active'"
                type="button"
                class="icon-btn subs-more-btn"
                :title="t('common.more')"
                :aria-label="t('common.more')"
                @click.stop="toggleRowMenu(row, $event)"
              >
                <Icon name="more" size="sm" :stroke-width="2.4" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.subscriptions.noSubscriptionsYet')"
              :description="t('admin.subscriptions.assignFirstSubscription')"
              :action-text="t('admin.subscriptions.assignSubscription')"
              @action="showAssignModal = true"
            />
          </template>
        </DataTable>
        <SubscriptionRowActionsMenu
          :subscription="rowMenuSubscription"
          :position="rowMenuPosition"
          @reset-quota="runRowMenuAction(() => handleResetQuota(rowMenuSubscription!))"
          @revoke="runRowMenuAction(() => handleRevoke(rowMenuSubscription!))"
        />
      </template>

      <!-- Pagination -->
      <template #pagination>
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
      </template>
    </TablePageLayout>

    <!-- Assign Subscription Modal -->
    <SubscriptionAssignDialog
      :open="showAssignModal"
      :groups="groups"
      @close="showAssignModal = false"
      @saved="loadSubscriptions"
    />

    <!-- Adjust Subscription Modal -->
    <SubscriptionAdjustDialog
      :open="showExtendModal"
      :subscription="extendingSubscription"
      @close="closeExtendModal"
      @saved="loadSubscriptions"
    />

    <!-- Revoke Confirmation Dialog -->
    <ConfirmDialog
      :show="showRevokeDialog"
      :title="t('admin.subscriptions.revokeSubscription')"
      :message="t('admin.subscriptions.revokeConfirm', { user: revokingSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.revoke')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmRevoke"
      @cancel="showRevokeDialog = false"
    />

    <!-- Restore Confirmation Dialog -->
    <ConfirmDialog
      :show="showRestoreDialog"
      :title="t('admin.subscriptions.restoreSubscription')"
      :message="t('admin.subscriptions.restoreConfirm', { user: restoringSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.restore')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmRestore"
      @cancel="showRestoreDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaConfirm"
      :title="t('admin.subscriptions.resetQuotaTitle')"
      :message="t('admin.subscriptions.resetQuotaConfirm', { user: resettingSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.resetQuota')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmResetQuota"
      @cancel="showResetQuotaConfirm = false"
    />
    <SubscriptionGuideDialog :open="showGuideModal" @close="showGuideModal = false" />
    <Fab class="subs-fab" :label="t('admin.subscriptions.assignSubscription')" @click="showAssignModal = true">
      <Icon name="plus" size="md" />
      {{ t('admin.subscriptions.assignSubscription') }}
    </Fab>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { UserSubscription, Group } from '@/types'
import type { SimpleUser } from '@/api/admin/usage'
import type { Column } from '@/components/common/types'
import { formatDateTimeToMinute } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import Fab from '@/components/ui/Fab.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import SubscriptionRowActionsMenu from '@/components/admin/subscriptions/SubscriptionRowActionsMenu.vue'
import SubscriptionGuideDialog from '@/components/admin/subscriptions/SubscriptionGuideDialog.vue'
import SubscriptionAssignDialog from '@/components/admin/subscriptions/SubscriptionAssignDialog.vue'
import SubscriptionAdjustDialog from '@/components/admin/subscriptions/SubscriptionAdjustDialog.vue'
import {
  getRemainingDurationParts,
  getRemainingExpiryDuration,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'

const { t } = useI18n()
const appStore = useAppStore()

// Guide modal state
const showGuideModal = ref(false)

// User column display mode: 'email' or 'username'
const userColumnMode = ref<'email' | 'username'>('email')
const USER_COLUMN_MODE_KEY = 'subscription-user-column-mode'

const loadUserColumnMode = () => {
  try {
    const saved = localStorage.getItem(USER_COLUMN_MODE_KEY)
    if (saved === 'email' || saved === 'username') {
      userColumnMode.value = saved
    }
  } catch (e) {
    console.error('Failed to load user column mode:', e)
  }
}

const saveUserColumnMode = () => {
  try {
    localStorage.setItem(USER_COLUMN_MODE_KEY, userColumnMode.value)
  } catch (e) {
    console.error('Failed to save user column mode:', e)
  }
}

const setUserColumnMode = (mode: 'email' | 'username') => {
  userColumnMode.value = mode
  saveUserColumnMode()
}

// All available columns
const allColumns = computed<Column[]>(() => [
  {
    key: 'user',
    label: userColumnMode.value === 'email'
      ? t('admin.subscriptions.columns.user')
      : t('admin.users.columns.username'),
    sortable: false
  },
  { key: 'group', label: t('admin.subscriptions.columns.group'), sortable: false },
  { key: 'usage', label: t('admin.subscriptions.columns.usage'), sortable: false },
  { key: 'expires_at', label: t('admin.subscriptions.columns.expires'), sortable: true },
  { key: 'status', label: t('admin.subscriptions.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.subscriptions.columns.actions'), sortable: false }
])

// Columns that can be toggled (exclude user and actions which are always visible)
const toggleableColumns = computed(() =>
  allColumns.value.filter(col => col.key !== 'user' && col.key !== 'actions')
)

// Hidden columns set
const hiddenColumns = reactive<Set<string>>(new Set())

// Default hidden columns
const DEFAULT_HIDDEN_COLUMNS: string[] = []

// localStorage key
const HIDDEN_COLUMNS_KEY = 'subscription-hidden-columns'

// Load saved column settings
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed.forEach(key => hiddenColumns.add(key))
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
    }
  } catch (e) {
    console.error('Failed to load saved columns:', e)
    DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
  }
}

// Save column settings to localStorage
const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// Toggle column visibility
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

// Check if column is visible
const isColumnVisible = (key: string) => !hiddenColumns.has(key)

// Filtered columns for display
const columns = computed<Column[]>(() =>
  allColumns.value.filter(col =>
    col.key === 'user' || col.key === 'actions' || !hiddenColumns.has(col.key)
  )
)

// Column dropdown state
const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)
const rowMenuSubscriptionId = ref<number | null>(null)
const rowMenuPosition = ref<{ top: number; left: number } | null>(null)
const rowMenuSubscription = computed(() =>
  rowMenuSubscriptionId.value === null
    ? null
    : subscriptions.value.find((s) => s.id === rowMenuSubscriptionId.value) || null
)

const closeRowMenu = () => {
  rowMenuSubscriptionId.value = null
  rowMenuPosition.value = null
}

const toggleRowMenu = (row: UserSubscription, event: MouseEvent) => {
  if (rowMenuSubscriptionId.value === row.id) {
    closeRowMenu()
    return
  }
  const buttonEl = event.currentTarget as HTMLElement
  const rect = buttonEl.getBoundingClientRect()
  const menuEstWidth = 200
  const menuEstHeight = 120
  const left = Math.max(8, Math.min(rect.right - menuEstWidth, window.innerWidth - menuEstWidth - 8))
  const spaceBelow = window.innerHeight - rect.bottom
  const top = spaceBelow < menuEstHeight && rect.top > spaceBelow
    ? Math.max(8, rect.top - menuEstHeight)
    : rect.bottom + 4
  rowMenuPosition.value = { top, left }
  rowMenuSubscriptionId.value = row.id
}

const runRowMenuAction = (action: () => void) => {
  action()
  closeRowMenu()
}

// Filter options
const statusOptions = computed(() => [
  { value: '', label: t('admin.subscriptions.allStatus') },
  { value: 'active', label: t('admin.subscriptions.status.active') },
  { value: 'expired', label: t('admin.subscriptions.status.expired') },
  { value: 'revoked', label: t('admin.subscriptions.status.revoked') }
])

const subscriptions = ref<UserSubscription[]>([])
const groups = ref<Group[]>([])
const loading = ref(false)
let abortController: AbortController | null = null

// Toolbar user filter (fuzzy search -> select user_id)
const showMobileFilters = ref(false)
const filterUserKeyword = ref('')
const filterUserResults = ref<SimpleUser[]>([])
const filterUserLoading = ref(false)
const showFilterUserDropdown = ref(false)
const selectedFilterUser = ref<SimpleUser | null>(null)
let filterUserSearchTimeout: ReturnType<typeof setTimeout> | null = null

const filters = reactive({
  status: 'active',
  group_id: '',
  platform: '',
  user_id: null as number | null
})

// Sorting state
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showAssignModal = ref(false)
const showExtendModal = ref(false)
const showRevokeDialog = ref(false)
const showRestoreDialog = ref(false)
const showResetQuotaConfirm = ref(false)
const resettingSubscription = ref<UserSubscription | null>(null)
const resettingQuota = ref(false)
const extendingSubscription = ref<UserSubscription | null>(null)
const revokingSubscription = ref<UserSubscription | null>(null)
const restoringSubscription = ref<UserSubscription | null>(null)

// Group options for filter (all groups)
const groupOptions = computed(() => [
  { value: '', label: t('admin.subscriptions.allGroups') },
  ...groups.value.map((g) => ({ value: g.id.toString(), label: g.name }))
])

const platformFilterOptions = computed(() => [
  { value: '', label: t('admin.subscriptions.allPlatforms') },
  ...GROUP_PLATFORM_OPTIONS
])

const applyFilters = () => {
  pagination.page = 1
  loadSubscriptions()
}

const loadSubscriptions = async () => {
  if (abortController) {
    abortController.abort()
  }
  const requestController = new AbortController()
  abortController = requestController
  const { signal } = requestController

  loading.value = true
  try {
    const response = await adminAPI.subscriptions.list(
      pagination.page,
      pagination.page_size,
      {
        status: (filters.status as any) || undefined,
        group_id: filters.group_id ? parseInt(filters.group_id) : undefined,
        platform: filters.platform || undefined,
        user_id: filters.user_id || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      {
        signal
      }
    )
    if (signal.aborted || abortController !== requestController) return
    subscriptions.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (signal.aborted || error?.name === 'AbortError' || error?.code === 'ERR_CANCELED') {
      return
    }
    appStore.showError(t('admin.subscriptions.failedToLoad'))
    console.error('Error loading subscriptions:', error)
  } finally {
    if (abortController === requestController) {
      loading.value = false
      abortController = null
    }
  }
}

const loadGroups = async () => {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch (error) {
    console.error('Error loading groups:', error)
  }
}

// Toolbar user filter search with debounce
const debounceSearchFilterUsers = () => {
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  filterUserSearchTimeout = setTimeout(searchFilterUsers, 300)
}

const searchFilterUsers = async () => {
  const keyword = filterUserKeyword.value.trim()

  // Clear active user filter if user modified the search keyword
  if (selectedFilterUser.value && keyword !== selectedFilterUser.value.email) {
    selectedFilterUser.value = null
    filters.user_id = null
    applyFilters()
  }

  if (!keyword) {
    filterUserResults.value = []
    return
  }

  filterUserLoading.value = true
  try {
    filterUserResults.value = await adminAPI.usage.searchUsers(keyword)
  } catch (error) {
    console.error('Failed to search users:', error)
    filterUserResults.value = []
  } finally {
    filterUserLoading.value = false
  }
}

const selectFilterUser = (user: SimpleUser) => {
  selectedFilterUser.value = user
  filterUserKeyword.value = user.email
  showFilterUserDropdown.value = false
  filters.user_id = user.id
  applyFilters()
}

const clearFilterUser = () => {
  selectedFilterUser.value = null
  filterUserKeyword.value = ''
  filterUserResults.value = []
  showFilterUserDropdown.value = false
  filters.user_id = null
  applyFilters()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadSubscriptions()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadSubscriptions()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadSubscriptions()
}

const handleExtend = (subscription: UserSubscription) => {
  extendingSubscription.value = subscription
  showExtendModal.value = true
}

const closeExtendModal = () => {
  showExtendModal.value = false
  extendingSubscription.value = null
}

const handleRevoke = (subscription: UserSubscription) => {
  revokingSubscription.value = subscription
  showRevokeDialog.value = true
}

const confirmRevoke = async () => {
  if (!revokingSubscription.value) return

  try {
    await adminAPI.subscriptions.revoke(revokingSubscription.value.id)
    appStore.showSuccess(t('admin.subscriptions.subscriptionRevoked'))
    showRevokeDialog.value = false
    revokingSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToRevoke'))
    console.error('Error revoking subscription:', error)
  }
}

const handleRestore = (subscription: UserSubscription) => {
  restoringSubscription.value = subscription
  showRestoreDialog.value = true
}

const confirmRestore = async () => {
  if (!restoringSubscription.value) return

  try {
    await adminAPI.subscriptions.restore(restoringSubscription.value.id)
    appStore.showSuccess(t('admin.subscriptions.subscriptionRestored'))
    showRestoreDialog.value = false
    restoringSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToRestore'))
    console.error('Error restoring subscription:', error)
  }
}

const handleResetQuota = (subscription: UserSubscription) => {
  resettingSubscription.value = subscription
  showResetQuotaConfirm.value = true
}

const confirmResetQuota = async () => {
  if (!resettingSubscription.value) return
  if (resettingQuota.value) return
  resettingQuota.value = true
  try {
    await adminAPI.subscriptions.resetQuota(resettingSubscription.value.id, { daily: true, weekly: true, monthly: true })
    appStore.showSuccess(t('admin.subscriptions.quotaResetSuccess'))
    showResetQuotaConfirm.value = false
    resettingSubscription.value = null
    await loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToResetQuota'))
    console.error('Error resetting quota:', error)
  } finally {
    resettingQuota.value = false
  }
}

// Helper functions
const getDaysRemaining = (expiresAt: string): number | null => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return null
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

const formatRemainingExpiry = (expiresAt: string): string | null => {
  const duration = getRemainingExpiryDuration(expiresAt)
  if (!duration) return null
  if (duration.unit === 'days') {
    return t('admin.subscriptions.daysRemaining', { days: duration.days })
  }
  if (duration.hours) {
    return t('admin.subscriptions.hoursMinutesRemaining', {
      hours: duration.hours,
      minutes: duration.minutes
    })
  }
  return t('admin.subscriptions.minutesRemaining', { minutes: duration.minutes })
}

const isExpiringSoon = (expiresAt: string): boolean => {
  const days = getDaysRemaining(expiresAt)
  return days !== null && days <= 7
}

const getProgressWidth = (used: number | null | undefined, limit: number | null): string => {
  if (!limit || limit === 0) return '0%'
  const usedValue = used ?? 0
  const percentage = Math.min((usedValue / limit) * 100, 100)
  return `${percentage}%`
}

const getProgressClass = (used: number | null | undefined, limit: number | null): string => {
  if (!limit || limit === 0) return ''
  const usedValue = used ?? 0
  const percentage = (usedValue / limit) * 100
  if (percentage >= 90) return 'progress-bar-danger'
  if (percentage >= 70) return 'progress-bar-warning'
  return ''
}

const formatResetDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.resetInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.resetInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.resetInMinutes', { minutes: parts.minutes })
}

const formatQuotaEndDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.quotaEndsInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.quotaEndsInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.quotaEndsInMinutes', { minutes: parts.minutes })
}

const formatDailyUsageWindow = (subscription: UserSubscription): string => {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    return parts ? formatQuotaEndDuration(parts) : t('admin.subscriptions.windowNotActive')
  }

  return formatResetTime(subscription.daily_window_start, 'daily')
}

// Format reset time based on window start and period type
const formatResetTime = (windowStart: string | null, period: 'daily' | 'weekly' | 'monthly'): string => {
  if (!windowStart) return t('admin.subscriptions.windowNotActive')

  const start = new Date(windowStart)
  const now = new Date()

  // Calculate reset time based on period
  let resetTime: Date
  switch (period) {
    case 'daily':
      resetTime = new Date(start.getTime() + 24 * 60 * 60 * 1000)
      break
    case 'weekly':
      resetTime = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
      break
    case 'monthly':
      resetTime = new Date(start.getTime() + 30 * 24 * 60 * 60 * 1000)
      break
  }

  const parts = getRemainingDurationParts(resetTime, now)

  return parts ? formatResetDuration(parts) : t('admin.subscriptions.windowNotActive')
}

// Handle click outside to close dropdowns
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('[data-filter-user-search]')) showFilterUserDropdown.value = false
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false
  }
  if (
    rowMenuSubscriptionId.value !== null &&
    !target.closest('.subs-row-menu') &&
    !target.closest('.subs-more-btn')
  ) {
    closeRowMenu()
  }
}

onMounted(() => {
  loadUserColumnMode()
  loadSavedColumns()
  loadSubscriptions()
  loadGroups()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
})
</script>

<style scoped>
.subs-mobile-filters {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.subs-fab {
  display: none;
}
@media (max-width: 767px) {
  .subs-create-desktop {
    display: none;
  }
  .subs-fab {
    display: inline-flex;
  }
}
.usage-row {
  @apply space-y-1;
}

.usage-label {
  @apply w-10 flex-shrink-0 text-xs font-medium text-muted;
}

.usage-amount {
  @apply whitespace-nowrap text-xs tabular-nums text-muted;
}

.reset-info {
  @apply flex items-center gap-1 pl-12 text-[10px] text-accent;
}

.subs-use-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 9px;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--accent);
  cursor: pointer;
  transition: background 0.15s ease;
}

.subs-use-btn:hover {
  background: color-mix(in oklch, var(--accent) 10%, transparent);
}

.subs-more-btn {
  width: 28px;
  height: 28px;
}

.subs-dropdown {
  top: 100%;
  right: 0;
  margin-top: 6px;
  min-width: 200px;
  max-height: 60vh;
  overflow-y: auto;
}

.subs-dropdown-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.subs-filter-user-dropdown {
  top: 100%;
  left: 0;
  margin-top: 4px;
  width: 100%;
  max-height: 240px;
  overflow-y: auto;
}

/* ---------- Mobile card mode (DataTable built-in) ---------- */
@media (max-width: 767px) {
  :deep(.data-table-mobile-card [data-field='usage']) {
    flex-direction: column;
    align-items: stretch;
  }
  :deep(.data-table-mobile-card [data-field='usage'] > span:first-child) {
    margin-bottom: 4px;
  }
  :deep(.data-table-mobile-card [data-field='usage'] > div) {
    width: 100%;
    max-width: 100%;
    min-width: 0;
  }
}
</style>
