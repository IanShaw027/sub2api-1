<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Doughnut } from 'vue-chartjs'
import type { ErrorDistributionResponse } from '@/api/admin/ops'
import type { ChartState } from '../types'
import { sumNumbers } from '../utils/opsFormatters'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  errorDistribution: ErrorDistributionResponse | null
  loading: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()

const emptyErrorHintText = computed(() => t('admin.ops.charts.emptyError'))

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const colors = {
  blue: '#3b82f6',
  red: '#ef4444',
  orange: '#f59e0b',
  gray: '#9ca3af',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}

interface ErrorCategory {
  label: string
  count: number
  color: string
}

const errorAttribution = computed(() => {
  if (!props.errorDistribution) return []

  let upstream = 0 // 502, 503, 504
  let client = 0 // 4xx
  let system = 0 // 500
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
  return errorAttribution.value.reduce((prev, current) => (prev.count > current.count ? prev : current))
})

const errorTotalCount = computed(() => sumNumbers(props.errorDistribution?.items?.map(i => i.count) ?? []))
const errorChartState = computed<ChartState>(() => {
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
        <svg class="h-4 w-4 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        {{ t('admin.ops.charts.errorDistribution') }}
        <HelpTooltip :content="t('admin.ops.tooltips.errorDistChart')" />
      </h3>
    </div>

    <div class="relative h-64">
      <div v-if="errorChartState === 'ready' && errorChartData" class="flex h-full flex-col">
        <div class="flex-1">
          <Doughnut :data="errorChartData" :options="{ ...baseOptions, cutout: '65%' }" />
        </div>
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
        <div v-if="errorChartState === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('admin.ops.charts.loading') }}</div>
        <EmptyState v-else :title="t('admin.ops.charts.noDataTitle')" :description="emptyErrorHintText" />
      </div>
    </div>
  </div>
</template>

