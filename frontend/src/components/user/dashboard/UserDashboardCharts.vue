<template>
 <div class="dash-charts-row">
 <GlassCard class="dash-chart-card" padding="md">
 <div class="dash-chart-header">
 <div class="dash-chart-heading">
 <span class="dash-chart-title">{{ t('dashboard.tokenUsageTrend') }}</span>
 <span class="dash-chart-sub">{{ trendRangeLabel }}</span>
 </div>
 <SegmentedControl
 class="segmented-sm"
 :model-value="granularity"
 :options="granularityOptions"
 @update:model-value="onGranularityChange"
 />
 </div>
 <div class="dash-chart-body">
 <TokenUsageTrend :trend-data="trend" :loading="loading" bare />
 </div>
 </GlassCard>

 <GlassCard class="dash-dist-card" padding="md">
 <div class="dash-chart-header">
 <div class="dash-chart-heading">
 <span class="dash-chart-title">{{ t('dashboard.modelDistribution') }}</span>
 <span class="dash-chart-sub">{{ t('dashboard.last7Days') }}</span>
 </div>
 </div>
 <div v-if="loading" class="dash-dist-loading"><LoadingSpinner size="md" /></div>
 <div v-else-if="modelRows.length === 0" class="dash-dist-empty">{{ t('dashboard.noDataAvailable') }}</div>
 <div v-else class="dash-dist-list">
 <div v-for="row in modelRows" :key="row.model" class="dash-dist-row">
 <div class="dash-dist-row-top">
 <span class="dash-dist-name">{{ row.model }}</span>
 <span class="dash-dist-meta">${{ formatCost(row.actual_cost) }} · {{ row.pct.toFixed(1) }}%</span>
 </div>
 <div class="progress">
 <div class="progress-bar" :style="{ width: `${row.pct}%` }"></div>
 </div>
 </div>
 </div>
 </GlassCard>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { TrendDataPoint, ModelStat } from '@/types'
import { formatCostFixed as formatCost } from '@/utils/format'

const props = defineProps<{ loading: boolean, granularity: string, trend: TrendDataPoint[], models: ModelStat[] }>()
const emit = defineEmits<{
 'update:granularity': [value: string]
 granularityChange: []
}>()
const { t } = useI18n()

const granularityOptions = computed(() => [
 { value: 'day', label: t('dashboard.day') },
 { value: 'hour', label: t('dashboard.hour') }
])

function onGranularityChange(value: string) {
 emit('update:granularity', value)
 emit('granularityChange')
}

const trendRangeLabel = computed(() => {
 const count = props.trend?.length || 0
 return count > 0 ? `${count} · ${t('dashboard.recentUsage')}` : t('dashboard.tokenUsageTrend')
})

const modelRows = computed(() => {
 const list = props.models ?? []
 if (!list.length) return []
 const maxCost = Math.max(...list.map((m) => m.actual_cost), 0.0001)
 return [...list]
 .sort((a, b) => b.actual_cost - a.actual_cost)
 .map((m) => ({
 model: m.model,
 actual_cost: m.actual_cost,
 pct: maxCost > 0 ? Math.min(100, (m.actual_cost / maxCost) * 100) : 0
 }))
})
</script>
<style scoped>
.dash-charts-row {
 display: grid;
 grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
 gap: 12px;
 align-items: start;
}
.dash-chart-card,
.dash-dist-card {
 min-width: 0;
 padding: 16px 18px !important;
 display: flex;
 flex-direction: column;
 gap: 14px;
}
.dash-chart-header {
 display: flex;
 align-items: center;
 justify-content: space-between;
 gap: 12px;
}
.dash-chart-heading {
 display: flex;
 flex-direction: column;
 gap: 2px;
 min-width: 0;
}
.dash-chart-title {
 font-size: 14px;
 font-weight: 600;
 color: var(--foreground);
}
.dash-chart-sub {
 font-size: 12px;
 color: var(--muted);
}
.dash-chart-body {
 flex: 1;
 min-height: 0;
}
.dash-dist-loading,
.dash-dist-empty {
 display: flex;
 align-items: center;
 justify-content: center;
 height: 180px;
 font-size: 13px;
 color: var(--muted);
}
.dash-dist-list {
 display: flex;
 flex-direction: column;
 gap: 10px;
 max-height: 260px;
 overflow-y: auto;
}
.dash-dist-row {
 display: flex;
 flex-direction: column;
 gap: 5px;
}
.dash-dist-row-top {
 display: flex;
 align-items: baseline;
 justify-content: space-between;
 gap: 8px;
}
.dash-dist-name {
 font-family: var(--font-mono);
 font-size: 12px;
 color: var(--foreground);
 overflow: hidden;
 text-overflow: ellipsis;
 white-space: nowrap;
}
.dash-dist-meta {
 flex: none;
 font-size: 12px;
 color: var(--muted);
 font-variant-numeric: tabular-nums;
}

@media (max-width: 1023px) {
 .dash-charts-row {
 grid-template-columns: minmax(0, 1fr);
 }
}
</style>
