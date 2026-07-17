<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Today Revenue (success) -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="stat-icon shrink-0 bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-400">
          <Icon name="dollar" size="md" :stroke-width="2" />
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-ink-soft dark:text-dark-400">{{ t('payment.admin.todayRevenue') }}</p>
          <p class="text-xl font-bold tabular-nums text-ink dark:text-white">{{ formatMoney(stats.today_amount) }}</p>
          <p class="text-xs text-ink-soft dark:text-dark-400">
            {{ stats.today_count }} {{ t('payment.admin.orders') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Total Revenue (accent — admin finance) -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="stat-icon shrink-0 bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
          <Icon name="creditCard" size="md" :stroke-width="2" />
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-ink-soft dark:text-dark-400">{{ t('payment.admin.totalRevenue') }}</p>
          <p class="text-xl font-bold tabular-nums text-ink dark:text-white">{{ formatMoney(stats.total_amount) }}</p>
          <p class="text-xs text-ink-soft dark:text-dark-400">
            {{ stats.total_count }} {{ t('payment.admin.orders') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Today Orders (brand) -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="stat-icon shrink-0 bg-brand-50 text-brand-600 dark:bg-brand-950/40 dark:text-brand-300">
          <Icon name="chart" size="md" :stroke-width="2" />
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-ink-soft dark:text-dark-400">{{ t('payment.admin.todayOrders') }}</p>
          <p class="text-xl font-bold tabular-nums text-ink dark:text-white">{{ stats.today_count }}</p>
        </div>
      </div>
    </div>

    <!-- Average Amount (warning) -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="stat-icon shrink-0 bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-400">
          <Icon name="chart" size="md" :stroke-width="2" />
        </div>
        <div class="min-w-0">
          <p class="text-xs font-medium text-ink-soft dark:text-dark-400">{{ t('payment.admin.avgAmount') }}</p>
          <p class="text-xl font-bold tabular-nums text-ink dark:text-white">{{ formatMoney(stats.avg_amount) }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { DashboardStats } from '@/types/payment'

const { t } = useI18n()

defineProps<{
  stats: DashboardStats
}>()

function formatMoney(value: number): string {
  return value.toFixed(2)
}
</script>
