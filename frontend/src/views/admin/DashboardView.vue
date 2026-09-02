<template>
  <AppLayout>
    <div class="dash-page">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <GlassCard class="dash-hero" padding="lg">
          <div class="dash-hero-copy">
            <p class="dash-hero-kicker">{{ t('admin.dashboard.description') }}</p>
            <h1 class="dash-hero-title">
              {{ t('admin.dashboard.todayRequests') }}
              <span class="dash-hero-accent">{{ stats.today_requests }}</span>
            </h1>
            <p class="dash-hero-desc">
              {{ stats.normal_accounts }} {{ t('common.active') }}
              · {{ stats.error_accounts }} {{ t('common.error') }}
              · {{ formatNumber(stats.total_requests) }} {{ t('common.total') }}
            </p>
            <div class="dash-hero-actions">
              <Button size="md" @click="router.push('/admin/ops')">{{ t('nav.ops') }}</Button>
              <Button variant="secondary" size="md" @click="router.push('/admin/accounts')">
                {{ t('admin.dashboard.manageAccounts') }}
                <span v-if="stats.error_accounts > 0" class="dash-hero-badge">{{ stats.error_accounts }}</span>
              </Button>
            </div>
          </div>
          <div class="dash-hero-stats">
            <div class="dash-hero-mini">
              <span>{{ t('admin.dashboard.ok') }}</span>
              <strong>
                <StatusBadge tone="success" dot :label="t('common.active')" />
              </strong>
              <span>{{ stats.normal_accounts }} / {{ stats.total_accounts }}</span>
            </div>
            <div class="dash-hero-mini">
              <span>RPM</span>
              <strong>{{ formatTokens(stats.rpm) }}</strong>
              <span>TPM {{ formatTokens(stats.tpm) }}</span>
            </div>
            <div class="dash-hero-mini">
              <span>{{ t('admin.dashboard.avgResponse') }}</span>
              <strong>{{ formatDuration(stats.average_duration_ms) }}</strong>
              <span>{{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}</span>
            </div>
          </div>
          <div class="dash-hero-bars" aria-hidden="true">
            <span v-for="(h, i) in heroBarHeights" :key="i" :style="{ height: h }"></span>
          </div>
        </GlassCard>

        <div class="dash-stat-grid">
          <StatCard :label="t('admin.dashboard.apiKeys')" :value="stats.total_api_keys" :sub="`${stats.active_api_keys} ${t('common.active')}`" />
          <StatCard :label="t('admin.dashboard.accounts')" :value="stats.total_accounts" :sub="`${stats.normal_accounts} ${t('common.active')}`" />
          <StatCard :label="t('admin.dashboard.users')" :value="`+${stats.today_new_users}`" :sub="`${t('common.total')}: ${formatNumber(stats.total_users)}`" />
          <StatCard :label="t('admin.dashboard.performance')" :value="formatTokens(stats.rpm)" sub="RPM" />
          <StatCard :label="t('admin.dashboard.avgResponse')" :value="formatDuration(stats.average_duration_ms)" :sub="`${stats.active_users} ${t('admin.dashboard.activeUsers')}`" />
          <StatCard :label="t('admin.dashboard.todayRequests')" :value="stats.today_requests" :sub="`${t('common.total')}: ${formatNumber(stats.total_requests)}`" />
          <StatCard :label="t('admin.dashboard.todayTokens')" :value="formatTokens(stats.today_tokens)">
            <template #sparkline>
              <HelpTooltip width-class="w-56">
                <template #trigger>
                  <div class="dash-breakdown">
                    <template v-for="(item, index) in todayTokenBreakdownItems" :key="item.key">
                      <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                      <span v-if="index < todayTokenBreakdownItems.length - 1" class="text-muted">/</span>
                    </template>
                  </div>
                </template>
                <div class="space-y-1.5">
                  <div v-for="item in todayTokenBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                    <span>{{ item.label }}</span>
                    <span class="font-semibold">${{ formatCost(item.value) }}</span>
                  </div>
                </div>
              </HelpTooltip>
            </template>
          </StatCard>
          <StatCard :label="t('admin.dashboard.totalTokens')" :value="formatTokens(stats.total_tokens)">
            <template #sparkline>
              <HelpTooltip width-class="w-56">
                <template #trigger>
                  <div class="dash-breakdown">
                    <template v-for="(item, index) in totalTokenBreakdownItems" :key="item.key">
                      <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                      <span v-if="index < totalTokenBreakdownItems.length - 1" class="text-muted">/</span>
                    </template>
                  </div>
                </template>
                <div class="space-y-1.5">
                  <div v-for="item in totalTokenBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                    <span>{{ item.label }}</span>
                    <span class="font-semibold">${{ formatCost(item.value) }}</span>
                  </div>
                </div>
              </HelpTooltip>
            </template>
          </StatCard>
          <StatCard :label="t('admin.dashboard.todayCost')" :value="`$${formatCost(stats.today_actual_cost)}`">
            <template #sparkline>
              <HelpTooltip width-class="w-56">
                <template #trigger>
                  <div class="dash-breakdown">
                    <template v-for="(item, index) in todayFinancialBreakdownItems" :key="item.key">
                      <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                      <span v-if="index < todayFinancialBreakdownItems.length - 1" class="text-muted">/</span>
                    </template>
                  </div>
                </template>
                <div class="space-y-1.5">
                  <div v-for="item in todayFinancialBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                    <span>{{ item.label }}</span>
                    <span class="font-semibold">${{ formatCost(item.value) }}</span>
                  </div>
                </div>
              </HelpTooltip>
            </template>
          </StatCard>
          <StatCard :label="t('admin.dashboard.totalCost')" :value="`$${formatCost(stats.total_actual_cost)}`">
            <template #sparkline>
              <HelpTooltip width-class="w-56">
                <template #trigger>
                  <div class="dash-breakdown">
                    <template v-for="(item, index) in totalFinancialBreakdownItems" :key="item.key">
                      <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                      <span v-if="index < totalFinancialBreakdownItems.length - 1" class="text-muted">/</span>
                    </template>
                  </div>
                </template>
                <div class="space-y-1.5">
                  <div v-for="item in totalFinancialBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                    <span>{{ item.label }}</span>
                    <span class="font-semibold">${{ formatCost(item.value) }}</span>
                  </div>
                </div>
              </HelpTooltip>
            </template>
          </StatCard>
        </div>

        <GlassCard padding="md">
          <h2 class="dash-section-title">{{ t('admin.dashboard.quickActions') }}</h2>
          <div class="dash-actions">
            <button
              v-if="canUseBatchImage"
              type="button"
              class="dash-action"
              @click="router.push('/batch-image')"
            >
              <span class="dash-action-icon"><Icon name="sparkles" size="md" :stroke-width="2" /></span>
              <span class="min-w-0 flex-1 text-left">
                <span class="dash-action-title">{{ t('admin.dashboard.batchImage') }}</span>
                <span class="dash-action-desc">{{ t('admin.dashboard.batchImageDesc') }}</span>
              </span>
              <Icon name="chevronRight" size="sm" class="text-muted" />
            </button>
            <button type="button" class="dash-action" @click="router.push('/admin/groups')">
              <span class="dash-action-icon"><Icon name="grid" size="md" :stroke-width="2" /></span>
              <span class="min-w-0 flex-1 text-left">
                <span class="dash-action-title">{{ t('admin.dashboard.groupPricing') }}</span>
                <span class="dash-action-desc">{{ t('admin.dashboard.groupPricingDesc') }}</span>
              </span>
              <Icon name="chevronRight" size="sm" class="text-muted" />
            </button>
          </div>
        </GlassCard>

        <div class="dash-charts">
          <GlassCard padding="md">
            <div class="dash-toolbar">
              <div class="flex items-center gap-2">
                <span class="dash-label">{{ t('admin.dashboard.timeRange') }}:</span>
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <Button variant="secondary" :disabled="chartsLoading" @click="loadDashboardStats">
                {{ t('common.refresh') }}
              </Button>
              <div class="ml-auto flex items-center gap-2">
                <span class="dash-label">{{ t('admin.dashboard.granularity') }}:</span>
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </GlassCard>

          <div class="dash-chart-grid">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <GlassCard padding="md">
            <h3 class="dash-section-title">{{ t('admin.dashboard.recentUsage') }} (Top 12)</h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div v-else class="flex h-full items-center justify-center text-sm text-muted">
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </GlassCard>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import StatCard from '@/components/ui/StatCard.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

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
import { Line } from 'vue-chartjs'

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

const appStore = useAppStore()
const router = useRouter()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const heroBarHeights = computed(() => {
  const points = trendData.value.slice(-14)
  const max = Math.max(1, ...points.map((point) => point.requests || 0))
  if (!points.length) {
    return Array.from({ length: 14 }, (_, i) => `${8 + ((i * 17) % 40)}px`)
  }
  return points.map((point) => `${Math.max(8, Math.round(((point.requests || 0) / max) * 48))}px`)
})
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

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

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
  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16',
    '#06b6d4',
    '#a855f7'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

// Format helpers
const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}

const asNumber = (value: number | null | undefined): number => {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

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

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  // Auto-select granularity based on date range
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  // The summary cards are scoped to the selected range as well, so refresh
  // the snapshot stats when the date window changes.
  loadDashboardStats()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
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
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
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
      start_date: startDate.value,
      end_date: endDate.value,
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

onMounted(() => {
  void refreshBatchImageAccess()
  loadDashboardStats()
})
</script>

<style scoped>
.dash-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.dash-hero {
  position: relative;
  overflow: hidden;
  display: grid;
  grid-template-columns: 1.25fr 1fr;
  gap: 24px;
}
.dash-hero-title {
  margin: 0;
  font-family: var(--display);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
}
.dash-hero-accent { color: var(--accent); }
.dash-hero-kicker,
.dash-hero-desc,
.dash-label {
  color: var(--muted);
  font-size: 13px;
}
.dash-hero-actions {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}
.dash-hero-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--danger);
  color: #fff;
  font-size: 10.5px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.dash-hero-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  align-self: center;
}
.dash-hero-mini {
  padding: 14px 16px;
  border-radius: 12px;
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  border: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.dash-hero-mini span { font-size: 11.5px; color: var(--muted); font-weight: 600; }
.dash-hero-mini strong {
  font-family: var(--display);
  font-size: 20px;
  font-weight: 800;
}
.dash-hero-bars {
  grid-column: 1 / -1;
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 48px;
}
.dash-hero-bars span {
  flex: 1;
  border-radius: 3px;
  background: color-mix(in oklch, var(--accent) 45%, transparent);
}
.dash-stat-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}
.dash-section-title {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
}
.dash-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.dash-action {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 12px;
  background: color-mix(in oklch, var(--surface-secondary) 70%, transparent);
  border: 0;
  cursor: pointer;
  text-align: left;
}
.dash-action-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
}
.dash-action-title { display: block; font-size: 14px; font-weight: 600; color: var(--foreground); }
.dash-action-desc { display: block; font-size: 12px; color: var(--muted); }
.dash-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 16px;
}
.dash-chart-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.dash-charts { display: flex; flex-direction: column; gap: 16px; }
.dash-breakdown {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
}
.dash-tone-success { color: var(--success-text); }
.dash-tone-warning { color: var(--warning-text); }
.dash-tone-muted { color: var(--muted); }
.dash-tone-accent { color: var(--accent); }
.dash-tone-danger { color: var(--danger-text); }
@media (max-width: 1100px) {
  .dash-hero, .dash-stat-grid, .dash-chart-grid, .dash-actions, .dash-hero-stats {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 767px) {
  .dash-hero, .dash-stat-grid, .dash-chart-grid, .dash-actions, .dash-hero-stats {
    grid-template-columns: 1fr;
  }
  .dash-hero-title { font-size: 24px; }
}
</style>
