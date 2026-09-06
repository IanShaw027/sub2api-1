// Auto-refresh dropdown + periodic incremental account refresh for AccountsView.vue
// (frontend-health-cleanup 6.4 split).
import { ref, toRaw, type ComputedRef, type Ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { adminAPI } from '@/api/admin'
import { buildGrokUsageRefreshKey, buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'
import type { Account } from '@/types'
import type { AccountSortOrder } from './useAccountListState'

const AUTO_REFRESH_STORAGE_KEY = 'account-auto-refresh'
const AUTO_REFRESH_INTERVALS = [5, 10, 15, 30] as const
const AUTO_REFRESH_SILENT_WINDOW_MS = 15000

interface UseAccountAutoRefreshOptions {
  t: (key: string, params?: any) => string
  accounts: Ref<Account[]>
  pagination: { page: number; page_size: number; total: number; pages: number }
  params: Record<string, unknown>
  loading: Ref<boolean>
  hasPendingListSync: Ref<boolean>
  syncAccountListDerivedParams: () => void
  refreshTodayStatsBatch: () => Promise<void>
  /** Relays a refreshed row into any dialog/menu state that's currently holding it. */
  syncAccountRefs: (nextAccount: Account) => void
  upstreamBillingNow: Ref<number>
  loadUpstreamBillingProbeGlobalState: () => Promise<void>
  load: () => Promise<void>
  usageManualRefreshToken: Ref<number>
  isAnyModalOpen: ComputedRef<boolean>
  menu: { show: boolean }
  showAccountToolsDropdown: Ref<boolean>
  /** Owned by useAccountToolbarMenus — read here only to pause ticking while it's open. */
  showAutoRefreshDropdown: Ref<boolean>
}

export function useAccountAutoRefresh(options: UseAccountAutoRefreshOptions) {
  const {
    t,
    accounts,
    pagination,
    params,
    loading,
    hasPendingListSync,
    syncAccountListDerivedParams,
    refreshTodayStatsBatch,
    syncAccountRefs,
    upstreamBillingNow,
    loadUpstreamBillingProbeGlobalState,
    load,
    usageManualRefreshToken,
    isAnyModalOpen,
    menu,
    showAccountToolsDropdown,
    showAutoRefreshDropdown
  } = options

  const autoRefreshIntervals = AUTO_REFRESH_INTERVALS
  const autoRefreshEnabled = ref(false)
  const autoRefreshIntervalSeconds = ref<(typeof autoRefreshIntervals)[number]>(30)
  const autoRefreshCountdown = ref(0)
  const autoRefreshETag = ref<string | null>(null)
  const autoRefreshFetching = ref(false)
  const autoRefreshSilentUntil = ref(0)

  const autoRefreshIntervalLabel = (sec: number) => {
    if (sec === 5) return t('admin.accounts.refreshInterval5s')
    if (sec === 10) return t('admin.accounts.refreshInterval10s')
    if (sec === 15) return t('admin.accounts.refreshInterval15s')
    if (sec === 30) return t('admin.accounts.refreshInterval30s')
    return `${sec}s`
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

  const resetETag = () => {
    autoRefreshETag.value = null
  }

  /** Called once from the view's onMounted, after the initial load kicks off. */
  const initAutoRefreshTicker = () => {
    if (autoRefreshEnabled.value) {
      autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
      resumeAutoRefresh()
    } else {
      pauseAutoRefresh()
    }
  }

  return {
    autoRefreshIntervals,
    autoRefreshEnabled,
    autoRefreshIntervalSeconds,
    autoRefreshCountdown,
    autoRefreshETag,
    autoRefreshFetching,
    autoRefreshIntervalLabel,
    setAutoRefreshEnabled,
    setAutoRefreshInterval,
    enterAutoRefreshSilentWindow,
    refreshAccountsIncrementally,
    handleManualRefresh,
    pauseAutoRefresh,
    resumeAutoRefresh,
    resetETag,
    initAutoRefreshTicker
  }
}

export type AccountAutoRefreshState = ReturnType<typeof useAccountAutoRefresh>
