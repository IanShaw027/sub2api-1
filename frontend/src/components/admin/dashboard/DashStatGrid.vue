<template>
  <div class="dash-stat-grid">
    <StatCard
      :label="t('admin.dashboard.totalUsers')"
      :value="formatNumber(stats.total_users)"
      :sub="`${t('admin.dashboard.activeUsers')} ${formatNumber(stats.active_users)}`"
      :delta="t('admin.dashboard.todayNew', { count: formatNumber(stats.today_new_users) })"
      :delta-tone="stats.today_new_users > 0 ? 'up' : 'neutral'"
      :title="t('admin.dashboard.currentInventory')"
    >
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.totalApiKeys')"
      :value="formatNumber(stats.total_api_keys)"
      :sub="`${t('admin.dashboard.activeApiKeys')} ${formatNumber(stats.active_api_keys)}`"
      :delta="t('admin.dashboard.activeRatio', { rate: activeKeyRatio })"
      delta-tone="neutral"
      :title="t('admin.dashboard.currentInventory')"
    >
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.totalAccounts')"
      :value="formatNumber(stats.total_accounts)"
      :sub="accountsSubLabel"
      :delta="accountsDelta.text"
      :delta-tone="accountsDelta.tone"
      :title="t('admin.dashboard.currentInventory')"
    >
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.periodRequests')"
      :title="rangeLabel"
      :value="formatNumber(stats.total_requests)"
      :sub="`${t('admin.dashboard.todayRequests')} ${formatNumber(stats.today_requests)}`"
      :delta="requestsDelta?.text"
      :delta-tone="requestsDelta?.tone ?? 'neutral'"
    >
      <template v-if="requestsSpark.line" #sparkline><DashSparkline :path="requestsSpark" /></template>
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.todayTokens')"
      :value="formatTokens(stats.today_tokens)"
      :sub="t('admin.dashboard.cacheHitRate', { rate: cacheHitRate })"
    >
      <template #sparkline>
        <DashBreakdownSpark :items="todayTokenBreakdownItems" :format="formatTokens" unit="" />
      </template>
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.periodTokens')"
      :title="rangeLabel"
      :value="formatTokens(stats.total_tokens)"
      :sub="`${t('admin.dashboard.input')} ${formatTokens(stats.total_input_tokens)}`"
      :delta="tokensDelta?.text"
      :delta-tone="tokensDelta?.tone ?? 'neutral'"
    >
      <template #sparkline>
        <DashBreakdownSpark :path="tokensSpark" :items="totalTokenBreakdownItems" :format="formatTokens" unit="" />
      </template>
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.todayCost')"
      :value="`$${formatCost(stats.today_actual_cost)}`"
      :sub="`${t('admin.dashboard.standard')} $${formatCost(stats.today_cost)}`"
    >
      <template #sparkline>
        <DashBreakdownSpark :items="todayFinancialBreakdownItems" :format="formatCost" />
      </template>
    </StatCard>

    <StatCard
      :label="t('admin.dashboard.periodCost')"
      :title="rangeLabel"
      :value="`$${formatCost(stats.total_actual_cost)}`"
      :sub="`${t('admin.dashboard.standard')} $${formatCost(stats.total_cost)}`"
      :delta="costDelta?.text"
      :delta-tone="costDelta?.tone ?? 'neutral'"
    >
      <template #sparkline>
        <DashBreakdownSpark :path="costSpark" :items="totalFinancialBreakdownItems" :format="formatCost" />
      </template>
    </StatCard>
  </div>
</template>

<script setup lang="ts">
/**
 * The 8-tile summary stat grid at the top of the admin dashboard. All values
 * and derived deltas/breakdowns are computed by the view and passed in.
 */
import { useI18n } from 'vue-i18n'
import type { DashboardStats } from '@/types'
import StatCard from '@/components/ui/StatCard.vue'
import DashSparkline from './DashSparkline.vue'
import DashBreakdownSpark from './DashBreakdownSpark.vue'
import type { BreakdownItem, SparkPath } from './types'
import { formatCost, formatNumber, formatTokens } from './useDashboardFormat'

type Delta = { text: string; tone: 'up' | 'down' } | null

defineProps<{
  stats: DashboardStats
  rangeLabel: string
  requestsSpark: SparkPath
  tokensSpark: SparkPath
  costSpark: SparkPath
  activeKeyRatio: string
  cacheHitRate: string
  accountsSubLabel: string
  accountsDelta: { text: string; tone: 'up' | 'down' | 'neutral' }
  requestsDelta: Delta
  tokensDelta: Delta
  costDelta: Delta
  todayTokenBreakdownItems: BreakdownItem[]
  totalTokenBreakdownItems: BreakdownItem[]
  todayFinancialBreakdownItems: BreakdownItem[]
  totalFinancialBreakdownItems: BreakdownItem[]
}>()

const { t } = useI18n()
</script>

<style scoped>
.dash-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.dash-stat-grid :deep(.ui-stat-card-sparkline > div) {
  margin-left: 0;
  width: 100%;
  height: 100%;
}

@media (max-width: 1180px) {
  .dash-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 767px) {
  .dash-stat-grid {
    gap: 10px;
  }
  .dash-stat-grid :deep(.ui-stat-card-value) {
    font-size: var(--fs-24);
  }
  .dash-stat-grid :deep(.ui-stat-card-label) {
    font-size: var(--fs-11-5);
  }
  .dash-stat-grid :deep(.ui-stat-card-delta) {
    font-size: var(--fs-10-5);
    padding: 2px 5px;
    border-radius: var(--radius-5);
  }
  .dash-stat-grid :deep(.ui-stat-card-sub) {
    display: none;
  }
  .dash-stat-grid :deep(.ui-stat-card-sparkline) {
    width: 100%;
    height: 24px;
  }
}
</style>
