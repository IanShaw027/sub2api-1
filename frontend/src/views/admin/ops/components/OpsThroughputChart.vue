<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Line } from 'vue-chartjs'
import type { ChartComponentRef } from 'vue-chartjs'
import { sumNumbers, formatHistoryLabel } from '../utils/opsFormatters'
import type { ChartState } from '../types'
import type { OpsMetrics } from '@/api/admin/ops'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  metricsHistory: OpsMetrics[]
  loading: boolean
  timeRange: string
}

const props = defineProps<Props>()
const { t } = useI18n()

const emptyRequestHintText = computed(() => t('admin.ops.charts.emptyRequest'))

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const colors = {
  blue: '#3b82f6',
  blueAlpha: '#3b82f620',
  green: '#10b981',
  greenAlpha: '#10b98120',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}

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
  if (throughputChartData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

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
    },
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
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-2">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
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
          :title="t('admin.ops.charts.resetZoomHint')"
          @click="resetThroughputZoom"
        >
          {{ t('admin.ops.charts.resetZoom') }}
        </button>
        <button
          type="button"
          class="inline-flex items-center rounded-lg border border-gray-200 bg-white px-2 py-1 text-[11px] font-semibold text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:bg-dark-800"
          :disabled="throughputChartState !== 'ready'"
          :title="t('admin.ops.charts.downloadHint')"
          @click="downloadThroughputChart"
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
</template>
