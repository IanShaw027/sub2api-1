<script setup lang="ts">
import { computed } from 'vue'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend } from 'chart.js'
import { Line } from 'vue-chartjs'
import { baseChartOptions, useChartTheme } from '@/utils/chartTheme'
import { chartDataSchema } from '../chatTools'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{ data: unknown }>()
const theme = useChartTheme()
const chart = computed(() => { const parsed = chartDataSchema.safeParse(props.data); return parsed.success ? parsed.data : null })
const chartData = computed(() => ({
  labels: chart.value?.data.map(point => String(point[chart.value!.xKey])) || [],
  datasets: chart.value?.series.map((series, index) => ({
    label: series.name,
    data: chart.value!.data.map(point => Number(point[series.key])),
    borderColor: theme.value.series[index % theme.value.series.length],
    backgroundColor: theme.value.series[index % theme.value.series.length],
    borderWidth: 2, pointRadius: chart.value!.data.length > 50 ? 0 : 3, tension: 0.15,
  })) || [],
}))
const options = computed(() => ({
  ...baseChartOptions(theme.value), responsive: true, maintainAspectRatio: false,
  scales: {
    x: { ticks: { color: theme.value.text }, grid: { color: theme.value.grid }, title: { display: Boolean(chart.value?.xLabel), text: chart.value?.xLabel, color: theme.value.text } },
    y: { ticks: { color: theme.value.text }, grid: { color: theme.value.grid }, title: { display: Boolean(chart.value?.yLabel), text: chart.value?.yLabel, color: theme.value.text } },
  },
}))
</script>

<template>
  <figure v-if="chart" class="local-chat-chart"><figcaption v-if="chart.title">{{ chart.title }}</figcaption><div class="local-chat-chart-canvas"><Line :data="chartData" :options="options" :aria-label="chart.title || 'Chart'" role="img" /></div></figure>
</template>

<style scoped>
.local-chat-chart { width: 100%; min-width: 0; margin: 16px 0; }
.local-chat-chart figcaption { font-size: 13px; color: var(--foreground); font-weight: 600; margin-bottom: 12px; }
.local-chat-chart-canvas { height: 290px; width: 100%; min-width: 0; }
@media (max-width: 600px) { .local-chat-chart-canvas { height: 230px; } }
</style>
