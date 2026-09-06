<template>
  <div class="glass-card p-4">
    <h3 class="mb-4 text-sm font-semibold text-foreground">
      {{ t('payment.admin.dailyRevenue') }}
    </h3>
    <div class="h-64">
      <div v-if="loading" class="flex h-full items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <Line v-else-if="chartData" :data="chartData" :options="chartOptions" />
      <div
        v-else
        class="flex h-full items-center justify-center text-sm text-muted"
      >
        {{ t('payment.admin.noData') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
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
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { DailyPaymentStats } from '@/types/payment'
import { alpha, baseChartOptions, useChartTheme } from '@/utils/chartTheme'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const theme = useChartTheme()

const props = defineProps<{
  data: DailyPaymentStats[]
  loading?: boolean
}>()

const chartData = computed(() => {
  if (!props.data || props.data.length === 0) return null
  const currencies = [...new Set(props.data.flatMap(day => Object.keys(day.amount)))].sort()
  const series = theme.value.series
  return {
    labels: props.data.map(d => d.date),
    datasets: [
      ...currencies.map((currency, index) => {
        const color = series[index % series.length]
        return {
          label: `${currency} ${t('payment.admin.revenue')}`,
          data: props.data.map(day => day.amount[currency] || 0),
          borderColor: color,
          backgroundColor: alpha(color, 12),
          fill: true,
          tension: 0.3,
          pointRadius: 3,
          pointHoverRadius: 5,
        }
      }),
      {
        label: t('payment.admin.orderCount'),
        data: props.data.map(d => d.count),
        borderColor: theme.value.success,
        backgroundColor: alpha(theme.value.success, 12),
        fill: false,
        tension: 0.3,
        pointRadius: 3,
        pointHoverRadius: 5,
        yAxisID: 'y1',
      }
    ]
  }
})

const chartOptions = computed(() => {
  const base = baseChartOptions(theme.value)
  return {
    ...base,
    interaction: { mode: 'index' as const, intersect: false },
    scales: {
      x: base.scales.x,
      y: {
        ...base.scales.y,
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        title: { display: true, text: t('payment.admin.revenue'), color: theme.value.text },
      },
      y1: {
        type: 'linear' as const,
        display: true,
        position: 'right' as const,
        title: { display: true, text: t('payment.admin.orderCount'), color: theme.value.text },
        grid: { drawOnChartArea: false },
        border: { display: false },
        ticks: { color: theme.value.text, font: { family: theme.value.font.family, size: theme.value.font.size } },
      }
    },
    plugins: {
      ...base.plugins,
      legend: { ...base.plugins.legend, position: 'top' as const },
    }
  }
})
</script>
