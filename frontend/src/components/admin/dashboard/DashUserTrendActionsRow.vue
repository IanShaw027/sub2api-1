<template>
  <div class="dash-row dash-row-trend">
    <section class="glass-card dash-panel">
      <header class="dash-panel-head">
        <div class="dash-panel-heading">
          <span class="dash-panel-title">{{ t('admin.dashboard.userUsageTrend') }}</span>
          <span class="dash-panel-sub">{{ t('admin.dashboard.recentUsage') }}</span>
        </div>
      </header>
      <div class="dash-user-trend">
        <div v-if="userTrendLoading" class="dash-panel-loading"><LoadingSpinner size="md" /></div>
        <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
        <div v-else class="empty-state dash-empty">
          <span class="empty-state-title">{{ t('admin.dashboard.noDataAvailable') }}</span>
        </div>
      </div>
    </section>

    <section class="glass-card dash-panel">
      <header class="dash-panel-head">
        <span class="dash-panel-title">{{ t('admin.dashboard.quickActions') }}</span>
      </header>
      <div class="dash-actions">
        <button
          v-if="canUseBatchImage"
          type="button"
          class="dash-action"
          @click="router.push('/batch-image')"
        >
          <span class="dash-action-icon"><Icon name="sparkles" size="md" :stroke-width="2" /></span>
          <span class="dash-action-body">
            <span class="dash-action-title">{{ t('admin.dashboard.batchImage') }}</span>
            <span class="dash-action-desc">{{ t('admin.dashboard.batchImageDesc') }}</span>
          </span>
          <Icon name="chevronRight" size="sm" class="text-muted" />
        </button>
        <button type="button" class="dash-action" @click="router.push('/admin/groups')">
          <span class="dash-action-icon"><Icon name="grid" size="md" :stroke-width="2" /></span>
          <span class="dash-action-body">
            <span class="dash-action-title">{{ t('admin.dashboard.groupPricing') }}</span>
            <span class="dash-action-desc">{{ t('admin.dashboard.groupPricingDesc') }}</span>
          </span>
          <Icon name="chevronRight" size="sm" class="text-muted" />
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
/**
 * Row 5 of the admin dashboard: the per-user token usage trend line chart
 * and the quick-actions shortcut list.
 */
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Line } from 'vue-chartjs'
import type { ChartData, ChartOptions } from 'chart.js'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  userTrendLoading: boolean
  userTrendChartData: ChartData<'line'> | null
  lineOptions: ChartOptions<'line'>
  canUseBatchImage: boolean
}>()

const { t } = useI18n()
const router = useRouter()
</script>

<style scoped>
.dash-row {
  display: grid;
  gap: 12px;
}

.dash-row-trend {
  grid-template-columns: 2fr 1fr;
}

.dash-panel {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.dash-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 28px;
}

.dash-panel-heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.dash-panel-title {
  font-size: var(--fs-14);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.dash-panel-sub {
  font-size: var(--fs-12);
  line-height: 1.3;
  color: var(--muted);
}

.dash-panel-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
}

.dash-empty {
  flex: 1;
  padding: 24px 16px;
}

/* ---------------------------- user trend / actions ---------------------------- */
.dash-user-trend {
  height: 256px;
  min-height: 0;
}

.dash-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dash-action {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: var(--radius-field);
  background: color-mix(in oklch, var(--surface-secondary) 70%, transparent);
  border: 1px solid color-mix(in oklch, var(--border) 60%, transparent);
  cursor: pointer;
  text-align: left;
}

.dash-action:hover {
  background: var(--surface-secondary);
}

.dash-action-icon {
  width: 40px;
  height: 40px;
  flex: none;
  border-radius: var(--radius-btn);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
}

.dash-action-body {
  min-width: 0;
  flex: 1;
}

.dash-action-title {
  display: block;
  font-size: var(--fs-14);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.dash-action-desc {
  display: block;
  font-size: var(--fs-12);
  line-height: 1.3;
  color: var(--muted);
}

/* ============================== responsive ============================== */
@media (max-width: 1180px) {
  .dash-row-trend {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767px) {
  .dash-panel {
    padding: 0;
    gap: 0;
  }
  .dash-panel-head {
    padding: 12px 14px 10px;
    border-bottom: 1px solid var(--border);
  }
  .dash-panel-title {
    font-size: var(--fs-13);
  }
  .dash-actions,
  .dash-user-trend,
  .dash-panel-loading,
  .dash-empty {
    margin: 0;
  }
  .dash-actions {
    padding: 12px 14px;
  }
  .dash-user-trend {
    padding: 12px 14px;
    height: 240px;
  }
  .dash-empty {
    margin: 14px;
  }
}
</style>
