// Data loading, sorting, column visibility and status-chip logic for AccountsView.vue
// (frontend-health-cleanup 6.4 split). Bundles everything that revolves around "what
// rows are currently loaded and how are they sorted/filtered/columned" so the view
// only has to wire this composable up to the table + filter-bar markup.
import { computed, reactive, ref, toRaw, watch, type ComputedRef, type Ref } from 'vue'
import { useTableLoader } from '@/composables/useTableLoader'
import { useAccountColumnVisibility } from '@/composables/useAccountColumnVisibility'
import { adminAPI } from '@/api/admin'
import type { Account, WindowStats } from '@/types'
import { buildDefaultTodayStats } from '../accountRowHelpers'

export const ACCOUNT_SORT_STORAGE_KEY = 'account-table-sort'
export type AccountSortOrder = 'asc' | 'desc'
export type AccountSortState = {
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

function loadInitialAccountSortState(): AccountSortState {
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

export type AccountStatusBucket = 'active' | 'rate_limited' | 'error' | 'unschedulable'

function accountStatusBucket(account: Account, now: number): AccountStatusBucket {
  if (account.status === 'error') return 'error'
  if (account.rate_limit_reset_at && new Date(account.rate_limit_reset_at).getTime() > now) {
    return 'rate_limited'
  }
  if (!account.schedulable || account.status !== 'active') return 'unschedulable'
  return 'active'
}

interface UseAccountListStateOptions {
  t: (key: string, params?: any) => string
  isSimpleMode: ComputedRef<boolean> | Ref<boolean>
  clearSelection: () => void
  /** Resets any ETag caches owned by sibling composables (auto-refresh / upstream billing). */
  resetCaches: () => void
  /** Bumps the usage-window manual-refresh token, same as an explicit manual refresh. */
  bumpUsageManualRefreshToken: () => void
}

/**
 * Table data loading, sorting, column visibility, today's-stats batch and status-chip
 * summary. Everything here revolves around `accounts`/`pagination`/`params` from the
 * shared useTableLoader instance.
 */
export function useAccountListState(options: UseAccountListStateOptions) {
  const { t, isSimpleMode, clearSelection, resetCaches, bumpUsageManualRefreshToken } = options

  const { hiddenColumns, isColumnVisible, toggleColumnVisibility } = useAccountColumnVisibility()

  const sortState = reactive<AccountSortState>(loadInitialAccountSortState())

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

  const hasPendingListSync = ref(false)
  const todayStatsByAccountId = ref<Record<string, WindowStats>>({})
  const todayStatsLoading = ref(false)
  const todayStatsError = ref<string | null>(null)
  const todayStatsReqSeq = ref(0)
  const pendingTodayStatsRefresh = ref(false)

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

  const isFirstLoad = ref(true)

  type AccountLoadOptions = {
    refreshTodayStats?: boolean
  }

  const load = async (loadOptions: AccountLoadOptions = {}) => {
    const requestParams = params as any
    syncAccountListDerivedParams()
    hasPendingListSync.value = false
    resetCaches()
    pendingTodayStatsRefresh.value = false
    if (isFirstLoad.value) {
      requestParams.lite = '1'
    }
    await baseLoad()
    if (isFirstLoad.value) {
      isFirstLoad.value = false
      delete requestParams.lite
    }
    if (loadOptions.refreshTodayStats !== false) await refreshTodayStatsBatch()
  }

  const reload = async () => {
    syncAccountListDerivedParams()
    hasPendingListSync.value = false
    resetCaches()
    pendingTodayStatsRefresh.value = false
    await baseReload()
    await refreshTodayStatsBatch()
  }

  const debouncedReload = () => {
    clearSelection()
    syncAccountListDerivedParams()
    hasPendingListSync.value = false
    resetCaches()
    pendingTodayStatsRefresh.value = true
    baseDebouncedReload()
  }

  const handlePageChange = (page: number) => {
    syncAccountListDerivedParams()
    hasPendingListSync.value = false
    resetCaches()
    pendingTodayStatsRefresh.value = true
    baseHandlePageChange(page)
  }

  const handlePageSizeChange = (size: number) => {
    syncAccountListDerivedParams()
    hasPendingListSync.value = false
    resetCaches()
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
    resetCaches()
    pendingTodayStatsRefresh.value = true
    load()
  }

  watch(loading, (isLoading, wasLoading) => {
    if (wasLoading && !isLoading && pendingTodayStatsRefresh.value) {
      pendingTodayStatsRefresh.value = false
      refreshTodayStatsBatch().catch((error) => {
        console.error('Failed to refresh account today stats after table load:', error)
      })
    }
  })

  const syncPendingListChanges = async () => {
    hasPendingListSync.value = false
    await load()
    // Keep behavior consistent with manual refresh.
    bumpUsageManualRefreshToken()
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
    if (!isSimpleMode.value) {
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

  // Shared by "select all N results" (useAccountSelection) and "bulk edit filtered
  // results" (useAccountBulkActions) — both need a plain snapshot of the current
  // filters/sort to hand to a fresh, page-agnostic API request.
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

  // Shared by the export flow (useAccountBulkActions) and row-filter-matching
  // (useAccountRowActions) — a plain snapshot of the current filters in the shape
  // the accounts list API expects.
  const buildAccountQueryFilters = () => ({
    platform: (params as any).platform || '',
    type: (params as any).type || '',
    status: (params as any).status || '',
    group: (params as any).group || '',
    privacy_mode: (params as any).privacy_mode || '',
    search: (params as any).search || '',
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  })

  return {
    hiddenColumns,
    isColumnVisible,
    sortState,
    shouldIncludeSchedulerScore,
    syncAccountListDerivedParams,
    accounts,
    loading,
    params,
    pagination,
    hasPendingListSync,
    todayStatsByAccountId,
    todayStatsLoading,
    todayStatsError,
    refreshTodayStatsBatch,
    isFirstLoad,
    load,
    reload,
    debouncedReload,
    handlePageChange,
    handlePageSizeChange,
    handleSort,
    syncPendingListChanges,
    toggleColumn,
    allColumns,
    toggleableColumns,
    cols,
    statusBucketCounts,
    statusBucketTotal,
    statusChips,
    applyStatusChip,
    buildBulkEditFilterSnapshot,
    buildAccountQueryFilters
  }
}

export type AccountListState = ReturnType<typeof useAccountListState>
