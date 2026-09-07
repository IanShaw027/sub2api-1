<template>
 <div :class="bare ? '' : 'glass-card p-4'">
 <div v-if="!bare" class="mb-4 flex flex-wrap items-start justify-between gap-3">
   <div class="min-w-0 flex-1 flex-col gap-0.5">
     <h3 class="text-sm font-semibold text-foreground">{{ t('admin.dashboard.tokenUsageTrend') }}</h3>
     <span class="text-xs text-muted">{{ trendSubtitle }}</span>
   </div>
 </div>
 <div v-if="!loading && trendData.length" class="mb-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
   <div v-for="item in trendKpis" :key="item.label" class="trend-kpi" :style="{ '--kpi-color': item.color }">
     <span class="trend-kpi-label"><span class="trend-kpi-dot" />{{ item.label }}</span>
     <span class="trend-kpi-value">{{ item.value }}</span>
   </div>
 </div>
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
import { alpha, baseChartOptions, cssVar, useChartTheme } from '@/utils/chartTheme'

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

const trendAnnotationPlugin = {
 id: 'sub2apiTrendAnnotations',
 afterDraw(chart: any) {
  const { ctx, chartArea, tooltip } = chart
  if (!chartArea) return

  const active = tooltip?.getActiveElements?.()?.[0]
  if (active) {
   const x = active.element.x
   ctx.save()
   ctx.strokeStyle = alpha(cssVar('--muted'), 42)
   ctx.setLineDash([4, 4])
   ctx.beginPath()
   ctx.moveTo(x, chartArea.top)
   ctx.lineTo(x, chartArea.bottom)
   ctx.stroke()
   ctx.restore()
  }

  chart.data.datasets.forEach((dataset: any, datasetIndex: number) => {
   const meta = chart.getDatasetMeta(datasetIndex)
   const point = meta?.data?.[meta.data.length - 1]
   if (!point || meta.hidden || dataset.hidden) return
   const raw = dataset.data?.[dataset.data.length - 1]
   if (raw === null || raw === undefined) return
   const value = dataset.yAxisID === 'yPercent' ? `${Number(raw).toFixed(1)}%` : formatTokens(Number(raw))
   const color = dataset.borderColor || cssVar('--accent')
   ctx.save()
   ctx.font = '10px Inter, sans-serif'
   const width = ctx.measureText(value).width + 10
   const x = Math.min(point.x + 6, chartArea.right - width)
   const offset = (datasetIndex - (chart.data.datasets.length - 1) / 2) * 14
   const y = Math.min(chartArea.bottom - 18, Math.max(chartArea.top + 2, point.y - 18 + offset))
   ctx.fillStyle = color
   ctx.beginPath()
   ctx.roundRect(x, y, width, 16, 5)
   ctx.fill()
   ctx.fillStyle = cssVar('--surface')
   ctx.textBaseline = 'middle'
   ctx.fillText(value, x + 5, y + 8)
   ctx.restore()
  })
 }
}

ChartJS.register(trendAnnotationPlugin)

const { t } = useI18n()

const props = defineProps<{
 trendData: TrendDataPoint[]
 loading?: boolean
 /** Render just the chart, without the self-contained card + title (parent supplies its own header). */
 bare?: boolean
}>()

const theme = useChartTheme()

const trendSubtitle = computed(() => {
 const rows = props.trendData ?? []
 if (rows.length < 2) return t('admin.dashboard.tokenUsageTrend')
 return `${t('admin.dashboard.last7Days')} · ${rows[0].date} – ${rows[rows.length - 1].date}`
})

const trendKpis = computed(() => {
 const rows = props.trendData ?? []
 const sum = (key: 'input_tokens' | 'output_tokens' | 'cache_read_tokens') => rows.reduce((total, row) => total + Number(row[key] || 0), 0)
 const hitTokens = sum('cache_read_tokens')
 const promptTokens = rows.reduce((total, row) => total + Number(row.input_tokens || 0) + Number(row.cache_read_tokens || 0) + Number(row.cache_creation_tokens || 0), 0)
 return [
   { label: t('admin.dashboard.input'), value: formatTokens(sum('input_tokens')), color: theme.value.series[0] },
   { label: t('admin.dashboard.output'), value: formatTokens(sum('output_tokens')), color: theme.value.series[1] },
   { label: t('admin.dashboard.trendCacheRead'), value: formatTokens(hitTokens), color: theme.value.series[3] },
   { label: t('admin.dashboard.trendCacheHitRate'), value: `${promptTokens ? ((hitTokens / promptTokens) * 100).toFixed(1) : '0.0'}%`, color: theme.value.danger }
 ]
})

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

<style scoped>
.trend-kpi {
 display: flex;
 min-width: 0;
 flex-direction: column;
 gap: 2px;
 border-radius: 10px;
 background: var(--surface-2);
 padding: 8px 10px;
}
.trend-kpi-label { display: inline-flex; align-items: center; gap: 6px; color: var(--muted); font-size: 11px; }
.trend-kpi-dot { width: 6px; height: 6px; flex: none; border-radius: 999px; background: var(--kpi-color, var(--accent)); }
.trend-kpi-value { color: var(--foreground); font-family: var(--font-display, Inter, sans-serif); font-size: 17px; font-weight: 800; font-variant-numeric: tabular-nums; }
</style>
