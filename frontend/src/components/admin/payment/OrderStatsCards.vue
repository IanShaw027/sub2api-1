<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Today Revenue -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-[color-mix(in_oklch,var(--success)_16%,transparent)] p-2">
          <Icon name="dollar" size="md" class="text-green-600" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-muted">{{ t('payment.admin.todayRevenue') }}</p>
          <p v-for="[currency, amount] in sortedAmounts(stats.today_amount)" :key="currency" class="text-xl font-bold text-foreground">
            {{ formatMoney(currency, amount) }}
          </p>
          <p class="text-xs text-muted">
            {{ stats.today_count }} {{ t('payment.admin.orders') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Total Revenue -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-2">
          <Icon name="creditCard" size="md" class="text-accent" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-muted">{{ t('payment.admin.totalRevenue') }}</p>
          <p v-for="[currency, amount] in sortedAmounts(stats.total_amount)" :key="currency" class="text-xl font-bold text-foreground">
            {{ formatMoney(currency, amount) }}
          </p>
          <p class="text-xs text-muted">
            {{ stats.total_count }} {{ t('payment.admin.orders') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Today Orders -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-purple-100 p-2">
          <Icon name="chart" size="md" class="text-purple-600" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-muted">{{ t('payment.admin.todayOrders') }}</p>
          <p class="text-xl font-bold text-foreground">{{ stats.today_count }}</p>
        </div>
      </div>
    </div>

    <!-- Average Amount -->
    <div class="card p-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-amber-100 p-2">
          <Icon name="chart" size="md" class="text-warning-text" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-muted">{{ t('payment.admin.avgAmount') }}</p>
          <p v-for="[currency, amount] in sortedAmounts(stats.avg_amount)" :key="currency" class="text-xl font-bold text-foreground">
            {{ formatMoney(currency, amount) }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CurrencyAmounts, DashboardStats } from '@/types/payment'

const { t } = useI18n()

defineProps<{
  stats: DashboardStats
}>()

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}
</script>
