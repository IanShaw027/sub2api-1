<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.oauthCapacity.title')"
    width="full"
    @close="emit('close')"
  >
    <div class="space-y-6">
      <div
        class="flex flex-wrap items-end justify-between gap-3 rounded-xl border border-line bg-surface p-3 shadow-sm"
      >
        <div class="flex min-w-0 flex-1 flex-wrap items-end gap-3">
          <div class="flex flex-col gap-1">
            <span class="text-xs font-medium text-muted">
              {{ t('admin.accounts.oauthCapacity.range') }}
            </span>
            <div class="inline-flex rounded-lg border border-line p-0.5">
              <button
                v-for="option in rangeOptions"
                :key="option"
                type="button"
                class="rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors"
                :class="
 selectedRange === option
 ? 'bg-accent text-white shadow-sm'
 : 'text-muted hover:bg-surface-2'
 "
                @click="selectedRange = option"
              >
                {{ option }}
              </button>
            </div>
          </div>
          <div class="flex min-w-[14rem] flex-col gap-1">
            <span class="text-xs font-medium text-muted">
              {{ t('admin.accounts.oauthCapacity.scope') }}
            </span>
            <Select
              :model-value="selectedGroupId"
              :options="groupSelectOptions"
              @update:model-value="setGroupId"
            />
          </div>
        </div>
        <div class="flex items-center gap-2">
          <span v-if="overview" class="text-xs text-muted">
            {{ formatTime(overview.generated_at) }}
          </span>
          <button
            type="button"
            class="btn btn-secondary px-2.5"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="load(true)"
          >
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          </button>
        </div>
      </div>

      <p class="text-sm text-muted">
        {{ t('admin.accounts.oauthCapacity.hint') }}
      </p>

      <div v-if="loading && !overview" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <div
        v-else-if="error"
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-danger-text"
      >
        {{ error }}
      </div>

      <template v-else-if="overview">
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div
            v-for="metric in accountMetrics"
            :key="metric.label"
            class="glass-card min-w-0 p-4"
            :class="metric.cardClass"
          >
            <div class="mb-2 flex items-center justify-between">
              <span class="text-xs font-medium text-muted">{{ metric.label }}</span>
              <div class="rounded-lg p-1.5" :class="metric.iconWrapClass">
                <Icon :name="metric.icon" size="sm" :class="metric.iconClass" />
              </div>
            </div>
            <p class="text-2xl font-bold" :class="metric.valueClass">{{ metric.value }}</p>
          </div>
        </div>

        <div class="glass-card p-4">
          <h4 class="mb-3 text-sm font-semibold text-foreground">
            {{ t('admin.accounts.oauthCapacity.rateLimitDistribution') }}
          </h4>
          <div class="grid grid-cols-4 gap-2 sm:grid-cols-8">
            <div
              v-for="bucket in rateLimitRows"
              :key="bucket.label"
              class="rounded-lg border border-line bg-surface-2 px-2 py-2.5 text-center"
            >
              <div class="text-[11px] text-muted">{{ bucket.label }}</div>
              <div
                class="mt-1 text-base font-semibold"
                :class="bucket.value ? 'text-amber-700' : 'text-muted'"
              >
                {{ bucket.value }}
              </div>
            </div>
          </div>
        </div>

        <div class="grid gap-3 md:grid-cols-2">
          <div
            v-for="window in overview.total.windows"
            :key="window.window"
            class="rounded-lg border p-4"
            :class="windowCardClass(window.alert)"
          >
            <div class="mb-2 flex items-center justify-between">
              <div class="text-sm font-medium">{{ windowLabel(window.window) }}</div>
              <span
                v-if="window.alert"
                class="text-xs font-medium uppercase"
                :class="window.alert === 'critical' ? 'text-danger-text' : 'text-amber-700'"
              >
                {{ window.alert === 'critical'
                  ? t('admin.accounts.oauthCapacity.alertCritical')
                  : t('admin.accounts.oauthCapacity.alertWarning') }}
              </span>
            </div>
            <div class="text-3xl font-semibold">
              {{ formatPercent(window.remaining_percent) }}
            </div>
            <div class="mt-2 space-y-1 text-xs text-muted">
              <div>
                {{ t('admin.accounts.oauthCapacity.remaining') }}
                · {{ t('admin.accounts.oauthCapacity.used') }} {{ formatPercent(window.used_percent) }}
                · {{ t('admin.accounts.oauthCapacity.burnRate') }}
                {{ window.remaining_percent == null ? '—' : `${window.burn_rate.toFixed(2)}x` }}
              </div>
              <div>
                {{ t('admin.accounts.oauthCapacity.resetAt') }} {{ formatTime(window.reset_at) }}
                · {{ t('admin.accounts.oauthCapacity.exhaustsAt') }} {{ formatTime(window.exhausts_at) }}
              </div>
              <div>{{ t('admin.accounts.oauthCapacity.measured', { count: window.measured_accounts }) }}</div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div
            v-for="kpi in forecastKpis"
            :key="kpi.label"
            class="glass-card min-w-0 border p-4"
            :class="kpi.cardClass"
          >
            <div class="mb-2 text-xs font-medium text-muted">{{ kpi.label }}</div>
            <p class="font-mono text-xl font-bold" :class="kpi.valueClass">{{ kpi.value }}</p>
          </div>
        </div>

        <div
          v-if="overview.total.suggest_accounts"
          class="rounded-lg border border-amber-200 bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-3 text-sm text-amber-800"
        >
          {{ t('admin.accounts.oauthCapacity.suggestAdd', { count: overview.total.suggest_accounts }) }}
        </div>

        <div class="glass-card p-4">
          <h4 class="mb-3 text-sm font-semibold text-foreground">
            {{ t('admin.accounts.oauthCapacity.hourlyTrend') }}
          </h4>
          <div class="relative min-h-48">
            <div
              v-if="loading && series"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/60"
            >
              <LoadingSpinner />
            </div>
            <div v-if="chartData" class="h-72 w-full">
              <Line :data="chartData" :options="chartOptions" />
            </div>
            <div
              v-else
              class="flex h-48 items-center justify-center text-sm text-muted"
            >
              {{ t('admin.accounts.oauthCapacity.noTrendData') }}
            </div>
          </div>
        </div>

        <div class="glass-card overflow-hidden p-0">
          <div class="border-b border-line px-4 py-3">
            <h4 class="text-sm font-semibold text-foreground">
              {{ t('admin.accounts.oauthCapacity.planMix') }}
            </h4>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead class="bg-surface-2 text-xs uppercase text-muted">
                <tr>
                  <th class="px-3 py-2.5">{{ t('admin.accounts.oauthCapacity.plan') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.accounts') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.schedulable') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.errors') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.rateLimited') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-line">
                <tr v-for="plan in overview.total.plan_counts" :key="plan.plan_type">
                  <td class="px-3 py-2.5 font-semibold uppercase">{{ plan.plan_type }}</td>
                  <td class="px-3 py-2.5 text-right">{{ plan.total }}</td>
                  <td class="px-3 py-2.5 text-right">{{ plan.schedulable }}</td>
                  <td class="px-3 py-2.5 text-right" :class="plan.errors ? 'text-red-600' : ''">{{ plan.errors }}</td>
                  <td class="px-3 py-2.5 text-right" :class="plan.rate_limited ? 'text-amber-600' : ''">{{ plan.rate_limited }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-if="overview.groups?.length" class="glass-card overflow-hidden p-0">
          <div class="border-b border-line px-4 py-3">
            <h4 class="text-sm font-semibold text-foreground">
              {{ t('admin.accounts.oauthCapacity.groups') }}
            </h4>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead class="bg-surface-2 text-xs uppercase text-muted">
                <tr>
                  <th class="px-3 py-2.5">{{ t('admin.accounts.oauthCapacity.groups') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.accounts') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.schedulable') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.errors') }}</th>
                  <th class="px-3 py-2.5 text-right">{{ t('admin.accounts.oauthCapacity.rateLimited') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-line">
                <tr v-for="group in overview.groups" :key="group.group_id ?? group.group_name">
                  <td class="px-3 py-2.5 font-medium">
                    {{ group.group_name === 'ungrouped' ? t('admin.accounts.oauthCapacity.ungrouped') : group.group_name }}
                  </td>
                  <td class="px-3 py-2.5 text-right">{{ group.accounts.total }}</td>
                  <td class="px-3 py-2.5 text-right">{{ group.accounts.schedulable }}</td>
                  <td class="px-3 py-2.5 text-right" :class="group.accounts.errors ? 'text-red-600' : ''">
                    {{ group.accounts.errors }}
                  </td>
                  <td class="px-3 py-2.5 text-right" :class="group.accounts.rate_limited ? 'text-amber-600' : ''">
                    {{ group.accounts.rate_limited }}
                  </td>
                </tr>
              </tbody>
            </table>
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
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Title,
  Tooltip,
  type ChartData,
  type ChartOptions,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import { getOverview, getTimeseries, type OAuthCapacityOverview } from '@/api/admin/oauthCapacity'
import type { CapacityRange, CapacityTimeseries } from '@/api/admin/capacity'

ChartJS.register(Title, Tooltip, Legend, LineElement, LinearScale, PointElement, CategoryScale)

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { t, locale } = useI18n()
const loading = ref(false)
const error = ref('')
const overview = ref<OAuthCapacityOverview | null>(null)
const series = ref<CapacityTimeseries | null>(null)
const selectedRange = ref<CapacityRange>('24h')
const selectedGroupId = ref<number | null>(null)
const knownGroups = ref<{ id: number; name: string }[]>([])
const rangeOptions: CapacityRange[] = ['12h', '24h', '48h', '7d']
let loadSeq = 0

const groupSelectOptions = computed(() => [
  { value: null, label: t('admin.accounts.oauthCapacity.allGroups') },
  ...knownGroups.value.map((group) => ({ value: group.id, label: group.name }))
])

const accountMetrics = computed(() => {
  if (!overview.value) return []
  const accounts = overview.value.total.accounts
  return [
    {
      label: t('admin.accounts.oauthCapacity.accounts'),
      value: accounts.total,
      icon: 'users' as const,
      cardClass: 'border-line',
      iconWrapClass: 'bg-surface-2',
      iconClass: 'text-muted',
      valueClass: 'text-foreground'
    },
    {
      label: t('admin.accounts.oauthCapacity.schedulable'),
      value: accounts.schedulable,
      icon: 'check' as const,
      cardClass: 'border-emerald-200',
      iconWrapClass: 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)]',
      iconClass: 'text-success-text',
      valueClass: 'text-success-text'
    },
    {
      label: t('admin.accounts.oauthCapacity.errors'),
      value: accounts.errors,
      icon: 'exclamationTriangle' as const,
      cardClass: accounts.errors
        ? 'border-red-200 bg-red-50'
        : 'border-line',
      iconWrapClass: accounts.errors ? 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)]' : 'bg-surface-2',
      iconClass: accounts.errors ? 'text-danger-text' : 'text-muted',
      valueClass: accounts.errors ? 'text-danger-text' : 'text-foreground'
    },
    {
      label: t('admin.accounts.oauthCapacity.rateLimited'),
      value: accounts.rate_limited,
      icon: 'clock' as const,
      cardClass: accounts.rate_limited
        ? 'border-amber-200 bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]'
        : 'border-line',
      iconWrapClass: accounts.rate_limited ? 'bg-amber-100' : 'bg-surface-2',
      iconClass: accounts.rate_limited ? 'text-warning-text' : 'text-muted',
      valueClass: accounts.rate_limited ? 'text-amber-700' : 'text-foreground'
    }
  ]
})

const rateLimitRows = computed(() => {
  const buckets = overview.value?.total.rate_limits
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

const forecastKpis = computed(() => {
  const kpis = series.value?.kpis
  const shortfall = overview.value?.total.first_shortfall_at ?? kpis?.first_shortfall_at ?? null
  return [
    {
      label: t('admin.accounts.capacityForecast.kpiCurrentAvailable'),
      value: kpis ? usd(kpis.current_available_usd) : '—',
      cardClass: 'border-emerald-200',
      valueClass: 'text-success-text'
    },
    {
      label: t('admin.accounts.capacityForecast.kpiFutureForecastSpend'),
      value: kpis ? usd(kpis.future_forecast_spend_usd) : '—',
      cardClass: 'border-blue-200',
      valueClass: 'text-foreground'
    },
    {
      label: t('admin.accounts.oauthCapacity.firstShortfall'),
      value: shortfall ? formatTime(shortfall) : t('admin.accounts.oauthCapacity.noShortfall'),
      cardClass: shortfall
        ? 'border-red-200'
        : 'border-line',
      valueClass: shortfall ? 'text-danger-text' : 'text-muted'
    },
    {
      label: t('admin.accounts.capacityForecast.kpiSuggestAccounts'),
      value: String(overview.value?.total.suggest_accounts ?? kpis?.suggest_accounts ?? 0),
      cardClass: 'border-purple-200',
      valueClass: 'text-foreground'
    }
  ]
})

const isDarkMode = computed(
  () => typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
)

const chartData = computed<ChartData<'line'> | null>(() => {
  const points = series.value?.points
  if (!points?.length) return null
  const hasValues = points.some(
    (point) => point.spend_usd != null || point.available_usd != null || point.forecast_spend_usd != null
  )
  if (!hasValues) return null

  const blue = '#3b82f6'
  const green = '#10b981'
  return {
    labels: points.map((point) => formatBucketLabel(point.bucket_start)),
    datasets: [
      {
        label: t('admin.accounts.capacityForecast.chartActualSpend'),
        data: points.map((point) => (point.segment === 'future' ? null : point.spend_usd)),
        borderColor: blue,
        backgroundColor: `${blue}20`,
        borderWidth: 2,
        tension: 0.3,
        fill: false,
        pointRadius: 0,
        spanGaps: false
      },
      {
        label: t('admin.accounts.capacityForecast.chartForecastSpend'),
        data: points.map((point) => (point.segment === 'past' ? null : point.forecast_spend_usd ?? point.spend_usd)),
        borderColor: blue,
        borderDash: [6, 4],
        borderWidth: 2,
        tension: 0.3,
        fill: false,
        pointRadius: 0,
        spanGaps: false
      },
      {
        label: t('admin.accounts.capacityForecast.chartAvailable'),
        data: points.map((point) => (point.segment === 'future' ? null : point.available_usd)),
        borderColor: green,
        backgroundColor: `${green}20`,
        borderWidth: 2,
        tension: 0.3,
        fill: false,
        pointRadius: 0,
        spanGaps: false
      }
    ]
  }
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
        labels: { color: text, usePointStyle: true, pointStyle: 'circle', boxWidth: 6, font: { size: 11 } }
      },
      tooltip: {
        callbacks: {
          title(items: TooltipItem<'line'>[]) {
            const point = series.value?.points[items[0]?.dataIndex ?? 0]
            return point ? `${formatBucketLabel(point.bucket_start)} · ${point.segment}` : ''
          },
          label(item: TooltipItem<'line'>) {
            if (item.parsed.y == null) return `${item.dataset.label}: —`
            return `${item.dataset.label}: ${usd(item.parsed.y)}`
          }
        }
      }
    },
    scales: {
      x: { ticks: { color: text, maxRotation: 0, autoSkip: true, maxTicksLimit: 12, font: { size: 10 } }, grid: { color: grid } },
      y: {
        beginAtZero: true,
        ticks: {
          color: text,
          font: { size: 10 },
          callback(value) {
            return `$${Number(value).toFixed(0)}`
          }
        },
        grid: { color: grid }
      }
    },
    animation: false
  }
})

function setGroupId(value: string | number | boolean | null) {
  selectedGroupId.value = typeof value === 'number' ? value : null
}

function windowLabel(window: string) {
  if (window === '5h') return t('admin.accounts.oauthCapacity.window5h')
  if (window === '7d') return t('admin.accounts.oauthCapacity.window7d')
  return window
}

function windowCardClass(alert?: string) {
  if (alert === 'critical') return 'border-red-300 bg-red-50'
  if (alert === 'warning') return 'border-amber-300 bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]'
  return 'border-line'
}

function formatPercent(value: number | null | undefined) {
  return value == null ? '—' : `${value.toFixed(1)}%`
}

function formatTime(value?: string | null) {
  if (!value) return '—'
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
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

function usd(value: number) {
  return new Intl.NumberFormat(locale.value, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value)
}

async function load(force = false) {
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  try {
    const groupId = selectedGroupId.value ?? undefined
    const [nextOverview, nextSeries] = await Promise.all([
      getOverview(groupId),
      getTimeseries({ group_id: groupId, range: selectedRange.value, force })
    ])
    if (seq !== loadSeq) return
    overview.value = nextOverview
    series.value = nextSeries
    if (groupId == null) {
      knownGroups.value = (nextOverview.groups ?? [])
        .filter((group) => typeof group.group_id === 'number')
        .map((group) => ({ id: group.group_id as number, name: group.group_name }))
    }
  } catch (caught) {
    if (seq !== loadSeq) return
    error.value = caught instanceof Error ? caught.message : t('admin.accounts.oauthCapacity.loadFailed')
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) void load(false)
  },
  { immediate: true }
)

watch([selectedRange, selectedGroupId], () => {
  if (props.show && overview.value) void load(false)
})
</script>
