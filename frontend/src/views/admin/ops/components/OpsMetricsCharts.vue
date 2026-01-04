<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar, Doughnut, Line } from 'vue-chartjs'
import { sumNumbers, formatHistoryLabel } from '../utils/opsFormatters'
import type { ChartState } from '../types'
import type {
  ProviderHealthData,
  LatencyHistogramResponse,
  ErrorDistributionResponse,
  OpsMetrics,
  OpsDashboardOverview
} from '@/api/admin/ops'

interface Props {
  hasLoadedOnce: boolean
  loading: boolean
  timeRange: string
  providers: ProviderHealthData[]
  latencyData: LatencyHistogramResponse | null
  errorDistribution: ErrorDistributionResponse | null
  latestMetrics: OpsMetrics | null
  metricsHistory: OpsMetrics[]
  overview: OpsDashboardOverview | null
}

const props = defineProps<Props>()
const { t } = useI18n()

const emptyRequestHintText = computed(() => t('admin.ops.charts.emptyRequest'))
const emptyErrorHintText = computed(() => t('admin.ops.charts.emptyError'))

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const colors = {
  blue: '#3b82f6',
  blueAlpha: '#3b82f620',
  green: '#10b981',
  greenAlpha: '#10b98120',
  red: '#ef4444',
  orange: '#f59e0b',
  purple: '#8b5cf6',
  gray: '#9ca3af',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}

// Chart Data: Latency Distribution
const latencyHasData = computed(() => (props.latencyData?.total_requests ?? 0) > 0)
const latencyChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (latencyHasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const latencyChartData = computed(() => {
  if (!props.latencyData || !latencyHasData.value) return null
  return {
    labels: props.latencyData.buckets.map(b => b.range),
    datasets: [
      {
        label: t('admin.ops.charts.requestCountLabel'),
        data: props.latencyData.buckets.map(b => b.count),
        backgroundColor: colors.blue,
        borderRadius: 4,
        barPercentage: 0.6
      }
    ]
  }
})

// Chart Data: Error Distribution
const errorTotalCount = computed(() => sumNumbers(props.errorDistribution?.items?.map(i => i.count) ?? []))
const errorChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (errorTotalCount.value > 0) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const errorChartData = computed(() => {
  if (!props.errorDistribution || errorTotalCount.value <= 0) return null
  return {
    labels: props.errorDistribution.items.map(i => i.code),
    datasets: [
      {
        data: props.errorDistribution.items.map(i => i.count),
        backgroundColor: [
          '#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899'
        ],
        borderWidth: 0
      }
    ]
  }
})

// Chart Data: Provider SLA (Horizontal Bar)
const providerTotalRequests = computed(() => sumNumbers(props.providers.map(p => p.request_count)))
const providerChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (props.providers.length > 0 && providerTotalRequests.value > 0) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const providerChartData = computed(() => {
  if (!props.providers.length || providerTotalRequests.value <= 0) return null
  // Sort providers by error rate (desc) to highlight issues
  const sorted = [...props.providers].sort((a, b) => a.error_rate - b.error_rate)
  return {
    labels: sorted.map(p => p.name),
    datasets: [
      {
        label: t('admin.ops.charts.errorRateLabel'),
        data: sorted.map(p => p.error_rate),
        backgroundColor: sorted.map(p => p.error_rate > 5 ? colors.red : p.error_rate > 1 ? colors.orange : colors.green),
        borderRadius: 4,
        barPercentage: 0.6
      }
    ]
  }
})

// Chart Data: Throughput Trend
const throughputChartData = computed(() => {
  const totalRequests = sumNumbers(props.metricsHistory.map(m => m.request_count))
  if (!props.metricsHistory.length || totalRequests <= 0) return null
  return {
    labels: props.metricsHistory.map(m => formatHistoryLabel(m.updated_at, props.timeRange)),
    datasets: [
      {
        label: 'QPS',
        data: props.metricsHistory.map(m => m.qps ?? 0),
        borderColor: colors.blue,
        backgroundColor: colors.blueAlpha,
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        pointHitRadius: 10
      },
      {
        label: 'TPS (K)',
        data: props.metricsHistory.map(m => (m.tps ?? 0) / 1000),
        borderColor: colors.green,
        backgroundColor: colors.greenAlpha,
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        pointHitRadius: 10,
        yAxisID: 'y1'
      }
    ]
  }
})

const throughputChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (throughputChartData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

// Common Options
const baseOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: colors.text, font: { size: 10 } }
    },
    y: {
      beginAtZero: true,
      grid: { color: colors.grid, borderDash: [4, 4] },
      ticks: { color: colors.text, font: { size: 10 } }
    }
  }
}))

const throughputOptions = computed(() => ({
  ...baseOptions.value,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'end' as const,
      labels: { color: colors.text, usePointStyle: true, boxWidth: 6, font: { size: 10 } }
    },
    tooltip: {
      backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
      titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
      bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
      borderColor: colors.grid,
      borderWidth: 1,
      padding: 10,
      displayColors: true,
      callbacks: {
        label: (context: any) => {
           let label = context.dataset.label || ''
           if (label) label += ': '
           if (context.raw !== null) {
              label += context.parsed.y.toFixed(1)
           }
           return label
        }
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: colors.text, font: { size: 10 }, maxTicksLimit: 8 }
    },
    y: {
      type: 'linear' as const,
      display: true,
      position: 'left' as const,
      grid: { color: colors.grid, borderDash: [4, 4] },
      ticks: { color: colors.text, font: { size: 10 } }
    },
    y1: {
      type: 'linear' as const,
      display: true,
      position: 'right' as const,
      grid: { display: false },
      ticks: { color: colors.green, font: { size: 10 } }
    }
  }
}))

const providerOptions = computed(() => ({
  ...baseOptions.value,
  indexAxis: 'y' as const, // Horizontal Bar
  plugins: { legend: { display: false } },
  scales: {
    x: {
      beginAtZero: true,
      grid: { color: colors.grid, borderDash: [4, 4] },
      ticks: { color: colors.text, font: { size: 10 } }
    },
    y: {
      grid: { display: false },
      ticks: { color: colors.text, font: { size: 11, weight: 'bold' as const } }
    }
  }
}))
</script>

<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
    <!-- Row 1: Throughput (2/3) + Error Distribution (1/3) -->
    
    <!-- Throughput Trend -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-2">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
          :title="$t('admin.ops.tooltips.throughputChart')"
        >
          <svg class="h-4 w-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
          {{ $t('admin.ops.charts.throughput') }}
        </h3>
        <div class="flex items-center gap-2 text-xs text-gray-500">
          <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-blue-500"></span>QPS</span>
          <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-green-500"></span>TPS(K)</span>
        </div>
      </div>
      <div class="h-64">
        <Line v-if="throughputChartState === 'ready' && throughputChartData" :data="throughputChartData" :options="throughputOptions" />
        <div v-else class="flex h-full flex-col items-center justify-center gap-2 text-gray-400">
          <div v-if="throughputChartState === 'loading'" class="animate-pulse text-sm">{{ $t('admin.ops.charts.loading') }}</div>
          <div v-else class="text-sm">{{ emptyRequestHintText }}</div>
        </div>
      </div>
    </div>

    <!-- Error Distribution -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-1">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
          :title="$t('admin.ops.tooltips.errorDistChart')"
        >
          <svg class="h-4 w-4 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          {{ $t('admin.ops.charts.errorDistribution') }}
        </h3>
      </div>
      <div class="relative h-64">
        <div v-if="errorChartState === 'ready' && errorChartData" class="flex h-full flex-col">
          <div class="flex-1">
             <Doughnut :data="errorChartData" :options="{ ...baseOptions, cutout: '65%' }" />
          </div>
          <!-- Custom Legend -->
          <div class="mt-4 flex flex-wrap justify-center gap-3">
            <div v-for="(item, idx) in errorDistribution?.items.slice(0, 4)" :key="item.code" class="flex items-center gap-1.5 text-xs">
              <span class="h-2 w-2 rounded-full" :style="{ backgroundColor: ['#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6'][idx] }"></span>
              <span class="font-medium text-gray-700 dark:text-gray-300">{{ item.code }}</span>
              <span class="text-gray-400">{{ item.percentage }}%</span>
            </div>
          </div>
        </div>
        <div v-else class="flex h-full flex-col items-center justify-center gap-2 text-gray-400">
           <div v-if="errorChartState === 'loading'" class="animate-pulse text-sm">{{ $t('admin.ops.charts.loading') }}</div>
           <div v-else class="text-sm">{{ emptyErrorHintText }}</div>
        </div>
      </div>
    </div>

    <!-- Row 2: Latency & Provider Health -->

    <!-- Latency Histogram -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-1">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
          :title="$t('admin.ops.tooltips.latencyChart')"
        >
          <svg class="h-4 w-4 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {{ $t('admin.ops.charts.latency') }}
        </h3>
      </div>
      <div class="h-48">
        <Bar v-if="latencyChartState === 'ready' && latencyChartData" :data="latencyChartData" :options="baseOptions" />
        <div v-else class="flex h-full items-center justify-center text-sm text-gray-400">
           {{ latencyChartState === 'loading' ? 'Loading...' : emptyRequestHintText }}
        </div>
      </div>
    </div>

    <!-- Provider Health (Horizontal Bar) -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-2">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
          :title="$t('admin.ops.tooltips.providerChart')"
        >
          <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {{ $t('admin.ops.charts.providerErrorRate') }}
        </h3>
      </div>
      <div class="h-48">
        <Bar v-if="providerChartState === 'ready' && providerChartData" :data="providerChartData" :options="providerOptions" />
        <div v-else class="flex h-full items-center justify-center text-sm text-gray-400">
           {{ providerChartState === 'loading' ? 'Loading...' : emptyRequestHintText }}
        </div>
      </div>
    </div>
  </div>
</template>
