<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar, Doughnut, Line } from 'vue-chartjs'
import type { ChartComponentRef } from 'vue-chartjs'
import { sumNumbers, formatHistoryLabel } from '../utils/opsFormatters'
import type { ChartState } from '../types'
import type {
  ProviderHealthData,
  LatencyHistogramResponse,
  ErrorDistributionResponse,
  OpsMetrics,
  OpsDashboardOverview
} from '@/api/admin/ops'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

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

// --- 错误归因逻辑 (Error Attribution) ---
// 将错误码分类为：上游 (502/503)、客户端 (4xx)、系统 (500)
interface ErrorCategory {
  label: string
  count: number
  color: string
}

const errorAttribution = computed(() => {
  if (!props.errorDistribution) return []
  
  let upstream = 0 // 502, 503, 504
  let client = 0   // 4xx
  let system = 0   // 500
  let other = 0

  props.errorDistribution.items.forEach(item => {
    const code = parseInt(item.code)
    if ([502, 503, 504].includes(code)) upstream += item.count
    else if (code >= 400 && code < 500) client += item.count
    else if (code === 500) system += item.count
    else other += item.count
  })

  const result: ErrorCategory[] = []
  if (upstream > 0) result.push({ label: t('admin.ops.charts.attribution.upstream'), count: upstream, color: colors.orange })
  if (client > 0) result.push({ label: t('admin.ops.charts.attribution.client'), count: client, color: colors.blue })
  if (system > 0) result.push({ label: t('admin.ops.charts.attribution.system'), count: system, color: colors.red })
  if (other > 0) result.push({ label: t('admin.ops.charts.attribution.other'), count: other, color: colors.gray })
  
  return result
})

const topErrorReason = computed(() => {
  if (errorAttribution.value.length === 0) return null
  return errorAttribution.value.reduce((prev, current) => (prev.count > current.count) ? prev : current)
})

// Chart Data: Error Distribution (Replaced with Attribution)
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
    labels: errorAttribution.value.map(i => i.label),
    datasets: [
      {
        data: errorAttribution.value.map(i => i.count),
        backgroundColor: errorAttribution.value.map(i => i.color),
        borderWidth: 0
      }
    ]
  }
})

// --- 供应商健康度拆解 (Provider Breakdown) ---
// 堆叠柱状图：绿色代表成功，红色代表失败
const providerTotalRequests = computed(() => sumNumbers(props.providers.map(p => p.request_count)))
const providerChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (props.providers.length > 0 && providerTotalRequests.value > 0) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const providerChartData = computed(() => {
  if (!props.providers.length || providerTotalRequests.value <= 0) return null
  // 按请求量排序，优先看大户
  const sorted = [...props.providers].sort((a, b) => b.request_count - a.request_count)
  
  return {
    labels: sorted.map(p => p.name),
    datasets: [
      {
        label: t('admin.ops.charts.provider.successRequests'),
        data: sorted.map(p => p.request_count * (1 - p.error_rate / 100)), // 估算成功数
        // Use a color-blind friendlier palette (avoid red/green as the only signal).
        backgroundColor: colors.blue,
        borderRadius: 4,
        barPercentage: 0.6,
        stack: 'total'
      },
      {
        label: t('admin.ops.charts.provider.failedRequests'),
        data: sorted.map(p => p.request_count * (p.error_rate / 100)), // 估算失败数
        backgroundColor: colors.orange,
        borderRadius: 4,
        barPercentage: 0.6,
        stack: 'total'
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
    ,
    // Enable x-axis zoom/pan (dataZoom-like) for deeper exploration.
    // - Wheel / pinch zoom on x
    // - Ctrl + drag to pan (prevents accidental page scroll)
    zoom: {
      pan: {
        enabled: true,
        mode: 'x' as const,
        modifierKey: 'ctrl' as const
      },
      zoom: {
        wheel: { enabled: true },
        pinch: { enabled: true },
        mode: 'x' as const
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
  plugins: { 
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label || ''
          const value = context.raw || 0
          // 计算百分比
          const total = context.chart.data.datasets.reduce((acc: number, ds: any) => acc + (ds.data[context.dataIndex] || 0), 0)
          const percentage = total > 0 ? ((value / total) * 100).toFixed(1) : 0
          return `${label}: ${Math.round(value)} (${percentage}%)`
        }
      }
    }
  },
  scales: {
    x: {
      beginAtZero: true,
      stacked: true, // 堆叠
      grid: { color: colors.grid, borderDash: [4, 4] },
      ticks: { color: colors.text, font: { size: 10 } }
    },
    y: {
      stacked: true, // 堆叠
      grid: { display: false },
      ticks: { color: colors.text, font: { size: 11, weight: 'bold' as const } }
    }
  }
}))

const throughputChartRef = ref<ChartComponentRef | null>(null)
function resetThroughputZoom() {
  const chart: any = throughputChartRef.value?.chart
  if (!chart) return
  if (typeof chart.resetZoom === 'function') chart.resetZoom()
}

function downloadThroughputChart() {
  const chart: any = throughputChartRef.value?.chart
  if (!chart || typeof chart.toBase64Image !== 'function') return
  const url = chart.toBase64Image('image/png', 1)
  const a = document.createElement('a')
  a.href = url
  a.download = `ops-throughput-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.png`
  a.click()
}
</script>

<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
    <!-- Row 1: Throughput (2/3) + Error Distribution (1/3) -->
    
    <!-- Throughput Trend -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-2">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
        >
          <svg class="h-4 w-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
          {{ $t('admin.ops.charts.throughput') }}
          <HelpTooltip :content="t('admin.ops.tooltips.throughputChart')" />
        </h3>
        <div class="flex items-center gap-2 text-xs text-gray-500">
          <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-blue-500"></span>QPS</span>
          <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-green-500"></span>TPS(K)</span>
          <button
            type="button"
            class="ml-2 inline-flex items-center rounded-lg border border-gray-200 bg-white px-2 py-1 text-[11px] font-semibold text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:bg-dark-800"
            :disabled="throughputChartState !== 'ready'"
            @click="resetThroughputZoom"
            :title="t('admin.ops.charts.resetZoomHint')"
          >
            {{ t('admin.ops.charts.resetZoom') }}
          </button>
          <button
            type="button"
            class="inline-flex items-center rounded-lg border border-gray-200 bg-white px-2 py-1 text-[11px] font-semibold text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:bg-dark-800"
            :disabled="throughputChartState !== 'ready'"
            @click="downloadThroughputChart"
            :title="t('admin.ops.charts.downloadHint')"
          >
            {{ t('admin.ops.charts.download') }}
          </button>
        </div>
      </div>
      <div class="h-64">
        <Line
          v-if="throughputChartState === 'ready' && throughputChartData"
          ref="throughputChartRef"
          :data="throughputChartData"
          :options="throughputOptions"
        />
        <div v-else class="flex h-full items-center justify-center">
          <div v-if="throughputChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ $t('admin.ops.charts.loading') }}</div>
          <EmptyState
            v-else
            :title="t('admin.ops.charts.noDataTitle')"
            :description="emptyRequestHintText"
          />
        </div>
      </div>
    </div>

    <!-- Error Distribution (Attribution) -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-1">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
        >
          <svg class="h-4 w-4 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          {{ $t('admin.ops.charts.errorDistribution') }}
          <HelpTooltip :content="t('admin.ops.tooltips.errorDistChart')" />
        </h3>
      </div>
      <div class="relative h-64">
        <div v-if="errorChartState === 'ready' && errorChartData" class="flex h-full flex-col">
          <div class="flex-1">
             <Doughnut :data="errorChartData" :options="{ ...baseOptions, cutout: '65%' }" />
          </div>
          <!-- Custom Legend with Diagnosis -->
          <div class="mt-4 flex flex-col items-center gap-2">
            <div v-if="topErrorReason" class="text-xs font-bold text-gray-900 dark:text-white">
              {{ t('admin.ops.labels.topReason') }} <span :style="{ color: topErrorReason.color }">{{ topErrorReason.label }}</span>
            </div>
            <div class="flex flex-wrap justify-center gap-3">
              <div v-for="item in errorAttribution" :key="item.label" class="flex items-center gap-1.5 text-xs">
                <span class="h-2 w-2 rounded-full" :style="{ backgroundColor: item.color }"></span>
                <span class="text-gray-500 dark:text-gray-400">{{ item.count }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="flex h-full items-center justify-center">
           <div v-if="errorChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ $t('admin.ops.charts.loading') }}</div>
           <EmptyState
             v-else
             :title="t('admin.ops.charts.noDataTitle')"
             :description="emptyErrorHintText"
           />
        </div>
      </div>
    </div>

    <!-- Row 2: Latency & Provider Health -->

    <!-- Latency Histogram -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-1">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
        >
          <svg class="h-4 w-4 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {{ $t('admin.ops.charts.latency') }}
          <HelpTooltip :content="t('admin.ops.tooltips.latencyChart')" />
        </h3>
      </div>
      <div class="h-48">
        <Bar v-if="latencyChartState === 'ready' && latencyChartData" :data="latencyChartData" :options="baseOptions" />
        <div v-else class="flex h-full items-center justify-center">
          <div v-if="latencyChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('admin.ops.charts.loading') }}</div>
          <EmptyState
            v-else
            :title="t('admin.ops.charts.noDataTitle')"
            :description="emptyRequestHintText"
          />
        </div>
      </div>
    </div>

    <!-- Provider Health (Breakdown) -->
    <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-2">
      <div class="mb-4 flex items-center justify-between">
        <h3 
          class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white"
        >
          <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {{ $t('admin.ops.charts.providerErrorRate') }}
          <HelpTooltip :content="t('admin.ops.tooltips.providerChart')" />
        </h3>
      </div>
      <div class="h-48">
        <Bar v-if="providerChartState === 'ready' && providerChartData" :data="providerChartData" :options="providerOptions" />
        <div v-else class="flex h-full items-center justify-center">
          <div v-if="providerChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('admin.ops.charts.loading') }}</div>
          <EmptyState
            v-else
            :title="t('admin.ops.charts.noDataTitle')"
            :description="emptyRequestHintText"
          />
        </div>
      </div>
    </div>
  </div>
</template>
