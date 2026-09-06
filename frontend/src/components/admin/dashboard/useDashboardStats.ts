/**
 * Loads all dashboard snapshot data: the summary stats + trend + model split
 * snapshot, the per-user usage trend, the spending ranking, platform health,
 * and the recent-events feed. Depends on the caller's date-range/granularity
 * refs so it can be re-run whenever they change.
 */
import { ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  ModelStat,
  TrendDataPoint,
  UserSpendingRankingItem,
  UserUsageTrendPoint
} from '@/types'
import { asNumber, formatEventTime } from './useDashboardFormat'

interface PlatformHealthRawItem {
  platform: string
  total_accounts: number
  available_count: number
  rate_limit_count: number
  error_count: number
}

interface EventRow {
  id: number
  time: string
  tone: 'danger' | 'warning' | 'accent' | 'success'
  message: string
}

const severityTone = (severity: string, statusCode: number): EventRow['tone'] => {
  const value = (severity || '').toLowerCase()
  if (value === 'critical' || value === 'fatal' || value === 'error') return 'danger'
  if (value === 'warning' || value === 'warn' || statusCode === 429) return 'warning'
  if (value === 'info') return 'accent'
  return statusCode >= 500 ? 'danger' : 'warning'
}

export function useDashboardStats(range: {
  startDate: Ref<string>
  endDate: Ref<string>
  granularity: Ref<'day' | 'hour'>
}) {
  const { t } = useI18n()
  const appStore = useAppStore()

  const stats = ref<DashboardStats | null>(null)
  const loading = ref(false)
  const chartsLoading = ref(false)
  const userTrendLoading = ref(false)
  const rankingLoading = ref(false)
  const rankingError = ref(false)
  const snapshotError = ref(false)

  // Chart data
  const trendData = ref<TrendDataPoint[]>([])
  const modelStats = ref<ModelStat[]>([])
  const userTrend = ref<UserUsageTrendPoint[]>([])
  const rankingItems = ref<UserSpendingRankingItem[]>([])
  const rankingTotalActualCost = ref(0)
  const rankingTotalRequests = ref(0)
  const rankingTotalTokens = ref(0)
  let chartLoadSeq = 0
  let usersTrendLoadSeq = 0
  let rankingLoadSeq = 0
  const rankingLimit = 12

  const platformHealthRaw = ref<PlatformHealthRawItem[]>([])
  const recentEvents = ref<EventRow[]>([])

  // Load data
  const loadDashboardSnapshot = async (includeStats: boolean) => {
    const currentSeq = ++chartLoadSeq
    if (includeStats && !stats.value) {
      loading.value = true
    }
    chartsLoading.value = true
    if (includeStats) snapshotError.value = false
    try {
      const response = await adminAPI.dashboard.getSnapshotV2({
        start_date: range.startDate.value,
        end_date: range.endDate.value,
        granularity: range.granularity.value,
        include_stats: includeStats,
        include_trend: true,
        include_model_stats: true,
        include_group_stats: false,
        include_users_trend: false
      })
      if (currentSeq !== chartLoadSeq) return
      if (includeStats && response.stats) {
        stats.value = response.stats
      }
      trendData.value = response.trend || []
      modelStats.value = response.models || []
    } catch (error) {
      if (currentSeq !== chartLoadSeq) return
      appStore.showError(t('admin.dashboard.failedToLoad'))
      if (includeStats) snapshotError.value = true
      console.error('Error loading dashboard snapshot:', error)
    } finally {
      if (currentSeq === chartLoadSeq) {
        loading.value = false
        chartsLoading.value = false
      }
    }
  }

  const loadUsersTrend = async () => {
    const currentSeq = ++usersTrendLoadSeq
    userTrendLoading.value = true
    try {
      const response = await adminAPI.dashboard.getUserUsageTrend({
        start_date: range.startDate.value,
        end_date: range.endDate.value,
        granularity: range.granularity.value,
        limit: 12
      })
      if (currentSeq !== usersTrendLoadSeq) return
      userTrend.value = response.trend || []
    } catch (error) {
      if (currentSeq !== usersTrendLoadSeq) return
      console.error('Error loading users trend:', error)
      userTrend.value = []
    } finally {
      if (currentSeq === usersTrendLoadSeq) {
        userTrendLoading.value = false
      }
    }
  }

  const loadUserSpendingRanking = async () => {
    const currentSeq = ++rankingLoadSeq
    rankingLoading.value = true
    rankingError.value = false
    try {
      const response = await adminAPI.dashboard.getUserSpendingRanking({
        start_date: range.startDate.value,
        end_date: range.endDate.value,
        limit: rankingLimit
      })
      if (currentSeq !== rankingLoadSeq) return
      rankingItems.value = response.ranking || []
      rankingTotalActualCost.value = response.total_actual_cost || 0
      rankingTotalRequests.value = response.total_requests || 0
      rankingTotalTokens.value = response.total_tokens || 0
    } catch (error) {
      if (currentSeq !== rankingLoadSeq) return
      console.error('Error loading user spending ranking:', error)
      rankingItems.value = []
      rankingTotalActualCost.value = 0
      rankingTotalRequests.value = 0
      rankingTotalTokens.value = 0
      rankingError.value = true
    } finally {
      if (currentSeq === rankingLoadSeq) {
        rankingLoading.value = false
      }
    }
  }

  /** Per-platform account availability powers the "platform health" panel. */
  const loadPlatformHealth = async () => {
    try {
      const response = await adminAPI.ops.getAccountAvailabilityStats()
      const platforms = response?.platform ? Object.values(response.platform) : []
      platformHealthRaw.value = platforms
        .filter((item) => item && asNumber(item.total_accounts) > 0)
        .sort((a, b) => asNumber(b.total_accounts) - asNumber(a.total_accounts))
    } catch (error) {
      console.error('Error loading platform availability:', error)
      platformHealthRaw.value = []
    }
  }

  /** Latest unresolved ops errors power the "recent events" panel. */
  const loadRecentEvents = async () => {
    try {
      const response = await adminAPI.ops.listErrorLogs({
        page: 1,
        page_size: 6,
        sort_by: 'created_at',
        sort_order: 'desc'
      })
      const items = response?.items || []
      recentEvents.value = items.map((item) => ({
        id: item.id,
        time: formatEventTime(item.created_at),
        tone: severityTone(item.severity, asNumber(item.status_code)),
        message: [item.account_name || item.platform, item.message].filter(Boolean).join(' · ')
      }))
    } catch (error) {
      console.error('Error loading recent events:', error)
      recentEvents.value = []
    }
  }

  const loadDashboardStats = async () => {
    await Promise.all([
      loadDashboardSnapshot(true),
      loadUsersTrend(),
      loadUserSpendingRanking()
    ])
  }

  const loadChartData = async () => {
    await Promise.all([
      loadDashboardSnapshot(false),
      loadUsersTrend(),
      loadUserSpendingRanking()
    ])
  }

  return {
    stats,
    loading,
    chartsLoading,
    userTrendLoading,
    rankingLoading,
    rankingError,
    snapshotError,
    trendData,
    modelStats,
    userTrend,
    rankingItems,
    rankingTotalActualCost,
    rankingTotalRequests,
    rankingTotalTokens,
    platformHealthRaw,
    recentEvents,
    loadDashboardStats,
    loadChartData,
    loadPlatformHealth,
    loadRecentEvents
  }
}
