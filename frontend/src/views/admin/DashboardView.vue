<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Calm loading skeleton for KPI strip -->
      <div v-if="loading" class="space-y-4">
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <div v-for="n in 5" :key="`kpi-${n}`" class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <Skeleton variant="rect" :width="40" :height="40" class="shrink-0 rounded-control" />
              <div class="min-w-0 flex-1 space-y-2">
                <Skeleton variant="text" width="50%" height="12px" />
                <Skeleton variant="text" width="40%" height="22px" />
                <Skeleton variant="text" width="35%" height="11px" />
              </div>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div v-for="n in 2" :key="`chart-${n}`" class="card p-4">
            <Skeleton variant="text" width="30%" height="14px" class="mb-4" />
            <Skeleton variant="rect" width="100%" height="220px" />
          </div>
        </div>
      </div>

      <template v-else-if="stats">
        <!-- Stats Cards — brand / semantic soft icon wells -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <!-- Total API Keys (accent — admin primary) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
                <Icon name="key" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.apiKeys') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  {{ stats.total_api_keys }}
                </p>
                <p class="text-xs text-success">
                  {{ stats.active_api_keys }} {{ t('common.active') }}
                </p>
              </div>
            </div>
          </div>

          <!-- Service Accounts (brand teal) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-brand-50 text-brand-600 dark:bg-brand-950/40 dark:text-brand-300">
                <Icon name="server" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.accounts') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  {{ stats.total_accounts }}
                </p>
                <p class="text-xs">
                  <span class="text-success"
                    >{{ stats.normal_accounts }} {{ t('common.active') }}</span
                  >
                  <span v-if="stats.error_accounts > 0" class="ml-1 text-danger"
                    >{{ stats.error_accounts }} {{ t('common.error') }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- New Users Today (success) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-400">
                <Icon name="userPlus" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.users') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-success">
                  +{{ stats.today_new_users }}
                </p>
                <p class="text-xs text-ink-soft dark:text-dark-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_users) }}
                </p>
              </div>
            </div>
          </div>

          <!-- Performance (RPM/TPM) — brand cyan accent -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-brand-100 text-brand-700 dark:bg-brand-950/40 dark:text-brand-300">
                <Icon name="bolt" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.performance') }}
                </p>
                <div class="flex items-baseline gap-2">
                  <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                    {{ formatTokens(stats.rpm) }}
                  </p>
                  <span class="text-xs text-ink-soft dark:text-dark-400">RPM</span>
                </div>
                <div class="flex items-baseline gap-2">
                  <p class="text-sm font-semibold tabular-nums text-brand-600 dark:text-brand-300">
                    {{ formatTokens(stats.tpm) }}
                  </p>
                  <span class="text-xs text-ink-soft dark:text-dark-400">TPM</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Avg Response Time (warning — latency) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-400">
                <Icon name="clock" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.avgResponse') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  {{ formatDuration(stats.average_duration_ms) }}
                </p>
                <p class="text-xs text-ink-soft dark:text-dark-400">
                  {{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}
                </p>
              </div>
            </div>
          </div>
          <!-- Today Requests (success) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-400">
                <Icon name="chart" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.todayRequests') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  {{ stats.today_requests }}
                </p>
                <p class="text-xs text-ink-soft dark:text-dark-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_requests) }}
                </p>
              </div>
            </div>
          </div>

          <!-- Today Tokens (warning) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-400">
                <Icon name="cube" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.todayTokens') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  {{ formatTokens(stats.today_tokens) }}
                </p>
                <HelpTooltip width-class="w-56">
                  <template #trigger>
                    <div class="mt-2 flex flex-wrap items-center gap-x-1 gap-y-0.5 text-xs font-semibold tabular-nums">
                      <template v-for="(item, index) in todayTokenBreakdownItems" :key="item.key">
                        <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                        <span v-if="index < todayTokenBreakdownItems.length - 1" class="text-ink-faint dark:text-dark-500">/</span>
                      </template>
                    </div>
                  </template>
                  <div class="space-y-1.5">
                    <div v-for="item in todayTokenBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                      <span>{{ item.label }}</span>
                      <span class="font-semibold tabular-nums">${{ formatCost(item.value) }}</span>
                    </div>
                  </div>
                </HelpTooltip>
              </div>
            </div>
          </div>

          <!-- Total Tokens (accent) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
                <Icon name="database" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.totalTokens') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  {{ formatTokens(stats.total_tokens) }}
                </p>
                <HelpTooltip width-class="w-56">
                  <template #trigger>
                    <div class="mt-2 flex flex-wrap items-center gap-x-1 gap-y-0.5 text-xs font-semibold tabular-nums">
                      <template v-for="(item, index) in totalTokenBreakdownItems" :key="item.key">
                        <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                        <span v-if="index < totalTokenBreakdownItems.length - 1" class="text-ink-faint dark:text-dark-500">/</span>
                      </template>
                    </div>
                  </template>
                  <div class="space-y-1.5">
                    <div v-for="item in totalTokenBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                      <span>{{ item.label }}</span>
                      <span class="font-semibold tabular-nums">${{ formatCost(item.value) }}</span>
                    </div>
                  </div>
                </HelpTooltip>
              </div>
            </div>
          </div>

          <!-- Today Consumption (brand) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-brand-50 text-brand-600 dark:bg-brand-950/40 dark:text-brand-300">
                <Icon name="dollar" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.todayCost') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  ${{ formatCost(stats.today_actual_cost) }}
                </p>
                <HelpTooltip width-class="w-56">
                  <template #trigger>
                    <div class="mt-2 flex flex-wrap items-center gap-x-1 gap-y-0.5 text-xs font-semibold tabular-nums">
                      <template v-for="(item, index) in todayFinancialBreakdownItems" :key="item.key">
                        <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                        <span v-if="index < todayFinancialBreakdownItems.length - 1" class="text-ink-faint dark:text-dark-500">/</span>
                      </template>
                    </div>
                  </template>
                  <div class="space-y-1.5">
                    <div v-for="item in todayFinancialBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                      <span>{{ item.label }}</span>
                      <span class="font-semibold tabular-nums">${{ formatCost(item.value) }}</span>
                    </div>
                  </div>
                </HelpTooltip>
              </div>
            </div>
          </div>

          <!-- Total Consumption (accent) -->
          <div class="card min-w-0 p-4">
            <div class="flex items-center gap-3">
              <div class="stat-icon shrink-0 bg-accent-100 text-accent-700 dark:bg-accent-950/40 dark:text-accent-300">
                <Icon name="creditCard" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.totalCost') }}
                </p>
                <p class="text-xl font-bold tabular-nums text-ink dark:text-white">
                  ${{ formatCost(stats.total_actual_cost) }}
                </p>
                <HelpTooltip width-class="w-56">
                  <template #trigger>
                    <div class="mt-2 flex flex-wrap items-center gap-x-1 gap-y-0.5 text-xs font-semibold tabular-nums">
                      <template v-for="(item, index) in totalFinancialBreakdownItems" :key="item.key">
                        <span :class="item.textClass">${{ formatCost(item.value) }}</span>
                        <span v-if="index < totalFinancialBreakdownItems.length - 1" class="text-ink-faint dark:text-dark-500">/</span>
                      </template>
                    </div>
                  </template>
                  <div class="space-y-1.5">
                    <div v-for="item in totalFinancialBreakdownItems" :key="item.key" class="flex items-center justify-between gap-4">
                      <span>{{ item.label }}</span>
                      <span class="font-semibold tabular-nums">${{ formatCost(item.value) }}</span>
                    </div>
                  </div>
                </HelpTooltip>
              </div>
            </div>
          </div>
        </div>

        <!-- Quick Actions -->
        <div class="card p-4">
          <div class="mb-3 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-ink dark:text-white">
              {{ t('admin.dashboard.quickActions') }}
            </h2>
          </div>
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
            <button
              v-if="canUseBatchImage"
              type="button"
              class="group flex items-center gap-3 rounded-lg bg-page p-3 text-left transition-colors hover:bg-accent-50 dark:bg-dark-800/50 dark:hover:bg-accent-950/30"
              @click="router.push('/batch-image')"
            >
              <span class="stat-icon flex h-10 w-10 flex-shrink-0 bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
                <Icon name="sparkles" size="md" :stroke-width="2" />
              </span>
              <span class="min-w-0 flex-1">
                <span class="block text-sm font-medium text-ink dark:text-white">
                  {{ t('admin.dashboard.batchImage') }}
                </span>
                <span class="block text-xs text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.batchImageDesc') }}
                </span>
              </span>
              <Icon name="chevronRight" size="sm" class="text-ink-faint group-hover:text-accent-500" />
            </button>
            <button
              type="button"
              class="group flex items-center gap-3 rounded-lg bg-page p-3 text-left transition-colors hover:bg-brand-50 dark:bg-dark-800/50 dark:hover:bg-brand-950/30"
              @click="router.push('/admin/groups')"
            >
              <span class="stat-icon flex h-10 w-10 flex-shrink-0 bg-brand-50 text-brand-600 dark:bg-brand-950/40 dark:text-brand-300">
                <Icon name="grid" size="md" :stroke-width="2" />
              </span>
              <span class="min-w-0 flex-1">
                <span class="block text-sm font-medium text-ink dark:text-white">
                  {{ t('admin.dashboard.groupPricing') }}
                </span>
                <span class="block text-xs text-ink-soft dark:text-dark-400">
                  {{ t('admin.dashboard.groupPricingDesc') }}
                </span>
              </span>
              <Icon name="chevronRight" size="sm" class="text-ink-faint group-hover:text-brand-600" />
            </button>
          </div>
        </div>

        <!-- Charts Section -->
        <div class="space-y-6">
          <!-- Date Range Filter -->
          <div class="card p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-ink-body dark:text-dark-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-ink-body dark:text-dark-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
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

          <!-- User Usage Trend (Full Width) -->
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-ink dark:text-white">
              {{ t('admin.dashboard.recentUsage') }} (Top 12)
            </h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-ink-soft dark:text-dark-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>
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
import Skeleton from '@/components/common/Skeleton.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { chartColor } from '@/utils/chartPalette'

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

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: chartColor(idx),
    backgroundColor: `${chartColor(idx)}20`,
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

const asNumber = (value: number | null | undefined): number => {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
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

const todayTokenBreakdownItems = computed(() => [
  {
    key: 'actual',
    label: t('admin.dashboard.actual'),
    value: asNumber(stats.value?.today_actual_cost),
    textClass: 'text-success'
  },
  {
    key: 'account',
    label: t('admin.dashboard.accountCost'),
    value: asNumber(stats.value?.today_account_cost),
    textClass: 'text-warning'
  },
  {
    key: 'standard',
    label: t('admin.dashboard.standard'),
    value: asNumber(stats.value?.today_cost),
    textClass: 'text-ink-soft dark:text-dark-400'
  }
])

const totalTokenBreakdownItems = computed(() => [
  {
    key: 'actual',
    label: t('admin.dashboard.actual'),
    value: asNumber(stats.value?.total_actual_cost),
    textClass: 'text-success'
  },
  {
    key: 'account',
    label: t('admin.dashboard.accountCost'),
    value: asNumber(stats.value?.total_account_cost),
    textClass: 'text-warning'
  },
  {
    key: 'standard',
    label: t('admin.dashboard.standard'),
    value: asNumber(stats.value?.total_cost),
    textClass: 'text-ink-soft dark:text-dark-400'
  }
])

const financialBreakdownItems = computed(() => [
  {
    key: 'balance',
    label: t('admin.dashboard.balanceConsumption'),
    todayValue: asNumber(stats.value?.today_balance_actual_cost),
    totalValue: asNumber(stats.value?.total_balance_actual_cost),
    textClass: 'text-success'
  },
  {
    key: 'subscription',
    label: t('admin.dashboard.subscriptionConsumption'),
    todayValue: asNumber(stats.value?.today_subscription_actual_cost),
    totalValue: asNumber(stats.value?.total_subscription_actual_cost),
    textClass: 'text-accent-600 dark:text-accent-300'
  },
  {
    key: 'recharge',
    label: t('admin.dashboard.rechargeAmount'),
    todayValue: asNumber(stats.value?.today_recharge_amount),
    totalValue: asNumber(stats.value?.total_recharge_amount),
    textClass: 'text-brand-600 dark:text-brand-300'
  },
  {
    key: 'refund',
    label: t('admin.dashboard.refundAmount'),
    todayValue: asNumber(stats.value?.today_refund_amount),
    totalValue: asNumber(stats.value?.total_refund_amount),
    textClass: 'text-danger'
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
  startDate.value = range.startDate
  endDate.value = range.endDate

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

  loadChartData()
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
    loadDashboardSnapshot(true),
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
</style>
