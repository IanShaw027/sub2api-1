<template>
 <GlassCard padding="md" class="dash-list-card">
 <div class="card-header dash-list-header">
 <span class="card-title">{{ t('dashboard.recentUsage') }}</span>
 <span class="card-subtitle">{{ t('dashboard.last7Days') }}</span>
 </div>
 <div v-if="loading" class="dash-list-loading">
 <LoadingSpinner size="lg" />
 </div>
 <div v-else-if="data.length === 0" class="dash-list-empty">
 <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
 </div>
 <div v-else class="dash-usage-rows">
 <div v-for="log in data" :key="log.id" class="dash-usage-row">
 <div class="dash-usage-row-left">
 <span class="dash-brand-tile" :style="{ background: platformTileBackground(platformFromModel(log.model)) }">
 {{ log.model?.slice(0, 1).toUpperCase() || '?' }}
 </span>
 <div class="min-w-0">
 <p class="dash-usage-model">{{ log.model }}</p>
 <p class="dash-usage-time">{{ formatDateTime(log.created_at) }}</p>
 </div>
 </div>
 <div class="dash-usage-row-right">
 <p class="dash-usage-cost">
 <span class="dash-usage-actual" :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>
 <span class="dash-usage-standard" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
 </p>
 <p class="dash-usage-tokens">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
 </div>
 </div>

 <router-link to="/usage" class="dash-usage-viewall">
 {{ t('dashboard.viewAllUsage') }}
 <Icon name="arrowRight" size="sm" />
 </router-link>
 </div>
 </GlassCard>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import { formatDateTime } from '@/utils/format'
import { platformTileBackground, platformFromModel } from '@/utils/platformTile'
import type { UsageLog } from '@/types'

defineProps<{
 data: UsageLog[]
 loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>
<style scoped>
.dash-list-card {
 padding: 0 !important;
}
.dash-list-header {
 flex-direction: row;
 align-items: center;
 justify-content: space-between;
}
.dash-list-loading,
.dash-list-empty {
 padding: 24px 20px;
}
.dash-usage-rows {
 display: flex;
 flex-direction: column;
 padding: 6px 12px 12px;
}
.dash-usage-row {
 display: flex;
 align-items: center;
 justify-content: space-between;
 gap: 12px;
 padding: 10px 8px;
 border-radius: 10px;
 transition: background 0.15s ease;
}
.dash-usage-row:hover {
 background: color-mix(in oklch, var(--surface-secondary) 70%, transparent);
}
.dash-usage-row-left {
 display: flex;
 align-items: center;
 gap: 10px;
 min-width: 0;
}
.dash-brand-tile {
 flex: none;
 display: inline-flex;
 align-items: center;
 justify-content: center;
 width: 22px;
 height: 22px;
 border-radius: 7px;
 color: white;
 font-size: 11px;
 font-weight: 700;
 box-shadow: inset 0 0 0 1px color-mix(in oklch, white 14%, transparent);
}
.dash-usage-model {
 margin: 0;
 font-family: var(--font-mono);
 font-size: 12.5px;
 font-weight: 600;
 color: var(--foreground);
 overflow: hidden;
 text-overflow: ellipsis;
 white-space: nowrap;
}
.dash-usage-time {
 margin: 1px 0 0;
 font-size: 11.5px;
 color: var(--muted);
}
.dash-usage-row-right {
 flex: none;
 text-align: right;
}
.dash-usage-cost {
 margin: 0;
 font-size: 12.5px;
 font-weight: 600;
 font-variant-numeric: tabular-nums;
}
.dash-usage-actual {
 color: var(--success-text);
}
.dash-usage-standard {
 font-weight: 400;
 color: var(--muted);
}
.dash-usage-tokens {
 margin: 1px 0 0;
 font-size: 11.5px;
 color: var(--muted);
 font-variant-numeric: tabular-nums;
}
.dash-usage-viewall {
 display: flex;
 align-items: center;
 justify-content: center;
 gap: 6px;
 padding: 10px 0 2px;
 font-size: 12.5px;
 font-weight: 600;
 color: var(--accent);
 transition: color 0.15s ease;
}
.dash-usage-viewall:hover {
 color: var(--foreground);
}
</style>
