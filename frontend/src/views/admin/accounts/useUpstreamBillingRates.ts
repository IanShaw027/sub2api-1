// Upstream billing rate polling + single/bulk "probe now" actions for
// AccountsView.vue (frontend-health-cleanup 6.4 split).
import { reactive, ref, toRaw, watch, type ComputedRef, type Ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Account, UpstreamBillingProbeSnapshot } from '@/types'
import type { AccountSortState } from './useAccountListState'

interface UseUpstreamBillingRatesOptions {
  t: (key: string, params?: any) => string
  appStore: { showError: (message: string) => void; showSuccess: (message: string) => void }
  accounts: Ref<Account[]>
  pagination: { page: number; page_size: number; total: number; pages: number }
  params: Record<string, unknown>
  sortState: AccountSortState
  loading: Ref<boolean>
  syncAccountListDerivedParams: () => void
  /** Relays a refreshed row into any dialog/menu state that's currently holding it. */
  syncAccountRefs: (nextAccount: Account) => void
  /** listState.load — used to reconcile a page-boundary crossing after a probe. */
  load: (options?: { refreshTodayStats?: boolean }) => Promise<void>
  isAnyModalOpen: ComputedRef<boolean>
  menu: { show: boolean }
  showAccountToolsDropdown: Ref<boolean>
  showAutoRefreshDropdown: Ref<boolean>
  selIds: Ref<number[]>
  /** useAccountRowActions.patchAccountInList — applies a merged row back into the table. */
  patchAccountInList: (updatedAccount: Account) => void
}

export function useUpstreamBillingRates(options: UseUpstreamBillingRatesOptions) {
  const {
    t,
    appStore,
    accounts,
    pagination,
    params,
    sortState,
    loading,
    syncAccountListDerivedParams,
    syncAccountRefs,
    load,
    isAnyModalOpen,
    menu,
    showAccountToolsDropdown,
    showAutoRefreshDropdown,
    selIds,
    patchAccountInList
  } = options

  const probingUpstreamBilling = reactive(new Set<number>())
  const upstreamBillingProbeGloballyEnabled = ref<boolean | undefined>(undefined)
  const upstreamBillingNow = ref(Date.now())
  const upstreamBillingRateETag = ref<string | null>(null)
  const upstreamBillingRateRefreshing = ref(false)
  let upstreamBillingRateAbortController: AbortController | null = null

  useIntervalFn(() => { upstreamBillingNow.value = Date.now() }, 60_000)

  // Mirrors the "bump the upstream-billing clock whenever a table load just
  // finished" half of the original combined `watch(loading, ...)` — the other
  // half (today-stats pending refresh) lives in useAccountListState.ts.
  watch(loading, (isLoading, wasLoading) => {
    if (wasLoading && !isLoading) {
      upstreamBillingNow.value = Date.now()
    }
  })

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

  const loadUpstreamBillingProbeGlobalState = async () => {
    try {
      const settings = await adminAPI.accounts.getUpstreamBillingProbeSettings()
      upstreamBillingProbeGloballyEnabled.value = settings.enabled
    } catch (error) {
      console.error('Failed to load upstream billing probe settings:', error)
    }
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

  const resetETag = () => {
    upstreamBillingRateETag.value = null
  }

  const teardown = () => {
    upstreamBillingRateAbortController?.abort()
  }

  return {
    probingUpstreamBilling,
    upstreamBillingProbeGloballyEnabled,
    upstreamBillingNow,
    refreshUpstreamBillingSortedList,
    loadUpstreamBillingProbeGlobalState,
    handleProbeUpstreamBilling,
    handleBulkProbeUpstreamBilling,
    resetETag,
    teardown
  }
}

export type UpstreamBillingRatesState = ReturnType<typeof useUpstreamBillingRates>
