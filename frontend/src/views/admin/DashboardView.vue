<template>
  <AppLayout>
    <div class="dash-page">
      <div v-if="loading" class="dash-loading">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- ============================= 1 · Hero ============================= -->
        <section class="glass-card dash-hero">
          <span class="dash-hero-deco" aria-hidden="true">
            <span class="dash-hero-dots"></span>
            <span class="dash-hero-orb dash-hero-orb-accent"></span>
            <span class="dash-hero-orb dash-hero-orb-success"></span>
          </span>
          <span class="dash-hero-ring" aria-hidden="true"></span>

          <div class="dash-hero-copy">
            <span class="dash-hero-kicker">{{ heroKicker }}</span>
            <h1 class="dash-hero-title">
              {{ heroTitleLead }}<span class="dash-hero-accent">{{ formatNumber(stats.normal_accounts) }}</span>{{ t('admin.dashboard.heroTitleTail') }}
            </h1>
            <p class="dash-hero-desc">{{ heroDescription }}</p>
            <div class="dash-hero-actions">
              <Button @click="router.push('/admin/ops')">{{ t('admin.dashboard.viewOpsMonitor') }}</Button>
              <Button variant="secondary" @click="router.push('/admin/accounts')">
                {{ t('admin.dashboard.handleAbnormalAccounts') }}
                <span v-if="abnormalAccounts > 0" class="dash-hero-badge">{{ abnormalAccounts }}</span>
              </Button>
            </div>
          </div>

          <!-- Mobile (<=767px) hero variant: big today-requests figure + 48px bars -->
          <div class="dash-hero-mobile">
            <div class="dash-hero-mobile-top">
              <span class="dash-mini-label">{{ t('admin.dashboard.todayRequests') }}</span>
              <span class="dash-hero-chip" :class="`dash-hero-chip-${serviceTone}`">
                <span class="dash-pulse" :class="`dash-pulse-${serviceTone}`"></span>{{ serviceStatusLabel }}
              </span>
            </div>
            <div class="dash-hero-mobile-value">
              <span class="dash-hero-figure">{{ formatNumber(stats.today_requests) }}</span>
              <span v-if="requestsDelta" class="dash-hero-figure-delta" :class="`dash-delta-${requestsDelta.tone}`">
                {{ requestsDelta.text }}
              </span>
            </div>
            <div class="dash-hero-mobile-bars">
              <span
                v-for="bar in trendBars"
                :key="`m-${bar.key}`"
                :style="{ height: bar.height, background: bar.fill }"
              ></span>
            </div>
            <div class="dash-hero-mobile-axis">
              <span>{{ trendAxisLabels[0] }}</span>
              <span>{{ trendAxisLabels[1] }}</span>
              <span>{{ trendAxisLabels[2] }}</span>
            </div>
          </div>

          <div class="dash-hero-side">
            <div class="dash-hero-tools">
              <DateRangePicker
                class="dash-daterange"
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
              <div class="dash-granularity">
                <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
              </div>
              <Button
                variant="secondary"
                class="dash-refresh"
                :loading="chartsLoading"
                @click="loadDashboardStats"
              >
                {{ t('common.refresh') }}
              </Button>
            </div>

            <div class="dash-hero-stats">
              <div class="dash-hero-mini glass-inset">
                <span class="dash-mini-label">{{ t('admin.dashboard.serviceStatus') }}</span>
                <span class="dash-mini-status">
                  <span class="dash-pulse" :class="`dash-pulse-${serviceTone}`"></span>{{ serviceStatusLabel }}
                </span>
                <span class="dash-mini-sub">
                  {{ formatNumber(stats.normal_accounts) }} / {{ formatNumber(stats.total_accounts) }}
                  {{ t('admin.dashboard.accounts') }}
                </span>
              </div>
              <div class="dash-hero-mini glass-inset">
                <span class="dash-mini-label">{{ t('admin.dashboard.realtimeRpm') }}</span>
                <span class="dash-mini-value">{{ formatNumber(stats.rpm) }}</span>
                <span v-if="requestsDelta" class="dash-mini-sub" :class="`dash-delta-${requestsDelta.tone}`">
                  {{ requestsDelta.text }} {{ t('admin.dashboard.vsPrevPeriod') }}
                </span>
                <span v-else class="dash-mini-sub">TPM {{ formatTokens(stats.tpm) }}</span>
              </div>
              <div class="dash-hero-mini glass-inset">
                <span class="dash-mini-label">{{ t('admin.dashboard.realtimeTpm') }}</span>
                <span class="dash-mini-value">{{ formatTokens(stats.tpm) }}</span>
                <span class="dash-mini-sub">
                  {{ t('admin.dashboard.avgResponse') }} {{ formatDuration(stats.average_duration_ms) }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <!-- ============================= 2 · Stat cards ============================= -->
        <div class="dash-stat-grid">
          <StatCard
            :label="t('admin.dashboard.totalUsers')"
            :value="formatNumber(stats.total_users)"
            :sub="`${t('admin.dashboard.activeUsers')} ${formatNumber(stats.active_users)}`"
            :delta="t('admin.dashboard.todayNew', { count: formatNumber(stats.today_new_users) })"
            :delta-tone="stats.today_new_users > 0 ? 'up' : 'neutral'"
          >
            <template #sparkline><DashSparkline :path="flatSpark" /></template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.totalApiKeys')"
            :value="formatNumber(stats.total_api_keys)"
            :sub="`${t('admin.dashboard.activeApiKeys')} ${formatNumber(stats.active_api_keys)}`"
            :delta="t('admin.dashboard.activeRatio', { rate: activeKeyRatio })"
            delta-tone="neutral"
          >
            <template #sparkline><DashSparkline :path="flatSpark" /></template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.totalAccounts')"
            :value="formatNumber(stats.total_accounts)"
            :sub="accountsSubLabel"
            :delta="accountsDelta.text"
            :delta-tone="accountsDelta.tone"
          >
            <template #sparkline><DashSparkline :path="flatSpark" /></template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.todayRequests')"
            :value="formatNumber(stats.today_requests)"
            :sub="`${t('admin.dashboard.totalRequests')} ${formatNumber(stats.total_requests)}`"
            :delta="requestsDelta?.text"
            :delta-tone="requestsDelta?.tone ?? 'neutral'"
          >
            <template #sparkline><DashSparkline :path="requestsSpark" /></template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.todayTokens')"
            :value="formatTokens(stats.today_tokens)"
            :sub="t('admin.dashboard.cacheHitRate', { rate: cacheHitRate })"
            :delta="tokensDelta?.text"
            :delta-tone="tokensDelta?.tone ?? 'neutral'"
          >
            <template #sparkline>
              <DashBreakdownSpark :path="tokensSpark" :items="todayTokenBreakdownItems" :format="formatCost" />
            </template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.totalTokens')"
            :value="formatTokens(stats.total_tokens)"
            :sub="`${t('admin.dashboard.input')} ${formatTokens(stats.total_input_tokens)}`"
            :delta="t('common.total')"
            delta-tone="neutral"
          >
            <template #sparkline>
              <DashBreakdownSpark :path="tokensSpark" :items="totalTokenBreakdownItems" :format="formatCost" />
            </template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.todayCost')"
            :value="`$${formatCost(stats.today_actual_cost)}`"
            :sub="`${t('admin.dashboard.standard')} $${formatCost(stats.today_cost)}`"
            :delta="costDelta?.text"
            :delta-tone="costDelta?.tone ?? 'neutral'"
          >
            <template #sparkline>
              <DashBreakdownSpark :path="costSpark" :items="todayFinancialBreakdownItems" :format="formatCost" />
            </template>
          </StatCard>

          <StatCard
            :label="t('admin.dashboard.totalCost')"
            :value="`$${formatCost(stats.total_actual_cost)}`"
            :sub="`${t('admin.dashboard.standard')} $${formatCost(stats.total_cost)}`"
            :delta="t('common.total')"
            delta-tone="neutral"
          >
            <template #sparkline>
              <DashBreakdownSpark :path="costSpark" :items="totalFinancialBreakdownItems" :format="formatCost" />
            </template>
          </StatCard>
        </div>

        <!-- ============================= 3 · Trend + model split ============================= -->
        <div class="dash-row dash-row-trend">
          <section class="glass-card dash-panel">
            <header class="dash-panel-head">
              <div class="dash-panel-heading">
                <span class="dash-panel-title">{{ t('admin.dashboard.requestTrend') }}</span>
                <span class="dash-panel-sub">{{ trendSubtitle }}</span>
              </div>
              <div class="segmented segmented-sm dash-metric-switch" role="radiogroup">
                <button
                  v-for="option in trendMetricOptions"
                  :key="option.value"
                  type="button"
                  role="radio"
                  class="segmented-item"
                  :class="{ 'segmented-item-active': option.value === trendMetric }"
                  :aria-checked="option.value === trendMetric"
                  @click="trendMetric = option.value"
                >
                  {{ option.label }}
                </button>
              </div>
            </header>
            <div v-if="trendBars.length" class="dash-bars">
              <div v-for="bar in trendBars" :key="bar.key" class="dash-bar-col">
                <div class="dash-bar" :title="bar.title" :style="{ height: bar.height, background: bar.fill }"></div>
                <span class="dash-bar-label">{{ bar.label }}</span>
              </div>
            </div>
            <div v-else class="empty-state dash-empty">
              <span class="empty-state-title">{{ t('admin.dashboard.noDataAvailable') }}</span>
            </div>
          </section>

          <section class="glass-card dash-panel">
            <header class="dash-panel-head">
              <span class="dash-panel-title">
                {{ distView === 'models' ? t('admin.dashboard.modelDistribution') : t('admin.dashboard.spendingRankingTitle') }}
              </span>
              <div class="segmented segmented-sm dash-metric-switch" role="radiogroup">
                <button
                  type="button"
                  role="radio"
                  class="segmented-item"
                  :class="{ 'segmented-item-active': distView === 'models' }"
                  :aria-checked="distView === 'models'"
                  @click="distView = 'models'"
                >
                  {{ t('admin.dashboard.viewModelDistribution') }}
                </button>
                <button
                  type="button"
                  role="radio"
                  class="segmented-item"
                  :class="{ 'segmented-item-active': distView === 'users' }"
                  :aria-checked="distView === 'users'"
                  @click="distView = 'users'"
                >
                  {{ t('admin.dashboard.viewSpendingRanking') }}
                </button>
              </div>
            </header>

            <div v-if="distRowsLoading" class="dash-panel-loading"><LoadingSpinner size="md" /></div>
            <div v-else-if="distRows.length" class="dash-dist">
              <component
                :is="row.userId ? 'button' : 'div'"
                v-for="(row, index) in distRows"
                :key="row.key"
                :type="row.userId ? 'button' : undefined"
                class="dash-dist-row"
                :class="{ 'dash-dist-row-action': !!row.userId }"
                @click="row.userId ? goToUserUsageById(row.userId) : undefined"
              >
                <span class="dash-dist-tile" :style="{ background: row.tile }">
                  <PlatformIcon :platform="row.platform" size="xs" />
                </span>
                <span class="dash-dist-body">
                  <span class="dash-dist-line">
                    <span class="dash-dist-name">{{ row.name }}</span>
                    <span class="dash-dist-meta">${{ row.cost }} · {{ row.pct }}%</span>
                  </span>
                  <span class="dash-dist-track">
                    <span
                      class="dash-dist-fill"
                      :style="{ width: `${row.pct}%`, opacity: Math.max(0.24, 1 - index * 0.16) }"
                    ></span>
                  </span>
                </span>
              </component>
            </div>
            <div v-else class="empty-state dash-empty">
              <span class="empty-state-title">{{ t('admin.dashboard.noDataAvailable') }}</span>
            </div>
          </section>
        </div>

        <!-- ============================= 4 · Health + events ============================= -->
        <div class="dash-row dash-row-split">
          <section class="glass-card dash-panel">
            <header class="dash-panel-head">
              <span class="dash-panel-title">{{ t('admin.dashboard.platformHealth') }}</span>
              <button type="button" class="dash-panel-link" @click="router.push('/admin/accounts')">
                {{ t('admin.dashboard.manageAccounts') }} →
              </button>
            </header>
            <div v-if="platformHealth.length" class="dash-health">
              <div v-for="row in platformHealth" :key="row.platform" class="dash-health-row">
                <span class="dash-health-name">
                  <span class="dash-dist-tile" :style="{ background: platformTileBackground(row.platform) }">
                    <PlatformIcon :platform="row.platform" size="xs" />
                  </span>
                  <span class="dash-health-label">{{ platformLabel(row.platform) }}</span>
                </span>
                <span class="dash-health-bar">
                  <span class="dash-health-ok" :style="{ width: `${row.okPct}%` }"></span>
                  <span class="dash-health-warn" :style="{ width: `${row.limitPct}%` }"></span>
                  <span class="dash-health-err" :style="{ width: `${row.errorPct}%` }"></span>
                </span>
                <span class="dash-health-text">{{ row.text }}</span>
              </div>
            </div>
            <div v-else class="empty-state dash-empty">
              <span class="empty-state-title">{{ t('admin.dashboard.noPlatformHealth') }}</span>
              <span class="empty-state-description">{{ t('admin.dashboard.noPlatformHealthDesc') }}</span>
            </div>
          </section>

          <section class="glass-card dash-panel">
            <header class="dash-panel-head">
              <span class="dash-panel-title">{{ t('admin.dashboard.recentEvents') }}</span>
              <button type="button" class="dash-panel-link" @click="router.push('/admin/ops')">
                {{ t('nav.ops') }} →
              </button>
            </header>
            <div v-if="recentEvents.length" class="dash-events">
              <div v-for="event in recentEvents" :key="event.id" class="dash-event">
                <span class="dash-event-time">{{ event.time }}</span>
                <span class="dash-event-dot" :class="`dash-event-dot-${event.tone}`"></span>
                <span class="dash-event-msg" :title="event.message">{{ event.message }}</span>
              </div>
            </div>
            <div v-else class="empty-state dash-empty">
              <span class="empty-state-title">{{ t('admin.dashboard.noEvents') }}</span>
              <span class="empty-state-description">{{ t('admin.dashboard.noEventsDesc') }}</span>
            </div>
          </section>
        </div>

        <!-- ============================= 5 · User trend + quick actions ============================= -->
        <div class="dash-row dash-row-trend">
          <section class="glass-card dash-panel">
            <header class="dash-panel-head">
              <div class="dash-panel-heading">
                <span class="dash-panel-title">{{ t('admin.dashboard.userUsageTrend') }}</span>
                <span class="dash-panel-sub">{{ t('admin.dashboard.recentUsage') }}</span>
              </div>
            </header>
            <div class="dash-user-trend">
              <div v-if="userTrendLoading" class="dash-panel-loading"><LoadingSpinner size="md" /></div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div v-else class="empty-state dash-empty">
                <span class="empty-state-title">{{ t('admin.dashboard.noDataAvailable') }}</span>
              </div>
            </div>
          </section>

          <section class="glass-card dash-panel">
            <header class="dash-panel-head">
              <span class="dash-panel-title">{{ t('admin.dashboard.quickActions') }}</span>
            </header>
            <div class="dash-actions">
              <button
                v-if="canUseBatchImage"
                type="button"
                class="dash-action"
                @click="router.push('/batch-image')"
              >
                <span class="dash-action-icon"><Icon name="sparkles" size="md" :stroke-width="2" /></span>
                <span class="dash-action-body">
                  <span class="dash-action-title">{{ t('admin.dashboard.batchImage') }}</span>
                  <span class="dash-action-desc">{{ t('admin.dashboard.batchImageDesc') }}</span>
                </span>
                <Icon name="chevronRight" size="sm" class="text-muted" />
              </button>
              <button type="button" class="dash-action" @click="router.push('/admin/groups')">
                <span class="dash-action-icon"><Icon name="grid" size="md" :stroke-width="2" /></span>
                <span class="dash-action-body">
                  <span class="dash-action-title">{{ t('admin.dashboard.groupPricing') }}</span>
                  <span class="dash-action-desc">{{ t('admin.dashboard.groupPricingDesc') }}</span>
                </span>
                <Icon name="chevronRight" size="sm" class="text-muted" />
              </button>
            </div>
          </section>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  GroupPlatform,
  ModelStat,
  TrendDataPoint,
  UserSpendingRankingItem,
  UserUsageTrendPoint
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import DashSparkline from '@/components/admin/dashboard/DashSparkline.vue'
import DashBreakdownSpark from '@/components/admin/dashboard/DashBreakdownSpark.vue'
import type { SparkPath } from '@/components/admin/dashboard/types'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import StatCard from '@/components/ui/StatCard.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import { platformFromModel, platformLabel, platformTileBackground } from '@/utils/platformTile'
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

// ---------------------------------------------------------------- sparkline
const SPARK_W = 100
const SPARK_H = 28

/** Build the prototype's 100×28 area + line sparkline geometry for a series. */
const buildSpark = (values: number[]): SparkPath => {
  const points = values.length >= 2 ? values : [0, 0]
  const min = Math.min(...points)
  const max = Math.max(...points)
  const span = max - min
  const coords = points.map((value, index) => {
    const ratio = span > 0 ? (value - min) / span : 0.5
    const norm = 0.18 + 0.82 * ratio
    return [(index * SPARK_W) / (points.length - 1), SPARK_H - norm * (SPARK_H - 4)] as const
  })
  const line = `M${coords.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' L')}`
  return { line, area: `${line} L${SPARK_W},${SPARK_H} L0,${SPARK_H} Z` }
}

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
const trendMetric = ref<'requests' | 'tokens' | 'cost'>('requests')
const distView = ref<'models' | 'users'>('models')
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

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

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

const platformHealthRaw = ref<
  { platform: string; total_accounts: number; available_count: number; rate_limit_count: number; error_count: number }[]
>([])

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

interface EventRow {
  id: number
  time: string
  tone: 'danger' | 'warning' | 'accent' | 'success'
  message: string
}

const recentEvents = ref<EventRow[]>([])

const severityTone = (severity: string, statusCode: number): EventRow['tone'] => {
  const value = (severity || '').toLowerCase()
  if (value === 'critical' || value === 'fatal' || value === 'error') return 'danger'
  if (value === 'warning' || value === 'warn' || statusCode === 429) return 'warning'
  if (value === 'info') return 'accent'
  return statusCode >= 500 ? 'danger' : 'warning'
}

const formatEventTime = (value: string): string => {
  if (!value) return '--:--'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '--:--'
  return `${String(parsed.getHours()).padStart(2, '0')}:${String(parsed.getMinutes()).padStart(2, '0')}`
}

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

/** Per-platform account availability powers the “platform health” panel. */
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

/** Latest unresolved ops errors power the “recent events” panel. */
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

/* ================================= hero ================================= */
.dash-hero {
  position: relative;
  border-radius: var(--radius-hero);
  padding: 26px 30px;
  display: grid;
  grid-template-columns: 1.25fr 1fr;
  gap: 24px;
  flex: none;
}

.dash-hero-deco {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  overflow: hidden;
  pointer-events: none;
}

.dash-hero-dots {
  position: absolute;
  inset: 0;
  left: 45%;
  background: radial-gradient(color-mix(in oklch, var(--foreground) 7%, transparent) 1px, transparent 1.3px) 0 0 /
    16px 16px;
  mask-image: linear-gradient(90deg, transparent, black 45%);
  -webkit-mask-image: linear-gradient(90deg, transparent, black 45%);
}

.dash-hero-orb {
  position: absolute;
  border-radius: 50%;
}

.dash-hero-orb-accent {
  right: -90px;
  top: -140px;
  width: 380px;
  height: 380px;
  background: radial-gradient(
    circle at 35% 35%,
    color-mix(in oklch, var(--accent) 60%, white) 0%,
    color-mix(in oklch, var(--accent) 30%, transparent) 42%,
    transparent 70%
  );
}

.dash-hero-orb-success {
  right: 220px;
  bottom: -160px;
  width: 260px;
  height: 260px;
  background: radial-gradient(circle at 50% 50%, color-mix(in oklch, var(--success) 30%, transparent) 0%, transparent 65%);
}

/* gradient hairline ring — mobile hero variant only */
.dash-hero-ring {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: inherit;
  padding: 1px;
  background: linear-gradient(
    135deg,
    color-mix(in oklch, var(--accent) 55%, transparent),
    transparent 35%,
    transparent 65%,
    color-mix(in oklch, var(--success) 45%, transparent)
  );
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  pointer-events: none;
}

.dash-hero-copy {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 10px;
}

.dash-hero-kicker {
  font-size: 12.5px;
  color: var(--muted);
  line-height: 1.3;
  margin-bottom: 3px;
}

.dash-hero-title {
  margin: 0;
  font-family: var(--display);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
}

.dash-hero-accent {
  color: var(--accent);
}

.dash-hero-desc {
  margin: 0;
  font-size: 13.5px;
  color: var(--muted);
  max-width: 540px;
  line-height: 1.55;
}

.dash-hero-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

.dash-hero-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--danger);
  color: #fff;
  font-size: 10.5px;
  line-height: 1.3;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.dash-hero-side {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
  min-width: 0;
}

.dash-hero-tools {
  position: absolute;
  top: 0;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.dash-daterange :deep(.date-picker-trigger) {
  height: 28px;
  padding: 0 10px;
  gap: 6px;
  font-size: 12px;
}

.dash-granularity {
  width: 88px;
}

.dash-granularity :deep(.select-trigger) {
  height: 28px;
  padding: 0 8px 0 10px;
  gap: 6px;
  font-size: 12px;
}

.dash-hero-tools .dash-refresh {
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
}

.dash-hero-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.dash-hero-mini {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  min-width: 0;
  min-height: 111px;
}

.dash-mini-label {
  font-size: 11.5px;
  color: var(--muted);
  font-weight: 600;
  line-height: 1.3;
}

.dash-mini-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

.dash-mini-value {
  font-family: var(--display);
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.dash-mini-sub {
  font-size: 11.5px;
  color: var(--muted);
  line-height: 1.3;
}

.dash-delta-up {
  color: var(--success-text);
  font-weight: 600;
}

.dash-delta-down {
  color: var(--danger-text);
  font-weight: 600;
}

.dash-pulse {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  flex: none;
  animation: dash-pulse 2s infinite;
}

.dash-pulse-success {
  background: var(--success);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--success) 25%, transparent);
}

.dash-pulse-warning {
  background: var(--warning);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--warning) 25%, transparent);
}

.dash-pulse-danger {
  background: var(--danger);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--danger) 25%, transparent);
}

@keyframes dash-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

/* --------------------------- mobile hero variant --------------------------- */
.dash-hero-mobile {
  display: none;
  position: relative;
  flex-direction: column;
  gap: 12px;
}

.dash-hero-mobile-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.dash-hero-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11.5px;
  line-height: 1.3;
  font-weight: 600;
  box-shadow: inset 0 0 0 1px color-mix(in oklch, currentColor 22%, transparent);
}

.dash-hero-chip-success {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.dash-hero-chip-warning {
  background: color-mix(in oklch, var(--warning) 18%, transparent);
  color: var(--warning-text);
}

.dash-hero-chip-danger {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
  color: var(--danger-text);
}

.dash-hero-mobile-value {
  display: flex;
  align-items: flex-end;
  gap: 10px;
}

.dash-hero-figure {
  font-family: var(--display);
  font-size: 40px;
  font-weight: 800;
  letter-spacing: -0.04em;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.dash-hero-figure-delta {
  font-size: 12px;
  line-height: 1.3;
  font-weight: 600;
  padding: 3px 7px;
  border-radius: 6px;
  margin-bottom: 4px;
  background: var(--surface-secondary);
}

.dash-hero-figure-delta.dash-delta-up {
  background: color-mix(in oklch, var(--success) 16%, transparent);
}

.dash-hero-figure-delta.dash-delta-down {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
}

.dash-hero-mobile-bars {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 48px;
}

.dash-hero-mobile-bars span {
  flex: 1;
  border-radius: 3px 3px 1px 1px;
}

.dash-hero-mobile-axis {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  line-height: 1.3;
  color: var(--muted);
  font-family: var(--font-mono);
}

/* ============================== stat cards ============================== */
.dash-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.dash-stat-grid :deep(.ui-stat-card-sparkline > div) {
  margin-left: 0;
  width: 100%;
  height: 100%;
}

/* ================================ panels ================================ */
.dash-row {
  display: grid;
  gap: 12px;
}

.dash-row-trend {
  grid-template-columns: 2fr 1fr;
}

.dash-row-split {
  grid-template-columns: 1fr 1fr;
}

.dash-panel {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.dash-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 28px;
}

.dash-panel-heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.dash-panel-title {
  font-size: 14px;
  line-height: 1.3;
  font-weight: 600;
  color: var(--foreground);
}

.dash-panel-sub {
  font-size: 12px;
  line-height: 1.3;
  color: var(--muted);
}

.dash-panel-link {
  font-size: 12.5px;
  line-height: 1.3;
  font-weight: 600;
  color: var(--accent);
  text-decoration: none;
  white-space: nowrap;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  font-family: inherit;
}

.dash-panel-link:hover {
  text-decoration: underline;
}

.dash-panel-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
}

.dash-empty {
  flex: 1;
  padding: 24px 16px;
}

.dash-row-split .dash-empty {
  min-height: 151px;
}

.dash-metric-switch {
  flex: none;
}

/* ------------------------------- bar chart ------------------------------- */
.dash-bars {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 150px;
  padding-top: 6px;
  margin-top: 2px;
}

.dash-bar-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  height: 100%;
  justify-content: flex-end;
}

.dash-bar {
  width: 100%;
  border-radius: 6px 6px 3px 3px;
}

.dash-bar-label {
  font-size: 10.5px;
  line-height: 1.3;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* ---------------------------- model distribution ---------------------------- */
.dash-dist {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.dash-dist-row {
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: transparent;
  padding: 0;
  width: 100%;
  text-align: left;
  color: inherit;
  font: inherit;
}

.dash-dist-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.dash-dist-row-action {
  cursor: pointer;
}

.dash-dist-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 12.5px;
  line-height: 1.3;
}

.dash-dist-tile {
  display: none;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 7px;
  color: #fff;
  flex: none;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.14);
}

.dash-dist-name {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.dash-dist-meta {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  flex: none;
}

.dash-dist-track {
  display: block;
  height: 6px;
  border-radius: 999px;
  background: var(--surface-tertiary);
  overflow: hidden;
}

.dash-dist-fill {
  display: block;
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, color-mix(in oklch, var(--accent) 70%, white), var(--accent));
}

/* ------------------------------ platform health ------------------------------ */
.dash-health {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 151px;
  overflow-y: auto;
}

.dash-health-row {
  display: grid;
  grid-template-columns: 120px 1fr 150px;
  align-items: center;
  gap: 14px;
  font-size: 12.5px;
  line-height: 1.3;
}

.dash-health-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.dash-health-name .dash-dist-tile {
  display: inline-flex;
  width: 22px;
  height: 22px;
  border-radius: 6px;
}

.dash-health-label {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dash-health-bar {
  display: flex;
  height: 8px;
  border-radius: 999px;
  overflow: hidden;
  background: var(--surface-tertiary);
  gap: 2px;
}

.dash-health-ok { background: var(--success); }
.dash-health-warn { background: var(--warning); }
.dash-health-err { background: var(--danger); }

.dash-health-text {
  color: var(--muted);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

/* -------------------------------- events -------------------------------- */
.dash-events {
  display: flex;
  flex-direction: column;
  max-height: 151px;
  overflow-y: auto;
}

.dash-event {
  display: grid;
  grid-template-columns: 44px 8px 1fr;
  gap: 10px;
  align-items: center;
  padding: 8px 0;
  border-top: 1px solid var(--border);
  font-size: 12.5px;
  line-height: 1.3;
}

.dash-event-time {
  font-family: var(--font-mono);
  font-size: 11.5px;
  line-height: 1.3;
  color: var(--muted);
}

.dash-event-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
}

.dash-event-dot-danger { background: var(--danger); }
.dash-event-dot-warning { background: var(--warning); }
.dash-event-dot-accent { background: var(--accent); }
.dash-event-dot-success { background: var(--success); }

.dash-event-msg {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---------------------------- user trend / actions ---------------------------- */
.dash-user-trend {
  height: 256px;
  min-height: 0;
}

.dash-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dash-action {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 12px;
  background: color-mix(in oklch, var(--surface-secondary) 70%, transparent);
  border: 1px solid color-mix(in oklch, var(--border) 60%, transparent);
  cursor: pointer;
  text-align: left;
}

.dash-action:hover {
  background: var(--surface-secondary);
}

.dash-action-icon {
  width: 40px;
  height: 40px;
  flex: none;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
}

.dash-action-body {
  min-width: 0;
  flex: 1;
}

.dash-action-title {
  display: block;
  font-size: 14px;
  line-height: 1.3;
  font-weight: 600;
  color: var(--foreground);
}

.dash-action-desc {
  display: block;
  font-size: 12px;
  line-height: 1.3;
  color: var(--muted);
}

/* ============================== responsive ============================== */
@media (max-width: 1180px) {
  .dash-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .dash-row-trend,
  .dash-row-split {
    grid-template-columns: 1fr;
  }
  .dash-hero {
    grid-template-columns: 1fr;
  }
  .dash-hero-tools {
    position: static;
    justify-content: flex-start;
  }
  .dash-hero-mini {
    min-height: 0;
  }
}

@media (max-width: 767px) {
  .dash-page {
    gap: 14px;
  }
  .dash-hero {
    padding: 18px 18px 16px;
    border-radius: 18px;
    gap: 12px;
  }
  .dash-hero-copy,
  .dash-hero-stats {
    display: none;
  }
  .dash-hero-mobile {
    display: flex;
  }
  .dash-hero-ring {
    display: block;
  }
  .dash-hero-dots {
    display: none;
  }
  .dash-hero-orb-accent {
    right: -70px;
    top: -90px;
    width: 240px;
    height: 240px;
  }
  .dash-hero-orb-success {
    display: none;
  }
  .dash-hero-tools {
    flex-wrap: wrap;
    justify-content: flex-start;
  }
  .dash-stat-grid {
    gap: 10px;
  }
  .dash-stat-grid :deep(.ui-stat-card-value) {
    font-size: 24px;
  }
  .dash-stat-grid :deep(.ui-stat-card-label) {
    font-size: 11.5px;
  }
  .dash-stat-grid :deep(.ui-stat-card-delta) {
    font-size: 10.5px;
    padding: 2px 5px;
    border-radius: 5px;
  }
  .dash-stat-grid :deep(.ui-stat-card-sub) {
    display: none;
  }
  .dash-stat-grid :deep(.ui-stat-card-sparkline) {
    width: 100%;
    height: 24px;
  }
  .dash-panel {
    padding: 0;
    gap: 0;
  }
  .dash-panel-head {
    padding: 12px 14px 10px;
    border-bottom: 1px solid var(--border);
  }
  .dash-panel-title {
    font-size: 13px;
  }
  .dash-dist,
  .dash-health,
  .dash-events,
  .dash-actions,
  .dash-bars,
  .dash-empty,
  .dash-user-trend,
  .dash-panel-loading {
    margin: 0;
  }
  .dash-bars {
    padding: 12px 14px 14px;
    height: 160px;
  }
  .dash-bar-label {
    font-size: 9.5px;
  }
  .dash-dist {
    gap: 0;
  }
  .dash-dist-row {
    flex-direction: row;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
  }
  .dash-dist-tile {
    display: inline-flex;
  }
  .dash-health {
    padding: 12px 14px;
  }
  .dash-health-row {
    grid-template-columns: 110px 1fr;
    grid-template-areas: 'name bar' 'text text';
    row-gap: 6px;
  }
  .dash-health-name { grid-area: name; }
  .dash-health-bar { grid-area: bar; }
  .dash-health-text {
    grid-area: text;
    text-align: left;
  }
  .dash-events {
    padding: 0 14px 8px;
  }
  .dash-actions {
    padding: 12px 14px;
  }
  .dash-user-trend {
    padding: 12px 14px;
    height: 240px;
  }
  .dash-empty {
    margin: 14px;
  }
}
</style>
