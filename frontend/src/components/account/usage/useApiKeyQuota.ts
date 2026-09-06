import { computed, type ComputedRef } from 'vue'
import type { Account } from '@/types'

export interface QuotaBarInfo {
  utilization: number
  resetsAt: string | null
}

/**
 * API Key / Bedrock account quota progress bars (1d / 7d / total), derived purely from
 * the account's quota_* fields and extra reset-mode metadata.
 */
export function useApiKeyQuota(account: ComputedRef<Account> | { value: Account }) {
  const makeQuotaBar = (
    used: number,
    limit: number,
    startKey?: string
  ): QuotaBarInfo => {
    const utilization = limit > 0 ? (used / limit) * 100 : 0
    let resetsAt: string | null = null
    if (startKey) {
      const extra = account.value.extra as Record<string, unknown> | undefined
      const isDaily = startKey.includes('daily')
      const mode = isDaily
        ? (extra?.quota_daily_reset_mode as string) || 'rolling'
        : (extra?.quota_weekly_reset_mode as string) || 'rolling'

      if (mode === 'fixed') {
        // Use pre-computed next reset time for fixed mode
        const resetAtKey = isDaily ? 'quota_daily_reset_at' : 'quota_weekly_reset_at'
        resetsAt = (extra?.[resetAtKey] as string) || null
      } else {
        // Rolling mode: compute from start + period
        const startStr = extra?.[startKey] as string | undefined
        if (startStr) {
          const startDate = new Date(startStr)
          const periodMs = isDaily ? 24 * 60 * 60 * 1000 : 7 * 24 * 60 * 60 * 1000
          resetsAt = new Date(startDate.getTime() + periodMs).toISOString()
        }
      }
    }
    return { utilization, resetsAt }
  }

  const hasApiKeyQuota = computed(() => {
    if (account.value.type !== 'apikey' && account.value.type !== 'bedrock') return false
    return (
      (account.value.quota_daily_limit ?? 0) > 0 ||
      (account.value.quota_weekly_limit ?? 0) > 0 ||
      (account.value.quota_limit ?? 0) > 0
    )
  })

  const quotaDailyBar = computed((): QuotaBarInfo | null => {
    const limit = account.value.quota_daily_limit ?? 0
    if (limit <= 0) return null
    return makeQuotaBar(account.value.quota_daily_used ?? 0, limit, 'quota_daily_start')
  })

  const quotaWeeklyBar = computed((): QuotaBarInfo | null => {
    const limit = account.value.quota_weekly_limit ?? 0
    if (limit <= 0) return null
    return makeQuotaBar(account.value.quota_weekly_used ?? 0, limit, 'quota_weekly_start')
  })

  const quotaTotalBar = computed((): QuotaBarInfo | null => {
    const limit = account.value.quota_limit ?? 0
    if (limit <= 0) return null
    return makeQuotaBar(account.value.quota_used ?? 0, limit)
  })

  return { hasApiKeyQuota, quotaDailyBar, quotaWeeklyBar, quotaTotalBar }
}
