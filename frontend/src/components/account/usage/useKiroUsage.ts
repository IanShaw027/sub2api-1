import { computed, type ComputedRef, type Ref } from 'vue'
import type { AccountUsageInfo, WindowStats } from '@/types'

/** Kiro OAuth 30-day subscription quota derived state. */
export function useKiroUsage(usageInfo: Ref<AccountUsageInfo | null> | ComputedRef<AccountUsageInfo | null>) {
  const kiroQuotaStats = computed<WindowStats | null>(() => {
    if (usageInfo.value?.kiro_quota?.window_stats) return usageInfo.value.kiro_quota.window_stats
    return { requests: 0, tokens: 0, cost: 0, standard_cost: 0, user_cost: 0 }
  })

  const kiroOverageCapabilityValue = computed(() => (usageInfo.value?.kiro_overage_capability || '').trim().toUpperCase())

  const kiroOverageDisplay = computed(() => {
    if (usageInfo.value?.kiro_overage_enabled === true) return 'true'
    if (usageInfo.value?.kiro_overage_enabled === false) return 'false'
    const capability = kiroOverageCapabilityValue.value
    if (['SUPPORTED', 'ENABLED', 'AVAILABLE', 'CAPABLE'].includes(capability)) return 'false'
    return 'null'
  })

  const formatKiroMoney = (value?: number | null): string => {
    if (value == null || Number.isNaN(value)) return '0.00'
    return value.toFixed(2)
  }

  return { kiroQuotaStats, kiroOverageDisplay, formatKiroMoney }
}
