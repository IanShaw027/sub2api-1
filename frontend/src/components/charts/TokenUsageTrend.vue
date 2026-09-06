<template>
 <div :class="bare ? '' : 'glass-card p-4'">
 <h3 v-if="!bare" class="mb-4 text-sm font-semibold text-foreground">
 {{ t('admin.dashboard.tokenUsageTrend') }}
 </h3>
 <div v-if="loading" class="h-48" role="status" :aria-label="t('common.loading')" aria-busy="true">
 <Skeleton height="100%" />
 </div>
 <div v-else-if="trendData.length > 0 && chartData" class="h-48">
 <Line :data="chartData" :options="lineOptions" />
 </div>
 <div
 v-else
 class="flex h-48 items-center justify-center text-sm text-muted"
 >
 {{ t('admin.dashboard.noDataAvailable') }}
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
 Title,
 Tooltip,
 Legend,
 Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import Skeleton from '@/components/common/Skeleton.vue'
import type { TrendDataPoint } from '@/types'
import { alpha, baseChartOptions, useChartTheme } from '@/utils/chartTheme'

ChartJS.register(
 CategoryScale,
 LinearScale,
 PointElement,
 LineElement,
 Title,
 Tooltip,
 Legend,
 Filler
)

const { t } = useI18n()

const props = defineProps<{
 trendData: TrendDataPoint[]
 loading?: boolean
 /** Render just the chart, without the self-contained card + title (parent supplies its own header). */
 bare?: boolean
}>()

const theme = useChartTheme()

const chartData = computed(() => {
 if (!props.trendData?.length) return null

 const series = theme.value.series
 const input = series[0]
 const output = series[1]
 const cacheCreation = series[2]
 const cacheRead = series[3]
 const cacheHitRate = theme.value.danger

 return {
 labels: props.trendData.map((d) => d.date),
 datasets: [
 {
 label: t('admin.dashboard.input'),
 data: props.trendData.map((d) => d.input_tokens),
 borderColor: input,
 backgroundColor: alpha(input, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: t('admin.dashboard.output'),
 data: props.trendData.map((d) => d.output_tokens),
 borderColor: output,
 backgroundColor: alpha(output, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: t('admin.dashboard.trendCacheCreation'),
 data: props.trendData.map((d) => d.cache_creation_tokens),
 borderColor: cacheCreation,
 backgroundColor: alpha(cacheCreation, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: t('admin.dashboard.trendCacheRead'),
 data: props.trendData.map((d) => d.cache_read_tokens),
 borderColor: cacheRead,
 backgroundColor: alpha(cacheRead, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: t('admin.dashboard.trendCacheHitRate'),
 data: props.trendData.map((d) => {
 const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
 return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
 }),
 borderColor: cacheHitRate,
 backgroundColor: alpha(cacheHitRate, 16),
 borderDash: [5, 5],
 fill: false,
 tension: 0.3,
 yAxisID: 'yPercent'
 }
 ]
 }
})

const lineOptions = computed(() => {
 const base = baseChartOptions(theme.value)
 const legendFont = { ...base.plugins.legend.labels.font, size: 12 }
 return {
 ...base,
 interaction: {
 intersect: false,
 mode: 'index' as const
 },
 plugins: {
 ...base.plugins,
 legend: {
 ...base.plugins.legend,
 position: 'top' as const,
 labels: {
 ...base.plugins.legend.labels,
 pointStyle: 'circle',
 padding: 15,
 font: legendFont
 }
 },
 tooltip: {
 ...base.plugins.tooltip,
 callbacks: {
 label: (context: any) => {
 if (context.dataset.yAxisID === 'yPercent') {
 return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
 }
 return `${context.dataset.label}: ${formatTokens(context.raw)}`
 },
 footer: (tooltipItems: any) => {
 const dataIndex = tooltipItems[0]?.dataIndex
 if (dataIndex !== undefined && props.trendData[dataIndex]) {
 const data = props.trendData[dataIndex]
 return `Actual: $${formatCost(data.actual_cost)} | Standard: $${formatCost(data.cost)}`
 }
 return ''
 }
 }
 }
 },
 scales: {
 x: base.scales.x,
 y: {
 ...base.scales.y,
 ticks: {
 ...base.scales.y.ticks,
 callback: (value: string | number) => formatTokens(Number(value))
 }
 },
 yPercent: {
 position: 'right' as const,
 min: 0,
 max: 100,
 grid: {
 drawOnChartArea: false
 },
 border: { display: false },
 ticks: {
 color: theme.value.danger,
 font: base.scales.y.ticks.font,
 callback: (value: string | number) => `${value}%`
 }
 }
 }
 }
})

const formatTokens = (value: number): string => {
 if (value >= 1_000_000_000) {
 return `${(value / 1_000_000_000).toFixed(2)}B`
 } else if (value >= 1_000_000) {
 return `${(value / 1_000_000).toFixed(2)}M`
 } else if (value >= 1_000) {
 return `${(value / 1_000).toFixed(2)}K`
 }
 return value.toLocaleString()
}

const formatCost = (value: number): string => {
 if (value >= 1000) {
 return (value / 1000).toFixed(2) + 'K'
 } else if (value >= 1) {
 return value.toFixed(2)
 } else if (value >= 0.01) {
 return value.toFixed(3)
 }
 return value.toFixed(4)
}
</script>
