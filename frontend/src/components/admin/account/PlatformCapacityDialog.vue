<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.capacityForecast.title')"
    width="full"
    @close="emit('close')"
  >
    <div class="space-y-6">
      <!-- Toolbar -->
      <div
        class="flex flex-wrap items-end justify-between gap-3 rounded-xl border border-line bg-surface p-3 shadow-sm"
      >
        <div class="flex min-w-0 flex-1 flex-wrap items-end gap-3">
          <div class="flex min-w-[10rem] flex-col gap-1">
            <span class="text-xs font-medium text-muted">
              {{ t('admin.accounts.capacityForecast.platform') }}
            </span>
            <Select
              :model-value="selectedPlatform"
              :options="platformOptions"
              :searchable="false"
              @update:model-value="setPlatform"
            >
              <template #selected>
                <span class="flex items-center gap-1.5">
                  <PlatformIcon :platform="(selectedPlatform as GroupPlatform)" size="sm" />
                  <span>{{ selectedPlatformLabel }}</span>
                </span>
              </template>
              <template #option="{ option, selected }">
                <span class="flex min-w-0 items-center gap-1.5">
                  <PlatformIcon :platform="(option.value as GroupPlatform)" size="sm" />
                  <span class="truncate">{{ option.label }}</span>
                </span>
                <Icon v-if="selected" name="check" size="sm" class="text-accent" />
              </template>
            </Select>
          </div>

          <div class="flex min-w-[12rem] flex-col gap-1">
            <span class="text-xs font-medium text-muted">
              {{ t('admin.accounts.capacityForecast.group') }}
            </span>
            <Select
              :model-value="selectedGroupId"
              :options="groupSelectOptions"
              @update:model-value="setGroupId"
            />
          </div>

          <div class="flex flex-col gap-1">
            <span class="text-xs font-medium text-muted">
              {{ t('admin.accounts.capacityForecast.range') }}
            </span>
            <div class="inline-flex rounded-lg border border-line p-0.5">
              <button
                v-for="option in rangeOptions"
                :key="option.value"
                type="button"
                class="rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors"
                :class="
 selectedRange === option.value
 ? 'bg-primary-600 text-white shadow-sm'
 : 'text-muted hover:bg-surface-2'
 "
                @click="selectedRange = option.value"
              >
                {{ option.label }}
              </button>
            </div>
          </div>

          <div class="flex flex-col gap-1">
            <span class="text-xs font-medium text-transparent">·</span>
            <button type="button" class="btn btn-secondary" @click="openProbeModal">
              <Icon name="beaker" size="sm" />
              <span>{{ t('admin.accounts.capacityForecast.probe') }}</span>
            </button>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <span v-if="series" class="text-xs text-muted">
            {{ t('admin.accounts.capacityForecast.generatedAt', { time: formatTime(series.generated_at) }) }}
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
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-danger-text"
      >
        {{ error }}
      </div>

      <template v-else-if="series">
        <!-- KPI bar -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div
            v-for="kpi in kpiItems"
            :key="kpi.key"
            class="card min-w-0 border p-4"
            :class="kpi.cardClass"
          >
            <div class="mb-2 flex items-center justify-between gap-2">
              <span class="truncate text-xs font-medium text-muted">{{ kpi.label }}</span>
              <div class="rounded-lg p-1.5" :class="kpi.iconWrapClass">
                <Icon :name="kpi.icon" size="sm" :class="kpi.iconClass" />
              </div>
            </div>
            <p class="truncate text-xl font-bold" :class="kpi.valueClass">{{ kpi.value }}</p>
          </div>
        </div>

        <!-- Trend chart -->
        <div class="card p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h4 class="text-sm font-semibold text-foreground">
              {{ t('admin.accounts.capacityForecast.chartTitle') }}
            </h4>
            <div class="flex flex-wrap items-center gap-3 text-[11px] text-muted">
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 rounded bg-blue-500" />
                {{ t('admin.accounts.capacityForecast.chartActualSpend') }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 border-t-2 border-dashed border-blue-500" />
                {{ t('admin.accounts.capacityForecast.chartForecastSpend') }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 rounded bg-emerald-500" />
                {{ t('admin.accounts.capacityForecast.chartAvailable') }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-0.5 w-4 border-t-2 border-dashed border-emerald-500" />
                {{ t('admin.accounts.capacityForecast.chartForecastAvailable') }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="inline-block h-2 w-2 rounded-full bg-red-400/60" />
                {{ t('admin.accounts.capacityForecast.chartShortfallBand') }}
              </span>
            </div>
          </div>

          <div class="relative min-h-48">
            <div
              v-if="loading && series"
              class="absolute inset-0 z-10 flex items-center justify-center bg-white/60"
            >
              <LoadingSpinner />
            </div>
            <div v-if="chartData" class="h-[360px] w-full">
              <Line :data="chartData" :options="chartOptions" :plugins="chartPlugins" />
            </div>
            <div
              v-else
              class="flex h-48 items-center justify-center text-sm text-muted"
            >
              {{ t('admin.accounts.capacityForecast.noData') }}
            </div>
          </div>
        </div>

        <!-- Events -->
        <div class="card overflow-hidden p-0">
          <div class="border-b border-line px-4 py-3">
            <h4 class="text-sm font-semibold text-foreground">
              {{ t('admin.accounts.capacityForecast.eventsTitle') }}
            </h4>
          </div>
          <div
            v-if="events.length === 0"
            class="px-4 py-6 text-center text-sm text-muted"
          >
            {{ t('admin.accounts.capacityForecast.eventsEmpty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[40rem] text-left text-sm">
              <thead class="bg-surface-2 text-xs text-muted">
                <tr>
                  <th class="px-3 py-2.5">{{ t('admin.accounts.capacityForecast.eventsTime') }}</th>
                  <th class="px-3 py-2.5">{{ t('admin.accounts.capacityForecast.eventsType') }}</th>
                  <th class="px-3 py-2.5">{{ t('admin.accounts.capacityForecast.eventsDetail') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-line">
                <tr v-for="(event, index) in events" :key="`${event.at}-${index}`" class="bg-surface">
                  <td class="whitespace-nowrap px-3 py-2.5 font-mono text-xs text-muted">
                    {{ formatTime(event.at) }}
                  </td>
                  <td class="px-3 py-2.5">
                    <span
                      class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
                      :class="
 event.type === 'recover'
 ? 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text'
 : 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text'
 "
                    >
                      {{
                        event.type === 'recover'
                          ? t('admin.accounts.capacityForecast.eventTypeRecover')
                          : t('admin.accounts.capacityForecast.eventTypeShortfall')
                      }}
                    </span>
                  </td>
                  <td class="px-3 py-2.5 text-foreground">
                    <div>{{ eventDetail(event) }}</div>
                    <div
                      v-if="eventRecommendation(event)"
                      class="mt-0.5 text-xs text-muted"
                    >
                      {{
                        t('admin.accounts.capacityForecast.eventRecommendation', {
                          count: eventRecommendation(event)!.suggest_accounts,
                          basis: eventRecommendation(event)!.basis
                        })
                      }}
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>

    <!-- Probe (test) modal -->
    <BaseDialog
      :show="showProbeModal"
      :title="t('admin.accounts.capacityForecast.probeTitle')"
      width="normal"
      :z-index="60"
      @close="closeProbeModal"
    >
      <div class="space-y-4">
        <p class="text-sm text-muted">
          {{ t('admin.accounts.capacityForecast.probeDesc') }}
        </p>
        <label
          class="flex items-center justify-between gap-3 rounded-lg border border-line bg-surface-2 px-3 py-2.5"
        >
          <span class="text-sm text-foreground">
            {{ t('admin.accounts.capacityForecast.probeIncludeNormal') }}
          </span>
          <Toggle v-model="probeIncludeNormal" />
        </label>

        <div
          v-if="probing"
          class="flex items-center gap-2 rounded-lg border border-line bg-surface-2 px-3 py-2.5 text-sm text-muted"
        >
          <LoadingSpinner size="sm" />
          {{ t('admin.accounts.capacityForecast.probeRunning') }}
        </div>

        <div
          v-if="probeError"
          class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-danger-text"
        >
          {{ probeError }}
        </div>

        <div
          v-if="probeResult"
          class="space-y-2 rounded-lg border border-line bg-surface-2 p-3"
        >
          <p class="text-sm text-foreground">
            {{
              t('admin.accounts.capacityForecast.probeResultSummary', {
                probed: probeResult.probed,
                total: probeResult.total,
                ok: probeResult.ok,
                rateLimited: probeResult.rate_limited,
                failed: probeResult.failed,
                duration: (probeResult.duration_ms / 1000).toFixed(1)
              })
            }}
          </p>
          <div v-if="probeResult.failures.length > 0">
            <button
              type="button"
              class="text-xs font-medium text-accent hover:underline"
              @click="showProbeFailures = !showProbeFailures"
            >
              {{ t('admin.accounts.capacityForecast.probeFailuresTitle', { count: probeResult.failures.length }) }}
              <Icon :name="showProbeFailures ? 'chevronUp' : 'chevronDown'" size="sm" class="inline" />
            </button>
            <ul
              v-if="showProbeFailures"
              class="mt-2 max-h-40 space-y-1 overflow-y-auto text-xs text-muted"
            >
              <li
                v-for="failure in probeResult.failures"
                :key="failure.account_id"
                class="rounded bg-surface px-2 py-1"
              >
                <span class="font-medium">{{ failure.name }}</span>: {{ failure.error }}
              </li>
            </ul>
          </div>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="probing" @click="closeProbeModal">
          {{ probeResult ? t('admin.accounts.capacityForecast.probeClose') : t('admin.accounts.capacityForecast.probeCancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="probing" @click="runProbe">
          <Icon name="beaker" size="sm" />
          <span>{{ t('admin.accounts.capacityForecast.probeStart') }}</span>
        </button>
      </template>
    </BaseDialog>
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
  type Plugin,
  type ScriptableContext,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { adminAPI } from '@/api/admin'
import { platformLabel } from '@/utils/platformColors'
import type {
  CapacityEvent,
  CapacityPlatform,
  CapacityProbeResult,
  CapacityRange,
  CapacityRecommendation,
  CapacityTimeseries
} from '@/api/admin/capacity'
import type { AdminGroup, GroupPlatform } from '@/types'

ChartJS.register(Title, Tooltip, Legend, LineElement, LinearScale, PointElement, CategoryScale, Filler)

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t, locale } = useI18n()

const PLATFORMS: CapacityPlatform[] = ['openai', 'anthropic', 'gemini', 'grok', 'antigravity']

const series = ref<CapacityTimeseries | null>(null)
const loading = ref(false)
const error = ref('')
const selectedPlatform = ref<CapacityPlatform>('openai')
const selectedGroupId = ref<number | null>(null)
const selectedRange = ref<CapacityRange>('24h')
const groups = ref<AdminGroup[]>([])
let loadSeq = 0

const rangeOptions = [
  { value: '12h' as const, label: '12h' },
  { value: '24h' as const, label: '24h' },
  { value: '48h' as const, label: '48h' },
  { value: '7d' as const, label: '7d' }
]

const platformOptions = computed(() =>
  PLATFORMS.map((platform) => ({ value: platform, label: platformLabel(platform) }))
)

const selectedPlatformLabel = computed(() => platformLabel(selectedPlatform.value))

function setPlatform(value: string | number | boolean | null) {
  if (typeof value === 'string' && (PLATFORMS as string[]).includes(value)) {
    selectedPlatform.value = value as CapacityPlatform
  }
}

function setGroupId(value: string | number | boolean | null) {
  selectedGroupId.value = typeof value === 'number' ? value : null
}

const groupSelectOptions = computed(() => [
  { value: null, label: t('admin.accounts.capacityForecast.allGroups') },
  ...groups.value.map((group) => ({ value: group.id, label: group.name }))
])

const events = computed<CapacityEvent[]>(() => series.value?.events ?? [])

const isDarkMode = computed(
  () => typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
)

const kpiItems = computed(() => {
  if (!series.value) return []
  const kpis = series.value.kpis
  const hasShortfall = !!kpis.first_shortfall_at
  return [
    {
      key: 'available',
      label: t('admin.accounts.capacityForecast.kpiCurrentAvailable'),
      value: usd(kpis.current_available_usd),
      icon: 'dollar' as const,
      cardClass: 'border-emerald-200 bg-gradient-to-br from-emerald-50 to-white',
      iconWrapClass: 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)]',
      iconClass: 'text-success-text',
      valueClass: 'text-foreground'
    },
    {
      key: 'forecastSpend',
      label: t('admin.accounts.capacityForecast.kpiFutureForecastSpend'),
      value: usd(kpis.future_forecast_spend_usd),
      icon: 'trendingUp' as const,
      cardClass: 'border-blue-200 bg-gradient-to-br from-blue-50 to-white',
      iconWrapClass: 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]',
      iconClass: 'text-accent',
      valueClass: 'text-foreground'
    },
    {
      key: 'shortfall',
      label: t('admin.accounts.capacityForecast.kpiFirstShortfall'),
      value: hasShortfall
        ? formatTime(kpis.first_shortfall_at as string)
        : t('admin.accounts.capacityForecast.kpiNoShortfall'),
      icon: hasShortfall ? ('exclamationTriangle' as const) : ('check' as const),
      cardClass: hasShortfall
        ? 'border-red-200 bg-gradient-to-br from-red-50 to-white'
        : 'border-emerald-200 bg-gradient-to-br from-emerald-50 to-white',
      iconWrapClass: hasShortfall ? 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)]' : 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)]',
      iconClass: hasShortfall ? 'text-danger-text' : 'text-success-text',
      valueClass: hasShortfall ? 'text-danger-text' : 'text-success-text'
    },
    {
      key: 'suggest',
      label: t('admin.accounts.capacityForecast.kpiSuggestAccounts'),
      value: String(kpis.suggest_accounts),
      icon: 'calculator' as const,
      cardClass: 'border-purple-200 bg-gradient-to-br from-purple-50 to-white',
      iconWrapClass: 'bg-purple-100',
      iconClass: 'text-purple-600',
      valueClass: 'text-foreground'
    }
  ]
})

const currentBucketIndex = computed(() => {
  const points = series.value?.points ?? []
  const idx = points.findIndex((p) => p.segment === 'current')
  return idx >= 0 ? idx : null
})

// Buckets where the forecast says available quota will run below forecast spend.
const gapBucketIndexes = computed(() => {
  const points = series.value?.points ?? []
  const idxs: number[] = []
  points.forEach((point, idx) => {
    if (
      typeof point.forecast_available_usd === 'number' &&
      typeof point.forecast_spend_usd === 'number' &&
      point.forecast_available_usd < point.forecast_spend_usd
    ) {
      idxs.push(idx)
    }
  })
  return idxs
})

const eventsByBucket = computed(() => {
  const map = new Map<string, CapacityEvent[]>()
  for (const event of series.value?.events ?? []) {
    const key = truncateHour(event.at)
    const bucketEvents = map.get(key) ?? []
    bucketEvents.push(event)
    map.set(key, bucketEvents)
  }
  return map
})

const chartData = computed<ChartData<'line'> | null>(() => {
  const points = series.value?.points
  if (!points || points.length === 0) return null

  const labels = points.map((p) => formatBucketLabel(p.bucket_start))

  // null = gap (no data / not applicable to this segment); Chart.js skips
  // plotting instead of inventing a value. Solid and dashed datasets both
  // include the "current" bucket's value so the two lines visually meet
  // at the past/future boundary without fabricating data.
  const actualSpend = points.map((p) => (p.segment === 'future' ? null : p.spend_usd))
  const forecastSpend = points.map((p) => {
    if (p.segment === 'past') return null
    if (typeof p.forecast_spend_usd === 'number') return p.forecast_spend_usd
    return p.segment === 'current' ? p.spend_usd : null
  })
  const actualAvailable = points.map((p) => (p.segment === 'future' ? null : p.available_usd))
  const forecastAvailable = points.map((p) => {
    if (p.segment === 'past') return null
    if (typeof p.forecast_available_usd === 'number') return p.forecast_available_usd
    return p.segment === 'current' ? p.available_usd : null
  })

  const blue = '#3b82f6'
  const green = '#10b981'

  const recoveredPointRadius = (ctx: ScriptableContext<'line'>) =>
    (points[ctx.dataIndex]?.recovered_usd ?? 0) > 0 ? 5 : 0

  const datasets: ChartData<'line'>['datasets'] = [
    {
      label: t('admin.accounts.capacityForecast.chartActualSpend'),
      data: actualSpend,
      borderColor: blue,
      backgroundColor: `${blue}20`,
      borderWidth: 2,
      tension: 0.3,
      fill: false,
      pointRadius: 0,
      pointHoverRadius: 4,
      pointHitRadius: 10,
      spanGaps: false,
      order: 1
    },
    {
      label: t('admin.accounts.capacityForecast.chartForecastSpend'),
      data: forecastSpend,
      borderColor: blue,
      backgroundColor: 'transparent',
      borderWidth: 2,
      borderDash: [6, 4],
      tension: 0.3,
      fill: false,
      pointRadius: 0,
      pointHoverRadius: 4,
      pointHitRadius: 10,
      spanGaps: false,
      order: 1
    },
    {
      label: t('admin.accounts.capacityForecast.chartAvailable'),
      data: actualAvailable,
      borderColor: green,
      backgroundColor: `${green}20`,
      borderWidth: 2,
      tension: 0.3,
      fill: false,
      pointRadius: recoveredPointRadius,
      pointHoverRadius: 6,
      pointBackgroundColor: green,
      pointHitRadius: 10,
      spanGaps: false,
      order: 2
    },
    {
      label: t('admin.accounts.capacityForecast.chartForecastAvailable'),
      data: forecastAvailable,
      borderColor: green,
      backgroundColor: 'transparent',
      borderWidth: 2,
      borderDash: [6, 4],
      tension: 0.3,
      fill: false,
      pointRadius: recoveredPointRadius,
      pointHoverRadius: 6,
      pointBackgroundColor: green,
      pointHitRadius: 10,
      spanGaps: false,
      order: 2
    }
  ]

  return { labels, datasets }
})

// Vertical marker at the "now" bucket.
const nowLinePlugin: Plugin<'line'> = {
  id: 'capacityNowLine',
  afterDraw(chart) {
    const idx = currentBucketIndex.value
    if (idx == null) return
    const xScale = chart.scales.x
    const yScale = chart.scales.y
    if (!xScale || !yScale) return
    const x = xScale.getPixelForValue(idx)
    const { ctx } = chart
    ctx.save()
    ctx.beginPath()
    ctx.setLineDash([4, 4])
    ctx.lineWidth = 1.5
    ctx.strokeStyle = isDarkMode.value ? 'rgba(226,232,240,0.55)' : 'rgba(51,65,85,0.55)'
    ctx.moveTo(x, yScale.top)
    ctx.lineTo(x, yScale.bottom)
    ctx.stroke()
    ctx.restore()
  }
}

// Semi-transparent red bands over buckets where the forecast projects a shortfall.
const shortfallBandPlugin: Plugin<'line'> = {
  id: 'capacityShortfallBands',
  beforeDatasetsDraw(chart) {
    const idxs = gapBucketIndexes.value
    if (idxs.length === 0) return
    const xScale = chart.scales.x
    const yScale = chart.scales.y
    if (!xScale || !yScale) return
    const { ctx } = chart
    const p0 = xScale.getPixelForValue(0)
    const p1 = xScale.getPixelForValue(1)
    const diff = Math.abs(p1 - p0)
    const half = Number.isFinite(diff) && diff > 0 ? diff / 2 : 10
    ctx.save()
    ctx.fillStyle = isDarkMode.value ? 'rgba(248,113,113,0.16)' : 'rgba(248,113,113,0.14)'
    for (const idx of idxs) {
      const xCenter = xScale.getPixelForValue(idx)
      ctx.fillRect(xCenter - half, yScale.top, half * 2, yScale.bottom - yScale.top)
    }
    ctx.restore()
  }
}

const chartPlugins = [shortfallBandPlugin, nowLinePlugin]

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
            if (!point || point.recovered_usd <= 0) return []
            const lines: string[] = []
            const recoverEvents = (eventsByBucket.value.get(truncateHour(point.bucket_start)) ?? []).filter(
              (event) => event.type === 'recover'
            )
            if (recoverEvents.length === 0) {
              lines.push(`+${usd(point.recovered_usd)}`)
            }
            for (const event of recoverEvents) {
              lines.push(
                t('admin.accounts.capacityForecast.eventRecoverDetail', {
                  count: event.account_count,
                  window: event.window_kind,
                  amount: usd(event.amount_usd)
                })
              )
            }
            return lines
          }
        }
      }
    },
    scales: {
      x: {
        ticks: { color: text, maxRotation: 0, autoSkip: true, maxTicksLimit: 12, font: { size: 10 } },
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

function eventDetail(event: CapacityEvent): string {
  const key =
    event.type === 'recover'
      ? 'admin.accounts.capacityForecast.eventRecoverDetail'
      : 'admin.accounts.capacityForecast.eventShortfallDetail'
  return t(key, {
    count: event.account_count,
    window: event.window_kind,
    amount: usd(event.amount_usd)
  })
}

function eventRecommendation(event: CapacityEvent): CapacityRecommendation | null {
  if (event.type !== 'shortfall') return null
  const recommendations = series.value?.recommendations ?? []
  if (recommendations.length === 0) return null
  return recommendations.find((rec) => rec.at === event.at) ?? recommendations[0] ?? null
}

// Probe (test accounts) modal state
const showProbeModal = ref(false)
const probeIncludeNormal = ref(false)
const probing = ref(false)
const probeResult = ref<CapacityProbeResult | null>(null)
const probeError = ref('')
const showProbeFailures = ref(false)

function openProbeModal() {
  probeResult.value = null
  probeError.value = ''
  probeIncludeNormal.value = false
  showProbeFailures.value = false
  showProbeModal.value = true
}

function closeProbeModal() {
  if (probing.value) return
  showProbeModal.value = false
}

async function runProbe() {
  probing.value = true
  probeError.value = ''
  probeResult.value = null
  try {
    const result = await adminAPI.capacity.probeCapacity({
      platform: selectedPlatform.value,
      group_id: selectedGroupId.value ?? undefined,
      include_normal: probeIncludeNormal.value
    })
    probeResult.value = result
    await load(true)
  } catch (caught) {
    probeError.value = extractErrorMessage(caught, t('admin.accounts.capacityForecast.probeFailed'))
  } finally {
    probing.value = false
  }
}

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll(selectedPlatform.value as GroupPlatform)
  } catch {
    groups.value = []
  }
}

async function load(force = false) {
  const seq = ++loadSeq
  loading.value = true
  error.value = ''
  try {
    const data = await adminAPI.capacity.getCapacityTimeseries({
      platform: selectedPlatform.value,
      group_id: selectedGroupId.value ?? undefined,
      range: selectedRange.value,
      force
    })
    if (seq !== loadSeq) return
    series.value = data
  } catch (caught) {
    if (seq !== loadSeq) return
    error.value = extractErrorMessage(caught, t('admin.accounts.capacityForecast.loadFailed'))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function extractErrorMessage(caught: unknown, fallback: string): string {
  if (caught && typeof caught === 'object' && 'message' in caught) {
    const message = (caught as { message?: unknown }).message
    if (typeof message === 'string' && message) return message
  }
  if (caught instanceof Error) return caught.message
  return fallback
}

function usd(value: number) {
  return new Intl.NumberFormat(locale.value, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value || 0)
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

function truncateHour(iso: string): string {
  const date = new Date(iso)
  date.setUTCMinutes(0, 0, 0)
  return date.toISOString()
}

watch(
  () => props.show,
  (show) => {
    if (show) {
      void loadGroups()
      void load(false)
    }
  },
  { immediate: true }
)

watch([selectedPlatform, selectedGroupId, selectedRange], (newValues, oldValues) => {
  const [newPlatform] = newValues
  const [oldPlatform] = oldValues
  if (newPlatform !== oldPlatform && selectedGroupId.value !== null) {
    // Platform changed while a group from the old platform was selected: reset it.
    // This re-triggers the watcher (groupId changes), so skip loading on this pass.
    selectedGroupId.value = null
    return
  }
  if (newPlatform !== oldPlatform) {
    void loadGroups()
  }
  if (props.show) void load(false)
})
</script>
