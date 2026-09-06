// Desktop-viewport detection + batched per-account usage-window loading for
// AccountsView.vue (frontend-health-cleanup 6.4 split). Batching exists so that
// scrolling through many rows doesn't fire one usage request per row.
import { ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { Account, AccountUsageInfo } from '@/types'
import { accountSupportsBatchUsage } from '../accountRowHelpers'

const DESKTOP_VIEWPORT_QUERY = '(min-width: 768px)'
const USAGE_BATCH_CACHE_TTL = 5 * 60 * 1000

/**
 * Desktop-viewport tracking + batched account usage loading. `setupViewportListener`/
 * `teardownViewportListener` must be called from the view's onMounted/onUnmounted so
 * the media-query listener lifecycle stays centralized with the other window listeners.
 */
export function useAccountUsageBatch() {
  const isDesktopViewport = ref(
    typeof window === 'undefined' ? true : window.matchMedia(DESKTOP_VIEWPORT_QUERY).matches
  )
  let desktopViewportMediaQuery: MediaQueryList | null = null
  let desktopViewportListener: ((event: MediaQueryListEvent) => void) | null = null

  const usageManualRefreshToken = ref(0)

  const usageBatchByAccountId = ref<Record<string, AccountUsageInfo | null>>({})
  const usageBatchErrorByAccountId = ref<Record<string, string | null>>({})
  const usageBatchLoadingByAccountId = ref<Record<string, boolean>>({})
  const usageBatchRequestTokenByAccountId = ref<Record<string, number>>({})
  const usageBatchCache = new Map<number, { data: AccountUsageInfo; ts: number }>()
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

  const setupViewportListener = () => {
    if (typeof window === 'undefined') return
    desktopViewportMediaQuery = window.matchMedia(DESKTOP_VIEWPORT_QUERY)
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

  const teardownViewportListener = () => {
    if (usageBatchFlushTimer !== null) {
      clearTimeout(usageBatchFlushTimer)
      usageBatchFlushTimer = null
    }
    pendingUsageBatchIds.clear()
    if (desktopViewportMediaQuery && desktopViewportListener) {
      if (typeof desktopViewportMediaQuery.removeEventListener === 'function') {
        desktopViewportMediaQuery.removeEventListener('change', desktopViewportListener)
      } else {
        desktopViewportMediaQuery.removeListener(desktopViewportListener)
      }
    }
    desktopViewportListener = null
    desktopViewportMediaQuery = null
  }

  // Called from the view's watch(accounts, ...) to drop batch state for rows that
  // have scrolled/paginated out of view.
  const pruneUsageBatchToVisibleAccounts = (rows: Account[]) => {
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
  }

  return {
    isDesktopViewport,
    usageManualRefreshToken,
    usageBatchByAccountId,
    usageBatchErrorByAccountId,
    usageBatchLoadingByAccountId,
    handleAccountUsageLoaded,
    queueBatchedUsage,
    pruneUsageBatchToVisibleAccounts,
    setupViewportListener,
    teardownViewportListener
  }
}

export type AccountUsageBatchState = ReturnType<typeof useAccountUsageBatch>
