<template>
  <AppLayout>
    <div class="dash-page">
      <div v-if="snapshotError && !stats" class="notice notice-warning" role="alert">
        <span>{{ t('admin.dashboard.failedToLoad') }}</span>
        <button type="button" class="dash-panel-link" @click="loadDashboardStats">{{ t('common.refresh') }}</button>
      </div>
      <div v-if="loading && !stats" class="dash-loading">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <DashHeroSection
          :stats="stats"
          :hero-kicker="heroKicker"
          :hero-title-lead="heroTitleLead"
          :hero-description="heroDescription"
          :abnormal-accounts="abnormalAccounts"
          :service-tone="serviceTone"
          :service-status-label="serviceStatusLabel"
          :requests-delta="requestsDelta"
          :trend-bars="trendBars"
          :trend-axis-labels="trendAxisLabels"
          :granularity-options="granularityOptions"
          :charts-loading="chartsLoading"
          :on-date-range-change="onDateRangeChange"
          v-model:granularity="granularity"
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          @load-chart-data="loadChartData"
          @load-dashboard-stats="loadDashboardStats"
        />

        <DashStatGrid
          :stats="stats"
          :flat-spark="flatSpark"
          :requests-spark="requestsSpark"
          :tokens-spark="tokensSpark"
          :cost-spark="costSpark"
          :active-key-ratio="activeKeyRatio"
          :cache-hit-rate="cacheHitRate"
          :accounts-sub-label="accountsSubLabel"
          :accounts-delta="accountsDelta"
          :requests-delta="requestsDelta"
          :tokens-delta="tokensDelta"
          :cost-delta="costDelta"
          :today-token-breakdown-items="todayTokenBreakdownItems"
          :total-token-breakdown-items="totalTokenBreakdownItems"
          :today-financial-breakdown-items="todayFinancialBreakdownItems"
          :total-financial-breakdown-items="totalFinancialBreakdownItems"
        />

        <DashTrendDistRow
          :trend-bars="trendBars"
          :trend-subtitle="trendSubtitle"
          :trend-metric-options="trendMetricOptions"
          v-model:trend-metric="trendMetric"
          v-model:dist-view="distView"
          :dist-rows-loading="distRowsLoading"
          :dist-rows="distRows"
          @user-row-click="goToUserUsageById"
        />

        <DashHealthEventsRow :platform-health="platformHealth" :recent-events="recentEvents" />

        <DashUserTrendActionsRow
          :user-trend-loading="userTrendLoading"
          :user-trend-chart-data="userTrendChartData"
          :line-options="lineOptions"
          :can-use-batch-image="canUseBatchImage"
        />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const { t } = useI18n()
import type {
  GroupPlatform,
  TrendDataPoint,
  UserUsageTrendPoint
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DashHeroSection from '@/components/admin/dashboard/DashHeroSection.vue'
import DashStatGrid from '@/components/admin/dashboard/DashStatGrid.vue'
import DashTrendDistRow from '@/components/admin/dashboard/DashTrendDistRow.vue'
import DashHealthEventsRow from '@/components/admin/dashboard/DashHealthEventsRow.vue'
import DashUserTrendActionsRow from '@/components/admin/dashboard/DashUserTrendActionsRow.vue'
import { useDashboardRange } from '@/components/admin/dashboard/useDashboardRange'
import { useDashboardStats } from '@/components/admin/dashboard/useDashboardStats'
import {
  asNumber,
  buildSpark,
  formatCost,
  formatDuration,
  formatNumber,
  formatTokens
} from '@/components/admin/dashboard/useDashboardFormat'
import { platformFromModel, platformTileBackground } from '@/utils/platformTile'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { useTheme } from '@/composables/useTheme'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const router = useRouter()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

// Date range + granularity
const { granularity, startDate, endDate, granularityOptions, applyRangeGranularity } = useDashboardRange()

// Stats/chart data loading
const {
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
  platformHealthRaw,
  recentEvents,
  loadDashboardStats,
  loadChartData,
  loadPlatformHealth,
  loadRecentEvents
} = useDashboardStats({ startDate, endDate, granularity })

const trendMetric = ref<'requests' | 'tokens' | 'cost'>('requests')
const distView = ref<'models' | 'users'>('models')

const { isDark: isDarkMode } = useTheme()

/** Resolve a CSS color expression (var()/color-mix()) to a concrete value the Canvas API can use. */
const resolveCssColor = (value: string): string => {
  if (typeof document === 'undefined') return value
  const probe = document.createElement('span')
  probe.style.color = value
  document.body.appendChild(probe)
  const resolved = getComputedStyle(probe).color
  document.body.removeChild(probe)
  return resolved
}

// Chart colors (resolved from design tokens so chart.js never falls back to its default palette)
const chartColors = computed(() => {
  void isDarkMode.value
  return {
    text: resolveCssColor('var(--muted)'),
    grid: resolveCssColor('var(--border)')
  }
})

const CHART_PALETTE_TOKENS = [
  'var(--accent)',
  'var(--success)',
  'var(--warning)',
  'var(--danger)',
  'color-mix(in oklch, var(--accent) 55%, var(--success) 45%)',
  'color-mix(in oklch, var(--accent) 55%, var(--danger) 45%)',
  'color-mix(in oklch, var(--success) 55%, var(--warning) 45%)',
  'color-mix(in oklch, var(--warning) 55%, var(--danger) 45%)',
  'color-mix(in oklch, var(--accent) 70%, var(--foreground) 30%)',
  'color-mix(in oklch, var(--success) 70%, var(--foreground) 30%)',
  'color-mix(in oklch, var(--danger) 70%, var(--foreground) 30%)',
  'color-mix(in oklch, var(--warning) 70%, var(--foreground) 30%)'
]

const chartPaletteLine = computed(() => {
  void isDarkMode.value
  return CHART_PALETTE_TOKENS.map((token) => resolveCssColor(token))
})

const chartPaletteFill = computed(() => {
  void isDarkMode.value
  return CHART_PALETTE_TOKENS.map((token) => resolveCssColor(`color-mix(in oklch, ${token} 18%, transparent)`))
})

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, point.tokens)
  })

  const sortedDates = Array.from(allDates).sort()
  const lineColors = chartPaletteLine.value
  const fillColors = chartPaletteFill.value

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: lineColors[idx % lineColors.length],
    backgroundColor: fillColors[idx % fillColors.length],
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

// ---------------------------------------------------------------- hero copy
const displayLocale = (): string => {
  const lang = typeof document !== 'undefined' ? document.documentElement.lang : ''
  return lang === 'zh' ? 'zh-CN' : 'en-US'
}

const heroKicker = computed(() => {
  const now = new Date()
  let date = ''
  try {
    date = new Intl.DateTimeFormat(displayLocale(), { dateStyle: 'full' }).format(now)
  } catch {
    date = now.toDateString()
  }
  const hour = now.getHours()
  const key =
    hour < 12
      ? 'admin.dashboard.heroGreetingMorning'
      : hour < 18
        ? 'admin.dashboard.heroGreetingAfternoon'
        : 'admin.dashboard.heroGreetingEvening'
  return `${date} · ${t(key)}`
})

const abnormalAccounts = computed(
  () => asNumber(stats.value?.error_accounts) + asNumber(stats.value?.overload_accounts)
)

const serviceTone = computed<'success' | 'warning' | 'danger'>(() => {
  if (asNumber(stats.value?.error_accounts) > 0) return 'danger'
  if (asNumber(stats.value?.ratelimit_accounts) > 0) return 'warning'
  return 'success'
})

const serviceStatusLabel = computed(() => {
  if (serviceTone.value === 'danger') return t('admin.dashboard.statusError')
  if (serviceTone.value === 'warning') return t('admin.dashboard.statusRateLimited')
  return t('admin.dashboard.ok')
})

const heroTitleLead = computed(() =>
  serviceTone.value === 'success'
    ? t('admin.dashboard.heroTitleLead')
    : t('admin.dashboard.heroTitleAlertLead')
)

const heroDescription = computed(() => {
  const summary = t('admin.dashboard.heroSummary', {
    requests: formatNumber(stats.value?.today_requests),
    tokens: formatTokens(stats.value?.today_tokens),
    cost: formatCost(stats.value?.today_actual_cost),
    duration: formatDuration(asNumber(stats.value?.average_duration_ms))
  })
  const errors = asNumber(stats.value?.error_accounts)
  const limited = asNumber(stats.value?.ratelimit_accounts)
  if (errors === 0 && limited === 0) {
    return `${summary}${t('admin.dashboard.heroAllHealthy')}`
  }
  return `${summary}${t('admin.dashboard.heroIssues', { error: errors, ratelimit: limited })}`
})

const activeKeyRatio = computed(() => {
  const total = asNumber(stats.value?.total_api_keys)
  if (total <= 0) return '0'
  return Math.round((asNumber(stats.value?.active_api_keys) / total) * 100).toString()
})

const cacheHitRate = computed(() => {
  const total = asNumber(stats.value?.today_tokens)
  if (total <= 0) return '0'
  return Math.round((asNumber(stats.value?.today_cache_read_tokens) / total) * 100).toString()
})

const accountsSubLabel = computed(() => {
  const active = `${t('admin.dashboard.activeAccounts')} ${formatNumber(stats.value?.normal_accounts)}`
  const errors = asNumber(stats.value?.error_accounts)
  if (errors <= 0) return active
  return `${active} · ${t('admin.dashboard.statusError')} ${formatNumber(errors)}`
})

const accountsDelta = computed<{ text: string; tone: 'up' | 'down' | 'neutral' }>(() => {
  const errors = asNumber(stats.value?.error_accounts)
  if (errors > 0) {
    return { text: t('admin.dashboard.abnormalCount', { count: errors }), tone: 'down' }
  }
  return { text: t('admin.dashboard.accountsHealthy'), tone: 'up' }
})

// ------------------------------------------------------- trend derived data
const sparkPoints = computed(() => trendData.value.slice(-12))

const flatSpark = computed(() => buildSpark(sparkPoints.value.map(() => 1)))
const requestsSpark = computed(() => buildSpark(sparkPoints.value.map((point) => asNumber(point.requests))))
const tokensSpark = computed(() => buildSpark(sparkPoints.value.map((point) => asNumber(point.total_tokens))))
const costSpark = computed(() => buildSpark(sparkPoints.value.map((point) => asNumber(point.actual_cost))))

const periodDelta = (pick: (point: TrendDataPoint) => number) => {
  const points = trendData.value
  if (points.length < 2) return null
  const current = pick(points[points.length - 1])
  const previous = pick(points[points.length - 2])
  if (!Number.isFinite(previous) || previous <= 0) return null
  const change = ((current - previous) / previous) * 100
  const rounded = Math.round(change * 10) / 10
  return {
    text: `${rounded >= 0 ? '↑' : '↓'} ${Math.abs(rounded)}%`,
    tone: (rounded >= 0 ? 'up' : 'down') as 'up' | 'down'
  }
}

const requestsDelta = computed(() => periodDelta((point) => asNumber(point.requests)))
const tokensDelta = computed(() => periodDelta((point) => asNumber(point.total_tokens)))
const costDelta = computed(() => periodDelta((point) => asNumber(point.actual_cost)))

const trendMetricOptions = computed(() => [
  { value: 'requests' as const, label: t('admin.dashboard.requestsShort') },
  { value: 'tokens' as const, label: t('admin.dashboard.tokensShort') },
  { value: 'cost' as const, label: t('admin.dashboard.costShort') }
])

const trendMetricValue = (point: TrendDataPoint): number => {
  if (trendMetric.value === 'tokens') return asNumber(point.total_tokens)
  if (trendMetric.value === 'cost') return asNumber(point.actual_cost)
  return asNumber(point.requests)
}

const formatTrendLabel = (date: string): string => {
  if (!date) return ''
  const timePart = date.includes('T') ? date.split('T')[1] : date.includes(' ') ? date.split(' ')[1] : ''
  if (timePart) return timePart.slice(0, 5)
  const parts = date.split('-')
  if (parts.length >= 3) return `${Number(parts[1])}/${Number(parts[2])}`
  return date
}

const formatMetricValue = (value: number): string => {
  if (trendMetric.value === 'cost') return `$${formatCost(value)}`
  if (trendMetric.value === 'tokens') return formatTokens(value)
  return formatNumber(value)
}

const BAR_LAST = 'linear-gradient(180deg,var(--accent),color-mix(in oklch,var(--accent) 55%,transparent))'
const BAR_REST =
  'linear-gradient(180deg,color-mix(in oklch,var(--accent) 50%,transparent),color-mix(in oklch,var(--accent) 16%,transparent))'

const trendBars = computed(() => {
  const points = trendData.value.slice(-14)
  const max = Math.max(1, ...points.map(trendMetricValue))
  return points.map((point, index) => ({
    key: point.date || String(index),
    height: `${Math.max(4, Math.round((trendMetricValue(point) / max) * 100))}%`,
    label: formatTrendLabel(point.date),
    title: formatMetricValue(trendMetricValue(point)),
    fill: index === points.length - 1 ? BAR_LAST : BAR_REST
  }))
})

const trendAxisLabels = computed(() => {
  const bars = trendBars.value
  if (!bars.length) return ['', '', t('admin.dashboard.todayLabel')]
  return [
    bars[0].label,
    bars[Math.floor(bars.length / 2)].label,
    t('admin.dashboard.todayLabel')
  ]
})

const trendSubtitle = computed(() => {
  const points = trendData.value.slice(-14)
  const total = points.reduce((sum, point) => sum + trendMetricValue(point), 0)
  return t('admin.dashboard.trendSubtitle', {
    count: points.length,
    unit:
      granularity.value === 'hour'
        ? t('admin.dashboard.unitHour')
        : t('admin.dashboard.unitDay'),
    total: formatMetricValue(total)
  })
})

// -------------------------------------------------- model / spending split
interface DistRow {
  key: string
  name: string
  cost: string
  pct: number
  platform: GroupPlatform
  tile: string
  userId?: number
}

const distRowsLoading = computed(() =>
  distView.value === 'users' ? rankingLoading.value : chartsLoading.value
)

const distRows = computed<DistRow[]>(() => {
  if (distView.value === 'users') {
    if (rankingError.value) return []
    const items = [...rankingItems.value]
      .sort((a, b) => asNumber(b.actual_cost) - asNumber(a.actual_cost))
      .slice(0, 5)
    const total = items.reduce((sum, item) => sum + asNumber(item.actual_cost), 0) || 1
    return items.map((item) => ({
      key: `user-${item.user_id}`,
      name: item.username?.trim() || item.email?.trim() || `#${item.user_id}`,
      cost: formatCost(item.actual_cost),
      pct: Math.round((asNumber(item.actual_cost) / total) * 100),
      platform: 'openai' as GroupPlatform,
      tile: 'var(--accent)',
      userId: item.user_id
    }))
  }

  const sorted = [...modelStats.value].sort((a, b) => asNumber(b.actual_cost) - asNumber(a.actual_cost))
  if (!sorted.length) return []
  const total = sorted.reduce((sum, item) => sum + asNumber(item.actual_cost), 0) || 1
  const top = sorted.slice(0, 4)
  const rest = sorted.slice(4)
  const rows: DistRow[] = top.map((item) => {
    const platform = platformFromModel(item.model) as GroupPlatform
    return {
      key: item.model,
      name: item.model,
      cost: formatCost(item.actual_cost),
      pct: Math.round((asNumber(item.actual_cost) / total) * 100),
      platform,
      tile: platformTileBackground(platform)
    }
  })
  if (rest.length) {
    const restCost = rest.reduce((sum, item) => sum + asNumber(item.actual_cost), 0)
    rows.push({
      key: '__other__',
      name: t('admin.dashboard.spendingRankingOther'),
      cost: formatCost(restCost),
      pct: Math.round((restCost / total) * 100),
      platform: 'openai' as GroupPlatform,
      tile: 'var(--surface-tertiary)'
    })
  }
  return rows
})

// ------------------------------------------------------------ health/events
interface HealthRow {
  platform: GroupPlatform
  okPct: number
  limitPct: number
  errorPct: number
  text: string
}

const platformHealth = computed<HealthRow[]>(() =>
  platformHealthRaw.value.map((item) => {
    const ok = asNumber(item.available_count)
    const limited = asNumber(item.rate_limit_count)
    const errors = asNumber(item.error_count)
    const total = Math.max(1, asNumber(item.total_accounts) || ok + limited + errors)
    const parts = [t('admin.dashboard.healthOk', { ok, total })]
    if (limited > 0) parts.push(t('admin.dashboard.healthRateLimited', { count: limited }))
    if (errors > 0) parts.push(t('admin.dashboard.healthError', { count: errors }))
    return {
      platform: item.platform as GroupPlatform,
      okPct: (ok / total) * 100,
      limitPct: (limited / total) * 100,
      errorPct: (errors / total) * 100,
      text: parts.join(' · ')
    }
  })
)

const goToUserUsageById = (userId: number) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(userId),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Breakdown tooltips (kept from the previous dashboard, surfaced on the sparkline)
const todayTokenBreakdownItems = computed(() => [
  {
    key: 'actual',
    label: t('admin.dashboard.actual'),
    value: asNumber(stats.value?.today_actual_cost),
    textClass: 'dash-tone-success'
  },
  {
    key: 'account',
    label: t('admin.dashboard.accountCost'),
    value: asNumber(stats.value?.today_account_cost),
    textClass: 'dash-tone-warning'
  },
  {
    key: 'standard',
    label: t('admin.dashboard.standard'),
    value: asNumber(stats.value?.today_cost),
    textClass: 'dash-tone-muted'
  }
])

const totalTokenBreakdownItems = computed(() => [
  {
    key: 'actual',
    label: t('admin.dashboard.actual'),
    value: asNumber(stats.value?.total_actual_cost),
    textClass: 'dash-tone-success'
  },
  {
    key: 'account',
    label: t('admin.dashboard.accountCost'),
    value: asNumber(stats.value?.total_account_cost),
    textClass: 'dash-tone-warning'
  },
  {
    key: 'standard',
    label: t('admin.dashboard.standard'),
    value: asNumber(stats.value?.total_cost),
    textClass: 'dash-tone-muted'
  }
])

const financialBreakdownItems = computed(() => [
  {
    key: 'balance',
    label: t('admin.dashboard.balanceCost'),
    todayValue: asNumber(stats.value?.today_balance_actual_cost),
    totalValue: asNumber(stats.value?.total_balance_actual_cost),
    textClass: 'dash-tone-success'
  },
  {
    key: 'subscription',
    label: t('admin.dashboard.subscriptionCost'),
    todayValue: asNumber(stats.value?.today_subscription_actual_cost),
    totalValue: asNumber(stats.value?.total_subscription_actual_cost),
    textClass: 'dash-tone-accent'
  },
  {
    key: 'recharge',
    label: t('admin.dashboard.rechargeAmount'),
    todayValue: asNumber(stats.value?.today_recharge_amount),
    totalValue: asNumber(stats.value?.total_recharge_amount),
    textClass: 'dash-tone-success'
  },
  {
    key: 'refund',
    label: t('admin.dashboard.refundAmount'),
    todayValue: asNumber(stats.value?.today_refund_amount),
    totalValue: asNumber(stats.value?.total_refund_amount),
    textClass: 'dash-tone-danger'
  }
])

const todayFinancialBreakdownItems = computed(() =>
  financialBreakdownItems.value.map((item) => ({
    key: item.key,
    label: item.label,
    value: item.todayValue,
    textClass: item.textClass
  }))
)

const totalFinancialBreakdownItems = computed(() =>
  financialBreakdownItems.value.map((item) => ({
    key: item.key,
    label: item.label,
    value: item.totalValue,
    textClass: item.textClass
  }))
)

// Date range change handler: auto-pick granularity, then reload the
// range-scoped summary stats.
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  applyRangeGranularity(range)
  loadDashboardStats()
}

onMounted(() => {
  void refreshBatchImageAccess()
  loadDashboardStats()
  void loadPlatformHealth()
  void loadRecentEvents()
})
</script>

<style scoped>
/* ============================== page shell ============================== */
.dash-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dash-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
}

@media (max-width: 767px) {
  .dash-page {
    gap: 14px;
  }
}
</style>
