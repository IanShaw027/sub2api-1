<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar } from 'vue-chartjs'
import type { ProviderHealthData } from '@/api/admin/ops'
import type { ChartState } from '../types'
import { sumNumbers } from '../utils/opsFormatters'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  providers: ProviderHealthData[]
  loading: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()

const emptyRequestHintText = computed(() => t('admin.ops.charts.emptyRequest'))

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const colors = {
  blue: '#3b82f6',
  orange: '#f59e0b',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}

const providerTotalRequests = computed(() => sumNumbers(props.providers.map(p => p.request_count)))
const providerChartState = computed<ChartState>(() => {
  if (props.providers.length > 0 && providerTotalRequests.value > 0) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

const providerChartData = computed(() => {
  if (!props.providers.length || providerTotalRequests.value <= 0) return null
  const sorted = [...props.providers].sort((a, b) => b.request_count - a.request_count)

  return {
    labels: sorted.map(p => p.name),
    datasets: [
      {
        label: t('admin.ops.charts.provider.successRequests'),
        data: sorted.map(p => p.request_count * (1 - p.error_rate / 100)),
        backgroundColor: colors.blue,
        borderRadius: 4,
        barPercentage: 0.6,
        stack: 'total'
      },
      {
        label: t('admin.ops.charts.provider.failedRequests'),
        data: sorted.map(p => p.request_count * (p.error_rate / 100)),
        backgroundColor: colors.orange,
        borderRadius: 4,
        barPercentage: 0.6,
        stack: 'total'
      }
    ]
  }
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

const providerOptions = computed(() => ({
  ...baseOptions.value,
  indexAxis: 'y' as const,
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label || ''
          const value = context.raw || 0
          const total = context.chart.data.datasets.reduce(
            (acc: number, ds: any) => acc + (ds.data[context.dataIndex] || 0),
            0
          )
          const percentage = total > 0 ? ((value / total) * 100).toFixed(1) : 0
          return `${label}: ${Math.round(value)} (${percentage}%)`
        }
      }
    }
  },
  scales: {
    x: {
      beginAtZero: true,
      stacked: true,
      grid: { color: colors.grid, borderDash: [4, 4] },
      ticks: { color: colors.text, font: { size: 10 } }
    },
    y: {
      stacked: true,
      grid: { display: false },
      ticks: { color: colors.text, font: { size: 11, weight: 'bold' as const } }
    }
  }
}))
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
        <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        {{ t('admin.ops.charts.providerErrorRate') }}
        <HelpTooltip :content="t('admin.ops.tooltips.providerChart')" />
      </h3>
    </div>

    <div class="h-48">
      <Bar v-if="providerChartState === 'ready' && providerChartData" :data="providerChartData" :options="providerOptions" />
      <div v-else class="flex h-full items-center justify-center">
        <div v-if="providerChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('admin.ops.charts.loading') }}</div>
        <EmptyState v-else :title="t('admin.ops.charts.noDataTitle')" :description="emptyRequestHintText" />
      </div>
    </div>
  </div>
</template>

