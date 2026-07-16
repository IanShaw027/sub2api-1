<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.oauthCapacity.title')"
    width="full"
    @close="emit('close')"
  >
    <div class="space-y-6">
      <!-- Toolbar -->
      <div
        class="flex flex-wrap items-end justify-between gap-3 rounded-xl border border-gray-100 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex min-w-0 flex-1 flex-wrap items-end gap-3">
          <div class="flex flex-col gap-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.oauthCapacity.range') }}
            </span>
            <div class="inline-flex rounded-lg border border-gray-200 p-0.5 dark:border-dark-600">
              <button
                v-for="option in rangeOptions"
                :key="option.value"
                type="button"
                class="rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors"
                :class="
                  selectedRange === option.value
                    ? 'bg-primary-600 text-white shadow-sm'
                    : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
                "
                @click="selectedRange = option.value"
              >
                {{ option.label }}
              </button>
            </div>
          </div>

          <div class="flex flex-col gap-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.oauthCapacity.windowToggle') }}
            </span>
            <div class="inline-flex rounded-lg border border-gray-200 p-0.5 dark:border-dark-600">
              <button
                v-for="option in windowOptions"
                :key="option.value"
                type="button"
                class="rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors"
                :class="
                  selectedWindow === option.value
                    ? 'bg-emerald-600 text-white shadow-sm'
                    : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
                "
                @click="selectedWindow = option.value"
              >
                {{ option.label }}
              </button>
            </div>
          </div>

          <div class="flex min-w-[14rem] flex-col gap-1">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.oauthCapacity.scope') }}
            </span>
            <Select
              v-model="selectedScopeKey"
              :options="scopeSelectOptions"
              :searchable="scopeSelectOptions.length > 5 ? 'auto' : false"
            />
          </div>
        </div>

        <div class="flex items-center gap-2">
          <span v-if="series" class="text-xs text-gray-500 dark:text-gray-400">
            {{ formatTime(series.generated_at) }}
          </span>
          <button
            type="button"
            class="btn btn-secondary px-2.5"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="load(true)"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <div v-if="loading && !series" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <div
        v-else-if="error"
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-300"
      >
        {{ error }}
      </div>

      <template v-else-if="series">
        <!-- Health cards -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div
            v-for="metric in accountMetrics"
            :key="metric.label"
            class="card min-w-0 p-4"
            :class="metric.cardClass"
          >
            <div class="mb-2 flex items-center justify-between">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</span>
              <div class="rounded-lg p-1.5" :class="metric.iconWrapClass">
                <Icon :name="metric.icon" size="sm" :class="metric.iconClass" />
              </div>
            </div>
            <p class="text-2xl font-bold" :class="metric.valueClass">{{ metric.value }}</p>
          </div>
        </div>

        <!-- Rate-limit buckets -->
        <div class="card p-4">
          <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.accounts.oauthCapacity.rateLimitDistribution') }}
          </h4>
          <div class="grid grid-cols-4 gap-2 sm:grid-cols-8">
            <div
              v-for="bucket in rateLimitRows"
              :key="bucket.label"
              class="rounded-lg border border-gray-100 bg-gray-50 px-2 py-2.5 text-center dark:border-dark-700 dark:bg-dark-900/40"
            >
              <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ bucket.label }}</div>
              <div
                class="mt-1 text-base font-semibold"
                :class="bucket.value ? 'text-amber-700 dark:text-amber-300' : 'text-gray-400 dark:text-gray-500'"
              >
                {{ bucket.value }}
              </div>
            </div>
          </div>
        </div>

        <!-- KPI cards -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <div
            v-for="kpi in kpiCards"
            :key="kpi.label"
            class="card min-w-0 border p-4"
            :class="kpi.cardClass"
          >
            <div class="mb-2 flex items-center justify-between gap-2">
              <span class="truncate text-xs font-medium text-gray-500 dark:text-gray-400">{{ kpi.label }}</span>
              <div class="rounded-lg p-1.5" :class="kpi.iconWrapClass">
                <Icon :name="kpi.icon" size="sm" :class="kpi.iconClass" />
              </div>
            </div>
            <p class="font-mono text-xl font-bold" :class="kpi.valueClass">{{ kpi.value }}</p>
            <p v-if="kpi.sub" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ kpi.sub }}</p>
          </div>
        </div>

        <!-- Trend chart -->
        <div class="card p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accounts.oauthCapacity.hourlyTrend') }}
            </h4>
            <div class="flex flex-wrap items-center gap-3 text-[11px] text-gray-500 dark:text-gray-400">
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 rounded bg-blue-500" />
                {{ t('admin.accounts.oauthCapacity.legendSpendSolid') }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 border-t-2 border-dashed border-blue-500" />
                {{ t('admin.accounts.oauthCapacity.legendSpendDashed') }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 rounded bg-emerald-500" />
                {{ t('admin.accounts.oauthCapacity.legendAvailable') }}
              </span>
              <span v-if="selectedWindow === 'both'" class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 rounded bg-cyan-500" />
                {{ t('admin.accounts.oauthCapacity.legendAvailable7d') }}
              </span>
            </div>
          </div>

		  <div class="relative min-h-48">
		    <div
		      v-if="loading && series"
		      class="absolute inset-0 z-10 flex items-center justify-center bg-white/60 dark:bg-dark-800/60"
		    >
		      <LoadingSpinner />
		    </div>
		    <div v-if="chartData" class="h-72 w-full">
		      <Line :data="chartData" :options="chartOptions" />
		    </div>
		    <div
		      v-else
		      class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
		    >
		      {{ t('admin.accounts.oauthCapacity.noTrendData') }}
		    </div>
		  </div>
          <p class="mt-3 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.oauthCapacity.persistenceNote') }}
          </p>
        </div>

        <!-- Plans + forecast -->
        <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.5fr)]">
          <div class="card overflow-hidden p-0">
            <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.accounts.oauthCapacity.planMix') }}
              </h4>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.accounts.oauthCapacity.recommendationNote') }}
              </p>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[44rem] text-left text-sm">
                <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900 dark:text-gray-400">
                  <tr>
                    <th class="px-3 py-2.5">{{ t('admin.accounts.oauthCapacity.plan') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.accounts') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.schedulable') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.errors') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.rateLimited') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.unit5h') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.unit7d') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.add5h') }}</th>
                    <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.add7d') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="plan in planRows" :key="plan.plan_type" class="bg-white dark:bg-dark-800">
                    <td class="px-3 py-2.5 font-semibold uppercase text-gray-900 dark:text-white">
                      {{ plan.plan_type }}
                    </td>
                    <td class="px-3 py-2.5 text-right">{{ plan.total }}</td>
                    <td class="px-3 py-2.5 text-right">{{ plan.schedulable }}</td>
                    <td
                      class="px-3 py-2.5 text-right"
                      :class="plan.errors ? 'text-red-600 dark:text-red-300' : ''"
                    >
                      {{ plan.errors }}
                    </td>
                    <td class="px-3 py-2.5 text-right">{{ plan.rate_limited }}</td>
                    <td class="px-3 py-2.5 text-right font-mono">{{ baselineValue(plan.plan_type, '5h') }}</td>
                    <td class="px-3 py-2.5 text-right font-mono">{{ baselineValue(plan.plan_type, '7d') }}</td>
                    <td class="px-3 py-2.5 text-right font-mono">{{ recommendationValue(plan.plan_type, '5h') }}</td>
                    <td class="px-3 py-2.5 text-right font-mono">{{ recommendationValue(plan.plan_type, '7d') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="card space-y-3 p-4">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accounts.oauthCapacity.forecastInputs') }}
            </h4>
            <div class="space-y-2">
              <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.oauthCapacity.recent3h') }}</div>
                <div class="mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white">
                  {{ usd(series.forecast.recent_three_hour_usd) }}
                </div>
              </div>
              <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.oauthCapacity.previousDay3h') }}</div>
                <div class="mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white">
                  {{ usd(series.forecast.previous_day_same_period_usd) }}
                </div>
              </div>
              <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.oauthCapacity.rpmWindow') }}</div>
                <div class="mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white">
                  {{ usd(series.forecast.recent_rpm_window_usd) }}
                  <span class="text-xs font-normal text-gray-500">
                    ({{ usd(series.forecast.rpm_hourly_rate_usd) }}/h)
                  </span>
                </div>
              </div>
              <div class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.oauthCapacity.blendedRate') }}</div>
                <div class="mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white">
                  {{ usd(series.forecast.blended_hourly_rate_usd) }}/h
                </div>
              </div>
            </div>
            <p class="text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.oauthCapacity.groupAllocationNote') }}
            </p>
          </div>
        </div>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Title,
  Tooltip,
  type ChartData,
  type ChartOptions,
  type ScriptableContext,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type {
  OpenAIOAuthCapacityRange,
  OpenAIOAuthCapacityTimeseries,
  OpenAIOAuthCapacityWindowKind,
  OpenAIOAuthPlanCount
} from '@/api/admin/accounts'

ChartJS.register(Title, Tooltip, Legend, LineElement, LinearScale, PointElement, CategoryScale, Filler)

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t, locale } = useI18n()

const series = ref<OpenAIOAuthCapacityTimeseries | null>(null)
const loading = ref(false)
const error = ref('')
const selectedScopeKey = ref<string | number>('total')
const selectedRange = ref<OpenAIOAuthCapacityRange>('24h')
const selectedWindow = ref<OpenAIOAuthCapacityWindowKind>('5h')
const knownGroups = ref<Array<{ id: number; name: string }>>([])
let loadSeq = 0

const rangeOptions = [
  { value: '12h' as const, label: '12h' },
  { value: '24h' as const, label: '24h' },
  { value: '48h' as const, label: '48h' },
  { value: '7d' as const, label: '7d' }
]

const windowOptions = computed(() => [
  { value: '5h' as const, label: '5h' },
  { value: '7d' as const, label: '7d' },
  { value: 'both' as const, label: t('admin.accounts.oauthCapacity.bothWindows') }
])

const scopeSelectOptions = computed(() => [
  { value: 'total', label: t('admin.accounts.oauthCapacity.allGroups') },
  ...knownGroups.value.map((group) => ({
    value: `group:${group.id}`,
    label: group.name
  }))
])

const accountMetrics = computed(() => {
  if (!series.value) return []
  const health = series.value.health
  return [
    {
      label: t('admin.accounts.oauthCapacity.accounts'),
      value: health.total,
      icon: 'users' as const,
      cardClass: 'border-blue-200 bg-gradient-to-br from-blue-50 to-white dark:border-blue-800/30 dark:from-blue-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-blue-100 dark:bg-blue-900/30',
      iconClass: 'text-blue-600 dark:text-blue-400',
      valueClass: 'text-gray-900 dark:text-white'
    },
    {
      label: t('admin.accounts.oauthCapacity.schedulable'),
      value: health.schedulable,
      icon: 'check' as const,
      cardClass: 'border-emerald-200 bg-gradient-to-br from-emerald-50 to-white dark:border-emerald-800/30 dark:from-emerald-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-emerald-100 dark:bg-emerald-900/30',
      iconClass: 'text-emerald-600 dark:text-emerald-400',
      valueClass: 'text-gray-900 dark:text-white'
    },
    {
      label: t('admin.accounts.oauthCapacity.errors'),
      value: health.errors,
      icon: 'exclamationTriangle' as const,
      cardClass: 'border-red-200 bg-gradient-to-br from-red-50 to-white dark:border-red-800/30 dark:from-red-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-red-100 dark:bg-red-900/30',
      iconClass: 'text-red-600 dark:text-red-400',
      valueClass: health.errors ? 'text-red-600 dark:text-red-300' : 'text-gray-900 dark:text-white'
    },
    {
      label: t('admin.accounts.oauthCapacity.rateLimited'),
      value: series.value.rate_limits.total,
      icon: 'clock' as const,
      cardClass: 'border-amber-200 bg-gradient-to-br from-amber-50 to-white dark:border-amber-800/30 dark:from-amber-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-amber-100 dark:bg-amber-900/30',
      iconClass: 'text-amber-600 dark:text-amber-400',
      valueClass: series.value.rate_limits.total
        ? 'text-amber-700 dark:text-amber-300'
        : 'text-gray-900 dark:text-white'
    }
  ]
})

const planRows = computed<OpenAIOAuthPlanCount[]>(() => series.value?.plan_counts ?? [])

const rateLimitRows = computed(() => {
  const buckets = series.value?.rate_limits
  if (!buckets) return []
  return [
    { label: '≤10m', value: buckets.up_to_10m },
    { label: '10-30m', value: buckets.from_10m_to_30m },
    { label: '30m-1h', value: buckets.from_30m_to_1h },
    { label: '1-3h', value: buckets.from_1h_to_3h },
    { label: '3-5h', value: buckets.from_3h_to_5h },
    { label: '5h-1d', value: buckets.from_5h_to_1d },
    { label: '1-3d', value: buckets.from_1d_to_3d },
    { label: '>3d', value: buckets.over_3d }
  ]
})

const kpiCards = computed(() => {
  if (!series.value) return []
  const summary = series.value.summary
  const shortfall = summary.projected_shortfall_usd
  const hasCapacity = summary.capacity_usd != null && summary.capacity_accounts > 0
  const dash = '—'
  return [
    {
      label: t('admin.accounts.oauthCapacity.spent'),
      value: usd(summary.spent_usd),
      sub: '',
      icon: 'dollar' as const,
      cardClass: 'border-blue-200 bg-gradient-to-br from-blue-50 to-white dark:border-blue-800/30 dark:from-blue-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-blue-100 dark:bg-blue-900/30',
      iconClass: 'text-blue-600 dark:text-blue-400',
      valueClass: 'text-gray-900 dark:text-white'
    },
    {
      label: t('admin.accounts.oauthCapacity.available'),
      value: hasCapacity && summary.available_usd != null ? usd(summary.available_usd) : dash,
      sub: hasCapacity && summary.used_percent != null
        ? t('admin.accounts.oauthCapacity.usedPercent', { value: percent(summary.used_percent) })
        : t('admin.accounts.oauthCapacity.unmeasured'),
      icon: 'chartBar' as const,
      cardClass: 'border-emerald-200 bg-gradient-to-br from-emerald-50 to-white dark:border-emerald-800/30 dark:from-emerald-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-emerald-100 dark:bg-emerald-900/30',
      iconClass: 'text-emerald-600 dark:text-emerald-400',
      valueClass: hasCapacity ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-400'
    },
    {
      label: t('admin.accounts.oauthCapacity.forecastRemaining'),
      value: usd(summary.forecast_remaining_usd),
      sub: t('admin.accounts.oauthCapacity.untilReset'),
      icon: 'bolt' as const,
      cardClass: 'border-purple-200 bg-gradient-to-br from-purple-50 to-white dark:border-purple-800/30 dark:from-purple-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-purple-100 dark:bg-purple-900/30',
      iconClass: 'text-purple-600 dark:text-purple-400',
      valueClass: 'text-gray-900 dark:text-white'
    },
    {
      label: t('admin.accounts.oauthCapacity.projected'),
      value: usd(summary.projected_cycle_spend_usd),
      sub: hasCapacity && summary.capacity_usd != null
        ? usd(summary.capacity_usd) + ' cap'
        : t('admin.accounts.oauthCapacity.unmeasured'),
      icon: 'calculator' as const,
      cardClass: 'border-cyan-200 bg-gradient-to-br from-cyan-50 to-white dark:border-cyan-800/30 dark:from-cyan-900/10 dark:to-dark-700',
      iconWrapClass: 'bg-cyan-100 dark:bg-cyan-900/30',
      iconClass: 'text-cyan-600 dark:text-cyan-400',
      valueClass: 'text-gray-900 dark:text-white'
    },
    {
      label: t('admin.accounts.oauthCapacity.shortfall'),
      value: shortfall != null ? usd(shortfall) : dash,
      sub: t(`admin.accounts.oauthCapacity.confidence_${summary.confidence}`),
      icon: 'exclamationTriangle' as const,
      cardClass: shortfall != null && shortfall > 0
        ? 'border-red-200 bg-gradient-to-br from-red-50 to-white dark:border-red-800/30 dark:from-red-900/10 dark:to-dark-700'
        : 'border-gray-200 bg-gradient-to-br from-gray-50 to-white dark:border-dark-700 dark:from-dark-900/40 dark:to-dark-700',
      iconWrapClass: shortfall != null && shortfall > 0 ? 'bg-red-100 dark:bg-red-900/30' : 'bg-gray-100 dark:bg-dark-700',
      iconClass: shortfall != null && shortfall > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-500',
      valueClass: shortfall != null && shortfall > 0 ? 'text-red-600 dark:text-red-300' : 'text-gray-500'
    }
  ]
})

const isDarkMode = computed(() =>
  typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
)

const chartData = computed<ChartData<'line'> | null>(() => {
  if (!series.value?.points?.length) return null
  const points = series.value.points
  const hasAnySpend = points.some((p) => p.display_spend_usd != null || p.available_usd != null)
  if (!hasAnySpend) return null

  const labels = points.map((p) => formatBucketLabel(p.bucket_start))
  // null = gap (no data); Chart.js skips plotting instead of inventing zeros.
  const spendSolid = points.map((p) =>
    p.segment === 'future' ? null : p.display_spend_usd
  )
  const spendDashed = points.map((p) =>
    p.segment === 'past' ? null : p.display_spend_usd
  )
  const availableData = points.map((p) => p.available_usd)
  const available7dData = points.map((p) =>
    typeof p.available_7d_usd === 'number' ? p.available_7d_usd : null
  )
  const capacityData = points.map((p) => p.capacity_usd)

  const blue = '#3b82f6'
  const green = '#10b981'
  const cyan = '#06b6d4'
  const gray = isDarkMode.value ? '#6b7280' : '#9ca3af'

  const datasets: ChartData<'line'>['datasets'] = [
    {
      label: t('admin.accounts.oauthCapacity.chartSpend'),
      data: spendSolid,
      borderColor: blue,
      backgroundColor: `${blue}20`,
      borderWidth: 2,
      tension: 0.35,
      fill: false,
      pointRadius: (ctx: ScriptableContext<'line'>) =>
        points[ctx.dataIndex]?.segment === 'current' ? 4 : 0,
      pointHoverRadius: 5,
      pointBackgroundColor: blue,
      pointHitRadius: 12,
      spanGaps: false,
      order: 1
    },
    {
      label: t('admin.accounts.oauthCapacity.legendSpendDashed'),
      data: spendDashed,
      borderColor: blue,
      backgroundColor: 'transparent',
      borderWidth: 2,
      borderDash: [6, 4],
      tension: 0.35,
      fill: false,
      pointRadius: 0,
      pointHoverRadius: 4,
      pointHitRadius: 10,
      spanGaps: false,
      order: 1
    },
    {
      label: t('admin.accounts.oauthCapacity.chartAvailable'),
      data: availableData,
      borderColor: green,
      backgroundColor: `${green}20`,
      borderWidth: 2,
      tension: 0.35,
      fill: false,
      pointRadius: 0,
      pointHoverRadius: 4,
      pointHitRadius: 10,
      spanGaps: false,
      order: 2
    }
  ]

  // Capacity only when any point has a real capacity value.
  if (capacityData.some((v) => typeof v === 'number')) {
    datasets.push({
      label: t('admin.accounts.oauthCapacity.chartCapacity'),
      data: capacityData,
      borderColor: gray,
      backgroundColor: 'transparent',
      borderWidth: 1,
      borderDash: [2, 3],
      tension: 0,
      fill: false,
      pointRadius: 0,
      pointHitRadius: 8,
      spanGaps: false,
      order: 3
    })
  }

  if (selectedWindow.value === 'both' && available7dData.some((v) => typeof v === 'number')) {
    datasets.splice(3, 0, {
      label: t('admin.accounts.oauthCapacity.chartAvailable7d'),
      data: available7dData as Array<number | null>,
      borderColor: cyan,
      backgroundColor: 'transparent',
      borderWidth: 2,
      tension: 0.35,
      fill: false,
      pointRadius: 0,
      pointHoverRadius: 4,
      pointHitRadius: 10,
      spanGaps: false,
      order: 2
    })
  }

  return { labels, datasets }
})

const chartOptions = computed<ChartOptions<'line'>>(() => {
  const grid = isDarkMode.value ? '#374151' : '#e5e7eb'
  const text = isDarkMode.value ? '#e5e7eb' : '#374151'

  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: {
        position: 'top',
        align: 'end',
        labels: {
          color: text,
          usePointStyle: true,
          pointStyle: 'circle',
          boxWidth: 6,
          font: { size: 11 }
        }
      },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        borderColor: grid,
        borderWidth: 1,
        padding: 10,
        callbacks: {
          title(items: TooltipItem<'line'>[]) {
            const idx = items[0]?.dataIndex ?? 0
            const point = series.value?.points[idx]
            if (!point) return ''
            return `${formatBucketLabel(point.bucket_start)} · ${point.segment}`
          },
          label(item: TooltipItem<'line'>) {
            if (item.parsed.y == null) return `${item.dataset.label}: —`
            return `${item.dataset.label}: ${usd(item.parsed.y)}`
          },
          afterBody(items: TooltipItem<'line'>[]) {
            const idx = items[0]?.dataIndex ?? 0
            const point = series.value?.points[idx]
            if (!point) return []
            const lines: string[] = []
            if (point.spent_usd != null) {
              lines.push(`${t('admin.accounts.oauthCapacity.spent')}: ${usd(point.spent_usd)}`)
            }
            if (point.forecast_usd != null) {
              lines.push(`${t('admin.accounts.oauthCapacity.forecastRemaining')}: ${usd(point.forecast_usd)}`)
            }
            if (point.used_percent != null) {
              lines.push(`${t('admin.accounts.oauthCapacity.used')}: ${percent(point.used_percent)}`)
            }
            if (point.shortfall_risk_usd != null && point.shortfall_risk_usd > 0) {
              lines.push(`${t('admin.accounts.oauthCapacity.shortfall')}: ${usd(point.shortfall_risk_usd)}`)
            }
            if (point.sealed) {
              lines.push(t('admin.accounts.oauthCapacity.sealedHour'))
            }
            return lines
          }
        }
      }
    },
    scales: {
      x: {
        ticks: {
          color: text,
          maxRotation: 0,
          autoSkip: true,
          maxTicksLimit: 12,
          font: { size: 10 },
          callback(_value, index) {
            const label = (chartData.value?.labels?.[index] as string) || ''
            if (series.value?.points[index]?.segment === 'current') return `${label}*`
            return label
          }
        },
        grid: { color: grid }
      },
      y: {
        ticks: {
          color: text,
          font: { size: 10 },
          callback(value) {
            const n = typeof value === 'number' ? value : Number(value)
            return `$${n.toFixed(0)}`
          }
        },
        grid: { color: grid },
        beginAtZero: true
      }
    },
    animation: false
  }
})

async function load(force = false) {
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  try {
    const params: Parameters<typeof adminAPI.accounts.getOpenAIOAuthCapacityTimeseries>[0] = {
      range: selectedRange.value,
      window: selectedWindow.value,
      force
    }
    const scopeKey = String(selectedScopeKey.value)
    if (scopeKey.startsWith('group:')) {
      params.group_id = Number(scopeKey.replace('group:', ''))
    }
    const data = await adminAPI.accounts.getOpenAIOAuthCapacityTimeseries(params)
    if (seq !== loadSeq) return
    series.value = data

    if (force || knownGroups.value.length === 0) {
      try {
        const overview = await adminAPI.accounts.getOpenAIOAuthCapacity(force)
        if (seq !== loadSeq) return
        knownGroups.value = (overview.groups ?? [])
          .filter((g) => typeof g.group_id === 'number')
          .map((g) => ({ id: g.group_id as number, name: g.group_name }))
      } catch {
        // Non-fatal.
      }
    }
  } catch (caught) {
    if (seq !== loadSeq) return
    error.value = caught instanceof Error ? caught.message : t('admin.accounts.oauthCapacity.loadFailed')
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function usd(value: number) {
  return new Intl.NumberFormat(locale.value, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value || 0)
}

function percent(value: number) {
  return `${(value || 0).toFixed(1)}%`
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }).format(new Date(value))
}

function formatBucketLabel(value: string) {
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    hour12: false
  }).format(new Date(value))
}

function recommendationValue(planType: string, window: '5h' | '7d') {
  const item = series.value?.recommendations.find((recommendation) => recommendation.plan_type === planType)
  const value = window === '5h' ? item?.five_hour_accounts : item?.seven_day_accounts
  if (typeof value !== 'number' || value === 0) return '-'
  return `+${value}`
}

function baselineValue(planType: string, window: '5h' | '7d') {
  const item = series.value?.baselines.find(
    (baseline) => baseline.plan_type === planType && baseline.window === window
  )
  return item ? usd(item.median_capacity_usd) : '-'
}

watch(
  () => props.show,
  (show) => {
    if (show) void load(false)
  },
  { immediate: true }
)

watch([selectedRange, selectedWindow, selectedScopeKey], () => {
  if (props.show && series.value) void load(false)
})
</script>
