import { computed, type ComputedRef, type Ref } from 'vue'
import type { WindowStats } from '@/types'
import { formatCompactNumber } from '@/utils/format'

/**
 * Formatters for the "today stats" row (requests / tokens / cost / user_cost) shared
 * between the Gemini usage block and the plain Key/Bedrock today-stats row.
 */
export function useTodayStatsFormat(todayStats: Ref<WindowStats | null | undefined> | ComputedRef<WindowStats | null | undefined>) {
  const formatKeyRequests = computed(() => {
    if (!todayStats.value) return ''
    return formatCompactNumber(todayStats.value.requests, { allowBillions: false })
  })

  const formatKeyTokens = computed(() => {
    if (!todayStats.value) return ''
    return formatCompactNumber(todayStats.value.tokens)
  })

  const formatKeyCost = computed(() => {
    if (!todayStats.value) return '0.00'
    return todayStats.value.cost.toFixed(2)
  })

  const formatKeyUserCost = computed(() => {
    if (!todayStats.value || todayStats.value.user_cost == null) return '0.00'
    return todayStats.value.user_cost.toFixed(2)
  })

  return { formatKeyRequests, formatKeyTokens, formatKeyCost, formatKeyUserCost }
}
