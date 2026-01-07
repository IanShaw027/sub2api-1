<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar } from 'vue-chartjs'
import type { LatencyHistogramResponse } from '@/api/admin/ops'
import type { ChartState } from '../types'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  latencyData: LatencyHistogramResponse | null
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
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}

const latencyHasData = computed(() => (props.latencyData?.total_requests ?? 0) > 0)
const latencyChartState = computed<ChartState>(() => {
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
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
        <svg class="h-4 w-4 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        {{ t('admin.ops.charts.latency') }}
        <HelpTooltip :content="t('admin.ops.tooltips.latencyChart')" />
      </h3>
    </div>
    <div class="h-48">
      <Bar v-if="latencyChartState === 'ready' && latencyChartData" :data="latencyChartData" :options="baseOptions" />
      <div v-else class="flex h-full items-center justify-center">
        <div v-if="latencyChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('admin.ops.charts.loading') }}</div>
        <EmptyState v-else :title="t('admin.ops.charts.noDataTitle')" :description="emptyRequestHintText" />
      </div>
    </div>
  </div>
</template>

