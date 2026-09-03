<template>
 <div :class="bare ? '' : 'glass-card p-4'">
 <h3 v-if="!bare" class="mb-4 text-sm font-semibold text-foreground">
 {{ t('admin.dashboard.tokenUsageTrend') }}
 </h3>
 <div v-if="loading" class="flex h-48 items-center justify-center">
 <LoadingSpinner />
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
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'
import { useTheme } from '@/composables/useTheme'

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

const { isDark: isDarkMode } = useTheme()

// Resolve design tokens to concrete colour strings Chart.js/canvas can use.
// Recomputes whenever the theme toggles (isDarkMode is read so it stays reactive).
function cssVar(name: string, fallback: string): string {
 if (typeof window === 'undefined') return fallback
 const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
 return value || fallback
}

const chartColors = computed(() => {
 void isDarkMode.value
 return {
 text: cssVar('--muted', '#6b7280'),
 grid: cssVar('--border', '#e5e7eb'),
 input: cssVar('--accent', '#3b82f6'),
 output: cssVar('--success', '#10b981'),
 cacheCreation: cssVar('--warning', '#f59e0b'),
 cacheRead: cssVar('--accent', '#06b6d4'),
 cacheHitRate: cssVar('--danger', '#8b5cf6')
 }
})

function alpha(color: string, pct: number): string {
 return `color-mix(in oklch, ${color} ${pct}%, transparent)`
}

const chartData = computed(() => {
 if (!props.trendData?.length) return null

 return {
 labels: props.trendData.map((d) => d.date),
 datasets: [
 {
 label: 'Input',
 data: props.trendData.map((d) => d.input_tokens),
 borderColor: chartColors.value.input,
 backgroundColor: alpha(chartColors.value.input, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: 'Output',
 data: props.trendData.map((d) => d.output_tokens),
 borderColor: chartColors.value.output,
 backgroundColor: alpha(chartColors.value.output, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: 'Cache Creation',
 data: props.trendData.map((d) => d.cache_creation_tokens),
 borderColor: chartColors.value.cacheCreation,
 backgroundColor: alpha(chartColors.value.cacheCreation, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: 'Cache Read',
 data: props.trendData.map((d) => d.cache_read_tokens),
 borderColor: chartColors.value.cacheRead,
 backgroundColor: alpha(chartColors.value.cacheRead, 16),
 fill: true,
 tension: 0.3
 },
 {
 label: 'Cache Hit Rate',
 data: props.trendData.map((d) => {
 const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
 return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
 }),
 borderColor: chartColors.value.cacheHitRate,
 backgroundColor: alpha(chartColors.value.cacheHitRate, 16),
 borderDash: [5, 5],
 fill: false,
 tension: 0.3,
 yAxisID: 'yPercent'
 }
 ]
 }
})

const lineOptions = computed(() => ({
 responsive: true,
 maintainAspectRatio: false,
 interaction: {
 intersect: false,
 mode: 'index' as const
 },
 plugins: {
 legend: {
 position: 'top' as const,
 labels: {
 color: chartColors.value.text,
 usePointStyle: true,
 pointStyle: 'circle',
 padding: 15,
 font: {
 size: 10.5
 }
 }
 },
 tooltip: {
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
 x: {
 grid: {
 color: chartColors.value.grid
 },
 ticks: {
 color: chartColors.value.text,
 font: {
 size: 10.5
 }
 }
 },
 y: {
 grid: {
 color: chartColors.value.grid
 },
 ticks: {
 color: chartColors.value.text,
 font: {
 size: 10.5
 },
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
 ticks: {
 color: chartColors.value.cacheHitRate,
 font: {
 size: 10.5
 },
 callback: (value: string | number) => `${value}%`
 }
 }
 }
}))

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
